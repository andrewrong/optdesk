# Backfill any past day: `ot backfill --day D` repairs the keys of D whose own window has closed, whatever the cursor says, while the scheduler runs

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-10
Revision: 3

Source: owner 2026-11-10: "why do you follow your wrong system instead of reality" (a failed past day is to be repaired now, not when the cursor allows it). Fact base: the cursor diagnosis of 2026-11-10 (day 2026-11-09 holds the cursor at 2026-11-06 because its `fred_series` keys are inside their 96 h window; four keys of that day, `account.capture`, `master.session_day`, `obs.earnings_date` and `obs.option_eod`, were expired at their window close and stored as `no_value` with reason `expired`; `ot backfill --day 2026-11-09` refuses "ahead of the cursor").

Read on origin/main at f356426: `AGENTS.md`, `docs/design/modules.md`, `docs/design/backfill.md`, `docs/design/scheduler.md` (Decisions 4, 5, 6, 8, 9, 11 and the Close table), `docs/contracts/runner.md`, `internal/runner/api.go`, `internal/runner/reconcile/{reconcile,backfill,runkey}.go`, `internal/runner/sched/{gaps,ticker}.go`, `internal/store/ops/ledger.go` (`acquireSQL`), `cmd/ot/{backfill,runonce,scheduler,schedstatus,wire}.go`, `internal/jobs/fredseries/job.toml`.

Needs the owner: nothing. The one trade-off (a backfill pauses the scheduler's ticks, Decision 3) is stated there.

## Goal

`ot backfill --day D [--dataset ID]` repairs any past day D that has a stored plan: it re-runs the keys of D that are missing, `no_value` or expired, one key at a time, and only a key whose own window has closed (or whose dataset has no window rule on D at all; a window that is merely missing is put first, Decision 8). A key still inside its window (a `fred_series` key within 96 h) is left alone and reported. No cursor moves. The expired gap of a key clears when the backfill captures it. The owner does not stop the scheduler (its ticks pause while a backfill runs, Decision 3). A key that a source marked `no_value` before its window closed is in its window and is not re-run until the close; that is stated, not repaired.

## Decisions

1. **The cursor test goes; a per-key window test replaces it.** `Backfill` no longer returns `RefusedAheadOfCursor`. The refusal that stays is `RefusedNoPlan` (no stored plan: the day was never planned, so it is future or never reconciled, and nothing is invented). For each stored key (or the `--dataset` keys): `present` is not run (`AlreadyPresent`); a missing or `no_value` key whose window has not closed at the clock is not run and is listed in a new field `InWindow` (one rule for a missing window: before the keys are tested, `Backfill` puts the window of every dataset of the day that has none stored and whose job has a window rule for D, Decision 8, so a day left without windows by a crash is repaired by the backfill itself, which `Reconcile` cannot do for a day behind the cursor; after that, a key whose dataset still has no stored window has no window rule on D, counts as closed and is run exactly as `Reconcile` runs it, with `closeAt` zero, where an empty answer is `no_value` at once, as `docs/design/backfill.md` Behaviour 6 says. So a stored key is never a final `no_value` because its window was merely missing); every other key runs once, as today. Why this is safe is Decision 2.

2. **What the cursor protects, and why each holds without it.** The cursor is written only by `Reconcile` (`SetCursor` at `reconcile.go`), and the design text gives it three jobs: (a) every day at or before it is complete (`reconcile.go` doc comment; `docs/design/scheduler.md` Decision 4: "the cursor holds on a gap; the walk does not"); (b) the day after it is the only day `run-once` collects (`docs/design/backfill.md` Goal), so plans and windows are created in order by one writer; (c) `Backfill` of a day at or behind it never meets a key whose empty answer is still "explainable" (`docs/design/backfill.md`: an empty answer before the close cannot happen behind the cursor). Without the cursor test:
   - (a) A backfill never marks a key `no_value` before that key's window closes: the missing windows are put first (Decision 8), the test of Decision 1 then skips the key, and the key runner marks an empty answer `no_value` only at or after `closeAt` (`runkey.go` `verdict`: `empty_before_close` before it). So a backfill completes a day no earlier than `Reconcile` would (`expireIfClosed` also acts only at the close). The cursor still moves only in `Reconcile`, from the marks.
   - (b) `Backfill` calls no `SetCursor` and, for a stored key whose dataset has a window, no `PutExpected` or `PutWindow` (they are written only for added keys and for datasets with no stored window, Decision 8), and a day with no stored plan is refused. A day ahead of the cursor with a plan was planned by `Reconcile` itself, in order.
   - (c) This is the case Decision 1 handles directly: the "empty before close" ambiguity is why a key inside its window is not run.
   - The run row id is `day|dataset|subject|attempt` with attempt `len(runs)+1`; two writers could collide on it. The writers are made exclusive in Decision 3.
   - A key the backfill leaves missing (a retryable error) is, on the next tick, closed again by `expireIfClosed` (Close table row 3) with reason `expired`: the gap stays honest and nothing is lost.

3. **The fence is not shared; the run lease is made exclusive instead, so the owner does not stop the scheduler.** The advisory fence (`holdOffScheduler`) is one `pg_try_advisory_lock` held for the scheduler's whole life (`scheduler.md` Decision 5); it cannot be shared. It is also more than `backfill` needs. Read from the code: every key run in the process goes through `Reconcile` or `Backfill`, both of which hold `ops.run_lease` while they run and renew it every 30 s (`holdLease`; the only call of `RunKey` outside tests is `reconcile.go`; the ticker's only runner call is `Reconcile`, `ticker.go`); the ticker ignores the error of `Reconcile`, so a `Retryable` "lease is held" costs one 15-minute tick and nothing else. What does not work today is the owner: `Acquire` is `... WHERE expires_at <= now() OR owner = EXCLUDED.owner` and every caller passes the constant `"runner"`, so two processes of owner `runner` both get the lease. So: `Backfill` takes the lease under its own owner `backfill:` plus eight random hex digits (made once per `Backfill` call), which is never equal to `runner` nor to another backfill, so `Acquire` returns `Retryable` while a `Reconcile` runs and the scheduler's `Reconcile` returns `Retryable` while a backfill runs. `ot backfill` no longer takes the advisory fence. The lease must be renewed, or a backfill longer than the 10-minute TTL would lose it to a scheduler tick: `NewBackfillerWithRule` builds the reconciler with no scheduler ledger, so `ot backfill` builds its backfiller with `deps.newReconciler` (`reconcile.NewReconciler`, which returns the backfiller with the scheduler ledger, as `scheduler.go` does) and opens `deps.openSchedulerLedger`; `holdLease` takes the owner as a parameter (the constant `owner` stays the value of `Reconcile`). Consequences, stated:
   - While `ot backfill` runs, the scheduler's ticks do no `Reconcile` (a skipped tick, retried 15 minutes later). A backfill of one day with `--dataset` is short; a whole day with `option_eod` can be long (a run bound is up to the job's `RunBound`, default 30 min). A key of today whose window closes during a long backfill is expired by the next `Reconcile` after it, as it would be after any skipped tick; the owner should use `--dataset` for a long key.
   - A killed `ot backfill` leaves the lease for at most the 10-minute TTL (the scheduler's ticks retry; the same bound `scheduler.md` Decision 6 states for a killed scheduler).
   - `ot run-once` keeps the advisory fence and owner `runner`; a backfill started during a `run-once` gets `Retryable` from the lease, and the message says so.
   - Two `ot backfill` calls at once exclude each other by the distinct owners (the lease is held by one).
   - Simpler way considered: keep the fence and only drop the cursor test and add the window test. It reaches the goal for 2026-11-09 (ahead of the cursor, so each tick walks it and `Sync` clears its gaps), but the owner would stop the scheduler for every repair, which is what was refused; the cost of not doing so is Decisions 3 to 5.
   - If, against this reading, the owner prefers certainty over uptime, the fallback is the old rule (stop the scheduler); nothing in this design removes it, the fence code of `run-once` stays.

4. **The gap clears at once, and also for a day behind the cursor.** `ot scheduler status` shows the open gaps (`gaps: N`, one line each); they are written by `GapSyncer.Sync` for the days a tick walked, which are the days after the cursor (`ticker.go` `walked`). A day at or behind the cursor is never walked again, so the "next Sync" of `scheduler.md` (Close table, clearing rule) never comes for it. So `ot backfill` itself calls a new `KeyGapSyncer.SyncKeys(ctx, day)` after the run: it recomputes only the key gaps of that day (the same `keyGaps` as `Sync`, and an `applyKeys` that writes and clears the key gaps and skips both job-level loops of `apply`) and leaves job-level `preflight_failed` gaps alone (calling `Sync` with `disabled` nil would clear them, because `apply` treats a job gap of the day that is not in `disabled` as stale). `SyncKeys` is not wrapped by the notifying wrapper of `cmd/ot`, so a gap it writes sends no alert; it shows in `ot scheduler status` and in the exit code of the command. It is a new interface, not a new method of `GapSyncer`, so the fakes and the notifying wrapper of `cmd/ot` that implement `GapSyncer` keep compiling.

5. **A genuine `no_value` clears the gap.** Today `keyKind` ORs the `expired` reason over all runs, so a key that the backfill ends as a true `no_value` (the source says there is nothing, a run with reason `backfill` and no error) would keep its `expired` gap for ever, against the clearing rule ("present, or `no_value` with a reason other than `expired` and `crashed_repeatedly`", `docs/contracts/runner.md`). `keyKind` changes so that the latest verdict run decides `expired`: a later verdict run (a done run without an error) resets it. A later retryable run does not reset it, so a failed backfill keeps the gap.

6. **What `ot scheduler status` prints after the repair.** Status has no cursor line (the file has no `cursor`); it prints the instance, runs, next slots and `gaps: N` with one line `<reason> <dataset> <day>` per open gap, and is red while a gap is open. Today for 2026-11-09 it shows the four expired gaps (`expired account.capture 2026-11-09`, `... master.session_day ...`, `... obs.earnings_date ...`, `... obs.option_eod ...`; one record per dataset and day). After `ot backfill --day 2026-11-09`: a captured key's line disappears at once (Decision 4); a key that ended a genuine `no_value` also disappears (Decision 5); a key that failed again keeps its `expired` line and the command exits 1 naming the count. The `fred_series` keys of that day are in their window: they have no gap line before or after (`keyKind` returns none for an open window), the command lists them as `in window` and exits 0 for them. The cursor stays at 2026-11-06 until the FRED keys close, about 2026-11-13, and the status does not show it.

7. **Exit codes and report.** `ot backfill` exits 0 when every key it was asked to run is `present`, `no_value` by a verdict, or in its window (reported), and 1 when a key is still missing or the lease is held (`Retryable`, message "the runner lease is held, retry in a minute"). The `done` log line gains `in_window`.

8. **Backfill puts the windows a day lacks and adds the keys that today's plan has for day D and the stored plan lacks.** The stored plan of a day is frozen at its first `Reconcile`, so a key added to a job later (six `fred_series` series were added to `job.toml` on 2026-11-08 and 2026-11-09) is not in the stored plan of 2026-11-06 or earlier, and a backfill that runs only stored keys reports `done` and stores nothing for them, although the source publishes that day. This reverses `docs/design/backfill.md` Behaviour 2 ("`PutExpected` is not called") and Behaviour 6 and its out-of-scope line about a changed plan, for added keys and missing windows only; the docs issue (6) amends them. The one rule applies to every day that has a stored plan, whether it is ahead of the cursor or not (the cursor is not read, Decision 1); a day ahead of the cursor gets its added keys from `Backfill` as well as from `Reconcile`'s own plan rewrite. After the lease and the refusal `RefusedNoPlan`, in this order, validating before any write:
   1. The sessions of D: `r.walk(ctx, D-1, D)` (`window.go`), taking the entry of D (an empty, non-nil `daySessions` when D is a session of no calendar). Two plans are read: `pAll = r.plan(ctx, day, nil)`, which finds the job of every stored key as `backfillPlan` does today, and `pDay = r.plan(ctx, day, sess)`, which is what `Reconcile` would plan for D (a job is planned only when its own calendar has a session on D). The added keys are the keys of `pDay` that are not in the stored keys (`ExpectedKey` equality) and whose dataset has a window for D (`windowFor` returns a window from the session of D); a key with no window rule for D is not added and is not an error.
   2. The `--dataset` filter (`selectKeys`) is applied to the stored and added keys together, so a dataset whose keys are all added is found; a dataset with no key even after the add is the existing error `dataset X has no key on the day`; every selected key must have a job in `pAll` (the existing error `no job returns a key of dataset X`). These errors return before any write: an unknown `--dataset` or a stored key with no job leaves the ledger as it was.
   3. The windows. For every dataset of the stored and added keys (not only the selected ones, so that no added key is stored without its window) whose stored window is missing (`closeOf` returns a zero time) and that has a window rule for D, `Backfill` calls `putWindows` (`reconcile.go`) with `plan{keys: those keys, job: pAll.job}` and `sess`. The windows are written before the plan, so a crash or a failed `PutWindow` leaves the plan as it was and the next backfill repeats the step; a window put twice is the same row, as `Reconcile` puts it every tick.
   4. When there is an added key it writes `PutExpected(day, pDay.hash, stored + added)`. `pDay.hash` is the hash `Reconcile` computes for D, so a later `Reconcile` of a day ahead of the cursor sees the same hash and does not rewrite the plan.
   5. The runs go through the existing `backfillKey` path and Decision 1's window test.

   The windows put in step 3 are real writes of a backfill that finds no key to run (an all-present day with no stored window gets its windows); this is intended, and the tests of the existing `backfill_test.go` that assert "no write" stop counting window puts (issue 7).

   Invariants: no stored key is removed or changed (the new plan is the stored keys plus the added ones, in that order); a stored key that today's plan no longer returns stays in the plan and is an error as before; `SetCursor` is never called (the cursor stays where `Reconcile` left it and a day behind it is not re-walked); a day with no stored plan is still refused (`RefusedNoPlan`), nothing is invented for a day the runner never planned; a second backfill of the same day finds no added key and no missing window and writes nothing; an error of step 2 writes nothing. The lease of Decision 3 is held for the writes, so `Reconcile` cannot write the same plan at the same time.

## Modules

No change to `docs/design/modules.md`: runner (`internal/runner/**`) and ot (`cmd/**`) are existing rows and no depends cell changes (`ot` already imports `runner`, `internal/runner/reconcile` and `internal/runner/sched`). No `go.mod` edit, no migration, no new table.

| issue | module |
|---|---|
| backfill-any-day-interface | runner |
| backfill-by-window | runner |
| backfill-gap-clear | runner |
| backfill-cmd | ot |
| backfill-drop-refusal | runner |
| backfill-docs | docs |
| backfill-add-plan-keys | runner |

## Interface

One interface file changes, `internal/runner/api.go` (owner: runner). Everything is added; nothing is renamed or removed in the first issue, so `cmd/ot` compiles between issues.

| name | declaration |
|---|---|
| `runner.BackfillResult.InWindow` (interface) | field `InWindow []contracts.ExpectedKey`: keys not run because their window had not closed |
| `runner.KeyGapSyncer` (interface) | `interface{ SyncKeys(ctx context.Context, day contracts.Day) error }` |
| `runner.NewKeyGapSyncerFunc` (interface) | `func(store.OpsLedger, store.SchedulerLedger, clockwork.Clock) KeyGapSyncer`, implemented by `sched.NewKeyGapSyncer` |
| doc of `Backfiller` and `BackfillRefusal` (interface) | says: no cursor test; `RefusedAheadOfCursor` is declared and never returned (removed by `backfill-drop-refusal`); the lease is taken under an owner of its own; the sentence "A backfill never moves the cursor, and never writes a plan or a window" becomes: it writes only the missing windows of a stored day and the keys its plan lacks, and never moves the cursor |

`Backfiller.Backfill` and `NewReconcilerFunc` keep their signatures (`reconcile.NewReconciler` already returns the backfiller with the scheduler ledger, so the lease is renewed). `cmd/ot` gets `deps.newKeyGapSyncer`.

## Issues

Every test calls `t.Parallel()`, writes files only under a temporary directory, uses a fake clock, never sleeps, opens no port and touches no live broker or database. Acceptance is checked offline by reading, grep and git; test runs are under "Checked by CI". `BASE` is `git merge-base HEAD origin/main`.

### 1 backfill-any-day-interface (interface)

Module: runner. Size: S.
Files: `internal/runner/api.go`, `internal/runner/api_test.go`, `docs/contracts/runner.md`
Design: docs/design/backfill-any-day.md#Interface
Order: first.
Goal: the declarations of `## Interface`.
Acceptance:
1. `grep -c "InWindow" internal/runner/api.go` is at least 1, `grep -c "KeyGapSyncer" internal/runner/api.go` is at least 2 and `grep -c "^type NewKeyGapSyncerFunc " internal/runner/api.go` is 1.
2. `grep -c "RefusedAheadOfCursor" internal/runner/api.go` is still at least 1 and `grep -c "never returned" internal/runner/api.go` is at least 1.
3. `grep -c "owner of its own" docs/contracts/runner.md` is 1, `grep -c "A day after the cursor" docs/contracts/runner.md` is 0, `grep -c "scheduler stopped" docs/contracts/runner.md` is 0, `grep -c "one day at or behind the cursor" docs/contracts/runner.md` is 0 and `grep -c "are never called" docs/contracts/runner.md` is 0 (the `## Backfill` paragraph and the Close-table note are rewritten: no cursor test, the lease under its own owner, windows and added keys written).
4. `grep -c "writes a plan or a window" internal/runner/api.go` is 0 (the doc comment of `Backfiller` is rewritten as in `## Interface`).
5. `git diff --name-only BASE` lists exactly the three Files.

### 2 backfill-by-window

Module: runner. Size: M.
Files: `internal/runner/reconcile/backfill.go`, `internal/runner/reconcile/reconcile.go`, `internal/runner/reconcile/backfill_test.go`, `internal/runner/reconcile/backfill_window_test.go`
Design: docs/design/backfill-any-day.md#Decisions
Order: after 1.
Goal: Decisions 1, 2 and 3 in the runner: no cursor test, the per-key window test with `InWindow`, the lease under the owner `backfill:` plus random hex.
Acceptance:
1. `grep -c "RefusedAheadOfCursor" internal/runner/reconcile/backfill.go` is 0 and `grep -c "ledger.Cursor" internal/runner/reconcile/backfill.go` is 0.
2. `grep -c "SetCursor\|PutExpected\|PutWindow" internal/runner/reconcile/backfill.go` is 0 (the doc comment of `Backfill`, which names the three, is reworded; issue 7 adds the writes of Decision 8, and until it lands a key whose dataset has no stored window runs as closed).
3. `grep -c "InWindow" internal/runner/reconcile/backfill.go` is at least 1, and `grep -c "backfill:" internal/runner/reconcile/reconcile.go` is at least 1 (the owner prefix), while `grep -cE 'owner[[:space:]]+= "runner"' internal/runner/reconcile/reconcile.go` is still 1.
4. `grep -c "TestBackfillRefusals" internal/runner/reconcile/backfill_test.go` is 1 and the table holds no case of the cursor: `grep -rc "RefusedAheadOfCursor\|ahead of cursor" internal/runner/reconcile | grep -v ":0" | wc -l` prints 0. `holdLease` takes the owner: `grep -c "func (r \*reconciler) holdLease(ctx context.Context, owner string" internal/runner/reconcile/reconcile.go` is 1. The Retryable error of the lease test is checked through the fake ledger of `fakes_test.go`, so `Backfill` renews with a scheduler ledger fake in `TestBackfillOwnLease` (a renewal is counted after a 31 s advance of the fake clock).
5. `grep -rhE "^func (TestBackfillAheadOfCursor|TestBackfillInWindowKept|TestBackfillExpiredKey|TestBackfillOwnLease|TestBackfillCursorUnmoved)\(" internal/runner/reconcile | wc -l` prints 5 (new; each name is under 32 characters). The expired-key test: a key stored `no_value` with a run of reason `expired`, window closed, cursor before the day, is run and becomes `present`; the in-window test: a key with an open window is not run (the job fake counts 0 calls) and is in `InWindow`; the lease test: a fake ledger whose lease is held by owner `runner` makes `Backfill` return a Retryable error, and a held lease of owner `backfill:` makes `Reconcile` return a Retryable error; the cursor test: the cursor of the fake ledger is equal after the call.
6. `wc -l internal/runner/reconcile/backfill.go internal/runner/reconcile/reconcile.go internal/runner/reconcile/backfill_window_test.go | grep -v total | awk '$1>=600' | wc -l` prints 0.
7. `git diff --name-only BASE` lists exactly the four Files.

### 3 backfill-gap-clear

Module: runner. Size: S.
Files: `internal/runner/sched/gaps.go`, `internal/runner/sched/gaps_test.go`
Design: docs/design/backfill-any-day.md#Decisions
Order: after 1.
Goal: Decisions 4 and 5: `SyncKeys` writes and clears the key gaps of one day only, and a later verdict run resets `expired` in `keyKind`.
Acceptance:
1. `grep -c "func NewKeyGapSyncer" internal/runner/sched/gaps.go` is 1 and `grep -c "var _ runner.NewKeyGapSyncerFunc" internal/runner/sched/gaps.go` is 1.
2. `grep -rhE "^func (TestSyncKeysClears|TestSyncKeysKeepsJobGap|TestGenuineNoValueClears|TestFailedBackfillKeepsGap)\(" internal/runner/sched | wc -l` prints 4 (new): after a key that had an expired run gets a present mark the gap is cleared for a day behind the cursor; an open `preflight_failed` gap of the same day is still open after `SyncKeys`; a key that has an `expired` run and then a verdict run of reason `backfill` without an error code and mark `no_value` has no gap; one whose later run is retryable keeps the `expired` gap.
3. `grep -c "func (g \*gapSyncer) applyKeys" internal/runner/sched/gaps.go` is 1.
4. `wc -l internal/runner/sched/gaps.go` prints a number under 600.
5. `git diff --name-only BASE` lists exactly the two Files.

### 4 backfill-cmd

Module: ot. Size: M.
Files: `cmd/ot/backfill.go`, `cmd/ot/backfill_test.go`, `cmd/ot/wire.go`, `docs/contracts/ot.md`
The field `deps.newBackfiller` stays in `wire.go` (the fakes of other tests set it); nothing reads it after this issue, and its removal is left out.
Design: docs/design/backfill-any-day.md#Decisions
Order: after 1, 2 and 3.
Goal: Decisions 3, 4, 6 and 7 on the command line: `ot backfill` builds its backfiller with `deps.newReconciler` and the scheduler ledger, takes no advisory fence, calls `SyncKeys` after the run, and reports `in window` keys.
Acceptance:
1. `grep -c "holdOffScheduler" cmd/ot/backfill.go` is 0 and `grep -c "holdOffScheduler" cmd/ot/runonce.go` is still at least 1.
2. `grep -c "newBackfiller\|NewBackfillerWithRule" cmd/ot/backfill.go` is 0, `grep -c "newReconciler" cmd/ot/backfill.go` is at least 1 and `grep -c "openSchedulerLedger" cmd/ot/backfill.go` is at least 1.
2b. `grep -c "newKeyGapSyncer" cmd/ot/wire.go` is at least 2 and `grep -c "sched.NewKeyGapSyncer" cmd/ot/wire.go` is 1.
3. `grep -rc "RefusedAheadOfCursor\|ahead of the cursor" cmd docs/contracts/ot.md | grep -v ":0" | wc -l` prints 0 (the code, its test rows and the text of `ot.md` go in this issue), `grep -c "not planned yet" cmd/ot/backfill.go` is 1 (the text of `RefusedNoPlan` names a day with no plan as not planned yet or never reconciled) and `grep -c "in_window" cmd/ot/backfill.go` is at least 1.
4. `grep -rhE "^func (TestBackfillAheadDay|TestBackfillSyncsGaps|TestBackfillLeaseHeld|TestBackfillNoFence)\(" cmd/ot | wc -l` prints 4 (new; fakes only): a day ahead of the fake cursor is backfilled with exit 0; `SyncKeys` is called once with the day; a held lease gives exit 1 with the text "the runner lease is held"; the fake `openFence` is never called.
5. `grep -n "stop the scheduler" docs/contracts/ot.md | grep -c backfill` prints 0 and `grep -c "ot run-once. and .ot backfill. try the same lock" docs/contracts/ot.md` prints 0 (the sentence of the `ot scheduler` entry now names `ot run-once` only), and `grep -c "moves neither the cursor" docs/contracts/ot.md` prints 0 (the `ot backfill` entry says that it puts the missing windows and the added keys of a stored day and never moves the cursor).
6. `git diff --name-only BASE` lists exactly the four Files.

### 5 backfill-drop-refusal (interface)

Module: runner. Size: S.
Files: `internal/runner/api.go`, `internal/runner/api_test.go`
Design: docs/design/backfill-any-day.md#Interface
Order: after 1 and 4.
Goal: remove the constant `RefusedAheadOfCursor`, which nothing returns or reads.
Acceptance:
1. `grep -rc "RefusedAheadOfCursor" internal cmd | grep -v ":0" | wc -l` prints 0.
2. `grep -c "RefusedNoPlan" internal/runner/api.go` is at least 1.
3. `git diff --name-only BASE` lists exactly the two Files.

### 6 backfill-docs

Module: docs. Size: S.
Files: `docs/design/scheduler.md`, `docs/design/backfill.md`
Design: docs/design/backfill-any-day.md#Decisions
Order: after 4.
Goal: the two older designs say what is now true: backfill has no cursor test, no fence, and the hand path for an expired key is `ot backfill` with the scheduler running.
Acceptance:
1. `grep -c "stop the scheduler, run" docs/design/scheduler.md` prints 0 and `grep -c "backfill-any-day" docs/design/scheduler.md` is at least 1.
2. `grep -c "ahead_of_cursor" docs/design/backfill.md` is 0 or the line holding it also holds "superseded by backfill-any-day".
2b. `grep -c "PutExpected. is not called" docs/design/backfill.md` prints 0, `grep -c "are never called" docs/design/backfill.md` prints 0 (Behaviour 6), `grep -c "Takes the same lease as" docs/design/backfill.md` prints 0 (Behaviour 4: the lease is taken under the owner of Decision 3), `grep -c "added keys" docs/design/backfill.md` is at least 1 and `grep -c "only the stored keys are run" docs/design/backfill.md` prints 0 (Behaviour 2 and the out-of-scope line about a changed plan say that backfill adds the keys today's plan has and the stored plan lacks, Decision 8 of backfill-any-day).
3. `git diff --name-only BASE` lists exactly the two Files.

### 7 backfill-add-plan-keys

Module: runner. Size: S.
Files: `internal/runner/reconcile/backfill.go`, `internal/runner/reconcile/backfill_test.go`, `internal/runner/reconcile/backfill_addkeys_test.go`
Design: docs/design/backfill-any-day.md#Decisions
Order: after 2 and 6.
Goal: Decision 8 in the runner: `Backfill` validates, puts the missing windows, then adds the keys of the plan of D that the stored plan lacks (`PutExpected` with that plan's hash), and never moves the cursor.
Acceptance:
1. `grep -c "PutExpected(" internal/runner/reconcile/backfill.go` is 1, `grep -c "putWindows(" internal/runner/reconcile/backfill.go` is at least 1 and `grep -c "SetCursor(" internal/runner/reconcile/backfill.go` is 0.
2. `grep -rhE "^func (TestBackfillAddsMissingKeys|TestBackfillAddKeysNoWindow|TestBackfillNoPlanStillRefused|TestBackfillAddKeysIdempotent|TestBackfillPutsMissingWindow|TestBackfillAddedDataset|TestBackfillErrorWritesNone)\(" internal/runner/reconcile | wc -l` prints 7 (new; fake ledger and fake clock): a stored plan with key A and a job whose plan returns A and B gives one `PutWindow` for the dataset of B, then one `PutExpected` with today's hash and A then B (the fake ledger records the order), B in `Filled`, no `SetCursor`; an added key whose dataset has no window rule for the day is not added and no error; a day with an empty stored plan is `RefusedNoPlan` with no `PutExpected`; a second call writes nothing; a stored key with a `PutExpected` and no window, then a backfill, gives one `PutWindow`, the key in `InWindow` while the window is open and no `no_value` mark, and a second backfill writes nothing; `--dataset` naming a dataset whose keys are all added runs them and does not return `dataset X has no key on the day`; a failing `PutWindow` returns the error with no `PutExpected`; `--dataset nope` (or a stored key with no job) while the plan has an added key returns the error with no `PutWindow` and no `PutExpected`; a day that is a session of no calendar adds nothing; `PutExpected` gets the hash of the plan that `Reconcile` makes for D.
3. The window puts are now writes of a backfill, so the helper `writes()` of the spy ledger in `backfill_test.go` stops counting them: `grep -A2 "func (s \*spyLedger) writes" internal/runner/reconcile/backfill_test.go | grep -c windowsPut` prints 0, `grep -c "l.writes() != 0" internal/runner/reconcile/backfill_test.go` is still 6 (the assertions of `TestBackfillPresentIsNotRun`, of the live case of `TestBackfillLiveRunning` and of the `nope` and `gone` cases of `TestBackfillDatasetFilter` stay as they are and still pass), `git diff BASE | grep -c '^-.*\(assert\|require\|t\.Errorf\|t\.Fatalf\)'` prints 0, and `grep -c "windowsPut.Load()" internal/runner/reconcile/backfill_addkeys_test.go` is at least 1 (the new tests count window puts themselves).
4. `wc -l internal/runner/reconcile/backfill.go internal/runner/reconcile/backfill_addkeys_test.go | grep -v total | awk '$1>=600' | wc -l` prints 0.
5. `git diff --name-only BASE` lists exactly the three Files.

## Checked by CI

ci-go runs `go test ./internal/runner/... ./cmd/ot -race -count=1` (the tests named above), `go vet`, the go-arch-lint of `scripts/arch`, and the `Contracts: unchanged` check. No acceptance item above runs a test.

## Left out

- Re-running a `present` key (`--force`): a stored value is not rewritten (unchanged from `backfill.md`).
- A key inside its window: the scheduler owns it; the backfill reports it. No `--early` flag; an owner who wants a FRED value before 96 h has the scheduler's own retries.
- Moving the cursor or letting `Backfill` plan a day: the cursor is `Reconcile`'s alone.
- Keeping the advisory fence and only replacing the cursor test (the smaller change): rejected because the owner refused stopping the scheduler for every repair; argued in Decision 3 and `## Adversarial`.
- Interruption of a long backfill by a scheduler tick (a preemptible lease): not needed while ticks are 15 minutes and a backfill takes `--dataset`; stated in Decision 3.
- Why the four keys of 2026-11-09 failed (run rows `ops.capture_run` error codes): not in the code; the backfill run rows will show it.
- The cause of `no_value` for a key a source marks before its close (Goal): it waits for the close.
- `ot series-load` (the owner's command). Decision 8 applies to every day with a stored plan, ahead of the cursor or not; a day with no stored plan is refused.
- A stored key that today's plan no longer returns: still an error, never removed (Decision 8).
- A job-level view of in-window keys in `ot scheduler status`: status stays a read of gaps and runs.

## Adversarial

One fresh reader answered six questions against the code; its text is in the dispatcher scratchpad (backfill-any-day.adversarial.md, not in the repository). Adopted: the no-window case is stated as equal to `Reconcile`'s own run (Decision 1); the lease renewal is made explicit, with `ot backfill` built on `newReconciler` and the scheduler ledger and `holdLease` taking the owner (Decision 3, issues 2 and 4); `SyncKeys` gets its own `applyKeys` and the missing alert is stated (Decision 4, issue 3); the stale refusal text of `docs/contracts/runner.md` and the message for a day with no plan are in the acceptance (issues 1 and 4); the constant `RefusedAheadOfCursor` is checked in the tests of issues 2 and 4 so that issue 5 compiles; the uptime claim is reworded and an early `no_value` is stated as waiting for its close (Goal). Added after the reading: Decision 8 and issue 7 (backfill adds the plan keys the stored plan lacks), from the data-repair of 2026-11-10, where the stored plan of 2026-11-06 lacked six `fred_series` keys. Second review of #1192 adopted: validation before any write and the `writes()` helper of `backfill_test.go` (issue 7), the sessions and the hash of Decision 8 read from the plan `Reconcile` makes, and three more stale sentences checked (`runner.md`, `ot.md`, `backfill.md` Behaviour 4). Review of #1192 adopted: one rule for a missing window (put first, Decision 8), windows written before the plan, today's hash named, the dataset filter applied after the add, and the stale texts of `runner.md`, `ot.md`, `api.go` and `backfill.md` checked by their own sentences. Not adopted: the simpler way (keep the fence), because the owner refused stopping the scheduler; the reasoning is in Decision 3.
