"""Session codes of the gateway: 2110, 1101, the 10197 policy; a client id of 0 is refused."""

from __future__ import annotations

import asyncio
from pathlib import Path

import pytest
from kit.fakeclock import FakeClock
from test_ibgateway import FakeIb, fill, make, spin

from optdesk.ibkr import spec
from optdesk.ibkr.gateways import ConfigError, from_env, load_gateways
from optdesk.ibkr.ibgateway import IbGateway
from optdesk.ibkr.model import QuoteRow, QuoteTarget


async def _opened(clock: FakeClock) -> tuple[IbGateway, FakeIb]:
    gateway, fake = make(clock)
    await gateway.open_session()
    return gateway, fake


def test_2110_marks_the_session_lost() -> None:
    async def run() -> tuple[bool, bool]:
        gateway, fake = await _opened(FakeClock())
        before = (await gateway.status()).connected
        fake.errorEvent.emit(-1, 2110, "connectivity broken", None)
        return before, (await gateway.status()).connected

    assert asyncio.run(run()) == (True, False)


def test_2110_ends_a_waiting_quote_disconnected() -> None:
    async def run() -> list[str]:
        gateway, fake = await _opened(FakeClock())
        rows: list[QuoteRow] = []

        async def keep(row: QuoteRow) -> None:
            rows.append(row)

        task = asyncio.ensure_future(
            gateway.quotes(spec.SNAPSHOT, [QuoteTarget(41)], keep)
        )
        await spin()
        fake.errorEvent.emit(-1, 2110, "connectivity broken", None)
        await task
        return [row.status for row in rows]

    assert asyncio.run(run()) == ["disconnected"]


def test_1101_gives_a_new_generation_and_asks_the_old_lines_again() -> None:
    async def run() -> tuple[int, int, list[int], list[str]]:
        gateway, fake = await _opened(FakeClock())
        first = (await gateway.status()).generation
        rows: list[QuoteRow] = []

        async def keep(row: QuoteRow) -> None:
            rows.append(row)

        task = asyncio.ensure_future(
            gateway.quotes(spec.SNAPSHOT, [QuoteTarget(41)], keep)
        )
        await spin()
        ticker = fake.tickers[41]  # ib_async hands the same ticker back
        fake.errorEvent.emit(-1, 1101, "data lost", None)
        fill(ticker)
        ticker.updateEvent.emit(ticker)
        await task
        return (
            first,
            (await gateway.status()).generation,
            fake.subscribed,
            [row.status for row in rows],
        )

    first, after, subscribed, statuses = asyncio.run(run())
    assert after == first + 1
    assert subscribed == [41, 41]
    assert statuses == ["ok"]


def test_1100_then_1101_revives_the_session_with_a_new_generation() -> None:
    async def run() -> tuple[bool, int, int]:
        gateway, fake = await _opened(FakeClock())
        first = (await gateway.status()).generation
        fake.errorEvent.emit(-1, 1100, "lost", None)
        fake.errorEvent.emit(-1, 1101, "restored, data lost", None)
        state = await gateway.status()
        return state.connected, first, state.generation

    connected, first, after = asyncio.run(run())
    assert connected and after == first + 1


def test_1101_does_not_revive_a_session_the_socket_ended() -> None:
    async def run() -> bool:
        gateway, fake = await _opened(FakeClock())
        for handler in list(fake.client.apiEnd.handlers):
            handler()
        fake.errorEvent.emit(-1, 1101, "restored, data lost", None)
        return (await gateway.status()).connected

    assert asyncio.run(run()) is False


class Run:
    """A quotes() call of one stock and one option held under a 10197."""

    def __init__(self) -> None:
        self.clock = FakeClock()
        self.rows: dict[int, QuoteRow] = {}

    async def start(self) -> tuple[FakeIb, asyncio.Future[None]]:
        gateway, fake = make(self.clock)
        await gateway.open_session()

        async def keep(row: QuoteRow) -> None:
            self.rows[row.con_id] = row

        task = asyncio.ensure_future(
            gateway.quotes(
                spec.SNAPSHOT,
                [QuoteTarget(40, sec_type="STK"), QuoteTarget(42, sec_type="OPT")],
                keep,
            )
        )
        await spin()
        fill(fake.tickers[40])
        fake.tickers[40].updateEvent.emit(fake.tickers[40])
        await spin()
        fake.errorEvent.emit(-1, 10197, "competing session", None)
        await spin()
        return fake, task

    def statuses(self) -> dict[int, str]:
        return {con_id: row.status for con_id, row in self.rows.items()}


def test_10197_keeps_received_quotes_and_cancels_the_lines() -> None:
    async def run() -> tuple[dict[int, str], list[int], bool]:
        run = Run()
        fake, task = await run.start()
        return run.statuses(), fake.cancelled, task.done()

    statuses, cancelled, done = asyncio.run(run())
    assert statuses == {40: "ok"}
    assert 42 in cancelled
    assert not done


def test_10197_probe_waits_5_then_10_then_20_seconds() -> None:
    async def run() -> list[int]:
        run = Run()
        fake, task = await run.start()
        before = len(fake.subscribed)
        seen: list[int] = []
        # each probe looks for 5 s, then the next interval runs: probes at 5, 20, 45 s
        for step in (4, 1, 14, 1, 24, 1):
            await run.clock.advance(step)
            seen.append(len(fake.subscribed) - before)
        task.cancel()
        await asyncio.gather(task, return_exceptions=True)
        return seen

    seen = asyncio.run(run())
    # one underlying and one option: two lines per probe
    assert seen == [0, 2, 2, 4, 4, 6]


def test_10197_deadline_ends_the_unfinished_rows_disconnected() -> None:
    async def run() -> tuple[dict[int, str], dict[int, int]]:
        run = Run()
        _, task = await run.start()
        await run.clock.advance(120)
        await task
        return run.statuses(), {c: r.error_code for c, r in run.rows.items()}

    statuses, codes = asyncio.run(run())
    assert statuses == {40: "ok", 42: "disconnected"}
    assert codes == {40: 0, 42: 0}


def test_10197_unfinished_items_are_retried_when_the_probe_answers() -> None:
    async def run() -> tuple[dict[int, str], int]:
        run = Run()
        fake, task = await run.start()
        fake.autofill = True  # the market data is back: probe and retry both fill
        await run.clock.advance(10)
        await task
        return run.statuses(), fake.subscribed.count(42)

    statuses, requests = asyncio.run(run())
    assert statuses == {40: "ok", 42: "ok"}
    assert requests >= 3  # the first line, the probe, the retry


def test_probe_targets_pick_one_underlying_and_one_option() -> None:
    targets = [
        QuoteTarget(1, sec_type="OPT"),
        QuoteTarget(2, sec_type="STK"),
        QuoteTarget(3, sec_type="OPT"),
        QuoteTarget(4, sec_type="IND"),
    ]
    assert [t.con_id for t in spec.probe_targets(targets)] == [2, 1]
    assert [t.con_id for t in spec.probe_targets(targets[:1])] == [1]
    assert spec.probe_targets([]) == []


def _gateway_file(tmp_path: Path, client_id: int) -> Path:
    path = tmp_path / "gateways.toml"
    path.write_text(
        "[[gateway]]\n"
        'name = "main"\nport = 4002\naccount = "main"\n'
        f'collect = ["market_data"]\nrequired = true\nclient_id = {client_id}\n'
    )
    return path


def test_client_id_0_in_the_gateways_file_is_rejected(tmp_path: Path) -> None:
    with pytest.raises(ConfigError, match="reserved for the read-only probe"):
        load_gateways(_gateway_file(tmp_path, 0))


def test_a_negative_client_id_is_rejected(tmp_path: Path) -> None:
    with pytest.raises(ConfigError, match="client_id"):
        load_gateways(_gateway_file(tmp_path, -3))


def test_client_id_33_is_accepted(tmp_path: Path) -> None:
    specs, _ = load_gateways(_gateway_file(tmp_path, 33))
    assert specs[0].client_id == 33


def test_client_id_0_of_the_single_gateway_keys_is_rejected() -> None:
    env = {"OT_IBKR_GATEWAY_PORT": "4002", "OT_IBKR_CLIENT_ID": "0"}
    with pytest.raises(ConfigError, match="reserved for the read-only probe"):
        from_env(env)
