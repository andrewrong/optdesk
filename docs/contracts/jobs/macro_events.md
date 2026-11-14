# macro_events

Design: `docs/design/macro-events.md#Decisions`.

## Inputs

- `internal/jobs/macroevents/job.toml`, embedded: `version = 1`, `id = "macro_events"` and a `[schedule]` table with the keys of `internal/jobs/fredseries/job.toml`: `calendar = "XNYS"`, `anchor = "time"`, `zone = "America/New_York"`, `offset = "6h"`, `run_bound = "5m"`, `restart_sensitive = false`. There is no `[window]` table.
- A `ReleaseSource` (`Dates(ctx, releaseID, from)`), a `macroevent.Writer` and a `clock.Clock`, passed to `New` (`NewMacroEventsFunc`). `NewRelease(baseURL, key)` builds the `ReleaseSource` over `net/http`: one GET of `fred/release/dates` with `release_id`, `realtime_start` = the run day, `include_release_dates_with_no_data=true`, `sort_order=asc`, `file_type=json` and the key. It drops dates before the run day, maps 429 and 5xx to Retryable and any other non-200 to Permanent, and no error text holds the key or the URL.
- `internal/jobs/macroevents/hand_events.toml`, embedded with `go:embed`: one `[[source]]` per kind and year with `kind`, `year`, `url`, `read_day` and `dates`. A source without `url` or `read_day` is refused.

## Outputs

- `Plan(day)`: 13 keys `ExpectedKey{obs.macro_event, <kind>}` sorted by kind, and the lowercase hex SHA-256 of the job id, the manifest version, the day and the sorted kinds, joined with a newline.
- Kinds. FRED, source `fred`, `source_ref` the release id: `cpi` (10), `employment_situation` (50), `pce` (54), `gdp` (53), `ppi` (46), `retail_sales` (9), `jolts` (192), `h41` (20); fetched one at a time in this order. File, source `file`, `source_ref` the url of the row, `run_day` the `read_day` of the row: `fomc` (the last day of the meeting), `fomc_minutes` (that day + 21 days), `boj`, `quarter_end`, `us_tax`.
- `Run`: one `Writer.ReplaceFuture(kind, source, runDay = the run day, events)` per kind. `Returned` is the events given, `Usable` the rows written. A kind with a fetch error is left unchanged and named in the result; the other kinds are still written, and the first error is returned (its own kind when typed, else Retryable). An empty answer is passed on and stores nothing. The result text (the `JobResult` has no text field, so it is logged at info level) names the last date per kind and "kept stored rows of <kind>: answer shorter or empty" for every kind the writer held.

## Invariants

- The delete rules (empty answer, latest date before the stored latest, a row dated the run day or earlier, a row in the answer) and `held_day` are those of `docs/contracts/store.md`; the job applies none of them itself.
- No skip rule: every run fetches. A repeated run upserts the same rows, deletes nothing more and holds nothing new. Force only repairs a past day: a forced run of a past day writes the rows of that day, and a forced run of the same day keeps today's event.
- The package imports contracts, clock and `store/macroevent` only.

## Contract tests

- `internal/jobs/macroevents/run_test.go`: `TestRunWritesKinds`, `TestRunEmptyAnswerStoresNothing`, `TestRunShortAnswerHeld`, `TestRunForceSameDayKeepsToday`, `TestRunOneKindFails`, `TestRunTwiceChangesNothing`.
- `plan_test.go`: `TestPlanKeys`, `TestManifestSchedule`. `release_test.go`: `TestReleaseQuery`, `TestReleaseErrorKinds`, `TestReleaseErrorNeverHoldsKey`. `hand_test.go`: `TestHandRejectsNoURL`, `TestHandRowsHaveURLAndReadDay`, `TestHandCountPerKindAndYear`, `TestMinutesAre21DaysAfter`.

## Version history

- v1: `macro_events`, manifest version 1.
