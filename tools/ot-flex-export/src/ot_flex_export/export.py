"""Write every revision of flex_trade_row of an option-trader DuckDB file as JSON Lines."""

import json
from pathlib import Path

import duckdb

TABLE = "flex_trade_row"

# Columns of option-trader's flex/store.py, in the order of Decision 7.
COLUMNS = (
    "alias",
    "level_of_detail",
    "transaction_id",
    "revision",
    "attributes",
    "attributes_sha256",
    "recorded_at",
)


class SchemaError(Exception):
    """The table does not have exactly the expected columns."""


def export(db: Path, out: Path) -> int:
    """Write the rows of `db` to `out`, one JSON object per line; return the row count.

    Rows are ordered by alias, level of detail, transaction id and revision.
    `attributes` and `attributes_sha256` are written as stored; `recorded_at` as ISO text.
    """
    con = duckdb.connect(str(db), read_only=True)
    try:
        found = tuple(
            r[0]
            for r in con.execute(
                "SELECT column_name FROM information_schema.columns "
                "WHERE table_name = ? ORDER BY ordinal_position",
                [TABLE],
            ).fetchall()
        )
        if found != COLUMNS:
            raise SchemaError(
                f"table {TABLE} has columns {list(found)}, expected {list(COLUMNS)}"
            )
        rows = con.execute(
            "SELECT alias, level_of_detail, transaction_id, revision, attributes, "
            "attributes_sha256, recorded_at "
            f"FROM {TABLE} ORDER BY alias, level_of_detail, transaction_id, revision"
        ).fetchall()
    finally:
        con.close()
    with out.open("w", encoding="utf-8") as f:
        for row in rows:
            obj = dict(zip(COLUMNS, row, strict=True))
            obj["recorded_at"] = obj["recorded_at"].isoformat()
            f.write(json.dumps(obj, ensure_ascii=False))
            f.write("\n")
    return len(rows)


def main(argv: list[str]) -> int:
    """Command line: DB OUT.jsonl. Prints counts only, never an attribute value."""
    if len(argv) != 2:
        print("usage: python -m ot_flex_export DB OUT.jsonl")
        return 2
    try:
        count = export(Path(argv[0]), Path(argv[1]))
    except SchemaError as e:
        print(f"refused: {e}")
        return 1
    except duckdb.Error as e:
        print(f"cannot read the database: {type(e).__name__}")
        return 1
    print(f"rows written: {count}")
    return 0
