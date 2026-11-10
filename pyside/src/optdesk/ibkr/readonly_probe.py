"""The read-only probe: proof that a gateway rejects what could change an account.

After every handshake a short second connection with client id 0 sends one
reqOpenOrders and passes only when the gateway answers with the rejection
"Read-Only mode" (error 321) within 5 s. An open-orders end, or no answer,
means the gateway is not read-only: GatewayNotReadOnly is raised, and being a
SystemExit it is not caught by the supervisor's retry, so the process stops and
the starter does not restart it. A probe whose handshake fails or runs out of its
budget raises ProbeUnknown, an ordinary exception: not proof and not a refusal,
the connection stays closed and the supervisor retries. The whole probe,
handshake included, runs under one 5 s budget. The probe has its own send guard that allows
message id 5 and nothing else, the order ids refused first.
This file imports no ib_async.
"""

from __future__ import annotations

import asyncio
from collections.abc import Callable
from typing import Any

from optdesk.ibkr.timing import Clock, with_timeout
from optdesk.ibkr.transport import ORDER_IDS, NotAllowed, OrderRefused

PROBE_CLIENT_ID = 0
PROBE_TIMEOUT_S = 5.0
READ_ONLY_CODE = 321  # "Read-Only mode": the gateway refused reqOpenOrders
OPEN_ORDERS_ID = 5  # message id of reqOpenOrders, the only id the probe sends


class GatewayNotReadOnly(SystemExit):
    """The gateway did not reject reqOpenOrders; a stop, never a retry."""

    def __init__(self, reason: str) -> None:
        super().__init__(f"gateway is not proven read-only: {reason}")


class ProbeUnknown(Exception):
    """The probe could not be made (handshake failed or timed out); retry, not stop."""


# The probe has its own guard and does not reuse transport.install_guard: that
# function takes no list of allowed ids, and transport.py is owner-only and
# outside the files of this issue.
def check_probe(message_id: object) -> None:
    """Raise OrderRefused for an order id and NotAllowed for any id but 5."""
    if message_id in ORDER_IDS:
        raise OrderRefused(f"message {message_id} places or changes orders")
    if message_id != OPEN_ORDERS_ID:
        raise NotAllowed(f"message {message_id} is not allowed on the probe")


def _frame_id(frame: str) -> object:
    head = frame.split("\0", 1)[0]
    return int(head) if head.isdigit() else head


def install_probe_guard(client: Any) -> None:
    """Guard client.send and client.sendMsg so only reqOpenOrders gets through."""
    send = client.send
    send_msg = client.sendMsg

    def guarded_send(*fields: object, **options: Any) -> None:
        check_probe(fields[0] if fields else None)
        send(*fields, **options)

    def guarded_send_msg(msg: str | None) -> None:
        if msg:
            check_probe(_frame_id(msg))
        send_msg(msg)

    client.send = guarded_send
    client.sendMsg = guarded_send_msg


async def prove_read_only(
    *,
    host: str,
    port: int,
    make_ib: Callable[[], Any],
    clock: Clock,
    install: Callable[[Any], None] = install_probe_guard,
) -> None:
    """Pass when the gateway answers reqOpenOrders with error 321 within 5 s.

    The handshake and the answer share the one 5 s budget. Raises
    GatewayNotReadOnly on an open-orders end or on no answer, and ProbeUnknown
    (the connection's error as __cause__) when the handshake fails or times out.
    """
    ib = make_ib()
    rejected = asyncio.Event()

    def on_error(req_id: int, code: int, text: str, contract: Any) -> None:
        if code == READ_ONLY_CODE:
            rejected.set()

    install(ib.client)
    ib.errorEvent += on_error
    request: asyncio.Future[Any] | None = None
    waiter = asyncio.ensure_future(rejected.wait())
    began = clock.monotonic()
    try:
        try:
            await with_timeout(
                clock,
                ib.client.connectAsync(
                    host, port, PROBE_CLIENT_ID, timeout=PROBE_TIMEOUT_S
                ),
                PROBE_TIMEOUT_S,
            )
        except Exception as error:
            raise ProbeUnknown(f"probe handshake failed: {error!r}") from error
        request = asyncio.ensure_future(ib.reqOpenOrdersAsync())

        async def answer() -> None:
            await asyncio.wait({waiter, request}, return_when=asyncio.FIRST_COMPLETED)

        try:
            left = max(0.0, PROBE_TIMEOUT_S - (clock.monotonic() - began))
            await with_timeout(clock, answer(), left)
        except TimeoutError:
            raise GatewayNotReadOnly("no answer within 5 s") from None
        if not rejected.is_set():
            raise GatewayNotReadOnly("reqOpenOrders was not rejected")
    finally:
        for task in (waiter, request):
            if task is not None and not task.done():
                task.cancel()
        await asyncio.gather(
            waiter, *([request] if request else []), return_exceptions=True
        )
        session = ib
        session.disconnect()
