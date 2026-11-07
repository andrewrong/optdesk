"""Server of the ibkr module: configuration, the socket, the supervisor and the one loop.

The process serves SessionService, ContractsService and MarketDataService over
a Unix socket under $OT_HOME, with an IbGateway that has the send guard
installed. This file holds the one asyncio.run of the package.
"""

from __future__ import annotations

import asyncio
import logging
import os
import signal
import stat
import sys
from collections.abc import Mapping
from dataclasses import dataclass
from pathlib import Path
from typing import Protocol

from optdesk.ibkr.ibgateway import IbGateway
from optdesk.ibkr.model import SessionState
from optdesk.ibkr.timing import Clock, SystemClock
from optdesk.ibkr.transport import (
    build_server,
    configure_logging,
    install_guard,
    log_event,
)

_LOG = logging.getLogger("optdesk.ibkr.server")

BACKOFF_FIRST_S = 1.0
BACKOFF_CAP_S = 30.0
POLL_S = 1.0


class ConfigError(Exception):
    """A required variable is missing or a value is not usable; names the variable only."""


@dataclass(frozen=True)
class Config:
    """Settings of the process, read from the environment."""

    home: Path
    host: str
    port: int
    client_id: int
    lines: int
    snapshots: int
    market_data_type: int


def _number(env: Mapping[str, str], name: str, default: int | None) -> int:
    raw = env.get(name)
    if raw is None or raw == "":
        if default is None:
            raise ConfigError(f"{name} is required")
        return default
    try:
        value = int(raw)
    except ValueError:
        raise ConfigError(f"{name} must be a whole number") from None
    if value <= 0:
        raise ConfigError(f"{name} must be above zero")
    return value


def load_config(env: Mapping[str, str]) -> Config:
    """Read the settings from env; a missing or bad variable raises ConfigError."""
    home = env.get("OT_HOME", "")
    if not home:
        raise ConfigError("OT_HOME is required")
    return Config(
        home=Path(home),
        host=env.get("OT_IBKR_GATEWAY_HOST") or "127.0.0.1",
        port=_number(env, "OT_IBKR_GATEWAY_PORT", None),
        client_id=_number(env, "OT_IBKR_CLIENT_ID", 33),
        lines=_number(env, "OT_IBKR_LINES", 70),
        snapshots=_number(env, "OT_IBKR_SNAPSHOTS", 50),
        market_data_type=_number(env, "OT_IBKR_MARKET_DATA_TYPE", 1),
    )


def socket_path(home: Path) -> Path:
    """Return the socket path under home."""
    return home / "run" / "ibkr.sock"


def prepare_socket(home: Path) -> Path:
    """Create the run directory (mode 0700), remove a stale socket, return the path.

    Any other kind of file at the path is an error.
    """
    path = socket_path(home)
    path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    path.parent.chmod(0o700)
    try:
        mode = path.lstat().st_mode
    except FileNotFoundError:
        return path
    if not stat.S_ISSOCK(mode):
        raise ConfigError("a file that is not a socket is at the socket path")
    path.unlink()
    return path


def next_delay(delay: float) -> float:
    """Return the backoff after delay: doubled, capped."""
    return min(delay * 2, BACKOFF_CAP_S)


class Supervised(Protocol):
    """The two calls the supervisor makes on a gateway."""

    async def open_session(self) -> None: ...

    async def status(self) -> SessionState: ...


async def supervise(gateway: Supervised, clock: Clock) -> None:
    """Keep the session open: connect, poll, and reconnect with a capped backoff.

    Runs until cancelled.
    """
    delay = BACKOFF_FIRST_S
    while True:
        try:
            await gateway.open_session()
        except Exception:  # noqa: BLE001 - any connect failure means retry
            log_event(_LOG, "connect_failed")
            await clock.sleep(delay)
            delay = next_delay(delay)
            continue
        state = await gateway.status()
        log_event(_LOG, "connect_ok", generation=state.generation)
        delay = BACKOFF_FIRST_S
        while (await gateway.status()).connected:
            await clock.sleep(POLL_S)
        log_event(_LOG, "session_lost")


async def serve(config: Config, clock: Clock | None = None) -> None:
    """Serve the three services until SIGINT or SIGTERM."""
    clock = clock or SystemClock()
    gateway = IbGateway(
        install_guard,
        host=config.host,
        port=config.port,
        client_id=config.client_id,
        market_data_type=config.market_data_type,
        lines=config.lines,
        snapshots=config.snapshots,
        clock=clock,
    )
    path = prepare_socket(config.home)
    server = build_server(gateway, clock)
    previous = os.umask(0o177)
    try:
        server.add_insecure_port(f"unix:{path}")
    finally:
        os.umask(previous)
    await server.start()
    path.chmod(0o600)
    stopping = asyncio.Event()
    loop = asyncio.get_running_loop()
    for sig in (signal.SIGINT, signal.SIGTERM):
        loop.add_signal_handler(sig, stopping.set)
    supervisor = asyncio.ensure_future(supervise(gateway, clock))
    log_event(_LOG, "server_started")
    try:
        await stopping.wait()
    finally:
        supervisor.cancel()
        await asyncio.gather(supervisor, return_exceptions=True)
        await server.stop(grace=1)
        log_event(_LOG, "server_stopped")


def main(env: Mapping[str, str]) -> int:
    """Run the service with the settings of env; return the exit status."""
    try:
        config = load_config(env)
    except ConfigError as exc:
        print(f"optdesk.ibkr: {exc}", file=sys.stderr)
        return 2
    configure_logging()
    asyncio.run(serve(config))
    return 0
