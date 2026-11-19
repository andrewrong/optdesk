# store/coverage

Package `internal/store/coverage` declares the read-only view of the runner's ledger that the data health page reads (`docs/design/data-health.md`, Interface, entry 1). It holds declarations only; `internal/store/ops` implements it, as the page role `ot_ro_pages`, through the view `rd.coverage_day_r1`. It also holds `WindowSessions` (30, the session days shown and repaired) and `LookbackDays` (60 calendar days, enough for 30 sessions).

## States

`ok`: the key is present and the source gave a value.

`no_value`: the key is marked `no_value` and the newest run of the key finished clean: the source answered "nothing for this key".

`expired`: the key is marked `no_value` and the newest run of the key is not a clean finish: the runner closed the key at its window after a failed, partial or crashed run. A real gap, not an answer.

`missing`: the plan of the day expected the key and it is not present.

## Cell fields

`Day` is the session day of the cell.

`Dataset` is the ledger dataset id (not a job id).

`Keys` is the number of expected keys of the (day, dataset); `Keys == OK + NoValue + Expired + Missing`.

`OK`, `NoValue`, `Expired` and `Missing` count the keys in each state.

`MissingKeys` holds at most 20 missing subject keys, ascending.

## Meaning of ok

ok means the runner recorded the key as stored. It is the runner's record that the data was written in that turn, not a count of data rows; a table restored or pruned outside the runner leaves the ledger green.

## Implementation

`internal/store/ops` implements `Reader` (`OpenCoverage`, opened with the pages DSN like `OpenGapReader`). `Cells` reads `rd.coverage_day_r1` (migration `0051_coverage_view.sql`: one row per session day and dataset, columns `session_day, dataset_id, keys, ok, no_value, expired, missing, missing_keys`, owner `ot_owner`, `SELECT` for `ot_ro_pages`); `Sessions` reads `rd.session_day_r1`; `NewestObservations` reads `rd.series_point_r1`. The view counts in SQL, so no per-key row reaches Go. For a key marked `no_value` the newest `ops.capture_run` row (the one finished last, a row still running first) decides: `expired` when its status is not `done`, its `error_code` is not empty or its `reason_code` is `expired` or `crashed_repeatedly`; otherwise `no_value`. A `no_value` key with no run row at all counts as `no_value`.

## Repeated failures

`FailureReader` (`RepeatedFailures(ctx, from, to)`, never nil, unordered) is a separate interface implemented by the same value as `Reader`, so fakes of `Reader` keep compiling. The rule (`docs/design/repeated-failures.md`, Decision 2): a key (dataset and subject key) is listed in a kind, `failed` or `empty`, when it has runs of that kind on `FailureMinDays` (3) or more distinct session days among the last `FailureWindowDays` (14) session days of its calendar and its newest counted done run is failed or empty. Both constants are written only in `api.go`.
