"""One log line per state change of supervise, and the size ceilings of the two hooked files."""

from __future__ import annotations

import asyncio
import logging
from pathlib import Path

import pytest
from kit.fakeclock import FakeClock

import optdesk.ibkr.ibgateway as ibgateway_module
import optdesk.ibkr.server as server_module
from optdesk.ibkr.model import SessionState
from optdesk.ibkr.server import supervise

DAY_S = 24 * 3600


class DownThenUp:
    """open_session fails with a message that holds a port and an account id until `up_at`."""

    def __init__(self, clock: FakeClock, up_at: float) -> None:
        self.clock = clock
        self.up_at = up_at
        self.opened = False

    async def open_session(self) -> None:
        if self.clock.monotonic() < self.up_at:
            raise ConnectionError("refused at port 4001 for account ACCT-MARKER")
        self.opened = True

    async def status(self) -> SessionState:
        return SessionState(self.opened, 1, 1)


class TestLogOnStateChange:
    def test_a_day_down_logs_one_session_down_then_one_session_up(
        self, caplog: pytest.LogCaptureFixture
    ) -> None:
        async def run() -> None:
            clock = FakeClock()
            gateway = DownThenUp(clock, DAY_S)
            task = asyncio.ensure_future(supervise(gateway, clock, "main"))
            for _ in range(DAY_S // 60):
                await clock.advance(60)
            before = [r.getMessage() for r in caplog.records]
            assert [m for m in before if m.startswith("session_down")] == [
                "session_down gateway=main"
            ]
            assert len(before) == 1  # no line per attempt
            for _ in range(3):
                await clock.advance(60)
            task.cancel()
            await asyncio.gather(task, return_exceptions=True)

        with caplog.at_level(logging.INFO, logger="optdesk.ibkr.server"):
            asyncio.run(run())
        lines = [r.getMessage() for r in caplog.records]
        assert len([m for m in lines if m.startswith("session_down")]) == 1
        ups = [m for m in lines if m.startswith("session_up")]
        assert len(ups) == 1 and "last_failure=ConnectionError" in ups[0]
        assert all("gateway=main" in m for m in lines)
        text = "\n".join(lines)
        assert "4001" not in text and "ACCT-MARKER" not in text


class TestSizeCeiling:
    @pytest.mark.parametrize("module", [ibgateway_module, server_module])
    def test_the_two_hooked_files_stay_under_800_lines(self, module: object) -> None:
        file = getattr(module, "__file__", None)
        assert file is not None
        assert len(Path(file).read_text().splitlines()) < 800
