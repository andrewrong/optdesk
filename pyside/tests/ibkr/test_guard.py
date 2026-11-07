"""The send guard over a real ib_async client with a recording stub connection."""

from __future__ import annotations

import asyncio
from typing import Any

import pytest
from ib_async import Client, Contract, Order

from optdesk.ibkr import transport
from optdesk.ibkr.transport import NotAllowed, OrderRefused, check, install_guard

# ids of the TWS protocol: positions request 61, account summary request 62
ORDER_IDS = (3, 4, 21, 58)
ALLOWED_IDS = (1, 2, 9, 20, 25, 49, 59, 71, 78)


class _Conn:
    """Stands in for the broker connection and records every frame."""

    def __init__(self) -> None:
        self.frames: list[bytes] = []
        self.numBytesSent = 0
        self.numMsgSent = 0

    async def connectAsync(self, host: str, port: int) -> None:
        return None

    def sendMsg(self, msg: bytes) -> None:
        self.frames.append(msg)

    def disconnect(self) -> None:
        return None


_CLIENT: Any = Client


def _client() -> tuple[Any, _Conn]:
    client = _CLIENT(object())
    conn = _Conn()
    client.conn = conn
    client.connState = _CLIENT.CONNECTED
    return client, conn


def _spy(monkeypatch: pytest.MonkeyPatch) -> list[object]:
    seen: list[object] = []

    def spying(message_id: object) -> None:
        seen.append(message_id)
        check(message_id)

    monkeypatch.setattr(transport, "check", spying)
    return seen


def test_orders_always_refused() -> None:
    for message_id in ORDER_IDS:
        with pytest.raises(OrderRefused):
            check(message_id)


def test_allowlist_passes() -> None:
    for message_id in ALLOWED_IDS:
        check(message_id)


def test_unknown_id_refused() -> None:
    for message_id in (61, 62, 5, 7, 6, 0):
        with pytest.raises(NotAllowed):
            check(message_id)
    with pytest.raises(NotAllowed):
        check("x")


def test_real_client_refuses_orders() -> None:
    client, conn = _client()
    install_guard(client)
    contract = Contract(conId=1)
    refused: list[Any] = [
        lambda: client.placeOrder(5, contract, Order()),
        lambda: client.cancelOrder(5, ""),
        lambda: client.reqGlobalCancel(),
        lambda: client.exerciseOptions(5, contract, 1, 1, "", 1),
    ]
    for attempt in refused:
        with pytest.raises(OrderRefused):
            attempt()
    assert conn.frames == []
    client.reqMktData(7, contract, "100,101", False, False, [])
    client.reqMarketDataType(1)
    assert len(conn.frames) == 2


def test_sendmsg_direct_is_guarded() -> None:
    client, conn = _client()
    install_guard(client)
    with pytest.raises(OrderRefused):
        client.sendMsg("3\x0045\x00")
    with pytest.raises(NotAllowed):
        client.sendMsg("61\x001\x00")
    client.sendMsg("59\x001\x001\x00")
    client.sendMsg(None)  # the drain call of the client's own throttle
    assert len(conn.frames) == 1


def test_handshake_frame_is_constant_prefix(monkeypatch: pytest.MonkeyPatch) -> None:
    seen = _spy(monkeypatch)

    async def run() -> _Conn:
        client, conn = _client()
        client.connState = _CLIENT.DISCONNECTED
        install_guard(client)
        task = asyncio.ensure_future(client.connectAsync("host", 1, 33, 0))
        for _ in range(5):
            await asyncio.sleep(0)
        client.connState = _CLIENT.CONNECTED
        client.startApi()
        client.apiStart.emit()
        await task
        return conn

    conn = asyncio.run(run())
    assert conn.frames[0].startswith(b"API\x00")
    assert b"v" in conn.frames[0][4:]
    assert len(conn.frames) == 2
    assert set(seen) == {71}  # the one later frame passed the guard


def test_guard_installed_before_start_api(monkeypatch: pytest.MonkeyPatch) -> None:
    seen = _spy(monkeypatch)
    client, conn = _client()
    install_guard(client)
    client.startApi()
    assert seen[0] == 71 and len(conn.frames) == 1
