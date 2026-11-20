"""Clock probe, wife clock only, farm probe, grace and the history wait; a FakeClock throughout."""

from __future__ import annotations

import asyncio
from collections.abc import Awaitable, Callable
from typing import Any

import pytest
from kit.fakeclock import FakeClock
from test_ibgateway import FakeIb, make

from optdesk.ibkr.api import HistoryKey, HistoryRequest, HistoryUnavailable
from optdesk.ibkr.farm_probe import FARM_NAME, FarmProbe
from optdesk.ibkr.farms import Farms
from optdesk.ibkr.health_constants import (
    FARM_GRACE_S,
    FARM_PROBE_EVERY_S,
    HISTORY_WAIT_S,
    PROBE_EVERY_S,
    PROBE_WAIT_S,
)
from optdesk.ibkr.model import SessionState
from optdesk.ibkr.server import supervise
from optdesk.ibkr.session_probe import watch_for


class ProbedFake:
    """A gateway the watch can probe: counts opens, probes and history requests."""

    def __init__(self, clock: FakeClock, name: str) -> None:
        self.farms = Farms(name, clock)
        self.connected = False
        self.opened = 0
        self.fail_time = False
        self.time_asks = 0
        self.bar_asks = 0

    async def open_session(self) -> None:
        self.opened += 1
        self.connected = True

    async def status(self) -> SessionState:
        return SessionState(
            self.connected,
            self.opened,
            1,
            state=self.farms.state(self.connected, False),
            farms=self.farms.snapshot(),
        )

    async def probe_time(self) -> object:
        self.time_asks += 1
        if self.fail_time:
            raise ConnectionError("no answer")
        return object()

    async def probe_bars(self) -> object:
        self.bar_asks += 1
        return object()

    def close_session(self) -> None:
        self.connected = False


async def _run(
    clock: FakeClock,
    gateways: list[tuple[ProbedFake, bool]],
    script: Callable[[], Awaitable[None]],
) -> None:
    tasks = [
        asyncio.ensure_future(
            supervise(g, clock, g.farms.name, watch_for(g, clock, history=history))
        )
        for g, history in gateways
    ]
    try:
        await script()
    finally:
        for task in tasks:
            task.cancel()
        await asyncio.gather(*tasks, return_exceptions=True)


class TestClockProbe:
    def test_two_failures_keep_the_session_the_third_reopens_it(self) -> None:
        clock = FakeClock()
        gateway = ProbedFake(clock, "main")

        async def script() -> None:
            await clock.advance(1)
            gateway.fail_time = True
            await clock.advance(PROBE_EVERY_S)
            await clock.advance(PROBE_EVERY_S)
            assert gateway.opened == 1 and gateway.connected
            await clock.advance(PROBE_EVERY_S)
            await clock.advance(5)
            assert gateway.opened == 2 and gateway.connected

        asyncio.run(_run(clock, [(gateway, False)], script))

    def test_a_good_probe_between_clears_the_count(self) -> None:
        clock = FakeClock()
        gateway = ProbedFake(clock, "main")

        async def script() -> None:
            await clock.advance(1)
            for fails in (True, True, False, True, True):
                gateway.fail_time = fails
                await clock.advance(PROBE_EVERY_S)
            assert gateway.opened == 1 and gateway.connected

        asyncio.run(_run(clock, [(gateway, False)], script))

    def test_a_probe_that_never_answers_fails_after_probe_wait(self) -> None:
        clock = FakeClock()
        gateway = ProbedFake(clock, "main")

        async def hang() -> object:
            await asyncio.Event().wait()
            return object()

        gateway.probe_time = hang  # type: ignore[method-assign]

        async def script() -> None:
            await clock.advance(1)
            for _ in range(3):
                await clock.advance(PROBE_EVERY_S)
                await clock.advance(PROBE_WAIT_S)
            await clock.advance(5)
            assert gateway.opened == 2

        asyncio.run(_run(clock, [(gateway, False)], script))


class TestWifeClockOnly:
    def test_wife_gets_no_history_request_and_main_failures_do_not_touch_it(
        self,
    ) -> None:
        clock = FakeClock()
        main, wife = ProbedFake(clock, "main"), ProbedFake(clock, "wife")

        async def script() -> None:
            await clock.advance(1)
            main.fail_time = True
            for _ in range(3):
                await clock.advance(PROBE_EVERY_S)
            await clock.advance(FARM_PROBE_EVERY_S)
            assert main.opened >= 2
            assert wife.opened == 1 and wife.connected
            assert wife.bar_asks == 0 and wife.time_asks >= 3
            assert (await wife.status()).state == "up"

        asyncio.run(_run(clock, [(main, True), (wife, False)], script))


class TestFarmProbe:
    @staticmethod
    def _probe(
        clock: FakeClock, farms: Farms, good: bool, answers: list[bool]
    ) -> FarmProbe:
        async def ask() -> object:
            if not answers.pop(0):
                raise HistoryUnavailable("history_unavailable")
            return object()

        return FarmProbe(clock, farms, lambda: good, ask)

    def test_three_failures_set_farm_broken_and_a_good_answer_clears_it(self) -> None:
        async def run() -> list[str]:
            clock = FakeClock()
            farms = Farms("main", clock)
            probe = self._probe(clock, farms, True, [False, False, False, True])
            seen = []
            for _ in range(3):
                await probe.step()
                seen.append(farms.state(True, False))
            assert [(f.name, f.up) for f in farms.snapshot()] == [(FARM_NAME, False)]
            await probe.step()
            seen.append(farms.state(True, False))
            return seen

        assert asyncio.run(run()) == ["up", "up", "farm_broken", "up"]

    def test_a_failing_clock_probe_sets_no_farm_broken(self) -> None:
        async def run() -> str:
            clock = FakeClock()
            farms = Farms("main", clock)
            probe = self._probe(clock, farms, False, [False] * 5)
            for _ in range(5):
                await probe.step()
            return farms.state(True, False)

        assert asyncio.run(run()) == "up"

    def test_a_request_that_hangs_fails_after_probe_wait_of_the_clock(self) -> None:
        async def run() -> int:
            clock = FakeClock()

            async def ask() -> object:
                await asyncio.Event().wait()
                return object()

            probe = FarmProbe(clock, Farms("main", clock), lambda: True, ask)
            task = asyncio.ensure_future(probe.step())
            await clock.advance(PROBE_WAIT_S - 1)
            assert not task.done()
            await clock.advance(1)
            await task
            return probe.fails

        assert asyncio.run(run()) == 1


class TestFarmGrace:
    def test_down_for_less_than_the_grace_is_not_broken_and_longer_is(self) -> None:
        async def run() -> list[str]:
            clock = FakeClock()
            farms = Farms("main", clock)
            farms.record(2105, "broken:ushmds", farms.now_ms())
            seen = [farms.state(True, False)]
            await clock.advance(FARM_GRACE_S - 1)
            seen.append(farms.state(True, False))
            await clock.advance(1)
            seen.append(farms.state(True, False))
            farms.record(2106, "ok:ushmds", farms.now_ms())
            seen.append(farms.state(True, False))
            return seen

        assert asyncio.run(run()) == ["up", "up", "farm_broken", "up"]


class HistoryIb(FakeIb):
    def reqHistoricalDataAsync(self, *args: Any) -> Awaitable[list[object]]:
        async def never() -> list[object]:
            await asyncio.Event().wait()
            return []

        return never()


class TestHistoryWait:
    def test_bars_raises_history_unavailable_after_the_wait(self) -> None:
        async def run() -> None:
            clock = FakeClock()
            gateway, _ = make(clock, HistoryIb(clock))
            await gateway.open_session()
            request = HistoryRequest(
                HistoryKey("SPY", "STK", "SMART", "USD", 756733), "trades", 1, ""
            )
            task = asyncio.ensure_future(gateway.bars(request))
            await clock.advance(HISTORY_WAIT_S)
            with pytest.raises(HistoryUnavailable, match="history_unavailable"):
                await task

        asyncio.run(run())
