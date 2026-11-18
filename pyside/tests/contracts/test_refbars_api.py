"""Contract test of optdesk.refbars.api: declarations only, and the golden answer."""

from __future__ import annotations

import ast
import json
import re
from pathlib import Path
from typing import get_type_hints

import optdesk.refbars as package
from optdesk.refbars import api
from optdesk.refbars.api import (
    Answer,
    Bar,
    Clock,
    Fetcher,
    PairResult,
    PairSpec,
    Request,
)

ANSWER = Path(__file__).resolve().parent.parent / "refbars" / "answer.json"
ERROR_CODES = {"", "rate_limited", "no_data", "network", "timeout", "source_changed"}


def test_refbars_api_is_declarations_only() -> None:
    tree = ast.parse(Path(api.__file__).read_text())
    for node in tree.body:
        if isinstance(node, ast.Import | ast.ImportFrom):
            names = [node.module or ""] if isinstance(node, ast.ImportFrom) else [
                a.name for a in node.names
            ]
            assert not any("ibkr" in n or "sqlite3" in n or "socket" in n for n in names)
            continue
        if isinstance(node, ast.Expr) and isinstance(node.value, ast.Constant):
            continue
        if isinstance(node, ast.Assign | ast.AnnAssign):
            continue
        assert isinstance(node, ast.ClassDef), ast.dump(node)
        for member in node.body:
            if isinstance(member, ast.FunctionDef):
                body = [
                    s
                    for s in member.body
                    if not (isinstance(s, ast.Expr) and isinstance(s.value, ast.Constant))
                ]
                assert all(
                    isinstance(s, ast.Expr) and s.value.value is ...  # type: ignore[attr-defined]
                    for s in body
                ), member.name
            else:
                assert isinstance(member, ast.AnnAssign | ast.Expr | ast.Pass), ast.dump(member)
    init = ast.parse(Path(package.__file__).read_text())
    assert all(
        isinstance(n, ast.Expr) and isinstance(n.value, ast.Constant) for n in init.body
    )
    assert len(init.body) == 1


def test_golden_answer_has_the_declared_keys() -> None:
    answer: Answer = json.loads(ANSWER.read_text())
    assert set(answer) == set(get_type_hints(Answer))
    results = answer["results"]
    assert [r["error_code"] for r in results] == ["", "rate_limited", "no_data"]
    for result in results:
        assert set(result) == set(get_type_hints(PairResult))
        assert result["error_code"] in ERROR_CODES
        for bar in result["bars"]:
            assert set(bar) == set(get_type_hints(Bar))
            assert re.fullmatch(r"\d{4}-\d{2}-\d{2}", bar["session_day"])
            assert re.fullmatch(r"\d+\.\d{4}", bar["close"])
    assert len(results[0]["bars"]) == 2
    assert results[1]["bars"] == [] and results[2]["bars"] == []
    assert set(get_type_hints(Request)) == {"pairs", "from_day", "to_day"}
    assert set(get_type_hints(PairSpec)) == {"symbol", "currency"}
    assert Fetcher is not None and Clock is not None
