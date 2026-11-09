"""Write the old figures of the parallel-run days as figure.jsonl.

Two sources: saved page JSON read through the `pages` paths of the figure register
(`PAGES_DIR/<DAY>/<page>.json`, the first segment of a path names the page file), and the rows
of the stored `reading` table. Only days from `figures_from` to `figures_to` are written.
"""

import json
import tomllib
from datetime import date
from decimal import Decimal, InvalidOperation
from pathlib import Path

import duckdb

from ot_history_export.accounts import check_columns

DEFAULT_REGISTER = Path("docs/conventions/register.toml")

READING_COLUMNS = (
    "name", "version", "subject_kind", "subject", "day", "rev", "value", "unit", "state",
)


def _decimal_text(v: object) -> str | None:
    """Decimal text of a number or numeric string; None for anything else (never zero)."""
    if isinstance(v, bool) or v is None:
        return None
    try:
        d = Decimal(str(v))
    except InvalidOperation:
        return None
    return format(d, "f") if d.is_finite() else None


def _places(text: str) -> int:
    return len(text.split(".")[1]) if "." in text else 0


def _walk(doc: object, segments: list[str]) -> object:
    """Follow `segments` through dicts and lists (a digit segment indexes a list); KeyError if absent."""
    for s in segments:
        if isinstance(doc, dict) and s in doc:
            doc = doc[s]
        elif isinstance(doc, list) and s.isdigit() and int(s) < len(doc):
            doc = doc[int(s)]
        else:
            raise KeyError(s)
    return doc


def _line(name: str, version: int, kind: str, subject: str, day: date, text: str, src: str) -> dict:
    return {
        "name": name,
        "version": version,
        "subject_kind": kind,
        "subject": subject,
        "day": day.isoformat(),
        "value": text,
        "places": _places(text),
        "from": src,
    }


def _page_lines(
    pages_dir: Path, register: Path, start: date, end: date, counts: dict[str, int]
) -> list[dict]:
    with register.open("rb") as f:
        figures = tomllib.load(f).get("figure", [])
    lines: list[dict] = []
    for day_dir in sorted(p for p in pages_dir.iterdir() if p.is_dir()):
        try:
            day = date.fromisoformat(day_dir.name)
        except ValueError:
            continue
        if not start <= day <= end:
            continue
        cache: dict[str, object] = {}
        for fig in figures:
            written = False
            for path in fig.get("pages", []):
                page, *rest = path.split(".")
                if page not in cache:
                    file = day_dir / f"{page}.json"
                    cache[page] = (
                        json.loads(file.read_text(encoding="utf-8")) if file.is_file() else None
                    )
                try:
                    raw = _walk(cache[page], rest) if cache[page] is not None else _missing()
                except KeyError:
                    counts["path_not_found"] += 1
                    continue
                text = _decimal_text(raw)
                if text is None:
                    counts["no_old_value"] += 1
                    continue
                if not written:
                    lines.append(_line(fig["id"], fig["version"], "", "", day, text, "page"))
                    written = True
    return lines


def _missing() -> object:
    raise KeyError("page")


def _reading_lines(
    con: duckdb.DuckDBPyConnection, start: date, end: date, counts: dict[str, int]
) -> list[dict]:
    rows = con.execute(
        "SELECT name, version, subject_kind, subject, day, value FROM ("
        "SELECT *, row_number() OVER (PARTITION BY name, version, subject_kind, subject, day "
        "ORDER BY rev DESC) AS rn FROM reading WHERE day BETWEEN ? AND ?) "
        "WHERE rn = 1 ORDER BY name, version, subject_kind, subject, day",
        [start, end],
    ).fetchall()
    lines: list[dict] = []
    for name, version, kind, subject, day, value in rows:
        text = _decimal_text(value)
        if text is None:
            counts["no_old_value"] += 1
            continue
        lines.append(_line(name, version, kind, subject, day, text, "reading"))
    return lines


def export_figures(
    db: Path,
    pages_dir: Path,
    outdir: Path,
    figures_from: date,
    figures_to: date,
    register: Path = DEFAULT_REGISTER,
) -> dict[str, int]:
    """Write `outdir/figure.jsonl` for the days `figures_from` to `figures_to` inclusive.

    A line has name, version, subject_kind, subject, day, value (decimal text), places (of the
    old display, the digits after the point of the old value) and from (`page` or `reading`).
    A `pages` path missing in a saved page is counted `path_not_found`; a found value that is
    null or not a number is counted `no_old_value`; neither is written, never as zero.
    Returns the counts: lines, from_page, from_reading, path_not_found, no_old_value.
    """
    counts = {"path_not_found": 0, "no_old_value": 0}
    con = duckdb.connect(str(db), read_only=True)
    try:
        check_columns(con, "reading", READING_COLUMNS)
        reading = _reading_lines(con, figures_from, figures_to, counts)
    finally:
        con.close()
    page = _page_lines(pages_dir, register, figures_from, figures_to, counts)
    outdir.mkdir(parents=True, exist_ok=True)
    with (outdir / "figure.jsonl").open("w", encoding="utf-8") as f:
        for line in page + reading:
            f.write(json.dumps(line, ensure_ascii=False))
            f.write("\n")
    return {
        "lines": len(page) + len(reading),
        "from_page": len(page),
        "from_reading": len(reading),
        **counts,
    }
