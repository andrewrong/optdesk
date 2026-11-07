"""Server of the ibkr module: configuration, the sockets, the supervisors and the one loop.

The process serves SessionService, ContractsService, MarketDataService and
AccountService (each by the collect words of its gateway) over
a Unix socket per gateway under $OT_HOME (and ibkr.sock for the default one),
each gateway an IbGateway with the send guard installed, and answers GET /health
when OT_IBKR_HEALTH_PORT is set. This file holds the one asyncio.run of the package.
"""

from __future__ import annotations

import asyncio
import logging
import os
import signal
import stat
import sys
from collections.abc import Callable, Mapping
from dataclasses import dataclass
from pathlib import Path
from typing import Protocol

import grpc.aio

from optdesk.ibkr.gateways import (
    COLLECT_WORDS,
    FALLBACK_NAME,
    ConfigError,
    GatewaySpec,
    from_env,
    load_gateways,
    positive_int,
)
from optdesk.ibkr.health import PROBE_DEADLINE_S, HealthServer, Probe, start_health
from optdesk.ibkr.ibgateway import IbGateway
from optdesk.ibkr.model import Gateway, SessionState
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


@dataclass(frozen=True)
class Config:
    """Settings of the process: the gateways and the keys that apply to all of them."""

    home: Path
    gateways: tuple[GatewaySpec, ...]
    default: str
    lines: int
    snapshots: int
    health_port: int | None = None
    named_sockets: bool = True


def load_config(env: Mapping[str, str]) -> Config:
    """Read the settings from env; a missing or bad variable raises ConfigError."""
    home = env.get("OT_HOME", "")
    if not home:
        raise ConfigError("OT_HOME is required")
    file = env.get("OT_IBKR_GATEWAYS")
    if file:
        gateways, default = load_gateways(file)
    else:
        gateways, default = from_env(env), FALLBACK_NAME
    health = env.get("OT_IBKR_HEALTH_PORT")
    return Config(
        home=Path(home),
        gateways=gateways,
        default=default,
        lines=positive_int(env, "OT_IBKR_LINES", 70),
        snapshots=positive_int(env, "OT_IBKR_SNAPSHOTS", 50),
        health_port=positive_int(env, "OT_IBKR_HEALTH_PORT", None) if health else None,
        named_sockets=bool(file),
    )


def socket_path(home: Path, name: str | None = None) -> Path:
    """Return the socket path under home: ibkr.sock, or ibkr-<name>.sock."""
    return home / "run" / ("ibkr.sock" if name is None else f"ibkr-{name}.sock")


def prepare_socket(home: Path, name: str | None = None) -> Path:
    """Create the run directory (mode 0700), remove a stale socket, return the path.

    Any other kind of file at the path is an error.
    """
    path = socket_path(home, name)
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


GatewayFactory = Callable[[GatewaySpec], Gateway]


def gateway_factory(
    config: Config, clock: Clock, build: Callable[..., Gateway] = IbGateway
) -> GatewayFactory:
    """Return the factory that builds a guarded gateway for a spec."""

    def make(spec: GatewaySpec) -> Gateway:
        return build(
            install_guard,
            host=spec.host,
            port=spec.port,
            client_id=spec.client_id,
            market_data_type=spec.market_data_type,
            lines=config.lines,
            snapshots=config.snapshots,
            clock=clock,
        )

    return make


def _probe(gateway: Supervised, loop: asyncio.AbstractEventLoop) -> Probe:
    def probe() -> bool:
        future = asyncio.run_coroutine_threadsafe(gateway.status(), loop)
        try:
            return future.result(PROBE_DEADLINE_S).connected
        except BaseException:
            future.cancel()
            raise

    return probe


def _socket_paths(config: Config, spec: GatewaySpec) -> list[Path]:
    """Prepare the sockets of spec: its own by name, and ibkr.sock for the default."""
    if not set(spec.collect) & set(COLLECT_WORDS):
        return []
    paths = []
    if config.named_sockets:
        paths.append(prepare_socket(config.home, spec.name))
    if spec.name == config.default and "market_data" in spec.collect:
        paths.append(prepare_socket(config.home))
    return paths


async def serve(
    config: Config,
    clock: Clock | None = None,
    *,
    make_gateway: GatewayFactory | None = None,
    stop: asyncio.Event | None = None,
    on_ready: Callable[[int | None], None] | None = None,
) -> None:
    """Serve the sockets of every gateway until stop is set, or SIGINT or SIGTERM."""
    clock = clock or SystemClock()
    factory = make_gateway or gateway_factory(config, clock)
    gateways = {spec.name: factory(spec) for spec in config.gateways}
    plan = [(spec, _socket_paths(config, spec)) for spec in config.gateways]
    servers: list[grpc.aio.Server] = []
    supervisors: list[asyncio.Future[None]] = []
    health: HealthServer | None = None
    started = False
    try:
        for spec, paths in plan:
            if not paths:
                continue
            server = build_server(gateways[spec.name], clock, spec=spec)
            servers.append(server)
            previous = os.umask(0o177)
            try:
                for path in paths:
                    server.add_insecure_port(f"unix:{path}")
            finally:
                os.umask(previous)
        for server in servers:
            await server.start()
        started = True
        for _, paths in plan:
            for path in paths:
                path.chmod(0o600)
        loop = asyncio.get_running_loop()
        if config.health_port is not None:
            probes = {
                spec.name: (spec.required, _probe(gateways[spec.name], loop))
                for spec in config.gateways
            }
            health = start_health(probes, config.health_port)
        stopping = stop
        if stopping is None:
            stopping = asyncio.Event()
            for sig in (signal.SIGINT, signal.SIGTERM):
                loop.add_signal_handler(sig, stopping.set)
        supervisors = [
            asyncio.ensure_future(supervise(gateways[spec.name], clock))
            for spec in config.gateways
        ]
        log_event(_LOG, "server_started")
        if on_ready is not None:
            on_ready(None if health is None else health.port)
        await stopping.wait()
    finally:
        for supervisor in supervisors:
            supervisor.cancel()
        await asyncio.gather(*supervisors, return_exceptions=True)
        if health is not None:
            await asyncio.to_thread(health.stop)
        for server in servers:
            await server.stop(grace=1)
        if started:
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
