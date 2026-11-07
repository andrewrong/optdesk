"""The hand-written SessionStatus mirrors the generated PingResponse."""

from __future__ import annotations

import dataclasses

from google.protobuf.descriptor import FieldDescriptor

from optdesk.contracts import api as pb
from optdesk.ibkr.api import SessionStatus

# Fields of PingResponse that the by-hand probe does not carry.
UNMIRRORED = {"generation", "market_data_type"}


def test_session_status_matches_ping_response() -> None:
    proto = {field.name for field in pb.PingResponse.DESCRIPTOR.fields}
    mine = {field.name for field in dataclasses.fields(SessionStatus)}
    assert mine == proto - UNMIRRORED
    proto_types = {f.name: f.type for f in pb.PingResponse.DESCRIPTOR.fields}
    for field in dataclasses.fields(SessionStatus):
        assert field.type in ("bool", bool)
        assert proto_types[field.name] == FieldDescriptor.TYPE_BOOL
    assert UNMIRRORED <= proto
