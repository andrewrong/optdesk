"""The command: request in, answer out, the rate stop and the deadline."""

from __future__ import annotations

import datetime
import io
import json
import os
from pathlib import Path
from types import SimpleNamespace
from typing import Any
from zoneinfo import ZoneInfo

from optdesk.refbars.cli import run

NY = ZoneInfo("America/New_York")
COLUMNS = ["Open", "High", "Low", "Close", "Adj Close", "Volume"]
GOLDEN = Path(__file__).parent / "answer.json"


class _RateLimit(Exception):
    pass


class _Missing(Exception):
    pass


class _Errors:
    YFRateLimitError = _RateLimit
    YFPricesMissingError = _Missing
    YFTzMissingError = _Missing


class _Series:
    def __init__(self, rows: list[tuple[datetime.datetime, float]]) -> None:
        self._rows = rows

    def items(self) -> list[tuple[datetime.datetime, float]]:
        return self._rows


class _Frame:
    def __init__(self, rows: list[tuple[datetime.datetime, float]]) -> None:
        self._rows = rows
        self.columns = COLUMNS
        self.empty = not rows

    def __getitem__(self, name: str) -> _Series:
        return _Series(self._rows)


class _Clock:
    def __init__(self) -> None:
        self.now = 0.0
        self.slept: list[float] = []

    def monotonic(self) -> float:
        return self.now

    def sleep(self, seconds: float) -> None:
        self.slept.append(seconds)
        self.now += seconds


class _Yf:
    def __init__(self, clock: _Clock, outcomes: dict[str, Any], cost: float = 0.0) -> None:
        self.clock = clock
        self.outcomes = outcomes
        self.cost = cost
        self.config = SimpleNamespace(debug=SimpleNamespace(hide_exceptions=True))
        self.exceptions = _Errors
        self.cache: list[str] = []
        self.cache_existed: list[bool] = []
        self.asked: list[str] = []

    def set_tz_cache_location(self, path: str) -> None:
        self.cache.append(path)
        self.cache_existed.append(os.path.isdir(path))

    def Ticker(self, symbol: str) -> Any:
        yf = self

        class _Ticker:
            def history(self, **kwargs: Any) -> Any:
                yf.asked.append(symbol)
                yf.clock.now += yf.cost
                outcome = yf.outcomes[symbol]
                if isinstance(outcome, Exception):
                    raise outcome
                return outcome

        return _Ticker()


def _request(*pairs: tuple[str, str]) -> str:
    return json.dumps(
        {
            "pairs": [{"symbol": s, "currency": c} for s, c in pairs],
            "from_day": "2026-11-16",
            "to_day": "2026-11-17",
        }
    )


def _go(text: str, yf: _Yf, clock: _Clock, deadline: str = "100") -> tuple[int, str, str]:
    out, err = io.StringIO(), io.StringIO()
    code = run(["--deadline", deadline], io.StringIO(text), out, err, clock, yf)
    return code, out.getvalue(), err.getvalue()


def test_answer_matches_golden() -> None:
    golden = json.loads(GOLDEN.read_text(encoding="utf-8"))
    clock = _Clock()
    yf = _Yf(
        clock,
        {
            "AAPL": _Frame(
                [
                    (datetime.datetime(2026, 11, 16, tzinfo=NY), 189.5),
                    (datetime.datetime(2026, 11, 17, tzinfo=NY), 190.25),
                ]
            ),
            "ZZZZ": _Frame([]),
            "7203.T": _RateLimit(),
        },
    )
    # The golden lists the rate-limited pair before the no_data one; the command stops at the
    # first rate_limited, so the request puts the no_data pair first and the answer is compared
    # by pair.
    code, out, err = _go(_request(("AAPL", "USD"), ("ZZZZ", "USD"), ("7203", "JPY")), yf, clock)
    assert code == 0
    assert err == ""
    answer = json.loads(out)
    assert [r["symbol"] for r in answer["results"]] == ["AAPL", "ZZZZ", "7203"]
    by_symbol = {r["symbol"]: r for r in answer["results"]}
    assert {r["symbol"]: r for r in golden["results"]} == by_symbol
    assert clock.slept == [1.0, 1.0]
    assert out.count("\n") == 1


def test_stops_after_rate_limit_and_marks_the_rest() -> None:
    clock = _Clock()
    yf = _Yf(clock, {"A": _RateLimit(), "B": _Frame([])})
    code, out, _ = _go(_request(("A", "USD"), ("B", "USD")), yf, clock)
    assert code == 0
    assert [r["error_code"] for r in json.loads(out)["results"]] == ["rate_limited"] * 2
    assert yf.asked == ["A"]


def test_deadline_answers_rest_timeout_keeping_bars() -> None:
    clock = _Clock()
    bars = _Frame([(datetime.datetime(2026, 11, 16, tzinfo=NY), 10.0)])
    yf = _Yf(clock, {"A": bars, "B": bars, "C": bars}, cost=4.0)
    code, out, _ = _go(_request(("A", "USD"), ("B", "USD"), ("C", "USD")), yf, clock, "10")
    results = json.loads(out)["results"]
    assert code == 0
    assert [r["error_code"] for r in results] == ["", "", "timeout"]
    assert results[0]["bars"] == [{"session_day": "2026-11-16", "open": "", "high": "", "low": "", "close": "10.0000", "volume": ""}]
    assert results[2]["bars"] == []
    assert yf.asked == ["A", "B"]


def test_over_ten_pairs_exit_2() -> None:
    clock = _Clock()
    yf = _Yf(clock, {})
    pairs = [(f"S{i}", "USD") for i in range(11)]
    code, out, err = _go(_request(*pairs), yf, clock)
    assert code == 2
    assert out == ""
    assert "more than 10" in err
    assert yf.asked == []


def test_bad_json_exit_2() -> None:
    clock = _Clock()
    code, out, _ = _go("not json", _Yf(clock, {}), clock)
    assert (code, out) == (2, "")


def test_tz_cache_is_a_temporary_directory_removed_at_exit() -> None:
    clock = _Clock()
    yf = _Yf(clock, {})
    _go(_request(), yf, clock)
    assert yf.cache_existed == [True]
    assert not os.path.exists(yf.cache[0])
