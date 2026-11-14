# ot job `option_eod`

Order: 20

Family file: `cmd/ot/wire_optioneod.go`; registered with `registerJob` in an `init()` (`cmd/ot/jobreg.go`). The common rules of every job are in `docs/contracts/ot.md`.

## Window

The job states its own window through `runner.Windowed`; `ot` passes the zero `runner.WindowRule{}` (see `docs/contracts/jobs.md` for the job). It is built through `buildOptionEod` over the watch list (`optioneod.NewWithWatch`); the dry run uses the same constructor path.

## Lines it reads

- `OT_PG_DSN_OPTION_EOD`: the writer store (required).
- `OT_PG_DSN_MASTER`: the instrument store (required).
- `$OT_HOME/watch.toml`: the watch list.
- The broker socket `$OT_HOME/run/ibkr.sock`.

## Left out

- A failed broker dial at start (`broker is not reachable`, the `startOff` of the family): the job holds a stand-in broker whose every call answers retryable, and `ot scheduler` lists it as disabled; the probe dials anew and brings it back on the first passing tick.

## Notes

The two option jobs and `quote_snapshot` share the one fresh dial of a probe pass. `--dry-run --job option_eod` smoke-tests it against the live socket.

## Forced run

A forced run (`ot backfill --force`, or the force box of the job console) replaces the rows of today's slot: the store entries of `docs/design/option-captures.md` (Decision 4) write the new rows over the stored ones. It is offered only for today; a past day is refused as before. It never replaces a stored good quote with a worse one: a later row that is an error, a timeout or off-session leaves the stored quote in place.

## Version

v1: wired by `ot run-once`, `ot backfill`, `ot scheduler`; moved to its own document with the job registry (no behaviour change).
