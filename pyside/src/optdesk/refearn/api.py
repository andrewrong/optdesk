"""Interface of optdesk.refearn: declarations only, standard library only.

The command `python -m optdesk.refearn --deadline SECONDS` reads one JSON `Request` on
stdin and writes one JSON `Answer` on stdout; it exits 0 whenever it wrote the answer.
Rules every producer keeps:

1. `from_day` is `YYYY-MM-DD`; at most 10 symbols per request.
2. `results` has one entry per requested symbol, in the order of the request.
3. An empty `error_code` is ok; the others are `rate_limited`, `no_data`, `network`,
   `timeout` and `source_changed` (the same as `optdesk.refbars.api`).
4. `dates` are `YYYY-MM-DD` text in ascending order, none before `from_day`; empty when
   the symbol has an error code.
"""

from typing import Protocol, TypedDict, runtime_checkable

ERROR_CODES: tuple[str, ...] = ("rate_limited", "no_data", "network", "timeout", "source_changed")


class SymbolSpec(TypedDict):
    """One symbol asked for."""

    symbol: str
    currency: str


class Request(TypedDict):
    """The JSON on stdin."""

    symbols: list[SymbolSpec]
    from_day: str


class SymbolResult(TypedDict):
    """The answer for one symbol; `error_code` is empty when ok."""

    symbol: str
    currency: str
    error_code: str
    dates: list[str]


class Answer(TypedDict):
    """The JSON on stdout."""

    results: list[SymbolResult]


@runtime_checkable
class Fetcher(Protocol):
    """Reads the next earnings dates of one symbol from the outside source; never retries."""

    def __call__(self, symbol: SymbolSpec, from_day: str) -> SymbolResult:
        """Return the result of the symbol; a failure is an `error_code`, not an exception."""
        ...
