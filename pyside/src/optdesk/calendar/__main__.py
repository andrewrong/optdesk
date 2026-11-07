"""Entry of `python -m optdesk.calendar`: the only place that reads the clock."""

from __future__ import annotations

import datetime
import sys

from optdesk.calendar.cli import run

if __name__ == "__main__":
    sys.exit(
        run(
            sys.argv[1:],
            lambda: datetime.datetime.now(datetime.UTC),
            sys.stdout,
            sys.stderr,
        )
    )
