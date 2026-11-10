"""ticker_fields drops IB's no-value markers: NaN, UNSET_DOUBLE and, for fields
that cannot be negative, any negative value (IB sends -1 for "no quote")."""

from __future__ import annotations

import math
import sys
from typing import Any

from ib_async import Ticker
from ib_async.objects import OptionComputation

from optdesk.ibkr import spec
from optdesk.ibkr.ibgateway import ticker_fields


def make(**values: Any) -> Ticker:
    ticker = Ticker()
    for name, value in values.items():
        setattr(ticker, name, value)
    return ticker


def test_minus_one_price_is_absent() -> None:
    found = ticker_fields(make(bid=-1.0, ask=-1.0, last=7.5, close=7.0))
    assert "bid" not in found
    assert "ask" not in found
    assert found["last"] == 7.5
    assert found["close"] == 7.0


def test_minus_one_size_is_absent() -> None:
    found = ticker_fields(make(bidSize=-1.0, askSize=-1.0, lastSize=-1.0, volume=-1.0))
    assert found == {}


def test_negative_greek_is_kept() -> None:
    greeks = OptionComputation(
        0, impliedVol=0.2, delta=-0.4, gamma=0.01, vega=0.1, theta=-0.05, undPrice=-1.0
    )
    found = ticker_fields(make(modelGreeks=greeks))
    assert found["delta"] == -0.4
    assert found["theta"] == -0.05
    assert "und_price" not in found


def test_zero_size_is_kept() -> None:
    found = ticker_fields(make(bidSize=0.0, callOpenInterest=0.0))
    assert found["bid_size"] == 0.0
    assert "open_interest" not in found  # a lone 0.0 is a placeholder (#1183)


def test_unset_double_is_absent() -> None:
    greeks = OptionComputation(
        0, impliedVol=sys.float_info.max, delta=sys.float_info.max
    )
    found = ticker_fields(make(bid=sys.float_info.max, modelGreeks=greeks))
    assert found == {}


def test_sentinel_snapshot_partial() -> None:
    row = spec.kind_spec(spec.SNAPSHOT)
    held = make(bid=-1.0, ask=-1.0, last=7.5, close=7.0)
    assert spec.quote_status(row, ticker_fields(held)) == spec.STATUS_PARTIAL
    empty = make(bid=-1.0, ask=-1.0, last=math.nan, close=math.nan)
    assert spec.quote_status(row, ticker_fields(empty)) == spec.STATUS_NO_DATA
