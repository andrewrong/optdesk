"""The three services over a fake Gateway and a real grpc.aio channel on a Unix socket."""

from __future__ import annotations

import asyncio
from collections.abc import AsyncIterator, Awaitable, Callable, Sequence
from contextlib import asynccontextmanager
from dataclasses import dataclass, field

import grpc
from kit.sockdir import short_socket_dir

from optdesk.contracts import api as pb
from optdesk.ibkr import spec
from optdesk.ibkr.api import (
    DividendKey,
    DividendReading,
    HistoryBar,
    HistoryPaced,
    HistoryRequest,
)
from optdesk.ibkr.model import (
    ChainRow,
    QuoteRow,
    QuoteTarget,
    ResolveResult,
    SessionState,
)
from optdesk.ibkr.transport import build_server


def row(
    con_id: int,
    status: str = "ok",
    *,
    bid: float | None = None,
    error_code: int = 0,
    error_text: str = "",
) -> QuoteRow:
    return QuoteRow(
        con_id=con_id,
        status=status,
        market_data_type=1,
        generation=3,
        requested_at=1.0,
        sent_at=2.0,
        received_at=3.0,
        fields=() if bid is None else (("bid", bid),),
        error_code=error_code,
        error_text=error_text,
    )


@dataclass
class FakeGateway:
    state: SessionState = field(default_factory=lambda: SessionState(True, 3, 1))
    chain_rows: list[ChainRow] = field(default_factory=list)
    resolved: ResolveResult | BaseException = field(
        default_factory=lambda: ResolveResult(True, 7)
    )
    rows: list[QuoteRow] = field(default_factory=list)
    seen: list[QuoteTarget] = field(default_factory=list)
    quote_error: BaseException | None = None
    hang: bool = False
    released: asyncio.Event = field(default_factory=asyncio.Event)

    async def open_session(self) -> None:
        return None

    async def status(self) -> SessionState:
        return self.state

    async def chains(
        self, *, sec_type: str, symbol: str, con_id: int
    ) -> Sequence[ChainRow]:
        return self.chain_rows

    async def resolve(
        self,
        *,
        sec_type: str,
        symbol: str,
        exchange: str,
        currency: str,
        expiry: str,
        strike: str,
        right: str,
        multiplier: str,
        trading_class: str,
    ) -> ResolveResult:
        if isinstance(self.resolved, BaseException):
            raise self.resolved
        return self.resolved

    async def quotes(
        self,
        kind: str,
        targets: Sequence[QuoteTarget],
        on_quote: Callable[[QuoteRow], Awaitable[None]],
    ) -> None:
        self.seen.extend(targets)
        try:
            for item in self.rows:
                await on_quote(item)
            if self.quote_error is not None:
                raise self.quote_error
            if self.hang:
                await asyncio.Event().wait()
        finally:
            self.released.set()


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


def contract(con_id: int = 0) -> pb.ContractSpec:
    return pb.ContractSpec(
        sec_type=pb.SEC_TYPE_STOCK, symbol="SPY", exchange="SMART", con_id=con_id
    )


async def code_of(call: Awaitable[object]) -> grpc.StatusCode:
    try:
        await call
    except grpc.aio.AioRpcError as exc:
        return exc.code()
    raise AssertionError("the call succeeded")


async def snapshot(
    channel: grpc.aio.Channel, con_ids: list[int]
) -> list[pb.SnapshotResponse]:
    stub = pb.MarketDataServiceStub(channel)
    request = pb.SnapshotRequest(
        kind=pb.QUOTE_KIND_SNAPSHOT, contracts=[contract(c) for c in con_ids]
    )
    return [item async for item in stub.Snapshot(request)]


def test_ping_reports_connected_generation_and_type() -> None:
    async def run() -> pb.PingResponse:
        async with serving(FakeGateway()) as channel:
            return await pb.SessionServiceStub(channel).Ping(pb.PingRequest())

    got = asyncio.run(run())
    assert got.connected and got.generation == 3
    assert got.market_data_type == pb.MARKET_DATA_TYPE_LIVE


def test_list_chains_returns_chain() -> None:
    async def run() -> pb.ListChainsResponse:
        gateway = FakeGateway(
            chain_rows=[ChainRow("SMART", "SPY", "100", ("20260116",), ("95", "100.5"))]
        )
        async with serving(gateway) as channel:
            request = pb.ListChainsRequest(underlying=contract(756733))
            return await pb.ContractsServiceStub(channel).ListChains(request)

    got = asyncio.run(run())
    assert got.underlying_con_id == 756733
    (chain,) = got.chains
    assert chain.exchange == "SMART" and list(chain.strikes) == ["95", "100.5"]


def test_resolve_found_and_not_listed() -> None:
    async def run() -> tuple[pb.ResolveContractResponse, pb.ResolveContractResponse]:
        gateway = FakeGateway(resolved=ResolveResult(True, 7, "SMART", "SPY", "100"))
        async with serving(gateway) as channel:
            stub = pb.ContractsServiceStub(channel)
            request = pb.ResolveContractRequest(contract=contract())
            found = await stub.ResolveContract(request)
            gateway.resolved = ResolveResult(False, 0, error_code=200, error_text="no")
            missing = await stub.ResolveContract(request)
            return found, missing

    found, missing = asyncio.run(run())
    assert found.status == pb.RESOLVE_STATUS_FOUND and found.contract.con_id == 7
    assert missing.status == pb.RESOLVE_STATUS_NOT_LISTED and missing.error_code == 200


def test_resolve_found_copies_hours() -> None:
    async def run() -> pb.ResolveContractResponse:
        gateway = FakeGateway(
            resolved=ResolveResult(
                True,
                7,
                trading_hours="20261116:0930-20261116:1600",
                liquid_hours="20261116:0930-20261116:1500",
                time_zone_id="US/Eastern",
            )
        )
        async with serving(gateway) as channel:
            stub = pb.ContractsServiceStub(channel)
            return await stub.ResolveContract(
                pb.ResolveContractRequest(contract=contract())
            )

    found = asyncio.run(run())
    assert found.trading_hours == "20261116:0930-20261116:1600"
    assert found.liquid_hours == "20261116:0930-20261116:1500"
    assert found.time_zone_id == "US/Eastern"


def test_resolve_not_listed_leaves_hours_empty() -> None:
    async def run() -> pb.ResolveContractResponse:
        gateway = FakeGateway(resolved=ResolveResult(False, 0, error_code=200))
        async with serving(gateway) as channel:
            stub = pb.ContractsServiceStub(channel)
            return await stub.ResolveContract(
                pb.ResolveContractRequest(contract=contract())
            )

    missing = asyncio.run(run())
    assert (missing.trading_hours, missing.liquid_hours, missing.time_zone_id) == (
        "",
        "",
        "",
    )


def test_resolve_several_is_failed_precondition() -> None:
    async def run() -> grpc.StatusCode:
        gateway = FakeGateway(resolved=spec.AmbiguousResolve("two contracts"))
        async with serving(gateway) as channel:
            request = pb.ResolveContractRequest(contract=contract())
            return await code_of(
                pb.ContractsServiceStub(channel).ResolveContract(request)
            )

    assert asyncio.run(run()) == grpc.StatusCode.FAILED_PRECONDITION


def test_snapshot_streams_one_quote_per_contract() -> None:
    async def run() -> list[pb.SnapshotResponse]:
        gateway = FakeGateway(
            rows=[row(1, bid=1.5), row(2, "partial"), row(3, "no_data")]
        )
        async with serving(gateway) as channel:
            return await snapshot(channel, [1, 2, 3])

    got = asyncio.run(run())
    quotes = [item.quote for item in got]
    assert [q.contract.con_id for q in quotes] == [1, 2, 3]
    assert [q.status for q in quotes] == [
        pb.QUOTE_STATUS_OK,
        pb.QUOTE_STATUS_PARTIAL,
        pb.QUOTE_STATUS_NO_DATA,
    ]
    assert quotes[0].bid == 1.5 and quotes[0].generation == 3


def test_snapshot_zero_con_id_is_invalid_argument() -> None:
    async def run() -> list[grpc.StatusCode]:
        async with serving(FakeGateway()) as channel:
            return [
                await code_of(snapshot(channel, [0])),
                await code_of(snapshot(channel, [])),
            ]

    assert asyncio.run(run()) == [grpc.StatusCode.INVALID_ARGUMENT] * 2


def test_snapshot_passes_spec_fields() -> None:
    async def run() -> list[QuoteTarget]:
        gateway = FakeGateway(rows=[row(7)])
        async with serving(gateway) as channel:
            stub = pb.MarketDataServiceStub(channel)
            request = pb.SnapshotRequest(
                kind=pb.QUOTE_KIND_SNAPSHOT,
                contracts=[
                    pb.ContractSpec(
                        sec_type=pb.SEC_TYPE_OPTION,
                        con_id=7,
                        exchange="OSE.JPN",
                        currency="JPY",
                        trading_class="225",
                    )
                ],
            )
            [item async for item in stub.Snapshot(request)]
        return gateway.seen

    assert asyncio.run(run()) == [QuoteTarget(7, "OSE.JPN", "JPY", "225", "OPT")]


def test_snapshot_cancel_releases_lines() -> None:
    async def run() -> bool:
        gateway = FakeGateway(rows=[row(1)], hang=True)
        async with serving(gateway) as channel:
            stub = pb.MarketDataServiceStub(channel)
            request = pb.SnapshotRequest(
                kind=pb.QUOTE_KIND_SNAPSHOT, contracts=[contract(1), contract(2)]
            )
            call = stub.Snapshot(request)
            await call.read()
            call.cancel()
            await asyncio.wait_for(gateway.released.wait(), 5)
            return gateway.released.is_set()

    assert asyncio.run(run())


def test_gateway_timeout_is_grpc_error_not_negative() -> None:
    async def run() -> list[grpc.StatusCode]:
        gateway = FakeGateway(resolved=TimeoutError(), quote_error=TimeoutError())
        async with serving(gateway) as channel:
            request = pb.ResolveContractRequest(contract=contract())
            return [
                await code_of(
                    pb.ContractsServiceStub(channel).ResolveContract(request)
                ),
                await code_of(snapshot(channel, [1])),
            ]

    assert asyncio.run(run()) == [grpc.StatusCode.DEADLINE_EXCEEDED] * 2


def test_disconnect_gives_disconnected_quotes() -> None:
    async def run() -> list[int]:
        gateway = FakeGateway(rows=[row(1, "disconnected"), row(2, "disconnected")])
        async with serving(gateway) as channel:
            return [item.quote.status for item in await snapshot(channel, [1, 2])]

    assert asyncio.run(run()) == [pb.QUOTE_STATUS_DISCONNECTED] * 2


def test_error_text_only_on_error_status() -> None:
    async def run() -> list[str]:
        gateway = FakeGateway(
            rows=[
                row(1, "error", error_code=162, error_text="why"),
                row(2, "no_subscription", error_code=354, error_text="sub"),
                row(3, "partial", error_text="stray"),
                row(4, "ok", error_text="stray"),
            ]
        )
        async with serving(gateway) as channel:
            return [i.quote.error_text for i in await snapshot(channel, [1, 2, 3, 4])]

    assert asyncio.run(run()) == ["why", "sub", "", ""]


def test_no_order_rpc() -> None:
    names = {
        name.lower()
        for servicer in (
            pb.SessionServiceServicer,
            pb.ContractsServiceServicer,
            pb.MarketDataServiceServicer,
        )
        for name in vars(servicer)
        if name[0].isupper()
    }
    assert names == {
        "ping", "listchains", "resolvecontract", "snapshot", "bars", "listcontracts"
    }


def test_snapshot_unspecified_kind_is_invalid_argument() -> None:
    async def run() -> grpc.StatusCode:
        async with serving(FakeGateway()) as channel:
            stub = pb.MarketDataServiceStub(channel)
            request = pb.SnapshotRequest(
                kind=pb.QUOTE_KIND_UNSPECIFIED, contracts=[contract(1)]
            )

            async def drain() -> None:
                async for _ in stub.Snapshot(request):
                    pass

            return await code_of(drain())

    assert asyncio.run(run()) == grpc.StatusCode.INVALID_ARGUMENT


@dataclass
class FakeReference(FakeGateway):
    readings: list[DividendReading] = field(default_factory=list)
    keys: list[DividendKey] = field(default_factory=list)

    async def dividends(
        self,
        keys: Sequence[DividendKey],
        on_reading: Callable[[DividendReading], Awaitable[None]],
    ) -> None:
        self.keys.extend(keys)
        for item in self.readings:
            await on_reading(item)


async def dividends(
    channel: grpc.aio.Channel, keys: list[pb.ContractKey]
) -> list[pb.DividendProjection]:
    stub = pb.ReferenceServiceStub(channel)
    return [item async for item in stub.Dividends(iter(keys))]


def test_dividends_stream_one_projection_per_key() -> None:
    key = DividendKey("AAPL", "STK", "SMART", "USD", 7)
    gateway = FakeReference(
        readings=[
            DividendReading(key, "ok", "1", None, "20260102", "0.25", 0, 3.0),
            DividendReading(key, "timeout", None, None, None, None, 0, 4.0),
        ]
    )

    async def run() -> list[pb.DividendProjection]:
        async with serving(gateway) as channel:
            return await dividends(
                channel,
                [pb.ContractKey(symbol="AAPL", sec_type=pb.SEC_TYPE_STOCK, con_id=7)],
            )

    first, second = asyncio.run(run())
    assert gateway.keys == [DividendKey("AAPL", "STK", "", "", 7)]
    assert first.status == pb.DIVIDEND_STATUS_OK
    assert first.key.con_id == 7 and first.key.sec_type == pb.SEC_TYPE_STOCK
    assert first.HasField("past_12m") and first.past_12m == "1"
    assert not first.HasField("next_12m")
    assert first.next_ex_date == "20260102" and first.next_amount == "0.25"
    assert first.received_at.seconds == 3
    assert second.status == pb.DIVIDEND_STATUS_TIMEOUT
    assert not second.HasField("next_ex_date")


def test_dividends_without_keys_is_invalid_argument() -> None:
    async def run() -> grpc.StatusCode:
        async with serving(FakeReference()) as channel:
            return await code_of(dividends(channel, []))

    assert asyncio.run(run()) == grpc.StatusCode.INVALID_ARGUMENT


def test_dividends_on_gateway_without_reference_is_unimplemented() -> None:
    async def run() -> grpc.StatusCode:
        async with serving(FakeGateway()) as channel:
            return await code_of(
                dividends(channel, [pb.ContractKey(symbol="AAPL", con_id=7)])
            )

    assert asyncio.run(run()) == grpc.StatusCode.UNIMPLEMENTED


@dataclass
class FakeHistory(FakeGateway):
    answer: list[HistoryBar] = field(default_factory=list)
    asked: list[HistoryRequest] = field(default_factory=list)
    paced: bool = False

    async def bars(self, request: HistoryRequest) -> Sequence[HistoryBar]:
        self.asked.append(request)
        if self.paced:
            raise HistoryPaced("paced")
        return self.answer


async def bars(
    channel: grpc.aio.Channel,
    *,
    con_id: int = 7,
    kind: pb.BarKind.ValueType = pb.BAR_KIND_TRADES,
    days: int = 5,
    end_day: str = "",
) -> pb.BarsResponse:
    return await pb.MarketDataServiceStub(channel).Bars(
        pb.BarsRequest(
            contract=contract(con_id), kind=kind, duration_days=days, end_day=end_day
        )
    )


def test_bars_answer_maps_rows_and_unset_volume() -> None:
    gateway = FakeHistory(
        answer=[
            HistoryBar("2026-01-02", "1", "2", "0.5", "1.5", "100"),
            HistoryBar("2026-01-05", "1.5", "2", "1", "1.75", None),
        ]
    )

    async def run() -> pb.BarsResponse:
        async with serving(gateway) as channel:
            return await bars(channel, kind=pb.BAR_KIND_MIDPOINT, days=400)

    got = asyncio.run(run())
    assert [(b.day, b.close) for b in got.bars] == [
        ("2026-01-02", "1.5"),
        ("2026-01-05", "1.75"),
    ]
    assert got.bars[0].volume == "100" and not got.bars[1].HasField("volume")
    assert gateway.asked[0].kind == "midpoint" and gateway.asked[0].duration_days == 400
    assert gateway.asked[0].key.con_id == 7 and gateway.asked[0].key.sec_type == "STK"


def test_bars_option_implied_vol_kind() -> None:
    gateway = FakeHistory(answer=[HistoryBar("2026-01-02", "0.28", "0.3", "0.27", "0.2813", None)])

    async def run() -> pb.BarsResponse:
        async with serving(gateway) as channel:
            return await bars(channel, kind=pb.BAR_KIND_OPTION_IMPLIED_VOL)

    got = asyncio.run(run())
    assert gateway.asked[0].kind == "option_implied_vol"
    assert [b.close for b in got.bars] == ["0.2813"]


def test_missing_con_id_is_invalid() -> None:
    async def run() -> grpc.StatusCode:
        async with serving(FakeHistory()) as channel:
            return await code_of(bars(channel, con_id=0))

    assert asyncio.run(run()) == grpc.StatusCode.INVALID_ARGUMENT


def test_duration_outside_range_is_invalid() -> None:
    async def run() -> list[grpc.StatusCode]:
        async with serving(FakeHistory()) as channel:
            return [await code_of(bars(channel, days=days)) for days in (0, -1, 731)]

    assert set(asyncio.run(run())) == {grpc.StatusCode.INVALID_ARGUMENT}


def test_unset_kind_and_bad_end_day_are_invalid() -> None:
    async def run() -> list[grpc.StatusCode]:
        async with serving(FakeHistory()) as channel:
            return [
                await code_of(bars(channel, kind=pb.BAR_KIND_UNSPECIFIED)),
                await code_of(bars(channel, end_day="2026/01/02")),
            ]

    assert set(asyncio.run(run())) == {grpc.StatusCode.INVALID_ARGUMENT}


def test_end_day_with_adjusted_last_is_invalid() -> None:
    gateway = FakeHistory()

    async def run() -> grpc.StatusCode:
        async with serving(gateway) as channel:
            return await code_of(
                bars(channel, kind=pb.BAR_KIND_ADJUSTED_LAST, end_day="2026-01-02")
            )

    assert asyncio.run(run()) == grpc.StatusCode.INVALID_ARGUMENT
    assert gateway.asked == []


def test_paced_answer_is_resource_exhausted() -> None:
    async def run() -> grpc.StatusCode:
        async with serving(FakeHistory(paced=True)) as channel:
            return await code_of(bars(channel))

    assert asyncio.run(run()) == grpc.StatusCode.RESOURCE_EXHAUSTED


def test_bars_on_gateway_without_history_is_unimplemented() -> None:
    async def run() -> grpc.StatusCode:
        async with serving(FakeGateway()) as channel:
            return await code_of(bars(channel))

    assert asyncio.run(run()) == grpc.StatusCode.UNIMPLEMENTED
