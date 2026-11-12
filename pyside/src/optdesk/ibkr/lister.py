"""ListingGateway: the real option contracts of one expiry and class, one request.

IbGateway is not edited; this subclass adds list_contracts through the details
lane. The lane's width (2 to 5) would let list calls overlap, so one lock lets
one list call be in flight at a time. The wait for the lock is outside the lane's
timeout. The lock is taken with acquire and released in a finally that wraps the
whole lane call: the lane may raise TimeoutError before it sends anything (no
start allowed), and a caller may be cancelled while it waits in the lane's
admit; a lock released only by the request would leak then.
"""

from __future__ import annotations

import asyncio
from collections.abc import Sequence
from typing import Any

from ib_async import Contract, RequestError

from optdesk.ibkr import spec
from optdesk.ibkr.ibgateway import IbGateway, _decimal_text
from optdesk.ibkr.model import ListedContract


class ListingGateway(IbGateway):
    """An IbGateway that also serves list_contracts (the ListerGateway protocol)."""

    def __init__(self, *args: Any, **kwargs: Any) -> None:
        super().__init__(*args, **kwargs)
        self._list_lock = asyncio.Lock()

    async def list_contracts(
        self,
        *,
        symbol: str,
        exchange: str,
        currency: str,
        expiry: str,
        right: str,
        trading_class: str,
    ) -> Sequence[ListedContract]:
        ib, generation = self._session()
        contract = Contract(
            secType="OPT",
            symbol=symbol,
            exchange=exchange or "SMART",
            currency=currency or "USD",
            lastTradeDateOrContractMonth=expiry,
            right=right,
            tradingClass=trading_class,
        )
        await self._list_lock.acquire()
        try:
            details = await self._send_details(
                generation, lambda: ib.reqContractDetailsAsync(contract)
            )
        except RequestError as exc:
            if spec.classify_error(exc.code).not_listed:
                return []
            raise spec.BrokerError(exc.code, exc.message) from exc
        finally:
            self._list_lock.release()
        return [_listed(d.contract) for d in details]


def _listed(found: Any) -> ListedContract:
    return ListedContract(
        con_id=int(found.conId),
        symbol=found.symbol,
        exchange=found.exchange,
        currency=found.currency,
        expiry=found.lastTradeDateOrContractMonth,
        strike=_decimal_text(float(found.strike)),
        right=found.right,
        multiplier=found.multiplier,
        trading_class=found.tradingClass,
    )
