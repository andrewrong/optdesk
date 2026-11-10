# ot job `option_ledger`

Order: 50

Family file: `cmd/ot/optionledger.go`; registered with `registerJob` in an `init()` (`cmd/ot/jobreg.go`). The common rules of every job are in `docs/contracts/ot.md`.

## Window

The job states its own window through `runner.Windowed`; `ot` passes the zero `runner.WindowRule{}` (see `docs/contracts/jobs.md` for the job).

## Lines it reads

- `OT_PG_DSN_OPTION_LEDGER` (the trade store and, through `deps.openFlexSections`, the Flex section store).
- `$OT_HOME/flex.toml` (`flex.Load`) and the variables it names (`token_env`, `account_env`).

## Left out

- Absent, never failed, with one log line: an empty `OT_PG_DSN_OPTION_LEDGER`, no `flex.toml` (`flex.ErrFlexNotConfigured`) or an unreadable one.
- A login whose token variable is empty gets no source and one log line naming the login and the variable: a `required` login fails the job result, any other leaves it ok with a `degraded` record.
- If the section store cannot be opened the section step is off (one `degraded` record, no value).

## Notes

It follows `option_eod`. No token, account number or DSN value is printed or logged. `ot ledger` and `ot ledger-import` read the same store.

## Version

v19: introduced; v20: gets the Flex section store; moved to its own document with the job registry (no behaviour change).
