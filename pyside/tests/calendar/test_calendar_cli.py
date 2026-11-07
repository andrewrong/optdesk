"""Tests of run: the command line, with a fake clock and a temporary directory."""

from __future__ import annotations

import datetime
import io
import json
from importlib import metadata
from pathlib import Path

import pytest

from optdesk.calendar.cli import run
from optdesk.calendar.generate import GENERATOR_VERSION

CLOCK = datetime.datetime(2024, 1, 5, 6, 7, 8, tzinfo=datetime.UTC)


def _now() -> datetime.datetime:
    return CLOCK


def _run(argv: list[str]) -> tuple[int, str]:
    out, err = io.StringIO(), io.StringIO()
    code = run(argv, _now, out, err)
    return code, err.getvalue()


def _good(out: Path) -> list[str]:
    return [
        "--calendar", "XNYS", "--from", "2023-06-01", "--to", "2023-07-31", "--out", str(out),
    ]  # fmt: skip


def test_writes_file_with_provenance(tmp_path: Path) -> None:
    target = tmp_path / "days.json"
    code, _ = _run(_good(target))
    assert code == 0
    data = json.loads(target.read_text(encoding="utf-8"))
    assert data["provenance"] == {
        "generator": "optdesk.calendar",
        "generator_version": GENERATOR_VERSION,
        "source_library": "exchange_calendars",
        "source_library_version": metadata.version("exchange_calendars"),
        "generated_at": "2024-01-05T06:07:08Z",
    }
    assert len(data["rows"]) == 61


def test_two_runs_are_byte_identical(tmp_path: Path) -> None:
    a, b = tmp_path / "a.json", tmp_path / "b.json"
    assert _run(_good(a))[0] == 0
    assert _run(_good(b))[0] == 0
    assert a.read_bytes() == b.read_bytes()


def _bad_cases(out: Path) -> dict[str, list[str]]:
    o = str(out)
    return {
        "invalid date": ["--calendar", "XNYS", "--from", "2023-13-01", "--to", "2023-07-31", "--out", o],
        "to before from": ["--calendar", "XNYS", "--from", "2023-07-31", "--to", "2023-06-01", "--out", o],
        "missing out": ["--calendar", "XNYS", "--from", "2023-06-01", "--to", "2023-07-31"],
        "no calendar": ["--from", "2023-06-01", "--to", "2023-07-31", "--out", o],
        "repeated calendar": ["--calendar", "XNYS", "--calendar", "XNYS", "--from", "2023-06-01", "--to", "2023-07-31", "--out", o],
        "unsupported id": ["--calendar", "XLON", "--from", "2023-06-01", "--to", "2023-07-31", "--out", o],
        "unknown option": ["--bogus", "--out", o],
    }  # fmt: skip


@pytest.mark.parametrize(
    "case",
    [
        "invalid date",
        "to before from",
        "missing out",
        "no calendar",
        "repeated calendar",
        "unsupported id",
        "unknown option",
    ],
)
def test_usage_errors_return_2_and_write_nothing(tmp_path: Path, case: str) -> None:
    target = tmp_path / "days.json"
    code, err = _run(_bad_cases(target)[case])
    assert code == 2
    assert err != ""
    assert list(tmp_path.iterdir()) == []


def test_unwritable_out_returns_1(tmp_path: Path) -> None:
    code, _ = _run(_good(tmp_path / "missing_dir" / "days.json"))
    assert code == 1
