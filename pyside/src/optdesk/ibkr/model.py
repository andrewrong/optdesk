"""Gateway seam of the ibkr module: plain values and the Gateway protocol.

Declarations only. The values are frozen dataclasses of plain types; this file
imports neither ib_async nor generated code, so the transport and the adapter
meet here without knowing each other.
"""

from __future__ import annotations

from collections.abc import Awaitable, Callable, Sequence
from dataclasses import dataclass
from typing import NamedTuple, Protocol


@dataclass(frozen=True)
class ChainRow:
    """One exchange, trading class and multiplier of an underlying.

    strikes is the union over all expirations; expirations are YYYYMMDD.
    """

    exchange: str
    trading_class: str
    multiplier: str
    expirations: tuple[str, ...]
    strikes: tuple[str, ...]


@dataclass(frozen=True)
class ResolveResult:
    """Answer of a resolve that reached the end.

    found is False for a contract that is not listed; con_id is 0 then.
    """

    found: bool
    con_id: int
    exchange: str = ""
    trading_class: str = ""
    multiplier: str = ""
    error_code: int = 0
    error_text: str = ""


@dataclass(frozen=True)
class QuoteRow:
    """One quote of one contract; status is a lower-case word such as "ok"."""

    con_id: int
    status: str
    market_data_type: int
    generation: int
    requested_at: float
    sent_at: float
    received_at: float
    fields: tuple[tuple[str, float], ...] = ()
    error_code: int = 0
    error_text: str = ""


@dataclass(frozen=True)
class QuoteTarget:
    """One contract to quote: the con_id and the venue fields the caller asked for.

    An empty field means "not given"; sec_type is the TWS word ("STK", "OPT").
    """

    con_id: int
    exchange: str = ""
    currency: str = ""
    trading_class: str = ""
    sec_type: str = ""


class SessionState(NamedTuple):
    """State of the broker session."""

    connected: bool
    generation: int
    market_data_type: int


class Gateway(Protocol):
    """The broker as the transport sees it; every method is async."""

    async def open_session(self) -> None:
        """Connect and set the market-data type; raise when it cannot."""
        ...

    async def status(self) -> SessionState:
        """Return the state of the session."""
        ...

    async def chains(
        self, *, sec_type: str, symbol: str, con_id: int
    ) -> Sequence[ChainRow]:
        """List the chains of an underlying."""
        ...

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
        """Resolve one fully specified contract."""
        ...

    async def quotes(
        self,
        kind: str,
        targets: Sequence[QuoteTarget],
        on_quote: Callable[[QuoteRow], Awaitable[None]],
    ) -> None:
        """Call on_quote once per target, in the order answers complete.

        The exchange is SMART when the target's exchange is empty.
        """
        ...


@dataclass(frozen=True)
class AccountValueRow:
    """One summary figure: a tag of the fixed list, a decimal text and a currency."""

    tag: str
    value: str
    currency: str


@dataclass(frozen=True)
class PositionRow:
    """One open position, without the account it belongs to.

    sec_type is the TWS word ("STK", "OPT"); quantity and avg_cost are decimal text.
    """

    sec_type: str
    symbol: str
    con_id: int
    exchange: str
    currency: str
    expiry: str
    strike: str
    right: str
    multiplier: str
    trading_class: str
    quantity: str
    avg_cost: str


@dataclass(frozen=True)
class AccountRows:
    """Rows of one account read in one call; a list that was not asked for is empty."""

    generation: int
    values: tuple[AccountValueRow, ...] = ()
    positions: tuple[PositionRow, ...] = ()


@dataclass(frozen=True)
class ExecutionRow:
    """One fill with its commission, without the account or the order it belongs to.

    time is RFC 3339 UTC text; side is BOT or SLD; the numbers are decimal text.
    The contract fields are those of PositionRow.
    """

    exec_id: str
    perm_id: int
    time: str
    sec_type: str
    symbol: str
    con_id: int
    exchange: str
    currency: str
    expiry: str
    strike: str
    right: str
    multiplier: str
    trading_class: str
    side: str
    quantity: str
    price: str
    commission: str
    commission_currency: str


@dataclass(frozen=True)
class ExecutionRows:
    """The complete executions of one account; pending counts those left out.

    A row is complete when its commission report is in. pending is the number of
    executions whose report had not come within the deadline.
    """

    generation: int
    executions: tuple[ExecutionRow, ...] = ()
    pending: int = 0


class MultipleAccounts(Exception):
    """The login reports no account or more than one; carries no count and no id."""

    word = "multiple_accounts"

    def __init__(self) -> None:
        super().__init__("the login does not report exactly one account")


class AccountGateway(Protocol):
    """A broker that serves the summary, the positions and the executions of its one account."""

    async def account(self, *, summary: bool, positions: bool) -> AccountRows:
        """Read the asked lists in one call; raise MultipleAccounts for a login of several."""
        ...

    async def executions(self) -> ExecutionRows:
        """Read the executions of the day with their commissions.

        Raise MultipleAccounts for a login of several.
        """
        ...
