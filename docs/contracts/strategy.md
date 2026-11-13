# Contract: strategy

Module `strategy` (`internal/strategy`). Design: `docs/design/strategy-framework.md` (Interface, Decisions 1, 2, 3, 8, 9).
The package imports `contracts` and `compute` and no store, job or page.

## Interface

`strategy.Strategy` is the one interface a strategy implements:

- `ID() string`: the strategy column of a result row and the strategy of its `[[need]]` row, e.g. `short_put`.
- `Version() string`: the rule version, e.g. `put_v1`; it changes whenever a rule changes.
- `Params() Params`: `Values` (named decimal or text values) and `Hash` (SHA-1 hex of the sorted `key=value` lines).
- `Evaluate(Snapshot) []Evaluation`: one `Evaluation` per candidate structure, `RankInUnderlying` set, `Rank` 0.
- `Rank([]Evaluation) []Evaluation`: sets `Rank` over the rows of the run and returns them in the strategy's order.
- `Manage(Position, Snapshot) Action`: `Action.Kind` is `hold` or `close`; `Reason` is empty for hold and a word for close.

Types: `Params`, `Snapshot` (what one call reads; reuses `compute.OptionQuote`, `EarningsInfo`, `DividendInfo`,
`AliasFunds`), `Evaluation`, `Leg`, `Check` (name = the gate word; status pass, block or unknown), `Position`, `Action`.
Days are `YYYY-MM-DD`; money and rates are decimal text.

## Rules

- Every method is pure: no I/O, no clock, exact decimal text arithmetic. The same input gives the same output, live and in replay.
- `Facts` omits an absent figure: the key is not in the map (never an empty value standing for "absent").
  A strategy copies `Snapshot.Context` into `Facts`.
- `Evaluation.Key` is unique within a day and a strategy: the instrument ids of the legs joined by `:` and the alias when the row is per alias.
- `Status` is `ok`, `blocked` or `unknown`; `Rank` is 0 when the status is not `ok`.
- `Manage` is declared in step 1 and called from step 2 (replay); nothing calls it in step 1.
