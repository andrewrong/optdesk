# source rank, step 9 of the storage plan: the order of sources in a job.toml and FRED H.10 as the fill source of the USDJPY rate

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-14
Revision: 3

Meaning of "void" (the plan names it only in the cut order, step 9; the repository gives it a meaning and this design keeps it): `status = 'void'` is a stored key whose value must not be read. It is a legal status of `obs.fx_rate`, `obs.price_bar` and `obs.series_point`; the views hide it (`rd.fx_rate_r1` is `WHERE status = 'ok'`, `rd.series_point_r1` is `WHERE status <> 'void'`); an upsert replaces it. No code writes it and this design adds no writer (Left out).

Source: owner 2026-11-12 "fallback, not cascade", 2026-11-12 dev mode ("any job runs when triggered; only a same-job concurrency guard"), 2026-11-13 "overwrite by natural key and keep no old value" and "storage plan v2 OK"; storage plan v2 of the advisor (2026-11-13), P4 (fill automatic, cross-check writes nothing, override refused), P7, cut order step 9, section 6. Precedents copied, not re-argued: `data-repair-fx.md` (landed: `FxRateUpserter`, `status`, the grants), `series-registry.md` (landed: `secondary_source`), `fundamentals-keys.md` (entry shape, migration order).

Goal: the USDJPY rate has a second source. After the IBKR midpoint run, `fx_refresh` asks FRED H.10 (`DEXJPUS`, JPY per USD) for the days of the last `fill_lookback_days` and stores each day it gets and the table has no row for, with source `fred_h10`, whether or not the IBKR step succeeded. It never replaces a stored row. The rank of the sources is the order of the names in `job.toml`. The pages and the figures read `rd.fx_rate_r1` with the same columns; the only visible change is a rate on a day IBKR failed, with 2 decimals.

## Today

All paths at origin/main f7d990f5.

- `internal/jobs/fxrefresh/run.go:34-` the manifest struct (`version`, `id`, `duration_days`, `schedule`); `New` takes the broker, the history client, `bar.FxRateWriter`, `bar.FxRateReader` (kept for the signature only) and the clock; `Run` asserts `bar.FxRateUpserter`, asks MIDPOINT bars ending on the day and upserts them with `writtenBy = "fx_refresh:" + day`. `job.toml` has no source list. `types_test.go` pins `NewFxRefreshFunc`, so the constructor is not changed.
- `bar.FxRateWriter.InsertFxRates` (`internal/store/bar/api.go`, `barpg/store.go:132`) inserts rows whose key is missing and leaves an existing key alone (`ON CONFLICT DO NOTHING`); `barpg.Store` implements it. This is the fill; no new store method is needed.
- `migrations/0039_fx_rate_repair.sql`: `obs.fx_rate (pair, rate_day, rate numeric, source, received_at, status ok|no_value|void, updated_at, written_by)`; `fx_rate_source_ck` of `0021_bars.sql:41` still reads `source = 'ibkr_midpoint'`; `ot_w_bar` keeps table INSERT.
- `contracts/datasets/obs.fx_rate.r1.json:16` and `internal/contracts/gen/records.go:1694-1718` (`FxRateSource`, one value, `UnmarshalJSON` refuses others); `docs/contracts/datasets/obs.fx_rate.md:12`. `contracts/datasets/obs.series_point.r1.json:31` the unit enum (`percent`, `ratio`, `ccy_amount`, `count`, `index_points`). Precedent for a unit added with generated output: `8d53b7d8` (fixtures, dataset document, `records.go`, `schema_test.go`).
- `internal/sources/fred/mapping.go:55-` `seriesUnits` lists the series the client accepts; `mapObservations` (`:115`) refuses a series id that is not in it ("no unit configured for series"); each row is checked against recorded metadata in `internal/sources/fred/testdata/series_units.json` by `mapping_test.go:181-294`. `DEXJPUS` is in none of them (`git grep DEXJPUS -- internal cmd migrations contracts` finds nothing), so FRED cannot return it today.
- `fred_series` second source is merged (`60474f7c`, #2272): `fredseries.NewWithSecondary(src, second, w, clk)` and `fillGaps` exist, `New` passes a nil second source, and `cmd/ot` never calls `NewWithSecondary` (`git grep NewWithSecondary -- cmd` finds nothing): the NY Fed fill is not wired. That wiring is not part of this design.
- `cmd/ot/wire.go:535-541` `jobFactory.fxRefresh` calls `f.d.newFxRefresh`; `jobFactory.src` (`cmd/ot/runonce.go:278`) is the FRED source, built only when a FRED key is set (`reasonNoFredKey`). `cmd/ot/wire.go` has 796 lines and is not edited here. `cmd/ot/jobconsole_jobs.go:95` already has `fx_refresh` as an any-day row with Force.
- Migrations on main end at `0044`; `0045` (readings, #2288), `0046` (`fundamentals-keys.md`) and `0047` (`option-captures.md`) belong to open designs. This design's migration is the next free number at the cut, expected `0048`.

## Decisions

1. Rank is the order of names in `job.toml`. `fx_refresh` gains `sources = ["ibkr_midpoint", "fred_h10"]` and `fill_lookback_days = 14`. The first name is the primary: its answer is written with `UpsertFxRates` as today. Every later name is a fill source: its rows are written with `InsertFxRates` only. `New` fails when the list is empty, does not start with `ibkr_midpoint`, or names a source the job cannot serve. For series the rank is the registry's `source` and `secondary_source` (landed); no series code changes.
2. Fill (plan P4, owner "fallback, not cascade"). A fill source writes a key only when the table has none: `InsertFxRates` leaves a stored row, of any source, status or value, untouched. So a second source never overrides. A later IBKR answer replaces a FRED rate through the primary's own upsert (it differs, and IBKR outranks FRED by Decision 1). A stored `void` or `no_value` row is not filled (no writer makes one; Left out).
3. The FRED request. After the primary step, in the same `Run`, whatever the primary's result (an IBKR failure is exactly when the fill is needed), the job calls the attached source directly for series `DEXJPUS` over `[day - fill_lookback_days, day]`; it does not read the stored series, because a fallback that depended on another job's stored output would be a cascade. The run keeps the primary's result: a failed fetch or insert is logged and never changes it. A point is skipped when its value is nil (FRED's "." for a holiday or weekend) or not positive; any other becomes `contracts.FxRate{Pair: USDJPY, Rate: strconv.FormatFloat(v, 'f', -1, 64), Source: fred_h10}` (the point value is a `float64`; FRED gives at most two decimals, so the shortest decimal text is that text) and goes to `InsertFxRates`. FRED publishes H.10 with a lag; a gap is filled by a later run; any run of any day does it. No time-of-day guard.
4. The FRED path (the client refuses unknown series). A new unit `jpy_per_usd` is added to the `obs.series_point` r1 unit enum with its generated output (entry 2); `seriesUnits` gets the row `"DEXJPUS": {unit: jpyPerUsd}`, `testdata/series_units.json` gets the recorded FRED metadata of the series and `mapping_test.go` its row (entry 3). The unit tells readers that the value is a rate, not an amount; nothing stores it as a series point here (the fx job converts the point to an `FxRate`).
5. Override is not offered. The plan allows "a replacement by a lower-ranked source ... by a forced console run that names the source". The console request has no source (`jobrequest.Request` is job, day, force) and `contracts.Job.Run` has no force argument. The refusal holds because the fill source has the insert method only.
6. Contract and database. The `source` enum of `obs.fx_rate` r1 gains `fred_h10`, generated output with it (entry 2). The migration of entry 4 (expected number 0048, the next free one at the cut) replaces `fx_rate_source_ck` with `source IN ('ibkr_midpoint', 'fred_h10')` (a closed list, as the existing pattern; every new source costs a migration, Left out); no grant, view or column changes. The Down part restores the old check and refuses (an exception, nothing changed) when a `fred_h10` row exists. Owner-only DDL, one transaction.
7. Wiring. `fxrefresh.AttachSecondary(j contracts.Job, second sources.Source) bool` attaches the fill source to a job `New` built and reports false for any other job (a test fake); its type is the interface `fxrefresh.AttachSecondaryFunc` (entry 1), so `NewFxRefreshFunc`, its tests and the `cmd/ot` fakes keep working. `cmd/ot/wire_fxsecond.go` holds the package variable `attachFxSecondary fxrefresh.AttachSecondaryFunc = fxrefresh.AttachSecondary` and the function that builds `fx_refresh` with `f.src` attached when it is not nil; without a FRED key there is no fill and `fx_refresh` runs as today. `jobFactory.fxRefresh` (`cmd/ot/wire.go:535`) is the only place `fx_refresh` is built (`preflight.go:112` and `wire.go:586` both call it, not `registerJob`), so `wire.go` changes: the body of `fxRefresh` is replaced in place by a call to the new function. The change rebases on whatever else edits `wire.go` and does not grow it (it stands at 796 lines, 800 is the ceiling).
8. Own code behind an interface. The attach function is our glue behind the `contracts.Job` interface; the FRED client is the existing `internal/sources/fred`; no mainstream library knows our key and statuses.
9. No old value is kept (owner 2026-11-13); this design deletes none.
10. What a filled day is, in plain words. A FRED fill is FRED's H.10 rate, the noon New York buying rate, not a close and not an IBKR midpoint, with 2 decimals. It is written by insert only on `(pair, rate_day)`: it fills a day the table has no row for and never changes a row. It stays until a forced `fx_refresh` of a day inside the five-day window replaces it: that run asks IBKR for the bars ending on the day, and the primary's upsert replaces a `fred_h10` row whatever its value, because the compared columns are rate, source and status and the source differs. A day older than the window keeps FRED's rate. The only other writer of `obs.fx_rate` is that same `fx_refresh` upsert (`git grep -n "UpsertFxRates\|InsertFxRates" -- '*.go'` finds no other non-test caller; `noBarStore` in `cmd/ot/wire.go:499` is a stand-in that refuses).

## Modules

No row of `docs/design/modules.md` changes. Entries touch jobs (`internal/jobs/**`, `docs/contracts/jobs/**`; entries 1 and 5), contracts (`contracts/**`, `internal/contracts/**`, `docs/contracts/datasets/**`; entry 2), sources (`internal/sources/**`; entry 3), store (`migrations/**`, `internal/store/**`, `docs/contracts/store.md`; entry 4) and ot (`cmd/**`; entry 6). New import edges, all inside the rows' depends cells: `internal/jobs/fxrefresh` gains an import of `internal/sources` (it imports none today; `jobs` depends on `sources`), `cmd/ot` already imports `fxrefresh` and `sources`.

## Interface

One interface change, entry 1, in `internal/jobs/fxrefresh/types.go` (additive; `NewFxRefreshFunc` is unchanged):

    // AttachSecondaryFunc attaches a fill source to an fx_refresh job built by New and reports whether j was such a
    // job (false for any other contracts.Job, so a fake is left alone). A job without a source never asks one.
    // The implementation is fxrefresh.AttachSecondary.
    type AttachSecondaryFunc func(j contracts.Job, second sources.Source) bool

The data changes are entry 2 (the `source` enum of `obs.fx_rate` r1 and the `unit` enum of `obs.series_point` r1).

## Issues

Build order: every entry compiles and its tests pass against main. BASE is `git merge-base origin/main HEAD`. Entry 4 holds the auto-deploy (migration); entry 5 merges after the window ran it. The pull request of entry 2 carries the line `Contracts: obs.fx_rate r1->r1, obs.series_point r1->r1; enum gains fred_h10 and jpy_per_usd (additive)`, the form `checks.yml` accepts (`<id> rN->rN+1` entries, comma separated, then `; <note>`) and the form of #411, the pull request that added the `index_points` unit; the pull requests of the other entries carry `Contracts: unchanged`.

### 1 source-rank-api (interface)

Module: jobs. Size: S.
Order: first.
Files: `internal/jobs/fxrefresh/types.go`, `internal/jobs/fxrefresh/types_test.go`
Design: docs/design/source-rank.md#Interface
Acceptance:
1. `types.go` declares `AttachSecondaryFunc` as in the Interface section, with its doc, and `NewFxRefreshFunc` is unchanged.
2. `types_test.go` has the new contract test `TestAttachSecondaryContract` (a fake function returns true for a fake job that records the source and false for a job it does not know; the caller reads both answers).

### 2 source-rank-contract

Module: contracts. Size: S.
Order: first.
Files: `contracts/datasets/obs.fx_rate.r1.json`, `contracts/datasets/obs.series_point.r1.json`, `contracts/fixtures/obs.fx_rate.r1.json`, `contracts/fixtures/obs.series_point.r1.json`, `contracts/fixtures/MANIFEST.synthetic-v1.json`, `internal/contracts/gen/records.go`, `docs/contracts/datasets/obs.fx_rate.md`, `docs/contracts/datasets/obs.series_point.md`, `internal/contracts/schema_test.go`
Design: docs/design/source-rank.md#Decisions (4, 6)
`contracts/generate.sh` writes the generated file. The `description` of `obs.fx_rate` r1 no longer says "insert only" (the table is upserted and holds a second source's rate). The two fixture files each gain one line that uses the new value, and the manifest hashes of the two files follow, as in `8d53b7d8`. `internal/contracts/schema_test.go` reads the fixtures (`recs[0]` of the fx fixture at `:493`, the series fixture) and is edited for them; `internal/contracts/api_test.go` is not edited.
Acceptance:
1. The `source` enum of `obs.fx_rate` r1 holds `ibkr_midpoint` and `fred_h10`, and the `unit` enum of `obs.series_point` r1 holds `jpy_per_usd` after the existing values; the generated records have `FxRateSourceFredH10` and `SeriesUnitJpyPerUsd`, accept the new values and still refuse any other; both dataset documents list the values. The generated output is what `contracts/generate.sh` writes from the source files.
2. Both fixtures validate against their schemas with the new line, and the manifest names their new hashes.
3. `schema_test.go` has the new tests `TestFxRateSourceFredH10Accepted` (a record with source `fred_h10` validates, source `stooq` is refused) and `TestSeriesUnitJpyPerUsdAccepted`.

### 3 source-rank-fred

Module: sources. Size: S.
Order: after 2 (it uses the generated unit constant).
Files: `internal/sources/fred/mapping.go`, `internal/sources/fred/testdata/series_units.json`, `internal/sources/fred/mapping_test.go`
Design: docs/design/source-rank.md#Decisions (4)
Acceptance:
1. The client maps a `DEXJPUS` observation: a value becomes a point with unit `jpy_per_usd`, a "." observation becomes a point without a value, and the series is no longer refused with "no unit configured".
2. `testdata/series_units.json` holds the recorded FRED metadata of `DEXJPUS` and the metadata comparison in `mapping_test.go` passes with the new `seriesUnits` row; `mapping_test.go` has the new test `TestMapDEXJPUSValueAndMissing` (inline observations in FRED's JSON form).

### 4 source-rank-store

Module: store. Size: S.
Order: after 2; after the store entries that carry the migrations of the readings design (`readings.md`, #2288), of `fundamentals-keys.md` and of `option-captures.md` (named by design entry; this entry takes the next free migration number at the cut, expected 0048, and rebases on them: all edit `docs/contracts/store.md`). Holds the auto-deploy: it carries a migration.
Files: `migrations/<next free number>_fx_rate_source.sql`, `internal/store/barpg/fill_pg_test.go`, `internal/store/barpg/store_pg_test.go`, `internal/store/barpg/upsert_pg_test.go`, `docs/contracts/store.md`
Design: docs/design/source-rank.md#Decisions (2, 6, 10)
`store_pg_test.go` and `upsert_pg_test.go` list the migrations their databases apply and gain the new file; they are edited for that list only.
Acceptance:
1. The migration has an Up and a Down part, is owned by `ot_owner`, and changes only `fx_rate_source_ck`: it admits `fred_h10` beside `ibkr_midpoint`; the Down part restores the old check and refuses, changing nothing, when a `fred_h10` row exists. No `GRANT` or `REVOKE` line is written.
2. As `ot_w_bar`, `InsertFxRates` stores a `fred_h10` row for a missing key and leaves a stored `ibkr_midpoint` row of the same key unchanged; a source name outside the list is refused; `UpsertFxRates` of an `ibkr_midpoint` row replaces a stored `fred_h10` row of the same day, even when the rates are numerically equal, because the source differs (the forced run of Decision 10), and leaves a stored `ibkr_midpoint` row with an equal rate unchanged.
3. `fill_pg_test.go` (tag `pg`) has the new tests `TestInsertFredRateForMissingKey`, `TestInsertNeverReplacesIbkrRate`, `TestSourceOutsideListRefused`, `TestUpsertReplacesFredRate`, `TestMigrationDownRefusesFredRows`.
4. `docs/contracts/store.md` states that `fred_h10` is a fill source stored by insert only and replaced by the primary's upsert, and names the migration file (`git grep -n "fx_rate_source" docs/contracts/store.md` finds it).

### 5 source-rank-job

Module: jobs. Size: M.
Order: after 1, 3 and 4; merges after the window ran the migration of entry 4 (before it, a `fred_h10` insert is refused by the old check and logged; nothing else changes).
Files: `internal/jobs/fxrefresh/run.go`, `internal/jobs/fxrefresh/secondary.go`, `internal/jobs/fxrefresh/secondary_test.go`, `internal/jobs/fxrefresh/job.toml`, `internal/jobs/fxrefresh/run_test.go`, `docs/contracts/jobs/fx_refresh.md`
Design: docs/design/source-rank.md#Decisions (1 to 3, 7, 10)
`run_test.go` builds the job through `New` with a fake writer and is edited for the new manifest keys and for the second source of its fixtures; `types_test.go` is not edited here (entry 1).
Acceptance:
1. `job.toml` carries `sources = ["ibkr_midpoint", "fred_h10"]` and `fill_lookback_days = 14`; `New` returns an error for a manifest whose list is empty, does not start with `ibkr_midpoint` or names an unknown source; `NewFxRefreshFunc` and `New` keep their signatures; `secondary.go` declares `AttachSecondary` as an `AttachSecondaryFunc` (compile-time assertion).
2. `AttachSecondary` attaches the source to a job `New` built and returns false for any other `contracts.Job`.
3. With a source attached, a run asks it for `DEXJPUS` over the lookback range and stores each point whose value is present and positive as source `fred_h10`, rate text the shortest decimal of the value, through `InsertFxRates`; a nil or non-positive point is not written; a stored rate is never replaced by a FRED value.
4. The fill runs after the primary step whatever its result: when IBKR fails or answers nothing the FRED days are still stored, and the `Run` result is the primary's in every case. A failed FRED fetch, a failed insert and a job without a source leave the `Run` result unchanged and are logged; no source attached means no FRED call. The FRED call does not depend on any stored series.
5. `secondary_test.go` has the new tests `TestFillInsertsFredPointsWithValue`, `TestFillNeverReplacesStoredRate`, `TestNoValuePointSkipped`, `TestNonPositiveFredValueSkipped` (nil, zero and negative points; a value such as 153.42 is stored as "153.42"), `TestFillRunsWhenPrimaryFails`, `TestSecondaryFailureKeepsResult`, `TestNoSourceNoCall`, `TestAttachSecondaryForeignJob`, `TestNewRefusesBadSourceList` (fake source, fake writer, fake clock; no sleep).
6. `docs/contracts/jobs/fx_refresh.md` states the rank, the fill, that a run of any day does the fill, and what a filled day is (Decision 10).

### 6 source-rank-wire

Module: ot. Size: S.
Order: after 1 and 5 (it calls `AttachSecondary` through the interface of entry 1).
Files: `cmd/ot/wire.go`, `cmd/ot/wire_fxsecond.go`, `cmd/ot/wire_fxsecond_test.go`
Design: docs/design/source-rank.md#Decisions (7)
`wire.go` gets the in-place body replacement only and rebases on other edits without growth. Tests of `cmd/ot` that replace `d.newFxRefresh` with a fake keep working because `AttachSecondary` returns false for a fake job; they are not edited.
Acceptance:
1. With a FRED key the `fx_refresh` job built by the scheduler path and by the preflight path (`preflight.go:112`) has the FRED source attached; without a key it is built as before.
2. The attach goes through the package variable `attachFxSecondary` of `wire_fxsecond.go`, and `wire.go` stays under the 800-line ceiling.
3. `wire_fxsecond_test.go` has the new tests `TestFxRefreshGetsFredSourceWithKey` and `TestFxRefreshWithoutKeyHasNoFill` (a fake FRED source counts its calls over one run with a fake broker and store; the test does not assign the package variable).

## Checked by CI

Entry 1: `go test ./internal/jobs/fxrefresh/...`. Entry 2: `go test ./internal/contracts/...` and the generated-output check of the contracts workflow. Entry 3: `go test ./internal/sources/...`. Entry 4: the `pg` workflow. Entry 5: `go test ./internal/jobs/fxrefresh/...`. Entry 6: `go test ./cmd/ot/...`.

## Left out

- The bought daily-bar history (EODHD or another) and every price-bar second source: it waits for a reader's need (plan P5, cut order step 9).
- A void writer. A void by absence from an answer is unsafe (a hiccup, a holiday or a window shift would destroy a good rate and every page would read the day before); no reader needs a void yet. The safe form, a forced pull over a range the owner chose, comes with the console range field (step 2 of the plan).
- The cross-check (plan P4, console run, writes nothing, on demand and weekly for FX): owed to schema v2 P4 and not delivered here. It needs a place to show the differences; no page or console report exists. option-trader's ADR 0004 compares ECB with stored IDEALPRO mids and sets the threshold only after four weeks of shadow measurement; the same would apply here.
- The override by a forced console run that names the source (Decision 5): needs a source field in `jobrequest.Request` and a path to the job.
- Filling a stored `void` or `no_value` row (no writer makes one).
- The wiring of the NY Fed second source of `fred_series` (merged in #2272, not called from `cmd/ot`).
- FX pairs other than USDJPY (the plan did not verify that H.10 covers the pairs the accounts need).
- A registered-names table in place of the closed source check: every new source costs a migration and a contract change; plan P5 may want names registered instead.

## Doubts

1. FRED H.10 rates are noon New York buying rates with 2 decimals; IBKR midpoints have 4 and are session closes (Decision 10). A filled day differs a little from what IBKR would have said; the row's `source` says so.
2. That `DEXJPUS` covers the days the accounts need, and how many days FRED lags behind (H.10 is published weekly; the lag was not checked against a real answer, no FRED key is used here): not verified. The lookback of 14 days assumes a lag under two weeks.
3. Most fills arrive after the five-day IBKR window, so most stay FRED's 2-decimal rate (Decision 10).

## Adversarial

Answers to the points of the dispatcher's scratchpad (source-rank-adversarial.md) and of the advisor's review (r1).

1. Gap against the goal. Adopted: the false claim that FRED serves any series id is corrected (Today, Decision 4) and entries 2 and 3 add the unit, the `seriesUnits` row, the metadata fixture and the test. The cross-check and the override stay Left out and are named for the owner (the cross-check as owed to schema v2 P4). Rank as a validation list only: kept; "first writes by upsert, the rest by insert" is its meaning.
2. Void by absence. Adopted: the void writer is deleted; Left out names the reason.
3. Simpler way. Adopted: the fill is `InsertFxRates`. Reading DEXJPUS from stored series: not adopted, it would make the fx job depend on another job's output (a cascade); the call to FRED is direct (Decision 3).
4. Lock-in. The closed source check is kept (the existing pattern) and its cost is noted in Left out. `AttachSecondary` is now an interface (`AttachSecondaryFunc`) with a contract test, and the factory builds `fx_refresh` in one function.
5. Entry order and false green. Adopted: the job entry is after the store entry and merges after its migration ran; the sources entry lands before the job entry, so the FRED leg is real.
6. Wiring and files. `wire.go` cannot stay unedited: the factory method lives there (Decision 7); it is edited in place and does not grow. The `run_test.go` edit is unconditional; #2272 is merged, and what is wired is stated in Today.
7. Float conversion (advisor). The point value is a `float64`; the job converts it with `strconv.FormatFloat(v, 'f', -1, 64)`, skips nil and non-positive values, and a new test pins "153.42" (Decision 3, entry 5). A value with more than the two decimals FRED gives would print as it is; the store's `numeric` column keeps it.
8. "No other fx writer exists" (advisor). Checked with `git grep -n "UpsertFxRates\|InsertFxRates" -- '*.go'`: the only non-test caller of `UpsertFxRates` is `fxrefresh/run.go:186`; `InsertFxRates` has no non-test caller but the one this design adds, and the stand-in `noBarStore`. So the forced `fx_refresh` run is the only path that replaces a filled day (Decision 10).
9. The fill after a primary failure (B1): adopted, Decision 3 and `TestFillRunsWhenPrimaryFails`.
