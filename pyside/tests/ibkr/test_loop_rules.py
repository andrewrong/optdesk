"""One event loop, no patching: what the adapter may call and how the loop is shared."""

from __future__ import annotations

import ast
import asyncio
from pathlib import Path
from typing import Self

import grpc
from kit.fakeclock import FakeClock
from kit.sockdir import short_socket_dir

from optdesk.contracts import api as pb
from optdesk.ibkr.ibgateway import IbGateway
from optdesk.ibkr.model import SessionState
from optdesk.ibkr.transport import build_server

SRC = Path(__file__).resolve().parents[2] / "src" / "optdesk"
IBKR = SRC / "ibkr"

# Methods of `ib` and `client` the package may call: async ones, the two sends
# that are one write each, and the guard's own wrapping.
ALLOWED = {
    "connectAsync",
    "reqContractDetailsAsync",
    "reqSecDefOptParamsAsync",
    "reqHistoricalDataAsync",
    "reqMarketDataType",
    "reqMktData",
    "cancelMktData",
    "send",
    "sendMsg",
}

# The calls of the account read: accepted in accounts.py only, refused elsewhere.
ACCOUNT_ONLY = {
    "reqAccountSummary",
    "reqPositions",
    "cancelAccountSummary",
    "cancelPositions",
    "managedAccounts",
}
ACCOUNT_FILE = "accounts.py"

# The one call of the read-only probe: accepted in readonly_probe.py only.
PROBE_ONLY = {"reqOpenOrdersAsync"}
PROBE_FILE = "readonly_probe.py"


async def probe_passes() -> None:
    """A read-only probe that passes, in place of the second connection."""


def _receiver(call: ast.Call) -> str | None:
    func = call.func
    if not isinstance(func, ast.Attribute):
        return None
    target = func.value
    if isinstance(target, ast.Name) and target.id in {"ib", "client"}:
        return target.id
    if isinstance(target, ast.Attribute) and target.attr == "client":
        return "client"
    return None


def _calls_by_file() -> dict[str, set[str]]:
    found: dict[str, set[str]] = {}
    for path in sorted(IBKR.glob("*.py")):
        names = found.setdefault(path.name, set())
        for node in ast.walk(ast.parse(path.read_text())):
            if isinstance(node, ast.Call) and _receiver(node):
                assert isinstance(node.func, ast.Attribute)
                names.add(node.func.attr)
    return found


def test_only_async_calls_on_ib_and_client() -> None:
    by_file = _calls_by_file()
    for name, calls in by_file.items():
        allowed = ALLOWED | ACCOUNT_ONLY if name == ACCOUNT_FILE else ALLOWED
        if name == PROBE_FILE:
            allowed = ALLOWED | PROBE_ONLY
        assert calls <= allowed, (name, calls - allowed)
    assert "connectAsync" in set().union(*by_file.values())


def test_account_calls_only_in_accounts_py() -> None:
    by_file = _calls_by_file()
    requests = ACCOUNT_ONLY - {"managedAccounts"}
    assert requests <= by_file[ACCOUNT_FILE]
    for name, calls in by_file.items():
        if name != ACCOUNT_FILE:
            assert not calls & ACCOUNT_ONLY, name


def test_no_module_imports_nest_asyncio() -> None:
    for path in SRC.rglob("*.py"):
        for node in ast.walk(ast.parse(path.read_text())):
            names: list[str] = []
            if isinstance(node, ast.Import):
                names = [alias.name for alias in node.names]
            elif isinstance(node, ast.ImportFrom):
                names = [node.module or ""]
            assert not any("nest_asyncio" in name for name in names), path


class Turns:
    """Counts loop turns between two awaits: a stalled loop shows as a large gap."""

    def __init__(self) -> None:
        self.count = 0
        self.worst = 0
        self.stop = False

    async def run(self) -> None:
        last = 0
        while not self.stop:
            await asyncio.sleep(0)
            self.count += 1
            last = self.count
        self.worst = last


class LoopGateway:
    """A gateway that records the loop its status runs on; the rest is not called."""

    def __init__(self) -> None:
        self.loops: list[asyncio.AbstractEventLoop] = []

    async def status(self) -> SessionState:
        self.loops.append(asyncio.get_running_loop())
        return SessionState(True, 1, 1)


class QuietEvent:
    def __iadd__(self, handler: object) -> Self:
        return self


class QuietClient:
    def __init__(self) -> None:
        self.apiEnd = QuietEvent()

    async def connectAsync(self, host: str, port: int, client_id: int) -> None:
        return None


class QuietIb:
    def __init__(self) -> None:
        self.client = QuietClient()
        self.errorEvent = QuietEvent()
        self.RaiseRequestErrors = False

    def reqMarketDataType(self, value: int) -> None:
        return None


def test_one_loop_no_patch() -> None:
    async def run() -> tuple[bool, bool, int, bool]:
        loop = asyncio.get_running_loop()
        gateway = LoopGateway()
        turns = Turns()
        monitor = asyncio.ensure_future(turns.run())
        events = asyncio.ensure_future(asyncio.sleep(0))
        with short_socket_dir() as directory:
            target = f"unix:{directory / 's.sock'}"
            server = build_server(gateway)  # type: ignore[arg-type]
            server.add_insecure_port(target)
            await server.start()
            async with grpc.aio.insecure_channel(target) as channel:
                reply = await pb.SessionServiceStub(channel).Ping(pb.PingRequest())
            await server.stop(None)
        await events
        turns.stop = True
        await monitor
        return (
            hasattr(loop, "_nest_patched"),
            gateway.loops == [loop],
            turns.count,
            reply.connected,
        )

    patched, same, turns, connected = asyncio.run(run())
    assert not patched and same and connected and turns > 0


def test_gateway_runs_under_plain_asyncio_run() -> None:
    async def run() -> bool:
        gateway = IbGateway(
            lambda client: None,
            host="h",
            port=1,
            client_id=33,
            clock=FakeClock(),
            ib_factory=QuietIb,
            readonly_probe=probe_passes,
        )
        await gateway.open_session()
        state = await gateway.status()
        return state.connected and not hasattr(
            asyncio.get_running_loop(), "_nest_patched"
        )

    assert asyncio.run(run())
