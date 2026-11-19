"""The yfinance call for one pair, as an `api.Fetcher`; a failure is an error code."""

from __future__ import annotations

import datetime
import math
import sys
from decimal import ROUND_HALF_UP, Decimal
from typing import Any

from optdesk.refbars.api import Bar, Fetcher, PairResult, PairSpec
from optdesk.refbars.symbols import map_pair

TIMEOUT_SECONDS = 10
_EXPECTED_COLUMNS = {"Open", "High", "Low", "Close", "Adj Close", "Volume"}
_PLACES = Decimal("0.0001")


def close_text(value: float) -> str:
    """Return the close as decimal text of 4 places, without the float artefact."""
    return str(Decimal(repr(float(value))).quantize(_PLACES, rounding=ROUND_HALF_UP))


def _code_of(yf: Any, exc: Exception) -> str:
    errors = yf.exceptions
    if isinstance(exc, errors.YFRateLimitError):
        return "rate_limited"
    if isinstance(exc, (errors.YFPricesMissingError, errors.YFTzMissingError)):
        return "no_data"
    names = {cls.__name__ for cls in type(exc).__mro__}
    if isinstance(exc, TimeoutError) or "Timeout" in names:
        return "timeout"
    if isinstance(exc, OSError) or "ConnectionError" in names:
        return "network"
    return "source_changed"


def _bars(frame: Any, zone: datetime.tzinfo, from_day: str, to_day: str) -> list[Bar] | None:
    """Read the closes; None when the column set is not the expected one."""
    if not _EXPECTED_COLUMNS.issubset(set(frame.columns)):
        return None
    bars: list[Bar] = []
    for stamp, value in frame["Close"].items():
        if value is None or math.isnan(float(value)):
            continue
        if stamp.tzinfo is not None:
            stamp = stamp.astimezone(zone)
        day = stamp.date().isoformat()
        if from_day <= day <= to_day:
            bars.append({"session_day": day, "close": close_text(float(value))})
    bars.sort(key=lambda bar: bar["session_day"])
    return bars


def make_fetcher(yf: Any) -> Fetcher:
    """Return a fetcher over the yfinance module (or an object shaped like it); no retry."""

    def fetch(pair: PairSpec, from_day: str, to_day: str) -> PairResult:
        result: PairResult = {
            "symbol": pair["symbol"],
            "currency": pair["currency"],
            "error_code": "",
            "bars": [],
        }
        target = map_pair(pair["symbol"], pair["currency"])
        if target is None:
            result["error_code"] = "no_data"
            return result
        end = datetime.date.fromisoformat(to_day) + datetime.timedelta(days=1)
        debug = yf.config.debug
        previous = debug.hide_exceptions
        debug.hide_exceptions = False
        try:
            frame = yf.Ticker(target.yahoo_symbol).history(
                start=from_day,
                end=end.isoformat(),
                interval="1d",
                auto_adjust=False,
                actions=False,
                timeout=TIMEOUT_SECONDS,
            )
        except Exception as exc:  # noqa: BLE001 - every failure becomes a code
            result["error_code"] = _code_of(yf, exc)
            if result["error_code"] == "source_changed":
                print(f"refbars: unexpected {type(exc).__name__}", file=sys.stderr)
            return result
        finally:
            debug.hide_exceptions = previous
        if frame is None or frame.empty:
            result["error_code"] = "no_data"
            return result
        bars = _bars(frame, target.zone, from_day, to_day)
        if bars is None:
            result["error_code"] = "source_changed"
        elif not bars:
            result["error_code"] = "no_data"
        else:
            result["bars"] = bars
        return result

    return fetch
