# ot job `iv_history`

Order: 150

Family file: `cmd/ot/wire_ivhistory.go`; registered with `registerJob` in an `init()` (`cmd/ot/jobreg.go`). The common rules of every job are in `docs/contracts/ot.md`. The job itself is `internal/jobs/ivhistory` (see `docs/contracts/jobs.md`).

## Key and reading

The job plans one key `market.iv_ibkr|underlying|<name>` per name of the watch list and stores one `market.iv_ibkr` reading per bar of IBKR's implied-volatility history. The close of a bar becomes the stored text through `ivbar.FromClose`.

## Lines it reads

- `OT_PG_DSN_READINGS`: the reading store opened for `compute_figures` (read and write).
- `$OT_HOME/watch.toml`.
- The history client of `bars_refresh` and `fx_refresh` (`$OT_HOME/run/ibkr.sock`, `deps.newHistory`); dialled by the job itself only when their step did not run.

## Left out

- Absent for a deps without `newHistory` or without the reading store.
- Broker down at start, or `bars_refresh` left on its stand-in history client (no `OT_PG_DSN_BAR`): one log line `job left out` naming `iv_history` and `broker is not reachable`; the other jobs run as before. It is never probed back in (a family without a probe case stays out until a restart).

## Notes

A history client the job dials goes to the closer set that `withWiring` closes. Its slot in the day is in `internal/jobs/ivhistory/job.toml` (before `compute_figures`); its wiring order is 150.

## Version

v1: new family `iv_history` (compute-stage2, entry 8).
