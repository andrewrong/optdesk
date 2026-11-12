"""The history pacer and the words of a bars request, over a fake gateway and the kit's fake clock."""

from __future__ import annotations

import asyncio
from dataclasses import dataclass, field
from typing import Any

import pytest
from kit.fakeclock import FakeClock

from optdesk.ibkr import spec
from optdesk.ibkr.api import HistoryBar, HistoryKey, HistoryRequest
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


async def drain(clock: FakeClock, task: asyncio.Future[Any]) -> None:
    """Advance the fake time in steps until the task is done."""
    for _ in range(200):
        if task.done():
            return
        await clock.advance(5)


def test_starts_are_one_at_a_time_11_seconds_apart() -> None:
    async def run() -> list[float]:
        clock, gateway, reader = setup()
        tasks = [await_later(reader, gateway, request(con_id=i + 1)) for i in range(3)]
        await asyncio.sleep(0)
        await clock.advance(10)
        assert len(gateway.sent) == 1
        await clock.advance(1)
        assert len(gateway.sent) == 2
        await clock.advance(11)
        await asyncio.gather(*tasks)
        return [t for t, _ in gateway.sent]

    assert asyncio.run(run()) == [0.0, 11.0, 22.0]


def test_no_cap_after_55_requests() -> None:
    async def run() -> list[float]:
        clock, gateway, reader = setup()
        for i in range(70):
            task = await_later(reader, gateway, request(con_id=i + 1))
            await drain(clock, task)
            assert task.done()
            task.result()
        return [t for t, _ in gateway.sent]

    times = asyncio.run(run())
    assert len(times) == 70
    assert times[69] == 11.0 * 69


class PacingGateway:
    """Answers the first `codes` sends with a BrokerError of that code, then bars."""

    def __init__(
        self, clock: FakeClock, codes: list[int], text: str = "pacing violation"
    ) -> None:
        self.clock = clock
        self.codes = codes
        self.text = text
        self.sent: list[float] = []

    async def fetch(self, words: WireRequest) -> list[HistoryBar]:
        self.sent.append(self.clock.monotonic())
        if len(self.sent) <= len(self.codes):
            raise spec.BrokerError(self.codes[len(self.sent) - 1], self.text)
        return [HistoryBar("2026-01-02", "1", "2", "0.5", "1.5", None)]


def test_pacing_answer_162_is_retried_after_60_seconds() -> None:
    async def run() -> tuple[list[float], int]:
        clock = FakeClock()
        gateway = PacingGateway(clock, [162])
        task = asyncio.ensure_future(
            HistoryReader(clock).bars(request(), gateway.fetch)
        )
        await drain(clock, task)
        return gateway.sent, len(task.result())

    sent, n = asyncio.run(run())
    assert sent == [0.0, 60.0]
    assert n == 1


def test_162_without_pacing_text_is_raised_at_once() -> None:
    async def run() -> list[float]:
        clock = FakeClock()
        gateway = PacingGateway(clock, [162], "HMDS query returned no data")
        task = asyncio.ensure_future(
            HistoryReader(clock).bars(request(), gateway.fetch)
        )
        await drain(clock, task)
        with pytest.raises(spec.BrokerError) as info:
            task.result()
        assert info.value.code == 162
        return gateway.sent

    assert asyncio.run(run()) == [0.0]


def test_pacing_violation_text_is_case_insensitive() -> None:
    async def run() -> list[float]:
        clock = FakeClock()
        gateway = PacingGateway(clock, [162], "Pacing Violation: too many requests")
        task = asyncio.ensure_future(
            HistoryReader(clock).bars(request(), gateway.fetch)
        )
        await drain(clock, task)
        task.result()
        return gateway.sent

    assert asyncio.run(run()) == [0.0, 60.0]


@pytest.mark.parametrize("codes", [[162, 162], [200]])
def test_second_162_and_other_codes_are_raised(codes: list[int]) -> None:
    async def run() -> list[float]:
        clock = FakeClock()
        gateway = PacingGateway(clock, codes)
        task = asyncio.ensure_future(
            HistoryReader(clock).bars(request(), gateway.fetch)
        )
        await drain(clock, task)
        with pytest.raises(spec.BrokerError) as info:
            task.result()
        assert info.value.code == codes[-1]
        return gateway.sent

    assert len(asyncio.run(run())) == len(codes)


def test_identical_request_waits_15_seconds() -> None:
    async def run() -> list[float]:
        clock, gateway, reader = setup()
        await reader.bars(request(), gateway.fetch)
        other = await_later(reader, gateway, request(con_id=2))
        same = await_later(reader, gateway, request())
        await clock.advance(10)
        assert len(gateway.sent) == 1
        await clock.advance(12)
        await asyncio.gather(other, same)
        return [t for t, _ in gateway.sent]

    assert asyncio.run(run()) == [0.0, 11.0, 22.0]


def await_later(
    reader: HistoryReader, gateway: FakeGateway, req: HistoryRequest
) -> asyncio.Future[object]:
    return asyncio.ensure_future(reader.bars(req, gateway.fetch))


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
