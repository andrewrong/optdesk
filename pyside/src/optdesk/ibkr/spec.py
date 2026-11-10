"""Request specification of each quote kind, status rules and error codes.

One table keyed by the kind. The values are the ones written in the comment of
quote_types.proto; a live measurement corrects them here and there. No
strategy lives here: no strike choice, no expiry window, no price arithmetic.
This file has no third-party imports and never logs.
"""

from __future__ import annotations

from collections.abc import Mapping
from dataclasses import dataclass

OPTION_EOD = "option_eod"
UNDERLYING_IV = "underlying_iv"
SNAPSHOT = "snapshot"
DIVIDENDS = "dividends"

# Seconds an OPTION_EOD quote waits for open_interest after its release fields.
OI_GRACE_S = 5.0

STATUS_OK = "ok"
STATUS_PARTIAL = "partial"
STATUS_NO_DATA = "no_data"
STATUS_NO_SUBSCRIPTION = "no_subscription"
STATUS_ERROR = "error"
STATUS_NO_VALUE = "no_value"
STATUS_TIMEOUT = "timeout"
STATUS_DISCONNECTED = "disconnected"

CODE_NOT_LISTED = 200
CODE_LINE_LIMIT = 101
CODE_NO_SUBSCRIPTION = 354


@dataclass(frozen=True)
class KindSpec:
    """How one kind of quote is requested and when it is complete."""

    generic_ticks: str
    release_fields: frozenset[str]
    wanted_fields: tuple[str, ...]
    timeout_s: float
    # Fields that arrive after the release fields: the line is kept at most
    # grace_s seconds for them, and a quote without them ends PARTIAL.
    grace_fields: frozenset[str] = frozenset()
    grace_s: float = 0.0


_MODEL = ("implied_vol", "delta", "gamma", "vega", "theta", "und_price")

TABLE: Mapping[str, KindSpec] = {
    OPTION_EOD: KindSpec(
        generic_ticks="100,101",
        release_fields=frozenset({"bid", "ask", "close", *_MODEL}),
        wanted_fields=("bid", "ask", "close", "volume", "open_interest", *_MODEL),
        timeout_s=20,
        grace_fields=frozenset({"open_interest"}),
        grace_s=OI_GRACE_S,
    ),
    UNDERLYING_IV: KindSpec(
        generic_ticks="104,106",
        release_fields=frozenset({"option_implied_vol", "historical_vol"}),
        wanted_fields=("option_implied_vol", "historical_vol"),
        timeout_s=20,
    ),
    SNAPSHOT: KindSpec(
        generic_ticks="",
        release_fields=frozenset({"bid", "ask"}),
        wanted_fields=(
            "bid",
            "ask",
            "last",
            "bid_size",
            "ask_size",
            "last_size",
            "close",
            "volume",
            "event_ts",
        ),
        timeout_s=20,
    ),
    # Tick 456 arrives once as one text of four values; the marker field is
    # set when it has arrived, whatever it holds.
    DIVIDENDS: KindSpec(
        generic_ticks="456",
        release_fields=frozenset({"dividend_tick"}),
        wanted_fields=("dividend_tick",),
        timeout_s=10,
    ),
}


class UnknownKind(ValueError):
    """The kind is not a row of the table."""


class AmbiguousResolve(Exception):
    """A resolve returned several results; nothing is cached for it."""


class BrokerError(Exception):
    """The gateway answered a request with an error code that has no rule."""

    def __init__(self, code: int, text: str) -> None:
        super().__init__(f"broker error {code}: {text}")
        self.code = code
        self.text = text


def kind_spec(kind: str) -> KindSpec:
    """Return the row of a kind or raise UnknownKind."""
    try:
        return TABLE[kind]
    except KeyError:
        raise UnknownKind(kind) from None


def is_complete(spec: KindSpec, got: Mapping[str, float]) -> bool:
    """True when every field that releases the line has arrived."""
    return spec.release_fields <= got.keys()


def quote_status(spec: KindSpec, got: Mapping[str, float]) -> str:
    """Status of a quote that ended without an error: OK, PARTIAL or NO_DATA."""
    if is_complete(spec, got):
        if spec.grace_fields <= got.keys():
            return STATUS_OK
        return STATUS_PARTIAL
    if any(name in got for name in spec.wanted_fields):
        return STATUS_PARTIAL
    return STATUS_NO_DATA


@dataclass(frozen=True)
class ErrorRule:
    """What an IBKR error code means for a request."""

    not_listed: bool = False
    shrink_budget: bool = False
    quote_status: str = STATUS_ERROR


def classify_error(code: int) -> ErrorRule:
    """Map an IBKR error code to its rule; other codes are errors, code kept."""
    if code == CODE_NOT_LISTED:
        return ErrorRule(not_listed=True)
    if code == CODE_LINE_LIMIT:
        return ErrorRule(shrink_budget=True)
    if code == CODE_NO_SUBSCRIPTION:
        return ErrorRule(quote_status=STATUS_NO_SUBSCRIPTION)
    return ErrorRule()


def is_informational(code: int) -> bool:
    """Codes that report a state of the gateway, not the failure of a request."""
    return 2100 <= code < 2200 or code in (10090, 10167)
