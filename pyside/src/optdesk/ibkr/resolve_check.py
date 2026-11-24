"""The answer of a resolve is IB's own contract; a contract that differs from the request is refused."""

from __future__ import annotations

from decimal import Decimal, InvalidOperation
from typing import Any

from optdesk.ibkr import spec
from optdesk.ibkr.model import ResolveResult


def _decimal(text: str) -> Decimal | None:
    try:
        return Decimal(text).normalize()
    except InvalidOperation:
        return None


def _number_text(value: float) -> str:
    """Canonical decimal text of a number; zero (not an option) is empty."""
    if not value:
        return ""
    return format(Decimal(repr(float(value))).normalize(), "f")


def _right_of(raw: str) -> str:
    word = (raw or "").strip().upper()[:1]
    return word if word in ("C", "P") else ""


def _same_text(asked: str, got: str) -> bool:
    return asked == got


def _clip(text: str) -> str:
    """A value cut to 24 characters, so the text of a many-field refusal stays under the scrub limit."""
    return text[:24]


def _same_number(asked: str, got: str) -> bool:
    a, b = _decimal(asked), _decimal(got)
    return asked == got if a is None or b is None else a == b


def answer(
    details: Any,
    *,
    expiry: str,
    strike: str,
    right: str,
    multiplier: str,
    trading_class: str,
) -> ResolveResult:
    """The found ResolveResult holding IB's fields, or AmbiguousResolve naming each difference.

    A request field that is empty is not compared; IB's value for it is answered.
    """
    found = details.contract
    got = {
        "expiry": found.lastTradeDateOrContractMonth or "",
        "strike": _number_text(found.strike),
        "right": _right_of(found.right),
        "trading_class": found.tradingClass or "",
        "multiplier": found.multiplier or "",
    }
    differing = [
        f"{name} {_clip(asked)}->{_clip(got[name]) or 'none'}"
        for name, asked, same in (
            ("expiry", expiry, _same_text),
            ("strike", strike, _same_number),
            ("right", right, _same_text),
            ("trading_class", trading_class, _same_text),
            ("multiplier", multiplier, _same_number),
        )
        if asked and not same(asked, got[name])
    ]
    if differing:
        raise spec.AmbiguousResolve(
            "resolve differs (requested->returned): " + "; ".join(differing)
        )
    return ResolveResult(
        True,
        int(found.conId),
        exchange=found.exchange,
        trading_class=got["trading_class"],
        multiplier=got["multiplier"],
        expiry=got["expiry"],
        strike=got["strike"],
        right=got["right"],
        **spec.hours_fields(details),
    )
