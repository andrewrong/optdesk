"""Read-only live tests that assert fresh data: the deployed quotes and bars are of the latest session."""

from __future__ import annotations

from collections.abc import Callable
from datetime import UTC, date, datetime, timedelta
from typing import Any

import exchange_calendars

from live.client import LiveClient
from optdesk.contracts import api

Record = Callable[[str, object], None]

AFTER_HOURS = timedelta(hours=4)
BAR_DAYS = 10
MIN_DAYS_OUT = 14
CALENDAR_YEARS = 2


def _calendar(calendar_id: str) -> exchange_calendars.ExchangeCalendar:
    today = date.today()
    return exchange_calendars.get_calendar(
        calendar_id,
        start=(today - timedelta(days=365 * CALENDAR_YEARS)).isoformat(),
        end=(today + timedelta(days=30)).isoformat(),
    )


def _latest_session(
    calendar: exchange_calendars.ExchangeCalendar, now: datetime
) -> Any:
    """The latest session that has opened at `now`."""
    return calendar.minute_to_session(now, direction="previous")


def _stock(symbol: str) -> api.ContractSpec:
    return api.ContractSpec(
        sec_type=api.SEC_TYPE_STOCK, symbol=symbol, exchange="SMART", currency="USD"
    )


def _n225() -> api.ContractSpec:
    return api.ContractSpec(
        sec_type=api.SEC_TYPE_INDEX, symbol="N225", exchange="OSE.JPN", currency="JPY"
    )


def _resolved(client: LiveClient, spec: api.ContractSpec) -> api.ContractSpec:
    reply = client.resolve_contract(spec)
    assert reply.status == api.RESOLVE_STATUS_FOUND, f"{spec.symbol} not resolved"
    assert reply.contract.con_id > 0
    return reply.contract


def _one_quote(
    client: LiveClient, kind: api.QuoteKind.ValueType, spec: api.ContractSpec
) -> api.Quote:
    replies = client.snapshot(kind, [spec])
    assert len(replies) == 1
    return replies[0].quote


def _last_bar_day(client: LiveClient, spec: api.ContractSpec) -> date:
    bars = client.recent_bars(_resolved(client, spec), BAR_DAYS)
    assert bars, f"no bars of {spec.symbol}"
    return date.fromisoformat(bars[-1].day)


def _assert_bars_end_on_latest_day(calendar_id: str, last: date, now: datetime) -> None:
    calendar = _calendar(calendar_id)
    started = _latest_session(calendar, now).date()
    if calendar.is_trading_minute(now):
        # The session of today is still running: a daily bar of it may be missing.
        completed = calendar.minute_to_session(now, direction="previous")
        completed = calendar.previous_session(completed)
        floor = completed.date()
    else:
        floor = started
    assert floor <= last <= started, (
        f"{calendar_id}: last bar {last}, latest trading day {started}"
    )


def test_live_quote_in_session(
    main_client: LiveClient, record_property: Record
) -> None:
    now = datetime.now(UTC)
    spy = _resolved(main_client, _stock("SPY"))
    quote = _one_quote(main_client, api.QUOTE_KIND_SNAPSHOT, spy)
    stamp = quote.event_ts if quote.HasField("event_ts") else quote.received_at
    moment = stamp.ToDatetime(tzinfo=UTC)
    calendar = _calendar("XNYS")
    session = _latest_session(calendar, now)
    opened = calendar.session_open(session).to_pydatetime()
    closed = calendar.session_close(session).to_pydatetime()
    record_property("as_of", session.date().isoformat())
    assert opened <= moment <= closed + AFTER_HOURS, (
        f"SPY quote at {moment.isoformat()} is outside the session "
        f"{opened.isoformat()}..{closed.isoformat()} and its after-hours window"
    )


def test_live_bars_latest_day(main_client: LiveClient, record_property: Record) -> None:
    now = datetime.now(UTC)
    spy_last = _last_bar_day(main_client, _stock("SPY"))
    n225_last = _last_bar_day(main_client, _n225())
    record_property("as_of", max(spy_last, n225_last).isoformat())
    _assert_bars_end_on_latest_day("XNYS", spy_last, now)
    _assert_bars_end_on_latest_day("XTKS", n225_last, now)


def test_live_n225_no_sentinel(
    main_client: LiveClient, record_property: Record
) -> None:
    n225 = _resolved(main_client, _n225())
    quote = _one_quote(main_client, api.QUOTE_KIND_SNAPSHOT, n225)
    record_property("as_of", datetime.now(UTC).date().isoformat())
    for name in ("bid", "ask"):
        if quote.HasField(name):
            assert getattr(quote, name) != -1, f"N225 {name} is the sentinel -1"


def test_live_oi_present(main_client: LiveClient, record_property: Record) -> None:
    underlying = _resolved(main_client, _stock("SPY"))
    chain = next(
        c
        for c in main_client.list_chains(underlying).chains
        if c.trading_class == "SPY"
    )
    earliest = (date.today() + timedelta(days=MIN_DAYS_OUT)).strftime("%Y%m%d")
    expiry = next(e for e in sorted(chain.expirations) if e >= earliest)
    strikes = sorted(chain.strikes, key=float)
    option = _resolved(
        main_client,
        api.ContractSpec(
            sec_type=api.SEC_TYPE_OPTION,
            symbol=underlying.symbol,
            exchange="SMART",
            currency="USD",
            expiry=expiry,
            strike=strikes[len(strikes) // 2],
            right=api.RIGHT_PUT,
            multiplier=chain.multiplier,
            trading_class="SPY",
        ),
    )
    quote = _one_quote(main_client, api.QUOTE_KIND_OPTION_EOD, option)
    record_property("as_of", datetime.now(UTC).date().isoformat())
    assert quote.HasField("open_interest"), (
        f"no open interest in {api.QuoteStatus.Name(quote.status)}"
    )
    assert quote.open_interest >= 0
