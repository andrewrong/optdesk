# Compute inputs: the FRED market series the figures read

Author: the optdesk designer
Owner approved: (pending)
Revision: 3

Source: the owner's decisions of 2026-11-07 and 2026-11-08 (order of work compute, then risk and alerts, then strategy), the scheduler design `docs/design/scheduler.md` rev 8 (pull request 335 of andrewrong/optdesk, head b348fd7, not on main yet), `docs/research/go-quant-ecosystem.md` (on main, #343), the restart plan v2.5 rev 2.1 text (cited P:<line>), the compute design adversarial review, rev 1 of the four files (kept by the dispatcher, not committed; REV1:<finding>) and the review of rev 2 of the first draft (ADV:<line>).
Read on main at af7e583: `AGENTS.md`, `docs/design/modules.md`, `docs/GLOSSARY.md`, `contracts/datasets/obs.series_point.r1.json`, `contracts/fixtures/MANIFEST.synthetic-v1.json`, `internal/contracts/schema_test.go`, `internal/contracts/page_schema_test.go`, `scripts/hygiene/main.go`, `migrations/0002_series.sql`, `internal/sources/fred/mapping.go`, `internal/jobs/fredseries/job.toml`, `internal/jobs/fredseries/run.go`, `cmd/ot/backfill.go`, `docs/contracts/ot.md`.
One of four designs (`compute-core.md`, this file, `compute-figures.md`, `compute-calendar.md`). Order of the four: core, figures, FRED, calendar. This file lands after `compute-figures.md`: `series-unit-index-points` comes after figures' `reading-dataset` (both edit `contracts/generate.sh`, `records.go`, `schema_test.go` and the manifest), `store-series-unit-index-points` (migration 0014) after `store-reading-tables` (0013), `series-history-loader` after `jobs-compute-api` (same `api.go`), `series-load-wire` after `compute-jobs-wire` (same `wire.go`). `compute-calendar.md` lands after this file. Like figures, the chain waits for the scheduler design to be merged: `sched-store-ops` and `sched-jobs-schedule` are named below.

Goal: store the series the figures read, in the unit the glossary gives them, and the history the figures need before a first run is useful. Today `internal/jobs/fredseries/job.toml` fetches `DGS10` only and nothing loads history, so every figure of `compute-figures.md` would be missing.

## What main holds today

- `obs.series_point` r1 has the unit enum `percent, ratio, ccy_amount, count` (`contracts/datasets/obs.series_point.r1.json`, `migrations/0002_series.sql:7`). The glossary stores VIX, VXN, RVX and SP500 in index points (GLOSSARY.md:75), which none of the four expresses.
- `seriesUnits` in `internal/sources/fred/mapping.go` holds `DGS10` and `DGS2`; a series absent there is refused; each row is checked against recorded FRED metadata in `internal/sources/fred/testdata/series_units.json`, and the check compares only the word `Percent`.
- The job fetches one observation date per run (`From: day, To: day`, `internal/jobs/fredseries/run.go:74`). `ot backfill --day` re-runs the keys of a day that has a stored plan and refuses a day with none (`docs/contracts/ot.md`), so it cannot load history before the first collection day.
- `contracts/fixtures/MANIFEST.synthetic-v1.json` lists every fixture file with its SHA-256: `TestFixtureManifest` (`internal/contracts/page_schema_test.go:107`) checks each hash and `scripts/hygiene` fails a fixture not listed. A change to a fixture file is a change to the manifest (the first draft's "manifest unchanged" was wrong, ADV:84).
- `TestSeriesPointFixtureValues` (`internal/contracts/schema_test.go:165-176`) asserts exactly three records and, in its loop, `series_id == "DGS10"` and `unit == "percent"` for every record: a fourth record of `VIXCLS` fails it unless the loop is rewritten (REV1:R5).

## Decisions

1. A new unit, `index_points`, is added to `series_unit` (additive to r1) rather than storing index levels as `ratio`: a ratio is a quotient and the glossary says the Cboe indices are "not a fraction, not a percent".
2. The series stored: `SP500`, `VIXCLS`, `VXNCLS`, `RVXCLS` (`index_points`), `VXVCLS` (`index_points`; the FRED id of the 3-month volatility index, VIX3M) and `DGS3MO` (`percent`, the rate of own IV30, GLOSSARY.md:76). The implementer confirms with one fetch that FRED carries each id and records its `units` text in `series_units.json`; an id FRED does not carry is left out and the pull request says so (then `market.vix_term_ratio` stays missing). Not verified by this design: that `VXVCLS` is still published.
3. History comes from a one-off command, not from the job: `ot series-load --series ID --from DAY --to DAY` fetches the range once and inserts it through `store.SeriesWriter`, which is idempotent. The loop over series, the refusal of a range over 5 years and the per-series writes are logic, so they live in `internal/jobs/serieshistory` behind `jobs.SeriesHistoryLoader` (own glue sits behind an interface, owner's rule; a library or another loader can replace it); `ot series-load` only wires it (row ot: "no logic beyond wiring"). The interface and its one implementation are one issue: both are in the jobs module, and nothing uses the interface between them (REV1:R3). The window `--from`/`--to` is given by the owner when running the command; no default is in code. The 3-year rank needs 3 years, the 200-day mean 200 sessions and HV30 31 closes; the daily job adds one day at a time. The owner runs the command (the owner's standing word covers `run-once`, `calendar load` and `page`; this command needs the owner's word before the window runs it). Until it has run, every 3-year percentile, the 200-day gap and HV30 are missing; `compute-figures.md` tests that first-run state (`TestFirstRunOnEmptyStores`).
4. Migration numbers: `0012_scheduler.sql` is owned by `sched-store-ops` of the scheduler design (pull request 335); `0013_readings.sql` is owned by `store-reading-tables` of `compute-figures.md`; `0014_series_unit_index_points.sql` is owned by `store-series-unit-index-points` here. The three are on main in that order with no gap, so each later issue depends on the earlier one by name; the acceptance checks that numbers are unique and gapless and fails (the worker renumbers) if another migration took 0014 first.

## Modules

Edit of `docs/design/modules.md` in this design's pull request: the list "one constructor each" gains `internal/jobs/serieshistory` (a hand merge with the same line edited by the other designs). `cmd/ot` stays the composition root: `series-load-wire` uses `newFRED`, `openSeries` and a new `newSeriesHistory` of `deps`.

| issue | module | order |
|---|---|---|
| series-unit-index-points (interface) | contracts | after `reading-dataset` (compute-figures.md; same generated files) |
| store-series-unit-index-points | store | after series-unit-index-points, `store-reading-tables` (0013), `sched-store-ops` (0012) |
| fred-market-series | sources | after series-unit-index-points |
| fred-market-manifest | jobs | after fred-market-series, store-series-unit-index-points and `sched-jobs-schedule` (same file) |
| series-history-loader (interface and implementation) | jobs, plus repo (judged: arch files) | after fred-market-series, `jobs-compute-api` (compute-figures.md; same files) |
| series-load-wire | ot | after series-history-loader, `compute-jobs-wire` (compute-figures.md; same files) |

## Interface

Owner: contracts (the unit), store (the migration), sources (the unit table).

- `obs.series_point` r1: `series_unit` gains `index_points`; the generated `contracts.SeriesUnit` gains its constant; no other field changes.
- jobs: `internal/jobs/api.go` gains `SeriesHistoryLoader` (`Load(ctx, sources.FetchPlan) ([]SeriesLoaded, error)`), `SeriesLoaded{SeriesID string; Fetched int}` and `NewSeriesHistoryFunc func(sources.Source, store.SeriesWriter) (SeriesHistoryLoader, error)`; the loader returns `contracts.TypedError` values only.
- `ot series-load --series ID --from DAY --to DAY` (wiring only): `--series` repeatable, all three required, real profile only, the same refusals as `ot backfill` for the demo profile; prints the count of points inserted per series and no value.

## Issues

### series-unit-index-points (interface)

- Module: contracts. Size: S. Depends on: reading-dataset (compute-figures.md).
- Order: after reading-dataset: both edit `generate.sh`, `records.go`, `schema_test.go` and the manifest. Interface change to a dataset r1, additive.
- Design: docs/design/compute-fred-series.md#Interface. Contracts: obs.series_point r1 (enum gains `index_points`; additive).
- Files: contracts/datasets/obs.series_point.r1.json, internal/contracts/gen/records.go, docs/contracts/datasets/obs.series_point.md, internal/contracts/schema_test.go, contracts/fixtures/obs.series_point.r1.json, contracts/fixtures/MANIFEST.synthetic-v1.json

- Goal: add `index_points` to `series_unit`, regenerate, add one fixture record with that unit (series `VIXCLS`, synthetic value 18.2), refresh the SHA-256 of the fixture in the manifest, and rewrite `TestSeriesPointFixtureValues`: the four records are checked against per-record tables `ids = [DGS10, DGS10, DGS10, VIXCLS]`, `units = [percent, percent, percent, index_points]`, dates, values and statuses, with no `"DGS10"` or `"percent"` literal left in the loop.

Acceptance:

1. `grep -c "index_points" contracts/datasets/obs.series_point.r1.json` is at least 1 and `sh contracts/generate.sh && git diff --exit-code internal/contracts/gen docs/contracts/datasets` is clean after the entry.
2. `go test ./internal/contracts -run 'TestDatasetFixtures/obs.series_point|TestSeriesPointFixtureValues|TestFixtureManifest' -race -count=1` passes with the new record; a record with unit `points` fails the dataset test; `sed -n '/^func TestSeriesPointFixtureValues/,/^}/p' internal/contracts/schema_test.go | grep -c 'r\["series_id"\] != "DGS10"\|r\["unit"\] != "percent"'` prints 0; the same function says `len(recs) != 4`.
3. `go run ./scripts/hygiene` prints `hygiene: ok` (the manifest lists the changed file) and `go test ./internal/contracts -run TestFixtureManifest -race -count=1` passes (the new hash is the recorded one).
4. `git diff $(git merge-base HEAD origin/main) -- contracts/datasets/obs.option_eod.r1.json contracts/datasets/obs.quote_snapshot.r1.json contracts/proto` is empty; the pull request body starts its contracts line `Contracts: obs.series_point r1 (enum gains index_points; additive)`.
5. `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the six paths of the Files line.

### store-series-unit-index-points

- Module: store. Size: S. Depends on: series-unit-index-points, store-reading-tables (`compute-figures.md`), sched-store-ops (scheduler design).
- Order: after those, so that 0012 and 0013 are on main first (Decision 4). Migration `0014_series_unit_index_points.sql`.
- Design: docs/design/compute-fred-series.md#Decisions
- Files: migrations/0014_series_unit_index_points.sql, internal/store/series/unit_test.go

- Goal: `ALTER TYPE obs.series_unit ADD VALUE IF NOT EXISTS 'index_points'` under `-- +goose NO TRANSACTION`, with a Down that says the value cannot be dropped. `unit_test.go` reads the migration by a path found from `go.mod` and checks the file text and that the Go writer maps the unit without error.

Acceptance:

1. `grep -c "ADD VALUE IF NOT EXISTS 'index_points'" migrations/0014_series_unit_index_points.sql` prints 1; the file has `-- +goose Up`, `-- +goose NO TRANSACTION` and `-- +goose Down`.
2. `go test ./internal/store/series -race -count=1` passes; `TestIndexPointsAccepted` and `TestMigration0014Text` pass.
3. `ls migrations | grep -c "^0014_series_unit_index_points.sql$"` prints 1 and `ls migrations | sed 's/_.*//' | sort | awk 'NR>1 && $1 != p+1 {bad=1} {p=$1+0} END {exit bad}'` exits 0 (unique and gapless, whatever else landed); `git diff $(git merge-base HEAD origin/main) -- migrations/0002_series.sql` is empty.
4. `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the two paths of the Files line.

Checked by CI: the pg migration run that applies 0014 to a database and `go vet -tags pg ./internal/store/series` (compile only; the offline checks are items 1 to 3, ADV:71).

### fred-market-series

- Module: sources. Size: S. Depends on: series-unit-index-points.
- Order: after series-unit-index-points. Needs the network once (the implementer records the FRED metadata).
- Design: docs/design/compute-fred-series.md#Decisions
- Files: internal/sources/fred/mapping.go, internal/sources/fred/mapping_test.go, internal/sources/fred/testdata/series_units.json

- Goal: `seriesUnits` gains `SP500`, `VIXCLS`, `VXNCLS`, `RVXCLS`, `VXVCLS` (`index_points`) and `DGS3MO` (`percent`); `series_units.json` (entries `{series_id, units}`, as on main) gains each id with the units text FRED gives, fetched once from FRED's series metadata, with the fetch date stated in the pull request; a hand-written file cannot be told from a fetched one offline, so the reviewer opens each series page of FRED once and no CI step proves it; `compareUnits` also checks `Index` against `index_points` (today it knows only `Percent`).

Acceptance:

1. `go test ./internal/sources/fred -race -count=1` passes; `TestSeriesUnitsMatchMetadata` covers every key of `seriesUnits`; `TestSeriesUnitsDetectsWrongClaim` also fails a table that says `index_points` for a `Percent` series.
2. `go test ./internal/sources/fred -run 'TestMapping|TestFixtureMapping' -race -count=1 -v` passes for `VIXCLS` (value kept as index points, `.` is `no_value`) and for `DGS3MO` (percent kept as received).
3. `grep -c '"SP500"\|"VIXCLS"\|"VXNCLS"\|"RVXCLS"\|"DGS3MO"' internal/sources/fred/mapping.go` prints 5.
4. `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the three paths of the Files line.

### fred-market-manifest

- Module: jobs. Size: S. Depends on: fred-market-series, store-series-unit-index-points, sched-jobs-schedule (scheduler design).
- Order: after those; `internal/jobs/fredseries/job.toml` is also edited by `sched-jobs-schedule`, so this entry merges after it.
- Design: docs/design/compute-fred-series.md#Decisions
- Files: internal/jobs/fredseries/job.toml, internal/jobs/fredseries/plan_test.go

- Goal: `series` becomes `["DGS10", "DGS3MO", "SP500", "VIXCLS", "VXNCLS", "RVXCLS", "VXVCLS"]` (without the last if `fred-market-series` left it out).

Acceptance:

1. `go test ./internal/jobs/fredseries -race -count=1` passes; `TestEmbeddedManifest` expects the seven series and `TestPlanHash` changes with the list.
2. `grep -c "^series = " internal/jobs/fredseries/job.toml` prints 1 and the line holds `SP500`, `VIXCLS` and `DGS3MO`.
3. `git diff $(git merge-base HEAD origin/main) -- internal/jobs/fredseries/job.toml | grep -c '^[-+]\[\(window\|schedule\)\]'` prints 0 (the window and schedule tables are untouched).
4. `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the two paths of the Files line.

### series-history-loader (interface and implementation)

- Module: jobs, plus repo (judged: the regenerated arch files). Size: M. Depends on: fred-market-series, jobs-compute-api (compute-figures.md).
- Order: after both (`api.go` and `jobs.md` are edited by `jobs-compute-api` too). Interface change: the three declarations of the Interface section, and the one implementation, in one pull request (REV1:R3). The owner runs the command that uses it; no CI step runs it.
- Design: docs/design/compute-fred-series.md#Interface
- Files: internal/jobs/api.go, internal/jobs/api_test.go, docs/contracts/jobs.md, internal/jobs/serieshistory/loader.go, internal/jobs/serieshistory/loader_test.go, arch.toml, .go-arch-lint.yml

- Goal: the declarations of the Interface section with their text in `jobs.md` (a range over 5 years is `Permanent`; a source error is passed on; the points are written once per series) and `api_test.go` from both sides with a fake loader; and `serieshistory.New` (`jobs.NewSeriesHistoryFunc`): `Load` refuses a plan with an empty series list, `From` after `To` or a range over 5 years (`Permanent`), makes one `Fetch` per series over `From`..`To` through the injected `sources.Source`, writes each series with one `InsertSeriesPoints` call through `store.SeriesWriter` (idempotent) and returns the count fetched per series (the writer returns an error only). The source error text is not passed on beyond its class. No clock and no environment are read.

Acceptance:

1. `go test ./internal/jobs/serieshistory -race -count=1` passes with fakes (`t.Parallel()`, no network, no database): `TestLoadTwoSeries` (writer called once per series), `TestLoadTwiceSamePoints` (a second call inserts the same points without error), `TestRangeOverFiveYearsRefused` (a 6-year range, nothing fetched), `TestFromAfterToRefused`, `TestEmptySeriesRefused`, `TestSourceErrorStopsBeforeWrite`.
2. `grep -c "time.Now\|time.Sleep\|os.Getenv" internal/jobs/serieshistory/*.go` prints 0; `go list -deps ./internal/jobs/serieshistory | grep -c "pgx\|grpcbroker"` prints 0.
3. `go run ./scripts/arch && git diff --exit-code arch.toml .go-arch-lint.yml` is clean; each Go file is under 600 lines.
4. `grep -c "SeriesHistoryLoader\|SeriesLoaded\|NewSeriesHistoryFunc" internal/jobs/api.go` is at least 3; `go test ./internal/jobs -race -count=1` passes.
5. `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the seven paths of the Files line.

### series-load-wire

- Module: ot. Size: S. Depends on: series-history-loader, compute-jobs-wire (compute-figures.md; both edit `wire.go`, `ot.md`).
- Order: after both. The owner runs the command with the window the owner chooses; no CI step runs it.
- Design: docs/design/compute-fred-series.md#Interface
- Files: cmd/ot/seriesload.go, cmd/ot/seriesload_test.go, cmd/ot/wire.go, cmd/ot/wire_test.go, docs/contracts/ot.md

- Goal: the subcommand of the Interface section as wiring only: parse the flags (`--from` and `--to` required, no default), refuse the demo profile as `backfill` does, read the FRED key as `ot run-once` reads it, build the source (`deps.newFRED`), the writer (`deps.openSeries`) and the loader (`deps.newSeriesHistory`, new in `wire.go`), call `Load` once and print the count per series; no loop over series, no range rule and no write call in `cmd/ot`. `docs/contracts/ot.md` lists the new one with its flags and states the count of subcommands that the registrations give (eight on main; a count written in words is replaced by the number the check below prints, so an earlier subcommand of the scheduler does not break it).

Acceptance:

1. `go test ./cmd/ot -run TestSeriesLoad -race -count=1` passes with a fake loader in `deps` (no network, no database, `t.Parallel()`, `t.TempDir()`): the plan handed to the loader holds the flags as given, the demo profile is refused, a missing flag is an error, the count lines are printed, a loader error is shown by class only.
2. `grep -c "InsertSeriesPoints\|Fetch(" cmd/ot/seriesload.go` prints 0; `grep -c "series-load" docs/contracts/ot.md` is at least 2 and the subcommand count in `ot.md` equals `grep -h 'register("' cmd/ot/*.go | grep -v _test | wc -l`.
3. `grep -c "time.Sleep\|t.Setenv" cmd/ot/seriesload_test.go` prints 0.
4. `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the five paths of the Files line.

## Left out

- The first history load itself (which ranges, when): the owner's act after `series-load-wire` and `fred-market-manifest` have merged, with the window the owner chooses. Until it is done the 3-year ranks, the 200-day gap and HV30 are missing with `short_history`; that is the designed answer, not a fault.
- Rejected: loading history through the daily job (it would change the plan and the window rule of `fredseries`, which the scheduler design also edits); rejected: `ot backfill` for days before the first collection (it refuses a day with no stored plan).
- Rejected: storing index levels as `ratio` (Decision 1). Rejected: a second dataset for market series (the table already holds a series by `series_id`).
- The IBKR index closes (`index_closes` of the glossary) and closes of stocks: no dataset, no rpc; a bars design.
- Other FRED series the pages name (CPI and others): their own page designs.

## Adversarial

Read: the compute design adversarial review, rev 1 of the four files (kept by the dispatcher, not committed; REV1). Older answers (ADV, rev 2 of the first draft: vintages, migration numbers, MANIFEST, offline checks) stand as in rev 1 of this file. Adopted means the text above changed.

- REV1:13 (history load is the owner's manual act; first run is missing everywhere): adopted. Decision 3 says what is missing until the load, and `compute-figures.md` tests it (`TestFirstRunOnEmptyStores`) instead of leaving it in prose.
- REV1:15 (shared generated files, `wire.go`, `ot.md` strings, migration numbers): adopted. `series-unit-index-points` comes after `reading-dataset`; `series-load-wire` after `compute-jobs-wire`; the subcommand count is checked against the registrations, not written as "nine"; the migration acceptance checks unique and gapless numbers, not `tail -3`.
- REV1:R1 (no closes beyond SP500; first-run state is prose): the first-run state is adopted as a test (REV1:13); IBKR index closes stay out (no dataset), named in Left out.
- REV1:R2 (FRED lag against `MaxAgeDays` 4 days): adopted in `compute-figures.md` Decision 7 (8 calendar days for FRED figures, `data_day` kept); an irreversible `ALTER TYPE ... ADD VALUE` and the new enum value reaching TS types are kept: the glossary needs the unit (GLOSSARY.md:75), the value is additive on r1, and Decision 1 names why `ratio` is wrong.
- REV1:R3 (skip the loader chain by a `from` override of `ot backfill` or `fred_series`): not adopted: `ot backfill` re-runs keys of a stored plan and the job fetches one date by contract (`run.go:74`); an override changes the plan hash and the window rule that the scheduler design also edits. Merging the interface issue into the loader issue: adopted (one module, one pull request, nothing uses the interface between them). The loader, its wiring and the ninth subcommand stay: they are the least change to `fredseries`.
- REV1:R4 (lock-in: `index_points` forever, the seven ids, a public subcommand): kept and named; no alternative stores a Cboe index level without a wrong unit.
- REV1:R5 (`TestSeriesPointFixtureValues` hard-codes DGS10 and percent): adopted. The test is rewritten per record and the acceptance greps that the literals are gone (checked on main: schema_test.go:165-176).
- REV1:R6 (`fred-market-series` needs the network; `ls | tail`; 0014 waits for 0012 and 0013): the network part is kept and said plainly (the file is checked by a reviewer opening FRED once; no offline test can prove it); `tail -3` adopted as a set check; the dependency on 0012 and 0013 stays because the gapless check is the reason for it.
- Cross-file (the chain waits for PR 335): adopted by naming `sched-store-ops` and `sched-jobs-schedule` and stating that the chain is not cut before they merge.
- Page schemas that enumerate series units (the advisor's check): `contracts/schemas/page.macro_rates.v1.json:21` and `page.macro_rates.v2.json:28` list `percent, ratio, ccy_amount, count` (and so does `web/src/generated/macro_rates.v2.ts:38`); no other file under `contracts/schemas` does (a grep of `ccy_amount` finds these three only). The page lists one series: `seriesIDs = []string{"DGS10"}` (`internal/pages/macro_rates/builder.go:28`, unit `percent`), so no series of this design reaches it and `index_points` cannot be written into that page. Closed: no schema change belongs to this design. A page design that adds a new series id to a page that carries the unit enum brings the schema change as its own interface entry.
