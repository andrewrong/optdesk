# ot job `fred_series`

Order: 10

Family file: `cmd/ot/wire_fred.go`; registered with `registerJob` in an `init()` (`cmd/ot/jobreg.go`). The common rules of every job are in `docs/contracts/ot.md`.

## Window

The job states its own window through `runner.Windowed`; `ot` passes the zero `runner.WindowRule{}` (see `docs/contracts/jobs.md` for the job).

## Lines it reads

- `FRED_API_KEY` (required by `ot run-once` and `ot backfill`; `ot scheduler` lets it be empty).
- `OT_PG_DSN_SERIES_FRED`: the series store (required).

## Left out

- An empty `FRED_API_KEY` leaves it out of `ot scheduler` (reason `FRED_API_KEY is not set`, the `startOff` of the family). The job exists so that the ticker knows its schedule; the key is read once at start, so the probe fails again until a restart with the key.
- The probe (`prober.probe`) builds it anew when the key is set.

## Notes

Wired first. It runs even when the broker is down.

## Version

v1: wired by `ot run-once` (v4 of `docs/contracts/ot.md`), `ot backfill` and `ot scheduler`; moved to its own document with the job registry (no behaviour change).
