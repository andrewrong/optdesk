# ot job `dividend_capture`

Order: 40

Family file: `cmd/ot/wire_dividend.go`; registered with `registerJob` in an `init()` (`cmd/ot/jobreg.go`). The common rules of every job are in `docs/contracts/ot.md`.

## Window

The job states its own window through `runner.Windowed`; `ot` passes the zero `runner.WindowRule{}` (see `docs/contracts/jobs.md` for the job).

## Lines it reads

- `OT_PG_DSN_EVENTS`: the dividend store (`events.OpenDividend`, role `ot_w_events`).
- `OT_PG_DSN_MASTER` through the instrument store opened for `option_eod`.
- `$OT_HOME/watch.toml`.
- A reference client dialled at `$OT_HOME/run/ibkr.sock` (`deps.newReference`).

## Left out

- Absent for a deps without `newDividendCapture`, `openDividend` or `newReference`.
- Broker down at start: one log line `job left out` naming `dividend_capture` and `broker is not reachable`.
- `OT_PG_DSN_EVENTS` empty: the same line with `OT_PG_DSN_EVENTS is not set`.
- No reference client is dialled in either case and the other jobs run as before. It is never probed back in (a family without a probe case stays out until a restart).

## Notes

It follows `option_eod` and `option_eod_jp`. What it opens goes to the closer set that `withWiring` closes.

On the job console the row is an any-day row with Force. A forced run is one more Plan and so one more fetch from the reference source.

## Version

v20: wired after `option_eod`; moved to its own document with the job registry (no behaviour change).
