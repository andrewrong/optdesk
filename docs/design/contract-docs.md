# Contract documents: jobs and ot

Author: advisor of optdesk (first draft)
Source: `## Left out` of `docs/design/week1-ot.md` and `docs/design/week1-wiring.md` ("`docs/contracts/jobs.md` and the
contract documents of ot: not touched here"). Read on main at c63cbb4.

Goal: the contract documents describe what merged code does today. `docs/contracts/jobs.md` already exists on main
(merged with the job); it is true as far as it goes but omits the error kinds of `Run`, the constructor check and the
manifest check. No `docs/contracts/ot.md` exists. The documents follow the headings of `docs/contracts/store.md`
(Responsibility, Inputs, Outputs, Invariants, Version history, Contract tests, Forbidden). Documentation only: no code,
no test, no interface file changes.

## Modules

None changed. `docs/contracts/jobs.md` is matched first by the row `jobs` (`docs/contracts/jobs.md`). There is no row
for an ot contract document: `cmd/**` is the ot row, so `docs/contracts/ot.md` is matched by the last row `docs`
(`docs/**`). One issue per module follows from that.

| issue | module |
|---|---|
| jobs-contract-doc | jobs |
| ot-contract-doc | docs |

## Interface

No interface change. Neither issue is marked (interface); the two are independent and have no sibling dependency.

## Issues

### jobs-contract-doc

Files: docs/contracts/jobs.md
Depends on: merged work only (`internal/jobs/fredseries` and `internal/jobs/api.go` are on main).

Goal: complete the contract of the `fred_series` job against `internal/jobs/fredseries/plan.go` and `run.go`.

Acceptance:

1. `docs/contracts/jobs.md` names every field of `internal/jobs/fredseries/job.toml` (`version`, `id`, `series`,
   `[window]` with `rule` and `lateness_hours`) and says that `New` fails when `id` or `series` is empty: grep for
   `lateness_hours` and for a sentence with `id` and `series` and "empty" or "needs".
2. The document says `New` returns an error when the source, the writer or the clock is nil, and builds the job from
   the embedded manifest only.
3. The Outputs section gives the error kinds of `Run`: a fetch error is returned unchanged when it is a
   `contracts.TypedError`, else `Permanent`; a failed `InsertSeriesPoints` is `Retryable` and sets `Usable` to 0.
4. The document names `jobs.Registry` and `jobs.NewFredSeriesFunc` as the two declarations of `internal/jobs/api.go`.
5. The Contract tests section lists exactly the test functions that exist in `internal/jobs/api_test.go`
   (`TestRegistryContract`) and in `plan_test.go` and `run_test.go` of `internal/jobs/fredseries`; `grep -n 'func Test'`
   on those three files gives the names the document cites, and no name the document cites is missing from them.
6. No file other than `docs/contracts/jobs.md` is changed, and the changed document stays under 600 lines.
7. The result matches the part of this design named jobs-contract-doc: the document states nothing that the code on
   main does not do.

### ot-contract-doc

Files: docs/contracts/ot.md
Depends on: merged work only (`cmd/ot` is on main).

Goal: a new contract document of the `ot` binary, from `cmd/ot/main.go`, `runonce.go`, `page.go`, `serve.go`,
`setup.go`, `day.go`, `env.go` and `version.go`.

Acceptance:

1. `docs/contracts/ot.md` exists with the headings Responsibility, Inputs, Outputs, Invariants, Version history, Contract
   tests and Forbidden, as in `docs/contracts/store.md`.
2. The Inputs section lists the subcommands `run-once`, `page`, `serve`, `setup` and `version`, and their flags exactly
   as the code declares them: `--day` (run-once, page, serve; `YYYY-MM-DD`, default today in `America/New_York`), `--out`
   (page; default standard output), `--addr` (serve; default `127.0.0.1:8080`), and the one argument `pg` of `setup`.
   Grep for each flag name and each default in the document.
3. It lists the environment: `OT_HOME`, `OT_PROFILE`, `OT_PG_ADMIN_DSN` (setup), and the `.env` names read from
   `$OT_HOME/.env`: `OT_PG_DSN_RUNNER` and `OT_PG_DSN_SERIES_FRED` and `FRED_API_KEY` (run-once), `OT_PG_DSN_PAGES` (page and
   serve). It says that values are never printed and that the file is read without loading it into the process
   environment.
4. It states the exit codes of `run` in `cmd/ot/main.go`: 0 on success, 1 when a subcommand returns an error (this
   includes a flag parse error), 2 when the profile cannot be loaded, no subcommand is given or the name is unknown.
   Grep for "exit" and each of `0`, `1`, `2` in that section.
5. It states the demo profile rule: `run-once` is refused; `page` and `serve` use three fixed `DGS10` points and open no
   store; `setup` is not restricted by the code and the document does not claim it is.
6. It states what is not done: `setup pg` runs no migration and `master.session_day` stays empty, so a real `run-once`
   ends in `UnknownCalendar` until the owner applies the migrations; `run-once` uses `runner.NewFunc` with the zero window
   rule (calendar `XNYS`, no offsets).
7. The Contract tests section lists test names that exist in `cmd/ot/*_test.go` (for example `TestRunOnceDemoRefused`,
   `TestUnknownCommandIsUsageError`, `TestSetupArgumentMustBePg`, `TestServeOnBuildError500`); each cited name appears in a
   `func Test` line there.
8. The Forbidden section says: no secret or DSN value in any log line or error text, no `time.Sleep`, and no call of an
   implementation outside `cmd/ot/wire.go` by name.
9. No file other than `docs/contracts/ot.md` is changed, the document stays under 600 lines, and it matches the part of
   this design named ot-contract-doc.

## Left out

- Documents for modules without a contract file on main (`broker`, `compute`, `notify`): their first issues come in
  later weeks.
- Putting `docs/contracts/ot.md` in a row of its own, or adding it to the `ot` row: not chosen; a change of
  `docs/design/modules.md` is a design of its own for one file, and the `docs` row already owns it.
- One combined issue for both documents: not chosen, the files belong to two modules.
- A generated flag reference from the code: not chosen, nothing generates it and the flag list is five lines.
- The stale lines in `week1-ot.md` and `week1-wiring.md` ("does not exist on origin/main"): accepted designs are not
  edited.
- Anything about behaviour that code does not have yet (migrations from `cmd/ot`, the window rule, the broker): the
  documents describe merged code only.

## Adversarial

pending
