import json
from datetime import date
from pathlib import Path

import duckdb
import pytest

from ot_history_export.__main__ import main
from ot_history_export.accounts import SchemaError, export_account
from ot_history_export.figures import export_figures

DDL = [
    "CREATE TABLE account_snapshot (snapshot_id INTEGER, alias VARCHAR, taken_at TIMESTAMP, "
    "session_day DATE)",
    "CREATE TABLE account_position (snapshot_id INTEGER, symbol VARCHAR, sec_type VARCHAR, "
    "exchange VARCHAR, currency VARCHAR, expiry VARCHAR, strike DECIMAL(18,4), "
    "multiplier VARCHAR, option_right VARCHAR, con_id BIGINT, quantity DECIMAL(18,4), "
    "avg_cost DECIMAL(18,4))",
    "CREATE TABLE account_summary (snapshot_id INTEGER, tag VARCHAR, value DECIMAL(18,4), "
    "currency VARCHAR)",
    "CREATE TABLE reading (name VARCHAR, version INTEGER, subject_kind VARCHAR, subject VARCHAR, "
    "day DATE, rev INTEGER, value VARCHAR, unit VARCHAR, state VARCHAR)",
]

REGISTER = """
[[figure]]
id = "fig.one"
version = 1
pages = ["pg.card.a", "other.card.a"]

[[figure]]
id = "fig.two"
version = 2
pages = ["pg.card.missing"]

[[figure]]
id = "fig.three"
version = 1
pages = ["pg.card.nothing", "pg.list.1.v"]
"""


def make_db(path: Path, ddl: list[str] = DDL) -> duckdb.DuckDBPyConnection:
    con = duckdb.connect(str(path))
    for stmt in ddl:
        con.execute(stmt)
    return con


def seed_accounts(con: duckdb.DuckDBPyConnection) -> None:
    con.executemany(
        "INSERT INTO account_snapshot VALUES (?, ?, ?, ?)",
        [
            (1, "main", "2026-01-05 10:00:00", "2026-01-05"),
            (2, "main", "2026-01-05 15:30:00", "2026-01-05"),
            (3, "main", "2026-01-06 15:30:00", "2026-01-06"),
            (4, "side", "2026-01-05 15:00:00", "2026-01-05"),
        ],
    )
    con.executemany(
        "INSERT INTO account_position VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
        [
            (1, "AAA", "option", "SMART", "USD", "20260116", 100, "100", "put", 11, -1, 1.5),
            (2, "BBB", "option", "SMART", "USD", "20260116", 50, "100", "call", 12, -2, 2.25),
            (2, "CCC", "stock", "SMART", "USD", "", None, "1", "", 13, 10, 3),
        ],
    )
    con.executemany(
        "INSERT INTO account_summary VALUES (?, ?, ?, ?)",
        [(1, "NetLiquidation", 1000, "USD"), (2, "NetLiquidation", 1200.5, "USD"),
         (2, "Cash", None, "USD")],
    )


def read_lines(path: Path) -> list[dict]:
    return [json.loads(x) for x in path.read_text(encoding="utf-8").splitlines()]


def test_last_snapshot_of_day(tmp_path: Path) -> None:
    con = make_db(tmp_path / "a.duckdb")
    seed_accounts(con)
    con.close()
    counts = export_account(tmp_path / "a.duckdb", tmp_path / "out", date(2026, 1, 31))
    lines = read_lines(tmp_path / "out" / "account.jsonl")
    assert [(x["alias"], x["session_day"]) for x in lines] == [
        ("main", "2026-01-05"), ("main", "2026-01-06"), ("side", "2026-01-05"),
    ]
    first = lines[0]
    assert first["captured_at"] == "2026-01-05T15:30:00"
    assert [p["symbol"] for p in first["positions"]] == ["BBB", "CCC"]
    assert first["positions"][1]["strike"] is None
    assert first["positions"][0]["quantity"] == "-2.0000"
    assert first["values"] == [
        {"tag": "Cash", "value": None, "currency": "USD"},
        {"tag": "NetLiquidation", "value": "1200.5000", "currency": "USD"},
    ]
    assert counts == {"lines": 3, "positions": 2, "values": 2, "later_day_refused": 0}


def test_through_refuses_later_account_day(
    tmp_path: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    con = make_db(tmp_path / "a.duckdb")
    seed_accounts(con)
    con.close()
    with pytest.raises(SystemExit):
        main(["account", str(tmp_path / "a.duckdb"), str(tmp_path / "out")])
    assert not (tmp_path / "out").exists()
    counts = export_account(tmp_path / "a.duckdb", tmp_path / "out", date(2026, 1, 5))
    lines = read_lines(tmp_path / "out" / "account.jsonl")
    assert {x["session_day"] for x in lines} == {"2026-01-05"}
    assert counts["later_day_refused"] == 1
    capsys.readouterr()
    assert main(["account", str(tmp_path / "a.duckdb"), str(tmp_path / "o2"),
                 "--through", "2026-01-05"]) == 0
    printed = capsys.readouterr().out
    assert "later_day_refused: 1" in printed
    for secret in ("main", "side", "AAA", "1200"):
        assert secret not in printed


def test_refuses_account_id(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    con = make_db(tmp_path / "a.duckdb")
    seed_accounts(con)
    con.execute("INSERT INTO account_snapshot VALUES (9, 'X1234567', '2026-01-05 10:00:00', "
                "'2026-01-05')")
    con.close()
    with pytest.raises(SchemaError):
        export_account(tmp_path / "a.duckdb", tmp_path / "out", date(2026, 1, 31))
    assert not (tmp_path / "out" / "account.jsonl").exists()
    assert main(["account", str(tmp_path / "a.duckdb"), str(tmp_path / "out"),
                 "--through", "2026-01-31"]) == 1
    assert "X1234567" not in capsys.readouterr().out


def test_refuses_changed_columns(tmp_path: Path) -> None:
    ddl = [s.replace("currency VARCHAR)", "currency VARCHAR, extra VARCHAR)")
           if "account_summary" in s else s for s in DDL]
    make_db(tmp_path / "a.duckdb", ddl).close()
    with pytest.raises(SchemaError):
        export_account(tmp_path / "a.duckdb", tmp_path / "out", date(2026, 1, 31))
    ddl = [s.replace("unit VARCHAR, ", "") if "TABLE reading" in s else s for s in DDL]
    make_db(tmp_path / "b.duckdb", ddl).close()
    with pytest.raises(SchemaError):
        export_figures(tmp_path / "b.duckdb", tmp_path, tmp_path / "o",
                       date(2026, 1, 1), date(2026, 1, 31), tmp_path / "r.toml")
    assert not (tmp_path / "out" / "account.jsonl").exists()


def make_figures(tmp_path: Path) -> tuple[Path, Path, Path]:
    con = make_db(tmp_path / "f.duckdb")
    con.executemany(
        "INSERT INTO reading VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
        [
            ("r.level", 1, "index", "XYZ", "2026-01-05", 1, "10.5", "pt", "ok"),
            ("r.level", 1, "index", "XYZ", "2026-01-05", 2, "10.50", "pt", "ok"),
            ("r.level", 1, "index", "XYZ", "2026-01-06", 1, None, "pt", "gap"),
            ("r.level", 1, "index", "XYZ", "2026-01-09", 1, "99", "pt", "ok"),
            ("r.level", 1, "index", "XYZ", "2026-01-01", 1, "98", "pt", "ok"),
        ],
    )
    con.close()
    pages = tmp_path / "pages"
    (pages / "2026-01-05").mkdir(parents=True)
    (pages / "2026-01-05" / "pg.json").write_text(
        json.dumps({"card": {"a": 12.3, "nothing": None, "z": 0}, "list": [{"v": 1}, {"v": "7.250"}]})
    )
    (pages / "2026-01-09").mkdir()
    (pages / "2026-01-09" / "pg.json").write_text(json.dumps({"card": {"a": 5}}))
    (pages / "2025-12-31").mkdir()
    (pages / "2025-12-31" / "pg.json").write_text(json.dumps({"card": {"a": 6}}))
    reg = tmp_path / "register.toml"
    reg.write_text(REGISTER)
    return tmp_path / "f.duckdb", pages, reg


def test_figures_bound(tmp_path: Path) -> None:
    db, pages, reg = make_figures(tmp_path)
    export_figures(db, pages, tmp_path / "out", date(2026, 1, 2), date(2026, 1, 8), reg)
    lines = read_lines(tmp_path / "out" / "figure.jsonl")
    assert lines
    assert all("2026-01-02" <= x["day"] <= "2026-01-08" for x in lines)
    assert {x["from"] for x in lines} == {"page", "reading"}
    reading = [x for x in lines if x["from"] == "reading"]
    assert reading == [{
        "name": "r.level", "version": 1, "subject_kind": "index", "subject": "XYZ",
        "day": "2026-01-05", "value": "10.50", "places": 2, "from": "reading",
    }]


def test_path_not_found_counted(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    db, pages, reg = make_figures(tmp_path)
    counts = export_figures(db, pages, tmp_path / "out", date(2026, 1, 5), date(2026, 1, 5), reg)
    # fig.one other.card.a (no page file), fig.two pg.card.missing
    assert counts["path_not_found"] == 2
    lines = read_lines(tmp_path / "out" / "figure.jsonl")
    one = next(x for x in lines if x["name"] == "fig.one")
    assert (one["value"], one["from"], one["places"]) == ("12.3", "page", 1)
    assert main(["figures", str(db), str(pages), str(tmp_path / "o2"), "--figures-from",
                 "2026-01-05", "--figures-to", "2026-01-05", "--register", str(reg)]) == 0
    printed = capsys.readouterr().out
    assert "path_not_found: 2" in printed
    assert "12.3" not in printed


def test_missing_is_not_zero(tmp_path: Path) -> None:
    db, pages, reg = make_figures(tmp_path)
    counts = export_figures(db, pages, tmp_path / "out", date(2026, 1, 5), date(2026, 1, 6), reg)
    lines = read_lines(tmp_path / "out" / "figure.jsonl")
    names = {(x["name"], x["day"]) for x in lines}
    assert ("fig.two", "2026-01-05") not in names
    assert ("r.level", "2026-01-06") not in names  # null reading
    three = next(x for x in lines if x["name"] == "fig.three")
    assert three["value"] == "7.250" and three["places"] == 3  # nothing(null) skipped, list found
    assert counts["no_old_value"] == 2
    assert all(x["value"] != "0" for x in lines)
