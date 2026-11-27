"""The earnings dates of one symbol out of the Yahoo calendar, and the error codes."""

from __future__ import annotations

import datetime
from typing import Any

from optdesk.refearn.api import SymbolSpec
from optdesk.refearn.fetch import make_fetcher

D = datetime.date
US: SymbolSpec = {"symbol": "BRK.B", "currency": "USD"}


class RateLimitError(Exception):
    pass


class ConnectionError(Exception):
    pass


def _fetch(outcome: Any, spec: SymbolSpec = US, from_day: str = "2026-11-01") -> Any:
    asked: list[str] = []

    def calendar_of(yahoo_symbol: str) -> Any:
        asked.append(yahoo_symbol)
        if isinstance(outcome, Exception):
            raise outcome
        return outcome

    result = make_fetcher(calendar_of)(spec, from_day)
    return result, asked


def test_one_date_calendar() -> None:
    result, asked = _fetch({"Earnings Date": [D(2026, 12, 5)]})
    assert asked == ["BRK-B"]
    assert result == {
        "symbol": "BRK.B",
        "currency": "USD",
        "error_code": "",
        "dates": ["2026-12-05"],
    }


def test_two_date_range_is_ascending() -> None:
    result, _ = _fetch({"Earnings Date": [D(2026, 12, 9), D(2026, 12, 5)]})
    assert result["dates"] == ["2026-12-05", "2026-12-09"]


def test_date_before_from_day_is_dropped() -> None:
    result, _ = _fetch({"Earnings Date": [D(2026, 10, 30), D(2026, 11, 1)]})
    assert result["dates"] == ["2026-11-01"]


def test_only_past_dates_is_no_data() -> None:
    result, _ = _fetch({"Earnings Date": [D(2026, 10, 30)]})
    assert (result["error_code"], result["dates"]) == ("no_data", [])


def test_empty_calendar_missing_key_and_empty_list_are_no_data() -> None:
    calendars: list[Any] = [
        {},
        {"Dividend Date": D(2026, 12, 1)},
        {"Earnings Date": []},
    ]
    for calendar in calendars:
        result, _ = _fetch(calendar)
        assert (result["error_code"], result["dates"]) == ("no_data", [])


def test_jpy_symbol_and_no_route() -> None:
    result, asked = _fetch(
        {"Earnings Date": [D(2026, 12, 5)]}, {"symbol": "7203", "currency": "JPY"}
    )
    assert asked == ["7203.T"] and result["error_code"] == ""
    result, asked = _fetch(
        {"Earnings Date": [D(2026, 12, 5)]}, {"symbol": "X", "currency": "EUR"}
    )
    assert asked == [] and result["error_code"] == "no_data"


def test_exceptions_map_to_codes() -> None:
    class YFRateLimitError(Exception):
        pass

    class ReadTimeout(Exception):
        pass

    cases = [
        (YFRateLimitError(), "rate_limited"),
        (TimeoutError(), "timeout"),
        (ReadTimeout(), "timeout"),
        (ConnectionError(), "network"),
        (OSError(), "network"),
        (ValueError(), "source_changed"),
    ]
    for exc, code in cases:
        result, _ = _fetch(exc)
        assert (result["error_code"], result["dates"]) == (code, [])


def test_unexpected_shape_is_source_changed() -> None:
    calendars: list[Any] = [
        {"Earnings Date": "2026-12-05"},
        {"Earnings Date": ["2026-12-05"]},
        {"Earnings Date": [D(2026, 12, 5), None]},
        {"Earnings Date": D(2026, 12, 5)},
        ["Earnings Date"],
    ]
    for calendar in calendars:
        result, _ = _fetch(calendar)
        assert (result["error_code"], result["dates"]) == ("source_changed", [])


def _fetch_both(
    calendar: Any, days: Any, spec: SymbolSpec = US, from_day: str = "2026-11-27"
) -> Any:
    asked: list[str] = []

    def calendar_of(yahoo_symbol: str) -> Any:
        asked.append(yahoo_symbol)
        return calendar

    return make_fetcher(calendar_of, lambda _s: days)(spec, from_day), asked


class TestCalendarBeforeFromDayUsesEarningsDates:
    def test_later_days_ascending(self) -> None:
        result, _ = _fetch_both(
            {"Earnings Date": [D(2026, 11, 18)]},
            [D(2027, 2, 24), D(2026, 11, 18), D(2027, 2, 20)],
        )
        assert result["error_code"] == ""
        assert result["dates"] == ["2027-02-20", "2027-02-24"]


class TestCalendarAndDatesBothBeforeFromDay:
    def test_no_data(self) -> None:
        result, _ = _fetch_both({"Earnings Date": [D(2026, 11, 18)]}, [D(2026, 11, 6)])
        assert (result["error_code"], result["dates"]) == ("no_data", [])


class TestSpaceInSymbolAsksDash:
    def test_brk_b(self) -> None:
        _, asked = _fetch_both({}, [], {"symbol": "BRK B", "currency": "USD"})
        assert asked == ["BRK-B"]
