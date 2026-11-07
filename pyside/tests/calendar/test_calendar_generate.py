"""Tests of build_file and render_file on the real exchange_calendars library (offline)."""

from __future__ import annotations

import datetime
import json
import socket
from typing import Any

import pytest

from optdesk.calendar.api import (
    SUPPORTED_CALENDARS,
    CalendarFile,
    CalendarProvenance,
    GenerateError,
    SessionRow,
)
from optdesk.calendar.generate import GENERATOR_VERSION, build_file, render_file, to_utc

FIRST = datetime.date(2023, 1, 1)
LAST = datetime.date(2023, 12, 31)
CLOCK = datetime.datetime(2024, 1, 5, tzinfo=datetime.UTC)


def _year() -> CalendarFile:
    return build_file(["XNYS"], FIRST, LAST, CLOCK)


def check_like_loader(file: CalendarFile) -> None:
    """The rules of checkProvenance and checkRows of cmd/ot/calendar.go."""
    p = file.provenance
    for value in (
        p.generator,
        p.generator_version,
        p.source_library,
        p.source_library_version,
        p.generated_at,
    ):
        assert value != ""
    datetime.datetime.fromisoformat(p.generated_at)
    assert file.rows
    last: dict[str, datetime.date] = {}
    for row in file.rows:
        assert row.calendar_id != ""
        if row.calendar_id in last:
            assert row.session_day > last[row.calendar_id]
        last[row.calendar_id] = row.session_day
        if row.is_session:
            assert row.open_at < row.close_at
        else:
            assert row.open_at == row.close_at
            assert not row.early_close


def test_every_day_has_a_row_in_order() -> None:
    rows = _year().rows
    days = [r.session_day for r in rows]
    count = (LAST - FIRST).days + 1
    assert days == [FIRST + datetime.timedelta(days=i) for i in range(count)]
    assert {r.calendar_id for r in rows} == {"XNYS"}


def test_weekend_and_labor_day_are_not_sessions() -> None:
    by_day = {r.session_day: r for r in _year().rows}
    saturday = next(d for d in by_day if d.weekday() == 5)
    assert not by_day[saturday].is_session
    sept1 = datetime.date(LAST.year, 9, 1)
    labor_day = sept1 + datetime.timedelta(days=(7 - sept1.weekday()) % 7)
    assert labor_day.weekday() == 0
    assert not by_day[labor_day].is_session


def test_early_closes_are_shorter_than_the_longest_session() -> None:
    rows = _year().rows
    sessions = [r for r in rows if r.is_session]
    longest = max(r.close_at - r.open_at for r in sessions)
    early = [r for r in sessions if r.early_close]
    assert early
    for r in early:
        assert r.close_at - r.open_at < longest


def test_instants_are_utc_aware_and_ordered() -> None:
    for r in _year().rows:
        for t in (r.open_at, r.close_at):
            assert t.tzinfo is not None
            assert t.utcoffset() == datetime.timedelta(0)
        if r.is_session:
            assert r.open_at < r.close_at
        else:
            assert r.open_at == datetime.datetime.combine(
                r.session_day, datetime.time.min, datetime.UTC
            )


def test_rows_pass_the_loader_rules() -> None:
    f = _year()
    check_like_loader(f)
    assert f.provenance.generator_version == GENERATOR_VERSION


def test_range_beyond_the_library_default_builds() -> None:
    first = FIRST.replace(year=FIRST.year + 30)
    last = LAST.replace(year=LAST.year + 30)
    f = build_file(["XNYS"], first, last, CLOCK)
    assert any(r.is_session for r in f.rows)
    check_like_loader(f)


def test_unsupported_id_is_rejected() -> None:
    assert "XLON" not in SUPPORTED_CALENDARS
    with pytest.raises(GenerateError, match="XLON"):
        build_file(["XLON"], FIRST, LAST, CLOCK)
    with pytest.raises(GenerateError, match="NOPE"):
        build_file(["NOPE"], FIRST, LAST, CLOCK)


def test_repeated_id_is_rejected() -> None:
    with pytest.raises(GenerateError, match="XNYS"):
        build_file(["XNYS", "XNYS"], FIRST, LAST, CLOCK)


def test_last_before_first_is_rejected() -> None:
    with pytest.raises(GenerateError, match="2023-01-01") as info:
        build_file(["XNYS"], LAST, FIRST, CLOCK)
    assert "2023-12-31" in str(info.value)
    assert "/" not in str(info.value)


def test_naive_datetime_is_rejected_by_the_conversion() -> None:
    with pytest.raises(GenerateError):
        to_utc(datetime.datetime(2023, 1, 3, 12, 0))


def test_build_needs_no_network(monkeypatch: pytest.MonkeyPatch) -> None:
    def refuse(*args: Any, **kwargs: Any) -> None:
        raise AssertionError("network used")

    monkeypatch.setattr(socket.socket, "connect", refuse)
    check_like_loader(_year())


def test_render_shape() -> None:
    f = build_file(
        ["XNYS"], datetime.date(2023, 7, 3), datetime.date(2023, 7, 4), CLOCK
    )
    text = render_file(f)
    assert text.endswith("\n")
    data = json.loads(text)
    assert data["provenance"]["generated_at"] == "2024-01-05T00:00:00Z"
    assert data["provenance"]["generator"] == "optdesk.calendar"
    assert list(data["rows"][0]) == [
        "calendar_id",
        "session_day",
        "is_session",
        "open_at",
        "close_at",
        "early_close",
    ]
    assert data["rows"][0]["open_at"].endswith("Z")
    assert data["rows"][1]["is_session"] is False
    assert data["rows"][1]["open_at"] == "2023-07-04T00:00:00Z"
    row_lines = [ln for ln in text.splitlines() if '"calendar_id"' in ln]
    assert len(row_lines) == 2


def test_render_uses_fixed_values() -> None:
    day = datetime.date(2024, 3, 1)
    utc = datetime.UTC
    row = SessionRow(
        "X",
        day,
        True,
        datetime.datetime(2024, 3, 1, 1, 2, 3, tzinfo=utc),
        datetime.datetime(2024, 3, 1, 4, 5, 6, tzinfo=utc),
        True,
    )
    prov = CalendarProvenance("g", "1", "lib", "2", "2024-03-01T00:00:00Z")
    data = json.loads(render_file(CalendarFile(prov, (row,))))
    assert data["rows"][0] == {
        "calendar_id": "X",
        "session_day": "2024-03-01",
        "is_session": True,
        "open_at": "2024-03-01T01:02:03Z",
        "close_at": "2024-03-01T04:05:06Z",
        "early_close": True,
    }
