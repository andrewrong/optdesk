"""Contract test of optdesk.contracts.api, from both sides: a caller and a fake servicer."""

from __future__ import annotations

import asyncio
import importlib
import pathlib
from collections.abc import AsyncIterator
from importlib import resources
from unittest.mock import MagicMock

import grpc

from optdesk.contracts import api
from optdesk.contracts.api import (
    DIVIDEND_STATUS_NO_VALUE,
    DIVIDEND_STATUS_OK,
    QUOTE_KIND_OPTION_EOD,
    QUOTE_KIND_SNAPSHOT,
    RESOLVE_STATUS_FOUND,
    SEC_TYPE_OPTION,
    AccountRequest,
    AccountResponse,
    AccountServiceServicer,
    AccountServiceStub,
    AccountValue,
    ContractKey,
    ContractSpec,
    ContractsServiceServicer,
    ContractsServiceStub,
    DividendProjection,
    DividendStatus,
    Execution,
    ExecutionsRequest,
    ExecutionsResponse,
    ListChainsRequest,
    ListChainsResponse,
    MarketDataServiceServicer,
    MarketDataServiceStub,
    Position,
    Quote,
    ReferenceServiceServicer,
    ReferenceServiceStub,
    ResolveContractRequest,
    ResolveContractResponse,
    SessionServiceServicer,
    SessionServiceStub,
    SnapshotRequest,
    add_AccountServiceServicer_to_server,
)

REPO = pathlib.Path(__file__).resolve().parents[3]
GENPB = pathlib.Path(str(resources.files("optdesk.contracts"))) / "genpb"


def test_api_exports_all_services() -> None:
    for name in api.__all__:
        assert hasattr(api, name), name
    servicers = (
        SessionServiceServicer,
        ContractsServiceServicer,
        MarketDataServiceServicer,
    )
    stubs = (SessionServiceStub, ContractsServiceStub, MarketDataServiceStub)
    assert all(isinstance(s, type) for s in servicers + stubs)


def test_caller_builds_snapshot_request() -> None:
    spec = ContractSpec(
        sec_type=SEC_TYPE_OPTION,
        symbol="SPY",
        expiry="20260116",
        strike="600",
        con_id=123,
    )
    request = SnapshotRequest(kind=QUOTE_KIND_OPTION_EOD, contracts=[spec])
    assert request.kind == QUOTE_KIND_OPTION_EOD
    assert len(request.contracts) == 1
    assert request.contracts[0].con_id == 123
    assert request.contracts[0].strike == "600"


class _FakeContracts(ContractsServiceServicer):
    async def ListChains(
        self,
        request: ListChainsRequest,
        context: grpc.aio.ServicerContext[ListChainsRequest, ListChainsResponse],
    ) -> ListChainsResponse:
        return ListChainsResponse(underlying_con_id=request.underlying.con_id)

    async def ResolveContract(
        self,
        request: ResolveContractRequest,
        context: grpc.aio.ServicerContext[
            ResolveContractRequest, ResolveContractResponse
        ],
    ) -> ResolveContractResponse:
        return ResolveContractResponse(
            status=RESOLVE_STATUS_FOUND, contract=request.contract
        )


def test_fake_servicer_satisfies_base() -> None:
    request = ResolveContractRequest(contract=ContractSpec(symbol="SPY"))

    async def run() -> ResolveContractResponse:
        servicer = _FakeContracts()
        # Called in process: the context is not used by the fake.
        return await servicer.ResolveContract(request, MagicMock())

    response = asyncio.run(run())
    assert response.status == RESOLVE_STATUS_FOUND
    assert response.contract.symbol == "SPY"


def test_generated_python_is_current(tmp_path: pathlib.Path) -> None:
    protos = sorted(str(p) for p in (REPO / "contracts/proto/ibkr/v1").glob("*.proto"))
    args = [
        "protoc",
        f"-I{REPO / 'contracts/proto'}",
        f"-I{resources.files('grpc_tools') / '_proto'}",
        f"--python_out={tmp_path}",
        f"--mypy_out={tmp_path}",
        f"--grpc_python_out={tmp_path}",
        f"--mypy_grpc_out={tmp_path}",
        *protos,
    ]
    protoc = importlib.import_module("grpc_tools.protoc")
    assert protoc.main(args) == 0
    for directory in (tmp_path, tmp_path / "ibkr", tmp_path / "ibkr" / "v1"):
        (directory / "__init__.py").write_text("")
    generated = sorted(p for p in tmp_path.rglob("*.py*") if p.is_file())
    assert generated
    for path in generated:
        text = path.read_text().replace(
            "from ibkr.v1 import", "from optdesk.contracts.genpb.ibkr.v1 import"
        )
        committed = GENPB / path.relative_to(tmp_path)
        assert committed.read_text() == text, str(committed)
    committed_files = {
        p.relative_to(GENPB) for p in GENPB.rglob("*") if p.suffix in (".py", ".pyi")
    }
    assert committed_files == {p.relative_to(tmp_path) for p in generated}


def test_snapshot_kind_and_sizes_exist() -> None:
    quote = Quote(bid_size=1.0, ask_size=2.0, last_size=3.0)
    assert QUOTE_KIND_SNAPSHOT == 3
    assert (quote.bid_size, quote.ask_size, quote.last_size) == (1.0, 2.0, 3.0)
    assert quote.HasField("event_ts") is False
    assert quote.HasField("bid") is False


class _FakeAccount(AccountServiceServicer):
    async def Account(
        self,
        request: AccountRequest,
        context: grpc.aio.ServicerContext[AccountRequest, AccountResponse],
    ) -> AccountResponse:
        return AccountResponse(
            account="B",
            generation=3,
            values_collected=True,
            positions_collected=False,
            values=[AccountValue(tag="NetLiquidation", value="100.5", currency="USD")],
            positions=[
                Position(
                    contract=ContractSpec(symbol="SPY"), quantity="-2", avg_cost="1.25"
                ),
                Position(
                    contract=ContractSpec(symbol="QQQ"), quantity="1", avg_cost="3"
                ),
            ],
        )

    async def Executions(
        self,
        request: ExecutionsRequest,
        context: grpc.aio.ServicerContext[ExecutionsRequest, ExecutionsResponse],
    ) -> ExecutionsResponse:
        return ExecutionsResponse(
            account="B",
            generation=3,
            collected=True,
            pending=1,
            executions=[
                Execution(
                    exec_id="0001",
                    perm_id=77,
                    time="2026-11-09T15:30:00Z",
                    contract=ContractSpec(symbol="SPY"),
                    side="BOT",
                    quantity="2",
                    price="1.25",
                    commission="0",
                    commission_currency="USD",
                )
            ],
        )


def _public_methods(cls: type) -> set[str]:
    return {n for n in dir(cls) if not n.startswith("_")}


def test_account_service_surface() -> None:
    assert _public_methods(AccountServiceServicer) == {"Account", "Executions"}
    channel = MagicMock()
    stub = AccountServiceStub(channel)
    assert _public_methods(type(stub)) == set()
    assert vars(stub).keys() == {"Account", "Executions"}

    async def run() -> AccountResponse:
        return await _FakeAccount().Account(AccountRequest(), MagicMock())

    response = asyncio.run(run())
    assert response.account == "B"
    assert response.generation == 3
    assert [v.tag for v in response.values] == ["NetLiquidation"]
    assert [p.contract.symbol for p in response.positions] == ["SPY", "QQQ"]
    assert response.positions[0].quantity == "-2"
    assert response.values_collected is True
    assert response.positions_collected is False

    async def run_executions() -> ExecutionsResponse:
        return await _FakeAccount().Executions(ExecutionsRequest(), MagicMock())

    executions = asyncio.run(run_executions())
    assert (executions.account, executions.collected, executions.pending) == (
        "B",
        True,
        1,
    )
    assert executions.executions[0].perm_id == 77
    assert executions.executions[0].commission == "0"
    assert executions.executions[0].contract.symbol == "SPY"


def test_account_service_registers_on_server() -> None:
    server = MagicMock()
    add_AccountServiceServicer_to_server(_FakeAccount(), server)
    (handlers,) = server.add_generic_rpc_handlers.call_args.args
    (generic,) = handlers
    assert generic.service_name() == "ibkr.v1.AccountService"
    assert (
        server.add_registered_method_handlers.call_args.args[0]
        == "ibkr.v1.AccountService"
    )
    methods = server.add_registered_method_handlers.call_args.args[1]
    assert set(methods) == {"Account", "Executions"}


class _FakeReference(ReferenceServiceServicer):
    async def Dividends(
        self,
        request_iterator: AsyncIterator[ContractKey],
        context: grpc.aio.ServicerContext[ContractKey, DividendProjection],
    ) -> AsyncIterator[DividendProjection]:
        async for key in request_iterator:
            if key.symbol == "KO":
                yield DividendProjection(
                    key=key,
                    status=DIVIDEND_STATUS_OK,
                    next_ex_date="20261130",
                    next_amount="0.51",
                )
            else:
                yield DividendProjection(key=key, status=DIVIDEND_STATUS_NO_VALUE)


def test_reference_service_dividends_stream() -> None:
    assert _public_methods(ReferenceServiceServicer) == {"Dividends"}
    assert vars(ReferenceServiceStub(MagicMock())).keys() == {"Dividends"}

    async def keys() -> AsyncIterator[ContractKey]:
        yield ContractKey(symbol="KO", exchange="SMART", currency="USD", con_id=8894)
        yield ContractKey(symbol="TSLA", exchange="SMART", currency="USD")

    async def run() -> list[DividendProjection]:
        stream = _FakeReference().Dividends(keys(), MagicMock())
        return [item async for item in stream]

    answers = asyncio.run(run())
    assert [a.key.symbol for a in answers] == ["KO", "TSLA"]
    assert answers[0].key.con_id == 8894
    assert answers[0].next_ex_date == "20261130"
    assert answers[0].HasField("past_12m") is False
    assert answers[1].status == DividendStatus.Value("DIVIDEND_STATUS_NO_VALUE")
    assert answers[1].HasField("next_ex_date") is False


def test_account_servicer_executions_stub() -> None:
    from optdesk.ibkr.transport import AccountServicer

    servicer = AccountServicer(MagicMock(), "B", ())

    async def run() -> ExecutionsResponse:
        return await servicer.Executions(ExecutionsRequest(), MagicMock())

    response = asyncio.run(run())
    assert response.account == "B"
    assert response.collected is False
    assert len(response.executions) == 0
