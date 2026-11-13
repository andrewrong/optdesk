Order: 55

# ot job `statement_capture`

Family file: `cmd/ot/statementcapture.go`; registered with `registerJob` in an `init()` (`cmd/ot/jobreg.go`). The common rules of every job are in `docs/contracts/ot.md`; the job itself is `docs/contracts/jobs/statement_capture.md`. Design: `docs/design/statement-account.md#5 statement-wire`.

## Lines it reads

- `OT_PG_DSN_OPTION_LEDGER`: the flexsection store (opened through `openFlexSections`, read for the sections `OpenPosition`, `EquitySummaryByReportDateInBase` and `ChangeInNAV`).
- `OT_PG_DSN_ACCOUNT`: the account store the captures are written to.
- `OT_PG_DSN_POSMARK`: the mark store the marks of the captured positions are written to.
- `$OT_HOME/accounts.toml`: its `statement` line names the aliases the job captures.

The calendar and the clock are those of the run.

## Left out

The job is left out, with one "job left out" line naming the missing line or file and no value, and that is not an error, when:

- the book has no statement alias (no `statement` line),
- `accounts.toml` cannot be loaded,
- one of the three lines of `.env` is not set, or a store cannot be opened.

## Console

The row of the job console is `anyDay("statement_capture", "insert_only", textInsertOnly)`: any day, force no, because a stored capture of a statement day is never replaced.

## Page configs

`positionsPageConfig` and `householdConfig` set `Statement` to the statement aliases of the book (nil for a book that cannot be loaded and for the demo book).

## Version

v1: wired at order 55.
