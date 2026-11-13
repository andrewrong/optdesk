"""Snapshot accepts the quote kind OPTION_OPEN; an unknown kind stays INVALID_ARGUMENT."""

from __future__ import annotations

import asyncio
from collections.abc import AsyncIterator, Awaitable, Callable, Sequence
from contextlib import asynccontextmanager
from dataclasses import dataclass, field

import grpc
from kit.sockdir import short_socket_dir

from optdesk.contracts import api as pb
from optdesk.ibkr import spec, transport
from optdesk.ibkr.model import QuoteRow, QuoteTarget, SessionState
from optdesk.ibkr.transport import build_server


@dataclass
class KindGateway:
    kinds: list[str] = field(default_factory=list)

    async def open_session(self) -> None:
        return None

    async def status(self) -> SessionState:
        return SessionState(True, 1, 1)

    async def quotes(
        self,
        kind: str,
        targets: Sequence[QuoteTarget],
        on_quote: Callable[[QuoteRow], Awaitable[None]],
    ) -> None:
        self.kinds.append(kind)
        for target in targets:
            await on_quote(
                QuoteRow(
                    con_id=target.con_id,
                    status="ok",
                    market_data_type=1,
                    generation=1,
                    requested_at=1.0,
                    sent_at=2.0,
                    received_at=3.0,
                    fields=(),
                    error_code=0,
                    error_text="",
                )
            )


@asynccontextmanager
async def serving(gateway: KindGateway) -> AsyncIterator[grpc.aio.Channel]:
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


def request(kind: pb.QuoteKind.ValueType) -> pb.SnapshotRequest:
    return pb.SnapshotRequest(
        kind=kind,
        contracts=[pb.ContractSpec(sec_type=pb.SEC_TYPE_OPTION, con_id=5)],
    )


def test_option_open_kind_reaches_the_gateway() -> None:
    async def run() -> tuple[list[str], int]:
        gateway = KindGateway()
        async with serving(gateway) as channel:
            stub = pb.MarketDataServiceStub(channel)
            got = [i async for i in stub.Snapshot(request(pb.QUOTE_KIND_OPTION_OPEN))]
        return gateway.kinds, len(got)

    kinds, count = asyncio.run(run())
    assert kinds == [spec.OPTION_OPEN] and count == 1


def test_unknown_kind_is_invalid_argument() -> None:
    async def run() -> grpc.StatusCode:
        async with serving(KindGateway()) as channel:
            stub = pb.MarketDataServiceStub(channel)
            try:
                [i async for i in stub.Snapshot(request(pb.QUOTE_KIND_UNSPECIFIED))]
            except grpc.aio.AioRpcError as exc:
                return exc.code()
        raise AssertionError("the call succeeded")

    assert asyncio.run(run()) == grpc.StatusCode.INVALID_ARGUMENT


def test_option_open_is_a_logged_word() -> None:
    assert "option_open" in transport.WORDS


def test_quote_row_missing_reaches_proto() -> None:
    row = QuoteRow(5, "partial", 1, 1, 1.0, 2.0, 3.0)
    assert list(transport.quote_to_proto(row).missing) == []
    row = QuoteRow(
        5,
        "partial",
        1,
        1,
        1.0,
        2.0,
        3.0,
        missing=(("open_interest", 2, 0), ("bid", 4, 354)),
    )
    got = [
        (m.field, m.reason, m.error_code) for m in transport.quote_to_proto(row).missing
    ]
    assert got == [("open_interest", 2, 0), ("bid", 4, 354)]
