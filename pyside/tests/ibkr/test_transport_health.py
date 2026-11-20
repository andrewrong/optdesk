"""Ping state and farms, and the retryable statuses of the pyside transport."""

from __future__ import annotations

import asyncio
from collections.abc import AsyncIterator, Sequence
from contextlib import asynccontextmanager
from dataclasses import dataclass, field
from pathlib import Path

import grpc
from kit.sockdir import short_socket_dir

from optdesk.contracts import api as pb
from optdesk.ibkr.api import HistoryBar, HistoryRequest, HistoryUnavailable
from optdesk.ibkr.model import ChainRow, Farm, ResolveResult, SessionState
from optdesk.ibkr.transport import build_server

UP, SOFT_LOST, DOWN, FARM_BROKEN = 1, 2, 3, 4  # SessionState of session.proto


@dataclass
class FakeGateway:
    state: SessionState = field(default_factory=lambda: SessionState(True, 3, 1))
    bars_error: BaseException | None = None

    async def open_session(self) -> None:
        return None

    async def status(self) -> SessionState:
        return self.state

    async def chains(
        self, *, sec_type: str, symbol: str, con_id: int
    ) -> Sequence[ChainRow]:
        return []

    async def resolve(self, **kwargs: str) -> ResolveResult:
        return ResolveResult(True, 7)

    async def quotes(self, kind: str, targets: object, on_quote: object) -> None:
        return None

    async def bars(self, request: HistoryRequest) -> Sequence[HistoryBar]:
        if self.bars_error is not None:
            raise self.bars_error
        return []


@asynccontextmanager
async def serving(gateway: FakeGateway) -> AsyncIterator[grpc.aio.Channel]:
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


TRANSPORT = Path(__file__).parents[2] / "src" / "optdesk" / "ibkr" / "transport.py"


def _ping(state: SessionState) -> pb.PingResponse:
    async def run() -> pb.PingResponse:
        async with serving(FakeGateway(state=state)) as channel:
            return await pb.SessionServiceStub(channel).Ping(pb.PingRequest())

    return asyncio.run(run())


class TestPingCarriesFarms:
    def test_state_and_every_farm(self) -> None:
        got = _ping(
            SessionState(
                True,
                3,
                1,
                state="farm_broken",
                farms=(Farm("ushmds", False, 1000), Farm("usfarm", True, 2000)),
            )
        )
        assert got.state == FARM_BROKEN
        assert [(f.name, f.up, f.since_unix_ms) for f in got.farms] == [
            ("ushmds", False, 1000),
            ("usfarm", True, 2000),
        ]
        assert got.connected and got.generation == 3
        assert got.market_data_type == pb.MARKET_DATA_TYPE_LIVE

    def test_each_state_word(self) -> None:
        for word, value in {
            "up": UP,
            "soft_lost": SOFT_LOST,
            "down": DOWN,
        }.items():
            assert _ping(SessionState(True, 1, 1, state=word)).state == value

    def test_default_state_is_up_without_farms(self) -> None:
        got = _ping(SessionState(True, 3, 1))
        assert got.state == UP and not got.farms


def _bars_error(gateway: FakeGateway) -> grpc.aio.AioRpcError:
    async def run() -> grpc.aio.AioRpcError:
        async with serving(gateway) as channel:
            request = pb.BarsRequest(
                contract=pb.ContractSpec(
                    sec_type=pb.SEC_TYPE_STOCK, symbol="SPY", exchange="SMART", con_id=7
                ),
                kind=pb.BAR_KIND_TRADES,
                duration_days=5,
            )
            try:
                await pb.MarketDataServiceStub(channel).Bars(request)
            except grpc.aio.AioRpcError as exc:
                return exc
        raise AssertionError("the call succeeded")

    return asyncio.run(run())


class TestHistoryUnavailableStatus:
    def test_unavailable_with_message(self) -> None:
        exc = _bars_error(FakeGateway(bars_error=HistoryUnavailable("no answer")))
        assert exc.code() == grpc.StatusCode.UNAVAILABLE
        assert exc.details() == "history_unavailable"


class TestInFlightReconnect:
    def test_lost_session_is_session_reconnecting(self) -> None:
        exc = _bars_error(
            FakeGateway(bars_error=ConnectionError("the gateway session was lost"))
        )
        assert exc.code() == grpc.StatusCode.UNAVAILABLE
        assert exc.details() == "session_reconnecting"

    def test_other_errors_stay_internal(self) -> None:
        exc = _bars_error(FakeGateway(bars_error=RuntimeError("x")))
        assert exc.code() == grpc.StatusCode.INTERNAL


class TestSizeCeiling:
    def test_transport_stays_under_800_lines(self) -> None:
        assert len(TRANSPORT.read_text().splitlines()) < 800
