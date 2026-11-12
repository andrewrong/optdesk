# Console run lock: two runs of one job never overlap, in memory; a console run takes no database lease

Author: the optdesk window (draft)
Owner approved: (pending)
Date: 2026-11-12
Source: owner 2026-11-12 09:48Z: every job of the scheduler can be re-run by hand; two runs of the same job must not run at the same time; the whole-process lock only prevents another process and does not control re-runs between tasks; no queueing.

## Goal

Today a console run is refused `lease_held` whenever `ops.run_lease` is held under another owner: `BackfillWith` takes it under a random owner `backfill:<hex>` (`internal/runner/reconcile/backfill_force.go`, `holdLease`), the scheduler tick under `runner` (`reconcile.go`), and a console run killed by a deploy leaves its owner holding the lease for `leaseTTL`. The console turn also runs on the one worker of `internal/runner/sched/ticker.go` (`runTurn`), and a request is held while a slot start is near (`SlotHold`, `slot_near`): the console waits for a tick and for a slot, which is a queue. `Reconcile` also walks the keys of all jobs one after another, so a long job makes the others miss their slot (owner 09:55Z: "queuing makes jobs miss their time; jobs are independent"). After this design a console run takes no lease, scheduled runs of different jobs run in parallel, a run is refused only by a run of the same job, and nothing waits.

## Modules

- runner (`internal/runner/**`): `reconcile` (the per-job lock, a goroutine per job in `Reconcile`, no lease in `BackfillWith`), `sched` (the console loop off the tick worker).
- jobs (`internal/jobs/**`): four jobs declare their upstream (Decision 6).
- ot (`cmd/**`): the console host starts a goroutine per claimed request, stops setting `Hold`, and maps the new reason.
- pages (`internal/pages/**`): the text of the new reason.

## Interface

One interface file changes, `internal/runner/api.go`: a new optional method of a job, beside `Scheduled` and `Bounded` (the places where a job gives the scheduler its schedule and run bound today; `job.toml` is read by each job package with no shared schema, so a toml key would need a parse in every package):

    type Dependent interface { After() []string } // ids of the jobs of the same day that this job reads

`BackfillOptions.Hold` and `SlotHold` stay in `api.go` unused by the console (a later design may delete them); the doc comment of `ConsoleTurner` ("runs on the worker, so it never overlaps a Reconcile") is left as it is and is stale until then, which no code reads. The refusal is a `KeyRun` of group `GroupLeaseHeld` with `ReasonCode` `job_running`, a free string. The interface issue lands before the issues that use it.

## Decisions

1. `BackfillWith` takes no lease. The process exclusion of serve is the scheduler fence (`ticker.watchFence`), the exclusion between runs is Decision 2, and the live-run check of a key (`closeExpired`) stays. `Reconcile` keeps `holdLease` under the owner `runner` as today, and `Backfill` (the bare call of `ot backfill`) keeps its own `backfill:<hex>` lease, so `ot run-once` and the backfill command behave as before. `ops.run_lease` is not shared and not counted: no change to `Acquire`, `Release` or the renewal.
2. One in-memory try-lock per job id, held in the `reconcile` package around every job run: `bareRun` (forced re-run, no run row), `runMissingKey` (console run of a missing key) and the key run of `Reconcile`. A busy job is not run and not waited for. `BackfillWith` writes a `lease_held` line with `ReasonCode` `job_running`; `Reconcile` leaves the key for the next tick. Per job, not per job and key: "two runs of the same job". The lock is a field of the reconciler, so Reconcile and `BackfillWith` of one process share it.
3. The slot hold is removed from the console path: `cmd/ot` stops setting `BackfillOptions.Hold` and the request is no longer held for a slot (`turn` no longer calls `h.q.Hold`; `slot_near` is no longer produced). The per-job lock replaces it.
4. The console loop runs off the tick worker. The trigger is the ticker loop's own console turn (`submit(turn{console: ...})`, `ticker.go`): it now calls `ConsoleTurn` on the loop goroutine, which only reads the queue and claims requests, and starts one goroutine per claimed request. Those goroutines are drained in `Run` with `drainGrace` and `killGrace`, and their context is cancelled on fence loss, so no job runs after the fence is lost. A request goroutine calls nothing owned by the worker: not `refresh`, not the `disabled` map; it reads the job set the console host was given. Different jobs run at once; the same job is refused by Decision 2.
5. Scheduled runs are parallel too. `Reconcile` starts the keys of each due job in a goroutine of its own, one per job, and waits for them before it moves the cursor; the keys of one job still run in order inside its goroutine. The only guard is Decision 2: a job that is already running (a console run, or the previous tick that has not ended) is skipped for this tick and the next tick retries it. `Reconcile` keeps `holdLease` once for the whole call under the owner `runner`, not per job, and its renewal runs until every job goroutine has returned.
6. Dependencies are declared and run by the scheduler, not by waiting. A job gives its upstream jobs by `After() []string` (`runner.Dependent`), next to its `Schedule()` method. When a run of an upstream job for day D ends ok (a `done` run, or a forced re-run that returned without error), `reconcile` starts each dependent of that job for D in a goroutine of its own, at once, under the per-job lock of Decision 2; a dependent that is running is skipped. A dependent's own dependents follow the same way when it ends ok. Unrelated jobs never wait for each other. If the upstream has not ended ok by the dependent's own slot, the dependent runs at its slot on what is stored, as today (the fallback, not a cascade: a failed upstream never blocks a dependent). A console re-run of an upstream job triggers its dependents too, with the same force as the console request. The pairs, from the slots in each `job.toml` and the job's own reads:
   - `option_open_plan` -> `option_open`; `option_open_plan_jp` -> `option_open_jp`.
   - `option_eod`, `option_eod_jp`, `quote_snapshot` -> `compute_figures` -> `option_candidates`.
   - `account_capture` -> `position_marks` (the manifest of `position_marks` says it is placed after the account job).
   Only an upstream's completion triggers dependents, never the reverse: a run of a dependent, from the console or at its slot, never starts its upstream (owner 09:59Z). Before it runs, `reconcile` reads the run ledger (`ledger.Runs` of the day) for each id in `After()`; an upstream with no ok run for D is a missing upstream. The dependent still runs on what is stored, with no refusal and no wait, and its key line carries `ReasonCode` `upstream_missing` (set only when the run left that code empty; the outcome and the group of the line are unchanged) naming the upstream ids in the log line of the run (`slog` attribute `upstream`). The mark is written where the line is made: `bareRun`, `runMissingKey` and the key run of `Reconcile` in `reconcile`.
   Other pairs are added the same way by a later issue; none is guessed here.
7. `cmd/ot` `classify`: a result with no `ran` or `rerun` line and at least one `lease_held` line of reason `job_running` ends the request `refused` `job_running`. Any other `lease_held` end stays as it is.
8. The page shows `job_running` as "this job is already running; press Run again when it ends".

## Issues

1. runner interface: Decision 6, declaration. Files: `internal/runner/api.go`, `internal/runner/api_test.go`. Acceptance: `api.go` declares `Dependent` with `After() []string` and its doc says the ids are jobs of the same day that the job reads; `api_test.go` shows a type with the method satisfies it. Design line: this file, Interface.

2. runner: Decisions 1, 2, 5 and the trigger of 6. Files: `internal/runner/reconcile/backfill_force.go`, `internal/runner/reconcile/backfill.go`, `internal/runner/reconcile/reconcile.go`, `internal/runner/reconcile/runkey.go`, `internal/runner/reconcile/fakes_test.go`, `internal/runner/reconcile/backfill_test.go`, `internal/runner/reconcile/backfill_force_test.go`, `internal/runner/reconcile/backfill_stop_test.go`, `internal/runner/reconcile/backfill_job_test.go`, `internal/runner/reconcile/backfill_window_test.go`, `internal/runner/reconcile/reconcile_test.go`. Acceptance: (a) `BackfillWith` has no `holdLease` call and `Backfill` and `Reconcile` still have theirs; (b) `bareRun`, `runMissingKey` and the key run of `Reconcile` each take the job's try-lock and release it on every return; (c) a busy job gives `BackfillWith` a `GroupLeaseHeld` line with `ReasonCode` "job_running" and leaves the key to `Reconcile`; neither waits; (d) `Reconcile` starts one goroutine per due job, waits for all of them before the cursor moves, and calls `holdLease` once; (e) an upstream run that ends ok starts its dependents (from `Dependent`) in goroutines of their own, a busy dependent is skipped, and a forced console re-run starts them with force; (f) no path starts an upstream from a dependent, and a dependent whose upstream has no ok run of the day runs anyway with `ReasonCode` "upstream_missing" on its line. Contracts: unchanged.
3. runner sched (independent of 2): Decision 4, runner part, and the tick call of Decision 5. Files: `internal/runner/sched/ticker.go`, `internal/runner/sched/ticker_test.go`, `internal/runner/sched/ticker_console_test.go`. Acceptance: `ticker.go` has no `ConsoleTurn` call inside `runTurn` and a console turn is called from the loop goroutine; `Run` waits for request goroutines for `drainGrace`, then cancels them and waits `killGrace`; a lost fence cancels their context. The worker returns from a Reconcile turn only after the job goroutines of Decision 5 have returned, under the same timeout and drain as today.
4. jobs: Decision 6, the pairs. Files: `internal/jobs/optionopen/pull.go`, `internal/jobs/optionopen/jp.go`, `internal/jobs/computefigures/schedule.go`, `internal/jobs/computefigures/job_test.go`, `internal/jobs/optioncandidates/job.go`, `internal/jobs/positionmarks/manifest.go`, `internal/jobs/optionopen/jp_test.go`, `internal/jobs/optionopen/pull_test.go`. Acceptance: `After()` of `option_open` returns ["option_open_plan"], of `option_open_jp` ["option_open_plan_jp"], of `compute_figures` ["option_eod", "option_eod_jp", "quote_snapshot"], of `option_candidates` ["compute_figures"], of `position_marks` ["account_capture"]; each test names its job's list. (Any listed test file that does not exist is created by that issue.) Depends on issue 1.
5. ot (Depends on #1861, same files; after 3): Decisions 3, 4 (ot part) and 7. Files: `cmd/ot/jobconsole_turn.go`, `cmd/ot/jobconsole_sched.go`, `cmd/ot/jobconsole_host.go`, `cmd/ot/jobconsole_turn_test.go`, `cmd/ot/jobconsole_scheduler_test.go`. Acceptance: `jobconsole_turn.go` no longer sets `Hold` nor calls `q.Hold`; each claimed request runs in its own goroutine that the host tracks for the drain; `classify` returns `refused` `job_running` as Decision 7.
6. pages: Decision 8. Files: `internal/pages/job_console/text.go`, `internal/pages/job_console/builder_test.go`. Acceptance: `refusedText` has a `job_running` case with the text of Decision 8.

Order: 1, then 2 to 3 and 4 (independent of each other), then 5, then 6. Issue 5 is not merged before 2.

## Left out

- A queue of waiting runs, a wait, a retry: the owner said none. A dependency is a trigger by completion, never a queue.
- The page text of `upstream_missing` on a `ran` or `rerun` line: the mark is in the stored line and the log; a page issue follows when the owner wants it shown.
- A guessed dependency: only the pairs of Decision 6 are declared; others (for example `option_list_preopen` and `option_open_resolve_gap` before the plan) are a later issue after a check of what each job reads.
- A lock per job and key, or per dataset.
- A change of `ops.run_lease`, its owners or its renewal: `Reconcile`, `ot backfill` and `ot run-once` keep it.
- Deleting `BackfillOptions.Hold`, `SlotHold` and the stale `ConsoleTurner` comment: an interface change for a later design.
- Stopping a running job from the page.
- Accepted for development mode: several broker jobs running at once on the pyside gateway may hit IBKR pacing limits. No limit on concurrent jobs is added; a pacing failure is a job error as today.

## Adversarial

Answered from `console-lock-adversarial.md`.

- Gap, the lock misses `bareRun` (blocking): adopted; Decision 2 puts the lock in the reconcile package around `bareRun`, `runMissingKey` and Reconcile's key run, not in `runkey.go` alone.
- Gap, the slot hold is a wait (blocking): adopted; Decision 3 removes it from the console path and leaves the field in `api.go` so there is no interface change.
- Gap, no trigger for the console turn off the worker: adopted; Decision 4 names the loop's own console turn and the goroutine per claimed request.
- Risk, a shared owner `runner` lets `ot run-once` in and its release deletes serve's row (blocking): adopted by dropping the shared lease. `BackfillWith` takes none; `Reconcile` is unchanged, so the owners are as today. The holder count, the shared renewal and the release race do not arise.
- Simpler way, no lease in `BackfillWith`: adopted, Decision 1. Moving the console off the worker is kept, because a request must start while a tick runs.
- False claims (run-once is refused by `Acquire`; the key runner path covers `bareRun`): both removed from the design.
- Files missing (`jobconsole_sched.go`, `jobconsole_scheduler_test.go`, `jobconsole_turn_test.go`, reconcile `fakes_test.go` and the backfill tests, `ticker_console_test.go`): adopted, in issues 1 to 3.
- Owner 09:55Z, scheduled runs parallel too: adopted as Decision 5, guarded only by the per-job lock.
- Owner 09:59Z, a downstream run alone must not re-run the upstream: adopted in Decision 6 (a basic check of the ledger, a mark, no refusal).
- Owner 09:57Z, "with a dependency the scheduler runs the upstream first, then the next job; unrelated jobs never queue": adopted as Decision 6 (trigger by completion, fallback at the dependent's own slot); the declaration is one new optional method, `Dependent.After`, next to `Schedule()`, which makes `api.go` an interface change with its own first issue.
- Console goroutine not drained or cancelled on fence loss; `disabled` map race: adopted, Decision 4.
- Lock-in of deleting the hold: avoided by leaving `Hold` in `api.go`.
