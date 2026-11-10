# ot job `earnings_refresh`

Order: 90

Family file: `cmd/ot/wire_earnings.go`; registered with `registerJob` in an `init()` (`cmd/ot/jobreg.go`). The common rules of every job are in `docs/contracts/ot.md`.

## Window

The job states its own window through `runner.Windowed`; `ot` passes the zero `runner.WindowRule{}` (see `docs/contracts/jobs.md` for the job). The scheduler runs it once per `XNYS` session day, after the close.

## Lines it reads

- `OT_PG_DSN_EVENTS` (the earnings store, role `ot_w_events`) and `ALPHA_VANTAGE_API_KEY`: both optional.
- `JQUANTS_API_KEY`: optional, only the `JP` source.
- `$OT_HOME/watch.toml`.

## Left out

- Held, not absent: the job is built and kept for the scheduler's ticker but left out of `ot run-once` and `ot backfill`, with one log line naming the line (`ALPHA_VANTAGE_API_KEY is not set` or `OT_PG_DSN_EVENTS is not set`); `ot run-once` prints `job=earnings_refresh day=<day> outcome=left_out reason=<the line>`.
- `ot scheduler` lists it as disabled with that reason (`cfg.earningsOff()` is the `startOff` of the family and the held reason of its build). Both lines are read once at start, so the job comes back only after a restart.
- An empty `JQUANTS_API_KEY` leaves only the JP source out (log line `JQUANTS_API_KEY is not set`): JP entries get `unsupported` rows with source `none`.

## Notes

J-Quants is fetched at most once per day, one call per JP entry, with no second call and no retry after an `error` (Decision 12 of `docs/design/event-sources.md`).

## Version

v11: introduced; v12: the J-Quants source; moved to its own document with the job registry (no behaviour change).
