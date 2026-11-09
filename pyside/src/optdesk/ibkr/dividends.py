"""Dividend projection of one contract from IBKR generic tick 456.

The tick is one text of four comma-separated values: past 12 months, next 12
months, next ex-date YYYYMMDD, next amount. An empty or malformed text is
"no_value", never zero. The request, the release rule and the wait come from
the dividends row of spec.TABLE; the wait runs on the injected clock. The
broker session is reached through a Subscribe function and nothing else, so
this file imports neither ib_async nor generated code and never logs.
"""

from __future__ import annotations

import asyncio
from collections.abc import Callable
from datetime import datetime
from decimal import Decimal, InvalidOperation

from optdesk.ibkr import spec
from optdesk.ibkr.api import DividendKey, DividendReading
from optdesk.ibkr.timing import Clock, with_timeout

TickText = Callable[[str], None]
TickError = Callable[[int, str], None]
Release = Callable[[], None]
# subscribe(key, generic_ticks, on_text, on_error) starts the stream and returns
# the function that releases it.
Subscribe = Callable[[DividendKey, str, TickText, TickError], Release]

_VALUES = 4


def _decimal(text: str) -> str | None:
    try:
        number = Decimal(text)
    except InvalidOperation:
        return None
    if not number.is_finite() or number < 0:
        return None
    return format(number.normalize(), "f")


def _ex_date(text: str) -> str | None:
    if len(text) != 8 or not text.isdigit():
        return None
    try:
        datetime.strptime(text, "%Y%m%d")
    except ValueError:
        return None
    return text


def parse_tick(
    text: str,
) -> tuple[str | None, str | None, str | None, str | None] | None:
    """Return (past_12m, next_12m, next_ex_date, next_amount), or None for no value.

    None when the text does not hold four values or the ex-date is empty or
    not a calendar date. An empty or malformed number is None in its place.
    """
    parts = [part.strip() for part in text.split(",")]
    if len(parts) != _VALUES:
        return None
    past, following, date, amount = parts
    ex_date = _ex_date(date)
    if ex_date is None:
        return None
    return (_decimal(past), _decimal(following), ex_date, _decimal(amount))


async def read_dividend(
    clock: Clock, key: DividendKey, subscribe: Subscribe
) -> DividendReading:
    """Ask for tick 456 of key, wait for it on clock and release the stream."""
    row_spec = spec.kind_spec(spec.DIVIDENDS)
    event = asyncio.Event()
    arrived: dict[str, float] = {}
    texts: list[str] = []
    failure: list[tuple[int, str]] = []

    def on_text(text: str) -> None:
        texts.append(text)
        arrived["dividend_tick"] = 1.0
        if spec.is_complete(row_spec, arrived):
            event.set()

    def on_error(code: int, text: str) -> None:
        if not failure:
            failure.append((code, text))
        event.set()

    def reading(
        status: str,
        values: tuple[str | None, str | None, str | None, str | None] = (
            None,
            None,
            None,
            None,
        ),
        code: int = 0,
    ) -> DividendReading:
        return DividendReading(key, status, *values, code, clock.now().timestamp())

    release = subscribe(key, row_spec.generic_ticks, on_text, on_error)
    try:
        await with_timeout(clock, event.wait(), row_spec.timeout_s)
    except TimeoutError:
        return reading(spec.STATUS_TIMEOUT)
    finally:
        release()
    if texts:
        values = parse_tick(texts[-1])
        if values is None:
            return reading(spec.STATUS_NO_VALUE)
        return reading(spec.STATUS_OK, values)
    code, _ = failure[0]
    return reading(spec.STATUS_ERROR, code=code)
