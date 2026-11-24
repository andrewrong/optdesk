"""A resolve answers IB's own contract; a contract that differs from the request is refused."""

from __future__ import annotations

import asyncio
from typing import Any, ClassVar

import grpc
import pytest
from ib_async import Contract, ContractDetails
from kit.fakeclock import FakeClock
from test_ibgateway import _spec, opened
from test_transport import FakeGateway, serving

from optdesk.contracts import api as pb
from optdesk.ibkr import spec
from optdesk.ibkr.model import ResolveResult

ASKED = {
    "sec_type": "OPT",
    "symbol": "SPY",
    "exchange": "SMART",
    "currency": "USD",
    "expiry": "20261120",
    "strike": "100",
    "right": "C",
    "multiplier": "100",
    "trading_class": "SPY",
}
IB: dict[str, Any] = {
    "conId": 99,
    "exchange": "CBOE",
    "lastTradeDateOrContractMonth": "20261120",
    "strike": 100.0,
    "right": "C",
    "tradingClass": "SPY",
    "multiplier": "100",
}


def _resolve(request: dict[str, str], **ib: Any) -> Any:
    async def run() -> Any:
        gateway, fake = await opened(FakeClock())

        async def details(contract: Contract) -> list[ContractDetails]:
            return [ContractDetails(contract=Contract(**{**IB, **ib}))]

        fake.details = details
        return await gateway.resolve(**request)

    return asyncio.run(run())


class TestResolveMismatchRefused:
    @pytest.mark.parametrize(
        ("ib", "field"),
        [
            ({"strike": 105.0}, "strike 100->105"),
            ({"right": "P"}, "right C->P"),
            (
                {"lastTradeDateOrContractMonth": "20261127"},
                "expiry 20261120->20261127",
            ),
            ({"tradingClass": "SPYW"}, "trading_class SPY->SPYW"),
            ({"multiplier": "10"}, "multiplier 100->10"),
        ],
    )
    def test_refused_with_field_named(self, ib: dict[str, Any], field: str) -> None:
        with pytest.raises(spec.AmbiguousResolve) as err:
            _resolve(ASKED, **ib)
        assert field in str(err.value)

    def test_grpc_failed_precondition_without_contract(self) -> None:
        async def run() -> grpc.RpcError:
            gateway = FakeGateway(resolved=spec.AmbiguousResolve("strike 100->105"))
            async with serving(gateway) as channel:
                request = pb.ResolveContractRequest(
                    contract=pb.ContractSpec(
                        sec_type=pb.SEC_TYPE_OPTION,
                        symbol="SPY",
                        expiry="20261120",
                        strike="100",
                        right=pb.RIGHT_CALL,
                    )
                )
                with pytest.raises(grpc.aio.AioRpcError) as err:
                    await pb.ContractsServiceStub(channel).ResolveContract(request)
                return err.value

        err: Any = asyncio.run(run())
        assert err.code() == grpc.StatusCode.FAILED_PRECONDITION
        assert "strike 100->105" in err.details()

    def test_grpc_text_names_every_field_when_all_differ(self) -> None:
        with pytest.raises(spec.AmbiguousResolve) as raised:
            _resolve(
                ASKED,
                strike=105.0,
                right="P",
                lastTradeDateOrContractMonth="20261127",
                tradingClass="SPYW",
                multiplier="10",
            )

        async def run() -> Any:
            async with serving(FakeGateway(resolved=raised.value)) as channel:
                req = pb.ResolveContractRequest(
                    contract=pb.ContractSpec(
                        sec_type=pb.SEC_TYPE_OPTION,
                        symbol="SPY",
                        expiry="20261120",
                        strike="100",
                        right=pb.RIGHT_CALL,
                    )
                )
                with pytest.raises(grpc.aio.AioRpcError) as err:
                    await pb.ContractsServiceStub(channel).ResolveContract(req)
                return err.value

        err = asyncio.run(run())
        assert err.code() == grpc.StatusCode.FAILED_PRECONDITION
        for name in ("expiry", "strike", "right", "trading_class", "multiplier"):
            assert name in err.details()

    def test_not_cached(self) -> None:
        async def run() -> int:
            gateway, fake = await opened(FakeClock())
            calls = 0

            async def details(contract: Contract) -> list[ContractDetails]:
                nonlocal calls
                calls += 1
                return [ContractDetails(contract=Contract(**{**IB, "strike": 105.0}))]

            fake.details = details
            for _ in range(2):
                with pytest.raises(spec.AmbiguousResolve):
                    await gateway.resolve(**ASKED)
            return calls

        assert asyncio.run(run()) == 2


class TestResolveAnswersIbFields:
    def test_gateway_holds_ib_values(self) -> None:
        got = _resolve(
            {**ASKED, "exchange": "SMART"},
            exchange="CBOE",
            tradingClass="SPY",
            strike=100.0,
        )
        assert got.con_id == 99 and got.strike == "100" and got.right == "C"
        assert got.expiry == "20261120" and got.exchange == "CBOE"
        assert got.trading_class == "SPY" and got.multiplier == "100"

    def test_answer_carries_ib_fields_over_the_request(self) -> None:
        async def run() -> pb.ResolveContractResponse:
            gateway = FakeGateway(
                resolved=ResolveResult(
                    True,
                    99,
                    "CBOE",
                    "SPYW",
                    "100",
                    expiry="20261120",
                    strike="100",
                    right="P",
                )
            )
            async with serving(gateway) as channel:
                request = pb.ResolveContractRequest(
                    contract=pb.ContractSpec(
                        sec_type=pb.SEC_TYPE_OPTION,
                        symbol="SPY",
                        expiry="20261120",
                        strike="100.0",
                        right=pb.RIGHT_CALL,
                    )
                )
                return await pb.ContractsServiceStub(channel).ResolveContract(request)

        c = asyncio.run(run()).contract
        assert (c.con_id, c.strike, c.right, c.expiry) == (
            99,
            "100",
            pb.RIGHT_PUT,
            "20261120",
        )
        assert (c.exchange, c.trading_class, c.multiplier) == ("CBOE", "SPYW", "100")


class TestResolveEmptyRequestFieldNotCompared:
    def test_stock_request(self) -> None:
        got = _resolve(_spec(), lastTradeDateOrContractMonth="", strike=0.0, right="")
        assert got.found and got.con_id == 99 and got.strike == ""

    def test_option_without_class_or_multiplier(self) -> None:
        got = _resolve(
            {**ASKED, "trading_class": "", "multiplier": ""},
            tradingClass="SPYW",
            multiplier="10",
        )
        assert got.trading_class == "SPYW" and got.multiplier == "10"


class TestResolveDateAndRightForms:
    @pytest.mark.parametrize("asked", ["20261211", "2026-12-11"])
    @pytest.mark.parametrize(
        "got", ["20261211", "20261211 15:00:00 Asia/Tokyo", "20261211 15:00 JST"]
    )
    def test_expiry_compared_by_date_only(self, asked: str, got: str) -> None:
        found = _resolve({**ASKED, "expiry": asked}, lastTradeDateOrContractMonth=got)
        assert found.found

    def test_other_date_with_time_is_refused(self) -> None:
        with pytest.raises(spec.AmbiguousResolve) as err:
            _resolve(
                {**ASKED, "expiry": "20261211"},
                lastTradeDateOrContractMonth="20261218 15:00:00 Asia/Tokyo",
            )
        assert "expiry" in str(err.value)

    @pytest.mark.parametrize(
        ("asked", "got"), [("CALL", "C"), ("C", "CALL"), ("PUT", "P"), ("p", "PUT")]
    )
    def test_right_normalised_on_both_sides(self, asked: str, got: str) -> None:
        assert _resolve({**ASKED, "right": asked}, right=got).found

    def test_call_against_put_is_refused(self) -> None:
        with pytest.raises(spec.AmbiguousResolve):
            _resolve({**ASKED, "right": "CALL"}, right="PUT")


class TestResolveJpOptionPasses:
    def test_found(self) -> None:
        request = {
            **ASKED,
            "symbol": "NK225",
            "exchange": "OSE.JPN",
            "currency": "JPY",
            "expiry": "20261211",
            "strike": "40000",
            "multiplier": "1000",
            "trading_class": "NK225",
        }
        got = _resolve(
            request,
            exchange="OSE.JPN",
            lastTradeDateOrContractMonth="20261211",
            strike=40000.0,
            multiplier="1000",
            tradingClass="NK225",
        )
        assert got.found and got.exchange == "OSE.JPN" and got.strike == "40000"
        assert got.multiplier == "1000"


class TestResolveGatewayShapedAnswer:
    REQUEST: ClassVar[dict[str, str]] = {
        **ASKED,
        "expiry": "2026-12-11",
        "strike": "720.00",
    }
    SHAPED: ClassVar[dict[str, Any]] = {
        "lastTradeDateOrContractMonth": "20261211",
        "multiplier": "100",
        "right": "C",
        "strike": 720.0,
    }

    def test_found(self) -> None:
        assert _resolve(self.REQUEST, **self.SHAPED).found

    def test_other_strike_refused(self) -> None:
        with pytest.raises(spec.AmbiguousResolve) as err:
            _resolve(self.REQUEST, **{**self.SHAPED, "strike": 690.0})
        assert "strike 720.00->690" in str(err.value)
