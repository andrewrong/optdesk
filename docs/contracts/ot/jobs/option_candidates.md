# ot job `option_candidates`

Order: 110

Family file: `cmd/ot/candidates.go`; registered with `registerJob` in an `init()` (`cmd/ot/jobreg.go`). The common rules of every job are in `docs/contracts/ot.md`.

## Window

The job states its own window through `runner.Windowed`; `ot` passes the zero `runner.WindowRule{}` (see `docs/contracts/jobs.md` for the job). The scheduler runs it at `session_close` plus 30 minutes; it puts only.

## Lines it reads

- `OT_PG_DSN_CANDIDATE`: the candidate store (role `ot_w_candidate`), optional.
- The compute stores, the events reader (`OT_PG_DSN_EVENTS`) and the account reader (`OT_PG_DSN_ACCOUNT`) when set, and `$OT_HOME/watch.toml`.

## Left out

- Absent for a deps without `newOptionCandidates`.
- Held: with an empty `OT_PG_DSN_CANDIDATE` the job is built over a stand-in writer, left out of `ot run-once` and `ot backfill` with the named line `OT_PG_DSN_CANDIDATE is not set` and listed as disabled by `ot scheduler` with that reason. `candidatesOff()` is the one source of the reason, so the family gives no `startOff`; the probe keeps it out until a restart.

## Notes

It is the last family. `ot candidates` reads the rows through `OT_PG_DSN_PAGES` and does not read `OT_PG_DSN_CANDIDATE`.

## Version

v16: introduced; moved to its own document with the job registry (no behaviour change).
