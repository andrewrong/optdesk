# Backfill: collect a day at or behind the cursor

Author: advisor of optdesk (first draft)
Date: 2026-11-07
Source: issues #146, #147 and #148. Read on main at d8bdc5f: `internal/runner/reconcile/reconcile.go`,
`internal/runner/api.go`, `internal/store/api.go` (`OpsLedger`), `cmd/ot/runonce.go`, `cmd/ot/wire.go`,
`internal/jobs/fredseries`, `docs/contracts/ot.md`.

Amended by `docs/design/backfill-any-day.md` (issue #1314): backfill has no cursor test and no advisory fence; it takes the
run lease under its own owner (Decision 3) so `ot backfill` runs while the scheduler runs; it puts the windows a day lacks
and adds the keys that today's plan has and the stored plan lacks (Decision 8). Behaviours 1, 2, 4 and 6, the texts after
them and the last out-of-scope lines below say what is true now; the `## Issues` entries are the history of the first build.

Goal (superseded by backfill-any-day on the cursor limit: the day may be ahead of the cursor too): an explicit, narrow path that collects one day at or behind the reconcile cursor. `ot run-once` collects the day
after the cursor and, since #148, refuses a day at or before it; `Reconcile` never looks behind the cursor either. The FRED
DGS10 value of 2026-11-06 was lost because an empty fetch was marked final (`capture_present` status `no_value`); #147 stops
that going forward (an empty fetch before the window close stays missing) but cannot repair a key already marked, and the
cursor is at 2026-11-06. The new path is `ot backfill --day YYYY-MM-DD [--dataset ID]`. It re-runs the keys of that day
that are missing or marked `no_value`, does not move the cursor and (as first built) did not change any plan or window; backfill-any-day supersedes the cursor limit and the no-change rule on two points: any day with a stored plan is accepted, and the windows a day lacks and the added keys are put (Behaviours 1, 2 and 6). No new dependency,
no new service, no new table, no migration.

## Modules

No change to `docs/design/modules.md`: every path below belongs to an existing row, and no row's depends cell changes.
`cmd/**` is `ot`, which already imports `runner` and the implementation package `internal/runner/reconcile`.
`jobs`, `store`, `sources` and `clock` are not touched: the ledger already has `Expected`, `Present`, `Runs`, `Window`,
`RecordRun`, `MarkPresent`, `Acquire` and `Release`, and `MarkPresent` is an upsert, so a `no_value` row can be replaced
by `present`. The series writer inserts with `ON CONFLICT DO NOTHING`, so a repeated backfill writes nothing twice.

| issue | module |
|---|---|
| runner-backfill-interface | runner |
| backfill | runner and ot |

The interface entry lies in one module. The entry `backfill` lies in two, `runner` and `ot`: this design's cut licenses it,
because the real constructor `reconcile.NewBackfillerWithRule` is named in `newDeps` of `cmd/ot` and neither half can merge
without the other (a sibling `Depends on` is not used; the entry after the interface needs only merged work). No issue edits `go.mod`, `go.sum`, `package.json` or `pnpm-lock.yaml`.

## Interface

One interface file changes: `internal/runner/api.go` (owner: runner). Everything below is added; nothing is renamed and
`Reconciler`, `NewFunc`, `WindowRule` and `NewWithRuleFunc` stay as they are, so the fakes of `cmd/ot` and of the reconcile
tests keep compiling.

| name | owner | file | declaration |
|---|---|---|---|
| `runner.Backfiller` | runner | internal/runner/api.go | `interface{ Backfill(ctx context.Context, day contracts.Day, datasets []string) (BackfillResult, error) }` |
| `runner.BackfillResult` | runner | internal/runner/api.go | `struct{ Day contracts.Day; Refused BackfillRefusal; AlreadyPresent, Filled, Absent, Missing []contracts.ExpectedKey; InWindow []contracts.ExpectedKey }` (`InWindow` added by backfill-any-day) |
| `runner.BackfillRefusal` | runner | internal/runner/api.go | `type BackfillRefusal string` with the constants `RefusedAheadOfCursor` ("ahead_of_cursor", superseded by backfill-any-day: the constant is removed) and `RefusedNoPlan` ("no_plan"); empty means the day was not refused |
| `runner.NewBackfillerWithRuleFunc` | runner | internal/runner/api.go | `func(store.OpsLedger, clock.Calendar, clock.Clock, []contracts.Job, WindowRule) Backfiller`; implemented by `reconcile.NewBackfillerWithRule` |

No error value is declared in `api.go`: no `api.go` of the repository holds an initialiser, so a refusal is data in the
result (`Refused`, with a nil error and every other field empty) and `ot` turns it into its own message and exit 1. An
error return is kept for failures (lease, ledger, unknown dataset, key without a job).

`BackfillResult` fields: `Filled` keys now `present` (they were missing or `no_value`); `Absent` keys the job answered with
no value or an empty answer after the window close (stored `no_value`, as `Reconcile` stores it); `Missing` keys with no
usable answer, left unmarked (a job error); `AlreadyPresent` keys that were `present` and were not run; `InWindow` (added by backfill-any-day) keys that were missing or `no_value`, were not run because their window had not closed at the clock, and are left as they were.

`ot` composes `deps.newBackfiller` (type `runner.NewBackfillerWithRuleFunc`, real value `reconcile.NewBackfillerWithRule`)
beside `deps.newRunner`.

### CLI form (owner: ot, documented in `docs/contracts/ot.md`)

`ot backfill --day YYYY-MM-DD [--dataset ID]`

- `--day` is required (no default: a backfill is never "today"); a value that is not `YYYY-MM-DD` is an error, as in
  `dayOrDefault`. `--dataset` is optional, one dataset id (`fred_series` has the dataset ids its `Plan` returns); absent
  means every key of the day. A dataset id with no key on that day is an error that names it.
- Real profile only; the demo profile is refused with its own text (below). Same `.env` keys as `run-once`
  (`OT_PG_DSN_RUNNER`, `OT_PG_DSN_SERIES_FRED`, `FRED_API_KEY`); values are never printed; no pgx error is wrapped.
- Writes nothing to standard output. On success it logs one line `backfill <day> done` with `status=ok` and the four
  counts. Exit 1 when the command refused, or any selected key ended in `Missing`; `Absent` is not an error.

### Behaviour (owner: runner, `reconcile.Backfill`)

1. **Refusal, before any write.** There is no cursor test: the cursor is not read, and a day ahead of the cursor, at it or
   behind it is treated alike (`RefusedAheadOfCursor` is removed, backfill-any-day Decision 1). A day whose stored plan is
   empty (`Expected` returns no keys) returns `Refused == runner.RefusedNoPlan`, with a nil error and no write.
   The lower bound is therefore the first day the runner ever planned: a non-session day and a day before collection began
   are both refused, and there is no constant to maintain. A key whose window has not closed is not run and is listed in
   `InWindow`; every other missing or `no_value` key runs.
2. **Keys.** The keys are the stored `Expected` keys of the day (what the day expected when it was reconciled), filtered by
   `datasets`. Backfill also adds the keys that today's plan has for the day and the stored plan lacks (the added keys, backfill-any-day
   Decision 8): they are appended to the stored plan with `PutExpected(day, hash of today's plan, stored + added)`, after
   their windows are put; no stored key is removed or changed, and a stored key that today's plan no longer returns stays in
   the plan and is an error as below. The `datasets` filter applies to the stored and added keys together.
   **Key to job:** the existing lookup of `reconcile.go`, the method `plan(ctx, day)`, which calls `Plan(ctx, day)` of every
   registered job and returns `p.job`, the map from `contracts.ExpectedKey` to job. Backfill uses the map of `plan(ctx, day, nil)` only to find the job of a key; the hash it writes for the added keys is
   that of the plan `Reconcile` makes for the day (backfill-any-day Decision 8, step 4). A job's id is not a dataset id: the map is the only relation, and `contracts.Job`
   is not extended. All keys are validated before any key runs: a `datasets` id with no stored key on the day, or a stored key
   that is absent from the map (no registered job returns it today, for example a series dropped from the manifest), is an
   error that names the dataset id; then nothing has run and nothing is written.
3. **Which keys run.** Status `present`: not run, listed in `AlreadyPresent`. Status `no_value` or no status: run once in
   this call, whatever the earlier runs and the two-run cap say. Finality of an earlier attempt is overridden for these
   keys only and only by this explicit command: `Reconcile` is unchanged and still treats `no_value` as final.
4. **Ledger.** Takes the same run lease as `Reconcile` (same TTL, renewed while it runs) under its own owner `backfill:` plus
   eight random hex digits, made once per call (backfill-any-day Decision 3), so it cannot overlap `Reconcile`, `run-once`
   or another backfill, and the scheduler keeps running: its tick returns Retryable while a backfill runs. There is no
   advisory fence. A held lease fails as `Acquire` fails (Retryable). Each key gets a new run row with ID `<day>|<dataset>|<subject>|<n>`
   where `n` is the number of runs of that key plus one (the existing minting), `ReasonCode` `backfill`, and the result
   fields `Reconcile` sets. A running run with a live lease makes the key `Missing` without a new run; an expired one is
   closed as `crashed` first, as `Reconcile` does. The first run of a key is never rewritten.
5. **Marks and classification.** `run` returns only `(bool, error)` and `reconcile.go` is not edited, so `backfill.go` has its
   own helper `backfillKey` that calls `run` and then reads `Present` for that day once: `present` is `Filled`, `no_value`
   is `Absent`, a key with neither mark (or still `no_value` after a job error, told apart by the new run row's `ErrorCode`)
   is `Missing`. Status `present` when the job returns usable rows (the runner never reads content: the job's `Usable` is
   believed, see Left out for the series writer's conflict key); `no_value` for a typed `NoValue` error or an empty answer
   after the stored window close; an empty answer before the close (the rule rests on the key's own window close, not on the cursor: such a key is not run and is listed in `InWindow`, Behaviour 1) and any other job error leave the status as it was, the run row carries the error.
   A key never moves from `present` to anything else.
6. **Cursor, plan, window.** `SetCursor` is never called. `PutWindow` is called only for a dataset of the day that has no stored
   window and whose job has a window rule for the day, and `PutExpected` only when there are added keys (Behaviour 2);
   both happen before any key runs. The close of the window is read with `Window(dataset, day)`; a dataset with still no
   window has no close, and an empty answer is then `no_value`.
7. **Idempotent.** A second call for the same day and datasets finds every filled key `present` and runs nothing; the
   result lists them in `AlreadyPresent`. A key that stays `Absent` is run again by each call (one new run row per call),
   because the explicit command is the owner asking again.

Texts, owned by `cmd/ot`: the text for `RefusedAheadOfCursor` is gone with the constant; for `RefusedNoPlan` `no plan was recorded for day <day>: nothing to backfill` (superseded by backfill-any-day issue 4: `cmd/ot/backfill.go` and `docs/contracts/ot.md` say "not planned yet or was never reconciled"). The demo profile gets its own text:
`backfill needs a real profile; the demo profile opens no store`. `run-once` keeps its #148 error and adds the hint
`use ot backfill --day <day>`. The owner finds the day to backfill by reading `ops.capture_present` by hand; there is no
list mode.

## Issues

Items under `Checked by CI` are decided by the workflows of the head commit; every other item is a fact read from the diff,
with an offline grep or a named test. Every issue edits the files of one module, except the one cross-module entry `backfill`, which this design licenses
(`## Modules`). Both entries build from merged work and the interface alone; neither needs a sibling's output.

### runner-backfill-interface (interface)

The runner declares the backfill contract
  Goal: `internal/runner/api.go` declares `Backfiller`, `BackfillResult`, `BackfillRefusal` with its two constants and `NewBackfillerWithRuleFunc` exactly as `## Interface` gives them.
  Files: internal/runner/api.go, internal/runner/api_test.go
  Design: docs/design/backfill.md#interface
  Acceptance:
  1. `api.go` holds exactly these declarations: the types `Backfiller`, `BackfillResult` and `BackfillRefusal`, the constants `RefusedAheadOfCursor` and `RefusedNoPlan`, and the func type `NewBackfillerWithRuleFunc`, with doc comments that state refusals, that the cursor is not moved and that a `present` key is not run; no `errors.New`, no `var` initialiser, no logic; `Reconciler`, `NewFunc`, `WindowRule` and `NewWithRuleFunc` are unchanged (`git diff` shows only added lines in `api.go`).
  2. `api_test.go` tests the contract from both sides: a fake `Backfiller` that returns a `BackfillResult` with `Refused` set to `RefusedAheadOfCursor` or `RefusedNoPlan` and a nil error, and a caller that branches on `Refused` (no `errors.Is`, no error variable).
  3. `go build ./...` has no new dependency (`git diff -- go.mod go.sum` is empty); no file outside the two is in the diff.
  4. Every test added calls `t.Parallel()` and none sleeps.
  Checked by CI: ci-go (build, vet, test, golangci-lint, go-arch-lint); the design verdict on the pull request (an interface change).
  Depends on: none (merged work only). Order: before `backfill`.
  Size: S

### backfill (runner and ot)

The runner backfills a day at or behind the cursor and `ot backfill` wires it
  Goal: `reconcile.NewBackfillerWithRule` returns a `runner.Backfiller` that follows `## Interface`, Behaviour 1 to 7, and `ot backfill --day YYYY-MM-DD [--dataset ID]` opens the stores, wires the `fred_series` job as `run-once` does and calls it.
  Files: internal/runner/reconcile/backfill.go, internal/runner/reconcile/backfill_test.go, docs/contracts/runner.md, cmd/ot/backfill.go, cmd/ot/backfill_test.go, cmd/ot/runonce.go, cmd/ot/runonce_test.go, cmd/ot/wire.go, cmd/ot/main_test.go, docs/contracts/ot.md
  Design: docs/design/backfill.md#behaviour-owner-runner-reconcilebackfill and docs/design/backfill.md#cli-form-owner-ot-documented-in-docscontractsotmd
  Cross-module: this one entry lies in two modules, runner and ot, licensed by this design (see `## Modules`): the real wiring in `newDeps` names the constructor, so a split would leave `ot` unbuildable until the runner half merged.
  Acceptance:
  1. Scope: no file outside the `Files` line is in the diff (`git diff --stat`); `internal/runner/reconcile/reconcile.go` is not among them. `internal/runner/reconcile/backfill.go` holds `var _ runner.NewBackfillerWithRuleFunc = NewBackfillerWithRule`, reuses the existing `run` and `markCrashed` methods and adds the helper `backfillKey` that classifies Filled, Absent and Missing by reading `Present` after `run`; `Reconcile` is unchanged. The only size rule is AGENTS.md: a new file under 600 lines.
  2. `TestBackfillRefusals` (table, fake ledger and fake calendar of `fakes_test.go` style): empty cursor and a day after the cursor give a nil error and `Refused == runner.RefusedAheadOfCursor`; a day at or before the cursor with no stored plan gives `runner.RefusedNoPlan`; in all cases the fake ledger recorded no `RecordRun`, `MarkPresent`, `SetCursor`, `PutExpected` or `PutWindow`.
  3. `TestBackfillNoValueBecomesPresent`: cursor at day D, key marked `no_value` with one finished run of reason `empty_before_close` or an empty run, job returns one usable row; result `Filled` holds the key, the ledger holds `present`, a second run row with ID ending `|2` and reason `backfill` exists, the first run row is byte-for-byte as before, and `Cursor` still reads D.
  4. `TestBackfillPresentIsNotRun`: a `present` key is in `AlreadyPresent`, the job's `Run` is never called, no run row is added.
  5. `TestBackfillIdempotent`: two calls in a row; the second runs no job and returns the key in `AlreadyPresent`; a key whose job returns an empty answer after the window close stays `no_value`, is listed in `Absent`, and each call adds one run row.
  6. `TestBackfillJobError`: a job error leaves the key unmarked (or `no_value` when it was), lists it in `Missing`, and records the error code in the run row; a job `NoValue` error lists the key in `Absent`.
  7. `TestBackfillDatasetFilter`: with `datasets` set only those keys run; an id with no key on the day is an error naming it; a stored key that today's `Plan` does not return is an error naming its dataset id (the lookup of Behaviour 2); in both cases no job `Run` was called, even for keys that came before the bad one (validate all, then run).
  8. `TestBackfillLease`: the lease is acquired once and released on every path (success, refusal, job error); a fake ledger whose `Acquire` fails makes `Backfill` return that error and run nothing.
  9. `TestBackfillLiveRunning`: a running run with a live lease gives `Missing` and no new run row; an expired one is closed `crashed` and the key is run.
  10. `docs/contracts/runner.md` gains a short paragraph for `Backfill` that says the cursor, plans and windows are untouched.
  11. Every test uses the fake clock of the kit (no sleep), `t.Parallel()`, no database, no network.
  12. `ot backfill` is registered in `cmd/ot/backfill.go` (`register("backfill", ...)`); `deps` has `newBackfiller runner.NewBackfillerWithRuleFunc`, set to `reconcile.NewBackfillerWithRule` in `newDeps` (item 1 creates the constructor in the same entry, so the real value compiles at this merge); the test deps of `main_test.go` get a fake. `runonce.go` is edited only to share the store and job wiring (a helper both commands call) and to add the hint to its cursor error; `run-once` behaviour is otherwise unchanged and `runonce_test.go` changes only the asserted text of the cursor error (the hint).
  13. `TestBackfillFlags` (table): no `--day` is an error naming the flag; `--day 2026-13-40` is an error; the demo profile is refused with the text `backfill needs a real profile; the demo profile opens no store`; none of these opens a store (the fake `openLedger` is not called).
  14. `TestBackfillCallsRunner`: with fakes, `Backfill` is called once with the parsed day and the `--dataset` value (a nil slice without the flag); the log line is `backfill <day> done` with `status=ok`, standard output is empty, and every opened store is closed (fake `Close` counters) on success, on a runner error and on a refusal.
  15. `TestBackfillErrors`: a fake `Backfiller` returning `Refused` `RefusedAheadOfCursor` or `RefusedNoPlan` (nil error) gives exit 1 with a message that names the day and says `run-once` for the first; a result with a non-empty `Missing` gives exit 1 and names the count; a result with only `Absent` gives exit 0.
  16. No error text contains a DSN, a key value or a pgx message (the test greps the captured stderr for the fake values).
  17. `docs/contracts/ot.md` lists `backfill` (the subcommand count in its first paragraph, the `--day` table row for backfill as required, `--dataset`, the log line and exit codes) and adds `run-once` pointing at it.
  18. Every test uses `t.Parallel()` and `t.TempDir()`, none sleeps, none connects to a database.
  Checked by CI: ci-go (build, vet, test, golangci-lint, go-arch-lint components `runner/reconcile` and `ot`; each new file stays under 600 lines).
  Depends on: runner-backfill-interface merged (it implements the declared names).
  Size: L

## Left out

- The series writer's conflict key (module store, with the job in module jobs). `insertPointSQL` is `ON CONFLICT DO NOTHING`
  on `(series_id, observation_date, realtime_start)`. If an earlier fetch stored a `.` point (nil value) with the same
  `realtime_start` as the recovered value, the real value is dropped while the job still reports `Usable > 0`, and the key
  would be marked `present` with no value stored. The runner never reads content, so it cannot check this; marking `present`
  on the job's word is kept, and the fix (the job counts only rows actually inserted, or the writer updates a nil row) is a
  follow-up for jobs/store, not this design. For the DGS10 day of 2026-11-06 the first fetch is said to have been empty
  (no row at all); the owner checks `obs.series_point` for that day by hand before running the command.

- Moving the cursor backwards, or a `--force` that re-runs `present` keys. Rewriting a stored value is not possible anyway:
  the series writer is `ON CONFLICT DO NOTHING`.
- A range (`--from`, `--to`) or an "all gaps" mode. One day per call; a loop in the shell does a range. A range wants a gap
  report from the ledger first.
- Backfill of days before the first recorded plan (collection before the runner existed). It wants a plan for a day the
  ledger never saw, which is a separate decision about `PutExpected` for the past.
- Changing `Reconcile` so a later `run-once` heals a `no_value` itself; #147 covers new days.
- Backfilling a day whose stored plan differs from today's plan: the added keys (keys of today's plan the stored plan lacks) are added and run (Behaviour 2); a stored key that today's `Plan` no longer returns stays in the plan and is an error, and no stored key is removed or changed.
- Datasets other than `fred_series`: `ot` wires one job today (as `run-once`); a second job is wired in both commands by
  the issue that adds it.

## Adversarial

1. Gap (validate all keys first, own demo text, no list mode): adopted for the first two (Behaviour 2, Texts); list mode not adopted, the owner reads `ops.capture_present` (Left out of scope, said in Texts).
2. Writer conflict key may swallow the recovered value: stated, not fixed here: it belongs to jobs/store, named in Left out; the runner cannot read content, so `present` stays on the job's `Usable`.
3. Fold the interface issue into the next entry: not adopted, AGENTS.md lands the interface issue first; the split stays.
4. Lock-in (api.go initialisers): adopted, refusals are a `BackfillRefusal` field, no `errors.New` in `api.go`; exit-code and `ReasonCode` `backfill` vocabulary accepted and written in the Interface.
5. ot-backfill needs runner-backfill: first adopted as a `Depends on`, then replaced after the design session of abe0d44 by one cross-module entry (see 7).
6. Slip causes: adopted `runonce_test.go` in Files, the 200-line cap dropped (600-line rule only), and the Absent/Missing helper `backfillKey` named with `reconcile.go` unedited.
7. Design session of abe0d44 (crewd, verdict changes): answered. The two issues after the interface became one cross-module entry licensed under `## Modules`; Interface acceptance 2 now tests `Refused`; the key-to-job lookup is named (`plan`); the declarations are listed by name; the broken sentence in Behaviour 1 is fixed.
8. Design session of 92189e4 (crewd, verdict changes): answered. Acceptance 10 and 17 no longer say `no other file is in the diff`; the scope is stated once in Acceptance 1; two files named `backfill.go` are now named by path.

Full answer: scratchpad adversarial-backfill.md of the dispatcher window (2026-11-07)
