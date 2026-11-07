"""Contract test of optdesk.ibkr.api, from both sides: a fake and a caller."""

from __future__ import annotations

import ast
import dataclasses
import inspect

import pytest

from optdesk.ibkr import api
from optdesk.ibkr.api import Session, SessionStatus


class _FakeSession:
    def ping(self) -> SessionStatus:
        return SessionStatus(connected=True)


def _is_connected(session: Session) -> bool:
    return session.ping().connected


def test_fake_satisfies_session() -> None:
    assert isinstance(_FakeSession(), Session)


def test_caller_reads_connected() -> None:
    assert _is_connected(_FakeSession()) is True


def test_session_status_is_frozen() -> None:
    status = SessionStatus(connected=False)
    with pytest.raises(dataclasses.FrozenInstanceError):
        delattr(status, "connected")


def test_api_declares_only() -> None:
    tree = ast.parse(inspect.getsource(api))
    funcs = [n for n in ast.walk(tree) if isinstance(n, (ast.FunctionDef, ast.AsyncFunctionDef))]
    assert funcs
    for f in funcs:
        assert len(f.body) <= 2
        assert all(isinstance(s, ast.Expr) and isinstance(s.value, ast.Constant) for s in f.body)
