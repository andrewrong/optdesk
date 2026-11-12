"""ContractsService.ListContracts over a fake ListerGateway and a real channel."""

from __future__ import annotations

import asyncio
from collections.abc import AsyncIterator, Awaitable, Sequence
from contextlib import asynccontextmanager
from dataclasses import dataclass, field
from typing import Any

import grpc
from kit.sockdir import short_socket_dir

from optdesk.contracts import api as pb
from optdesk.ibkr import spec
from optdesk.ibkr.model import ListedContract, SessionState
from optdesk.ibkr.transport import build_server


@dataclass
class ListerFake:
    rows: list[ListedContract] = field(default_factory=list)
    error: BaseException | None = None
    seen: list[dict[str, str]] = field(default_factory=list)

    async def open_session(self) -> None:
        return None

    async def status(self) -> SessionState:
        return SessionState(True, 1, 1)

    async def list_contracts(self, **asked: str) -> Sequence[ListedContract]:
        self.seen.append(asked)
        if self.error is not None:
            raise self.error
        return self.rows


class PlainFake:
    """A Gateway without list_contracts."""

    async def open_session(self) -> None:
        return None

    async def status(self) -> SessionState:
        return SessionState(True, 1, 1)


@asynccontextmanager
async def serving(gateway: Any) -> AsyncIterator[grpc.aio.Channel]:
    with short_socket_dir() as directory:
        target = f"unix:{directory / 's.sock'}"
        server = build_server(gateway)
        server.add_insecure_port(target)
        await server.start()
        async with grpc.aio.insecure_channel(target) as channel:
            try:
                yield channel
            finally:
                await server.stop(None)


def request(**fields: Any) -> pb.ListContractsRequest:
    base: dict[str, Any] = {
        "underlying": pb.ContractSpec(sec_type=pb.SEC_TYPE_STOCK, symbol="SPY"),
        "expiry": "20260116",
        "trading_class": "SPY",
    }
    return pb.ListContractsRequest(**{**base, **fields})


async def code_of(call: Awaitable[object]) -> Any:
    try:
        await call
    except grpc.aio.AioRpcError as exc:
        return exc.code()
    raise AssertionError("the call succeeded")


def listed(con_id: int, right: str, strike: str) -> ListedContract:
    return ListedContract(
        con_id, "SPY", "SMART", "USD", "20260116", strike, right, "100", "SPY"
    )


def test_both_rights_are_listed_with_their_con_ids() -> None:
    async def run() -> tuple[pb.ListContractsResponse, ListerFake]:
        fake = ListerFake(rows=[listed(11, "C", "600"), listed(12, "P", "12.5")])
        async with serving(fake) as channel:
            stub = pb.ContractsServiceStub(channel)
            return await stub.ListContracts(request()), fake

    got, fake = asyncio.run(run())
    assert [(c.con_id, c.right, c.strike) for c in got.contracts] == [
        (11, pb.RIGHT_CALL, "600"),
        (12, pb.RIGHT_PUT, "12.5"),
    ]
    assert all(c.sec_type == pb.SEC_TYPE_OPTION for c in got.contracts)
    assert fake.seen[0]["right"] == "" and fake.seen[0]["trading_class"] == "SPY"


def test_a_right_is_passed_as_one_letter() -> None:
    async def run() -> ListerFake:
        fake = ListerFake()
        async with serving(fake) as channel:
            stub = pb.ContractsServiceStub(channel)
            await stub.ListContracts(request(right=pb.RIGHT_PUT))
        return fake

    assert asyncio.run(run()).seen[0]["right"] == "P"


def test_a_gateway_without_the_method_is_unimplemented() -> None:
    async def run() -> Any:
        async with serving(PlainFake()) as channel:
            stub = pb.ContractsServiceStub(channel)
            return await code_of(stub.ListContracts(request()))

    assert asyncio.run(run()) == grpc.StatusCode.UNIMPLEMENTED


def test_a_bad_request_is_invalid_argument() -> None:
    async def run() -> list[Any]:
        async with serving(ListerFake()) as channel:
            stub = pb.ContractsServiceStub(channel)
            return [
                await code_of(stub.ListContracts(request(expiry="2026"))),
                await code_of(stub.ListContracts(request(trading_class=""))),
                await code_of(
                    stub.ListContracts(request(underlying=pb.ContractSpec()))
                ),
            ]

    assert asyncio.run(run()) == [grpc.StatusCode.INVALID_ARGUMENT] * 3


def test_timeout_and_disconnect_are_errors_not_partial_lists() -> None:
    async def run() -> list[Any]:
        fake = ListerFake(rows=[listed(1, "C", "1")], error=TimeoutError())
        async with serving(fake) as channel:
            stub = pb.ContractsServiceStub(channel)
            first = await code_of(stub.ListContracts(request()))
            fake.error = ConnectionError("lost")
            second = await code_of(stub.ListContracts(request()))
            fake.error = spec.BrokerError(162, "pacing")
            return [first, second, await code_of(stub.ListContracts(request()))]

    assert asyncio.run(run()) == [
        grpc.StatusCode.DEADLINE_EXCEEDED,
        grpc.StatusCode.UNAVAILABLE,
        grpc.StatusCode.UNKNOWN,
    ]
