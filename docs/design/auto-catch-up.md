# auto catch-up: the scheduler finds the repairable gaps of every dataset by itself and queues them

Author: the optdesk designer
Owner approved: (pending)
Date: 2026-11-17 (revision 3, after the advisor's two blocking points on revision 2)

Source: owner words 2026-11-17 11:14Z (ask optdesk-4677): "你连续缺了好几天，你发一个请求就把这几天或者稍微冗余一些的去采集回来不就好了吗，手动补这个功能是我主动发起的，但是作为采集者你必须要尽可能保证数据的完整性"; 11:13Z: data the source can still give must be filled, data it cannot give (live option snapshots) may be lost, and the page tells the two apart per dataset. Earlier owner principles: late data is collected once it lands; our own guard is never a reason to call data unrecoverable; every job runs alone and is idempotent; dev mode, no time guards, only a same-job concurrency guard. Facts read on `origin/main` after `git fetch` (2026-11-17, #2676 merged) and on `GET /data-health`; nothing was run, no database read. This design replaces part A of the draft `late-data-recovery.md` except its `listedJob` replan (entry 2); its part B was rejected by the owner and is not here. Revision 2 takes the simpler way decided by the window: no new interface, the pass reuses the owner's repair path.

## 你会看到什么

- 不用再按「repair」。采集器自己每 30 分钟看一遍 `/data-health`：最近 30 个交易日里，哪个数据集缺了天（missing 黄、expired 红、not_run 灰），源头还给得出来的，就自动排一个补采请求，和你按按钮排出来的请求一模一样（在 `/job-console` 里能看到）。连续缺了好几天就排好几个请求（每个缺的日子一个，一遍最多 40 个，是试用值），一遍排不完下一遍接着排。同一个 job 的同一天，每个日历日最多自动排一次，不管上一次怎么结束；价格 K 线、汇率、FRED 这种一次取一段到那天的 job，一遍只排最新的那个缺口日。
- 灰色 not_run（意思是「这天从没排过计划」，不等于没有数据）以后也能补，也会被自动补：这些天的请求会把源头给的那一段再取一遍，重复取是安全的（按 key 覆盖，不会出现两份）。
- 源头只给「现在」的数据（期权实时快照等）补不回来：格子继续写原因，不再试。页面今天对这类格子写的是各 job 自己的原因（例如「nothing read proves that a second run equals the first」），只有 earnings 和 dividend 写「lost: the source gives only the present」。其中有几个原因其实是我们自己的限制（不是源头的限制），本设计不动它们，列在 Left out，等你决定。
- 一个数据集由几个 job 写（账户捕获：`account_capture` 只能当天，`statement_capture` 可以补任何一天；readings：`compute_figures` 可补，`iv_history` 只能当天），自动补采会用上能补的那个。
- 「repair」按钮还在，是你主动补的额外手段。
- 已知的限制：bars（价格 K 线）一天最多给 10 个新名字做首次长回补（`backfill_names_per_run = 10`），其余名字会落到 no_value，自动补采不会取 no_value 的格子；这是我们自己的上限，写在 Left out 和 Doubts。

## Today

Evidence is `file:line` on origin/main.

- **The plan model.** `Reconcile` walks only the session days after the cursor, runs each day's due jobs, and holds the cursor on a day that is not done (`internal/runner/reconcile/reconcile.go:103-139`). A missing key of a day it walks is run even after its window closed, reason `late` (`reconcile.go:393-426`), so an outage after the cursor is caught up, one run set per day. At the window close a key with only retryable runs becomes `no_value` with reason `expired` and is not planned again (`closeRow` `:432-460`, `expireIfClosed` `:466-487`; Decision 8 of `docs/design/scheduler.md`). The ledger holds plans only from about 2026-11-06, so the first 22 session days of the page (10-07 .. 11-05) are `not_run` in every dataset.
- **How the ledger records a day.** `PutExpected` stores the plan (`reconcile.go:250-256`); a key is `present` after a usable run (`MarkPresent`) or `no_value` after expiry. The page cell is `not_run` when a session day has no ledger row of the dataset (`internal/pages/data_health_page/cells.go:13-14,82`): "never planned", not "no data".
- **The console plans any day (#2676).** `planned` calls `PlanDay` for any day with no expected key, whatever the cursor (`cmd/ot/jobconsole_turn.go:189-205`; `PlanDay` stores plan and windows only, `reconcile/planday.go:17-39`). So a request for an old `not_run` day runs.
- **The page still refuses it.** A `not_run` cell prints "not planned: repair of an unplanned day is not built yet" and has no button (`internal/pages/data_health_page/repair.go:62-69`); `GapDays` counts only `missing` and `expired` cells with a button (`:120-131`). A past cell of a `today_only` or `listed_only` job prints that job's own `WhyText` (the guard text of `jobTable`, `cmd/ot/jobconsole_jobs.go:40-57,76-110`); only present-only jobs (`earnings_refresh`, `dividend_capture`, `cmd/ot/data_health_site.go:158-160`) and a dataset with no job print `lostText` (`repair.go:76-89`).
- **The owner's repair path.** `repairGaps` takes the first job of `dataHealthJobs()` that lists the dataset (`repairJobOf`, `cmd/ot/data_health_repair.go:55-65`) and queues one plain request per gap day through `jobrequest.Enqueue`, an open request of the same job, day and force being kept (`:68-103`). Two datasets have two jobs: `account.capture` (`account_capture` today-only first, then `statement_capture` any-day) and `readings.reading` (`compute_figures` any-day, `iv_history` today-only). `statement_capture` is therefore never repaired by the page.
- **Nothing starts a repair by itself.** The ticker runs `Reconcile` and `Sync` and claims console requests (`internal/runner/sched/ticker.go:531-559`). `claimNext` is in `cmd/ot/jobconsole_turn.go:53`; the scheduler builds the host from a `jobrequest.Consumer` only (`cmd/ot/jobconsole_sched.go:59-74`, `cmd/ot/scheduler.go:184`); `jobRequestOpeners` also holds `Poster` and `Reader` openers (`cmd/ot/jobconsole_open.go:16-20`), and the page's coverage reader is opened with `ops.OpenCoverage` on the pages DSN (`cmd/ot/data_health_repair.go:32-36`). `clockTimeout` and the phase limits are in `cmd/ot/jobconsole_host.go:21-26,117`.
- **The scheduler's DSN cannot insert a request.** The scheduler opens the consumer with the runner DSN (`cmd/ot/scheduler.go:184`, `jobconsole_sched.go:59-73`); role `ot_runner` has SELECT and UPDATE of seven columns on `settings.job_request`, and INSERT is granted to `ot_w_settings` alone (`migrations/0035_job_request.sql:53,65-67`); `jobrequestpg` inserts `(job_id, session_day, force)` only (`internal/store/jobrequestpg/store.go:101`), and the id is `GENERATED ALWAYS AS IDENTITY` (`0035:12`), which needs no sequence grant. The coverage reader needs no new secret: the scheduler already holds `cfg.pagesDSN` (`scheduler.go:214`). `internal/store/setup/settings_pg_test.go` (~`:300`) asserts that `ot_runner` cannot insert.
- **Which "today".** The page's today and the cell days are the New York day (`marketZone`, `cmd/ot/day.go:13,27-31`, used by `repairGaps` through `dayOrDefault`); a job with a `Zone` (a Tokyo-zone `today_only` job) gets its own local day only inside the verdict (`jobToday`, `internal/pages/data_health_page/repair.go:93-102`), and the pass takes the cells as that verdict gives them.
- **Jobs that already fetch a window; re-fetching is an upsert by key.**
  - `bars_refresh`: a key with no bar in `backfill_days = 400` pulls 400 days on its first run, but at most `backfill_names_per_run = 10` keys per run day do; a key past the cap returns "no request, no error" (`barsrefresh/run.go:66-84,104-113`, `job.toml`); a normal day pulls 5, a past-day run (`End = day`) pulls 400 (`run.go:25-31`).
  - `fx_refresh`: MIDPOINT `duration_days = 5` ending at the day, and a second source fills the last 14 days (`fxrefresh/run.go:156-161,196-203`).
  - `fred_series`: the 15 `windowed` series from `day - 14`, the 7 plain ones for the day only (`fredseries/run.go:78-90`). The windowed ones include weekly and monthly series (WALCL, NFCI, TLAACBW027SBOG): they have no observation on most session days, so a day's cell is `ok` only through the run that marks the key, never through stored rows of that day.
  - `fundamentals`: the whole document of each name up to the day (`fundamentals/run.go:98-120`); facts are keyed by filing and period day, not by session day.
  - `iv_history`: 5 / 400 days (`ivhistory/run.go:117-119`), today-only on the console by guard (`jobconsole_jobs.go:96`).
- **Is the 22-of-30 `not_run` only a ledger gap?** Not provable from code. For bars the 10-names-per-day cap means only some keys got the 400-day first pull (the adversarial read of 2026-11-17 showed that an earlier claim of "every key" was wrong). FX is complete for 5+14 days, FRED windowed series for 14 days, the rest unknown. One read-only count of `price_bar` rows per day, or the first pass itself (each request is a run, the cell turns `ok` or `no_value`), settles it. This design does not depend on the answer: it fetches.

Per dataset on `/data-health` (28 datasets x 30 session days; "gaps" is `missing` plus `expired` with a button on the page now):

| dataset (jobs) | source | past days? | one request covers | pass does |
|---|---|---|---|---|
| obs.price_bar (bars_refresh) gaps 4 | IBKR TRADES bars | yes, 400 d | a day's request pulls 400 d per key (first-pull cap 10 names) | request per gap day |
| obs.fx_rate (fx_refresh) gaps 2 | IBKR MIDPOINT, FRED H10 | yes, 5 d + 14 d fill | 19 d | per gap day |
| obs.series_point (fred_series) gaps 3 | FRED API | yes, years | 14 d for windowed, 1 d for plain | per gap day |
| obs.fundamental_fact (fundamentals) gaps 2 | SEC documents | yes, whole document | all | per gap day (one full fetch each) |
| obs.macro_event (macro_events) | FRED release dates, hand file | yes | per day | per gap day |
| account.capture (statement_capture; account_capture today-only) | IBKR Flex statement; account now | statement yes (insert-only) | per statement day | statement_capture per gap day |
| readings.reading (compute_figures gaps 8; iv_history today-only) | own store; IBKR IV bars | compute yes if inputs stored | per day | compute_figures per gap day |
| candidates.option_candidate gaps 2, strategy.result gaps 1 (option_candidates, book_risk) | own store | yes if inputs stored; the option chain of a past day is lost | per day | per gap day, ends no_value when input is lost |
| obs.earnings_date, obs.dividend_expectation | broker expectations | no | - | none; cell says "lost: the source gives only the present" |
| obs.option_eod, _index, _jp, obs.option_open, .jp, master.option_list, .jp, obs.quote_snapshot, account.position_mark, master.session_day, master.session_hours_notice | live chains, quotes, account and calendar state | no | - | none; cell prints the job's own WhyText (a guard text, see Left out 2) |
| master.option_contract(.gap, .jp), plan.option_open_item(.jp), optionledger.flex_trade_row | listed-only by our own guards | the Flex ledger yes | - | none (Left out 2) |

## Decisions

1. **The pass is the owner's repair, run by the scheduler.** For every dataset the pass runs the core of `repairGaps` (new `repairGapsFor`, shared by the page's POST and the pass), so the rule of "repairable" is the page's own verdict (an `any_day` job that is not present-only gets a button; a `today_only` job only for today), the days are `GapDays` over the page's 30 session days, and the requests are ordinary console requests (`Force: false`, visible on `/job-console`, run one job at a time by the existing per-job lock). No manifest field, no table, no new interface in `runner`, no change to any job.
2. **A dataset maps to all its jobs.** `repairJobOf` (first job) is replaced by `repairJobsOf` (every job that lists the dataset, in table order), and the owner's POST and the pass queue each job's own button days: `statement_capture` for `account.capture`, `compute_figures` (and, for today, `iv_history`) for `readings.reading`. `Enqueue` keeps an open request of the same job and day, so two jobs never double a request.
3. **The pass is bounded in one pass and over a calendar day, and has no memory of its own.** One pass runs under `clockTimeout` of 2 minutes (trial value) and enqueues at most 40 requests (trial value), oldest gap first across datasets in table order; a pass that stops at the cap or the timeout is continued by the next one. Over time: **at most one automatic request per (job, day) per calendar day, whatever its end state.** A (job, day) is not enqueued when `jobrequest.Reader.Recent(ctx, 1000)` (declared at `internal/store/jobrequest/api.go:90`; `Poster` embeds `Reader`, `api.go:98-99`, so the Poster reads it) holds a request of that job and day, in any state (queued, running, done, failed, refused, interrupted, withdrawn), whose `RequestedAt` falls on today's calendar day, the New York day of the injected clock (`dayOrDefault`, `cmd/ot/day.go:27-31`, as the page's today; a Tokyo-zone job is counted on the same day, so the skip is a property of the pass's day, not of the job's). An owner's own press of repair counts as that day's request too. A `done` request that left the day a gap (the source gave nothing) is therefore taken again tomorrow, not every 30 minutes; no in-memory map and no time throttle, and a restart changes nothing. The bound is gap days x jobs per calendar day, and the 40-request cap per pass.
   **A job that fetches a window ending at the day is requested for its newest gap day only, per pass**: `bars_refresh` (a past-day run pulls 400 days ending at the day, `barsrefresh/run.go:25-31`), `fx_refresh` (MIDPOINT 5 days and a 14-day fill ending at the day, `fxrefresh/run.go:156-161,196-203`) and `fred_series` (its windowed series from `day - 14` to the day, `fredseries/run.go:78-90`). One request already re-fetches the older gap days of its window; when the newest day stays a gap it is blocked for the rest of the day by the rule above and the next pass takes the next newest. The set of such jobs is a table `windowEndingJobs` in `catchup.go`.
4. **Trigger and wiring.** `claimNext` (`jobconsole_turn.go:53`) runs one pass when it finds no request to claim and 30 minutes of the injected clock have passed since the last pass, and once after start (trial value 30 minutes: pacing of a scan, not a refusal to run; the owner's button is never held back by it). A pass only enqueues; it runs no job. The pass posts with the **runner connection**: the host gains a `jobrequest.Poster` opened with the runner DSN the scheduler already holds, which works only after entry 5 grants `ot_runner` the insert (Decision 9); no new DSN, no env or secret change. The coverage reader is opened with `ops.OpenCoverage` on `cfg.pagesDSN`, already in the scheduler's config. `openConsoleHost` gains that one parameter (`pagesDSN`) and its one caller, `scheduler.go:184`, passes `cfg.pagesDSN`; with either opener or the DSN missing the pass is off and one warn line says so, as `consoleOff` does for the console.
5. **The page.** A `not_run` cell goes through the verdict of a `missing` cell, `GapDays` counts `not_run` cells that have a button, and the text "not planned" is gone, so the owner's "repair N days" is the same set the pass takes. A `today_only` past cell keeps the job's own WhyText; this design does not rename it to "lost".
6. **Derived jobs plan the day again on a miss** (part A of `late-data-recovery.md`, Decision 2): `listedJob.Run` builds the inner job's plan again when no snapshot holds the keys (`cmd/ot/listedjob.go:36,145-172`; `compute_figures` is wrapped at `wire_computefigures.go:40`), so a gap older than the 7 newest days of snapshots runs.
7. **Late sources.** An `expired` day is a gap like any other, so the pass takes it again; a day whose cell is `no_value` is not taken (the source said it has nothing). The newest day itself, late today, is the normal runner's (retries every 10 minutes while empty before the close).
8. **No fewer-requests machinery now.** A request per gap day re-fetches an overlapping window (bars 400 d, FRED 14 d, fx 19 d), which is the owner's "a little redundant" but not "one request for the run". Reading stored rows first (`runner.Postcondition`, `internal/runner/api.go:194`, which only `compute_figures` and a few non-ranged jobs have) would need a new reader in each job constructor for bars, FRED and fundamentals; it is Left out 4 and can be a later optimisation without changing this design.
9. **The runner role may insert a request (entry 5, owner-merged).** A new migration (the next free number after `0053`) grants INSERT on `settings.job_request` to `ot_runner` for the three columns the Poster's insert sets, `(job_id, session_day, force)`, and nothing else; no sequence grant (identity column). The `ot_w_settings` grants stay; `ot_runner` still cannot delete, update `job_id` or `force`, or insert `state` or `keys`. This changes who may act with what permission, so its pull request is merged by the owner, and a migration holds the deploy for the owner. The migration's Down revokes the grant.

## Modules

No row of `docs/design/modules.md` changes (`migrations/**` belongs to store, `modules.md:68,91`). Entries 1, 2 and 4 edit ot (`cmd/**`), entry 3 edits pages (`internal/pages/**`), entry 5 edits store (`migrations/**`, `internal/store/setup/**`). Every `cmd/ot` test that entry 3 would turn red is fixed in entry 1, which lands first and makes each day of its fixtures `ok` unless a test sets it, so no entry breaks a test of another module. Entry 5 changes the permission test of its own module (`settings_pg_test.go`), and nothing else tests the grant.

## Interface

None. No `api.go`, `api.py`, JSON schema or generated type is edited; `runner.Postcondition` and `jobrequest.Poster`/`Reader` are used as they are. The page payload and `contracts/fixtures/page.data_health_page.v1.ok.json` are unchanged (the fixture keeps "not planned" as sample data only).

## Issues

Order: 1, 2 and 5 first (independent; 1 and 2 are ot, 5 is store), 3 after 1 (3 turns `not_run` days into gaps; 1 first pads every fixture day with an `ok` cell, so the `cmd/ot` repair tests no longer depend on `not_run` days), 4 after 1, 2, 3 and 5 (the pass cannot enqueue before 5 is merged and its migration applied). Every entry compiles and keeps main green on its own.

### 1 repair-all-jobs-of-dataset

Module: ot. Order: first.
Files: `cmd/ot/data_health_repair.go`, `cmd/ot/data_health_repair_test.go`
Contracts: unchanged
Design: docs/design/auto-catch-up.md#Decisions (Decisions 1 and 2)
Acceptance:
1. `repairJobOf` is replaced by `repairJobsOf` (`TestRepairCountEqualsGapDays`, which calls it, is changed to `repairJobsOf(ds)[0]`), which returns every job of `dataHealthJobs()` that lists the dataset, in table order; `repairGaps` queues, for each of them, the days that job's verdict gives a button, and returns the id of the first request queued: `TestRepairReachesEveryJobOfDataset` (new: for `account.capture` a past gap day queues `statement_capture` and not `account_capture`, today's gap queues `account_capture` too; for `readings.reading` a past gap day queues `compute_figures` and not `iv_history`).
2. The enqueue loop is one function `repairGapsFor(ctx, cells, job, poster, cap) (queued int, capped bool, err error)` shared by the owner's POST path and later callers; the caller filters the cells before the call, and `cap` is the number still allowed (0 = no cap): `TestRepairGapsForStopsAtCap` (new: five gap days and a cap of 2 queue two requests, oldest first, and the call returns queued = 2, capped = true).
3. The test fake `repairCoverage` itself answers an `ok` cell for every session day (2026-11-02 .. 11-06) of a dataset that the test gave no cell for, so in every test of the file (those using `repairOver`, `repairSite` or `repairCoverage{...}` directly) the days without a cell are no longer `not_run`: `TestRepairQueuesOldestFirst`, `TestRepairSkipsOkAndNoValue`, `TestRepairWindowJobOnePerDay`, `TestRepairTodayOnlyPastDay`, `TestRepairPresentOnlyPastDay`, `TestRepairExistingRequestIsKept`, `TestRepairIgnoresFormDays` and `TestRepairNothingQueued` keep their expected requests with that padding (`TestRepairNothingQueued` changed to give no cell for its dataset and expect zero requests because every day is padded `ok`); `TestRepairCountEqualsGapDays` also adds an `ok` cell for 2026-11-06 to its own `cells` so its gap stays 3; the other tests of the file pass unchanged.
4. No file outside the Files line is in the diff; each new test calls `t.Parallel()` and never sleeps.

### 2 listed-job-replan

Module: ot. Order: first.
Files: `cmd/ot/listedjob.go`, `cmd/ot/listedjob_test.go`
Contracts: unchanged
Design: docs/design/auto-catch-up.md#Decisions (Decision 6)
Acceptance:
1. When no snapshot holds the keys, `listedJob.Run` builds the inner job's plan for the day and runs the keys when that plan holds them; the snapshot map and its 7-day eviction are unchanged: `TestListedRunReplansOnMiss` (new: a day older than the 7 newest, and a fresh `listedJob` as after a restart, both run the keys).
2. A plan that lacks a key still ends Permanent "no plan for these keys": `TestListedRunPlanLacksKeyStaysPermanent` (new).
3. `TestListedKeepsDaysBounded` and `TestListedEvictsOldestPlanned` are changed: an evicted day now runs through the new plan and the test asserts the keys ran (not Permanent "no plan for these keys"); the other existing tests of `listedjob_test.go` pass unchanged; `cmd/ot/listedjob.go` grows by at most 40 lines; no file outside the Files line is in the diff.

### 3 catch-up-page-verdict

Module: pages. Order: after 1.
Files: `internal/pages/data_health_page/repair.go`, `internal/pages/data_health_page/repair_test.go`, `internal/pages/data_health_page/builder_test.go`, `internal/pages/data_health_page/cells_test.go`
Contracts: unchanged
Design: docs/design/auto-catch-up.md#Decisions (Decision 5)
Acceptance:
1. `verdict` has no branch of its own for `StateNotRun` beyond `beforeSlot`; a `not_run` cell and a `missing` cell of the same day get the same `Repair`, `WhyText` and `DueText`, and the constant `notPlanText` is gone, so `git grep -n "notPlanText\|not planned" -- internal/pages/data_health_page/repair.go internal/pages/data_health_page/repair_test.go` prints nothing (the glossary line in `strings.go` is wording only and is left): `TestNotRunFollowsTheVerdictOfMissing` (new: any-day, today-only past and today, listed-only, present-only, no job).
2. `GapDays` lists `not_run`, `missing` and `expired` cells that have a button, oldest first: `TestGapDaysCountsNotRun` (new: a `not_run` day of an any-day job is listed; of a today-only past day and of a listed-only job is not).
3. `TestRepairVerdicts` has the cases "not_run any_day" (button), "not_run today_only past" (the job's `WhyText`), "not_run today_only today" (button), "not_run no job" (`lostText`), "not_run no form" (none); `TestNotRunNeverButton` is removed.
4. `TestDatasetRows` and `TestGapDaysOnRow` in `builder_test.go` (their `GapDays` with `not_run` any-day cells become the new count), `TestGapDaysAndRepairAll` in `repair_test.go` (its 2 gaps become the new count) and the `not_run` cases of `cells_test.go` are updated to the new count and verdict, and `TestRepairAllCountsNotRunDays` (new, `repair_test.go`) shows `RepairAll` is `button` when the only gap days are `not_run`.
5. `contracts/` and `web/` are unchanged; no file outside the Files line is in the diff.

### 4 catch-up-pass

Module: ot. Order: after 1, 2, 3 and 5.
Files: `cmd/ot/catchup.go`, `cmd/ot/catchup_test.go`, `cmd/ot/jobconsole_turn.go`, `cmd/ot/jobconsole_turn_test.go`, `cmd/ot/jobconsole_sched.go`, `cmd/ot/jobconsole_host.go`, `cmd/ot/scheduler.go`
Contracts: unchanged
Design: docs/design/auto-catch-up.md#Decisions (Decisions 1, 3, 4 and 7)
Acceptance:
1. `catchup.go` declares `catchUpPass(ctx)` that, for every dataset of `dataHealthJobs()`, reads the cells as the page does, drops the cells the skip of acceptance 2 and the window rule of acceptance 3 drop, and calls `repairGapsFor` for each job of the dataset with the cap left of 40 requests in all (40 minus the `queued` counts returned so far) and under `clockTimeout` of 2 minutes: `TestCatchUpPassEnqueuesGapsOldestFirst` (new, fake poster and cells; it is the owner's case end to end: a multi-day gap of one dataset, one pass, and the queue holds those days oldest first), `TestCatchUpPassStopsAtCap` (new: 45 gap days enqueue 40 and the next pass the rest), `TestCatchUpPassStopsAtTimeout` (new, fake clock).
2. At most one request per (job, day) per calendar day, whatever its end state: a (job, day) with any request in `Recent(ctx, 1000)` whose `RequestedAt` is on the New York day of the clock is not enqueued, in every state; the same job and day is taken again on the next day; `no_value` cells are never taken: `TestCatchUpPassOneRequestPerJobDayPerCalendarDay` (new, table over every state including `done` with the cell still a gap, and the next day with the fake clock moved), `TestCatchUpPassNeverTakesNoValue` (new).
3. A job in `windowEndingJobs` (`bars_refresh`, `fx_refresh`, `fred_series`) is requested for its newest gap day only in one pass; a job outside it keeps one request per gap day: `TestCatchUpPassWindowJobNewestGapOnly` (new: three gap days of a bars dataset enqueue one request, the newest; when it is blocked by acceptance 2 the next pass takes the next newest; a `statement_capture` gap of three days enqueues three).
4. `claimNext` calls the pass when no request was claimed and 30 minutes of the fake clock passed since the last pass, and once after start; the pass runs no job: `TestClaimNextRunsCatchUpEveryThirtyMinutes` (new, in `catchup_test.go`, fake clock, no sleep).
5. `openConsoleHost` builds the host with a `jobrequest.Poster` opened with the runner DSN it already receives and a `coverage.Reader` from `ops.OpenCoverage` on a new parameter `pagesDSN`; with either missing the pass is off and one warn line is written, and the console still works: `TestOpenConsoleHostWithoutCoverageKeepsConsole` (new). `newConsoleHost` and its call in `jobconsole_turn_test.go:207`, and `openConsoleHost` and its only caller `scheduler.go:184` (passes `cfg.pagesDSN`), are updated together; no other DSN, env name or secret is added.
6. The pass logs one "catch-up" line per pass with `requested`, `skipped_today`, `skipped_window`: `TestCatchUpPassLogsCounts` (new, in `catchup_test.go`, reads the log buffer); each new test calls `t.Parallel()`; no file outside the Files line is in the diff; `jobconsole_turn.go` and `jobconsole_host.go` stay under 600 lines.

### 5 runner-enqueue-grant

Module: store. Order: first.
Files: `migrations/0054_runner_enqueue_grant.sql`, `internal/store/setup/settings_pg_test.go`
Contracts: unchanged
Design: docs/design/auto-catch-up.md#Decisions (Decision 9)
Owner case: a change of who may act or with what permission
Acceptance:
1. The new migration `migrations/0054_runner_enqueue_grant.sql` grants `INSERT (job_id, session_day, force)` on `settings.job_request` to `ot_runner` and nothing else (no table, view, sequence or column more); its Down revokes exactly that grant.
2. `TestRunnerRoleUpdatesJobRequestColumns` is changed: its `INSERT INTO settings.job_request (job_id, session_day, force) VALUES ('x', '2026-11-11', false)` line leaves its list of denied statements (`TestSettingsRolesCannotUpdate` is unchanged); `TestRunnerCanInsertRequestColumns` (new, tag `pg`, runs in GitHub CI) shows `ot_runner` inserts a row with those three columns and still gets a permission error for an insert that sets `state` or `keys`, for an update of `job_id` or `force`, for a delete, and for an insert into `settings.job_request_withdraw`.
3. The diff changes only `migrations/0054_runner_enqueue_grant.sql` and `internal/store/setup/settings_pg_test.go`.

## Checked by CI

`ci-go`: `go test ./cmd/ot/... ./internal/pages/...` and `scripts/arch` (entries 1-4); the PostgreSQL job with tag `pg` for entry 5.

## Checked by owner

Entry 5: the owner merges its pull request; the migration holds the deploy for the owner (rebuild `ot`, setup pg, migrate, setup pg), and entry 4 is not merged before the migration is applied to the deployed database.

After deploy: the scheduler log has a "catch-up" line each 30 minutes; `/job-console` shows requests of every repairable dataset; `not_run` cells of `obs.price_bar` turn `ok` or `no_value`; no cell on `/data-health` prints "not planned". That the grey cells of each dataset turn green or say why they cannot, and that the number of auto requests per pass (40, newest gap only for bars, fx and FRED) does not crowd out the scheduled jobs (they share the one console worker).

## Left out

1. **Part B of `late-data-recovery.md`** (stored-close fallback for JP options): rejected by the owner 2026-11-17. Its part A runner reopen (`late_source` run) is not needed (Decision 7).
2. **Guards that are not source limits**: `iv_history` (`not_proved`), `option_ledger` (`caches_failures`, a Flex range query), `option_open_resolve*` (`heavy_and_shares_cap`), `calendar_hours_*`, `underlying_resolve`, `option_list_*` (`not_proved`): the page shows their guard text. Each needs its own change and a proof that a second run equals the first. Open question for the owner.
3. **The bars first-pull cap** `backfill_names_per_run = 10`: our own limit; keys past it end `no_value` and the pass never takes `no_value`. Lifting the cap for a past-day run, or marking such keys `missing`, is a change in `internal/jobs/barsrefresh` and needs its own design.
4. **Fewer requests** (reading stored rows before a request, or one request for a run of days): needs a reader in the constructors of bars, FRED and fundamentals (`NewFredSeriesFunc` and `NewFundamentalsFunc` take only a writer) and `runner.Postcondition` or a new method; not needed for completeness. FRED weekly and monthly series and fundamentals have no per-session-day rows, so a stored-row check would not work for them anyway.
5. **A manifest field per job for the look-back**: the look-back is the page's 30 session days (`coverage.WindowSessions`, `internal/store/coverage/api.go:14`); a field would repeat what `jobTable` says. Changing the page's window now changes what is collected.
6. **Live option snapshots**: lost by the source's limit, accepted by the owner (11:13Z).
7. **A page text "tried today, still a gap"**: no such text exists on `/data-health` today, so the page is not changed for it; the owner sees the day's request on `/job-console`.

## Relation to the registered drafts

- `p1-console-plans-past-day`: already merged as #2676; withdraw the draft.
- `p1-health-notrun-repair`: superseded by entry 3 (its acceptance 4 is reversed: the repair set equals the automatic set); a registered draft is frozen, so withdraw it (`crew ask <p> draft --withdraw`) and cut entry 3.
- P1 leftovers 1 and 8 are closed by this design; 9 is the accepted loss; 10 and 11 are Left out 2; 4 is a text only, not covered.

## Doubts

1. A source that never has a day (a request ends `done`, the cell stays a gap) is requested once per calendar day for as long as the day is inside the 30 session days, then leaves the page's window. If the log shows that daily request is noise, add a cool-down of several days; not now.
2. Two jobs on one dataset (account.capture) share one cell; a gap day of `statement_capture` shows for the whole dataset and `account_capture` (today-only) adds today only.
3. Whether 40 requests, 30 minutes and `Recent(ctx, 1000)` (more than a day's requests) are right; they are trial values to be read from the first logs.

## Adversarial

The read is the window's file `adversarial-auto-catch-up-2026-11-17.md` (kept outside the repository). Each point:

1. Gap against the goal, partial: adopted. Datasets map to all their jobs (Decision 2, entry 1); the "lost" claim was wrong and the text now says what the page prints (Today, Decision 5); the "expired and later ok" rule is dropped (Decision 7), so `stateOf` hiding one `no_value` key no longer matters; iv_history and the other guards are named for the owner (Left out 2) but not changed, as the window told me to keep the cut small; the newest late day stays with the normal runner (Decision 7).
2. Unbounded pass and missing wiring: adopted. `clockTimeout` of 2 minutes, a cap of 40 requests, the wiring of a poster and a coverage reader in `jobconsole_sched.go` and `jobconsole_host.go` (Decisions 3 and 4, entry 4). The fundamentals SEC-rate risk is bounded by the cap and recorded as Left out 4.
3. Simpler way: adopted entirely. `Stored` and entries 1-3 are gone; the pass calls the core of `repairGaps`; `Postcondition` is named only as a later optimisation (Decision 8).
4. Lock-in: adopted. No new interface, no `present` mark without a run row, the 6-hour in-memory map is replaced by queue state; the page-verdict-as-policy lock-in remains and is Left out 5.
5. Claim most likely false: adopted in all six parts: the bars cap (Today, Left out 3); the console plans past days since #2676 (the dependency and the old Decision 8 are deleted); fx has a reader and fundamentals and FRED do not (Left out 4); the idempotence claims kept; `claimNext` is in `jobconsole_turn.go:53`; FRED weekly and monthly series and fundamentals without per-day rows are described in Today and Left out 4.
6. Likeliest cause of a slip: adopted. Files lists are complete (`jobconsole_turn.go`, `jobconsole_turn_test.go`, `jobconsole_sched.go`, `jobconsole_host.go`, `data_health_repair_test.go`, `builder_test.go`, `cells_test.go`), each entry stays in one module, the `cmd/ot` repair tests are padded with `ok` days in entry 1 before entry 3 makes `not_run` a gap, `scheduler.go` is on entry 4's Files, no entry waits on an unmerged work outside this file, the contracts fixture and its test are left alone as sample data.
(a) `MarkPresent` without a run row: moot, the design never does it.
(b) Readers: fx yes, bars yes, fundamentals and FRED no: no reader is needed now (Left out 4).
(c) Same-job guard and "no time guards": adopted. The pass only enqueues, `Enqueue` keeps an open request, the console's per-job lock stays; the 30-minute interval is pacing of a scan and the 6-hour throttle is gone; the pass has its own timeout.
(d) Modules and Files: adopted; modules as in the Modules section, Files completed. `docs/contracts/runner.md` is no longer touched because `runner` no longer changes.

Revision 3, the advisor's two blocking points:
1. Entry 4 could not enqueue: the scheduler holds the runner DSN and `ot_runner` has no INSERT. Answered by entry 5 (grant of three columns, owner-merged, migration holds the deploy), Decision 9, Decision 4 (post with the runner connection, no new DSN; only the existing `cfg.pagesDSN` is passed on) and entry 4's order after 5; the Modules section names store for entry 5.
2. "Bounded" did not hold over time: Decision 3 now gives one request per (job, day) per New York calendar day whatever the end state (`TestCatchUpPassOneRequestPerJobDayPerCalendarDay`), and newest-gap-only per pass for `bars_refresh`, `fx_refresh` and `fred_series` (`TestCatchUpPassWindowJobNewestGapOnly`); Doubt 1 is replaced; the page is not touched (Left out 7).
