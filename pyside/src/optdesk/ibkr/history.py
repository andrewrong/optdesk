"""Daily bars of the ibkr module: the pacer and the words of a bars request.

Pure of ib_async: the adapter hands `HistoryReader.bars` a fetch function and
this file decides when it may run and what words it sends. Time comes from the
injected Clock. Nothing here logs.
"""

from __future__ import annotations

from collections import deque
from collections.abc import Awaitable, Callable, Sequence
from dataclasses import dataclass
from datetime import date

from optdesk.ibkr.api import HistoryBar, HistoryPaced, HistoryRequest
from optdesk.ibkr.timing import Clock

MAX_REQUESTS = 55  # IBKR: 55 historical requests per 10 minutes
WINDOW_S = 600.0
IDENTICAL_S = 15.0  # IBKR refuses an identical request within 15 seconds
MAX_WAIT_S = 300.0  # a longer wait is a paced answer
MAX_DAYS_AS_D = 365

BAR_SIZE = "1 day"
FORMAT_DATE = 1  # exchange date, as IBKR gives it

_WHAT_TO_SHOW = {
    "trades": "TRADES",
    "adjusted_last": "ADJUSTED_LAST",
    "midpoint": "MIDPOINT",
    "option_implied_vol": "OPTION_IMPLIED_VOLATILITY",
}


@dataclass(frozen=True)
class WireRequest:
    """The words of one historical request: IBKR's own names, in the order of reqHistoricalData."""

    end_date: str
    duration: str
    bar_size: str
    what_to_show: str
    format_date: int


def duration_text(days: int) -> str:
    """Map days to a TWS duration: "N D" up to 365, else whole years rounded up."""
    if days <= MAX_DAYS_AS_D:
        return f"{days} D"
    return f"{-(-days // MAX_DAYS_AS_D)} Y"


def end_date_text(end_day: str) -> str:
    """Map YYYY-MM-DD to IBKR's UTC end-date text at the end of that day; empty stays empty."""
    if not end_day:
        return ""
    try:
        day = date.fromisoformat(end_day)
    except ValueError:
        raise ValueError("end_day must be YYYY-MM-DD or empty") from None
    if end_day != day.isoformat():
        raise ValueError("end_day must be YYYY-MM-DD or empty")
    return f"{day:%Y%m%d}-23:59:59"


def wire_request(request: HistoryRequest) -> WireRequest:
    """Return the words IBKR is sent for the request; ValueError for a word it has no name for."""
    try:
        what = _WHAT_TO_SHOW[request.kind]
    except KeyError:
        raise ValueError("kind is not one of trades, adjusted_last, midpoint or option_implied_vol") from None
    return WireRequest(
        end_date=end_date_text(request.end_day),
        duration=duration_text(request.duration_days),
        bar_size=BAR_SIZE,
        what_to_show=what,
        format_date=FORMAT_DATE,
    )


class HistoryReader:
    """Paces historical requests on one clock and sends them through a fetch function.

    At most 55 starts per 10 minutes and no identical request within 15 seconds.
    A request that would have to wait more than 300 seconds raises HistoryPaced
    and takes no slot.
    """

    def __init__(self, clock: Clock) -> None:
        self._clock = clock
        self._starts: deque[float] = deque()
        self._last: dict[HistoryRequest, float] = {}

    def _wait(self, request: HistoryRequest) -> float:
        now = self._clock.monotonic()
        while self._starts and now - self._starts[0] >= WINDOW_S:
            self._starts.popleft()
        wait = 0.0
        if len(self._starts) >= MAX_REQUESTS:
            wait = self._starts[0] + WINDOW_S - now
        sent = self._last.get(request)
        if sent is not None:
            wait = max(wait, sent + IDENTICAL_S - now)
        return wait

    async def _take_slot(self, request: HistoryRequest) -> None:
        while (wait := self._wait(request)) > 0:
            if wait > MAX_WAIT_S:
                raise HistoryPaced("historical requests are paced")
            await self._clock.sleep(wait)
        now = self._clock.monotonic()
        self._starts.append(now)
        self._last = {r: t for r, t in self._last.items() if now - t < IDENTICAL_S}
        self._last[request] = now

    async def bars(
        self,
        request: HistoryRequest,
        fetch: Callable[[WireRequest], Awaitable[Sequence[HistoryBar]]],
    ) -> Sequence[HistoryBar]:
        """Wait for a slot, then fetch; ValueError for a request with no words."""
        words = wire_request(request)
        await self._take_slot(request)
        return await fetch(words)
