"""The OPTION_OPEN row of spec.py: release, timeout and grace."""

from __future__ import annotations

from optdesk.ibkr.spec import OPTION_OPEN, is_complete, kind_spec, quote_status

MODEL = {
    "implied_vol": 0.3,
    "delta": 0.5,
    "gamma": 0.1,
    "vega": 0.2,
    "theta": -0.1,
    "und_price": 100.0,
}


def test_kind_value_and_ticks() -> None:
    assert OPTION_OPEN == "option_open"
    row = kind_spec(OPTION_OPEN)
    assert row.generic_ticks == "100,101"
    assert row.timeout_s == 15


def test_released_on_bid_ask_and_model_without_close_or_open_interest() -> None:
    row = kind_spec(OPTION_OPEN)
    got = {"bid": 1.0, "ask": 1.1, **MODEL}
    assert is_complete(row, got)
    assert quote_status(row, got) == "ok"
    assert not is_complete(row, {"bid": 1.0, "ask": 1.1, "close": 1.0})
    assert not is_complete(row, {"bid": 1.0, **MODEL})


def test_no_grace() -> None:
    row = kind_spec(OPTION_OPEN)
    assert row.grace_fields == frozenset()
    assert row.grace_s == 0.0


def test_incomplete_is_partial_or_no_data() -> None:
    row = kind_spec(OPTION_OPEN)
    assert quote_status(row, {"bid": 1.0, "ask": 1.1}) == "partial"
    assert quote_status(row, {}) == "no_data"
