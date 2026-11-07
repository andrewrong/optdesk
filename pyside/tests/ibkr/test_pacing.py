"""Pacing rules over the fake clock: window, line budget and details lane."""

from __future__ import annotations

import asyncio

import pytest
from kit.fakeclock import FakeClock

from optdesk.ibkr.pacing import (
    DEFAULT_LINES,
    DEFAULT_SNAPSHOTS,
    DETAILS_MAX,
    DETAILS_MIN,
    DETAILS_PAUSE_S,
    DETAILS_START,
    DETAILS_TIMEOUT_S,
    DetailsLane,
    LineBudget,
    MessagePacer,
)


async def _spin(turns: int = 10) -> None:
    for _ in range(turns):
        await asyncio.sleep(0)


def test_pacer_forty_starts_then_waits() -> None:
    async def run() -> tuple[int, int, int]:
        clock = FakeClock()
        pacer = MessagePacer(clock)
        for _ in range(40):
            await pacer.start()
        done: list[int] = []

        async def forty_first() -> None:
            await pacer.start()
            done.append(1)

        task = asyncio.ensure_future(forty_first())
        await _spin()
        before = len(done)
        await clock.advance(1)
        await task
        return before, len(done), clock.parked

    assert asyncio.run(run()) == (0, 1, 0)


def test_pacer_counts_cancels() -> None:
    async def run() -> tuple[list[str], list[str]]:
        clock = FakeClock()
        pacer = MessagePacer(clock)
        for _ in range(39):
            await pacer.start()
        sent: list[str] = []
        pacer.cancel(lambda: sent.append("c1"))
        pacer.cancel(lambda: sent.append("c2"))
        await _spin()
        first = list(sent)
        await clock.advance(1)
        return first, sent

    first, after = asyncio.run(run())
    assert first == ["c1"]
    assert after == ["c1", "c2"]


def test_cancel_in_finally_does_not_block() -> None:
    async def run() -> tuple[bool, list[str], list[str]]:
        clock = FakeClock()
        pacer = MessagePacer(clock)
        for _ in range(40):
            await pacer.start()
        sent: list[str] = []

        async def worker() -> None:
            try:
                await asyncio.Event().wait()
            finally:
                pacer.cancel(lambda: sent.append("cancel"))

        task = asyncio.ensure_future(worker())
        await _spin()
        task.cancel()
        await asyncio.gather(task, return_exceptions=True)
        returned = task.cancelled()
        before = list(sent)
        await clock.advance(1)
        return returned, before, sent

    returned, before, after = asyncio.run(run())
    assert returned and before == [] and after == ["cancel"]


def test_budget_blocks_at_limit_and_releases() -> None:
    async def run() -> tuple[int, int, int]:
        budget = LineBudget(2)
        release = asyncio.Event()
        got: list[int] = []

        async def hold(n: int) -> None:
            async with budget.lease():
                got.append(n)
                await release.wait()

        tasks = [asyncio.ensure_future(hold(n)) for n in range(3)]
        await _spin()
        blocked = len(got)
        release.set()
        await asyncio.gather(*tasks)
        return blocked, len(got), budget.in_use

    assert asyncio.run(run()) == (2, 3, 0)


def test_lease_released_on_exception() -> None:
    async def run() -> int:
        budget = LineBudget(1)
        with pytest.raises(RuntimeError):
            async with budget.lease():
                raise RuntimeError("boom")
        async with budget.lease():
            pass
        return budget.in_use

    assert asyncio.run(run()) == 0


def test_budget_shrinks_on_101_not_below_one() -> None:
    async def run() -> list[int]:
        budget = LineBudget(DEFAULT_LINES)
        seen = [budget.limit]
        async with budget.lease(), budget.lease(), budget.lease():
            budget.shrink(200)
            seen.append(budget.limit)
            budget.shrink(101)
            seen.append(budget.limit)
        budget.shrink(101)
        seen.append(budget.limit)
        return seen

    assert asyncio.run(run()) == [70, 70, 2, 1]


def test_snapshot_cap_is_fifty() -> None:
    async def run() -> tuple[int, int]:
        budget = LineBudget(DEFAULT_SNAPSHOTS)
        release = asyncio.Event()
        running = 0

        async def hold() -> None:
            nonlocal running
            async with budget.lease():
                running += 1
                await release.wait()

        tasks = [asyncio.ensure_future(hold()) for _ in range(60)]
        await _spin()
        at_cap = running
        release.set()
        await asyncio.gather(*tasks)
        return at_cap, running

    assert asyncio.run(run()) == (50, 60)


async def _answer(clock: FakeClock, lane: DetailsLane, seconds: float) -> None:
    """One request of the lane that takes the given fake seconds."""

    async def work() -> int:
        await clock.sleep(seconds)
        return 1

    task = asyncio.ensure_future(lane.run(work))
    await _spin()
    await clock.advance(seconds)
    await task


def test_lane_starts_at_two() -> None:
    async def run() -> tuple[int, int, int]:
        clock = FakeClock()
        lane = DetailsLane(clock)
        release = asyncio.Event()
        started: list[int] = []

        async def work() -> int:
            started.append(1)
            await release.wait()
            return 1

        tasks = [asyncio.ensure_future(lane.run(work)) for _ in range(4)]
        await _spin()
        n = len(started)
        release.set()
        await asyncio.gather(*tasks)
        return lane.limit, n, DETAILS_START

    assert asyncio.run(run()) == (2, 2, 2)


def test_lane_rises_after_ten_fast_answers() -> None:
    async def run() -> list[int]:
        clock = FakeClock()
        lane = DetailsLane(clock)
        seen: list[int] = []
        for _ in range(9):
            await _answer(clock, lane, 0.5)
        seen.append(lane.limit)
        await _answer(clock, lane, 0.5)
        seen.append(lane.limit)
        return seen

    assert asyncio.run(run()) == [2, 3]


def test_lane_never_rises_above_max() -> None:
    async def run() -> int:
        clock = FakeClock()
        lane = DetailsLane(clock)
        for _ in range(100):
            await _answer(clock, lane, 0.1)
        return lane.limit

    assert asyncio.run(run()) == DETAILS_MAX


def test_lane_halves_after_slow_mean_not_below_one() -> None:
    async def run() -> list[int]:
        clock = FakeClock()
        lane = DetailsLane(clock)
        for _ in range(10):
            await _answer(clock, lane, 0.1)
        for _ in range(10):
            await _answer(clock, lane, 0.1)
        seen = [lane.limit]  # 4
        for _ in range(10):
            await _answer(clock, lane, 4)
        seen.append(lane.limit)  # 2
        for _ in range(10):
            await _answer(clock, lane, 4)
        seen.append(lane.limit)  # 1
        for _ in range(10):
            await _answer(clock, lane, 4)
        seen.append(lane.limit)  # still 1
        return seen

    assert asyncio.run(run()) == [4, 2, 1, DETAILS_MIN]


def test_lane_window_emptied_on_change() -> None:
    async def run() -> list[int]:
        clock = FakeClock()
        lane = DetailsLane(clock)
        for _ in range(10):
            await _answer(clock, lane, 0.5)  # limit 3, window emptied
        seen = [lane.limit]
        for _ in range(9):
            await _answer(clock, lane, 0.5)  # nine answers: not enough
        seen.append(lane.limit)
        await _answer(clock, lane, 0.5)
        seen.append(lane.limit)
        return seen

    assert asyncio.run(run()) == [3, 3, 4]


async def _timed_out(clock: FakeClock, lane: DetailsLane) -> asyncio.Future[int]:
    """Start a request that never answers and run the clock to its timeout."""
    never: asyncio.Future[int] = asyncio.get_running_loop().create_future()

    async def work() -> int:
        return await never

    task = asyncio.ensure_future(lane.run(work))
    await _spin()
    await clock.advance(DETAILS_TIMEOUT_S)
    with pytest.raises(TimeoutError):
        await task
    return never


def test_timeout_drops_limit_and_pauses_thirty_seconds() -> None:
    async def run() -> tuple[int, bool, bool]:
        clock = FakeClock()
        lane = DetailsLane(clock)
        for _ in range(10):
            await _answer(clock, lane, 0.5)
        assert lane.limit == 3
        never = await _timed_out(clock, lane)
        never.set_result(0)
        await _spin()
        limit = lane.limit
        started: list[float] = []

        async def work() -> int:
            started.append(clock.monotonic())
            return 1

        await clock.advance(15)
        task = asyncio.ensure_future(lane.run(work))
        await _spin()
        await clock.advance(DETAILS_PAUSE_S - 15 - 1)
        held = not started
        await clock.advance(1)
        await task
        return limit, held, bool(started)

    assert asyncio.run(run()) == (DETAILS_MIN, True, True)


def test_first_answer_after_timeout_is_no_sample() -> None:
    async def run() -> list[int]:
        clock = FakeClock()
        lane = DetailsLane(clock)
        await _timed_out(clock, lane)  # stays unresolved: one of at most two
        await clock.advance(DETAILS_PAUSE_S)
        seen: list[int] = []
        # the first answer is no sample, so ten answers hold only nine samples
        for _ in range(10):
            await _answer(clock, lane, 0.2)
        seen.append(lane.limit)
        await _answer(clock, lane, 0.2)
        seen.append(lane.limit)
        return seen

    assert asyncio.run(run()) == [1, 2]


def test_no_start_while_oldest_older_than_five_seconds() -> None:
    async def run() -> tuple[bool, bool]:
        clock = FakeClock()
        lane = DetailsLane(clock)
        release = asyncio.Event()
        started: list[str] = []

        async def slow() -> int:
            started.append("slow")
            await release.wait()
            return 1

        async def quick() -> int:
            started.append("quick")
            return 1

        first = asyncio.ensure_future(lane.run(slow))
        await _spin()
        await clock.advance(6)
        second = asyncio.ensure_future(lane.run(quick))
        await _spin()
        held = started == ["slow"]
        release.set()
        await asyncio.gather(first, second)
        return held, started == ["slow", "quick"]

    assert asyncio.run(run()) == (True, True)


def test_no_start_with_three_unresolved_timeouts() -> None:
    async def run() -> tuple[bool, bool]:
        clock = FakeClock()
        lane = DetailsLane(clock)
        pending = []
        for _ in range(3):
            pending.append(await _timed_out(clock, lane))
            await clock.advance(DETAILS_PAUSE_S)
        started: list[int] = []

        async def work() -> int:
            started.append(1)
            return 1

        task = asyncio.ensure_future(lane.run(work))
        await _spin()
        await clock.advance(1)
        held = not started
        pending[0].set_result(0)
        await _spin()
        await task
        return held, bool(started)

    assert asyncio.run(run()) == (True, True)
