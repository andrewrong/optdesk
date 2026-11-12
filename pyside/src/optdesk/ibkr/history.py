"""Daily bars of the ibkr module: the pacer and the words of a bars request.

Pure of ib_async: the adapter hands `HistoryReader.bars` a fetch function and
this file decides when it may run and what words it sends. Time comes from the
injected Clock. Nothing here logs.
"""

from __future__ import annotations

from collections.abc import Awaitable, Callable, Sequence
from dataclasses import dataclass
from datetime import date

from optdesk.ibkr import spec
from optdesk.ibkr.api import HistoryBar, HistoryRequest
from optdesk.ibkr.timing import Clock

MIN_GAP_S = 11.0  # every start waits this long after the previous one (bars of 1 min and larger)
IDENTICAL_S = 15.0  # IBKR refuses an identical request within 15 seconds
PACING_RETRY_S = 60.0  # wait after an IBKR pacing answer before the one retry
PACING_CODE = 162
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
        raise ValueError(
            "kind is not one of trades, adjusted_last, midpoint or option_implied_vol"
        ) from None
    return WireRequest(
        end_date=end_date_text(request.end_day),
        duration=duration_text(request.duration_days),
        bar_size=BAR_SIZE,
        what_to_show=what,
        format_date=FORMAT_DATE,
    )


class HistoryReader:
    """Paces historical requests on one clock and sends them through a fetch function.

    Starts are one at a time, in the order callers arrive, at least 11 seconds
    apart, and no identical request starts within 15 seconds. A pacing violation
    (code 162 with that text) is waited out for 60 seconds and fetched once more;
    any other answer, 162 with other text included, is raised at once.
    """

    def __init__(self, clock: Clock) -> None:
        self._clock = clock
        self._next_start = 0.0
        self._last: dict[HistoryRequest, float] = {}

    async def _take_slot(self, request: HistoryRequest) -> None:
        now = self._clock.monotonic()
        start = max(now, self._next_start)
        sent = self._last.get(request)
        if sent is not None:
            start = max(start, sent + IDENTICAL_S)
        self._next_start = start + MIN_GAP_S
        self._last = {r: t for r, t in self._last.items() if start - t < IDENTICAL_S}
        self._last[request] = start
        if start > now:
            await self._clock.sleep(start - now)

    async def bars(
        self,
        request: HistoryRequest,
        fetch: Callable[[WireRequest], Awaitable[Sequence[HistoryBar]]],
    ) -> Sequence[HistoryBar]:
        """Wait for a slot, then fetch; ValueError for a request with no words."""
        words = wire_request(request)
        await self._take_slot(request)
        try:
            return await fetch(words)
        except spec.BrokerError as exc:
            if exc.code != PACING_CODE or "pacing violation" not in exc.text.lower():
                raise
        await self._clock.sleep(PACING_RETRY_S)
        await self._take_slot(request)
        return await fetch(words)
