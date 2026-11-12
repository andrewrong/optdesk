"""ListContracts of ContractsService: validation, the gateway call and the answer.

transport.py delegates here, so the file of the guard and the servicers stays small.
Generated types are imported through optdesk.contracts.api only.
"""

from __future__ import annotations

import re
from collections.abc import Sequence

from optdesk.contracts import api as pb
from optdesk.ibkr.model import ListedContract, ListerGateway

_RIGHT_WORDS = {pb.RIGHT_CALL: "C", pb.RIGHT_PUT: "P", pb.RIGHT_UNSPECIFIED: ""}
_RIGHT_VALUES = {"C": pb.RIGHT_CALL, "P": pb.RIGHT_PUT}
_UNDERLYING_TYPES = (pb.SEC_TYPE_STOCK, pb.SEC_TYPE_INDEX)
_DAY = re.compile(r"[0-9]{8}")

BAD_REQUEST = (
    "underlying needs a stock or index type and a symbol; expiry is YYYYMMDD; "
    "trading_class is required; right is call, put or unspecified"
)


def problem(request: pb.ListContractsRequest) -> str:
    """Return why request cannot be served, or the empty string."""
    under = request.underlying
    if (
        under.sec_type not in _UNDERLYING_TYPES
        or not under.symbol
        or not _DAY.fullmatch(request.expiry)
        or not request.trading_class
        or request.right not in _RIGHT_WORDS
    ):
        return BAD_REQUEST
    return ""


async def answer(
    gateway: ListerGateway, request: pb.ListContractsRequest
) -> Sequence[ListedContract]:
    """Ask the gateway for the contracts of a request that passed problem()."""
    under = request.underlying
    return await gateway.list_contracts(
        symbol=under.symbol,
        exchange=under.exchange,
        currency=under.currency,
        expiry=request.expiry,
        right=_RIGHT_WORDS[request.right],
        trading_class=request.trading_class,
    )


def to_response(rows: Sequence[ListedContract]) -> pb.ListContractsResponse:
    """Convert the listed contracts to the response, in the order given."""
    return pb.ListContractsResponse(
        contracts=[
            pb.ContractSpec(
                sec_type=pb.SEC_TYPE_OPTION,
                symbol=row.symbol,
                exchange=row.exchange,
                currency=row.currency,
                expiry=row.expiry,
                strike=row.strike,
                right=_RIGHT_VALUES.get(row.right, pb.RIGHT_UNSPECIFIED),
                multiplier=row.multiplier,
                trading_class=row.trading_class,
                con_id=row.con_id,
            )
            for row in rows
        ]
    )
