"""Interface of the ibkr module: the session probe of ibkr.v1.

Declarations only. Mirrors contracts/proto/ibkr/v1/session.proto by hand and
imports no generated code.
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import Protocol, runtime_checkable


@dataclass(frozen=True)
class SessionStatus:
    """Answer of a session probe."""

    connected: bool


@runtime_checkable
class Session(Protocol):
    """A broker session that can be probed."""

    def ping(self) -> SessionStatus:
        """Return whether the session is connected."""
        ...
