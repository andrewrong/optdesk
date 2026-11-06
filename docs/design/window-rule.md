# Window rule: a job's `[window]` reaches the reconciler

Author: advisor of optdesk (first draft)
Source: the "window rule" lines of `## Left out` of `docs/design/week1-ot.md` and `docs/design/week1-wiring.md`.
Read on main at c63cbb4: `internal/runner/reconcile/window.go` already holds `reconcile.WindowRule` (`CalendarID`,
`OpenOffset`, `CloseOffset`, method `Window`) and `reconcile.NewWithRule`; `reconcile.New` is `NewWithRule` with the
zero rule (`DefaultCalendarID` is `XNYS`). `internal/runner/api.go` has only `Reconciler` and `NewFunc`, so `cmd/ot`
cannot name the rule type or the constructor with a rule. `internal/jobs/fredseries/job.toml` already has a `[window]`
table (`rule = "after_close"`, `lateness_hours = 24`) that `plan.go` decodes into `window` and nothing reads.
`cmd/ot/runonce.go` builds the reconciler with `d.newRunner` (`runner.NewFunc`, set to `reconcile.New` in `wire.go`)
over `d.newCalendar(sd)`, where `sd` is the session-day store opened with the runner DSN; that already is the read of
`master.session_day` through the store `sessionday` reader, and this design leaves it as it is.

Goal of the design: the rule of a real run comes from the job manifest instead of the zero rule. This design lands two of
the three steps: the rule type and a constructor type in `runner`, and the reading of `[window]` in the job. The third,
the use in `ot run-once`, is cut after issue 75 merges (see `## Left out`). Until then the two entries land inert: the
real run still uses the zero rule.

## Modules

No change to `docs/design/modules.md`, `arch.toml` or `.go-arch-lint.yml`: no new package and no new import edge
(`internal/jobs/fredseries` already imports go-toml; `runner` already imports `clock`, `contracts` and `store`;
`jobs` does not import `runner`, which is why the job returns plain values, see `## Interface`).

| issue | module | order |
|---|---|---|
| runner-window-rule (interface) | runner | first |
| jobs-window-reader | jobs | any |

No entry edits `go.mod`, `go.sum`, `contracts/`, `cmd/` or `docs/design/modules.md`. The `ot` step is not an entry here: it edits
`cmd/ot/wire.go`, which open issue 75 also edits, so it is cut by a follow-up after 75 merges.

## Interface

Landed in #93

Owner: `runner` (`internal/runner/api.go`), declarations only:

```go
// WindowRule turns a session into the window of a dataset: the window opens OpenOffset after the session
// open and closes CloseOffset after the session close.
type WindowRule struct {
	CalendarID  string // empty means the default calendar (XNYS)
	OpenOffset  time.Duration
	CloseOffset time.Duration
}

// NewWithRuleFunc is the constructor of the reconciler with a window rule (internal/runner/reconcile.NewWithRule).
type NewWithRuleFunc func(store.OpsLedger, clock.Calendar, clock.Clock, []contracts.Job, WindowRule) Reconciler
```

`runner.WindowRule` is the type only, with no methods (`api.go` holds declarations only). `reconcile.WindowRule` is
removed, with no copy; its two methods become unexported functions of `internal/runner/reconcile/window.go`:
`calendarIDOf(rule runner.WindowRule) string` and `windowFor(rule runner.WindowRule, datasetID string, s clock.Session)
(store.Window, bool)`. `reconcile.NewWithRule` has the type `runner.NewWithRuleFunc`; `runner.NewFunc` and
`reconcile.New` are unchanged.

Not an interface change, so only one entry is marked: the job exposes its rule as an optional method of the value
`fredseries.New` returns, `Window() (calendarID string, openOffset, closeOffset time.Duration)`. The method is not in
`internal/jobs/api.go`, and `contracts.Job` is unchanged. The follow-up `ot` step asserts a local one-method interface on
the job; a job without the method gets the zero rule, because the existing `run-once` fakes have no such method and the
zero rule is today's behaviour.

The `[window]` table of `job.toml`:

| key | meaning |
|---|---|
| `rule` | required; the only accepted value is `after_close` |
| `lateness_hours` | integer, at least 0 (absent means 0); becomes `CloseOffset` in hours |
| `calendar` | optional string; becomes `CalendarID`; absent means empty, so the default calendar |

`OpenOffset` is always 0 (the window opens at the session open): the reconciler reads only the close instant of a
window to schedule the repair and the crash re-run, so `after_close` is carried by `CloseOffset` alone.

## Issues

Items under `Checked by CI` are decided by the workflows of the head commit; every other item is a fact read from the
diff, with an offline grep or a named test.

### runner-window-rule (interface)

The rule type and the constructor type in the runner interface, with `reconcile` using them.
  Goal: `cmd/ot` can name the window rule and the constructor that takes it through `internal/runner/api.go` alone.
  Files: internal/runner/api.go, internal/runner/api_test.go, internal/runner/reconcile/reconcile.go, internal/runner/reconcile/window.go, internal/runner/reconcile/window_test.go, internal/runner/reconcile/reconcile_test.go, docs/contracts/runner.md.
  Design: docs/design/window-rule.md#interface
  Acceptance:
  1. `internal/runner/api.go` declares `type WindowRule struct` with exactly the fields `CalendarID string`, `OpenOffset time.Duration` and `CloseOffset time.Duration`, and `type NewWithRuleFunc func(store.OpsLedger, clock.Calendar, clock.Clock, []contracts.Job, WindowRule) Reconciler`; the file has no function body (offline: `grep -c '^func ' internal/runner/api.go` prints 0).
  2. `TestWindowRuleContract` in `internal/runner/api_test.go` checks both sides: a fake `NewWithRuleFunc` records the rule it is given, and a caller passes `WindowRule{CalendarID: "X", OpenOffset: time.Minute, CloseOffset: time.Hour}` and reads the same values back.
  3. No copy of the type stays in `reconcile`: `grep -rc 'type WindowRule' internal/runner/reconcile` prints 0 for every file, and `grep -c 'func (r WindowRule)' internal/runner/reconcile/window.go` prints 0. `window.go` holds the unexported functions `calendarIDOf` and `windowFor` of `## Interface`, with the logic of the former methods `calendarID` and `Window`.
  3a. The call sites use them: `reconcile.go` line `id := r.rule.calendarID()` of `sessions` becomes `calendarIDOf(r.rule)`, and the `r.rule.Window(k.DatasetID, s)` call of `reconcileDay` becomes `windowFor(r.rule, k.DatasetID, s)` (offline: `grep -c 'r.rule.calendarID\|r.rule.Window' internal/runner/reconcile/reconcile.go` prints 0 and `grep -c 'calendarIDOf(r.rule)\|windowFor(r.rule' internal/runner/reconcile/reconcile.go` prints 2); the field `rule` has the type `runner.WindowRule`; `NewWithRule(..., rule runner.WindowRule)`; `New` passes `runner.WindowRule{}`; `reconcile_test.go` holds `var _ runner.NewWithRuleFunc = NewWithRule` and `var _ runner.NewFunc = New`, and its `newRec` passes `runner.WindowRule{CalendarID: "XNYS"}`.
  4. The default calendar still applies: `TestNewWithRuleEmptyCalendar` in `reconcile_test.go` uses a fake calendar that records the id it is asked for and asserts it is `DefaultCalendarID` when the rule's `CalendarID` is empty and `"X"` when it is set to `X`.
  5. The behaviour of the window is unchanged: in `window_test.go` the two calls `rule.Window("ds", s)` of `TestWindowRuleOffsets` (the session and the non-session day) become `windowFor(rule, "ds", s)`, `rule` is a `runner.WindowRule`, and `TestWindowThanksgivingAndEarlyClose` (which builds `rule` as `runner.WindowRule{CalendarID: "XNYS", CloseOffset: time.Hour}` and goes through `NewWithRule`) keeps its expectations; both tests pass with no changed expected value (offline: `git diff` of `window_test.go` shows no changed `want` or expected-time line).
  6. `docs/contracts/runner.md` names `runner.WindowRule` and `runner.NewWithRuleFunc` in its Inputs and no longer says the hook to `job.toml` belongs to a follow-up design (offline: `grep -c 'reconcile.WindowRule' docs/contracts/runner.md` prints 0).
  7. Every test added or changed calls `t.Parallel()`, none sleeps, and each non-test file stays under 600 lines.
  8. No file outside the seven listed is in the diff, and the result matches the `## Interface` part of `docs/design/window-rule.md`.
  Checked by CI: ci-go (build, vet, test, golangci-lint, go-arch-lint).
  Depends on: none. Order: first.
  Size: S

### jobs-window-reader

The `[window]` table of the fred_series manifest is validated and exposed as a method of the job.
  Goal: `fredseries.New` fails on a bad `[window]` table, and the job it returns answers `Window()` with the values of the table.
  Files: internal/jobs/fredseries/window.go, internal/jobs/fredseries/window_test.go, internal/jobs/fredseries/plan.go, internal/jobs/fredseries/job.toml, internal/jobs/fredseries/run.go, docs/contracts/jobs.md.
  Design: docs/design/window-rule.md#interface
  Acceptance:
  1. `window.go` (new) holds `func windowOf(m manifest) (calendarID string, openOffset, closeOffset time.Duration, err error)` and `func (j *job) Window() (calendarID string, openOffset, closeOffset time.Duration)`; the window struct of `plan.go` gains the field `Calendar string` with the tag `toml:"calendar"`; `parseManifest` and `planFrom` are not changed in behaviour, so a manifest without a `[window]` table still plans (`TestPlanHash` of `plan_test.go` passes as it is).
  2. `New` in `run.go` calls `windowOf` and returns its error; the error names the key and carries no value other than the key name (`TestWindowErrors`).
  3. `TestWindowOf` in `window_test.go`, table driven over manifest bytes: `rule = "after_close"`, `lateness_hours = 24`, `calendar = "XNYS"` gives `("XNYS", 0, 24h)`; no `calendar` key gives the empty id; `lateness_hours` absent gives 0.
  4. `TestWindowErrors`: a missing `[window]` table, `rule = "before_open"`, an empty `rule` and `lateness_hours = -1` each return an error from `windowOf` and from `New`.
  5. `TestEmbeddedWindow`: `New` over fake source, writer and clock returns a job whose `Window()` is `("XNYS", 0, 24*time.Hour)`; `job.toml` carries `calendar = "XNYS"` next to the existing `rule` and `lateness_hours` lines, and its other keys are unchanged.
  6. `docs/contracts/jobs.md` lists the three keys of `[window]` and the method `Window`; it says `contracts.Job` and `internal/jobs/api.go` are unchanged (offline: `git diff --stat` shows `internal/jobs/api.go` absent).
  7. `internal/jobs/fredseries` does not import `internal/runner` (offline: `grep -c 'internal/runner' internal/jobs/fredseries/window.go internal/jobs/fredseries/run.go internal/jobs/fredseries/plan.go` prints 0 for each); every test calls `t.Parallel()`, none sleeps or reads the network, and `window.go` stays under 600 lines.
  8. No file outside the six listed is in the diff, and the result matches the `## Interface` part of `docs/design/window-rule.md`.
  Checked by CI: ci-go (build, vet, test, golangci-lint, go-arch-lint).
  Depends on: none. Order: any.
  Size: S

## Left out

- The `ot` step (`ot-run-once-window`): `deps.newRunner` in `cmd/ot/wire.go` changes to `runner.NewWithRuleFunc` (assigned `reconcile.NewWithRule`), and `runonce.go` asserts a local interface with `Window() (string, time.Duration, time.Duration)` on the job and passes `runner.WindowRule{...}`; a job without the method gets the zero rule, because the existing `run-once` fakes have no such method. It is cut by a follow-up once issue 75 (which edits `wire.go`) merges, because no sibling or unmerged issue may be a dependency. Until then the two entries of this design land inert and the real run still uses the zero rule.
- `OpenOffset` is unused for now: the manifest never sets it (always 0) and the reconciler reads only the close instant of a window; `after_close` is carried by `CloseOffset = lateness_hours` alone.
- A `WindowRule` in `contracts` or in `jobs/api.go` shared by runner and jobs: two interface entries are not allowed in one design, and `jobs` may not import `runner` (its depends cell in `modules.md`); plain values through an optional method cost one local interface in `ot`.
- A method `Window` on `contracts.Job`: it would change every job and fake; one job exists, and the optional method can be promoted when a second job needs it.
- Several jobs with different rules: the reconciler takes one rule for all jobs; run-once has one job. A per-dataset rule waits for the second job.
- An `OpenOffset` key or a rule other than `after_close`: no consumer reads the open instant; add keys with the first job that needs them.
- Populating `master.session_day`: issue 75 loads files, and the generator of `docs/design/pyside-layout.md` produces them; until both land a real run ends in `UnknownCalendar`, as before.
- Reading the manifest from a file in `OT_HOME` instead of the embedded one: no owner need yet.

## Adversarial

Source: `window-rule.adversarial.md`, kept on the dispatcher's machine.

1. Gap against the goal: adopted; the `ot` step needs `wire.go`, which issue 75 edits, so it is cut, and the document says the two entries land inert and the real run keeps the zero rule until the follow-up.
2. Moving the type breaks its methods: adopted; `api.go` holds the type only, the logic moves to `calendarIDOf` and `windowFor`, and the acceptance names both call sites of `reconcile.go` and both test calls of `window_test.go`.
3. A simpler way: not adopted; keeping `reconcile.WindowRule` would make `ot` name an implementation type, so the runner-owned type stays.
4. Lock-in: adopted in part; `OpenOffset` unused and the optional three-value method are recorded in `## Left out`, and the method is not frozen because the `ot` step is deferred; one rule per reconciler stays named.
5. Claims most likely false: adopted; the `ot` item that depended on the field count left with the entry, and the grep items for methods and the type copy were added.
6. Likeliest slip: adopted; items 3 and 3a forbid a method body in `api.go` and a copy of the type, and the `ot` worker no longer starts before issue 75.
