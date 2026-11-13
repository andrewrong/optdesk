"""Why each field of an option quote is missing: a pure function, no I/O.

OPTION_EOD and OPTION_OPEN quotes list every field of bid, ask, last, volume,
open_interest, implied_vol, delta, gamma, vega and theta that was not
delivered with a reason; other kinds list nothing. The reason numbers are
those of the proto enum MissingReason (docs/design/option-field-reasons.md).
This file also holds the readers of a ticker value that ibgateway shares, so
it imports nothing from ibgateway.
"""

from __future__ import annotations

import math
from collections.abc import Mapping
from typing import Any

from ib_async.util import UNSET_DOUBLE

from optdesk.ibkr import spec

NOT_RECEIVED_RELEASE = 1
NOT_RECEIVED_GRACE = 2
INVALID = 3
ERROR = 4
RECEIVED_ZERO = 5
MARKET_DATA_TYPE = 6

_LIVE = 1
_INTEREST = ("callOpenInterest", "putOpenInterest")
_GREEKS = {
    "implied_vol": "impliedVol",
    "delta": "delta",
    "gamma": "gamma",
    "vega": "vega",
    "theta": "theta",
}
_SIGNED = frozenset({"delta", "gamma", "vega", "theta"})
_TICKER_ATTRIBUTES = {
    "bid": ("bid",),
    "ask": ("ask",),
    "last": ("last",),
    "volume": ("volume", "callVolume", "putVolume"),
    "open_interest": _INTEREST,
}
_FIELDS = (
    "bid",
    "ask",
    "last",
    "volume",
    "open_interest",
    "implied_vol",
    "delta",
    "gamma",
    "vega",
    "theta",
)


def _number(value: object, *, signed: bool = True) -> float | None:
    """A number, or None for NaN, UNSET_DOUBLE and, unless signed, a negative."""
    if isinstance(value, bool) or not isinstance(value, int | float):
        return None
    if math.isnan(value) or value == UNSET_DOUBLE or (value < 0 and not signed):
        return None
    return float(value)


def _open_interest(ticker: Any) -> float | None:
    """The first of call and put open interest that is above zero, else None.

    The target carries no right, so either field may be the one IB fills; a
    0.0 can be a placeholder, so it counts as absent.
    """
    for name in _INTEREST:
        number = _number(getattr(ticker, name, None))
        if number is not None and number > 0:
            return number
    return None


def _raw(ticker: Any, field: str) -> list[object]:
    if field in _GREEKS:
        greeks = getattr(ticker, "modelGreeks", None)
        return [getattr(greeks, _GREEKS[field], None)] if greeks is not None else []
    return [getattr(ticker, name, None) for name in _TICKER_ATTRIBUTES[field]]


def _invalid(ticker: Any, field: str) -> bool:
    """The ticker holds a value _number drops that is neither NaN nor unset."""
    for value in _raw(ticker, field):
        if isinstance(value, bool) or not isinstance(value, int | float):
            continue
        if math.isnan(value) or value == UNSET_DOUBLE:
            continue
        if _number(value, signed=field in _SIGNED) is None:
            return True
    return False


def _received_zero(ticker: Any) -> bool:
    """Both call and put open interest are 0.0; one 0.0 is a placeholder."""
    values = [getattr(ticker, name, None) for name in _INTEREST]
    return all(
        isinstance(v, int | float) and not isinstance(v, bool) and v == 0.0
        for v in values
    )


def missing_fields(
    kind_spec: spec.KindSpec,
    ticker: Any,
    got: Mapping[str, float],
    error_code: int,
    grace_ended: bool,
) -> tuple[tuple[str, int, int], ...]:
    """(field, reason, error_code) for each listed field that is not in got."""
    if kind_spec not in (spec.TABLE[spec.OPTION_EOD], spec.TABLE[spec.OPTION_OPEN]):
        return ()
    stale = int(getattr(ticker, "marketDataType", _LIVE)) != _LIVE
    rows: list[tuple[str, int, int]] = []
    for field in _FIELDS:
        if field in got:
            continue
        if error_code != 0:
            rows.append((field, ERROR, error_code))
        elif stale:
            rows.append((field, MARKET_DATA_TYPE, 0))
        elif field == "open_interest" and _received_zero(ticker):
            rows.append((field, RECEIVED_ZERO, 0))
        elif field in kind_spec.grace_fields and grace_ended:
            rows.append((field, NOT_RECEIVED_GRACE, 0))
        elif _invalid(ticker, field):
            rows.append((field, INVALID, 0))
        else:
            rows.append((field, NOT_RECEIVED_RELEASE, 0))
    return tuple(rows)
