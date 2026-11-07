"""Builds and renders the session-day file from exchange_calendars (offline, no clock)."""

from __future__ import annotations

import datetime
import json
from collections.abc import Sequence
from importlib import metadata
from typing import Any

import exchange_calendars

from optdesk.calendar.api import (
    SUPPORTED_CALENDARS,
    CalendarFile,
    CalendarProvenance,
    GenerateError,
    SessionRow,
)

# Changed by hand in the same pull request as any change of the rules of the interface.
GENERATOR_VERSION = "1"

_GENERATOR = "optdesk.calendar"
_LIBRARY = "exchange_calendars"
_UTC = datetime.UTC


def to_utc(value: Any) -> datetime.datetime:
    """Convert a library timestamp or datetime to timezone-aware UTC; a naive one is an error."""
    moment: datetime.datetime = (
        value.to_pydatetime() if hasattr(value, "to_pydatetime") else value
    )
    if not isinstance(moment, datetime.datetime) or moment.utcoffset() is None:
        raise GenerateError("the library gave an instant without a timezone")
    return moment.astimezone(_UTC)


def _format_instant(moment: datetime.datetime) -> str:
    return moment.astimezone(_UTC).strftime("%Y-%m-%dT%H:%M:%SZ")


def _rows_for(
    calendar_id: str, first: datetime.date, last: datetime.date
) -> list[SessionRow]:
    try:
        calendar = exchange_calendars.get_calendar(
            calendar_id, start=first.isoformat(), end=last.isoformat()
        )
        sessions = {
            ts.date(): ts for ts in calendar.sessions if first <= ts.date() <= last
        }
        early = {ts.date() for ts in calendar.early_closes}
        opens = {day: to_utc(calendar.opens[ts]) for day, ts in sessions.items()}
        closes = {day: to_utc(calendar.closes[ts]) for day, ts in sessions.items()}
    except GenerateError:
        raise
    except Exception as exc:
        raise GenerateError(
            f"calendar {calendar_id} cannot be built for {first.isoformat()}..{last.isoformat()}"
        ) from exc
    rows: list[SessionRow] = []
    for offset in range((last - first).days + 1):
        day = first + datetime.timedelta(days=offset)
        if day in sessions:
            rows.append(
                SessionRow(
                    calendar_id, day, True, opens[day], closes[day], day in early
                )
            )
        else:
            sentinel = datetime.datetime.combine(day, datetime.time.min, _UTC)
            rows.append(SessionRow(calendar_id, day, False, sentinel, sentinel, False))
    return rows


def build_file(
    calendar_ids: Sequence[str],
    first: datetime.date,
    last: datetime.date,
    generated_at: datetime.datetime,
) -> CalendarFile:
    """Build the file for the ids over first..last; raise GenerateError on a bad input."""
    if last < first:
        raise GenerateError(
            f"range {first.isoformat()}..{last.isoformat()} ends before it starts"
        )
    if generated_at.utcoffset() is None:
        raise GenerateError("generated_at has no timezone")
    seen: set[str] = set()
    for calendar_id in calendar_ids:
        if calendar_id not in SUPPORTED_CALENDARS:
            raise GenerateError(f"calendar id {calendar_id!r} is not supported")
        if calendar_id in seen:
            raise GenerateError(f"calendar id {calendar_id!r} is repeated")
        seen.add(calendar_id)
    rows: list[SessionRow] = []
    for calendar_id in calendar_ids:
        rows.extend(_rows_for(calendar_id, first, last))
    provenance = CalendarProvenance(
        generator=_GENERATOR,
        generator_version=GENERATOR_VERSION,
        source_library=_LIBRARY,
        source_library_version=metadata.version(_LIBRARY),
        generated_at=_format_instant(generated_at),
    )
    return CalendarFile(provenance, tuple(rows))


def render_file(file: CalendarFile) -> str:
    """Render the file as JSON, one row per line, keys in the order of the dataset schema."""
    p = file.provenance
    head = {
        "generator": p.generator,
        "generator_version": p.generator_version,
        "source_library": p.source_library,
        "source_library_version": p.source_library_version,
        "generated_at": p.generated_at,
    }
    lines = []
    for r in file.rows:
        row = {
            "calendar_id": r.calendar_id,
            "session_day": r.session_day.isoformat(),
            "is_session": r.is_session,
            "open_at": _format_instant(r.open_at),
            "close_at": _format_instant(r.close_at),
            "early_close": r.early_close,
        }
        lines.append("    " + json.dumps(row))
    body = ",\n".join(lines)
    return (
        '{\n  "provenance": '
        + json.dumps(head)
        + ',\n  "rows": [\n'
        + body
        + "\n  ]\n}\n"
    )
