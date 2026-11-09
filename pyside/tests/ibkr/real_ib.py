"""A real ib_async IB whose send is stubbed: the library runs, nothing leaves the process.

The library validates, hashes and encodes every request itself; only the five
calls at the edge (client.isConnected, isReady, serverVersion, getReqId and
client.conn.sendMsg) are replaced. Every message handed to sendMsg is recorded
as its encoded text. Each call of real_ib builds a new IB; nothing is shared.
"""

from __future__ import annotations

from typing import Any

from ib_async import IB

SERVER_VERSION = 178
ACCOUNT = "DU" + "7654321"  # synthetic, one account as the gateway requires


class RealIb:
    """A real IB and the messages it handed to the stubbed send."""

    def __init__(self) -> None:
        self.ib = IB()
        self.sent: list[bytes] = []
        self._req_id = 0
        client = self.ib.client
        client.isConnected = lambda: True  # type: ignore[method-assign]
        client.isReady = lambda: True  # type: ignore[method-assign]
        client.serverVersion = lambda: SERVER_VERSION  # type: ignore[method-assign]
        client.getReqId = self._next_id  # type: ignore[method-assign]
        client.conn.sendMsg = self.sent.append  # type: ignore[method-assign]
        self.ib.wrapper.accounts = [ACCOUNT]

    def _next_id(self) -> int:
        self._req_id += 1
        return self._req_id


def real_ib() -> RealIb:
    """Build a new real IB with its send stubbed."""
    return RealIb()


def sent_messages(stub: RealIb) -> list[list[str]]:
    """Return the sent messages as lists of fields, the length prefix dropped."""
    messages: list[list[str]] = []
    for raw in stub.sent:
        body: Any = raw[4:] if len(raw) > 4 else raw
        messages.append(bytes(body).decode(errors="replace").rstrip("\0").split("\0"))
    return messages
