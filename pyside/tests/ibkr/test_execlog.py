"""executions_read: the text of the line."""

from __future__ import annotations

from optdesk.ibkr.execlog import executions_read_line


def test_executions_read_line() -> None:
    assert executions_read_line("main", 5, 2, 1, 2) == (
        "executions_read raw=5 dropped_account=2 dropped_commission=1 stored=2"
        " alias=main"
    )
