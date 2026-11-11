# ot job `underlying_resolve`

Order: 200

Family file: `cmd/ot/wire_underlying.go`; registered with `registerJob` in an `init()` (`cmd/ot/jobreg.go`). The common rules of every job are in `docs/contracts/ot.md`.

## Window

The job states its own window through `runner.Windowed` (`internal/jobs/underlyingresolve`); `ot` passes the zero `runner.WindowRule{}`. The scheduler runs it in the evening; it settles the pending names of the stored underlying list through the broker and writes the result back with `underlying.Settler`.

## Lines it reads

- `OT_PG_DSN_UNDERLYING_RESOLVE` (`underlying.DSNEnvResolve`): the store the job settles through. It is the only line this job reads; it does not use `OT_PG_DSN_RUNNER`.
- The broker, as every broker job.

## Left out

- Absent for a deps without `openUnderlying`.
- Held: with an empty `OT_PG_DSN_UNDERLYING_RESOLVE` the job is left out, and only this job; the other families keep running. The reason `OT_PG_DSN_UNDERLYING_RESOLVE is not set` is logged once.
- It starts off with the broker down (`startOff: brokerDownOff`), like the other broker jobs.

## Notes

The other families do not read this line: each of them reads the stored list through `OT_PG_DSN_RUNNER` at every Plan (`listedJob`, `cmd/ot/wire_underlying.go`), and an empty list fails that Plan with `underlying list is empty`.

## Version

v1: introduced with issue 1708; the family is registered and settles the pending names of the stored list.
