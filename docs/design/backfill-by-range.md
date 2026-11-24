# Backfill by range: a job sizes its own request from the rows it already holds, one request per key covers every missing day, and the broker gets one budget

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-24
Revision: 2

Source: owner 2026-11-24: "补数据千万不要死补：看缺啥补啥，有时一次请求就补完；不要连续好几次请求去补", and his answer "a" of the same day to the simple set (the job sizes its window from the stored rows; no new field, no migration, no new interface). Read on origin/main at 6f84b436: `AGENTS.md`, `docs/design/modules.md`, `docs/design/auto-catch-up.md`, `docs/design/bars-fallback.md` (format), `cmd/ot/{catchup,data_health_repair}.go`, `internal/jobs/{barsrefresh,ivhistory,fxrefresh,fredseries}/run.go`, `internal/broker/grpcbroker/history.go`, `pyside/src/optdesk/ibkr/{history,ibgateway,transport,api}.py`.

Needs the owner: nothing. Open points are under Left out.

## Goal

A repair of missing data sends the fewest requests that cover what is missing. A job looks at the rows it stores, finds where the stored rows stop, and asks the source once per key for the range from there up to the requested day. A run that misses two days is one request per key for those two days, not 400 days and not one request per day. The repair queue holds one request per job (the newest gap day), because that one request fills every older day up to the job's lookback. A failed history call is retried at most once after a pause. The gateway account has one budget of history requests that no caller can exceed. Same job runs one at a time (unchanged). Dev mode: no time-of-day guard is added.

## Today

Facts checked on 6f84b436; the first two corrections are against the adversarial read of revision 1.

- `catchUp` (`cmd/ot/catchup.go`) already reads `requestedToday` and applies `dropTaken`, so a (job, day) is requested at most once per New York day; it already keeps only the newest gap day for the three jobs of `windowEndingJobs` (`bars_refresh`, `fx_refresh`, `fred_series`). Two details matter: `dropTaken` runs before `newestGapOnly`, so when the newest gap day was requested today the next older day becomes "the newest" and is enqueued; and the owner's repair (`repairGaps` in `cmd/ot/data_health_repair.go`) enqueues every gap day of every job, with no newest-only rule.
- `iv_history` (`internal/jobs/ivhistory/run.go`) makes one `Bars` call per key and has no retry loop of its own. Its defect is the window: `BarQuery` has no `End`, so a run for a past day fetches the days ending today; it asks `DailyDays` (5) when a stored reading is ok in the lookback and `BackfillDays` (400) when none is.
- `bars_refresh` (`internal/jobs/barsrefresh/run.go`) reads the stored bars of the key up to the day (`PriceBars(..., firstDay, day)`), already sets `End = day` for a past run, but asks `BackfillDays` (400) for any past run and for the recovery of yfinance rows, and `pull` calls `Bars` up to `historyAttempts` (3) times in a row with no pause. A restated day makes `collect` pull the 400 days again, with the same loop.
- `fx_refresh` always asks `duration_days` (5); its `bar.FxRateReader` is held and unused. `fred_series` fetches the plain series for the day only, the windowed ones from day minus `lookback_days` (14) and the long ones from day minus `long_lookback_days` (460); its writer is a `store.SeriesWriter` and the real value also reads points (`store.SeriesReader`).
- The pyside `HistoryReader` (`pyside/src/optdesk/ibkr/history.py`) is the one place every history request of the account passes (`ibgateway.py` builds one). It already honours the end day (`end_date_text`), spaces starts by 11 s, and after a pacing answer waits 60 s and sends once more. It has no count of requests. `api.HistoryPaced` exists and `transport.py` already answers it with `RESOURCE_EXHAUSTED`, which `grpcbroker.Bars` turns into a Retryable error; nothing raises it today.
- Jobs whose source has no history (`quote_snapshot`, `option_eod`, option chains) are already skipped for a past day by the repair and the catch-up (`TestCatchUpPastDayStillSkipsOtherTodayOnly`).

## Decisions

Owner rules this design is written against, each self-checked in `## Adversarial`: (1) look up what is missing and fetch only that, one request when one can cover it; (2) never several requests in a row; (3) bounded retries so IBKR never locks the account; (4) the same job runs one at a time.

1. **The job sizes its window from its stored rows.** The request stays `{Job, Day}`. For the key it reads the rows it already stores up to `Day`, finds the first day after the newest stored good row (for bars: the day after the newest stored bar, or the oldest yfinance bar when a stored bar came from yfinance, because those are replaced; for iv_history: the day after the newest ok reading; for fx_refresh: the day after the newest stored rate), and asks one request that ends at `Day` and starts there. The length is the calendar days from that start to `Day`, never less than the job's own daily length (`daily_days`, `duration_days`) and never more than its backfill length. A key with no stored good row in its lookback keeps its first-fill length (`backfill_days`), and `claimFirstPull` still caps those. Present days inside the window are harmless: every job already writes by key and keeps or compares what is stored.
2. **One request per key, said plainly.** IBKR history is one contract per request, so "one request" is per key: a run of 20 keys makes 20 requests, each as short as its gap. A restated day still makes `bars_refresh` pull the full backfill once more for that key (existing behaviour; rare).
3. **`iv_history` sets `End`.** For a day before the UTC date of the clock its `BarQuery` carries `End = Day`, as `bars_refresh` and `fx_refresh` do; a run for today sends no `End`. This fixes the wrong range of a past-day repair.
4. **`fred_series` sizes only a past-day run.** FRED is not the broker, and its daily run keeps its lookbacks because the windowed and long series are re-read on purpose (late and revised values). A run for a day before the UTC date reads the stored points of the series of each group, and fetches from the day after the oldest of the groups' newest stored valued points (never earlier than the group's own lookback start, never later than the day). A series with no stored valued point keeps the group's lookback start. A writer without `SeriesReader` leaves the run as it is today.
5. **The queue holds the newest gap day of a job, for every job that sizes its window.** `windowEndingJobs` becomes `bars_refresh`, `fx_refresh`, `fred_series`, `iv_history`. Both callers apply it: the catch-up (and it applies `newestGapOnly` before `dropTaken`, so a newest day requested today does not let an older day through) and the owner's repair button (new there). A hole older than a stored day is not reached by the window of a newer request; it is the newest gap on the next pass and is requested then. No new `Request` field, no migration, no new interface.
6. **One budget, in `HistoryReader`.** The reader keeps the start times of the last 600 s in memory; a request that would be the 51st is refused before any slot or send with `HistoryPaced` (answered `RESOURCE_EXHAUSTED`, Retryable in Go). The limit is 50 per 600 s, below IBKR's 60. The count is of real sends, so the reader's own second send after a pacing answer is counted. The memory is lost with a restart, as the 11 s spacing already is.
7. **No stacked retries.** `bars_refresh` keeps one retry: `historyAttempts` becomes 2, the retry waits `retryPause` (60 s, the time IBKR names for a pacing answer) through an unexported pause function the job's tests replace, and a run makes at most `histDownAfter` (2) retries in all, so a bad run costs at most two pauses. The reader stops retrying: after a pacing answer it raises `HistoryPaced` at once instead of sleeping 60 s and sending again, so the Go job owns the only retry of the chain. `iv_history` and `fx_refresh` still have none; a Retryable answer reaches the runner, as today.
8. **Past days of quotes and option chains are not backfilled from the broker.** Unchanged and pinned: the repair and the catch-up skip a past day for a job whose source has no history; this design adds no path that fetches one.

## Modules

No change to `docs/design/modules.md`: the rows jobs (`internal/jobs/**`), ot (`cmd/**`) and pyside (`pyside/**`) exist, no depends cell changes (`ot` already imports `jobs`; no job imports another package it did not). No `go.mod` or `pyproject.toml` edit, no migration, no new table, no file under `contracts/`.

| issue | module |
|---|---|
| bfr-bars-window | jobs |
| bfr-bars-retry | jobs |
| bfr-iv-window | jobs |
| bfr-fx-window | jobs |
| bfr-fred-window | jobs |
| bfr-queue-newest | ot |
| bfr-ib-budget | pyside |

## Interface

No interface file changes. `internal/jobs/api.go`, `internal/broker/api.go`, `internal/store/**/api.go` and `pyside/src/optdesk/ibkr/api.py` keep every declaration (`HistoryPaced` is already declared there and already mapped). Contracts: unchanged.

## Issues

Every test calls `t.Parallel()`, writes files only under a temporary directory, uses a fake clock or a replaced pause, never sleeps, opens no port and touches no live broker or database. Acceptance is checked by reading, grep and git; test runs are under "Checked by CI". `BASE` is `git merge-base HEAD origin/main`.

### 1 bfr-bars-window

Module: jobs. Size: M.
Files: `internal/jobs/barsrefresh/run.go`, `internal/jobs/barsrefresh/window.go`, `internal/jobs/barsrefresh/window_test.go`, `internal/jobs/barsrefresh/run_test.go`, `internal/jobs/barsrefresh/repair_test.go`, `internal/jobs/barsrefresh/fallback_test.go`
Design: docs/design/backfill-by-range.md#1-bfr-bars-window
- Order: first.
Goal: Decisions 1 and 2 for `bars_refresh`.
Acceptance:
1. `window.go` (new) declares `windowDays(stored []contracts.PriceBar, day time.Time, daily, backfill int) int` (new): with no stored bar it returns `backfill`; with a stored bar it returns the calendar days from the day after the newest stored bar (or from the oldest stored bar whose source is yfinance, when any is) through `day`, at least `daily` and at most `backfill`: `TestWindowDays` (new, table: no bar, newest bar yesterday, newest bar eight days back, a yfinance bar nine days back with newer ibkr bars, a gap longer than `backfill`).
2. `runKey` in `run.go` no longer sets `days = r.j.m.BackfillDays` for `r.past` or for `hasSource(stored, sourceYfinance)`; it takes `windowDays(stored, ...)` for a key that has a stored bar in the backfill window, and keeps `days = r.j.m.BackfillDays` (with `claimFirstPull`) for a key with none: `TestRunPastDayAsksOnlyTheGap` (new: a stored bar three days before the repaired day gives one query with `End` set to the day and `DurationDays` 5 (three days counted inclusively, raised to the `daily_days` floor of 5), not 400), `TestRunYfinanceRowsAskFromTheOldest` (new: yfinance bars nine days back give one query of 10 days, not 400), `TestRunNoStoredBarKeepsBackfill` (new: no bar gives one query of 400).
3. The tests of `run_test.go`, `repair_test.go` and `fallback_test.go` that expected `DurationDays != 400` for a repair of a key with stored bars (`repair_test.go` around the past-day query, `fallback_test.go` around the recovery query) are changed to the sized length; no test is deleted.
4. A restated day still pulls `BackfillDays` once: the `collect` branch `again && days != r.j.m.BackfillDays` is unchanged.
5. `run.go` and `window.go` both stay under 600 lines, and `git diff --name-only BASE` lists only the Files.
6. The result matches docs/design/backfill-by-range.md#1-bfr-bars-window.

### 2 bfr-bars-retry

Module: jobs. Size: S.
Files: `internal/jobs/barsrefresh/run.go`, `internal/jobs/barsrefresh/job.go`, `internal/jobs/barsrefresh/run_test.go`, `internal/jobs/barsrefresh/fallback_test.go`
Design: docs/design/backfill-by-range.md#2-bfr-bars-retry
- Order: after 1 (bfr-bars-window, same file `run.go`).
Goal: Decision 7 in `bars_refresh`.
Acceptance:
1. `historyAttempts` in `run.go` is 2, its comment says one call and one retry after `retryPause`, and `retryPause` is declared as 60 seconds: `TestPullRetriesOnceAfterPause` (new: a Retryable first answer, then success, gives two queries and one recorded pause of 60 s; two Retryable answers give two queries, one pause and a Retryable error).
2. The job holds an unexported field `pause func(ctx context.Context, d time.Duration) error` in `job.go` whose default waits on a timer or the end of the context, and the package tests replace it so that no test waits: `TestPullPauseStopsWithContext` (new: a cancelled context ends the pull after the first call with no second query).
3. `runState` counts retries; once `histDownAfter` retries were made in one run a later failing key is not retried: `TestRunStopsRetryingAfterTwo` (new: five failing keys make five first calls and two retries, so seven queries and two pauses).
4. A Permanent answer is never retried (`TestPullPermanentIsNotRetried`, new), and the existing tests that compare a query count with `historyAttempts` stay valid.
5. `git diff --name-only BASE` lists only the Files, and no `time.Sleep` is added to the package.
6. The result matches docs/design/backfill-by-range.md#2-bfr-bars-retry.

### 3 bfr-iv-window

Module: jobs. Size: M.
Files: `internal/jobs/ivhistory/run.go`, `internal/jobs/ivhistory/run_test.go`
Design: docs/design/backfill-by-range.md#3-bfr-iv-window
- Order: first.
Goal: Decisions 1 and 3 for `iv_history`.
Acceptance:
1. `runKey` in `run.go` sets `End` of the `BarQuery` to the day when the day is before the UTC date of the clock, and leaves it empty for today: `TestRunPastDaySetsEnd` (new).
2. The length of the query is the calendar days from the day after the newest stored ok reading through the day, at least `DailyDays` and at most `BackfillDays`; with no ok reading in the lookback it is `BackfillDays` as before: `TestRunAsksOnlyTheGap` (new: newest ok reading six days back gives 6 days, ending at the day), `TestRunNoOkReadingAsksBackfill` (new: 400).
3. A stored `missing_input` reading of a day does not move the start: a repair of that day, with newer stored days, asks a window that contains it (`TestRunRepairsStoredMissingDay`, new).
4. The existing tests at the `DurationDays` assertions of `run_test.go` (400 first run, 5 second run) still hold.
5. `git diff --name-only BASE` lists only the Files.
6. The result matches docs/design/backfill-by-range.md#3-bfr-iv-window.

### 4 bfr-fx-window

Module: jobs. Size: S.
Files: `internal/jobs/fxrefresh/run.go`, `internal/jobs/fxrefresh/run_test.go`
Design: docs/design/backfill-by-range.md#4-bfr-fx-window
- Order: first.
Goal: Decision 1 for `fx_refresh`.
Acceptance:
1. `runPair` reads `LatestFxRate(ctx, pair, day)` through the reader the job holds (the comment "kept for the signature only" is removed) and asks `DurationDays` of at least `duration_days` and at most `fill_lookback_days` that reaches from the day after the newest stored rate through the day; a pair with no stored rate asks `duration_days`: `TestRunAsksOnlyTheGap` (new: newest rate eight days back asks 8 days; none asks 5), `TestRunGapLongerThanLookbackIsCapped` (new: a rate 30 days back asks 14).
2. A reader error is Retryable and no request is sent: `TestRunReaderErrorIsRetryable` (new).
3. `git diff --name-only BASE` lists only the Files.
4. The result matches docs/design/backfill-by-range.md#4-bfr-fx-window.

### 5 bfr-fred-window

Module: jobs. Size: M.
Files: `internal/jobs/fredseries/run.go`, `internal/jobs/fredseries/gap.go`, `internal/jobs/fredseries/gap_test.go`, `internal/jobs/fredseries/run_test.go`
Design: docs/design/backfill-by-range.md#5-bfr-fred-window
- Order: first.
Goal: Decision 4.
Acceptance:
1. `gap.go` (new) declares `gapFrom(ctx, r store.SeriesReader, ids []string, groupFrom, day contracts.Day) (contracts.Day, error)` (new): the day after the oldest of the newest stored valued points of the ids, not before `groupFrom` and not after `day`; an id with no valued point gives `groupFrom`: `TestGapFrom` (new, table).
2. `Run` for a day before the UTC date of the clock calls it for the windowed and the long group when `j.w` also implements `store.SeriesReader`, and passes the result as `From`; a run for today and a writer without the reader keep `groupFrom`: `TestRunPastDayFetchesFromTheGap` (new: newest points two days back give `From` of the day after them, not 14 or 460 days back), `TestRunTodayKeepsLookback` (new).
3. A reader error leaves the group at `groupFrom` (the run is not failed by the optimisation): `TestRunReaderErrorKeepsLookback` (new).
4. `git diff --name-only BASE` lists only the Files.
5. The result matches docs/design/backfill-by-range.md#5-bfr-fred-window.

### 6 bfr-queue-newest

Module: ot. Size: M.
Files: `cmd/ot/catchup.go`, `cmd/ot/catchup_test.go`, `cmd/ot/data_health_repair.go`, `cmd/ot/data_health_repair_test.go`
Design: docs/design/backfill-by-range.md#6-bfr-queue-newest
- Order: after 1 (bfr-bars-window), 3 (bfr-iv-window), 4 (bfr-fx-window) and 5 (bfr-fred-window): the queue holds one day only once every job fills the older ones.
Goal: Decisions 5 and 8.
Acceptance:
1. `windowEndingJobs` in `catchup.go` is exactly `bars_refresh`, `fx_refresh`, `fred_series`, `iv_history`, and its comment names the four.
2. In `catchUp` the call `newestGapOnly` comes before `dropTaken`: `TestCatchUpNewestRequestedTodayLetsNoOlderDayThrough` (new: gap days 11-02, 11-03, 11-04 of `fx_refresh`, a request of 11-04 already made today, so no request for 11-02 or 11-03 is enqueued).
3. `repairGaps` in `data_health_repair.go` keeps only the newest gap day for a job of `windowEndingJobs` before `repairGapsFor`, and every gap day for another job (`fundamentals` stays one per day); `repairGapsFor` is unchanged: `TestRepairWindowJobsNewestOnly` (new: `bars_refresh`, `fred_series`, `iv_history`, `fx_refresh` get one request for the newest day; `fundamentals` gets all three).
4. `TestRepairQueuesOldestFirst`, `TestRepairWindowJobOnePerDay` and `TestRepairCountEqualsGapDays` and the `fx_refresh` expectation of the multi-job test are changed to the newest-only result, and `TestCatchUpPastDayStillSkipsOtherTodayOnly` is unchanged.
5. `git diff --name-only BASE` lists only the Files.
6. The result matches docs/design/backfill-by-range.md#6-bfr-queue-newest.

### 7 bfr-ib-budget

Module: pyside. Size: M.
Files: `pyside/src/optdesk/ibkr/history.py`, `pyside/tests/ibkr/test_history.py`
Design: docs/design/backfill-by-range.md#7-bfr-ib-budget
- Order: first.
Goal: Decisions 6 and 7 in the reader.
Acceptance:
1. `history.py` declares `BUDGET_REQUESTS = 50` and `BUDGET_WINDOW_S = 600.0`; `HistoryReader.bars` raises `HistoryPaced` (from `optdesk.ibkr.api`) before taking a slot and before any send when 50 sends started within the last 600 s of the injected clock; a send older than 600 s no longer counts: `test_budget_refuses_the_51st` (new), `test_budget_window_slides` (new, fake clock), `test_refused_request_takes_no_slot` (new: the next accepted request starts 11 s after the last real send, not after the refused one).
2. After a pacing answer (code 162 with "pacing violation") the reader no longer sleeps `PACING_RETRY_S` or sends again: it raises `HistoryPaced`; `PACING_RETRY_S` is removed: `test_pacing_answer_raises_without_retry` (new: one send, no sleep of 60 s). The test that expected the 60 s retry is changed or removed.
3. A send that raises a broker error other than pacing still counts toward the budget and is raised as before (`test_error_send_counts`, new).
4. `git diff --name-only BASE` lists only the Files, and `pyside/src/optdesk/ibkr/api.py` is unchanged.
5. The result matches docs/design/backfill-by-range.md#7-bfr-ib-budget.

## Checked by CI

ci-go runs `go test` of `./internal/jobs/barsrefresh`, `./internal/jobs/ivhistory`, `./internal/jobs/fxrefresh`, `./internal/jobs/fredseries` and `./cmd/ot`, `go vet`, the architecture checks and the `Contracts: unchanged` check; ci-python runs `ruff check`, `mypy --strict` and `pytest -n auto` of `pyside`. No acceptance item above runs a test.

## Left out

- A `From` field on `jobrequest.Request`, a per-job attempts table and a daily request budget in PostgreSQL (revision 1): the job reads its own rows and the reader holds the budget, so none is needed.
- A hole older than a stored day inside one request: a window starts after the newest stored row, so an older hole is the newest gap on a later pass (Decision 5). Finding the oldest hole needs the calendar in each job; not done.
- `fx_refresh` gaps longer than `fill_lookback_days` (14): the primary window is capped there, as the fill source already is; an older gap is for the owner's explicit request.
- Sizing the daily run of `fred_series` (Decision 4): its lookbacks exist for late and revised values.
- A budget shared by several processes or kept across a restart: the reader is the one choke point of the account and a restart also loses its pacing memory.
- Marking a gap as unrecoverable from the source: no new reason code on the data-health page.
- Backfilling quotes or option chains for a past day from the broker (Decision 8).

## Adversarial

One fresh reader answered six questions against revision 1; its text is in the dispatcher scratchpad (`backfill-by-range.adversarial.md`, not in the repository). Adopted: the range reaches no `Job.Run` (the central gap), so revision 2 has no range type and the job sizes its window itself; "one request" is per key (Decision 2); merging contiguous days is dropped, a wide window over present days is harmless; the request budget is the reader's, not the caller's bookkeeping; the stacked retries are named and the layers reduced to one (Decision 7); the migration, the `From` field and the attempts table are dropped, so the chain is no longer serial behind a migration hold. Corrected after reading the code, because the adversarial text was wrong on them: `catchUp` already dedupes per day with `requestedToday` and `dropTaken` (the real defect is their order against `newestGapOnly`); `iv_history` has no retry loop (its defect is the missing `End`); `history.py` already honours the end day. Owner rules checked: look up what is missing and fetch only that (Decision 1: the window starts after the newest stored row); never several requests in a row (the queue holds one request per job, one request per key); bounded retries (Decision 7 plus the 50 per 600 s budget); same job one at a time (nothing here starts a run). Not adopted: a per-job daily budget in `job.toml`, because IBKR counts per account.
