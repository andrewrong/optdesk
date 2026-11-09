"""Interface of the ibkr module: the session probe, the dividend reads and the bars of ibkr.v1.

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


@dataclass(frozen=True)
class HistoryKey:
    """A listed contract whose daily bars are asked for; sec_type is the TWS word ("STK")."""

    symbol: str
    sec_type: str
    exchange: str
    currency: str
    con_id: int


@dataclass(frozen=True)
class HistoryRequest:
    """One request for daily bars.

    kind is "trades", "adjusted_last" or "midpoint". duration_days is 1 to 730.
    end_day is YYYY-MM-DD or empty for now; it is empty with "adjusted_last".
    """

    key: HistoryKey
    kind: str
    duration_days: int
    end_day: str


@dataclass(frozen=True)
class HistoryBar:
    """One daily bar: day is YYYY-MM-DD, prices and volume are decimal text.

    volume is None when the broker has none.
    """

    day: str
    open: str
    high: str
    low: str
    close: str
    volume: str | None


class HistoryPaced(Exception):
    """The pacer would wait longer than its limit; the request was not sent."""


@runtime_checkable
class History(Protocol):
    """A broker session that serves daily bars."""

    async def bars(self, request: HistoryRequest) -> Sequence[HistoryBar]:
        """Return the bars of the request, oldest first; raise HistoryPaced when paced."""
        ...
