"""Pacing of the IBKR adapter: message window, line leases and the details lane.

Three small objects over a Clock. They never log; they return, wait or raise.
"""

from __future__ import annotations

import asyncio
from collections import deque
from collections.abc import AsyncIterator, Awaitable, Callable
from contextlib import asynccontextmanager
from typing import Any

from optdesk.ibkr.timing import Clock, with_timeout

DEFAULT_STARTS_PER_SECOND = 40
DEFAULT_LINES = 70
DEFAULT_SNAPSHOTS = 50
DETAILS_MIN = 1
DETAILS_START = 2
DETAILS_MAX = 5
DETAILS_TIMEOUT_S = 20
DETAILS_PAUSE_S = 30

_WINDOW_S = 1.0
_SAMPLE_COUNT = 10
_FAST_MEAN_S = 1.0
_SLOW_MEAN_S = 3.0
_OLDEST_MAX_S = 5.0
_UNRESOLVED_MAX = 2
_LINE_LIMIT_CODE = 101


class MessagePacer:
    """Sliding window of message starts; a cancel counts as a start.

    A cancel is queued and sent by a drain task, so a cancel inside a finally
    block never waits and the rate still holds.
    """

    def __init__(
        self, clock: Clock, starts_per_second: int = DEFAULT_STARTS_PER_SECOND
    ) -> None:
        self._clock = clock
        self._limit = starts_per_second
        self._starts: deque[float] = deque()
        self._cancels: deque[Callable[[], None]] = deque()
        self._drain: asyncio.Future[None] | None = None

    async def start(self) -> None:
        """Return once a start slot is free and take it."""
        while True:
            now = self._clock.monotonic()
            while self._starts and now - self._starts[0] >= _WINDOW_S:
                self._starts.popleft()
            if len(self._starts) < self._limit:
                self._starts.append(now)
                return
            await self._clock.sleep(self._starts[0] + _WINDOW_S - now)

    def cancel(self, send: Callable[[], None]) -> None:
        """Queue a cancel; send runs when the window allows. Never waits."""
        self._cancels.append(send)
        if self._drain is None or self._drain.done():
            self._drain = asyncio.ensure_future(self._drain_cancels())

    async def _drain_cancels(self) -> None:
        while self._cancels:
            await self.start()
            self._cancels.popleft()()


class LineBudget:
    """Leases of market-data lines (or of snapshots in flight), released in finally."""

    def __init__(self, limit: int = DEFAULT_LINES) -> None:
        self._limit = max(1, limit)
        self._in_use = 0
        self._waiters: list[asyncio.Future[None]] = []

    @property
    def limit(self) -> int:
        return self._limit

    @property
    def in_use(self) -> int:
        return self._in_use

    def shrink(self, code: int) -> None:
        """On error 101 lower the limit to the lines in use less one, not below one."""
        if code == _LINE_LIMIT_CODE:
            self._limit = max(1, min(self._limit, self._in_use - 1))

    @asynccontextmanager
    async def lease(self) -> AsyncIterator[None]:
        """Hold one line; wait while the budget is full."""
        while self._in_use >= self._limit:
            waiter: asyncio.Future[None] = asyncio.get_running_loop().create_future()
            self._waiters.append(waiter)
            try:
                await waiter
            finally:
                if waiter in self._waiters:
                    self._waiters.remove(waiter)
        self._in_use += 1
        try:
            yield
        finally:
            self._in_use -= 1
            for waiter in self._waiters:
                if not waiter.done():
                    waiter.set_result(None)


class DetailsLane:
    """Adaptive in-flight limit for contract-details style requests.

    Starts at DETAILS_START, rises by one when a window of ten answers started
    at the current limit has a mean under one second, halves (not below one)
    over three seconds, and empties the window at each change. A request that
    runs DETAILS_TIMEOUT_S seconds drops the limit to one and pauses starts for
    DETAILS_PAUSE_S seconds.
    """

    def __init__(self, clock: Clock) -> None:
        self._clock = clock
        self._limit = DETAILS_START
        self._samples: list[float] = []
        self._inflight: dict[int, float] = {}
        self._next_id = 0
        self._unresolved = 0
        self._skip_next = False
        self._pause_until = 0.0
        self._waiters: list[asyncio.Future[None]] = []

    @property
    def limit(self) -> int:
        return self._limit

    @property
    def in_flight(self) -> int:
        return len(self._inflight)

    async def run[T](self, make: Callable[[], Awaitable[T]]) -> T:
        """Wait for a start, run make, and learn from how long it took.

        Raises TimeoutError when no start was allowed or no answer came in
        DETAILS_TIMEOUT_S seconds.
        """
        await self._admit()
        ident = self._next_id
        self._next_id += 1
        started = self._clock.monotonic()
        started_limit = self._limit
        self._inflight[ident] = started
        task = asyncio.ensure_future(make())
        try:
            result = await with_timeout(
                self._clock, asyncio.shield(task), DETAILS_TIMEOUT_S
            )
        except TimeoutError:
            if not task.done():
                self._on_timeout(task)
            raise
        except BaseException:
            task.cancel()
            raise
        finally:
            self._inflight.pop(ident, None)
            self._wake()
        self._on_answer(self._clock.monotonic() - started, started_limit)
        return result

    def _blocked_for(self) -> float | None:
        now = self._clock.monotonic()
        if now < self._pause_until:
            return self._pause_until - now
        if len(self._inflight) >= self._limit or self._unresolved > _UNRESOLVED_MAX:
            return float("inf")
        if self._inflight and now - min(self._inflight.values()) > _OLDEST_MAX_S:
            return float("inf")
        return None

    async def _admit(self) -> None:
        deadline = self._clock.monotonic() + DETAILS_TIMEOUT_S
        while (blocked := self._blocked_for()) is not None:
            remaining = deadline - self._clock.monotonic()
            if remaining <= 0:
                raise TimeoutError("no start allowed in the details lane")
            await self._wait_change(min(blocked, remaining))

    async def _wait_change(self, seconds: float) -> None:
        waiter: asyncio.Future[None] = asyncio.get_running_loop().create_future()
        self._waiters.append(waiter)
        try:
            await with_timeout(self._clock, waiter, seconds)
        except TimeoutError:
            pass
        finally:
            if waiter in self._waiters:
                self._waiters.remove(waiter)

    def _wake(self) -> None:
        for waiter in self._waiters:
            if not waiter.done():
                waiter.set_result(None)

    def _on_timeout(self, task: asyncio.Future[Any]) -> None:
        self._limit = DETAILS_MIN
        self._samples.clear()
        self._pause_until = self._clock.monotonic() + DETAILS_PAUSE_S
        self._unresolved += 1
        self._skip_next = True
        task.add_done_callback(self._late_answer)

    def _late_answer(self, task: asyncio.Future[Any]) -> None:
        if not task.cancelled():
            task.exception()  # retrieved so the loop does not report it
        self._unresolved = max(0, self._unresolved - 1)
        self._skip_next = False
        self._wake()

    def _on_answer(self, seconds: float, started_limit: int) -> None:
        if self._skip_next:
            self._skip_next = False
            return
        if started_limit != self._limit:
            return
        self._samples.append(seconds)
        if len(self._samples) < _SAMPLE_COUNT:
            return
        mean = sum(self._samples) / len(self._samples)
        if mean < _FAST_MEAN_S and self._limit < DETAILS_MAX:
            self._limit += 1
            self._samples.clear()
        elif mean > _SLOW_MEAN_S and self._limit > DETAILS_MIN:
            self._limit = max(DETAILS_MIN, self._limit // 2)
            self._samples.clear()
        else:
            self._samples.clear()
