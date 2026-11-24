"""The quote_done log record of one contract of a snapshot.

One line when the quote of a contract ends: the time to the release fields,
the time waited for open interest, the status and the names of the missing
fields. No con_id, no symbol and no field value.
"""

from __future__ import annotations

import logging
from collections.abc import Callable, Mapping

from optdesk.ibkr import spec


def quote_done_line(
    status: str,
    t_release_ms: int,
    t_oi_ms: int,
    missing: tuple[tuple[str, int, int], ...],
) -> str:
    names = ",".join(sorted(name for name, _, _ in missing)) or "-"
    return (
        f"quote_done status={status} t_release_ms={t_release_ms}"
        f" t_oi_ms={t_oi_ms} missing={names}"
    )


class QuoteTimes:
    """Marks on the monotonic clock of a quote: start, release, end."""

    def __init__(self, monotonic: Callable[[], float]) -> None:
        self._now = monotonic
        self.began = monotonic()
        self._released: float | None = None

    def release(self) -> float:
        """Mark the end of the wait for the release fields; seconds since start."""
        self._released = self._now()
        return self._released - self.began

    def milliseconds(self) -> tuple[int, int]:
        """(t_release_ms, t_oi_ms); a quote that never released has no grace."""
        end = self._now()
        released = end if self._released is None else self._released
        return round((released - self.began) * 1000), round((end - released) * 1000)


def log_quote_done(
    log: logging.Logger,
    times: QuoteTimes,
    kind: spec.KindSpec,
    got: Mapping[str, float],
    error: tuple[int, str] | None,
    missing: tuple[tuple[str, int, int], ...],
) -> None:
    """Write the quote_done record at INFO; call it when the line is released."""
    if error is not None and not spec.is_complete(kind, got):
        status = spec.classify_error(error[0]).quote_status
    else:
        status = spec.quote_status(kind, got)
    log.info(quote_done_line(status, *times.milliseconds(), missing))
