# Option candidates: short puts first, then covered calls and the ex-dividend guard, generated from the stored chain, gated, ranked by annualized yield

Author: the optdesk designer (draft)
Owner approved: owner answers 2026-11-09 (the four questions); the pull request approval is pending
Date: 2026-11-09
Revision: 3 (owner answers of 2026-11-09 applied; revision 2 was 186019a)

Source: owner 2026-11-09: options trading comes first and is done completely before valuation; the owner types a buy price per company by hand for now (the automated valuation, pull request 607, is parked). Binding owner decisions: a put qualifies when strike minus premium is at most the buy price; gates plus annualized-yield order, no composite score; earnings dates and ex-dividend dates gate; an early-assignment and ex-dividend guard for short calls; the margin cushion and the assignment cash never go to Telegram; two IBKR accounts, alias A and B. Dispatcher direction: puts first, they do not wait for the dividend chain; the option side must be usable soon.
Read on main at 4a73feb: `AGENTS.md`, `docs/design/modules.md`, `docs/design/ease-v2.md`, `docs/design/report-page-producer.md`, `docs/design/option-jobs.md`, `docs/design/option-pages-v22.md`, `contracts/datasets/obs.option_eod.r1.json`, `internal/jobs/api.go`, `internal/jobs/optioneod/{job.toml,select.go}`, `internal/store/api.go`, `internal/compute/api.go`, `arch.toml`, `contracts/proto/ibkr/v1`. On branches in review: `docs/design/event-sources.md` (606), `docs/design/account-data.md` (561), `docs/design/valuation.md` (607, only its `watch.toml` lines).
Read in option-trader (read only; nothing copied): ADR 0004 (OSE.JPN routing), 0009 (gates and order), 0011 and 0013 (capital), 0016 (earnings), 0020 (covered call rule), `candidates/puts.py`, `candidates/params/put_v1.toml`, `put_jp_v1.toml`, `strategies/params/covered_call_v1.toml`. Plan v2.5 section 11, rows W8-9 (candidates) and W11 (candidate pages, ledger, tracking).

Goal: after each option_eod capture, one stored, ranked list per day of the short puts the owner may sell, each blocked contract with the reason of every gate it failed; later the covered calls and a guard verdict for the short calls already open. A page reads the table (next design, named below).

## What main holds today

- Chain capture: `option_eod` writes `obs.option_eod` r1 (bid, ask, sizes, open interest, underlying price, IV, delta; `*float64` in the generated record); the terms are in `store.OptionTermsReader` (at most 500 ids per call). Its manifest names SPY only, `max_expiries = 3` (the three nearest expiries, weeklies under 21 days), `strike_band = 0.10` and `max_strikes_per_side = 8`: too narrow for a seller, whose put sits at the buy price, often 15 to 25 percent below the spot.
- Nothing computes a candidate: no package, dataset, job or reader (gap table: "missing, silent drop").
- In review: earnings and ex-dividend rows with `jobs.WatchEntry` and `$OT_HOME/watch.toml` (606), the account positions and values (561). `DGS3MO` is loaded by `fred_series` (unit `percent`).

## Decisions

1. Where. A pure rule package `internal/compute/optioncand` (no I/O, no clock) behind `compute.OptionCandidates`, so a library can replace it. Exact arithmetic with `math/big.Rat` of the standard library, because go.mod holds no decimal library and the rule needs only exact sums and ratios. A job `option_candidates` (jobs) reads the stores, calls the rule and writes `candidates.option_candidate` r1 through `store.CandidateWriter`. Computed rows are stored because pages compute nothing. Typed columns, not one JSON payload (revision 1 review, section 3): the page and `ot candidates` read columns; the gate results are one ordered JSON array text.
2. Put first, calls later. The first cut (issues 1 to 8) is puts: it needs `WatchEntry` and the earnings reader of 606, the account reader of 561 for the funding gate, and nothing of the dividend chain or its probe. Calls and the guard are issues 9 to 12, ordered after 606 `dividend-store`; puts never wait for them. Until a reader exists its gate answers `unknown` (Decision 5), so the puts ship with earnings and account data only when those land.
3. Owner prices in `watch.toml`. 606 owns the file, `WatchEntry` with its `BuyPrice` and `SalePrice` fields (decimal text in the currency of the contract, empty unset) and the loader that refuses a bad value (its `earnings-job` and `watch-loader-earnings-wire`); this design adds no field, no loader and no second file. When valuation (607) lands it may propose a band; the owner's word wins, a permanent precedence stated here.
4. Numbers. Stored `bid`, `ask`, `delta` and the underlying price are `*float64`. The job converts each with `strconv.FormatFloat(v, 'f', -1, 64)` (shortest text that round-trips) and then rounds half-even to 4 places before `big.Rat.SetString`, so 1.99 is 199/100 and 0.1 is 1/10; `CandidateInput` carries decimal text only. `DGS3MO` is in percent: the rate ratio is `value / 100`, exactly, from the same text conversion. Yield is stored as decimal text to 6 places.
5. Status rule. Each gate is `pass`, `block` or `unknown`; the row status is `blocked` if any gate blocked, else `unknown` if any is unknown, else `ok`. Only `ok` rows are ranked. An `unknown` gate marks its row and is stored with its reason; it never freezes a day, because rows are keyed by run (Decision 9) and the reader takes the latest run.
6. Which contracts. For each US underlying of the watch list with a chain at D, the latest XNYS session day on or before the run day (gate `chain_fresh`: any other D gives `unknown stale_chain` on every row), and each contract with DTE (expiry minus D, calendar days) in `min_dte..max_dte`. Moneyness is not filtered: puts and calls are evaluated at every captured strike and the gates decide; an in-the-money row gets the mark `itm`. The premium is the end-of-day bid at the 16:15 freeze; the owner sells next session, so it is an estimate, shown as such (`bid_eod` in the columns).
7. Put gates, in order; each row keeps the first reason code of every gate it did not pass:
   1. `buy_price`: `strike - bid <= BuyPrice` (inclusive, per share, before fees). No `BuyPrice` is `block no_buy_price`.
   2. `quote`: row status ok, bid above 0, ask present, `bid_size >= min_bid_size`, market data type 1 or 2 (3 and 4 are `unknown`), `(ask - bid) / mid <= max_spread`, `open_interest >= min_open_interest`.
   3. `events`: next results date from the 606 reader, where a row counts while it is at most `max_event_age_sessions` XNYS session days old (counted with the calendar, so a Friday row is one session old on Monday and a holiday does not break it). A `listed` date from D to the expiry, inclusive, is `block earnings_before_expiry`. A `listed` date after the expiry passes with the mark `earnings_unconfirmed` (606: a vendor date refuses or marks, never clears). `unsupported`, `error`, no row, a stale row, or an expiry beyond the 3-month horizon is `unknown`. `absent` is `unknown` unless the company reported recently: when the reader holds a `listed` date R in `D - 100 .. D - 1` and `expiry < R + min_report_gap_days` (60, advisor-set), the next report cannot fall inside the contract and the gate passes with the mark `earnings_beyond_horizon`; so a name that just reported is not blocked until the next date appears. The owner's `report_date_override` replaces the vendor date as in 606.
   4. `funding`, per alias (A, B): `strike x multiplier` is at most `AvailableFunds` of that alias from the newest `account.value` capture, same currency; a capture older than one session day is `unknown`, a different currency is `unknown no_fx` (no conversion is built, Left out). The row stores `fits` (alias to `fit`, `no_fit`, `unknown`); the gate passes when one alias is `fit`. This is weaker than ADR 0009, which used settled cash minus all open and working puts; that stricter rule is the named next step of the same gate (Left out).
   5. `cash_rate` (owner answer 3a: yield above the 13-week Treasury bill): annualized yield above `DGS3MO / 100` of at most `max_rate_age_days` (7) old, else `block yield_below_cash` (a yield equal to the rate does not pass); no fresh rate is `unknown`. (ADR 0009 used the treasury.gov 13-week rate; the stored FRED series is the same instrument and is already loaded.)
8. Ranking, no composite score. Annualized yield `= (bid / strike) x (365 / DTE)` for a put, `(bid / spot) x (365 / DTE)` for a call. Order of `ok` rows: yield descending, then DTE ascending, then strike ascending, then instrument id. `rank` is the position in the run, `rank_in_underlying` within a company. Stored, never ranked: break-even (put `strike - bid`, call `strike + bid`), delta, IV, `preferred_dte` (30 to 45). Blocked and unknown rows are stored with their reasons, without a rank.
9. Run and key. Key `(session_day, run_id, kind, instrument_id, alias)` plus a `rule_version` column; insert-only, a rerun is a new `run_id` (the runner's run id), and `CandidateReader(day, kind)` returns the rows of the newest run of that day (by `computed_at`). Order in `run-once`: `option_eod`, `earnings_refresh`, `account_capture`, `option_candidates` last (the job after those it reads); in the scheduler `option_candidates` is anchored at `session_close` plus 30 minutes and the earnings and account jobs are scheduled before it by their own designs; if one is late the row is `unknown` and the owner reruns `run-once`, which replaces the view with a newer run.
10. Capture follows the owner's prices (issue 1). `option_eod` takes its underlyings from the watch list (US stock entries). Its strike range per underlying is derived from `BuyPrice`: from `0.80 x BuyPrice` up to `max(BuyPrice x 1.05, spot)`, at most `max_strikes` (24) strikes; with no `BuyPrice` the old band of 10 percent applies. Expiries are those with DTE in `min_dte..max_dte` instead of the nearest three. The held short calls (issue 9) add their own expiries and strikes whatever their DTE.
11. Covered calls (issues 9 to 12). Shares are the stock rows of `account.position` per alias at the newest capture; free shares = shares minus `multiplier x` open short calls of that underlying. Gates: `shares` (`block no_free_shares`), `strike_floor` (owner answer 1a: `strike >= SalePrice` if set, else `strike >= avg_cost`, the average cost of the shares in that alias; neither is `unknown`), `quote`, `events` as for puts, and `guard`. One row per alias that holds the shares.
12. Early-assignment and ex-dividend guard. With the next ex-date X (606 `present` row, `next_ex_date`) from D to the expiry and amount `d` per share: `extrinsic = max(0, bid - max(0, spot - strike))`; `extrinsic < d x div_guard_factor` (1.0, advisor-set) is `block early_assignment_risk` (holders exercise the day before the ex-date). A `no_value` row passes with the mark `no_dividend`; `error`, `unsupported`, no row or a stale row is `unknown`. Every ex-date is projected (606 Decision 2). The same function judges short calls already open: kind `held_call_guard`, per alias, verdict `risk`, `ok` or `unknown`; their contracts are captured whatever their DTE (Decision 10), so the verdict is not `unknown` by construction.
13. Privacy. `fits`, `alias`, shares and guard rows come from account data and are stored in one table read only by the roles of the pages and `ot candidates`. The job and the rule import no `notify` and no sender; tests pin it (ADR 0010 stands). The margin cushion and assignment cash have no row here (`account-figures`).
14. Japan is next, not dropped. The owner has the JP market data subscription (answer 4a). After the US puts, a probe of Japanese option chains comes next: IBKR routes them as `OSE.JPN` (old ADR 0004), which the pyside contract can carry in `Contract.Exchange`; whether the gateway returns usable single-stock chains and quotes is untested and `OT` has no JP option underlying today. Then the design `option-candidates-jp` (JP parameters from the old `put_jp_v1.toml` and `covered_call_jp_v1.toml`, the JP calendar, 09:00 to 15:40 JST hours). 606 gives JP earnings (J-Quants) and, after its probe, dividends. Until then a JP entry gives no row and `ot candidates` prints one line naming it, so it is never silent.
15. Owner-confirmed defaults (answer 2a) and advisor-set values (keys of `job.toml`, the owner may change them): DTE 21 to 45, spread at most 10 percent, open interest at least 10 and the dividend guard are kept as written; the earnings override is `report_date_override` of `watch.toml`, the old acknowledgement is not rebuilt. The values: `min_dte 21`, `max_dte 45`, `min_bid_size 1` (the three ported from `put_v1.toml`), `max_spread 0.10`, `min_open_interest 10`, `div_guard_factor 1.0`, `max_rate_age_days 7`, `max_event_age_sessions 2`, `min_report_gap_days 60`, `max_strikes 24`.

## The option side, whole

What the owner needs to run options completely, with its design and state. This table is the input for sequencing; a missing row names the next design. Order: the US puts first (issues 1 to 8 and 12), then the probe of Japanese option chains (IBKR `OSE.JPN`, old ADR 0004; the owner has the JP market data subscription), then the design `option-candidates-jp`; the US calls and the guard (issues 9 to 11) follow the dividend chain. Japan is next, not dropped.

| piece | design | state |
|---|---|---|
| capture of the option chain | `option-jobs.md`, job `option_eod` | merged; SPY only: issue 1 of this design (watch list, band from the buy price, DTE window) and issue 9 (held calls) |
| earnings and ex-dividend dates | `event-sources.md` | in review PR 606 (earnings first, dividends behind its probe) |
| watch list and the owner's buy and sale price | `event-sources.md` (list, fields and loader) | list, `BuyPrice`, `SalePrice` and the loader: PR 606 (`earnings-job`, `watch-loader-earnings-wire`) |
| account positions, values, executions (accounts A and B) | `account-data.md` | in review PR 561; collector and Go client merged |
| candidates, gates, ranking: short puts | this design | this design, issues 2 to 8 |
| covered calls and the ex-dividend guard, held calls too | this design | this design, issues 9 to 12, after 606 `dividend-store` |
| a page for the candidates (`option_page`, `stock_option_page`) | views in `option-pages-v22.md` | builder missing; next design `option-candidates-page`, started right after this one is approved (until then `ot candidates` prints the list) |
| ledger, cost basis, wheel | none | missing; next design `option-ledger`, needs `account.execution` history and assignment rows (PR 561, Flex or `ot import`) |
| assignment cash by date per account, margin cushion | `account-data.md` Decision 9 names them | missing; next design `account-figures`, needs PR 561 |
| tracking of open positions, P and L, roll and close advice | `tracking-household-reports.md` is the view only | missing builder and data; next design `option-tracking`, needs `option-ledger` |
| alerts: market and missed-job on Telegram | `risk-alerts.md`, `notify.md` | merged (market only, ADR 0010 amended) |
| local notice for the guard, cushion, assignment cash (never Telegram) | none | missing; next design `option-notices`, needs `account-figures` |
| Japan options | none | next after the US puts: probe of JP chains (`OSE.JPN`), then design `option-candidates-jp`; the owner has the JP data subscription (Decision 14) |
| valuation and the automatic buy price | `valuation.md` | parked, PR 607; the hand-typed buy price replaces it |

## Modules

New directories: `internal/compute/optioncand`, `internal/jobs/optioncandidates`, `internal/store/candidate`, `internal/pages/candidate_text`. This pull request edits `docs/design/modules.md` by four names in the sentence that lists the implementation packages `cmd/ot` may import and regenerates `arch.toml` with `go run ./scripts/arch` (the precedent is `internal/compute/ease`; `.go-arch-lint.yml` is tree-derived and appears with the first issue of each package). Interface files: `internal/compute/api.go`, `internal/store/api.go`, `internal/jobs/api.go` (each additive), the dataset in `contracts/`.

| # | issue | module | order |
|---|---|---|---|
| 1 | option-eod-watch (interface and implementation) | jobs | after 606 `earnings-job` |
| 2 | candidate-compute-interface (interface) | compute, plus repo (licensed pair) | first |
| 3 | compute-put-candidates | compute | after 2 |
| 4 | candidate-dataset (interface) | contracts | after 561 `account-proto`, 606 `earnings-datasets` |
| 5 | candidate-store-api (interface) | store | after 4, 561 `account-store-api`, 606 `earnings-store` |
| 6 | candidate-store-impl | store, plus repo (licensed pair) | after 5, 561 `account-store-migration` and `account-store-impl`, 606 `earnings-store` and `dividend-store` |
| 7 | candidates-job-puts (interface and implementation) | jobs, plus repo (licensed pair) | after 1, 3, 6, 561 `account-job`, 606 `earnings-job` |
| 8 | candidate-text | pages | after 5 |
| 9 | option-eod-held-calls | jobs | after 1, 561 `account-store-impl` |
| 10 | compute-call-candidates | compute | after 3 |
| 11 | candidates-job-calls (interface and implementation) | jobs, plus repo (licensed pair) | after 7, 9, 10, 606 `dividend-store` and `dividend-job` |
| 12 | candidates-wire | ot | after 7, 8, 606 `watch-loader-earnings-wire`, 561 `account-wire`; calls are wired by a second pass of the same file once 11 has merged |

Puts are complete at issue 12 without 9 to 11; the owner sees puts in `ot candidates` as soon as 1 to 8 and 12 have merged.

## Interface

- jobs, `internal/jobs/api.go`: `WatchEntry` and its `BuyPrice, SalePrice string` (decimal text, empty unset) are declared by 606 (`earnings-job`); this design declares none of them. `type NewOptionEodWatchFunc func(broker.Broker, store.InstrumentResolver, store.OptionEodWriter, []WatchEntry, clock.Clock) (contracts.Job, error)` (a second constructor so `NewOptionEodFunc` and `cmd/ot` keep building). `type CandidateStores struct{ OptionEod store.OptionEodReader; Terms store.OptionTermsReader; Names store.InstrumentNameReader; Series store.SeriesReader; Earnings store.EarningsDateReader; Accounts store.AccountReader; Candidates store.CandidateWriter; SessionDays store.SessionDayReader }` and `type NewOptionCandidatesFunc func(CandidateStores, compute.OptionCandidates, []WatchEntry, clock.Clock) (contracts.Job, error)`. Issue 11 adds `NewOptionCandidatesCallsFunc` taking `CandidateStores` plus `Dividends store.DividendReader` (no signature above changes). A nil reader makes its gates `unknown`.
- compute, `internal/compute/api.go`: `CandidateKind` (`short_put`, `covered_call`, `held_call_guard`), `GateStatus` (`pass`, `block`, `unknown`), `GateResult{Gate, Status, Reason}`, `OptionQuote`, `CandidateInput` (day, underlying, spot, quotes, `BuyPrice`, `SalePrice`, earnings rows, optional dividend row, rate, per-alias shares, avg cost, funds, open short calls; decimals as text), `Candidate`, `CandidateParams` (Decision 15), `type OptionCandidates interface { Evaluate(CandidateInput, CandidateParams) []Candidate }`, `type NewOptionCandidatesFunc func() OptionCandidates`. Declared once for puts and calls (issue 2), so the later call issues add no interface change.
- contracts: dataset `candidates.option_candidate` r1: key `(session_day, run_id, kind, instrument_id, alias)`; columns `rule_version, issuer, underlying_id, expiry, dte, right, strike, multiplier, bid_eod, ask, spot, buy_price, sale_price, break_even, annualized_yield, delta, iv, status, gates` (ordered JSON array text), `marks`, `fits`, `next_earnings, earnings_state, next_ex_date, ex_amount, extrinsic, rank, rank_in_underlying, computed_at`; money as decimal text; `alias` empty for puts.
- store: `CandidateWriter.InsertCandidates(ctx, []Candidate) (int, error)`, `CandidateReader.Candidates(ctx, day, kind) ([]Candidate, error)` (newest run of the day, ranked rows first), `store.DSNEnvCandidate = "OT_PG_DSN_CANDIDATE"`, plain row type `store.Candidate`.
- pages: `candidate_text.Format(rows []store.Candidate, all bool) string` (no sort, no arithmetic: text of stored columns). ot: `ot candidates [--day D] [--kind K] [--all]`, wiring only.

## Issues

Every test calls `t.Parallel()`, writes files only under `t.TempDir()`, uses a fake clock, never sleeps, opens no port, and touches no live broker or database (a test that needs PostgreSQL carries the tag `pg`). Acceptance is checked offline by the command shown.

### 1 option-eod-watch (interface and implementation)

Module: jobs. Size: M.
Files: `internal/jobs/api.go`, `internal/jobs/api_test.go`, `internal/jobs/optioneod/job.toml`, `internal/jobs/optioneod/select.go`, `internal/jobs/optioneod/select_test.go`, `internal/jobs/optioneod/manifest.go`, `internal/jobs/optioneod/plan.go`, `internal/jobs/optioneod/plan_test.go`, `internal/jobs/optioneod/run.go`, `internal/jobs/optioneod/run_test.go`, `internal/jobs/optioneod/watch.go`, `internal/jobs/optioneod/watch_test.go`, `docs/contracts/jobs.md`
Design: docs/design/option-candidates.md#decisions (Decision 10)
Order: after 606 `earnings-job` (it declares `WatchEntry` with `BuyPrice` and `SalePrice` in the same `api.go`, `api_test.go`, `jobs.md`).
Goal: `optioneod.NewWithWatch` (the type `NewOptionEodWatchFunc`) takes the underlyings from the `WatchEntry` list (US stock entries only); expiries are those with DTE in `min_dte..max_dte`; the strikes of an entry with a `BuyPrice` run from `0.80 x BuyPrice` to `max(BuyPrice x 1.05, spot)`, at most `max_strikes`; an entry without one keeps the 10 percent band. `optioneod.New` and `NewOptionEodFunc` are unchanged and the manifest without the new keys behaves as today, byte for byte.
Acceptance:
1. `go test ./internal/jobs/optioneod -race -count=1` passes with `TestWatchUnderlyingsUSOnly`, `TestDTEWindowSelectsMonthlies`, `TestNoWindowKeepsNearestExpiries`, `TestStrikeRangeFromBuyPrice`, `TestNoBuyPriceKeepsBand`, `TestMaxStrikesCap` each present once (`grep -c` of each name is 1); `TestStrikeRangeFromBuyPrice` has buy price 100, spot 130 and requires strikes 80 to 130 only.
2. `git diff $(git merge-base HEAD origin/main) -- internal/jobs/api.go | grep '^-' | grep -vc '^---'` prints 0; `grep -c "NewOptionEodWatchFunc" internal/jobs/api.go` prints 1.
3. `grep -c "min_dte\|max_dte\|max_strikes" docs/contracts/jobs.md` is at least 3; `git diff --name-only $(git merge-base HEAD origin/main)` lists no path outside the Files line; each new file is under 600 lines.
Checked by CI: `go test ./internal/jobs/...`, go-arch-lint. Whether the gateway returns the monthlies and the deeper strikes, and whether the run fits `budget_minutes`, is not checkable offline; the first run is read with `ot candidates`.

### 2 candidate-compute-interface (interface)

Module: compute, plus repo (the licensed pair: `.go-arch-lint.yml`). Size: S.
Files: `internal/compute/api.go`, `internal/compute/api_test.go`, `docs/contracts/compute.md`, `internal/compute/optioncand/doc.go`, `.go-arch-lint.yml`
Design: docs/design/option-candidates.md#interface
Order: first.
Goal: the compute types of `## Interface` with doc comments for each gate word and reason code, and a paragraph of `docs/contracts/compute.md` that fixes the promises: exact arithmetic, the float-to-decimal step, `DGS3MO` is percent, unknown is not a pass, only `ok` rows ranked, yield and break-even formulas. The contract test uses a fake `OptionCandidates` and a caller that checks the order of the gate results and that a blocked row has no rank.
Acceptance:
1. `grep -c "OptionCandidates\|CandidateInput\|CandidateParams\|GateResult" internal/compute/api.go` is at least 4; `git diff $(git merge-base HEAD origin/main) -- internal/compute/api.go | grep '^-' | grep -vc '^---'` prints 0.
2. `go test ./internal/compute -race -count=1` passes; `grep -c "time.Sleep\|time.Now()" internal/compute/api_test.go` prints 0; `wc -l < internal/compute/api_test.go` prints less than 800 (464 on main).
3. `grep -c "annualized\|break-even\|percent" docs/contracts/compute.md` is at least 3; `go list -deps ./internal/compute | grep -c "internal/compute/optioncand"` prints 0.
4. `go run ./scripts/arch --check` prints `arch: ok`; `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the Files line.
Checked by CI: ci-go, `go run ./scripts/arch && git diff --exit-code arch.toml .go-arch-lint.yml`.

### 3 compute-put-candidates

Module: compute. Size: L.
Files: `internal/compute/optioncand/optioncand.go`, `internal/compute/optioncand/put.go`, `internal/compute/optioncand/events.go`, `internal/compute/optioncand/rank.go`, `internal/compute/optioncand/exact.go`, `internal/compute/optioncand/put_test.go`, `internal/compute/optioncand/events_test.go`, `internal/compute/optioncand/rank_test.go`, `internal/compute/optioncand/exact_test.go`, `internal/compute/optioncand/purity_test.go`, `internal/compute/optioncand/testdata/`
Design: docs/design/option-candidates.md#decisions (Decisions 4 to 8)
Order: after 2.
Goal: `New()` returns an `OptionCandidates` that applies, for `short_put`, the gates of Decision 7 in order, the status rule of Decision 5 and the ranking of Decision 8 on `big.Rat`.
Acceptance:
1. `grep -rhE "^func (TestBuyPriceBoundary|TestNoBuyPriceBlocks|TestQuoteGate|TestEarningsListedInsideBlocks|TestEarningsAfterExpiryMarks|TestEarningsUnknownBlocks|TestEarningsAbsentAfterRecentReport|TestEventAgeInSessions|TestStaleChain|TestFundingPerAlias|TestNoFxIsUnknown|TestCashRatePercent|TestUnknownIsNotPass|TestRankByYield|TestRankTieOrder|TestBlockedHasNoRank|TestDeterministic|TestNoFloatNoClockNoIO)\(" internal/compute/optioncand | wc -l` prints 18, and `go test ./internal/compute/optioncand -race -count=1` passes.
2. `TestBuyPriceBoundary`: strike 100, bid "2.00", buy price "98.00" passes, bid "1.99" blocks. `TestCashRatePercent`: a rate text "4.20" compares as 0.042 (a yield 0.05 passes, 0.04 blocks). `TestEventAgeInSessions`: a row dated Friday is fresh on Monday and a row from Wednesday is stale on the next Monday, with a fake calendar. `TestEarningsAbsentAfterRecentReport`: `absent` with a `listed` date 40 days back and expiry in 30 days passes with the mark `earnings_beyond_horizon`; with no earlier date it is `unknown`.
3. `TestNoFloatNoClockNoIO` parses the package files with `go/parser` and fails on any identifier `float32` or `float64`, any import of `os`, `net/http`, `database/sql`, `time` calls `Now` or `Sleep`, or an import of `internal/store`, `internal/notify` or `internal/jobs` (the AST ignores comments, so a word in a comment cannot fail it).
4. `TestDeterministic` runs the same input shuffled 20 times (fixed seed) and requires identical output; `grep -c "t.Parallel()"` of each test file is at least its count of `^func Test`.
5. `git diff --name-only $(git merge-base HEAD origin/main)` lists no path outside the Files line (`testdata/` counts as one); each new file is under 600 lines.
Checked by CI: ci-go, golangci-lint, go-arch-lint. Whether the advisor-set limits suit the owner is decided by reading the first stored day.

### 4 candidate-dataset (interface)

Module: contracts. Size: M.
Files: `contracts/datasets/candidates.option_candidate.r1.json`, `contracts/generate.sh`, `internal/contracts/gen/records.go`, `internal/contracts/api.go`, `docs/contracts/datasets/candidates.option_candidate.md`, `contracts/fixtures/candidates.option_candidate.r1.json`, `contracts/fixtures/MANIFEST.synthetic-v1.json`, `internal/contracts/schema_test.go`
Design: docs/design/option-candidates.md#interface
Order: after 561 `account-proto` and 606 `earnings-datasets` (the same `generate.sh`, `records.go`).
Goal: the dataset of the Interface section, with the generated Go record and its document committed together; the fixture is marked synthetic.
Acceptance:
1. `test -f contracts/datasets/candidates.option_candidate.r1.json`, and `bash contracts/generate.sh && git diff --exit-code internal/contracts/gen` leaves no change.
2. `go test ./internal/contracts -race -count=1` passes with the fixture validated; `grep -c "candidates.option_candidate" contracts/fixtures/MANIFEST.synthetic-v1.json` is at least 1.
3. `grep -c '"run_id"\|"rule_version"' contracts/datasets/candidates.option_candidate.r1.json` is at least 2, and its `x-primary-key` equals `["session_day","run_id","kind","instrument_id","alias"]`.
4. `git diff --name-only $(git merge-base HEAD origin/main)` lists no path outside the Files line.
Checked by CI: the generate check of `checks.yml`; the pull request body carries `Contracts: changed`.

### 5 candidate-store-api (interface)

Module: store. Size: S.
Files: `internal/store/api.go`, `internal/store/api_candidate_test.go`, `docs/contracts/store.md`
Design: docs/design/option-candidates.md#interface
Order: after 4, 561 `account-store-api`, 606 `earnings-store` (the same `api.go`, `store.md`).
Goal: `CandidateWriter`, `CandidateReader`, `store.Candidate`, `DSNEnvCandidate`, declarations only. `internal/store/api_test.go` is 800 lines and is not edited; the contract test is the new file `internal/store/api_candidate_test.go`: a fake on one side, a caller that reads a day written by two runs and sees only the newer run, ranked rows first.
Acceptance:
1. `grep -c "type CandidateWriter interface\|type CandidateReader interface" internal/store/api.go` prints 2; `git diff $(git merge-base HEAD origin/main) -- internal/store/api.go | grep '^-' | grep -vc '^---'` prints 0; `grep -c "database/sql\|pgx" internal/store/api.go` equals its count on main.
2. `go test ./internal/store -run TestCandidateContract -race -count=1` passes; the diff does not list `internal/store/api_test.go`.
3. `grep -c "OT_PG_DSN_CANDIDATE\|CandidateReader" docs/contracts/store.md` is at least 2.
Checked by CI: ci-go, go-arch-lint.

### 6 candidate-store-impl

Module: store, plus repo (the licensed pair: `.go-arch-lint.yml`). Size: M.
Files: `migrations/NNNN_candidates.sql` (NNNN the next free number at cut time), `internal/store/candidate/doc.go`, `internal/store/candidate/store.go`, `internal/store/candidate/read.go`, `internal/store/candidate/store_test.go`, `internal/store/candidate/store_pg_test.go` (tag `pg`), `internal/store/setup/setup.go`, `internal/store/setup/setup_test.go`, `internal/store/setup/candidate_pg_test.go` (tag `pg`), `.go-arch-lint.yml`
Design: docs/design/option-candidates.md#decisions (Decisions 1, 9, 13)
Order: after 5; after 561 `account-store-migration` and `account-store-impl`, 606 `earnings-store` and `dividend-store` (same `setup.go`, the migration number and `.go-arch-lint.yml`: this entry takes the next free number and rebases on the last of them).
Goal: a table `candidates.option_candidate` with the dataset key, insert-only, the role `ot_w_candidate` (INSERT and SELECT) and `SELECT` for the pages role; `candidate.Open(ctx, dsn)` implements both interfaces; the reader returns the newest run of the day.
Acceptance:
1. `grep -c "CREATE TABLE candidates.option_candidate\|ot_w_candidate" migrations/NNNN_candidates.sql` is at least 2 and `grep -ci "GRANT.*\(UPDATE\|DELETE\)" migrations/NNNN_candidates.sql` prints 0.
2. `go test ./internal/store/candidate ./internal/store/setup -race -count=1` passes without a database (query text, row mapping, an empty day gives an empty slice, a zero day is an error, two runs give the newer one).
3. `grep -rl "os.Getenv" internal/store/candidate` prints nothing (the DSN is passed in).
4. `grep -c "^func TestCandidateRoundTrip\|^func TestCandidateNewestRunWins" internal/store/candidate/store_pg_test.go` prints 2 and the file starts with `//go:build pg`; the diff lists no path outside the Files line.
Checked by CI: the two `pg` tests run only in GitHub CI; `go run ./scripts/arch --check`.

### 7 candidates-job-puts (interface and implementation)

Module: jobs, plus repo (the licensed pair: `.go-arch-lint.yml`). Size: L.
Files: `internal/jobs/api.go`, `internal/jobs/api_test.go`, `internal/jobs/optioncandidates/job.go`, `internal/jobs/optioncandidates/job.toml`, `internal/jobs/optioncandidates/input.go`, `internal/jobs/optioncandidates/convert.go`, `internal/jobs/optioncandidates/schedule.go`, `internal/jobs/optioncandidates/job_test.go`, `internal/jobs/optioncandidates/input_test.go`, `internal/jobs/optioncandidates/convert_test.go`, `internal/jobs/optioncandidates/schedule_test.go`, `internal/jobs/optioncandidates/testdata/`, `docs/contracts/jobs.md`, `.go-arch-lint.yml`
Design: docs/design/option-candidates.md#decisions (Decisions 1, 4, 5, 9, 13, 15)
Order: after 1, 3, 6, 561 `account-job` and 606 `earnings-job` (they create job packages and regenerate `.go-arch-lint.yml`; this entry rebases on the last of them).
Goal: `optioncandidates.New` (the type `NewOptionCandidatesFunc`) runs at `session_close` plus 30 minutes; for each watched US underlying it reads the newest chain day, the terms (in chunks of 500 ids), the earnings rows, the newest account captures per alias and `DGS3MO`, converts floats to decimal text (Decision 4), calls `Evaluate` for `short_put` and writes the rows under a new run id. A nil reader gives `unknown` gates, not an error; a store error is a failed run with a named code. The job holds no dividend reader.
Acceptance:
1. `go test ./internal/jobs/optioncandidates -race -count=1` passes with `TestNilEarningsIsUnknown`, `TestRerunIsNewRun`, `TestOnlyUSEntries`, `TestAliasesAandBAreSeparate`, `TestStoreErrorFailsRun`, `TestScheduleIsCloseAfterEod`, `TestTermsChunkedAt500`, `TestStaleChainDay`, `TestJPEntryNamedNotSilent` each present once.
2. `TestFloatToDecimal` in `convert_test.go` feeds 1.99, 0.1, 100.00049 and 1e-7 and requires the texts "1.99", "0.1", "100" and "0" (shortest round-trip text, then half-even to 4 places, trailing zeros dropped); the test for `DGS3MO` "4.20" requires the ratio 0.042.
3. `grep -rl "internal/notify\|telegram" internal/jobs/optioncandidates` prints nothing; `go list -deps ./internal/jobs/optioncandidates | grep -c "internal/notify"` prints 0; `grep -rc "Dividend" internal/jobs/optioncandidates/job.go` prints 0.
4. `grep -c "min_dte\|max_dte\|min_bid_size\|max_spread\|min_open_interest\|max_rate_age_days\|max_event_age_sessions\|min_report_gap_days" internal/jobs/optioncandidates/job.toml` is 8.
5. `git diff $(git merge-base HEAD origin/main) -- internal/jobs/api.go | grep '^-' | grep -vc '^---'` prints 0; the diff lists no path outside the Files line; each new file is under 600 lines.
Checked by CI: ci-go, go-arch-lint, `go run ./scripts/arch --check`.

### 8 candidate-text

Module: pages. Size: S.
Files: `internal/pages/candidate_text/format.go`, `internal/pages/candidate_text/format_test.go`, `internal/pages/candidate_text/testdata/`, `docs/contracts/pages.md`
Design: docs/design/option-candidates.md#interface
Order: after 5.
Goal: `Format(rows, all)` returns the plain text of the stored rows in the stored order: for `ok` rows company, kind, alias, expiry, strike, bid, annualized yield, break-even and marks; with `all` the blocked and unknown rows with their gate reasons; a line per JP entry is not its job (the command adds it). No sort, no arithmetic.
Acceptance:
1. `go test ./internal/pages/candidate_text -race -count=1` passes with `TestFormatOrderKept`, `TestFormatAllShowsReasons`, `TestFormatEmpty` each present once.
2. `grep -rc "sort\.\|big\.\|float64" internal/pages/candidate_text/format.go` prints 0; `grep -c "candidate_text" docs/contracts/pages.md` is at least 1.
3. The diff lists no path outside the Files line.
Checked by CI: ci-go, go-arch-lint.

### 9 option-eod-held-calls

Module: jobs. Size: M.
Files: `internal/jobs/optioneod/held.go`, `internal/jobs/optioneod/held_test.go`, `internal/jobs/optioneod/run.go`, `internal/jobs/optioneod/run_test.go`, `internal/jobs/optioneod/plan.go`, `internal/jobs/optioneod/plan_test.go`
Design: docs/design/option-candidates.md#decisions (Decisions 10 and 12)
Order: after 1 (same files), 561 `account-store-impl`.
Goal: `optioneod` optionally takes a `store.AccountReader` (a nil reader changes nothing) and adds the expiry and strike of every open short call of a watched underlying, from the newest `account.position` capture, to the day's contract list whatever its DTE.
Acceptance:
1. `go test ./internal/jobs/optioneod -race -count=1` passes with `TestHeldCallAddedAtTenDTE`, `TestHeldCallDeduplicated`, `TestNilAccountReaderUnchanged` each present once.
2. `git diff $(git merge-base HEAD origin/main) -- internal/jobs/api.go | grep -c .` prints 0; the diff lists no path outside the Files line.
Checked by CI: ci-go.

### 10 compute-call-candidates

Module: compute. Size: M.
Files: `internal/compute/optioncand/call.go`, `internal/compute/optioncand/guard.go`, `internal/compute/optioncand/call_test.go`, `internal/compute/optioncand/guard_test.go`, `internal/compute/optioncand/testdata/`
Design: docs/design/option-candidates.md#decisions (Decisions 11 and 12)
Order: after 3.
Goal: `Evaluate` handles `covered_call` (gates of Decision 11) and `held_call_guard` (Decision 12), same status rule and ranking.
Acceptance:
1. `grep -rhE "^func (TestCallFreeShares|TestCallStrikeFloorAvgCost|TestCallStrikeFloorSalePrice|TestGuardExtrinsicBelowDividend|TestGuardNoValuePasses|TestGuardUnknownStaleRow|TestGuardHeldCall|TestCallOneRowPerAlias)\(" internal/compute/optioncand | wc -l` prints 8, and `go test ./internal/compute/optioncand -race -count=1` passes (including `TestNoFloatNoClockNoIO` of issue 3).
2. `TestGuardExtrinsicBelowDividend`: spot "110", strike "100", bid "10.40", dividend "0.50" gives extrinsic 0.40 and blocks; bid "10.60" passes.
3. The diff lists no path outside the Files line; each new file is under 600 lines.
Checked by CI: ci-go, golangci-lint, go-arch-lint.

### 11 candidates-job-calls (interface and implementation)

Module: jobs, plus repo (the licensed pair: `.go-arch-lint.yml`). Size: M.
Files: `internal/jobs/api.go`, `internal/jobs/api_test.go`, `internal/jobs/optioncandidates/calls.go`, `internal/jobs/optioncandidates/calls_test.go`, `internal/jobs/optioncandidates/job.toml`, `docs/contracts/jobs.md`, `.go-arch-lint.yml`
Design: docs/design/option-candidates.md#interface
Order: after 7 (same files), 9, 10, 606 `dividend-store` and `dividend-job`.
Goal: `NewOptionCandidatesCallsFunc` adds the dividend reader and the call and guard kinds to the same run; `NewOptionCandidatesFunc` and puts behave as before; `div_guard_factor` joins `job.toml`.
Acceptance:
1. `go test ./internal/jobs/optioncandidates -race -count=1` passes with `TestCallsNeedDividendReader`, `TestPutsUnchangedByCalls`, `TestHeldCallGuardRows` each present once; `grep -c "div_guard_factor" internal/jobs/optioncandidates/job.toml` prints 1.
2. `git diff $(git merge-base HEAD origin/main) -- internal/jobs/api.go | grep '^-' | grep -vc '^---'` prints 0; `grep -rl "internal/notify" internal/jobs/optioncandidates` prints nothing; the diff lists no path outside the Files line.
Checked by CI: ci-go, go-arch-lint.

### 12 candidates-wire

Module: ot. Size: M.
Files: `cmd/ot/candidates.go`, `cmd/ot/candidates_test.go`, `cmd/ot/runonce.go`, `cmd/ot/runonce_test.go`, `cmd/ot/wire.go`, `cmd/ot/wire_test.go`, `cmd/ot/scheduler.go`, `cmd/ot/main.go`, `cmd/ot/env.go`, `docs/contracts/ot.md`
Design: docs/design/option-candidates.md#decisions (Decision 9)
Order: after 7, 8, 606 `watch-loader-earnings-wire`, 561 `account-wire` (the same `runonce.go`, `wire.go`, `env.go`). The calls are wired by a second pull request of this entry once 11 has merged; the entry is not closed before.
Goal: wiring only. `option_candidates` registers in `run-once` after `option_eod`, `earnings_refresh` and `account_capture`, and in the scheduler; `option_eod` uses `NewWithWatch`; a missing `OT_PG_DSN_CANDIDATE` disables only this job with a named line; `ot candidates` opens the store, calls `candidate_text.Format` and prints it, with one line per JP entry of the watch list ("no option route: see option-candidates-jp").
Acceptance:
1. `go test ./cmd/ot -race -count=1` passes with `TestCandidatesCommandPrints`, `TestCandidatesRegisteredAfterInputs`, `TestMissingCandidateDSNDisablesOnlyIt`, `TestCandidatesNamesJPEntry` each present once.
2. `grep -c "OT_PG_DSN_CANDIDATE\|ot candidates" docs/contracts/ot.md` is at least 2; `grep -c "sort\.\|big\.\|float64" cmd/ot/candidates.go` prints 0; `grep -c "notify\|telegram" cmd/ot/candidates.go` prints 0.
3. The diff lists no path outside the Files line; each new file is under 600 lines.
Checked by CI: `go test ./cmd/ot`, golangci-lint. The owner runs `ot migrate`, `ot setup pg` and adds `OT_PG_DSN_CANDIDATE` to `.env` before the first run.

## Left out

- The candidate page: the next design `option-candidates-page` (builder for `option_page` and `stock_option_page`, reading `store.CandidateReader`); `ot candidates` is the view until then.
- Japan options are next, not dropped (Decision 14): the probe of JP option chains after the US puts, then `option-candidates-jp`. Ledger, wheel, assignment cash, margin cushion, tracking, local notices: the next designs of the table, none is a row here.
- Funding by open and working puts and concentration caps (ADR 0011, 0013) and FX conversion; fees; the ITM probability N(-d2), the volatility spread and IV rank (they need the bars design); spreads, naked short calls, rolls. Register ids `option.candidates.*` of the old register are not ported; annualized yield and break-even are plain arithmetic in `docs/contracts/compute.md`.
- A coverage check of the chain (a run that ends early on its budget leaves fewer contracts, which looks like fewer candidates): named, not built; the first day is read by hand.
- An owner acknowledgement that clears an earnings block (old ADR 0016): the tool is `report_date_override` of 606; no separate record.
- Rejected: a composite score (owner decision); one JSON payload column or a dataset without a run id (review); computing in the page builder; a new file for the prices.

## Owner answers (2026-11-09)

1. Covered calls: the strike floor is your `sale_price` when set, else the average cost of the shares in that account. Adopted (Decision 11).
2. Defaults kept: DTE 21 to 45, spread at most 10 percent, open interest at least 10, guard when the extrinsic value is below the next dividend. Earnings override through `report_date_override` in `watch.toml`; the old acknowledgement is not rebuilt (Decision 15, Left out).
3. The yield gate is the annualized yield above the 13-week Treasury bill (FRED `DGS3MO`, percent converted to a ratio). Adopted (Decision 7.5).
4. Japan options: you have the JP market data subscription. After the US puts a probe of JP option chains (`OSE.JPN`) comes next, then the design `option-candidates-jp` (Decision 14).

## What the owner gets

- You type one or two prices per company in the file you already keep, `watch.toml`: `buy_price` (the most you would pay for the shares) and, if you like, `sale_price` (the least you would sell them for).
- After each close the tool lists the puts you could sell, best annualized yield first. A put is listed only when strike minus the premium is at or below your buy price, the quote is real, no results date falls before expiry, the cash fits in account A or B, and the yield is above the 13-week Treasury bill rate. Anything it cannot check shows as not cleared, with the reason, and a rerun after the missing data arrives replaces the list.
- Puts come first: they do not wait for the dividend data. Covered calls on shares you hold, with the ex-dividend assignment warning (also for calls you already hold), follow as issues 9 to 12.
- You read it with `ot candidates` on your own machine; a page comes in the next design. Nothing about accounts, cash or margin goes to Telegram, ever. Japanese options come next, after the US puts (a probe of the JP chains, then their own design): until then the tool says so by name. Covered calls use your `sale_price` as the strike floor, else your average cost in that account.
- Before it shows real rows you add `OT_PG_DSN_CANDIDATE` (and the 606 and 561 keys and DSNs) to `.env`, run `ot migrate` and `ot setup pg`, and fill `watch.toml` with each company's identifiers and prices.

## Adversarial

The adversarial review of revision 1 is in the dispatcher scratchpad, not in the repository. Answers:

- 1a JP absent: adopted. Decision 14 states the route (`OSE.JPN` as in the old ADR 0004, a probe first), the next design `option-candidates-jp`, a named line in the output, and an owner question.
- 1b capture cannot reach the put: adopted. Decision 10 and issue 1 derive the strike range from `BuyPrice`, add the DTE window and a strike cap; the per-entry band key is dropped.
- 1c guard on held calls mostly unknown: adopted. Issue 9 captures every open short call whatever its DTE.
- 1d staleness: adopted. Gate `chain_fresh` (Decision 6); the end-of-day bid is named an estimate.
- 1e unknown freezes the day, key without capture or run: adopted. Run id and `rule_version` in the key, newest run read, run-once order named (Decision 9).
- 1f earnings: adopted for `absent` after a recent report (Decision 7.3, `min_report_gap_days`), and event age in session days; one key for both sources. The two-source inconsistency is removed.
- 1g rule gaps: adopted in part. Moneyness is not filtered, so the put at the buy price above spot is evaluated; the funding gate says plainly it is weaker than ADR 0009 and gives `no_fx` as unknown. Not adopted: the stricter funding rule (its own later step, Left out).
- 1h owner steps: adopted (What the owner gets lists them).
- 2 and 3 put first and simpler way: adopted for puts first (Decision 2, issues 9 to 12 after the dividend chain, `CandidateStores` without a dividend reader). Not adopted: one JSON payload column (typed columns kept so the page and text read without parsing); the key gets the run id as the review asked.
- 4 lock-in: adopted for the key. The precedence of the owner's price is stated as permanent (Decision 3). `CandidateInput` is declared once with the call fields, so issues 10 and 11 add no interface change.
- 5 claims: `DGS3MO` percent conversion and the float-to-decimal step adopted (Decision 4, tests in issues 3 and 7); `absent` rule and `present` versus `no_value` wording adopted; 500-id chunking tested in issue 7; issue 6 now follows `dividend-store`.
- 6 order gaps: adopted (issues 6, 7, 11 name 561 and 606 entries that regenerate `.go-arch-lint.yml`); the issue 3 grep is replaced by an AST test; `ot` is wiring only, the text formatter is issue 8 in pages.
- Owner questions: answered 2026-11-09 (section Owner answers).
- WatchEntry: 606 (head 3d17f3a) declares `BuyPrice` and `SalePrice` (its `earnings-job`, test `TestWatchEntryPrices`) and loads them (its `watch-loader-earnings-wire`, test `TestWatchPricesLoaded`); the two interface and loader issues of revision 3 are deleted and the entries here depend on 606's.
