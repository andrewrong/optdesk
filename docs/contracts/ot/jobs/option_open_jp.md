# ot job `option_open_jp`

Order: 180

Family file: `cmd/ot/wire_optionopen.go`; registered with `registerJob` in an `init()` (`cmd/ot/jobreg.go`). The common rules of every job are in `docs/contracts/ot.md`. Design: `docs/design/option-open-pull.md#Decisions` (Decision 21).

## Window

`session_open` plus 5 minutes on `XTKS` (09:05 JST); the lunch break 11:30 to 12:30 JST asks nothing. The job states its own window and slot through its manifest in `internal/jobs/optionopen`; `ot` passes the zero `runner.WindowRule{}`.

## Lines it reads

- `OT_PG_DSN_OPTION_OPEN` (env file): the store of `optionopenpg`, opened by the family and closed through the closer set.
- `OT_PG_DSN_MASTER`: the instrument store.
- `$OT_HOME/watch.toml`: the watch list.
- The broker socket `$OT_HOME/run/ibkr.sock`.

## Left out

- `OT_PG_DSN_OPTION_OPEN` missing (or the instrument or bar store missing): the job is left out with one startup line that names the job and the line; the other jobs run.
- The store cannot be opened: the job is held with that reason; the scheduler lists it as disabled and `run-once` leaves it out.
- A failed broker dial at start (`broker is not reachable`, the `startOff` of the family, set only when `OT_PG_DSN_OPTION_OPEN` is set): the job holds a stand-in broker and `ot scheduler` lists it as disabled.

## Version

v1: wired by `ot run-once`, `ot scheduler` (issue #1516).
