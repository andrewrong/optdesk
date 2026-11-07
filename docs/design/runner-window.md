# Runner window per job: each dataset closes by the rule of the job that plans it

Author: the optdesk dispatcher (draft)
Owner approved: (pending)
Date: 2026-11-07
Source: `runner-window-per-dataset` of `docs/design/store-week2.md` (`## Left out`, second point), named a hard prerequisite of the `cmd/ot` wiring by the option jobs design (pull request 188, `docs/design/option-jobs.md`, `## Left out`, point 2).
Read on main at 0d89c06: `AGENTS.md`, `docs/design/modules.md`, `store-week2.md`, `backfill.md`, `store-master.md` (format), `internal/runner/api.go`, `internal/runner/reconcile/**`, `cmd/ot/runonce.go`, `internal/jobs/fredseries/**`, `docs/contracts/runner.md`; `gh pr diff 188`. Open pull requests that touch `internal/runner` or `cmd/ot`: none.

Goal: the freeze and the slots of option_eod and quote_snapshot get a window of their own, whose close lies after the start guard of the job, while the FRED rule keeps its late close. Today one rule serves every job, so an empty answer of a guard is marked `no_value` at once. This design makes the reconciler read the window from the job that plans a key. Nothing else moves.

## Facts on 0d89c06

- `runner.WindowRule{CalendarID, OpenOffset, CloseOffset}` is one value for the whole reconciler (`internal/runner/api.go:24`); `reconcile.NewWithRule` stores it in `reconciler.rule` (`reconcile.go:37`, `:46`).
- The rule is used twice: `sessions` takes the calendar id from it (`reconcile.go:85`) and `reconcileDay` calls `windowFor(r.rule, datasetID, s)` once per dataset of the plan (loop `reconcile.go:160-175`, call at `:167`), puts the window with `PutWindow` and keeps its `CloseAt` in `closes` (`window.go:21-31`).
- `Reconcile` takes the lease with `Acquire` as its first call (`reconcile.go:52-53`); the jobs are read by `plan` (`reconcile.go:122`).
- `run` marks `no_value` for an empty answer unless `closeAt` is set and the clock is before it, in which case the run is recorded with reason `empty_before_close` and the key stays missing (`reconcile.go:284-290`); a run with a `NoValue` error is `no_value` at once (`:278-279`). A key whose dataset has no window has `closeAt` zero and keeps `no_value` for an empty answer.
- `cmd/ot/runonce.go:96` builds the runner with `ruleOf(job)`: the optional method `Window() (calendarID string, openOffset, closeOffset time.Duration)` of the single job (`runonce.go:100-113`, its private interface `windowed`). `fredseries` has that method (`internal/jobs/fredseries/window.go:20-23`), built from the `[window]` table of its `job.toml` (`rule = "after_close"`, `lateness_hours = 96`, `calendar = "XNYS"`; `windowOf` is `window.go:8-18`). The open offset is always 0.
- With a second job in the list, `ruleOf` could still pass only one rule, so the FRED close (96 hours after the session close) would apply to the option datasets too, or the option rule to FRED.
- `runner.NewBackfillerWithRuleFunc` is declared (merged as 0ae030a) and takes the same `WindowRule`. The backfiller reads the close of a day from the ledger with `Window(dataset, day)` and never calls `PutWindow` (`backfill.md:101-102`), so it does not share the window helper and nothing here depends on it.
- Early-close days: the calendar's `CloseAt` is the early close (13:00 local, `docs/contracts/runner.md`), so a close offset in hours is added to an earlier instant on those days.
- `jobs` may not import `runner` (`modules.md:86`, depends `contracts clock sources store`), which is why the method returns plain values.

## Modules

No change to `docs/design/modules.md` or `arch.toml`. `internal/runner/**` and `docs/contracts/runner.md` are runner (`modules.md:85`). No issue edits `go.mod`, `go.sum`, `cmd/**` or `internal/jobs/**`.

| issue | module |
|---|---|
| runner-window-interface | runner |
| runner-window | runner |

Both entries lie in one module each. They are two entries because the interface entry is judged by a design review and lands first (`AGENTS.md`).

## Interface

Owner: runner, `internal/runner/api.go` (declarations only). One declaration is added; no existing declaration changes, so `WindowRule`, `NewWithRuleFunc`, `NewBackfillerWithRuleFunc`, `cmd/ot` and its tests keep compiling.

```go
// Windowed is the optional method of a job: the window rule of every dataset the job plans.
// The values are plain so that a job package need not import runner. The window opens
// openOffset after the session open and closes closeOffset after the session close; an empty
// calendarID means the calendar of the reconciler. A job without this method gets the rule
// the reconciler was built with.
type Windowed interface {
	Window() (calendarID string, openOffset, closeOffset time.Duration)
}
```

The rule: lateness counts from the session close; freeze_after is wall-clock. The close of a job, `CloseAt + lateness`, must be at or after `freeze_after + budget` and at or after the last slot plus `grace`, on every session day of the calendar, including early closes. A lateness that holds on a regular day can fail on an early close, because the early `CloseAt` is the base. The values are chosen by the option-jobs issues as settings with code defaults: option_eod 5 hours and quote_snapshot 4 hours (2 hours fails on a 13:00 local close: the close would be 15:00 local, before a slot at 15:30 local plus its grace). For option_eod the arithmetic is 13:00 local plus 5 hours is 18:00 local, at or after 16:15 plus the 8-minute budget (16:23 local); 4 hours already holds, 3 does not. FRED keeps 96.

Semantics (written into `docs/contracts/runner.md` by the second entry):

- For a key, the window of its dataset is built from the job that planned the key (the merged plan already records that job per key; the first job to plan a dataset wins): the `Window()` values if the job has the method, else the reconciler's `WindowRule`. A job plans its datasets under one rule; this is enough because each job of the option design has one dataset (`obs.option_eod`, `obs.quote_snapshot`) and `fredseries` has one.
- One calendar per day loop. The session calendar of the day loop stays the reconciler's `WindowRule.CalendarID` (default `XNYS`). A job whose calendar id, after the default is applied, differs from it is a mismatch, and `Reconcile` returns an error that names the job id.
- Control-flow change: today `Reconcile` begins with `Acquire` (`reconcile.go:53`) and reads the jobs later, in `plan`. With this design `Reconcile` first walks `r.jobs` and checks each job that implements `Windowed` against the calendar of the loop, and only then calls `Acquire`. The mismatch error is returned before the lease, before `PutExpected`, before `PutWindow`: nothing is written, nothing is held.
- Nothing else changes: `no_value` for an empty answer at or after the close, `empty_before_close` before it, the cap of two counting runs, the cursor.
- A typo in a job's `Window()` signature (a different return type or order) makes the type assertion fail without any error, and the job silently gets the reconciler's rule. The runner cannot see it. Each job that implements the method carries a compile-time assertion (see `## Left out`).

How a job declares its window: as `fredseries` does today. The `job.toml` of the job has a `[window]` table (`rule = "after_close"`, `lateness_hours`, optional `calendar`), and the package has `Window()` built from it; the option jobs read the hours from settings with the defaults above. `fredseries` already has the method and is not edited.

## Issues

Items under `Checked by CI` are decided by the workflows of the head commit; every other item is an offline grep or a read of the diff, run without network, service or database. Every test uses `t.Parallel()` and `t.TempDir()`; none sleeps (time comes from the fake clock of the test kit).

### runner-window-interface (interface)

The Windowed declaration
  Goal: `internal/runner/api.go` declares `Windowed` of `## Interface` and a test shows the contract from both sides.
  Files: internal/runner/api.go, internal/runner/api_test.go
  Design: docs/design/runner-window.md#interface
  Acceptance:
  1. `git diff internal/runner/api.go` shows additions only; they are the type `Windowed` with its comment; `grep -cE '^func ' internal/runner/api.go` prints 0.
  2. `git diff --stat` lists the two files above and no other.
  3. `grep -n 'func TestWindowedContract' internal/runner/api_test.go` prints one line.
  4. Every test added calls `t.Parallel()` and none sleeps.
  Checked by CI: ci-go (build, vet, test, golangci-lint, go-arch-lint), `checks` (arch); the design verdict on the pull request (an interface change); `TestWindowedContract`: a fake job with `Window()` satisfies `runner.Windowed` (`var _ runner.Windowed = fakeWindowed{}`), and a caller reads back the three values it returned, with an empty calendar id and a negative open offset kept as given.
  Pull request body: `Contracts: unchanged`.
  Depends on: none (merged work only). Order: before `runner-window`.
  Size: S

### runner-window (runner)

The reconciler reads the window from the job of the key
  Goal: `Reconcile` puts, for each dataset, the window of the job that planned it, an empty answer is retried until that window closes, and a calendar mismatch is refused before the lease.
  Files: internal/runner/reconcile/reconcile.go, internal/runner/reconcile/window.go, internal/runner/reconcile/window_test.go, internal/runner/reconcile/reconcile_test.go, internal/runner/reconcile/fakes_test.go, docs/contracts/runner.md
  Design: docs/design/runner-window.md#interface
  Acceptance:
  1. `grep -rn 'windowFor(r.rule' internal/runner/reconcile --include=reconcile.go --include=window.go` prints nothing; `grep -n 'runner.Windowed' internal/runner/reconcile/window.go` prints one line (the only type assertion).
  2. `git diff --stat` lists only the files above; `git diff -- internal/runner/api.go cmd internal/jobs go.mod go.sum` is empty.
  3. `docs/contracts/runner.md` names `runner.Windowed`, says a job without it gets the reconciler's rule, says lateness counts from the session close, and names the calendar mismatch error; `grep -c 'Windowed' docs/contracts/runner.md` prints at least 2; no open or close time is written in Go (`grep -nE '[0-9]{2}:[0-9]{2}' internal/runner/reconcile/window.go` prints nothing).
  4. In `reconcile.go` the job walk with the calendar check comes before the call of `Acquire` (`grep -n 'Acquire\|checkCalendars' internal/runner/reconcile/reconcile.go` lists the check first).
  5. `grep -c 'func TestWindowPerJob\|func TestWindowedJobWithoutMethod\|func TestWindowCalendarMismatch\|func TestEmptyRetriedUntilOwnClose\|func TestReconcileEarlyCloseOptionWindow' internal/runner/reconcile/*_test.go` totals 5 over the files; there is no `TestWindowEarlyClose`, and `TestWindowThanksgivingAndEarlyClose` (`window_test.go:23`) is extended, not duplicated. Every test added calls `t.Parallel()` and none sleeps.
  6. `TestEmptyRetriedUntilOwnClose` fails on the parent commit (the empty answer is `no_value` at once) and passes with the change; the pull request description shows both runs.
  Checked by CI: ci-go (build, vet, test, golangci-lint, go-arch-lint); and the named tests, all with the fake ledger, calendar and clock of the package (the new job fake with a `Window()` method lives in `fakes_test.go`):
  - `TestWindowPerJob`: two jobs, one with close offset 5 hours and one with 96, plan one dataset each; `PutWindow` receives two windows whose `CloseAt` is the session close plus the offset of the job of that dataset.
  - `TestWindowedJobWithoutMethod`: a job without `Window()` gets the window of the `WindowRule` the reconciler was built with, with the old no-offset result when that rule is zero.
  - `TestWindowCalendarMismatch`: a job whose calendar id differs from the reconciler's makes `Reconcile` return an error with the job id; the ledger saw no `Acquire`, no `PutExpected`, no `PutWindow`; an empty id and `XNYS` both pass.
  - `TestWindowThanksgivingAndEarlyClose` (existing, extended): on a session with an early `CloseAt`, the window close is that instant plus the offset given; a non-session day gets no window.
  - `TestReconcileEarlyCloseOptionWindow`: the fake calendar gains an early-close session (13:00 local); a job whose offsets are the option_eod defaults (5 hours) plans a dataset on that day; the `PutWindow` call has `CloseAt` at or after 16:23 local. This is the enforceable test of the rule in `## Interface`: a lateness that is too small for an early close fails it.
  - `TestEmptyRetriedUntilOwnClose`: a job with a 5-hour offset whose `Run` returns an empty answer with a nil error at a fake time between the session close and the window close leaves the key unmarked with reason `empty_before_close`; the same answer after the window close marks `no_value`; a second job without `Window()` and a zero rule marks `no_value` at once.
  Pull request body: `Contracts: unchanged`.
  Depends on: runner-window-interface. Order: after it.
  Size: M

## Rollout

Merge order is the table of `## Modules`. `cmd/ot` is not edited: it still passes `ruleOf(job)` as the reconciler's rule, which for the one FRED job equals the rule the reconciler now reads from the job itself, so behaviour today is unchanged. The option jobs wiring (a later design) drops `ruleOf`, passes the zero rule and names no `Depends on` other than this design's two issues.

## Left out

- A calendar per job or per dataset: the session loop has one calendar, and a mismatch is an error returned before `Acquire`. A per-job calendar is left out until a JPX dataset exists; two calendars in one run need one loop per calendar and a cursor per calendar, its own design. Until then a mismatch is an error, never a silent wrong window.
- A window per dataset inside one job (a job planning two datasets with different closes): no job needs it; the method would take the dataset id then.
- Removing `ruleOf` and the private interface `windowed` of `cmd/ot/runonce.go` belongs to the design of the ot-collect wiring, because it edits `cmd/**`; `TestRunOnceRuleFromJob` and `TestRunOnceZeroRule` change with it.
- The compile-time assertion in each job that implements the method (a line of the form `var _ interface{ Window() (string, time.Duration, time.Duration) } = (*job)(nil)`), which turns a signature typo into a build error: it belongs to the jobs issues (design 188 for the option jobs; `fredseries` is not edited here).
- The values of option_eod (5 hours) and quote_snapshot (4 hours) as settings with code defaults, and the test that checks them against the manifest: set in the issues of design 188, under the rule of `## Interface`. That design's `## Left out` text ("a hard prerequisite") becomes "needs the issues of `runner-window.md`".
- Day completeness: a day with a key empty before its close holds the cursor, and a later invocation reconciles it after midnight where a job guard may answer `NoValue`; scheduling of the retry is the wiring design's question, not this one.
- Alternatives not chosen: a map of dataset id to rule in the constructor (changes `NewWithRuleFunc` and the backfiller constructor, both declared and used by `cmd/ot`); a field of the rule holding the map (makes `WindowRule` not comparable, which `api_test.go` and `runonce_test.go` rely on); a window rule inside `contracts.Job` (an interface change of another module, for a value only the runner reads); a global close offset large enough for every job (the FRED close, 96 hours, would delay `no_value` of every key by days).

## Adversarial

The adversarial review (optdesk advisor, 2026-11-07) raised six points; the full answer is in the author's scratchpad. Each point and its outcome:

1. The goal "close after the start guard" is enforced nowhere. Adopted: the rule is written in `## Interface` (lateness counts from the session close, freeze_after is wall-clock), the values are chosen by the option-jobs issues, and `TestReconcileEarlyCloseOptionWindow` asserts a close at or after 16:23 local on an early-close day.
2. The close is per dataset, the guard per key, so 2 hours fails for slots after 15:00 local on an early close; day completeness couples datasets. Adopted for the lateness (option_eod 5 hours, quote_snapshot 4 hours, settings with defaults); the completeness and scheduling part is named in `## Left out`, not solved here.
3. Drop the interface entry for an unexported interface in `reconcile`. Not adopted: `AGENTS.md` says other modules use a module only through its interface file, and jobs live in another module, so the optional method is declared in `api.go`.
4. Lock-in: a signature typo silently falls back; one calendar; one window per job; double path in `cmd/ot`. Adopted: the silent fallback is stated in `## Interface` and a compile-time assertion per job is named in `## Left out`; the other three are stated there and in `## Rollout`. One calendar per loop is made an error before `Acquire`, with a test that no `Acquire` happens.
5. False claims and cites. Adopted: the backfill coupling and its bullet are deleted (the backfiller reads the close from the ledger); the line ranges of `fredseries/window.go` and the `reconcile.go` loop, and the test name `TestRunOnceRuleFromJob`, are corrected; `TestWindowEarlyClose` is dropped in favour of extending `TestWindowThanksgivingAndEarlyClose`.
6. Likeliest slip in the issues: `fakes_test.go` missing from `Files`, and the mismatch check before `Acquire` is a control-flow change not described. Adopted: `fakes_test.go` is in `Files` and the change is described in `## Interface` and checked in acceptance 4.
