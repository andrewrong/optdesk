"""The Yahoo symbol and the session-day zone of a (symbol, currency) pair."""

from __future__ import annotations

from dataclasses import dataclass
from zoneinfo import ZoneInfo

_NEW_YORK = ZoneInfo("America/New_York")
_TOKYO = ZoneInfo("Asia/Tokyo")


@dataclass(frozen=True)
class YahooTarget:
    """What to ask Yahoo for and the zone in which its index dates are session days."""

    yahoo_symbol: str
    zone: ZoneInfo


def map_pair(symbol: str, currency: str) -> YahooTarget | None:
    """Map a pair; None means the pair has no route (its code is `no_data`).

    USD keeps the symbol with a dot changed to a dash (BRK.B is BRK-B); JPY appends `.T`.
    """
    if currency == "USD":
        return YahooTarget(symbol.replace(".", "-"), _NEW_YORK)
    if currency == "JPY":
        return YahooTarget(f"{symbol}.T", _TOKYO)
    return None
