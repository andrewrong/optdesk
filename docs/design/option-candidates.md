# Option candidates: short puts and covered calls, generated from the stored chain, gated, ranked by annualized yield

Author: the optdesk designer (first draft)
Owner approved: (pending)
Date: 2026-11-09
Revision: 1

Source: owner 2026-11-09: options trading comes first and is done completely before valuation; the owner types a buy price per company by hand for now (the automated valuation, pull request 607, is parked). Binding owner decisions: a put qualifies when strike minus premium is at most the buy price; gates plus annualized-yield order, no composite score; earnings dates and ex-dividend dates gate; an early-assignment and ex-dividend guard for short calls; the margin cushion and the assignment cash never go to Telegram; two IBKR accounts, alias A and B.
Read on main at 4a73feb: `AGENTS.md`, `docs/design/modules.md`, `docs/design/ease-v2.md`, `docs/design/report-page-producer.md`, `docs/design/option-jobs.md`, `docs/design/option-pages-v22.md`, `contracts/datasets/obs.option_eod.r1.json`, `internal/jobs/api.go`, `internal/jobs/optioneod/{job.toml,select.go}`, `internal/store/api.go`, `internal/compute/api.go`, `arch.toml`. On branches in review: `docs/design/event-sources.md` (606), `docs/design/account-data.md` (561), `docs/design/valuation.md` (607, only its `watch.toml` lines).
Read in option-trader (read only; nothing copied): ADR 0009 (gates and order), 0011 and 0013 (capital), 0016 (earnings), 0020 (covered call rule), `candidates/puts.py` and `candidates/params/put_v1.toml`, `strategies/params/covered_call_v1.toml`. Plan v2.5 section 11, rows W8-9 (candidates, compute) and W11 (candidate pages, ledger, tracking).

Goal: after each option_eod capture, one stored, ranked list per day of the short puts and covered calls that the owner may sell, each with the reason of every gate it failed, plus a guard verdict for the short calls already open. The pages come later from this table.

## What main holds today

- Chain capture: the job `option_eod` writes `obs.option_eod` r1 (bid, ask, sizes, open interest, underlying price, IV, delta, per contract per session day); the terms (right, strike, expiry, multiplier) are in `store.OptionTermsReader`. Its manifest names one underlying, SPY, `max_expiries = 3` (the three nearest expiries) and `strike_band = 0.10`. For a seller that is too narrow twice: the nearest three expiries are weeklies under 21 days, and a strike 10 percent from the spot misses a put at the owner's buy price.
- Nothing computes a candidate: no package, dataset, job or reader. `compute-core` and `compute-figures` Left out ranks; the gap table calls the row "missing, silent drop".
- Not on main, in review: earnings and ex-dividend rows and `jobs.WatchEntry` with `$OT_HOME/watch.toml` (606), the account positions and values (561). Until they land the gates below answer `unknown`, which blocks (Decision 5): the table is right but empty-handed.

## Decisions

1. Where. A pure rule package `internal/compute/optioncand` (module compute: no I/O, no clock, exact arithmetic with `math/big.Rat` of the standard library, because go.mod holds no decimal library and the rule needs only exact sums and ratios; the package sits behind `compute.OptionCandidates`, so a library can replace it). A job `option_candidates` (module jobs) reads the stores, calls it and writes the dataset `candidates.option_candidate` r1 through `store.CandidateWriter`. Pages and `ot candidates` read through `store.CandidateReader`. Computed rows are stored, because the `pages` module computes nothing.
2. Owner-typed prices live in `watch.toml`. 606 owns that file and `WatchEntry` (one `[[underlying]]` per company, keyed by `issuer`, a typed field list read by a loader in `cmd/ot`). A field added to the same table is the smallest place: no second file, no second key, the owner edits one list, valuation reads the same file. `WatchEntry` gains `BuyPrice` and `SalePrice`, decimal text in the currency of the contract, both optional (`buy_price = "180.00"`). The type is declared by 606's `earnings-job`, so the interface issue is `after` it. When valuation lands it may fill a band elsewhere; these two stay the owner's word and win.
3. Which contracts. For every watched US underlying with a stored chain of the latest captured session day D, and every contract whose DTE (expiry minus D, calendar days) is in `min_dte..max_dte` (21..45, ported from `put_v1.toml`; 30..45 is preferred and only marks `preferred_dte`). Puts: strike below the spot (out of the money). Calls: strike at or above the spot. Japan: no option capture exists, so a JP entry gives no row; it is named, not skipped silently (Left out).
4. Short put gates, in this order; each is `pass`, `block` or `unknown`, and the row keeps the first reason code of each gate it did not pass:
   1. `buy_price`: `strike - bid <= BuyPrice` (the owner's rule, premium taken at the bid, per share, before fees). No `BuyPrice` is `block no_buy_price`.
   2. `quote`: row status ok, bid above 0, ask present, `bid_size >= min_bid_size`, market data type 1 or 2 (3 and 4 are `unknown`), spread `(ask - bid) / mid <= max_spread` and `open_interest >= min_open_interest` (the two limits are advisor-set, below).
   3. `events`: next results date from the 606 earnings reader. A `listed` date from D to the expiry, both included, is `block earnings_before_expiry`. A `listed` date after the expiry passes and sets the mark `earnings_unconfirmed` (606 Decision 4: a vendor date refuses or marks, never clears). `absent`, `unsupported`, `error`, a missing row, a row more than 2 days old and an expiry beyond the 3-month horizon of the source are `unknown`. The owner's `report_date_override` replaces the vendor date as 606 has it.
   4. `funding`, per account alias: `strike x multiplier` (one contract) is at most `AvailableFunds` of that alias from the latest `account.value` capture, in the same currency; a capture older than one session day is `unknown`. The row stores `fits` (alias to `fit`, `no_fit` or `unknown`); the gate passes when at least one alias is `fit`. Cash for puts already open and concentration caps are Left out.
   5. `cash_rate`: annualized yield (Decision 6) at least the stored `DGS3MO` of at most 7 days old (ported from ADR 0009 gate 6), else `block yield_below_cash`; no fresh rate is `unknown`.
5. Unknown is not a pass. A gate that cannot decide gives `unknown`, and the row's status is `blocked` if any gate blocked, else `unknown` if any gate is unknown, else `ok`. Only `ok` rows are ranked. The old system's rule (ADR 0009: "unknown earnings gives UNKNOWN, not PASS") and 606 Decision 4 both say so.
6. Ranking, no composite score. Annualized yield `= (bid / strike) x (365 / DTE)` for a put, `(bid / spot) x (365 / DTE)` for a call, a ratio kept exact and stored as decimal text to 6 places. Order of `ok` rows: annualized yield descending, then DTE ascending, then strike ascending, then instrument id. `rank` is the position in the whole day, `rank_in_underlying` within a company. Also stored, never ranked: break-even (put `strike - bid`, call `strike + bid`), delta, IV, `preferred_dte`. Blocked and unknown rows are stored too, with their reasons, so the owner sees why a contract failed; they have no rank.
7. Covered calls. Shares are the `account.position` stock rows of the underlying, per alias, at the latest capture: free shares = shares minus `multiplier x` the open short calls on the same underlying. Gates in order: `shares` (free shares at least one multiplier in some alias, else `block no_free_shares`); `strike_floor` (`strike >= SalePrice` when the owner set one, else `strike >= avg_cost` of the shares in that alias, per share; an alias with neither is `unknown`); `quote`, `events` as for puts (the owner's decision is that earnings gate; ADR 0020 only marked them); `guard` (Decision 8). A call row carries the alias it would be written against, so a company held in A and B has one row per alias.
8. Early-assignment and ex-dividend guard. For a call with an ex-dividend date X from D to the expiry (606 stores the next ex-date and the amount per share; IBKR does not say declared or projected, so every date is projected): `extrinsic = max(0, bid - max(0, spot - strike))`; `extrinsic < amount x div_guard_factor` (1.0, advisor-set) is `block early_assignment_risk`, because holders exercise the day before the ex-date. A `no_value` dividend row passes with the mark `no_dividend`; `error`, `unsupported`, no row, or a row older than 3 days is `unknown`. The same function judges the short calls already open (rows of kind `held_call_guard`, per alias, from the positions): verdict `risk`, `ok` or `unknown`, `unknown` when the held contract has no capture that day. The guard rows are local: they are the input of a later notice, never of Telegram.
9. Privacy. `fits`, `alias`, shares, and the guard rows come from account data. They are stored in one table readable only by the roles of the pages and `ot candidates` (local), the job never calls `notify`, and a test pins that no code path from this package reaches a Telegram sender (ADR 0010 stands; margin cushion and assignment cash have no row here at all, they are `account-figures`, below).
10. Advisor-set values (the owner may change them; they are keys of `job.toml`, not code): `min_dte 21`, `max_dte 45`, `min_bid_size 1`, `max_spread 0.10`, `min_open_interest 10`, `div_guard_factor 1.0`, `max_rate_age_days 7`, `max_event_age_days 2`. The first three are ported from `put_v1.toml`; the rest are new.
11. Chain coverage. The `option_eod` manifest cannot feed these gates (Current state). Entry 8 gives it the watch list as underlyings and a DTE window (`min_dte`, `max_dte`) in place of the nearest expiries, and a wider strike band. The job `option_candidates` runs at `session_close` plus 30 minutes, after `option_eod` (its freeze is at 16:15).

## The option side, whole

What the owner needs to run options completely, with its design and state. This table is the input for sequencing; a row marked missing names the next design.

| piece | design | state |
|---|---|---|
| capture of the option chain | `option-jobs.md`, job `option_eod` | merged; SPY only and nearest expiries: fixed by `option-eod-watch` of this design |
| earnings and ex-dividend dates | `event-sources.md` | in review PR 606 |
| watch list and the owner's buy and sale price | `event-sources.md` (list), this design (two fields) | list in PR 606; fields this design |
| account positions, values, executions (accounts A and B) | `account-data.md` | in review PR 561; collector pyside and the Go client merged (`pyside-account-b.md`) |
| candidates, gates, ranking | this design | this design |
| early-assignment and ex-dividend guard, held calls too | this design (Decision 8) | this design |
| ledger, cost basis, wheel (assigned shares, then calls) | none | missing; next design `option-ledger`: needs `account.execution` history and assignment rows; waits for PR 561 and for `ot import` or Flex (561 Left out) |
| assignment cash by date per account | `account-data.md` Decision 9 names it | missing; next design `account-figures` (assignment cash, margin cushion from `account.value`), needs PR 561 |
| tracking of open positions, P and L, roll and close advice | `tracking-household-reports.md` is the view only | missing builder and data; next design `option-tracking`, needs `option-ledger` |
| alerts: market and missed-job on Telegram | `risk-alerts.md`, `notify.md` | merged (market only, ADR 0010 amended) |
| local notice for the guard, the margin cushion, assignment cash | none | missing; next design `option-notices` (a page banner or local notice, never Telegram), needs `account-figures` |
| pages: candidates (`option_page`, `stock_option_page`), positions, ledger, tracking, structures | `option-pages-v22.md` (views), `tracking-household-reports.md` | views merged, builders missing; next designs `option-candidates-page` (right after this design), then the others |
| valuation and the automatic buy price | `valuation.md` | parked, PR 607; the hand-typed buy price replaces it for now |

## Modules

New directories: `internal/compute/optioncand`, `internal/jobs/optioncandidates`, `internal/store/candidate`. This pull request edits `docs/design/modules.md` by three names in the sentence that lists the implementation packages `cmd/ot` may import and regenerates `arch.toml` with `go run ./scripts/arch` (the precedent is `internal/compute/ease`). Interface files: `internal/compute/api.go`, `internal/store/api.go`, `internal/jobs/api.go` (each additive), the dataset in `contracts/`.

| # | issue | module | order |
|---|---|---|---|
| 1 | watch-owner-prices-interface (interface) | jobs | after event-sources `earnings-job` |
| 2 | watch-owner-prices-loader | ot | after 1, event-sources `watch-loader-earnings-wire` |
| 3 | option-eod-watch (interface and implementation) | jobs | after 1 |
| 4 | candidate-compute-interface (interface) | compute, plus repo (the licensed pair) | first |
| 5 | compute-candidates | compute | after 4 |
| 6 | candidate-dataset (interface) | contracts | after account-data `account-proto`, event-sources `earnings-datasets` |
| 7 | candidate-store-api (interface) | store | after 6, account-data `account-store-api` |
| 8 | candidate-store-impl | store, plus repo (the licensed pair) | after 7, account-data `account-store-migration`, event-sources `earnings-store` |
| 9 | candidates-job (interface and implementation) | jobs, plus repo (the licensed pair) | after 1, 3, 5, 8, event-sources `dividend-store`, account-data `account-store-impl` |
| 10 | candidates-wire | ot | after 9, event-sources `watch-loader-earnings-wire`, account-data `account-wire` |

## Interface

- jobs, `internal/jobs/api.go`: `WatchEntry` (declared by 606) gains `BuyPrice, SalePrice string` (decimal text, empty when unset); `type NewOptionEodWatchFunc func(broker.Broker, store.InstrumentResolver, store.OptionEodWriter, []WatchEntry, clock.Clock) (contracts.Job, error)` (a second constructor, so `NewOptionEodFunc` and `cmd/ot` keep building, the pattern of `NewComputeFiguresEaseFunc`); `type NewOptionCandidatesFunc func(CandidateStores, compute.OptionCandidates, compute.Rules, []WatchEntry, clock.Clock) (contracts.Job, error)` with `CandidateStores{OptionEod store.OptionEodReader; Terms store.OptionTermsReader; Names store.InstrumentNameReader; Catalog store.OptionEodCatalog; Series store.SeriesReader; Earnings store.EarningsDateReader; Dividends store.DividendReader; Accounts store.AccountReader; Candidates store.CandidateWriter; SessionDays store.SessionDayReader}`. A nil `Earnings`, `Dividends` or `Accounts` makes its gates `unknown`.
- compute, `internal/compute/api.go`: `type CandidateKind string` (`short_put`, `covered_call`, `held_call_guard`); `type GateStatus string` (`pass`, `block`, `unknown`); `type GateResult struct{ Gate string; Status GateStatus; Reason string }`; `type OptionQuote` (terms and quote of one contract, decimals as text); `type CandidateInput` (day, underlying, spot, quotes, `BuyPrice`, `SalePrice`, earnings, dividend, rate, per-alias shares, avg cost and funds, open short calls); `type Candidate` (kind, alias, instrument, the gate results in order, status, marks, annualized yield, break-even, `Rank`, `RankInUnderlying`); `type CandidateParams` (the keys of Decision 10); `type OptionCandidates interface { Evaluate(CandidateInput, CandidateParams) []Candidate }` and `type NewOptionCandidatesFunc func() OptionCandidates`. Pure: no I/O, no clock.
- contracts: dataset `candidates.option_candidate` r1: key `(session_day, capture_id, kind, instrument_id, alias)`; columns `issuer, underlying_id, expiry, dte, right, strike, multiplier, bid, ask, spot, buy_price, sale_price, break_even, annualized_yield, delta, iv, status, reasons` (the gate results as ordered text), `marks`, `fits` (alias map as text), `next_earnings, earnings_state, next_ex_date, ex_amount, extrinsic, rank, rank_in_underlying, rule_version, computed_at`; money as decimal text; `alias` is empty for puts.
- store: `type CandidateWriter interface { InsertCandidates(ctx, []Candidate) (int, error) }`, `type CandidateReader interface { Candidates(ctx, day contracts.Day, kind string) ([]Candidate, error) }` (ordered by `rank`, rows without rank last by instrument id), `store.DSNEnvCandidate = "OT_PG_DSN_CANDIDATE"`; row type `store.Candidate` is a plain struct of the dataset columns.
- ot: `ot candidates [--day D] [--kind short_put|covered_call|held_call_guard]` prints the ranked rows as text, local only.

## Issues

Every test calls `t.Parallel()`, writes files only under `t.TempDir()`, uses a fake clock, never sleeps, opens no port, and no test touches a live broker or database (a test that needs PostgreSQL carries the tag `pg`). Acceptance is checked offline by the command shown.

### 1 watch-owner-prices-interface (interface)

Module: jobs. Size: S.
Files: `internal/jobs/api.go`, `internal/jobs/api_test.go`, `docs/contracts/jobs.md`
Design: docs/design/option-candidates.md#decisions (Decision 2)
Order: after event-sources `earnings-job` (it declares `WatchEntry` in the same files).
Goal: `WatchEntry` gains `BuyPrice` and `SalePrice` with doc comments (decimal text, currency of the contract, empty means unset, the owner's word wins over any automatic value), and the contract test shows a caller reading both from a fake loader.
Acceptance:
1. `grep -c "BuyPrice\|SalePrice" internal/jobs/api.go` is at least 2, and `git diff $(git merge-base HEAD origin/main) -- internal/jobs/api.go | grep '^-' | grep -vc '^---'` prints 0 (additions only).
2. `go test ./internal/jobs -race -count=1 -run WatchEntry` passes; `grep -c "t.Parallel()" internal/jobs/api_test.go` is at least `grep -c "^func Test" internal/jobs/api_test.go`.
3. `grep -c "buy_price\|BuyPrice" docs/contracts/jobs.md` is at least 1.
4. `git diff --name-only $(git merge-base HEAD origin/main)` lists no path outside the Files line.
Checked by CI: `go build ./...`, golangci-lint, go-arch-lint.

### 2 watch-owner-prices-loader

Module: ot. Size: S.
Files: `cmd/ot/watch.go`, `cmd/ot/watch_test.go`, `cmd/ot/testdata/watch_prices.toml` (new), `docs/contracts/ot.md`
Design: docs/design/option-candidates.md#decisions (Decision 2)
Order: after watch-owner-prices-interface and event-sources `watch-loader-earnings-wire` (it creates `cmd/ot/watch.go`).
Goal: the loader of `watch.toml` reads `buy_price` and `sale_price` per `[[underlying]]`; a value that is not a positive decimal with at most 4 places, or a `sale_price` below a `buy_price` of the same entry, is a named refusal that names the issuer and never the file path of the owner.
Acceptance:
1. `go test ./cmd/ot -race -count=1 -run 'TestWatchPrices'` passes; `grep -c "^func TestWatchPrices" cmd/ot/watch_test.go` is at least 4 (loaded, empty is unset, refused text, sale below buy).
2. `grep -c "buy_price\|sale_price" docs/contracts/ot.md` is at least 2.
3. `git diff --name-only $(git merge-base HEAD origin/main)` lists no path outside the Files line.
Checked by CI: `go test ./cmd/ot`, golangci-lint.

### 3 option-eod-watch (interface and implementation)

Module: jobs. Size: M.
Files: `internal/jobs/api.go`, `internal/jobs/api_test.go`, `internal/jobs/optioneod/job.toml`, `internal/jobs/optioneod/select.go`, `internal/jobs/optioneod/select_test.go`, `internal/jobs/optioneod/manifest.go`, `internal/jobs/optioneod/plan.go`, `internal/jobs/optioneod/plan_test.go`, `internal/jobs/optioneod/run.go`, `internal/jobs/optioneod/run_test.go`, `internal/jobs/optioneod/watch.go`, `internal/jobs/optioneod/watch_test.go`, `docs/contracts/jobs.md`
Design: docs/design/option-candidates.md#decisions (Decision 11)
Order: after watch-owner-prices-interface (same `api.go`, `api_test.go`, `jobs.md`).
Goal: `optioneod.NewWithWatch` (the type `NewOptionEodWatchFunc`) takes the underlyings from the `WatchEntry` list (US stock entries only) instead of the manifest table; the manifest gains `min_dte` and `max_dte` (expiries inside the window replace `max_expiries`; absent keys keep today's rule byte for byte) and the band default stays `0.10` with a per-entry override key `strike_band`; `optioneod.New` and `NewOptionEodFunc` are unchanged.
Acceptance:
1. `go test ./internal/jobs/optioneod -race -count=1` passes with `TestWatchUnderlyingsUSOnly`, `TestDTEWindowSelectsMonthlies`, `TestNoWindowKeepsNearestExpiries`, `TestStrikeBandOverride` each present once (`grep -c` of each name is 1).
2. `git diff $(git merge-base HEAD origin/main) -- internal/jobs/api.go | grep '^-' | grep -vc '^---'` prints 0; `grep -c "NewOptionEodWatchFunc" internal/jobs/api.go` prints 1.
3. `grep -c "min_dte\|max_dte" docs/contracts/jobs.md` is at least 2.
4. `git diff --name-only $(git merge-base HEAD origin/main)` lists no path outside the Files line; each new file is under 600 lines.
Checked by CI: `go test ./internal/jobs/...`, go-arch-lint. Whether the real gateway returns the monthly expiries and the deeper strikes is not checkable offline; the first run is read with `ot candidates`.

### 4 candidate-compute-interface (interface)

Module: compute, plus repo (the licensed pair: `.go-arch-lint.yml`). Size: S.
Files: `internal/compute/api.go`, `internal/compute/api_test.go`, `docs/contracts/compute.md`, `internal/compute/optioncand/doc.go`, `.go-arch-lint.yml`
Design: docs/design/option-candidates.md#interface
Order: first.
Goal: the compute types of `## Interface`, with doc comments that say what each gate word and reason code means, and one paragraph of `docs/contracts/compute.md` that fixes the promises: exact arithmetic, unknown is not a pass, only `ok` rows ranked, yield and break-even formulas. The contract test uses a fake `OptionCandidates` and a caller that checks the order of the gate results and that a `blocked` row has no rank.
Acceptance:
1. `grep -c "OptionCandidates\|CandidateInput\|CandidateParams\|GateResult" internal/compute/api.go` is at least 4; `git diff $(git merge-base HEAD origin/main) -- internal/compute/api.go | grep '^-' | grep -vc '^---'` prints 0.
2. `go test ./internal/compute -race -count=1` passes; `grep -c "time.Sleep\|time.Now()" internal/compute/api_test.go` prints 0; `wc -l < internal/compute/api_test.go` prints less than 800 (it is 464 on main; past 800 the contract test would move to a new file).
3. `grep -c "annualized\|break-even" docs/contracts/compute.md` is at least 2; `go list -deps ./internal/compute | grep -c "internal/compute/optioncand"` prints 0.
4. `go run ./scripts/arch --check` prints `arch: ok`; `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the Files line.
Checked by CI: ci-go, `go run ./scripts/arch && git diff --exit-code arch.toml .go-arch-lint.yml`.

### 5 compute-candidates

Module: compute. Size: L.
Files: `internal/compute/optioncand/optioncand.go`, `internal/compute/optioncand/put.go`, `internal/compute/optioncand/call.go`, `internal/compute/optioncand/events.go`, `internal/compute/optioncand/guard.go`, `internal/compute/optioncand/rank.go`, `internal/compute/optioncand/exact.go`, `internal/compute/optioncand/put_test.go`, `internal/compute/optioncand/call_test.go`, `internal/compute/optioncand/events_test.go`, `internal/compute/optioncand/guard_test.go`, `internal/compute/optioncand/rank_test.go`, `internal/compute/optioncand/exact_test.go`, `internal/compute/optioncand/testdata/`
Design: docs/design/option-candidates.md#decisions (Decisions 3 to 8)
Order: after candidate-compute-interface.
Goal: `New()` returns an `OptionCandidates` that applies the gates of Decisions 4, 7 and 8 in order, the status rule of Decision 5 and the ranking of Decision 6, on `math/big.Rat`, with no float on a price or a yield.
Acceptance:
1. `grep -rhE "^func (TestBuyPriceBoundary|TestNoBuyPriceBlocks|TestQuoteGate|TestEarningsListedInsideBlocks|TestEarningsAfterExpiryMarks|TestEarningsUnknownBlocks|TestFundingPerAlias|TestCashRateStale|TestUnknownIsNotPass|TestRankByYield|TestRankTieOrder|TestBlockedHasNoRank|TestCallFreeShares|TestCallStrikeFloorAvgCost|TestCallStrikeFloorSalePrice|TestGuardExtrinsicBelowDividend|TestGuardNoValuePasses|TestGuardHeldCall|TestDeterministic)\(" internal/compute/optioncand | wc -l` prints 19, and `go test ./internal/compute/optioncand -race -count=1` passes.
2. `TestBuyPriceBoundary`: strike 100, bid 2.00, buy price 98.00 passes and bid 1.99 blocks (the boundary is inclusive, exact). `TestGuardExtrinsicBelowDividend`: spot 110, strike 100, bid 10.40, dividend 0.50 gives extrinsic 0.40 and blocks; bid 10.60 passes.
3. `grep -rc "float64\|float32\|time.Now()\|time.Sleep\|os\.\|net/http\|database/sql" internal/compute/optioncand/*.go | grep -v '_test.go' | grep -v ':0$'` prints nothing; `go list -deps ./internal/compute/optioncand | grep -c "internal/store\|internal/notify\|internal/jobs"` prints 0.
4. `TestDeterministic` runs the same input shuffled 20 times (seed fixed in the test) and requires identical output; `grep -c "t.Parallel()"` of each test file is at least its count of `^func Test`.
5. `git diff --name-only $(git merge-base HEAD origin/main)` lists no path outside the Files line; each new file is under 600 lines.
Checked by CI: ci-go, golangci-lint, go-arch-lint. Whether the advisor-set limits (Decision 10) suit the owner is decided by reading the first stored day.

### 6 candidate-dataset (interface)

Module: contracts. Size: M.
Files: `contracts/datasets/candidates.option_candidate.r1.json`, `contracts/generate.sh`, `internal/contracts/gen/records.go`, `internal/contracts/api.go`, `docs/contracts/datasets/candidates.option_candidate.md`, `contracts/fixtures/candidates.option_candidate.r1.json`, `contracts/fixtures/MANIFEST.synthetic-v1.json`, `internal/contracts/schema_test.go`
Design: docs/design/option-candidates.md#interface
Order: after account-data `account-proto` and event-sources `earnings-datasets` (the same generated files; `contracts/generate.sh` and `records.go`).
Goal: the dataset schema of the Interface section with the generated Go record and its document, committed together; the fixture is marked synthetic.
Acceptance:
1. `test -f contracts/datasets/candidates.option_candidate.r1.json`, and `bash contracts/generate.sh && git diff --exit-code internal/contracts/gen` leaves no change.
2. `go test ./internal/contracts -race -count=1` passes with the fixture validated; `grep -c "candidates.option_candidate" contracts/fixtures/MANIFEST.synthetic-v1.json` is at least 1.
3. `grep -c '"required"' contracts/datasets/candidates.option_candidate.r1.json` is at least 1 and the key array equals `["session_day","capture_id","kind","instrument_id","alias"]`.
4. `git diff --name-only $(git merge-base HEAD origin/main)` lists no path outside the Files line.
Checked by CI: the generate check of `checks.yml`, `Contracts: changed` in the pull request body (a file under `contracts/datasets`).

### 7 candidate-store-api (interface)

Module: store. Size: S.
Files: `internal/store/api.go`, `internal/store/api_candidate_test.go`, `docs/contracts/store.md`
Design: docs/design/option-candidates.md#interface
Order: after candidate-dataset, and after account-data `account-store-api` and event-sources `earnings-store` (the same `api.go`, `store.md`).
Goal: `CandidateWriter`, `CandidateReader`, `store.Candidate`, `DSNEnvCandidate`, declarations only. `internal/store/api_test.go` is 800 lines and is not edited; the contract test is the new file `internal/store/api_candidate_test.go`, a fake on one side and a caller that reads a day and sees ranked rows before unranked ones.
Acceptance:
1. `grep -c "type CandidateWriter interface\|type CandidateReader interface" internal/store/api.go` prints 2; `git diff $(git merge-base HEAD origin/main) -- internal/store/api.go | grep '^-' | grep -vc '^---'` prints 0; `grep -c "database/sql\|pgx" internal/store/api.go` equals its count on main.
2. `go test ./internal/store -run TestCandidateContract -race -count=1` passes; `git diff --name-only $(git merge-base HEAD origin/main)` does not list `internal/store/api_test.go`.
3. `grep -c "OT_PG_DSN_CANDIDATE\|CandidateReader" docs/contracts/store.md` is at least 2.
Checked by CI: ci-go, go-arch-lint.

### 8 candidate-store-impl

Module: store, plus repo (the licensed pair: `.go-arch-lint.yml`). Size: M.
Files: `migrations/NNNN_candidates.sql` (NNNN the next free number at cut time), `internal/store/candidate/doc.go`, `internal/store/candidate/store.go`, `internal/store/candidate/read.go`, `internal/store/candidate/store_test.go`, `internal/store/candidate/store_pg_test.go` (tag `pg`), `internal/store/setup/setup.go`, `internal/store/setup/setup_test.go`, `internal/store/setup/candidate_pg_test.go` (tag `pg`), `.go-arch-lint.yml`
Design: docs/design/option-candidates.md#decisions (Decisions 1 and 9)
Order: after candidate-store-api; after account-data `account-store-migration` and event-sources `earnings-store` (same `setup.go`; the migration number is the next free one at cut time).
Goal: a table `candidates.option_candidate` with the key of the dataset, `ON CONFLICT DO NOTHING`, the role `ot_w_candidate` (INSERT and SELECT only) and `SELECT` for the role of the pages; `candidate.Open(ctx, dsn)` implements both interfaces; the reader orders ranked rows first.
Acceptance:
1. `grep -c "CREATE TABLE candidates.option_candidate\|ot_w_candidate" migrations/NNNN_candidates.sql` is at least 2 and `grep -c "UPDATE\|DELETE" migrations/NNNN_candidates.sql` prints 0 (no grant of them).
2. `go test ./internal/store/candidate ./internal/store/setup -race -count=1` passes without a database (query text, row mapping, an empty day gives an empty slice, a reversed or zero day is an error).
3. `grep -rc "OT_PG_DSN_CANDIDATE\|os.Getenv" internal/store/candidate/*.go | grep -v ':0$'` prints nothing (the DSN is passed in).
4. `grep -c "^func TestCandidateRoundTrip\|^func TestCandidateConflictKeepsFirst" internal/store/candidate/store_pg_test.go` prints 2 and the file starts with `//go:build pg`; `git diff --name-only $(git merge-base HEAD origin/main)` lists no path outside the Files line.
Checked by CI: the two `pg` tests run only in GitHub CI; `go run ./scripts/arch --check`.

### 9 candidates-job (interface and implementation)

Module: jobs, plus repo (the licensed pair: `.go-arch-lint.yml`). Size: L.
Files: `internal/jobs/api.go`, `internal/jobs/api_test.go`, `internal/jobs/optioncandidates/job.go`, `internal/jobs/optioncandidates/job.toml`, `internal/jobs/optioncandidates/input.go`, `internal/jobs/optioncandidates/schedule.go`, `internal/jobs/optioncandidates/job_test.go`, `internal/jobs/optioncandidates/input_test.go`, `internal/jobs/optioncandidates/schedule_test.go`, `internal/jobs/optioncandidates/testdata/`, `docs/contracts/jobs.md`, `.go-arch-lint.yml`
Design: docs/design/option-candidates.md#decisions (Decisions 1, 9, 10, 11)
Order: after watch-owner-prices-interface, option-eod-watch (same `api.go`, `api_test.go`, `jobs.md`), compute-candidates, candidate-store-impl, event-sources `dividend-store`, account-data `account-store-impl` (`.go-arch-lint.yml`: this entry rebases on the last of them).
Goal: `optioncandidates.New` (the type `NewOptionCandidatesFunc`) runs at `session_close` plus 30 minutes; for each watched US underlying it reads the latest option_eod day, the terms, the earnings and dividend rows, the newest account captures per alias and `DGS3MO`, builds the `CandidateInput`, calls `Evaluate` and writes the rows once per capture id (a rerun adds none). A nil reader gives `unknown` gates, not an error; a store error is a failed run with a named code.
Acceptance:
1. `go test ./internal/jobs/optioncandidates -race -count=1` passes with `TestNilEarningsIsUnknown`, `TestRerunWritesNothingNew`, `TestOnlyUSEntries`, `TestAliasesAandBAreSeparateRows`, `TestStoreErrorFailsRun`, `TestScheduleIsCloseAfterEod` each present once.
2. `grep -rc "internal/notify\|telegram" internal/jobs/optioncandidates | grep -v ':0$'` prints nothing; `go list -deps ./internal/jobs/optioncandidates | grep -c "internal/notify"` prints 0.
3. `grep -c "min_dte\|max_dte\|min_bid_size\|max_spread\|min_open_interest\|div_guard_factor\|max_rate_age_days\|max_event_age_days" internal/jobs/optioncandidates/job.toml` is 8.
4. `git diff $(git merge-base HEAD origin/main) -- internal/jobs/api.go | grep '^-' | grep -vc '^---'` prints 0; `git diff --name-only $(git merge-base HEAD origin/main)` lists no path outside the Files line; each new file is under 600 lines.
Checked by CI: ci-go, go-arch-lint, `go run ./scripts/arch --check`.

### 10 candidates-wire

Module: ot. Size: M.
Files: `cmd/ot/candidates.go`, `cmd/ot/candidates_test.go`, `cmd/ot/runonce.go`, `cmd/ot/runonce_test.go`, `cmd/ot/wire.go`, `cmd/ot/wire_test.go`, `cmd/ot/scheduler.go`, `cmd/ot/main.go`, `cmd/ot/env.go`, `docs/contracts/ot.md`
Design: docs/design/option-candidates.md#interface
Order: after candidates-job, event-sources `watch-loader-earnings-wire`, account-data `account-wire` (the same `runonce.go`, `wire.go`, `env.go`).
Goal: `option_candidates` registers in `run-once` and the scheduler, and `option_eod` uses `NewWithWatch` with the loaded list; a missing `OT_PG_DSN_CANDIDATE` disables only this job with a named line; `ot candidates` prints the ranked `ok` rows (company, kind, alias, expiry, strike, bid, annualized yield, break-even, marks) and, with `--all`, the blocked rows with their reasons. Output is local text, written to stdout only.
Acceptance:
1. `go test ./cmd/ot -race -count=1` passes with `TestCandidatesCommandRanked`, `TestCandidatesAllShowsReasons`, `TestCandidatesRegistered`, `TestMissingCandidateDSNDisablesOnlyIt` each present once.
2. `grep -c "OT_PG_DSN_CANDIDATE\|ot candidates" docs/contracts/ot.md` is at least 2.
3. `grep -rc "notify\|telegram" cmd/ot/candidates.go` prints 0.
4. `git diff --name-only $(git merge-base HEAD origin/main)` lists no path outside the Files line; each new file is under 600 lines.
Checked by CI: `go test ./cmd/ot`, golangci-lint. The owner runs `ot migrate`, `ot setup pg` and adds `OT_PG_DSN_CANDIDATE` to `.env` before the first run.

## Left out

- Pages: `option_page` and `stock_option_page` builders are the design `option-candidates-page`; until then `ot candidates` is the view.
- Ledger, cost basis, wheel, assignment cash, margin cushion, tracking, local notices: the next designs of the table above; none is a row here.
- Funding by open puts and concentration caps (ADR 0011, 0013), the 13-week-rate gate beyond `DGS3MO`, fees, the ITM probability N(-d2), the volatility spread and IV rank (they need the bars design), spreads, naked short calls, rolls: each its own design, named so that none is silently dropped. The register ids `option.candidates.*` of the old register (earnings date, IV context, ITM probability, excess yield) are not ported here; annualized yield and break-even are plain arithmetic documented in `docs/contracts/compute.md`.
- Japan: no option capture and no JP earnings or dividend rows in the cut; a JP entry yields no row.
- An owner acknowledgement that clears an earnings block (the old system's ADR 0016): the owner's tool is the `report_date_override` of 606 and, to sell through results, deleting the date; a separate acknowledgement record is not built.
- Rejected: a composite score (owner decision); computing candidates in the page builder (pages compute nothing); a new file for the prices (606's `watch.toml` already is the owner's list).

## What the owner gets

- You type one or two prices per company in the file you already keep, `watch.toml`: `buy_price` (the most you would pay for the shares) and, if you like, `sale_price` (the least you would sell them for).
- After each close the tool lists the puts you could sell, best annualized yield first. A put is on the list only when strike minus the premium is at or below your buy price, the quote is real, no results date falls before expiry, the cash fits in account A or B, and the yield beats the Treasury bill rate. Anything it cannot check counts as not cleared and shows why.
- Covered calls on shares you hold are listed the same way, per account, with the strike at or above your sale price (or your cost) and a warning when an ex-dividend date makes early assignment likely. Calls you already hold are checked against the same warning.
- You read it with `ot candidates`, on your own machine. Nothing about accounts, cash or margin goes to Telegram, ever.
- Before it shows anything real you add `OT_PG_DSN_CANDIDATE` to `.env` and run `ot migrate` and `ot setup pg`; the earnings and dividend sources of design 606 and the account data of 561 must be in first, or every row says unknown.

## Adversarial

Pending. The adversarial review of revision 1 comes before the design review; its findings and the answers go here.
