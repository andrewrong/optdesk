"""The library's own account-summary request on a 1102 is gone, so the two-request limit holds.

A real ib_async IB answers error 1102 with a reqAccountSummary it never cancels;
open_session takes that handler out, and the 1102 handler of IbGateway stays.
"""

from __future__ import annotations

import asyncio
from typing import Any, Self

import ib_async
import pytest
from ib_async import RequestError
from kit.fakeclock import FakeClock
from real_ib import ACCOUNT, RealIb, real_ib, sent_messages
from test_ibgateway_real_ib import _ID, _connected, _turn, _until

from optdesk.ibkr.accounts import SUMMARY_REQ_ID, drop_resubscribe, read_account
from optdesk.ibkr.pacing import MessagePacer

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


def test_drop_resubscribe_fails_loud_on_a_real_ib_without_the_handler(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    ib = real_ib().ib
    monkeypatch.delattr(type(ib), "_onError")
    with pytest.raises(RuntimeError, match="_onError"):
        drop_resubscribe(ib)


def test_drop_resubscribe_pins_the_library_handler() -> None:
    ib = real_ib().ib
    assert hasattr(ib_async.IB, "_onError")
    assert ib._onError in ib.errorEvent
    drop_resubscribe(ib)
    assert ib._onError not in ib.errorEvent


async def _read(stub: RealIb, answers: list[int | None]) -> Any:
    """Read the summary; answer the nth summary request with answers[n] (None ends it)."""
    ib = stub.ib
    ib.RaiseRequestErrors = True
    clock = FakeClock()
    task = asyncio.ensure_future(
        read_account(
            ib, MessagePacer(clock), clock, generation=1, summary=True, positions=False
        )
    )
    for n, code in enumerate(answers):
        for _ in range(200):
            if len(_summary_requests(stub)) > n or task.done():
                break
            await _turn()
        if code is None:
            ib.wrapper.accountSummaryEnd(SUMMARY_REQ_ID)
        else:
            ib.wrapper.error(SUMMARY_REQ_ID, code, "limit", "")
    for _ in range(50):
        if task.done():
            break
        await _turn()
    return task


def test_read_account_retries_once_after_322() -> None:
    async def run() -> tuple[Any, list[list[str]]]:
        stub = real_ib()
        task = await _read(stub, [322, None])
        return await task, sent_messages(stub)

    rows, sent = asyncio.run(run())
    assert rows.account == ACCOUNT
    assert [m[0] for m in sent] == [
        _ID["summary"],
        CANCEL_SUMMARY_ID,
        _ID["summary"],
        CANCEL_SUMMARY_ID,
    ]


def test_read_account_raises_after_two_322() -> None:
    async def run() -> int:
        stub = real_ib()
        task = await _read(stub, [322, 322])
        with pytest.raises(RequestError):
            await task
        return len(_summary_requests(stub))

    assert asyncio.run(run()) == 2


def test_read_account_does_not_retry_other_codes() -> None:
    async def run() -> int:
        stub = real_ib()
        task = await _read(stub, [200])
        with pytest.raises(RequestError):
            await task
        return len(_summary_requests(stub))

    assert asyncio.run(run()) == 1
