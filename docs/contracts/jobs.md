# jobs contract

## Responsibility

The jobs the binary runs. `fredseries.New` builds the `fred_series` job: one FRED series (DGS10) fetched through a `sources.Source` and written through a `store.SeriesWriter`. The job implements `contracts.Job`; the constructor has the type `fredseries.NewFredSeriesFunc`.

`New` returns an error when the source, the writer or the clock is nil, and when the embedded manifest is invalid.

## Job documents

- [docs/contracts/jobs/fred_series.md](jobs/fred_series.md)
- [docs/contracts/jobs/account_capture.md](jobs/account_capture.md)
- [docs/contracts/jobs/iv_history.md](jobs/iv_history.md)
- [docs/contracts/jobs/option_eod.md](jobs/option_eod.md)
- [docs/contracts/jobs/option_eod_jp.md](jobs/option_eod_jp.md)
- [docs/contracts/jobs/compute_figures.md](jobs/compute_figures.md)
- [docs/contracts/jobs/serieshistory.md](jobs/serieshistory.md)
- [docs/contracts/jobs/calendar_refresh.md](jobs/calendar_refresh.md)
- [docs/contracts/jobs/calendar_hours.md](jobs/calendar_hours.md)
- [docs/contracts/jobs/earnings_refresh.md](jobs/earnings_refresh.md)
- [docs/contracts/jobs/dividend_capture.md](jobs/dividend_capture.md)
- [docs/contracts/jobs/option_ledger.md](jobs/option_ledger.md)
- [docs/contracts/jobs/cutover.md](jobs/cutover.md)
- [docs/contracts/jobs/option_candidates.md](jobs/option_candidates.md)
- [docs/contracts/jobs/bars_refresh.md](jobs/bars_refresh.md)
- [docs/contracts/jobs/fx_refresh.md](jobs/fx_refresh.md)
- [docs/contracts/jobs/fundamentals.md](jobs/fundamentals.md)
- [docs/contracts/jobs/position_marks.md](jobs/position_marks.md)
- [docs/contracts/jobs/statement_capture.md](jobs/statement_capture.md)

## Schedule

Each of the three jobs holds a `[schedule]` table in its embedded `job.toml`, read with go-toml/v2 beside `[window]`, and answers three optional plain-value methods (standard-library types only; no job package imports `internal/runner`, and a job has no preflight, which `cmd/ot` owns):

- `Schedule() (calendarID, anchor, zone string, offset time.Duration)`, from the keys `calendar`, `anchor`, `zone`, `offset`. `anchor` is `session_open`, `session_close` (`zone` must be empty) or `time` (`zone` is required and `offset`, in `[0, 24h)`, is the wall time since midnight in it). `offset` is a Go duration string such as `-30m`; absent means 0.
- `RunBound() time.Duration`, from `run_bound` (a duration, at least 0; absent means 0).
- `RestartSensitive() bool`, from `restart_sensitive` (default false).

A manifest without the table has no schedule: `Schedule()` returns empty strings and `RunBound()` 0. A table that is present needs `calendar` and a valid `anchor`. `New` fails on a bad table; the error names the key (`schedule.anchor`, `schedule.zone`, `schedule.offset`, `schedule.run_bound`, `schedule.calendar`) and no value.

Proposed slots, confirmed by the owner at review: `option_eod` `session_close` `0s`, bound 16m (8 minutes plus its budget), restart-sensitive; `fred_series` `time` 07:30 in `America/New_York`, bound 5m, not restart-sensitive; `quote_snapshot` `session_close` `-30m`, bound 16m, restart-sensitive.

## Contract tests

- `internal/jobs/api_test.go`: `TestWatchEntryPrices`, `TestCaptureFollowsPlannedKey`, `TestImportResultContract`; `internal/jobs/api_users_test.go`: `TestAPITypeHasTwoUserPkgs`, `TestUsersCounterOnTempTree`; the types of a job live in its package and are tested in its `types_test.go`: `computefigures` (`TestNewComputeFiguresFuncContract`, `TestNewComputeFiguresEaseFuncContract`), `serieshistory` (`TestNewSeriesHistoryFuncContract`), `calendarrefresh` (`TestCalendarRefreshContract`, `TestCalendarRefreshErrorClasses`), `optioneod` (`TestOptionEodWindowContract`).
- `schedule_test.go` of each of the three packages: `TestEmbeddedSchedule`, `TestBadScheduleTable`.

## Forbidden

- Importing another jobs package or pgx.
- `time.Sleep`, or reading the system time.
- Writing a missing value as 0.
