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

## Book risk

Types in `internal/strategy/api.go`: `RiskClass`, `RiskLeg`, `RiskHolding`, `RiskFunds`, `RiskBook`, `RiskPoint`,
`RiskClassResult`, `RiskExcluded`, `RiskTotal`, `RiskOutcome`, `RiskCandidate`, `RiskFit`; interfaces
`RiskStresser` (`Stress(RiskBook) RiskOutcome`) and `RiskFitter` (`Fit(RiskBook, RiskOutcome, RiskCandidate) RiskFit`),
both pure. Design: `docs/design/strategy-risk.md` (Decisions 1, 2, 5, 6, 8, 12).

Sources (tags: [V] read in the source, [S] reading not fully confirmed, [T] our own choice, [O] owner config):

- FINRA Rule 4210(g)(2)(F) [V]: the portfolio-margin valuation uses ten equidistant intervals over the range of each product class:
  +6 percent / -8 percent for a high-capitalization broad-based index (`broad_high`), +-10 percent for a non-high-capitalization
  broad-based index (`broad_other`), +-15 percent for equity, narrow-based index and other equity-based products (`equity`).
  Ten intervals give eleven points, `low + k * (high - low) / 10` for k = 0 to 10; that "ten" means intervals and not points is [S].
  Which symbol is in which class is the owner's class table [O]; non-US names are stressed at the `equity` range [T].
- Rule 4210(g)(7) [V]: the requirement is the greater of the largest theoretical loss among the valuation points and the per-contract minimum.
  Here the largest loss of a class is its lowest point; the per-contract minimum is not applied [T].
- Rule 4210(g)(8) [V]: positions are grouped by class into portfolios, netted fully inside a portfolio at each point, and the total is the
  sum of the greatest loss of each portfolio; no offset between classes is modelled [V].

Every page that prints a figure prints: "this is the broker-dealer portfolio-margin ladder used as a sourced scenario, not your account's margin."
It is the fact `basis_note` of the row.

### Rows

Strategy `book_risk`, one row per alias and currency (key `<alias>:<currency>`) and one per currency for the household
(key `household:<currency>`). `underlying_id` is 0. Status is `ok` when the total of that currency is complete, `unknown`
otherwise, never `blocked` (Decision 12).

Fact keys of `book_risk` (none exists before this change; an absent figure is an absent key):

- `worst_loss_<currency>`: the total, only for a complete currency (nothing of it excluded); an incomplete currency has no total.
- `ladder_<class>`: the eleven points (move, profit and loss, assignment cash).
- `worst_<class>`: the worst point of the class with its move.
- `oldest_spot_day_<class>`: the oldest spot day among the class's holdings.
- `assignment_cash`: cash needed by short puts assigned at the worst point, never for a cash-settled European leg.
- `excluded`: the excluded holdings with reasons and counts.
- `not_stressed_<currency>`: count of holdings of another currency not stressed.
- `range_basis`: the range used, `finra_equity_applied` for a non-US name [T].
- `basis_note`: the sentence above.
- `equity_loss_vs_excess_liquidity`: the complete total beside `ExcessLiquidity`; absent when the total is not complete or the alias has no funds.
- `margin_change_not_modelled`: `true` [T]; no check passes or blocks on it.

Strategy `fit`, one row per ok candidate and alias with funds (key `<candidate key>:<alias>`), `underlying_id` of its candidate.
Status is `ok` when `fit_capital` passes, `blocked` when it blocks, `unknown` otherwise. One check, `fit_capital`.
Fact keys:

- `capital`: the capital of the structure (exchange formula).
- `open_put_strikes`: sum of `strike x multiplier x quantity` of the alias's open short puts in that currency.
- `worst_loss_with`: the alias's worst loss in the candidate's currency with the candidate added; absent when the alias has no complete total in that currency or the row has no `iv`.

### Reason words

`no_class`, `no_vol`, `stale_mark`, `expired`, `not_in_ledger`, `no_capture` (exclusions of a holding); `no_funds`, `no_fx`, `no_rule`,
`capital` (of `fit_capital`: `unknown no_funds`, `unknown no_fx`, `no_rule` for a structure no formula covers, `block capital`).
