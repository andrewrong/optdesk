"""Read-only live tests against the deployed sockets; see the package docstring to run."""

from __future__ import annotations

import json
import os
import urllib.request
from collections.abc import Callable
from datetime import date, timedelta
from typing import cast

import grpc
import pytest

from live.client import LiveClient
from optdesk.contracts import api

HEALTH_TIMEOUT_S = 8.0
MIN_DAYS_OUT = 14


def _stock(
    symbol: str, exchange: str = "SMART", currency: str = "USD"
) -> api.ContractSpec:
    return api.ContractSpec(
        sec_type=api.SEC_TYPE_STOCK, symbol=symbol, exchange=exchange, currency=currency
    )


def _resolved(client: LiveClient, spec: api.ContractSpec) -> api.ContractSpec:
    reply = client.resolve_contract(spec)
    assert reply.status == api.RESOLVE_STATUS_FOUND
    assert reply.contract.con_id > 0
    return reply.contract


def _code(call: Callable[[], object]) -> grpc.StatusCode:
    with pytest.raises(grpc.RpcError) as caught:
        call()
    return cast(grpc.Call, caught.value).code()


def test_live_health_gateways() -> None:
    port = os.environ.get("OT_IBKR_HEALTH_PORT")
    if not port:
        pytest.skip("OT_IBKR_HEALTH_PORT is not set")
    url = f"http://127.0.0.1:{port}/health"
    with urllib.request.urlopen(url, timeout=HEALTH_TIMEOUT_S) as reply:
        assert reply.status == 200
        body = json.loads(reply.read())
    gateways = body["gateways"]
    assert gateways
    assert all(v is True for v in gateways.values())


def test_live_ping_main(main_client: LiveClient) -> None:
    assert main_client.ping().connected


def test_live_ping_wife(wife_client: LiveClient) -> None:
    assert wife_client.ping().connected


def test_live_ping_default(default_client: LiveClient) -> None:
    assert default_client.ping().connected


def test_live_account_main(main_client: LiveClient) -> None:
    reply = main_client.account()
    assert len(reply.values) > 0
    assert len(reply.positions) > 0


def test_live_account_wife(wife_client: LiveClient) -> None:
    reply = wife_client.account()
    assert len(reply.values) > 0
    assert len(reply.positions) > 0


def test_live_executions_answers(
    main_client: LiveClient, record_property: Callable[[str, object], None]
) -> None:
    reply = main_client.executions()
    record_property("collected", reply.collected)
    assert reply.pending >= 0


def test_live_resolve_contract(main_client: LiveClient) -> None:
    for spec in (_stock("AAPL"), _stock("SPY"), _stock("7203", "TSEJ", "JPY")):
        _resolved(main_client, spec)


def _spy_chains(client: LiveClient) -> tuple[api.ContractSpec, api.Chain]:
    underlying = _resolved(client, _stock("SPY"))
    reply = client.list_chains(underlying)
    classes = [c.trading_class for c in reply.chains]
    chain = next((c for c in reply.chains if c.trading_class == "SPY"), None)
    assert chain is not None, f"no chain of trading class SPY among {classes}"
    return underlying, chain


def test_live_list_chains_spy_trading_class(main_client: LiveClient) -> None:
    _, chain = _spy_chains(main_client)
    assert chain.trading_class == "SPY"
    assert len(chain.expirations) > 10


def test_live_snapshot_underlying_iv_spy(main_client: LiveClient) -> None:
    underlying = _resolved(main_client, _stock("SPY"))
    replies = main_client.snapshot(api.QUOTE_KIND_UNDERLYING_IV, [underlying])
    assert len(replies) == 1
    assert replies[0].quote.status == api.QUOTE_STATUS_OK


def test_live_option_eod_spy_put(
    main_client: LiveClient, record_property: Callable[[str, object], None]
) -> None:
    underlying, chain = _spy_chains(main_client)
    earliest = (date.today() + timedelta(days=MIN_DAYS_OUT)).strftime("%Y%m%d")
    expiry = next(e for e in sorted(chain.expirations) if e >= earliest)
    strikes = sorted(chain.strikes, key=float)
    strike = strikes[len(strikes) // 2]
    option = _resolved(
        main_client,
        api.ContractSpec(
            sec_type=api.SEC_TYPE_OPTION,
            symbol=underlying.symbol,
            exchange="SMART",
            currency="USD",
            expiry=expiry,
            strike=strike,
            right=api.RIGHT_PUT,
            multiplier=chain.multiplier,
            trading_class="SPY",
        ),
    )
    replies = main_client.snapshot(api.QUOTE_KIND_OPTION_EOD, [option])
    assert len(replies) == 1
    status = replies[0].quote.status
    record_property("quote_status", api.QuoteStatus.Name(status))
    assert status in (api.QUOTE_STATUS_OK, api.QUOTE_STATUS_PARTIAL)


def test_live_dividends_aapl(main_client: LiveClient) -> None:
    aapl = _resolved(main_client, _stock("AAPL"))
    key = api.ContractKey(
        symbol="AAPL",
        sec_type=api.SEC_TYPE_STOCK,
        exchange="SMART",
        currency="USD",
        con_id=aapl.con_id,
    )
    replies = main_client.dividends([key])
    assert len(replies) == 1
    assert replies[0].status == api.DIVIDEND_STATUS_OK
    assert replies[0].HasField("next_ex_date")
    assert replies[0].next_ex_date


def test_live_wife_market_data_unimplemented(wife_client: LiveClient) -> None:
    spec = api.ContractSpec(
        sec_type=api.SEC_TYPE_STOCK,
        symbol="SPY",
        exchange="SMART",
        currency="USD",
        con_id=1,
    )
    code = _code(lambda: wife_client.snapshot(api.QUOTE_KIND_UNDERLYING_IV, [spec]))
    assert code == grpc.StatusCode.UNIMPLEMENTED


def test_live_bars_not_implemented(main_client: LiveClient) -> None:
    request = api.BarsRequest(
        contract=api.ContractSpec(
            sec_type=api.SEC_TYPE_STOCK,
            symbol="SPY",
            exchange="SMART",
            currency="USD",
            con_id=1,
        ),
        kind=api.BAR_KIND_TRADES,
        duration_days=5,
    )
    assert _code(lambda: main_client.bars(request)) == grpc.StatusCode.UNIMPLEMENTED
