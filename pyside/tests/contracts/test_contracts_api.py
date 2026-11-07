"""Contract test of optdesk.contracts.api, from both sides: a caller and a fake servicer."""

from __future__ import annotations

import importlib
import importlib.resources
import os
import pathlib
import sys
from typing import Any

import pytest

from optdesk.contracts import api

_ROOT = pathlib.Path(__file__).resolve().parents[3]
_GENPB = _ROOT / "pyside" / "src" / "optdesk" / "contracts" / "genpb"


def test_api_exports_all_services() -> None:
    for name in api.__all__:
        assert hasattr(api, name), name
    for service in ("Session", "Contracts", "MarketData"):
        assert isinstance(getattr(api, f"{service}ServiceServicer"), type)
        assert isinstance(getattr(api, f"{service}ServiceStub"), type)
        assert callable(getattr(api, f"add_{service}ServiceServicer_to_server"))


def test_caller_builds_snapshot_request() -> None:
    option = api.ContractSpec(
        sec_type=api.SecType.SEC_TYPE_OPTION,
        symbol="SPY",
        expiry="20261218",
        strike="500",
        right=api.Right.RIGHT_CALL,
        con_id=123456,
    )
    request = api.SnapshotRequest(
        kind=api.QuoteKind.QUOTE_KIND_OPTION_EOD, contracts=[option]
    )
    assert request.kind == api.QuoteKind.QUOTE_KIND_OPTION_EOD
    assert len(request.contracts) == 1
    assert request.contracts[0].con_id == 123456
    assert request.contracts[0].sec_type == api.SecType.SEC_TYPE_OPTION


class _FakeContracts(api.ContractsServiceServicer):
    def ListChains(
        self, request: api.ListChainsRequest, context: Any
    ) -> api.ListChainsResponse:
        return api.ListChainsResponse(underlying_con_id=request.underlying.con_id)

    def ResolveContract(
        self, request: api.ResolveContractRequest, context: Any
    ) -> api.ResolveContractResponse:
        return api.ResolveContractResponse(
            status=api.ResolveStatus.RESOLVE_STATUS_FOUND,
            contract=request.contract,
        )


def test_fake_servicer_satisfies_base() -> None:
    servicer = _FakeContracts()
    request = api.ResolveContractRequest(contract=api.ContractSpec(symbol="SPY"))
    response = servicer.ResolveContract(request, None)
    assert isinstance(response, api.ResolveContractResponse)
    assert response.status == api.ResolveStatus.RESOLVE_STATUS_FOUND
    assert response.contract.symbol == "SPY"


def test_generated_python_is_current(
    tmp_path: pathlib.Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    # The same options as contracts/generate-pyproto.sh; the plugins sit beside the interpreter.
    monkeypatch.setenv(
        "PATH", f"{pathlib.Path(sys.executable).parent}:{os.environ['PATH']}"
    )
    protoc = importlib.import_module("grpc_tools.protoc")
    include = importlib.resources.files("grpc_tools") / "_proto"
    out = tmp_path / "genpb"
    out.mkdir()
    sources = sorted((_ROOT / "contracts" / "proto" / "ibkr" / "v1").glob("*.proto"))
    code = protoc.main(
        [
            "grpc_tools.protoc",
            f"-I{_ROOT / 'contracts' / 'proto'}",
            f"-I{include}",
            f"--python_out={out}",
            f"--mypy_out={out}",
            f"--grpc_python_out={out}",
            f"--mypy_grpc_out={out}",
            *[str(p) for p in sources],
        ]
    )
    assert code == 0
    for path in sorted(out.rglob("*.py*")):
        text = path.read_text()
        path.write_text(
            text.replace(
                "from ibkr.v1 import", "from optdesk.contracts.genpb.ibkr.v1 import"
            )
        )
    for package in (out, out / "ibkr", out / "ibkr" / "v1"):
        (package / "__init__.py").write_text("")

    generated = sorted(p.relative_to(out) for p in out.rglob("*") if p.is_file())
    committed = sorted(
        p.relative_to(_GENPB)
        for p in _GENPB.rglob("*")
        if p.is_file() and "__pycache__" not in p.parts
    )
    assert generated == committed
    for rel in generated:
        assert (out / rel).read_text() == (_GENPB / rel).read_text(), str(rel)
