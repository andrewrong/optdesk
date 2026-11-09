"""Every IbGateway request path that sends to IB, run through a real ib_async IB.

The send of the IB is stubbed (real_ib.py); the library's own validation, hashing
and encoding run, so a request it refuses raises here as it does live.
"""

from __future__ import annotations

import asyncio
from collections.abc import Awaitable, Callable
from typing import Any

import pytest
from ib_async import IB
from kit.fakeclock import FakeClock
from real_ib import RealIb, real_ib, sent_messages

from optdesk.ibkr.api import DividendKey, DividendReading
from optdesk.ibkr.ibgateway import IbGateway
from optdesk.ibkr.model import QuoteRow, QuoteTarget
from optdesk.ibkr.spec import OPTION_EOD

_ID = {"quotes": "1", "details": "9", "chains": "78", "summary": "62", "pos": "61"}


async def _connected(stub: RealIb) -> IbGateway:
    async def connect_async(host: str, port: int, client_id: int) -> None:
        return None

    stub.ib.client.connectAsync = connect_async  # type: ignore[method-assign,assignment]
    gateway = IbGateway(
        lambda client: None,
        host="127.0.0.1",
        port=4002,
        client_id=33,
        clock=FakeClock(),
        ib_factory=lambda: stub.ib,
    )
    await gateway.open_session()
    stub.sent.clear()
    return gateway


async def _drive(
    stub: RealIb, call: Callable[[IbGateway], Awaitable[Any]]
) -> list[list[str]]:
    """Run call until a message reaches the send stub; a library error is raised."""
    gateway = await _connected(stub)
    task = asyncio.ensure_future(call(gateway))
    try:
        for _ in range(200):
            if stub.sent or task.done():
                break
            await asyncio.sleep(0)
        if task.done() and not stub.sent:
            task.result()
    finally:
        task.cancel()
        await asyncio.gather(task, return_exceptions=True)
    return sent_messages(stub)


def _run(call: Callable[[IbGateway], Awaitable[Any]]) -> list[list[str]]:
    return asyncio.run(_drive(real_ib(), call))


def test_real_ib_is_a_real_ib() -> None:
    assert isinstance(real_ib().ib, IB)
    assert real_ib().ib is not real_ib().ib


def test_real_ib_quotes_send() -> None:
    async def noop(row: QuoteRow) -> None:
        return None

    sent = _run(lambda g: g.quotes(OPTION_EOD, [QuoteTarget(con_id=756733)], noop))
    assert sent and sent[0][0] == _ID["quotes"]
    assert "756733" in sent[0]


@pytest.mark.xfail(strict=True, reason="dividends con_id 0 fix not landed")
def test_real_ib_dividends_send() -> None:
    async def noop(reading: DividendReading) -> None:
        return None

    key = DividendKey("SPY", "STK", "SMART", "USD", 0)
    sent = _run(lambda g: g.dividends([key], noop))
    assert sent and sent[0][0] == _ID["quotes"]


def test_real_ib_resolve_send() -> None:
    sent = _run(
        lambda g: g.resolve(
            sec_type="STK",
            symbol="SPY",
            exchange="SMART",
            currency="USD",
            expiry="",
            strike="",
            right="",
            multiplier="",
            trading_class="",
        )
    )
    assert sent and sent[0][0] == _ID["details"]
    assert "SPY" in sent[0]


def test_real_ib_chains_send() -> None:
    sent = _run(lambda g: g.chains(sec_type="STK", symbol="SPY", con_id=756733))
    assert sent and sent[0][0] == _ID["chains"]
    assert "756733" in sent[0]


def test_real_ib_account_send() -> None:
    sent = _run(lambda g: g.account(summary=True, positions=True))
    assert sent
    ids = [message[0] for message in sent]
    assert _ID["summary"] in ids
    assert _ID["pos"] in ids


def test_real_ib_executions_send() -> None:
    sent = _run(lambda g: g.executions())
    assert sent and sent[0][0] == "7"
