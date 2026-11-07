"""The health URL: GET /health on localhost, 200 when every required gateway is connected.

The probes run on every request, all at once, under one deadline; a probe that
raises or has not answered by the deadline counts as not connected. The body
is {"gateways": {"<name>": true|false}} and holds nothing else.
"""

from __future__ import annotations

import concurrent.futures
import json
import socketserver
import threading
from collections.abc import Callable, Collection, Mapping
from concurrent.futures import Future, ThreadPoolExecutor
from http import HTTPStatus
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Any
from urllib.parse import urlsplit

PROBE_DEADLINE_S = 2.0
HOST = "localhost"

Probe = Callable[[], bool]
Probes = Mapping[str, tuple[bool, Probe]]
Wait = Callable[
    [list[Future[bool]], float],
    tuple[set[Future[bool]], set[Future[bool]]],
]


def render(states: Mapping[str, bool], required: Collection[str]) -> tuple[int, bytes]:
    """Return the status code and the body: 200 when every name in required is up."""
    ok = all(states.get(name, False) for name in required)
    body = json.dumps({"gateways": dict(states)}).encode()
    code = HTTPStatus.OK if ok else HTTPStatus.SERVICE_UNAVAILABLE
    return int(code), body


def _read(probes: Probes, wait: Wait) -> dict[str, bool]:
    names = list(probes)
    pool = ThreadPoolExecutor(max_workers=max(1, len(names)))
    try:
        futures = {name: pool.submit(probes[name][1]) for name in names}
        done, _ = wait(list(futures.values()), PROBE_DEADLINE_S)
        states: dict[str, bool] = {}
        for name, future in futures.items():
            states[name] = (
                future in done
                and future.exception() is None
                and future.result() is True
            )
        return states
    finally:
        pool.shutdown(wait=False, cancel_futures=True)


class _Server(ThreadingHTTPServer):
    """Binds localhost over IPv4; skips the fully qualified name lookup of HTTPServer."""

    probes: Probes
    wait: Wait

    def server_bind(self) -> None:
        socketserver.TCPServer.server_bind(self)
        host, port = self.server_address[:2]
        self.server_name = str(host)
        self.server_port = port


class _Handler(BaseHTTPRequestHandler):
    server: _Server

    def _reply(self, code: int, body: bytes) -> None:
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        if self.command != "HEAD":
            self.wfile.write(body)

    def do_GET(self) -> None:
        if urlsplit(self.path).path != "/health":
            self._reply(int(HTTPStatus.NOT_FOUND), b"{}")
            return
        probes = self.server.probes
        states = _read(probes, self.server.wait)
        required = [name for name, (needed, _) in probes.items() if needed]
        self._reply(*render(states, required))

    def _not_allowed(self) -> None:
        self._reply(int(HTTPStatus.METHOD_NOT_ALLOWED), b"{}")

    do_HEAD = do_POST = do_PUT = do_DELETE = do_PATCH = do_OPTIONS = _not_allowed

    def log_message(self, format: str, *args: Any) -> None:
        """Write nothing: crew polls every few seconds."""


class HealthServer:
    """A started listener; port is the bound port."""

    def __init__(self, server: _Server, thread: threading.Thread) -> None:
        self._server = server
        self._thread = thread
        self.host: str = str(server.server_address[0])
        self.port: int = int(server.server_address[1])

    @property
    def alive(self) -> bool:
        """Whether the serving thread still runs."""
        return self._thread.is_alive()

    def stop(self) -> None:
        """Stop serving, close the socket and wait for the thread; blocks."""
        self._server.shutdown()
        self._server.server_close()
        self._thread.join()


def start_health(
    probes: Probes,
    port: int,
    wait: Wait = concurrent.futures.wait,
) -> HealthServer:
    """Listen on localhost:port (0 lets the OS choose) and serve GET /health.

    probes maps a gateway name to (required, probe). wait is the seam a test
    replaces; it is never set outside tests. A bind failure raises OSError.
    """
    server = _Server((HOST, port), _Handler)
    server.probes = probes
    server.wait = wait
    thread = threading.Thread(target=server.serve_forever, name="health", daemon=True)
    thread.start()
    return HealthServer(server, thread)
