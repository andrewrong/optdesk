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
    def __init__(
        self,
        rows: list[tuple[datetime.datetime, float]],
        fields: dict[str, list[float]] | None = None,
    ) -> None:
        self._rows = rows
        self._fields = fields or {}
        self.columns = [*COLUMNS, *self._fields]
        self.empty = not rows

    def __getitem__(self, name: str) -> _Series:
        if name in self._fields:
            return _Series(
                [(s, self._fields[name][i]) for i, (s, _) in enumerate(self._rows)]
            )
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
    def __init__(
        self, clock: _Clock, outcomes: dict[str, Any], cost: float = 0.0
    ) -> None:
        self.clock = clock
        self.outcomes = outcomes
        self.cost = cost
        self.config = SimpleNamespace(debug=SimpleNamespace(hide_exceptions=True))
        self.exceptions = _Errors
        self.cache: list[str] = []
        self.cache_existed: list[bool] = []
        self.asked: list[str] = []
        self.kwargs: list[dict[str, Any]] = []
        self.adjusted: dict[str, Any] = {}

    def set_tz_cache_location(self, path: str) -> None:
        self.cache.append(path)
        self.cache_existed.append(os.path.isdir(path))

    def Ticker(self, symbol: str) -> Any:
        yf = self

        class _Ticker:
            def history(self, **kwargs: Any) -> Any:
                yf.asked.append(symbol)
                yf.kwargs.append(kwargs)
                yf.clock.now += yf.cost
                outcome = yf.outcomes[symbol]
                if kwargs.get("auto_adjust") and symbol in yf.adjusted:
                    outcome = yf.adjusted[symbol]
                if isinstance(outcome, Exception):
                    raise outcome
                return outcome

        return _Ticker()


def _request(*pairs: tuple[str, str], **flags: Any) -> str:
    return json.dumps(
        {
            "pairs": [{"symbol": s, "currency": c} for s, c in pairs],
            "from_day": "2026-11-16",
            "to_day": "2026-11-17",
            **flags,
        }
    )


def _go(
    text: str, yf: _Yf, clock: _Clock, deadline: str = "100"
) -> tuple[int, str, str]:
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
                ],
                {"Dividends": [0.0, 0.26], "Stock Splits": [2.0, 0.0]},
            ),
            "ZZZZ": _Frame([]),
            "7203.T": _RateLimit(),
        },
    )
    yf.adjusted["AAPL"] = _Frame(
        [
            (datetime.datetime(2026, 11, 16, tzinfo=NY), 94.5),
            (datetime.datetime(2026, 11, 17, tzinfo=NY), 95.25),
        ]
    )
    # The golden lists the rate-limited pair before the no_data one; the command stops at the
    # first rate_limited, so the request puts the no_data pair first and the answer is compared
    # by pair.
    request = _request(
        ("AAPL", "USD"),
        ("ZZZZ", "USD"),
        ("7203", "JPY"),
        with_actions=True,
        adjusted=True,
    )
    code, out, err = _go(request, yf, clock)
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
    code, out, _ = _go(
        _request(("A", "USD"), ("B", "USD"), ("C", "USD")), yf, clock, "10"
    )
    results = json.loads(out)["results"]
    assert code == 0
    assert [r["error_code"] for r in results] == ["", "", "timeout"]
    assert results[0]["bars"] == [
        {
            "session_day": "2026-11-16",
            "open": "10.0000",
            "high": "10.0000",
            "low": "10.0000",
            "close": "10.0000",
            "volume": "10.0000",
        }
    ]
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


class TestCliActionsRequest:
    def test_flags_reach_the_fetch_and_the_answer(self) -> None:
        clock = _Clock()
        day = datetime.datetime(2026, 11, 16, tzinfo=NY)
        yf = _Yf(
            clock,
            {"A": _Frame([(day, 10.0)], {"Dividends": [0.5], "Stock Splits": [0.0]})},
        )
        yf.adjusted["A"] = _Frame([(day, 9.0)])
        _, out, _ = _go(
            _request(("A", "USD"), with_actions=True, adjusted=True), yf, clock
        )
        result = json.loads(out)["results"][0]
        assert result["dividends"] == [{"day": "2026-11-16", "value": "0.5000"}]
        assert result["splits"] == []
        assert result["bars"][0]["adj_close"] == "9.0000"
        assert [k["auto_adjust"] for k in yf.kwargs] == [False, True]
        assert yf.kwargs[0]["actions"] is True

    def test_request_without_the_flags_answers_as_before(self) -> None:
        clock = _Clock()
        day = datetime.datetime(2026, 11, 16, tzinfo=NY)
        yf = _Yf(clock, {"A": _Frame([(day, 10.0)])})
        _, out, _ = _go(_request(("A", "USD")), yf, clock)
        result = json.loads(out)["results"][0]
        assert "dividends" not in result and "splits" not in result
        assert "adj_close" not in result["bars"][0]
        assert len(yf.kwargs) == 1 and yf.kwargs[0]["actions"] is False

    def test_flag_that_is_not_a_bool_is_refused(self) -> None:
        clock = _Clock()
        code, out, err = _go(
            _request(("A", "USD"), adjusted="yes"), _Yf(clock, {}), clock
        )
        assert (code, out) == (2, "")
        assert "usage error" in err
