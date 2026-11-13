"""missing_fields says why each field of an option quote is missing."""

from __future__ import annotations

import math

from ib_async import Ticker
from ib_async.util import UNSET_DOUBLE

from optdesk.ibkr import spec
from optdesk.ibkr.missing import missing_fields

EOD = spec.TABLE[spec.OPTION_EOD]
RECEIVED_RELEASE, GRACE, INVALID, ERROR, ZERO, DATA_TYPE = 1, 2, 3, 4, 5, 6


def reasons(rows: tuple[tuple[str, int, int], ...]) -> dict[str, tuple[int, int]]:
    return {field: (reason, code) for field, reason, code in rows}


def ticker(**values: float) -> Ticker:
    result = Ticker()
    result.marketDataType = 1
    for name, value in values.items():
        setattr(result, name, value)
    return result


def test_missing_received_zero_open_interest() -> None:
    both = ticker(callOpenInterest=0.0, putOpenInterest=0.0)
    one = ticker(callOpenInterest=0.0, putOpenInterest=math.nan)
    assert reasons(missing_fields(EOD, both, {}, 0, False))["open_interest"][0] == ZERO
    got = reasons(missing_fields(EOD, one, {}, 0, False))
    assert got["open_interest"][0] == RECEIVED_RELEASE


def test_missing_market_data_type_delayed() -> None:
    delayed = ticker()
    delayed.marketDataType = 3
    assert reasons(missing_fields(EOD, delayed, {"bid": 1.0}, 0, False)) == {
        f: (DATA_TYPE, 0)
        for f in (
            "ask",
            "last",
            "volume",
            "open_interest",
            "implied_vol",
            "delta",
            "gamma",
            "vega",
            "theta",
        )
    }
    live = reasons(missing_fields(EOD, ticker(), {"bid": 1.0}, 0, False))
    assert live["ask"] == (RECEIVED_RELEASE, 0)


def test_missing_grace_ended_open_interest() -> None:
    got = {"bid": 1.0}
    after = reasons(missing_fields(EOD, ticker(), got, 0, True))
    before = reasons(missing_fields(EOD, ticker(), got, 0, False))
    assert after["open_interest"] == (GRACE, 0)
    assert after["ask"] == (RECEIVED_RELEASE, 0)
    assert before["open_interest"] == (RECEIVED_RELEASE, 0)


def test_missing_invalid_negative_volume() -> None:
    got = reasons(
        missing_fields(
            EOD,
            ticker(volume=-1.0, ask=UNSET_DOUBLE, last=math.nan),
            {},
            0,
            False,
        )
    )
    assert got["volume"] == (INVALID, 0)
    assert got["ask"] == (RECEIVED_RELEASE, 0)
    assert got["last"] == (RECEIVED_RELEASE, 0)


def test_missing_error_code() -> None:
    got = reasons(missing_fields(EOD, ticker(), {"bid": 1.0}, 354, True))
    assert "bid" not in got
    assert set(got.values()) == {(ERROR, 354)}


def test_missing_other_kinds_empty() -> None:
    for kind in (spec.UNDERLYING_IV, spec.SNAPSHOT):
        assert missing_fields(spec.TABLE[kind], ticker(), {}, 354, True) == ()
