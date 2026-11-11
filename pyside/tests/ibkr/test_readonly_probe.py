"""The read-only probe and the gateway's session-loss and client-id-in-use codes."""

from __future__ import annotations

import asyncio
from collections.abc import Awaitable, Callable
from typing import Any

import pytest
from ib_async import OptionChain
from kit.fakeclock import FakeClock
from test_ibgateway import FakeClient, FakeIb

from optdesk.ibkr.ibgateway import IbGateway
from optdesk.ibkr.readonly_probe import (
    PROBE_CLIENT_ID,
    GatewayNotReadOnly,
    ProbeUnknown,
    install_probe_guard,
    prove_read_only,
)
from optdesk.ibkr.server import supervise
from optdesk.ibkr.transport import NotAllowed, OrderRefused


class ProbeClient(FakeClient):
    """Records the timeout it got; `handshake` is a coroutine the connect awaits."""

    timeout: float | None = None
    handshake: Callable[[], Awaitable[None]] | None = None

    async def connectAsync(
        self, host: str, port: int, client_id: int, timeout: float = 2.0
    ) -> None:
        self.timeout = timeout
        if self.handshake is not None:
            await self.handshake()
        await super().connectAsync(host, port, client_id)
        self.sent: list[object] = []
        self.send = self._send
        self.sendMsg = lambda msg: None

    def _send(self, *fields: object, **options: Any) -> None:
        self.sent.append(fields[0])


class ProbeIb(FakeIb):
    """Answers the open-orders request as `answer` says: 321, an end, or nothing."""

    def __init__(self, clock: FakeClock, answer: str) -> None:
        super().__init__(clock)
        self.answer = answer
        self.pc = ProbeClient(self)
        self.client = self.pc
        self.requests = 0

    def reqOpenOrdersAsync(self) -> Awaitable[list[object]]:
        self.requests += 1
        if self.answer == "321":
            self.errorEvent.emit(-1, 321, "Read-Only mode", None)
        loop = asyncio.get_running_loop()
        future: asyncio.Future[list[object]] = loop.create_future()
        if self.answer == "end":
            future.set_result([])
        return future


def probe_of(clock: FakeClock, ib: ProbeIb) -> Callable[[], Awaitable[None]]:
    async def run() -> None:
        await prove_read_only(
            host="h", port=1, make_ib=lambda: ib, clock=clock, install=lambda c: None
        )

    return run


def test_probe_passes_on_error_321() -> None:
    async def run() -> tuple[tuple[str, int, int] | None, bool, int]:
        clock = FakeClock()
        ib = ProbeIb(clock, "321")
        await probe_of(clock, ib)()
        return ib.connect_args, ib.disconnected, ib.requests

    assert asyncio.run(run()) == (("h", 1, PROBE_CLIENT_ID), True, 1)
    assert PROBE_CLIENT_ID == 0


def test_probe_fails_on_open_orders_end() -> None:
    async def run() -> None:
        clock = FakeClock()
        ib = ProbeIb(clock, "end")
        await probe_of(clock, ib)()

    with pytest.raises(GatewayNotReadOnly):
        asyncio.run(run())


def test_probe_fails_when_nothing_answers_within_five_seconds() -> None:
    async def run() -> tuple[bool, bool]:
        clock = FakeClock()
        ib = ProbeIb(clock, "silent")

        async def attempt() -> SystemExit | None:
            try:
                await probe_of(clock, ib)()
            except SystemExit as stop:  # a SystemExit ends a bare task's loop
                return stop
            return None

        task = asyncio.ensure_future(attempt())
        await clock.advance(4.9)
        early = task.done()
        await clock.advance(0.2)
        stop = await task
        return early, isinstance(stop, GatewayNotReadOnly) and ib.disconnected

    assert asyncio.run(run()) == (False, True)


def test_probe_guard_allows_only_open_orders() -> None:
    sent: list[object] = []

    class Client:
        def send(self, *fields: object, **options: Any) -> None:
            sent.append(fields[0])

        def sendMsg(self, msg: str | None) -> None:
            sent.append(msg)

    client = Client()
    install_probe_guard(client)
    client.send(5)
    for order_id in (3, 4, 21, 58):
        with pytest.raises(OrderRefused):
            client.send(order_id)
    for other in (1, 9, 59, 92, 94, 0):
        with pytest.raises(NotAllowed):
            client.send(other)
    client.sendMsg("5\x001\x00")
    with pytest.raises(OrderRefused):
        client.sendMsg("3\x00x")
    with pytest.raises(NotAllowed):
        client.sendMsg("1\x00x")
    assert sent == [5, "5\x001\x00"]


async def _passes() -> None:
    return None


def _gateway(
    clock: FakeClock, ib: FakeIb, probe: Callable[[], Awaitable[None]]
) -> IbGateway:
    return IbGateway(
        lambda client: None,
        host="h",
        port=1,
        client_id=33,
        clock=clock,
        ib_factory=lambda: ib,
        readonly_probe=probe,
    )


def test_failed_probe_leaves_the_gateway_not_connected() -> None:
    async def run() -> bool:
        clock = FakeClock()

        async def fails() -> None:
            raise GatewayNotReadOnly("no rejection")

        gateway = _gateway(clock, FakeIb(clock), fails)
        with pytest.raises(GatewayNotReadOnly):
            await gateway.open_session()
        return (await gateway.status()).connected

    assert asyncio.run(run()) is False


def test_probe_runs_before_the_market_data_type_and_the_connected_mark() -> None:
    async def run() -> tuple[list[str], bool]:
        clock = FakeClock()
        ib = FakeIb(clock)
        seen: list[bool] = []

        async def probe() -> None:
            ib.log.append("probe")
            seen.append((await gateway.status()).connected)

        gateway = _gateway(clock, ib, probe)
        await gateway.open_session()
        return ib.log, seen == [False]

    assert asyncio.run(run()) == (["connect", "probe", "market_data_type"], True)


def test_gateway_not_read_only_is_a_stop_the_supervisor_cannot_catch() -> None:
    assert not issubclass(GatewayNotReadOnly, Exception)
    assert issubclass(GatewayNotReadOnly, SystemExit)


@pytest.mark.parametrize("code", [2110, 1100])
def test_session_loss_codes_without_a_contract_mark_the_session_lost(
    code: int,
) -> None:
    async def run() -> tuple[bool, bool]:
        clock = FakeClock()
        ib = FakeIb(clock)
        gateway = _gateway(clock, ib, _passes)
        await gateway.open_session()
        before = (await gateway.status()).connected
        ib.errorEvent.emit(-1, code, "lost", None)
        return before, (await gateway.status()).connected

    assert asyncio.run(run()) == (True, False)


def test_10197_keeps_the_session_up_for_the_quote_policy() -> None:
    # issue 1477: a 10197 holds and probes the quotes; the socket stays up
    async def run() -> bool:
        clock = FakeClock()
        ib = FakeIb(clock)
        gateway = _gateway(clock, ib, _passes)
        await gateway.open_session()
        ib.errorEvent.emit(-1, 10197, "competing session", None)
        return (await gateway.status()).connected

    assert asyncio.run(run()) is True


def test_open_items_end_with_a_connection_error_on_1100() -> None:
    async def run() -> None:
        clock = FakeClock()
        ib = FakeIb(clock)

        async def never() -> list[OptionChain]:
            await asyncio.Event().wait()
            return []

        ib.reqSecDefOptParamsAsync = lambda *words: never()  # type: ignore[method-assign]
        gateway = _gateway(clock, ib, _passes)
        await gateway.open_session()
        task = asyncio.ensure_future(
            gateway.chains(sec_type="STK", symbol="AAA", con_id=1)
        )
        await asyncio.sleep(0)
        ib.errorEvent.emit(-1, 1100, "connectivity lost", None)
        await task

    with pytest.raises(ConnectionError):
        asyncio.run(run())


class InUseClient(FakeClient):
    async def connectAsync(self, host: str, port: int, client_id: int) -> None:
        self.ib.errorEvent.emit(-1, 326, "client id in use", None)
        raise TimeoutError


def test_client_id_in_use_stops_and_is_not_retried() -> None:
    async def run() -> None:
        clock = FakeClock()
        ib = FakeIb(clock)
        ib.client = InUseClient(ib)
        gateway = _gateway(clock, ib, _passes)
        await gateway.open_session()

    with pytest.raises(SystemExit) as stop:
        asyncio.run(run())
    assert not isinstance(stop.value, Exception)


def test_other_connect_failure_stays_an_ordinary_error() -> None:
    class Down(FakeClient):
        async def connectAsync(self, host: str, port: int, client_id: int) -> None:
            raise ConnectionRefusedError

    async def run() -> None:
        clock = FakeClock()
        ib = FakeIb(clock)
        ib.client = Down(ib)
        await _gateway(clock, ib, _passes).open_session()

    with pytest.raises(ConnectionRefusedError):
        asyncio.run(run())


def test_probe_connect_gets_the_five_second_budget_as_timeout() -> None:
    async def run() -> float | None:
        clock = FakeClock()
        ib = ProbeIb(clock, "321")
        await probe_of(clock, ib)()
        return ib.pc.timeout

    assert asyncio.run(run()) == 5.0


def test_slow_handshake_leaves_the_rest_of_five_seconds_for_the_answer() -> None:
    async def run() -> tuple[bool, bool, float]:
        clock = FakeClock()
        ib = ProbeIb(clock, "silent")
        ib.pc.handshake = lambda: clock.sleep(3.0)

        async def attempt() -> SystemExit | None:
            try:
                await probe_of(clock, ib)()
            except SystemExit as stop:
                return stop
            return None

        task = asyncio.ensure_future(attempt())
        await clock.advance(3.0)
        await clock.advance(1.9)
        early = task.done()
        await clock.advance(0.2)
        stop = await task
        return early, isinstance(stop, GatewayNotReadOnly), clock.monotonic()

    early, refused, ended = asyncio.run(run())
    assert (early, refused) == (False, True)
    assert ended == pytest.approx(5.1)


def test_probe_handshake_that_raises_is_unknown_and_disconnected() -> None:
    async def boom() -> None:
        raise ConnectionRefusedError

    async def run() -> tuple[ProbeUnknown, bool]:
        clock = FakeClock()
        ib = ProbeIb(clock, "321")
        ib.pc.handshake = boom
        try:
            await probe_of(clock, ib)()
        except ProbeUnknown as unknown:
            return unknown, ib.disconnected
        raise AssertionError("no ProbeUnknown")

    unknown, disconnected = asyncio.run(run())
    assert isinstance(unknown.__cause__, ConnectionRefusedError)
    assert disconnected
    assert issubclass(ProbeUnknown, Exception)
    assert not issubclass(ProbeUnknown, SystemExit)


def test_probe_handshake_pending_at_five_seconds_is_unknown() -> None:
    async def run() -> tuple[bool, bool, bool]:
        clock = FakeClock()
        ib = ProbeIb(clock, "321")
        ib.pc.handshake = lambda: asyncio.Event().wait()  # type: ignore[assignment]

        async def attempt() -> BaseException | None:
            try:
                await probe_of(clock, ib)()
            except BaseException as stop:  # noqa: BLE001
                return stop
            return None

        task = asyncio.ensure_future(attempt())
        await clock.advance(4.9)
        early = task.done()
        await clock.advance(0.2)
        stop = await task
        return early, isinstance(stop, ProbeUnknown), ib.disconnected

    assert asyncio.run(run()) == (False, True, True)


def test_unknown_probe_leaves_not_connected_and_supervise_retries() -> None:
    async def run() -> tuple[bool, int, bool]:
        clock = FakeClock()
        ib = ProbeIb(clock, "321")
        calls = 0

        async def probe() -> None:
            nonlocal calls
            calls += 1
            if calls == 1:
                raise ProbeUnknown("handshake timed out")

        gateway = _gateway(clock, ib, probe)
        task = asyncio.ensure_future(supervise(gateway, clock, "main"))
        await clock.advance(0.0)
        first = (await gateway.status()).connected
        await clock.advance(60.0)
        up = (await gateway.status()).connected
        failed = task.done()
        task.cancel()
        await asyncio.gather(task, return_exceptions=True)
        return first, calls, up and not failed

    assert asyncio.run(run()) == (False, 2, True)


def test_two_open_sessions_of_one_gateway_probe_one_after_the_other() -> None:
    async def run() -> list[str]:
        clock = FakeClock()
        log: list[str] = []
        release = asyncio.Event()

        async def probe() -> None:
            log.append("start")
            await release.wait()
            log.append("end")

        gateway = _gateway(clock, FakeIb(clock), probe)
        first = asyncio.ensure_future(gateway.open_session())
        second = asyncio.ensure_future(gateway.open_session())
        await clock.advance(0.0)
        release.set()
        await asyncio.gather(first, second, return_exceptions=True)
        return log

    assert asyncio.run(run()) == ["start", "end", "start", "end"]
