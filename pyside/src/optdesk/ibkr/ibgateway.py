"""The adapter over ib_async: IbGateway implements Gateway.

The only file of src that imports ib_async. The connection is made on the
client after the guard is installed, with ib_async's async methods only; every
send waits for the message pacer first. The objects of ib_async are named `ib`
and `ib.client` and nothing else of ours is. This file returns statuses and
raises errors, and the transport turns them into records; its one log record is
`dividend_failed`, written through the stdlib logger of this module when a key
fails with an unexpected exception (type name and message only).
"""

from __future__ import annotations

import asyncio
import logging
import math
from collections.abc import Awaitable, Callable, Sequence
from decimal import Decimal
from typing import Any

from ib_async import IB, Contract, RequestError

from optdesk.ibkr import spec
from optdesk.ibkr.accounts import read_account, read_executions
from optdesk.ibkr.api import DividendKey, DividendReading
from optdesk.ibkr.dividends import Release, TickError, TickText, read_dividend
from optdesk.ibkr.model import (
    AccountRows,
    ChainRow,
    ExecutionRows,
    QuoteRow,
    QuoteTarget,
    ResolveResult,
    SessionState,
)
from optdesk.ibkr.pacing import (
    DEFAULT_LINES,
    DEFAULT_SNAPSHOTS,
    DetailsLane,
    LineBudget,
    MessagePacer,
)
from optdesk.ibkr.timing import Clock, SystemClock, with_timeout

_log = logging.getLogger(__name__)

_DIVIDEND_LOG_MAX = 120  # characters of an exception text kept in dividend_failed

_PRICE_FIELDS = (
    ("bid", "bid"),
    ("ask", "ask"),
    ("last", "last"),
    ("close", "close"),
    ("bid_size", "bidSize"),
    ("ask_size", "askSize"),
    ("last_size", "lastSize"),
    ("option_implied_vol", "impliedVolatility"),
    ("historical_vol", "histVolatility"),
)
_GREEK_FIELDS = (
    ("implied_vol", "impliedVol"),
    ("delta", "delta"),
    ("gamma", "gamma"),
    ("vega", "vega"),
    ("theta", "theta"),
    ("und_price", "undPrice"),
)


def _number(value: object) -> float | None:
    if isinstance(value, bool) or not isinstance(value, int | float):
        return None
    return None if math.isnan(value) else float(value)


def _first(ticker: Any, *names: str) -> float | None:
    for name in names:
        number = _number(getattr(ticker, name, None))
        if number is not None:
            return number
    return None


def ticker_fields(ticker: Any) -> dict[str, float]:
    """Read the fields of a ticker that have a value, under the names of Quote."""
    found: dict[str, float] = {}
    for name, attribute in _PRICE_FIELDS:
        number = _first(ticker, attribute)
        if number is not None:
            found[name] = number
    greeks = getattr(ticker, "modelGreeks", None)
    for name, attribute in _GREEK_FIELDS:
        number = _first(greeks, attribute) if greeks is not None else None
        if number is not None:
            found[name] = number
    volume = _first(ticker, "volume", "callVolume", "putVolume")
    if volume is not None:
        found["volume"] = volume
    interest = _first(ticker, "callOpenInterest", "putOpenInterest")
    if interest is not None:
        found["open_interest"] = interest
    moment = getattr(ticker, "time", None)
    if moment is not None:
        found["event_ts"] = float(moment.timestamp())
    return found


def _decimal_text(strike: float) -> str:
    return format(Decimal(repr(strike)).normalize(), "f")


class _Job:
    """One quote in flight: woken by an update, an error or a disconnect."""

    def __init__(self, con_id: int) -> None:
        self.con_id = con_id
        self.event = asyncio.Event()
        self.error: tuple[int, str] | None = None
        self.on_error: Callable[[int, str], None] | None = None


class IbGateway:
    """Gateway over ib_async; the constructor requires the guard installer."""

    def __init__(
        self,
        guard: Callable[[Any], None],
        *,
        host: str,
        port: int,
        client_id: int,
        market_data_type: int = 1,
        lines: int = DEFAULT_LINES,
        snapshots: int = DEFAULT_SNAPSHOTS,
        clock: Clock | None = None,
        ib_factory: Callable[[], Any] = IB,
    ) -> None:
        self._install = guard
        self._host = host
        self._port = port
        self._client_id = client_id
        self._wanted_type = market_data_type
        self._clock: Clock = clock or SystemClock()
        self._make_ib = ib_factory
        self._pacer = MessagePacer(self._clock)
        self._lines = LineBudget(lines)
        self._snapshots = LineBudget(snapshots)
        self._lane = DetailsLane(self._clock)
        self._ib: Any = None
        self._connected = False
        self._generation = 0
        self._data_type = 0
        self._jobs: set[_Job] = set()
        self._lost = asyncio.Event()
        self._account_lock = asyncio.Lock()

    # --- session ----------------------------------------------------------

    async def open_session(self) -> None:
        """Connect behind the guard and set the market-data type once."""
        previous = self._ib
        self._connected = False
        self._lost = asyncio.Event()
        if previous is not None:
            previous.disconnect()
        ib = self._make_ib()
        self._install(ib.client)
        ib.client.apiEnd += lambda: self._on_end(ib)
        ib.errorEvent += self._on_error
        ib.RaiseRequestErrors = True
        self._ib = ib
        await ib.client.connectAsync(
            self._host,
            self._port,
            self._client_id,
        )
        await self._pacer.start()
        ib.reqMarketDataType(self._wanted_type)
        self._data_type = self._wanted_type
        self._generation += 1
        self._connected = True

    async def status(self) -> SessionState:
        return SessionState(self._connected, self._generation, self._data_type)

    def _on_end(self, ib: Any) -> None:
        if self._ib is not ib:
            return
        self._connected = False
        self._lost.set()
        for job in self._jobs:
            job.event.set()

    def _on_error(self, req_id: int, code: int, text: str, contract: Any) -> None:
        if spec.is_informational(code):
            return
        if spec.classify_error(code).shrink_budget:
            self._lines.shrink(code)
        con_id = getattr(contract, "conId", 0)
        for job in self._jobs:
            if con_id and job.con_id == con_id and job.error is None:
                job.error = (code, text)
                job.event.set()
                if job.on_error is not None:
                    job.on_error(code, text)

    def _current(self, generation: int) -> bool:
        return self._connected and self._generation == generation

    def _session(self) -> tuple[Any, int]:
        if not self._connected:
            raise ConnectionError("the gateway session is not connected")
        return self._ib, self._generation

    async def _until_lost[T](self, work: Awaitable[T]) -> T:
        """Await work, or raise ConnectionError when the session is lost first."""
        task = asyncio.ensure_future(work)
        lost = asyncio.ensure_future(self._lost.wait())
        try:
            await asyncio.wait({task, lost}, return_when=asyncio.FIRST_COMPLETED)
            if task.done():
                return task.result()
            raise ConnectionError("the gateway session was lost")
        finally:
            for pending in (task, lost):
                if not pending.done():
                    pending.cancel()
            await asyncio.gather(task, lost, return_exceptions=True)

    # --- contracts --------------------------------------------------------

    async def _send_details[T](
        self, generation: int, request: Callable[[], Awaitable[T]]
    ) -> T:
        async def send() -> T:
            await self._pacer.start()
            if not self._current(generation):
                raise ConnectionError("the gateway session was lost")
            return await request()

        return await self._until_lost(self._lane.run(send))

    async def chains(
        self, *, sec_type: str, symbol: str, con_id: int
    ) -> Sequence[ChainRow]:
        ib, generation = self._session()
        try:
            found = await self._send_details(
                generation,
                lambda: ib.reqSecDefOptParamsAsync(symbol, "", sec_type, con_id),
            )
        except RequestError as exc:
            raise spec.BrokerError(exc.code, exc.message) from exc
        return [
            ChainRow(
                exchange=chain.exchange,
                trading_class=chain.tradingClass,
                multiplier=chain.multiplier,
                expirations=tuple(sorted(chain.expirations)),
                strikes=tuple(_decimal_text(s) for s in sorted(chain.strikes)),
            )
            for chain in found
        ]

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
        ib, generation = self._session()
        contract = Contract(
            secType=sec_type,
            symbol=symbol,
            exchange=exchange,
            currency=currency,
            lastTradeDateOrContractMonth=expiry,
            strike=float(strike) if strike else 0.0,
            right=right,
            multiplier=multiplier,
            tradingClass=trading_class,
        )
        try:
            details = await self._send_details(
                generation, lambda: ib.reqContractDetailsAsync(contract)
            )
        except RequestError as exc:
            if spec.classify_error(exc.code).not_listed:
                return ResolveResult(
                    False, 0, error_code=exc.code, error_text=exc.message
                )
            raise spec.BrokerError(exc.code, exc.message) from exc
        if not details:
            return ResolveResult(False, 0)
        if len(details) > 1:
            raise spec.AmbiguousResolve(f"{len(details)} results for one contract")
        found = details[0].contract
        return ResolveResult(
            True,
            int(found.conId),
            exchange=found.exchange,
            trading_class=found.tradingClass,
            multiplier=found.multiplier,
        )

    # --- account ----------------------------------------------------------

    async def account(self, *, summary: bool, positions: bool) -> AccountRows:
        """Read the summary and the positions of the one account; calls are serialized."""
        async with self._account_lock:
            ib, generation = self._session()
            return await self._until_lost(
                read_account(
                    ib,
                    self._pacer,
                    self._clock,
                    generation=generation,
                    summary=summary,
                    positions=positions,
                )
            )

    async def executions(self) -> ExecutionRows:
        """Read the executions of the day, complete rows only; calls are serialized."""
        async with self._account_lock:
            ib, generation = self._session()
            return await self._until_lost(
                read_executions(ib, self._pacer, self._clock, generation=generation)
            )

    # --- quotes -----------------------------------------------------------

    async def quotes(
        self,
        kind: str,
        targets: Sequence[QuoteTarget],
        on_quote: Callable[[QuoteRow], Awaitable[None]],
    ) -> None:
        row_spec = spec.kind_spec(kind)
        generation = self._generation

        async def one(target: QuoteTarget) -> None:
            async with self._snapshots.lease():
                row = await self._quote(row_spec, target, generation)
            await on_quote(row)

        tasks = [asyncio.ensure_future(one(target)) for target in targets]
        try:
            await asyncio.gather(*tasks)
        finally:
            for task in tasks:
                task.cancel()
            await asyncio.gather(*tasks, return_exceptions=True)

    def _cancel(self, ib: Any, contract: Any) -> None:
        try:
            ib.cancelMktData(contract)
        except ConnectionError:
            pass  # the session is gone; its lines went with it

    async def _quote(
        self, row_spec: spec.KindSpec, target: QuoteTarget, generation: int
    ) -> QuoteRow:
        con_id = target.con_id
        requested = self._clock.now().timestamp()

        def row(status: str, **rest: Any) -> QuoteRow:
            return QuoteRow(
                con_id,
                status,
                rest.pop("data_type", self._data_type),
                generation,
                requested,
                rest.pop("sent", requested),
                self._clock.now().timestamp(),
                **rest,
            )

        if not self._current(generation):
            return row(spec.STATUS_DISCONNECTED)
        ib = self._ib
        async with self._lines.lease():
            await self._pacer.start()
            if not self._current(generation):
                return row(spec.STATUS_DISCONNECTED)
            job = _Job(con_id)
            self._jobs.add(job)
            contract = Contract(conId=con_id, exchange=target.exchange or "SMART")
            if target.currency:
                contract.currency = target.currency
            if target.trading_class:
                contract.tradingClass = target.trading_class
            if target.sec_type:
                contract.secType = target.sec_type
            sent = self._clock.now().timestamp()
            ticker: Any = None
            watch: Callable[..., None] | None = None
            timed_out = False
            try:
                ticker = ib.reqMktData(contract, row_spec.generic_ticks, False, False)

                def watch(*_: object) -> None:
                    if spec.is_complete(row_spec, ticker_fields(ticker)):
                        job.event.set()

                ticker.updateEvent += watch
                watch()
                try:
                    await with_timeout(
                        self._clock, job.event.wait(), row_spec.timeout_s
                    )
                except TimeoutError:
                    timed_out = True
            finally:
                self._jobs.discard(job)
                if ticker is not None and watch is not None:
                    ticker.updateEvent -= watch
                self._pacer.cancel(lambda: self._cancel(ib, contract))
            got = ticker_fields(ticker)
            data_type = int(getattr(ticker, "marketDataType", self._data_type))
        fields = tuple(sorted(got.items()))
        if spec.is_complete(row_spec, got):
            return row(spec.STATUS_OK, sent=sent, data_type=data_type, fields=fields)
        if not self._current(generation):
            return row(spec.STATUS_DISCONNECTED, sent=sent)
        if job.error is not None:
            code, text = job.error
            status = spec.classify_error(code).quote_status
            return row(
                status,
                sent=sent,
                data_type=data_type,
                fields=fields,
                error_code=code,
                error_text=text,
            )
        assert timed_out
        return row(
            spec.quote_status(row_spec, got),
            sent=sent,
            data_type=data_type,
            fields=fields,
        )

    # --- dividends --------------------------------------------------------

    async def dividends(
        self,
        keys: Sequence[DividendKey],
        on_reading: Callable[[DividendReading], Awaitable[None]],
    ) -> None:
        """Call on_reading once per key with its tick 456 reading.

        Each key holds a market-data line while it waits; a lost session raises
        ConnectionError. ib_async drops a tick it cannot parse, so such a key
        is answered as a timeout. Any other exception of a key (not a lost
        session, not a BrokerError) answers that key with status error, code 0.
        """

        async def one(key: DividendKey) -> None:
            try:
                async with self._lines.lease():
                    await self._pacer.start()
                    ib, generation = self._session()
                    con_id = key.con_id
                    if con_id <= 0:
                        matches = await self._send_details(
                            generation,
                            lambda: ib.reqContractDetailsAsync(
                                Contract(
                                    symbol=key.symbol,
                                    secType=key.sec_type,
                                    exchange=key.exchange or "SMART",
                                    currency=key.currency,
                                )
                            ),
                        )
                        if len(matches) != 1:
                            await on_reading(
                                self._unanswered(
                                    key,
                                    spec.STATUS_NO_VALUE
                                    if not matches
                                    else spec.STATUS_ERROR,
                                )
                            )
                            return
                        con_id = matches[0].contract.conId
                    reading = await self._until_lost(
                        read_dividend(
                            self._clock,
                            key,
                            lambda k, ticks, on_text, on_error: (
                                self._subscribe_dividend(
                                    ib, k, con_id, ticks, on_text, on_error
                                )
                            ),
                        )
                    )
            except (ConnectionError, spec.BrokerError):
                raise
            except Exception as exc:  # noqa: BLE001
                _log.error(
                    "dividend_failed type=%s message=%s",
                    type(exc).__name__,
                    str(exc)[:_DIVIDEND_LOG_MAX],
                )
                reading = self._unanswered(key, spec.STATUS_ERROR)
            await on_reading(reading)

        tasks = [asyncio.ensure_future(one(key)) for key in keys]
        try:
            await asyncio.gather(*tasks)
        finally:
            for task in tasks:
                task.cancel()
            await asyncio.gather(*tasks, return_exceptions=True)

    def _unanswered(self, key: DividendKey, status: str) -> DividendReading:
        return DividendReading(
            key, status, None, None, None, None, 0, self._clock.now().timestamp()
        )

    def _subscribe_dividend(
        self,
        ib: Any,
        key: DividendKey,
        con_id: int,
        ticks: str,
        on_text: TickText,
        on_error: TickError,
    ) -> Release:
        job = _Job(con_id)
        job.on_error = on_error
        self._jobs.add(job)
        contract = Contract(
            conId=con_id,
            symbol=key.symbol,
            secType=key.sec_type,
            exchange=key.exchange or "SMART",
            currency=key.currency,
        )
        ticker = ib.reqMktData(contract, ticks, False, False)

        def watch(*_: object) -> None:
            found = getattr(ticker, "dividends", None)
            if found is not None:
                on_text(_dividend_text(found))

        ticker.updateEvent += watch
        watch()

        def release() -> None:
            self._jobs.discard(job)
            ticker.updateEvent -= watch
            self._pacer.cancel(lambda: self._cancel(ib, contract))

        return release


def _dividend_text(found: Any) -> str:
    """Rebuild the four-value text of tick 456 from the parsed ib_async value."""

    def number(value: float | None) -> str:
        return "" if value is None else _decimal_text(value)

    moment = found.nextDate
    return ",".join(
        (
            number(found.past12Months),
            number(found.next12Months),
            "" if moment is None else moment.strftime("%Y%m%d"),
            number(found.nextAmount),
        )
    )


__all__ = ["IbGateway", "ticker_fields"]
