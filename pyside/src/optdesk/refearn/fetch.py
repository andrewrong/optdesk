"""The Yahoo calendar of one symbol, as an `api.Fetcher`; a failure is an error code."""

from __future__ import annotations

import datetime
import sys
from collections.abc import Callable
from typing import Any

from optdesk.refbars.symbols import map_pair
from optdesk.refearn.api import Fetcher, SymbolResult, SymbolSpec

KEY = "Earnings Date"


def _code_of(exc: Exception) -> str:
    names = {cls.__name__ for cls in type(exc).__mro__}
    if "YFRateLimitError" in names:
        return "rate_limited"
    if isinstance(exc, TimeoutError) or any("Timeout" in name for name in names):
        return "timeout"
    if isinstance(exc, OSError) or any("ConnectionError" in name for name in names):
        return "network"
    return "source_changed"


def _failure_code(exc: Exception) -> str:
    code = _code_of(exc)
    if code == "source_changed":
        print(f"refearn: unexpected {type(exc).__name__}", file=sys.stderr)
    return code


def _dates(calendar: Any, from_day: str) -> tuple[str, list[str]]:
    """Return (error code, dates) of a calendar; the code is empty when there are dates."""
    if not isinstance(calendar, dict):
        return "source_changed", []
    if KEY not in calendar:
        return "no_data", []
    value = calendar[KEY]
    if not isinstance(value, list):
        return "source_changed", []
    if not all(
        isinstance(d, datetime.date) and not isinstance(d, datetime.datetime)
        for d in value
    ):
        return "source_changed", []
    days = sorted({d.isoformat() for d in value if d.isoformat() >= from_day})
    return ("", days) if days else ("no_data", [])


def _later_dates(days: Any, from_day: str) -> tuple[str, list[str]]:
    """Return (error code, dates) of a list of dates; only days on or after from_day count."""
    if not isinstance(days, list) or not all(
        isinstance(d, datetime.date) and not isinstance(d, datetime.datetime)
        for d in days
    ):
        return "source_changed", []
    later = sorted({d.isoformat() for d in days if d.isoformat() >= from_day})
    return ("", later) if later else ("no_data", [])


def make_fetcher(
    calendar_of: Callable[[str], Any],
    dates_of: Callable[[str], Any] | None = None,
) -> Fetcher:
    """Return a fetcher over `calendar_of(yahoo_symbol)`, which gives `Ticker.calendar`; no retry.

    When the calendar has no date on or after from_day and `dates_of(yahoo_symbol)` is given,
    its list of the next earnings dates is read and the days on or after from_day
    are answered. A space in a symbol is read as a dot.
    """

    def fetch(symbol: SymbolSpec, from_day: str) -> SymbolResult:
        result: SymbolResult = {
            "symbol": symbol["symbol"],
            "currency": symbol["currency"],
            "error_code": "",
            "dates": [],
        }
        target = map_pair(symbol["symbol"].replace(" ", "."), symbol["currency"])
        if target is None:
            result["error_code"] = "no_data"
            return result
        try:
            calendar = calendar_of(target.yahoo_symbol)
        except Exception as exc:  # noqa: BLE001 - every failure becomes a code
            result["error_code"] = _failure_code(exc)
            return result
        result["error_code"], result["dates"] = _dates(calendar, from_day)
        if result["error_code"] == "no_data" and dates_of is not None:
            try:
                days = dates_of(target.yahoo_symbol)
            except Exception as exc:  # noqa: BLE001 - every failure becomes a code
                result["error_code"] = _failure_code(exc)
                return result
            result["error_code"], result["dates"] = _later_dates(days, from_day)
        return result

    return fetch
