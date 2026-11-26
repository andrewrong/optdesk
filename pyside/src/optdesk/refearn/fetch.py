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


def make_fetcher(calendar_of: Callable[[str], Any]) -> Fetcher:
    """Return a fetcher over `calendar_of(yahoo_symbol)`, which gives `Ticker.calendar`; no retry."""

    def fetch(symbol: SymbolSpec, from_day: str) -> SymbolResult:
        result: SymbolResult = {
            "symbol": symbol["symbol"],
            "currency": symbol["currency"],
            "error_code": "",
            "dates": [],
        }
        target = map_pair(symbol["symbol"], symbol["currency"])
        if target is None:
            result["error_code"] = "no_data"
            return result
        try:
            calendar = calendar_of(target.yahoo_symbol)
        except Exception as exc:  # noqa: BLE001 - every failure becomes a code
            result["error_code"] = _code_of(exc)
            if result["error_code"] == "source_changed":
                print(f"refearn: unexpected {type(exc).__name__}", file=sys.stderr)
            return result
        result["error_code"], result["dates"] = _dates(calendar, from_day)
        return result

    return fetch
