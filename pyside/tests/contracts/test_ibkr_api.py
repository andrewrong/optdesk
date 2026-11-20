"""Contract test of optdesk.ibkr.api, from both sides: a fake and a caller."""

from __future__ import annotations

import ast
import asyncio
import dataclasses
import inspect
from collections.abc import Awaitable, Callable, Sequence

import pytest

from optdesk.ibkr import api, model
from optdesk.ibkr.api import (
    DividendKey,
    DividendReading,
    FarmBoard,
    History,
    HistoryBar,
    HistoryKey,
    HistoryPaced,
    HistoryRequest,
    HistoryUnavailable,
    Reference,
    Session,
    SessionStatus,
)


class _FakeSession:
    def ping(self) -> SessionStatus:
        return SessionStatus(connected=True)


def _is_connected(session: Session) -> bool:
    return session.ping().connected


def test_fake_satisfies_session() -> None:
    assert isinstance(_FakeSession(), Session)


def test_caller_reads_connected() -> None:
    assert _is_connected(_FakeSession()) is True


def test_session_status_is_frozen() -> None:
    status = SessionStatus(connected=False)
    with pytest.raises(dataclasses.FrozenInstanceError):
        status.connected = True  # type: ignore[misc]


def test_api_declares_only() -> None:
    tree = ast.parse(inspect.getsource(api))
    funcs = [
        n
        for n in ast.walk(tree)
        if isinstance(n, (ast.FunctionDef, ast.AsyncFunctionDef))
    ]
    assert funcs
    for f in funcs:
        assert len(f.body) <= 2
        assert all(
            isinstance(s, ast.Expr) and isinstance(s.value, ast.Constant)
            for s in f.body
        )


class _FakeReference:
    async def dividends(
        self,
        keys: Sequence[DividendKey],
        on_reading: Callable[[DividendReading], Awaitable[None]],
    ) -> None:
        for key in keys:
            await on_reading(
                DividendReading(key, "no_value", None, None, None, None, 0, 1.0)
            )


def test_fake_satisfies_reference_and_caller_reads_readings() -> None:
    fake = _FakeReference()
    assert isinstance(fake, Reference)
    seen: list[DividendReading] = []

    async def collect(reading: DividendReading) -> None:
        seen.append(reading)

    key = DividendKey("AAPL", "STK", "SMART", "USD", 1)
    asyncio.run(fake.dividends([key], collect))
    assert [(r.key, r.status, r.next_ex_date) for r in seen] == [
        (key, "no_value", None)
    ]


def test_dividend_reading_is_frozen() -> None:
    reading = DividendReading(
        DividendKey("A", "STK", "", "", 0), "ok", None, None, None, None, 0, 0.0
    )
    with pytest.raises(dataclasses.FrozenInstanceError):
        reading.status = "error"  # type: ignore[misc]


class _FakeHistory:
    async def bars(self, request: HistoryRequest) -> Sequence[HistoryBar]:
        return [HistoryBar("2026-01-02", "1", "2", "0.5", "1.5", None)]


def test_fake_satisfies_history_and_caller_reads_bars() -> None:
    fake = _FakeHistory()
    assert isinstance(fake, History)
    key = HistoryKey("SPY", "STK", "SMART", "USD", 1)
    got = asyncio.run(fake.bars(HistoryRequest(key, "trades", 5, "")))
    assert [(b.day, b.close, b.volume) for b in got] == [("2026-01-02", "1.5", None)]


def test_history_values_are_frozen() -> None:
    bar = HistoryBar("2026-01-02", "1", "2", "0.5", "1.5", None)
    with pytest.raises(dataclasses.FrozenInstanceError):
        bar.close = "2"  # type: ignore[misc]


def test_history_paced_is_an_exception() -> None:
    assert issubclass(HistoryPaced, Exception)


class _FakeFarmBoard:
    def __init__(self) -> None:
        self.rows: list[model.Farm] = []

    def record(self, code: int, text: str, now: int) -> None:
        self.rows.append(model.Farm(name=text, up=code == 2104, since=now))

    def snapshot(self) -> tuple[model.Farm, ...]:
        return tuple(self.rows)


class TestFarmBoardContract:
    def test_fake_satisfies_farm_board(self) -> None:
        assert isinstance(_FakeFarmBoard(), FarmBoard)

    def test_caller_reads_snapshot(self) -> None:
        board: FarmBoard = _FakeFarmBoard()
        board.record(2103, "usfarm", 5)
        assert board.snapshot() == (model.Farm(name="usfarm", up=False, since=5),)

    def test_history_unavailable_is_an_exception(self) -> None:
        with pytest.raises(HistoryUnavailable):
            raise HistoryUnavailable("history_unavailable")

    def test_api_declares_no_session_state(self) -> None:
        assert not hasattr(api, "SessionState")


class TestSessionStateDefaults:
    def test_old_arguments_still_work(self) -> None:
        s = model.SessionState(True, 3, 1)
        assert s.soft_lost is False
        assert s.state == "up"
        assert s.farms == ()

    def test_new_fields_are_set_by_keyword(self) -> None:
        farm = model.Farm(name="ushmds", up=False, since=9)
        s = model.SessionState(
            connected=True,
            generation=1,
            market_data_type=1,
            state="farm_broken",
            farms=(farm,),
        )
        assert s.state == "farm_broken"
        assert s.farms == (farm,)
