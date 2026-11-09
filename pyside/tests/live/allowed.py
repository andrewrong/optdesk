"""The closed set of rpc names the live suite may call. Nothing else is reachable."""

from __future__ import annotations

ALLOWED_RPCS: frozenset[str] = frozenset(
    {
        "Ping",
        "Account",
        "Executions",
        "ResolveContract",
        "ListChains",
        "Snapshot",
        "Dividends",
    }
)

# Stub calls pinned to one test each: Bars is a planned stub, so exactly one test
# asserts that it is still UNIMPLEMENTED and fails the day it lands.
PINNED_STUBS: frozenset[str] = frozenset({"Bars"})
