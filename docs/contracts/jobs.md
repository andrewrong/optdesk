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

## compute_figures

`computefigures.New(jobs.ComputeStores, compute.Figures, compute.Rules, compute.Units, compute.SeriesOps, clock.Clock)` builds the job `compute_figures` (type `jobs.NewComputeFiguresFunc`); the design is `docs/design/compute-figures.md`. `jobs.ComputeStores` holds the seven store interfaces the job uses: `Catalog` (`store.OptionEodCatalog`), `OptionEod` (`store.OptionEodReader`), `Terms` (`store.OptionTermsReader`), `Names` (`store.InstrumentNameReader`), `SessionDays` (`store.SessionDayReader`), `Series` (`store.SeriesReader`) and `Readings` (`store.ReadingStore`). The job is declared here; its implementation is a later issue.

- Ease: `computefigures.NewWithEase` (type `jobs.NewComputeFiguresEaseFunc`) takes a `compute.Ease` after the `compute.SeriesOps` and before the clock; `New` is `NewWithEase` with nil. Only a job built with an `Ease` plans the key `{readings.reading, "macro.ease|market|WALCL"}`, and its `Postcondition` is a reading row of the as-of day, ok or missing, like every other key; a job built with nil has no `macro.ease` key and runs no ease step. The job reads the seven series of `[ease]` in `job.toml` and stores the reading of `docs/design/ease-v2.md` Decision 7. The ease is stored after the market figures and after IVP and IVR, so its failure (a series read that fails leaves the key without a row, named by the postcondition) stops no other key.
- Keys: one `{readings.reading, "name|subject_kind|subject"}` per (rule, subject). `subject_kind` is `underlying` (subject is the symbol of `InstrumentNames`, else `underlying:<id>`) or `market` (subject is the FRED id of the first input).
- Steps, in order: IV30, then the market figures, then IVP and IVR from the stored IV30 readings, so a missing IV30 gives a missing IVP with `no_input`. A step that fails does not stop the others; its keys are written missing or stay absent, and the postcondition names them.
- As-of day: the slot day. `Postcondition`: every planned key has a reading row for the as-of day, ok or missing.
- Iv30 input rules (Decision 10): subjects are the underlyings of `OptionEodCatalog.OptionEodLatest(ctx, day)` that have a row of the day itself in `OptionEod(ctx, id, day, day)`, named by `InstrumentNames`. Of one `instrument_id` the row with the latest `received_at` is used, and only `status` 1. `OptionTerms` is asked in chunks of 500 ids; a row without a current terms version is dropped and counted. The mid is (bid+ask)/2 when both are present, `ask >= bid` and `ask > 0`, else that side is absent; an IV absent or not above 0 is 0. `T` runs from the close of the as-of day's session row (calendar `XNYS`) to the `close_at` of the expiration day's session row, or its `open_at` for an `AM` settlement; an expiration day that is not a session day drops that expiry; minutes are floored. The rate is the newest `DGS3MO` point not after the as-of day; none gives a missing `no_input`; a point more than 4 calendar days before the as-of day (`max_age_days` of `market.iv30_own`) gives a missing `stale_input` (stored `missing_input`, `reason_code` 3); a point of an earlier day within 4 calendar days is used and the parameter `stale_rate` is true.
- Retry classes: a store error is `Retryable`; a bad rule (an unknown id or version, a rule the core rejects) is `Permanent`. A figure the core marks missing is stored `missing_input` with its reason, never skipped.

## SeriesHistoryLoader

`jobs.SeriesHistoryLoader` has `Load(ctx, sources.FetchPlan) ([]jobs.SeriesLoaded, error)`; `jobs.SeriesLoaded` is `{SeriesID string; Fetched int}`; the constructor type is `jobs.NewSeriesHistoryFunc` (`serieshistory.New(sources.Source, store.SeriesWriter)`). The design is `docs/design/compute-fred-series.md#series-history-loader`; `ot series-load` only wires it.

- `Load` refuses an empty series list, `From` after `To` and a range over 5 years as `Permanent`, before any fetch.
- One `Fetch` per series over `From`..`To`; every series is fetched before the first write, so a source error writes nothing.
- The points of a series are written once, with one `InsertSeriesPoints` call (idempotent), and `Load` returns the count fetched per series.
- A source or writer error is passed on by class only: a `TypedError` keeps its kind, an untyped fetch error is `Permanent`, an untyped write error `Retryable`; the text is not passed on. Only `contracts.TypedError` values are returned.
- No clock and no environment are read.

## calendar_refresh

`jobs.NewCalendarRefreshFunc(clock.SessionSource, store.SessionDayStore, clock.SessionDayDiffer, clock.Clock, jobs.CalendarObserver)` builds the job `calendar_refresh` (implementation: `internal/jobs/calendarrefresh.New`, a later issue); the design is `docs/design/compute-calendar.md`. `jobs.CalendarObserver` is `func(clock.SessionDayChange)`.

- Id `calendar_refresh`. Key: one `{master.session_day, "XNYS"}`.
- Span: from the day of the injected clock minus 7 to that day plus 400, inclusive. `Run` reads the stored rows and the fresh rows of the source over the span, takes the diff with the differ, upserts only added and changed rows through the store and deletes none.
- Retry classes: an empty fresh answer is `Permanent` (nothing is overwritten with nothing). A source error keeps the class the source gave: exit 1, exit 2 and any other nonzero exit of the command are `Permanent`, a failure to start or wait for the process (a kill at the run bound included) is `Retryable`. A store error is `Retryable` unless already typed.
- `Returned` and `Usable` are the fresh rows read and checked.
- `Postcondition`: the stored rows cover every day from the as-of day to the span end; false until the first successful refresh, true again on a no-change day.
- Observer: called once per changed day, in day order, with the `clock.SessionDayChange`; the wiring writes one log line per call (`component=calendar_refresh`, day, kind added or changed, before and after). It is the only trace of the diff; no table holds it.

## Version history

- v1: `fred_series`, manifest version 1.
- v1: `option_eod`, manifest version 1.
- v1: `compute_figures`, interface only (`jobs.ComputeStores`, `jobs.NewComputeFiguresFunc`).
- v1: `compute_figures` stores `macro.ease` when built with an `Ease` (`jobs.NewComputeFiguresEaseFunc`, additive).
- v1: `SeriesHistoryLoader`, with `internal/jobs/serieshistory`.
- v1: `calendar_refresh`, interface only (`jobs.CalendarObserver`, `jobs.NewCalendarRefreshFunc`).

## Contract tests

- `internal/jobs/api_test.go`: `TestRegistryContract`, `TestNewComputeFiguresFuncContract`, `TestNewComputeFiguresEaseFuncContract`, `TestNewSeriesHistoryFuncContract`, `TestCalendarRefreshContract`, `TestCalendarRefreshErrorClasses`, the interface.
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
