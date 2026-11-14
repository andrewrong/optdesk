# ot strategy-replay

Design: `docs/design/strategy-replay.md#3 ot-strategy-replay` (Decisions 1, 2, 7, 8, 10 and 11). Files:
`cmd/ot/strategy_replay.go` (flags, wiring), `strategy_replay_lots.go` (the lots), `strategy_replay_market.go`
(quotes and closes), `strategy_replay_print.go` (the output).

## Nothing is written

The command reads and writes nothing. It holds the stores through their reader interfaces only, calls no
`Insert`, `Upsert` or `Record` method and opens each store with a line of `$OT_HOME/.env`, read-only:
`OT_PG_DSN_OPTION_LEDGER` (the trade rows), `OT_PG_DSN_PAGES` (option quotes, terms, instrument names),
`OT_PG_DSN_RUNNER` (the XNYS session days), `OT_PG_DSN_BAR` (the daily closes). A failed open prints a fixed text
that names the line and never the DSN. The demo profile is an error. The same stores give the same text.

## Flags

- `--lots short_puts|short_lots`: the lot set; default `short_puts`; another value is an error that names both.
  A lot is short when its quantity is negative; `short_puts` keeps the puts.
- `--alias a`: one account alias; default every stored alias.
- `--as-of DAY`: the as-of day `YYYY-MM-DD`; default today in the market zone. A lot whose settle day is after it
  is `not_expired`.
- `--early-keep K` (default `0.50`) and `--early-dte N` (default `21`): the early variant `early_<K as percent>_<N>`,
  the `shortput` strategy in `close_mode=early`. Hold is the engine's own `hold`.
- `--min-lots N` (default 20, trial value [T]): a line over fewer lots carries `under N [T]: no conclusion`.

## Output

In this order: the first and last stored quote day (`none` when there is none); the lot set, the as-of day, each
variant name with the `Params.Hash` of its strategy; the three notes (early assignment is not modelled and a day
without a stored quote is a gap, not a guess; the stored quote is the morning snapshot 90 minutes after the open
and settlement is the close; a rolled lot's actual does not include the lot it was rolled into); the coverage
counts; the lots still open; the excluded lots as `excluded <variant>:<reason>: n` in sorted order; then one row per
report line, every figure as `value (n=lots)`. Fates, settle days and the actual (gross per share, assigned lots at
credit minus intrinsic value at the settle-day close; net beside it) follow Decision 2.

A stored quote is, per contract and day, the ok row of the latest `received_at`; two rows with one time go to the
larger capture id. A day the store lacks is absent, never filled. An underlying that has no `option_eod` row since
the first entry has no instrument id here, so its lots have no close and are counted `no_settle_price`.
