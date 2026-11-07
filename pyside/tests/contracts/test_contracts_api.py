"""Contract test of optdesk.contracts.api, from both sides: a caller and a fake servicer."""

from __future__ import annotations

import asyncio
import importlib
import pathlib
from importlib import resources
from unittest.mock import MagicMock

import grpc

from optdesk.contracts import api
from optdesk.contracts.api import (
    QUOTE_KIND_OPTION_EOD,
    RESOLVE_STATUS_FOUND,
    SEC_TYPE_OPTION,
    ContractSpec,
    ContractsServiceServicer,
    ContractsServiceStub,
    ListChainsRequest,
    ListChainsResponse,
    MarketDataServiceServicer,
    MarketDataServiceStub,
    ResolveContractRequest,
    ResolveContractResponse,
    SessionServiceServicer,
    SessionServiceStub,
    SnapshotRequest,
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
