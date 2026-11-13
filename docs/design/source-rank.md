# source rank, step 9 of the storage plan: the order of sources in a job.toml and FRED H.10 as the fill source of the USDJPY rate

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-14
Revision: 2

Meaning of "void" (the plan names it only in the cut order, step 9; the repository gives it a meaning and this design keeps it): `status = 'void'` is a stored key whose value must not be read. It is a legal status of `obs.fx_rate`, `obs.price_bar` and `obs.series_point`; the views hide it (`rd.fx_rate_r1` is `WHERE status = 'ok'`, `rd.series_point_r1` is `WHERE status <> 'void'`); an upsert replaces it. No code writes it and this design adds no writer (Left out).

Source: owner 2026-11-12 "fallback, not cascade", 2026-11-12 dev mode ("any job runs when triggered; only a same-job concurrency guard"), 2026-11-13 "overwrite by natural key and keep no old value" and "storage plan v2 OK"; storage plan v2 of the advisor (2026-11-13), P4 (fill automatic, cross-check writes nothing, override refused), P7, cut order step 9, section 6. Precedents copied, not re-argued: `data-repair-fx.md` (landed: `FxRateUpserter`, `status`, the grants), `series-registry.md` (landed: `secondary_source`), `fundamentals-keys.md` (entry shape, migration order).

Goal: the USDJPY rate has a second source. After the IBKR midpoint run, `fx_refresh` asks FRED H.10 (`DEXJPUS`, JPY per USD) for the days of the last `fill_lookback_days` and stores each day it gets and the table has no row for, with source `fred_h10`. It never replaces a stored row. The rank of the sources is the order of the names in `job.toml`. The pages and the figures read `rd.fx_rate_r1` with the same columns; the only visible change is a rate on a day IBKR failed, with 2 decimals.

## Today

All paths at origin/main f7d990f5.

- `internal/jobs/fxrefresh/run.go:34-` the manifest struct (`version`, `id`, `duration_days`, `schedule`); `New` takes the broker, the history client, `bar.FxRateWriter`, `bar.FxRateReader` (kept for the signature only) and the clock; `Run` asserts `bar.FxRateUpserter`, asks MIDPOINT bars ending on the day and upserts them with `writtenBy = "fx_refresh:" + day`. `job.toml` has no source list. `types_test.go` pins `NewFxRefreshFunc`, so the constructor is not changed.
- `bar.FxRateWriter.InsertFxRates` (`internal/store/bar/api.go`, `barpg/store.go:132`) inserts rows whose key is missing and leaves an existing key alone (`ON CONFLICT DO NOTHING`); `barpg.Store` implements it. This is the fill; no new store method is needed.
- `migrations/0039_fx_rate_repair.sql`: `obs.fx_rate (pair, rate_day, rate numeric, source, received_at, status ok|no_value|void, updated_at, written_by)`; `fx_rate_source_ck` of `0021_bars.sql:41` still reads `source = 'ibkr_midpoint'`; `ot_w_bar` keeps table INSERT.
- `contracts/datasets/obs.fx_rate.r1.json:16` and `internal/contracts/gen/records.go:1694-1718` (`FxRateSource`, one value, `UnmarshalJSON` refuses others); `docs/contracts/datasets/obs.fx_rate.md:12`. `contracts/datasets/obs.series_point.r1.json:31` the unit enum (`percent`, `ratio`, `ccy_amount`, `count`, `index_points`). Precedent for a unit added with generated output: `8d53b7d8` (fixtures, dataset document, `records.go`, `schema_test.go`).
- `internal/sources/fred/mapping.go:55-` `seriesUnits` lists the series the client accepts; `mapObservations` (`:115`) refuses a series id that is not in it ("no unit configured for series"); each row is checked against recorded metadata in `internal/sources/fred/testdata/series_units.json` by `mapping_test.go:181-294`. `DEXJPUS` is in none of them (`git grep DEXJPUS -- internal cmd migrations contracts` finds nothing), so FRED cannot return it today.
- `fred_series` second source is merged (`60474f7c`, #2272): `fredseries.NewWithSecondary(src, second, w, clk)` and `fillGaps` exist, `New` passes a nil second source, and `cmd/ot` never calls `NewWithSecondary` (`git grep NewWithSecondary -- cmd` finds nothing): the NY Fed fill is not wired. That wiring is not part of this design.
- `cmd/ot/wire.go:535-541` `jobFactory.fxRefresh` calls `f.d.newFxRefresh`; `jobFactory.src` (`cmd/ot/runonce.go:278`) is the FRED source, built only when a FRED key is set (`reasonNoFredKey`). `cmd/ot/wire.go` has 796 lines and is not edited here. `cmd/ot/jobconsole_jobs.go:95` already has `fx_refresh` as an any-day row with Force.
- Migrations on main end at `0044`; `0045` (readings, #2288), `0046` (`fundamentals-keys.md`) and `0047` (`option-captures.md`) belong to open designs. This design takes `0048`.

## Decisions

1. Rank is the order of names in `job.toml`. `fx_refresh` gains `sources = ["ibkr_midpoint", "fred_h10"]` and `fill_lookback_days = 14`. The first name is the primary: its answer is written with `UpsertFxRates` as today. Every later name is a fill source: its rows are written with `InsertFxRates` only. `New` fails when the list is empty, does not start with `ibkr_midpoint`, or names a source the job cannot serve. For series the rank is the registry's `source` and `secondary_source` (landed); no series code changes.
2. Fill (plan P4, owner "fallback, not cascade"). A fill source writes a key only when the table has none: `InsertFxRates` leaves a stored row, of any source, status or value, untouched. So a second source never overrides. A later IBKR answer replaces a FRED rate through the primary's own upsert (it differs, and IBKR outranks FRED by Decision 1). A stored `void` or `no_value` row is not filled (no writer makes one; Left out).
3. The FRED request. After the primary step, in the same `Run`, the job calls the attached source directly for series `DEXJPUS` over `[day - fill_lookback_days, day]`; it does not read the stored series, because a fallback that depended on another job's stored output would be a cascade. A point without a value (holiday, weekend) is skipped; a point with a value becomes `contracts.FxRate{Pair: USDJPY, Rate: value, Source: fred_h10}` and goes to `InsertFxRates`. FRED publishes H.10 with a lag of days, so a gap is filled by a later run; any run of any day does it. Like `fredseries` the fallback never changes the result of `Run`: a failed fetch or insert is logged and the result stays that of the primary. No time-of-day guard.
4. The FRED path (the client refuses unknown series). A new unit `jpy_per_usd` is added to the `obs.series_point` r1 unit enum with its generated output (entry 1); `seriesUnits` gets the row `"DEXJPUS": {unit: jpyPerUsd}`, `testdata/series_units.json` gets the recorded FRED metadata of the series and `mapping_test.go` its row (entry 2). The unit tells readers that the value is a rate, not an amount; nothing stores it as a series point here (the fx job converts the point to an `FxRate`).
5. Override is not offered. The plan allows "a replacement by a lower-ranked source ... by a forced console run that names the source". The console request has no source (`jobrequest.Request` is job, day, force) and `contracts.Job.Run` has no force argument. The refusal holds because the fill source has the insert method only.
6. Contract and database. The `source` enum of `obs.fx_rate` r1 gains `fred_h10`, generated output with it (entry 1). Migration `0048_fx_rate_source.sql` replaces `fx_rate_source_ck` with `source IN ('ibkr_midpoint', 'fred_h10')` (a closed list, as the existing pattern; every new source costs a migration, Left out); no grant, view or column changes. The Down part restores the old check and refuses (an exception, nothing changed) when a `fred_h10` row exists. Owner-only DDL, one transaction.
7. Wiring. `fxrefresh.AttachSecondary(j contracts.Job, second sources.Source) bool` attaches the fill source to a job `New` built and reports false for any other job (a test fake), so `NewFxRefreshFunc`, its tests and the `cmd/ot` fakes keep working. A new file `cmd/ot/wire_fxsecond.go` holds the one function the factory uses to build `fx_refresh` with `f.src` attached when it is not nil; without a FRED key there is no fill and `fx_refresh` runs as today. `jobFactory.fxRefresh` (`cmd/ot/wire.go:535`) is the only place `fx_refresh` is built (`preflight.go:112` and `wire.go:586` both call it, not `registerJob`), so `wire.go` must change: the body of `fxRefresh` is replaced in place by a call to the new function, a change that adds no line.
8. Own code behind an interface. The attach function is our glue behind the `contracts.Job` interface; the FRED client is the existing `internal/sources/fred`; no mainstream library knows our key and statuses.
9. No old value is kept (owner 2026-11-13); this design deletes none.

## Modules

No row of `docs/design/modules.md` changes. Entries touch contracts (`contracts/**`, `internal/contracts/**`, `docs/contracts/datasets/**`; entry 1), sources (`internal/sources/**`; entry 2), store (`migrations/**`, `internal/store/**`, `docs/contracts/store.md`; entry 3), jobs (entry 4) and ot (entry 5). New import edges: none (`fxrefresh` already imports `sources`, `contracts`, `internal/store/bar`; `cmd/ot` already imports `fxrefresh`). The change adds no interface file and no declaration of an interface package; the data contract changes are entry 1.

## Issues

Build order: every entry compiles and its tests pass against main. BASE is `git merge-base origin/main HEAD`. Entry 3 holds the auto-deploy (migration); entry 4 merges after the window ran it. Contracts: changed (entry 1 edits two files under `contracts/datasets` with their generated output); the other entries carry `Contracts: unchanged`.

### 1 source-rank-contract

Module: contracts. Size: S.
Order: first.
Files: `contracts/datasets/obs.fx_rate.r1.json`, `contracts/datasets/obs.series_point.r1.json`, `contracts/fixtures/MANIFEST.synthetic-v1.json`, `internal/contracts/gen/records.go`, `docs/contracts/datasets/obs.fx_rate.md`, `docs/contracts/datasets/obs.series_point.md`, `internal/contracts/schema_test.go`
Design: docs/design/source-rank.md#Decisions (4, 6)
`contracts/generate.sh` writes the generated file and the manifest hash follows `8d53b7d8`; `internal/contracts/api_test.go` uses `contracts.FxRate` and is not edited.
Acceptance:
1. The `source` enum of `obs.fx_rate` r1 holds `ibkr_midpoint` and `fred_h10`, and the `unit` enum of `obs.series_point` r1 holds `jpy_per_usd` after the existing values; the generated records have `FxRateSourceFredH10` and `SeriesUnitJpyPerUsd`, accept the new values and still refuse any other; both dataset documents list the values. The generated output is what `contracts/generate.sh` writes from the source files.
2. `schema_test.go` has the new tests `TestFxRateSourceFredH10Accepted` (a record with source `fred_h10` validates, source `stooq` is refused) and `TestSeriesUnitJpyPerUsdAccepted`.

### 2 source-rank-fred

Module: sources. Size: S.
Order: after 1 (it uses the generated unit constant).
Files: `internal/sources/fred/mapping.go`, `internal/sources/fred/testdata/series_units.json`, `internal/sources/fred/mapping_test.go`
Design: docs/design/source-rank.md#Decisions (4)
Acceptance:
1. The client maps a `DEXJPUS` observation: a value becomes a point with unit `jpy_per_usd`, a "." observation becomes a point without a value, and the series is no longer refused with "no unit configured".
2. `testdata/series_units.json` holds the recorded FRED metadata of `DEXJPUS` and the metadata comparison in `mapping_test.go` passes with the new `seriesUnits` row; `mapping_test.go` has the new test `TestMapDEXJPUSValueAndMissing` (inline observations in FRED's JSON form).

### 3 source-rank-store

Module: store. Size: S.
Order: after 1; after the store entries that carry `0045` (readings entry 2), `0046` (fundamentals-keys entry 2) and `0047` (the migration entry of option-captures); this entry takes `0048` and rebases on them (all edit `docs/contracts/store.md`). Holds the auto-deploy: it carries a migration.
Files: `migrations/0048_fx_rate_source.sql`, `internal/store/barpg/fill_pg_test.go`, `internal/store/barpg/store_pg_test.go`, `internal/store/barpg/upsert_pg_test.go`, `docs/contracts/store.md`
Design: docs/design/source-rank.md#Decisions (2, 6)
`store_pg_test.go` and `upsert_pg_test.go` list the migrations their databases apply and gain `0048`; they are edited for that list only.
Acceptance:
1. `0048_fx_rate_source.sql` has an Up and a Down part, is owned by `ot_owner`, and changes only `fx_rate_source_ck`: it admits `fred_h10` beside `ibkr_midpoint`; the Down part restores the old check and refuses, changing nothing, when a `fred_h10` row exists. No `GRANT` or `REVOKE` line is written.
2. As `ot_w_bar`, `InsertFxRates` stores a `fred_h10` row for a missing key and leaves a stored `ibkr_midpoint` row of the same key unchanged; a source name outside the list is refused.
3. `fill_pg_test.go` (tag `pg`) has the new tests `TestInsertFredRateForMissingKey`, `TestInsertNeverReplacesIbkrRate`, `TestSourceOutsideListRefused`, `TestMigration0048DownRefusesFredRows`.
4. `docs/contracts/store.md` states that `fred_h10` is a fill source stored by insert only, and `0048` (`git grep -n "0048" docs/contracts/store.md` finds it).

### 4 source-rank-job

Module: jobs. Size: M.
Order: after 1 and 2; merge after the window ran the migration of entry 3 (before it, a `fred_h10` insert is refused by the old check and logged; nothing else changes).
Files: `internal/jobs/fxrefresh/run.go`, `internal/jobs/fxrefresh/secondary.go`, `internal/jobs/fxrefresh/secondary_test.go`, `internal/jobs/fxrefresh/job.toml`, `internal/jobs/fxrefresh/run_test.go`, `internal/jobs/fxrefresh/types_test.go`, `docs/contracts/jobs/fx_refresh.md`
Design: docs/design/source-rank.md#Decisions (1 to 3, 7)
`run_test.go` builds the job through `New` with a fake writer and gains the manifest keys of the new `job.toml` in its fixtures; `types_test.go` pins `NewFxRefreshFunc` and is edited only if the constructor test needs it.
Acceptance:
1. `job.toml` carries `sources = ["ibkr_midpoint", "fred_h10"]` and `fill_lookback_days = 14`; `New` returns an error for a manifest whose list is empty, does not start with `ibkr_midpoint` or names an unknown source; `NewFxRefreshFunc` and `New` keep their signatures.
2. `AttachSecondary` attaches the source to a job `New` built and returns false for any other `contracts.Job`.
3. With a source attached, a run asks it for `DEXJPUS` over the lookback range and stores each point with a value as source `fred_h10` through `InsertFxRates`; a point without a value is not written; a stored rate is never replaced by a FRED value; the `Run` result is that of the primary step in every case.
4. A failed FRED fetch, a failed insert and a job without a source each leave the `Run` result unchanged and are logged; no source attached means no FRED call. The FRED call does not depend on any stored series.
5. `secondary_test.go` has the new tests `TestFillInsertsFredPointsWithValue`, `TestFillNeverReplacesStoredRate`, `TestNoValuePointSkipped`, `TestSecondaryFailureKeepsResult`, `TestNoSourceNoCall`, `TestAttachSecondaryForeignJob`, `TestNewRefusesBadSourceList` (fake source, fake writer, fake clock; no sleep).
6. `docs/contracts/jobs/fx_refresh.md` states the rank, the fill and that a run of any day does the fill.

### 5 source-rank-wire

Module: ot. Size: S.
Order: after 4 (it calls `AttachSecondary`).
Files: `cmd/ot/wire.go`, `cmd/ot/wire_fxsecond.go`, `cmd/ot/wire_fxsecond_test.go`
Design: docs/design/source-rank.md#Decisions (7)
`wire.go` stands at 796 lines and gets the in-place body replacement only; the new code is in the new file. Tests of `cmd/ot` that replace `d.newFxRefresh` with a fake keep working because `AttachSecondary` returns false for a fake job; they are not edited.
Acceptance:
1. With a FRED key the built `fx_refresh` job has the FRED source attached; without one it is built as before.
2. `fx_refresh` built by the scheduler path and by the preflight path (`preflight.go:112`) both have the source attached, and `wire.go` stays under the 800-line ceiling (the body of `fxRefresh` is replaced, none is added to).
3. `wire_fxsecond_test.go` has the new tests `TestFxRefreshGetsFredSourceWithKey` and `TestFxRefreshWithoutKeyHasNoFill` (a fake FRED source counts its calls over one run with a fake broker and store).

## Checked by CI

Entry 1: `go test ./internal/contracts/...` and the generated-output check of the contracts workflow. Entry 2: `go test ./internal/sources/...`. Entry 3: the `pg` workflow. Entry 4: `go test ./internal/jobs/fxrefresh/...`. Entry 5: `go test ./cmd/ot/...`.

## Left out

- The bought daily-bar history (EODHD or another) and every price-bar second source: it waits for a reader's need (plan P5, cut order step 9).
- A void writer. A void by absence from an answer is unsafe (a hiccup, a holiday or a window shift would destroy a good rate and every page would read the day before); no reader needs a void yet. The safe form, a forced pull over a range the owner chose, comes with the console range field (step 2 of the plan).
- The cross-check (plan P4, writes nothing): it needs a place to show the differences; no page or console report exists. option-trader's ADR 0004 compares ECB with stored IDEALPRO mids and sets the threshold only after four weeks of shadow measurement; the same would apply here.
- The override by a forced console run that names the source (Decision 5): needs a source field in `jobrequest.Request` and a path to the job.
- Filling a stored `void` or `no_value` row (no writer makes one).
- The wiring of the NY Fed second source of `fred_series` (merged in #2272, not called from `cmd/ot`).
- FX pairs other than USDJPY (the plan did not verify that H.10 covers the pairs the accounts need).
- A registered-names table in place of the closed source check: every new source costs a migration and a contract change; plan P5 may want names registered instead.

## Doubts

1. FRED H.10 rates have 2 decimals and are noon New York buying rates; IBKR midpoints have 4 and are session closes. A filled day differs a little from what IBKR would have said; the row's `source` says so and an IBKR run replaces it while the day is inside the five-day window (an older day stays FRED's).
2. That `DEXJPUS` covers the days the accounts need is not verified.

## Adversarial

Answers to the six points of the dispatcher's scratchpad (source-rank-adversarial.md).

1. Gap against the goal. Adopted: the false claim that FRED serves any series id is corrected (Today, Decision 4) and entries 1 and 2 add the unit, the `seriesUnits` row, the metadata fixture and the test. The cross-check and the override stay Left out and are named for the owner. Rank as a validation list only: kept; "first writes by upsert, the rest by insert" is its meaning.
2. Void by absence. Adopted: Decision 4 of revision 1, `FxRateVoider`, the void write, their tests and the doubt are deleted; Left out names the reason.
3. Simpler way. Adopted: the fill is `InsertFxRates` and the `Interface` section is gone with its marking; the dead `no_value` arm is gone. Reading DEXJPUS from stored series: not adopted, it would make the fx job depend on another job's output (a cascade); the call to FRED is direct (Decision 3).
4. Lock-in. The closed source check is kept (the existing pattern) and its cost is noted in Left out. `AttachSecondary` stays a small attach function behind `contracts.Job`; the factory does not gain a second construction path (the builder is one function in the new file).
5. Entry order and false green. Adopted: entry 4 merges after entry 3 ran, and entry 2 lands before entry 4, so the FRED leg is real, not only green on fakes.
6. Wiring and files. Adopted: `wire.go` cannot stay unedited: the factory method that builds `fx_refresh` lives there (Decision 7); the item is result-style and the file is edited in place only; the `run_test.go` edit is unconditional; #2272 is stated as merged with what is and is not wired.
