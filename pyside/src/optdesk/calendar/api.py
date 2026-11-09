"""Interface of optdesk.calendar: declarations only, standard library only.

The file format is the one `ot calendar load` reads. Rules every producer keeps:

1. One row for every calendar day from `first` to `last` inclusive, per calendar id,
   ordered by id as given and then by `session_day` ascending; each id appears once.
2. A session day: `is_session` true, `open_at` before `close_at`; `early_close` true
   exactly when the source lists the session as an early close.
3. A day that is not a session: `is_session` false, `early_close` false, and both
   instants at 00:00:00Z of its `session_day` (a sentinel, not an instant).
"""

from __future__ import annotations

import datetime
from collections.abc import Callable, Sequence
from dataclasses import dataclass
from typing import Protocol, TextIO, runtime_checkable

SUPPORTED_CALENDARS: tuple[str, ...] = ("XNYS", "XTKS")


@dataclass(frozen=True)
class CalendarProvenance:
    """Where a file came from; the five strings of the file header."""

    generator: str
    generator_version: str
    source_library: str
    source_library_version: str
    generated_at: str


@dataclass(frozen=True)
class SessionRow:
    """One calendar day of one calendar; `open_at` and `close_at` are timezone-aware UTC."""

    calendar_id: str
    session_day: datetime.date
    is_session: bool
    open_at: datetime.datetime
    close_at: datetime.datetime
    early_close: bool


@dataclass(frozen=True)
class CalendarFile:
    """A provenance header and the rows of the file."""

    provenance: CalendarProvenance
    rows: tuple[SessionRow, ...]


class GenerateError(Exception):
    """An id outside the supported set, an id the library rejects, a bad range or a duplicate id."""


@runtime_checkable
class Builder(Protocol):
    """Builds the file for the ids over first..last; `generated_at` is an argument, so no clock."""

    def __call__(
        self,
        calendar_ids: Sequence[str],
        first: datetime.date,
        last: datetime.date,
        generated_at: datetime.datetime,
    ) -> CalendarFile:
        """Return the file; raise GenerateError on a bad id, a duplicate id or a bad range."""
        ...


@runtime_checkable
class Renderer(Protocol):
    """Renders a file as JSON text."""

    def __call__(self, file: CalendarFile) -> str:
        """Return the JSON text: keys in the order of master.session_day.r1.json,
        instants as RFC 3339 with the suffix Z, one row per line, a final newline."""
        ...


@runtime_checkable
class Command(Protocol):
    """The command line `python -m optdesk.calendar`."""

    def __call__(
        self,
        argv: Sequence[str],
        now: Callable[[], datetime.datetime],
        out: TextIO,
        err: TextIO,
    ) -> int:
        """Run `--calendar ID [--calendar ID ...] --from YYYY-MM-DD --to YYYY-MM-DD --out PATH`;
        return 0, 2 for a usage or range error, 1 otherwise."""
        ...
