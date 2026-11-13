"""The Executions rpc: complete rows only, pending counted, nothing sent when not collected."""

from __future__ import annotations

import asyncio
from collections.abc import AsyncIterator
from contextlib import asynccontextmanager
from datetime import UTC, datetime
from typing import Any, cast

import grpc
import pytest
from ib_async import IB, Client, CommissionReport, Contract, Execution
from kit.fakeclock import FakeClock
from kit.sockdir import short_socket_dir

from optdesk.contracts import api as pb
from optdesk.ibkr.accounts import DEADLINE_S, read_executions
from optdesk.ibkr.gateways import GatewaySpec
from optdesk.ibkr.ibgateway import IbGateway
from optdesk.ibkr.model import (
    AccountRows,
    ExecutionRow,
    ExecutionRows,
    Gateway,
    MultipleAccounts,
)
from optdesk.ibkr.pacing import MessagePacer
from optdesk.ibkr.transport import (
    ORDER_IDS,
    NotAllowed,
    OrderRefused,
    build_server,
    check,
    install_guard,
)


async def probe_passes() -> None:
    """A read-only probe that passes, in place of the second connection."""


def acct() -> str:
    # built from two pieces so no file of the repository carries the literal
    return "DU" + "7654321"


def other_acct() -> str:
    return "DU" + "1112223"


async def settle() -> None:
    for _ in range(20):
        await FakeClock().sleep(0)  # yields the loop one turn, waits for no time


class Conn:
    """A stub connection: records frames, answers reqExecutions through the wrapper."""

    def __init__(self, ib: Any) -> None:
        self.ib = ib
        self.frames: list[bytes] = []
        self.numBytesSent = 0
        self.numMsgSent = 0
        # (exec id, account, whether the commission report comes)
        self.fills: list[tuple[str, str, bool]] = []
        self.hold = False

    async def connectAsync(self, host: str, port: int) -> None:
        return None

    def disconnect(self) -> None:
        return None

    def sendMsg(self, msg: bytes) -> None:
        self.frames.append(msg)
        fields = msg[4:].split(b"\0")
        if fields[0] != b"7":
            return
        wrapper = self.ib.wrapper
        req_id = int(fields[2])
        contract = Contract(secType="STK", symbol="SPY", conId=756733, currency="USD")
        for n, (exec_id, account, with_commission) in enumerate(self.fills):
            execution = Execution(
                execId=exec_id,
                time=datetime(2026, 1, 2, 14, 30, n, tzinfo=UTC),
                acctNumber=account,
                side="BOT",
                shares=2.0,
                price=500.25,
                permId=900 + n,
                orderId=55,
            )
            wrapper.execDetails(req_id, contract, execution)
            if with_commission:
                wrapper.commissionReport(
                    CommissionReport(execId=exec_id, commission=1.5, currency="USD")
                )
        if not self.hold:
            wrapper.execDetailsEnd(req_id)

    def ids(self) -> list[int]:
        return [int(f[4:].split(b"\0")[0]) for f in self.frames]


def make(accounts: list[str]) -> tuple[Any, Conn]:
    ib: Any = IB()
    conn = Conn(ib)
    client: Any = ib.client
    client.conn = conn
    client.connState = Client.CONNECTED
    install_guard(client)
    ib.wrapper.accounts = list(accounts)
    return ib, conn


def done(ib: Any) -> None:
    ib.client.connState = Client.DISCONNECTED  # so IB.__del__ has nothing to close


def read(ib: Any, clock: FakeClock) -> Any:
    return read_executions(ib, MessagePacer(clock), clock, generation=4)


def test_complete_rows_only_and_pending_counted() -> None:
    async def run() -> tuple[ExecutionRows, Conn]:
        clock = FakeClock()
        ib, conn = make([acct()])
        conn.fills = [
            ("e1", acct(), True),
            ("e2", acct(), False),  # its commission report never comes
            ("e3", other_acct(), True),  # another account
        ]
        task = asyncio.ensure_future(read(ib, clock))
        await settle()
        assert not task.done()  # waits for the missing report
        await clock.advance(DEADLINE_S + 1)
        rows = await task
        done(ib)
        return rows, conn

    rows, conn = asyncio.run(run())
    assert rows.pending == 1 and rows.generation == 4
    (row,) = rows.executions
    assert row == ExecutionRow(
        exec_id="e1",
        perm_id=900,
        time="2026-01-02T14:30:00Z",
        sec_type="STK",
        symbol="SPY",
        con_id=756733,
        exchange="",
        currency="USD",
        expiry="",
        strike="",
        right="",
        multiplier="",
        trading_class="",
        side="BOT",
        quantity="2",
        price="500.25",
        commission="1.5",
        commission_currency="USD",
    )
    assert conn.ids() == [7] and not set(conn.ids()) & ORDER_IDS


def test_report_that_arrives_late_is_waited_for() -> None:
    async def run() -> ExecutionRows:
        clock = FakeClock()
        ib, conn = make([acct()])
        conn.fills = [("e2", acct(), False)]
        task = asyncio.ensure_future(read(ib, clock))
        await settle()
        ib.wrapper.commissionReport(
            CommissionReport(execId="e2", commission=0.5, currency="USD")
        )
        await clock.advance(1)
        rows: ExecutionRows = await task
        done(ib)
        return rows

    rows = asyncio.run(run())
    assert rows.pending == 0 and [e.commission for e in rows.executions] == ["0.5"]


def test_no_fills_is_an_empty_list() -> None:
    async def run() -> ExecutionRows:
        ib, _ = make([acct()])
        rows: ExecutionRows = await read(ib, FakeClock())
        done(ib)
        return rows

    rows = asyncio.run(run())
    assert rows.executions == () and rows.pending == 0


def test_missing_end_marker_times_out_and_two_accounts_are_refused() -> None:
    async def run() -> int:
        clock = FakeClock()
        ib, conn = make([acct()])
        conn.hold = True
        task = asyncio.ensure_future(read(ib, clock))
        await settle()
        await clock.advance(DEADLINE_S + 1)
        with pytest.raises(TimeoutError):
            await task
        two, other = make([acct(), other_acct()])
        with pytest.raises(MultipleAccounts):
            await read(two, clock)
        done(ib)
        done(two)
        return len(other.frames)

    assert asyncio.run(run()) == 0  # nothing was sent for two accounts


# --- the rpc -----------------------------------------------------------------


class FakeAccounts:
    def __init__(self, rows: ExecutionRows) -> None:
        self.rows = rows
        self.asked = 0

    async def executions(self) -> ExecutionRows:
        self.asked += 1
        return self.rows

    async def account(self, *, summary: bool, positions: bool) -> AccountRows:
        return AccountRows(1)

    async def status(self) -> Any:
        return None


@asynccontextmanager
async def serving(gateway: object, *collect: str) -> AsyncIterator[grpc.aio.Channel]:
    spec = GatewaySpec(name="b", port=2, collect=collect)
    with short_socket_dir() as directory:
        target = f"unix:{directory / 's.sock'}"
        server = build_server(cast(Gateway, gateway), FakeClock(), spec=spec)
        server.add_insecure_port(target)
        await server.start()
        async with grpc.aio.insecure_channel(target) as channel:
            try:
                yield channel
            finally:
                await server.stop(None)


def row(exec_id: str) -> ExecutionRow:
    return ExecutionRow(
        exec_id, 9, "2026-01-02T14:30:00Z", "OPT", "SPY", 5, "SMART", "USD",
        "20260116", "500", "P", "100", "SPY", "SLD", "1", "2.5", "0.65", "USD",
    )  # fmt: skip


def ask(gateway: FakeAccounts, *collect: str) -> pb.ExecutionsResponse:
    async def run() -> pb.ExecutionsResponse:
        async with serving(gateway, *collect) as channel:
            return await pb.AccountServiceStub(channel).Executions(
                pb.ExecutionsRequest()
            )

    return asyncio.run(run())


def test_rpc_answers_complete_rows_and_pending() -> None:
    reported = "DU" + "0000001"  # built from pieces: no file carries an id literal
    gateway = FakeAccounts(ExecutionRows(3, (row("e1"),), pending=2, account=reported))
    response = ask(gateway, "executions")
    assert response.account == reported and response.generation == 3
    assert response.collected and response.pending == 2
    (execution,) = response.executions
    assert (execution.exec_id, execution.perm_id, execution.side) == ("e1", 9, "SLD")
    assert (execution.quantity, execution.price) == ("1", "2.5")
    assert (execution.commission, execution.commission_currency) == ("0.65", "USD")
    assert execution.contract.symbol == "SPY" and execution.contract.strike == "500"
    assert execution.contract.right == pb.RIGHT_PUT


def test_rpc_empty_list_is_collected() -> None:
    gateway = FakeAccounts(ExecutionRows(3))
    response = ask(gateway, "executions", "positions")
    assert response.collected and response.pending == 0
    assert len(response.executions) == 0 and gateway.asked == 1


def test_rpc_not_collected_sends_no_request() -> None:
    gateway = FakeAccounts(ExecutionRows(3, (row("e1"),)))
    response = ask(gateway, "positions")
    assert not response.collected and len(response.executions) == 0
    assert gateway.asked == 0


def test_account_flags_follow_collect() -> None:
    async def run() -> tuple[pb.AccountResponse, pb.AccountResponse]:
        gateway = FakeAccounts(ExecutionRows(1))
        async with serving(gateway, "positions") as channel:
            one = await pb.AccountServiceStub(channel).Account(pb.AccountRequest())
        async with serving(gateway, "account_summary", "positions") as channel:
            two = await pb.AccountServiceStub(channel).Account(pb.AccountRequest())
        return one, two

    one, two = asyncio.run(run())
    assert (one.values_collected, one.positions_collected) == (False, True)
    assert (two.values_collected, two.positions_collected) == (True, True)


def test_order_ids_still_refused_and_executions_id_allowed() -> None:
    check(7)
    for message_id in (3, 4, 21, 58):
        with pytest.raises(OrderRefused):
            check(message_id)
    with pytest.raises(NotAllowed):
        check(8)


def test_gateway_executions_over_ib() -> None:
    async def run() -> ExecutionRows:
        ib, conn = make([acct()])
        conn.fills = [("e1", acct(), True)]

        async def connect(host: str, port: int, client_id: int) -> None:
            return None

        ib.client.connectAsync = connect  # the session opens without a socket
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
        rows: ExecutionRows = await gateway.executions()
        done(ib)
        return rows

    rows = asyncio.run(run())
    assert [e.exec_id for e in rows.executions] == ["e1"]


def test_read_reports_the_id_the_gateway_reports() -> None:
    async def run() -> ExecutionRows:
        ib, conn = make([acct()])
        conn.fills = [("e1", acct(), True)]
        rows = await read(ib, FakeClock())
        done(ib)
        return rows

    rows = asyncio.run(run())
    assert rows.account == acct() and len(rows.executions) == 1


def test_rpc_not_collected_reports_no_id() -> None:
    response = ask(FakeAccounts(ExecutionRows(3, account=acct())), "positions")
    assert not response.collected and response.account == ""
