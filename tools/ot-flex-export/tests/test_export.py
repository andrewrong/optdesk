import json
from pathlib import Path

import duckdb
import pytest

from ot_flex_export.export import SchemaError, export, main

DDL = (
    "CREATE TABLE flex_trade_row (alias VARCHAR, level_of_detail VARCHAR, "
    "transaction_id VARCHAR, revision INTEGER, attributes VARCHAR, "
    "attributes_sha256 VARCHAR, recorded_at TIMESTAMP)"
)

ROWS = [
    ("B", "EXECUTION", "20", 1, '{"symbol":"ZZZ"}', "h3", "2026-01-03 10:00:00"),
    ("A", "EXECUTION", "9", 2, '{"symbol":"YYY","n":2}', "h2", "2026-01-02 10:00:00"),
    ("A", "EXECUTION", "10", 1, '{"symbol":"XXX"}', "h1", "2026-01-01 10:00:00"),
    ("A", "EXECUTION", "9", 1, '{"symbol":"YYY","n":1}', "h0", "2026-01-01 09:30:15.5"),
]


def make_db(path: Path, ddl: str = DDL, rows: list[tuple] | None = ROWS) -> Path:
    con = duckdb.connect(str(path))
    con.execute(ddl)
    if rows:
        con.executemany("INSERT INTO flex_trade_row VALUES (?, ?, ?, ?, ?, ?, ?)", rows)
    con.close()
    return path


def test_output_order_and_values(tmp_path: Path) -> None:
    db = make_db(tmp_path / "a.duckdb")
    out = tmp_path / "o.jsonl"
    assert export(db, out) == 4
    lines = out.read_text(encoding="utf-8").splitlines()
    objs = [json.loads(line) for line in lines]
    assert [(o["alias"], o["transaction_id"], o["revision"]) for o in objs] == [
        ("A", "10", 1),
        ("A", "9", 1),
        ("A", "9", 2),
        ("B", "20", 1),
    ]
    assert list(objs[1]) == [
        "alias", "level_of_detail", "transaction_id", "revision",
        "attributes", "attributes_sha256", "recorded_at",
    ]
    assert objs[1] == {
        "alias": "A",
        "level_of_detail": "EXECUTION",
        "transaction_id": "9",
        "revision": 1,
        "attributes": '{"symbol":"YYY","n":1}',
        "attributes_sha256": "h0",
        "recorded_at": "2026-01-01T09:30:15.500000",
    }


def test_opens_read_only(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    db = make_db(tmp_path / "a.duckdb")
    seen: list[dict] = []
    real = duckdb.connect

    def spy(*args: object, **kwargs: object) -> duckdb.DuckDBPyConnection:
        seen.append(dict(kwargs))
        return real(*args, **kwargs)  # type: ignore[arg-type]

    monkeypatch.setattr(duckdb, "connect", spy)
    export(db, tmp_path / "o.jsonl")
    assert seen == [{"read_only": True}]
    # a missing file is not created
    missing = tmp_path / "none.duckdb"
    with pytest.raises(duckdb.Error):
        export(missing, tmp_path / "o2.jsonl")
    assert not missing.exists()


def test_refuses_other_columns(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    ddl = DDL.replace("recorded_at TIMESTAMP", "recorded_at TIMESTAMP, extra VARCHAR")
    db = make_db(tmp_path / "a.duckdb", ddl, None)
    out = tmp_path / "o.jsonl"
    with pytest.raises(SchemaError):
        export(db, out)
    assert not out.exists()
    assert main([str(db), str(out)]) == 1
    assert "refused" in capsys.readouterr().out


def test_prints_counts_only(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    db = make_db(tmp_path / "a.duckdb")
    assert main([str(db), str(tmp_path / "o.jsonl")]) == 0
    printed = capsys.readouterr().out
    assert "4" in printed
    for secret in ("XXX", "YYY", "ZZZ", "symbol"):
        assert secret not in printed
