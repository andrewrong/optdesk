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
        con_ids: Sequence[int],
        on_quote: Callable[[QuoteRow], Awaitable[None]],
    ) -> None:
        """Call on_quote once per contract, in the order answers complete."""
        ...
