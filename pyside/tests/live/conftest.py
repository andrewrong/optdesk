"""Switch, socket directory and channel fixtures of the live suite."""

from __future__ import annotations

import os
from collections.abc import Iterator
from pathlib import Path

import grpc
import pytest

from live.client import LiveClient

SWITCH = "OT_LIVE_PYSIDE"
_ALWAYS = "test_guard.py"

DEFAULT_SOCKET = "ibkr.sock"
MAIN_SOCKET = "ibkr-main.sock"
WIFE_SOCKET = "ibkr-wife.sock"


def pytest_collection_modifyitems(
    config: pytest.Config, items: list[pytest.Item]
) -> None:
    if os.environ.get(SWITCH) == "1":
        return
    skip = pytest.mark.skip(reason=f"live suite: set {SWITCH}=1 to run")
    for item in items:
        if Path(str(item.fspath)).name == _ALWAYS:
            continue
        if "tests/live" in Path(str(item.fspath)).as_posix():
            item.add_marker(skip)


@pytest.fixture(scope="session")
def run_dir() -> Path:
    home = os.environ.get("OT_HOME")
    if not home:
        pytest.skip("OT_HOME is not set")
    return Path(home) / "run"


def _client(path: Path) -> Iterator[LiveClient]:
    channel = grpc.insecure_channel("unix:" + str(path))
    try:
        yield LiveClient(channel)
    finally:
        channel.close()


@pytest.fixture(scope="session")
def default_client(run_dir: Path) -> Iterator[LiveClient]:
    yield from _client(run_dir / DEFAULT_SOCKET)


@pytest.fixture(scope="session")
def main_client(run_dir: Path) -> Iterator[LiveClient]:
    yield from _client(run_dir / MAIN_SOCKET)


@pytest.fixture(scope="session")
def wife_client(run_dir: Path) -> Iterator[LiveClient]:
    yield from _client(run_dir / WIFE_SOCKET)
