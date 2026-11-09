"""The one module that builds stubs; each call passes a timeout and only the allowed rpcs exist."""

from __future__ import annotations

from collections.abc import Iterable

import grpc

from optdesk.contracts import api

PING_TIMEOUT_S = 8.0
ACCOUNT_TIMEOUT_S = 30.0
CHAINS_TIMEOUT_S = 45.0
SNAPSHOT_TIMEOUT_S = 45.0


class LiveClient:
    """Wrappers of the read rpcs on one channel, each with its timeout."""

    def __init__(self, channel: grpc.Channel) -> None:
        self._session = api.SessionServiceStub(channel)
        self._account_stub = api.AccountServiceStub(channel)
        self._contracts = api.ContractsServiceStub(channel)
        self._market = api.MarketDataServiceStub(channel)
        self._reference = api.ReferenceServiceStub(channel)

    def ping(self) -> api.PingResponse:
        return self._session.Ping(api.PingRequest(), timeout=PING_TIMEOUT_S)

    def account(self) -> api.AccountResponse:
        return self._account_stub.Account(
            api.AccountRequest(), timeout=ACCOUNT_TIMEOUT_S
        )

    def executions(self) -> api.ExecutionsResponse:
        return self._account_stub.Executions(
            api.ExecutionsRequest(), timeout=ACCOUNT_TIMEOUT_S
        )

    def resolve_contract(self, spec: api.ContractSpec) -> api.ResolveContractResponse:
        return self._contracts.ResolveContract(
            api.ResolveContractRequest(contract=spec), timeout=CHAINS_TIMEOUT_S
        )

    def list_chains(self, underlying: api.ContractSpec) -> api.ListChainsResponse:
        return self._contracts.ListChains(
            api.ListChainsRequest(underlying=underlying), timeout=CHAINS_TIMEOUT_S
        )

    def snapshot(
        self, kind: api.QuoteKind.ValueType, contracts: Iterable[api.ContractSpec]
    ) -> list[api.SnapshotResponse]:
        stream = self._market.Snapshot(
            api.SnapshotRequest(kind=kind, contracts=contracts),
            timeout=SNAPSHOT_TIMEOUT_S,
        )
        return list(stream)

    def dividends(
        self, keys: Iterable[api.ContractKey]
    ) -> list[api.DividendProjection]:
        stream = self._reference.Dividends(iter(list(keys)), timeout=SNAPSHOT_TIMEOUT_S)
        return list(stream)

    def bars(self, request: api.BarsRequest) -> api.BarsResponse:
        """The pinned stub call: used by the one test that expects UNIMPLEMENTED."""
        return self._market.Bars(request, timeout=CHAINS_TIMEOUT_S)
