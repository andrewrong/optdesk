# Compute calendar: the trading calendar from pyside, refreshed daily with a diff

Author: the optdesk designer
Owner approved: (pending)
Revision: 3

Source: the owner's decisions of 2026-11-07 and 2026-11-08 ("trading calendar stays from pyside, refreshed daily with a diff"), the scheduler design `docs/design/scheduler.md` rev 8 (pull request 335 of andrewrong/optdesk, head b348fd7, not on main yet), the restart plan v2.5 rev 2.1 text (cited P:<line>), the compute design adversarial review, rev 1 of the four files (kept by the dispatcher, not committed; REV1:<finding>) and the review of rev 2 of the first draft (ADV:<line>).
Read on main at af7e583: `AGENTS.md`, `docs/design/modules.md`, `docs/deploy/pyside.md`, `internal/clock/api.go`, `internal/clock/calendar/calendar.go`, `internal/store/api.go`, `internal/store/setup/setup.go`, `migrations/0004_session_day_grant.sql`, `migrations/0005_session_day_loader.sql`, `internal/contracts/gen/records.go` (`SessionDay`), `pyside/src/optdesk/calendar/api.py`, `cli.py`, `__main__.py`, `pyside/pyproject.toml`, `cmd/ot/calendar.go`, `cmd/ot/runonce.go`, `internal/runner/reconcile/reconcile.go`.
One of four designs, the last: order core, figures, FRED, calendar. It lands after `compute-fred-series.md` (`calendar-jobs-api` edits `internal/jobs/api.go` and `jobs.md` after `series-history-loader`; `calendar-jobs-wire` edits `wire.go` and `ot.md` after `series-load-wire`) and, like them, after the scheduler design has merged (`sched-jobs-schedule`, `sched-cmd-run`, `sched-deploy-runbook` named below). Nothing in `compute-figures.md` waits for this file.

Goal: the trading calendar stays where the owner put it, in pyside; the scheduler's `calendar_refresh` job asks pyside's own generator for the next 400 days every session day, compares the answer with `master.session_day`, and writes only the days that differ, logging each one; no yearly hand load is needed.

## What main holds today

- The generator is `python -m optdesk.calendar --calendar XNYS --from DAY --to DAY --out FILE` (`pyside/src/optdesk/calendar/cli.py`, `__main__.py`): offline, no network, `SUPPORTED_CALENDARS = ("XNYS",)`; the file has a `provenance` header and one row per calendar day (the rules are the docstring of `api.py`). The library is `exchange_calendars>=4,<5` (`pyside/pyproject.toml:7`), a pinned static library. By modules.md the path `pyside/src/optdesk/calendar/**` is clock's (the clock row comes first).
- `ot calendar load FILE` (`cmd/ot/calendar.go`, package main) reads such a file, checks it and upserts it with the DSN line `OT_PG_DSN_LOADER` (`store.DSNEnvLoader`), the role `ot_loader`: SELECT and INSERT on `master.session_day` and UPDATE of its four columns (`migrations/0005`). The role `ot_runner` has only SELECT on it (`migrations/0004:1-3`). `run-once` and `backfill` open the session-day store with the runner DSN (`cmd/ot/runonce.go:114-120`) and their required `.env` lines are the six at `runonce.go:43` (no loader line).
- `internal/clock/api.go` has `SessionSource` (`SessionDays(ctx, calendarID, from, to) ([]contracts.SessionDay, error)`, line 35) and `NewCalendarFunc`. `store` has `SessionDayStore` (reader, writer and `Close`) opened by `OpenSessionDayFunc(ctx, dsn)`. `contracts.SessionDay` is the generated row. `.golangci.yml` excludes `internal/clock/` from `forbidigo`; `noctx` is on.
- pyside runs as a host service started with `uv run --no-sync --project pyside python -m optdesk.ibkr` (`docs/deploy/pyside.md:23-24`), so `uv` and the checkout exist on the host; the scheduler is also a host service (scheduler design Decision 2).
- The runner marks a key present when `JobResult.Usable > 0` and no error (`internal/runner/reconcile/reconcile.go:318-330`); an empty answer before the window closes is retried.

## Decisions

1. The source is the existing command line of pyside, not a gRPC service (REV1:K3, REV1:12). The first draft added `CalendarService` (a proto in `ibkr.v1`, a Python servicer, a second socket, a Go client, five issues, one owner-only merge). Compared with the owner's word, the simpler path keeps it: the calendar still comes from pyside, is refreshed daily and is diffed. It also keeps the module rules: the command is clock's own Python half (its paths are clock's), and Go reaches it through `clock.SessionSource`, a clock interface, so no module uses another through anything but its interface. It adds no socket, no generated tree and no `buf breaking` (the proto is untouched). Its cost is named: the host needs `uv` and the checkout as working directory, as the pyside unit does (`calendar-deploy-note`). The path of `uv` is a setting, not an assumption: the env line `OT_CALENDAR_UV`, default `uv` found on `PATH` (`docs/deploy/pyside.md` writes `<uv path>` because the host's path is the owner's). Exit codes: 1 (a deterministic `GenerateError` or an unwritable output, pyside `cli.py:60-72`) and 2 (usage, `cli.py:56`) are `Permanent`; only a failure to start or wait for the process is `Retryable`.
2. Departure from plan rev 2.1, named: the plan has the calendar generated once a year and cross-checked against IBKR TradingHours, with no calendar rpc (P:37-38, P:222, P:263). This design refreshes daily and has no TradingHours cross-check: the source is a pinned library, so an unscheduled closure is not found by anything here (REV1:K2); a cross-check is a later design. The diff is honest about what it will show: it is empty on a day when only the library's data did not change, but the span (today minus 7 to today plus 400) moves by one day each session day, so a new day is added daily, and a library bump (an early close added, a holiday moved) shows as changed days (REV1:12).
3. `calendar_refresh` runs once per session day (scheduler stage 1) over today minus 7 to today plus 400 days. Its key is `{master.session_day, "XNYS"}`. It reads the stored rows, asks the source for the fresh rows, takes the diff, upserts only added and changed rows, deletes none, and an empty fresh answer is `Permanent` (never overwrite with nothing); exit 1 and exit 2 of the command are `Permanent` and only a failure to start the process is `Retryable` (Decision 1). The job has `run_bound = "5m"` in its `[schedule]` table: the command is killed by its context at that deadline, and a killed process is a failure to wait, so `Retryable`. `Returned` and `Usable` are the number of fresh rows read and checked, so the runner marks the key present (reconcile.go:325). The diff is surfaced by one log line per changed day (`component=calendar_refresh`, day, kind added or changed, before and after), written through an observer function the wiring gives the job. A durable event or a table of changes needs a migration, a store and a page and is its own design (REV1:K1); the log is what the owner's word "with a diff" is met by here.
4. Postcondition: the stored rows cover every day from the as-of day to the span end, one row per calendar day (the generator's rule). It is false until the first refresh has succeeded, which is what a postcondition is for: the seed (`ot calendar load`, yearly) need not reach the span end, and on a no-change day after a success it is true (REV1:K5).
5. Who writes (REV1:4). The job gets a `store.SessionDayStore` opened with the line `OT_PG_DSN_LOADER`, the role `ot_loader` that already has the grant (`migrations/0005`); no migration, no grant to `ot_runner`, whose write access would be wider. This departs from the scheduler design, whose line 91 (rev 8) says writers are `ot_runner` for the ledger and the runs, and a job may open its own store with a role named in its design (calendar: `ot_loader`; compute: `ot_w_readings`), listed in the deploy runbook: the calendar job's store opens with the `ot_loader` DSN, which was named nowhere in rev 6 ("one writer role"); rev 8 amends that line. No narrower role exists: migration 0005 already makes `ot_loader` a role of `master.session_day` only. This is a second-opinion case for the window, with ONE brief shared with `compute-jobs-wire` (`compute-figures.md#Second opinion`: "the scheduler process holds the runner, readings and loader DSNs"): the process that runs jobs (`ot run-once`, `ot backfill`, `ot scheduler`) now holds the loader DSN, which until now only the person's `ot calendar load` used, so who may act with that permission changes. `calendar-jobs-wire` names the DSN and marks it. The two other session-day uses (the window rule and the calendar) keep the runner DSN.
6. `ot calendar load` stays, as the seed path and the manual path. Its reader (`readCalendarFile`, package main) and the new file parser of `calendarcli` read the same format twice; unifying them is a later cleanup, named under Rejected, not done here.

## Modules

Edits of `docs/design/modules.md` in this design's pull request: the list "one constructor each" gains `internal/clock/calendarcli` and `internal/jobs/calendarrefresh` (a hand merge with the same line edited by the other designs; `internal/clock/calendar` is already there). No other cell changes: pyside's depends is not touched.

| issue | module | order |
|---|---|---|
| clock-calendar-interface (interface) | clock | first |
| clock-diff | clock | after clock-calendar-interface |
| calendar-cli-source | clock, plus repo (judged: arch files) | after clock-calendar-interface |
| calendar-jobs-api (interface) | jobs | after clock-calendar-interface, `series-history-loader` (compute-fred-series.md; same files), `sched-jobs-schedule` |
| calendar-refresh-job | jobs, plus repo (judged: arch files) | after calendar-jobs-api, clock-diff, calendar-cli-source |
| calendar-jobs-wire | ot; second-opinion case | after calendar-refresh-job, `series-load-wire` (compute-fred-series.md), `sched-cmd-run` |
| calendar-deploy-note | docs | after calendar-jobs-wire, `sched-deploy-runbook` |

## Interface

- clock: `internal/clock/api.go` gains `SessionDayChange{Day contracts.Day; Before, After *contracts.SessionDay}`, `SessionDayDiffer` (`Diff(old, fresh []contracts.SessionDay) []SessionDayChange`; sorted by day; a day only in `fresh` has `Before` nil; a day only in `old` has `After` nil), `NewSessionDayDifferFunc func() SessionDayDiffer`, `CommandFunc func(ctx context.Context, argv []string) (exitCode int, err error)` (err: the command could not start or be waited for) and `NewCommandSessionSourceFunc func(argv []string, workDir string, run CommandFunc) SessionSource` (`argv` is the command prefix, `workDir` the directory for its temporary file).
- jobs: `internal/jobs/api.go` gains `CalendarObserver func(clock.SessionDayChange)` and `NewCalendarRefreshFunc func(clock.SessionSource, store.SessionDayStore, clock.SessionDayDiffer, clock.Clock, CalendarObserver) (contracts.Job, error)`.

## Issues

### clock-calendar-interface (interface)

- Module: clock. Size: S. Depends on: none.
- Order: first. Interface change: `internal/clock/api.go`.
- Design: docs/design/compute-calendar.md#Interface
- Files: internal/clock/api.go, internal/clock/api_test.go, docs/contracts/clock.md

- Goal: the five declarations of the Interface section and their text in the contract document; `api_test.go` tests the contract from both sides with a fake differ and a fake command.

Acceptance:

1. `grep -c "SessionDayDiffer\|SessionDayChange\|NewSessionDayDifferFunc\|CommandFunc\|NewCommandSessionSourceFunc" internal/clock/api.go` is at least 5 and `grep -c "^func " internal/clock/api.go` equals its count on main.
2. `go test ./internal/clock -race -count=1` passes; `grep -c "time.Sleep" internal/clock/api_test.go` prints 0.
3. `go list -deps ./internal/clock | grep optdesk` lists only `internal/contracts` and its `gen`.
4. `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the three paths of the Files line.

### clock-diff

- Module: clock. Size: S. Depends on: clock-calendar-interface.
- Order: after the interface.
- Design: docs/design/compute-calendar.md#Interface
- Files: internal/clock/calendar/diff.go, internal/clock/calendar/diff_test.go

- Goal: `calendar.NewDiffer` (`clock.NewSessionDayDifferFunc`): compares two slices by day; a changed row is any difference of `IsSession`, `EarlyClose`, `OpenAt` or `CloseAt`; sorted by day; a day only in the old slice is reported with `After` nil.

Acceptance:

1. `go test ./internal/clock/calendar -race -count=1` passes: `TestDiffAdded`, `TestDiffEarlyCloseChanged`, `TestDiffHolidayMoved` (two changes, sorted), `TestDiffOnlyInOld` (`After` nil), `TestDiffEqualIsEmpty`, `TestDiffUnsortedInput`.
2. `grep -c "time.Now\|time.Sleep" internal/clock/calendar/diff.go` prints 0; `go list -deps ./internal/clock/calendar | grep optdesk` lists only `internal/clock`, `internal/contracts` and its `gen`.
3. `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the two paths of the Files line.

### calendar-cli-source

- Module: clock, plus repo (judged: the regenerated arch files). Size: M. Depends on: clock-calendar-interface.
- Order: after the interface; parallel with `clock-diff`.
- Design: docs/design/compute-calendar.md#Decisions
- Files: internal/clock/calendarcli/source.go, internal/clock/calendarcli/command.go, internal/clock/calendarcli/source_test.go, internal/clock/calendarcli/command_test.go, internal/clock/calendarcli/testdata/calendar_xnys.json, arch.toml, .go-arch-lint.yml

- Goal: `calendarcli.New` (`clock.NewCommandSessionSourceFunc`): `SessionDays` makes a directory under `workDir` (`os.MkdirTemp`, removed on every path), runs `argv` plus `--calendar ID --from DAY --to DAY --out FILE` through `run`, reads and checks the file, returns the rows. Checks (all `Permanent`): the `provenance` header has its five strings, every row has the requested calendar id and a day in range, days are unique and ascending, one row per calendar day, a session day has `open_at` before `close_at`. Exit code 1 (a deterministic generator error) and exit code 2 (usage) are `Permanent`; only a failure to start or wait for the process (`CommandFunc` returns an error) is `Retryable`, and any other nonzero exit is `Permanent` too; no error text holds a path of the file's content. `command.go` is `calendarcli.Exec` (`exec.CommandContext`, output discarded), the real `CommandFunc`. `testdata/calendar_xnys.json` is a file written once by the real generator over 2026-11-02 to 2026-11-30 (the implementer runs `uv run --project pyside python -m optdesk.calendar ...` and says so in the pull request).

Acceptance:

1. `go test ./internal/clock/calendarcli -race -count=1` passes with a fake `CommandFunc` that copies the testdata file to the `--out` path of its argv (`t.TempDir()`, no network, no sleep): `TestSessionDaysParsesGeneratedFile`, `TestArgvHoldsCalendarFromToOut`, `TestExitCode1IsPermanent`, `TestExitCode2IsPermanent`, `TestOtherExitIsPermanent`, `TestStartFailureIsRetryable`, `TestMissingProvenancePermanent`, `TestDuplicateDayPermanent`, `TestWrongCalendarIdPermanent`, `TestTempDirRemoved`; `TestExecRunsCommand` re-executes the test binary as the child (the helper-process pattern) and reads its exit code 3.
2. `grep -c "time.Now\|time.Sleep" internal/clock/calendarcli/*.go` prints 0; `grep -c "CommandContext" internal/clock/calendarcli/command.go` prints 1; `go list -deps ./internal/clock/calendarcli | grep optdesk` lists only `internal/clock`, `internal/contracts` and its `gen`.
3. `git diff $(git merge-base HEAD origin/main) -- pyside` is empty; `go run ./scripts/arch && git diff --exit-code arch.toml .go-arch-lint.yml` is clean; each Go file is under 600 lines.
4. `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the seven paths of the Files line.

Checked by CI: the pyside workflow keeps the generator itself honest; the command through `uv` on the host is the integration the owner sees at the first scheduler day.

### calendar-jobs-api (interface)

- Module: jobs. Size: S. Depends on: clock-calendar-interface, series-history-loader (compute-fred-series.md), sched-jobs-schedule (scheduler design).
- Order: after all three (`api.go` and `jobs.md` are edited by the earlier designs, and `sched-jobs-schedule` asserts `api.go` unchanged in its own pull request). Interface change: `internal/jobs/api.go`, `docs/contracts/jobs.md`.
- Design: docs/design/compute-calendar.md#Interface
- Files: internal/jobs/api.go, internal/jobs/api_test.go, docs/contracts/jobs.md

- Goal: `CalendarObserver` and `NewCalendarRefreshFunc` of the Interface section and the document of the job: id `calendar_refresh`, key, span, the retry classes of Decision 3 (empty answer `Permanent`, source error from the command `Retryable` or `Permanent` as the source says), `Postcondition`, the observer line; `api_test.go` tests the contract from both sides with a fake job.

Acceptance:

1. `grep -c "CalendarObserver\|NewCalendarRefreshFunc" internal/jobs/api.go` is at least 2; `grep -c "CalendarRows" internal/jobs/api.go docs/contracts/jobs.md` prints 0.
2. `go test ./internal/jobs -race -count=1` passes; `go list -deps ./internal/broker/... | grep -c "internal/jobs"` prints 0.
3. `grep -c "calendar_refresh" docs/contracts/jobs.md` is at least 2.
4. `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the three paths of the Files line.

### calendar-refresh-job

- Module: jobs, plus repo (judged: the regenerated arch files). Size: M. Depends on: calendar-jobs-api, clock-diff, calendar-cli-source.
- Order: after all three.
- Design: docs/design/compute-calendar.md#Decisions
- Files: internal/jobs/calendarrefresh/job.go, internal/jobs/calendarrefresh/schedule.go, internal/jobs/calendarrefresh/job.toml, internal/jobs/calendarrefresh/job_test.go, arch.toml, .go-arch-lint.yml

- Goal: the job of Decisions 3 and 4 (`calendarrefresh.New`, `jobs.NewCalendarRefreshFunc`): `Plan` gives the one key; `Run` reads the stored and the fresh rows over today minus 7 to today plus 400 (the day from the injected clock), takes the diff, upserts only added and changed rows through the store, calls the observer once per change and deletes nothing. `schedule.go` and `job.toml` give the `[schedule]` table (proposed: calendar `XNYS`, anchor `session_open`, offset `-30m`, `run_bound` `5m`, `restart_sensitive` false) and the plain-value methods and `Postcondition` as `sched-runner-interface` and `sched-jobs-schedule` declare them (read from main when cut); the job imports no runner type.

Acceptance:

1. `go test ./internal/jobs/calendarrefresh -race -count=1` passes with fakes (source, store, differ, observer, fake clock): `TestUpsertsOnlyChanged`, `TestNoChangeWritesNothing` (`Usable` still above 0), `TestNeverDeletes` (a day only stored is kept), `TestObserverOncePerChange`, `TestEmptyFreshIsPermanent`, `TestSourceErrorKeepsItsClass`, `TestSpanFromClock` (today minus 7, plus 400), `TestPostconditionFalseUntilSpanEndStored` (false on a store seeded to the as-of day only, true after `Run`, true on a second no-change run), `TestScheduleValues` (read from `job.toml`; `RunBound()` is 5 minutes).
2. `grep -c "time.Now\|time.Sleep\|DELETE" internal/jobs/calendarrefresh/*.go` prints 0; `go list -deps ./internal/jobs/calendarrefresh | grep -c "calendarcli\|grpcbroker\|internal/runner"` prints 0 (it takes `clock.SessionSource`, not the adapter).
3. `go run ./scripts/arch && git diff --exit-code arch.toml .go-arch-lint.yml` is clean; each Go file is under 600 lines.
4. `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the six paths of the Files line.

### calendar-jobs-wire

- Module: ot. Size: S. Depends on: calendar-refresh-job, series-load-wire (compute-fred-series.md), sched-cmd-run (scheduler design; all edit `wire.go`, `runonce.go`, `ot.md`). Second-opinion case for the window: `run-once`, `backfill` and `ot scheduler` now hold the DSN of `ot_loader` (Decision 5).
- Order: last of the four designs.
- Design: docs/design/compute-calendar.md#Interface
- Brief: docs/design/compute-figures.md#Second opinion (ONE brief shared with `compute-jobs-wire`: "the scheduler process holds the runner, readings and loader DSNs").
- Second opinion: required; the advisor answers before the merge and the owner merges the pull request (the pull request waits for the owner's word).
- Advisor: pending (filled by the dispatcher)
- Files: cmd/ot/wire.go, cmd/ot/wire_test.go, cmd/ot/runonce.go, docs/contracts/ot.md

- Goal: `deps` gains `newCommandSessionSource` (`calendarcli.New`), `newDiffer` and `newCalendarRefresh`; `withWiring` opens a second session-day store with `store.DSNEnvLoader` and hands it to the job; the `.env` lines required by `run-once`, `backfill` and `ot scheduler` gain `OT_PG_DSN_LOADER` (the role exists since `ot setup pg` of week 1); the command is the `uv` path of the env line `OT_CALENDAR_UV` (default `uv` on `PATH`) followed by the arguments of `calendarrefresh/job.toml` (`run --no-sync --project pyside python -m optdesk.calendar`), the observer writes `a.log.Info("calendar change", ...)`; five jobs are wired (the four of `compute-jobs-wire` and `calendar_refresh`); `docs/contracts/ot.md` says five jobs and names the loader line.

Acceptance:

1. `go test ./cmd/ot -race -count=1` passes: `TestRunOnceWiresFiveJobs`, `TestRunOnceNeedsLoaderDSN` (the error names the line, never its value), `TestCalendarJobGetsLoaderStore` (the store opened with the loader DSN is the one handed to the job; the window store keeps the runner DSN), `TestCalendarChangeIsLogged`, `TestCalendarUvPathSetting` (the line sets the program of the argv; absent gives `uv`), `TestDemoProfileOpensNoStore`.
2. `grep -c "five jobs" docs/contracts/ot.md` is at least 1 and `grep -c "four jobs" docs/contracts/ot.md` prints 0; `grep -c "OT_PG_DSN_LOADER" docs/contracts/ot.md` is at least 1.
3. `git diff $(git merge-base HEAD origin/main) -- migrations internal/store/setup` is empty (no grant, no role is changed).
4. `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the four paths of the Files line.

### calendar-deploy-note

- Module: docs. Size: S. Depends on: calendar-jobs-wire, sched-deploy-runbook (scheduler design).
- Order: after both; the owner confirms the wording (it states a deployment need).
- Design: docs/design/compute-calendar.md#Decisions
- Files: docs/deploy/scheduler.md

- Goal: one section: the scheduler's `calendar_refresh` runs `uv` from the working directory of the scheduler, so `uv` is on `PATH` and the working directory is the checkout root that holds `pyside/` (the need of the pyside unit, `docs/deploy/pyside.md:23-24`); the loader DSN line `OT_PG_DSN_LOADER` and the `OT_CALENDAR_UV` line are in `.env` (this section lists them for the runbook; the wire issue lists them in `ot.md`); what a failed command looks like (a retryable run; the next session day's refresh covers the span again).

Acceptance:

1. `grep -c "calendar_refresh" docs/deploy/scheduler.md` is at least 2 and `grep -c "OT_PG_DSN_LOADER" docs/deploy/scheduler.md` is at least 1.
2. `go run ./scripts/hygiene` prints `hygiene: ok`; `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the one path of the Files line.

## Left out

- A durable event or table of calendar changes (a migration, a store, a page): a later design; the log line is what this one gives (Decision 3).
- A daily run on days that are not session days (a holiday moved to a weekday is found on the next session day; scheduler stage 2); calendars other than XNYS (the generator supports one); the IBKR TradingHours cross-check (Decision 2).
- Rejected: the `CalendarService` rpc, its proto, servicer, second socket and Go client (Decision 1); registering anything in `build_server` (owner-only); a Go calendar library; the first draft's `jobs.CalendarRows` row type; the yearly hand load as the only path; unifying `readCalendarFile` of `cmd/ot` with the parser of `calendarcli` now (Decision 6).
- Not changed: `ot calendar load`, `master.session_day`, any grant, any migration, pyside.

## Adversarial

Read: the compute design adversarial review, rev 1 of the four files (kept by the dispatcher, not committed; REV1). Older answers (ADV, rev 2 of the first draft: no source picked, session-day slots, no trigger or change table, `clock.SessionSource`, one `Design:` line, regenerated arch files) stand as in rev 1 of this file. Adopted means the text above changed.

- REV1:4 (`ot_runner` has only SELECT; first live run is "permission denied"; no loader DSN in `run-once`): adopted, Decision 5 and `calendar-jobs-wire`: the role is `ot_loader` (`migrations/0005`), the DSN line `OT_PG_DSN_LOADER`, wired in `withWiring` by that issue in the `ot` module; no grant and no migration (a grant to `ot_runner` would be wider); marked a second-opinion case because the job-running process now holds that DSN. A test (`TestCalendarJobGetsLoaderStore`) sees the wiring offline; the permission itself is only proved by the first live day.
- REV1:9 (conditional `Postcondition`, signature in an unmerged design): adopted. The job depends on `sched-jobs-schedule` and takes the declarations of `sched-runner-interface` from main when cut; `TestPostconditionFalseUntilSpanEndStored` is unconditional.
- REV1:12 (a daily refresh of a pinned library gives an empty diff; no IBKR cross-check): adopted as honesty, not as removal: Decision 2 says what the diff shows (a new day daily, library bumps as changes) and names the dropped cross-check as a departure from the plan; the owner's word "daily" is kept.
- REV1:15 (the one-line list of modules.md, textual order): not removable; named in Modules.
- REV1:K1 (the diff is only logged): adopted in part. The diff is logged one line per day through an observer and the run row shows the key present; a durable event is Left out because it needs a migration, a store and a page.
- REV1:K2 (role problem; static library; second socket undocumented): the role problem adopted (REV1:4); the static library named (Decision 2); the second socket no longer exists.
- REV1:K3 (simpler way: run the existing CLI through the existing load path): adopted with a change. The scheduler's job runs the pyside CLI (`calendar-cli-source`) and writes through the job's own store, not through `ot calendar load` (a `package main` function a job cannot call); five issues, two generated trees, an owner-only merge and a socket are gone; two issues are added (`calendar-cli-source`, `calendar-deploy-note`) and the interface issues of the differ and the job constructor moved in from `compute-figures.md`. Weekly instead of daily: not adopted, the owner said daily.
- REV1:K4 (lock-in: public `CalendarService`, a second socket, `default_builder` outside `api.py`): gone with the service; the new public surface is two Go constructor types in clock and one in jobs.
- REV1:K5 (the postcondition is false on day one): the postcondition is kept and explained (Decision 4): false until the first refresh succeeds, which is correct; with the write path fixed (REV1:4) it becomes true and stays true on a no-change day.
- REV1:K6 (`calendar-proto` owner-only, `buf breaking`; `calendar-pyside-service` and `server.py`; the "four jobs" grep): moot: those issues are gone; the "four jobs" grep is now `grep -c "four jobs" ... prints 0` after `compute-jobs-wire` set it to four (the order is stated).
- Advisor conditions of 2026-11-08 (rev 3): `run_bound` 5m, exit mapping and the `uv` setting (Decisions 1 and 3); the departure from scheduler.md line 91 named and that line amended in rev 8 of the scheduler design (Decision 5); the loader and readings DSN lines are listed by `calendar-jobs-wire` and `compute-jobs-wire` in `ot.md` and by `calendar-deploy-note` in the runbook; the two wire issues share one brief.
