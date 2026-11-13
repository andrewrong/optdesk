# source rank, step 9 of the storage plan: the order of sources in a job.toml, the void mark, and FRED H.10 as the second source of the USDJPY rate

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-14
Revision: 1

Meaning of "void" (the plan names it only in the cut order, step 9; the repository gives it a meaning, and this design uses that one): `status = 'void'` is a stored key whose value must not be read. It is already a legal status of `obs.fx_rate`, `obs.price_bar` and `obs.series_point`; the readers' views hide it (`rd.fx_rate_r1` is `WHERE status = 'ok'`, `rd.series_point_r1` is `WHERE status <> 'void'`); a fill or an upsert replaces it; and no code writes it today (`git grep -n "'void'" -- internal migrations` finds only the migrations, `internal/store/series/upsert.go` and tests). `docs/design/data-repair-fx.md` Decision 10 says "marking a day the source no longer has needs the range of a forced pull". This design adds the first writer, for the FX rate only.

Source: owner 2026-11-12 "fallback, not cascade", 2026-11-12 dev mode ("any job runs when triggered; only a same-job concurrency guard"), 2026-11-13 "overwrite by natural key and keep no old value" and "storage plan v2 OK"; storage plan v2 of the advisor (2026-11-13), P4 (fill automatic, cross-check writes nothing, override refused), P7, cut order step 9, section 6. Precedents copied, not re-argued: `data-repair-fx.md` (landed: `FxRateUpserter`, `FxWriteCount`, the grants, `status`), `series-registry.md` (landed: `secondary_source`, `FillSeriesPoints`), `fundamentals-keys.md` (the entry shape and the migration order).

Goal: the USDJPY rate has a second source. After the IBKR midpoint run, `fx_refresh` asks FRED H.10 (`DEXJPUS`, JPY per USD) for the days of the last `fill_lookback_days` that have no readable rate and stores each one it gets with source `fred_h10`; it never replaces a readable rate. When the IBKR answer lacks a day between two days it has, and that day is stored from IBKR, the day is marked `void` and the fill may replace it later. The rank of the sources is the order of the names in `job.toml`. The pages and the figures read `rd.fx_rate_r1` with the same columns; the only visible change is a rate on a day that IBKR failed, with a 2-decimal value.

## Today

All paths at origin/main f7d990f5.

- `internal/jobs/fxrefresh/run.go:34-` the manifest struct (`version`, `id`, `duration_days`, `schedule`); `New` takes the broker, the history client, `bar.FxRateWriter` and `bar.FxRateReader` (the reader "is kept for the signature only"), the clock; `Run` asserts `bar.FxRateUpserter`, asks MIDPOINT bars ending on the day and upserts them with `writtenBy = "fx_refresh:" + day`. `internal/jobs/fxrefresh/job.toml` has no source list. `types.go` holds `NewFxRefreshFunc`; `types_test.go` pins its shape, so the constructor is not changed here.
- `migrations/0039_fx_rate_repair.sql`: `obs.fx_rate (pair, rate_day, rate numeric, source, received_at, status ok|no_value|void, updated_at, written_by)`; `fx_rate_source_ck` of `0021_bars.sql:41` still reads `source = 'ibkr_midpoint'`; `ot_w_bar` has `UPDATE (rate, source, status, updated_at, written_by)` and `SELECT (pair, rate_day, rate, source, status)` and table INSERT. `rate` may be NULL, `fx_rate_rate_ck` is `status = 'no_value' OR rate > 0` (a NULL rate passes it, as `internal/store/barpg/store_pg_test.go:359` shows).
- `contracts/datasets/obs.fx_rate.r1.json:16` and `internal/contracts/gen/records.go:1694-1718` (`FxRateSource`, one value, `UnmarshalJSON` refuses others); `docs/contracts/datasets/obs.fx_rate.md:12`. Precedent for an enum change with its generated output: `8d53b7d8` (index_points unit).
- `internal/store/bar/api.go` (94 lines) holds `FxRateWriter`, `FxWriteCount`, `FxRateUpserter`, `FxRateReader`; `internal/store/barpg/store.go` (243 lines) implements them; `read.go` reads through `rd.fx_rate_r1`, so a void row is not seen.
- `internal/sources/api.go:31` `sources.Source.Fetch(ctx, FetchPlan{SeriesIDs, From, To})` returns `[]contracts.SeriesPoint`; `internal/sources/fred` serves any series id, a day FRED marks "." comes back without a value (`mapObservations`, `mapping.go:115`). `internal/jobs/fredseries/secondary.go` on `origin/claude/issue-2272` (not merged) is the pattern: `NewWithSecondary`, a fallback that never changes the result of `Run` and logs every failure, points written with `FillSeriesPoints` only. `obs.series_definition.secondary_source` (migration 0043, `'' | 'nyfed'`) is the series rank and needs no change here.
- `cmd/ot/wire.go:535-541` `jobFactory.fxRefresh` calls `f.d.newFxRefresh`; `jobFactory.src` (`cmd/ot/runonce.go:278`) is the FRED source, built only when a FRED key is set (`reasonNoFredKey`, `wire_fred.go`). `cmd/ot/wire.go` has 796 lines and may not grow. `cmd/ot/jobconsole_jobs.go:95` already has `fx_refresh` as `kindAnyDay` with Force: no console change.
- Migrations on main end at `0044`; `0045` (readings, #2288), `0046` (`fundamentals-keys.md`) and `0047` (`option-captures.md`) belong to open designs. This design takes `0048`.

## Decisions

1. Rank is the order of names in `job.toml`. `fx_refresh` gains `sources = ["ibkr_midpoint", "fred_h10"]` and `fill_lookback_days = 14`. The first name is the primary: its answer is written with `UpsertFxRates` (a differing value replaces, as today). Every later name is a fill source: its rows are written with `FillFxRates` only. `New` fails when the list is empty, names a source the job cannot serve, or does not start with `ibkr_midpoint`. For series the rank is the registry's `source` and `secondary_source` (landed); no series code changes here.
2. Fill (plan P4, owner "fallback, not cascade"). `bar.FxRateFiller.FillFxRates` inserts a row whose key is missing and replaces a stored row only when its status is `no_value` or `void`; a stored `ok` row is Unchanged whatever its source or value. So a second source never overrides, and an IBKR rate that arrives later replaces a FRED rate through the primary's own upsert (it differs, and IBKR outranks FRED by Decision 1).
3. The FRED request. After the primary step, in the same `Run`, the job reads the readable rates of `[day - fill_lookback_days, day]` through `FxRateReader.FxRates`, and asks the attached source for series `DEXJPUS` over the days of that range that have none. A FRED point without a value (holiday, weekend) is skipped; a point with a value becomes `contracts.FxRate{Pair: USDJPY, Rate: value, Source: fred_h10}` and goes to `FillFxRates` with `writtenBy = "fx_refresh:" + day + ":fred_h10"`. FRED publishes H.10 with a lag of days, so a gap is filled by a later run; any run of any day (scheduled, console, with or without Force) does it. Like `fredseries` the fallback never changes the result of `Run`: no key, no source attached, a failed fetch or a failed fill is logged and the run result stays that of the primary. No time-of-day guard.
4. The void mark (meaning above). After a non-empty primary answer the job marks `void` every stored row of the primary's source on a day that lies strictly between the first and the last day of the answer and is not in it. Never on an empty answer, never at the edges of the window, never a row of another source, never a `no_value` row. `bar.FxRateVoider.VoidFxRates(ctx, pair, source, days, writtenBy)` sets `status = 'void'` and `rate = NULL` on the stored rows of that pair and source whose status is `ok` and returns how many it changed; the key and `received_at` stay. A void row is hidden from every reader; a later upsert of the primary or a fill replaces it (Decision 2 and `UpsertFxRates`, whose compared columns include `status`). Only FX has a void writer: price bars and series have no forced range to decide on (Left out).
5. Override is not offered. The plan allows "a replacement by a lower-ranked source ... by a forced console run that names the source". The console request has no source (`jobrequest.Request` is job, day, force) and `contracts.Job.Run` has no force argument, so no path to that run exists; adding one is Left out. The refusal holds because the secondary has the fill method only.
6. Contract and database. The `source` enum of `obs.fx_rate` r1 gains `fred_h10`, generated output with it (entry 2). Migration `0048_fx_rate_source.sql` replaces `fx_rate_source_ck` with `source IN ('ibkr_midpoint', 'fred_h10')`; no grant, view or column changes (the grants of `0039` already cover the fill and the void update). The Down part restores the old check and refuses (an exception, nothing changed) when a `fred_h10` row exists. Owner-only DDL, one transaction.
7. Statements (as `data-repair-fx.md` Decision 4). Fill: one statement per row in one transaction, `INSERT ... ON CONFLICT (pair, rate_day) DO UPDATE SET rate, source, status = 'ok', updated_at, written_by WHERE stored status <> 'ok' RETURNING pair`. Void: one `UPDATE obs.fx_rate SET status = 'void', rate = NULL, updated_at = now(), written_by WHERE pair = $1 AND source = $2 AND rate_day = ANY($3) AND status = 'ok'`; both use only the columns `ot_w_bar` can already read and update. An empty list makes no call.
8. Wiring. `fxrefresh.AttachSecondary(j contracts.Job, second sources.Source) bool` attaches the fill source to a job `New` built and reports false for any other job (a test fake), so `NewFxRefreshFunc`, its tests and the `cmd/ot` fakes keep working. `jobFactory.fxRefresh` calls it with `f.src` when `f.src` is not nil; without a FRED key there is no fill and `fx_refresh` runs as today. The new wiring lives in a new file `cmd/ot/wire_fxsecond.go`; `wire.go` changes only the body of `fxRefresh` and ends no longer than at BASE.
9. Own code behind an interface. The fill and void writers are our SQL behind two small store interfaces; nothing mainstream replaces them (a library does not know our key and statuses). The FRED client is the existing `internal/sources/fred`.
10. No old value is kept (owner 2026-11-13): a voided rate has its value set to NULL; a wrong value is repaired by another run from a source that still has the right one.

## Modules

No row of `docs/design/modules.md` changes: `internal/store/bar` and `internal/store/barpg` are already under `store`, `internal/jobs/fxrefresh` under `jobs`, `cmd/ot` under `ot`, the dataset files under `contracts`. Entries touch store (`internal/store/**`, `migrations/**`, `docs/contracts/store.md`; entries 1 and 3), contracts (entry 2), jobs (entry 4) and ot (entry 5). New import edges: none (`internal/jobs/fxrefresh` already imports `internal/store/bar`, `sources` and `contracts`; `cmd/ot` already imports `fxrefresh` and `sources`).

## Interface

One interface change, entry 1, in `internal/store/bar/api.go` (additive; nothing existing changes, no fake breaks):

    // FxRateFiller writes a rate of a lower-ranked source: a missing key is inserted, a stored row whose status is
    // no_value or void is replaced, a stored ok row is never changed. Optional method of the value OpenBarFunc
    // returns; the job asserts it before it asks a second source.
    type FxRateFiller interface {
        FillFxRates(ctx context.Context, rows []contracts.FxRate, writtenBy string) (FxWriteCount, error)
    }

    // FxRateVoider marks stored ok rows of one pair and one source on the given days void: status void, rate NULL,
    // the key kept. It returns the number of rows changed; rows of another source or of another status are not
    // touched. An empty list makes no call. Optional method of the same value.
    type FxRateVoider interface {
        VoidFxRates(ctx context.Context, pair, source string, days []contracts.Day, writtenBy string) (int, error)
    }

The other interface change is data, entry 2: the `source` enum of `obs.fx_rate` r1 gains `fred_h10`.

## Issues

Build order: every entry compiles and its tests pass against main. BASE is `git merge-base origin/main HEAD`. Entry 3 holds the auto-deploy (migration); entries 4 and 5 merge after the window ran it. Contracts: changed (entry 2 edits `contracts/datasets/obs.fx_rate.r1.json` with its generated output); the other entries carry `Contracts: unchanged`. Test runs are in the `## Checked by CI` list of each entry.

### 1 source-rank-api (interface)

Module: store. Size: S.
Order: first.
Files: `internal/store/bar/api.go`, `internal/store/bar/api_test.go`
Design: docs/design/source-rank.md#Interface
Acceptance:
1. `api.go` declares `FxRateFiller` and `FxRateVoider` as in the Interface section, with docs and no function body; the existing declarations are unchanged (`git diff BASE -- internal/store/bar/api.go` shows added lines only).
2. `api_test.go` has new contract tests `TestFxRateFillerContract` (a fake that fills a missing key, replaces a `no_value` row, leaves an `ok` row; the caller reads the written and the unchanged count) and `TestFxRateVoiderContract` (a fake voids only the ok rows of the named source; an empty list returns 0).
3. `git diff --name-only BASE -- internal/store/api.go internal/store/api_test.go` prints nothing.


### 2 source-rank-contract

Module: contracts. Size: S.
Order: first.
Files: `contracts/datasets/obs.fx_rate.r1.json`, `internal/contracts/gen/records.go`, `docs/contracts/datasets/obs.fx_rate.md`, `internal/contracts/schema_test.go`
Design: docs/design/source-rank.md#Decisions (6)
`contracts/generate.sh` writes the generated file; `internal/contracts/api_test.go` uses `contracts.FxRate` and is not edited.
Acceptance:
1. The `source` enum of `obs.fx_rate` r1 holds `ibkr_midpoint` and `fred_h10`; the generated record has the constant `FxRateSourceFredH10` and its `UnmarshalJSON` accepts the value and still refuses any other; the dataset document lists both values. The generated output is the one `contracts/generate.sh` writes from the source file.
2. `schema_test.go` has the new test `TestFxRateSourceFredH10Accepted` (a record with source `fred_h10` validates against the schema; a record with source `stooq` is refused).


### 3 source-rank-store

Module: store. Size: M.
Order: after 1 and 2; after the store entries that carry `0045` (readings entry 2), `0046` (fundamentals-keys entry 2) and `0047` (the migration entry of option-captures); this entry takes `0048` and rebases on them (all edit `docs/contracts/store.md`). Holds the auto-deploy: it carries a migration.
Files: `migrations/0048_fx_rate_source.sql`, `internal/store/barpg/fill.go`, `internal/store/barpg/fill_test.go`, `internal/store/barpg/fill_pg_test.go`, `internal/store/barpg/store_pg_test.go`, `internal/store/barpg/upsert_pg_test.go`, `docs/contracts/store.md`
Design: docs/design/source-rank.md#Decisions (2, 4, 6, 7)
`store_pg_test.go` and `upsert_pg_test.go` list the migrations their databases apply (`newDatabase`, `applyUp`) and gain `0048`; they are edited for that list only.
Acceptance:
1. `0048_fx_rate_source.sql` has an Up and a Down part, is owned by `ot_owner`, and changes only `fx_rate_source_ck`: it admits `fred_h10` beside `ibkr_midpoint`; the Down part restores the old check and refuses, changing nothing, when a `fred_h10` row exists. No `GRANT` or `REVOKE` line is written.
2. `barpg.Store` fills by Decision 7: a missing key is inserted with its source; a stored `no_value` or `void` row is replaced; a stored `ok` row is left whatever the new row's value or source; the count gives the rows written and the rows unchanged; an empty list makes no database call. The writes are in `fill.go` with compile-time assertions for `bar.FxRateFiller` and `bar.FxRateVoider`.
3. `VoidFxRates` voids only the `ok` rows of the given pair, source and days (value NULL, key and `received_at` kept) and returns their number; a row of another source, a `no_value` row and a day not listed are untouched; `rd.fx_rate_r1` no longer shows a voided row.
4. As `ot_w_bar` alone: the fill, the void and an upsert over a voided row all succeed with the grants of `0039` and no new grant; the role still cannot delete or truncate.
5. `fill_test.go` (fake database) has new tests `TestFillSendsOneStatementPerRow`, `TestFillEmptyListMakesNoCall`, `TestVoidEmptyListMakesNoCall`, `TestVoidSendsPairSourceAndDays`; `fill_pg_test.go` (tag `pg`) has new tests `TestFillNeverReplacesOkRow`, `TestFillReplacesVoidAndNoValue`, `TestVoidHidesFromView`, `TestUpsertReplacesVoidRow`, `TestMigration0048DownRefusesFredRows`, `TestMigration0048AcceptsFredH10Only`.
6. `docs/contracts/store.md` states the fill rule, the void meaning and `0048` (`git grep -n "0048" docs/contracts/store.md` finds it).
7. `git diff --name-only BASE` lists only paths of the Files line; every new source file is under 600 lines.


### 4 source-rank-job

Module: jobs. Size: M.
Order: after 1; merge after the window ran the migration of entry 3 (before it, a `fred_h10` row is refused and the fallback logs it; nothing else changes).
Files: `internal/jobs/fxrefresh/run.go`, `internal/jobs/fxrefresh/secondary.go`, `internal/jobs/fxrefresh/secondary_test.go`, `internal/jobs/fxrefresh/job.toml`, `internal/jobs/fxrefresh/run_test.go`, `internal/jobs/fxrefresh/types_test.go`, `docs/contracts/jobs/fx_refresh.md`
Design: docs/design/source-rank.md#Decisions (1 to 5, 8)
`run_test.go` builds the job through `New` with a fake writer; `types_test.go` pins `NewFxRefreshFunc`; both are edited only if the new manifest keys require it.
Acceptance:
1. `job.toml` carries `sources = ["ibkr_midpoint", "fred_h10"]` and `fill_lookback_days = 14`; `New` returns an error for a manifest whose list is empty, does not start with `ibkr_midpoint` or names an unknown source; `NewFxRefreshFunc` and `New` keep their signatures.
2. `AttachSecondary` attaches the source to a job `New` built and returns false for any other `contracts.Job`.
3. With a source attached and a writer that has the fill method, a run asks the source for `DEXJPUS` over the days of the lookback range that have no readable rate and stores each point with a value as source `fred_h10` through the fill method; a day FRED gives without a value is not written; a stored IBKR rate is never replaced by a FRED value; the `Run` result is that of the primary step in every case.
4. A failed FRED fetch, a failed fill, a writer without the fill method and a job without a source each leave the `Run` result unchanged and are logged; no source attached means no FRED call.
5. After a non-empty IBKR answer, a stored IBKR row on a day strictly between the first and the last answered day and missing from the answer is voided through the void method; nothing is voided for an empty answer, for the edge days of the answer, for a row of another source or when the writer has no void method.
6. A later IBKR answer that holds a voided day replaces it through the upsert, and a later FRED point fills it when IBKR does not answer.
7. `secondary_test.go` has new tests `TestFillFromSecondaryOnlyMissingDays`, `TestFillNeverOverridesOkRate`, `TestNoValuePointSkipped`, `TestSecondaryFailureKeepsResult`, `TestVoidInsideAnswerOnly`, `TestVoidNotOnEmptyAnswer`, `TestAttachSecondaryForeignJob`, `TestNewRefusesBadSourceList` (fake source, fake writer, fake clock; no sleep).
8. `docs/contracts/jobs/fx_refresh.md` states the rank, the fill, the void rule and that a run of any day does the fill.
9. `git diff --name-only BASE` lists only paths of the Files line; every new source file is under 600 lines.


### 5 source-rank-wire

Module: ot. Size: S.
Order: after 4 (it calls `AttachSecondary`).
Files: `cmd/ot/wire.go`, `cmd/ot/wire_fxsecond.go`, `cmd/ot/wire_fxsecond_test.go`
Design: docs/design/source-rank.md#Decisions (8)
`wire.go` stands at 796 lines. Tests of `cmd/ot` that replace `d.newFxRefresh` with a fake (`runonce_test.go`, `scheduler_test.go`, `backfill_test.go`) keep working because `AttachSecondary` returns false for a fake job; they are not edited.
Acceptance:
1. `jobFactory.fxRefresh` attaches the FRED source to the built job when a FRED key is set and builds the job as before without one; the body change in `wire.go` leaves the file no longer than at BASE (`wc -l cmd/ot/wire.go` prints at most the BASE count), the new code is in `wire_fxsecond.go`.
2. `wire_fxsecond_test.go` has new tests `TestFxRefreshGetsFredSourceWithKey` and `TestFxRefreshWithoutKeyHasNoFill` (a fake FRED source counts its calls over one run with a fake broker and store; a fake `newFxRefresh` job is built without error).
3. `git diff --name-only BASE` lists only paths of the Files line.


## Checked by CI

Entry 1: `go test ./internal/store/bar/...`. Entry 2: `go test ./internal/contracts/...` and the generated-output check of the contracts workflow. Entry 3: `go test ./internal/store/barpg/...` and the `pg` workflow. Entry 4: `go test ./internal/jobs/fxrefresh/...`. Entry 5: `go test ./cmd/ot/...`.

## Left out

- The bought daily-bar history (EODHD or another), and every price-bar second source: it waits for a reader's need (plan P5, cut order step 9). The void writer for bars waits with it.
- The cross-check (plan P4: compare the second source with the stored values, write nothing). It needs a place to show the differences; no page or console report exists, and a log line is not a report. option-trader's ADR 0004 compares ECB with stored IDEALPRO mids and sets the threshold only after four weeks of shadow measurement; the same would apply here. Not designed until the owner names where the report is read.
- The override by a forced console run that names the source (Decision 5): needs a source field in `jobrequest.Request` and a path to the job.
- The void mark for `obs.price_bar` and `obs.series_point`, and a void written by a range field of the console (step 2 of the plan).
- A second source for any series (a series has one publisher; NY Fed as `secondary_source` is landed and its wiring is #2272).
- FX pairs other than USDJPY (the plan did not verify that FRED H.10 covers the pairs the accounts need; USDJPY is `DEXJPUS`).
- Removal of the unused `FxRateWriter` insert path (`data-repair-fx.md` Left out).

## Doubts

1. Void by absence (Decision 4): a stored IBKR day strictly inside the answered range and missing from it is treated as retracted. If IBKR omits a day only because of the window it chose, a good rate is hidden until the next run restores it (the primary's upsert replaces a void row with the same value). The restriction to the inside of the answer and to a non-empty answer is the guard; the owner may prefer no void writer until a forced range exists.
2. FRED H.10 rates have 2 decimals and are noon New York buying rates; IBKR midpoints have 4 and are session closes. A filled day differs from what IBKR would have said by a small amount; the row's `source` says so and an IBKR run replaces it.
3. That FRED `DEXJPUS` covers the days the accounts need is not verified (the plan's own "not verified").

## Adversarial

- A FRED value overriding a good IBKR rate: refused by the fill statement (`WHERE stored status <> 'ok'`), tested by `TestFillNeverReplacesOkRate` and `TestFillNeverOverridesOkRate`.
- A migration not yet run when entry 4 lands: the fill is refused by the old check and logged; the primary step is untouched.
- A run with no FRED key: no source attached, the job is as today.
