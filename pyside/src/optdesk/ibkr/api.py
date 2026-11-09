"""Interface of the ibkr module: the session probe and the dividend reads of ibkr.v1.

Declarations only. Mirrors contracts/proto/ibkr/v1/session.proto and
reference.proto by hand and imports no generated code.
"""

from __future__ import annotations

from collections.abc import Awaitable, Callable, Sequence
from dataclasses import dataclass
from typing import Protocol, runtime_checkable


@dataclass(frozen=True)
class SessionStatus:
    """Answer of a session probe."""

    connected: bool


@runtime_checkable
class Session(Protocol):
    """A broker session that can be probed."""

    def ping(self) -> SessionStatus:
        """Return whether the session is connected."""
        ...


@dataclass(frozen=True)
class DividendKey:
    """A listed contract to ask about; sec_type is the TWS word ("STK"); con_id 0 is unknown."""

    symbol: str
    sec_type: str
    exchange: str
    currency: str
    con_id: int


@dataclass(frozen=True)
class DividendReading:
    """Answer for one DividendKey.

    status is "ok" (parsable next ex-date), "no_value" (empty or malformed tick;
    every value is None, never zero), "timeout" or "error". The four values are
    canonical decimal text, next_ex_date is YYYYMMDD; received_at is epoch seconds.
    """

    key: DividendKey
    status: str
    past_12m: str | None
    next_12m: str | None
    next_ex_date: str | None
    next_amount: str | None
    error_code: int
    received_at: float


@runtime_checkable
class Reference(Protocol):
    """A broker session that serves reference data."""

    async def dividends(
        self,
        keys: Sequence[DividendKey],
        on_reading: Callable[[DividendReading], Awaitable[None]],
    ) -> None:
        """Call on_reading once per key, in the order answers complete."""
        ...
