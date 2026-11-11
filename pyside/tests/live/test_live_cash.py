"""Read-only live test of USD.JPY: resolve and MIDPOINT daily bars; skipped unless OT_LIVE_PYSIDE=1 (see conftest.py)."""

from __future__ import annotations

from decimal import Decimal

from live.client import LiveClient
from optdesk.contracts import api

BAR_DAYS = 5


def test_live_cash_usd_jpy_midpoint_bars(main_client: LiveClient) -> None:
    reply = main_client.resolve_contract(
        api.ContractSpec(
            sec_type=api.SEC_TYPE_CASH,
            symbol="USD",
            exchange="IDEALPRO",
            currency="JPY",
        )
    )
    assert reply.status == api.RESOLVE_STATUS_FOUND
    assert reply.contract.sec_type == api.SEC_TYPE_CASH
    assert reply.contract.con_id > 0
    request = api.BarsRequest(
        contract=reply.contract,
        kind=api.BAR_KIND_MIDPOINT,
        duration_days=BAR_DAYS,
        end_day="",
    )
    bars = main_client.bars(request).bars
    # zero bars is a failure: fxrefresh counts an empty answer as success
    assert len(bars) >= 1
    for b in bars:
        low, high = Decimal(b.low), Decimal(b.high)
        assert low <= Decimal(b.close) <= high
        assert Decimal(b.close) > 0
