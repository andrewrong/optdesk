"""Constants of the gateway health design (Decision 8), trial values the owner may change.

The one place that defines them. PROBE_WAIT_S is not PROBE_DEADLINE_S of health.py,
which bounds the local /health probe.
"""

from __future__ import annotations

PROBE_EVERY_S = 60.0  # clock probe, per gateway
PROBE_FAILS = 3  # consecutive failed clock probes that close the session
HISTORY_WAIT_S = 60.0  # one history request waits this long, then HistoryUnavailable
FARM_PROBE_EVERY_S = 300.0  # history probe, main only
FARM_PROBE_FAILS = 3  # consecutive failed history probes that set farm_broken
PROBE_WAIT_S = 10.0  # one probe request waits this long
FARM_GRACE_S = 600.0  # a farm down for less than this raises no farm_broken
