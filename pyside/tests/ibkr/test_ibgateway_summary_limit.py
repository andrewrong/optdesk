"""The library's own account-summary request on a 1102 is gone, so the two-request limit holds.

A real ib_async IB answers error 1102 with a reqAccountSummary it never cancels;
open_session takes that handler out, and the 1102 handler of IbGateway stays.
"""

from __future__ import annotations

import asyncio
from typing import Any, Self

from real_ib import RealIb, real_ib, sent_messages
from test_ibgateway_real_ib import _ID, _connected, _turn, _until

from optdesk.ibkr.accounts import SUMMARY_REQ_ID, drop_resubscribe

CANCEL_SUMMARY_ID = "63"


def _summary_requests(stub: RealIb) -> list[list[str]]:
    return [m for m in sent_messages(stub) if m and m[0] == _ID["summary"]]


async def _emit_1102(stub: RealIb, times: int) -> None:
    for _ in range(times):
        stub.ib.errorEvent.emit(-1, 1102, "connectivity restored", None)
    for _ in range(10):
        await _turn()


def test_error_1102_sends_no_account_summary() -> None:
    async def run(times: int) -> list[list[str]]:
        stub = real_ib()
        await _connected(stub)
        await _emit_1102(stub, times)
        return sent_messages(stub)

    for times in (1, 3):
        sent = asyncio.run(run(times))
        assert [m for m in sent if m and m[0] == _ID["summary"]] == []


def test_error_1102_still_revives_the_gateway() -> None:
    async def run() -> tuple[bool, bool]:
        stub = real_ib()
        gateway = await _connected(stub)
        stub.ib.errorEvent.emit(-1, 1100, "connectivity lost", None)
        lost = (await gateway.status()).connected
        await _emit_1102(stub, 1)
        return lost, (await gateway.status()).connected

    lost, restored = asyncio.run(run())
    assert lost is False
    assert restored is True


def test_read_account_after_three_restores() -> None:
    async def run() -> tuple[Any, list[list[str]]]:
        stub = real_ib()
        gateway = await _connected(stub)
        await _emit_1102(stub, 3)
        task = asyncio.ensure_future(gateway.account(summary=True, positions=False))
        await _until(stub, task, 1)
        stub.ib.wrapper.accountSummaryEnd(SUMMARY_REQ_ID)
        rows = await task
        return rows, sent_messages(stub)

    rows, sent = asyncio.run(run())
    assert rows is not None
    assert [m[0] for m in sent] == [_ID["summary"], CANCEL_SUMMARY_ID]
    assert sent[0][2] == str(SUMMARY_REQ_ID)


def test_drop_resubscribe_leaves_an_ib_without_the_handler_alone() -> None:
    class Event(list[Any]):
        def __isub__(self, other: Any) -> Self:
            raise AssertionError("nothing to remove")

    class Bare:
        errorEvent = Event()

    drop_resubscribe(Bare())
