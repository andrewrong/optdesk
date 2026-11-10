# ot job `quote_snapshot`

Order: 60

Family file: `cmd/ot/wire_quotesnapshot.go`; registered with `registerJob` in an `init()` (`cmd/ot/jobreg.go`). The common rules of every job are in `docs/contracts/ot.md`.

## Window

The job states its own window through `runner.Windowed`; `ot` passes the zero `runner.WindowRule{}` (see `docs/contracts/jobs.md` for the job). The slot keys use the manifest zone of the job; its grace is the job's own.

## Lines it reads

- `OT_PG_DSN_QUOTE_SNAPSHOT`: the writer store (required).
- `OT_PG_DSN_MASTER`: the instrument store.
- The broker socket `$OT_HOME/run/ibkr.sock`.

## Left out

- A failed broker dial at start: `broker is not reachable` (`startOff` of the family); the stand-in broker answers retryable; the probe brings the job back.

## Notes

`bars_refresh` and `fx_refresh` (wired by `cmd/ot/wire.go`) come before it, `compute_figures` after it.

## Version

v1: wired by `ot run-once`, `ot backfill`, `ot scheduler`; moved to its own document with the job registry (no behaviour change).
