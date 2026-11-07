"""Time seam of the ibkr module: a Clock, its system form and the one timeout.

Every wait and every timeout of src goes through a Clock, so a fake clock in a
test governs all of them and no test waits in real time. This file imports no
ib_async and no generated code.
"""

from __future__ import annotations

import asyncio
import time
from collections.abc import Awaitable
from datetime import UTC, datetime
from typing import Protocol, cast


class Clock(Protocol):
    """Source of time and of waiting."""

    def monotonic(self) -> float:
        """Return seconds on a clock that never goes back."""
        ...

    def now(self) -> datetime:
        """Return the current time, timezone-aware, in UTC."""
        ...

    async def sleep(self, seconds: float) -> None:
        """Wait for the given number of seconds of this clock."""
        ...


class SystemClock:
    """The real clock; the only place of src that calls asyncio.sleep."""

    def monotonic(self) -> float:
        return time.monotonic()

    def now(self) -> datetime:
        return datetime.now(UTC)

    async def sleep(self, seconds: float) -> None:
        await asyncio.sleep(seconds)


async def with_timeout[T](clock: Clock, awaitable: Awaitable[T], seconds: float) -> T:
    """Await awaitable, or raise TimeoutError once clock.sleep(seconds) returns.

    The awaitable and the sleep race; the loser is cancelled and awaited. This
    is the only timeout mechanism of src.
    """

    async def run() -> object:
        return await awaitable

    async def expire() -> object:
        await clock.sleep(seconds)
        return None

    work = asyncio.ensure_future(run())
    timer = asyncio.ensure_future(expire())
    try:
        await asyncio.wait({work, timer}, return_when=asyncio.FIRST_COMPLETED)
        if work.done():
            return cast(T, work.result())
        raise TimeoutError(f"timed out after {seconds} s")
    finally:
        for task in (work, timer):
            if not task.done():
                task.cancel()
        await asyncio.gather(work, timer, return_exceptions=True)
