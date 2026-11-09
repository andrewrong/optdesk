"""Write the last snapshot of each alias and session day of an option-trader DuckDB file as account.jsonl.

Reads only account_snapshot, account_position and account_summary. The column lists below are
the ones this exporter was written for; a table whose columns differ is refused.
"""

import json
import re
from datetime import date
from pathlib import Path

import duckdb

TABLES: dict[str, tuple[str, ...]] = {
    "account_snapshot": ("snapshot_id", "alias", "taken_at", "session_day"),
    "account_position": (
        "snapshot_id", "symbol", "sec_type", "exchange", "currency", "expiry", "strike",
        "multiplier", "option_right", "con_id", "quantity", "avg_cost",
    ),
    "account_summary": ("snapshot_id", "tag", "value", "currency"),
}

# An IBKR account id: letters then digits. Only aliases leave the old system.
_ACCOUNT_ID = re.compile(r"^[A-Za-z]{1,3}[0-9]{5,}$")


class SchemaError(Exception):
    """A table has other columns than expected, or the file holds an account id."""


def check_columns(con: duckdb.DuckDBPyConnection, table: str, expected: tuple[str, ...]) -> None:
    """Raise SchemaError unless `table` has exactly the `expected` columns, in order."""
    found = tuple(
        r[0]
        for r in con.execute(
            "SELECT column_name FROM information_schema.columns "
            "WHERE table_name = ? ORDER BY ordinal_position",
            [table],
        ).fetchall()
    )
    if found != expected:
        raise SchemaError(f"table {table} has columns {list(found)}, expected {list(expected)}")


def _text(v: object) -> str | None:
    return None if v is None else str(v)


def export_account(db: Path, outdir: Path, through: date) -> dict[str, int]:
    """Write `outdir/account.jsonl`: one line per alias and session day up to `through`.

    A line carries alias, session_day, captured_at (ISO text), positions and values; the
    snapshot of a day is the one with the latest taken_at (then the highest snapshot_id).
    Days after `through` are not written and are counted as `later_day_refused`.
    Quantities and prices are written as decimal text, a missing one as null, never zero.
    Returns the counts: lines, positions, values, later_day_refused.
    """
    con = duckdb.connect(str(db), read_only=True)
    try:
        for table, cols in TABLES.items():
            check_columns(con, table, cols)
        for (alias,) in con.execute("SELECT DISTINCT alias FROM account_snapshot").fetchall():
            if alias is not None and _ACCOUNT_ID.match(alias):
                raise SchemaError("an alias column holds a value that looks like an account id")
        later = con.execute(
            "SELECT count(*) FROM (SELECT DISTINCT alias, session_day FROM account_snapshot "
            "WHERE session_day > ?)",
            [through],
        ).fetchone()
        snaps = con.execute(
            "SELECT snapshot_id, alias, taken_at, session_day FROM ("
            "SELECT *, row_number() OVER (PARTITION BY alias, session_day "
            "ORDER BY taken_at DESC, snapshot_id DESC) AS rn FROM account_snapshot "
            "WHERE session_day <= ?) WHERE rn = 1 ORDER BY alias, session_day",
            [through],
        ).fetchall()
        lines: list[dict[str, object]] = []
        n_pos = n_val = 0
        for sid, alias, taken_at, day in snaps:
            positions = [
                {
                    k: (_text(v) if k in _DECIMALS else v)
                    for k, v in zip(TABLES["account_position"][1:], row, strict=True)
                }
                for row in con.execute(
                    "SELECT * EXCLUDE (snapshot_id) FROM account_position WHERE snapshot_id = ? "
                    "ORDER BY symbol, sec_type, expiry, strike, option_right, con_id",
                    [sid],
                ).fetchall()
            ]
            values = [
                {"tag": t, "value": _text(v), "currency": c}
                for t, v, c in con.execute(
                    "SELECT tag, value, currency FROM account_summary WHERE snapshot_id = ? "
                    "ORDER BY tag, currency",
                    [sid],
                ).fetchall()
            ]
            n_pos += len(positions)
            n_val += len(values)
            lines.append({
                "alias": alias,
                "session_day": day.isoformat(),
                "captured_at": taken_at.isoformat(),
                "positions": positions,
                "values": values,
            })
    finally:
        con.close()
    outdir.mkdir(parents=True, exist_ok=True)
    with (outdir / "account.jsonl").open("w", encoding="utf-8") as f:
        for line in lines:
            f.write(json.dumps(line, ensure_ascii=False))
            f.write("\n")
    return {
        "lines": len(lines),
        "positions": n_pos,
        "values": n_val,
        "later_day_refused": later[0] if later else 0,
    }


_DECIMALS = frozenset({"quantity", "avg_cost", "strike", "multiplier"})
