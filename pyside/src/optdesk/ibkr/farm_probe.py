"""The farm probe, main only: one tiny history request every FARM_PROBE_EVERY_S.

FARM_PROBE_FAILS consecutive failures while the clock probe is good mark the
history farm broken; the first good answer clears it. Only a wait that ran out
counts as a failure. Every wait goes through the injected Clock.
"""

from __future__ import annotations

from collections.abc import Awaitable, Callable

from optdesk.ibkr.api import HistoryKey, HistoryRequest, HistoryUnavailable
from optdesk.ibkr.farms import Farms
from optdesk.ibkr.health_constants import (
    FARM_PROBE_EVERY_S,
    FARM_PROBE_FAILS,
    HISTORY_WAIT_S,
    PROBE_WAIT_S,
)
from optdesk.ibkr.timing import Clock, with_timeout


async def history_wait[T](clock: Clock, request: Awaitable[T]) -> T:
    """Await a history request for HISTORY_WAIT_S, else raise HistoryUnavailable."""
    try:
        return await with_timeout(clock, request, HISTORY_WAIT_S)
    except TimeoutError:
        raise HistoryUnavailable("history_unavailable") from None


FARM_NAME = "ushmds"  # the US history farm the probe request goes through
PROBE_REQUEST = HistoryRequest(  # one daily bar of a liquid ETF, named here only
    key=HistoryKey("SPY", "STK", "SMART", "USD", 756733),
    kind="trades",
    duration_days=1,
    end_day="",
)


class FarmProbe:
    """Counts failed history probes and tells the farm board."""

    def __init__(
        self,
        clock: Clock,
        farms: Farms,
        clock_good: Callable[[], bool],
        ask: Callable[[], Awaitable[object]],
    ) -> None:
        self._clock = clock
        self._farms = farms
        self._clock_good = clock_good
        self._ask = ask
        self.fails = 0

    async def step(self) -> None:
        """Run one probe; it does nothing while the clock probe is failing."""
        if not self._clock_good():
            return
        try:
            await with_timeout(self._clock, self._ask(), PROBE_WAIT_S)
        except (TimeoutError, HistoryUnavailable):
            self.fails += 1
            if self.fails >= FARM_PROBE_FAILS:
                self._farms.probe_broken(FARM_NAME)
        except Exception:  # noqa: BLE001 - a refusal or a lost session is not a dead farm
            return
        else:
            self.fails = 0
            self._farms.probe_ok(FARM_NAME)

    async def run(self) -> None:
        """Probe every FARM_PROBE_EVERY_S until cancelled."""
        while True:
            await self._clock.sleep(FARM_PROBE_EVERY_S)
            await self.step()
