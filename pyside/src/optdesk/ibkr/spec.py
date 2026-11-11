"""Request specification of each quote kind, status rules and error codes.

One table keyed by the kind. The values are the ones written in the comment of
quote_types.proto; a live measurement corrects them here and there. No
strategy lives here: no strike choice, no expiry window, no price arithmetic.
This file has no third-party imports and never logs.
"""

from __future__ import annotations

from collections.abc import Awaitable, Callable, Mapping, Sequence
from dataclasses import dataclass
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from optdesk.ibkr.model import QuoteTarget
    from optdesk.ibkr.timing import Clock

OPTION_EOD = "option_eod"
OPTION_OPEN = "option_open"
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
CODE_CONNECTIVITY_LOST = 1100
CODE_DATA_LOST = 1101  # connectivity restored, market data lost: resubscribe
CODE_DATA_KEPT = 1102  # connectivity restored, market data kept
CODE_SERVER_LOST = 2110  # connectivity between TWS and the server is broken
CODE_COMPETING_SESSION = 10197  # another login took the market data

# The 10197 probe: wait each interval in turn, then probe one underlying and one
# option for PROBE_WAIT_S; the sum of the intervals is the deadline of the wait.
PROBE_INTERVALS_S = (5.0, 10.0, 20.0)
PROBE_WAIT_S = 5.0
SESSION_CODES = frozenset(
    {
        CODE_CONNECTIVITY_LOST,
        CODE_DATA_LOST,
        CODE_DATA_KEPT,
        CODE_SERVER_LOST,
        CODE_COMPETING_SESSION,
    }
)


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
    # The open pull: no close, no open-interest grace.
    OPTION_OPEN: KindSpec(
        generic_ticks="100,101",
        release_fields=frozenset({"bid", "ask", *_MODEL}),
        wanted_fields=("bid", "ask", "close", "volume", "open_interest", *_MODEL),
        timeout_s=15,
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


_UNDERLYING_TYPES = frozenset({"STK", "IND"})


def probe_targets(targets: Sequence[QuoteTarget]) -> list[QuoteTarget]:
    """Pick the 10197 probe: the first underlying and the first option of targets.

    A batch with only one of the two probes that one; the order of targets keeps.
    """
    under = next((t for t in targets if t.sec_type in _UNDERLYING_TYPES), None)
    option = next((t for t in targets if t.sec_type not in _UNDERLYING_TYPES), None)
    return [t for t in (under, option) if t is not None]


async def probe_until_back(clock: Clock, probe: Callable[[], Awaitable[bool]]) -> bool:
    """After a 10197: wait each of PROBE_INTERVALS_S, then probe; True when one answers.

    False is the deadline, the end of the last probe.
    """
    for interval in PROBE_INTERVALS_S:
        await clock.sleep(interval)
        if await probe():
            return True
    return False


async def look_until(clock: Clock, answered: Callable[[], bool]) -> bool:
    """Look once a second for PROBE_WAIT_S; True as soon as answered() is."""
    for _ in range(int(PROBE_WAIT_S)):
        await clock.sleep(1)
        if answered():
            return True
    return False
