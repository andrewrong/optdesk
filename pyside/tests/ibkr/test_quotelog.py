"""quote_done: the text of the line and one record per contract of a snapshot."""

from __future__ import annotations

import asyncio
import logging

import pytest
from kit.fakeclock import FakeClock
from test_ibgateway import opened, spin
from test_ibquote_oi import BASE, model

from optdesk.ibkr.missing import NOT_RECEIVED_GRACE
from optdesk.ibkr.model import QuoteRow, QuoteTarget
from optdesk.ibkr.quotelog import quote_done_line
from optdesk.ibkr.spec import OI_GRACE_S


def test_quote_done_line() -> None:
    missing = ((("volume"), 1, 0), ("open_interest", NOT_RECEIVED_GRACE, 0))
    assert quote_done_line("partial", 1500, 20000, missing) == (
        "quote_done status=partial t_release_ms=1500 t_oi_ms=20000"
        " missing=open_interest,volume"
    )
    assert quote_done_line("ok", 0, 0, ()) == (
        "quote_done status=ok t_release_ms=0 t_oi_ms=0 missing=-"
    )


def test_quote_done_logged_once_per_contract(caplog: pytest.LogCaptureFixture) -> None:
    async def run() -> list[QuoteRow]:
        clock = FakeClock()
        gateway, ib = await opened(clock, lines=2)
        rows: list[QuoteRow] = []

        async def keep(row: QuoteRow) -> None:
            rows.append(row)

        task = asyncio.ensure_future(
            gateway.quotes("option_eod", [QuoteTarget(8), QuoteTarget(9)], keep)
        )
        await spin(50)
        for con_id, oi in ((8, None), (9, 1234.0)):
            ticker = ib.tickers[con_id]
            for name, value in BASE.items():
                setattr(ticker, name, value)
            model(ticker)
            if oi is not None:
                ticker.callOpenInterest = oi
                ticker.last = 1.05
                ticker.volume = 10.0
            ticker.updateEvent.emit(ticker)
            await spin(50)
        await clock.advance(OI_GRACE_S)
        await task
        return rows

    with caplog.at_level(logging.INFO):
        asyncio.run(run())
    lines = [r.getMessage() for r in caplog.records if "quote_done" in r.getMessage()]
    assert len(lines) == 2
    partial = [x for x in lines if "status=partial" in x]
    full = [x for x in lines if "t_oi_ms=0 " in x]
    assert len(partial) == 1 and len(full) == 1
    assert "t_oi_ms=20000" in partial[0] and "open_interest" in partial[0].split(
        "missing="
    )[1].split(",")
    assert full[0].endswith("missing=-") and "status=ok" in full[0]
