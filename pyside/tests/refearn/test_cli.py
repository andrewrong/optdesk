"""The command: one request in, one answer out, the rate stop and the deadline."""

from __future__ import annotations

import datetime
import io
import json
from types import SimpleNamespace
from typing import Any

from optdesk.refearn.cli import run


class _Clock:
    def __init__(self) -> None:
        self.now = 0.0

    def monotonic(self) -> float:
        return self.now

    def sleep(self, seconds: float) -> None:
        self.now += seconds


class _Yf:
    """Stands in for the yfinance module; `calendars` maps a Yahoo symbol to a calendar or error."""

    def __init__(self, calendars: dict[str, Any]) -> None:
        self.calendars = calendars
        self.config = SimpleNamespace(debug=SimpleNamespace(hide_exceptions=True))

    def set_tz_cache_location(self, path: str) -> None:
        pass

    def Ticker(self, symbol: str) -> Any:
        outcome = self.calendars[symbol]
        if isinstance(outcome, Exception):
            raise outcome
        return SimpleNamespace(calendar=outcome)


class YFRateLimitError(Exception):
    pass


def _run(
    request: Any, yf: _Yf, argv: tuple[str, ...] = ("--deadline", "60")
) -> tuple[int, str, str]:
    out, err = io.StringIO(), io.StringIO()
    code = run(list(argv), io.StringIO(json.dumps(request)), out, err, _Clock(), yf)
    return code, out.getvalue(), err.getvalue()


def _spec(symbol: str) -> dict[str, str]:
    return {"symbol": symbol, "currency": "USD"}


def test_round_trip_one_answer_in_order() -> None:
    yf = _Yf({"AAPL": {"Earnings Date": [datetime.date(2026, 12, 5)]}, "MSFT": {}})
    code, out, _ = _run(
        {"symbols": [_spec("AAPL"), _spec("MSFT")], "from_day": "2026-11-01"}, yf
    )
    assert code == 0
    assert out.count("\n") == 1
    assert json.loads(out) == {
        "results": [
            {
                "symbol": "AAPL",
                "currency": "USD",
                "error_code": "",
                "dates": ["2026-12-05"],
            },
            {"symbol": "MSFT", "currency": "USD", "error_code": "no_data", "dates": []},
        ]
    }


def test_rate_limit_stops_the_rest_and_still_exits_0() -> None:
    yf = _Yf({"A": YFRateLimitError(), "B": {}})
    code, out, _ = _run(
        {"symbols": [_spec("A"), _spec("B")], "from_day": "2026-11-01"}, yf
    )
    assert code == 0
    assert [r["error_code"] for r in json.loads(out)["results"]] == [
        "rate_limited",
        "rate_limited",
    ]


def test_deadline_gives_timeout_for_the_symbols_not_reached() -> None:
    yf = _Yf({"A": {}, "B": {}})
    code, out, _ = _run(
        {"symbols": [_spec("A"), _spec("B")], "from_day": "2026-11-01"},
        yf,
        ("--deadline", "0.5"),
    )
    assert code == 0
    assert [r["error_code"] for r in json.loads(out)["results"]] == [
        "no_data",
        "timeout",
    ]


def test_bad_request_exits_2_with_no_answer() -> None:
    requests: list[Any] = [
        {"symbols": []},
        {"symbols": [_spec("A")] * 11, "from_day": "2026-11-01"},
    ]
    for request in requests:
        code, out, _ = _run(request, _Yf({}))
        assert (code, out) == (2, "")
