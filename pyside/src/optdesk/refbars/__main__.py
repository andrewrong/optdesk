"""Entry point of `python -m optdesk.refbars --deadline SECONDS`."""

from __future__ import annotations

import sys
import threading
import time

import yfinance

from optdesk.refbars.cli import run


class _SystemClock:
    def monotonic(self) -> float:
        return time.monotonic()

    def sleep(self, seconds: float) -> None:
        threading.Event().wait(seconds)


if __name__ == "__main__":
    sys.exit(run(sys.argv[1:], sys.stdin, sys.stdout, sys.stderr, _SystemClock(), yfinance))
