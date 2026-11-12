# Single runner: only the scheduler runs jobs; no database lease; restart closes running rows

Author: the optdesk window (draft)
Owner approved: (pending)
Date: 2026-11-12
Source: owner 2026-11-12 15:39Z (optdesk-3414): only the scheduler process runs jobs; `ot run-once` and `ot backfill` only enqueue a console request; the database lease goes; on start the scheduler closes every `running` row as failed `restart`.

## Goal

Today three processes can run a job: the scheduler, `ot run-once` (`reconcileDay`, `Reconcile`) and `ot backfill` (`Backfill`), so `ops.run_lease` (`ledger.go` `acquireSQL`, `reconcile.go` `leaseTTL`, `holdLease`, `backfillOwner`) and the `LeaseUntil` liveness of `ops.capture_run` rows (`runkey.go`, `nextAttempt`, `closeExpired`, `sched/gaps.go`) keep them apart, and a killed run blocks for `leaseTTL`. After this design one process runs jobs (the table `ops.run_lease` and the column `lease_until` stay, unused; Left out). The two commands post a request to the job console queue and may wait for its outcome. A `running` row is a live run only while this process holds the try-lock of its job; any other `running` row is closed as crashed (Decisions 4 and 5). Same-job overlap stays refused by the in-process per-job try-lock (#1915, `console-run-lock.md` Decision 2).

## Modules

- store (`internal/store/**`, `migrations/**`): a new small interface `store.RunCloser` (`CloseRunning`), implemented by the ops ledger; the write of a run no longer reopens a closed row or replaces its finish; migration 0039 drops the CHECK that a running row has `lease_until`.
- runner (`internal/runner/**`): `DayPlanner.PlanDay` (plan only); no lease taken or read; a `running` row of a job whose in-process try-lock is free is crashed.
- ot (`cmd/**`): the scheduler closes `running` rows after it holds the fence; the console host plans an unplanned day; `run-once` and `backfill` are enqueue wrappers.
- docs (`docs/**`): the deploy runbook text (entry 6).

No row of `docs/design/modules.md` changes.

## Interface

Two interface changes, each its own issue, and `OpsLedger` is not touched, so no fake of it breaks and main stays green between entries. Issue 1, `internal/store/api.go` (owner store):

    // RunCloser is the optional method of the OpsLedger value the scheduler is given (asserted like runner.ConsoleClaimer).
    // CloseRunning closes every running run as done with ReasonCode reason and FinishedAt now; it returns the count.
    type RunCloser interface {
        CloseRunning(ctx context.Context, now time.Time, reason string) (closed int, err error)
    }

`api.go` is 800 lines on main and has no ceiling in `docs/design/ratchet.json`: issue 1 shortens existing doc comments by at least as many lines as the declaration adds, so `wc -l internal/store/api.go` is at most 800 after it. `Acquire`, `Release`, `RenewLease`, `Lease` and `Run.LeaseUntil` stay declared and are called by no code after issue 7 (the `cmd/ot` callers go in issue 5, the runner callers in issue 7). `jobrequest.Poster` and `Reader` (`Enqueue`, `Get`) are used as they are.

Issue 2, `internal/runner/api.go` (owner runner): declares

    // DayPlanner is the optional method of the backfiller NewReconcilerFunc returns (asserted like ConsoleClaimer).
    // PlanDay stores the plan and windows of one day; it runs no job and moves no cursor.
    type DayPlanner interface {
        PlanDay(ctx context.Context, day contracts.Day) error
    }

`Reconciler` and `ForceBackfiller` are unchanged. The console host calls `PlanDay` and `BackfillWith` and never `Reconcile`.

## Decisions

1. Only `ot scheduler` runs jobs. `ot serve` posts, the scheduler's console claims. `run-once` without `--dry-run` and `backfill` build no wiring, open no ledger and call no `Reconcile` or `Backfill` (grep of `cmd/ot`: `reconcileDay` in `runonce.go` and `backfill` in `backfill.go` are the only non-console callers; `--dry-run` writes nothing, takes no lease and stays).
2. A wrapper posts through `jobrequest.Poster` (the `ot_w_settings` role DSN, as `jobconsole_post.go`): `backfill --day D --job J [--force]` posts `{J, D, Force}`; `run-once [--day D] [--job J]` posts one request per job of the console job list (`jobconsole_jobs.go`) with kind `any_day` or `today_only` when `--job` is absent; `listed_only` jobs are skipped, and a `today_only` job is skipped when D is not today in its zone (the post would be refused). With `--job` the post is made and the page's own refusal is printed. `--dataset` is removed (a request has no dataset). `--wait` polls `Reader.Get` on the injected clock until the state is not queued or running, prints the key lines and exits non-zero for `failed`, `refused` and `interrupted`; without it the command prints the request id and the `EnqueueOutcome` and exits. `--wait` gives up after `--timeout` (default 120 s on the injected clock), prints `request N is still queued: the console may be off (no consumer is running)` and exits non-zero; the console is off when `openConsoleHost` returns nil, and nothing then claims a request.
3. No lease use. `holdLease`, `leaseTTL`, `backfillOwner`, the 30 s renewal and the calls of `Acquire`, `Release`, `RenewLease` are deleted from the runner and `cmd/ot`. The in-process try-lock of #1915 is the only run exclusion. The table and column stay (Left out).
4. No `LeaseUntil` reading. A `running` row is a live run only while this process holds the in-process try-lock of its job (#1915): the lock is held for the whole of a run, and a run of the scheduler or of the console is the only thing in this process that writes a live `running` row. So a `running` row of a job whose try-lock this process can take (or already holds, under `lockedTurn`) was not started by this process and is crashed. `nextAttempt`, called under the job's lock, treats every `running` row as the lapsed one is treated today: returned as `crashed`, not counted, closed by `markCrashed` at that tick of the key, and the key goes on by the same rule; it takes no clock for it. `closeExpired`, which the two backfill paths call before they take the lock, becomes `closeStale(ctx, jobID, runs)`: with no `running` row it returns false; else it tries the job's lock, and when a run of this process holds the lock it returns live true (the caller reports the key as missing or `lease_held`, as today), and when it takes the lock it closes each `running` row with `markCrashed`, releases the lock and returns false. The lapsed-lease crash count in `sched/gaps.go` is deleted (the gap view counts a `running` row as running). A new running row is written with a zero `LeaseUntil` (NULL), which the CHECK of `0003` forbids until 0039 is applied.
5. Restart closes at once, as crashed. After the fence is held and before the ticker's first turn, `cmd/ot/scheduler.go` asserts the ops ledger to `store.RunCloser` and calls `CloseRunning(ctx, now, "crashed")` and logs the count. The reason is `crashed`, the one the lapsed-lease path writes today, so `isCrash`, `nextAttempt`, `closeRow` and `sched/gaps.go` treat the row as they do now (a crash: not counted toward the two-run cap, a new attempt is planned). A deploy mid-run therefore counts toward `crashBrake` (3) exactly as a lapsed lease does today; nothing is added. If the ledger is not a `RunCloser` or `CloseRunning` returns an error the scheduler does not start ticking: `schedulerCmd` returns the error and exits non-zero (the keeper restarts it).
6. The previous process is gone when the fence is held, with the one exception below. The fence is `pg_try_advisory_lock(fenceKey)` on one dedicated connection (`ops.OpenFence`); a session advisory lock is freed when the backend ends the session: at once when the client process exits (the socket closes and the backend sees EOF), or when the server terminates the backend (`pg_terminate_backend`, server restart, keepalive timeout) with the client still alive. The service is restarted by crew (`docs/deploy/scheduler.md`: the keeper starts the new process after the old one exits; the old one drains on SIGTERM or exits 75), and the repository holds no code that starts a second process before the first has exited, so no grace period is added: a start while the old process still holds its connection does not get the lock: `lockFence` retries every `lockEvery` for `lockWait` (60 s) and then exits with the error, which is the wait for the old process to end. For the exception (backend ended, old client alive for the moment before `fence.Lost()` reaches `watchFence`) the old process may insert a `running` row after the new process ran `CloseRunning`. No guard is put on the insert: that row belongs to no run of the new process, so Decision 4 closes it as crashed at the next tick that reads that key. The final write of the old run is conditional: `recordRunSQL` ends its `ON CONFLICT ... DO UPDATE` with a guard (entry 3(b)), so a row closed by `CloseRunning` or `markCrashed` is never reopened or given a later finish by the old writer, while the deliberate rewrite of a closed row by `expireIfClosed` (same status, same finish, a new `ReasonCode`) still goes through (entry 3(b)). A file lock, a pid file or an insert guard on `pg_locks` was not chosen: the first two are per host and add a file, the guard needs the backend pid to travel from the fence to the ledger, which `cmd/ot` opens apart.
7. A day nobody planned. A console request is `BackfillWith`, which needs a stored plan and else is refused `no_plan`; `run-once` was `Reconcile`, which plans and also runs every due job and moves the cursor. The console must do only the plan, because its claim covers one job and the ticker's `Reconcile` is no longer serialised by a lease. So there is a plan-only method, `runner.DayPlanner.PlanDay` (entry 2 declares it, entry 4 implements it: plan, `PutExpected`, windows; no run, no cursor). The console host receives the ops ledger as a new constructor argument: `newConsoleHost(q, syncer, ledger, clk, log)` with `ledger store.OpsLedger`, and `openConsoleHost` (`cmd/ot/jobconsole_sched.go`), which already has `ops`, passes it. The console host (`cmd/ot/jobconsole_turn.go`, entry 5), before it runs a claimed request, reads `OpsLedger.Expected(day)` and, when it is empty and the day is after `OpsLedger.Cursor` (an empty `Cursor.Day`, no cursor yet, counts as before the day, so the console plans it; the case `run-once` accepted; it refused a day at or before the cursor), asserts its `ForceBackfiller` to `runner.DayPlanner` and calls `PlanDay(ctx, day)`; then it runs the one request under the lock of its job. Without a `DayPlanner`, or for a day at or before the cursor, the `no_plan` refusal stands, as `ot backfill` is today. The cursor moves only by the ticker's `Reconcile`.
8. A run in flight when the fence is lost stops by `cancelConsole()` as today; its row stays `running` and is closed by the next start.
9. Deploy order. Code deploys on merge and migrations hold: entry 3 merges, then the window runs `ot migrate` (0039) before entry 7 merges. Entry 5 (wrappers, plan-only console, close at start) lands before entry 7 removes the lease, so at no time is a job run unguarded: until entry 7 the lease still works, and entry 5 only reduces the callers. #1915 (reconcile files) is ordered before 4 and 7, #1918 (`cmd/ot` console files, `scheduler.go`) before 5; #1919 (jobs) does not overlap.

## Issues

1. (interface) store: `RunCloser` declared.
   Files: `internal/store/api.go`, `internal/store/api_test.go`
   Order: none.
   Acceptance: (a) `api.go` declares `RunCloser` with the doc of the Interface section and `OpsLedger` is unchanged, so every existing fake still satisfies it; (b) `api.go` has at most 800 lines, the doc comments shortened carry no change of meaning and no declaration is removed; (c) the contract test of `api_test.go` names `RunCloser` from both sides (a caller and a fake).

2. (interface) runner: `DayPlanner` declared.
   Files: `internal/runner/api.go`, `internal/runner/api_test.go`
   Order: none.
   Acceptance: (a) `api.go` declares `type DayPlanner interface { PlanDay(ctx context.Context, day contracts.Day) error }` with a doc saying it stores the plan and windows of one day, runs no job and moves no cursor, and is an optional method of the value `NewReconcilerFunc` returns as backfiller (asserted like `ConsoleClaimer`); (b) `Reconciler` and `ForceBackfiller` are unchanged; (c) the contract test of `api_test.go` names `DayPlanner` from both sides (a caller and a fake).

3. store ops: `RunCloser` implemented, closed rows kept closed, migration 0039.
   Files: `internal/store/ops/ledger.go`, `internal/store/ops/ledger_test.go`, `internal/store/ops/ledger_pg_test.go`, `migrations/0039_drop_run_lease_check.sql`
   Order: after 1.
   Acceptance: (a) `CloseRunning` is one `UPDATE ops.capture_run SET status='done', finished_at=$1, reason_code=$2 WHERE status='running'` returning `RowsAffected`; (a2) the value `ops.Open` returns is asserted to `store.RunCloser` in a compile-time check; (b) `recordRunSQL` ends its `DO UPDATE` with `WHERE ops.capture_run.status = 'running' OR (EXCLUDED.status = ops.capture_run.status AND EXCLUDED.finished_at IS NOT DISTINCT FROM ops.capture_run.finished_at)`: a closed row is changed only by a rewrite that keeps its status and finish (the reason rewrite of `expireIfClosed`), never by a write that reopens it or finishes it later; (c) `0039_drop_run_lease_check.sql` drops `CHECK (status <> 'running' OR lease_until IS NOT NULL)` of `0003`, its down part re-adds it and applies only while no `running` row has a NULL `lease_until`; (d) `ledger_pg_test.go` (tag `pg`) shows two running rows closed, a done row unchanged; (e) the same test shows a `RecordRun` of a closed row with the same status and finish and a new reason (`crashed_repeatedly`) changing the reason, and a `RecordRun` of status `running`, or of status `done` with a later finish, leaving the row as `CloseRunning` wrote it.

4. runner: `PlanDay` implemented.
   Files: `internal/runner/reconcile/planday.go` (new), `internal/runner/reconcile/planday_test.go` (new)
   Order: after 2.
   The issue also waits for issue #1915 (per-job try-lock) to merge.
   Acceptance: (a) the reconciler type of `reconcile` implements `runner.DayPlanner` (compile-time check); (b) `PlanDay` obtains the day's sessions as `Reconcile` does for one day, calls `plan`, `PutExpected` when the stored hash differs and `putWindows`, exactly as `reconcileOn` does before `Present`, and then returns: it calls no `runJobs`, no `SetCursor` and takes no lease; (c) a test with a fake ledger and two jobs shows an unplanned day gets its expected keys and windows stored, no job `Run` was called and the cursor is unchanged; (d) a second call with an unchanged plan writes no `PutExpected`.

5. ot: the scheduler closes `running` rows at start; `run-once` and `backfill` enqueue.
   Files: `cmd/ot/scheduler.go`, `cmd/ot/jobconsole_turn.go`, `cmd/ot/jobconsole_turn_test.go`, `cmd/ot/jobconsole_host.go`, `cmd/ot/jobconsole_sched.go`, `cmd/ot/runonce.go`, `cmd/ot/backfill.go`, `cmd/ot/enqueue_wait.go` (new), `cmd/ot/backfill_report.go`, `cmd/ot/scheduler_test.go`, `cmd/ot/runonce_test.go`, `cmd/ot/runonce_option_test.go`, `cmd/ot/runonce_lease_test.go` (deleted), `cmd/ot/backfill_test.go`, `cmd/ot/backfill_report_test.go`, `cmd/ot/enqueue_wait_test.go` (new), `docs/contracts/ot.md`
   Order: after 1, 3, 4.
   The issue also waits for issue #1918 to merge.
   Acceptance: (a) in `scheduler.go` the assertion to `store.RunCloser` and the call `CloseRunning(ctx, now, "crashed")` follow the successful `inst.TryLock` and precedes the first ticker turn, is not reachable when `TryLock` failed, and its error is returned by `schedulerCmd` before the ticker is built; (b) `runonce.go` and `backfill.go` (non-dry-run path) call none of `newReconciler`, `Reconcile`, `Backfill`, `openSchedulerLedger`, `withWiring`; `leaseRefusal` and `reconcileDay` are gone; (c) both commands call `Poster.Enqueue` with `{JobID, Day, Force}` and print the request id and the `EnqueueOutcome`; (d) `--wait` uses the injected clock (no `time.Sleep`) and the exit status follows Decision 2; (e) `--dataset` is rejected as an unknown flag; (f) `newConsoleHost` takes the ledger as in Decision 7 and `openConsoleHost` passes `ops`; `jobconsole_turn.go` calls `Expected(day)` and, for an empty plan on a day after `Cursor`, asserts the `ForceBackfiller` to `runner.DayPlanner` and calls `PlanDay(ctx, day)` once before `BackfillWith`, and calls no `Reconcile`; for a day at or before the cursor, or without a `DayPlanner`, the `no_plan` refusal stands ((new) test: a console turn on an unplanned day stores the plan and runs only the requested job, with a fake planner and backfiller recording their calls; (new) test: a day at or before the cursor is refused `no_plan` and `PlanDay` is not called); (g) the enqueue-all of `run-once` skips `listed_only` jobs and `today_only` jobs for another day, with a test per kind; (h) `--wait` past `--timeout` prints the console-off line and exits non-zero (test with the fake clock); (i) tests use a fake `Poster` and `Reader`; (j) `runonce.go` stays under its present line count.

6. docs: the deploy runbook states the new rule.
   Files: `docs/deploy/scheduler.md`, `docs/deploy/index-iv-history.md`
   Order: after 5.
   Acceptance: (a) `scheduler.md` states that the instance fence is the one-process guarantee (retried for 60 s at start), that a start closes running rows as `crashed`, and that `run-once` and `backfill` only enqueue and need the console on; (b) its crash-recovery text no longer says a lapsed lease marks a run `crashed`; (c) `index-iv-history.md` no longer says `run-once` runs synchronously.


7. runner: no lease, a `running` row of a free job lock is crashed.
   Files: `internal/runner/reconcile/reconcile.go`, `internal/runner/reconcile/backfill.go`, `internal/runner/reconcile/backfill_force.go`, `internal/runner/reconcile/runkey.go`, `internal/runner/reconcile/fakes_test.go`, `internal/runner/reconcile/reconcile_test.go`, `internal/runner/reconcile/backfill_test.go`, `internal/runner/reconcile/backfill_force_test.go`, `internal/runner/reconcile/backfill_window_test.go`, `internal/runner/reconcile/runkey_test.go`, `internal/runner/reconcile/count_test.go`, `internal/runner/sched/gaps.go`, `internal/runner/sched/gaps_test.go`, `internal/runner/sched/gaps_stop_test.go`, `internal/runner/sched/ticker_restart_test.go`, `docs/contracts/runner.md`
   Order: after 5.
   The issue also waits for issue #1915 to merge, and migration 0039 is applied first (Decision 9).
   Acceptance: (a) `reconcile.go`, `backfill.go`, `backfill_force.go` and `runkey.go` contain none of `holdLease`, `leaseTTL`, `backfillOwner`, `closeExpired`, `RenewLease`, `Acquire`, `Release`, `LeaseUntil`; (b) `nextAttempt` returns a `running` row as `crashed` (never as pending) and takes no clock for it, and `closeStale` is the one function the two backfill paths call in place of `closeExpired`, as Decision 4; (b2) a test with the fake ledger and the real job locks shows: a `running` row of a job whose lock is free is closed `crashed` at the job's turn of `Reconcile` and the key then runs a new attempt, and the same for `Backfill` and `BackfillWith`; a `running` row of a job whose lock the test holds is not closed and the key is reported missing (`lease_held`); no PostgreSQL server is needed; (c) `sched/gaps.go` counts a `running` row as running, never as crashed by lease; (d) the reason `crashed` is still written by no runner code and read by `isCrash` for rows `CloseRunning` and old runs wrote; (e) every removed test is replaced by one that fails when the code of (b) or (c) is reverted; (f) `docs/contracts/runner.md` names no lease use.

## Left out

- Dropping `ops.run_lease`, `lease_until`, `Acquire`, `Release`, `RenewLease`, `Lease` and `Run.LeaseUntil` (kept to avoid a second migration, an old-binary/new-schema window and about 16 files; a later cleanup issue may remove them).
- `--dataset` of `backfill` (a console request carries a job and a day only).
- `run-once --dry-run` and `probe` (read the broker, write nothing, take no lease).
- A console request left `running` at restart: owned by the console (`reasonRestart`, `jobconsole_host.go`), unchanged.
- Other `cmd/ot` commands that build jobs (`candidates`, `series-load`, `ledger-import`, `account-import`, `underlying`, `compare-figures`) call no `Reconcile` or `Backfill`.
- A pid file or OS file lock (Decision 6).

## Adversarial

Findings: the dispatcher scratchpad file single-runner-adversarial.md (not in the repository).

1. Adopted: reason `crashed`, deploys count toward the brake as lapsed leases do today, a `CloseRunning` error stops the start (Decision 5).
2. Adopted: ot lands before runner; 0039 applied before entry 7; one issue not possible, runner and ot are two modules (Decision 9, Order of 7).
3. Adopted: lease table and column kept; no 0040, no removal entry (Left out).
4. Adopted by 3 (no data lock-in); the CLI contract change (`--dataset` gone, async enqueue) is stated in Decision 2 and entry 5(e).
5. Adopted: exact release of the lock, no grace period because crew starts the new process after the old one exits, conditional final write in entry 3(b), (e); a late `running` row of the old process is closed as crashed by Decision 4 (the `pg_locks` insert guard of an earlier round is removed).
6. Adopted: entry 1(b) keeps `api.go` at 800 lines or fewer.

Review r1 (advisor notes, not in the repository): (1) adopted, guard of entry 3(b), tests 3(d), (e); (2) adopted, `store.RunCloser`; (3) adopted, entry 5; (4) adopted, Decision 7 and entry 5(f), (g); notes: `lockWait` in Decision 6, console-off `--wait` timeout in Decision 2 and entry 5(h).

Review r2: one blocking point, adopted: the console turn only plans (`DayPlanner.PlanDay`, entries 2 and 4, Decision 7); acceptance 4(c) and 5(f) test that an unplanned day gets its plan stored and only the requested job runs.

## Checked by CI

- `go build ./...`, `go vet`, `golangci-lint`, `go test ./internal/store/... ./internal/runner/... ./cmd/ot/...` (parallel, fake clock).
- The `pg`-tagged tests of `internal/store/ops` in GitHub CI, including migration 0039 up and down.
- The contract tests `internal/store/api_test.go` and `internal/runner/api_test.go`.
