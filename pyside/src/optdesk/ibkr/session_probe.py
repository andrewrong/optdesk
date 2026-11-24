"""The clock probe, the state log and the probed gateway of the ibkr module.

Every PROBE_EVERY_S the session asks the gateway for its time under PROBE_WAIT_S;
PROBE_FAILS failures in a row close the session and the supervisor opens a new
one. Each gateway has its own count. `main` also runs the farm probe; `wife` the
clock probe only. All waits go through the injected Clock.
"""

from __future__ import annotations

import asyncio
import logging
from collections.abc import Awaitable, Callable
from typing import Any, Protocol, runtime_checkable

from optdesk.ibkr.farm_probe import PROBE_REQUEST, FarmProbe
from optdesk.ibkr.farms import Farms
from optdesk.ibkr.health_constants import (
    PROBE_EVERY_S,
    PROBE_FAILS,
    PROBE_WAIT_S,
)
from optdesk.ibkr.lister import ListingGateway
from optdesk.ibkr.model import SessionState
from optdesk.ibkr.timing import Clock, with_timeout


class StateLog:
    """One log line per state change of one gateway: session_down, session_up, soft_loss, restore.

    The name is the configured one; no account id and no port. The type of the
    last failed attempt is a field of the next session_up, not a line of its own.
    """

    def __init__(self, logger: logging.Logger, name: str) -> None:
        self._log = logger
        self._name = name
        self._down = False
        self._soft = False
        self._reason = ""

    def _line(self, event: str, fields: str = "") -> None:
        self._log.info("%s gateway=%s%s", event, self._name, fields)

    def failed(self, exc: BaseException) -> None:
        """An attempt failed: log session_down once, keep the reason for session_up."""
        self._reason = type(exc).__name__
        self.down()

    def down(self) -> None:
        """The session is down; a line only on the change."""
        if not self._down:
            self._down = True
            self._soft = False
            self._line("session_down")

    def up(self, generation: int) -> None:
        """The session is up; a line carries the last failure type, once."""
        extra = f" last_failure={self._reason}" if self._reason else ""
        self._down = self._soft = False
        self._reason = ""
        self._line("session_up", f" generation={generation}{extra}")

    def soft_loss(self) -> None:
        """A 1100 or 2110 was seen."""
        if not self._soft:
            self._soft = True
            self._line("soft_loss")

    def restore(self) -> None:
        """A 1101 or 1102 restored the session."""
        if self._soft:
            self._soft = False
            self._line("restore")


class ClockProbe:
    """Counts consecutive failed clock probes of one session."""

    def __init__(
        self,
        clock: Clock,
        ask: Callable[[], Awaitable[object]],
        active: Callable[[], Awaitable[bool]],
    ) -> None:
        self._clock = clock
        self._ask = ask
        self._active = active
        self.fails = 0

    @property
    def good(self) -> bool:
        """True when the last probe was good (or none has failed)."""
        return self.fails == 0

    async def step(self) -> bool:
        """Run one probe; True when the failures reached PROBE_FAILS (close the session)."""
        if not await self._active():
            return False  # a lost session is the supervisor's, not a failed probe
        try:
            await with_timeout(self._clock, self._ask(), PROBE_WAIT_S)
        except Exception:  # noqa: BLE001 - any failed probe is one failure
            self.fails += 1
            return self.fails >= PROBE_FAILS
        self.fails = 0
        return False

    async def run(self) -> None:
        """Probe every PROBE_EVERY_S; return after the session was told to close."""
        while True:
            await self._clock.sleep(PROBE_EVERY_S)
            if await self.step():
                return


@runtime_checkable
class Probed(Protocol):
    """A gateway the watch can probe and close."""

    farms: Farms

    async def status(self) -> SessionState: ...

    async def probe_time(self) -> object: ...

    async def probe_bars(self) -> object: ...

    def close_session(self) -> None: ...


class ProbedGateway(ListingGateway):
    """ListingGateway that keeps a farm board and can be probed."""

    def __init__(self, *args: Any, name: str = "", **kwargs: Any) -> None:
        super().__init__(*args, **kwargs)
        self._alias = name
        self.farms = Farms(name, self._clock)

    def _on_error(self, req_id: int, code: int, text: str, contract: Any) -> None:
        if 2100 <= code < 2200:
            self.farms.record(code, text, self.farms.now_ms())
        super()._on_error(req_id, code, text, contract)

    async def status(self) -> SessionState:
        base = await super().status()
        return base._replace(
            state=self.farms.state(base.connected, base.soft_lost),
            farms=self.farms.snapshot(),
        )

    async def probe_time(self) -> object:
        """Ask the gateway for its current time (no data farm)."""
        if self._ib is None:
            raise ConnectionError("the gateway session is not connected")
        return await self._ib.reqCurrentTimeAsync()

    async def probe_bars(self) -> object:
        """Ask for one daily bar of the probe contract."""
        return await self.bars(PROBE_REQUEST)

    def close_session(self) -> None:
        """Close the socket; the supervisor then opens a new session."""
        session = self._ib
        if session is not None:
            session.disconnect()
            self._on_end(session)


def watch_for(
    gateway: object, clock: Clock, *, history: bool
) -> Callable[[], Awaitable[None]] | None:
    """Return the probes of one session of gateway, or None when it cannot be probed.

    The clock probe always runs; the farm probe only when history is true. The
    returned coroutine ends after it closed the session, and is cancelled by the
    supervisor when the session ends first.
    """
    if not isinstance(gateway, Probed):
        return None
    probed: Probed = gateway

    async def active() -> bool:
        return (await probed.status()).connected

    async def watch() -> None:
        probe = ClockProbe(clock, probed.probe_time, active)
        side: list[asyncio.Future[None]] = []
        if history:
            side.append(
                asyncio.ensure_future(
                    FarmProbe(
                        clock, probed.farms, lambda: probe.good, probed.probe_bars
                    ).run()
                )
            )
        try:
            await probe.run()
            probed.close_session()
        finally:
            for task in side:
                task.cancel()
            await asyncio.gather(*side, return_exceptions=True)

    return watch
