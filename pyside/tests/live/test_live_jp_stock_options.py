"""Read-only live probe of the option chains of two JP stocks on TSEJ (7974 listed, 278A not)."""

from __future__ import annotations

from collections.abc import Callable
from datetime import date, timedelta

from live.client import LiveClient
from optdesk.contracts import api

LISTED = "7974"
UNLISTED = "278A"
CHAIN_EXCHANGE = "OSE.JPN"
MIN_DAYS_OUT = 14
FIRST_EXPIRATIONS = 4

Record = Callable[[str, object], None]


def _stock(symbol: str) -> api.ContractSpec:
    return api.ContractSpec(
        sec_type=api.SEC_TYPE_STOCK, symbol=symbol, exchange="TSEJ", currency="JPY"
    )


def _resolved(client: LiveClient, symbol: str) -> api.ContractSpec:
    reply = client.resolve_contract(_stock(symbol))
    assert reply.status == api.RESOLVE_STATUS_FOUND
    assert reply.contract.con_id > 0
    return reply.contract


def _jp_chain(client: LiveClient, symbol: str) -> api.Chain | None:
    underlying = _resolved(client, symbol)
    reply = client.list_chains(underlying)
    return next(
        (
            c
            for c in reply.chains
            if c.exchange == CHAIN_EXCHANGE and c.trading_class == symbol
        ),
        None,
    )


def _stock_price(client: LiveClient, symbol: str) -> float:
    replies = client.snapshot(api.QUOTE_KIND_SNAPSHOT, [_resolved(client, symbol)])
    assert len(replies) == 1
    quote = replies[0].quote
    price = quote.last if quote.HasField("last") else quote.close
    assert price > 0, f"no last or close in {api.QuoteStatus.Name(quote.status)}"
    return price


def _nearest_strike(chain: api.Chain, price: float) -> str:
    assert chain.strikes
    return min(chain.strikes, key=lambda s: abs(float(s) - price))


def _listed_chain(client: LiveClient) -> api.Chain:
    chain = _jp_chain(client, LISTED)
    assert chain is not None, f"no chain of {CHAIN_EXCHANGE} and class {LISTED}"
    return chain


def test_live_jp_stock_chain(main_client: LiveClient, record_property: Record) -> None:
    chain = _listed_chain(main_client)
    assert chain.exchange == CHAIN_EXCHANGE
    assert chain.trading_class == "7974"
    assert chain.multiplier == "100"
    price = _stock_price(main_client, LISTED)
    record_property("multiplier", chain.multiplier)
    record_property("first_expirations", sorted(chain.expirations)[:FIRST_EXPIRATIONS])
    record_property("strike_text_nearest_price", _nearest_strike(chain, price))
    assert chain.expirations


def test_live_jp_no_options_278a(main_client: LiveClient) -> None:
    assert _jp_chain(main_client, UNLISTED) is None


def _option_spec(chain: api.Chain, strike: str, expiry: str) -> api.ContractSpec:
    return api.ContractSpec(
        sec_type=api.SEC_TYPE_OPTION,
        symbol=LISTED,
        exchange=CHAIN_EXCHANGE,
        currency="JPY",
        expiry=expiry,
        strike=strike,
        right=api.RIGHT_CALL,
        multiplier=chain.multiplier,
        trading_class=LISTED,
    )


def _resolved_option(client: LiveClient) -> api.ContractSpec:
    chain = _listed_chain(client)
    price = _stock_price(client, LISTED)
    earliest = (date.today() + timedelta(days=MIN_DAYS_OUT)).strftime("%Y%m%d")
    expiries = sorted(chain.expirations)
    expiry = next((e for e in expiries if e >= earliest), expiries[0])
    reply = client.resolve_contract(
        _option_spec(chain, _nearest_strike(chain, price), expiry)
    )
    assert reply.status == api.RESOLVE_STATUS_FOUND
    assert reply.contract.con_id > 0
    return reply.contract


def test_live_jp_option_resolve(
    main_client: LiveClient, record_property: Record
) -> None:
    option = _resolved_option(main_client)
    record_property("option_symbol", option.symbol)
    record_property("option_strike", option.strike)
    assert option.symbol == LISTED


def test_live_jp_thin_option_row(
    main_client: LiveClient, record_property: Record
) -> None:
    option = _resolved_option(main_client)
    replies = main_client.snapshot(api.QUOTE_KIND_OPTION_EOD, [option])
    assert len(replies) == 1
    quote = replies[0].quote
    record_property("quote_status", api.QuoteStatus.Name(quote.status))
    for name in ("bid", "ask", "close", "volume", "open_interest"):
        record_property(name, getattr(quote, name) if quote.HasField(name) else None)
    assert quote.status in (
        api.QUOTE_STATUS_OK,
        api.QUOTE_STATUS_PARTIAL,
        api.QUOTE_STATUS_NO_DATA,
    )
