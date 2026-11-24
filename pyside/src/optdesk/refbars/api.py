"""Interface of optdesk.refbars: declarations only, standard library only.

The command `python -m optdesk.refbars --deadline SECONDS` reads one JSON `Request` on
stdin and writes one JSON `Answer` on stdout; it exits 0 whenever it wrote the answer.
Rules every producer keeps:

1. Days are `YYYY-MM-DD`, inclusive; at most 10 pairs per request.
2. `results` has one entry per requested pair, in the order of the request.
3. An empty `error_code` is ok; the others are `rate_limited`, `no_data`, `network`,
   `timeout` and `source_changed`. A pair with an error code may keep the bars it has.
4. `open`, `high`, `low`, `close` are decimal text of 4 places, never a float, and `volume` is
   decimal text of 4 places too; each is the empty string when the source gave none. Bars are
   ordered by `session_day`.
"""

from typing import NotRequired, Protocol, TypedDict, runtime_checkable

ERROR_CODES: tuple[str, ...] = ("rate_limited", "no_data", "network", "timeout", "source_changed")


class PairSpec(TypedDict):
    """One instrument asked for."""

    symbol: str
    currency: str


class Request(TypedDict):
    """The JSON on stdin."""

    pairs: list[PairSpec]
    from_day: str
    to_day: str
    with_actions: NotRequired[bool]
    adjusted: NotRequired[bool]


class Bar(TypedDict):
    """One daily bar: the session day and the unadjusted prices and volume as decimal text.

    `adj_close` (decimal text) is present only when the request set `adjusted`.
    """

    session_day: str
    open: str
    high: str
    low: str
    close: str
    volume: str
    adj_close: NotRequired[str]


class Action(TypedDict):
    """One dividend or split: the day and the value as decimal text."""

    day: str
    value: str


class PairResult(TypedDict):
    """The answer for one pair; `error_code` is empty when ok."""

    symbol: str
    currency: str
    error_code: str
    bars: list[Bar]
    dividends: NotRequired[list[Action]]
    splits: NotRequired[list[Action]]


class Answer(TypedDict):
    """The JSON on stdout."""

    results: list[PairResult]


@runtime_checkable
class Fetcher(Protocol):
    """Reads the bars of one pair from the outside source; never retries."""

    def __call__(self, pair: PairSpec, from_day: str, to_day: str) -> PairResult:
        """Return the result of the pair; a failure is an `error_code`, not an exception."""
        ...


@runtime_checkable
class Clock(Protocol):
    """Time of the command, injected so that no test sleeps."""

    def monotonic(self) -> float:
        """Return seconds from an arbitrary start, never going back."""
        ...

    def sleep(self, seconds: float) -> None:
        """Wait for the seconds."""
        ...
