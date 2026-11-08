# Compute figures: the readings store, the rules table, the register entries and the figures job

Author: the optdesk designer
Owner approved: (pending)
Revision: 4

Source: the owner's decisions of 2026-11-07 and 2026-11-08 (order of work compute, then risk and alerts, then strategy; a mature library wherever one exists; own glue code, even thin, sits behind an interface so a library can replace it), the scheduler design `docs/design/scheduler.md` rev 8 (pull request 335 of andrewrong/optdesk, head b348fd7, not on main yet), `docs/research/go-quant-ecosystem.md` (on main, #343), the restart plan v2.5 rev 2.1 text (cited P:<line>), schema v3 of the database (section 3.9, `readings.reading`; a text the repository does not hold), the compute design adversarial review, rev 1 of the four files (kept by the dispatcher, not committed; REV1:<finding>) and the review of rev 2 of the first draft (ADV:<line>).
Read on main at af7e583: `AGENTS.md`, `docs/design/modules.md`, `docs/GLOSSARY.md`, `docs/conventions/register.toml` (header, all 44 entries), `docs/engineering.md`, `docs/contracts/jobs.md`, `internal/clock/api.go`, `internal/store/api.go`, `internal/store/setup/setup.go`, `migrations/0004` to `0011`, `internal/jobs/api.go`, `internal/contracts/api.go`, `contracts/datasets/obs.option_eod.r1.json`, `cmd/ot/wire.go`, `cmd/ot/runonce.go`, `cmd/ot/site.go`, `contracts/fixtures/MANIFEST.synthetic-v1.json`, `internal/contracts/schema_test.go`, `scripts/hygiene/main.go`.
One of four designs: `compute-core.md` (the pure package), `compute-fred-series.md` (series stored; migration 0014), this file, `compute-calendar.md` (the calendar refresh; owns the clock interface issue of the differ and its own jobs constructor type).
Order of the four: core, figures, FRED, calendar. The whole chain after the core lands after the scheduler design has merged: its issues `sched-store-ops` (migration 0012), `sched-runner-interface` (declares `Postcondition` and the plain-value methods) and `sched-jobs-schedule` (the `[schedule]` tables) are named in the Depends of the issues that need them. Nothing here is cut before they are on main.

Goal: turn the series and chains the stores hold into stored figures, once a day, with the rule that made each one. Each figure is a `readings.reading` row of schema v3 and has an entry in the register; nothing is a loose float.

## What main holds today

- No `readings` schema, no `rd.reading_*` view, no role `ot_w_readings` (`setup.go` has a `roles` slice without it, `roleOwner` to `roleMaster`); the last migration is `0011`; `0012_scheduler.sql` comes with pull request 335. Schema v3 section 3.9 defines `readings.reading(name, version, subject_kind, subject, day, rev, value numeric(38,12), unit reading_unit, currency, state, reason_code smallint, origin, cutoff, job_run_id uuid, recorded_at, rows_sha256, PK(name,version,subject_kind,subject,day,rev))`, children `reading_input`, `reading_input_key`, `reading_parameter`, `reading_row` and the winner view `rd.reading_r1` (highest `rev` of state `ok`; `missing_input` yields). `reading_unit` is `decimal, percent, vol_point, per_calendar_day, count, days, money, ratio, text_tag`. The repository holds none of this text: `store-reading-interface` writes the key, the winner rule and the enum into `docs/contracts/store.md`, so a reviewer can check them (REV1:F2).
- `docs/conventions/register.toml`: 44 `[[figure]]` entries, all version 1, every one with the same 15 keys (`id, version, title_zh, pages, formula, inputs, period_date, currency, excludes, gap_text, implemented_by, source_hash, agreed_on, agreed_in, history`); `implemented_by` is a list of Python names; `source_hash` is `"pending-recompute"` in all 44 (P:251). The header names `tests/conventions/test_register.py`; main has no `tests/` directory and no code that reads the register, so nothing checks an entry today. Rounding has no key: it is prose in `formula` and `currency` (register.toml:25 "rounded half-even to 2 places"). The plan names the rule hash as a stored column (P:140, P:226, P:338).
- `market.sp500_vs_200d` (register.toml:1115-1143): v1 formula is the IBKR SPX close spliced with FRED SP500, the 200-day mean of the spliced closes, the difference shown as a percent with one decimal, half up; the stored FRED-only reading is the fallback when no IBKR bar exists. History says "FRED lagged 4 sessions" (register.toml:1112, 1142).
- `store` has the pattern `OpenXFunc func(ctx, dsn) (XStore, error)` with `XStore` = Reader + Writer + `Close()` (store/api.go:155-181, 294-320). Readers on main: `OptionEodReader.OptionEod(ctx, underlyingID, from, to)`, `OptionEodCatalog.OptionEodLatest(ctx, since)`, `OptionTermsReader.OptionTerms(ctx, ids)` (at most 500 ids; expiration date, right, strike text, settlement type; no expiry time), `InstrumentNameReader.InstrumentNames(ctx, ids)`, `SeriesReader.SeriesPoints`, `SessionDayReader.SessionDays`. `cmd/ot/site.go:80-103` opens the option_eod, catalog, terms and names readers with the line `DSNEnvPages`.
- `obs.option_eod` r1 (contracts/datasets/obs.option_eod.r1.json) has `instrument_id`, `underlying_id` (int64), `status` (1 ok), `bid`, `ask`, `iv_decimal`, `received_at`, `underlying_price`, and no strike, right or expiry.

## Decisions

1. Storage is schema v3's `readings.reading`, with these departures, each named: (a) no `alias` foreign key yet (`subject` is a text id); (b) `reading_row` is not created (rows are named by `rows_sha256` and `reading_input_key`; added when a page needs a drill-down); (c) `job_run_id` is a deterministic uuid made like `jobs.CaptureID` from job id, as-of day and slot (no `ops.job_run` on main); (d) `cutoff` is the text `session_close` (owner 2026-11-08: "a").
2. No `derived.figure_value` float64 table (a parallel store, ADV:49). A value is the text of a decimal from `Units.RoundHalfEven(v, scale)`, scale 6 for every figure here; a missing figure has `value` null, state `missing_input`, `reason_code` 1 to 5 (the core's five reasons in the order of `compute-core.md` Decision 2: 1 `no_input`, 2 `short_history`, 3 `stale_input`, 4 `zero_range`, 5 `bad_input`). A stale input is therefore stored as `missing_input` with `reason_code` 3; the dataset enum has no `stale_input` state.
3. A reading carries: `rows_sha256` (sha256 of the sorted input rows in a fixed text form), `reading_input.code_version` (the rule hash), `convention_version`, and `reading_parameter` rows `data_day`, `observations`, `stale_rate` (Iv30 only), `window`, `years`. Both hashes are stored: the rule hash says which code made it, the input hash decides whether a rerun changes anything.
4. Insert-only. The writer inserts with `ON CONFLICT DO NOTHING` on the key with `rev`; the job reads the newest rev of the key, skips an equal state and `rows_sha256`, else writes `rev + 1`. Inputs are insert-only, so a rerun with the same rule cannot turn an `ok` into missing; a changed rule that does is hidden by v3's winner rule behind the earlier `ok` (the job logs it); kept, it is v3's rule (REV1:F2).
5. Register entries. Eight new ids, each a `[[figure]]` with all 15 keys: `id`, `version = 1`, `title_zh`, `pages = []` (no page reads them yet), `formula`, `inputs`, `period_date`, `currency` (the sentence "Unit <word>, stored as decimal text, rounded half-even to 6 places." with a capital U, as the entries and `TestRulesMatchRegister` (a case-sensitive regexp `^Unit (\w+), stored as decimal text, rounded half-even to (\d+) places\.`) have it, the word being the id's unit below), `excludes`, `gap_text`, `implemented_by`, `source_hash = "pending-recompute"`, and `agreed_on`, `agreed_in` carried by the issue text (the owner's approval date of this design and its path; a worker writes no agreement of its own). The ids: `market.iv30_own` (GLOSSARY.md:76), `market.ivp_own` (`ivp_v1`, GLOSSARY.md:78), `market.ivr_own`, `market.vix_term_ratio` (VIXCLS over VXVCLS), `market.vix_pctile_3y`, `market.vxn_pctile_3y`, `market.rvx_pctile_3y`, `market.sp500_pctile_3y` (the `N225_IV` rule of GLOSSARY.md:87: strictly below, fixed per entry). Units, words of the `reading_unit` enum of schema v3 (the same word is `Rule.Unit` in `rules.json` and `reading.unit` of the stored row): `market.iv30_own` `decimal` (IV as a decimal: 0.2 is 20 vol points); `market.ivp_own` `decimal` (a share of sessions, 0 to 1); `market.ivr_own` `decimal` ((x-min)/(max-min), 0 to 1); `market.vix_term_ratio` `ratio`; the four `*_pctile_3y` `decimal` (a share of days, 0 to 1); `market.sp500_vs_200d` v2 `ratio`. `market.hv30_own` is not registered: no dataset holds closes, so it has no producer (REV1:14); `HistoricalVol` stays in the core, tested. `implemented_by` of a Go figure is `internal/compute/figures.(*calc).<Method>` and, after it, the helpers of the same package the method calls (`internal/compute/figures.strikeStar`, ...).
6. `market.sp500_vs_200d` becomes version 2, a new `[[figure]]` beside v1, which is left as it is. This is not a refactor: v2 changes the source (FRED SP500 only; no IBKR splice), the formula (the mean of the last 200 FRED values, every one present), the unit (`ratio`: 0.0312 is +3.12%; the page makes the percent) and the rounding (half-even at 6 places, not half up at one decimal of a percent). The register header makes that a version bump with the advisor's agreement. The agreement recorded for the first draft ("refactor, no version bump") was given on the wrong premise and is withdrawn. Advisor: agrees to a new v2 [[figure]] beside v1 (2026-11-08). v1 is untouched; v2 has `pages = []` until a page design cites it by id and version; its unit is `ratio`, its scale 6 half-even. The issue text carries that entry's `agreed_on` and `agreed_in` (the owner's approval of this design; REV1:5). `Rules` is keyed by id and version and `rules.json` holds only the Go-implemented entries, so v1 (Python) is not in it.
7. Age of an input (`MaxAgeDays`, REV1:R2): 4 calendar days for IVP and IVR, the figures on stored own IV30 (GLOSSARY.md:72), and 4 for `market.iv30_own`, whose `MaxAgeDays` in `rules.json` (landed in #481) is 4 and applies to its one stored-series input, the `DGS3MO` rate (the option rows are of the as-of day itself); 8 for every figure on a FRED series other than that rate. A `DGS3MO` point older than 4 calendar days before the as-of day gives a missing IV30 with `stale_input`; a point within 4 days that is not of the as-of day gives an `ok` IV30 with the parameter `stale_rate` true (as `figures.Iv30` landed in #480: `StaleRate` is `rate.Day != asOf`). So `stale_rate` is the flag of a rate of 1 to 4 days old, `stale_input` the missing of a rate older. The landed `Iv30` does not check the age itself (it sets `DataDay` to the as-of day); `compute-figures-job` (`ivInput`) compares the rate point with `Rule.MaxAgeDays` before the call and writes the missing figure. The job runs at `session_close` plus 20 minutes with as-of = the slot day, but FRED publishes later: a VIXCLS point of today does not exist yet, and "FRED lagged 4 sessions" (register.toml:1112, 1142) is up to 6 calendar days, 8 across a three-day weekend with a holiday. A figure stays `ok` with its `data_day` parameter row saying which day it used; past 8 it is missing with the reason `stale_input`, stored as state `missing_input`, `reason_code` 3 (Decision 2).
8. IVP rule, flagged for the owner: GLOSSARY.md:78 (`ivp_v1`, at or below, 252 valid sessions, today excluded) wins over P:84 (strictly below); the legacy rule is not produced.
9. One job `compute_figures`, ordered steps in one package: IV30 first, then the market figures, then IVP and IVR from the stored IV30 readings, so a missing IV30 makes a missing IVP with `no_input`. A step that fails does not stop the others; its keys are written missing or stay absent, and the postcondition names them. Keys are `{readings.reading, "name|subject_kind|subject"}`; `subject_kind` is `underlying` (subject = the symbol of `InstrumentNames`, else `underlying:<id>`) or `market` (subject = the FRED id of the first input). The as-of day is the slot day; `Postcondition`: every planned key has a reading row for the as-of day (ok or missing). The slot is proposed `session_close` plus 20 minutes (`[schedule]` table of `sched-jobs-schedule`), the owner confirms.
10. Iv30 input mapping (REV1:8), one place, the files `ivinput.go` and `subjects.go` of `compute-figures-job`. Subjects: `Plan` takes the candidates from `OptionEodCatalog.OptionEodLatest(ctx, day)`, which returns every underlying with a row on or after the day (its `SessionDay` is the greatest day it has, `store/api.go:99`), then keeps those for which `OptionEod(ctx, id, day, day)` returns a row of the day itself (an underlying with only later rows is dropped and counted), and names them with `InstrumentNames`; `job.toml` holds rule ids and market figures only, never the held names. Rows: per underlying `OptionEod(ctx, id, day, day)`; of one `instrument_id` the row with the latest `received_at`; only `status` 1 is used. Contract data: `OptionTerms` in chunks of 500 ids gives the right, strike (decimal text to float64) and `ExpirationDate`; a row without a current terms version is dropped and counted. Per strike and right: the mid is (bid+ask)/2 when both are present, `ask >= bid` and `ask > 0`, else that side is absent (`HasCall` or `HasPut` false); the IV is `iv_decimal`, absent or not above 0 gives 0 (no model IV). Minutes: `T` is measured from the close instant of the as-of day's session row (`SessionDayReader`, calendar `XNYS`) to the expiry instant of the expiration day's session row: its `close_at`, or its `open_at` when the terms' settlement type is `AM` (any case); an expiration day that is not a session day drops that expiry; the minutes are floored. The rate is the newest `DGS3MO` point not after the as-of day (`SeriesOps.FromSeriesPoints`, then `Units.PercentToDecimal`); none gives a missing `no_input`; a point more than `MaxAgeDays` (4, Decision 7) calendar days before the as-of day gives a missing `stale_input`; a point of an earlier day within 4 days is used and `stale_rate` is true. Landed artefacts disagree: `rules.json` says 4 for `market.iv30_own`, the register `inputs` text says "no older than 8 calendar days", and `Iv30` checks no age. This design takes `rules.json` (4). The register sentence is wrong, and `compute-figures-job` cannot edit it (module jobs; `docs/conventions/register.toml` belongs to docs). The issue `docs-iv30-rate-age` (entry 9, module docs, before `compute-figures-job`) makes the `inputs` and `gap_text` of `market.iv30_own` say 4 with `stale_rate` and `stale_input`; the Iv30 sentence of `docs/contracts/jobs.md` (module jobs) is in the Files of `compute-figures-job`.
11. `docs/engineering.md` lines 7 to 11 (QuantLib, IBKR-IV, arch, hand-formulas, bump-and-reprice) are rewritten by `docs-compute-rules`: gonum and go-talib behind interfaces, IV only from IBKR model values except the glossary's own IV30, figures stored as readings with a rule hash. The wording is the owner's to approve.
12. The rule hash (REV1:F2, narrower than it reads): the sha256 of the go/printer-normalized source of every function listed in `implemented_by`, in order, followed by the canonical JSON of the rule's numbers without the hash. It does not cover the library version; `rows_sha256` and `convention_version` cover the rest.

## Modules

Edits of `docs/design/modules.md` in this design's pull request: the list "one constructor each" (read by `parseImpl`, scripts/arch/main.go:83) gains `internal/compute/rules`, `internal/store/reading`, `internal/jobs/computefigures`; the rows jobs and ot gain `compute` in depends; the row store gains the readings tables in its prose. The same one-line list is edited by the other three designs and by pull request 335 (`internal/runner/sched`): each rebase is a hand merge of that line, merged in the order of the four designs. `internal/contracts/api.go` is in the Files of `reading-dataset` (Interface, contracts). Not in the list: `internal/compute/contracttest` (a test kit, `compute-core.md` Decision 11).

The wire (`compute-jobs-wire`) calls these constructors, each in a package that the list of `ot` must name: `reading.Open` (`internal/store/reading`, this design), `rules.New` (`internal/compute/rules`, this design), `computefigures.New` (`internal/jobs/computefigures`, this design), `figures.New`, `own.NewUnits`, `own.NewSeriesOps`, `own.NewRolling` and `gonumstats.New` (`internal/compute/figures`, `own`, `gonumstats`: added to the list by `compute-core.md`, which merges first). `internal/compute/talibind` is not wired. The wire is cut only after the whole core (`compute-interface` to `compute-figures-iv`, `compute-own-glue`, `compute-gonum-stats`) and every issue before it in this file are on main; its Depends names them.

Size of `docs/conventions/register.toml`: 1509 lines on main, about 1780 after the nine entries of `docs-compute-rules` (nine tables of about 30 lines). `scripts/ratchet` counts only source files (`.go .py .ts .tsx .js .mjs .sh`), so a TOML file has no ceiling in `docs/design/ratchet.json` (the file is not on main) and none is needed; no split is made. If the reviewer reads the size rule of AGENTS.md as covering data files, the ceiling is the owner's to add: this design does not raise one.

| # | issue | module | order |
|---|---|---|---|
| 1 | reading-dataset (interface) | contracts | first of the generated-file editors (before `series-unit-index-points`) |
| 2 | store-reading-interface (interface) | store | after 1 (reading-dataset) |
| 3 | store-reading-tables | store, plus repo (judged: setup roles, `.go-arch-lint.yml`); second-opinion case | after 2 (store-reading-interface), and sched-store-ops |
| 4 | docs-compute-rules (interface) | docs | after the `compute-core.md` interface; carries v2 of Decision 6 (the advisor agreed) |
| 5 | compute-rules | compute, plus docs (judged) | after 3, 4 (store-reading-tables, docs-compute-rules), and compute-figures-series and compute-figures-iv (core) |
| 6 | jobs-compute-api (interface) | jobs | after 2 (store-reading-interface), and compute-interface (core), sched-runner-interface, sched-jobs-schedule |
| 7 | compute-figures-job | jobs, plus repo (judged: `.go-arch-lint.yml`) | after 3, 5, 6 (store-reading-tables, compute-rules, jobs-compute-api) |
| 8 | compute-jobs-wire | ot; second-opinion case | after 7 (compute-figures-job), and sched-cmd-run and the core issues |
| 9 | docs-iv30-rate-age | docs | after 4, 5 (docs-compute-rules, compute-rules), before 7 (compute-figures-job) |

The numbers are the entries of `## Issues` in this file; a name without a number is an issue of another design. Two entries that share a file name each other's number: `register.toml` (4, 5, 9), `.go-arch-lint.yml` (3, 5, 7).

## Interface

- contracts: dataset `readings.reading` r1 (`contracts/datasets/readings.reading.r1.json`): one record is the reading plus `inputs` (array of `{input_table, code_version, convention_version, keys}`) and `parameters` (array of `{param, value_num, value_tag}`); `value` is a string `^-?[0-9]{1,26}(\.[0-9]{1,12})?$` or null; `state` `ok` or `missing_input`; `reason_code` 1 to 5 or null; `origin` `compute_figures`; `rev` at least 1; `unit` the v3 enum. The Go name of the record is `contracts.ReadingRecord`: `internal/contracts/api.go` gains `type ReadingRecord = gen.ReadingRecord` (an alias with a doc comment, like `OptionEod`), and `generate.sh` names the root type (`--schema-root-type "$D/readings.reading.r1.json=ReadingRecord"`).
- store (the pattern of `SeriesStore`, `OpenSeriesFunc`): `ReadingWriter` (`InsertReadings(ctx, []contracts.ReadingRecord) (inserted int, err error)`), `ReadingReader` (`LatestReadings(ctx, name, subjectKind, subject string, from, to contracts.Day) ([]contracts.ReadingRecord, error)`: the newest rev of each day in the range, any state, ascending by day), `ReadingStore` (both and `Close() error`), `OpenReadingFunc func(ctx context.Context, dsn string) (ReadingStore, error)` (the implementation is `internal/store/reading.Open`) and `DSNEnvReadings = "OT_PG_DSN_READINGS"`. One store, one DSN, as `SeriesStore`: role `ot_w_readings` writes and reads; the pages read `rd.reading_r1` under their own role in a later design.
- register (interface issue `docs-compute-rules`): the nine `[[figure]]` tables and the glossary lines are the text that `compute-rules` implements and `TestRulesMatchRegister` compares with `rules.json`; the issue is an interface issue of this cut because the code must match what it fixes.
- `Rules` of the core, implemented by `internal/compute/rules` (`rules.New`, embedded `rules.json`).
- jobs: `internal/jobs/api.go` gains `ComputeStores{Catalog store.OptionEodCatalog; OptionEod store.OptionEodReader; Terms store.OptionTermsReader; Names store.InstrumentNameReader; SessionDays store.SessionDayReader; Series store.SeriesReader; Readings store.ReadingStore}` and `NewComputeFiguresFunc func(ComputeStores, compute.Figures, compute.Rules, compute.Units, compute.SeriesOps, clock.Clock) (contracts.Job, error)`; `docs/contracts/jobs.md` documents the job (keys, retry classes: a store error `Retryable`, a bad rule `Permanent`).

## Issues

### reading-dataset (interface)

- Module: contracts. Size: M. Depends on: none.
- Order: first (1) of the issues that edit `contracts/generate.sh`, `internal/contracts/gen/records.go`, `schema_test.go` and the manifest: `series-unit-index-points` (FRED design) depends on it (REV1:15). Interface change: new dataset r1, additive, and the alias `ReadingRecord` in `internal/contracts/api.go`. The `cutoff` text `session_close` is the owner's choice (2026-11-08).
- Design: docs/design/compute-figures.md#Interface. Contracts: readings.reading r1 (new, additive).
- Files: contracts/datasets/readings.reading.r1.json, contracts/generate.sh, internal/contracts/gen/records.go, internal/contracts/api.go, internal/contracts/api_test.go, docs/contracts/datasets/readings.reading.md, internal/contracts/schema_test.go, contracts/fixtures/readings.reading.r1.json, contracts/fixtures/MANIFEST.synthetic-v1.json

- Goal: the dataset of the Interface section; `generate.sh` lists it and names the root type `ReadingRecord`; `api.go` gains the alias `type ReadingRecord = gen.ReadingRecord` and `api_test.go` a test that a `ReadingRecord` is a `gen.ReadingRecord` (`TestReadingRecordAlias`); the fixture holds three synthetic records (an `ok` ivp with two parameters, a `missing_input` with reason 3, a rev 2 of the first); `TestDatasetFixtures` names the dataset (it hard-codes names, schema_test.go line 50); the manifest lists the fixture with its SHA-256 (`TestFixtureManifest`, page_schema_test.go line 107; `scripts/hygiene` fails an unlisted fixture).

Acceptance:

1. `grep -c 'readings.reading.r1.json=ReadingRecord' contracts/generate.sh` prints 1 and `grep -c "type ReadingRecord = gen.ReadingRecord" internal/contracts/api.go` prints 1; `grep -c "type ReadingRecord struct" internal/contracts/gen/records.go` prints 1.
2. `TestDatasetFixtures`, `TestFixtureManifest`, `TestReadingRejects` and `TestReadingRecordAlias` exist (`grep -c "func TestReadingRejects\|func TestReadingRecordAlias" internal/contracts/schema_test.go internal/contracts/api_test.go` is at least 2); `TestReadingRejects` fails a record with `value` "1e3", state `done` or `reason_code` 9.
3. `git diff $(git merge-base HEAD origin/main) -- contracts/proto contracts/datasets/obs.series_point.r1.json` is empty.
4. `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the nine paths of the Files line.

Checked by CI: `sh contracts/generate.sh && git diff --exit-code internal/contracts/gen docs/contracts/datasets` leaves no change; `go test ./internal/contracts -race -count=1`; `go run ./scripts/hygiene` prints `hygiene: ok`; the `test` check reads the pull request body, which starts `Contracts: readings.reading r1 (new dataset; additive)` from the moment the pull request is opened.

### store-reading-interface (interface)

- Module: store. Size: S. Depends on: reading-dataset.
- Order: after 1 (reading-dataset), which declares `contracts.ReadingRecord`. Interface change: `internal/store/api.go`.
- Design: docs/design/compute-figures.md#Interface
- Files: internal/store/api.go, internal/store/api_test.go, docs/contracts/store.md

- Goal: `ReadingWriter`, `ReadingReader`, `ReadingStore`, `OpenReadingFunc` and `DSNEnvReadings` as in the Interface section, with the doc comments of the neighbours; `docs/contracts/store.md` states the table key, the winner rule of `rd.reading_r1`, the `reading_unit` enum and the grants (Decision 1); `api_test.go` tests the contract from both sides with a fake store.

Acceptance:

1. `grep -c "ReadingWriter\|ReadingReader\|ReadingStore\|OpenReadingFunc\|DSNEnvReadings" internal/store/api.go` is at least 5; `grep -c "OpenReadingFunc func(ctx context.Context, dsn string) (ReadingStore, error)" internal/store/api.go` prints 1.
2. `go list -deps ./internal/store | grep -c "pgx"` is the number on main; `grep -c "contracts.ReadingRecord" internal/store/api.go` is at least 2.
3. `grep -c "numeric(38,12)\|rd.reading_r1\|per_calendar_day" docs/contracts/store.md` is at least 3.
4. `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the three paths of the Files line.

Checked by CI: `go test ./internal/store -race -count=1`.

### store-reading-tables

- Module: store, plus repo (judged: `internal/store/setup` roles and the regenerated `.go-arch-lint.yml`). Size: L. Depends on: store-reading-interface, sched-store-ops (scheduler design). Second-opinion case for the window: it creates a role and its grants, which changes who may act with what permission.
- Order: after 2 (store-reading-interface) and after sched-store-ops, so that 0012 is on main; owns `migrations/0013_readings.sql`.
- Brief: docs/design/compute-figures.md#Second opinion (the new role `ot_w_readings` and its grants).
- Second opinion: required; the advisor answers before the merge and the owner merges the pull request (the pull request waits for the owner's word).
- Advisor: pending (filled by the dispatcher)
- Design: docs/design/compute-figures.md#Decisions
- Files: migrations/0013_readings.sql, internal/store/reading/store.go, internal/store/reading/writer.go, internal/store/reading/reader.go, internal/store/reading/reading_test.go, internal/store/reading/pg_test.go, internal/store/setup/setup.go, internal/store/setup/setup_test.go, .go-arch-lint.yml

- Goal: schema `readings` with `reading` and the children `reading_input`, `reading_input_key`, `reading_parameter` (Decision 1 departures); enum `reading_unit`; view `rd.reading_r1`; role `ot_w_readings` in `setup.go` (env line `OT_PG_DSN_READINGS`) with INSERT and SELECT on the four tables and SELECT on the view, `SELECT` on `rd.reading_r1` to `ot_ro_pages`, nothing to `ot_runner`; `reading.Open` (`store.OpenReadingFunc`) returns a `ReadingStore` whose `Close` closes the pool; `writer.go` inserts a record with its children in one transaction, `ON CONFLICT DO NOTHING`; `reader.go` returns the newest rev per day in a range. `pg_test.go` carries the build tag `pg`.

Acceptance:

1. `TestMigration0013Shape` and `TestRoleListHasReadings` pass without a database. `TestMigration0013Shape` reads the file by a path found from `go.mod` and checks the tables, the key `(name, version, subject_kind, subject, day, rev)`, `numeric(38,12)`, the enum values, the grants, and that no `UPDATE`, `DELETE` or `TRUNCATE` is granted; `TestRoleListHasReadings` checks `setup.go` (role, env line).
2. The numbering is checked as a set, not by a tail: `ls migrations | grep -c "^0013_readings.sql$"` prints 1 and `ls migrations | sed 's/_.*//' | sort | awk 'NR>1 && $1 != p+1 {bad=1} {p=$1+0} END {exit bad}'` exits 0 (unique, gapless); `grep -c "derived.figure_value\|double precision" migrations/0013_readings.sql` prints 0.
3. `TestInsertSQLIdempotent`, `TestLatestPicksHighestRev` and `TestCloseClosesPool` pass against a recording fake of the pgx interface; `var _ store.OpenReadingFunc = Open` compiles.
4. `grep -c 'internal/store/reading' .go-arch-lint.yml` prints 1.
5. `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the nine paths of the Files line.

Checked by CI: `go test ./internal/store/reading ./internal/store/setup -race -count=1`; `go run ./scripts/arch && git diff --exit-code arch.toml .go-arch-lint.yml` leaves no change; `go vet -tags pg ./internal/store/reading` (compiles the tagged test only), the pg run of `pg_test.go` (insert, rev 2 wins, a missing row yields, the grants), the migration run.

### docs-compute-rules (interface)

- Module: docs. Size: M. Depends on: `compute-interface` (compute-core.md). The advisor agreed to `market.sp500_vs_200d` v2 (Decision 6); the issue text carries its `agreed_on` and `agreed_in`, and those of the eight new entries.
- Order: after the core interface (compute-core.md, compute-interface); 5 (compute-rules) follows it and edits `register.toml` too. Interface change: the register text of nine entries that `compute-rules` must match (an interface issue of this cut: it lands before the issue that uses it).
- Design: docs/design/compute-figures.md#Decisions
- Files: docs/engineering.md, docs/GLOSSARY.md, docs/conventions/register.toml

- Goal: Decision 11 for `engineering.md` lines 7 to 11; GLOSSARY entries for `market.vix_term_ratio` and the four 3-year percentiles (each cites the `N225_IV` rule of GLOSSARY.md line 87 and names `Below`), and one line that `ivp_v1` (GLOSSARY.md line 78) is the rule of `market.ivp_own`; nine new `[[figure]]` tables (the eight ids of Decision 5 and `market.sp500_vs_200d` version 2), each with the 15 keys and `source_hash = "pending-recompute"`; v1 of `market.sp500_vs_200d` is untouched.

Acceptance:

1. `grep -c '^id = "market\.\(iv30_own\|ivp_own\|ivr_own\|vix_term_ratio\|vix_pctile_3y\|vxn_pctile_3y\|rvx_pctile_3y\|sp500_pctile_3y\)"' docs/conventions/register.toml` prints 8; `grep -c '^id = "market.sp500_vs_200d"' docs/conventions/register.toml` prints 2; `grep -c '^\[\[figure\]\]' docs/conventions/register.toml` is 53.
2. A script-free key check, `awk '/^\[\[figure\]\]/{n++} /^(id|version|title_zh|pages|formula|inputs|period_date|currency|excludes|gap_text|implemented_by|source_hash|agreed_on|agreed_in|history) = /{k++} END{print k/n}' docs/conventions/register.toml`, prints 15 (all entries keep all keys); `grep -c 'rounded half-even to 6 places' docs/conventions/register.toml` is at least 9; `grep -c 'hv30_own' docs/conventions/register.toml` is the number on main.
3. `grep -c "QuantLib\|bump-and-reprice" docs/engineering.md` prints 0; `git diff $(git merge-base HEAD origin/main) -- docs/engineering.md | grep '^@@'` shows only hunks inside lines 7 to 11.
4. `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the three paths of the Files line.

Checked by CI: `go run ./scripts/hygiene` prints `hygiene: ok`.

### compute-rules

- Module: compute, plus docs (judged two-module entry: the register is under docs). Size: M. Depends on: docs-compute-rules, compute-own-glue, compute-figures-series, compute-figures-iv (compute-core.md): the hashes are of final bodies (REV1:3), so no later issue edits a hashed function.
- Order: after 3, 4 (store-reading-tables, docs-compute-rules: 4 adds the nine tables and both edit `register.toml`; 3 shares `.go-arch-lint.yml`) and after compute-own-glue, compute-figures-series and compute-figures-iv of compute-core.md (`own` must exist for the golden); fills the real hashes.
- Design: docs/design/compute-figures.md#Decisions
- Files: internal/compute/rules/rules.go, internal/compute/rules/rules.json, internal/compute/rules/rules_test.go, internal/compute/own/stored_text_test.go, internal/compute/own/testdata/stored_text.json, docs/conventions/register.toml, .go-arch-lint.yml

- Goal: `rules.New` (`compute.NewRulesFunc`): the table of Decisions 5 and 6, embedded from `rules.json` (id, version, convention, scale, `compare`, the numbers, `max_age_days` of Decision 7, `hash`); `Rule(id)` gives the highest version of the id. `rules_test.go` recomputes each hash by Decision 12 with `go/parser`, `go/printer`, `crypto/sha256` and compares with `rules.json` and `register.toml` (go-toml; the file found by walking up to `go.mod`); the nine entries get their real `source_hash`. The stored-text golden is not in `internal/compute/rules` and not in `figures`. The stored text is made by `Units.RoundHalfEven` (Decision 2), which is `own.NewUnits`; `figures` needs `Stats` and `Rolling` from sibling packages and may import only `contracts` and the root `compute` (modules.md derivation), so a golden there would run on fakes and not check the real text. The golden is `internal/compute/own/stored_text_test.go` with `testdata/stored_text.json` (one case per id: a literal input and the expected stored text, REV1:C1), in package `own`, which may import `contracts` and `compute` only: the test applies the real `own.NewUnits` rounding at the scale of each id, read from `internal/compute/rules/rules.json` as data (path found by walking up to `go.mod`, built with `filepath.Join`), so it imports no sibling package and needs no arch exception; what `figures` hands to `Units` is tested in the core's `figures` tests; `TestRulesMatchRegister` and `TestHashesMatchSource` keep that file the single source.

Acceptance:

1. These tests pass: `TestHashesMatchSource` (a body edit in a `t.TempDir()` copy changes the hash; a whitespace-only edit does not; a change of a listed helper does), `TestRulesMatchRegister` (same ids, versions, unit, scale, `compare`; the `currency` text says the rule's unit word and half-even at the rule's scale), `TestCompareFixedPerID` (`ivp` is `AtOrBelow`, each of the four percentiles is `Below`), `TestNoPendingForGoIds` (the test matches entries by (id, version), as landed: each (id, version) of `rules.json` has a 64-hex `source_hash` and every other entry still has `pending-recompute`, so `market.sp500_vs_200d` v1, Python, stays pending while v2 has its hash), `TestRegisterEntriesComplete` (every `[[figure]]` has the 15 keys; (id, version) is unique), `TestStoredTextGolden` (nine cases; one is the binary-exact tie 0.0078125 at scale 6; it lives in `internal/compute/own/stored_text_test.go`).
2. `git diff $(git merge-base HEAD origin/main) -- docs/conventions/register.toml | grep -c '^[-+]version'` prints 0; the diff touches only `source_hash` lines.
3. `grep -c "compute.NewRulesFunc" internal/compute/rules/rules_test.go` is at least 1; `go list -deps ./internal/compute/rules | grep -c "gonum\|go-talib"` prints 0; `go list -deps ./internal/compute/rules | grep -c "internal/compute/figures"` prints 0; `go list -deps -test ./internal/compute/own | grep -c "internal/compute/rules"` prints 0 (the golden imports no rules package; it reads `rules.json` as data).
4. `grep -c 'internal/compute/rules' .go-arch-lint.yml` prints 1; `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the seven paths of the Files line.

Checked by CI: `go test ./internal/compute/rules ./internal/compute/own -race -count=1`; `go run ./scripts/arch && git diff --exit-code arch.toml .go-arch-lint.yml` leaves no change.

### jobs-compute-api (interface)

- Module: jobs. Size: S. Depends on: store-reading-interface, compute-interface (compute-core.md: `NewComputeFiguresFunc` names `compute.Figures`, `Rules`, `Units` and `SeriesOps` of `internal/compute/api.go`), sched-runner-interface, sched-jobs-schedule (scheduler design: both edit `internal/jobs/api.go` and `jobs.md`, and `sched-jobs-schedule` asserts `api.go` unchanged in its own pull request).
- Order: after 2 (store-reading-interface) and after compute-interface, sched-runner-interface and sched-jobs-schedule. Interface change: `internal/jobs/api.go`, `docs/contracts/jobs.md`.
- Design: docs/design/compute-figures.md#Interface
- Files: internal/jobs/api.go, internal/jobs/api_test.go, docs/contracts/jobs.md

- Goal: `ComputeStores` and `NewComputeFiguresFunc` of the Interface section and the document of the job: id, keys, the rules of Decisions 9 and 10, `Postcondition`, the retry classes. `api_test.go` tests the contract from both sides with a fake job.

Acceptance:

1. `grep -c "ComputeStores\|NewComputeFiguresFunc" internal/jobs/api.go` is at least 2; `grep -c "time.Sleep" internal/jobs/api_test.go` prints 0.
2. `go list -deps ./internal/broker/... | grep -c "internal/jobs"` prints 0; `go list -deps ./internal/jobs | grep -c "internal/runner"` prints 0.
3. `grep -c "compute_figures" docs/contracts/jobs.md` is at least 2.
4. `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the three paths of the Files line.

Checked by CI: `go test ./internal/jobs -race -count=1`.

### compute-figures-job

- Module: jobs, plus repo (judged: the regenerated `.go-arch-lint.yml`). Size: L. Depends on: store-reading-tables, compute-rules, docs-iv30-rate-age, jobs-compute-api, sched-jobs-schedule (it adds the `[schedule]` table and the plain-value methods to its own manifest). The Iv30 input mapping (Decision 10) is part of this issue, not a sibling issue: `Run` calls `ivInput` and `subjects`, so one pull request carries the mapping and the job, and the job is tested against fakes of the store interfaces.
- Order: after 3, 5, 6, 9 (store-reading-tables, compute-rules, jobs-compute-api, docs-iv30-rate-age), and after sched-jobs-schedule; 3 and 5 edit `.go-arch-lint.yml` too.
- Design: docs/design/compute-figures.md#Decisions
- Files: internal/jobs/computefigures/ivinput.go, internal/jobs/computefigures/ivinput_test.go, internal/jobs/computefigures/subjects.go, internal/jobs/computefigures/subjects_test.go, internal/jobs/computefigures/job.go, internal/jobs/computefigures/steps.go, internal/jobs/computefigures/schedule.go, internal/jobs/computefigures/job.toml, internal/jobs/computefigures/job_test.go, .go-arch-lint.yml, docs/contracts/jobs.md

- Goal: the mapping of Decision 10 as functions of the package `computefigures` over the store interfaces (`ivinput.go`: `ivInput(ctx, stores, underlyingID, day) ([]compute.ExpiryQuotes, compute.Point, error)`; `subjects.go`: `subjects(ctx, stores, day) ([]subject, error)`), and the job of Decisions 3 to 9 (`computefigures.New`, `jobs.NewComputeFiguresFunc`): `Plan` lists one key per (rule, subject); `Run` runs the steps in order, builds each reading (value text, `rows_sha256`, rule hash, convention, parameters) and writes through `ReadingStore` with the skip-or-next-rev rule. The inputs of a series figure are read from `asOf` minus `Years*366+7` days (when `Years > 0`) or `2*max(Window, N)+14` days. A figure the core marks missing is stored `missing_input` with its reason, never skipped. `schedule.go` gives `Schedule()`, `RunBound()`, `RestartSensitive()` and `Postcondition` as `sched-runner-interface` declares them (read from `internal/runner/api.go` on main when cut); the job imports no runner type and the test holds a local interface with the same method set. Each file stays under 600 lines. `docs/contracts/jobs.md` (module jobs, so no second module) gets the age rule in its Iv30 input rules: a `DGS3MO` point more than 4 calendar days before the as-of day gives a missing `stale_input`; one of an earlier day within 4 days is used with `stale_rate` true.

Acceptance:

1. These tests of the mapping pass with fakes: `TestIvInputLatestReceivedWins`, `TestIvInputOnlyStatusOk`, `TestIvInputMidRules` (a zero bid, a crossed quote: that side absent), `TestIvInputNoIvIsZero`, `TestIvInputTermsChunkOf500` (501 ids make two calls), `TestIvInputAMSettlementUsesOpen`, `TestIvInputNonSessionExpiryDropped`, `TestIvInputMinutesFloored`, `TestIvInputRateIsNewestNotAfterDay`, `TestIvInputStaleRateRules` (a `DGS3MO` point 5 calendar days before the as-of day gives a missing IV30 with `stale_input`, `reason_code` 3; a point 1 to 4 days old gives an `ok` IV30 with `stale_rate` true, and 4 days old is still ok), `TestSubjectsNamedBySymbol` (an id without a name is `underlying:<id>`).
2. These tests of the job pass with fakes of the stores, `compute.Figures` and a fake clock: `TestPlanKeys` (sorted, one per rule and subject), `TestRunStoresOkAndMissing`, `TestRerunSameInputsWritesNothing`, `TestChangedInputWritesNextRev`, `TestIvpReadsStoredIv30` (a missing IV30 reading gives a missing IVP, reason `no_input`), `TestFirstRunOnEmptyStores` (every planned key stored `missing_input` with `short_history` or `no_input`, none skipped, REV1:13), `TestFredLagStaysOk` (the newest FRED point 5 days old is `ok` with `data_day` set; 9 days old is stored `missing_input` with `reason_code` 3, `stale_input`), `TestPostconditionRowPerKey`, `TestScheduleValues` (`session_close`, offset 20m, calendar `XNYS`, read from `job.toml`), `TestManifestHasNoNumbers` (outside `[schedule]`, `job.toml` holds ids and series names only).
3. `TestReadingCarriesBothHashes` (`code_version` equals the rule's `Hash`, `rows_sha256` is 64 hex) and `TestValueIsRoundedText` (0.0078125 at scale 6 gives "0.007812", a half-even tie that half up would round the other way) pass.
4. `grep -c "time.Now\|time.Sleep\|os.Getenv" internal/jobs/computefigures/ivinput.go internal/jobs/computefigures/subjects.go` prints 0 for each file; `grep -c "time.Now\|time.Sleep" internal/jobs/computefigures/*.go` prints 0; each Go file is under 600 lines.
5. `go list -deps ./internal/jobs/computefigures | grep -c "internal/store/\(series\|snapshot\|reading\|instrument\)"` prints 0 (the stores arrive through `ComputeStores`).
6. `grep -c 'internal/jobs/computefigures' .go-arch-lint.yml` prints 1; `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the eleven paths of the Files line. `grep -c "stale_input" docs/contracts/jobs.md` is at least 1 and `grep -c "4 calendar days" docs/contracts/jobs.md` is at least 1.

Checked by CI: `go test ./internal/jobs/computefigures -race -count=1`; `go run ./scripts/arch && git diff --exit-code arch.toml .go-arch-lint.yml` leaves no change.

### compute-jobs-wire

- Module: ot. Size: M. Depends on: compute-figures-job, sched-cmd-run (scheduler design: both edit `wire.go` and `runonce.go`), and the whole core of `compute-core.md` (`compute-interface` to `compute-figures-iv`, `compute-own-glue`, `compute-gonum-stats`), whose constructors it calls. Second-opinion case for the window: `run-once` now needs the readings role's DSN.
- Order: after 7 (compute-figures-job) and after sched-cmd-run; last of this file, and cut only when the core and every issue before it are on main (Modules).
- Design: docs/design/compute-figures.md#Interface
- Brief: docs/design/compute-figures.md#Second opinion (one brief, shared with `calendar-jobs-wire`).
- Second opinion: required; the advisor answers before the merge and the owner merges the pull request (the pull request waits for the owner's word).
- Advisor: pending (filled by the dispatcher)
- Files: cmd/ot/wire.go, cmd/ot/wire_test.go, cmd/ot/runonce.go, cmd/ot/runonce_test.go, cmd/ot/runonce_option_test.go, cmd/ot/backfill_test.go, docs/contracts/ot.md

- Goal: `deps` gains `openReading` (`store.OpenReadingFunc`, `reading.Open`), `newComputeFigures` (`jobs.NewComputeFiguresFunc`, `computefigures.New`) and the four values `NewComputeFiguresFunc` takes: `compute.Figures` (`figures.New(stats, rolling, ops, rules)`), `compute.Rules` (`rules.New`), `compute.Units` (`own.NewUnits`) and `compute.SeriesOps` (`own.NewSeriesOps`), with `gonumstats.New` for `Stats` and `own.NewRolling(stats)` for `Rolling` (the `ot` list of modules.md names all these packages; `talibind` is not wired); `run-once` and `backfill` wire four jobs; the stores of `ComputeStores` are opened with the lines `OT_PG_DSN_PAGES` (catalog, eod, terms, names, series: as `site.go line 80-103` does), `OT_PG_DSN_READINGS` (readings) and the runner store already open (session days); both new lines are required in `.env` (`ot setup pg`, the owner's, creates the readings line after migration 0013); `docs/contracts/ot.md` says four jobs. The tests that build the `.env` and the fake `deps` of `run-once` and `backfill` change with the two required lines and the fourth job: `runonce_test.go` (`TestRunOnceMissingKey`, `TestRunOnceClosesStores`, ...), `runonce_option_test.go` (`TestRunOnceWiresOptionJobs`, ...) and `backfill_test.go` (`TestBackfillCallsRunner`, ...) are in the Files line; the shared fake `deps` and `.env` helper stay where they are. `backfill.go` is not edited: it builds its jobs through `wire.go`; if it must change, the pull request says so and leaves it.

Acceptance:

1. `TestRunOnceWiresFourJobs`, `TestRunOnceNeedsReadingsDSN` (the error names the line, never its value) and `TestDemoProfileOpensNoStore` pass; the existing `TestRunOnceWiresOptionJobs`, `TestRunOnceMissingKey` and `TestBackfillCallsRunner` still exist (`grep -c "func TestRunOnceWiresOptionJobs\|func TestRunOnceMissingKey\|func TestBackfillCallsRunner" cmd/ot/runonce_option_test.go cmd/ot/runonce_test.go cmd/ot/backfill_test.go` is at least 3).
2. `grep -c "four jobs" docs/contracts/ot.md` is at least 1 and `grep -c "three jobs" docs/contracts/ot.md` prints 0.
3. `go list -deps ./cmd/ot | grep -c "internal/compute/talibind"` prints 0 (go-talib is not wired).
4. `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the seven paths of the Files line.

Checked by CI: `go test ./cmd/ot -race -count=1` (the whole package, including the changed tests of `run-once` and `backfill`).

### docs-iv30-rate-age

- Module: docs. Size: S. Depends on: docs-compute-rules, compute-rules (the entry and its hash are on main).
- Order: after 4, 5 (docs-compute-rules, compute-rules; both edit `register.toml` too), before 7 (compute-figures-job). Not an interface change: the entry stays version 1, the sentence is corrected to the landed `rules.json` (`MaxAgeDays` 4).
- Design: docs/design/compute-figures.md#Decisions
- Files: docs/conventions/register.toml

- Goal: in the entry `market.iv30_own`, `inputs` says "the newest point no older than 4 calendar days" in place of 8, and `gap_text` gains the case "A rate older than 4 calendar days: missing, stale_input; a rate of 1 to 4 days old is used with stale_rate." One line is added to its `history`. `version`, `source_hash` and `implemented_by` do not change (the hash covers function sources, not text).

Acceptance:

1. `grep -c "no older than 8 calendar days" docs/conventions/register.toml` prints the number on main minus 1 (the entry of `market.iv30_own` no longer has it); `awk '/^id = "market.iv30_own"/{p=1} p&&/no older than 4 calendar days/{print "ok"; exit}' docs/conventions/register.toml` prints ok.
2. The `gap_text` of `market.iv30_own` names `stale_input` and `stale_rate` (`awk '/^id = "market.iv30_own"/{p=1} p&&/^gap_text/{g=1} g&&/stale_rate/{print "ok"; exit}' docs/conventions/register.toml` prints ok).
3. `git diff $(git merge-base HEAD origin/main) -- docs/conventions/register.toml | grep -c '^[-+]\(version\|source_hash\)'` prints 0; `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the one path of the Files line.

## Left out

- Deferred: `reading_row` (drill-down), the `alias` foreign key, `ops.job_run`, figures of equity names (no closes dataset), days that are not session days (scheduler stage 2), Beta, `sp500_ma200` as a stored level (no v3 unit), the owner's real slot time, a `ReadingReader` for the pages.
- Deferred, not produced by any of the four designs (REV1:14): RV20 and IV-RV20 of N225, the N225_IV 3-year percentile, breadth, core returns, diagnosis ratios and ranks; `market.hv30_own` (no producer).
- Rejected: a `derived.figure_value` float64 table (a parallel float store, ADV:49); three jobs (IVP and IVR read the stored IV30 of the same run, and the scheduler of stage 1 has no ordering between jobs, so one job with ordered steps is the way to run IV30 before IVP); `stale_rate` as a column; the input hash alone; a Python figure service (a second runtime for one arithmetic that the Go-only runtime rule keeps in `internal/compute`).

## Second opinion

Two issues are second-opinion cases and carry `Brief:`, `Second opinion:` and `Advisor:` lines; their pull requests wait for the owner's merge.

- `store-reading-tables`: a new role `ot_w_readings` with INSERT and SELECT on the four `readings` tables, SELECT on `rd.reading_r1` to `ot_ro_pages`, and nothing to `ot_runner`. It is the first role of a computed store; the advisor checks the grants against Decision 1 and `setup.go`.
- `compute-jobs-wire` and `calendar-jobs-wire` (`compute-calendar.md`) share ONE brief: "the scheduler process holds the runner, readings and loader DSNs". After both land, `ot scheduler`, `ot run-once` and `ot backfill` hold the DSNs of `ot_runner`, `ot_w_readings` and `ot_loader`, so who may act with which permission changes. No narrower role exists to use instead: migration 0005 already makes `ot_loader` a role of `master.session_day` only, and `ot_w_readings` is the narrow role of the readings tables. The scheduler design (rev 8, Decision table line 91) names the departure from one writer role: `ot_runner` for the ledger and the runs; a job may open its own store with a role named in its design.
- Where the lines are listed: `OT_PG_DSN_READINGS` by `compute-jobs-wire` (`docs/contracts/ot.md`); `OT_PG_DSN_LOADER` by `calendar-jobs-wire` (`docs/contracts/ot.md`) and `calendar-deploy-note` (`docs/deploy/scheduler.md`); `sched-deploy-runbook` lists neither (its `env` has placeholders only).
- The reading `cutoff` text `session_close` (Decision 1) was chosen by the owner on 2026-11-08 ("a"); it is not part of this brief.

## Adversarial

Read: the compute design adversarial review, rev 1 of the four files (kept by the dispatcher, not committed; REV1). Older answers (ADV, rev 2 of the first draft: migration numbers, day semantics, `stale_rate`, one job, interface-first, MANIFEST, offline checks) stand as in rev 1 of this file. Adopted means the text above changed.

- REV1:3 (`compute-rules` hashes stubs of the IV functions): adopted. It depends on `compute-figures-iv`; no later issue edits a hashed function.
- REV1:4 (`ot_runner` cannot write `master.session_day`): concerns the calendar design (`compute-calendar.md`); here only the readings role is new, with its grants and DSN named, and marked a second-opinion case.
- REV1:5 (sp500_vs_200d is not a refactor): adopted. Decision 6: version 2, a new entry; "Advisor: agrees ... no version bump" is removed; that agreement was given on the wrong premise; the advisor has since agreed to v2 (2026-11-08).
- REV1:6 (register keys, `rounding`, the test that does not exist): adopted. Decision 5 uses the 15 keys of main; rounding is the sentence in `currency`; the checks are `awk`/`grep` over the file and `TestRegisterEntriesComplete`; no grep of a `rounding` key and no reference to `tests/conventions` remain; all eight new entries carry `title_zh`, `pages`, `gap_text`, `agreed_on`, `agreed_in`.
- REV1:7 (store API, leaks, jobs interface frozen without the stores): adopted. `ReadingStore` with `Close`, `OpenReadingFunc(ctx, dsn)`, one DSN; `ComputeStores` names all seven stores before the job exists.
- REV1:8 (Iv30 input mapping): adopted, Decision 10 and `ivinput.go` of `compute-figures-job` (a sibling issue `compute-iv-input` was merged into the job issue, because the job calls its unexported functions).
- REV1:9 (conditional Postcondition): adopted. The chain depends on `sched-runner-interface` and `sched-jobs-schedule` by name; no "omitted if" remains.
- REV1:14 (what the pages wait for): adopted as stated scope in Left out; `market.hv30_own` is dropped from the registered ids.
- REV1:15 (arch files, modules.md line collide): not adopted as removable; the order is stated (Modules).
- REV1:F1 (calendar items sit here): adopted. the differ's interface issue (now `clock-calendar-interface`) and the calendar job's constructor type moved to `compute-calendar.md`.
- REV1:F2 (schema v3 not in the repo; winner rule hides a same-day rerun; hash covers one function): adopted in part: store.md carries the text; the winner rule is kept (Decision 4); the hash covers the listed helpers and the numbers (Decision 12).
- REV1:F3 (drop the children tables for a first cut): not adopted: the plan asks for per-reading inputs and parameters (P:140), and the children are three small tables of one migration; the calendar half moved out as proposed.
- REV1:F4 (lock-in: migration shape, `cutoff`, `conv` tag, role): kept and named: Decisions 1, 12; `cutoff` chosen by the owner (session_close, 2026-11-08); the slot awaits the first job issue review. REV1:F5 (the false claim at F:28): see REV1:5. REV1:F6: `compute-rules` order and count (adopted: no count arithmetic, `TestNoPendingForGoIds`); `compute-figures-job` mapping and stores (adopted, see REV1:7, 8); `store-reading-tables` proof (the pg proof stays under Checked by CI; offline checks are the shape and recording-fake tests).
- REV1:R2 (FRED lag against `MaxAgeDays`): adopted, Decision 7 (4 for IV30, 8 for FRED).
- REV1:13 (first run on a fresh database): adopted as `TestFirstRunOnEmptyStores`.
- Cross-file: shared generated files: adopted (`reading-dataset` first; FRED's unit issue after it); wire files: `compute-jobs-wire` after `sched-cmd-run`; `docs/contracts/ot.md` strings "four jobs"/"nine subcommands" assume the order of the four designs after the scheduler; "Advisor: agrees" and "advisor change" lines as attestations: removed here and in the calendar design.
- REV1 cross-file, goal check (own glue behind an interface: the rule-hash recipe, `figures.New`'s `Rolling` over `Stats`): `Rolling` see `compute-core.md` Adversarial (already behind its interface, injected into `figures.New`). The recipe is not behind a new interface: no library does it, it makes no figure, and the hash reaches the code only as data, `Rule.Hash` through `Rules`; `rules_test.go` holds the recipe as the check of Decision 12. Not adopted; the owner may ask for one at the design review.
- A failed IV30 step blocks the rest? No (Decision 9): a step that fails does not stop the others. The steps run in order IV30, market figures, then IVP and IVR; a failed IV30 leaves its keys missing or absent and IVP and IVR of those names are `missing_input` with `no_input`, while the market figures run as before, and the postcondition names every key without a row.
- Units (the unit of a stored reading): each id has one word of the `reading_unit` enum, Decision 5, in the register entry, in `rules.json` and in the stored row; `TestRulesMatchRegister` checks all three agree.
