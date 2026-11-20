"""Health parts of the transport: the Ping payload and the retryable statuses.

transport.py calls into this file so that it stays under its line ceiling.
The state words are those of model.SessionState; the numbers are those of the
SessionState enum of session.proto, read from the generated descriptor so no
generated name is imported outside optdesk.contracts.api.
"""

from __future__ import annotations

import grpc

from optdesk.contracts import api as pb
from optdesk.ibkr.api import HistoryUnavailable
from optdesk.ibkr.model import SessionState

HISTORY_UNAVAILABLE_TEXT = "history_unavailable"
SESSION_RECONNECTING_TEXT = "session_reconnecting"

_STATE_ENUM = pb.PingResponse.DESCRIPTOR.fields_by_name["state"].enum_type


def _state_value(word: str) -> int:
    assert _STATE_ENUM is not None
    value = _STATE_ENUM.values_by_name.get(f"SESSION_STATE_{word.upper()}")
    return 0 if value is None else value.number


def retryable_status(exc: BaseException) -> tuple[grpc.StatusCode, str] | None:
    """Return UNAVAILABLE with a fixed word for a history stall or a lost session, else None."""
    if isinstance(exc, HistoryUnavailable):
        return grpc.StatusCode.UNAVAILABLE, HISTORY_UNAVAILABLE_TEXT
    if isinstance(exc, ConnectionError | OSError):
        return grpc.StatusCode.UNAVAILABLE, SESSION_RECONNECTING_TEXT
    return None


def ping_response(
    state: SessionState, data_type: pb.MarketDataType.ValueType
) -> pb.PingResponse:
    """Build the Ping answer: the old fields, the state and every farm of the board."""
    response = pb.PingResponse(
        connected=state.connected,
        generation=state.generation,
        market_data_type=data_type,
        state=_state_value(state.state),  # type: ignore[arg-type]
    )
    for farm in state.farms:
        response.farms.add(name=farm.name, up=farm.up, since_unix_ms=farm.since)
    return response
