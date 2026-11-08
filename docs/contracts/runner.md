# runner contract

## Responsibility

For each session day after the reconcile cursor, up to the requested day, the runner materialises the window of every dataset the registered jobs plan, reconciles expected keys against present keys, starts the repair and recovery runs, and advances the cursor over fully reconciled days. It talks to storage only through `store.OpsLedger`. Keys (`contracts.ExpectedKey`) are opaque; it never reads dataset content.

## Inputs

- `store.OpsLedger`, `clock.Calendar`, `clock.Clock` and the registered `[]contracts.Job` (`runner.NewFunc`, built by `internal/runner/reconcile.New`).
- The day: `Reconcile(ctx, day)` with a `contracts.Day`.
- A window rule: `runner.WindowRule` (calendar id, open offset, close offset), taken by `runner.NewWithRuleFunc` (built by `internal/runner/reconcile.NewWithRule`). `New` uses the default rule (calendar `XNYS`, no offsets).
- Per job: `Plan(ctx, day)` gives the expected keys and a plan hash.

## Outputs

- Through the ledger: the expected keys of a day (`PutExpected`, only when the merged plan hash changed), one `store.Window` per dataset and session day (`PutWindow`), the runs (`RecordRun`), the present keys (`MarkPresent`) and the cursor (`SetCursor`).
- `Run.ID` is `day|dataset_id|subject_key|attempt`, deterministic.
- Errors from the ledger or from `Plan` are returned; a gap is not an error, it holds the cursor.

## Invariants

- State machine of a key on a day: *missing* (no present row) → one repair run (attempt 1, `running` then `done`) → *present* or *no_value*; only an explained absence is final. A `NoValue` error is `no_value` at any time. An empty result before the window's `CloseAt` is missing: the key gets no mark, the run is recorded with reason `empty_before_close` and does not count toward the cap of two runs, so the key is fetched again by a later `Reconcile`. An empty result at or after `CloseAt` is `no_value`. The close is that of the key's dataset window; a key whose dataset has no window keeps the old rule (empty is `no_value`). Any other error leaves the key missing. The cursor holds on a missing key. A `running` run whose `LeaseUntil` is before the clock's now is crashed: it is closed (`done`, reason `crashed`) and re-run once (attempt 2). A `running` run with a live lease is pending: nothing is started. At most two counting runs per key and day (repair and crash re-run); a key still missing after them is an unresolved gap.
- A `done` run with a retryable error before the window's `CloseAt` does not count toward the cap, and the next call runs the key again; at or after `CloseAt` the missing key becomes `no_value` with the reason `expired` (the last run is recorded again with it), no new run starts, the day completes, and `ot backfill` can repair it behind the cursor. A `crashed` run is not covered by this and still counts.
- Window rule: for a session day the window opens at the calendar's `OpenAt` plus the open offset and closes at the calendar's `CloseAt` plus the close offset; a non-session day (Thanksgiving) gets no window; an early close is carried by the calendar's `CloseAt` (13:00 ET). No open or close time is written in Go.
- Window per job: the window of a dataset is built from the job that planned its keys (the first job to plan a dataset wins). A job that implements `runner.Windowed` (`Window() (calendarID, openOffset, closeOffset)`, plain values) gets that rule; a job without it gets the reconciler's `WindowRule`. An empty calendar id means the reconciler's calendar. Lateness counts from the session close: on an early close the offset is added to the early `CloseAt`, so an offset must hold on every session day.
- Calendar mismatch: the day loop has one calendar, the reconciler's (default `XNYS`). `Reconcile` first walks the jobs; a job whose calendar id differs makes it return an error naming the job id, before `Acquire`, `PutExpected` or `PutWindow`: nothing is held or written.
- A run that starts after the window's `CloseAt` is late (reason `late`); its keys are reconciled all the same.
- A day is fully reconciled when every expected key is `present` or `no_value`. The cursor advances only over fully reconciled days, in order, and stops at the first day with a gap or a pending run.
- The run lease (`Acquire`/`Release`) is held for the whole `Reconcile` call.
- No `time.Now` or `time.Sleep`: time comes from the injected `clock.Clock`, and from a `clockwork.Clock` where a timer is needed.
- The root package `internal/runner` holds only `api.go` and `api_test.go`; it depends on `contracts`, `clock`, `store` and `clockwork` (`github.com/jonboulle/clockwork`, because `NewSchedulerFunc` takes a `clockwork.Clock`) only, never `internal/jobs` or pgx.

## Backfill

`Backfill(ctx, day, datasets)` (`reconcile.NewBackfillerWithRule`) re-runs the keys of one day at or behind the cursor that are missing or marked `no_value`, under the same lease as `Reconcile`. It reads the stored plan (`Expected`) and calls `Plan` of the jobs only to find the job of each key. The cursor, the plans and the windows are untouched: `SetCursor`, `PutExpected` and `PutWindow` are never called. A day after the cursor, or with an empty cursor, is refused (`RefusedAheadOfCursor`); a day with no stored plan is refused (`RefusedNoPlan`); a refusal is data, with a nil error and no write. A `present` key is not run (`AlreadyPresent`). Each run row has the reason `backfill`; the result lists the keys as `Filled`, `Absent` (`no_value`) or `Missing` (no usable answer). Tests: `internal/runner/reconcile/backfill_test.go`.

## Key runner

`reconcile.NewKeyRunner(ops, sched, clk)` (`runner.NewKeyRunnerFunc`) builds the one path that runs a job for one key; `Reconcile` and `Backfill` call it for every key, so a tick and a hand backfill cannot differ, and `job.Run` is called nowhere else. `RunKey` writes the run row as `running` with a heartbeat, then renews `heartbeat_at` and `lease_until` every 30 s of the injected `clockwork.Clock`. The run context ends at `deadline`; `Reconcile` and `Backfill` give the window close, plus `RunBound` of the optional `Bounded` method (default 30 minutes); a dataset with no window has no close, so its deadline is now plus the bound. A deadline that is not after the start of the run cuts nothing. A run cut at the deadline or by a cancel (a drain) is never marked present: with `Returned > 0` it ends `partial` (a counted verdict, the key stays missing), with `Returned = 0` it ends `abandoned` with reason `abandoned_stop`. The row is written after the cut with a context that outlives the cancel. A job with the optional `Postcondition` is asked after a run that was not cut and ended without an error: the key is marked present only on `true` with a nil error; `false` or an error leaves a `done` run without a mark. A job without it keeps the rule of v3 (usable rows mark present).

`Reconcile` and `Backfill` hold `ops.run_lease` and, when built by `reconcile.NewReconciler(ops, sched, cal, clk, jobs, rule, kr)` (`runner.NewReconcilerFunc`), renew it through `store.SchedulerLedger.RenewLease` every 30 s while they run; a failed renewal cancels the work and is the error of the call. The lease is released by `defer` with a context that outlives a cancel. `New`, `NewWithRule` and `NewBackfillerWithRule` stay for the commands and renew nothing. Tests: `internal/runner/reconcile/runkey_test.go`.

## Version history

- v1: window materialisation, repair, crash re-run, cursor.
- v2: `Backfill` of a day at or behind the cursor.
- v3: a retryable run is retried until `CloseAt`, then expired (v2 is the `Backfill` line).

## Contract tests

- `internal/runner/api_test.go`: the interface from both sides.
- `internal/runner/reconcile/window_test.go`: `TestWindowThanksgivingAndEarlyClose`, `TestWindowRuleOffsets`, `TestWindowPerJob`, `TestWindowedJobWithoutMethod`, `TestWindowCalendarMismatch`, `TestEmptyRetriedUntilOwnClose`, `TestReconcileEarlyCloseOptionWindow`.
- `internal/runner/api_test.go`: also `TestWindowRuleContract`; `reconcile_test.go`: also `TestNewWithRuleEmptyCalendar` and `var _ runner.NewWithRuleFunc = reconcile.NewWithRule`.
- `internal/runner/reconcile/reconcile_test.go`: `TestReconcileRepairNoValueCrash`, `TestFullCycleLateRun`, `TestCursorHoldsOnGap`, and `var _ runner.NewFunc = reconcile.New`.
- `internal/runner/reconcile/retry_test.go`: `TestRetryableBeforeCloseRetries`, `TestRetryableAtOrAfterCloseExpired`, `TestExpiredDayCompletesCursorAdvances`, `TestCrashedRunIsNotExpired`.

## Jobs and slots

Declared in `internal/runner/api.go`, declarations only; `contracts.Job` is not edited.

- `Slot{JobID, CalendarID, Anchor, Zone string; Offset time.Duration}`: the plain schedule of one job.
- Optional methods of a job, standard-library types only so a job package imports no runner type: `Scheduled` (`Schedule() (calendarID, anchor, zone string, offset time.Duration)`), `Bounded` (`RunBound() time.Duration`), `Postcondition` (`Postcondition(ctx, day, datasetID, subjectKey string) (bool, error)`: whether the rows of the key exist now) and `RestartSensitive` (`RestartSensitive() bool`). A job without a method gets the default.
- `KeyRunner.RunKey(ctx, job, day, key, attempt, closeAt, deadline, reason)` runs one key through the job; the run context ends at `deadline`.
- `Scheduler` (`Run`, `LastTick`, `FenceHeld`): a library may implement it. `LastTick` is the instant of the last loop turn, or of construction before the first; `FenceHeld` is false once the fence is lost.
- `GapSyncer.Sync(ctx, day, disabled)`: `disabled` maps the id of a left-out job to its reason, nil for none.
- `ExecIdentityFunc` and the constructor types `NewSchedulerFunc`, `NewKeyRunnerFunc` and `NewReconcilerFunc`.

`Prober` (`Probe(ctx, jobID) (reason string)`, Decision 8) is an optional interface of the `Reconciler` given to `NewSchedulerFunc`; without it there is no re-probe. The ticker owns the `disabled` map (job id to reason): it copies the map it is given, and gives `GapSyncer.Sync` a fresh copy on every call, so `Sync` may keep or read it without a race. On every tick, before `Reconcile`, the ticker calls `Probe` for each job in the map, on the same worker goroutine as `Reconcile`, so a probe never overlaps a `Reconcile`. An empty reason removes the job from the map; a non-empty one keeps it with that reason.

Fixed run reasons (Decision 7 of `docs/design/scheduler.md`):

- `crashed`: the reconciler's mark on a `running` run whose lease lapsed.
- `abandoned_stop`: a run cut at a drain with no rows returned.
- `crashed_repeatedly`: written on the run at the close, after three runs of one `(day, dataset, subject)` ended `crashed` or `abandoned_stop`.

Counting: only `crashed` and `abandoned_stop` runs count toward the brake of three; `empty_before_close` runs and retryable runs before the close count for nothing. `Reconcile` counts the runs of the key that carry these reasons and starts no new run at three; the gap syncer counts the same runs through `store.OpsLedger` and writes the gap at three, and treats a run with `crashed_repeatedly` as proof of the stop.

## Forbidden

- Reading dataset content or interpreting a key.
- Importing `internal/jobs` or a database driver; touching ops tables except through `store.OpsLedger`.
- Hand-written open or close times, holiday lists or half-day rules.
- Reading the system time.
- More than one repair run and one crash re-run per key and day (retryable runs before the close are not counted).
