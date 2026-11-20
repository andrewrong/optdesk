"""The farm board: codes 2100 to 2199, the farm word, and a 2105 read at connect."""

from __future__ import annotations

import asyncio
import logging

import pytest
from kit.fakeclock import FakeClock
from test_ibgateway import FakeIb, probe_passes

from optdesk.ibkr.farms import Farms, farm_word
from optdesk.ibkr.session_probe import ProbedGateway


def _board(clock: FakeClock | None = None) -> Farms:
    return Farms("main", clock or FakeClock())


class TestFarmCodes:
    @pytest.mark.parametrize("code", [2103, 2105, 2157])
    def test_down_codes_set_the_named_farm_down(self, code: int) -> None:
        board = _board()
        board.record(code, "A farm is broken:ushmds", 1000)
        assert [(f.name, f.up, f.since) for f in board.snapshot()] == [
            ("ushmds", False, 1000)
        ]

    @pytest.mark.parametrize("code", [2104, 2106, 2158])
    def test_up_codes_set_the_named_farm_up(self, code: int) -> None:
        board = _board()
        board.record(2105, "broken:ushmds", 1000)
        board.record(code, "is OK:ushmds", 2000)
        assert [(f.name, f.up, f.since) for f in board.snapshot()] == [
            ("ushmds", True, 2000)
        ]

    @pytest.mark.parametrize(
        "text", ["no colon here", "x:USHMDS", "x:has space", "x:" + "a" * 17, "x:"]
    )
    def test_a_name_outside_the_rule_is_other(self, text: str) -> None:
        assert farm_word(text) == "other"
        board = _board()
        board.record(2105, text, 1)
        assert [f.name for f in board.snapshot()] == ["other"]

    def test_the_word_after_the_last_colon_counts(self) -> None:
        assert farm_word("a: b: us.farm1") == "us.farm1"

    @pytest.mark.parametrize("code", [2100, 2101, 2102, 2107, 2108, 2109, 2110, 2199])
    def test_other_codes_of_2100_to_2199_change_no_farm(self, code: int) -> None:
        board = _board()
        board.record(2105, "x:ushmds", 1)
        before = board.snapshot()
        board.record(code, "x:ushmds", 99)
        assert board.snapshot() == before

    def test_a_2105_read_at_connect_shows_the_farm_down(self) -> None:
        async def run() -> tuple[tuple[str, bool], ...]:
            clock = FakeClock()
            fake = FakeIb(clock)

            async def connect(host: str, port: int, client_id: int) -> None:
                fake.errorEvent.emit(
                    -1, 2105, "HMDS data farm connection is broken:ushmds", None
                )

            fake.client.connectAsync = connect  # type: ignore[method-assign]
            gateway = ProbedGateway(
                lambda client: None,
                host="h",
                port=1,
                client_id=33,
                clock=clock,
                ib_factory=lambda: fake,
                readonly_probe=probe_passes,
                name="main",
            )
            await gateway.open_session()
            state = await gateway.status()
            return tuple((f.name, f.up) for f in state.farms)

        assert asyncio.run(run()) == (("ushmds", False),)

    def test_a_farm_line_names_the_gateway_and_no_more(
        self, caplog: pytest.LogCaptureFixture
    ) -> None:
        board = _board()
        with caplog.at_level(logging.INFO, logger="optdesk.ibkr.farms"):
            now = board.now_ms()
            board.record(2105, "x:ushmds", now)
            board.record(2105, "x:ushmds", now)  # no change, no line
            board.record(2107, "x:ushmds", now)  # not one of the six
        assert [r.getMessage() for r in caplog.records] == [
            "farm_code gateway=main code=2105 farm=ushmds"
        ]
