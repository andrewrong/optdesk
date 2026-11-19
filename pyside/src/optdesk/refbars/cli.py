"""The command `python -m optdesk.refbars`: one JSON request in, one JSON answer out."""

from __future__ import annotations

import argparse
import contextlib
import json
import tempfile
from collections.abc import Sequence
from typing import Any, NoReturn, TextIO

from optdesk.refbars.api import Answer, Clock, PairResult, PairSpec, Request
from optdesk.refbars.fetch import make_fetcher

MAX_PAIRS = 10
GAP_SECONDS = 1.0


class _UsageError(Exception):
    pass


class _Parser(argparse.ArgumentParser):
    def error(self, message: str) -> NoReturn:
        raise _UsageError(message)


def _request(text: str) -> Request:
    try:
        raw = json.loads(text)
        pairs = [
            PairSpec(symbol=str(p["symbol"]), currency=str(p["currency"])) for p in raw["pairs"]
        ]
        request = Request(pairs=pairs, from_day=str(raw["from_day"]), to_day=str(raw["to_day"]))
    except (ValueError, KeyError, TypeError):
        raise _UsageError("the request is not the JSON of the interface") from None
    if len(request["pairs"]) > MAX_PAIRS:
        raise _UsageError(f"more than {MAX_PAIRS} pairs")
    return request


def _unreached(pair: PairSpec, code: str) -> PairResult:
    return {"symbol": pair["symbol"], "currency": pair["currency"], "error_code": code, "bars": []}


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
        parser = _Parser(prog="python -m optdesk.refbars", add_help=False)
        parser.add_argument("--deadline", type=float, required=True)
        args = parser.parse_args(list(argv))
        request = _request(stdin.read())
    except _UsageError as exc:
        err.write(f"usage error: {exc}\n")
        return 2
    deadline: float = args.deadline
    started = clock.monotonic()
    results: list[PairResult] = []
    stop = ""
    with tempfile.TemporaryDirectory(prefix="refbars-tz-") as cache_dir:
        yf.set_tz_cache_location(cache_dir)
        fetch = make_fetcher(yf)
        # yfinance may print; stdout carries the answer only.
        with contextlib.redirect_stdout(err):
            for index, pair in enumerate(request["pairs"]):
                if not stop:
                    if index > 0:
                        clock.sleep(GAP_SECONDS)
                    if clock.monotonic() - started >= deadline:
                        stop = "timeout"
                if stop:
                    results.append(_unreached(pair, stop))
                    continue
                result = fetch(pair, request["from_day"], request["to_day"])
                results.append(result)
                if result["error_code"] == "rate_limited":
                    stop = "rate_limited"
    answer: Answer = {"results": results}
    out.write(json.dumps(answer))
    out.write("\n")
    return 0
