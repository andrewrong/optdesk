"""Tick 456 reads over a fake gateway of this file and the FakeClock; nothing waits."""

from __future__ import annotations

import asyncio
from dataclasses import dataclass, field

import pytest
from kit.fakeclock import FakeClock

from optdesk.ibkr.api import DividendKey, DividendReading
from optdesk.ibkr.dividends import (
    Release,
    TickError,
    TickText,
    parse_tick,
    read_dividend,
)

KEY = DividendKey("AAPL", "STK", "SMART", "USD", 265598)


@dataclass
class FakeGateway:
    """Answers each subscription with the scripted texts, or stays silent."""

    texts: list[str] = field(default_factory=list)
    error: tuple[int, str] | None = None
    asked: list[str] = field(default_factory=list)
    open: int = 0
    released: int = 0

    def subscribe(
        self, key: DividendKey, ticks: str, on_text: TickText, on_error: TickError
    ) -> Release:
        self.asked.append(ticks)
        self.open += 1
        if self.error is not None:
            on_error(*self.error)
        for text in self.texts:
            on_text(text)

        def release() -> None:
            self.open -= 1
            self.released += 1

        return release


def read(gateway: FakeGateway) -> DividendReading:
    return asyncio.run(read_dividend(FakeClock(), KEY, gateway.subscribe))


def test_parse_four_values() -> None:
    gateway = FakeGateway(texts=["0.83,0.92,20130219,0.23"])
    got = read(gateway)
    assert gateway.asked == ["456"]
    assert (got.status, got.past_12m, got.next_12m) == ("ok", "0.83", "0.92")
    assert (got.next_ex_date, got.next_amount) == ("20130219", "0.23")
    assert got.error_code == 0 and got.key == KEY


@pytest.mark.parametrize("text", [",,,", "", "0.8,0.9,,0.2", "1,2,3"])
def test_empty_tick_is_no_value(text: str) -> None:
    got = read(FakeGateway(texts=[text]))
    assert got.status == "no_value"
    assert (got.past_12m, got.next_12m, got.next_ex_date, got.next_amount) == (
        None,
        None,
        None,
        None,
    )


@pytest.mark.parametrize("date", ["20261345", "2026-01-02", "abc", "2026010"])
def test_malformed_ex_date_is_no_value(date: str) -> None:
    got = read(FakeGateway(texts=[f"0.8,0.9,{date},0.2"]))
    assert got.status == "no_value" and got.next_ex_date is None
    assert parse_tick(f"0.8,0.9,{date},0.2") is None


def test_zero_is_a_value_and_empty_amount_is_unset() -> None:
    got = read(FakeGateway(texts=["0,0.00,20260102,"]))
    assert (got.status, got.past_12m, got.next_12m, got.next_amount) == (
        "ok",
        "0",
        "0",
        None,
    )


def test_silent_ticker_is_timeout() -> None:
    async def run() -> tuple[DividendReading, float]:
        clock = FakeClock()
        gateway = FakeGateway()
        task = asyncio.ensure_future(read_dividend(clock, KEY, gateway.subscribe))
        await clock.advance(9.9)
        assert not task.done()
        await clock.advance(0.2)
        return await task, clock.monotonic()

    got, elapsed = asyncio.run(run())
    assert got.status == "timeout" and got.next_ex_date is None
    assert elapsed == pytest.approx(10.1)


def test_error_is_error_with_code() -> None:
    got = read(FakeGateway(error=(354, "no subscription")))
    assert (got.status, got.error_code) == ("error", 354)


def test_subscription_released() -> None:
    answered = FakeGateway(texts=["1,1,20260102,1"])
    read(answered)
    failed = FakeGateway(error=(200, "x"))
    read(failed)

    async def silent() -> FakeGateway:
        clock = FakeClock()
        gateway = FakeGateway()
        task = asyncio.ensure_future(read_dividend(clock, KEY, gateway.subscribe))
        await clock.advance(10)
        await task
        return gateway

    timed_out = asyncio.run(silent())
    for gateway in (answered, failed, timed_out):
        assert (gateway.open, gateway.released) == (0, 1)
