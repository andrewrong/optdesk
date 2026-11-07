"""Configuration, the socket and the supervisor of the ibkr server."""

from __future__ import annotations

import asyncio
import os
import stat
from pathlib import Path

import pytest
from kit.fakeclock import FakeClock
from kit.sockdir import short_socket_dir

from optdesk.ibkr.model import SessionState
from optdesk.ibkr.server import (
    BACKOFF_CAP_S,
    ConfigError,
    load_config,
    prepare_socket,
    socket_path,
    supervise,
)


def test_socket_path_under_ot_home() -> None:
    config = load_config({"OT_HOME": "/h/x", "OT_IBKR_GATEWAY_PORT": "4002"})
    assert socket_path(config.home) == Path("/h/x/run/ibkr.sock")


def test_socket_modes() -> None:
    import socket

    with short_socket_dir() as home:
        path = prepare_socket(home)
        sock = socket.socket(socket.AF_UNIX)
        previous = os.umask(0o177)
        try:
            sock.bind(str(path))
        finally:
            os.umask(previous)
        path.chmod(0o600)
        try:
            assert stat.S_IMODE(path.parent.stat().st_mode) == 0o700
            assert stat.S_IMODE(path.stat().st_mode) == 0o600
        finally:
            sock.close()


def test_stale_socket_replaced() -> None:
    import socket

    with short_socket_dir() as home:
        path = prepare_socket(home)
        sock = socket.socket(socket.AF_UNIX)
        sock.bind(str(path))
        sock.close()  # the file stays: a stale socket
        assert path.exists()
        assert prepare_socket(home) == path
        assert not path.exists()


def test_regular_file_at_path_is_error(tmp_path: Path) -> None:
    path = socket_path(tmp_path)
    path.parent.mkdir(parents=True)
    path.write_text("not a socket")
    with pytest.raises(ConfigError):
        prepare_socket(tmp_path)
    assert path.read_text() == "not a socket"


def test_missing_variable_names_it() -> None:
    with pytest.raises(ConfigError, match="OT_HOME"):
        load_config({"OT_IBKR_GATEWAY_PORT": "4002"})
    with pytest.raises(ConfigError, match="OT_IBKR_GATEWAY_PORT") as caught:
        load_config({"OT_HOME": "/secret-home"})
    assert "/secret-home" not in str(caught.value)
    with pytest.raises(ConfigError, match="OT_IBKR_GATEWAY_PORT") as bad:
        load_config({"OT_HOME": "/h", "OT_IBKR_GATEWAY_PORT": "abc"})
    assert "abc" not in str(bad.value)


def test_config_defaults() -> None:
    config = load_config({"OT_HOME": "/h", "OT_IBKR_GATEWAY_PORT": "4002"})
    assert config.host == "127.0.0.1" and config.port == 4002
    assert config.client_id == 33 and config.lines == 70 and config.snapshots == 50
    assert config.market_data_type == 1


class RecordingClock(FakeClock):
    def __init__(self) -> None:
        super().__init__()
        self.asked: list[float] = []

    async def sleep(self, seconds: float) -> None:
        self.asked.append(seconds)
        await super().sleep(seconds)


class FlakyGateway:
    def __init__(self, failures: int) -> None:
        self.failures = failures
        self.generation = 0

    async def open_session(self) -> None:
        if self.failures:
            self.failures -= 1
            raise ConnectionError("down")
        self.generation += 1

    async def status(self) -> SessionState:
        return SessionState(self.generation > 0, self.generation, 1)


def test_supervisor_reconnects_with_backoff() -> None:
    async def run() -> tuple[list[float], int]:
        clock = RecordingClock()
        gateway = FlakyGateway(failures=8)
        task = asyncio.ensure_future(supervise(gateway, clock))
        for _ in range(12):
            for _ in range(10):
                await asyncio.sleep(0)
            await clock.advance(30)
        task.cancel()
        await asyncio.gather(task, return_exceptions=True)
        return clock.asked, gateway.generation

    asked, generation = asyncio.run(run())
    assert asked[:8] == [1.0, 2.0, 4.0, 8.0, 16.0, BACKOFF_CAP_S, BACKOFF_CAP_S, 30.0]
    assert generation == 1
