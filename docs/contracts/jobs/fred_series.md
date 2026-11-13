# fred_series

## Inputs

- `internal/jobs/fredseries/job.toml`, embedded: `version`, `id`, `series`, and one `[window]` table (`rule = "after_close"`, `lateness_hours = 96`, `calendar = "XNYS"`), read with github.com/pelletier/go-toml/v2. The fields are `version`, `id`, `series` (the list of series ids: 16 ids: DGS10, DGS3MO, SP500, VIXCLS, VXNCLS, RVXCLS, VXVCLS, WALCL, WDTGAL, RRPONTSYD, DFF, BAMLH0A0HYM2 and the new BAMLC0A0CM, BAA10Y, SOFR, WRESBAL; the last four are also `windowed`) and the `[window]` table with `rule`, `lateness_hours` and `calendar`. A manifest without an id or without a series list fails with the error "fredseries: manifest needs an id and a series list".
- A `sources.Source`, a `store.SeriesWriter` and a `clock.Clock`, passed to `New`.
- A `contracts.Day` and, for `Run`, the keys of `Plan`.

## Outputs

- `Plan(day)`: one `ExpectedKey{obs.series_point, <series id>}` per series, sorted by id, and the plan hash: the lowercase hex SHA-256 of the job id, the manifest version, the day and the sorted series ids, joined with a newline.
- `Run`: a `JobResult`; `Returned` is the points the source gave, `Usable` those with a value. Points are upserted in one `seriesdef.SeriesUpserter.UpsertSeriesPoints` call with `writtenBy = "fred_series:" + <day>`, by the key (series id, observation date); the job asserts the writer to `SeriesUpserter` at `Run`. A `no_value` point keeps its nil value and status.
- Errors of `Run` are `contracts.TypedError`, through `asTyped`, which returns an error unchanged when it is already typed and wraps it with a kind otherwise:
  - a fetch error that is a `contracts.TypedError` is returned unchanged, else it is `Permanent`; nothing is written.
  - a writer without `UpsertSeriesPoints` gives a `Retryable` error naming the missing method; nothing is written and `Usable` is 0.
  - a failed `UpsertSeriesPoints` is `Retryable` unless the writer's error is already typed (`asTyped`), in which case its own kind is kept; `Usable` is 0 then.

## Second source

- `NewWithSecondary(src, second, w, clk)` builds the job with a backup source; `New(src, w, clk)` is `NewWithSecondary(src, nil, w, clk)` (no fallback).
- After the primary fetch (also when it returned no points; not when it failed), the job asserts the writer to `seriesdef.SeriesRegistry` and reads `ActiveSeries`. For each series of the run whose `SecondarySource` is `nyfed` and that has no point with a value in the primary fetch, it fetches the same plan (same `From`, `To`) from the second source, for those series only, and writes the points with `seriesdef.SeriesFiller.FillSeriesPoints` (`writtenBy = "fred_series:" + <day> + ":nyfed"`), never the upsert.
- No second source, a writer without `SeriesRegistry` or `SeriesFiller`, an unreadable registry, or a series with an empty `secondary_source` means no fallback. A second-source or fill error never changes the `JobResult`: it is logged (slog warning) only.
- Tests: `internal/jobs/fredseries/secondary_test.go`.

## Invariants

- The source gets a `FetchPlan` with the series ids of the keys and `From` and `To` equal to the day.
- No write happens when the fetch fails or returns no points.
- The `[window]` keys: `rule` is required and `after_close` is the only accepted value; `lateness_hours` is an integer of at least 0 (absent means 0) and becomes the close offset in hours; `calendar` is an optional string and becomes the calendar id (absent means empty). `New` fails on a bad table; the error names the key and no value.
- The job answers the optional method `Window() (calendarID string, openOffset, closeOffset time.Duration)` with those values; the open offset is always 0. `contracts.Job` and `internal/jobs/api.go` are unchanged.
- The window rule is carried, not applied; the runner hookup belongs to the follow-up design.
- The package imports contracts, clock, sources, store and the `jobs` root only.

## Jobs and slots

`fred_series`, manifest version 1, window rule `after_close` with 96 hours lateness on calendar `XNYS`. Slots are assigned by the runner.

## Contract tests

- `internal/jobs/fredseries/plan_test.go`: `TestPlanHash`, `TestEmbeddedManifest`.
- `internal/jobs/fredseries/window_test.go`: `TestWindowOf`, `TestWindowErrors`, `TestEmbeddedWindow`.
- `internal/jobs/fredseries/run_test.go`: `TestRunWritesOnce`, `TestPlanLateRunRecovery`, `TestRunWithoutUpsertIsRetryable`, `TestRunUpsertsOnceWithWrittenBy`, `TestRunEmptyFetchMakesNoUpsert`.
- `internal/jobs/fredseries/seed_test.go`: `TestRegistrySeedCoversManifest` (every manifest id is in migration 0043).

There is no test yet: the wrapping of an untyped fetch error as `Permanent` and the `Retryable` kind of a failed `UpsertSeriesPoints` have no test (the fake writer never fails and no test gives the source an untyped error).

## Version history

- v1: `fred_series`, manifest version 1.
