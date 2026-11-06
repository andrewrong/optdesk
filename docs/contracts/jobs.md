# jobs contract

## Responsibility

The jobs the binary runs. `fredseries.New` builds the `fred_series` job: one FRED series (DGS10) fetched through a `sources.Source` and written through a `store.SeriesWriter`. The job implements `contracts.Job`; the constructor has the type `jobs.NewFredSeriesFunc`, and `jobs.Registry` lists the jobs the binary runs.

`New` returns an error when the source, the writer or the clock is nil, and when the embedded manifest is invalid.

## Inputs

- `internal/jobs/fredseries/job.toml`, embedded: `version`, `id`, `series`, and one `[window]` table (`rule = "after_close"`, `lateness_hours = 24`, `calendar = "XNYS"`), read with github.com/pelletier/go-toml/v2. The fields are `version`, `id`, `series` (the list of series ids) and the `[window]` table with `rule`, `lateness_hours` and `calendar`. A manifest without an id or without a series list fails with the error "fredseries: manifest needs an id and a series list".
- A `sources.Source`, a `store.SeriesWriter` and a `clock.Clock`, passed to `New`.
- A `contracts.Day` and, for `Run`, the keys of `Plan`.

## Outputs

- `Plan(day)`: one `ExpectedKey{obs.series_point, <series id>}` per series, sorted by id, and the plan hash: the lowercase hex SHA-256 of the job id, the manifest version, the day and the sorted series ids, joined with a newline.
- `Run`: a `JobResult`; `Returned` is the points the source gave, `Usable` those with a value. Points are written in one `InsertSeriesPoints` call; a `no_value` point keeps its nil value and status.
- Errors of `Run` are `contracts.TypedError`, through `asTyped`, which returns an error unchanged when it is already typed and wraps it with a kind otherwise:
  - a fetch error that is a `contracts.TypedError` is returned unchanged, else it is `Permanent`; nothing is written.
  - a failed `InsertSeriesPoints` is `Retryable` unless the writer's error is already typed (`asTyped`), in which case its own kind is kept; `Usable` is 0 then.

## Invariants

- The source gets a `FetchPlan` with the series ids of the keys and `From` and `To` equal to the day.
- No write happens when the fetch fails or returns no points.
- The `[window]` keys: `rule` is required and `after_close` is the only accepted value; `lateness_hours` is an integer of at least 0 (absent means 0) and becomes the close offset in hours; `calendar` is an optional string and becomes the calendar id (absent means empty). `New` fails on a bad table; the error names the key and no value.
- The job answers the optional method `Window() (calendarID string, openOffset, closeOffset time.Duration)` with those values; the open offset is always 0. `contracts.Job` and `internal/jobs/api.go` are unchanged.
- The window rule is carried, not applied; the runner hookup belongs to the follow-up design.
- The package imports contracts, clock, sources, store and the `jobs` root only.

## Version history

- v1: `fred_series`, manifest version 1.

## Contract tests

- `internal/jobs/api_test.go`: `TestRegistryContract`, the interface.
- `internal/jobs/fredseries/plan_test.go`: `TestPlanHash`, `TestEmbeddedManifest`.
- `internal/jobs/fredseries/window_test.go`: `TestWindowOf`, `TestWindowErrors`, `TestEmbeddedWindow`.
- `internal/jobs/fredseries/run_test.go`: `TestRunWritesOnce`, `TestPlanLateRunRecovery`.

There is no test yet: the wrapping of an untyped fetch error as `Permanent` and the `Retryable` kind of a failed `InsertSeriesPoints` have no test (the fake writer never fails and no test gives the source an untyped error).

## Jobs and slots

`fred_series`, manifest version 1, window rule `after_close` with 24 hours lateness on calendar `XNYS`. Slots are assigned by the runner.

## Forbidden

- Importing another jobs package or pgx.
- `time.Sleep`, or reading the system time.
- Writing a missing value as 0.
