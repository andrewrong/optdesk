"""The command `python -m optdesk.refearn`: one JSON request in, one JSON answer out."""

from __future__ import annotations

import argparse
import contextlib
import datetime
import json
import tempfile
from collections.abc import Sequence
from typing import Any, NoReturn, TextIO

from optdesk.refbars.api import Clock
from optdesk.refearn.api import Answer, Request, SymbolResult, SymbolSpec
from optdesk.refearn.fetch import make_fetcher

MAX_SYMBOLS = 10
GAP_SECONDS = 1.0
TIMEOUT_SECONDS = 10


class _UsageError(Exception):
    pass


class _Parser(argparse.ArgumentParser):
    def error(self, message: str) -> NoReturn:
        raise _UsageError(message)


def _request(text: str) -> Request:
    try:
        raw = json.loads(text)
        symbols = [
            SymbolSpec(symbol=str(s["symbol"]), currency=str(s["currency"]))
            for s in raw["symbols"]
        ]
        from_day = str(raw["from_day"])
        datetime.date.fromisoformat(from_day)
    except (ValueError, KeyError, TypeError):
        raise _UsageError("the request is not the JSON of the interface") from None
    if len(symbols) > MAX_SYMBOLS:
        raise _UsageError(f"more than {MAX_SYMBOLS} symbols")
    return Request(symbols=symbols, from_day=from_day)


def _unreached(symbol: SymbolSpec, code: str) -> SymbolResult:
    return {
        "symbol": symbol["symbol"],
        "currency": symbol["currency"],
        "error_code": code,
        "dates": [],
    }


def run(
    argv: Sequence[str],
    stdin: TextIO,
    out: TextIO,
    err: TextIO,
    clock: Clock,
    yf: Any,
) -> int:
    """Write the one answer to `out` and return 0; return 2 for a refused request."""
    try:
        parser = _Parser(prog="python -m optdesk.refearn", add_help=False)
        parser.add_argument("--deadline", type=float, required=True)
        args = parser.parse_args(list(argv))
        request = _request(stdin.read())
    except _UsageError as exc:
        err.write(f"usage error: {exc}\n")
        return 2
    deadline: float = args.deadline
    started = clock.monotonic()
    results: list[SymbolResult] = []
    stop = ""
    with tempfile.TemporaryDirectory(prefix="refearn-tz-") as cache_dir:
        yf.set_tz_cache_location(cache_dir)
        debug = yf.config.debug
        previous = debug.hide_exceptions
        debug.hide_exceptions = False

        def calendar_of(yahoo_symbol: str) -> Any:
            return yf.Ticker(yahoo_symbol).calendar

        fetch = make_fetcher(calendar_of)
        try:
            # yfinance may print; stdout carries the answer only.
            with contextlib.redirect_stdout(err):
                for index, symbol in enumerate(request["symbols"]):
                    if not stop:
                        if index > 0:
                            clock.sleep(GAP_SECONDS)
                        if clock.monotonic() - started >= deadline:
                            stop = "timeout"
                    if stop:
                        results.append(_unreached(symbol, stop))
                        continue
                    result = fetch(symbol, request["from_day"])
                    results.append(result)
                    if result["error_code"] == "rate_limited":
                        stop = "rate_limited"
        finally:
            debug.hide_exceptions = previous
    answer: Answer = {"results": results}
    out.write(json.dumps(answer))
    out.write("\n")
    return 0
