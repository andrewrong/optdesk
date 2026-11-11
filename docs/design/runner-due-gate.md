# Runner due gate: a Scheduled job is not started for day D before its slot instant on D

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-11
Revision: 5

Source: decision of optdesk-advisor, 2026-11-11 (round 1 replaced the date gate of revision 2). Read on origin/main: `AGENTS.md`, `docs/design/modules.md`, `docs/contracts/runner.md`, `internal/runner/api.go`, `internal/runner/sched/slots.go`, `internal/runner/sched/ticker.go`, `internal/runner/reconcile/**`, the job schedule and guard files named below.

Goal: a job that is scheduled for a start instant on day D is not started on D before that instant, so a guard that answers "wrong day" at 00:09Z can no longer make the key final and lose the New York session.

## Facts

- The reconciler has no "not yet" rule: `plan` (`internal/runner/reconcile/reconcile.go:135-141`) plans every job for the UTC date from 00:00Z, and `nextAttempt` (`:280-312`) starts any key that has no run.
- `runkey.go:139` records NoValue as ok and final. Seven jobs guard with an equality test of the day against their own zone's date and return NoValue "wrong day": `optioneod/run.go:119`, `accountcapture/job.go:104`, `dividendcapture/run.go:212`, `earningsrefresh/run.go:235`, `positionmarks/run.go:122`, `quotesnapshot/run.go:99`, `optionopen/pull.go:208`. At 00:09Z on D New York is on D-1, the guard answers "wrong day", the key is final, and the real slot is skipped.
- A zone gate does not serve. Five of the seven jobs have no zone: a session anchor forbids one (`optioneod/schedule.go:59-60`, `quotesnapshot/schedule.go:59-60`), and `optioneod`, `accountcapture`, `positionmarks`, `quotesnapshot` and optionopen's `job_pull.toml` declare none; their guards read the manifest zone (`optioneod/run.go:92`). With a UTC fallback the gate never holds `option_eod`.
- A date gate does not serve either, even with a zone. It only moves the early run to about 05:15Z, about 16 hours before the slot. `accountcapture/job.go:104`, `dividendcapture/run.go:212` and `earningsrefresh/run.go:235` write at once, and `optionopen/pull.go:208` pulls quotes at New York midnight; the key becomes final and the real slot is skipped.
- `sched/slots.go` `nextStarts` already computes the slot instant of a job on a day: `session_open` and `session_close` are the calendar session's `OpenAt` / `CloseAt` plus `Offset`; `time` is `Offset` since midnight of the day in `Zone`. The ticker wakes a turn at that instant. The reconciler computes no instant.
- `reconcileOn` (`reconcile.go:174-233`) computes `complete` from `p.keys` only. `plan` writes `p.hash` from the jobs it plans (`:149`), and `PutExpected` runs when the hash differs from the stored one.
- `backfillPlan` (`backfill.go:128-170`) calls `plan` and returns the error "no job returns a key of dataset" when a selected stored key's job is missing from the plan.
- `runner.Slot` and `runner.Scheduled` (`internal/runner/api.go`) carry the schedule as plain values: calendar id, anchor, zone, offset.

## Modules

Changed, and across modules: the import rule is changed first (its sentence lands in `docs/design/modules.md` with this design pull request; entry 2, below), then the runner entry (3) uses it.

- `docs` (`docs/design/modules.md`, owner-only path): this design pull request adds one sentence to the **depends** paragraph that names the one allowed edge between sub-packages of a row; it is not an issue.
- `repo` (`scripts/arch/**`): the generator derives that edge, with a test.
- `runner` (`internal/runner/** docs/contracts/runner.md`): `internal/runner/reconcile`, `internal/runner/sched` and a new package `internal/runner/shared`. `reconcile` and `sched` import `shared`; `shared` imports `runner`, `clock` and `contracts` and no sibling, so there is no cycle.

Why a rule change. The shared function first went in a package `internal/runner/slotat`, then in the root package. Neither is allowed today. The **depends** paragraph of `docs/design/modules.md`, which `scripts/arch` turns into go-arch-lint for `ci-go`, lets a sub-package `<name>/<sub>` import only the root components of its row's depends cell, the standard library, external modules, its own root `<name>`, and the declarations-only packages, test kits and `genpb` it names ("a sub-package component `<name>/<sub>` may import the same and the root component `<name>`"). So `reconcile` cannot import a sibling implementation package, whatever its name. The root `internal/runner` cannot take the function either: every root package of `internal/` except `profile` holds `api.go` and tests only, the `entry` cell says "one declarations-only file per package (`api.go`)" and AGENTS.md says "an interface is one file per package, declarations only", and `docs/contracts/runner.md` says "The root package `internal/runner` holds only `api.go` and `api_test.go`". A logic file in the root would be a second file of the interface package, imported by every module that uses `runner` (cmd/ot, jobs). A test kit does not fit either: a kit is imported by the sub-packages of the rows that depend on its row, "never by ... its own row's sub-packages", and a declarations-only package holds no logic. So the rule changes (option b).

## Interface

`internal/runner/api.go` is not edited: `Slot` and `Scheduled` already carry what the function needs. The new package `internal/runner/shared` is an implementation package, not an interface file: it holds logic (`At`), no row lists it in an entry cell, and no module outside `runner` imports it (the rule below lets only sibling sub-packages of its own row import it). `docs/design/modules.md` changes in its rule text only; no row, entry cell or depends cell changes.

## Decision

A job that is `runner.Scheduled` is not started for day D before its slot instant on D. The instant is the calendar session of D plus the anchor, zone and offset of the job, which is the instant `sched/slots.go` computes to wake a turn. Until then the job's keys are HELD and the day is not done. Session anchors need no zone lookup (the session instant carries it). A day whose instant has passed, which includes every past day, passes at once, so backfill and catch-up are unchanged. A job that is not `Scheduled` is unchanged.

### One function for the instant

The instant comes from ONE function used by both `reconcile` and `sched`:

`shared.At(s runner.Slot, sess clock.Session) (time.Time, error)` in `internal/runner/shared/slotat.go` (package `shared`). `sess` is the session of `s.CalendarID` on the day, with `IsSession` true; `At` returns `sess.OpenAt + Offset`, `sess.CloseAt + Offset`, or, for `time`, `Offset` since midnight of the date of `sess.Day` in `s.Zone`. An unknown anchor, or an unknown or empty zone for `time`, is an error naming the job, as `NextStarts` returns today.

The code moves out of `sched`: the `switch j.Anchor` and `wallTime` of `nextStarts` (`sched/slots.go`) become a call of `shared.At`, and the anchor constants move with them to `shared`. `sched` keeps its pre-validation of anchors and zones and its error text, so its tests pass unchanged. `reconcile` builds the `runner.Slot` of a job from `Scheduled.Schedule()` and the job's id and calls the same function with the session of the job's calendar that `plan` already has in `sess`. They cannot disagree because there is one body.

The import rule that allows it (the sentence lands in `docs/design/modules.md` with this design pull request; entry 2). Sentence added to the **depends** paragraph of `docs/design/modules.md`, after the sentence that lets a sub-package import the root component: "A sub-package `<name>/shared` may be imported by its row's other sub-packages, and it imports no sibling." `scripts/arch` derives it from the tree: the component `<name>/shared`, when the directory exists, gets the sub-package import list of its row and is added to the import list of the other sub-package components of the row; no other sibling edge is added. Test of the generator: a sub-package importing `<name>/shared` passes; a sub-package importing another sibling fails; `<name>/shared` importing a sibling fails.

### Plumbing

- `plan` keeps planning every job of the day, including held ones, and returns the held state in the plan: `plan.held` is the set of expected keys whose job is not yet due, computed only when `sess` is non-nil (a job is held when `r.clk.Now()` is before `shared.At`). `p.keys`, `p.job` and the job's hash are unchanged for a held job, so `p.hash` includes held jobs from the first pass and `PutExpected` is not rewritten when the job becomes due. A `continue` that skips the job in `plan` is wrong: the hash would change on the pass at which the job becomes due.
- `reconcileOn` skips a held key before `expireIfClosed` and `nextAttempt`, and sets `complete = false`: the day is not done, so the cursor stays before D. A held job writes no key (no `MarkPresent`) and no run row (no `RecordRun`). The windows of its datasets are put as today.
- On a later pass at or after the instant, the key is no longer held and is planned and run as today.
- A job whose instant on D has passed is not held. At 00:09Z an XTKS-session job whose slot has passed is started on D; a New York `option_eod`-like job whose slot is at the close is held.
- Only `reconcileOn` consults `plan.held`. `Backfill` sees the held jobs' keys: `plan` returns them in `p.keys` and `p.job`, so `backfillPlan` finds the job of every stored key and does not return "no job returns a key of dataset". `backfill.go` is unchanged: backfill names days the operator chose and ignores `held`; backfill of today returns the keys of every job, held or not, and no error.
- A job that is not `Scheduled` has no slot and is never held.
- A clock that is behind the instant by a step back is held again for that pass only; a run already recorded is not undone.

### Tests that pin today's behaviour

All in `internal/runner/reconcile/*_test.go`; the issue reads each and changes a test only where its fake clock is before the slot instant of one of its jobs on the day it reconciles. Most tests use a fake clock at or after the end of the day and a fake job that is not `Scheduled`, so they pass unchanged.

- `TestCursorHoldsOnGap` (`reconcile_test.go`), `TestWalkPastHeldDay`, `TestFullCycleLateRun`: a gap holds the cursor. They stay; "not yet" is one more reason for a gap.
- `TestCursorAdvancesOverCompletePrefixAcrossCalendars`, `TestUnionWalksBothCalendars`, `TestJobNotPlannedOnOtherCalendarHoliday`, `TestCalendarRunsOutDropsOnlyItsJobs`, `TestSingleCalendarUnchanged` (`calendars_test.go`): the cursor passes a day only when every job is complete. Where a fake job is `Scheduled` and the clock is before its instant, the clock moves to the instant or the test states that the job holds the cursor.
- `TestEmptyBeforeCloseStaysMissing`, `TestNoValueErrorFinalBeforeClose`, `TestRetryableBeforeCloseRetries`, `TestExpiredDayCompletesCursorAdvances`, `TestReconcileEarlyCloseOptionWindow` (window and retry tests): unchanged. `retry_test.go` stays out of Files.

## Issues

Entry id 1 is unused: the rule sentence for `docs/design/modules.md` lands with this design pull request, so it has no issue.

### 2 arch-shared-subpackage (repo)

Module: repo. Size: S.
Files: `scripts/arch/main.go`, `scripts/arch/main_test.go`
Design: docs/design/runner-due-gate.md#one-function-for-the-instant
Order: first.
Goal: `go run ./scripts/arch` derives the edge of `<name>/shared` and no other sibling edge.
Acceptance:
1. With a `internal/<name>/shared` directory, the generated import lists let every other sub-package component of the row import `<name>/shared`, and `<name>/shared` gets the sub-package list of its row and no sibling.
2. The cases of the Decision (a sub-package importing `<name>/shared` passes; importing another sibling fails; `shared` importing a sibling fails) are cases of `scripts/arch/main_test.go`.
3. With no `shared` directory the generated output is unchanged.
Checked by CI: ci-go.
Pull request body: `Contracts: unchanged`.

### 3 runner-due-gate (runner)

Module: runner. Size: M.
Files: `internal/runner/shared/slotat.go`, `internal/runner/shared/slotat_test.go`, `internal/runner/sched/slots.go`, `internal/runner/reconcile/reconcile.go`, `internal/runner/reconcile/reconcile_test.go`, `internal/runner/reconcile/calendars_test.go`, `internal/runner/reconcile/fakes_test.go`, `docs/contracts/runner.md`
Design: docs/design/runner-due-gate.md#decision
Order: after 2 (the import rule must be on main, or ci-go fails the import of `shared`).
Goal: a `Scheduled` job's keys are held, and the day is not done, until the slot instant of the job on the day; the instant is computed by `shared.At`, which `sched` uses too.
Acceptance:
1. At 00:09Z on D, a session-anchored `option_eod`-like fake job with no zone and the anchor `session_close` of XNYS is held: no run row, the cursor stays before D, the day is not done.
2. At the slot instant of that job on D (the close of D plus its offset) the job is started.
3. At 00:09Z on D an XTKS-session job whose slot instant has passed is started on D.
4. A past day starts at once: with the clock a day or more after D every job starts as before and the cursor advances as before.
5. `Backfill` of today, with a held job, returns the keys of every job of the day, held ones included, and no error.
6. The plan hash with a held job equals the plan hash after the job becomes due, so `PutExpected` is called once, not twice.
7. A job that is not `Scheduled` is unchanged: it is started at 00:09Z on D as before.
8. `docs/contracts/runner.md` (lines 5, 12, 38) states when keys are planned and that a `Scheduled` job is held until its slot instant, and lists the pinned tests of this design; `TestCursorHoldsOnGap`, `TestCursorAdvancesOverCompletePrefixAcrossCalendars` and `TestUnionWalksBothCalendars` are named in the pull request description, each with "unchanged" or the reason it changed.
9. `git diff --name-only $(git merge-base HEAD origin/main)` lists only the paths of Files above; `internal/runner/api.go` is not among them.
Checked by CI: ci-go.
Pull request body: `Contracts: unchanged`.

## Left out

- Changing the guards of the seven jobs. An optional later change gives their NoValue the reason text "not yet" instead of "wrong day"; it is not needed once the reconciler holds the key.
- The ticker's `dayOf` and its UTC walk limit stay. They break one job once the JP open jobs exist: `option_open_plan_jp` runs at open minus 45 minutes, which is 23:15Z of the previous UTC date (option-open-pull Decision 21). The ticker reconciles only up to the UTC date of now, so at 23:15Z on D-1 it does not walk the XTKS session day D, and the plan job runs only after 00:00Z, up to 45 minutes after its slot, or later when the 15-minute tick comes. This is not harmless; the limit is fixed before the JP open jobs are cut, in a separate issue in `internal/runner/sched`, not by this entry.
- The stop-gap of returning Retryable from the seven jobs on a wrong day: rejected because Retryable has no backoff, gives about 80 rows a day per job, and can retry forever.

## Adversarial

### Why not a date gate

A date gate (the job is held until D has begun in its zone, revision 2) is rejected, by two findings.

1. The zone is empty for five of the seven jobs: a session anchor forbids a zone (`optioneod/schedule.go:59-60`, `quotesnapshot/schedule.go:59-60`), and `optioneod`, `accountcapture`, `positionmarks`, `quotesnapshot` and optionopen's `job_pull.toml` have none; their guards read the manifest zone (`optioneod/run.go:92`). With a UTC fallback the gate never holds `option_eod`, and a test with a zoned fake job would pass while the real jobs stay broken.
2. Even with a zone, a date gate moves the early run only to about 05:15Z, about 16 hours before the slot. `accountcapture/job.go:104`, `dividendcapture/run.go:212` and `earningsrefresh/run.go:235` write at once, and `optionopen/pull.go:208` pulls quotes at New York midnight. The key becomes final and the real slot is skipped.

The slot instant needs no zone from the job for a session anchor and is the instant the ticker already wakes for.

### Other points

1. Held versus skipped: a skipped job leaves the day complete and the cursor passes D; a held key makes `complete` false. Acceptance 1 fails on a `continue`.
2. Hash: the held job's hash stays in `p.hash`; acceptance 6 fails if `PutExpected` runs twice.
3. Instant computed twice: rejected; one function, `shared.At`, used by `sched` and `reconcile`.
4. Backfill: `plan` returns held keys, so `backfillPlan` does not fail; acceptance 5.
5. Import rule: `reconcile` importing a sibling package (first `slotat`) is not allowed by the depends paragraph, and neither is a function in the root `internal/runner`, which is declarations-only (Modules). Rejected: citing text that allows the edge (there is none), a logic file in the root, a test kit (never imported by the row's own sub-packages). The rule changes first, by the narrowest edge (one fixed name, no cycle by construction), and this design crosses `docs` and `repo` for it: the rule sentence lands in `docs/design/modules.md` with this design pull request, and the generator follows in entry 2.
6. A gate on the previous close of the calendar: not adopted; New York's previous close is 21:00Z on D-1, before 00:09Z on D, and it breaks jobs that run before the open.

### Residual risks

- A `Scheduled` job with an unknown anchor or zone makes `At` return an error, which `plan` returns as `NextStarts` does today; the ticker already fails on the same value, so no new failure appears.
- A day that never becomes due, such as a closed market's day: the walk takes session days of the job's calendar only and `plan` plans only jobs with a session, so the gate adds no day the walk did not already hold.
- A held cursor delays the jobs of later days only until the instant; `TestWalkPastHeldDay` shows later days are still reconciled.
