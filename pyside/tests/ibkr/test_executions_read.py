"""The executions_read counts of read_executions and the line IbGateway.executions writes."""

from __future__ import annotations

import asyncio
import logging
from collections.abc import Iterator

import pytest
from ib_async import CommissionReport
from kit.fakeclock import FakeClock
from test_executions import acct, done, make, other_acct, probe_passes, settle

from optdesk.ibkr.accounts import DEADLINE_S, read_executions
from optdesk.ibkr.ibgateway import IbGateway
from optdesk.ibkr.model import ExecutionRows, MultipleAccounts
from optdesk.ibkr.pacing import MessagePacer
from optdesk.ibkr.transport import configure_logging


def read_counted(
    fills: list[tuple[str, str, bool]], *, late: bool = False
) -> tuple[ExecutionRows, list[tuple[int, int, int, int]]]:
    async def run() -> tuple[ExecutionRows, list[tuple[int, int, int, int]]]:
        clock = FakeClock()
        ib, conn = make([acct()])
        conn.fills = fills
        seen: list[tuple[int, int, int, int]] = []
        task = asyncio.ensure_future(
            read_executions(
                ib,
                MessagePacer(clock),
                clock,
                generation=4,
                on_counts=lambda *c: seen.append(c),
            )
        )
        await settle()
        if late:
            ib.wrapper.commissionReport(
                CommissionReport(execId="e2", commission=0.5, currency="USD")
            )
            await clock.advance(1)
        elif not task.done():
            await clock.advance(DEADLINE_S + 1)
        rows: ExecutionRows = await task
        done(ib)
        return rows, seen

    return asyncio.run(run())


def test_read_counts_each_drop_apart() -> None:
    rows, seen = read_counted(
        [
            ("e1", acct(), True),
            ("e2", acct(), True),
            ("e3", acct(), False),
            ("e4", other_acct(), True),
            ("e5", other_acct(), True),
        ]
    )
    assert rows.pending == 1 and [e.exec_id for e in rows.executions] == ["e1", "e2"]
    assert seen == [(5, 2, 1, 2)]


def test_read_counts_no_fills() -> None:
    rows, seen = read_counted([])
    assert rows.executions == () and seen == [(0, 0, 0, 0)]


def test_read_counts_late_report_is_stored() -> None:
    rows, seen = read_counted([("e2", acct(), False)], late=True)
    assert [e.exec_id for e in rows.executions] == ["e2"]
    assert seen == [(1, 0, 0, 1)]


@pytest.fixture
def production_logging() -> Iterator[None]:
    """configure_logging() for one test; the factory and the levels come back after.

    The root handlers are left alone: configure_logging adds none when pytest has
    already put its capture handlers there.
    """
    root, library = logging.getLogger(), logging.getLogger("ib_async")
    factory = logging.getLogRecordFactory()
    root_level, library_level = root.level, library.level
    library_propagate, library_handlers = library.propagate, list(library.handlers)
    configure_logging()
    yield
    logging.setLogRecordFactory(factory)
    root.setLevel(root_level)
    library.setLevel(library_level)
    library.propagate = library_propagate
    library.handlers[:] = library_handlers


@pytest.mark.usefixtures("production_logging")
def test_gateway_executions_logs_one_line_without_account(
    caplog: pytest.LogCaptureFixture,
) -> None:
    async def run() -> None:
        ib, conn = make([acct()])
        conn.fills = [("e1", acct(), True), ("e2", other_acct(), True)]

        async def connect(host: str, port: int, client_id: int) -> None:
            return None

        ib.client.connectAsync = connect
        ib.reqMarketDataType = lambda value: None
        gateway = IbGateway(
            lambda client: None,
            host="h",
            port=1,
            client_id=1,
            clock=FakeClock(),
            ib_factory=lambda: ib,
            readonly_probe=probe_passes,
        )
        await gateway.open_session()
        await gateway.executions()
        done(ib)

    with caplog.at_level(logging.DEBUG):
        asyncio.run(run())
    ours = caplog.records  # every record captured, from any logger
    expected = (
        "executions_read raw=2 dropped_account=1 dropped_commission=0 stored=1 alias="
    )
    lines = [r for r in ours if r.getMessage().startswith("executions_read")]
    assert [(r.levelno, r.getMessage()) for r in lines] == [(logging.INFO, expected)]
    text = " ".join(r.getMessage() for r in ours)
    assert acct() not in text and other_acct() not in text


def test_failed_read_writes_no_executions_read() -> None:
    async def run() -> list[tuple[int, int, int, int]]:
        clock = FakeClock()
        seen: list[tuple[int, int, int, int]] = []
        ib, conn = make([acct()])
        conn.hold = True
        task = asyncio.ensure_future(
            read_executions(
                ib, MessagePacer(clock), clock, generation=4,
                on_counts=lambda *c: seen.append(c),
            )
        )  # fmt: skip
        await settle()
        await clock.advance(DEADLINE_S + 1)
        with pytest.raises(TimeoutError):
            await task
        two, _ = make([acct(), other_acct()])
        with pytest.raises(MultipleAccounts):
            await read_executions(
                two, MessagePacer(clock), clock, generation=4,
                on_counts=lambda *c: seen.append(c),
            )  # fmt: skip
        done(ib)
        done(two)
        return seen

    assert asyncio.run(run()) == []
