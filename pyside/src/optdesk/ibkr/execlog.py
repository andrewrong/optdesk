"""The executions_read log record of one Executions call.

One line when the end marker of reqExecutions has come: the fills IB sent, the
ones the account filter dropped, the ones dropped for want of a commission
report and the rows answered. The gateway name, no account number and no field
of a fill.
"""

from __future__ import annotations

import logging


def executions_read_line(
    alias: str, raw: int, dropped_account: int, dropped_commission: int, stored: int
) -> str:
    return (
        f"executions_read raw={raw} dropped_account={dropped_account}"
        f" dropped_commission={dropped_commission} stored={stored} alias={alias}"
    )


def log_executions_read(
    log: logging.Logger,
    alias: str,
    raw: int,
    dropped_account: int,
    dropped_commission: int,
    stored: int,
) -> None:
    """Write the executions_read record at INFO."""
    log.info(
        executions_read_line(alias, raw, dropped_account, dropped_commission, stored)
    )
