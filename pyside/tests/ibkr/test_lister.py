"""ListingGateway over a fake ib and a FakeClock: one list call in flight, the lock never leaks."""

from __future__ import annotations

import asyncio
from collections.abc import Awaitable, Callable, Sequence
from typing import Any

from ib_async import Contract, ContractDetails
from kit.fakeclock import FakeClock
from test_ibgateway import FakeIb, probe_passes, spin

from optdesk.ibkr.lister import ListingGateway
from optdesk.ibkr.model import ListedContract
from optdesk.ibkr.pacing import DetailsLane


def listing(clock: FakeClock) -> tuple[ListingGateway, FakeIb]:
    fake = FakeIb(clock)
    gateway = ListingGateway(
        lambda client: None,
        host="h",
        port=1,
        client_id=33,
        clock=clock,
        ib_factory=lambda: fake,
        readonly_probe=probe_passes,
    )
    return gateway, fake


def row(con_id: int, strike: float = 600.0) -> ContractDetails:
    return ContractDetails(
        contract=Contract(
            conId=con_id,
            symbol="SPY",
            exchange="SMART",
            currency="USD",
            lastTradeDateOrContractMonth="20260116",
            strike=strike,
            right="C",
            multiplier="100",
            tradingClass="SPY",
        )
    )


async def instant(contract: Contract) -> list[ContractDetails]:
    return [row(1)]


def ask(gateway: ListingGateway) -> Awaitable[Sequence[ListedContract]]:
    return gateway.list_contracts(
        symbol="SPY",
        exchange="SMART",
        currency="USD",
        expiry="20260116",
        right="",
        trading_class="SPY",
    )


async def hang_on(gate: asyncio.Event) -> list[ContractDetails]:
    await gate.wait()
    return [row(1)]


class RecordingLane:
    """A lane that allows any number of starts and records the most in flight."""

    def __init__(self) -> None:
        self.running = 0
        self.peak = 0
        self.gate = asyncio.Event()

    async def run[T](self, make: Callable[[], Awaitable[T]]) -> T:
        self.running += 1
        self.peak = max(self.peak, self.running)
        try:
            await self.gate.wait()
            return await make()
        finally:
            self.running -= 1


class AdmitFailsOnce(DetailsLane):
    """The real lane, but the first admit allows no start."""

    failed = False

    async def _admit(self) -> None:
        if not self.failed:
            self.failed = True
            raise TimeoutError("no start allowed in the details lane")
        await super()._admit()


async def opened(clock: FakeClock) -> tuple[ListingGateway, FakeIb]:
    gateway, fake = listing(clock)
    await gateway.open_session()
    fake.details = instant
    return gateway, fake


async def outcome(task: asyncio.Future[Any]) -> str:
    try:
        await task
    except TimeoutError:
        return "timeout"
    return "done"


def test_two_concurrent_calls_are_never_in_flight_together() -> None:
    async def run() -> tuple[int, int]:
        gateway, _ = await opened(FakeClock())
        lane = RecordingLane()
        gateway._lane = lane  # type: ignore[assignment]
        first = asyncio.ensure_future(ask(gateway))
        second = asyncio.ensure_future(ask(gateway))
        await spin()
        lane.gate.set()
        await asyncio.gather(first, second)
        return lane.peak, lane.running

    assert asyncio.run(run()) == (1, 0)


def test_a_start_the_lane_never_allowed_releases_the_lock() -> None:
    async def run() -> tuple[str, str]:
        clock = FakeClock()
        gateway, _ = await opened(clock)
        gateway._lane = AdmitFailsOnce(clock)
        first = await outcome(asyncio.ensure_future(ask(gateway)))
        second = await outcome(asyncio.ensure_future(ask(gateway)))
        return first, second

    assert asyncio.run(run()) == ("timeout", "done")


def test_a_late_answer_and_the_pause_do_not_leak_the_lock() -> None:
    async def run() -> list[str]:
        clock = FakeClock()
        gateway, fake = await opened(clock)
        gate = asyncio.Event()
        fake.details = lambda contract: hang_on(gate)
        first = asyncio.ensure_future(ask(gateway))
        await spin()
        await clock.advance(20)
        seen = [await outcome(first)]
        fake.details = instant
        second = asyncio.ensure_future(ask(gateway))  # waits in admit, 30 s pause
        await spin()
        await clock.advance(3)
        gate.set()  # the first request answers a few seconds late
        await spin()
        await clock.advance(17)
        seen.append(await outcome(second))
        third = asyncio.ensure_future(ask(gateway))
        await spin()
        await clock.advance(10)
        seen.append(await outcome(third))
        return seen

    assert asyncio.run(run()) == ["timeout", "timeout", "done"]


def test_a_call_cancelled_in_admit_releases_the_lock() -> None:
    async def run() -> str:
        clock = FakeClock()
        gateway, _ = await opened(clock)
        gate = asyncio.Event()
        # two requests that fill the lane's start limit, not under the list lock
        holders = [
            asyncio.ensure_future(gateway._lane.run(lambda: hang_on(gate)))
            for _ in range(2)
        ]
        await spin()
        waiting = asyncio.ensure_future(ask(gateway))
        await spin()
        waiting.cancel()
        await asyncio.gather(waiting, return_exceptions=True)
        gate.set()
        await asyncio.gather(*holders)
        return await outcome(asyncio.ensure_future(ask(gateway)))

    assert asyncio.run(run()) == "done"


def test_the_wait_for_the_lock_is_not_counted_in_the_lane_timeout() -> None:
    async def run() -> tuple[str, str]:
        clock = FakeClock()
        gateway, fake = await opened(clock)
        gate = asyncio.Event()
        calls = 0

        async def slow(contract: Contract) -> list[ContractDetails]:
            nonlocal calls
            calls += 1
            if calls == 1:
                return await hang_on(gate)
            await clock.sleep(10)
            return [row(2)]

        fake.details = slow
        first = asyncio.ensure_future(ask(gateway))
        second = asyncio.ensure_future(ask(gateway))
        await spin()
        await clock.advance(15)  # the second has waited 15 s for the lock
        gate.set()
        await spin(50)
        await clock.advance(10)  # then its own request runs 10 s: 25 s in all
        return await outcome(first), await outcome(second)

    assert asyncio.run(run()) == ("done", "done")


def test_strikes_are_canonical_decimal_text() -> None:
    async def run() -> list[str]:
        gateway, fake = await opened(FakeClock())

        async def strikes(contract: Contract) -> list[ContractDetails]:
            return [row(1, 660.0), row(2, 12.5)]

        fake.details = strikes
        return [r.strike for r in await ask(gateway)]

    assert asyncio.run(run()) == ["660", "12.5"]
