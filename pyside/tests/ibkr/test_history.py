"""The history pacer and the words of a bars request, over a fake gateway and the kit's fake clock."""

from __future__ import annotations

import asyncio
from dataclasses import dataclass, field

import pytest
from kit.fakeclock import FakeClock

from optdesk.ibkr.api import HistoryBar, HistoryKey, HistoryPaced, HistoryRequest
from optdesk.ibkr.history import HistoryReader, WireRequest

KEY = HistoryKey("SPY", "STK", "SMART", "USD", 756733)


def request(
    kind: str = "trades", days: int = 5, end_day: str = "", con_id: int = 1
) -> HistoryRequest:
    key = HistoryKey("SPY", "STK", "SMART", "USD", con_id)
    return HistoryRequest(key, kind, days, end_day)


@dataclass
class FakeGateway:
    """Records the words of every send and the time it was sent."""

    clock: FakeClock
    sent: list[tuple[float, WireRequest]] = field(default_factory=list)

    async def fetch(self, words: WireRequest) -> list[HistoryBar]:
        self.sent.append((self.clock.monotonic(), words))
        return [HistoryBar("2026-01-02", "1", "2", "0.5", "1.5", None)]


def setup() -> tuple[FakeClock, FakeGateway, HistoryReader]:
    clock = FakeClock()
    return clock, FakeGateway(clock), HistoryReader(clock)


async def drain(clock: FakeClock, task: asyncio.Future[object]) -> None:
    """Advance the fake time in steps until the task is done."""
    for _ in range(200):
        if task.done():
            return
        await clock.advance(5)


def test_pacer_limits_to_55_per_10_minutes() -> None:
    async def run() -> list[float]:
        clock, gateway, reader = setup()
        for i in range(30):
            await reader.bars(request(con_id=i + 1), gateway.fetch)
        await clock.advance(300)
        for i in range(25):
            await reader.bars(request(con_id=i + 100), gateway.fetch)
        assert len(gateway.sent) == 55
        late = await_later(reader, gateway, request(con_id=500))
        await clock.advance(299)  # 599 s: the window still holds 55 starts
        assert len(gateway.sent) == 55
        await clock.advance(1)
        await late
        return [t for t, _ in gateway.sent]

    times = asyncio.run(run())
    assert times[:30] == [0.0] * 30
    assert times[30:55] == [300.0] * 25
    assert times[55] == 600.0


def test_identical_request_waits_15_seconds() -> None:
    async def run() -> list[float]:
        clock, gateway, reader = setup()
        await reader.bars(request(), gateway.fetch)
        other = await_later(reader, gateway, request(con_id=2))
        same = await_later(reader, gateway, request())
        await clock.advance(14)
        assert len(gateway.sent) == 2
        await clock.advance(1)
        await asyncio.gather(other, same)
        return [t for t, _ in gateway.sent]

    assert asyncio.run(run()) == [0.0, 0.0, 15.0]


def await_later(
    reader: HistoryReader, gateway: FakeGateway, req: HistoryRequest
) -> asyncio.Future[object]:
    return asyncio.ensure_future(reader.bars(req, gateway.fetch))


def test_wait_over_300_seconds_is_paced() -> None:
    async def run() -> tuple[int, str]:
        clock, gateway, reader = setup()
        for i in range(55):
            await reader.bars(request(con_id=i + 1), gateway.fetch)
        await clock.advance(299)  # the 56th would wait 301 s
        with pytest.raises(HistoryPaced):
            await reader.bars(request(con_id=99), gateway.fetch)
        sent = len(gateway.sent)
        await clock.advance(1)  # now it waits exactly 300 s: allowed
        task = await_later(reader, gateway, request(con_id=99))
        await drain(clock, task)
        return sent, "done" if task.done() else "open"

    assert asyncio.run(run()) == (55, "done")


@pytest.mark.parametrize(
    ("kind", "word"),
    [
        ("trades", "TRADES"),
        ("adjusted_last", "ADJUSTED_LAST"),
        ("midpoint", "MIDPOINT"),
        ("option_implied_vol", "OPTION_IMPLIED_VOLATILITY"),
    ],
)
def test_kind_maps_to_tws_word(kind: str, word: str) -> None:
    async def run() -> WireRequest:
        _, gateway, reader = setup()
        await reader.bars(request(kind), gateway.fetch)
        return gateway.sent[0][1]

    words = asyncio.run(run())
    assert words.what_to_show == word
    assert words.bar_size == "1 day"
    assert words.format_date == 1


@pytest.mark.parametrize(
    ("days", "text"),
    [
        (1, "1 D"),
        (5, "5 D"),
        (365, "365 D"),
        (366, "2 Y"),
        (365 * 2 + 1, "3 Y"),
        (400, "2 Y"),
        (730, "2 Y"),
    ],
)
def test_duration_days_maps_to_tws_string(days: int, text: str) -> None:
    async def run() -> str:
        _, gateway, reader = setup()
        await reader.bars(request(days=days), gateway.fetch)
        return gateway.sent[0][1].duration

    assert asyncio.run(run()) == text


@pytest.mark.parametrize(
    ("end_day", "text"),
    [("", ""), ("2026-01-02", "20260102-23:59:59")],
)
def test_end_day_maps_to_end_date_text(end_day: str, text: str) -> None:
    async def run() -> str:
        _, gateway, reader = setup()
        await reader.bars(request(end_day=end_day), gateway.fetch)
        return gateway.sent[0][1].end_date

    assert asyncio.run(run()) == text


def test_malformed_end_day_is_a_value_error() -> None:
    async def run() -> None:
        _, gateway, reader = setup()
        await reader.bars(request(end_day="2026/01/02"), gateway.fetch)

    with pytest.raises(ValueError, match="end_day"):
        asyncio.run(run())
