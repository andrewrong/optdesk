"""A CASH contract reaches the gateway as secType CASH for resolve and bars, over a fake gateway."""

from __future__ import annotations

import asyncio
from collections.abc import AsyncIterator, Sequence
from contextlib import asynccontextmanager
from dataclasses import dataclass, field

import grpc
from kit.sockdir import short_socket_dir

from optdesk.contracts import api as pb
from optdesk.ibkr.api import HistoryBar, HistoryRequest
from optdesk.ibkr.model import ResolveResult, SessionState
from optdesk.ibkr.transport import build_server

USD_JPY = pb.ContractSpec(
    sec_type=pb.SEC_TYPE_CASH, symbol="USD", exchange="IDEALPRO", currency="JPY"
)


@dataclass
class CashGateway:
    resolved: list[str] = field(default_factory=list)
    asked: list[HistoryRequest] = field(default_factory=list)

    async def open_session(self) -> None:
        return None

    async def status(self) -> SessionState:
        return SessionState(True, 1, 1)

    async def resolve(self, **words: str) -> ResolveResult:
        self.resolved.append(words["sec_type"])
        return ResolveResult(found=True, con_id=15016062)

    async def bars(self, request: HistoryRequest) -> Sequence[HistoryBar]:
        self.asked.append(request)
        return [HistoryBar("2026-11-10", "154.1", "154.9", "153.8", "154.5", None)]


@asynccontextmanager
async def serving(gateway: CashGateway) -> AsyncIterator[grpc.aio.Channel]:
    with short_socket_dir() as directory:
        target = f"unix:{directory / 's.sock'}"
        server = build_server(gateway)  # type: ignore[arg-type]
        server.add_insecure_port(target)
        await server.start()
        async with grpc.aio.insecure_channel(target) as channel:
            try:
                yield channel
            finally:
                await server.stop(None)


def test_resolve_sends_cash_and_answers_sec_type_cash() -> None:
    async def run() -> tuple[list[str], pb.ResolveContractResponse]:
        gateway = CashGateway()
        async with serving(gateway) as channel:
            stub = pb.ContractsServiceStub(channel)
            reply = await stub.ResolveContract(
                pb.ResolveContractRequest(contract=USD_JPY)
            )
        return gateway.resolved, reply

    words, reply = asyncio.run(run())
    assert words == ["CASH"]
    assert reply.status == pb.RESOLVE_STATUS_FOUND
    assert reply.contract.sec_type == pb.SEC_TYPE_CASH
    assert reply.contract.con_id == 15016062


def test_bars_sends_cash_with_the_midpoint_kind() -> None:
    async def run() -> tuple[list[HistoryRequest], int]:
        gateway = CashGateway()
        contract = pb.ContractSpec()
        contract.CopyFrom(USD_JPY)
        contract.con_id = 15016062
        async with serving(gateway) as channel:
            stub = pb.MarketDataServiceStub(channel)
            reply = await stub.Bars(
                pb.BarsRequest(
                    contract=contract, kind=pb.BAR_KIND_MIDPOINT, duration_days=5
                )
            )
        return gateway.asked, len(reply.bars)

    asked, count = asyncio.run(run())
    assert count == 1
    assert asked[0].key.sec_type == "CASH"
    assert asked[0].key.symbol == "USD" and asked[0].kind == "midpoint"
