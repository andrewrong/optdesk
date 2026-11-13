# ot job `calendar_hours_jp`

Order: 86

Family file: `cmd/ot/wire_calendar.go`; registered with `registerJob` in an `init()` (`cmd/ot/jobreg.go`). The common rules of every job are in `docs/contracts/ot.md`.

## Window

The job states its own window through `runner.Windowed` (`internal/jobs/calendarhours`, market `XTKS`, the Tokyo session); `ot` passes the zero `runner.WindowRule{}`. It compares the liquid hours IBKR reports with the session days of the loader store and replaces the notice rows of the market. The job console lists it as a today-only row without force.

## Lines it reads

- `OT_PG_DSN_MASTER`: the notice store (`hoursnoticepg`) it writes; the two calendar_hours jobs share one open store.
- `OT_PG_DSN_LOADER`: the session-day store it reads as the session source.
- The broker on the default socket, as `earnings_refresh` gets it (gateway 4001); the account resolver is not used.

## Left out

- With an empty `OT_PG_DSN_MASTER`, or when the open of the notice store fails, the job is left out: no job and no error, and one Warn line that names the job id and the line `OT_PG_DSN_MASTER` and holds no DSN.
- It starts off with the broker down (`startOff` returns `brokerDownOff` for a set line, "" for an empty one), like the other broker jobs.

## Version

v1: introduced with issue 2208; the family is registered at order 86.
