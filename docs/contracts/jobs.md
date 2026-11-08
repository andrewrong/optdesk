# jobs contract

## Responsibility

The jobs the binary runs. `fredseries.New` builds the `fred_series` job: one FRED series (DGS10) fetched through a `sources.Source` and written through a `store.SeriesWriter`. The job implements `contracts.Job`; the constructor has the type `jobs.NewFredSeriesFunc`, and `jobs.Registry` lists the jobs the binary runs.

`New` returns an error when the source, the writer or the clock is nil, and when the embedded manifest is invalid.

## Inputs

- `internal/jobs/fredseries/job.toml`, embedded: `version`, `id`, `series`, and one `[window]` table (`rule = "after_close"`, `lateness_hours = 96`, `calendar = "XNYS"`), read with github.com/pelletier/go-toml/v2. The fields are `version`, `id`, `series` (the list of series ids) and the `[window]` table with `rule`, `lateness_hours` and `calendar`. A manifest without an id or without a series list fails with the error "fredseries: manifest needs an id and a series list".
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

## option_eod

`optioneod.New(broker.Broker, store.InstrumentResolver, store.OptionEodWriter, clock.Clock)` builds the job `option_eod` (type `jobs.NewOptionEodFunc`) from its embedded `job.toml`; the design is `docs/design/option-jobs.md`. `Plan` gives one key `{obs.option_eod, SYMBOL}` per `[[underlying]]`, sorted, and a hash of the day and every manifest setting. A `Run` of one key: a `day` other than the day of the clock in the manifest `zone` is `NoValue` with "wrong day" and nothing else happens; before `freeze_after` (default 1615) it answers an empty result with a nil error and calls no port. Otherwise it takes the underlying's price (`Last`, else `Close`, else `UndPrice` of one `Snapshot(KindSnapshot)`; none is `Retryable`), selects from the chain the nearest `max_expiries` expirations on or after the day and the strikes inside the `strike_band` with at most `max_strikes_per_side` per side (an empty selection is `Permanent`), resolves each pair with `ResolveContract` (a pair not listed gets no row), resolves the instrument ids (the option with the 21-character `osi` identifier) and asks `Snapshot(KindOptionEod)` in chunks of `chunk`, writing the rows of a chunk before the next. The clock is read before every broker call; once `budget_minutes` (default 8) have passed the run stops with `Retryable` and keeps the rows written. All rows of a key carry one deterministic `capture_id`, `uuid.NewSHA1(uuid.NameSpaceOID, day|dataset|subject)`, and `snapshot_kind` `eod`. Status: OK and PARTIAL 1, NO_DATA 2, NO_SUBSCRIPTION, TIMEOUT, ERROR and DISCONNECTED 5 with the IBKR code or the status name as `error_code`. `Returned` is the rows written, `Usable` those of status 1; a writer error is `Retryable` with `Usable` 0. Errors of the broker and the resolver keep their kind, an untyped one is `Permanent`. The `[window]` lateness defaults to 5 hours and is answered by `Window()`.

## Schedule

Each of the three jobs holds a `[schedule]` table in its embedded `job.toml`, read with go-toml/v2 beside `[window]`, and answers three optional plain-value methods (standard-library types only; no job package imports `internal/runner`, and a job has no preflight, which `cmd/ot` owns):

- `Schedule() (calendarID, anchor, zone string, offset time.Duration)`, from the keys `calendar`, `anchor`, `zone`, `offset`. `anchor` is `session_open`, `session_close` (`zone` must be empty) or `time` (`zone` is required and `offset`, in `[0, 24h)`, is the wall time since midnight in it). `offset` is a Go duration string such as `-30m`; absent means 0.
- `RunBound() time.Duration`, from `run_bound` (a duration, at least 0; absent means 0).
- `RestartSensitive() bool`, from `restart_sensitive` (default false).

A manifest without the table has no schedule: `Schedule()` returns empty strings and `RunBound()` 0. A table that is present needs `calendar` and a valid `anchor`. `New` fails on a bad table; the error names the key (`schedule.anchor`, `schedule.zone`, `schedule.offset`, `schedule.run_bound`, `schedule.calendar`) and no value.

Proposed slots, confirmed by the owner at review: `option_eod` `session_close` `0s`, bound 16m (8 minutes plus its budget), restart-sensitive; `fred_series` `time` 07:30 in `America/New_York`, bound 5m, not restart-sensitive; `quote_snapshot` `session_close` `-30m`, bound 16m, restart-sensitive.

## Version history

- v1: `fred_series`, manifest version 1.
- v1: `option_eod`, manifest version 1.

## Contract tests

- `internal/jobs/api_test.go`: `TestRegistryContract`, the interface.
- `internal/jobs/fredseries/plan_test.go`: `TestPlanHash`, `TestEmbeddedManifest`.
- `schedule_test.go` of each of the three packages: `TestEmbeddedSchedule`, `TestBadScheduleTable`.
- `internal/jobs/fredseries/window_test.go`: `TestWindowOf`, `TestWindowErrors`, `TestEmbeddedWindow`.
- `internal/jobs/fredseries/run_test.go`: `TestRunWritesOnce`, `TestPlanLateRunRecovery`.
- `internal/jobs/optioneod`: `TestPlan`, `TestManifestDefaults`, `TestSelect`, `TestRunWritesRows`, `TestCaptureIDStable`, `TestNotListed`, `TestBudget`, `TestGuards`, `TestErrors`.

There is no test yet: the wrapping of an untyped fetch error as `Permanent` and the `Retryable` kind of a failed `InsertSeriesPoints` have no test (the fake writer never fails and no test gives the source an untyped error).

## Jobs and slots

`fred_series`, manifest version 1, window rule `after_close` with 96 hours lateness on calendar `XNYS`. Slots are assigned by the runner.

## Forbidden

- Importing another jobs package or pgx.
- `time.Sleep`, or reading the system time.
- Writing a missing value as 0.
