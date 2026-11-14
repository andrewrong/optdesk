# ot job `book_risk`

Order: 210

Family file: `cmd/ot/bookrisk.go`; registered with `registerJob` in an `init()` (`cmd/ot/jobreg.go`). The common rules of every job are in `docs/contracts/ot.md`. The job itself is `docs/contracts/jobs/book_risk.md`; the design is `docs/design/strategy-risk.md` (Decisions 9, 11 and 12).

## Window

The job states its own window through `runner.Windowed`; `ot` passes the zero `runner.WindowRule{}`. The scheduler runs it at `session_close` plus 45 minutes, after `option_candidates` (offset 30 minutes); it puts only. It has no dry run: `dryRunJobs` of `cmd/ot/dryrun.go` does not name it.

## Wiring

`cmd/ot/bookrisk.go` is the only file that imports `internal/strategy/risk/bsm`, `stress` and `fit`. It builds `stress.New(bsm.New())` and `fit.New(stresser)` and hands the job the stresser and the fitter in `bookrisk.Stores`. Its constructor is the ext deps `bookRiskDeps` (`cmd/ot/depsext.go`), so `deps` and `newDeps` are not edited.

## Lines it reads

- `OT_PG_DSN_CANDIDATE`: `strategy.result`, read (the candidate rows) and written; required.
- `OT_PG_DSN_OPTION_LEDGER`: the option ledger store; required.
- `OT_PG_DSN_ACCOUNT`: the account store; required.
- `OT_PG_DSN_POSMARK`: the mark store, optional (without it the share marks are absent).
- The compute stores (session days, option_eod, option terms, series) and `$OT_HOME/accounts.toml` for the aliases, main first.

## Left out

- Absent for a deps without the constructor.
- With an empty required line the job is left out of `ot run-once`, `ot backfill` and `ot scheduler`, with one logged line `<line> is not set` that names the key and never a value. A failed open is an error that names the key.
- The demo profile refuses `ot run-once` and `ot backfill` as for every job.

## Job console

An any-day row with force: the rows are insert-only under a new run id and a rerun writes a new run id that readers take as the newest.

## Version

v1: introduced.
