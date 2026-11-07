"""A fake Clock: sleep parks until advance reaches its time; nothing really waits."""

from __future__ import annotations

import asyncio
from datetime import UTC, datetime, timedelta


class FakeClock:
    """Implements optdesk.ibkr.timing.Clock over a time that only advance moves."""

    def __init__(self, start: datetime | None = None) -> None:
        self._start = start or datetime(2026, 1, 2, 14, 30, tzinfo=UTC)
        self._elapsed = 0.0
        self._parked: list[tuple[float, asyncio.Future[None]]] = []

    def monotonic(self) -> float:
        return self._elapsed

    def now(self) -> datetime:
        return self._start + timedelta(seconds=self._elapsed)

    @property
    def parked(self) -> int:
        """Number of sleepers waiting for advance."""
        return len(self._parked)

    async def sleep(self, seconds: float) -> None:
        if seconds <= 0:
            await asyncio.sleep(0)
            return
        future: asyncio.Future[None] = asyncio.get_running_loop().create_future()
        entry = (self._elapsed + seconds, future)
        self._parked.append(entry)
        try:
            await future
        finally:
            if entry in self._parked:
                self._parked.remove(entry)

    @staticmethod
    async def _settle() -> None:
        """Yield the loop a few turns so ready tasks reach their next sleep."""
        for _ in range(5):
            await asyncio.sleep(0)

    async def advance(self, seconds: float) -> None:
        """Move time forward, waking every sleeper whose time has come, in order."""
        await self._settle()
        target = self._elapsed + seconds
        while True:
            due = sorted(
                (e for e in self._parked if e[0] <= target), key=lambda e: e[0]
            )
            if not due:
                break
            when, future = due[0]
            self._parked.remove(due[0])
            self._elapsed = max(self._elapsed, when)
            if not future.done():
                future.set_result(None)
            await self._settle()
        self._elapsed = target
