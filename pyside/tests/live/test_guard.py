"""Guards of the live suite; runs in CI and never connects."""

from __future__ import annotations

import re
from pathlib import Path

from live.allowed import ALLOWED_RPCS
from optdesk.contracts import api

HERE = Path(__file__).parent


class _Recorder:
    """A channel that records the rpc names a stub registers and connects to nothing."""

    def __init__(self) -> None:
        self.names: set[str] = set()

    def _register(self, method: str, *_: object, **__: object) -> object:
        self.names.add(method.rsplit("/", 1)[-1])
        return lambda *a, **k: None

    unary_unary = unary_stream = stream_unary = stream_stream = _register


def _generated_rpcs() -> set[str]:
    recorder = _Recorder()
    for name in dir(api):
        if name.endswith("ServiceStub"):
            getattr(api, name)(recorder)
    return recorder.names


def _sources() -> dict[str, str]:
    return {p.name: p.read_text() for p in sorted(HERE.glob("*.py"))}


def test_guard_allowed_rpcs_exist_in_generated_stubs() -> None:
    rpcs = _generated_rpcs()
    assert ALLOWED_RPCS <= rpcs


def test_guard_only_allowed_methods_are_called() -> None:
    sources = _sources()
    used = set(re.findall(r"_stub\.(\w+)\(|self\._\w+\.(\w+)\(", sources["client.py"]))
    called = {a or b for a, b in used}
    assert called <= ALLOWED_RPCS
    rpcs = _generated_rpcs()
    for name, text in sources.items():
        if name in ("client.py", "test_guard.py"):
            continue
        direct = set(re.findall(r"\.([A-Z]\w*)\(", text))
        assert not (direct & rpcs), f"{name} calls a stub directly"


def test_guard_no_write_words_outside_guard() -> None:
    pattern = re.compile("order|place|cancel", re.IGNORECASE)
    for name, text in _sources().items():
        if name == "test_guard.py":
            continue
        assert not pattern.search(text), name
