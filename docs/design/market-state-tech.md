# market state, step 1: the tech layer reads NDX against SPX and shows one Chinese sentence on the home card (trial)

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-13
Revision: 2

Source: owner 2026-11-13, approval of the market-state proposal (advisor, asks 3756, 3761, 3763), step 1 "tech layer: NDX vs SPX, home row (trial)"; owner decisions of the same day: (a) the conclusion sentence of this layer is Chinese ("3. 中文"), conclusion first with numbers behind a tap, a reading and not a forecast, thresholds are trial until a replay. Advisor review r1 of Revision 1 (decisions binding): the sentence is `card.notes[0]`; sentence plus a grey English line is accepted until step 4; the sentence is Chinese and all texts live in `techsay`; the numbers are trial `TechParams` fields; the types stay in `blockapi`; the candidates mark and its wire are cut.

Goal: from the stored daily closes of NDX and SPX the system computes once per session a tech state, LEADING, IN LINE, LAGGING or LAGGING HARD, from the 1-month and 3-month return of NDX minus that of SPX. The state is stored with its numbers as a reading, one per day, so a later replay (step 2) can set the thresholds. The home market card shows one Chinese sentence as its first note and one grey English line of numbers as its second. Every threshold is labelled trial. It is not a forecast and it hides, sorts and blocks nothing.

Existing design: none owns this. `market-bars.md` stores the index bars; `market-rv20.md` and `compute-stage2.md` (iv_history) are the precedents for a step that reads index bars and for a function passed as an argument; `ui-english.md` fixed English pages (decision 1 is amended here, see Decision 9); `report-global-assets.md` is the shape of this file.

## Today

All paths at origin/main (621bd17b).

- Bars. `internal/jobs/barsrefresh/job.toml` has `[[index]]` SPX and NDX; `backfill_days = 400`; the daily pull asks 5 days; only basis `trades` is stored. Every `[[index]]` becomes an instrument of type `index` named by its symbol. Both series are stored today and no collector is needed. The bars are index levels (price return, no dividends).
- Readers. `store/bar.PriceBarReader.PriceBars(ctx, id, basis, from, to)` returns winner rows ascending by day (`internal/store/bar/api.go:26-32`). The names SPX and NDX become ids through `computefigures.IndexIDReader` (`types.go`), held in `computefigures.ComputeStores` next to `PriceBars`; `cmd/ot` opens both for compute_figures.
- compute_figures already stores market-kind readings (`kindMarket`, `internal/jobs/computefigures/subjects.go:19`), runs a step on index closes (`runRv20`, `rv.go:27`: it resolves names through `IndexIDs`, an unknown name is Retryable, an ambiguous one Permanent), stores through one function (`runState.store`, `steps.go:252`, which skips an equal state, reason and `rows_sha256` and else writes the next rev) and is already an any-day job in the console. Its plan is `job.Plan` (`job.go:193`), its key routing `job.Run` (`job.go:247`). Its manifest `job.toml` holds text only (`TestManifestHasNoNumbers`, `job_test.go:396`). `NewWithValuation` (`valuation.go:46`) is the precedent of a step that is added to the base job and plans its own keys.
- Readings. `readings.reading.value` is `numeric(38,12)` (`migrations/0013_readings.sql:15`) and the dataset pattern of `value` is decimal text (`contracts/datasets/readings.reading.r1.json:19`): a reading cannot hold the text LEADING or IN LINE as its value. `readings.reading_parameter.value_tag` is free text (`migrations/0013_readings.sql:73`), so the state name, with its space, is stored there. The unit `text_tag` is in the enum; `valuation.diagnosis` is the precedent of a reading with a numeric value and tag parameters (`valuation.go:303`). `origin` is the enum `compute_figures` or `owner_command` (`docs/contracts/datasets/readings.reading.md`); a step of compute_figures writes `compute_figures`. A reading record carries `Inputs` (`InputTable`, `Keys`, `CodeVersion`, `ConventionVersion`) and `Parameters` of `{param, value_num, value_tag}`.
- Page blocks. The home market member `internal/pages/home_market/member.go` has `Config.Readings compute.ReadingFunc` (`:43-47`), writes `card.notes` (list of strings of 1 to 200 characters, at most 20, schema `page.market_summary.v3.json`; the card has one note today) and `card.gaps`; `ivrv.go` is the precedent for a stored reading with a data day: `ivRvMaxAgeDays = 5`, stale reason code 3, `dataDayOf`. The limits of the member are `freshDays = 4` (level series, `member.go:38`) and `readingDays = 5` (`:39`, the window `newestReading` reads). `web/src/pages/home/CardNotes.tsx` shows each note as grey text. The schema has no field for a state with evidence and the schemas are pinned (`contracts/schemas/carryover.sha256`).
- English. `cmd/ot/english_test.go` walks the non-test `.go` files of `internal/pages`, `internal/compute/pagefig` and `cmd/ot` and fails on any CJK character outside the exact strings of `allowedCJK` (empty today); `internal/compute` outside `pagefig`, `internal/jobs` and `internal/store` are not walked. `web/test/english.test.ts` guards `web/src` only. `ui-english.md` item 10: no persisted row holds Chinese text produced by our code, so the sentence is written by a page package and is not stored.
- Register. `docs/conventions/register.toml` is in the docs row of `docs/design/modules.md` (`docs/**`), not owner-only; a figure entry has 15 keys (`TestRegisterEntriesComplete`); `TestNoPendingForGoIds` demands `pending-recompute` for every figure that is not in `rules.json`, and no test reads `implemented_by` of such a figure (precedent: `household.movers`, `docs/design/household-movers.md`).
- Sizes. `internal/compute/api.go` is 839 of its ceiling 840 and is not touched; `internal/compute/blockapi/api.go` is 283 lines; `cmd/ot/wire.go` is 795 lines and is not touched; `internal/jobs/computefigures/job.go` is 361 lines; `home_market/member.go` is 515 lines.

## Decisions

1. Reading. One reading per session day: name `market.tech_state` version 1, kind `market`, subject `NDX_SPX`, unit `text_tag`, key `market.tech_state|market|NDX_SPX`. The value is the rank of the state as decimal text: LEADING `1`, IN LINE `0`, LAGGING `-1`, LAGGING HARD `-2`. The state name is the tag parameter `state` (`LEADING`, `IN LINE`, `LAGGING`, `LAGGING HARD`). It is a description of the last 3 months, not a forecast, and the register says so. A change of a threshold is a new reading version; old rows stay.
2. Inputs. The `trades` closes of the instruments named `NDX` and `SPX` (type index) from 140 calendar days before the as-of day to the as-of day. The common days are the days on which both have a close, newest first; c0 is the newest common day. A day that only one series has is skipped, so both returns count the same sessions.
3. Returns. For n = 21 and n = 63 sessions (common days) `ret_n(X) = (c0 / c_n - 1) * 100` of NDX and of SPX, exact rationals (`math/big`, no float); `gap_n = ret_n(NDX) - ret_n(SPX)` in percentage points. Texts are rounded half-even to 2 places only for the stored evidence; the state is decided on the exact values. Named limit: index levels exclude dividends (price return); the yield difference of the two indexes is small against the thresholds and is not adjusted.
4. Raw state of a day, with the trial constants `Band` 1.00 and `Hard` 5.00 percentage points: LEADING when `gap_21 >= Band` and `gap_63 >= Band`; LAGGING HARD when `gap_21 <= -Band` and `gap_63 <= -Hard`; LAGGING when `gap_21 <= -Band` and `gap_63 <= -Band` (and not hard); otherwise IN LINE. The proposal gave only the shape; the two numbers are the designer's trial values and nothing supports them yet (Doubt 1). The sector-rank condition of the proposal waits for sector funds (Left out).
5. Two closes to change. The published state of c0 is a fold over the raw states of the newest up to `Fold` = 10 common days that have 64 closes behind them (oldest first): it starts as the first raw state and takes a raw state only when that raw state equals the previous day's raw state. The raw state of c0 is stored too (`raw`). No previous reading is read: the state is a function of the stored bars alone, which is what the replay needs.
6. Trial constants. `Band`, `Hard` and `Fold`, with the windows 21 and 63 and `MaxAgeDays` 5, are the fields of `blockapi.TechParams`, set in one function `techstate.TrialParams()` and nowhere else; they are not in `job.toml` (the manifest holds text only), not in `rules.json` (no hash of a threshold is re-derived for a trial) and the register row says trial. The replay (step 2) changes them there, with a new reading version. The stored evidence carries the values the run used.
7. Gaps. No instrument for NDX or SPX: the key fails Retryable and stores nothing (as `runRv20`; bars_refresh may create the instrument later in the day); an ambiguous name is Permanent. Stored as a `missing_input` reading with no value: no close at all, reason `no_input` (code 1); fewer than 64 common days, `short_history` (2); c0 more than `MaxAgeDays` = 5 calendar days before the as-of day, `stale_input` (3); a close that is not a positive decimal, `bad_input` (5). Never a made-up state.
8. Evidence, as parameters of the reading, all `value_tag` text so that a page shows them without arithmetic: `state`, `raw`, `data_day`, `ndx_1m`, `spx_1m`, `gap_1m`, `ndx_3m`, `spx_3m`, `gap_3m` (percent, 2 places, signed), `band`, `hard`, `sessions_1m` = 21, `sessions_3m` = 63, `trial` = `true`. `origin` is `compute_figures`; `inputs` is one entry with table `obs.price_bar` and the keys `<id of NDX>|trades` and `<id of SPX>|trades`; `rows_sha256` is `hashLines` (`steps.go:93`) of the lines `NDX|<day>|<close>` and `SPX|<day>|<close>` of every close read; `code_version` is a hash of id, version and the trial constants, `convention_version` is `tech_state_v1`. The existing `runState.store` writes it, so the same day with the same content is not stored twice.
9. Language. The Chinese texts are exactly four sentences, written in one new non-test file `internal/pages/techsay/say.go` (package `techsay`) and nowhere else, so that a switch of language is one edit:
    - LEADING: `科技近 1 个月、3 个月都跑赢大盘（试行，不是预测）。`
    - IN LINE: `科技与大盘大体同步（试行，不是预测）。`
    - LAGGING: `科技近 1 个月、3 个月都跑输大盘。科技股的新 put：先等一等，或减量、选更远的行权价（试行，不是预测）。`
    - LAGGING HARD: `科技近 3 个月明显跑输大盘。科技股的新 put：先等一等，或减量、选更远的行权价（试行，不是预测）。`
    The sentences are fixed text keyed by the state; they are not stored (`ui-english.md` item 10) and not composed in the browser. Numbers and labels stay English. This design amends `docs/design/ui-english.md` decision 1 ("English first, no hedge for a second language") for exactly these 4 sentences and for nothing else; the owner's word is 2026-11-13, "3. 中文", and because this design pull request is owner-merged, the merge is the amendment; `ui-english.md` itself is not edited. A later Chinese text needs its own owner word and its own entries.
10. How `cmd/ot/english_test.go` is scoped. `allowedCJK` gets exactly those four strings, with a comment that cites the owner's word (2026-11-13, "3. 中文") and this design's Decision 9. `TestGoTreesAreEnglish`, `guardedTrees` and `cjkRun` keep their logic: a fifth Chinese string anywhere, or a changed character in one of the four, is still a failure. No second test is added; that the four sit in one file is checked by `git grep` (entry 2). The web guard and the rule that no store, job or migration file holds CJK are not changed.
11. Where. A pure function `techstate.Classify` in `internal/compute/techstate` (no I/O, no clock, no float); its types and the function type `TechFunc` are declared in `internal/compute/blockapi/api.go` (an entry file already listed in `docs/design/modules.md`, so no row changes), and the header comment of that file is extended to say it also declares the tech state (the file is named for page figures; a package `techapi` would need an owner-only edit of `modules.md`). The step is a new step of the compute_figures job, not a new job: `computefigures.WithTech(job, TechDeps)` (`TechDeps` in `types.go`, `WithTech` in `tech.go`) returns the job with the key `market.tech_state|market|NDX_SPX` added to its plan and a step after `runMarket` in its run; `TechDeps` holds `Classify blockapi.TechFunc` and `Params blockapi.TechParams`, so the package imports no compute implementation (the iv_history shape). `WithTech` has no error result and returns the job unchanged when `Classify` is nil, when `PriceBars` or `IndexIDs` is nil, or when the job is not a computefigures job; so every `cmd/ot` test that fakes `newComputeFigures` or sets no index reader is unaffected. `cmd/ot/wire_computefigures.go` calls it on the job it returns (on the valuation job when there is one, since the keys of the joined job go to the job that planned them). No job, no console row, no manifest slot, no job document: the run is after bars_refresh at the existing +20m slot, and a past day is recomputed through the console, which already lists compute_figures as any-day.
12. Home row. The market card gets, from the newest `market.tech_state` reading (kind `market`, subject `NDX_SPX`) within `readingDays` = 5 calendar days of the page day, `card.notes[0]` = the sentence of the state and `card.notes[1]` = one English line with the numbers, `NDX minus SPX: 1 month -3.12 pt, 3 months -6.40 pt; trial thresholds, not a forecast`; the existing notes follow. The two are grey notes under the card (`CardNotes.tsx`): the sentence is first, not styled as a headline; the owner accepts this until the Market state page of step 4, where the schema is versioned once (advisor decision 1; Left out). No reading, a stale or a missing one gives one card gap (`no_data`, `stale_input`, `missing_input`) in English and no note; the gap never changes `run_status`. The stale limit is one number, 5 calendar days: the read window `readingDays`, the job's `MaxAgeDays` and the data-day check of the page (the data day more than 5 days before the page day, or a stored reason code 3) are all 5, as `ivRvMaxAgeDays` is; `freshDays` (4) belongs to the level series and is not used.
13. Left out of the card: any filter, sort, hidden row or changed gate; an order or a size number; a mark on put candidates (Left out).
14. Register. `docs/conventions/register.toml` gets `market.tech_state` v1 in its own entry, after entry 1 so that the function it names exists: formula of Decisions 3 to 5 with the trial constants, the words `trial` and `not a forecast` in the formula and the gap text, inputs of Decision 2, `period_date` the data day, gap text of Decision 7, `excludes` any forecast, any adjusted close and any sector fund, `implemented_by` `internal/compute/techstate.Classify`, `pages = ["market_summary.card"]`, `source_hash = "pending-recompute"`, no row in `rules.json`. The row names `Band` 1.00, `Hard` 5.00 and the two-close fold as trial.
15. Contracts unchanged: no schema, dataset or proto file changes. The reading name is new data under the existing dataset `readings.reading`.

## Modules

No row of `docs/design/modules.md` changes. The entries touch: compute (`internal/compute/**`, `docs/contracts/compute.md`), jobs (`internal/jobs/**`, `docs/contracts/jobs/**`), pages (`internal/pages/**`), ot (`cmd/**`, `docs/contracts/ot/**`) and docs (`docs/conventions/**`). New import edges: `internal/jobs/computefigures` imports `internal/compute/blockapi` (jobs depend on compute); `home_market` imports `internal/pages/techsay` and `internal/contracts`; `cmd/ot` imports `internal/compute/techstate` (ot depends on all). No page imports a job or the notifier.

## Interface

Module compute, `internal/compute/blockapi/api.go`; existing declarations stay; the header comment gains "and the tech state".

```go
// TechParams are the trial constants of the tech state: Band and Hard in percentage points as decimal text,
// Short and Long the session counts of the two windows, Fold the sessions of the two-close fold, MaxAgeDays the
// age of c0 in calendar days beyond which the reading is stale. techstate.TrialParams is their one source.
type TechParams struct {
	Band, Hard        string
	Short, Long, Fold int
	MaxAgeDays        int
}

// TechIn is the input of the tech state. Day is the as-of day. NDX and SPX are closes newest first with their
// days (CloseIn).
type TechIn struct {
	Day      contracts.Day
	NDX, SPX CloseIn
	Params   TechParams
}

// TechResult: State is LEADING, IN LINE, LAGGING or LAGGING HARD, empty when Reason is set; Rank is 1, 0, -1 or -2;
// Raw is the raw state of c0; Reason is empty or a missing reason of compute (no_input, short_history,
// stale_input, bad_input); DataDay is the day of c0 (empty when there is none). The six returns and gaps are
// decimal text of percent, 2 places, half-even, signed.
type TechResult struct {
	State, Raw                               string
	Rank                                     int
	Reason                                   compute.Reason
	DataDay                                  contracts.Day
	NDX1m, SPX1m, Gap1m, NDX3m, SPX3m, Gap3m string
}

// TechFunc evaluates the tech state, no I/O, no clock (internal/compute/techstate.Classify).
type TechFunc func(TechIn) TechResult
```

Module jobs, `internal/jobs/computefigures/types.go` (the package's own types file) and `tech.go`:

```go
// TechDeps is what the tech step of the compute_figures job needs. With Classify nil, or PriceBars or IndexIDs
// of the ComputeStores nil, WithTech returns the job as it is.
type TechDeps struct {
	Classify blockapi.TechFunc
	Params   blockapi.TechParams
}

// WithTech adds the key market.tech_state|market|NDX_SPX and its step to a computefigures job.
func WithTech(j contracts.Job, d TechDeps) contracts.Job
```

Module pages, `internal/pages/techsay/say.go`; it is not an entry file (a helper package like `gaps`), so it is declared here for the reviewers:

```go
// View is what a page shows of one stored reading: State empty and Gap set when there is nothing to show.
type View struct {
	State, Gap, Say, Numbers string
}

// FromReading reads a stored market.tech_state reading for the page day: the state is its `state` tag. A data day
// more than 5 calendar days before day, or a stored reason code 3, is stale_input; a reading that is not ok, has no
// state tag or an unknown one is missing_input; no reading is no_data. Say is the Chinese sentence of Decision 9;
// Numbers is the English line of Decision 12.
func FromReading(r *contracts.ReadingRecord, day contracts.Day) View
```

`Contracts: unchanged`.

## Issues

Build order: every entry compiles and its tests pass against main. BASE is `git merge-base origin/main HEAD`. Entry 2 comes before entry 3, because `TestGoTreesAreEnglish` fails on the Chinese file otherwise. Where a result needs a test run, it is in `## Checked by CI`, not in an acceptance list.

### 1 tech-state-api (interface)

Module: compute. Size: M.
Order: first.
Files: `internal/compute/blockapi/api.go`, `internal/compute/blockapi/api_test.go`, `docs/contracts/compute.md`, `internal/compute/techstate/techstate.go`, `internal/compute/techstate/techstate_test.go`
Design: docs/design/market-state-tech.md#Issues (Decisions 2 to 6, 11; Interface)
Acceptance:
1. `api.go` declares `TechParams`, `TechIn`, `TechResult` and `TechFunc` with the fields of the Interface section, no function body, and its header comment names the tech state; `git diff BASE -- internal/compute/api.go internal/pages internal/compute/pagefig` is empty.
2. `api_test.go` gains a new test that fills every new field through a fake `TechFunc`, reads each back, and checks that `techstate.Classify` is assignable to `TechFunc`, with `t.Parallel()` (the contract is tested from both sides).
3. `techstate.go` is under 300 lines, uses `math/big` and no float, imports no package of `internal/store`, `internal/jobs` or `internal/pages`, and defines `Classify`, `TrialParams` and an unexported `ratText` that rounds half-even to 2 places (`git grep -n "^func \(Classify\|TrialParams\|ratText\)(" -- internal/compute/techstate` prints three lines); `TrialParams` returns Band `1.00`, Hard `5.00`, Short 21, Long 63, Fold 10, MaxAgeDays 5. `docs/contracts/compute.md` lists the new types and `techstate.Classify` under Page figures, with the tests of item 4.
4. `techstate_test.go` (the tests named here are new, all `t.Parallel()`) names `TestTechLaggingBothWindows`, `TestTechLaggingHardOnLongGap`, `TestTechLeadingMirror`, `TestTechInLineInsideBand`, `TestTechBandEdgeIsExact` (a gap of exactly -1.00 is LAGGING, -0.999... is IN LINE, decided on the exact values and not the rounded text), `TestTechOneRawDayDoesNotChangeState`, `TestTechTwoRawDaysChangeState`, `TestTechUsesCommonDaysOnly` (a day only NDX has is skipped), `TestTechRankOfEachState`, `TestTechFewerThan64CommonDaysIsShortHistory`, `TestTechStaleAfterMaxAge`, `TestTechBadCloseIsBadInput`, `TestTechNoClosesIsNoInput`, `TestTechEvidenceTextsRoundHalfEven`.
5. `git diff --name-only BASE` lists only paths of the Files line.

### 2 tech-state-english

Module: ot. Size: S.
Order: first (before 3).
Files: `cmd/ot/english_test.go`
Design: docs/design/market-state-tech.md#Issues (Decisions 9, 10)
Acceptance:
1. `allowedCJK` holds exactly the four sentences of Decision 9 (each of the four strings of Decision 9 occurs once in the file: `git grep -c -F '<sentence>' -- cmd/ot/english_test.go` prints 1 for each), and its comment contains the text `2026-11-13` and `3. 中文` and `docs/design/market-state-tech.md`.
2. `TestGoTreesAreEnglish`, `guardedTrees` and `cjkRun` are unchanged: `git diff BASE -- cmd/ot/english_test.go | grep '^-' | grep -v '^---'` prints only the old `allowedCJK` declaration lines; no new `func Test` is added (`git diff BASE -- cmd/ot/english_test.go | grep -c '^+func Test'` prints 0).
3. `git diff BASE -- web contracts internal` is empty; `git diff --name-only BASE` lists only paths of the Files line.

### 3 tech-state-say

Module: pages. Size: S.
Order: after 2.
Files: `internal/pages/techsay/say.go`, `internal/pages/techsay/techsay_test.go`
Design: docs/design/market-state-tech.md#Interface and Decisions 8, 9, 12
Acceptance:
1. `say.go` holds the four Chinese sentences of Decision 9 as constants and no other CJK character (`git grep -n -P '[\x{4e00}-\x{9fff}]' -- internal/pages/techsay/say.go` prints exactly four lines), `FromReading` and the type `View` of the Interface section; it imports only `internal/contracts` and the standard library, uses no float, no `time.Now` and no arithmetic on the evidence (the numbers are the stored tags put into the English line of Decision 12), and is under 200 lines.
2. Each of the four sentences occurs in no other non-test `.go` file: `git grep -l '科技近 1 个月\|科技与大盘\|科技近 3 个月' -- internal cmd ':!*_test.go' ':!cmd/ot/english_test.go'` prints only `internal/pages/techsay/say.go`.
3. `techsay_test.go` (package `techsay_test`, the tests named here are new, all `t.Parallel()`) names `TestFromReadingEachState` (the four state tags give their sentence), `TestFromReadingNumbersLine` (the line is `NDX minus SPX: 1 month -3.12 pt, 3 months -6.40 pt; trial thresholds, not a forecast` for stored tags -3.12 and -6.40), `TestFromReadingNilIsNoData`, `TestFromReadingOldDataDayIsStale`, `TestFromReadingReasonThreeIsStale`, `TestFromReadingNotOkIsMissingInput`, `TestFromReadingUnknownStateIsMissingInput` and `TestSentencesAreShort` (every sentence is 1 to 200 characters).
4. `git diff --name-only BASE` lists only paths of the Files line; `git diff BASE -- cmd internal/pages/home_market` is empty.

### 4 tech-state-step

Module: jobs. Size: M.
Order: after 1.
Files: `internal/jobs/computefigures/tech.go`, `internal/jobs/computefigures/tech_test.go`, `internal/jobs/computefigures/job.go`, `internal/jobs/computefigures/job.toml`, `internal/jobs/computefigures/types.go`, `internal/jobs/computefigures/types_test.go`, `docs/contracts/jobs/compute_figures.md`
Design: docs/design/market-state-tech.md#Issues (Decisions 1, 2, 7, 8, 11)
The callers and same-package tests of the changed identifiers were searched at origin/main: `manifest` and `Plan`/`Run` of `job.go` are used by `job_test.go`, `manifest_rv20_test.go`, `ivrv_test.go` and `hv_test.go` of the package, which only decode the manifest or call `Plan`/`Run` on a job without the tech step; with no `WithTech` call the plan, the run and the plan hash are those of today, so none is edited.
Acceptance:
1. `job.toml` gains exactly two text keys, `tech = "market.tech_state"` and `tech_indices = ["NDX", "SPX"]`, and no number (`TestManifestHasNoNumbers` stays as it is); `job.go` adds the two manifest fields and, in `Plan` and `Run`, the key and its routing before the `known && kind == kindMarket` case, and stays under 420 lines.
2. `types.go` declares `TechDeps` as in the Interface section; `tech.go` defines `WithTech` and (under 250 lines) the step, holds no float, no `math/big` and no CJK, and imports `internal/compute/blockapi` and no other package of `internal/compute` than `compute` itself (`git grep -n "internal/compute/" -- internal/jobs/computefigures/tech.go` prints no line other than `blockapi` and `compute`).
3. With a tech step, `Plan` lists the one key `market.tech_state|market|NDX_SPX` more and a hash that covers the id, the version and the trial constants; without one, `Plan` is as before. `Run` reads the `trades` closes of both indices from 140 calendar days before the day, gives them to `Classify` and stores one reading as Decision 8 says: value the rank as decimal text, unit `text_tag`, the parameters of Decision 8 as `value_tag`, `origin` `compute_figures`, the input table `obs.price_bar` and `rows_sha256` of the closes read. For a `Reason` it stores `missing_input` with the code of Decision 7 and no value. An unknown index name is Retryable, an ambiguous one Permanent, a reader error Retryable; the run never stores a state it was not given; the reading goes through `runState.store`.
4. `WithTech` returns its job unchanged when `Classify` is nil, when `PriceBars` or `IndexIDs` is nil, or for a job of another type, and has no error result.
5. `tech_test.go` and `types_test.go` (fake readers and writer, a fake `TechFunc`, `t.Parallel()`, no sleep; the tests named here are new) name `TestWithTechAddsOneKey`, `TestWithTechNilClassifyIsSameJob`, `TestWithTechNoIndexReaderIsSameJob`, `TestWithTechForeignJobIsSameJob`, `TestTechStoresRankStateAndEvidence`, `TestTechStoresGapAsMissingInput`, `TestTechSameDayTwiceStoresOnce`, `TestTechUnknownIndexIsRetryable`, `TestTechAmbiguousIndexIsPermanent`, `TestTechReaderErrorIsRetryable`, `TestTechReadsOneHundredFortyDays` and `TestTechRowsHashIsTheCloses`; `types_test.go` gains `TestTechDepsFromBothSides` (new: a caller fills `Classify` and `Params`, the constructor reads them back).
6. `docs/contracts/jobs/compute_figures.md` gains the tech step: the key, the manifest keys, the rank and the `state` tag, the gaps and that the sentence is not stored; `git diff --name-only BASE` lists only paths of the Files line.

### 5 tech-state-wire

Module: ot. Size: S.
Order: after 1 and 4.
Files: `cmd/ot/wire_computefigures.go`, `cmd/ot/wire_computefigures_test.go`, `docs/contracts/ot/jobs/compute_figures.md`
Design: docs/design/market-state-tech.md#Issues (Decision 11)
`cmd/ot/wire.go` and `internal/compute/api.go` are not edited. The tests of `cmd/ot` that fake `newComputeFigures` (`buildcause_test.go`, `jp_wire_test.go`, `runonce_lease_test.go`, `runonce_option_test.go`, `runonce_test.go`, `scheduler_test.go`, `wire_computefigures_test.go`) return jobs that are not computefigures jobs, and `WithTech` returns those unchanged; the one test that sets the index reader, `index_ids_wire_test.go`, builds no job. The exact job-list tests (`jp_wire_test.go`, `preflight_test.go`, `optionledger_test.go`, `wire_test.go`) are not affected because no job is added. `newDeps` sets `openIndexIDs` to `openIndexIDsDefault` (`wire.go:343`) and `TestNewDepsFilled` skips that field (`wire_test.go:21`); with no index reader the step is left out, which is the case in the rigs.
Acceptance:
1. `wire_computefigures.go` has a function `techDeps()` that returns `computefigures.TechDeps{Classify: techstate.Classify, Params: techstate.TrialParams()}` and `computeFiguresOf` calls `computefigures.WithTech` on the plain job it returns when there is no fact store, and on the valuation job before `withEase` when there is one; the file stays under 200 lines.
2. `wire_computefigures_test.go` gains the new tests `TestTechDepsAreTheTrialOnes` (`Classify` is not nil and `Params` equals `techstate.TrialParams()`) and `TestComputeFiguresWithoutIndexReaderIsNotWrapped` (the job built with no index reader is the one the fake constructor returned).
3. `docs/contracts/ot/jobs/compute_figures.md` states that the step is wired when both readers exist and is left out otherwise.
4. `git diff BASE -- cmd/ot/wire.go cmd/ot/site.go cmd/ot/english_test.go internal` is empty; `git diff --name-only BASE` lists only paths of the Files line.

### 6 tech-state-register

Module: docs. Size: S.
Order: after 1.
Files: `docs/conventions/register.toml`
Design: docs/design/market-state-tech.md#Issues (Decision 14)
Acceptance:
1. `register.toml` has the entry `market.tech_state` version 1 with the 15 keys of the other entries (`id`, `version`, `title_zh`, `pages`, `formula`, `inputs`, `period_date`, `currency`, `excludes`, `gap_text`, `implemented_by`, `source_hash`, `agreed_on`, `agreed_in`, `history`), `pages = ["market_summary.card"]`, `source_hash = "pending-recompute"`, `agreed_in` naming `docs/design/market-state-tech.md`, `implemented_by = ["internal/compute/techstate.Classify"]` (`git grep -n "^func Classify(" -- internal/compute/techstate` prints one line).
2. The `formula` states Decisions 3 to 5 with `Band` 1.00, `Hard` 5.00 and the two-close fold of 10 sessions each marked trial, and the words `trial` and `not a forecast`; the `inputs` name the NDX and SPX `trades` closes; the `currency` text says the value is the rank 1, 0, -1 or -2 with the state name in the `state` tag; the `gap_text` states the four reasons of Decision 7; the `excludes` name any forecast, adjusted closes and sector funds.
3. `git diff BASE -- docs/conventions/register.toml | grep '^-' | grep -vc '^---'` prints 0 and only that entry is added; `internal/compute/rules/rules.json` and `docs/design` are not edited; `git diff --name-only BASE` lists only `docs/conventions/register.toml`.

### 7 tech-state-home

Module: pages. Size: M.
Order: after 3.
Files: `internal/pages/home_market/techstate.go`, `internal/pages/home_market/techstate_test.go`, `internal/pages/home_market/member.go`, `internal/pages/home_market/member_test.go`, `internal/pages/home_market/testdata/ok.golden.json`
Design: docs/design/market-state-tech.md#Issues (Decisions 9, 12)
`Config` is not edited: `cmd/ot/home_site.go` builds it with `Readings: read` and needs no change. The golden and `member_test.go` change because a reading source with no `market.tech_state` row now adds one card gap. The callers of `home_market.Build` were searched: `cmd/ot/home_site.go` only, and `internal/pages/home_page` composes the card by schema without a fixture of this member.
Acceptance:
1. `techstate.go` reads `market.tech_state` for kind `market` and subject `NDX_SPX` through `Config.Readings` over the last `readingDays` = 5 calendar days of the page day (newest day, highest revision), passes it to `techsay.FromReading` and returns the notes and the gap; it holds no CJK (`git grep -n -P '[\x{4e00}-\x{9fff}]' -- internal/pages/home_market` prints no line outside test files), no float and no arithmetic, and the stale limit of the page is the constant `readingDays`, not `freshDays` (`git grep -n freshDays -- internal/pages/home_market/techstate.go` prints no line).
2. `member.go` makes `card.notes[0]` the sentence and `card.notes[1]` the numbers line when the view has a state, the existing notes after them, and appends to `card.gaps` one gap of the view's reason with an English text under 200 characters otherwise; the gap does not change `run_status`; the other notes, gaps and cells are unchanged, `SchemaVersion` stays 3, and `member.go` stays under 560 lines.
3. `techstate_test.go` and `member_test.go` (the tests named here are new, `t.Parallel()`) name `TestTechNotesAreFirstAndInOrder` (sentence at index 0, numbers line at index 1, the old note at index 2), `TestTechNotesForEachState`, `TestTechNoReadingIsNoDataGap`, `TestTechStaleReadingIsStaleGap`, `TestTechGapKeepsRunStatus`, `TestTechReaderErrorIsError`, `TestTechNoteTextsFitTheSchema` (every note is 1 to 200 characters and the whole card validates against `page.market_summary.v3.json`); `ok.golden.json` is regenerated with the gap line and the existing expectations are otherwise unchanged.
4. `purity_test.go` is not edited; `git diff --name-only BASE` lists only paths of the Files line.

## Checked by CI

`go test ./internal/compute/... ./internal/jobs/computefigures/... ./internal/pages/... ./cmd/ot/... -race -count=1`, `go run ./scripts/arch --check`, `go build ./...`, `golangci-lint`, the register test of `internal/compute/rules` (entry 6: keys complete, `pending-recompute` demanded and present), the contracts generation check (no change expected) and `pnpm` tsc, eslint, vitest and build of `web/` (no file of `web/` changes; the check is that `web/test/english.test.ts` still passes). Results that only a run shows: the English guard passes with the four sentences present in `say.go` and fails on a fifth Chinese string (entries 2 and 3); the home card built with a stored reading validates against `page.market_summary.v3.json` and shows the sentence first (entry 7); the step stores one reading per day and not two, and a reading with value `-2` and the tag `LAGGING HARD` is accepted by the readings store tests (entry 4); every `cmd/ot` test that builds compute_figures still passes with the step left out (entry 5). The review runs no test.

## Left out

- The numbers behind a tap, as a collapsible on the home card, and the sentence styled as a headline: they need a schema version of `market_summary` and of `home_page` (the schemas are pinned), a web change and fixtures. They are carried by the Market state page of step 4; this cut shows the sentence as the first grey note and the numbers as the second (advisor decision 1).
- The candidates mark `科技落后` and its wire, the tech name list (`techsay.Names`), the mark string and the optional reader method of the candidates page: cut by the advisor (review r1). They return as their own design after the replay over 400 days; the use case, a tech put the owner is about to enter, is not served by this cut and the home sentence is the only place the state is shown.
- A replay of the stored state over the last 400 days and of the owner's own puts (step 2): a read-only report that calls `techstate.Classify` over stored bars; no backfill job is needed because the state is a function of the bars. Until then the history starts on the day the step first runs.
- Sector funds (XLK, SMH against SPY, rank among 11 sectors), QQQ, RSP: the sector funds of `report-global-assets.md` are stored, but the rank rule and SMH are step 4.
- Telegram on a change, the morning report lines: the stored reading has the state per day, so a notifier is a later job over it. The 11 market alerts are not touched.
- The market regime and story-heat layers, the sentence for a new put as the worst of three layers, a composite score, a position size or delta number, a news feed.
- A total-return series for NDX and SPX (dividends); a filter or sort by the state; hysteresis other than the two-close fold of Decision 5.

## Doubts

1. `Band` 1.00, `Hard` 5.00 percentage points and the two-close fold are the designer's trial numbers; the proposal gave only the shape. They are not backtested and the state can flap near the band, which the fold only dampens. The replay (step 2) is the guard; no number here is a claim about the market, and the sentence says trial and not a forecast.
2. Unchecked against the database: that SPX and NDX have 64 or more common days of `trades` bars on the owner's deploy (the first pull is 400 calendar days, so it should), and that the IBKR `trades` bars of the two indexes agree on days. Fewer days store `short_history` and the card says so.
3. Around long holidays the 140-day read gives fewer than 10 raw days for the fold (it never fails, it folds what exists), and the evidence does not say how many raw days were used.
4. `WithTech` sets the step on a built job by a type assertion on the package's own types (as `NewWithValuation` does for its base); a new wrapper type in `cmd/ot` that is not a computefigures job would silently get no step. The wrapper of today, `easeJoined`, is built after the call.

## Adversarial

Source: scratchpad file adversarial-market-state-tech.md (the full answer); one line per point.

- 1 Gap against the goal: adopted in part. Hole A (numbers behind a tap): accepted until step 4 (advisor decision 1, Decision 12, Left out). Hole B (the mark misses names outside the list): the mark and the list are cut with entries 8 and 9 (advisor), named in Left out with the use case not served. Hole C (history starts at first run): stated in Left out.
- 2 Biggest risk, origin, inputs and rows_sha256: adopted. Decision 8 states `origin` `compute_figures`, the `obs.price_bar` input with the two keys and `rows_sha256` of the closes; checking it found a second problem of Revision 1, that `value` is numeric, so the state is a tag and the value is the rank (Decision 1).
- 3 Simpler way, a step and not a job: adopted. Verified against code: the plan, key routing, `runState.store`, `IndexIDs`/`PriceBars` and the any-day console row exist (Today, Decision 11); no job, wire file, console row or job document remains; 9 entries became 7 (the two cut by the advisor, the job entry and the job wire replaced by entries 4 and 5). The one-file test of Revision 1 is dropped as suggested (Decision 10).
- 4 Lock-in: adopted. The owner's word is in the `allowedCJK` comment and Decision 9 states that `ui-english.md` decision 1 is amended for these 4 sentences by the owner-merged design; the version bump on a threshold change is Decision 1; `blockapi` stays by advisor decision 5 with its header comment extended (Decision 11).
- 5 Claim most likely false (cmd/ot rigs, `text_tag`, stale days): adopted. The `openIndexIDs` dependency is named in entry 5 and disappears as a risk because `WithTech` is a no-op without the index reader; `text_tag` was checked and drove the rank value; the three stale numbers are one, 5 (Decision 12).
- 6 Likeliest slip, `TestEveryForceNoJobHasReason` at 19: not applicable, no job and no console row are added, so `jobconsole_jobs.go` and its test are untouched; entry 2 item 2 of Revision 1 is rewritten as `git grep` and `git diff` checks (entry 2); `cmd/ot/home_site_test.go` was searched and has no card gap assertion.
- a (19 count in `jobconsole_jobs_test.go:90-91`): moot, see 6. b (origin, inputs, rows_sha256): adopted, see 2. c (a compute_figures step): adopted, see 3. d (one stale-days number): adopted, 5 days (Decision 12). e (grep-able entry 2, owner word in the comment): adopted. f (the tech list from the owner): moot, the list is cut with entry 8 (Left out). g (the `openIndexIDs` dependency and the exact job-list tests): adopted, named in entry 5, no longer a risk.
