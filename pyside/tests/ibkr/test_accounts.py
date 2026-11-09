"""The account read, the Account rpc and the log safety of the account data."""

from __future__ import annotations

import asyncio
import logging
from collections.abc import AsyncIterator, Awaitable, Iterator
from contextlib import asynccontextmanager
from typing import Any, Self, cast

import grpc
import pytest
from ib_async import IB, Client, Contract
from kit.fakeclock import FakeClock
from kit.sockdir import short_socket_dir

from optdesk.contracts import api as pb
from optdesk.ibkr import transport
from optdesk.ibkr.accounts import DEADLINE_S, TAGS, read_account
from optdesk.ibkr.gateways import GatewaySpec
from optdesk.ibkr.ibgateway import IbGateway
from optdesk.ibkr.model import (
    AccountGateway,
    AccountRows,
    AccountValueRow,
    ExecutionRows,
    Gateway,
    MultipleAccounts,
    PositionRow,
)
from optdesk.ibkr.pacing import MessagePacer
from optdesk.ibkr.transport import (
    FIELDS,
    INTERNAL_TEXT,
    ORDER_IDS,
    build_server,
    configure_logging,
    install_guard,
)


def shape() -> str:
    # built from two pieces so no file of the repository carries the literal
    return "DU" + "7654321"


def other_shape() -> str:
    return "DU" + "1112223"


async def settle() -> None:
    for _ in range(20):
        await asyncio.sleep(0)


class QuietEvent:
    def __iadd__(self, handler: object) -> Self:
        return self

    def __isub__(self, handler: object) -> Self:
        return self


class FakeClient:
    """Records the account calls and answers them through the real wrapper."""

    def __init__(self, ib: Any) -> None:
        self._ib = ib
        self.apiEnd = QuietEvent()
        self.calls: list[tuple[Any, ...]] = []
        self.summary: list[tuple[str, str, str, str]] = []
        self.positions: list[tuple[str, str, float, float]] = []
        self.hold = False
        self.fail_positions = False

    async def connectAsync(self, host: str, port: int, client_id: int) -> None:
        return None

    def isConnected(self) -> bool:
        return False

    def reqMarketDataType(self, value: int) -> None:
        return None

    def reqAccountSummary(self, req_id: int, group: str, tags: str) -> None:
        self.calls.append(("summary", req_id, group, tags))
        wrapper = self._ib.wrapper
        for account, tag, value, currency in self.summary:
            wrapper.accountSummary(req_id, account, tag, value, currency)
        if not self.hold:
            wrapper.accountSummaryEnd(req_id)

    def reqPositions(self) -> None:
        self.calls.append(("positions",))
        if self.fail_positions:
            raise RuntimeError("request failed")
        wrapper = self._ib.wrapper
        for account, symbol, size, cost in self.positions:
            contract = Contract(secType="STK", symbol=symbol, conId=7, currency="USD")
            wrapper.position(account, contract, size, cost)
        if not self.hold:
            wrapper.positionEnd()

    def cancelAccountSummary(self, req_id: int) -> None:
        self.calls.append(("cancel_summary", req_id))

    def cancelPositions(self) -> None:
        self.calls.append(("cancel_positions",))


def make_ib(accounts: list[str]) -> tuple[Any, FakeClient]:
    ib: Any = IB()
    fake = FakeClient(ib)
    ib.client = fake
    ib.wrapper.accounts = list(accounts)
    return ib, fake


def names(fake: FakeClient) -> list[str]:
    return [str(call[0]) for call in fake.calls]


def read(ib: Any, clock: FakeClock, **kw: Any) -> Awaitable[AccountRows]:
    return read_account(
        ib,
        MessagePacer(clock),
        clock,
        generation=1,
        summary=kw.get("summary", True),
        positions=kw.get("positions", True),
    )


def test_account_cancels_after_failure() -> None:
    async def run() -> list[str]:
        ib, fake = make_ib([shape()])
        fake.fail_positions = True
        with pytest.raises(RuntimeError):
            await read(ib, FakeClock())
        await settle()
        return names(fake)

    calls = asyncio.run(run())
    assert "cancel_summary" in calls and "cancel_positions" in calls


def test_account_cancel_after_timeout() -> None:
    async def run() -> list[str]:
        clock = FakeClock()
        ib, fake = make_ib([shape()])
        fake.hold = True
        task = asyncio.ensure_future(read(ib, clock))
        await settle()
        await clock.advance(DEADLINE_S + 1)
        with pytest.raises(TimeoutError):
            await task
        await settle()
        return names(fake)

    calls = asyncio.run(run())
    assert "cancel_summary" in calls and "cancel_positions" in calls


def test_summary_frame_carries_exactly_the_ten_tags() -> None:
    async def run() -> tuple[Any, ...]:
        ib, fake = make_ib([shape()])
        await read(ib, FakeClock(), positions=False)
        (call,) = [c for c in fake.calls if c[0] == "summary"]
        return call

    _, _, group, tags = asyncio.run(run())
    assert group == "All"
    assert tags.split(",") == list(TAGS) and len(TAGS) == 10
    assert len(set(TAGS)) == 10 and "$LEDGER:ALL" not in tags


def test_quantity_is_decimal_text() -> None:
    async def run() -> AccountRows:
        ib, fake = make_ib([shape()])
        fake.positions = [
            (shape(), "AAA", 10000000.0, 0.00000012),
            (shape(), "BBB", 0.5, 1234.5),
            (shape(), "CCC", -3.0, 1e21),
        ]
        fake.summary = [(shape(), "NetLiquidation", "1E+7", "USD")]
        return await read(ib, FakeClock())

    rows = asyncio.run(run())
    quantities = [(p.quantity, p.avg_cost) for p in rows.positions]
    assert quantities == [
        ("10000000", "0.00000012"),
        ("0.5", "1234.5"),
        ("-3", "1000000000000000000000"),
    ]
    assert [v.value for v in rows.values] == ["10000000"]
    assert not any("e" in v.lower() for pair in quantities for v in pair)


def test_second_positions_call_drops_closed_position() -> None:
    async def run() -> tuple[list[str], list[str]]:
        ib, fake = make_ib([shape()])
        fake.positions = [(shape(), "AAA", 1.0, 2.0), (shape(), "BBB", 1.0, 2.0)]
        first = await read(ib, FakeClock(), summary=False)
        fake.positions = [(shape(), "AAA", 1.0, 2.0), (shape(), "BBB", 0.0, 2.0)]
        second = await read(ib, FakeClock(), summary=False)
        return [p.symbol for p in first.positions], [p.symbol for p in second.positions]

    first, second = asyncio.run(run())
    assert first == ["AAA", "BBB"] and second == ["AAA"]


# --- the real client ---------------------------------------------------------


class SniffingConn:
    """A stub connection: records frames and answers the account requests."""

    def __init__(self, ib: Any) -> None:
        self.ib = ib
        self.frames: list[bytes] = []
        self.numBytesSent = 0
        self.numMsgSent = 0

    async def connectAsync(self, host: str, port: int) -> None:
        return None

    def disconnect(self) -> None:
        return None

    def sendMsg(self, msg: bytes) -> None:
        self.frames.append(msg)
        fields = msg[4:].split(b"\0")
        wrapper = self.ib.wrapper
        if fields[0] == b"62":
            req_id = int(fields[2])
            wrapper.accountSummary(req_id, shape(), "Cushion", "0.25", "USD")
            wrapper.accountSummaryEnd(req_id)
        elif fields[0] == b"61":
            wrapper.position(shape(), Contract(conId=7, symbol="SPY"), 2.0, 3.0)
            wrapper.positionEnd()

    def ids(self) -> list[int]:
        return [int(f[4:].split(b"\0")[0]) for f in self.frames]


def test_real_client_sends_only_account_ids() -> None:
    async def run() -> tuple[SniffingConn, AccountRows]:
        clock = FakeClock()
        ib: Any = IB()
        conn = SniffingConn(ib)
        client: Any = ib.client
        client.conn = conn
        client.connState = Client.CONNECTED
        install_guard(client)
        ib.wrapper.accounts = [shape()]
        rows = await read(ib, clock)
        await settle()
        client.connState = Client.DISCONNECTED  # so IB.__del__ has nothing to close
        return conn, rows

    conn, rows = asyncio.run(run())
    ids = conn.ids()
    assert set(ids) == {61, 62, 63, 64}
    assert not set(ids) & ORDER_IDS
    assert ids.count(63) == 1 and ids.count(64) == 1
    (summary,) = [f for f in conn.frames if f[4:].startswith(b"62\0")]
    assert summary[4:].split(b"\0")[4].decode().split(",") == list(TAGS)
    assert [v.tag for v in rows.values] == ["Cushion"] and rows.positions[
        0
    ].symbol == "SPY"


# --- the gateway and the rpc -------------------------------------------------


class Rig:
    """An IbGateway over a fake client, served through build_server."""

    def __init__(self, accounts: list[str]) -> None:
        self.ib, self.fake = make_ib(accounts)
        self.gateway = IbGateway(
            lambda client: None,
            host="h",
            port=1,
            client_id=1,
            clock=FakeClock(),
            ib_factory=lambda: self.ib,
        )


def account_spec(*collect: str) -> GatewaySpec:
    return GatewaySpec(name="b", port=2, account="alias_two", collect=collect)


@asynccontextmanager
async def serving(gateway: object, *collect: str) -> AsyncIterator[grpc.aio.Channel]:
    with short_socket_dir() as directory:
        target = f"unix:{directory / 's.sock'}"
        server = build_server(
            cast(Gateway, gateway), FakeClock(), spec=account_spec(*collect)
        )
        server.add_insecure_port(target)
        await server.start()
        async with grpc.aio.insecure_channel(target) as channel:
            try:
                yield channel
            finally:
                await server.stop(None)


class Capture(logging.Handler):
    def __init__(self) -> None:
        super().__init__(logging.DEBUG)
        self.messages: list[str] = []

    def emit(self, record: logging.LogRecord) -> None:
        self.messages.append(self.format(record))


@pytest.fixture
def capture() -> Iterator[Capture]:
    configure_logging()
    handler = Capture()
    root = logging.getLogger()
    root.addHandler(handler)
    before = root.level
    root.setLevel(logging.DEBUG)
    yield handler
    root.setLevel(before)
    root.removeHandler(handler)


async def code_of(call: Awaitable[object]) -> grpc.aio.AioRpcError:
    try:
        await call
    except grpc.aio.AioRpcError as exc:
        return exc
    raise AssertionError("the call succeeded")


def test_two_accounts_refused(capture: Capture) -> None:
    async def run() -> tuple[grpc.aio.AioRpcError, bool]:
        rig = Rig([shape(), other_shape()])
        await rig.gateway.open_session()
        rig.fake.summary = [(shape(), "Cushion", "1", "USD")]
        async with serving(rig.gateway, "account_summary", "positions") as channel:
            error = await code_of(
                pb.AccountServiceStub(channel).Account(pb.AccountRequest())
            )
        return error, bool(rig.fake.calls)

    error, sent = asyncio.run(run())
    assert error.code() == grpc.StatusCode.FAILED_PRECONDITION
    assert not sent  # nothing was requested
    joined = "\n".join(capture.messages)
    assert "multiple_accounts" in joined
    assert shape() not in joined and other_shape() not in joined
    (failed,) = [m for m in capture.messages if "rpc_failed" in m]
    assert "rpc=account" in failed and "status=multiple_accounts" in failed


def test_alias_replaces_id() -> None:
    async def run() -> pb.AccountResponse:
        rig = Rig([shape()])
        await rig.gateway.open_session()
        rig.fake.summary = [
            (shape(), "Cushion", "0.25", "USD"),
            (shape(), "$LEDGER:ALL", "9", "USD"),  # not a fixed tag
            (other_shape(), "NetLiquidation", "5", "USD"),  # another account
        ]
        rig.fake.positions = [
            (shape(), "SPY", 2.0, 3.5),
            (other_shape(), "QQQ", 1.0, 1.0),
        ]
        async with serving(rig.gateway, "account_summary", "positions") as channel:
            return await pb.AccountServiceStub(channel).Account(pb.AccountRequest())

    response = asyncio.run(run())
    assert response.account == "alias_two" and response.generation == 1
    assert [(v.tag, v.value, v.currency) for v in response.values] == [
        ("Cushion", "0.25", "USD")
    ]
    assert [
        (p.contract.symbol, p.quantity, p.avg_cost) for p in response.positions
    ] == [("SPY", "2", "3.5")]
    wire = response.SerializeToString()
    assert shape().encode() not in wire and other_shape().encode() not in wire


class FakeAccounts:
    def __init__(self, rows: AccountRows | None = None, error: Exception | None = None):
        self.rows = rows or AccountRows(3)
        self.error = error
        self.asked: list[tuple[bool, bool]] = []

    async def account(self, *, summary: bool, positions: bool) -> AccountRows:
        self.asked.append((summary, positions))
        if self.error is not None:
            raise self.error
        return self.rows

    async def executions(self) -> ExecutionRows:
        return ExecutionRows(self.rows.generation)

    async def status(self) -> Any:
        return None


def test_account_unimplemented_without_words() -> None:
    async def run() -> grpc.StatusCode:
        async with serving(FakeAccounts(), "market_data") as channel:
            return (
                await code_of(
                    pb.AccountServiceStub(channel).Account(pb.AccountRequest())
                )
            ).code()

    assert asyncio.run(run()) == grpc.StatusCode.UNIMPLEMENTED


def test_positions_empty_when_not_collected() -> None:
    rows = AccountRows(
        3,
        (AccountValueRow("Cushion", "1", "USD"),),
        (PositionRow("STK", "SPY", 7, "", "USD", "", "", "", "", "", "1", "2"),),
    )

    async def run() -> tuple[pb.AccountResponse, list[tuple[bool, bool]]]:
        fake = FakeAccounts(rows)
        async with serving(fake, "account_summary") as channel:
            return await pb.AccountServiceStub(channel).Account(
                pb.AccountRequest()
            ), fake.asked

    response, asked = asyncio.run(run())
    assert asked == [(True, False)]
    assert len(response.values) == 1 and len(response.positions) == 0


def test_account_log_has_no_values(capture: Capture) -> None:
    rows = AccountRows(
        3,
        (AccountValueRow("NetLiquidation", "98765.4", "USD"),),
        (PositionRow("STK", "SPYZ", 7, "", "USD", "", "", "", "", "", "11", "22"),),
    )

    async def run() -> None:
        async with serving(
            FakeAccounts(rows), "account_summary", "positions"
        ) as channel:
            await pb.AccountServiceStub(channel).Account(pb.AccountRequest())

    asyncio.run(run())
    lines = [m for m in capture.messages if "rpc_" in m]
    assert lines
    for line in lines:
        for item in line.split()[3:]:
            key, _, value = item.partition("=")
            assert key in FIELDS
            assert value in transport.WORDS or value.lstrip("-").isdigit()
    joined = "\n".join(capture.messages)
    for secret in ("NetLiquidation", "98765.4", "SPYZ", "alias_two", "USD"):
        assert secret not in joined
    assert any("rpc=account" in m and "count=2" in m for m in lines)


def test_account_unexpected_error_is_internal(capture: Capture) -> None:
    secret = f"boom {shape()}"

    async def run() -> grpc.aio.AioRpcError:
        fake = FakeAccounts(error=ValueError(secret))
        async with serving(fake, "account_summary") as channel:
            return await code_of(
                pb.AccountServiceStub(channel).Account(pb.AccountRequest())
            )

    error = asyncio.run(run())
    assert error.code() == grpc.StatusCode.INTERNAL
    assert error.details() == INTERNAL_TEXT
    everything = (error.details() or "") + "\n" + "\n".join(capture.messages)
    assert "boom" not in everything and shape() not in everything


def test_reads_are_serialized() -> None:
    async def run() -> tuple[int, int]:
        rig = Rig([shape()])
        await rig.gateway.open_session()
        rig.fake.hold = True
        first = asyncio.ensure_future(
            rig.gateway.account(summary=False, positions=True)
        )
        second = asyncio.ensure_future(
            rig.gateway.account(summary=False, positions=True)
        )
        await settle()
        sent_while_held = names(rig.fake).count("positions")
        rig.fake.hold = False
        rig.ib.wrapper.positionEnd()
        await settle()
        await asyncio.gather(first, second)
        return sent_while_held, names(rig.fake).count("positions")

    held, total = asyncio.run(run())
    assert held == 1 and total == 2


def test_multiple_accounts_from_the_gateway() -> None:
    async def run() -> None:
        rig = Rig([])
        await rig.gateway.open_session()
        with pytest.raises(MultipleAccounts):
            await rig.gateway.account(summary=True, positions=True)

    asyncio.run(run())


def test_ib_async_position_log_cannot_carry_an_id(capture: Capture) -> None:
    ib: Any = IB()
    library = logging.getLogger("ib_async")
    assert library.level > logging.INFO
    contract = Contract(conId=1, symbol="SPY", secType="STK")
    ib.wrapper.position(shape(), contract, 1.0, 2.0)
    assert not [m for m in capture.messages if "position" in m]
    wrapper_log = logging.getLogger("ib_async.wrapper")
    before = wrapper_log.level
    wrapper_log.setLevel(logging.INFO)  # as if the level were lowered
    try:
        ib.wrapper.position(shape(), contract, 1.0, 2.0)
    finally:
        wrapper_log.setLevel(before)
    assert [m for m in capture.messages if "position" in m]
    assert all(shape() not in m for m in capture.messages)


def test_fake_satisfies_the_account_protocol() -> None:
    gateway: AccountGateway = FakeAccounts()
    assert asyncio.run(gateway.account(summary=True, positions=True)).generation == 3
