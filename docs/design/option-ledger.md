# Option ledger: every option position per account with its events, premium, wheel cost basis and P&L, derived from stored account data and stored for the pages

Author: the optdesk designer (draft)
Owner approved: (pending) the five questions under `## Open questions`
Date: 2026-11-09
Revision: 1

Source: owner 2026-11-09: value investor and option seller, options first; two IBKR accounts, labels `a` and `b`, no account id in the repository; the margin cushion and the assignment cash never go to Telegram (ADR 0010 stands for account data); owner rule 2026-11-08: own glue code sits behind an interface so a library can replace it. Plan v2.5 section 11 row W11 (ledger, tracking) and the alignment rows `option P&L ledger / option_result builders` and `Ledger` (board 20) of `docs/design/alignment-v25.md` on `origin/design/alignment-v25` (pull request 546). The candidates design names this one as the next: `docs/design/option-candidates.md` (612), table "The option side, whole", row "ledger, cost basis, wheel".
Read on main at 96f1a9e: `AGENTS.md`, `docs/design/modules.md`, `ease-v2.md`, `risk-alerts.md`, `internal/store/api.go`, `internal/compute/api.go`, `internal/jobs/api.go`, `internal/pages/api.go`, `go.mod`, the file lists of `contracts/datasets`, `internal/{compute,store,jobs,pages}` and `migrations`. On branches in review: `docs/design/account-data.md` (561), `docs/design/option-candidates.md` (612), `docs/design/event-sources.md` (606), `docs/design/alignment-v25.md` (546). option-trader (read only, nothing copied): `private_book/executions.py` (executions come from a stored Flex report with signed quantity and per-premium-unit price; a correction is the same `ExecID` as the next `seq`), `private_book/positions.py` (a normalized cost basis per premium unit), `backtest/strategies/wheel.py` (a wheel simulation only). option-trader has no ledger of assignments and no cost basis net of premiums: this is new, not a port. No credential, `.env` or service data was read.

Goal: after each account capture, one stored view per alias of every option position the owner has held since the start day (open, closed, assigned, expired), with the events that changed it, the premium collected, the realized and unrealized profit and loss, and, for the wheel, the cost basis of the shares that an assignment gave, net of the premiums collected. `option-tracking` and `option-candidates-page` read it; `ot ledger` prints it until a page exists.

## What main holds today

- Nothing: no ledger, event, lot, cost-basis or P&L code, dataset or table. The only "ledger" names are the operations ledger of the runner (`store.OpsLedger`, `SchedulerLedger`), which this design does not touch and does not reuse; every name here carries `Option`.
- In review, and the inputs of this design: `account.position` captures per alias and `account.execution` rows (561, insert-only, alias only, decimals as text, the API gives the current trading day only so executions accumulate from the first run), `obs.option_eod` rows of held short calls (612 issue 9) and the owner's `watch.toml` (606).
- `compute` holds pure rule packages behind interfaces (`internal/compute/ease`, `alertrules`), `store` holds one role and one schema per data class, `jobs` holds one package per job with a `job.toml`.

## Decisions

1. Derived and stored. A pure rule package `internal/compute/optionledger` (no I/O, no clock) behind `compute.OptionLedger`, so a library can replace it. A job `option_ledger` reads stored account data, replays the whole history from the start day for each alias, and writes three datasets, `optionledger.event`, `optionledger.position` and `optionledger.cycle`, through `store.OptionLedgerWriter`. Pages compute nothing, so every figure they show is a stored column. The replay is deterministic: the same inputs give the same rows.
2. Inputs, all of them from 561 and 606 and nothing new from the broker. Option and stock executions (`account.execution`) give opens, closes and stock trades. Position captures (`account.position`) give the opening balance (Decision 5) and the detection of expiry and assignment (Decision 4). Marks come from the newest `obs.option_eod` row of the contract (mid of bid and ask) and the underlying price of the same row. The owner's file `ledger.toml` in `$OT_HOME` gives the start day and optional manual events (Decision 11). No Flex, no new outside access, no new credential. History before the start day is not in the ledger (Left out).
3. Event kinds: `seed_open` (a position held at the first capture), `open`, `close` (an option execution), `assign` (the option was exercised against the owner: a short put gives shares, a short call takes them), `expire` (worthless), `stock_buy`, `stock_sell` (a stock execution), `unresolved` (the option is gone and neither rule below can say why). The column `source` is `execution`, `detected`, `manual` or `seed`. Quantity is signed in the position (a short is negative), the event row carries side and an unsigned quantity.
4. Detection of expiry and assignment, never guessed. An option position present in the capture of session day D-1 and absent at D, with no closing execution of that contract on D, is resolved by the stock position of the same alias and underlying between the same two captures. Let `n = quantity x multiplier` of the option. A short put gone, with the stock position up by `n`: `assign`. A short call gone, with the stock down by `n`: `assign` (the shares are called away). The stock position unchanged and the expiry on or before D: `expire`. Any other case, a missed capture day included, is `unresolved`: the position is listed with that status, counted apart, and never enters a P&L figure. An assignment before the expiry date carries the note `early`. A manual event for the same contract and day replaces a detected one (Decision 11). The assignment price is the strike; the cash is `strike x multiplier x quantity`, stored in `cash_amount`.
5. The opening balance. At the first capture of an alias on or after the start day, each position with no event is a `seed_open` at the average cost IBKR reports, converted to a price per unit (the option average cost is per contract, so the rule divides by the multiplier; the first run is read against the broker screen and a test pins the conversion). A seeded option keeps its real opened date unknown (`opened_on` is the capture day, mark `seeded`); a seeded stock lot has a tax basis but no premium history, so its net basis equals its tax basis and carries the mark `premium_history_missing` until manual events or an import fill it.
6. Matching. Per alias and contract, closes match opens first in, first out; a close with no open (before the start day, or a manual gap) is `unresolved`, not a negative lot. Long and short positions are one mechanism (the sign of the quantity). A position is one open lot or the part of it a close left; a lot closed in two steps is two rows of the same open with the split noted. Stock lots are first in, first out too. Spreads are not grouped: each leg is its own position (Left out).
7. Premium and realized P&L, per position, in the currency of the contract, before tax. `premium_collected` is the received price x quantity x multiplier of a short open (for a long open the same field holds the paid amount, negative); `close_cost` is the paid amount of the close, `commissions` the sum of both legs, `realized_pnl = premium_collected - close_cost - commissions` for a closed position and `= premium_collected - commissions` for an expired one. An assigned position has no option P&L: its premium moves into the cost basis of the shares (Decision 8) and becomes profit only when those shares are sold or called away. `realized_pnl` of an assigned position is empty with the status `assigned`, and its premium is in the cycle.
8. Wheel cycle and cost basis. A cycle starts when a stock position of an alias and underlying appears (an assignment, a `stock_buy`, or a seed) and ends when the shares are flat. `cycle_id` is `<alias>:<symbol>:<n>`, `n` counting the cycles of that name in replay order. `tax_basis_total` is `strike x shares` of assignments plus the cost of stock bought plus stock commissions; this is the broker's basis and the tax basis. `premium_in_basis` is the premium of the assigned put plus, while the shares are held, every later option premium on the name (covered calls, further puts), minus the cost of closes and minus option commissions, under the scope parameter of Decision 12. `net_basis_total = tax_basis_total - premium_in_basis`, per share `net_basis_total / shares`, which may be negative. Shares leaving (a sale or a call assignment) take their share of both totals in proportion to the shares; a call assignment sells at the strike. When the cycle ends, `realized_pnl = proceeds - tax_basis_sold + premium_in_basis_sold`. Premiums on the name outside any cycle (a put that expired worthless) are realized option P&L of their positions, and `premium_other` states what the other scope of Decision 12 would have added, so the owner sees both numbers.
9. Unrealized. An open position: `unrealized_pnl = premium_collected - mark x quantity x multiplier - commissions paid so far` for a short, the mirror for a long, where `mark` is the mid of the newest `obs.option_eod` row of the contract, with `mark_day` stored. No row, or a row older than 5 session days, gives an empty `unrealized_pnl` and the mark `no_mark`; nothing is estimated. A cycle's `unrealized_stock_pnl = (stock_mark - net_basis_per_share) x shares` with the stock mark from the same source. Held short calls are captured by 612 issue 9; held puts and long options are not, so issue 12 of this design extends that capture to every held option, and until it merges short puts show `no_mark`.
10. Numbers. Money and quantities are `math/big.Rat` inside the rule (exact sums, no decimal library in `go.mod`, the precedent of 612 Decision 1), decimal text at every boundary, rounded half-even to 4 places in the stored columns. No float anywhere in the rule or the datasets. Every row carries its currency; no function adds two currencies and no FX conversion is built (Left out), so a total is per alias and currency.
11. The owner's file `ledger.toml` (in `$OT_HOME`, next to `watch.toml`): `start_day` (required, the day of the first account capture, text `YYYY-MM-DD`) and zero or more `[[event]]` tables with `alias`, `day`, `kind` (`assign`, `expire`, `close`, `stock_buy`, `stock_sell`), `symbol`, the option terms when it is an option, `quantity`, `price` and `commission` as decimal text. The loader refuses a bad value and names the line; manual events are inputs of the replay and win over a detected event for the same contract and day, a permanent precedence like the owner's buy price in 612. The file holds labels and numbers, no account id.
12. Parameters of the rule (keys of `job.toml`, the owner may change them): `premium_scope` is `assigned_and_later` (Decision 8; the alternative `all_since_flat` adds every premium on the name since the shares were last flat), `mark_max_age_sessions` is 5. `rule_version` is a column of every row and starts at 1.
13. Identity and run key. A row is keyed `(session_day, run_id, alias, seq)` for events, `(..., position_id)` for positions and `(..., cycle_id)` for cycles; `position_id` is `<alias>:<con_id or symbol:expiry:strike:right>:<opened_on>:<n>` so a replay gives the same id. Insert-only; a rerun is a new `run_id` (the runner's run id, as in the capture id of 561 Decision 5), and the reader takes the newest run of the day by `computed_at` per alias. The full replay is stored each run: the rows grow with events x runs, which is small at this scale and is named under Left out (retention). No `UPDATE` or `DELETE` is granted.
14. A role of its own, as for account data: `ot_w_optionledger` with INSERT and SELECT on the schema `optionledger`, DSN line `OT_PG_DSN_OPTION_LEDGER`, `store.DSNEnvOptionLedger`; `ot_ro_pages` gets USAGE and SELECT. Every row is account-derived, so it lives under that rule, not with market data.
15. Delivery rule, binding: nothing here reaches notify. A job result carries a job id, a status and an alias only; no position, premium or P&L. `internal/notify` imports none of the new packages (`scripts/arch` enforces it); the text formatter is imported by `cmd/ot` only.
16. Order. In `run-once`: `account_capture`, `option_eod`, then `option_ledger`; in the scheduler it is anchored at `session_close` plus 40 minutes, after `account_capture` and `option_eod` by their own designs. If an input is late, the day's capture is missing and a position that needed it is `unresolved` or `no_mark`; a rerun replaces the view.
17. Reader. `store.OptionLedgerReader` is what `option-tracking` and `option-candidates-page` use; `ot ledger [--alias a] [--status S] [--cycles]` prints the stored rows as text through `internal/pages/option_ledger_text`, formatting only, so the owner has the ledger before a page.

## Second opinion

Cases for the advisor (no Advisor line is written here): a new store of account-derived figures under a new role (`optionledger-store-migration`, `optionledger-store-impl`): questions (a) the role cannot update or delete; (b) the schema never reaches `Sender` and `notify` imports nothing of it; (c) alias is the only identity. No new outside access is added: the job reads only 561 and 606 tables and `obs.option_eod`.

## Modules

New directories: `internal/compute/optionledger`, `internal/jobs/optionledger`, `internal/store/optionledger`, `internal/pages/option_ledger_text`. This pull request edits `docs/design/modules.md` by four names in the sentence that lists the implementation packages `cmd/ot` may import (the precedent is `internal/compute/ease` and 612), and `arch.toml` is regenerated with `go run ./scripts/arch` so that its `--check` passes; no depends cell changes (compute depends on contracts only, and the rule uses plain types of its own; jobs already names store, compute and clock; pages already names store). Interface files, each additive and declarations only: `internal/compute/api.go`, `internal/store/api.go` (its test in a new file, because `internal/store/api_test.go` has 800 lines and is not in `docs/design/ratchet.json`), `internal/jobs/api.go`; the datasets in `contracts/`. A new sub-package needs `.go-arch-lint.yml`, a repo path regenerated by `go run ./scripts/arch`: the licensed two-module entries are the ones that create a package (precedent `risk-alerts.md` `alerts-store-interface`).

| # | issue | module | order |
|---|---|---|---|
| 1 | optionledger-compute-interface (interface) | compute, plus repo (licensed pair) | first |
| 2 | optionledger-dataset (interface) | contracts | after 561 `account-proto` |
| 3 | optionledger-store-api (interface) | store | after 2, 561 `account-store-api` |
| 4 | optionledger-events | compute | after 1 |
| 5 | optionledger-positions | compute | after 4 |
| 6 | optionledger-cycles | compute | after 5 |
| 7 | optionledger-store-migration | store | after 3, 561 `account-store-migration` |
| 8 | optionledger-store-impl | store, plus repo (licensed pair) | after 7, 561 `account-store-impl` |
| 9 | optionledger-job (interface and implementation) | jobs, plus repo (licensed pair) | after 6, 8, 561 `account-job` |
| 10 | optionledger-text | pages, plus repo (licensed pair) | after 3 |
| 11 | optionledger-wire | ot | after 9, 10, 561 `account-wire` |
| 12 | option-eod-held-all | jobs | after 612 issue 9 (`option-eod-held-calls`) |

The ledger shows positions and events at issue 11 and unrealized P&L of short puts at issue 12. Issues 4 to 6 are three compute files of one package, ordered because each reads the output types of the one before.

## Interface

| name | owner and file | declaration |
|---|---|---|
| `compute.OptionLedger` (`Replay(LedgerInput, LedgerParams) LedgerResult`), `compute.NewOptionLedgerFunc func() OptionLedger`; types `LedgerEventKind`, `LedgerInput`, `LedgerExecution`, `LedgerCapture`, `LedgerHeld` (a position row of a capture), `LedgerMark`, `ManualLedgerEvent`, `LedgerParams{PremiumScope string, MarkMaxAgeSessions int}`, `LedgerResult{Events, Positions, Cycles}`, `LedgerEventRow`, `LedgerPositionRow`, `LedgerCycleRow` | compute, `internal/compute/api.go` (issue 1) | declarations only, additions only; one alias per call; the input carries the XNYS session days as ascending day texts from the start day to the run day (supplied by the job from `store.SessionDayReader`), so the rule needs no calendar or clock; decimals as text |
| datasets `optionledger.event` r1, `optionledger.position` r1, `optionledger.cycle` r1 | contracts, `contracts/datasets/optionledger.*.r1.json` (issue 2) | event: `session_day, run_id, alias, seq, rule_version, kind, source, occurred_at, symbol, sec_type, expiry, strike, right, multiplier, currency, side, quantity, price, commission, commission_currency, cash_amount, exec_id, position_id, cycle_id, note, computed_at`; position: `session_day, run_id, alias, position_id, rule_version, symbol, expiry, strike, right, multiplier, currency, direction, quantity_opened, quantity_open, opened_on, closed_on, status, premium_collected, close_cost, commissions, realized_pnl, mark, mark_day, unrealized_pnl, dte, cycle_id, marks, computed_at`; cycle: `session_day, run_id, alias, cycle_id, rule_version, symbol, currency, opened_on, closed_on, status, shares, tax_basis_total, tax_basis_per_share, premium_in_basis, premium_other, net_basis_total, net_basis_per_share, stock_mark, stock_mark_day, unrealized_stock_pnl, realized_pnl, marks, computed_at`; money as decimal text, empty when not known |
| `store.OptionLedgerWriter` (`InsertOptionLedgerRun(ctx, OptionLedgerRun) error`, one transaction), `store.OptionLedgerReader` (`OptionLedgerPositions(ctx, alias, day) ([]OptionLedgerPosition, error)`, `OptionLedgerCycles(ctx, alias, day) ([]OptionLedgerCycle, error)`, `OptionLedgerEvents(ctx, alias, from, to) ([]OptionLedgerEvent, error)`; newest run of the day, rows sorted by key), `store.OptionLedgerStore` (both and `Close() error`), `store.DSNEnvOptionLedger = "OT_PG_DSN_OPTION_LEDGER"`, `store.OpenOptionLedgerFunc func(ctx, dsn) (OptionLedgerStore, error)`, `OptionLedgerRun{RunID, Alias, SessionDay, RuleVersion, ComputedAt, Events, Positions, Cycles}`, plain row types `OptionLedgerEvent`, `OptionLedgerPosition`, `OptionLedgerCycle` (the dataset columns, decimals as text; store imports neither compute nor the generated records, the job maps) | store, `internal/store/api.go` (issue 3) | declarations only, additions only; implementation `internal/store/optionledger.Open` |
| `jobs.OptionLedgerStores{Accounts store.AccountReader; OptionEod store.OptionEodReader; Terms store.OptionTermsReader; SessionDays store.SessionDayReader; Ledger store.OptionLedgerWriter}`, `jobs.OptionLedgerConfig{Aliases []string; StartDay string; Manual []compute.ManualLedgerEvent}`, `jobs.NewOptionLedgerJobFunc func(OptionLedgerStores, compute.OptionLedger, OptionLedgerConfig, clock.Clock) (contracts.Job, error)` | jobs, `internal/jobs/api.go` (issue 9) | additions only; a nil `OptionEod` makes every mark `no_mark` |
| `option_ledger_text.Format(positions []store.OptionLedgerPosition, cycles []store.OptionLedgerCycle, statuses []string) string` | pages, own package, not `internal/pages/api.go` | no sort, no arithmetic: text of stored columns; imported by `cmd/ot` only (as 612 `candidate_text`) |

## Issues

Every test calls `t.Parallel()`, writes files only under `t.TempDir()`, uses the fake clock of the test kit, never sleeps, opens no port, and touches no live broker or database (a test that needs PostgreSQL carries the tag `pg`). Fixtures use the labels `a` and `b`, invented symbols and invented figures. Acceptance is checked offline by the command shown; `BASE=$(git merge-base HEAD origin/main)`.

### 1 optionledger-compute-interface (interface)

Module: compute, plus repo (licensed pair: `.go-arch-lint.yml`). Size: S.
Files: `internal/compute/api.go`, `internal/compute/api_test.go`, `internal/compute/optionledger/doc.go`, `docs/contracts/compute.md`, `.go-arch-lint.yml`
Design: docs/design/option-ledger.md#interface
Order: first.
Goal: the compute declarations of the Interface table, declarations only, with a contract test from both sides (a fake `OptionLedger` and a caller); the package `optionledger` as a documented stub; `docs/contracts/compute.md` records the rule text of Decisions 3 to 9.
Acceptance:
1. `grep -c "type OptionLedger interface\|type NewOptionLedgerFunc " internal/compute/api.go` prints 2; `git diff $BASE -- internal/compute/api.go | grep '^-' | grep -vc '^---'` prints 0; `grep -c "var _ compute.OptionLedger" internal/compute/api_test.go` prints 1.
2. `go test ./internal/compute -run 'TestOptionLedger' -race -count=1` passes; `git diff $BASE -- internal/compute/api.go | grep '^+' | grep -c "float32\|float64"` prints 0 (no float is added).
3. `grep -c "optionledger\|premium_in_basis" docs/contracts/compute.md` is at least 2; `grep -c "compute/optionledger" .go-arch-lint.yml` is at least 1.
4. `git diff --name-only $BASE | grep -vxF -e internal/compute/api.go -e internal/compute/api_test.go -e internal/compute/optionledger/doc.go -e docs/contracts/compute.md -e .go-arch-lint.yml | wc -l | tr -d ' '` prints 0.
Checked by CI: ci-go; `go run ./scripts/arch --check` printing `arch: ok`; the design verdict.

### 2 optionledger-dataset (interface)

Module: contracts. Size: S.
Files: `contracts/datasets/optionledger.event.r1.json`, `contracts/datasets/optionledger.position.r1.json`, `contracts/datasets/optionledger.cycle.r1.json`, `contracts/generate.sh`, `docs/contracts/datasets/optionledger.event.md`, `docs/contracts/datasets/optionledger.position.md`, `docs/contracts/datasets/optionledger.cycle.md`, `internal/contracts/gen/records.go`, `internal/contracts/schema_test.go`
Design: docs/design/option-ledger.md#interface
Order: after 561 `account-proto` (same `generate.sh` and `records.go`; rebase on whichever lands first).
Goal: the three datasets of the Interface table with the keys of Decision 13, money as decimal text strings (no number type for money), the generated records committed with their source.
Acceptance:
1. `ls contracts/datasets/optionledger.*.r1.json | wc -l | tr -d ' '` prints 3; `grep -c '"run_id"\|"rule_version"' contracts/datasets/optionledger.event.r1.json` is at least 2; `grep -ci '"number"\|float\|double' contracts/datasets/optionledger.*.r1.json | grep -vc ':0'` prints 0.
2. `grep -c "optionledger\.\(event\|position\|cycle\)\.r1" contracts/generate.sh` is at least 3; `grep -c "OptionledgerEvent\|OptionLedgerEvent" internal/contracts/gen/records.go` is at least 1.
3. `go test ./internal/contracts -race -count=1` passes with a schema test that each of the three files parses and names its key.
4. `git diff --name-only $BASE | grep -vxE 'contracts/(datasets/optionledger\.[a-z]+\.r1\.json|generate\.sh)|docs/contracts/datasets/optionledger\.[a-z]+\.md|internal/contracts/(gen/records\.go|schema_test\.go)' | wc -l | tr -d ' '` prints 0.
Checked by CI: the generated-code check (`sh contracts/generate.sh` leaves no diff); ci-go; the design verdict.

### 3 optionledger-store-api (interface)

Module: store. Size: S.
Files: `internal/store/api.go`, `internal/store/optionledger_api_test.go`, `docs/contracts/store.md`
Design: docs/design/option-ledger.md#interface
Order: after 2 and 561 `account-store-api` (it names no account type, but both add to `api.go`: rebase on whichever lands first).
Goal: the store declarations of the Interface table, declarations only, the contract test from both sides (a fake `OptionLedgerStore` and a caller) in the new file `internal/store/optionledger_api_test.go`; `docs/contracts/store.md` records the schema `optionledger`, the keys of Decision 13, the insert-only rule and the delivery rule of Decision 15.
Acceptance:
1. `grep -c "type OptionLedgerStore interface\|type OptionLedgerReader interface\|type OptionLedgerWriter interface\|type OpenOptionLedgerFunc " internal/store/api.go` prints 4; `grep -c 'DSNEnvOptionLedger = "OT_PG_DSN_OPTION_LEDGER"' internal/store/api.go` prints 1; `git diff $BASE -- internal/store/api.go | grep '^-' | grep -vc '^---'` prints 0.
2. `grep -c "var _ store.OpenOptionLedgerFunc" internal/store/optionledger_api_test.go` prints 1; `grep -c "^type OptionLedger\(Event\|Position\|Cycle\|Run\) struct" internal/store/api.go` prints 4; `go test ./internal/store -run 'TestOptionLedger' -race -count=1` passes.
3. `grep -c "optionledger\|insert-only" docs/contracts/store.md` is at least 2; `wc -l < internal/store/api.go` prints less than 800.
4. `git diff --name-only $BASE | grep -vxF -e internal/store/api.go -e internal/store/optionledger_api_test.go -e docs/contracts/store.md | wc -l | tr -d ' '` prints 0.
Checked by CI: ci-go; the design verdict.

### 4 optionledger-events

Module: compute. Size: M.
Files: `internal/compute/optionledger/events.go`, `internal/compute/optionledger/detect.go`, `internal/compute/optionledger/manual.go`, `internal/compute/optionledger/replay.go`, `internal/compute/optionledger/events_test.go`, `internal/compute/optionledger/detect_test.go`, `internal/compute/optionledger/manual_test.go`, `internal/compute/optionledger/testdata/events_basic.json`, `internal/compute/optionledger/testdata/detect_cases.json`
Design: docs/design/option-ledger.md#decisions (Decisions 3 to 5, 10, 11)
Order: after 1.
Goal: `optionledger.New()` (the type `NewOptionLedgerFunc`) with `Replay` producing the ordered event rows: executions as `open`, `close`, `stock_buy`, `stock_sell`; the seed at the first capture (Decision 5); the detection of `assign`, `expire` and `unresolved` between consecutive captures (Decision 4), `early` noted; manual events replacing detected ones; ordering by day, then time, then `exec_id`; the same input gives the same output. Positions and cycles are empty until issues 5 and 6 (the result type exists since issue 1).
Acceptance:
1. `go test ./internal/compute/optionledger -race -count=1` passes with `TestDetectShortPutAssigned`, `TestDetectShortCallCalledAway`, `TestDetectExpiredWorthless`, `TestDetectEarlyAssignmentNoted`, `TestDetectMissedCaptureUnresolved`, `TestDetectClosedSameDayNotDetected`, `TestSeedConvertsOptionAverageCost`, `TestManualEventReplacesDetected`, `TestReplayDeterministic` each present once (`grep -c` of each name is 1).
2. `grep -c "float32\|float64" internal/compute/optionledger/*.go | grep -vc ':0'` prints 0; `ls internal/compute/optionledger/*.go | grep -v _test.go | xargs grep -l '"os"\|"time"\|net/http\|database/sql' | wc -l | tr -d ' '` prints 0 (no I/O and no clock in the non-test files).
3. `ls internal/compute/optionledger/*.go | grep -v _test.go | xargs wc -l | grep -v total | awk '$1>=600' | wc -l | tr -d ' '` prints 0; the diff lists no path outside the Files line.
Checked by CI: ci-go; golangci-lint.

### 5 optionledger-positions

Module: compute. Size: M.
Files: `internal/compute/optionledger/match.go`, `internal/compute/optionledger/pnl.go`, `internal/compute/optionledger/mark.go`, `internal/compute/optionledger/match_test.go`, `internal/compute/optionledger/pnl_test.go`, `internal/compute/optionledger/mark_test.go`, `internal/compute/optionledger/testdata/positions_golden.json`, `internal/compute/optionledger/replay.go`, `internal/compute/optionledger/replay_test.go`
Design: docs/design/option-ledger.md#decisions (Decisions 6, 7, 9, 10)
Order: after 4 (it reads the event rows; `replay.go` gains one call).
Goal: the position rows: first in, first out matching of closes to opens per alias and contract, the partial close split, the `unresolved` close with no open, `premium_collected`, `close_cost`, `commissions` and `realized_pnl` by the rule of Decision 7 (an assigned position has no option P&L), the mark and `unrealized_pnl` of Decision 9 with the age limit and `no_mark`; a golden file of positions for a short put sold and bought back, one expired, one assigned, one closed in two steps, one long call, one seeded.
Acceptance:
1. `go test ./internal/compute/optionledger -race -count=1` passes with `TestFIFOMatchesOldestOpen`, `TestPartialCloseSplitsLot`, `TestCloseWithoutOpenUnresolved`, `TestRealizedPnLClosedShort`, `TestRealizedPnLExpired`, `TestAssignedPositionHasNoOptionPnL`, `TestUnrealizedShortUsesMid`, `TestStaleMarkIsNoMark`, `TestLongPositionSigns`, `TestGoldenPositions` each present once.
2. `TestRealizedPnLClosedShort` sells 2 contracts at 1.50 and buys them back at 0.40 with multiplier 100 and commissions 2.60 in total and requires `realized_pnl` text `217.4000`: `grep -c '217.4000' internal/compute/optionledger/pnl_test.go` is at least 1.
3. `grep -c "float32\|float64" internal/compute/optionledger/*.go | grep -vc ':0'` prints 0; each new file is under 600 lines; the diff lists no path outside the Files line.
Checked by CI: ci-go; golangci-lint.

### 6 optionledger-cycles

Module: compute. Size: M.
Files: `internal/compute/optionledger/cycle.go`, `internal/compute/optionledger/basis.go`, `internal/compute/optionledger/cycle_test.go`, `internal/compute/optionledger/basis_test.go`, `internal/compute/optionledger/testdata/wheel_golden.json`, `internal/compute/optionledger/replay.go`, `internal/compute/optionledger/replay_test.go`
Design: docs/design/option-ledger.md#decisions (Decisions 8, 10, 12)
Order: after 5 (same `replay.go`).
Goal: the cycle rows of Decision 8: start at a stock position, the tax basis, the premium in basis under both values of `premium_scope` with `premium_other` the difference, the net basis total and per share (negative allowed), shares leaving in proportion (a sale or a call assignment), the cycle's realized P&L at the end, the unrealized stock P&L from the stock mark, the mark `premium_history_missing` for a seeded lot; a golden wheel: short put assigned, two covered calls expire, a third bought back at a loss, shares called away.
Acceptance:
1. `go test ./internal/compute/optionledger -race -count=1` passes with `TestBasisNetOfAssignedPutPremium`, `TestCoveredCallPremiumReducesBasis`, `TestBuybackCostRaisesBasis`, `TestCallAssignmentEndsCycle`, `TestPartialCallAwayProportional`, `TestScopeAllSinceFlatAddsExpiredPuts`, `TestNetBasisMayBeNegative`, `TestSeededStockMarksPremiumMissing`, `TestUnrealizedStockPnL`, `TestGoldenWheel` each present once.
2. `TestBasisNetOfAssignedPutPremium` has a put with strike 50, 1 contract sold at 1.20, assigned, and requires `tax_basis_per_share` `50.0000` and `net_basis_per_share` `48.8000`: `grep -c '48.8000' internal/compute/optionledger/basis_test.go` is at least 1.
3. `grep -c "float32\|float64" internal/compute/optionledger/*.go | grep -vc ':0'` prints 0; each new file is under 600 lines and `wc -l internal/compute/optionledger/replay.go` prints less than 600; the diff lists no path outside the Files line.
Checked by CI: ci-go; golangci-lint.

### 7 optionledger-store-migration

Module: store. Size: M.
Files: `migrations/NNNN_optionledger.sql` (NNNN the next free number at cut time), `internal/store/setup/setup.go`, `internal/store/setup/setup_test.go`, `internal/store/setup/optionledger_pg_test.go` (tag `pg`)
Design: docs/design/option-ledger.md#decisions (Decisions 13 and 14; second-opinion case)
Order: after 3 and 561 `account-store-migration` (same `setup.go`; this entry takes the next free number and rebases on whichever of 561, 606 and 612 lands first).
Goal: schema `optionledger` with the tables `optionledger.event`, `optionledger.position`, `optionledger.cycle` and the keys of Decision 13 (`numeric` for money and quantity, `date` and `timestamptz` for days and times); the role `ot_w_optionledger` (INSERT and SELECT only) made by `setup` with the DSN line; USAGE and SELECT for `ot_ro_pages`.
Acceptance:
1. `M=$(ls migrations/*_optionledger.sql)`; `grep -cE "CREATE TABLE( IF NOT EXISTS)? optionledger\.(event|position|cycle)" $M` prints 3; `grep -v '^--' $M | grep -ci "UPDATE\|DELETE\|TRUNCATE"` prints 0; `grep -ci "float\|double" $M` prints 0.
2. `grep -v '^--' $M | grep -c "ot_ro_pages"` is at least 2; `grep -v '^--' $M | grep -c "ot_w_optionledger"` is at least 2; `grep -c "ot_w_optionledger" internal/store/setup/setup.go` is at least 2.
3. `go test ./internal/store/setup -run 'TestRoles|TestOptionLedgerRole' -race -count=1` passes; `head -1 internal/store/setup/optionledger_pg_test.go` prints `//go:build pg`; `wc -l < internal/store/setup/setup.go` prints less than 800.
4. The diff lists no path outside the Files line.
Checked by CI: ci-go; the `pg` job (`TestOptionLedgerMigration`: it applies on a clean database, the role has the grants above and no more, `ot_ro_pages` can SELECT).

### 8 optionledger-store-impl

Module: store, plus repo (licensed pair: `.go-arch-lint.yml`). Size: M.
Files: `internal/store/optionledger/doc.go`, `internal/store/optionledger/store.go`, `internal/store/optionledger/read.go`, `internal/store/optionledger/store_test.go`, `internal/store/optionledger/read_test.go`, `internal/store/optionledger/store_pg_test.go` (tag `pg`), `.go-arch-lint.yml`
Design: docs/design/option-ledger.md#decisions (Decisions 13 and 14)
Order: after 7 and 561 `account-store-impl` (both regenerate `.go-arch-lint.yml`: rebase on whichever lands first).
Goal: `optionledger.Open` returns the `store.OptionLedgerStore` over pgx with the role `ot_w_optionledger`: `InsertOptionLedgerRun` is one transaction over the three tables and a repeated `run_id` and alias writes nothing twice; the reader returns the newest run of the day for the alias, rows sorted by key. Non-`pg` tests run against a fake `database` as in `internal/store/snapshot`.
Acceptance:
1. `go test ./internal/store/optionledger -race -count=1` passes; `grep -c "t.Parallel()" internal/store/optionledger/store_test.go` is at least 4 (insert, repeated run, empty day, two runs give the newer one).
2. `head -1 internal/store/optionledger/store_pg_test.go` prints `//go:build pg`; `grep -rl "pgx" internal --include=*.go | grep -vc '^internal/store/'` prints 0; `grep -rl "os.Getenv" internal/store/optionledger | wc -l | tr -d ' '` prints 0.
3. `grep -ci "update \|delete from\|truncate" internal/store/optionledger/store.go` prints 0; `wc -l internal/store/optionledger/*.go | grep -v total | awk '$1>=600' | wc -l | tr -d ' '` prints 0; `grep -c "store/optionledger" .go-arch-lint.yml` is at least 1.
4. `git diff --name-only $BASE | grep -vxE 'internal/store/optionledger/[a-z_]+\.go|\.go-arch-lint\.yml' | wc -l | tr -d ' '` prints 0.
Checked by CI: ci-go; the `pg` job (`TestOptionLedgerRoundTrip`, `TestOptionLedgerNewestRunWins`); `go run ./scripts/arch --check`.

### 9 optionledger-job (interface and implementation)

Module: jobs, plus repo (licensed pair: `.go-arch-lint.yml`). Size: M.
Files: `internal/jobs/api.go`, `internal/jobs/api_test.go`, `internal/jobs/optionledger/job.go`, `internal/jobs/optionledger/job.toml`, `internal/jobs/optionledger/schedule.go`, `internal/jobs/optionledger/load.go`, `internal/jobs/optionledger/map.go`, `internal/jobs/optionledger/job_test.go`, `internal/jobs/optionledger/schedule_test.go`, `internal/jobs/optionledger/load_test.go`, `internal/jobs/optionledger/map_test.go`, `docs/contracts/jobs.md`, `.go-arch-lint.yml`
Design: docs/design/option-ledger.md#decisions (Decisions 1, 2, 12, 15, 16; interface of the jobs row)
Order: after 6, 8 and 561 `account-job` (`.go-arch-lint.yml`).
Goal: the job `option_ledger` through interfaces only: per alias of the config, read the position captures and executions from `StartDay` to the run day (`AccountReader`, the days from `SessionDayReader`), the marks and underlying price from `OptionEodReader` (at most 500 contract ids per `OptionTermsReader` call), call `Replay`, map the rows to store types and write one run. A failed alias does not stop the other; a result names a failed alias and a status only; an empty `StartDay` fails the job naming the key `start_day` of `ledger.toml`; `job.toml` carries the schedule of Decision 16, the aliases (labels) and the two parameters of Decision 12, and no account id, socket or value.
Acceptance:
1. `go test ./internal/jobs/optionledger ./internal/jobs -race -count=1` passes with `TestOneRunPerAlias`, `TestFailedAliasDoesNotStopOther`, `TestEmptyStartDayFails`, `TestResultCarriesNoFigures`, `TestMarksChunkedAt500`, `TestNilOptionEodGivesNoMark`, `TestSameRunIDWritesOnce` each present once.
2. `grep -c "NewOptionLedgerJobFunc\|OptionLedgerStores\|OptionLedgerConfig" internal/jobs/api.go` is at least 3; `git diff $BASE -- internal/jobs/api.go | grep '^-' | grep -vc '^---'` prints 0; `grep -c "^\[schedule\]" internal/jobs/optionledger/job.toml` prints 1; `grep -ciE "DU[0-9]|U[0-9]{6,}|socket|token" internal/jobs/optionledger/job.toml` prints 0.
3. `grep -rln "internal/notify\|internal/store/optionledger\|internal/store/account\|grpcbroker" internal/jobs/optionledger | wc -l | tr -d ' '` prints 0; `grep -c "time.Sleep" internal/jobs/optionledger/*.go | grep -vc ':0'` prints 0; `grep -c "optionledger" .go-arch-lint.yml` is at least 1.
4. `git diff --name-only $BASE | grep -vxE 'internal/jobs/(api|api_test)\.go|internal/jobs/optionledger/[a-z_.]+|docs/contracts/jobs\.md|\.go-arch-lint\.yml' | wc -l | tr -d ' '` prints 0.
Checked by CI: ci-go; `go run ./scripts/arch --check`; golangci-lint.

### 10 optionledger-text

Module: pages, plus repo (licensed pair: `.go-arch-lint.yml`). Size: S.
Files: `internal/pages/option_ledger_text/doc.go`, `internal/pages/option_ledger_text/format.go`, `internal/pages/option_ledger_text/format_test.go`, `internal/pages/option_ledger_text/testdata/ledger.txt`, `.go-arch-lint.yml`
Design: docs/design/option-ledger.md#interface (the pages row)
Order: after 3.
Goal: `Format` over stored rows: open positions first, then closed, assigned, expired and `unresolved` listed apart with a count, one block per cycle with the basis lines (tax basis, premium in basis, net basis per share), amounts as stored text and the currency beside each, `no_mark` and `premium_history_missing` printed, no sum across currencies; a golden text file.
Acceptance:
1. `go test ./internal/pages/option_ledger_text -race -count=1` passes with `TestFormatGolden`, `TestUnresolvedListedApart`, `TestNoCrossCurrencySum`, `TestEmptyInputOneLine` each present once.
2. `grep -c "sort\.\|big\.\|float64\|strconv.ParseFloat" internal/pages/option_ledger_text/format.go` prints 0; `grep -rln "internal/notify" internal/pages/option_ledger_text | wc -l | tr -d ' '` prints 0; `grep -c "option_ledger_text" .go-arch-lint.yml` is at least 1.
3. The diff lists no path outside the Files line.
Checked by CI: ci-go; `go run ./scripts/arch --check`.

### 11 optionledger-wire

Module: ot. Size: M.
Files: `cmd/ot/optionledger.go`, `cmd/ot/ledgerfile.go`, `cmd/ot/ledgerfile_test.go`, `cmd/ot/optionledger_test.go`, `cmd/ot/runonce.go`, `cmd/ot/runonce_test.go`, `cmd/ot/wire.go`, `cmd/ot/wire_test.go`, `cmd/ot/main.go`, `docs/contracts/ot.md`
Design: docs/design/option-ledger.md#decisions (Decisions 11, 16, 17)
Order: after 9, 10 and 561 `account-wire` (same `wire.go` and `runonce.go`).
Goal: wiring only. The registry entry `option_ledger` (constructor with `optionledger.Open` under `OT_PG_DSN_OPTION_LEDGER`, the account reader and the readers of 561 and 612), the loader of `ledger.toml` (Decision 11: `start_day` and the `[[event]]` tables, a bad value refused with the line named, with the TOML library already in `go.mod`), and `ot ledger [--alias a] [--status S] [--cycles]` printing through `option_ledger_text`. A missing DSN or file leaves the other jobs as they were and names the line.
Acceptance:
1. `go test ./cmd/ot -race -count=1` passes with `TestLedgerFileLoads`, `TestLedgerFileRefusesBadDecimal`, `TestLedgerFileRefusesUnknownKind`, `TestLedgerFileNoAccountId`, `TestOptionLedgerInRegistry`, `TestMissingLedgerDSNKeepsOtherJobs`, `TestLedgerCommandPrintsStored` each present once.
2. `grep -c "OT_PG_DSN_OPTION_LEDGER\|ot ledger\|ledger.toml" docs/contracts/ot.md` is at least 3; `grep -c "sort\.\|big\.\|float64" cmd/ot/optionledger.go` prints 0; `grep -c "notify\|telegram" cmd/ot/optionledger.go cmd/ot/ledgerfile.go | grep -vc ':0'` prints 0.
3. `grep -c "option_ledger" cmd/ot/runonce.go` is at least 1; `wc -l cmd/ot/wire.go cmd/ot/runonce.go cmd/ot/main.go | grep -v total | awk '$1>=800' | wc -l | tr -d ' '` prints 0; each new file is under 600 lines; the diff lists no path outside the Files line.
Checked by CI: `go test ./cmd/ot`, golangci-lint, `go run ./scripts/arch --check`. The owner runs `ot migrate`, `ot setup pg`, adds `OT_PG_DSN_OPTION_LEDGER` to `.env`, writes `ledger.toml` with `start_day`, then `ot run-once option_ledger`.

### 12 option-eod-held-all

Module: jobs. Size: S.
Files: `internal/jobs/optioneod/held.go`, `internal/jobs/optioneod/held_test.go`, `internal/jobs/optioneod/testdata/held_positions.json`
Design: docs/design/option-ledger.md#decisions (Decision 9)
Order: after 612 issue 9 `option-eod-held-calls`, which creates the held-position selection; this entry edits the same files (the names above are those the issue is expected to create; if they differ at cut time, the entry is re-cut with the real names, the change itself is the same).
Goal: the held-contract selection of 612 issue 9 covers every option position of the alias list, long or short, put or call, whatever its DTE, so that the ledger has a mark for each; the short calls behave as 612 specifies, byte for byte.
Acceptance:
1. `go test ./internal/jobs/optioneod -race -count=1` passes with `TestHeldIncludesShortPuts`, `TestHeldIncludesLongOptions`, `TestHeldShortCallsUnchanged`, `TestHeldIgnoresStockRows` each present once.
2. `git diff --name-only $BASE | grep -vxE 'internal/jobs/optioneod/(held|held_test)\.go|internal/jobs/optioneod/testdata/held_positions\.json' | wc -l | tr -d ' '` prints 0; each new file is under 600 lines.
Checked by CI: `go test ./internal/jobs/...`, go-arch-lint. Whether the gateway returns marks for every held contract is read from the first `ot ledger` output; it is not checkable offline.

## Checked by CI

- `go test ./internal/compute/... ./internal/store/... ./internal/jobs/... ./internal/pages/... ./internal/contracts/... ./cmd/ot -race -count=1` (ci-go: parallel, `t.TempDir()`, fake clock, no sleep).
- `go test -tags pg ./internal/store/...` (the `pg` job, GitHub CI only).
- `go run ./scripts/arch --check` printing `arch: ok` (issues 1, 8, 9, 10, 11); the generated-code check of `contracts/generate.sh` (issue 2); both run Go tools that the acceptance checker refuses, as in `account-data.md`.
- golangci-lint over the new packages; the design verdict of each issue.

## Left out

- Dividends, interest and fees outside commissions: a ledger of cash events needs Flex or `ot import`. A dividend received on the shares does not lower the net basis here.
- History before the start day, and an exact opening balance: the seed (Decision 5) uses the broker's average cost, which does not know premiums. Flex and the import of option-trader's history (the cut-over design, alignment Decision 3.8) are the fix; manual events are the stopgap.
- Flex as a source of assignments and expiries (a new outside access with a token): detection from captures is the stage-1 rule and the `unresolved` status is its honesty; if the owner chooses C in Q1, this design waits.
- Spreads and combos as one strategy, and rolls as a linked close and open: each leg is a position; `option-tracking` may link a roll by contract and day. Stock splits, corporate actions, early exercise of a long option by the owner (a manual event).
- FX: no conversion; a total is per alias and currency. Tax lots, wash sales and tax reports: the tax basis is the broker's, not tax advice.
- A household view pooling the two accounts per name (Q5); a figure with a register id: the page designs add register entries for what they show, as `ease-v2.md` did; here the rule text is in `docs/contracts/compute.md`.
- Retention of old runs (Decision 13: the whole replay is stored each run). A delete grant is refused; a later design may add a compaction by owner-run script.
- Pages: `option-tracking` (open positions, roll and close advice, over `OptionLedgerReader`) and `option-candidates-page` are their own designs. The margin cushion, assignment cash by date and local notices are `account-figures` and `option-notices`. Telegram text about the ledger: never.
- Rejected: a ledger computed in the page builder (pages compute nothing); an `UPDATE` of detected events when Flex lands (insert-only, a new run replaces the view); one four-module interface entry; reusing the name `Ledger` (taken by `OpsLedger`).
- Lock-ins, named: the alias is the identity of every row; `position_id` and `cycle_id` shapes are permanent keys of append-only tables; `rule_version` 1 fixes the basis rule until a new version row set is written.

## Open questions

Owner decisions, one letter each; the recommendation is marked.

1. Truth for assignment and expiry. A: detect them from consecutive captures (Decision 4) and let you correct with manual events in `ledger.toml` (recommended: no new access, honest `unresolved`). B: detection only, no manual file (drops issue 11's loader, keeps `ot ledger`). C: build nothing until Flex is in, then use its explicit assignment rows.
2. Which premiums lower the cost basis of assigned shares. A: the assigned put's premium and every later premium on the name while you hold the shares (recommended: the premium that actually bought the shares, conservative; the other number is stored as `premium_other`). B: every premium on the name since the shares were last flat, expired puts included.
3. Where history starts. A: at the day of the first account capture, positions seeded at the broker's average cost, with `premium_history_missing` marked (recommended: usable now). B: first import option-trader's history, so the first wheel basis is complete; the ledger shows no cost basis until that import design lands.
4. Mark for unrealized P&L on open options. A: the mid of the end-of-day bid and ask (recommended: neutral, matches the candidates design). B: the ask for a short, the bid for a long (what closing now would cost, harsher).
5. Household view. A: per account only, wheels run per account (recommended). B: also one pooled cycle per name across `a` and `b`, a new rule and dataset column set (a later issue, not in this design).

## Adversarial

Self-review pass of revision 1, each point and its outcome:

- Names: the word "ledger" is taken by `store.OpsLedger` and `OpenLedgerFunc`; every new name carries `Option` (`OptionLedger...`, schema `optionledger`, role `ot_w_optionledger`). Checked against `internal/store/api.go` on main.
- Interface order: issue 1 is the compute interface and comes first; the dataset (2) and store (3) interfaces precede their implementations (4 to 6 use only issue 1's types; 7 and 8 follow 3); the job (9) is declared with its implementation as 612 does for `candidates-job-puts`; no issue uses an interface that lands after it.
- Sizes: `internal/store/api_test.go` has 800 lines, so the store contract test is a new file; `internal/jobs/api.go` (76 lines), `internal/compute/api.go` (303), `internal/store/api.go` (490) stay far below 800, and acceptance checks the store one; every new file is checked under 600.
- Inputs that are not built yet: `AccountReader`, `account.*`, held option capture are in review (561, 612); every issue that needs them names the issue it follows. The marks of short puts wait for issue 12; this is stated in Decision 9 and the Modules text, not hidden.
- Honesty of detection: a missed capture day or a same-day open and close that leaves no position is `unresolved` or not detected, never guessed; unresolved rows are excluded from P&L and printed apart (tests named in issues 4 and 5).
- Unverified assumption, named: the IBKR average cost of an option is per contract including the multiplier; Decision 5 divides by it, a test pins it, and the first run is compared with the broker screen by the owner.
- Privacy: alias only, fixtures use `a` and `b`; `job.toml` and `ledger.toml` hold no account id (grep in issues 9 and 11); nothing reaches notify (grep in issues 9, 10, 11); the file contains no local path and no account id.
- Acceptance commands avoid the sandbox-refused tools (`go run ./scripts/arch`, generate) and put them under CI; each command prints a number or passes a named test; the arithmetic examples in issues 5 and 6 are checked by hand: 2 contracts x 100 x (1.50 - 0.40) = 220.00, minus 2.60 gives 217.40; strike 50 less premium 1.20 per share gives 48.80.
- Open risks left to the review: the merge order with 561, 606 and 612 over `setup.go`, `.go-arch-lint.yml`, `records.go` and the `cmd/ot` import sentence of `modules.md` (each entry says it rebases on whichever lands first); and the expected file names of issue 12.
