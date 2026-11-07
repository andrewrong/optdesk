"""Contract test of optdesk.calendar.api, from both sides: fakes and a caller."""

from __future__ import annotations

import dataclasses
import datetime
import io
import json
from collections.abc import Callable, Sequence
from typing import TextIO

import pytest

from optdesk.calendar.api import (
    SUPPORTED_CALENDARS,
    Builder,
    CalendarFile,
    CalendarProvenance,
    Command,
    GenerateError,
    Renderer,
    SessionRow,
)

UTC = datetime.UTC
EARLY_DAY = datetime.date(2024, 1, 4)
WEEKEND = (5, 6)


class _FakeBuilder:
    def __call__(
        self,
        calendar_ids: Sequence[str],
        first: datetime.date,
        last: datetime.date,
        generated_at: datetime.datetime,
    ) -> CalendarFile:
        if last < first:
            raise GenerateError("range 2024-01-05..2024-01-01 is backwards")
        if len(set(calendar_ids)) != len(calendar_ids):
            raise GenerateError("duplicate calendar id")
        rows: list[SessionRow] = []
        for cid in calendar_ids:
            day = first
            while day <= last:
                if day.weekday() in WEEKEND:
                    sentinel = datetime.datetime.combine(day, datetime.time.min, tzinfo=UTC)
                    rows.append(SessionRow(cid, day, False, sentinel, sentinel, False))
                else:
                    start = datetime.datetime.combine(day, datetime.time(hour=14), tzinfo=UTC)
                    span = datetime.timedelta(hours=3 if day == EARLY_DAY else 6)
                    rows.append(SessionRow(cid, day, True, start, start + span, day == EARLY_DAY))
                day += datetime.timedelta(days=1)
        stamp = generated_at.isoformat()
        return CalendarFile(CalendarProvenance("fake", "1", "lib", "1", stamp), tuple(rows))


class _FakeRenderer:
    def __call__(self, file: CalendarFile) -> str:
        provenance = dataclasses.asdict(file.provenance)
        rows = [
            {
                "calendar_id": r.calendar_id,
                "session_day": r.session_day.isoformat(),
                "is_session": r.is_session,
                "open_at": r.open_at.isoformat(),
                "close_at": r.close_at.isoformat(),
                "early_close": r.early_close,
            }
            for r in file.rows
        ]
        return json.dumps({"provenance": provenance, "rows": rows}) + "\n"


class _FakeCommand:
    def __call__(
        self,
        argv: Sequence[str],
        now: Callable[[], datetime.datetime],
        out: TextIO,
        err: TextIO,
    ) -> int:
        if not argv:
            err.write("usage\n")
            return 2
        out.write(now().isoformat())
        return 0


def _build(ids: Sequence[str], first: datetime.date, last: datetime.date) -> CalendarFile:
    builder: Builder = _FakeBuilder()
    return builder(ids, first, last, datetime.datetime(2024, 2, 1, tzinfo=UTC))


FIRST = datetime.date(2024, 1, 1)
LAST = datetime.date(2024, 1, 10)


def test_fakes_satisfy_protocols() -> None:
    assert isinstance(_FakeBuilder(), Builder)
    assert isinstance(_FakeRenderer(), Renderer)
    assert isinstance(_FakeCommand(), Command)


def test_rule1_one_row_per_day_ordered() -> None:
    rows = _build(["XNYS", "AAAA"], FIRST, LAST).rows
    days = (LAST - FIRST).days + 1
    assert len(rows) == 2 * days
    assert [r.calendar_id for r in rows] == ["XNYS"] * days + ["AAAA"] * days
    for start in (0, days):
        got = [r.session_day for r in rows[start : start + days]]
        assert got == [FIRST + datetime.timedelta(days=i) for i in range(days)]


def test_rule2_session_day() -> None:
    sessions = [r for r in _build(["XNYS"], FIRST, LAST).rows if r.is_session]
    assert sessions
    for r in sessions:
        assert r.open_at < r.close_at
        assert r.open_at.utcoffset() == datetime.timedelta(0)
        assert r.close_at.utcoffset() == datetime.timedelta(0)
    assert [r.early_close for r in sessions if r.session_day == EARLY_DAY] == [True]
    assert not any(r.early_close for r in sessions if r.session_day != EARLY_DAY)


def test_rule3_non_session_day() -> None:
    closed = [r for r in _build(["XNYS"], FIRST, LAST).rows if not r.is_session]
    assert closed
    for r in closed:
        assert r.early_close is False
        midnight = datetime.datetime.combine(r.session_day, datetime.time.min, tzinfo=UTC)
        assert r.open_at == midnight
        assert r.close_at == midnight


def test_builder_error_carries_message() -> None:
    with pytest.raises(GenerateError) as info:
        _build(["XNYS"], LAST, FIRST)
    assert str(info.value) != ""
    assert isinstance(info.value, Exception)


def test_generate_error_message() -> None:
    assert str(GenerateError("bad id ZZZZ")) == "bad id ZZZZ"


def test_supported_calendars() -> None:
    assert SUPPORTED_CALENDARS == ("XNYS",)


def test_renderer_text_is_json_with_keys() -> None:
    renderer: Renderer = _FakeRenderer()
    decoded = json.loads(renderer(_build(["XNYS"], FIRST, LAST)))
    assert set(decoded) == {"provenance", "rows"}
    assert len(decoded["rows"]) == 10


def test_command_through_protocol() -> None:
    command: Command = _FakeCommand()
    out, err = io.StringIO(), io.StringIO()
    assert command([], lambda: datetime.datetime(2024, 1, 1, tzinfo=UTC), out, err) == 2
    assert err.getvalue() != ""
    assert command(["x"], lambda: datetime.datetime(2024, 1, 1, tzinfo=UTC), out, err) == 0
    assert out.getvalue().startswith("2024-01-01")


def test_dataclasses_are_frozen() -> None:
    row = _build(["XNYS"], FIRST, FIRST).rows[0]
    with pytest.raises(dataclasses.FrozenInstanceError):
        row.is_session = True  # type: ignore[misc]
