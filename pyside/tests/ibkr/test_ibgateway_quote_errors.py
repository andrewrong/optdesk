"""A quote target that fails is answered on its own; the other targets keep their rows."""

from __future__ import annotations

import asyncio
import logging

import pytest
from kit.fakeclock import FakeClock
from test_ibgateway import collect, opened, spin


def test_quotes_unexpected_error_keeps_other_targets(
    caplog: pytest.LogCaptureFixture,
) -> None:
    async def run(failing: bool) -> dict[int, tuple[str, int]]:
        clock = FakeClock()
        gateway, fake = await opened(clock, lines=3)
        fake.autofill = True
        if failing:
            fake.fail_for[82] = RuntimeError("boom")
        rows, task = await collect(gateway, "snapshot", [81, 82, 83])
        await spin(50)
        await clock.advance(21)
        await task
        return {r.con_id: (r.status, r.error_code) for r in rows}

    baseline = asyncio.run(run(False))
    with caplog.at_level(logging.ERROR):
        got = asyncio.run(run(True))
    assert sorted(got) == [81, 82, 83]
    assert got[82] == ("error", 0)
    assert got[81] == baseline[81]
    assert got[83] == baseline[83]
    failed = [
        r.getMessage() for r in caplog.records if "quote_failed" in r.getMessage()
    ]
    assert len(failed) == 1
    assert "type=RuntimeError" in failed[0]
    assert "82" not in failed[0]


def test_quotes_connection_error_still_raises() -> None:
    async def run() -> None:
        clock = FakeClock()
        gateway, fake = await opened(clock, lines=3)
        fake.autofill = True
        fake.fail_for[92] = ConnectionError("gone")
        _, task = await collect(gateway, "snapshot", [91, 92, 93])
        await spin(50)
        await clock.advance(21)
        await task

    with pytest.raises(ConnectionError):
        asyncio.run(run())
