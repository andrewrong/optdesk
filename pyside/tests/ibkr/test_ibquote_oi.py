"""OPTION_EOD waits a bounded grace for open_interest and ends PARTIAL without it."""

from __future__ import annotations

import asyncio

from ib_async import OptionComputation, Ticker
from kit.fakeclock import FakeClock
from test_ibgateway import FakeIb, fill, opened, spin

from optdesk.ibkr.ibgateway import IbGateway, ticker_fields
from optdesk.ibkr.model import QuoteRow, QuoteTarget
from optdesk.ibkr.spec import OI_GRACE_S

BASE = {"bid": 1.0, "ask": 1.1, "close": 1.05}


def model(ticker: Ticker) -> None:
    ticker.modelGreeks = OptionComputation(
        0,
        impliedVol=0.2,
        delta=0.5,
        gamma=0.1,
        vega=0.2,
        theta=-0.1,
        undPrice=100.0,
    )


def start(
    gateway: IbGateway, kind: str = "option_eod"
) -> tuple[list[QuoteRow], asyncio.Future[None]]:
    rows: list[QuoteRow] = []

    async def keep(row: QuoteRow) -> None:
        rows.append(row)

    task = asyncio.ensure_future(gateway.quotes(kind, [QuoteTarget(9)], keep))
    return rows, task


def feed(ib: FakeIb) -> Ticker:
    ticker = ib.tickers[9]
    fill(ticker, **BASE)
    model(ticker)
    ticker.updateEvent.emit(ticker)
    return ticker


async def feed_settled(ib: FakeIb) -> Ticker:
    ticker = feed(ib)
    await spin(50)
    return ticker


def test_oi_after_release_is_ok() -> None:
    async def run() -> tuple[QuoteRow, list[int]]:
        clock = FakeClock()
        gateway, ib = await opened(clock, lines=1)
        rows, task = start(gateway)
        await spin(50)
        ticker = await feed_settled(ib)
        await clock.advance(2)
        assert not rows and not ib.cancelled
        ticker.callOpenInterest = 1234.0
        ticker.updateEvent.emit(ticker)
        await spin(50)
        await task
        return rows[0], ib.cancelled

    row, cancelled = asyncio.run(run())
    assert row.status == "ok"
    assert dict(row.fields)["open_interest"] == 1234.0
    assert cancelled == [9]


def test_oi_missing_is_partial() -> None:
    async def run() -> tuple[QuoteRow, list[int]]:
        clock = FakeClock()
        gateway, ib = await opened(clock, lines=1)
        rows, task = start(gateway)
        await spin(50)
        await feed_settled(ib)
        await clock.advance(OI_GRACE_S + 1)
        await task
        return rows[0], ib.cancelled

    row, cancelled = asyncio.run(run())
    assert row.status == "partial"
    assert "open_interest" not in dict(row.fields)
    assert cancelled == [9]


def test_oi_zero_call_put_read() -> None:
    ticker = Ticker()
    ticker.callOpenInterest = 0.0
    ticker.putOpenInterest = 5200.0
    assert ticker_fields(ticker)["open_interest"] == 5200.0
    ticker = Ticker()
    ticker.callOpenInterest = 4100.0
    ticker.putOpenInterest = 0.0
    assert ticker_fields(ticker)["open_interest"] == 4100.0


def test_oi_lone_zero_is_absent() -> None:
    async def run(attribute: str) -> QuoteRow:
        clock = FakeClock()
        gateway, ib = await opened(clock, lines=1)
        rows, task = start(gateway)
        await spin(50)
        setattr(ib.tickers[9], attribute, 0.0)
        await feed_settled(ib)
        await clock.advance(OI_GRACE_S + 1)
        await task
        return rows[0]

    for attribute in ("callOpenInterest", "putOpenInterest"):
        row = asyncio.run(run(attribute))
        assert row.status == "partial"
        assert "open_interest" not in dict(row.fields)


def test_grace_inside_timeout() -> None:
    async def run() -> tuple[bool, bool]:
        clock = FakeClock()
        gateway, ib = await opened(clock, lines=1)
        rows, task = start(gateway)
        await spin(50)
        await clock.advance(18)
        await feed_settled(ib)
        await clock.advance(1.5)
        before = bool(rows)
        await clock.advance(0.6)
        await task
        return before, bool(rows) and rows[0].status == "partial"

    before, after = asyncio.run(run())
    assert not before
    assert after


def test_snapshot_kind_unchanged() -> None:
    async def run() -> QuoteRow:
        clock = FakeClock()
        gateway, ib = await opened(clock, lines=1)
        rows, task = start(gateway, "snapshot")
        await spin(50)
        fill(ib.tickers[9])
        ib.tickers[9].updateEvent.emit(ib.tickers[9])
        await spin(50)
        await task
        return rows[0]

    assert asyncio.run(run()).status == "ok"
