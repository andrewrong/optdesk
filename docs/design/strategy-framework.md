# strategy framework, step 1: a strategy is a unit behind one interface, its rows go to one generic table, and the short put moves in with rows equal to today's

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-13
Revision: 1

Source: owner 2026-11-13 (record `optdesk-owner-2026-11-13.md`, P1 to P6): strategies are separate units that read stored data and stored results, each added, removed and tuned on its own; rules come from professional sources and every number is marked sourced or trial; hold to expiry is the default exit with an optional early close, both to be run and compared; the strike of a short put is at or below the owner's buy price. The advisor's plan `option-strategy-plan.md` (sections 3, 4, 5, 6, 9; its labels [V] [S] [M] [O] [T] [NF] are kept where a number is quoted) and the valuation notes (`optdesk-owner-valuation-v1-2026-11-11.md`, section 4 and 8: a parameter set carries a version and a hash; the same function runs live and in replay; the watch list of today is a survivorship bias that a replay must say). Step 0 (`option-capture-need.md`, #2169) is merged.

Goal: (1) one interface, `strategy.Strategy`, that a strategy implements: an id, a rule version, parameters with a hash, a pure `Evaluate`, a pure `Rank` and a pure `Manage` (what to do with an open position of this strategy); (2) one generic result table that no strategy's shape is built into; (3) the short put (rule `put_v1`) moved behind the interface with stored rows equal to today's, proven on stored days; (4) the seam that step 2 (replay, choice of the close rule) and step 3 (account fit and the book stress) need, so that neither has to change an interface again. Steps 2 and 3 are not designed here: see Left out.

Existing design: `option-candidates.md` (the rule `put_v1`, its gates, Decisions 1, 6, 8, 9, 13, 15), `option-capture-need.md` (`Need`, the `[[need]]` rows), `compute-core.md`. Nothing of them changes behaviour.

## Today

All paths at origin/main (29c24a9a).

- The put rule is `compute.OptionCandidates.Evaluate(CandidateInput, CandidateParams) []Candidate` (`internal/compute/api.go:489-494`), implemented in `internal/compute/optioncand` (`put.go`, `rank.go`). It gets `Kinds`; `KindShortPut` is one of three (`api.go:308-316`). `internal/compute/api.go` has 839 lines against a ceiling of 840 (`docs/design/ratchet.json`), so nothing is added to it.
- The job `option_candidates` (`internal/jobs/optioncandidates`): `New` (puts) and `NewWithCalls` (puts, covered calls, held-call guards in one run; this one is built when the events DSN is set, `cmd/ot/wire.go:714`). Per underlying it builds a `compute.CandidateInput` (`job.go:151-224`, `calls.go:119-178`), calls `rule.Evaluate`, maps each `compute.Candidate` with `toStoreRow` (`convert.go:130`, rule version the constant `put_v1`, `convert.go:22`), ranks the run with `rankRun` (`convert.go:187`), stamps a run id that is a SHA-1 over the rows (`runIDOf`, `convert.go:166`) and appends to `candidates.option_candidate` (migration `0018_candidates.sql`, insert-only, newest run of a day wins).
- The table has a column per figure (`strike`, `bid_eod`, `annualized_yield`, ...) and CHECKs on `kind`, `status` and `option_right` (`0018_candidates.sql:46-48`), and `store.Candidate` (`internal/store/api.go:561`) repeats them. Readers of `store.CandidateReader`: `option_candidates_page`, `option_boards`, `option_tracking_page`, `option_notices_page`, `cmd/ot` (`candidates_site.go`, `live_quotes.go`, `notices_site.go`, `option_page_site.go`, `tracking_site.go`, `site.go`). A second strategy would need new columns or a new kind with half-empty columns.
- `internal/store/api.go` is at 800 lines (one more and it needs a ceiling), `cmd/ot/wire.go` at 795; new store packages sit beside the file (pattern of `internal/store/hoursnotice` and `hoursnoticepg`, #2181).
- The owner's closed trades: `compute.OptionLedger.Replay` turns the stored Flex rows into `compute.LedgerPosition` lots with fills, open price, close day and status (`api.go:536-577`); `vol-edge.md` counts 197 short lots since 2025-12, of which 117 are short puts and 51 covered calls. Option chains are stored only since 2026-11-07 (`option_eod`, migration 0006), so entry-day chains of his old trades do not exist.
- No option pricing model exists in `internal/` (no Black-Scholes); `optionstruct` has payoff and risk figures from prices.

## Decisions

1. **A strategy is a Go package behind `strategy.Strategy`** (Interface). No plugin loader and no rule language (advisor plan section 7). The list of strategies is built in `cmd/ot`; a strategy is added or removed by one line there and its `[[need]]` row in the job manifest. A test (new) fails when a registered strategy has no `[[need]]` row; a `[[need]]` row of a strategy not yet behind the interface (today `covered_call`) is allowed.
2. **New module `strategy`** (`internal/strategy/**`), because `compute/api.go` cannot grow and the plan asks for strategies independent of compute, store and jobs. It imports `contracts` and `compute` (the quote, earnings, dividend and funds types it reuses; it declares none of them again) and no store, job or page. `jobs` and `ot` depend on it; `store` does not (the store has its own row type, Decision 4).
3. **`Snapshot` is the input, `Evaluation` the output, both generic.** `Snapshot` carries what `compute.CandidateInput` carries minus `Kinds` and `OpenShortIDs`, plus `Issuer`, `UnderlyingID` and `Context` (named text facts the job knows about the underlying, today the earnings state and the next earnings day, which `toStoreRow` takes from `chainEarnings`). `Evaluation` carries `Key`, `Legs` (1 to n), ordered `Checks` (pass, block, unknown, with a reason; the gate word is the name), `Status`, `Facts` (named decimal or text values) and two ranks. A strategy copies `Snapshot.Context` into `Facts`. Absent figure: the key is not in `Facts` (today a nil pointer).
4. **One generic table `strategy.result`**, insert-only like today's, key `(session_day, run_id, strategy, key)`; the columns are the ones every strategy has (strategy, rule_version, param_hash, underlying_id, status, rank, rank_in_underlying, computed_at, settings_version) and three JSON texts `legs`, `checks`, `facts`; no per-figure column and no CHECK on strategy or kind (the CHECK on `kind` of `0018` is what blocks a second strategy today). The row type is `strategyresult.Row` in the store module; the store imports nothing of `strategy`; the job converts `Evaluation` to `Row` (job glue, `optioncandidates/shadow.go`). The owner's rule "one unique key, a rerun overwrites" (S1) is **not** applied here: a result row is a run of a computation of stored inputs with a run id and a settings version, the page reads the newest run, and equal-to-today needs the same rule; the run history costs nothing. Doubt 2.
5. **Role and view.** The table is written and read by the role `ot_w_candidate` (line `OT_PG_DSN_CANDIDATE`) and read by `ot_ro_pages` through the view `rd.strategy_result_r1`, as `candidates.option_candidate` is: the rows carry account-derived facts (fits by alias). No new role, no new line, no new secret; `internal/store/setup` is unchanged. Migration `0042_strategy_result.sql` (0041 is taken by `docs/design/data-repair-bars.md`, PR 2213: entry 5 is ordered after that migration's entry and neither is renumbered). The dataset `strategy.result r1` is declared in `contracts/datasets` with its generated record, as every table of the store is (`#1411` is the pattern).
6. **The short put moves behind the interface with the old path left running** (shadow, then cutover). Entry 6 makes the job evaluate the put through the strategy as well and write `strategy.result`, while the old `candidates.option_candidate` rows keep being written and read; entry 5 gives a reader that answers `store.CandidateReader` from `strategy.result` and a `Compare` that lists every difference; entry 8 swaps the one opener of the readers in `cmd/ot` after `ot strategy-compare` has printed no difference for every stored day. The page packages are not edited. The reader of the cutover serves `short_put` from the results only for a day that has a result run and everything else (other kinds, earlier days, and a day whose result insert failed) from the old reader, which keeps being written; a failed result insert therefore never empties a page. Deleting the old table, `toStoreRow` and the job's `rankRun` is Left out (after cutover, one owner migration). Cost: the put is evaluated twice per run for the length of the shadow, which is cheap (240 quotes an underlying).
7. **`shortput` is a thin adapter over the existing rule** (`compute.OptionCandidates`, `KindShortPut`): `Evaluate` builds the `CandidateInput` from the `Snapshot`, calls the rule and maps each `Candidate` to an `Evaluation`; `Rank` is the order of `rankRun` (yield descending, DTE, strike, instrument id ascending, on `Facts`). The put rule is not rewritten, so "results equal today's" is a property of the mapping, which entry 4 tests round trip (`Candidate -> Evaluation -> Row -> store.Candidate` equals `toStoreRow` of the same `Candidate`, for the table of cases of `put_test.go`'s fixtures) and entry 6 tests on the job. Own glue behind an interface: the adapter is the only code that knows both shapes; a strategy written later against `Strategy` needs no adapter.
8. **Parameters carry a version and a hash.** `Params{Values, Hash}`: `Values` are named decimal or text values with the keys of the strategy's manifest (for the put the `[params]` keys of `option_candidates/job.toml` and the three below), `Hash` is the SHA-1 hex of the sorted `key=value` lines (standard library; `internal/strategy/params`). `Version()` is the rule version (`put_v1`, as stored today). Both go on every row (`rule_version`, `param_hash`) so a replay row names the rule that made it (valuation notes, section 4).
9. **`Manage` is the management rule, the contested part** (plan section 2: SteadyOptions found no management better than none on SPX 2001 to 2020 [V]; tastylive 50% and 21 DTE is its own backtest [S]). `Manage(Position, Snapshot) Action` returns `hold` or `close` with a reason word. Short put parameters, all trial: `close_mode` = `expiry` (hold to expiry: the owner's habit [O], also what PUT does [V]; `Manage` always answers hold) or `early`; `early_keep` = `0.50` and `early_dte` = `21` (the alternative that the replay of step 2 compares; the numbers are [S] tastylive, close when the share of the credit kept reaches `early_keep` at the current ask, or the days to expiry fall to `early_dte`). Default `expiry`. Step 1 only declares and unit-tests it; nothing calls it until step 2.
10. **The strike rule is unchanged**: strike at or below the owner's buy price from valuation [O] (`buy_price` gate of `put.go`: `strike - bid <= BuyPrice`). `Snapshot.BuyPrice` stays the hand price of `watch.toml` until the valuation band exists (`valuation.md`); a replay will use the buy price of today for a past day and say so (step 2, Left out).
11. **What the owner sees: nothing changes**, and that is the test. The candidates page, home, tracking and notices show the same rows before and after cutover; `ot strategy-compare` prints `equal` or lists the differing keys and columns. What step 1 buys is that step 2 and 3 land as a section on the same page (the record of each close rule over his 197 lots and the stored days; one sentence on what the whole book loses at the FINRA moves) without a new table per strategy.

## Modules

One module is added and four rows change; `docs/design/modules.md` is owner-only, this design carries the edit and the owner's merge of this pull request approves it.

- `strategy` (new row, after `compute`): paths `internal/strategy/** docs/contracts/strategy.md`, entry `internal/strategy/api.go`, tests `internal/strategy/api_test.go`, depends `contracts compute`. Row in `## Contracts of the modules`: layer data; inputs a `Snapshot` and an open `Position`; outputs `Evaluation`, `Action`, `Params`; forbidden: any I/O, any clock, importing store, jobs or pages; contract document `docs/contracts/strategy.md`.
- `store` entry cell gains `internal/store/strategyresult` (declarations; the pg implementation is `internal/store/strategyresultpg`, an implementation package under the `internal/store/**` glob).
- `jobs` depends cell gains `strategy`; `ot` depends cell gains `strategy`.
- `contracts` row: no change (its globs already hold `contracts/datasets/**`, `internal/contracts/**`, `docs/contracts/datasets/**`).
- `compute`, `pages`, `web`, `pyside`: no change; `docs/design/ratchet.json` no change.

## Interface

Module strategy, `internal/strategy/api.go` (declarations only; imports `contracts` and `compute`):

```go
// Strategy is one option strategy as a unit (docs/design/strategy-framework.md). Every method is pure: no I/O,
// no clock, exact decimal text arithmetic.
type Strategy interface {
	ID() string      // "short_put": the strategy column of a result row and the strategy of its [[need]] row
	Version() string // the rule version, "put_v1": changes whenever a rule changes
	Params() Params
	// Evaluate gives one Evaluation per candidate structure of the snapshot, with RankInUnderlying set and Rank 0; Rank sets Rank.
	Evaluate(Snapshot) []Evaluation
	// Rank sets Rank over the rows of the run and returns them in the strategy's order.
	Rank([]Evaluation) []Evaluation
	// Manage says what to do with one open position of this strategy on the day of the snapshot.
	Manage(Position, Snapshot) Action
}

// Params are the named parameters of a strategy and the hash of them.
type Params struct {
	Values map[string]string // decimal or text; every number is trial unless its manifest comment names a source
	Hash   string            // SHA-1 hex of the sorted key=value lines (internal/strategy/params.Hash)
}

// Snapshot is everything one Evaluate or Manage reads. Days are YYYY-MM-DD, money and rates decimal text.
type Snapshot struct {
	RunDay, ChainDay contracts.Day
	SessionDays      []contracts.Day
	Issuer           string
	Underlying       string
	UnderlyingID     int64
	Spot             string
	Quotes           []compute.OptionQuote
	BuyPrice, SalePrice string // the owner's prices, empty when unset
	Earnings         *compute.EarningsInfo
	Dividend         *compute.DividendInfo
	Rate             string
	RateDay          contracts.Day
	Accounts         []compute.AliasFunds
	Context          map[string]string // named facts the caller knows; a strategy copies them into Facts
}

// Evaluation is one candidate structure with its checks. Key is unique within (day, strategy): the instrument
// ids of the legs joined by ":" and the alias when the row is per alias.
type Evaluation struct {
	Key              string
	Legs             []Leg
	Checks           []Check
	Status           string // ok, blocked or unknown
	Facts            map[string]string
	Rank             int // 0 when not ok
	RankInUnderlying int
}

type Leg struct {
	InstrumentID string
	Side         string // short or long
	Right        string // P or C
	Strike       string
	Expiry       contracts.Day
	Multiplier   string
	Quantity     int
}

type Check struct {
	Name   string // the gate word
	Status string // pass, block or unknown
	Reason string // the first reason code when not pass
}

// Position is one open lot of a strategy: what Manage needs; LedgerPosition fills it.
type Position struct {
	Contract   string
	Right      string
	Strike     string
	Expiry     contracts.Day
	OpenedOn   contracts.Day
	OpenPrice  string // credit per share
	Quantity   string // negative when short
	Multiplier string
}

// Action is the answer of Manage: Kind is hold or close; Reason is empty for hold and a word for close.
type Action struct{ Kind, Reason string }
```

Module store, `internal/store/strategyresult/api.go` (declarations only; imports `contracts`, `context`, `time`):

```go
// Row is one row of strategy.result r1. Legs, Checks and Facts are JSON text (array, array, object).
type Row struct {
	SessionDay       contracts.Day
	RunID, Strategy, RuleVersion, ParamHash, Key string
	UnderlyingID     int
	Status           string
	Rank, RankInUnderlying *int
	Legs, Checks, Facts string
	ComputedAt       time.Time
	SettingsVersion  *int64
}
type Writer interface {
	// InsertResults inserts the rows, insert-only; the same key again inserts nothing and is no error. It returns the count inserted.
	InsertResults(ctx context.Context, rows []Row) (int, error)
}
type Reader interface {
	// Results returns the rows of one strategy of one day written by the newest run of that day, ranked rows first.
	Results(ctx context.Context, day contracts.Day, strategy string) ([]Row, error)
}
type Store interface { Writer; Reader; Close() error }
type OpenFunc func(ctx context.Context, dsn string) (Store, error)
```

Module contracts: the record `StrategyResult` of dataset `strategy.result r1` (generated; `internal/contracts/api.go` gains the alias).

Module jobs, `internal/jobs/optioncandidates/types.go`: `CandidateStores` gains two optional fields, `Strategy strategy.Strategy` and `Results strategyresult.Writer`; both nil is today's job, exactly.

## Issues

Build order: entries 1, 2 and 3 are the interface entries and land first (they touch disjoint files, so in parallel); 4 needs 2; 5 needs 3 and 1; 6 needs 3 and 4 (its equality test runs the real `shortput`); 7 needs 4, 5 and 6; 8 needs 7 and is held by the dispatcher until `ot strategy-compare` has printed `equal` on every stored day (the window runs it; read-only). Every entry compiles and its tests pass against main. Where a result needs a test run it is in `## Checked by CI`.

| # | entry | module | size | order |
|---|---|---|---|---|
| 1 | dataset (interface) | contracts | S | first |
| 2 | strategy-api (interface) | strategy | M | first |
| 3 | result-api (interface) | store | S | first |
| 4 | short-put | strategy | L | after 2 |
| 5 | result-store | store | L | after 1, 3 |
| 6 | job-shadow | jobs | M | after 3, 4 |
| 7 | ot-wire | ot | M | after 4, 5, 6 |
| 8 | cutover | ot | S | after 7 |

### 1 dataset (interface)

Module: contracts. Size: S.
Order: first.
Files: `contracts/datasets/strategy.result.r1.json`, `contracts/generate.sh`, `docs/contracts/datasets/strategy.result.md`, `internal/contracts/api.go`, `internal/contracts/gen/records.go`, `internal/contracts/strategy_result_test.go`
Design: docs/design/strategy-framework.md#Interface (Decisions 4 and 5)
The pattern is #1411 (three datasets of the option open pull: the same six kinds of file). `internal/contracts/api.go` is edited for one alias; its other declarations stay. `gen/records.go` is produced by `contracts/generate.sh` and not edited by hand.
Acceptance:
1. `strategy.result.r1.json` declares the 15 columns of `strategyresult.Row` (session_day, run_id, strategy, rule_version, param_hash, key, underlying_id, status, rank, rank_in_underlying, legs, checks, facts, computed_at, settings_version), `x-dataset` `strategy.result`, `x-revision` `r1`, `x-primary-key` `["session_day","run_id","strategy","key"]`; `rank`, `rank_in_underlying` and `settings_version` may be null, no other column may; `status` is the enum ok, blocked, unknown; the description says insert-only and that a rerun is a new run id.
2. `generate.sh` lists the dataset, `gen/records.go` holds the generated `StrategyResult`, `internal/contracts/api.go` gains `type StrategyResult = gen.StrategyResult` and nothing else, and `docs/contracts/datasets/strategy.result.md` states the columns and the key.
3. `strategy_result_test.go` gains the new tests `TestStrategyResultRecordRoundTrip` (a record marshals and unmarshals equal; a null `rank` is accepted; a null `strategy` is refused; `t.Parallel()`).
4. The change edits only the paths of this entry's Files; the pull request body carries a `Contracts:` line naming `contracts/datasets/strategy.result.r1.json` as changed.
5. The result matches docs/design/strategy-framework.md, Decisions 4 and 5.

### 2 strategy-api (interface)

Module: strategy. Size: M.
Order: first.
Files: `internal/strategy/api.go`, `internal/strategy/api_test.go`, `internal/strategy/doc.go`, `docs/contracts/strategy.md`
Design: docs/design/strategy-framework.md#Interface (Decisions 1, 2, 3, 8, 9)
New package, no existing file changes. `docs/design/modules.md` carries the module row (this design's own change); the issue does not edit it.
Acceptance:
1. `api.go` declares `Strategy`, `Params`, `Snapshot`, `Evaluation`, `Leg`, `Check`, `Position` and `Action` with the fields and method sets of the Interface section, and nothing else: no function body, no variable, no constant that is not a declaration; it imports only `contracts` and `compute`.
2. `api_test.go` gains the new tests `TestStrategyFromBothSides` (a fake `Strategy` is built by one side and an `Evaluate`, `Rank` and `Manage` call by the other; every field of `Snapshot` and `Evaluation` is written and read back; `t.Parallel()`) and `TestStrategyPackageImportsNoStoreJobsPages` (reads the import lines of every non-test file of the package and fails on `internal/store`, `internal/jobs`, `internal/pages`; no network, no database).
3. `docs/contracts/strategy.md` states the interface, that every method is pure, that `Facts` omits an absent figure, that `Key` is unique within a day and strategy, and that `Manage` is declared in step 1 and called from step 2.
4. The change edits only the paths of this entry's Files; the pull request body carries `Contracts: unchanged` and a `Design:` line.
5. The result matches docs/design/strategy-framework.md, Interface.

### 3 result-api (interface)

Module: store. Size: S.
Order: first.
Files: `internal/store/strategyresult/api.go`, `internal/store/strategyresult/api_test.go`, `docs/contracts/store.md`
Design: docs/design/strategy-framework.md#Interface (Decisions 4 and 5)
Pattern: `internal/store/hoursnotice` (#2181: `api.go`, `api_test.go`, a paragraph of `docs/contracts/store.md`). `internal/store/api.go` (800 lines) is not edited.
Acceptance:
1. `api.go` declares `Row`, `Writer`, `Reader`, `Store` and `OpenFunc` as in the Interface section, declarations only, importing `context`, `time` and `contracts`.
2. `api_test.go` gains the new test `TestResultStoreFromBothSides` (a fake `Store` is written through `Writer` and read through `Reader` by a caller; a nil `Rank` stays nil; `t.Parallel()`).
3. `docs/contracts/store.md` gains a paragraph for `strategyresult` that names the table `strategy.result`, the view `rd.strategy_result_r1`, the role `ot_w_candidate`, the insert-only rule and the newest-run read.
4. The change edits only the paths of this entry's Files; `Contracts: unchanged` in the body.
5. The result matches docs/design/strategy-framework.md, Decisions 4 and 5.

### 4 short-put

Module: strategy. Size: L.
Order: after 2.
Files: `internal/strategy/shortput/shortput.go`, `internal/strategy/shortput/map.go`, `internal/strategy/shortput/rank.go`, `internal/strategy/shortput/manage.go`, `internal/strategy/shortput/params.go`, `internal/strategy/shortput/shortput_test.go`, `internal/strategy/shortput/manage_test.go`, `internal/strategy/shortput/rank_test.go`, `internal/strategy/params/params.go`, `internal/strategy/params/params_test.go`
Design: docs/design/strategy-framework.md#Issues (Decisions 7, 8, 9, 10)
`internal/compute/optioncand` is read through `compute.OptionCandidates` only and is not edited; its fixtures (`put_test.go`) are not imported: the new tests build their own `Snapshot` literals and a real `optioncand.New()` rule in the test files (an import of an implementation package from a test file only). No existing identifier changes, so no same-package test or pinned testdata is edited.
Acceptance:
1. `shortput.New(rule compute.OptionCandidates, p compute.CandidateParams, v Values)` returns a `strategy.Strategy` with `ID()` `short_put` and `Version()` `put_v1`; `Params()` holds every key of `compute.CandidateParams` as named text plus `close_mode`, `early_keep` and `early_dte`, and `Hash` from `params.Hash`; a `close_mode` other than `expiry` and `early`, a non-decimal `early_keep` or a non-integer `early_dte` is an error that names the key and never a value.
2. `Evaluate` builds a `compute.CandidateInput` with `Kinds` equal to `KindShortPut` only, from the `Snapshot` (every field the input has; `Issuer`, `UnderlyingID` and `Context` go to `Facts`, see 3), calls the rule, and maps each `Candidate` to an `Evaluation`: one `short` leg of quantity 1 with right `P`, `Key` the instrument id, `Checks` the gates in the rule's order, `Status` of the candidate, `RankInUnderlying` of the candidate and `Rank` 0 (Rank of 4 sets it). `map.go` holds the mapping and the module imports no sibling strategy package.
3. `Facts` holds, as text and only when the figure is present: `issuer`, `underlying_id`, `dte`, `bid`, `ask`, `spot`, `buy_price`, `sale_price`, `break_even`, `annualized_yield`, `delta`, `iv`, `extrinsic`, `marks` (JSON array text in the format `marksText` of `convert.go` writes; `shortput` writes it itself, it cannot import the job package, and entry 6's equality test fails on a byte difference), `fits` (JSON object text in the format `fitsText` writes, same rule), and every key of `Snapshot.Context` (`earnings_state`, `next_earnings`).
4. `rank.go` implements `Rank` with the order of `rankRun` (`internal/jobs/optioncandidates/convert.go:187`): ok rows by `annualized_yield` descending as an exact rational, then `dte`, then strike (the leg's `Strike`) ascending, then instrument id; rows that are not ok keep rank 0; `RankInUnderlying` is left as `Evaluate` set it.
5. `manage.go` implements `Manage`: with `close_mode` `expiry` it answers `{hold, ""}` for every input; with `early` it answers `{close, "early_keep"}` when `(OpenPrice - ask) / OpenPrice` is at least `early_keep` at the ask of the position's contract in `Snapshot.Quotes`, else `{close, "early_dte"}` when the days from `RunDay` to `Expiry` are at most `early_dte`, else hold; a missing quote with the DTE rule not reached is hold; exact arithmetic, no float.
6. `params.Hash` returns the SHA-1 hex of the sorted `key=value` lines joined by newlines, standard library only; the same map gives the same hash in any insertion order.
7. The tests, all new and `t.Parallel()`: `TestShortPutEvaluateMapsEveryColumn` (a table of at least six snapshots: ok, each gate blocked, unknown, no buy price, no account, in-the-money; every `Facts` key of item 3 and every leg field is compared with the literal expected value written in the test for each case); `TestShortPutRankEqualsRankRun` (the same rows as `Evaluation` and as the job's rows rank identically; the expected order is written out in the test, not computed from `rankRun`); `TestManageHoldIsDefault`, `TestManageEarlyKeep`, `TestManageEarlyDTE`, `TestNewRefusesBadValues`, `TestParamsHashIsOrderFree`.
8. The change edits only the paths of this entry's Files; `Contracts: unchanged`; the body names `internal/strategy/shortput` as an adapter with the line "own implementation, because the rule exists in compute and must not change in step 1".
9. The result matches docs/design/strategy-framework.md, Decisions 7 to 10.

### 5 result-store

Module: store. Size: L.
Order: after 1 and 3, and after entry 2 of `docs/design/data-repair-bars.md` (PR 2213, migration `0041_price_bar_repair.sql`).
Depends: the migration of that entry is merged first; goose refuses a lower version that appears after a higher one was applied, so `0042` lands after `0041` and neither file is renumbered.
Files: `migrations/0042_strategy_result.sql`, `internal/store/strategyresultpg/store.go`, `internal/store/strategyresultpg/candidates.go`, `internal/store/strategyresultpg/store_test.go`, `internal/store/strategyresultpg/candidates_test.go`, `internal/store/strategyresultpg/store_pg_test.go`
Design: docs/design/strategy-framework.md#Issues (Decisions 4, 5, 6)
Pattern: `internal/store/hoursnoticepg` and `internal/store/candidate`. `internal/store/setup` is not edited (roles and lines exist). The pg test carries the build tag `pg` and runs only in GitHub CI.
Acceptance:
1. `0042_strategy_result.sql` creates schema `strategy` and table `strategy.result` with the 15 columns of dataset entry 1 (`session_day date`, `rank`, `rank_in_underlying` and `settings_version` nullable, the JSON texts `text NOT NULL`), primary key `(session_day, run_id, strategy, key)`, owner `ot_owner`; the view `rd.strategy_result_r1` passes every column through; `GRANT INSERT, SELECT` on the table and `SELECT` on the view to `ot_w_candidate`, `SELECT` on the view to `ot_ro_pages`, `USAGE` on both schemas as `0018_candidates.sql` does; no grant to `ot_runner`; a `-- +goose Down` that reverses each statement. No other file under `migrations/` changes.
2. `store.go` defines `Open(ctx, dsn) (strategyresult.Store, error)`; `InsertResults` inserts with `ON CONFLICT DO NOTHING` on the key, returns the count inserted, and with no rows makes no database call; `Results` reads the view for the day and strategy, keeps the rows of the newest `computed_at` (as `candidate.newestRun` does), in the order of `internal/store/candidate/read.go:14` (rank ascending with unranked last, then the numeric instrument id of the key, then its alias, then run id); an empty day gives an empty slice, never nil; a zero day or an empty strategy is an error; a database error is `contracts.Retryable`, a constraint violation `contracts.Permanent`.
3. `candidates.go` defines `NewCandidateReader(res strategyresult.Reader, old store.CandidateReader) store.CandidateReader`, which answers `Candidates(ctx, day, kind)` for kind `short_put` from the rows of strategy `short_put` when the result store holds a run of that day, and hands every other kind (`covered_call`, `held_call_guard`) and every day with no result run to `old` unchanged (pages look back 7 days and the result table starts with the shadow: `option_candidates_page/groups.go`, `option_notices_page/inputs.go:74`, `option_tracking_page/context.go:41`), and rebuilds every `store.Candidate` column from the row (the columns of the row, the leg for expiry, strike, multiplier, instrument id and right `put`, and `Facts` for the rest, absent key meaning nil pointer); and `Compare(old store.CandidateReader, new strategyresult.Reader, day) ([]string, error)`, which returns one line per difference, comparing the rows as a sequence in the order above, naming the key and the column and, for the `fits` column, no value, and an empty slice when the two agree on every column except `RunID` and `ComputedAt`.
4. `store_test.go` (an in-memory fake of the database seam, `t.Parallel()`, the tests named here are new) names `TestInsertResultsNoRowsNoCall`, `TestResultsNewestRunWins`, `TestResultsRankedFirst`, `TestResultsRefuseZeroDay`; `candidates_test.go` (new, `t.Parallel()`) names `TestCandidateReaderRebuildsEveryColumn` (a row built from a full `store.Candidate` by the test's own packer comes back equal, a nil pointer column stays nil), `TestCandidateReaderDelegatesOtherKindsAndPreShadowDays` (a `covered_call` read and a day with no result run each return exactly what a fake `old` returns and call the result reader not at all; a day with a result run for `short_put` calls `old` not at all), `TestCandidateReaderOrderIsRankThenId` (unranked rows last, ties by id then alias), `TestCompareNamesTheDifferingColumn`, `TestCompareEmptyWhenEqual`; `store_pg_test.go` (tag `pg`, new, `t.TempDir` not needed) names `TestStrategyResultRolesAndView` (the writer inserts and reads; `ot_ro_pages` reads the view and cannot insert; `ot_runner` cannot read).
5. The change edits only the paths of this entry's Files; the pull request body carries `Contracts: unchanged` (the dataset is entry 1's) and names the migration.
6. The result matches docs/design/strategy-framework.md, Decisions 4 to 6.

### 6 job-shadow

Module: jobs. Size: M.
Order: after 3 and 4.
Files: `internal/jobs/optioncandidates/shadow.go`, `internal/jobs/optioncandidates/shadow_test.go`, `internal/jobs/optioncandidates/types.go`, `internal/jobs/optioncandidates/job.go`, `internal/jobs/optioncandidates/calls.go`, `internal/jobs/optioncandidates/job_test.go`, `internal/jobs/optioncandidates/calls_test.go`, `internal/jobs/optioncandidates/types_test.go`, `internal/jobs/optioncandidates/need_test.go`, `docs/contracts/jobs/option_candidates.md`
Design: docs/design/strategy-framework.md#Issues (Decisions 1, 3, 4, 6)
The same-package tests that build `CandidateStores` literals (`job_test.go`, `calls_test.go`, `types_test.go`) are listed because the struct gains two fields; they keep compiling with named fields, and only the new tests below are added to them. `need_test.go` gains the registry test. `job.go` is 325 lines and `calls.go` 281.
Acceptance:
1. `types.go` adds to `CandidateStores` the fields `Strategy strategy.Strategy` and `Results strategyresult.Writer`, and a comment that both nil is the job of today; `internal/jobs/api.go` is not edited.
2. `shadow.go` defines one function used by `Run` of `job` and of `callsJob`: when `Strategy` and `Results` are both set it builds a `strategy.Snapshot` from the same values that built the `compute.CandidateInput` of the underlying (`Issuer`, `UnderlyingID`, and `Context` with `earnings_state` and `next_earnings` from `chainEarnings`), calls `Strategy.Evaluate`, collects the evaluations of the run, calls `Strategy.Rank`, converts each to a `strategyresult.Row` (`Strategy`, `RuleVersion` and `ParamHash` from the strategy, `RunID`, `ComputedAt` and `SettingsVersion` copied from the old rows of the same run after `setRunID` and the settings stamp (in `callsJob` the id covers puts, calls and guards together; the shadow is called after it is set), `Legs`, `Checks`, `Facts` as JSON text with sorted keys) and calls `InsertResults` once per run after the old insert; an error of the shadow is logged with `slog` (the code, never a value) and does not fail the run or change its `JobResult`. With either field nil, no call is made.
3. `job.go` and `calls.go` change only to call that function at the two places where the put rows are complete; the old path, `toStoreRow`, `rankRun`, `runIDOf` and every row they write are unchanged.
4. `shadow_test.go` (fakes of the package, `t.Parallel()`, the tests named here are new) names `TestShadowRowsEqualOldRows` (a fake chain, earnings and account for three underlyings with ok, blocked and unknown rows; after `Run` the fake `Results` holds one row per old `short_put` row and `candidatereader` of entry 5 is not imported: the test rebuilds each old column from the row's own fields and compares to the old row column by column except `RunID` and `ComputedAt`), `TestShadowRunsInBothJobs` (the same on `NewWithCalls`: the put rows equal those of `New`, the covered-call and guard rows are not in `Results`), `TestShadowFailureDoesNotFailTheRun` (a `Results` that errors: `JobResult` equals the one without it and the old rows are written), `TestNilStrategyIsTodaysJob` (both nil: the old rows and the `JobResult` equal the ones of the existing tests' expectations, no call to a fake), and `TestEveryStrategyHasANeedRow` in `need_test.go` (a registered strategy id without a `[[need]]` row is an error; the `covered_call` row without a registered strategy is not).
5. `docs/contracts/jobs/option_candidates.md` states the two fields, the shadow, that it never fails a run, and that the old table is read until cutover; the change edits only the paths of this entry's Files; `Contracts: unchanged`.
6. The result matches docs/design/strategy-framework.md, Decisions 1, 3, 4 and 6.

### 7 ot-wire

Module: ot. Size: M.
Order: after 4, 5 and 6.
Files: `cmd/ot/strategy_wire.go`, `cmd/ot/strategy_compare.go`, `cmd/ot/strategy_wire_test.go`, `cmd/ot/strategy_compare_test.go`, `cmd/ot/candidates.go`, `cmd/ot/runonce.go`, `cmd/ot/wire.go`, `cmd/ot/wire_test.go`, `cmd/ot/candidates_test.go`, `cmd/ot/buildcause_test.go`, `cmd/ot/env_test.go`, `docs/contracts/ot.md`
Design: docs/design/strategy-framework.md#Issues (Decisions 1, 6, 11)
`cmd/ot/wire.go` is 795 lines: this entry adds at most the two lines of the `deps` field and its default and no more; if it needs more it reports it (AGENTS.md, Size). `candidates_test.go` builds a `jobFactory` and gains the new tests. `jobFactory` is declared in `runonce.go` (line 274) and gains one field.
Acceptance:
1. `strategy_wire.go` defines `buildStrategies` (the one place the list of strategies is built: `shortput.New` over `optioncand.New()` and the manifest's `[params]` read the way `optioncandidates` reads them) and `openStrategyResults`, which opens `strategyresultpg.Open` with `OT_PG_DSN_CANDIDATE`; `candidates.go` `optionCandidates()` sets `Strategy` and `Results` of `CandidateStores` from the factory, and `candidateJob` opens the results store beside the candidate store, adds it to `extra` closers, and leaves both nil when the candidate DSN is empty; a failed open of the results store is an error that names the key `OT_PG_DSN_CANDIDATE` and never a value.
2. `strategy_compare.go` registers the command `strategy-compare [--day DAY]` (default: the newest session day): it opens `OT_PG_DSN_PAGES` (role `ot_ro_pages`; both views are granted), calls `strategyresultpg.Compare` with the old reader (`openCandidateStore` of `wire.go`, unchanged) and the result reader (`strategyresultpg.Open`), prints `equal` and the number of rows, or one line per difference, and exits non-zero on a difference; the demo profile is refused as `candidates` refuses it; it writes nothing.
3. `strategy_wire_test.go` and `strategy_compare_test.go` (fakes, `t.Parallel()`, the tests named here are new) name `TestOptionCandidatesGetsTheStrategyAndResults`, `TestNoCandidateDSNLeavesBothNil`, `TestStrategyIdsHaveNeedRows` (every registered id is in the `strategy` column of `optioncandidates.Needs()`), `TestStrategyCompareEqualPrintsEqual`, `TestStrategyCompareDifferenceExitsNonZero`, `TestStrategyCompareDemoRefused`, and `TestShadowThroughRealReaderEqualsOldRows` (the real `optioncandidates` job with the real `shortput` and fakes of the stores runs once; the rows its fake `Results` captured go through the real `strategyresultpg.NewCandidateReader`, and every field of every `store.Candidate` it returns, `RunID` and `ComputedAt` included, equals the row its fake `CandidateWriter` captured; the fakes are in the test file, no database). `env_test.go` gains the `leakExcluded["strategy-compare"]` entry with its reason, as `candidates_test.go:30` does for `candidates`.
4. `docs/contracts/ot.md` names the command and its two outputs; the change edits only the paths of this entry's Files; `Contracts: unchanged`.
5. The result matches docs/design/strategy-framework.md, Decisions 1, 6 and 11.

### 8 cutover

Module: ot. Size: S.
Order: after 7. Held by the dispatcher until `ot strategy-compare` printed `equal` for every stored day since the shadow started.
Files: `cmd/ot/wire.go`, `cmd/ot/strategy_wire.go`, `cmd/ot/strategy_wire_test.go`, `cmd/ot/candidates_site_test.go`
Design: docs/design/strategy-framework.md#Issues (Decision 6)
Every page and command gets its `store.CandidateReader` from one seam, `deps.openCandidate` (`candidates.go:150`, `candidates_site.go:74`, `notices_site.go:123`, `option_page_site.go:88`, `site.go:297`), whose default is `openCandidateStore` (assigned at `wire.go:312`, defined at `wire.go:423`). The cutover changes that default only; no page package and none of those five files is edited. `wire.go` is 795 lines: the body of `openCandidateStore` is replaced by a call into `strategy_wire.go`, so the file does not grow.
Acceptance:
1. `strategy_wire.go` defines `openResultCandidateStore(ctx, dsn) (candidateStore, error)`: it opens `strategyresultpg.Open` with the given DSN and returns a `candidateStore` whose `Candidates` is `strategyresultpg.NewCandidateReader(result store, old reader opened with the same DSN by `openOldCandidateStore`)` and whose `Close` closes both; a failed open is a nil interface, not a typed nil. `wire.go` assigns it as the default of `d.openCandidate`; the old `openCandidateStore` body moves to `strategy_wire.go` as `openOldCandidateStore`, which `strategy-compare` of entry 7 keeps using; the old table, its writer, `toStoreRow` and `rankRun` are not edited.
2. `strategy_wire_test.go` gains the new tests `TestDefaultCandidateOpenerReadsResultTable` (`deps` built by the default wiring with fake stores: a `short_put` read of a day with a result run answers the result rows) and `TestCutoverFallsBackToOldRows` (a `covered_call` read, a `held_call_guard` read, a `short_put` read of a day with no result run, and a `short_put` read of a day whose result insert failed each answer the rows of the fake old reader and none of the result reader's) and `candidates_site_test.go` the new test `TestCandidatesPageSameRowsFromEitherReader` (the same rows through either reader give equal page JSON).
3. The change edits only the paths of this entry's Files; `Contracts: unchanged`.
4. The result matches docs/design/strategy-framework.md, Decision 6.

## Checked by CI

`go build ./...`, `go test ./internal/strategy/... ./internal/store/... ./internal/jobs/... ./cmd/ot/... ./internal/contracts/... -race -count=1`, the `pg` tagged tests of `internal/store/strategyresultpg` (GitHub CI only), `go run ./scripts/arch` (the new module row and its edges: strategy imports contracts and compute; jobs and ot import strategy; store imports no strategy), `golangci-lint`, the contracts generation check (entry 1: `gen/records.go` equals the generator's output) and the `web/` checks (no file of `web/` changes). Results only a run shows: that the shadow rows equal the old rows on real days is shown by `ot strategy-compare`, read-only, run by the window after the migration is applied and one run of `option_candidates` has written both tables; entry 8 is held on it. The migration is run by the window after merge (rebuild `ot`, `setup pg`, migrate, `setup pg`).

## Left out

- **Step 2, replay and the choice of the close rule.** Its own design, next. What it needs from this one is declared here: `Manage` and the three parameters (Decision 9), `Snapshot` as the one input of live and replay, `Params.Hash` and `rule_version` on every row, and the result table (a replay row is a row with its own `strategy` suffix or a second table; decided there). It is not in this design because (a) the owner's 197 lots (117 short puts) have no stored chain on their entry days (chains exist from 2026-11-07; `vol-edge.md`: delta and IV at entry cannot be rebuilt), so the replay of the close rule needs a repricing step first (Black formula on a forward from put-call parity, which `option-capture-need.md` Left out already assigns to a compute step beside the replay) and an own implementation or a library decision for it; (b) the first honest output is a coverage count (how many lots have a stored path, how many are modelled) that decides what the page may claim; (c) that makes at least five more issues, over the limit of this design. Owner decisions already given: hold to expiry is the default and the early close is run beside it and compared (P4).
- **Step 3, account fit and the whole-book stress at the FINRA moves** (equity and narrow index ±15%, broad high-cap index +6% / -8%, other broad indexes ±10% [V]; margin formulas of the Cboe page and rule 4210 [V]). Its own design: it reads `account.position` and the held options through `compute.OptionStructures`, needs a beta or index-proxy rule for the single names (method [NF], trial), and moves the funding gate (`fundingGate` in `put.go`) out of the put, which would change `put_v1`. It does not depend on step 2.
- Moving `compute.OptionCandidates`' covered call and held-call guard behind `Strategy` (step 4 of the plan); the put credit spread, the iron condor, the strangle (steps 5 to 7).
- Deleting `candidates.option_candidate`, `toStoreRow`, `rankRun`, `runIDOf` of the job and the old reader after cutover: one owner-run migration and one issue, after the pages have run on the result table for a time the owner picks.
- Moving `Need` into the interface (Decision 1 keeps the `[[need]]` rows in the manifest and tests the pairing); the capture bounds as the union of the needs (step 0 Left out).
- `Snapshot.Readings` (`iv_rv20`, `vol_edge`, the market state tag): added by the first strategy that gates on one, with that strategy's issue.
- Buy price history: a replay needs the buy price of the day (today only the current `watch.toml` value exists); until the valuation band is stored per day the replay uses today's and says so.
- Open owner question (step 2): how many of his lots are short puts. The brief says about 197 short-put trades; `vol-edge.md` says 197 short lots, of which 117 are puts and 51 covered calls. The ledger gives the count; step 2 asks him which set he means (puts or lots).
- Alternatives turned down: a column set per strategy (Decision 4); rewriting the put rule inside the new package (Decision 7); overwrite by key (Decision 4, Doubt 2); a new role and DSN line (Decision 5).

## Doubts

1. **Put alone against three kinds in one call.** `NewWithCalls` evaluates three kinds in one call; the shadow evaluates the put kind alone. Read at `optioncand.go:19-55`: each kind is ordered and ranked on its own, so put rows should be the same either way; `TestShadowRunsInBothJobs` (entry 6) proves it, and if it differs the issue reports it and the shadow follows production.
2. **Insert-only against the owner's overwrite rule (S1).** A rerun of the same inputs has the same run id and inserts nothing; a rerun with other inputs adds a run. Alternative: the key without `run_id` and an upsert. Cost of that: the settings version and the "which run was shown" history are lost and equal-to-today is harder to show. Recommended: keep insert-only; the owner can say otherwise.
3. **`Manage` sees one day.** `Manage(Position, Snapshot)` answers for one day; replay calls it day by day over the stored days of a lot, so a path of asks is the caller's loop, not an interface need. If step 2 finds it needs more, it changes the interface in its own first issue. The seam is a bet, not a proof; nothing in step 1 can prove it, and the lots have no stored entry-day chain (Today).
4. **`Version()` against `Params.Hash`.** Both are on every row, but the reader takes the newest run of a day whatever its hash; two parameter sets on one day are mixed only by a replay, which filters on the hash (step 2).
5. **`Context` as a bag of text.** It keeps two job facts out of the rule; a typed field replaces it when a second strategy needs one of them.

## Adversarial

A fresh agent answered six questions (`adversarial-strategy.md` in the scratchpad). It read a copy with `## Issues` duplicated by an editing slip; the duplicate is removed.

1. Gap against the goal (the seam may not be enough for replay; no whole-book interface): adopted in part, as Doubt 3: `Manage` is called day by day by the replay, and step 2 changes the interface in its own first issue if that is not enough. A book-level interface is step 3's.
2. Biggest unnamed risk (the duplicated Issues): adopted, removed.
3. Simpler way (drop shadow and cutover, prove equality by a fixture test): not adopted, and the fixture alone is not enough either: the proof the owner asked for is on his real days, which only the shadow and `ot strategy-compare` give. Both are now in: the new test `TestShadowThroughRealReaderEqualsOldRows` (entry 7) runs the real shadow conversion into the real reader and compares every field with the old row, and `Compare` checks the sequence. Cost: one extra evaluation per run.
4. Lock-in (`Facts` as text, frozen key names, newest run mixing parameter sets): kept and named: Decision 3 and Doubt 4 say the key names are frozen by the rebuild and that a replay filters on `param_hash`; a typed field replaces a key when a second strategy needs it.
5. Claim most likely false (a need row without a registered id is an error): adopted, the check is one-way (Decision 1, entry 6 item 4, entry 7 item 3); `covered_call` has a row and no registered strategy.
6. Likeliest slip: adopted. Entry 6 is after 3 and 4; the shadow runs after `setRunID` and copies `RunID`, `ComputedAt` and `SettingsVersion`; `buildcause_test.go` is in entry 7's Files; entry 4 item 7 compares literals written in the test and says `shortput` writes the `marks` and `fits` JSON itself; the `fits` compare line carries no value. Not adopted: the hold on entry 8 stays a dispatcher state, written under Order, because no diff can show a run of `ot`.
