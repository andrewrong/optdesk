"""Contract test of the seams of the ibkr module: Clock, FakeClock, Gateway, model."""

from __future__ import annotations

import asyncio
import dataclasses
from collections.abc import Awaitable, Callable, Sequence
from datetime import timedelta

import pytest
from kit.fakeclock import FakeClock

from optdesk.ibkr.model import (
    ChainRow,
    Gateway,
    QuoteRow,
    QuoteTarget,
    ResolveResult,
    SessionState,
)
from optdesk.ibkr.timing import Clock, SystemClock, with_timeout


class _FakeGateway:
    async def open_session(self) -> None:
        return None

    async def status(self) -> SessionState:
        return SessionState(connected=True, generation=1, market_data_type=1)

    async def chains(
        self, *, sec_type: str, symbol: str, con_id: int
    ) -> Sequence[ChainRow]:
        return [ChainRow("SMART", symbol, "100", ("20260116",), ("100", "105"))]

    async def resolve(
        self,
        *,
        sec_type: str,
        symbol: str,
        exchange: str,
        currency: str,
        expiry: str,
        strike: str,
        right: str,
        multiplier: str,
        trading_class: str,
    ) -> ResolveResult:
        return ResolveResult(found=True, con_id=7)

    async def quotes(
        self,
        kind: str,
        targets: Sequence[QuoteTarget],
        on_quote: Callable[[QuoteRow], Awaitable[None]],
    ) -> None:
        for target in targets:
            await on_quote(QuoteRow(target.con_id, "ok", 1, 1, 0.0, 0.0, 0.0))


def test_fake_gateway_satisfies_protocol() -> None:
    gateway: Gateway = _FakeGateway()
    seen: list[QuoteRow] = []

    async def collect(row: QuoteRow) -> None:
        seen.append(row)

    async def run() -> ResolveResult:
        await gateway.open_session()
        await gateway.quotes("option_eod", [QuoteTarget(3), QuoteTarget(4)], collect)
        return await gateway.resolve(
            sec_type="STK",
            symbol="SPY",
            exchange="SMART",
            currency="USD",
            expiry="",
            strike="",
            right="",
            multiplier="",
            trading_class="",
        )

    result = asyncio.run(run())
    assert result.found and result.con_id == 7
    assert [row.con_id for row in seen] == [3, 4]
    with pytest.raises(dataclasses.FrozenInstanceError):
        delattr(result, "con_id")


def test_fakeclock_sleep_parks_until_advance() -> None:
    async def run() -> list[str]:
        clock: Clock = FakeClock()
        fake = clock
        assert isinstance(fake, FakeClock)
        events: list[str] = []

        async def sleeper() -> None:
            await clock.sleep(5)
            events.append("woke")

        task = asyncio.ensure_future(sleeper())
        await asyncio.sleep(0)
        assert fake.parked == 1
        await fake.advance(4)
        assert events == [] and fake.monotonic() == 4
        await fake.advance(1)
        await task
        return events

    assert asyncio.run(run()) == ["woke"]


def test_with_timeout_uses_clock() -> None:
    async def never() -> int:
        await asyncio.Event().wait()
        return 1

    async def quick() -> int:
        return 2

    async def run() -> tuple[int, bool]:
        clock = FakeClock()
        assert await with_timeout(clock, quick(), 10) == 2
        task = asyncio.ensure_future(with_timeout(clock, never(), 3))
        await asyncio.sleep(0)
        await clock.advance(3)
        timed_out = False
        try:
            await task
        except TimeoutError:
            timed_out = True
        return 2, timed_out

    assert asyncio.run(run()) == (2, True)


def test_system_clock_is_utc_aware() -> None:
    now = SystemClock().now()
    assert now.utcoffset() == timedelta(0)
