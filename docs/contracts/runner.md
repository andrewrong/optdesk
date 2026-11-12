# runner contract

## Responsibility

For each session day after the reconcile cursor, up to the requested day, the runner materialises the window of every dataset the registered jobs plan, reconciles expected keys against present keys, starts the repair and recovery runs, and advances the cursor over fully reconciled days. It talks to storage only through `store.OpsLedger`. Keys (`contracts.ExpectedKey`) are opaque; it never reads dataset content. Keys are planned for every job of a day from the start of the walk; a `runner.Scheduled` job is held, not started, until its slot instant on the day (below), and a day with a held key is not done.

## Inputs

- `store.OpsLedger`, `clock.Calendar`, `clock.Clock` and the registered `[]contracts.Job` (`runner.NewFunc`, built by `internal/runner/reconcile.New`).
- The day: `Reconcile(ctx, day)` with a `contracts.Day`.
- A window rule: `runner.WindowRule` (calendar id, open offset, close offset), taken by `runner.NewWithRuleFunc` (built by `internal/runner/reconcile.NewWithRule`). `New` uses the default rule (calendar `XNYS`, no offsets).
- Per job: `Plan(ctx, day)` gives the expected keys and a plan hash. A job that implements `runner.Scheduled` also gives its schedule (calendar id, anchor, zone, offset); the instant of its slot on a day is `shared.At` (`internal/runner/shared`), the same function `sched` uses to wake a turn.

## Outputs

- Through the ledger: the expected keys of a day (`PutExpected`, only when the merged plan hash changed), one `store.Window` per dataset and session day (`PutWindow`), the runs (`RecordRun`), the present keys (`MarkPresent`) and the cursor (`SetCursor`).
- `Run.ID` is `day|dataset_id|subject_key|attempt`, deterministic.
- Errors from the ledger or from `Plan` are returned; a gap is not an error, it holds the cursor.

## Invariants

- State machine of a key on a day: *missing* (no present row) → one repair run (attempt 1, `running` then `done`) → *present* or *no_value*; only an explained absence is final. A `NoValue` error is `no_value` at any time. An empty result before the window's `CloseAt` is missing: the key gets no mark, the run is recorded with reason `empty_before_close` and does not count toward the two-run cap, so the key is fetched again by a later `Reconcile`. An empty result at or after `CloseAt` is `no_value`. The close is that of the key's dataset window; a key whose dataset has no window keeps the old rule (empty is `no_value`). Any other error leaves the key missing. The cursor holds on a missing key.
- Counting (v4, Decisions 7 and 8). A run counts when it is a verdict: `partial` (`store.RunPartial`), or `done` without a mark (a `Permanent` error, a `Postcondition` that is false or fails). A `partial` run does not mark the key present, and it is the only verdict that earns a second run. Start rule of `nextAttempt`: a run starts for a missing key when no run of it is `running` with a live lease, fewer than two runs count (at most two counted runs), the key is not stopped by the brake, and either no run counts or the latest counted run is `RunPartial`. Any other verdict (`Permanent`, `done` without a mark, `NoValue`) ends the key with no second run. Not counted: `empty_before_close` runs, retryable runs before the `CloseAt`, and `crashed` or `abandoned_stop` runs. A `running` run whose `LeaseUntil` is before the clock's now is closed (`done`, reason `crashed`) and the key goes on by the same rule; a `running` run with a live lease is pending and nothing starts.
- Brake: three runs of one `(day, dataset, subject)` that ended `crashed` or `abandoned_stop` stop the key with reason `crashed_repeatedly`: no new run before the close, the cursor holds, and the Close table below decides at the close. Only these two reasons count toward the brake. This is the "three" that sits next to the two-run cap.
- Close table (Decision 8). At or after `CloseAt`, `expireIfClosed` looks at a key that is still missing and has no `running` run, and takes the first row that matches; every row makes the key `no_value`, so the day completes and the cursor advances. A key that matches no row is not closed and `nextAttempt` decides: a key with no counted run (no run yet, only `empty_before_close` runs, or one or two `crashed` or `abandoned_stop` runs) gets a late run (reason `late`); a key with a `running` run waits for its lease. A dataset with no window has no close and no row applies.

| # | Case | Run whose reason is rewritten | Status and reason after | Gap kind |
|---|------|-------------------------------|-------------------------|----------|
| 1 | three runs ended `crashed` or `abandoned_stop` (the brake); wins over row 2 | the latest of those runs | key `no_value`; run reason `crashed_repeatedly` | `crashed_repeatedly` |
| 2 | else the latest counted run is `RunPartial` | that `partial` run | key `no_value`; run reason `expired`; run status stays `partial` | `partial` |
| 3 | else the last run is retryable and not `crashed` | the last run | key `no_value`; run reason `expired` | `expired` |
| 4 | else the latest counted run ended `done` without a mark (`Permanent`, or a false or failing `Postcondition`) | that run | key `no_value`; run reason `expired` | `expired` |

  A key closed with reason `expired` or `crashed_repeatedly` is repaired by hand: `ot backfill` of the day, which needs no cursor and no stopped scheduler; the gap clears when the key is present, or `no_value` with another reason.
- A retryable run before the `CloseAt` does not count, and the next call runs the key again; this is row 3 at the close, and `ot backfill` can repair the key behind the cursor. A `crashed` run is not a retryable answer and is never expired by row 3; v4 reverses v3 here: a crashed run no longer counts.
- Window rule: for a session day the window opens at the calendar's `OpenAt` plus the open offset and closes at the calendar's `CloseAt` plus the close offset; a non-session day (Thanksgiving) gets no window; an early close is carried by the calendar's `CloseAt` (13:00 ET). No open or close time is written in Go.
- Window per job: the window of a dataset is built from the job that planned its keys (the first job to plan a dataset wins). A job that implements `runner.Windowed` (`Window() (calendarID, openOffset, closeOffset)`, plain values) gets that rule; a job without it gets the reconciler's `WindowRule`. An empty calendar id means the reconciler's calendar. Lateness counts from the session close: on an early close the offset is added to the early `CloseAt`, so an offset must hold on every session day.
- Calendars: the calendars of a reconciler are the distinct calendars of its jobs (a job without `Window`, or with an empty calendar id, uses the reconciler's). `Reconcile` walks the union of the session days of those calendars after the cursor up to the day, in order; the cursor advances over the complete prefix of that one list. On a day a job is planned only when its own calendar has a session that day, and the window of each dataset is built from the session of the calendar of the job that planned it. A reconciler of one calendar walks and writes as before.
- Due gate: a `Scheduled` job is due on day D at its slot instant on D (the session `OpenAt` or `CloseAt` plus offset, or the offset since midnight of D in its zone). Before that instant `plan` still plans its keys, job and hash (so `PutExpected` runs once), but the keys are held: `reconcileOn` starts no run, writes no `MarkPresent` and no run row for them, sets the day not done, and the cursor stays before D. A day whose instant has passed, which includes every past day, is not held; a job that is not `Scheduled` is never held; `Backfill` ignores the hold. An unknown anchor, or an unknown or empty zone for `time`, is an error of `Reconcile` naming the job.
- Pinned tests of the due gate (`internal/runner/reconcile`): `TestCursorHoldsOnGap`, `TestWalkPastHeldDay`, `TestFullCycleLateRun`, `TestCursorAdvancesOverCompletePrefixAcrossCalendars`, `TestUnionWalksBothCalendars`, `TestJobNotPlannedOnOtherCalendarHoliday`, `TestCalendarRunsOutDropsOnlyItsJobs`, `TestSingleCalendarUnchanged`, `TestEmptyBeforeCloseStaysMissing`, `TestNoValueErrorFinalBeforeClose`, `TestRetryableBeforeCloseRetries`, `TestExpiredDayCompletesCursorAdvances`, `TestReconcileEarlyCloseOptionWindow`; they use jobs that are not `Scheduled` and pass unchanged.
- Known calendars: when the reconciler is built (and when the ticker is built) it asks `Session` of each job calendar other than its own; those that answer are its known calendars. A known calendar that later answers `UnknownCalendar` or `NoValue` has its jobs dropped for the days it cannot answer, on every walk, also one that starts after its last row, with one log line per walk (`calendar <id>: <error>, jobs dropped`); `Reconcile` returns nil and the jobs of the other calendars run. An error of the reconciler's own calendar, and the error of a calendar that was not known, is returned (naming the job for the latter). The ticker's `walked` uses the same union of the session days, and its slot walk (`nextStarts`) drops the slots of a known calendar the same way; the exported `sched.NextStarts` drops nothing and returns every calendar error naming the job.
- A run that starts after the window's `CloseAt` is late (reason `late`); its keys are reconciled all the same.
- A day is fully reconciled when every expected key is `present` or `no_value`. The cursor advances only over the complete prefix, in order: the cursor holds on a gap; the walk does not. A day with a gap or a pending run holds the cursor, and `Reconcile` still reconciles every later session day through the requested day in the same call.
- The run lease (`Acquire`/`Release`) is held for the whole `Reconcile` call.
- No `time.Now` or `time.Sleep`: time comes from the injected `clock.Clock`, and from a `clockwork.Clock` where a timer is needed.
- The root package `internal/runner` holds only `api.go` and `api_test.go`; it depends on `contracts`, `clock`, `store` and `clockwork` (`github.com/jonboulle/clockwork`, because `NewSchedulerFunc` takes a `clockwork.Clock`) only, never `internal/jobs` or pgx.

## Backfill

`Backfill(ctx, day, datasets)` (`reconcile.NewBackfillerWithRule`) re-runs the keys of one stored day that are missing or marked `no_value`, whether or not their window has closed, whatever the cursor says: there is no cursor test. It takes the lease under an owner of its own (`backfill:` plus random hex), so it and `Reconcile` exclude each other. It reads the stored plan (`Expected`) and calls `Plan` of the jobs to find the job of each key. It writes only the missing windows of a stored day and the keys its plan lacks (`PutWindow`, `PutExpected`), and never moves the cursor (`SetCursor` is never called). A day with no stored plan is refused (`RefusedNoPlan`); a refusal is data, with a nil error and no write. `RefusedAheadOfCursor` is declared and never returned (removed later by backfill-drop-refusal). A `present` key is not run (`AlreadyPresent`); a key inside its window is run (`InWindow` is declared and no longer produced). Each run row has the reason `backfill`; the result lists the keys as `Filled`, `Absent` (`no_value`) or `Missing` (no usable answer). `KeyGapSyncer.SyncKeys(ctx, day)` (`sched.NewKeyGapSyncer`, `runner.NewKeyGapSyncerFunc`) recomputes only the key gaps of one day. Tests: `internal/runner/reconcile/backfill_test.go`.

### ForceBackfiller

`BackfillWith(ctx, day, datasets, opt)` is `Backfill` with `BackfillOptions{Force, Job, Hold}`. `Job` narrows the call to the keys of one job (empty: every job of the datasets). `Force` runs keys that are present: a forced run of a present key is a bare run: no run row and no mark. `Hold`, when set, is asked before every key after the first; a true answer ends the call and the rest are listed `not_reached`. A selected dataset with no key on the day is refused as `RefusedNoKey` (`no_key`), as data with a nil error. In development mode (owner decision 2026-11-12), `Backfill` and `BackfillWith` run a key before its slot or inside its window, and `Reconcile` still holds a key before its slot; only the lease (`lease_held`) refuses a key, and `not_due` and `DueAt` are declared and no longer produced. `BackfillResult.Keys` has one `KeyRun` line per key: group (`already_present`, `not_due`, `rerun`, `ran`, `lease_held`, `not_reached`), status, error and reason codes, rows returned and usable, attempt, and the mark after the call. `SlotHold` answers held true for the first start of an enabled job in `(now, now+bound]` or an earlier start not yet marked covered, and held true with an empty job when the schedule is unread.

### ConsoleTurner

`ConsoleTurn(ctx, hold)` is an optional method of the `Reconciler` given to `NewSchedulerFunc`: one turn of the console's requests, run on the worker so it never overlaps a `Reconcile`; `took` is true when it handled a request. The turn has no bound of its own: each key runs under its job's own run bound; a cancelled ctx ends the turn interrupted.

## Key runner

`reconcile.NewKeyRunner(ops, sched, clk)` (`runner.NewKeyRunnerFunc`) builds the one path that runs a job for one key; `Reconcile` and `Backfill` call it for every key, so a tick and a hand backfill cannot differ, and `job.Run` is called nowhere else. `RunKey` writes the run row as `running` with a heartbeat, then renews `heartbeat_at` and `lease_until` every 30 s of the injected `clockwork.Clock`. The run context ends at `deadline`; `Reconcile` and `Backfill` give `max(CloseAt, start) + RunBound`, where `start` is the instant of the injected clock when the run begins and `RunBound` comes from the optional `Bounded` method (default 30 minutes): a run that starts before the close is cut `RunBound` after the close, and a late run or a `Backfill` of a closed day is cut `RunBound` after its own start, so a run is never open-ended and never already expired when it starts; a dataset with no window has no close, so its deadline is now plus the bound. A run cut at the deadline or by a cancel (a drain) is never marked present: with `Returned > 0` it ends `partial` (a counted verdict, the key stays missing and earns a second run), with `Returned = 0` it ends `abandoned` with reason `abandoned_stop`. The row is written after the cut with a context that outlives the cancel. A job with the optional `Postcondition` is asked after a run that was not cut and ended without an error: the key is marked present only on `true` with a nil error; `false` or an error leaves a `done` run without a mark. A job without it keeps the rule of v3 (usable rows mark present).

`Reconcile` and `Backfill` hold `ops.run_lease` and, when built by `reconcile.NewReconciler(ops, sched, cal, clk, jobs, rule, kr)` (`runner.NewReconcilerFunc`), renew it through `store.SchedulerLedger.RenewLease` every 30 s while they run; a failed renewal cancels the work and is the error of the call. The lease is released by `defer` with a context that outlives a cancel. `New`, `NewWithRule` and `NewBackfillerWithRule` stay for the commands and renew nothing. Tests: `internal/runner/reconcile/runkey_test.go`.

## Version history

- v1: window materialisation, repair, crash re-run, cursor.
- v2: `Backfill` of a day at or behind the cursor.
- v3: a retryable run is retried until `CloseAt`, then expired (v2 is the `Backfill` line).
- v4: only a `partial` run counts toward the second run (at most two counted runs); a `crashed` or `abandoned_stop` run no longer counts (reverses v3, where a crashed run still counted), and three of them stop the key with `crashed_repeatedly`; `expireIfClosed` follows the Close table of Decision 8.

## Contract tests

- `internal/runner/api_test.go`: the interface from both sides.
- `internal/runner/reconcile/window_test.go`: `TestWindowThanksgivingAndEarlyClose`, `TestWindowRuleOffsets`, `TestWindowPerJob`, `TestWindowedJobWithoutMethod`, `TestEmptyRetriedUntilOwnClose`, `TestReconcileEarlyCloseOptionWindow`.
- `internal/runner/api_test.go`: also `TestWindowRuleContract`; `reconcile_test.go`: also `TestNewWithRuleEmptyCalendar` and `var _ runner.NewWithRuleFunc = reconcile.NewWithRule`.
- `internal/runner/reconcile/reconcile_test.go`: `TestReconcileRepairNoValueCrash`, `TestFullCycleLateRun`, `TestCursorHoldsOnGap`, and `var _ runner.NewFunc = reconcile.New`.
- `internal/runner/reconcile/retry_test.go`: `TestRetryableBeforeCloseRetries`, `TestRetryableAtOrAfterCloseExpired`, `TestExpiredDayCompletesCursorAdvances`, `TestCrashedRunIsNotExpired`.
- `internal/runner/reconcile/count_test.go`: `TestAbandonedRunNotCounted`, `TestLapsedLeaseNotCounted`, `TestCrashedRepeatedlyStopsKey`, `TestPartialRerunUntilClose`, `TestCloseTableRows`.

- `internal/runner/reconcile/calendars_test.go`: `TestUnionWalksBothCalendars`, `TestJobNotPlannedOnOtherCalendarHoliday`, `TestWindowFromJobCalendar`, `TestSingleCalendarUnchanged`, `TestCursorAdvancesOverCompletePrefixAcrossCalendars`, `TestCalendarRunsOutDropsOnlyItsJobs`; `internal/runner/sched`: `TestWalkedMergesCalendars` (`ticker_calendars_test.go`), `TestSlotsDropRunOutCalendar` (`slots_calendar_test.go`).

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
- `console_stop`: a run cut by a console stop or by the console's own deadline.

Counting: only `crashed` and `abandoned_stop` runs count toward the brake of three; `empty_before_close` runs and retryable runs before the close count for nothing, and `console_stop` runs count for nothing, so they are no crash strike. `Reconcile` counts the runs of the key that carry these reasons and starts no new run at three; the gap syncer counts the same runs through `store.OpsLedger` and writes the gap at three, and treats a run with `crashed_repeatedly` as proof of the stop.

## Forbidden

- Reading dataset content or interpreting a key.
- Importing `internal/jobs` or a database driver; touching ops tables except through `store.OpsLedger`.
- Hand-written open or close times, holiday lists or half-day rules.
- Reading the system time.
- A second run of a key that is not after a `partial` run, or more than two counted runs per key and day; a fourth `crashed` or `abandoned_stop` run (the brake of three).
