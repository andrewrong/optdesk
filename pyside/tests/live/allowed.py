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
        "Bars",
    }
)
