"""Configuration, the socket and the supervisor of the ibkr server."""

from __future__ import annotations

import asyncio
import json
import os
import stat
import urllib.error
import urllib.request
from collections.abc import Awaitable, Callable
from pathlib import Path
from typing import Any, cast

import pytest
from kit.fakeclock import FakeClock
from kit.sockdir import short_socket_dir

from optdesk.ibkr.gateways import ConfigError, GatewaySpec
from optdesk.ibkr.model import Gateway, SessionState
from optdesk.ibkr.server import (
    BACKOFF_CAP_S,
    Config,
    gateway_factory,
    load_config,
    main,
    prepare_socket,
    serve,
    socket_path,
    supervise,
)
from optdesk.ibkr.transport import install_guard


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


def test_load_config_without_file_is_unchanged() -> None:
    config = load_config({"OT_HOME": "/h", "OT_IBKR_GATEWAY_PORT": "4002"})
    (spec,) = config.gateways
    assert spec.name == "default" == config.default
    assert spec.host == "127.0.0.1" and spec.port == 4002
    assert spec.client_id == 33 and config.lines == 70 and config.snapshots == 50
    assert spec.market_data_type == 1 and spec.required
    assert config.health_port is None and not config.named_sockets


def gateways_file(tmp_path: Path) -> Path:
    path = tmp_path / "gateways.toml"
    path.write_text(
        '[[gateway]]\nname = "a"\nport = 1\naccount = "x"\n'
        'collect = ["market_data"]\nrequired = true\n'
    )
    return path


def test_load_config_reads_the_file(tmp_path: Path) -> None:
    env = {"OT_HOME": "/h", "OT_IBKR_GATEWAYS": str(gateways_file(tmp_path))}
    config = load_config(env | {"OT_IBKR_HEALTH_PORT": "8123"})
    assert [g.name for g in config.gateways] == ["a"] and config.default == "a"
    assert config.health_port == 8123 and config.named_sockets


def test_bad_health_key_names_only_the_key() -> None:
    for bad in ("0", "text", "-3"):
        env = {"OT_HOME": "/h", "OT_IBKR_GATEWAY_PORT": "1", "OT_IBKR_HEALTH_PORT": bad}
        with pytest.raises(ConfigError, match="OT_IBKR_HEALTH_PORT") as caught:
            load_config(env)
        assert bad not in str(caught.value)


def test_main_exits_2_on_a_bad_gateways_file(tmp_path: Path) -> None:
    path = tmp_path / "gateways.toml"
    path.write_text('default = "a"\n')
    assert main({"OT_HOME": str(tmp_path), "OT_IBKR_GATEWAYS": str(path)}) == 2


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


class FakeGateway:
    def __init__(self, fail: bool = False) -> None:
        self.fail = fail
        self.opened = 0
        self.connected = False

    async def open_session(self) -> None:
        self.opened += 1
        if self.fail:
            raise ConnectionError("down")
        self.connected = True

    async def status(self) -> SessionState:
        return SessionState(self.connected, int(self.connected), 1)


def spec(
    name: str,
    port: int,
    *,
    required: bool = True,
    collect: tuple[str, ...] = ("market_data",),
) -> GatewaySpec:
    return GatewaySpec(
        name=name, port=port, account="x", collect=collect, required=required
    )


def make_config(
    home: Path,
    *specs: GatewaySpec,
    default: str | None = None,
    health_port: int | None = None,
) -> Config:
    return Config(
        home=home,
        gateways=specs,
        default=default or specs[0].name,
        lines=70,
        snapshots=50,
        health_port=health_port,
    )


async def run_serve(
    config: Config,
    fakes: dict[str, FakeGateway],
    check: Callable[[int | None], Awaitable[None]],
) -> None:
    """Start serve over fakes, run check once it is ready, then stop it."""
    stop = asyncio.Event()
    ready: asyncio.Future[int | None] = asyncio.get_running_loop().create_future()
    task = asyncio.ensure_future(
        serve(
            config,
            FakeClock(),
            make_gateway=lambda s: cast(Gateway, fakes[s.name]),
            stop=stop,
            on_ready=ready.set_result,
        )
    )
    try:
        done, _ = await asyncio.wait(
            {task, cast(asyncio.Future[Any], ready)},
            return_when=asyncio.FIRST_COMPLETED,
        )
        if task in done:
            await task
        port = await ready
        for _ in range(20):
            await asyncio.sleep(0)
        await check(port)
    finally:
        stop.set()
        await task


def get_health(port: int) -> tuple[int, dict[str, Any]]:
    try:
        with urllib.request.urlopen(f"http://localhost:{port}/health", timeout=5) as r:
            return r.status, json.loads(r.read())
    except urllib.error.HTTPError as err:
        return err.code, json.loads(err.read())


def test_serve_sockets_per_name_and_default() -> None:
    async def scenario(home: Path, config: Config, names: set[str]) -> None:
        async def check(_: int | None) -> None:
            run = home / "run"
            assert {p.name for p in run.iterdir()} == names
            assert stat.S_IMODE(run.stat().st_mode) == 0o700
            for p in run.iterdir():
                assert stat.S_ISSOCK(p.lstat().st_mode)
                assert stat.S_IMODE(p.lstat().st_mode) == 0o600

        await run_serve(config, {s.name: FakeGateway() for s in config.gateways}, check)

    import socket

    with short_socket_dir() as home:
        a, b = spec("a", 1), spec("b", 2)
        stale = prepare_socket(home, "a")
        old = socket.socket(socket.AF_UNIX)
        old.bind(str(stale))
        old.close()  # a stale socket file stays
        asyncio.run(
            scenario(
                home,
                make_config(home, a, b, default="b"),
                {"ibkr-a.sock", "ibkr-b.sock", "ibkr.sock"},
            )
        )
    with short_socket_dir() as home:
        asyncio.run(
            scenario(
                home,
                make_config(home, a, b),
                {"ibkr-a.sock", "ibkr-b.sock", "ibkr.sock"},
            )
        )
    with short_socket_dir() as home:
        only = spec("c", 3, collect=("positions",))
        asyncio.run(
            scenario(home, make_config(home, a, only), {"ibkr-a.sock", "ibkr.sock"})
        )
    with short_socket_dir() as home:
        path = socket_path(home, "a")
        path.parent.mkdir(parents=True)
        path.write_text("not a socket")
        with pytest.raises(ConfigError):
            asyncio.run(scenario(home, make_config(home, a), set()))
        assert path.read_text() == "not a socket"


def test_one_supervisor_per_entry() -> None:
    fakes = {n: FakeGateway() for n in ("a", "b", "c")}

    async def check(_: int | None) -> None:
        assert all(f.opened == 1 for f in fakes.values())

    with short_socket_dir() as home:
        config = make_config(
            home, spec("a", 1), spec("b", 2, required=False), spec("c", 3)
        )
        asyncio.run(run_serve(config, fakes, check))


def test_not_required_down_does_not_stop_the_others() -> None:
    fakes = {"a": FakeGateway(), "b": FakeGateway(fail=True)}

    async def check(port: int | None) -> None:
        assert port is not None
        status, body = await asyncio.to_thread(get_health, port)
        assert status == 200 and body == {"gateways": {"a": True, "b": False}}

    with short_socket_dir() as home:
        config = make_config(
            home, spec("a", 1), spec("b", 2, required=False), health_port=0
        )
        asyncio.run(run_serve(config, fakes, check))


def test_serve_starts_and_stops_health() -> None:
    fake = FakeGateway()
    seen: list[int] = []

    async def check(port: int | None) -> None:
        assert port is not None
        seen.append(port)
        assert (await asyncio.to_thread(get_health, port))[0] == 200
        fake.connected = False
        fake.fail = True
        assert (await asyncio.to_thread(get_health, port))[0] == 503

    with short_socket_dir() as home:
        asyncio.run(
            run_serve(
                make_config(home, spec("a", 1), health_port=0), {"a": fake}, check
            )
        )
    with pytest.raises(OSError):
        urllib.request.urlopen(f"http://localhost:{seen[0]}/health", timeout=2)


def test_serve_without_health_port_starts_no_listener() -> None:
    async def check(port: int | None) -> None:
        assert port is None

    with short_socket_dir() as home:
        asyncio.run(
            run_serve(make_config(home, spec("a", 1)), {"a": FakeGateway()}, check)
        )


class RecordingClient:
    def __init__(self) -> None:
        self.sent: list[object] = []

    def send(self, *fields: object, **_: Any) -> None:
        self.sent.append(fields)

    def sendMsg(self, msg: str | None) -> None:
        self.sent.append(msg)


def test_every_gateway_gets_the_guard() -> None:
    guards: list[Callable[[Any], None]] = []

    def build(guard: Callable[[Any], None], **_: Any) -> Gateway:
        guards.append(guard)
        return cast(Gateway, FakeGateway())

    with short_socket_dir() as home:
        config = make_config(home, spec("a", 1), spec("b", 2))
    factory = gateway_factory(config, FakeClock(), build)
    for s in config.gateways:
        factory(s)
    assert len(guards) == 2 and all(g is install_guard for g in guards)
    for guard in guards:
        client = RecordingClient()
        guard(client)
        with pytest.raises(Exception):  # noqa: B017 - the guard's refusal type is its own
            client.send(3, 1)  # an order id
        assert client.sent == []
