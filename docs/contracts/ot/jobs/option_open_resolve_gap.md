# ot job `option_open_resolve_gap`

Order: 190

Family file: `cmd/ot/wire_optionopen.go`; registered with `registerJob` in an `init()` (`cmd/ot/jobreg.go`). The common rules of every job are in `docs/contracts/ot.md`. Design: `docs/design/option-open-pull.md#Decisions`.

## Window

Pre-open gap pass: `session_open` minus 90 minutes. The job states its own window and slot through its manifest in `internal/jobs/optionopen`; `ot` passes the zero `runner.WindowRule{}`.

## Lines it reads

- `OT_PG_DSN_OPTION_OPEN` (process environment): the store of `optionopenpg`, opened by the family and closed through the closer set.
- `OT_PG_DSN_MASTER`: the instrument store; `OT_PG_DSN_BAR`: the bar store (not for `option_open`).
- `$OT_HOME/watch.toml`: the watch list.
- The broker socket `$OT_HOME/run/ibkr.sock` (a profile value can name `ibkr-main.sock`).
- `OT_OPEN_RESOLVE_MAX_NEW` (process environment): as for `option_open_resolve`. The family dials its own client with one resolve in flight.

## Left out

- `OT_PG_DSN_OPTION_OPEN` missing (or the instrument or bar store missing): the job is left out with one startup line that names the job and the line; the other jobs run.
- The store cannot be opened: the job is held with that reason; the scheduler lists it as disabled and `run-once` leaves it out.
- A failed broker dial at start (`broker is not reachable`, the `startOff` of the family): the job holds a stand-in broker and `ot scheduler` lists it as disabled.

## Version

v1: wired by `ot run-once`, `ot scheduler` (issue #1461).
