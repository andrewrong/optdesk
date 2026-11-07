"""The log filter: whitelist of fields, scrub of account shapes, silenced ib_async."""

from __future__ import annotations

import logging
from collections.abc import Iterator

import pytest

from optdesk.ibkr.transport import MAX_TEXT, configure_logging, log_event, scrub


def _shape() -> str:
    # built from two pieces so no file of the repository carries the literal
    return "DU" + "1234567"


def _alias() -> str:
    return "acct_" + "abc"


class _Capture(logging.Handler):
    def __init__(self) -> None:
        super().__init__(logging.DEBUG)
        self.messages: list[str] = []

    def emit(self, record: logging.LogRecord) -> None:
        self.messages.append(self.format(record))


@pytest.fixture
def capture() -> Iterator[_Capture]:
    configure_logging()
    handler = _Capture()
    root = logging.getLogger()
    root.addHandler(handler)
    before = root.level
    root.setLevel(logging.DEBUG)
    yield handler
    root.setLevel(before)
    root.removeHandler(handler)


def test_whitelist_drops_unknown_field(capture: _Capture) -> None:
    log_event(
        logging.getLogger("t.whitelist"),
        "rpc_done",
        rpc="ping",
        count=3,
        account="someone",
        status="not a word",
    )
    (line,) = capture.messages
    assert "rpc=ping" in line and "count=3" in line
    assert "account" not in line and "someone" not in line and "not a word" not in line


def test_account_shape_is_scrubbed(capture: _Capture) -> None:
    log = logging.getLogger("t.scrub")
    log.warning("connected as %s", _shape())
    log.warning("alias %s here", _alias())
    assert len(capture.messages) == 2
    for line in capture.messages:
        assert _shape() not in line and _alias() not in line
    assert "connected as" in capture.messages[0]


def test_error_text_passes_scrub() -> None:
    text = f"No security definition for account {_shape()} found"
    clean = scrub(text)
    assert _shape() not in clean
    assert clean.startswith("No security definition")


def test_ib_async_debug_never_reaches_handler(capture: _Capture) -> None:
    for name in ("ib_async.client", "ib_async.wrapper"):
        logging.getLogger(name).debug(">>> accounts %s", _shape())
    assert capture.messages == []


def test_long_free_text_dropped() -> None:
    assert scrub("x" * MAX_TEXT) == "x" * MAX_TEXT
    assert scrub("x" * (MAX_TEXT + 1)) == ""
