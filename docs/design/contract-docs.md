# Contract documents: jobs and ot

Author: advisor of optdesk (first draft, revision 2 after the adversarial and design reviews)
Source: the `## Left out` sections of `docs/design/week1-ot.md` and `docs/design/week1-wiring.md` ("`docs/contracts/jobs.md`
and the contract documents of ot: not touched here"). Read on main at c63cbb4.

Goal: the contract documents describe what merged code does today. `docs/contracts/jobs.md` already exists on main
(merged with the job); it is true as far as it goes but omits the error kinds of `Run`, the constructor check and the
manifest check. No `docs/contracts/ot.md` exists. This design completes the first, gives the second a home in the module
map, and names the issue that writes it. Documentation only: no code, no test, no interface file changes.

## Modules

One row changes in `docs/design/modules.md`, carried by this pull request: the row `ot` gets `docs/contracts/ot.md` in
its paths (`cmd/** docs/contracts/ot.md`) and in its contract-document cell of the second table (was `none`). The path
then belongs to `ot`, so a later `cmd/ot` issue updates the document in its own pull request. `docs/contracts/jobs.md`
is already matched by the row `jobs`.

| issue | module |
|---|---|
| jobs-contract-doc | jobs |

This pull request also fixes one stale line in `docs/design/week1-wiring.md` (`## Left out`: "jobs.md ... does not exist on
origin/main"); it has existed since the jobs issue.

## Interface

No interface change; the one entry is not marked (interface) and has no sibling dependency.

## Issues

### jobs-contract-doc

Files: docs/contracts/jobs.md
Depends on: merged work only (`internal/jobs/fredseries` and `internal/jobs/api.go` are on main).

Goal: complete the contract of the `fred_series` job against `internal/jobs/fredseries/plan.go` and `run.go`.

Acceptance:

1. `docs/contracts/jobs.md` names every field of `internal/jobs/fredseries/job.toml`: grep finds `version`, `id`,
   `series`, `[window]`, `rule` and `lateness_hours`, and the phrase "needs an id and a series list" (the text of the
   manifest error in `plan.go`).
2. The document says `New` returns an error when the source, the writer or the clock is nil: grep finds "nil".
3. The Outputs section gives the error kinds of `Run`: a fetch error that is a `contracts.TypedError` is returned
   unchanged, else it is `Permanent`; a failed `InsertSeriesPoints` is `Retryable` unless the writer's error is already
   typed (`asTyped`), and `Usable` is 0 then. Grep finds `asTyped`, `Permanent`, `Retryable` and "unless".
4. The document names `jobs.Registry` and `jobs.NewFredSeriesFunc`: grep finds both.
5. The Contract tests section cites `TestRegistryContract`, `TestPlanHash`, `TestEmbeddedManifest`, `TestRunWritesOnce`
   and `TestPlanLateRunRecovery`, and each appears in a `func Test` line of `internal/jobs/api_test.go`,
   `internal/jobs/fredseries/plan_test.go` or `internal/jobs/fredseries/run_test.go`. Grep finds "no test" in the
   document, saying the error kinds of `Run` have no test yet.
6. The document keeps the headings Responsibility, Inputs, Outputs, Invariants, Version history, Contract tests, Jobs and
   slots, Forbidden: grep for each `## ` line.
7. No file other than `docs/contracts/jobs.md` is changed, and it stays under 600 lines: `git diff --name-only` lists one
   path and `wc -l` is below 600.

## Left out

- `ot-contract-doc` (module ot, file `docs/contracts/ot.md`): not cut here. It should document `run-once`, `page`, `serve`,
  `setup`, `version` and, once they exist, `ot migrate` and `ot calendar load` of `docs/design/week1-run.md`. At this head
  `cmd/ot` has no migration subcommand, and `docs/design/week1-ot.md` `## Left out` says running migrations from `cmd/ot`
  needs a package outside the eleven the row `ot` may import, which is a change of `modules.md` and a design of its own
  (that of `week1-run.md`). It is the follow-up to cut after the issue `ot-migrate-load` (#75) merges, so the document is
  true at birth. Its items, found by the adversarial pass, are: flags and defaults (`--day`, `--out`, `--addr`), the
  environment names, the `.env` rules, exit codes each tied to its condition (a line with `2` and "unknown", one with `1`
  and "command failed", one with `0` and "success"), the demo profile when `OT_HOME` is unset, `serve` building per request
  and answering 500, and the `version` output. The path is already in the row `ot`, so that follow-up only adds the file.
- Documents for modules without a contract file on main (`broker`, `compute`, `notify`): their first issues come in
  later weeks.
- Leaving `docs/contracts/ot.md` in the `docs` row: not chosen, every other contract document sits in its module row.
- One combined issue for both documents: not chosen, the files belong to two modules.
- A generated flag reference from the code: not chosen, nothing generates it.
- The stale line of `week1-wiring.md`: fixed here. The line of `week1-ot.md` ("not touched here") is a true statement about
  that design and stays.
- Anything about behaviour that code does not have yet (the window rule, the broker): the documents describe merged code only.

## Adversarial

Read in contract-docs.adversarial.md (kept on the dispatcher's machine).

1. Gap: adopted. The list of ot behaviours the document must cover is recorded under `## Left out` for the follow-up.
2. Stale at birth: adopted. The ot document waits for `ot-migrate-load` (#75) and is left out of this design's issues.
3. Simpler way: partly adopted. The design stays only because the `modules.md` row change needs one; the ot issue is cut.
4. Lock-in: adopted. `docs/contracts/ot.md` moves to the `ot` row so that `cmd/ot` issues update it.
5. False claims: adopted. jobs item 3 reads "Retryable unless already typed"; the open "matches the design" items are replaced
   by greps of named headings, terms and tests.
6. Likeliest slip: adopted. The wording of jobs item 3 is explicit, and the exit-code greps of the follow-up tie each code
   to its condition.
