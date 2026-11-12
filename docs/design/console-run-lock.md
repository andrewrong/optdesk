# Console run lock: two runs of one job never overlap, in memory; a console run takes no database lease

Author: the optdesk window (draft)
Owner approved: (pending)
Date: 2026-11-12
Source: owner 2026-11-12 09:48Z: every job of the scheduler can be re-run by hand; two runs of the same job must not run at the same time; the whole-process lock only prevents another process and does not control re-runs between tasks; no queueing.

## Goal

Today a console run is refused `lease_held` whenever `ops.run_lease` is held under another owner: `BackfillWith` takes it under a random owner `backfill:<hex>` (`internal/runner/reconcile/backfill_force.go`, `holdLease`), the scheduler tick under `runner` (`reconcile.go`), and a console run killed by a deploy leaves its owner holding the lease for `leaseTTL`. The console turn also runs on the one worker of `internal/runner/sched/ticker.go` (`runTurn`), and a request is held while a slot start is near (`SlotHold`, `slot_near`): the console waits for a tick and for a slot, which is a queue. After this design a console run takes no lease, is refused only by a run of the same job, and nothing waits.

## Modules

- runner (`internal/runner/**`): `reconcile` (the per-job lock, no lease in `BackfillWith`), `sched` (the console loop off the tick worker).
- ot (`cmd/**`): the console host starts a goroutine per claimed request, stops setting `Hold`, and maps the new reason.
- pages (`internal/pages/**`): the text of the new reason.

## Interface

No interface file changes. `BackfillOptions.Hold` and `SlotHold` stay in `internal/runner/api.go` unused by the console (a later design may delete them); the doc comment of `ConsoleTurner` ("runs on the worker, so it never overlaps a Reconcile") is left as it is and is stale until then, which no code reads. The refusal is a `KeyRun` of group `GroupLeaseHeld` with `ReasonCode` `job_running`, a free string.

## Decisions

1. `BackfillWith` takes no lease. The process exclusion of serve is the scheduler fence (`ticker.watchFence`), the exclusion between runs is Decision 2, and the live-run check of a key (`closeExpired`) stays. `Reconcile` keeps `holdLease` under the owner `runner` as today, and `Backfill` (the bare call of `ot backfill`) keeps its own `backfill:<hex>` lease, so `ot run-once` and the backfill command behave as before. `ops.run_lease` is not shared and not counted: no change to `Acquire`, `Release` or the renewal.
2. One in-memory try-lock per job id, held in the `reconcile` package around every job run: `bareRun` (forced re-run, no run row), `runMissingKey` (console run of a missing key) and the key run of `Reconcile`. A busy job is not run and not waited for. `BackfillWith` writes a `lease_held` line with `ReasonCode` `job_running`; `Reconcile` leaves the key for the next tick. Per job, not per job and key: "two runs of the same job". The lock is a field of the reconciler, so Reconcile and `BackfillWith` of one process share it.
3. The slot hold is removed from the console path: `cmd/ot` stops setting `BackfillOptions.Hold` and the request is no longer held for a slot (`turn` no longer calls `h.q.Hold`; `slot_near` is no longer produced). The per-job lock replaces it.
4. The console loop runs off the tick worker. The trigger is the ticker loop's own console turn (`submit(turn{console: ...})`, `ticker.go`): it now calls `ConsoleTurn` on the loop goroutine, which only reads the queue and claims requests, and starts one goroutine per claimed request. Those goroutines are drained in `Run` with `drainGrace` and `killGrace`, and their context is cancelled on fence loss, so no job runs after the fence is lost. A request goroutine calls nothing owned by the worker: not `refresh`, not the `disabled` map; it reads the job set the console host was given. Different jobs run at once; the same job is refused by Decision 2.
5. `cmd/ot` `classify`: a result with no `ran` or `rerun` line and at least one `lease_held` line of reason `job_running` ends the request `refused` `job_running`. Any other `lease_held` end stays as it is.
6. The page shows `job_running` as "this job is already running; press Run again when it ends".

## Issues

1. runner: Decisions 1 and 2. Files: `internal/runner/reconcile/backfill_force.go`, `internal/runner/reconcile/backfill.go`, `internal/runner/reconcile/reconcile.go`, `internal/runner/reconcile/runkey.go`, `internal/runner/reconcile/fakes_test.go`, `internal/runner/reconcile/backfill_test.go`, `internal/runner/reconcile/backfill_force_test.go`, `internal/runner/reconcile/backfill_stop_test.go`, `internal/runner/reconcile/backfill_job_test.go`, `internal/runner/reconcile/backfill_window_test.go`, `internal/runner/reconcile/reconcile_test.go`. Acceptance: (a) `BackfillWith` has no `holdLease` call and `Backfill` and `Reconcile` still have theirs; (b) `bareRun`, `runMissingKey` and the key run of `Reconcile` each take the job's try-lock and release it on every return; (c) a busy job gives `BackfillWith` a `GroupLeaseHeld` line with `ReasonCode` "job_running" and leaves the key to `Reconcile`; neither waits. Contracts: unchanged.
2. runner sched (independent of 1): Decision 4, runner part. Files: `internal/runner/sched/ticker.go`, `internal/runner/sched/ticker_test.go`, `internal/runner/sched/ticker_console_test.go`. Acceptance: `ticker.go` has no `ConsoleTurn` call inside `runTurn` and a console turn is called from the loop goroutine; `Run` waits for request goroutines for `drainGrace`, then cancels them and waits `killGrace`; a lost fence cancels their context.
3. ot (Depends on #1861, same files; after 2): Decisions 3, 4 (ot part) and 5. Files: `cmd/ot/jobconsole_turn.go`, `cmd/ot/jobconsole_sched.go`, `cmd/ot/jobconsole_host.go`, `cmd/ot/jobconsole_turn_test.go`, `cmd/ot/jobconsole_scheduler_test.go`. Acceptance: `jobconsole_turn.go` no longer sets `Hold` nor calls `q.Hold`; each claimed request runs in its own goroutine that the host tracks for the drain; `classify` returns `refused` `job_running` as Decision 5.
4. pages: Decision 6. Files: `internal/pages/job_console/text.go`, `internal/pages/job_console/builder_test.go`. Acceptance: `refusedText` has a `job_running` case with the text of Decision 6.

Order: 1 and 2 first (independent), then 3, then 4. Issue 3 is safe before 1 only if the lock is in place; it is not merged before 1.

## Left out

- A queue of waiting runs, a wait, a retry: the owner said none.
- A lock per job and key, or per dataset.
- A change of `ops.run_lease`, its owners or its renewal: `Reconcile`, `ot backfill` and `ot run-once` keep it.
- Deleting `BackfillOptions.Hold`, `SlotHold` and the stale `ConsoleTurner` comment: an interface change for a later design.
- Stopping a running job from the page.

## Adversarial

Answered from `console-lock-adversarial.md`.

- Gap, the lock misses `bareRun` (blocking): adopted; Decision 2 puts the lock in the reconcile package around `bareRun`, `runMissingKey` and Reconcile's key run, not in `runkey.go` alone.
- Gap, the slot hold is a wait (blocking): adopted; Decision 3 removes it from the console path and leaves the field in `api.go` so there is no interface change.
- Gap, no trigger for the console turn off the worker: adopted; Decision 4 names the loop's own console turn and the goroutine per claimed request.
- Risk, a shared owner `runner` lets `ot run-once` in and its release deletes serve's row (blocking): adopted by dropping the shared lease. `BackfillWith` takes none; `Reconcile` is unchanged, so the owners are as today. The holder count, the shared renewal and the release race do not arise.
- Simpler way, no lease in `BackfillWith`: adopted, Decision 1. Moving the console off the worker is kept, because a request must start while a tick runs.
- False claims (run-once is refused by `Acquire`; the key runner path covers `bareRun`): both removed from the design.
- Files missing (`jobconsole_sched.go`, `jobconsole_scheduler_test.go`, `jobconsole_turn_test.go`, reconcile `fakes_test.go` and the backfill tests, `ticker_console_test.go`): adopted, in issues 1 to 3.
- Console goroutine not drained or cancelled on fence loss; `disabled` map race: adopted, Decision 4.
- Lock-in of deleting the hold: avoided by leaving `Hold` in `api.go`.
