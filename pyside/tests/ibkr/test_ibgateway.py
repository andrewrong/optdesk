"""IbGateway over a fake ib: only the async methods it calls, a FakeClock, synthetic values."""

from __future__ import annotations

import asyncio
import datetime
import logging
from collections.abc import Awaitable, Callable
from typing import Any, Self

import pytest
from ib_async import (
    Contract,
    ContractDetails,
    Dividends,
    OptionChain,
    RequestError,
    Ticker,
)
from kit.fakeclock import FakeClock

from optdesk.ibkr.api import DividendKey, DividendReading
from optdesk.ibkr.ibgateway import IbGateway
from optdesk.ibkr.model import QuoteRow, QuoteTarget
from optdesk.ibkr.spec import AmbiguousResolve, BrokerError
from optdesk.ibkr.transport import OrderRefused


async def probe_passes() -> None:
    """A read-only probe that passes, in place of the second connection."""


class FakeEvent:
    def __init__(self) -> None:
        self.handlers: list[Callable[..., None]] = []

    def __iadd__(self, handler: Callable[..., None]) -> Self:
        self.handlers.append(handler)
        return self

    def __isub__(self, handler: Callable[..., None]) -> Self:
        self.handlers.remove(handler)
        return self

    def emit(self, *args: object) -> None:
        for handler in list(self.handlers):
            handler(*args)


class FakeClient:
    def __init__(self, ib: FakeIb) -> None:
        self.ib = ib
        self.apiEnd = FakeEvent()

    async def connectAsync(self, host: str, port: int, client_id: int) -> None:
        self.ib.log.append("connect")
        self.ib.connect_args = (host, port, client_id)


class FakeIb:
    """Has only what IbGateway calls; every send is stamped with the fake time."""

    def __init__(self, clock: FakeClock, log: list[str] | None = None) -> None:
        self.clock = clock
        self.log: list[str] = log if log is not None else []
        self.client = FakeClient(self)
        self.errorEvent = FakeEvent()
        self.RaiseRequestErrors = False
        self.connect_args: tuple[str, int, int] | None = None
        self.sends: list[float] = []
        self.market_data_types: list[int] = []
        self.details: Callable[[Contract], Awaitable[list[ContractDetails]]] = (
            self._none
        )
        self.chain_rows: list[OptionChain] = []
        self.autofill = False
        self.refuse = False
        self.fail: Exception | None = None
        self.tickers: dict[int, Ticker] = {}
        self.dividends: Dividends | None = None
        self.mkt_contracts: list[Contract] = []
        self.cancel_contracts: list[Contract] = []
        self.subscribed: list[int] = []
        self.cancelled: list[int] = []
        self.disconnected = False
        self.wrapper_accounts = ["DU" + "7654321"]  # held by the library, never read

    async def _none(self, contract: Contract) -> list[ContractDetails]:
        return []

    def _stamp(self) -> None:
        self.sends.append(self.clock.monotonic())

    def reqMarketDataType(self, value: int) -> None:
        self._stamp()
        self.log.append("market_data_type")
        self.market_data_types.append(value)

    def reqContractDetailsAsync(
        self, contract: Contract
    ) -> Awaitable[list[ContractDetails]]:
        self._stamp()
        return self.details(contract)

    def reqSecDefOptParamsAsync(
        self, symbol: str, exchange: str, sec_type: str, con_id: int
    ) -> Awaitable[list[OptionChain]]:
        self._stamp()

        async def answer() -> list[OptionChain]:
            return self.chain_rows

        return answer()

    def reqMktData(
        self, contract: Contract, ticks: str, snap: bool, regulatory: bool
    ) -> Ticker:
        self._stamp()
        hash(contract)  # ib_async hashes it; a conId of 0 raises ValueError
        self.mkt_contracts.append(contract)
        ticker = Ticker(contract=contract)
        ticker.dividends = self.dividends
        self.tickers[contract.conId] = ticker
        self.subscribed.append(contract.conId)
        if self.fail is not None:
            raise self.fail
        if self.refuse:
            raise OrderRefused("refused as the guard would")
        if self.autofill:
            fill(ticker)
        return ticker

    def cancelMktData(self, contract: Contract) -> bool:
        self._stamp()
        self.cancelled.append(contract.conId)
        self.cancel_contracts.append(contract)
        return True

    def disconnect(self) -> None:
        self.disconnected = True


def fill(ticker: Ticker, **values: float) -> None:
    for name, value in (values or {"bid": 1.0, "ask": 1.1}).items():
        setattr(ticker, name, value)


async def spin(turns: int = 10) -> None:
    for _ in range(turns):
        await asyncio.sleep(0)


def make(
    clock: FakeClock, ib: FakeIb | None = None, **options: Any
) -> tuple[IbGateway, FakeIb]:
    fake = ib or FakeIb(clock)
    order = fake.log

    def guard(client: object) -> None:
        order.append("guard")

    gateway = IbGateway(
        guard,
        host="127.0.0.1",
        port=4002,
        client_id=33,
        clock=clock,
        ib_factory=lambda: fake,
        readonly_probe=probe_passes,
        **options,
    )
    return gateway, fake


async def opened(clock: FakeClock, **options: Any) -> tuple[IbGateway, FakeIb]:
    gateway, fake = make(clock, **options)
    await gateway.open_session()
    return gateway, fake


def details_of(con_id: int) -> ContractDetails:
    return ContractDetails(
        contract=Contract(
            conId=con_id, exchange="SMART", tradingClass="SPY", multiplier="100"
        )
    )


def test_gateway_requires_guard() -> None:
    constructor: Any = IbGateway
    with pytest.raises(TypeError):
        constructor(host="127.0.0.1", port=4002, client_id=33)


def test_guard_installed_before_connect() -> None:
    async def run() -> tuple[list[str], tuple[str, int, int] | None]:
        _, fake = await opened(FakeClock())
        return fake.log, fake.connect_args

    log, args = asyncio.run(run())
    assert log == ["guard", "connect", "market_data_type"]
    assert args == ("127.0.0.1", 4002, 33)


def test_market_data_type_set_once_per_session() -> None:
    async def run() -> tuple[list[int], list[int], int]:
        clock = FakeClock()
        first = FakeIb(clock)
        second = FakeIb(clock)
        ibs = [first, second]
        gateway = IbGateway(
            lambda client: None,
            host="h",
            port=1,
            client_id=33,
            market_data_type=3,
            clock=clock,
            ib_factory=lambda: ibs.pop(0),
            readonly_probe=probe_passes,
        )
        await gateway.open_session()
        await gateway.open_session()
        state = await gateway.status()
        return first.market_data_types, second.market_data_types, state.market_data_type

    assert asyncio.run(run()) == ([3], [3], 3)


def test_market_data_type_defaults_to_frozen() -> None:
    async def run() -> tuple[list[int], list[int], int]:
        clock = FakeClock()
        first = FakeIb(clock)
        second = FakeIb(clock)
        ibs = [first, second]
        gateway = IbGateway(
            lambda client: None,
            host="h",
            port=1,
            client_id=33,
            clock=clock,
            ib_factory=lambda: ibs.pop(0),
            readonly_probe=probe_passes,
        )
        await gateway.open_session()
        await gateway.open_session()
        state = await gateway.status()
        return first.market_data_types, second.market_data_types, state.market_data_type

    assert asyncio.run(run()) == ([2], [2], 2)


def test_generation_increments_on_reconnect() -> None:
    async def run() -> list[tuple[bool, int]]:
        clock = FakeClock()
        ibs = [FakeIb(clock), FakeIb(clock)]
        gateway = IbGateway(
            lambda client: None,
            host="h",
            port=1,
            client_id=33,
            clock=clock,
            ib_factory=lambda: ibs.pop(0),
            readonly_probe=probe_passes,
        )
        seen: list[tuple[bool, int]] = []
        for step in range(3):
            if step:
                await gateway.open_session()
            state = await gateway.status()
            seen.append((state.connected, state.generation))
        return seen

    assert asyncio.run(run()) == [(False, 0), (True, 1), (True, 2)]


def test_account_never_leaves_gateway(caplog: pytest.LogCaptureFixture) -> None:
    async def run() -> list[object]:
        clock = FakeClock()
        gateway, fake = await opened(clock)
        fake.details = lambda contract: _found(5)
        fake.chain_rows = [OptionChain("SMART", 1, "SPY", "100", ["20260116"], [100.0])]
        fake.autofill = True
        out: list[object] = [await gateway.status()]
        out.append(await gateway.chains(sec_type="STK", symbol="SPY", con_id=1))
        task = asyncio.ensure_future(gateway.resolve(**_spec()))
        await spin()
        out.append(await task)
        rows: list[QuoteRow] = []

        async def keep(row: QuoteRow) -> None:
            rows.append(row)

        await gateway.quotes("snapshot", [QuoteTarget(5)], keep)
        out.extend(rows)
        return out

    with caplog.at_level(logging.DEBUG):
        results = asyncio.run(run())
    shape = "DU" + "7654321"
    assert shape not in repr(results)
    assert [r for r in caplog.records if r.name != "asyncio"] == []


async def _found(con_id: int) -> list[ContractDetails]:
    return [details_of(con_id)]


def _spec() -> dict[str, str]:
    return {
        "sec_type": "STK",
        "symbol": "SPY",
        "exchange": "SMART",
        "currency": "USD",
        "expiry": "",
        "strike": "",
        "right": "",
        "multiplier": "",
        "trading_class": "",
    }


def test_chains_maps_option_chain() -> None:
    async def run() -> Any:
        gateway, fake = await opened(FakeClock())
        fake.chain_rows = [
            OptionChain(
                "SMART", 1, "SPY", "100", ["20260123", "20260116"], [102.5, 95.0, 100.0]
            )
        ]
        return await gateway.chains(sec_type="STK", symbol="SPY", con_id=1)

    (row,) = asyncio.run(run())
    assert row.exchange == "SMART" and row.trading_class == "SPY"
    assert row.strikes == ("95", "100", "102.5")
    assert row.expirations == ("20260116", "20260123")


def test_resolve_found_not_listed() -> None:
    async def run() -> tuple[Any, Any, Any]:
        clock = FakeClock()
        gateway, fake = await opened(clock)
        fake.details = lambda contract: _found(756733)
        found = await gateway.resolve(**_spec())

        async def listed_never(contract: Contract) -> list[ContractDetails]:
            raise RequestError(1, 200, "No security definition")

        fake.details = listed_never
        missing = await gateway.resolve(**_spec())
        fake.details = fake._none
        empty = await gateway.resolve(**_spec())
        return found, missing, empty

    found, missing, empty = asyncio.run(run())
    assert found.found and found.con_id == 756733 and found.exchange == "SMART"
    assert not missing.found and missing.con_id == 0 and missing.error_code == 200
    assert not empty.found and empty.error_code == 0


def test_resolve_other_broker_error_is_raised() -> None:
    async def run() -> None:
        gateway, fake = await opened(FakeClock())

        async def failing(contract: Contract) -> list[ContractDetails]:
            raise RequestError(1, 162, "pacing")

        fake.details = failing
        await gateway.resolve(**_spec())

    with pytest.raises(BrokerError):
        asyncio.run(run())


def test_resolve_several_results_is_error() -> None:
    async def run() -> None:
        gateway, fake = await opened(FakeClock())

        async def two(contract: Contract) -> list[ContractDetails]:
            return [details_of(1), details_of(2)]

        fake.details = two
        await gateway.resolve(**_spec())

    with pytest.raises(AmbiguousResolve):
        asyncio.run(run())


def test_resolve_timeout_is_error_not_negative() -> None:
    async def run() -> str:
        clock = FakeClock()
        gateway, fake = await opened(clock)

        async def never(contract: Contract) -> list[ContractDetails]:
            await asyncio.Event().wait()
            return []

        fake.details = never
        task = asyncio.ensure_future(gateway.resolve(**_spec()))
        await spin()
        await clock.advance(20)
        try:
            await task
        except TimeoutError:
            return "timeout"
        return "returned"

    assert asyncio.run(run()) == "timeout"


def test_resolve_goes_through_lane() -> None:
    async def run() -> tuple[int, int]:
        clock = FakeClock()
        gateway, fake = await opened(clock)
        running = 0
        peak = 0
        release = asyncio.Event()

        async def slow(contract: Contract) -> list[ContractDetails]:
            nonlocal running, peak
            running += 1
            peak = max(peak, running)
            await release.wait()
            running -= 1
            return [details_of(1)]

        fake.details = slow
        tasks = [asyncio.ensure_future(gateway.resolve(**_spec())) for _ in range(6)]
        await spin()
        at_once = running
        release.set()
        await asyncio.gather(*tasks)
        return at_once, peak

    assert asyncio.run(run()) == (2, 2)


def test_resolve_while_disconnected_is_connection_error() -> None:
    async def run() -> None:
        gateway, _ = make(FakeClock())
        await gateway.resolve(**_spec())

    with pytest.raises(ConnectionError):
        asyncio.run(run())


async def collect(
    gateway: IbGateway, kind: str, con_ids: list[int]
) -> tuple[list[QuoteRow], asyncio.Future[None]]:
    rows: list[QuoteRow] = []

    async def keep(row: QuoteRow) -> None:
        rows.append(row)

    return rows, asyncio.ensure_future(
        gateway.quotes(kind, [QuoteTarget(c) for c in con_ids], keep)
    )


def test_quotes_send_exchange() -> None:
    async def run(kind: str) -> list[str]:
        clock = FakeClock()
        gateway, fake = await opened(clock, lines=1)
        fake.autofill = True
        _, task = await collect(gateway, kind, [61])
        await spin(50)
        await clock.advance(21)
        await task
        contract = fake.tickers[61].contract
        assert contract is not None
        return [contract.exchange]

    for kind in ("snapshot", "option_eod"):
        assert asyncio.run(run(kind)) == ["SMART"]


def _sent_contract(targets: list[QuoteTarget]) -> Contract:
    async def run() -> Contract:
        clock = FakeClock()
        gateway, fake = await opened(clock, lines=1)
        fake.autofill = True
        rows: list[QuoteRow] = []

        async def keep(row: QuoteRow) -> None:
            rows.append(row)

        task = asyncio.ensure_future(gateway.quotes("snapshot", targets, keep))
        await spin(50)
        await clock.advance(21)
        await task
        contract = fake.tickers[targets[0].con_id].contract
        assert contract is not None
        return contract

    return asyncio.run(run())


def test_quotes_forward_exchange() -> None:
    jp = _sent_contract([QuoteTarget(7, "OSE.JPN", "JPY", "225")])
    assert (jp.conId, jp.exchange, jp.currency, jp.tradingClass) == (
        7,
        "OSE.JPN",
        "JPY",
        "225",
    )
    opt = _sent_contract([QuoteTarget(8, "OSE.JPN", "JPY", "225", "OPT")])
    assert opt.secType == "OPT"


def test_quotes_default_smart() -> None:
    contract = _sent_contract([QuoteTarget(5)])
    assert contract.conId == 5 and contract.exchange == "SMART"
    assert (contract.currency, contract.tradingClass, contract.secType) == ("", "", "")


def test_quotes_contract_keeps_con_id() -> None:
    async def run() -> list[int]:
        clock = FakeClock()
        gateway, fake = await opened(clock, lines=1)
        fake.autofill = True
        _, task = await collect(gateway, "snapshot", [71, 72])
        await spin(50)
        await clock.advance(1)
        await task
        return sorted(fake.tickers)

    assert asyncio.run(run()) == [71, 72]


def test_quotes_release_line_on_early_complete() -> None:
    async def run() -> tuple[list[str], list[int]]:
        clock = FakeClock()
        gateway, fake = await opened(clock, lines=1)
        fake.autofill = True
        rows, task = await collect(gateway, "snapshot", [11, 12, 13])
        await spin(50)
        await clock.advance(1)
        await task
        return [r.status for r in rows], fake.cancelled

    statuses, cancelled = asyncio.run(run())
    assert statuses == ["ok", "ok", "ok"]
    assert sorted(cancelled) == [11, 12, 13]


def test_quotes_release_line_when_send_is_refused() -> None:
    async def run() -> tuple[type[BaseException] | None, list[str], list[int]]:
        clock = FakeClock()
        gateway, fake = await opened(clock, lines=1)
        fake.refuse = True
        raised: type[BaseException] | None = None
        try:
            await gateway.quotes("snapshot", [QuoteTarget(21)], _drop)
        except OrderRefused as exc:
            raised = type(exc)
        await spin()
        fake.refuse = False
        fake.autofill = True
        rows, task = await collect(gateway, "snapshot", [22])
        await spin(50)
        await task
        return raised, [r.status for r in rows], fake.cancelled

    raised, statuses, cancelled = asyncio.run(run())
    assert raised is OrderRefused
    assert statuses == ["ok"]  # the single line was returned
    assert 21 in cancelled


async def _drop(row: QuoteRow) -> None:
    return None


def test_quotes_timeout_is_partial_or_no_data() -> None:
    async def run() -> dict[int, tuple[str, tuple[tuple[str, float], ...]]]:
        clock = FakeClock()
        gateway, fake = await opened(clock)
        rows, task = await collect(gateway, "snapshot", [31, 32])
        await spin()
        fill(fake.tickers[31], bid=1.0)
        fake.tickers[31].updateEvent.emit(fake.tickers[31])
        await clock.advance(20)
        await task
        return {r.con_id: (r.status, r.fields) for r in rows}

    got = asyncio.run(run())
    assert got[31] == ("partial", (("bid", 1.0),))
    assert got[32] == ("no_data", ())


def test_quotes_error_codes_set_status() -> None:
    async def run() -> dict[int, tuple[str, int]]:
        clock = FakeClock()
        gateway, fake = await opened(clock)
        rows, task = await collect(gateway, "snapshot", [41, 42])
        await spin()
        fake.errorEvent.emit(1, 354, "no subscription", Contract(conId=41))
        fake.errorEvent.emit(2, 2104, "farm ok", Contract(conId=42))  # not an error
        fake.errorEvent.emit(3, 162, "other", Contract(conId=42))
        await task
        return {r.con_id: (r.status, r.error_code) for r in rows}

    assert asyncio.run(run()) == {41: ("no_subscription", 354), 42: ("error", 162)}


def test_quotes_cap_fifty_in_flight() -> None:
    async def run() -> tuple[int, int]:
        clock = FakeClock()
        gateway, fake = await opened(clock)
        rows, task = await collect(gateway, "snapshot", list(range(1, 61)))
        await spin(50)
        await clock.advance(1)
        in_flight = len(fake.subscribed)
        fake.autofill = True
        for con_id in list(fake.subscribed):
            fill(fake.tickers[con_id])
            fake.tickers[con_id].updateEvent.emit(fake.tickers[con_id])
        for _ in range(10):
            await clock.advance(1)
        await task
        return in_flight, len(rows)

    assert asyncio.run(run()) == (50, 60)


def test_stop_checked_after_pacer_wait() -> None:
    async def run() -> tuple[int, int, set[str]]:
        clock = FakeClock()
        gateway, fake = await opened(clock)  # one send used by the type
        rows, task = await collect(gateway, "snapshot", list(range(1, 42)))
        await spin(50)
        sent = len(fake.subscribed)  # 39: the pacer window is full
        fake.client.apiEnd.emit()
        for _ in range(5):
            await clock.advance(1)
        await task
        return sent, len(fake.subscribed), {r.status for r in rows}

    sent, after, statuses = asyncio.run(run())
    assert sent == 39 and after == 39
    assert statuses == {"disconnected"}


def test_every_send_goes_through_pacer() -> None:
    async def run() -> list[float]:
        clock = FakeClock()
        gateway, fake = await opened(clock)
        fake.autofill = True
        _rows, task = await collect(gateway, "snapshot", list(range(1, 101)))
        for _ in range(12):
            await spin(20)
            await clock.advance(1)
        await task
        await clock.advance(5)
        assert len(fake.cancelled) == 100
        return fake.sends

    sends = asyncio.run(run())
    assert len(sends) == 201
    assert all(sends[i + 40] - sends[i] >= 1.0 for i in range(len(sends) - 40))


def test_disconnect_marks_unanswered_disconnected() -> None:
    async def run() -> tuple[dict[int, str], bool]:
        clock = FakeClock()
        gateway, fake = await opened(clock)
        rows, task = await collect(gateway, "snapshot", [51, 52])
        await spin()
        fill(fake.tickers[51])
        fake.tickers[51].updateEvent.emit(fake.tickers[51])
        await spin()
        fake.client.apiEnd.emit()
        await task
        state = await gateway.status()
        return {r.con_id: r.status for r in rows}, state.connected

    statuses, connected = asyncio.run(run())
    assert statuses == {51: "ok", 52: "disconnected"}
    assert not connected


def test_quote_rows_carry_generation_and_times() -> None:
    async def run() -> QuoteRow:
        clock = FakeClock()
        gateway, fake = await opened(clock)
        fake.autofill = True
        rows, task = await collect(gateway, "snapshot", [61])
        await spin(20)
        await task
        return rows[0]

    row = asyncio.run(run())
    assert row.generation == 1 and row.con_id == 61
    assert row.requested_at <= row.sent_at <= row.received_at
    assert dict(row.fields)["bid"] == 1.0


def _dividend_keys() -> list[DividendKey]:
    return [DividendKey("AAPL", "STK", "SMART", "USD", c) for c in (71, 72)]


def test_dividends_unexpected_error_is_error_status_per_key(
    caplog: pytest.LogCaptureFixture,
) -> None:
    async def run() -> list[DividendReading]:
        gateway, fake = await opened(FakeClock(), lines=2)
        fake.fail = ValueError("bad tick")
        readings: list[DividendReading] = []

        async def keep(reading: DividendReading) -> None:
            readings.append(reading)

        await gateway.dividends(_dividend_keys(), keep)
        return readings

    with caplog.at_level(logging.ERROR):
        readings = asyncio.run(run())
    assert sorted(r.key.con_id for r in readings) == [71, 72]
    assert {(r.status, r.error_code) for r in readings} == {("error", 0)}
    assert {r.past_12m for r in readings} == {None}
    failed = [
        r.getMessage() for r in caplog.records if "dividend_failed" in r.getMessage()
    ]
    assert len(failed) == 2
    assert all("ValueError" in m and "bad tick" in m for m in failed)
    assert all("DU" not in m and "71" not in m and "72" not in m for m in failed)


def test_dividends_connection_error_still_raises() -> None:
    async def run() -> None:
        gateway, _ = make(FakeClock())

        async def keep(reading: DividendReading) -> None:
            raise AssertionError("no reading expected")

        await gateway.dividends(_dividend_keys(), keep)

    with pytest.raises(ConnectionError):
        asyncio.run(run())


def _zero_key() -> DividendKey:
    return DividendKey("AAPL", "STK", "SMART", "USD", 0)


def _detail(con_id: int) -> ContractDetails:
    return ContractDetails(contract=Contract(conId=con_id, symbol="AAPL"))


async def _read_zero(
    details: list[ContractDetails], fake_error: Exception | None = None
) -> tuple[list[DividendReading], FakeIb]:
    gateway, fake = await opened(FakeClock(), lines=2)
    fake.fail = fake_error
    fake.dividends = Dividends(1.0, 2.0, datetime.date(2026, 12, 1), 0.25)

    async def found(contract: Contract) -> list[ContractDetails]:
        return details

    fake.details = found
    readings: list[DividendReading] = []

    async def keep(reading: DividendReading) -> None:
        readings.append(reading)

    await gateway.dividends([_zero_key()], keep)
    return readings, fake


def test_dividends_con_id_zero_key_is_resolved_and_read() -> None:
    readings, fake = asyncio.run(_read_zero([_detail(265598)]))
    assert [(r.key, r.status) for r in readings] == [(_zero_key(), "ok")]
    assert fake.subscribed == [265598]
    assert fake.cancelled == [265598]
    assert fake.mkt_contracts == fake.cancel_contracts
    assert fake.mkt_contracts[0] is fake.cancel_contracts[0]


def test_dividends_con_id_zero_no_match_is_no_value() -> None:
    readings, fake = asyncio.run(_read_zero([]))
    assert [(r.key, r.status, r.error_code) for r in readings] == [
        (_zero_key(), "no_value", 0)
    ]
    assert fake.subscribed == []


def test_dividends_con_id_zero_ambiguous_is_error_status() -> None:
    readings, fake = asyncio.run(_read_zero([_detail(1), _detail(2)]))
    assert [(r.key, r.status, r.error_code) for r in readings] == [
        (_zero_key(), "error", 0)
    ]
    assert fake.subscribed == []


def test_dividends_failed_log_message_is_cut(
    caplog: pytest.LogCaptureFixture,
) -> None:
    with caplog.at_level(logging.ERROR):
        readings, _ = asyncio.run(_read_zero([_detail(5)], ValueError("x" * 300)))
    assert [r.status for r in readings] == ["error"]
    failed = [
        r.getMessage() for r in caplog.records if "dividend_failed" in r.getMessage()
    ]
    assert len(failed) == 1
    assert len(failed[0].split("message=", 1)[1]) <= 120
