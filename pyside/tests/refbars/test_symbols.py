"""The Yahoo symbol and the session zone of a pair."""

from __future__ import annotations

from optdesk.refbars.symbols import map_pair


def test_us_dot_class_share_gets_dash() -> None:
    target = map_pair("BRK.B", "USD")
    assert target is not None
    assert target.yahoo_symbol == "BRK-B"
    assert str(target.zone) == "America/New_York"
    plain = map_pair("AAPL", "USD")
    assert plain is not None
    assert plain.yahoo_symbol == "AAPL"


def test_tokyo_gets_dot_t() -> None:
    target = map_pair("7203", "JPY")
    assert target is not None
    assert target.yahoo_symbol == "7203.T"
    assert str(target.zone) == "Asia/Tokyo"


def test_other_currency_is_no_data() -> None:
    assert map_pair("SAP", "EUR") is None
