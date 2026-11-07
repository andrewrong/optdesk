"""The command line `python -m optdesk.calendar`; the clock and the streams are arguments."""

from __future__ import annotations

import argparse
import datetime
from collections.abc import Callable, Sequence
from pathlib import Path
from typing import NoReturn, TextIO

from optdesk.calendar.api import SUPPORTED_CALENDARS, GenerateError
from optdesk.calendar.generate import build_file, render_file


class _UsageError(Exception):
    pass


class _Parser(argparse.ArgumentParser):
    def error(self, message: str) -> NoReturn:
        raise _UsageError(message)


def _date(text: str) -> datetime.date:
    try:
        value = datetime.datetime.strptime(text, "%Y-%m-%d").date()
    except ValueError:
        raise argparse.ArgumentTypeError("expected a date as YYYY-MM-DD") from None
    if value.isoformat() != text:
        raise argparse.ArgumentTypeError("expected a date as YYYY-MM-DD")
    return value


def _parser() -> argparse.ArgumentParser:
    parser = _Parser(prog="python -m optdesk.calendar", add_help=False)
    parser.add_argument("--calendar", action="append", default=[])
    parser.add_argument("--from", dest="first", type=_date, required=True)
    parser.add_argument("--to", dest="last", type=_date, required=True)
    parser.add_argument("--out", required=True)
    return parser


def run(
    argv: Sequence[str],
    now: Callable[[], datetime.datetime],
    out: TextIO,
    err: TextIO,
) -> int:
    """Write the session-day file; return 0, 2 for a usage or range error, 1 otherwise."""
    try:
        args = _parser().parse_args(list(argv))
        ids: list[str] = args.calendar
        if not ids:
            raise _UsageError("at least one --calendar is required")
        if len(set(ids)) != len(ids):
            raise _UsageError("--calendar is repeated")
        for calendar_id in ids:
            if calendar_id not in SUPPORTED_CALENDARS:
                raise _UsageError(f"calendar id {calendar_id!r} is not supported")
        if args.last < args.first:
            raise _UsageError("--to is before --from")
    except _UsageError as exc:
        err.write(f"usage error: {exc}\n")
        return 2
    try:
        text = render_file(build_file(ids, args.first, args.last, now()))
    except GenerateError as exc:
        err.write(f"error: {exc}\n")
        return 1
    try:
        with Path(args.out).open("w", encoding="utf-8", newline="\n") as handle:
            handle.write(text)
    except OSError:
        err.write("error: the output file cannot be written\n")
        return 1
    out.write(
        f"wrote {len(ids)} calendar(s), {args.first.isoformat()}..{args.last.isoformat()}\n"
    )
    return 0
