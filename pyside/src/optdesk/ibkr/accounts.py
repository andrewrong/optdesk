"""The read of the account summary and the positions of one account.

The one file of the package that sends the four account messages (ids 61 to 64
of the TWS protocol): it calls client.reqAccountSummary itself with a fixed list
of ten tags, not IB.reqAccountSummaryAsync (which sends every tag and never
cancels), and cancels both requests in a finally, so no subscription outlives a
call. Rows are collected per call and rebuilt from typed fields: the account id
the gateway reports is dropped here and never reaches a caller. This file
imports no ib_async; the objects are named `ib` and `client` as the loop rules
expect, and it never logs.
"""

from __future__ import annotations

import asyncio
from collections.abc import Awaitable
from datetime import UTC, datetime
from decimal import Decimal, InvalidOperation
from typing import Any

from optdesk.ibkr.model import (
    AccountRows,
    AccountValueRow,
    ExecutionRow,
    ExecutionRows,
    MultipleAccounts,
    PositionRow,
)
from optdesk.ibkr.pacing import MessagePacer
from optdesk.ibkr.timing import Clock, with_timeout

TAGS = (
    "NetLiquidation",
    "TotalCashValue",
    "AvailableFunds",
    "ExcessLiquidity",
    "BuyingPower",
    "GrossPositionValue",
    "EquityWithLoanValue",
    "InitMarginReq",
    "MaintMarginReq",
    "Cushion",
)
DEADLINE_S = 20.0
# Far above the counter of client.getReqId, which starts at 0; calls to one
# gateway are serialized, so one id is in flight at a time.
SUMMARY_REQ_ID = 2_000_000_000
EXECUTIONS_REQ_ID = SUMMARY_REQ_ID + 1
# How often the commission reports still missing are looked for after the end marker.
COMMISSION_POLL_S = 0.25
_POSITIONS_KEY = "positions"


def decimal_text(value: float | str) -> str:
    """Return value as decimal text with no exponent; raise ValueError if it is none."""
    try:
        number = Decimal(repr(value) if isinstance(value, float) else value.strip())
    except InvalidOperation:
        raise ValueError("not a decimal number") from None
    if not number.is_finite():
        raise ValueError("not a decimal number")
    return format(number.normalize(), "f") if number else "0"


def _strike(value: object) -> str:
    return (
        decimal_text(float(value)) if isinstance(value, int | float) and value else ""
    )


def _contract_fields(contract: Any) -> dict[str, Any]:
    return {
        "sec_type": str(contract.secType),
        "symbol": str(contract.symbol),
        "con_id": int(contract.conId),
        "exchange": str(contract.exchange),
        "currency": str(contract.currency),
        "expiry": str(contract.lastTradeDateOrContractMonth),
        "strike": _strike(contract.strike),
        "right": str(contract.right) if contract.right in ("P", "C") else "",
        "multiplier": str(contract.multiplier),
        "trading_class": str(contract.tradingClass),
    }


def position_row(position: Any) -> PositionRow:
    """Rebuild one position from typed fields; the account of position is dropped."""
    return PositionRow(
        **_contract_fields(position.contract),
        quantity=decimal_text(float(position.position)),
        avg_cost=decimal_text(float(position.avgCost)),
    )


def _value_rows(account: str, items: list[Any]) -> tuple[AccountValueRow, ...]:
    rows: dict[tuple[str, str], AccountValueRow] = {}
    for item in items:
        if item.account != account or item.tag not in TAGS:
            continue
        try:
            text = decimal_text(str(item.value))
        except ValueError:
            continue
        rows[(item.tag, item.currency)] = AccountValueRow(
            item.tag, text, str(item.currency)
        )
    order = {tag: n for n, tag in enumerate(TAGS)}
    return tuple(sorted(rows.values(), key=lambda r: (order[r.tag], r.currency)))


def _position_rows(account: str, items: list[Any]) -> tuple[PositionRow, ...]:
    return tuple(
        position_row(item)
        for item in items
        if item.account == account and item.position != 0
    )


async def read_account(
    ib: Any,
    pacer: MessagePacer,
    clock: Clock,
    *,
    generation: int,
    summary: bool,
    positions: bool,
) -> AccountRows:
    """Read the asked lists of the one account of ib, then cancel both requests.

    Raises MultipleAccounts unless the login reports exactly one account, and
    TimeoutError when the end messages do not come within DEADLINE_S of clock.
    """
    account = _one_account(ib)
    client = ib.client
    seen: list[Any] = []
    waiting: list[Awaitable[Any]] = []
    sent_summary = False
    sent_positions = False

    def collect(item: Any) -> None:
        seen.append(item)

    if summary:
        ib.accountSummaryEvent += collect
    try:
        if summary:
            waiting.append(ib.wrapper.startReq(SUMMARY_REQ_ID))
            await pacer.start()
            sent_summary = True
            client.reqAccountSummary(SUMMARY_REQ_ID, "All", ",".join(TAGS))
        if positions:
            waiting.append(ib.wrapper.startReq(_POSITIONS_KEY))
            await pacer.start()
            sent_positions = True
            client.reqPositions()
        results = await with_timeout(clock, asyncio.gather(*waiting), DEADLINE_S)
    finally:
        if summary:
            ib.accountSummaryEvent -= collect
        if sent_summary:
            pacer.cancel(lambda: _cancel_summary(client))
        if sent_positions:
            pacer.cancel(lambda: _cancel_positions(client))
    return AccountRows(
        generation=generation,
        values=_value_rows(account, seen) if summary else (),
        positions=_position_rows(account, results[-1]) if positions else (),
        account=account,
    )


def _cancel_summary(client: Any) -> None:
    try:
        client.cancelAccountSummary(SUMMARY_REQ_ID)
    except ConnectionError:
        pass  # the session is gone; its subscriptions went with it


def _cancel_positions(client: Any) -> None:
    try:
        client.cancelPositions()
    except ConnectionError:
        pass  # the session is gone; its subscriptions went with it


def _instant(value: datetime) -> str:
    moment = value.replace(tzinfo=UTC) if value.tzinfo is None else value
    return moment.astimezone(UTC).strftime("%Y-%m-%dT%H:%M:%SZ")


def _has_commission(fill: Any) -> bool:
    return bool(fill.commissionReport.execId == fill.execution.execId)


def execution_row(fill: Any) -> ExecutionRow:
    """Rebuild one complete fill from typed fields; account and order ids are dropped."""
    execution = fill.execution
    report = fill.commissionReport
    return ExecutionRow(
        exec_id=str(execution.execId),
        perm_id=int(execution.permId),
        time=_instant(execution.time),
        side=str(execution.side),
        quantity=decimal_text(float(execution.shares)),
        price=decimal_text(float(execution.price)),
        commission=decimal_text(float(report.commission)),
        commission_currency=str(report.currency),
        **_contract_fields(fill.contract),
    )


def _one_account(ib: Any) -> str:
    accounts = list(ib.managedAccounts())
    if len(accounts) != 1:
        raise MultipleAccounts
    return str(accounts[0])


async def read_executions(
    ib: Any, pacer: MessagePacer, clock: Clock, *, generation: int
) -> ExecutionRows:
    """Read the executions of the day of the one account of ib, complete rows only.

    Sends reqExecutions (id 7) once and waits for the end marker, then for the
    commission reports still missing, both within DEADLINE_S of clock. A fill
    whose report is not in by then is counted in pending and not answered.
    Raises MultipleAccounts unless the login reports exactly one account, and
    TimeoutError when the end marker does not come in time.
    """
    account = _one_account(ib)
    client = ib.client
    started = clock.monotonic()
    waiting = ib.wrapper.startReq(EXECUTIONS_REQ_ID)
    await pacer.start()
    # reqExecutions of the client: id 7, message version 3, an empty filter
    client.send(7, 3, EXECUTIONS_REQ_ID, 0, "", "", "", "", "", "")
    fills = await with_timeout(clock, waiting, DEADLINE_S)
    mine = {
        fill.execution.execId: fill
        for fill in fills
        if fill.execution.acctNumber == account
    }
    while (
        any(not _has_commission(f) for f in mine.values())
        and clock.monotonic() - started < DEADLINE_S
    ):
        await clock.sleep(COMMISSION_POLL_S)
    complete = [f for f in mine.values() if _has_commission(f)]
    rows = sorted(map(execution_row, complete), key=lambda r: (r.time, r.exec_id))
    return ExecutionRows(
        generation=generation,
        executions=tuple(rows),
        pending=len(mine) - len(complete),
        account=account,
    )
