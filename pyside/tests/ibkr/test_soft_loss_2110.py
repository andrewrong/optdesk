"""A 2110 at connect or later ends on a farm-ok code; a 1100 only on a 1101 or 1102."""

from __future__ import annotations

import asyncio
import logging

import pytest
from kit.fakeclock import FakeClock
from test_ibgateway import FakeIb, make

from optdesk.ibkr.ibgateway import IbGateway


async def _connect_with(code: int | None) -> tuple[IbGateway, FakeIb]:
    gateway, fake = make(FakeClock())
    plain = fake.client.connectAsync

    async def connect(host: str, port: int, client_id: int) -> None:
        await plain(host, port, client_id)
        if code is not None:
            fake.errorEvent.emit(-1, code, "text", None)

    fake.client.connectAsync = connect  # type: ignore[method-assign]
    await gateway.open_session()
    return gateway, fake


@pytest.mark.parametrize("farm_ok", [2104, 2106, 2158])
@pytest.mark.parametrize("at_connect", [True, False])
def test_new_2110_at_connect_ends_on_a_farm_ok_code(
    farm_ok: int, at_connect: bool
) -> None:
    async def run() -> list[tuple[bool, bool, int]]:
        gateway, fake = await _connect_with(2110 if at_connect else None)
        if not at_connect:
            fake.errorEvent.emit(-1, 2110, "broken", None)
        seen = []
        for _ in range(2):
            st = await gateway.status()
            seen.append((st.connected, st.soft_lost, st.generation))
            fake.errorEvent.emit(-1, farm_ok, "ok", None)
        gateway._session()  # answers, no ConnectionError
        return seen

    assert asyncio.run(run()) == [(False, True, 1), (True, False, 1)]


def test_new_a_farm_ok_without_a_loss_changes_nothing() -> None:
    async def run() -> tuple[bool, bool, int]:
        gateway, fake = await _connect_with(None)
        fake.errorEvent.emit(-1, 2104, "ok", None)
        st = await gateway.status()
        return st.connected, st.soft_lost, st.generation

    assert asyncio.run(run()) == (True, False, 1)


@pytest.mark.parametrize(("end", "generation"), [(1102, 1), (1101, 2)])
def test_new_1100_is_not_ended_by_a_farm_ok_code(end: int, generation: int) -> None:
    async def run() -> list[tuple[bool, bool, int]]:
        gateway, fake = await _connect_with(None)
        fake.errorEvent.emit(-1, 1100, "lost", None)
        fake.errorEvent.emit(-1, 2110, "broken", None)  # a 1100 stays a 1100
        seen = []
        for code in (2104, 2106, 2158, end):
            fake.errorEvent.emit(-1, code, "x", None)
            st = await gateway.status()
            seen.append((st.connected, st.soft_lost, st.generation))
        return seen

    soft = (False, True, 1)
    assert asyncio.run(run()) == [soft, soft, soft, (True, False, generation)]


def test_new_1300_ends_the_session() -> None:
    async def run() -> tuple[bool, bool, bool, bool]:
        gateway, fake = await _connect_with(None)
        fake.errorEvent.emit(-1, 1300, "socket port reset", None)
        st = await gateway.status()
        down = (st.connected, st.soft_lost, gateway._lost.is_set())
        # a 1300 in a soft loss ends it too: nothing but a new session revives it
        gateway2, fake2 = await _connect_with(1100)
        fake2.errorEvent.emit(-1, 1300, "socket port reset", None)
        fake2.errorEvent.emit(-1, 1102, "restored", None)
        st2 = await gateway2.status()
        return (*down, st2.connected or st2.soft_lost)

    assert asyncio.run(run()) == (False, False, True, False)


def test_new_session_code_line_is_logged_once(
    caplog: pytest.LogCaptureFixture,
) -> None:
    codes = (1100, 1101, 1102, 1300, 2110, 2104, 2106, 2158, 10197)

    async def run() -> None:
        gateway, fake = await _connect_with(None)
        gateway._alias = "wife"
        for code in (*codes, 2105, 354, 100):
            fake.errorEvent.emit(-1, code, "x", None)

    with caplog.at_level(logging.INFO):
        asyncio.run(run())
    lines = [r.getMessage() for r in caplog.records if "session_code" in r.getMessage()]
    assert [line.split()[2] for line in lines] == [f"code={c}" for c in codes]
    # the state after the code: a 1100 soft-lost, a 1102 back, a 1300 down
    assert lines[0] == "session_code alias=wife code=1100 state=soft_lost"
    assert lines[2] == "session_code alias=wife code=1102 state=connected"
    assert lines[3] == "session_code alias=wife code=1300 state=down"
