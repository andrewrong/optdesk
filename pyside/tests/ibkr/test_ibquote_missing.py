"""An OPTION_EOD quote reports the fields it did not deliver and why."""

from __future__ import annotations

import asyncio

from ib_async import OptionComputation
from kit.fakeclock import FakeClock
from test_ibgateway import collect, fill, opened, spin

from optdesk.ibkr.model import QuoteRow


def test_quote_reports_missing_after_grace() -> None:
    async def run() -> QuoteRow:
        clock = FakeClock()
        gateway, fake = await opened(clock)
        rows, task = await collect(gateway, "option_eod", [91])
        await spin()
        ticker = fake.tickers[91]
        fill(ticker, bid=1.0, ask=1.1, close=1.0, volume=3.0)
        ticker.modelGreeks = OptionComputation(
            tickAttrib=0,
            impliedVol=0.3,
            delta=0.5,
            gamma=0.1,
            vega=0.2,
            theta=-0.1,
            undPrice=10.0,
        )
        ticker.updateEvent.emit(ticker)
        await spin()
        await clock.advance(60)
        await task
        return rows[0]

    row = asyncio.run(run())
    assert row.status == "partial"
    assert row.missing == (("last", 1, 0), ("open_interest", 2, 0))
