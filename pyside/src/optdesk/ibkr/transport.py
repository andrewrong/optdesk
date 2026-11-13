"""Transport of the ibkr module: the send guard, the log filter and the gRPC servicers.

The guard refuses every order message of the TWS client and every message
outside a short allowlist; it is installed on the client before it connects.
The log filter keeps account shapes out of every record. The servicers turn
the services of ibkr.v1 into calls on a Gateway. Generated types are
imported through optdesk.contracts.api only.
"""

from __future__ import annotations

import asyncio
import logging
import re
import sys
from collections.abc import AsyncIterator, Awaitable, Callable, Collection
from typing import Any, TextIO, cast

import grpc

from optdesk.contracts import api as pb
from optdesk.ibkr import list_rpc, spec
from optdesk.ibkr.api import (
    DividendKey,
    DividendReading,
    History,
    HistoryBar,
    HistoryKey,
    HistoryPaced,
    HistoryRequest,
    Reference,
)
from optdesk.ibkr.gateways import GatewaySpec
from optdesk.ibkr.history import wire_request
from optdesk.ibkr.model import (
    AccountGateway,
    AccountRows,
    ChainRow,
    ExecutionRow,
    ExecutionRows,
    ExecutionsGateway,
    Gateway,
    ListerGateway,
    MultipleAccounts,
    PositionRow,
    QuoteRow,
    QuoteTarget,
    ResolveResult,
)
from optdesk.ibkr.timing import Clock, SystemClock

# --- the guard -------------------------------------------------------------

ORDER_IDS = frozenset(
    {3, 4, 21, 58}
)  # placeOrder, cancelOrder, exercise, global cancel
ALLOWED_IDS = frozenset(
    {1, 2, 7, 9, 20, 25, 49, 59, 71, 78}
    | {61, 62, 63, 64}  # reqPositions, reqAccountSummary and their two cancels
)


class OrderRefused(Exception):
    """An order message was about to be sent."""


class NotAllowed(Exception):
    """A message that is not on the allowlist was about to be sent."""


def check(message_id: object) -> None:
    """Raise OrderRefused for an order id and NotAllowed for any id not allowed."""
    if message_id in ORDER_IDS:
        raise OrderRefused(f"message {message_id} places or changes orders")
    if message_id not in ALLOWED_IDS:
        raise NotAllowed(f"message {message_id} is not on the allowlist")


def _frame_id(frame: str) -> object:
    head = frame.split("\0", 1)[0]
    return int(head) if head.isdigit() else head


def install_guard(client: Any) -> None:
    """Guard client.send and client.sendMsg; call it before the client connects."""
    send = client.send
    send_msg = client.sendMsg

    def guarded_send(*fields: object, **options: Any) -> None:
        check(fields[0] if fields else None)
        send(*fields, **options)

    def guarded_send_msg(msg: str | None) -> None:
        if msg:
            check(_frame_id(msg))
        send_msg(msg)

    client.send = guarded_send
    client.sendMsg = guarded_send_msg


# --- the log filter --------------------------------------------------------

EVENTS = frozenset(
    {
        "rpc_done",
        "rpc_failed",
        "server_started",
        "server_stopped",
        "connect_ok",
        "connect_failed",
        "session_lost",
    }
)
FIELDS = frozenset(
    {"rpc", "kind", "count", "status", "code", "generation", "elapsed_ms", "limit"}
)
WORDS = frozenset(
    {
        "ping",
        "list_chains",
        "list_contracts",
        "resolve_contract",
        "snapshot",
        "account",
        "executions",
        "multiple_accounts",
        "dividends",
        "bars",
        spec.STATUS_NO_VALUE,
        spec.STATUS_TIMEOUT,
        spec.OPTION_EOD,
        spec.OPTION_OPEN,
        spec.UNDERLYING_IV,
        spec.SNAPSHOT,
        spec.STATUS_OK,
        spec.STATUS_PARTIAL,
        spec.STATUS_NO_DATA,
        spec.STATUS_NO_SUBSCRIPTION,
        spec.STATUS_ERROR,
        spec.STATUS_DISCONNECTED,
        "found",
        "not_listed",
    }
)
MAX_TEXT = 200

_ACCOUNT_SHAPE = re.compile(r"acct_[a-z]{1,8}|\b[A-Za-z]+\d{5,9}(?![A-Za-z0-9])")


def _strip_accounts(text: str) -> str:
    return _ACCOUNT_SHAPE.sub("", text)


def scrub(text: str) -> str:
    """Remove account shapes from text and drop free text over MAX_TEXT characters."""
    clean = _strip_accounts(text)
    return clean if len(clean) <= MAX_TEXT else ""


def _allowed_value(value: object) -> bool:
    if isinstance(value, bool | int | float):
        return True
    return isinstance(value, str) and value in WORDS


def log_event(logger: logging.Logger, event: str, **fields: object) -> None:
    """Log one event of the fixed set with the whitelisted fields it is given."""
    if event not in EVENTS:
        raise ValueError(f"unknown event {event!r}")
    kept = " ".join(
        f"{name}={value}"
        for name, value in fields.items()
        if name in FIELDS and _allowed_value(value)
    )
    logger.info("%s %s", event, kept)


class _ScrubFactory:
    """Record factory: every record of the process is scrubbed before any handler."""

    def __init__(self, previous: Any) -> None:
        self._previous = previous

    def __call__(self, *args: Any, **kwargs: Any) -> logging.LogRecord:
        record: logging.LogRecord = self._previous(*args, **kwargs)
        try:
            text = record.getMessage()
        except (TypeError, ValueError):
            text = str(record.msg)
        record.msg = scrub(text)
        record.args = None
        if record.exc_info and record.exc_info[0] is not None:
            trace = logging.Formatter().formatException(record.exc_info)
            record.exc_text = _strip_accounts(trace)
        return record


def configure_logging(level: int = logging.INFO, stream: TextIO | None = None) -> None:
    """Scrub every record, send the records to a stream and silence ib_async frames."""
    if not isinstance(logging.getLogRecordFactory(), _ScrubFactory):
        logging.setLogRecordFactory(_ScrubFactory(logging.getLogRecordFactory()))
    root = logging.getLogger()
    root.setLevel(level)
    if not root.handlers:
        handler = logging.StreamHandler(stream or sys.stderr)
        handler.setFormatter(logging.Formatter("%(asctime)s %(levelname)s %(message)s"))
        root.addHandler(handler)
    library = logging.getLogger("ib_async")
    library.setLevel(logging.WARNING)
    library.handlers.clear()
    library.propagate = True


# --- the servicers ---------------------------------------------------------

_LOG = logging.getLogger("optdesk.ibkr")

_SEC_TYPES = {
    pb.SEC_TYPE_STOCK: "STK",
    pb.SEC_TYPE_INDEX: "IND",
    pb.SEC_TYPE_OPTION: "OPT",
    pb.SEC_TYPE_CASH: "CASH",
}
_RIGHTS = {pb.RIGHT_PUT: "P", pb.RIGHT_CALL: "C"}
_SEC_TYPE_VALUES = {word: value for value, word in _SEC_TYPES.items()}
_RIGHT_VALUES = {word: value for value, word in _RIGHTS.items()}
_KINDS = {
    pb.QUOTE_KIND_OPTION_EOD: spec.OPTION_EOD,
    pb.QUOTE_KIND_OPTION_OPEN: spec.OPTION_OPEN,
    pb.QUOTE_KIND_UNDERLYING_IV: spec.UNDERLYING_IV,
    pb.QUOTE_KIND_SNAPSHOT: spec.SNAPSHOT,
}
_STATUSES = {
    spec.STATUS_OK: pb.QUOTE_STATUS_OK,
    spec.STATUS_PARTIAL: pb.QUOTE_STATUS_PARTIAL,
    spec.STATUS_NO_DATA: pb.QUOTE_STATUS_NO_DATA,
    spec.STATUS_NO_SUBSCRIPTION: pb.QUOTE_STATUS_NO_SUBSCRIPTION,
    "timeout": pb.QUOTE_STATUS_TIMEOUT,
    spec.STATUS_ERROR: pb.QUOTE_STATUS_ERROR,
    spec.STATUS_DISCONNECTED: pb.QUOTE_STATUS_DISCONNECTED,
}
INTERNAL_TEXT = "internal error"
FAILED_ACCOUNTS_TEXT = "the login does not serve exactly one account"
_ERROR_STATUSES = frozenset({spec.STATUS_ERROR, spec.STATUS_NO_SUBSCRIPTION})
_DOUBLE_FIELDS = frozenset(
    {
        "bid",
        "ask",
        "last",
        "close",
        "volume",
        "open_interest",
        "implied_vol",
        "delta",
        "gamma",
        "vega",
        "theta",
        "und_price",
        "option_implied_vol",
        "historical_vol",
        "bid_size",
        "ask_size",
        "last_size",
    }
)

Context = grpc.aio.ServicerContext[Any, Any]


def _data_type(value: int) -> pb.MarketDataType.ValueType:
    return cast("pb.MarketDataType.ValueType", value)


def quote_to_proto(row: QuoteRow) -> pb.Quote:
    """Convert a QuoteRow; error text is set only on the error statuses."""
    quote = pb.Quote(
        status=_STATUSES.get(row.status, pb.QUOTE_STATUS_ERROR),
        market_data_type=_data_type(row.market_data_type),
        generation=row.generation,
        error_code=row.error_code,
    )
    quote.contract.con_id = row.con_id
    quote.requested_at.FromNanoseconds(int(row.requested_at * 1e9))
    quote.sent_at.FromNanoseconds(int(row.sent_at * 1e9))
    quote.received_at.FromNanoseconds(int(row.received_at * 1e9))
    for name, value in row.fields:
        if name in _DOUBLE_FIELDS:
            setattr(quote, name, value)
        elif name == "event_ts":
            quote.event_ts.FromNanoseconds(int(value * 1e9))
    for name, reason, code in row.missing:
        quote.missing.add(field=name, reason=cast(Any, reason), error_code=code)
    if row.status in _ERROR_STATUSES:
        quote.error_text = scrub(row.error_text)
    return quote


def _chain_to_proto(row: ChainRow) -> pb.Chain:
    return pb.Chain(
        exchange=row.exchange,
        trading_class=row.trading_class,
        multiplier=row.multiplier,
        expirations=row.expirations,
        strikes=row.strikes,
    )


def _status_of(exc: BaseException) -> tuple[grpc.StatusCode, str]:
    if isinstance(exc, TimeoutError):
        return grpc.StatusCode.DEADLINE_EXCEEDED, "the gateway did not answer in time"
    if isinstance(exc, ConnectionError | OSError):
        return grpc.StatusCode.UNAVAILABLE, "the gateway is not connected"
    if isinstance(exc, HistoryPaced):
        return grpc.StatusCode.RESOURCE_EXHAUSTED, "historical requests are paced"
    if isinstance(exc, MultipleAccounts):
        return grpc.StatusCode.FAILED_PRECONDITION, FAILED_ACCOUNTS_TEXT
    if isinstance(exc, spec.AmbiguousResolve):
        return grpc.StatusCode.FAILED_PRECONDITION, scrub(str(exc))
    if isinstance(exc, spec.BrokerError):
        return grpc.StatusCode.UNKNOWN, scrub(str(exc))
    return grpc.StatusCode.INTERNAL, INTERNAL_TEXT


class _Servicers:
    """What the three servicers share: the gateway, a clock and the rpc wrapper."""

    def __init__(self, gateway: Gateway, clock: Clock) -> None:
        self.gateway = gateway
        self.clock = clock

    async def call[T](
        self,
        rpc: str,
        context: Context,
        work: Awaitable[T],
        count: Callable[[T], int] | None = None,
    ) -> T:
        started = self.clock.monotonic()
        try:
            result = await work
        except (grpc.RpcError, asyncio.CancelledError):
            raise
        except Exception as exc:
            code, details = _status_of(exc)
            word = exc.word if isinstance(exc, MultipleAccounts) else ""
            log_event(_LOG, "rpc_failed", rpc=rpc, code=code.value[0], status=word)
            await context.abort(code, details)
            raise  # abort raises; unreachable
        elapsed = int((self.clock.monotonic() - started) * 1000)
        rows = {} if count is None else {"count": count(result)}
        log_event(_LOG, "rpc_done", rpc=rpc, elapsed_ms=elapsed, **rows)
        return result


class SessionServicer(pb.SessionServiceServicer):
    """SessionService: the probe."""

    def __init__(self, shared: _Servicers) -> None:
        self._shared = shared

    async def Ping(self, request: pb.PingRequest, context: Context) -> pb.PingResponse:
        state = await self._shared.call("ping", context, self._shared.gateway.status())
        return pb.PingResponse(
            connected=state.connected,
            generation=state.generation,
            market_data_type=_data_type(state.market_data_type),
        )


class ContractsServicer(pb.ContractsServiceServicer):
    """ContractsService: chains and one-contract resolve."""

    def __init__(self, shared: _Servicers) -> None:
        self._shared = shared

    async def ListChains(
        self, request: pb.ListChainsRequest, context: Context
    ) -> pb.ListChainsResponse:
        under = request.underlying
        sec_type = _SEC_TYPES.get(under.sec_type)
        if sec_type not in ("STK", "IND") or not under.symbol or under.con_id <= 0:
            await context.abort(
                grpc.StatusCode.INVALID_ARGUMENT,
                "underlying needs a stock or index type, a symbol and a con_id",
            )
        rows = await self._shared.call(
            "list_chains",
            context,
            self._shared.gateway.chains(
                sec_type=sec_type or "", symbol=under.symbol, con_id=under.con_id
            ),
        )
        return pb.ListChainsResponse(
            underlying_con_id=under.con_id,
            chains=[_chain_to_proto(row) for row in rows],
        )

    async def ResolveContract(
        self, request: pb.ResolveContractRequest, context: Context
    ) -> pb.ResolveContractResponse:
        c = request.contract
        sec_type = _SEC_TYPES.get(c.sec_type)
        right = _RIGHTS.get(c.right, "")
        is_option = sec_type == "OPT"
        if (
            sec_type is None
            or not c.symbol
            or (is_option and not (c.expiry and c.strike and right))
        ):
            await context.abort(
                grpc.StatusCode.INVALID_ARGUMENT,
                "contract needs a type and a symbol; an option is fully specified",
            )
        result: ResolveResult = await self._shared.call(
            "resolve_contract",
            context,
            self._shared.gateway.resolve(
                sec_type=sec_type or "",
                symbol=c.symbol,
                exchange=c.exchange,
                currency=c.currency,
                expiry=c.expiry,
                strike=c.strike,
                right=right,
                multiplier=c.multiplier,
                trading_class=c.trading_class,
            ),
        )
        response = pb.ResolveContractResponse(
            status=pb.RESOLVE_STATUS_FOUND
            if result.found
            else pb.RESOLVE_STATUS_NOT_LISTED,
            error_code=result.error_code,
            error_text=scrub(result.error_text),
        )
        if result.found:
            response.contract.CopyFrom(c)
            response.contract.con_id = result.con_id
            response.contract.exchange = result.exchange or c.exchange
            response.contract.trading_class = result.trading_class or c.trading_class
            response.contract.multiplier = result.multiplier or c.multiplier
        return response

    async def ListContracts(
        self, request: pb.ListContractsRequest, context: Context
    ) -> pb.ListContractsResponse:
        gateway = self._shared.gateway
        if not hasattr(gateway, "list_contracts"):
            await context.abort(grpc.StatusCode.UNIMPLEMENTED, "no contract lists")
        if why := list_rpc.problem(request):
            await context.abort(grpc.StatusCode.INVALID_ARGUMENT, why)
        rows = await self._shared.call(
            "list_contracts",
            context,
            list_rpc.answer(cast(ListerGateway, gateway), request),
            len,
        )
        return list_rpc.to_response(rows)


class MarketDataServicer(pb.MarketDataServiceServicer):
    """MarketDataService: one quote per requested contract."""

    def __init__(self, shared: _Servicers) -> None:
        self._shared = shared

    async def Snapshot(
        self, request: pb.SnapshotRequest, context: Context
    ) -> AsyncIterator[pb.SnapshotResponse]:
        kind = _KINDS.get(request.kind)
        targets = [
            QuoteTarget(
                c.con_id,
                c.exchange,
                c.currency,
                c.trading_class,
                _SEC_TYPES.get(c.sec_type, ""),
            )
            for c in request.contracts
        ]
        if kind is None or not targets or any(t.con_id <= 0 for t in targets):
            await context.abort(
                grpc.StatusCode.INVALID_ARGUMENT,
                "snapshot needs a kind and contracts that all carry a con_id",
            )
        queue: asyncio.Queue[QuoteRow | None] = asyncio.Queue()

        async def on_quote(row: QuoteRow) -> None:
            await queue.put(row)

        work = asyncio.ensure_future(
            self._shared.gateway.quotes(kind or "", targets, on_quote)
        )
        work.add_done_callback(lambda _: queue.put_nowait(None))
        started = self._shared.clock.monotonic()
        try:
            while (row := await queue.get()) is not None:
                yield pb.SnapshotResponse(quote=quote_to_proto(row))
            failure = work.exception()
        finally:
            if not work.done():
                work.cancel()
            await asyncio.gather(work, return_exceptions=True)
        if failure is not None:
            code, details = _status_of(failure)
            log_event(_LOG, "rpc_failed", rpc="snapshot", code=code.value[0])
            await context.abort(code, details)
        elapsed = int((self._shared.clock.monotonic() - started) * 1000)
        log_event(
            _LOG, "rpc_done", rpc="snapshot", count=len(targets), elapsed_ms=elapsed
        )

    async def Bars(self, request: pb.BarsRequest, context: Context) -> pb.BarsResponse:
        """Daily bars of one contract; the pacer of the gateway may refuse with RESOURCE_EXHAUSTED."""
        gateway = self._shared.gateway
        if not isinstance(gateway, History):
            await context.abort(
                grpc.StatusCode.UNIMPLEMENTED, "this gateway serves no history"
            )
        kind = _BAR_KINDS.get(request.kind)
        c = request.contract
        asked = HistoryRequest(
            HistoryKey(
                c.symbol,
                _SEC_TYPES.get(c.sec_type, ""),
                c.exchange,
                c.currency,
                c.con_id,
            ),
            kind or "",
            request.duration_days,
            request.end_day,
        )
        problem = _bars_problem(asked)
        if problem:
            await context.abort(grpc.StatusCode.INVALID_ARGUMENT, problem)
        rows = await self._shared.call(
            "bars", context, cast(History, gateway).bars(asked), count=len
        )
        return pb.BarsResponse(bars=[_bar_to_proto(row) for row in rows])


_BAR_KINDS = {
    pb.BAR_KIND_TRADES: "trades",
    pb.BAR_KIND_ADJUSTED_LAST: "adjusted_last",
    pb.BAR_KIND_MIDPOINT: "midpoint",
    pb.BAR_KIND_OPTION_IMPLIED_VOL: "option_implied_vol",
}
_MAX_BAR_DAYS = 730


def _bars_problem(asked: HistoryRequest) -> str:
    """Return why a bars request is refused, or an empty text."""
    if asked.key.con_id <= 0:
        return "bars needs a contract with a con_id"
    if not asked.kind:
        return "bars needs a kind"
    if not 1 <= asked.duration_days <= _MAX_BAR_DAYS:
        return "bars needs duration_days from 1 to 730"
    if asked.end_day and asked.kind == "adjusted_last":
        return "adjusted_last bars take no end_day"
    try:
        wire_request(asked)
    except ValueError as exc:
        return str(exc)
    return ""


def _bar_to_proto(row: HistoryBar) -> pb.Bar:
    return pb.Bar(
        day=row.day,
        open=row.open,
        high=row.high,
        low=row.low,
        close=row.close,
        volume=row.volume,
    )


_DIVIDEND_STATUSES = {
    spec.STATUS_OK: pb.DIVIDEND_STATUS_OK,
    spec.STATUS_NO_VALUE: pb.DIVIDEND_STATUS_NO_VALUE,
    spec.STATUS_TIMEOUT: pb.DIVIDEND_STATUS_TIMEOUT,
    spec.STATUS_ERROR: pb.DIVIDEND_STATUS_ERROR,
}


def dividend_to_proto(reading: DividendReading) -> pb.DividendProjection:
    """Convert a reading; a value that is None stays unset, never zero."""
    key = reading.key
    projection = pb.DividendProjection(
        key=pb.ContractKey(
            symbol=key.symbol,
            sec_type=_SEC_TYPE_VALUES.get(key.sec_type, pb.SEC_TYPE_UNSPECIFIED),
            exchange=key.exchange,
            currency=key.currency,
            con_id=key.con_id,
        ),
        status=_DIVIDEND_STATUSES.get(reading.status, pb.DIVIDEND_STATUS_ERROR),
        past_12m=reading.past_12m,
        next_12m=reading.next_12m,
        next_ex_date=reading.next_ex_date,
        next_amount=reading.next_amount,
        error_code=reading.error_code,
    )
    projection.received_at.FromNanoseconds(int(reading.received_at * 1e9))
    return projection


class ReferenceServicer(pb.ReferenceServiceServicer):
    """ReferenceService: the dividend projection of each contract key received."""

    def __init__(self, shared: _Servicers) -> None:
        self._shared = shared

    async def Dividends(
        self, request_iterator: AsyncIterator[pb.ContractKey], context: Context
    ) -> AsyncIterator[pb.DividendProjection]:
        gateway = self._shared.gateway
        if not isinstance(gateway, Reference):
            await context.abort(
                grpc.StatusCode.UNIMPLEMENTED, "this gateway serves no reference data"
            )
        keys = [
            DividendKey(
                key.symbol,
                _SEC_TYPES.get(key.sec_type, ""),
                key.exchange,
                key.currency,
                key.con_id,
            )
            async for key in request_iterator
        ]
        if not keys or any(not k.symbol and k.con_id <= 0 for k in keys):
            await context.abort(
                grpc.StatusCode.INVALID_ARGUMENT,
                "dividends needs keys that each carry a symbol or a con_id",
            )
        queue: asyncio.Queue[DividendReading | None] = asyncio.Queue()

        async def on_reading(reading: DividendReading) -> None:
            await queue.put(reading)

        work = asyncio.ensure_future(
            cast(Reference, gateway).dividends(keys, on_reading)
        )
        work.add_done_callback(lambda _: queue.put_nowait(None))
        started = self._shared.clock.monotonic()
        try:
            while (reading := await queue.get()) is not None:
                yield dividend_to_proto(reading)
            failure = work.exception()
        finally:
            if not work.done():
                work.cancel()
            await asyncio.gather(work, return_exceptions=True)
        if failure is not None:
            code, details = _status_of(failure)
            log_event(_LOG, "rpc_failed", rpc="dividends", code=code.value[0])
            await context.abort(code, details)
        elapsed = int((self._shared.clock.monotonic() - started) * 1000)
        log_event(
            _LOG, "rpc_done", rpc="dividends", count=len(keys), elapsed_ms=elapsed
        )


class AccountServicer(pb.AccountServiceServicer):
    """AccountService: the summary and the positions of the one account of a gateway.

    The account field answers the id the gateway reports, as it is; it is never logged.
    """

    def __init__(self, shared: _Servicers, collect: Collection[str]) -> None:
        self._shared = shared
        self._summary = "account_summary" in collect
        self._positions = "positions" in collect
        self._executions = "executions" in collect

    async def Account(
        self, request: pb.AccountRequest, context: Context
    ) -> pb.AccountResponse:
        if not (self._summary or self._positions):
            await context.abort(
                grpc.StatusCode.UNIMPLEMENTED, "this gateway serves no account data"
            )
        gateway = cast(AccountGateway, self._shared.gateway)
        rows: AccountRows = await self._shared.call(
            "account",
            context,
            gateway.account(summary=self._summary, positions=self._positions),
            lambda r: len(r.values) + len(r.positions),
        )
        return pb.AccountResponse(
            account=rows.account,
            generation=rows.generation,
            values_collected=self._summary,
            positions_collected=self._positions,
            values=[
                pb.AccountValue(tag=v.tag, value=v.value, currency=v.currency)
                for v in (rows.values if self._summary else ())
            ],
            positions=[
                _position_to_proto(p)
                for p in (rows.positions if self._positions else ())
            ],
        )

    async def Executions(
        self, request: pb.ExecutionsRequest, context: Context
    ) -> pb.ExecutionsResponse:
        if not self._executions:
            return pb.ExecutionsResponse(collected=False)
        gateway = cast(ExecutionsGateway, self._shared.gateway)
        rows: ExecutionRows = await self._shared.call(
            "executions",
            context,
            gateway.executions(),
            lambda r: len(r.executions),
        )
        return pb.ExecutionsResponse(
            account=rows.account,
            generation=rows.generation,
            collected=True,
            pending=rows.pending,
            executions=[_execution_to_proto(e) for e in rows.executions],
        )


def _contract_to_proto(row: PositionRow | ExecutionRow) -> pb.ContractSpec:
    return pb.ContractSpec(
        sec_type=_SEC_TYPE_VALUES.get(row.sec_type, pb.SEC_TYPE_UNSPECIFIED),
        symbol=row.symbol,
        exchange=row.exchange,
        currency=row.currency,
        expiry=row.expiry,
        strike=row.strike,
        right=_RIGHT_VALUES.get(row.right, pb.RIGHT_UNSPECIFIED),
        multiplier=row.multiplier,
        trading_class=row.trading_class,
        con_id=row.con_id,
    )


def _execution_to_proto(row: ExecutionRow) -> pb.Execution:
    return pb.Execution(
        exec_id=row.exec_id,
        perm_id=row.perm_id,
        time=row.time,
        contract=_contract_to_proto(row),
        side=row.side,
        quantity=row.quantity,
        price=row.price,
        commission=row.commission,
        commission_currency=row.commission_currency,
    )


def _position_to_proto(row: PositionRow) -> pb.Position:
    return pb.Position(
        contract=_contract_to_proto(row), quantity=row.quantity, avg_cost=row.avg_cost
    )


def build_server(
    gateway: Gateway,
    clock: Clock | None = None,
    *,
    spec: GatewaySpec | None = None,
) -> grpc.aio.Server:
    """Return a grpc.aio server over gateway; it is not bound.

    Without spec: Session, Contracts, MarketData and Reference. With spec, by its collect
    words: Session always, Contracts, MarketData and Reference with market_data, Account
    with account_summary or positions (answering the id the gateway reports).
    """
    shared = _Servicers(gateway, clock or SystemClock())
    server = grpc.aio.server()
    pb.add_SessionServiceServicer_to_server(SessionServicer(shared), server)
    collect = spec.collect if spec is not None else ("market_data",)
    if "market_data" in collect:
        pb.add_ContractsServiceServicer_to_server(ContractsServicer(shared), server)
        pb.add_MarketDataServiceServicer_to_server(MarketDataServicer(shared), server)
        pb.add_ReferenceServiceServicer_to_server(ReferenceServicer(shared), server)
    if spec is not None and (
        {"account_summary", "positions", "executions"} & set(collect)
    ):
        pb.add_AccountServiceServicer_to_server(
            AccountServicer(shared, collect), server
        )
    return server
