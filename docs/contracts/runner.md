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
- A run that starts after the window's `CloseAt` is late (reason `late`); its keys are reconciled all the same.
- A day is fully reconciled when every expected key is `present` or `no_value`. The cursor advances only over fully reconciled days, in order, and stops at the first day with a gap or a pending run.
- The run lease (`Acquire`/`Release`) is held for the whole `Reconcile` call.
- No `time.Now` or `time.Sleep`: time comes from the injected `clock.Clock`.
- The root package `internal/runner` holds only `api.go` and `api_test.go`; it depends on `contracts`, `clock` and `store` only, never `internal/jobs` or pgx.

## Backfill

`Backfill(ctx, day, datasets)` (`reconcile.NewBackfillerWithRule`) re-runs the keys of one day at or behind the cursor that are missing or marked `no_value`, under the same lease as `Reconcile`. It reads the stored plan (`Expected`) and calls `Plan` of the jobs only to find the job of each key. The cursor, the plans and the windows are untouched: `SetCursor`, `PutExpected` and `PutWindow` are never called. A day after the cursor, or with an empty cursor, is refused (`RefusedAheadOfCursor`); a day with no stored plan is refused (`RefusedNoPlan`); a refusal is data, with a nil error and no write. A `present` key is not run (`AlreadyPresent`). Each run row has the reason `backfill`; the result lists the keys as `Filled`, `Absent` (`no_value`) or `Missing` (no usable answer). Tests: `internal/runner/reconcile/backfill_test.go`.

## Version history

- v1: window materialisation, repair, crash re-run, cursor.
- v2: `Backfill` of a day at or behind the cursor.
- v3: a retryable run is retried until `CloseAt`, then expired (the `v2` line of issue #209; v2 was already taken by `Backfill`).

## Contract tests

- `internal/runner/api_test.go`: the interface from both sides.
- `internal/runner/reconcile/window_test.go`: `TestWindowThanksgivingAndEarlyClose`, `TestWindowRuleOffsets`.
- `internal/runner/api_test.go`: also `TestWindowRuleContract`; `reconcile_test.go`: also `TestNewWithRuleEmptyCalendar` and `var _ runner.NewWithRuleFunc = reconcile.NewWithRule`.
- `internal/runner/reconcile/reconcile_test.go`: `TestReconcileRepairNoValueCrash`, `TestFullCycleLateRun`, `TestCursorHoldsOnGap`, and `var _ runner.NewFunc = reconcile.New`.
- `internal/runner/reconcile/retry_test.go`: `TestRetryableBeforeCloseRetries`, `TestRetryableAtOrAfterCloseExpired`, `TestExpiredDayCompletesCursorAdvances`.

## Jobs and slots

None yet. The runner runs the registered jobs one key at a time on the calling goroutine; scheduling slots belong to the follow-up wiring design.

## Forbidden

- Reading dataset content or interpreting a key.
- Importing `internal/jobs` or a database driver; touching ops tables except through `store.OpsLedger`.
- Hand-written open or close times, holiday lists or half-day rules.
- Reading the system time.
- More than one repair run and one crash re-run per key and day (retryable runs before the close are not counted).
