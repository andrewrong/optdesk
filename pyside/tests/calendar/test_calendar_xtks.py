"""XTKS through build_file on a stub of exchange_calendars.get_calendar (no library data)."""

from __future__ import annotations

import datetime
from typing import Any

import exchange_calendars
import pytest

from optdesk.calendar.api import SUPPORTED_CALENDARS
from optdesk.calendar.generate import build_file

SESSION_DAY = datetime.date(2026, 11, 9)
MISSING_WEEKDAY = datetime.date(2026, 11, 23)
OPEN = datetime.datetime(2026, 11, 9, 0, 0, tzinfo=datetime.UTC)
CLOSE = datetime.datetime(2026, 11, 9, 6, 30, tzinfo=datetime.UTC)
CLOCK = datetime.datetime(2026, 11, 9, tzinfo=datetime.UTC)


class _StubCalendar:
    """One session; the lunch break is not a column, so open and close are the whole span."""

    def __init__(self) -> None:
        day = datetime.datetime(2026, 11, 9)
        self.sessions = [day]
        self.opens = {day: OPEN}
        self.closes = {day: CLOSE}
        self.early_closes: list[datetime.datetime] = []


@pytest.fixture(autouse=True)
def _stub(monkeypatch: pytest.MonkeyPatch) -> None:
    def get_calendar(name: str, **_: Any) -> _StubCalendar:
        assert name == "XTKS"
        return _StubCalendar()

    monkeypatch.setattr(exchange_calendars, "get_calendar", get_calendar)


def test_xtks_is_supported() -> None:
    assert "XTKS" in SUPPORTED_CALENDARS
    file = build_file(["XTKS"], SESSION_DAY, SESSION_DAY, CLOCK)
    assert [r.calendar_id for r in file.rows] == ["XTKS"]


def test_xtks_rows_use_first_open_and_last_close() -> None:
    (row,) = build_file(["XTKS"], SESSION_DAY, SESSION_DAY, CLOCK).rows
    assert row.is_session
    assert row.open_at == OPEN
    assert row.close_at == CLOSE
    assert not row.early_close


def test_xtks_non_session_day_has_sentinel_row() -> None:
    rows = build_file(["XTKS"], SESSION_DAY, MISSING_WEEKDAY, CLOCK).rows
    row = rows[-1]
    sentinel = datetime.datetime(2026, 11, 23, tzinfo=datetime.UTC)
    assert row.session_day == MISSING_WEEKDAY
    assert not row.is_session
    assert row.open_at == sentinel
    assert row.close_at == sentinel
    assert not row.early_close
