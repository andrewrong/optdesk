# Runner due gate: a job is planned on day D only once D has begun in the job's zone

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-11
Revision: 2

Source: decision of optdesk-advisor, 2026-11-11. Read on origin/main: `AGENTS.md`, `docs/design/modules.md`, `docs/design/runner-window.md`, `docs/design/scheduler.md`, `internal/runner/api.go`, `internal/runner/sched/ticker.go`, `internal/runner/reconcile/**`, the seven jobs named below.

Goal: a job whose guard returns NoValue "wrong day" for a day that has not begun in New York is no longer run on that day, so the New York session is not lost.

## Facts

- `internal/runner/sched/ticker.go` `dayOf` gives the run day in UTC.
- The reconciler has no "not yet" rule: `plan` (`internal/runner/reconcile/reconcile.go:135-141`) plans every job for the UTC date from 00:00Z, and `nextAttempt` (`:280-312`) starts any key that has no run.
- Seven jobs guard with an equality test of the day against their own zone's date and return NoValue "wrong day": `optioneod/run.go:119`, `accountcapture/job.go:104`, `dividendcapture/run.go:212`, `earningsrefresh/run.go:235`, `positionmarks/run.go:122`, `quotesnapshot/run.go:99`, `optionopen/pull.go:208`.
- `runkey.go:139` records NoValue as ok and final. At 00:09Z on date D New York is still on D-1, so the guard answers "wrong day", the key is final, and the session of D in New York is never captured.
- The reconciler has no zone of its own: `Windowed.Window()` returns the calendar id and offsets only, and `internal/clock` calendars carry no zone. The zone is the `zone` value of `Scheduled.Schedule()` (`internal/runner/api.go`), an optional method of the job that the reconciler receives as `contracts.Job`; all seven jobs above implement it.

## Modules

None changed. Only `internal/runner/reconcile`, inside the runner module.

## Interface

None. No method is added to `internal/runner/api.go`; the reconciler type-asserts `runner.Scheduled` on the job it already receives and reads the zone from `Schedule()`.

## Decision

In `internal/runner/reconcile` a job is not planned on day D until D <= the date of the clock in the job's own zone.

- The test uses the reconciler's clock and the zone of the job: the `zone` of `Scheduled.Schedule()`. A job that is not `Scheduled`, or has an empty zone, is tested in UTC, which is today's behaviour. There is no calendar-to-zone map.
- A job that is "not yet" on D is HELD for D: `plan` does not start it and reports the day as held, and `reconcileOn` returns done=false for D, so the cursor stays before D. Skipping the job with a plain `continue` is wrong: the day would then be complete without the job and the cursor would pass D, which is the same loss without even a NoValue row. No key is created for a held job.
- Only `Reconcile` (the cursor walk) holds a job. `Backfill` plans with `plan`, too; a backfill names days that are over, and a held job there is a bug to test, so the gate takes effect only for a day not before the zone date and is covered by a backfill test of today.
- On a later pass, once D has begun in the zone, the job is planned and run as today.
- A job whose zone date has reached D, and any day before today in the zone, is planned as today. A past day is unchanged.
- Jobs of other zones on the same UTC date are not held back: at 00:09Z an XTKS job is planned on the UTC date because Tokyo is already on that date, while a NY job of the same date is not.

### Tests that pin today's behaviour

All in `internal/runner/reconcile/*_test.go`. The tests below use a fake clock at or after the end of the day they reconcile, so every job is already due on D and they keep passing unchanged; the issue reads each and changes a test only where its clock is before the zone date of a job.

- `TestCursorHoldsOnGap` (`reconcile_test.go`), `TestWalkPastHeldDay`, `TestFullCycleLateRun`: a gap holds the cursor. They stay; "not yet" is one more reason for a gap.
- `TestCursorAdvancesOverCompletePrefixAcrossCalendars`, `TestUnionWalksBothCalendars`, `TestJobNotPlannedOnOtherCalendarHoliday`, `TestCalendarRunsOutDropsOnlyItsJobs`, `TestSingleCalendarUnchanged` (`calendars_test.go`): the cursor passes a day only when every job is complete. Where the fake clock is set to the UTC date of the day with a NY job and an Asian job, it changes: the clock moves to a time at which the NY job is due, or the test states that the NY job holds the cursor.
- `TestEmptyBeforeCloseStaysMissing`, `TestNoValueErrorFinalBeforeClose`, `TestRetryableBeforeCloseRetries`, `TestExpiredDayCompletesCursorAdvances`, `TestReconcileEarlyCloseOptionWindow` (window and retry tests): unchanged, they run on days that have begun in the zone.

## Issues

### 1 runner-due-gate (runner)

Module: runner. Size: S.
Files: `internal/runner/reconcile/reconcile.go`, `internal/runner/reconcile/reconcile_test.go`, `internal/runner/reconcile/calendars_test.go`, `internal/runner/reconcile/fakes_test.go`
Design: docs/design/runner-due-gate.md#decision
Order: first.
Goal: `plan` skips a job on day D while D is after the date of the clock in the job's zone, and that job counts as missing for D.
Acceptance:
1. A fake-clock test at 00:09Z on date D: a job of the calendar XNYS (zone America/New_York) is not started on D, a job of the calendar XTKS (zone Asia/Tokyo) of the same UTC date D is started and run to done, and the cursor still holds before D.
2. A fake-clock test at 05:01Z on date D (New York 00:01 on D in winter; 01:01 in summer): the NY job is started on D.
3. A past day is unchanged: a test with the clock one day or more after D starts every job as before and advances the cursor as before.
4. A test of `Backfill` for today and a test of a job that is not `Scheduled` (tested in UTC) pass.
5. `TestCursorHoldsOnGap`, `TestCursorAdvancesOverCompletePrefixAcrossCalendars` and `TestUnionWalksBothCalendars` are named in the pull request description, each with "unchanged" or the reason it changed.
6. `git diff --name-only $(git merge-base HEAD origin/main)` lists only paths under `internal/runner/reconcile/`.
Checked by CI: ci-go.
Pull request body: `Contracts: unchanged`.

## Left out

- Changing the guards of the seven jobs. An optional later change gives their NoValue the reason text "not yet" instead of "wrong day"; it is not needed once the reconciler holds the day.
- The ticker's `dayOf` and the walk limit: they keep the UTC date; the reconciler decides what is due. A zone ahead of UTC (Tokyo) has date D+1 while the limit is D, so its jobs run up to 15 hours late; harmless, and the stated bug (New York, behind UTC) needs no change. A later issue in `internal/runner/sched` may raise the limit if Asia jobs need it; no second entry now because nothing requires it.
- Gating on instants (a job due after the previous session's close of its calendar) instead of zones: rejected, New York's previous close is 21:00Z on D-1, which is before 00:09Z on D, so it would not hold the job and it breaks jobs that run before the open.
- The stop-gap of returning Retryable from the seven jobs on a wrong day: rejected because Retryable has no backoff, gives about 80 rows a day per job, and can retry forever.

## Adversarial

### Answers to the six points of the adversarial review

The full answer is in "scratchpad adversarial-duegate.md of the dispatcher window".

1. Held versus skipped: adopted. A skipped job leaves the day complete and the cursor passes D; the job is now held (done=false), and acceptance 1 starts the XTKS job to done to prove it.
2. No zone source in the reconciler: adopted. The zone is `Scheduled.Schedule()`'s, with UTC when absent; the calendar-zone fallback is removed. The walk limit is not changed (Left out, not needed for New York); `Backfill` of today is pinned by a test.
3. A simpler gate on instants: not adopted. Previous close of New York is before 00:09Z on D, so it does not hold, and it breaks pre-open jobs.
4. Lock-in of zone and schedule: adopted as a stated cost. Zone comes from `Schedule()` on purpose; the UTC-clock tests are the contract and say so.
5. False claims: adopted. The Facts and Interface lines are corrected; "tests keep passing" is now "the issue reads each and changes one only where its clock is before the zone date".
6. Likeliest slip, a `continue` in `plan`: adopted. The Decision forbids it and acceptance 1 fails on it.

### Residual risks

- A zone missing or unparsable: the job is tested in UTC and the reconciler logs once; it never blocks a day for ever.
- A day that never becomes due, such as a calendar day of a closed market: the walk takes session days of the job's calendar only, so the gate adds no day the walk did not already hold.
- A clock that steps back across midnight in the zone: the job is held again for that pass only; a run already recorded is not undone.
- A held cursor delays the jobs of later days only until D begins in New York (about five hours after 00:00Z); the walk past a held day (`TestWalkPastHeldDay`) still reconciles later days, so no other job waits.
