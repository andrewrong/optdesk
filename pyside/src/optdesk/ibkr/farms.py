"""The farm board: the data farms a gateway named, from its 2100 to 2199 messages.

Implements api.FarmBoard. Codes 2103, 2105 and 2157 mark a farm down, 2104, 2106
and 2158 mark it up; every other code of 2100 to 2199 changes nothing. A farm down
for FARM_GRACE_S (or named broken by the history probe) makes the state
`farm_broken`. Times come from the injected Clock. Log lines carry the configured
gateway name only.
"""

from __future__ import annotations

import logging
import re

from optdesk.ibkr.health_constants import FARM_GRACE_S
from optdesk.ibkr.model import Farm
from optdesk.ibkr.timing import Clock

_LOG = logging.getLogger("optdesk.ibkr.farms")

DOWN_CODES = frozenset({2103, 2105, 2157})
UP_CODES = frozenset({2104, 2106, 2158})
OTHER = "other"
_WORD = re.compile(r"[a-z0-9.]{1,16}")


def farm_word(text: str) -> str:
    """Return the word after the last colon of text when it is a farm name, else `other`."""
    word = text.rsplit(":", 1)[-1].strip() if ":" in text else ""
    return word if _WORD.fullmatch(word) else OTHER


class Farms:
    """Keeps the farms of one gateway; one instance per gateway."""

    def __init__(self, name: str, clock: Clock) -> None:
        self.name = name
        self._clock = clock
        self._farms: dict[str, Farm] = {}
        self._probe: str | None = None  # a farm the history probe named broken
        self._broken: str | None = None  # the farm last logged as farm_broken

    def reset(self) -> None:
        """Forget the farms, the history-probe mark and the logged broken name (a new session)."""
        self._farms.clear()
        self._probe = None
        self._broken = None

    def now_ms(self) -> int:
        """Return the unix milliseconds of the injected clock."""
        return int(self._clock.now().timestamp() * 1000)

    def record(self, code: int, text: str, now: int) -> None:
        """Note one message of 2100 to 2199; codes outside the six change no farm."""
        if code not in DOWN_CODES and code not in UP_CODES:
            return
        farm = farm_word(text)
        up = code in UP_CODES
        if up and farm == self._probe:
            self._probe = None  # an ok code clears what the history probe set
        known = self._farms.get(farm)
        if known is not None and known.up == up:
            self._note()
            return
        self._farms[farm] = Farm(farm, up, now)
        _LOG.info("farm_code gateway=%s code=%d farm=%s", self.name, code, farm)
        self._note()

    def snapshot(self) -> tuple[Farm, ...]:
        """Return the farms seen so far, by name."""
        return tuple(self._farms[k] for k in sorted(self._farms))

    def probe_broken(self, farm: str) -> None:
        """The history probe failed for farm: it is down and the state is farm_broken."""
        known = self._farms.get(farm)
        if known is None or known.up:
            self._farms[farm] = Farm(farm, False, self.now_ms())
        self._probe = farm
        self._note()

    def probe_ok(self, farm: str) -> None:
        """A history answer came: the probe's mark is cleared and the farm is up."""
        self._probe = None
        known = self._farms.get(farm)
        if known is not None and not known.up:
            self._farms[farm] = Farm(farm, True, self.now_ms())
        self._note()

    def state(self, connected: bool, soft_lost: bool) -> str:
        """Return up, soft_lost, down or farm_broken for a session with these flags."""
        broken = self._note()
        if not connected:
            return "soft_lost" if soft_lost else "down"
        return "farm_broken" if broken else "up"

    def _broken_farm(self) -> str | None:
        if self._probe is not None:
            return self._probe
        now = self.now_ms()
        for farm in self.snapshot():
            if not farm.up and now - farm.since >= FARM_GRACE_S * 1000:
                return farm.name
        return None

    def _note(self) -> str | None:
        broken = self._broken_farm()
        if broken != self._broken:
            if broken is None:
                _LOG.info("farm_ok gateway=%s farm=%s", self.name, self._broken)
            else:
                _LOG.info("farm_broken gateway=%s farm=%s", self.name, broken)
            self._broken = broken
        return broken
