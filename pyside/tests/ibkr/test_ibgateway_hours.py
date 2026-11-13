"""A resolve carries the trading hours, liquid hours and time zone of a found contract."""

from __future__ import annotations

import asyncio
from typing import Any

from ib_async import Contract, ContractDetails, RequestError
from kit.fakeclock import FakeClock
from test_ibgateway import _spec, opened

HOURS = "20261116:0930-20261116:1600;20261117:CLOSED"
LIQUID = "20261116:0930-20261116:1600"
ZONE = "US/Eastern"


def _details() -> list[ContractDetails]:
    return [
        ContractDetails(
            contract=Contract(conId=7, exchange="SMART", tradingClass="SPY"),
            tradingHours=HOURS,
            liquidHours=LIQUID,
            timeZoneId=ZONE,
        )
    ]


def test_resolve_found_carries_the_three_strings() -> None:
    async def run() -> Any:
        gateway, fake = await opened(FakeClock())

        async def answer(contract: Contract) -> list[ContractDetails]:
            return _details()

        fake.details = answer
        return await gateway.resolve(**_spec())

    result = asyncio.run(run())
    assert result.found
    assert result.trading_hours == HOURS
    assert result.liquid_hours == LIQUID
    assert result.time_zone_id == ZONE


def test_resolve_not_listed_and_empty_leave_them_empty() -> None:
    async def run() -> tuple[Any, Any]:
        gateway, fake = await opened(FakeClock())

        async def never(contract: Contract) -> list[ContractDetails]:
            raise RequestError(1, 200, "No security definition")

        fake.details = never
        missing = await gateway.resolve(**_spec())
        fake.details = fake._none
        empty = await gateway.resolve(**_spec())
        return missing, empty

    for result in asyncio.run(run()):
        assert not result.found
        assert (result.trading_hours, result.liquid_hours, result.time_zone_id) == (
            "",
            "",
            "",
        )
