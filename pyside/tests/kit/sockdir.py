"""A short temporary directory for Unix socket paths."""

from __future__ import annotations

import tempfile
from collections.abc import Iterator
from contextlib import contextmanager
from pathlib import Path


@contextmanager
def short_socket_dir() -> Iterator[Path]:
    """Yield a fresh directory whose path stays under the socket path limit."""
    with tempfile.TemporaryDirectory(prefix="ot") as name:
        yield Path(name)
