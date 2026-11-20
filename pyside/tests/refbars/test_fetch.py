"""The yfinance call for one pair, against a fake yfinance object and a fake clock."""

from __future__ import annotations

import datetime
import io
import json
from types import SimpleNamespace
from typing import Any
from zoneinfo import ZoneInfo

import pytest

from optdesk.refbars.cli import run
from optdesk.refbars.fetch import make_fetcher

NY = ZoneInfo("America/New_York")
COLUMNS = ["Open", "High", "Low", "Close", "Adj Close", "Volume"]


class _Missing(Exception):
    pass


class _Errors:
    class YFRateLimitError(Exception):
        pass

    class YFPricesMissingError(_Missing):
        pass

    class YFTzMissingError(_Missing):
        pass


class _Series:
    def __init__(self, rows: list[tuple[datetime.datetime, float]]) -> None:
        self._rows = rows

    def items(self) -> list[tuple[datetime.datetime, float]]:
        return self._rows


class _Frame:
    """Rows of (stamp, close); `fields` gives other columns by name, one value per row (NaN when absent)."""

    def __init__(
        self,
        rows: list[tuple[datetime.datetime, float]],
        columns: list[str],
        fields: dict[str, list[float]] | None = None,
    ) -> None:
        self._rows = rows
        self.columns = columns
        self._fields = fields or {}
        self.empty = not rows

    def __getitem__(self, name: str) -> _Series:
        if name == "Close":
            return _Series(self._rows)
        values = self._fields.get(name, [float("nan")] * len(self._rows))
        return _Series([(stamp, values[i]) for i, (stamp, _) in enumerate(self._rows)])


class _FakeClock:
    def __init__(self) -> None:
        self.now = 0.0
        self.slept: list[float] = []

    def monotonic(self) -> float:
        return self.now

    def sleep(self, seconds: float) -> None:
        self.slept.append(seconds)
        self.now += seconds


class _FakeYf:
    def __init__(self, outcomes: dict[str, Any]) -> None:
        self.outcomes = outcomes
        self.config = SimpleNamespace(debug=SimpleNamespace(hide_exceptions=True))
        self.exceptions = _Errors
        self.calls: list[tuple[str, dict[str, Any]]] = []
        self.hide_during_call: list[bool] = []
        self.cache: list[str] = []

    def set_tz_cache_location(self, path: str) -> None:
        self.cache.append(path)

    def Ticker(self, symbol: str) -> Any:
        yf = self

        class _Ticker:
            def history(self, **kwargs: Any) -> Any:
                yf.calls.append((symbol, kwargs))
                yf.hide_during_call.append(yf.config.debug.hide_exceptions)
                outcome = yf.outcomes[symbol]
                if isinstance(outcome, Exception):
                    raise outcome
                return outcome

        return _Ticker()


def _day(d: int, hour: int = 0) -> datetime.datetime:
    return datetime.datetime(2026, 11, d, hour, tzinfo=NY)


def _pair(symbol: str = "AAPL", currency: str = "USD") -> Any:
    return {"symbol": symbol, "currency": currency}


def test_auto_adjust_false_and_close_read() -> None:
    frame = _Frame([(_day(16), 189.5), (_day(17), 190.25)], COLUMNS)
    yf = _FakeYf({"AAPL": frame})
    result = make_fetcher(yf)(_pair(), "2026-11-16", "2026-11-17")
    assert yf.calls == [
        (
            "AAPL",
            {
                "start": "2026-11-16",
                "end": "2026-11-18",
                "interval": "1d",
                "auto_adjust": False,
                "actions": False,
                "timeout": 10,
            },
        )
    ]
    assert result["error_code"] == ""
    assert result["bars"] == [
        {"session_day": "2026-11-16", "open": "", "high": "", "low": "", "close": "189.5000", "volume": ""},
        {"session_day": "2026-11-17", "open": "", "high": "", "low": "", "close": "190.2500", "volume": ""},
    ]


def test_decimal_text_has_no_float_artefact() -> None:
    frame = _Frame([(_day(16), 187.1999969482422), (_day(17), 0.1 + 0.2)], COLUMNS)
    result = make_fetcher(_FakeYf({"AAPL": frame}))(_pair(), "2026-11-16", "2026-11-17")
    assert [b["close"] for b in result["bars"]] == ["187.2000", "0.3000"]


def test_session_day_is_the_exchange_date() -> None:
    late = datetime.datetime(2026, 11, 17, 3, tzinfo=datetime.UTC)  # 22:00 on the 16th in New York
    frame = _Frame([(late, 10.0)], COLUMNS)
    result = make_fetcher(_FakeYf({"AAPL": frame}))(_pair(), "2026-11-16", "2026-11-17")
    assert result["bars"] == [{"session_day": "2026-11-16", "open": "", "high": "", "low": "", "close": "10.0000", "volume": ""}]


def test_rate_limit_stops_batch() -> None:
    good = _Frame([(_day(16), 1.0)], COLUMNS)
    yf = _FakeYf({"AAPL": good, "MSFT": _Errors.YFRateLimitError(), "NVDA": good})
    request = {
        "pairs": [_pair("AAPL"), _pair("MSFT"), _pair("NVDA")],
        "from_day": "2026-11-16",
        "to_day": "2026-11-16",
    }
    clock, out = _FakeClock(), io.StringIO()
    code = run(["--deadline", "100"], io.StringIO(json.dumps(request)), out, io.StringIO(), clock, yf)
    results = json.loads(out.getvalue())["results"]
    assert code == 0
    assert [r["error_code"] for r in results] == ["", "rate_limited", "rate_limited"]
    assert [s for s, _ in yf.calls] == ["AAPL", "MSFT"]


def test_empty_frame_is_no_data_not_empty_bars() -> None:
    yf = _FakeYf(
        {
            "AAPL": _Frame([], COLUMNS),
            "ZZZZ": _Errors.YFPricesMissingError(),
        }
    )
    fetch = make_fetcher(yf)
    for symbol in ("AAPL", "ZZZZ"):
        result = fetch(_pair(symbol), "2026-11-16", "2026-11-17")
        assert result["error_code"] == "no_data"
        assert result["bars"] == []


def test_failures_map_to_codes() -> None:
    class ConnectionError(Exception):
        pass

    class Timeout(Exception):
        pass

    yf = _FakeYf(
        {
            "NET": ConnectionError(),
            "SLOW": Timeout(),
            "BADCOLS": _Frame([(_day(16), 1.0)], ["Close"]),
            "ODD": RuntimeError("x"),
        }
    )
    fetch = make_fetcher(yf)
    codes = {s: fetch(_pair(s), "2026-11-16", "2026-11-16")["error_code"] for s in yf.outcomes}
    assert codes == {
        "NET": "network",
        "SLOW": "timeout",
        "BADCOLS": "source_changed",
        "ODD": "source_changed",
    }


def test_other_currency_is_no_data_without_a_call() -> None:
    yf = _FakeYf({})
    result = make_fetcher(yf)(_pair("SAP", "EUR"), "2026-11-16", "2026-11-17")
    assert result["error_code"] == "no_data"
    assert yf.calls == []


def test_hide_exceptions_restored_after_test() -> None:
    frame = _Frame([(_day(16), 1.0)], COLUMNS)
    yf = _FakeYf({"AAPL": frame, "MSFT": _Errors.YFRateLimitError()})
    fetch = make_fetcher(yf)
    fetch(_pair("AAPL"), "2026-11-16", "2026-11-16")
    fetch(_pair("MSFT"), "2026-11-16", "2026-11-16")
    assert yf.hide_during_call == [False, False]
    assert yf.config.debug.hide_exceptions is True


@pytest.mark.parametrize("value", [float("nan")])
def test_nan_close_is_skipped(value: float) -> None:
    frame = _Frame([(_day(16), value), (_day(17), 2.0)], COLUMNS)
    result = make_fetcher(_FakeYf({"AAPL": frame}))(_pair(), "2026-11-16", "2026-11-17")
    assert result["bars"] == [{"session_day": "2026-11-17", "open": "", "high": "", "low": "", "close": "2.0000", "volume": ""}]


def test_bar_carries_ohlcv() -> None:
    frame = _Frame(
        [(_day(16), 189.5), (_day(17), 190.25)],
        COLUMNS,
        {
            "Open": [188.00005, 189.1999969482422],
            "High": [190.0, 191.5],
            "Low": [187.25, 188.0],
            "Volume": [1234567.0, 89.0],
        },
    )
    result = make_fetcher(_FakeYf({"AAPL": frame}))(_pair(), "2026-11-16", "2026-11-17")
    assert result["bars"] == [
        {
            "session_day": "2026-11-16",
            "open": "188.0001",
            "high": "190.0000",
            "low": "187.2500",
            "close": "189.5000",
            "volume": "1234567.0000",
        },
        {
            "session_day": "2026-11-17",
            "open": "189.2000",
            "high": "191.5000",
            "low": "188.0000",
            "close": "190.2500",
            "volume": "89.0000",
        },
    ]


def test_nan_open_is_empty_text() -> None:
    nan = float("nan")
    frame = _Frame(
        [(_day(16), 5.0)],
        COLUMNS,
        {"Open": [nan], "High": [nan], "Low": [nan], "Volume": [nan]},
    )
    result = make_fetcher(_FakeYf({"AAPL": frame}))(_pair(), "2026-11-16", "2026-11-16")
    assert result["error_code"] == ""
    assert result["bars"] == [
        {"session_day": "2026-11-16", "open": "", "high": "", "low": "", "close": "5.0000", "volume": ""}
    ]
