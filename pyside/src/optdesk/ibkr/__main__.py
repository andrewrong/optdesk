"""Entry point: python -m optdesk.ibkr."""

from __future__ import annotations

import os

from optdesk.ibkr.server import main

raise SystemExit(main(os.environ))
