# Restart resume: a restart during the open loses no fetched contract, and a stop caused by a deploy is not a failed attempt

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-18

Amends: `docs/design/option-open-pull.md` (Decision 8 is replaced by Decision 1 below, as a new Revision 2 of that file's Decision 8; every other Decision and Revision of it stands) and `docs/design/single-runner.md` (Decision 5) with `docs/design/scheduler.md` (Decision 7, the counting and the brake; the advisor's ruling named "Decisions 5 and 7 of single-runner.md", but the counting and the brake of three are Decision 7 of `scheduler.md`; Decision 7 of `single-runner.md` is the plan-only day and is not touched) and `docs/contracts/runner.md` (the Invariants "Counting" and "Brake", and the list of reason codes). This file edits none of those files; the issues below edit the code and `docs/contracts/runner.md`. Read together, this file wins where it differs.

Owner decision: 2026-11-18, asks optdesk-5047 and optdesk-5048: deploys will keep happening. A restart must lose no work already done: after any restart (a deploy, a crash, a reboot) a run continues from what is stored and asks only what is missing. The owner's result: "a restart during the open loses no fetched contract and re-asks only the missing ones".
Advisor ruling (binding): per job, plus one generic runner rule. The job is `option_open`; the generic rule is Decision 3.

Read on origin/main at 6b7110d8 (the first draft was read at c24b2d02; no file named here changed between them): `AGENTS.md`, `docs/design/modules.md`, `docs/design/option-open-pull.md` (Decisions 7, 8, 22), `docs/design/single-runner.md` (Decisions 4 to 7), `docs/design/scheduler.md` (Decision 7), `docs/contracts/runner.md`, `docs/deploy/scheduler.md`, `internal/jobs/optionopen/{core,pull,jp,job_pull.toml}` and their tests, `internal/store/openquote/api.go`, `internal/store/optionopenpg/queries.go`, `internal/runner/sched/{ticker,gaps}.go`, `internal/runner/reconcile/{reconcile,runkey,backfill_force}.go`, `cmd/ot/{scheduler,wire_optionopen}.go` and `cmd/ot/scheduler_close_test.go`, and the scheduler log of 2026-11-18 (`optdesk-scheduler.log` of the service).

## Today

- `pull.runKey` quotes a whole batch (up to `max_per_key` 120 contracts, budget 8 minutes) and calls `InsertOptionOpen` once, at the end. A kill in the middle loses every contract already fetched, and the next run of the key asks the whole batch again. Decision 8 of `option-open-pull.md` says so: "written at the end of the run (a crash writes nothing and the runner retries the key)".
- `InsertOptionOpen` is an upsert, not "left as it is": the doc of `openquote.OptionOpenWriter` still says a row whose key exists is left as it is, but `optionopenpg` writes `ON CONFLICT (session_day, capture_id, slot, instrument_id) DO UPDATE ... WHERE <guard>` (`optionOpenGuard`, ranks: status 1 is 2, status 2 is 1, any other status 0; a row replaces a stored one only when it ranks higher, or ranks the same, differs and, for rank 2 and 1, is not thinner). So a second write of the same item is idempotent, a stored not-ok row never blocks a later ok one, and an ok row is never replaced by a lower one (a later ok of the same rank does replace an ok row today; Decision 1 stops asking an item that is stored ok). The capture id is deterministic per (session day, slot, instrument) (`Core.captureID`), so one item is one row for its day and slot.
- There is no reader of the rows of one slot: `OptionOpenReader.OptionOpen` returns the newest row of each instrument of the day across slots.
- The ticker drains for `drainGrace` (30 s) after SIGTERM, then cancels the worker with a plain cancel and waits `killGrace` (5 s) (`internal/runner/sched/ticker.go`). A cut run is recorded `partial` (rows returned) or `abandoned_stop` (none). The runner counts a `partial` toward its two-run cap and an `abandoned_stop` toward `crashBrake`; `closeRunningAtStart` of `cmd/ot/scheduler.go` closes rows found running as `crashed`, which count toward the brake.
- A call cut by the cancel ends today as `error` `BROKER_ERROR` (`callStatus` in `core.go`: "context canceled" is neither a deadline nor Retryable), and `Core.group` builds rows only when `ask` has returned, so the chunks of a group already answered when the cancel comes are not made into rows.
- Window: the manifest has `stop_after = "1000"` (10:00 ET) but the job only logs a key that starts after it ("key batch-008 started after stop_after 1000"); the run goes on and stores late quotes as the open. This design does not change that (Left out).

### The log of batch-008 on 2026-11-18 (attempt 6 despite a brake of 3)

The scheduler log, lines of `batch-008`: attempt 1 finished 22:40:14 (KL), attempt 4 at 22:48:14, attempt 5 at 22:58:22, attempt 6 at 23:06:31 (finish times), each `outcome=failed error_code=retryable` ("0 of 36 planned items ok, under the share 0.50; 22 rows partial", the last "22 rows not asked"). Attempts 2 and 3 have no finished line; the restarts at 22:42:48 and 22:44:26 each log "closed running runs left by the previous process status=crashed". The reading, from the code:
1. The number is `len(runs)+1`: the reconciler passes `len(byKey[k])+1` to `r.run` (`internal/runner/reconcile/runkey.go`, `reconcileKeys`); the console path computes its own in `backfill_force.go`. It counts every row of the key, finished or crashed. It is the number of rows, not the number the brake counts.
2. The brake counts only rows whose reason is `crashed` or `abandoned_stop` (`isCrash`, `crashBrake` = 3). Before attempt 6 batch-008 had two (attempts 2 and 3). A third crashed row before attempt 4 would have stopped the key; it did not exist, so the brake of 3 was not exceeded.
3. Attempts 4, 5 and 6 ended with a Retryable error before the window close. `nextAttempt` counts such a run for nothing (`beforeClose && isRetryable`), so the two-run cap never filled either and a new run started at every pass. The cap and the brake both behaved as written.
4. Attempts 2 and 3 are `crashed` rows, not drained stops: those two restarts left a `running` row, so the process did not reach the drain that records a cut run. The log has no `batch-008` line after 23:11:58, which fits a third crashed row (restarts at 23:11:56, 23:13:28, 23:15:03) reaching the brake; that is an inference, not a line of the log. Whether a key asked the broker after `stop_after` that day is not shown by the finish times above.
The causes are therefore Decisions 1 and 2 (a run lost its fetched rows at every cut) and Decisions 3 and 4 (the stops of a deploy counted as attempts and crashes, drained or not).

## Decisions

1. Revision 2 of Decision 8 of `option-open-pull.md` (this text replaces that Decision whole; changed: where the rows are written and which statuses are asked again; every other word is as it was):

8. Final status per item, exactly one row per planned item, written to the store after each group of the run, final or not, and asked again after a restart only when its stored status is not final (see below); the key and the writer are as before: 1 `ok` (bid, ask and model IV), 2 `partial` (bid and ask without the model IV, or any other part of them, kept), 3 `no_data` (nothing came), 4 `not_listed`, 5 `no_subscription`, 6 `deadline`, 7 `deferred` (over a ceiling or a line budget, or outside the band chosen at the open spot: not asked, `outside_band` set, Decisions 3 and 12), 8 `error`, 9 `disconnected`. Final statuses are 1 `ok` and 4 `not_listed`: a stored final row is never asked again for its day and slot. Every other status (2, 3, 5, 6, 7, 8, 9) is asked again when the key runs again, whether the run is a restart, a retry or the second run of the runner; a `no_subscription` is not final because error 354 can be a moment of the market data session, and asking it again costs one quick answer. Order of `ok` rows: grouped by stock, then expiry, so that the spot of the stock sampled at the start and at the end of a group bounds the time skew (option-trader: a slow group or a spot move is flagged incoherent): the row carries `underlying_price` and `underlying_event_ts` of the group's end; a spot that moved 1 percent within a group sets the `skew` column of the group's rows, which the page prints ("skew"). `outside_band` is set by the pull core on a stored candidate that the band at the open spot leaves out; that row is not asked and has status `deferred` (Decision 12). Run summary: at the end of each run the pull core logs and returns one line, items, `ok`, `partial` and the seconds from the first `requested_at` to the last `received_at` (derived from the rows, no table), so the first pull measures itself. A `partial` that ended at the timeout is retried once with the classes of Decision 10, a `no_data` is not retried.

2. Write, read, drain (the part of the ruling for `option_open`, module jobs):
   - Writer: `Core` gets an optional `Flush func(ctx context.Context, rows []contracts.OptionOpen) error`. `Core.Pull` calls it with the rows added since its last call after each group (a stock and an expiry, the unit in which rows are built, because `skew` and the underlying price of a row are known only at the end of its group; a group is a few chunks of `chunk` 30 contracts at most), after each stock (for the rows of items not asked: deferred, error, the band) and once more at the end of `Pull`. Rows of every status are written, final or not: the upsert guard of Today makes that safe, and a stored `not_listed` row is what lets the next run skip the item. A flush error ends the run Retryable and `Pull` asks no more. A `Core` with no `Flush` (the refresh, the JP pull) behaves as before. `pull.runKey` sets `Flush` to `InsertOptionOpen` and no longer writes at the end; the idempotent write of the same item twice is the upsert.
   - Reader: before it builds its targets, `pull.runKey` reads the rows stored for the items of the batch for the run's day and the manifest slot through `OptionOpenSlot` (Interface), through `PullDeps.Reader` (new, optional) else the Writer when it also is one. An item whose stored row has a final status (1 `ok`, 4 `not_listed`) is not asked and its stored row is returned with the new rows; an item with no stored row or a stored status 2, 3, 5, 6, 7, 8 or 9 is asked. `planned`, `ok`, `Returned` and the postcondition `min_ok_share` count the whole batch, stored rows included. A read error ends the run Retryable. `NewPull` is refused (an error, as for a missing port) when neither `Reader` nor the Writer reads, so a wiring that cannot resume never runs silently; `NewPullJP` is not changed. A console re-run of a stored day resumes in the same way: it asks only the items not final, so the first quote stored as the open is never replaced by a later one.
   - Drain: when `ctx` is cancelled `Core` asks nothing more; an item whose call ended with the cancel gets no row (no `error` row for a broker call that was only cut); the chunks of the group in hand that were answered before the cancel are made into rows (no end-of-group spot sample: the row keeps the quote's own underlying price and `skew` false, the one value the cut leaves) and joined to the rows in hand; all of them are written by one final `Flush` with a context that is not cancelled (`context.WithoutCancel`) and a limit of 3 s, inside the 5 s `killGrace` that follows the cancel at the end of the 30 s `drainGrace` (the ticker keeps running chunks and writing groups during those 30 s, so a cut loses only the chunk whose call was in flight). The run then ends as the runner records a cut.
3. Ordered stop (the generic runner rule, module runner; amends Decision 5 of `single-runner.md`, Decision 7 of `scheduler.md` and `docs/contracts/runner.md`): a run ended by an ordered stop counts toward neither the two-run cap nor `crashBrake`. The ordered stop is the drain of the ticker when the loop ended for a stop that was asked: a SIGTERM or interrupt (`runErr` nil) or a replaced executable (`ErrExecutableChanged`). At the end of the grace the ticker cancels the worker with the cause `shared.ErrOrderedStop` (new, `internal/runner/shared`); for a lost fence (`ErrFenceLost`) or a frozen loop it cancels with a plain cancel as now, because those are not a deploy. `RunKey` reads `context.Cause` of its run context as it reads the console causes; a run cut with that cause gets the reason `ordered_stop` (new), with the status it gets today (`partial` when rows were returned, `abandoned` when none). `nextAttempt`, `closeRow` and `sched/gaps.go` skip an `ordered_stop` run as they skip a `console_stop` run: it is no attempt, no strike, no gap and hides no earlier run. `abandoned_stop` stays the reason of a cut with any other cause (the run bound, a lost fence, a frozen loop) and counts toward the brake as now. Which key goes on is as before: a key whose run ended `ordered_stop` has no counted run, so the next pass runs it, and with Decisions 1 and 2 that run asks only what is missing.
4. A restart that killed the process before it could record its stop (module ot): a run found still `running` at start is closed `crashed` and counts toward the brake only when the previous process was of the same build as this one (a crash, a reboot, a panic loop). When the build recorded in `ops.scheduler_state` by the previous process (`SchedulerStatus.Build` of `SchedulerLedger.Status`) is not empty and differs from this build, the previous process was replaced by a deploy, and `closeRunningAtStart` closes the rows with the reason `ordered_stop` (Decision 3 reads it as no attempt and no strike). An empty build, an unreadable one or an equal one keeps `crashed`, so the brake still stops a build that crashes at every start. `closeRunningAtStart` reads the build before `runScheduler` records this one (`SetState`), which it already does in this order. Decision 5 of `single-runner.md` ("a deploy mid-run therefore counts toward `crashBrake` exactly as a lapsed lease does") is true only for a restart of the same build from now on.

## Modules

No row of `docs/design/modules.md` changes: `openquote` is already an entry of the store row, `internal/runner/shared` is in the runner paths, and `cmd/ot` is the ot row. Paths of the entries are in rows that exist: store (`internal/store/**`; entries 1 and 2), jobs (`internal/jobs/**`; entry 3), runner (`internal/runner/**`, `docs/contracts/runner.md`; entry 4), ot (`cmd/**`; entries 5 and 6). `openquote.Stores` is not changed: `cmd/ot/wire_optionopen.go` implements it by hand in `noOpenStore`, so a new method in `Stores` would break the build of `cmd/ot` at the interface entry; the reader is a separate interface and entry 5 gives `noOpenStore` the method.

## Interface

Entry 1 changes `internal/store/openquote/api.go`:

- `OptionOpenSlotReader` (new): `OptionOpenSlot(ctx context.Context, day contracts.Day, slot string, instrumentIDs []int64) ([]contracts.OptionOpen, error)`. It returns the rows stored for the session day and the slot whose instrument id is in the list, one for each instrument (the newest by `received_at` if the table ever held two), ordered by instrument id; an instrument with no row gives none; no ids make no database call; a slot of `SlotRefresh` is a Permanent error; instants are UTC; it reads through `rd.option_open_r1`.
- `Stores` is unchanged. The store that `optionopenpg.Open` returns implements the new interface (entry 2), and `NewPull` finds it on the Writer by a type assertion when `PullDeps.Reader` is nil.
- The doc of `OptionOpenWriter.InsertOptionOpen` is corrected to the upsert of Today (rank 2, 1, 0 and the guard in one sentence); the method and its behaviour are unchanged.

## Issues

Each entry is one module; none spans two. Interface entries land first. All entries: `Plan: P1`, `Design: docs/design/restart-resume.md#<n>`, `Contracts: unchanged`. The checked head is `origin/main` `6b7110d8`; no migration is added. `internal/jobs/optionopen/pull_test.go` has 496 lines, `core.go` 613, `internal/runner/reconcile/runkey.go` 763 (an edit there stays small: it stays under 800); the new logic of entry 3 goes in new files. Order: 1 first and 4 first; 2 and 5 after 1; 3 after 1, 2 and 5 (a `NewPull` that refuses a Writer that cannot read needs the real store and the stand-in to read); 6 after 4 (it writes the reason that entry 4 makes a no-strike).

### 1 openquote-slot-reader (interface)

Module: store. Size: S. Order: first.
Files: `internal/store/openquote/api.go`, `internal/store/openquote/api_test.go`
The declaration of the Interface section and the corrected doc, with a test of the contract from both sides.

### 2 optionopenpg-slot-reader

Module: store. Size: S. Order: after 1.
Files: `internal/store/optionopenpg/queries.go`, `internal/store/optionopenpg/slot_test.go`, `internal/store/optionopenpg/slot_pg_test.go`
`OptionOpenSlot` over the view, and the compile-time check that the store implements `openquote.OptionOpenSlotReader`.

### 3 option-open-resume

Module: jobs. Size: L. Order: after 1, 2 and 5.
Files: `internal/jobs/optionopen/core.go`, `internal/jobs/optionopen/core_flush.go`, `internal/jobs/optionopen/pull.go`, `internal/jobs/optionopen/pull_resume.go`, `internal/jobs/optionopen/pull_test.go`, `internal/jobs/optionopen/core_flush_test.go`, `internal/jobs/optionopen/pull_resume_test.go`
`Flush` after each group and stock and at the end, the read of the stored rows of the slot at the start, the final write on a cancel with the answered chunks of the group in hand (Decision 2). `pull_test.go` changes because its fake gains `OptionOpenSlot` (it answers from what was written) and every test that reads `s.written` counts the rows written in total instead of one batch.

### 4 runner-ordered-stop

Module: runner. Size: M. Order: first (it touches no path of entries 1 to 3).
Files: `internal/runner/shared/orderedstop.go`, `internal/runner/sched/ticker.go`, `internal/runner/sched/gaps.go`, `internal/runner/reconcile/runkey.go`, `internal/runner/reconcile/reconcile.go`, `docs/contracts/runner.md`, `internal/runner/shared/orderedstop_test.go`, `internal/runner/sched/ticker_orderedstop_test.go`, `internal/runner/sched/gaps_orderedstop_test.go`, `internal/runner/reconcile/orderedstop_test.go`
The cause chosen by the reason of the stop, the reason `ordered_stop` and its three readers (Decision 3), and the places of the contract. No existing test changes: a cut with a plain cancel stays `abandoned_stop` (`TestRunKeyCutWithoutRowsIsAbandonedStop`, `TestUnmarkedRunKeyCutIsAbandonedStop`).

### 5 wire-slot-reader

Module: ot. Size: S. Order: after 1.
Files: `cmd/ot/wire_optionopen.go`, `cmd/ot/wire_optionopen_slot_test.go`
`noOpenStore` gets `OptionOpenSlot` (it answers its error, as its other methods do) and a compile-time check that it is an `openquote.OptionOpenSlotReader`, so the held-job stand-in still builds the job once `NewPull` needs a reader.

### 6 scheduler-previous-build

Module: ot. Size: M. Order: after 4.
Files: `cmd/ot/scheduler.go`, `cmd/ot/scheduler_close_test.go`, `cmd/ot/scheduler_build_test.go`
`closeRunningAtStart` reads the build of the previous process and chooses the reason (Decision 4). `scheduler_close_test.go` changes only if its fake scheduler ledger must answer `Status`; its assertion of the reason `crashed` stays valid because that fake answers an empty build.

## Checked by CI

- `ci-go`: `go build ./...`, `go vet ./...`, `go test ./internal/store/... ./internal/jobs/optionopen/... ./internal/runner/... ./cmd/ot/...`; the `pg` tests of entry 2 run in GitHub CI only.

## Left out

- `option_eod` is the next job to read for the same rule (it also writes once at the end of its run); this design does not touch it, nor any job but `option_open`. The Japan pull (`NewPullJP`) shares `Core` and keeps writing at the end of its run until a later entry sets its `Flush`.
- The pull window (`stop_after`) is not enforced: a first draft of this file stopped a key at 10:00 ET (`Until`, `AFTER_WINDOW` rows). It is left out because it is a time guard (a past day and an owner re-run after 10:00 ET would store no quote), against the owner rule of 2026-11-12 that a job runs when triggered, and because it is not part of the owner's goal. A resumed run after 10:00 ET asks only the items still missing and stores what comes, as a retry does today; whether to bound that is a design of its own, for the advisor.
- `docs/deploy/scheduler.md` (the restart section and the SIGTERM paragraph) says a cut run ends `partial` (counted) or `abandoned` with reason `abandoned_stop` (not counted); after entries 4 and 6 a drained stop ends `ordered_stop` and `abandoned_stop` is counted toward the brake as the code does today. The file is not in an entry here; it needs its own docs issue.
- Which signal and delay crew uses at a restart is not in the repository (`docs/deploy/scheduler.md` names SIGTERM and the exit 75 of a replaced binary only). Decision 4 does not depend on it: a restart of another build is no strike whether the process drained or was killed.
- The two earlier files (`option-open-pull.md`, `single-runner.md`, `scheduler.md`) are not edited; this file is read with them.
- Why batch-008 had 22 partial rows and no ok row (a data question of the broker at the open, not of restarts) is not answered here.
- A resumed run does not rebuild the band of a stock whose rows are final: it asks only the items still missing, at the spot of the moment.

## Adversarial

The raw review is `adversarial-raw.md` in the draft directory of this design (six questions, with file and line). Each point and the answer:

- Gap: a restart that kills before the drain still counts as a crash. Adopted: Decision 4 and entry 6 (previous build), because the design's own log reading shows the restarts of 22:42 and 22:44 left `crashed` rows.
- Gap: the answered chunks of a group cut by the drain were lost. Adopted: Decision 2 Drain and its test in entry 3.
- Biggest unnamed risk: `noOpenStore` in `cmd/ot` implements `openquote.Stores` by hand, so a method added to `Stores` breaks the build at entry 1, and a `NewPull` that refuses a Writer that cannot read would refuse the stand-in. Adopted: `Stores` is unchanged, entry 5 gives the stand-in the method, entry 3 is ordered after it; the Modules text no longer says `cmd/ot` is not edited.
- Biggest unnamed risk: the window guard is a time guard against the owner's dev-mode rule and blocks a re-run and a past day. Adopted: Decision 3 of the first draft and its entry are removed (Left out).
- Simpler way: drop the window (adopted, above); for restarts, one rule on the build instead of a ledger column (adopted as Decision 4). Not adopted: cancelling the worker at once on SIGTERM instead of after 30 s, because other jobs are not resumable.
- Lock-in: `no_subscription` as a final status would keep a moment of error 354 for the day. Adopted: final statuses are 1 and 4 only. `ordered_stop` as a reason code is additive and stays. A console re-run resumes: said in Decision 2.
- Claim most likely false: "`cmd/ot` is not edited" (corrected, above); "the cancel for a lost fence keeps a plain cancel" (the code waits and cancels for a lost fence too, so the cause is chosen by the reason of the stop: Decision 3); the cite of the attempt number (corrected to `reconcileKeys`); the reading of 23:06 and 23:11 as starts (they are finish times; item 4 of the log reading is softened); "both stay true" of `docs/deploy/scheduler.md` (not true for the drain; named in Left out). Checked and true: the upsert guard and ranks, the reader across slots, `drainGrace` and `killGrace`, `nextAttempt` and `closeRow` skipping `console_stop`, statuses 8 and 9 asked again without a broker call for a `Problem` target.
- Issue cut: entry 3 named five tests that read `s.written` and missed `TestItemWithoutAStoredContractEndsError`, `TestPullPastDayRunsToday`, `TestRunReadsThePlanOfItsGivenDay`, `TestRunFallsBackToTheClockDatePlan` and the retry test that reads `s.written[1]`. Adopted: the acceptance now lists them by the command that finds them. Order lines and the interface entry first are as the Issues section says.
