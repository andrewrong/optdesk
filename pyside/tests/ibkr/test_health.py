"""The health listener: status code, body, concurrency and shutdown."""

from __future__ import annotations

import json
import socket
import threading
import urllib.error
import urllib.request
from collections.abc import Iterator
from concurrent.futures import Future
from contextlib import contextmanager

import pytest

from optdesk.ibkr.health import HealthServer, Probes, render, start_health


def fetch(
    port: int, path: str = "/health", method: str = "GET"
) -> tuple[int, dict[str, object]]:
    request = urllib.request.Request(f"http://localhost:{port}{path}", method=method)
    try:
        with urllib.request.urlopen(request, timeout=5) as response:
            return response.status, json.loads(response.read())
    except urllib.error.HTTPError as err:
        return err.code, json.loads(err.read())


@contextmanager
def running(probes: Probes, **more: object) -> Iterator[HealthServer]:
    server = start_health(probes, 0, **more)  # type: ignore[arg-type]
    try:
        yield server
    finally:
        server.stop()


def up() -> bool:
    return True


def down() -> bool:
    return False


def test_all_required_connected_gives_200() -> None:
    with running({"a": (True, up), "b": (True, up)}) as server:
        assert fetch(server.port) == (200, {"gateways": {"a": True, "b": True}})


def test_a_required_down_gives_503() -> None:
    with running({"a": (True, up), "b": (True, down)}) as server:
        assert fetch(server.port)[0] == 503


def test_not_required_never_changes_the_code() -> None:
    for extra, required_probe, code in (
        (up, up, 200),
        (down, up, 200),
        (up, down, 503),
        (down, down, 503),
    ):
        with running({"r": (True, required_probe), "o": (False, extra)}) as server:
            assert fetch(server.port)[0] == code


def test_body_has_one_key_per_gateway() -> None:
    with running({"a": (True, up), "b": (False, down), "c": (False, up)}) as server:
        _, body = fetch(server.port)
    assert body == {"gateways": {"a": True, "b": False, "c": True}}


def test_body_holds_no_port_host_or_alias() -> None:
    status, body = render({"first": True}, ["first"])
    assert status == 200 and json.loads(body) == {"gateways": {"first": True}}
    assert set(json.loads(body)) == {"gateways"}


def test_probe_that_raises_counts_as_down() -> None:
    def boom() -> bool:
        raise RuntimeError("no")

    with running({"a": (True, up), "b": (False, boom)}) as server:
        assert fetch(server.port) == (200, {"gateways": {"a": True, "b": False}})
    with running({"a": (True, boom)}) as server:
        assert fetch(server.port)[0] == 503


def test_probes_run_concurrently() -> None:
    barrier = threading.Barrier(2, timeout=5)

    def meet() -> bool:
        barrier.wait()
        return True

    with running({"a": (True, meet), "b": (True, meet)}) as server:
        assert fetch(server.port) == (200, {"gateways": {"a": True, "b": True}})


def test_blocked_probe_counts_down_at_the_deadline() -> None:
    release = threading.Event()

    def blocked() -> bool:
        release.wait(10)
        return True

    def at_deadline(
        futures: list[Future[bool]], timeout: float
    ) -> tuple[set[Future[bool]], set[Future[bool]]]:
        return set(), set(futures)

    try:
        with running({"a": (True, blocked)}, wait=at_deadline) as server:
            assert fetch(server.port) == (503, {"gateways": {"a": False}})
    finally:
        release.set()


def test_other_path_is_404() -> None:
    with running({"a": (True, up)}) as server:
        assert fetch(server.port, "/other")[0] == 404


def test_other_method_is_405() -> None:
    with running({"a": (True, up)}) as server:
        assert fetch(server.port, method="POST")[0] == 405
        assert fetch(server.port, method="DELETE")[0] == 405


def test_listener_binds_localhost_only() -> None:
    with running({"a": (True, up)}) as server:
        assert server.port > 0
        assert server.host == "127.0.0.1"
        assert fetch(server.port)[0] == 200


def test_stop_ends_the_thread() -> None:
    server = start_health({"a": (True, up)}, 0)
    assert fetch(server.port)[0] == 200 and server.alive
    server.stop()
    assert not server.alive
    with (
        pytest.raises(OSError),
        socket.create_connection(("127.0.0.1", server.port), timeout=2),
    ):
        pass
