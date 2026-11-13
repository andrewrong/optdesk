# series registry, step 4 of the storage plan: one row per (series, date), a registry of what each series is, four credit and funding series, NY Fed as second source

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-13
Revision: 1

Source: owner 2026-11-13 S1 to S4 ("overwrite by key, a re-run replaces", "key = the series and the data's own date", "do not hard-wire the schema; a new series is a registry row", "liquidity later, background only"), "storage plan v2 OK" (07:11Z); storage plan v2 of the advisor, P4 (a secondary source only fills gaps and never overwrites the primary), P7, section 3, cut order step 4. The precedent is `data-repair-fx.md` (landed) and `data-repair-bars.md` (steps 2 and 3, drafted in parallel): table shape (`status`, `updated_at`, `written_by`), column grants, optional-upserter interface and any-day console row with Force are copied, not re-argued.

Goal: `obs.series_point` is keyed `(series_id, observation_date)` and a run replaces a differing value; `obs.series_definition` becomes the registry (title, category, frequency, source, the publisher's own id, backup source); four more FRED credit and funding series are collected; NY Fed fills a gap of a series that names it as backup. A forced past day on the `fred_series` console row repairs a series. For the owner: a revised macro value now shows the revised number (today the first value seen stays); page shapes and the front end do not change.

## Today

All paths at origin/main 29c24a9a.

- `migrations/0002_series.sql` creates `obs.series_definition (series_id, unit, currency)` and `obs.series_point (series_id, observation_date, realtime_start, realtime_end, value double precision, status, unit, currency, published_at, source, source_revision)`, key `(series_id, observation_date, realtime_start)`, the view `rd.series_point_r1` over all eleven columns; `0014` adds the unit `index_points`. `ot_w_series_fred` has INSERT on both tables; `ot_runner` SELECT on both; `ot_ro_pages` and `ot_runner` SELECT on the view.
- `internal/store/series/writer.go:74` `InsertSeriesPoints`: per row an `INSERT INTO obs.series_definition ... ON CONFLICT DO NOTHING` (unit and currency of the point) and `insertPointSQL ... ON CONFLICT DO NOTHING`. `reader.go:22` `SeriesPoints` reads the view ordered by date and `realtime_start`, so a revised value is a second row and every consumer sees both (`internal/compute/own/series.go:20` `FromSeriesPoints`). Interfaces `store.SeriesWriter`, `SeriesReader`, `SeriesStore` are in `internal/store/api.go:21-27,178`; that file and `api_test.go` stand at 800 lines and may not grow (plan section 4).
- `internal/jobs/fredseries/job.toml`: 12 series (`DGS10 DGS3MO SP500 VIXCLS VXNCLS RVXCLS VXVCLS WALCL WDTGAL RRPONTSYD DFF BAMLH0A0HYM2`), five of them windowed with `lookback_days = 14`; `run.go:70` fetches plain series for the day only and windowed ones from day minus 14, then one `InsertSeriesPoints`. `internal/sources/fred/mapping.go:55-70` maps each series id to unit and currency; the five windowed ones have `pinVintage` (realtime fields and `source_revision` become the observation date: "first value seen kept", `ease-v2.md` Decision 3).
- `cmd/ot/jobconsole_jobs.go:79` `anyDay("fred_series", "revision_rows", textRevisionRows)`: any day, no Force. `compute_figures` also uses `revision_rows`, so the code and the text stay.
- Tests that build the schema from named files: `internal/store/series/series_pg_test.go:108` applies `0002_series.sql` and `0003_ops_ledger.sql` only (tag `pg`).
- The plan names "four free FRED credit and funding series" without ids; the ids below are the designer's choice, not verified against FRED (Doubt 1).

## Decisions

1. Registry (plan P7). `obs.series_definition` keeps `series_id`, `unit`, `currency` and gains, all with defaults so the old insert still works: `title text default ''`, `category text default ''` (check: empty or one of `rates, credit, liquidity, volatility, fx, activity`), `frequency text default ''` (empty or `daily, weekly, monthly, quarterly`), `source text default 'fred'`, `source_series_id text default ''` (empty means the same as `series_id`), `secondary_source text default ''` (empty or `nyfed`), `seasonal_adjustment text default ''`, `first_observation date null`, `active boolean default true`. The migration fills the 12 series and inserts the four new ones (Decision 6); rows made later by the writer's `ON CONFLICT DO NOTHING` insert carry the defaults (an unclassified row is visible, not an error).
2. Points (plan P7, owner S2). Key `(series_id, observation_date)`. The migration first deletes every row that is not the newest `realtime_start` of its `(series_id, observation_date)`, then drops the old key and adds the new one. Added columns: `updated_at timestamptz` (set from `published_at` for stored rows, `now()` default) and `written_by text default ''`. `realtime_start`, `realtime_end`, `source_revision` stay as plain columns ("the value stands since"). `status` stays free text; `void` is added to the values the code knows. FRED revisions are not kept: this ends "first value seen kept" for the five pinned series (`ease-v2.md` Decision 3): a revised WALCL value now replaces the stored one. Decided by the owner (S1 and S2, 2026-11-13: overwrite by key for all dated data), not a question. `pinVintage` is left as it is (no sources change for it).
3. View. `rd.series_point_r1` is changed with `CREATE OR REPLACE VIEW`, same eleven columns and types, `WHERE status <> 'void'` added: the view keeps its owner (`ot_owner`) and its `SELECT` grants to `ot_ro_pages` and `ot_runner` (`0002_series.sql:48,58`), which a drop would lose. No reader, page, record or `contracts.SeriesPoint` changes; `internal/store/series/reader.go` is not edited.
4. Roles. `ot_w_series_fred` keeps INSERT on both tables and gets `UPDATE` on the columns `value, status, unit, currency, realtime_start, realtime_end, published_at, source, source_revision, updated_at, written_by` of `obs.series_point` and `SELECT` on `series_id, observation_date, value, status, unit, currency, realtime_start, realtime_end, published_at, source, source_revision` (arbiter and `DO UPDATE ... WHERE` columns), and `SELECT` on `obs.series_definition` (the job reads the registry). No DELETE, no TRUNCATE, no UPDATE of the key.
5. The write (as FX Decision 4). `INSERT ... ON CONFLICT (series_id, observation_date) DO UPDATE SET value, status, unit, currency, realtime_start, realtime_end, published_at, source, source_revision, updated_at = now(), written_by WHERE (stored value, status, unit, currency, source) IS DISTINCT FROM (new ones) RETURNING series_id`, one statement per row, and a row whose status is `no_value` never replaces a stored `present` one (a FRED "." does not erase a value, whoever wrote it), one transaction per call; a key twice in a call is applied in order; the sentinel screen of `screen` (`-1`, NaN, infinities stored as NULL with `sentinel_rejected`) still runs first. The definition insert stays `ON CONFLICT DO NOTHING`. Two counts: written and unchanged. An empty list makes no call. `InsertSeriesPoints` stays, now insert-only on the new key. The backup source (P4) writes with `FillSeriesPoints`: the same upsert statement with `DO UPDATE ... WHERE stored status IN ('no_value', 'void')`, so it fills a missing date or one FRED stored as `no_value` and never replaces a `present` value.
6. New series (plan P7: four free FRED credit and funding series). `BAMLC0A0CM` (ICE BofA US corporate OAS, credit, daily, percent), `BAA10Y` (Moody's Baa yield minus the 10-year, credit, daily, percent), `SOFR` (secured overnight financing rate, liquidity, daily, percent, `secondary_source = 'nyfed'`), `WRESBAL` (reserve balances at Federal Reserve Banks, liquidity, weekly, `ccy_amount` USD, millions so `shift` 6). They join `job.toml` `series` and `windowed`; their rows in `internal/sources/fred/mapping.go` are written like `BAMLH0A0HYM2` (with `pinVintage`). The legs are stored; a spread is computed by a reader (plan P7). No page reads them in this step: liquidity is background only (owner S4).
7. The job (as FX Decision 6). `fredseries.New` is unchanged and does not fail. `Run` asserts the writer to `seriesdef.SeriesUpserter`; a writer without it makes `Run` return a Retryable error naming the missing upsert (never a silent insert-only fallback). The points of the run are upserted in one call with `writtenBy = "fred_series:" + day`. The plan hash and the keys are unchanged.
8. Console. `fred_series` becomes `{ID: "fred_series", Kind: kindAnyDay, Force: true}`: the page offers a day and the Force box; a forced day pulls that day (windowed series: that day and the 14 before) and Decision 5 decides what changes. The reason code `revision_rows` stays for `compute_figures`.
9. Backup source (plan P4, P7: NY Fed second source, background only). `internal/sources/nyfed` is a `sources.Source` for `SOFR` only (the NY Fed markets API, daily rate by date), no key. `fredseries` gets `NewWithSecondary(src, second, w, clk)` (additive; `New` stays). After the primary fetch of a run, for each series of the run whose registry row has `secondary_source = 'nyfed'` and the job asks the second source for the same plan and writes with `FillSeriesPoints` (Decision 5: it fills a missing or `no_value` date and never replaces a `present` value; a later FRED run replaces it by Decision 5). The job reads the registry through the writer by asserting `seriesdef.SeriesRegistry`; a writer without it means no fallback. A second-source error is logged in the result, never fails the primary result. `ot` builds `NewWithSecondary` in `cmd/ot/wire_fred.go` only; `wire.go` (795 lines) is not edited.
10. Deploy (the migration holds the auto-deploy). Entry 2 carries migration `0043_series_registry.sql` (after `0041_price_bar_repair.sql` of `data-repair-bars.md` and `0042` of `strategy-framework`). The window runs after its merge: rebuild `ot`, `ot setup pg`, migrate, `ot setup pg` again. Entries 4 to 8 merge after the migration. Between the migration and entry 4 the old binary's insert works on the new key and stores a new date; a revised value is not stored until entry 4. If entry 4 lands first, `UpsertSeriesPoints` fails Retryable (no such key or column) and no data changes. The migration deletes superseded vintages; the Down part restores the old key (rows stay unique) and drops the new columns, and does not restore deleted vintages.
11. No old value is kept (owner S1). A replaced value is gone; a wrong forced overwrite is repaired only by another run from a source that still has the right value.

## Modules

`docs/design/modules.md`, row `store`: the entry cell gains `internal/store/seriesdef` (declarations only, like `internal/store/bar`). Entries touch store (`internal/store/**`, `migrations/**`, `docs/contracts/store.md`; entries 1, 2), sources (`internal/sources/**`, `docs/contracts/sources.md`; entries 3, 6), jobs (`internal/jobs/**`, `docs/contracts/jobs/**`; entries 4, 7) and ot (`cmd/**`; entries 5, 8). New import edges, all inside the rule of `modules.md` line 51: `internal/jobs/fredseries` imports `internal/store/seriesdef`; `internal/store/series` imports it; `cmd/ot` imports `internal/sources/nyfed`.

## Interface

One interface change, entry 1, the new declarations-only package `internal/store/seriesdef` (`api.go`, `api_test.go`; additive, nothing in `internal/store/api.go` changes):

    // SeriesDefinition is one registry row of obs.series_definition. Empty strings mean "not set".
    type SeriesDefinition struct {
        SeriesID, Title, Category, Frequency, Source, SourceSeriesID, SecondarySource, SeasonalAdjustment string
        Unit, Currency   string
        FirstObservation string // a day, YYYY-MM-DD, or empty
        Active           bool
    }

    // WriteCount is what one UpsertSeriesPoints call did: Written rows were inserted or replaced, Unchanged rows
    // were equal to the stored one.
    type WriteCount struct{ Written, Unchanged int }

    // SeriesUpserter writes points by the key (SeriesID, ObservationDate): a differing row replaces the stored one,
    // an equal row changes nothing, no old value is kept. writtenBy names the run (job and day). It is an optional
    // method of the value OpenSeriesFunc returns; the job asserts it at Run.
    type SeriesUpserter interface {
        UpsertSeriesPoints(ctx context.Context, rows []contracts.SeriesPoint, writtenBy string) (WriteCount, error)
    }

    // SeriesFiller writes backup points: a key that is missing, or stored no_value or void, is written; a present
    // value is never replaced. Optional method of the same value.
    type SeriesFiller interface {
        FillSeriesPoints(ctx context.Context, rows []contracts.SeriesPoint, writtenBy string) (WriteCount, error)
    }

    // SeriesRegistry reads the registry. It is an optional method of the same value.
    type SeriesRegistry interface {
        ActiveSeries(ctx context.Context) ([]SeriesDefinition, error)
    }

`store.SeriesWriter`, `SeriesReader`, `SeriesStore`, `OpenSeriesFunc` and `contracts.SeriesPoint` are unchanged: no fake breaks.

## Issues

Build order: every entry compiles and its tests pass against main. BASE is `git merge-base origin/main HEAD`. Entry 2 holds the auto-deploy (migration); entries 4 to 8 merge after the window ran it (Decision 10). `Contracts: unchanged` (no file under `contracts/schemas`, `contracts/datasets` or `contracts/proto`). Test runs are in `## Checked by CI`.

### 1 series-registry-api (interface)

Module: store. Size: S.
Order: first.
Files: `internal/store/seriesdef/api.go`, `internal/store/seriesdef/api_test.go`, `docs/design/modules.md`
Design: docs/design/series-registry.md#Interface
Acceptance:
1. `api.go` declares `SeriesDefinition`, `WriteCount`, `SeriesUpserter`, `SeriesFiller` and `SeriesRegistry` as in the Interface section, with docs and no function body, and is the only non-test `.go` file of the package.
2. `git diff BASE -- docs/design/modules.md` shows one changed line, the `store` row, with `internal/store/seriesdef` added to its entry cell and nothing else changed; `git diff --name-only BASE -- internal/store/api.go internal/store/api_test.go` prints nothing.
3. `api_test.go` has a new contract test `TestSeriesUpserterContract` (`t.Parallel()`): a fake counts a written and an unchanged row and the caller reads both back, an empty list returns zero counts; a second new test `TestSeriesRegistryContract` reads definitions through a fake.
4. `git diff --name-only BASE` lists only paths of the Files line.

### 2 series-registry-store

Module: store. Size: L.
Order: after 1; after data-repair-bars entry 2 (it adds `0041_price_bar_repair.sql` and edits `internal/store/setup/setup.go` and `docs/contracts/store.md`) and after strategy-framework entry 5 (result-store, migration `0042`); this entry takes `0043` and rebases on both. Holds the auto-deploy: it carries a migration (Decision 10).
Files: `migrations/0043_series_registry.sql`, `internal/store/series/upsert.go`, `internal/store/series/registry.go`, `internal/store/series/writer.go`, `internal/store/series/series_test.go`, `internal/store/series/upsert_test.go`, `internal/store/series/series_pg_test.go`, `internal/store/series/upsert_pg_test.go`, `internal/store/setup/setup.go`, `docs/contracts/store.md`
Design: docs/design/series-registry.md#Issues (Decisions 1 to 5, 10, 11)
Acceptance:
1. `0043_series_registry.sql` has an Up and a Down part, is owned by `ot_owner`, and in Up, in this order: adds the nine registry columns of Decision 1 with their defaults and checks; updates the 12 existing series and inserts the 4 of Decision 6 with the values stated there (`git grep -c "BAMLC0A0CM\|BAA10Y\|WRESBAL\|SOFR" migrations/0043_series_registry.sql` is at least 4); deletes the rows of `obs.series_point` that are not the newest `realtime_start` of their `(series_id, observation_date)`; drops the primary key and adds `(series_id, observation_date)`; adds `updated_at` (set from `published_at`) and `written_by`; changes the view with `CREATE OR REPLACE VIEW rd.series_point_r1` (the eleven columns of `0002_series.sql`, `WHERE status <> 'void'`; no `DROP VIEW`, no new owner or grant line for the view: `git grep -n "DROP VIEW" migrations/0043_series_registry.sql` finds only the Down part); grants for the writer role as in Decision 4.
2. `upsert.go` holds the statement of Decision 5 with `ON CONFLICT (series_id, observation_date) DO UPDATE ... WHERE ... IS DISTINCT FROM ... RETURNING series_id` and the method `UpsertSeriesPoints` on the store value; it runs `screen` first and the definition insert as `writer.go` does; an empty list makes no call. `upsert.go` also holds `FillSeriesPoints` (Decision 5); `registry.go` holds `ActiveSeries` reading `obs.series_definition WHERE active`. `git grep -n "InsertSeriesPoints" internal/store/series/writer.go` still finds the method and its SQL ends `ON CONFLICT DO NOTHING`; `git diff BASE -- internal/store/series/reader.go` is empty; `git diff --name-only BASE -- internal/store/api.go` prints nothing.
3. `series_test.go` (unit, fake database) gets a new test that the upsert sends one statement per row in one transaction with the writtenBy argument, a second new test for the sentinel (`-1` stored as NULL with `sentinel_rejected`) and a third for the empty list.
4. `series_pg_test.go` applies `0043_series_registry.sql` after `0002` and `0014` (its list of migration files names them; the registry seed uses the unit `index_points`); the new file `upsert_pg_test.go` (tag `pg`, new tests) shows (after inserting a series row, since a fresh test database has none of the 12), as the role `ot_w_series_fred` and with the column grants alone: a differing value replaces (counts 1 written), an equal value changes nothing (1 unchanged), the old `InsertSeriesPoints` SQL adds no row for an existing date; the migration keeps only the newest vintage of a date; the view hides a `void` row and shows `no_value`; a `no_value` row does not replace a `present` one; fill replaces `no_value` and not `present`; the role cannot DELETE or UPDATE `series_id` (named in the test as denied); a new test reads `rd.series_point_r1` after `0043` as `ot_ro_pages` and as `ot_runner` and gets rows.
5. `setup.go` changes only the comments of the role `ot_w_series_fred` if any (`git diff BASE -- internal/store/setup/setup.go` shows comment lines only), and `docs/contracts/store.md` states the new key, the registry columns, the grants and `0043` (`git grep -n "0043" docs/contracts/store.md` finds it).
6. `git diff --name-only BASE` lists only paths of the Files line; every new source file is under 600 lines.

### 3 series-registry-fred

Module: sources. Size: S.
Order: after 1 (parallel with 2, 4, 5, 6).
Files: `internal/sources/fred/mapping.go`, `internal/sources/fred/mapping_test.go`, `internal/sources/fred/testdata/series_units.json`, `docs/contracts/sources.md`
Design: docs/design/series-registry.md#Issues (Decision 6)
Acceptance:
1. `mapping.go` has rows for `BAMLC0A0CM` and `BAA10Y` (unit `percent`), `SOFR` (unit `percent`) and `WRESBAL` (unit `ccyAmount`, currency `USD`, `shift: 6`), each with `pinVintage: true`; no other row changes (`git diff BASE -- internal/sources/fred/mapping.go` shows four added lines in the table and nothing else).
2. `mapping_test.go` has a new test per added row that maps a recorded observation of the series to the unit, currency, scaled value and pinned realtime fields (inline in the test).
3. `internal/sources/fred/testdata/series_units.json` gets four entries, because `compareUnits` of `mapping_test.go` (`TestSeriesUnitsMatchMetadata`, `TestUnitsTextMatchesShift`) fails for a mapped id with no entry: `BAMLC0A0CM` Percent, `BAA10Y` Percent, `SOFR` Percent, `WRESBAL` Millions of U.S. Dollars, labelled in the file's own note as read on the FRED pages on 2026-11-13, not downloaded metadata.
4. `docs/contracts/sources.md` lists the four ids with their units.
5. `git diff --name-only BASE` lists only paths of the Files line.

### 4 series-registry-job

Module: jobs. Size: M.
Order: after 1, 2 and 3 (the seed test reads the migration; a manifest id without a mapping row fails the whole run); merge after the window ran the migration of entry 2.
Files: `internal/jobs/fredseries/run.go`, `internal/jobs/fredseries/run_test.go`, `internal/jobs/fredseries/job.toml`, `internal/jobs/fredseries/plan_test.go`, `internal/jobs/fredseries/seed_test.go`, `docs/contracts/jobs/fred_series.md`
Design: docs/design/series-registry.md#Issues (Decisions 6, 7)
`plan_test.go` and `run_test.go` use the series list and the plan hash that `job.toml` changes.
Acceptance:
1. `run.go` asserts the writer to `seriesdef.SeriesUpserter` at `Run`, returns a Retryable error naming the missing upsert when it is absent, and otherwise upserts all points of the run in one call with `writtenBy = "fred_series:" + <day>`; `git grep -n "InsertSeriesPoints" internal/jobs/fredseries/run.go` finds nothing.
2. `job.toml` `series` and `windowed` gain `BAMLC0A0CM`, `BAA10Y`, `SOFR`, `WRESBAL`; `lookback_days` and the schedule are unchanged; `plan_test.go` and `run_test.go` are updated to the 16 series and the new plan hash.
3. `run_test.go` has new tests: a writer without the upsert method makes `Run` return a Retryable error; a fake upserter receives one call with the written-by text; a run with an empty fetch makes no call.
4. `seed_test.go` has a new test that reads `../../../migrations/0043_series_registry.sql` and asserts that every id of `job.toml` `series` appears in it (the registry seed covers the manifest).
5. `docs/contracts/jobs/fred_series.md` states the upsert and the 16 series.
6. `git diff --name-only BASE` lists only paths of the Files line.

### 5 series-registry-console

Module: ot. Size: S.
Order: after 1; after data-repair-bars entry 4 (same table, same pinning tests; it lands after #2208).
Files: `cmd/ot/jobconsole_jobs.go`, `cmd/ot/jobconsole_jobs_test.go`, `cmd/ot/jobconsole_post_test.go`, `cmd/ot/jobconsole_site_test.go`
Design: docs/design/series-registry.md#Issues (Decision 8)
Acceptance:
1. `jobTable` has `{ID: "fred_series", Kind: kindAnyDay, Force: true}` in place of `anyDay("fred_series", "revision_rows", textRevisionRows)`; `revision_rows` and `textRevisionRows` stay (`compute_figures` uses them).
2. The numbers after `data-repair-bars.md` entry 4 (31 rows: 7 any-day, 16 today-only, 8 listed-only; force-yes `calendar_refresh`, `fx_refresh`, `bars_refresh`; 20 force-no) become: 31 rows, 7 any-day (`fred_series` is already in the set, so it does not change), 16 today-only, 8 listed-only; force-yes four (those three and `fred_series`); force-no 19. `jobconsole_jobs_test.go` pins them: the force-yes test (`TestForceYesIsCalendarFxAndBars` of bars entry 4) lists the four ids and is renamed to say so, `TestEveryForceNoJobHasReason` expects 19; `TestAnyDayIsExactlyTheSeven`, `TestTableIsTheWiredJobSet` (31) and `TestListedOnlyIsExactlyTheEight` are unchanged.
3. `jobconsole_post_test.go` `TestPostForceOnForceNoJob422` no longer posts Force on `fred_series` (it is accepted now) and uses another force-no job, `account_capture`; `jobconsole_site_test.go` expects the `fred_series` row with `Force` true and no `ForceWhy`.
4. `TestFxRefreshRunValidatesPastDayWithForce` ends with a refusal of Force for `fred_series` (bars entry 4 put it there as the force-no example); this entry changes that line to another force-no job (`compute_figures`) and the test keeps its other lines.
5. `git diff --name-only BASE` lists only paths of the Files line.

### 6 series-registry-nyfed

Module: sources. Size: M.
Order: after 1 (parallel with 2 to 5).
Files: `internal/sources/nyfed/client.go`, `internal/sources/nyfed/parse.go`, `internal/sources/nyfed/client_test.go`, `internal/sources/nyfed/testdata/sofr.json`, `docs/contracts/sources.md`
Design: docs/design/series-registry.md#Issues (Decision 9)
Acceptance:
1. `client.go` has `New(Config) (sources.Source, error)`; `Config` takes the base address and an HTTP client (as `internal/sources/fred` does, so tests use a local test server); no key, no secret, no address in the repository except the public base address as a constant default; every error is a `contracts.TypedError` (UNAVAILABLE, timeouts Retryable; a body that does not parse, Permanent).
2. `Fetch` answers only the series id `SOFR`; any other id is a Permanent error naming it. It maps each daily rate to a `contracts.SeriesPoint` with `Source: "nyfed"`, unit `percent`, `Value` the decimal parsed, `RealtimeStart` and `RealtimeEnd` the observation date, within `From` and `To`.
3. `client_test.go` has new tests on `testdata/sofr.json` (hand-written from the reading of the API's answer, not a recorded response, until one is recorded; it holds a value, a missing day and malformed records): the mapping, an unknown id, a malformed body, and a 503 as Retryable; they use a local test server and no real address.
4. `docs/contracts/sources.md` states the source, its one series and that it is a backup that fills gaps only.
5. A record whose date or rate is missing, empty or not a number makes the whole answer a Permanent error and `Fetch` returns no point (nothing is written); a new test per case.
6. No file outside the Files line changes, and the package imports no other source package. The host `markets.newyorkfed.org` is a new outside service: it needs the owner's word at merge (this design stays `Owner approved: (pending)` until then; listed as the owner question).

### 7 series-registry-fallback

Module: jobs. Size: M.
Order: after 4 and 6 (same files as 4; needs the source); after 2 on deploy.
Files: `internal/jobs/fredseries/run.go`, `internal/jobs/fredseries/secondary.go`, `internal/jobs/fredseries/secondary_test.go`, `internal/jobs/fredseries/run_test.go`, `docs/contracts/jobs/fred_series.md`
Design: docs/design/series-registry.md#Issues (Decision 9)
Acceptance:
1. `secondary.go` has `NewWithSecondary(src, second sources.Source, w store.SeriesWriter, clk clock.Clock) (contracts.Job, error)`; `New` is unchanged and equals `NewWithSecondary` with no second source (no fallback).
2. After the primary fetch, for each series of the run whose registry row (read by asserting the writer to `seriesdef.SeriesRegistry`) has `secondary_source = "nyfed"` the job fetches the same plan from the second source and writes the points with `FillSeriesPoints` (never the upsert).
3. A second-source error never changes the primary result: the result's `Err` is unchanged and the failure is reported in a logged text, tested.
4. `secondary_test.go` has new tests: the fallback runs for a registry row naming `nyfed` when the primary has no value and does not run when it has one; a series with an empty `secondary_source` never asks the second source; the fallback uses the fill method (the fake upserter is never called with second-source points); a writer without `SeriesRegistry` means no fallback.
5. `git diff --name-only BASE` lists only paths of the Files line; every new file is under 600 lines.

### 8 series-registry-wire

Module: ot. Size: S.
Order: after 5 and 7.
Files: `cmd/ot/wire_fred.go`, `cmd/ot/wire_fred_test.go`
Design: docs/design/series-registry.md#Issues (Decision 9)
Acceptance:
1. `wire_fred.go` builds the job with `fredseries.NewWithSecondary` and the `nyfed` source built by `nyfed.New`; the start-off rule for a missing FRED key is unchanged.
2. `wire_fred_test.go` has a new test that the job is built with a second source and that `wire.go` is not changed (`git diff --name-only BASE -- cmd/ot/wire.go` prints nothing).
3. `git diff --name-only BASE` lists only paths of the Files line.

## Checked by CI

`go test ./internal/store/... ./internal/sources/... ./internal/jobs/fredseries/... ./cmd/ot/... -race -count=1` (the `pg` tests of entry 2 with `0043` applied to a fresh database run only in GitHub CI), `go run ./scripts/arch --check`, `go build ./...`, `golangci-lint`, and the contracts generation check (no change expected). Results only a run shows: the upsert counts, as the role `ot_w_series_fred` with the column grants alone (entry 2, `upsert_pg_test.go`); the migration keeps the newest vintage and the old rows otherwise unchanged; the old insert SQL works on the migrated table.

## Left out

- A job that plans from the registry. Today `job.toml` lists the series the job pulls and the registry describes them; adding a series is a registry row (migration seed) and a `job.toml` line, with `seed_test.go` guarding that the two agree. Making the registry the only list changes `Plan` and the plan hash of every day; a later step.
- Point-in-time vintages (`obs.series_vintage`, plan P7): not built, nothing blocks it.
- Computed spreads and any page or reading over the four new series (liquidity is background, owner S4); a registry view for pages; `rd.series_definition`.
- NY Fed series other than `SOFR`, OpenBB, bought feeds; the `source` rank rules of step 9.
- Dropping `realtime_end` and `source_revision`, or moving `value` to `numeric`: contract changes of `obs.series_point.r1.json`; the dataset description text there stays.
- `serieshistory` (the job that loads history): it keeps `InsertSeriesPoints`, which is insert-only on the new key; it is not repaired by a forced run here.

## Doubts

1. Checked on the FRED pages on 2026-11-13: `BAMLC0A0CM` (ICE BofA US Corporate OAS, percent, daily, NSA; since April 2026 only three years of observations, like `BAMLH0A0HYM2`), `BAA10Y` (Moody's Baa yield relative to the 10-year, percent, daily, NSA), `SOFR` (percent, daily, NSA), `WRESBAL` (reserve balances, week average, millions of USD, weekly ending Wednesday, NSA). First observation dates were not on the pages, so `first_observation` is NULL in the seed. The NY Fed path `https://markets.newyorkfed.org/api/rates/secured/sofr/search.json?startDate=..&endDate=..` answered with records `effectiveDate` and `percentRate` (plus percentile fields); read through a summarizer, not as raw JSON, so the exact field spelling stays a doubt until a first live read-only check; the test data of entry 6 is hand-written.
2. Decided by the owner, recorded for the reader: overwrite by key (S1, S2) ends "first value seen kept" for `WALCL`, `WDTGAL`, `RRPONTSYD`, `DFF`, `BAMLH0A0HYM2` (`ease-v2.md` Decision 3); those series now take a published revision.
3. The migration number 0043 assumes `0041` of `data-repair-bars.md` and `0042` of `strategy-framework` land first; another migration that lands first moves the number and the test list of entry 2.
4. The grants follow the PostgreSQL rules for `ON CONFLICT ... DO UPDATE` as in the FX entry; no database was available here, the `pg` tests are the check.
5. `ot_w_series_fred` reads the registry through the same store value as the points (one DSN, `OT_PG_DSN_SERIES_FRED`); no new role or DSN.
6. A value replaced by a forced run changes what a later compute_figures run reads; that job is force-no and stored readings are not recomputed by this design.

## Adversarial

Full answers are kept with the dispatcher (not in the repository).
1. Gap (repair is a day, not a series; backup cannot fill a FRED no_value row): second adopted (FillSeriesPoints, trigger dropped); first stays in Left out (day only, as FX and bars).
2. Risk (backup erased by the next FRED run; trigger rarely fires): adopted (no_value never replaces present; fill runs whenever the registry names the backup). Whether compute_figures recomputes after a replaced value is not decided here (it is force-no today; named in Doubt 6).
3. Simpler (ship entries 1 to 5, backup later): not adopted, the plan names NY Fed as second source; entries 6 to 8 are last and can be dropped without touching 1 to 5.
4. Lock-in (vintages deleted, first-value-kept ended): already stated in Decisions 2, 10, 11; the unused void status stays, the plan lists it.
5. False claim (backup never replaces FRED): fixed as in 1. Entry counts depend on data-repair-bars landing: stated in each Order line, they are the numbers after it.
6. Slip (entry 4 before 3 or 2): adopted, Order of entry 4 is now after 1, 2 and 3; the CI test database seeds a series row; setup.go/store.md conflicts are in the Order line of entry 2.
