"""Contract test of optdesk.refearn.api: declarations only, a fake and a caller."""

from __future__ import annotations

import ast
import json
import re
from pathlib import Path
from typing import get_type_hints

import optdesk.refearn as package
from optdesk.refbars import api as refbars_api
from optdesk.refearn import api
from optdesk.refearn.api import Answer, Fetcher, Request, SymbolResult, SymbolSpec


class FakeFetcher:
    """Implements the Protocol: one date for AAPL, a rate limit for the rest."""

    def __call__(self, symbol: SymbolSpec, from_day: str) -> SymbolResult:
        if symbol["symbol"] == "AAPL":
            return {**symbol, "error_code": "", "dates": [from_day, "2026-12-01"]}
        return {**symbol, "error_code": "rate_limited", "dates": []}


def test_refearn_api_is_declarations_only() -> None:
    tree = ast.parse(Path(api.__file__).read_text())
    for node in tree.body:
        if isinstance(node, ast.Import | ast.ImportFrom | ast.Assign | ast.AnnAssign):
            continue
        if isinstance(node, ast.Expr) and isinstance(node.value, ast.Constant):
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
    assert len(init.body) == 1


def test_error_codes_are_those_of_refbars() -> None:
    assert api.ERROR_CODES == refbars_api.ERROR_CODES


def test_declared_keys() -> None:
    assert set(get_type_hints(SymbolSpec)) == {"symbol", "currency"}
    assert set(get_type_hints(Request)) == {"symbols", "from_day"}
    assert set(get_type_hints(SymbolResult)) == {"symbol", "currency", "error_code", "dates"}
    assert set(get_type_hints(Answer)) == {"results"}


def test_caller_builds_request_and_reads_answer_through_the_fake() -> None:
    fetcher = FakeFetcher()
    assert isinstance(fetcher, Fetcher)
    request: Request = json.loads(
        json.dumps(
            {
                "symbols": [
                    {"symbol": "AAPL", "currency": "USD"},
                    {"symbol": "MSFT", "currency": "USD"},
                ],
                "from_day": "2026-11-27",
            }
        )
    )
    assert len(request["symbols"]) <= 10
    answer: Answer = {"results": [fetcher(s, request["from_day"]) for s in request["symbols"]]}
    wire: Answer = json.loads(json.dumps(answer))
    assert [r["symbol"] for r in wire["results"]] == ["AAPL", "MSFT"]
    assert [r["error_code"] for r in wire["results"]] == ["", "rate_limited"]
    assert all(r["error_code"] in {"", *api.ERROR_CODES} for r in wire["results"])
    assert all(re.fullmatch(r"\d{4}-\d{2}-\d{2}", d) for d in wire["results"][0]["dates"])
    assert wire["results"][1]["dates"] == []
