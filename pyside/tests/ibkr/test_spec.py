"""The request table, status rules and error codes of spec.py."""

from __future__ import annotations

import ast
import re
from pathlib import Path

import pytest

from optdesk.ibkr import spec
from optdesk.ibkr.spec import (
    DIVIDENDS,
    OPTION_EOD,
    OPTION_OPEN,
    SNAPSHOT,
    TABLE,
    UNDERLYING_IV,
    UnknownKind,
    classify_error,
    is_complete,
    is_informational,
    kind_spec,
    quote_status,
)

REPO = Path(__file__).resolve().parents[3]


def test_option_eod_ticks() -> None:
    row = kind_spec(OPTION_EOD)
    assert row.generic_ticks == "100,101"
    assert {"bid", "ask", "close", "delta"} <= row.release_fields
    assert {"volume", "open_interest"} <= set(row.wanted_fields)


def test_underlying_iv_ticks() -> None:
    row = kind_spec(UNDERLYING_IV)
    assert row.generic_ticks == "104,106"
    assert row.release_fields == {"option_implied_vol", "historical_vol"}


def test_snapshot_kind_ticks() -> None:
    row = kind_spec(SNAPSHOT)
    assert row.generic_ticks == ""
    assert row.release_fields == {"bid", "ask"}
    assert row.timeout_s == 20
    assert set(TABLE) == {
        OPTION_EOD,
        OPTION_OPEN,
        UNDERLYING_IV,
        SNAPSHOT,
        DIVIDENDS,
    }


def test_timeout_is_twenty_seconds() -> None:
    excepted = {DIVIDENDS, OPTION_OPEN}
    assert all(
        row.timeout_s == 20 for kind, row in TABLE.items() if kind not in excepted
    )
    assert kind_spec(OPTION_OPEN).timeout_s == 15
    with pytest.raises(UnknownKind):
        kind_spec("weekly")


def test_status_rules() -> None:
    row = kind_spec(SNAPSHOT)
    assert quote_status(row, {"bid": 1.0, "ask": 1.1, "last": 1.05}) == "ok"
    assert quote_status(row, {"bid": 1.0, "last": 1.05}) == "partial"
    assert quote_status(row, {}) == "no_data"
    assert quote_status(row, {"und_price": 3.0}) == "no_data"
    assert is_complete(row, {"bid": 1.0, "ask": 1.1})
    assert not is_complete(row, {"bid": 1.0})


def test_error_codes() -> None:
    assert classify_error(200).not_listed
    assert classify_error(101).shrink_budget
    assert classify_error(354).quote_status == "no_subscription"
    other = classify_error(162)
    assert other.quote_status == "error"
    assert not other.not_listed and not other.shrink_budget
    assert is_informational(2104) and is_informational(10167)
    assert not is_informational(200)


def test_table_matches_proto_comment() -> None:
    proto = (REPO / "contracts/proto/ibkr/v1/quote_types.proto").read_text()
    comment = " ".join(
        line.removeprefix("//").strip()
        for line in proto.splitlines()
        if line.startswith("//")
    )
    for name, kind in (
        ("OPTION_EOD", OPTION_EOD),
        ("OPTION_OPEN", OPTION_OPEN),
        ("UNDERLYING_IV", UNDERLYING_IV),
        ("SNAPSHOT", SNAPSHOT),
    ):
        match = re.search(rf"{name}: (.*?)(?: [A-Z_]+:|These first values)", comment)
        assert match, name
        text = match.group(1)
        ticks = re.search(r'generic ticks (\d+) and (\d+)|ticks "(\d+),(\d+)"', text)
        expected = ",".join(g for g in ticks.groups() if g) if ticks else ""
        assert kind_spec(kind).generic_ticks == expected
        seconds = re.search(r"at most (\d+) s", text)
        assert seconds and kind_spec(kind).timeout_s == int(seconds.group(1))


def test_no_strategy_in_spec() -> None:
    tree = ast.parse(Path(spec.__file__).read_text())
    for node in ast.walk(tree):
        if isinstance(node, ast.FunctionDef):
            names = {a.arg for a in node.args.args}
            returns = ast.unparse(node.returns) if node.returns else ""
            takes_number = any(
                "price" in n or "strike" in n or "expiry" in n for n in names
            )
            assert not (takes_number and returns in ("float", "int")), node.name


def test_tick_456_in_spec() -> None:
    row = kind_spec(DIVIDENDS)
    assert row.generic_ticks == "456"
    assert row.timeout_s == 10
    assert not is_complete(row, {})
    assert is_complete(row, {"dividend_tick": 1.0})
