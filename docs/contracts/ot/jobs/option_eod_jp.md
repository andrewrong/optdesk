# ot job `option_eod_jp`

Order: 30

Family file: `cmd/ot/wire_optioneod.go (the same file as `option_eod`)`; registered with `registerJob` in an `init()` (`cmd/ot/jobreg.go`). The common rules of every job are in `docs/contracts/ot.md`.

## Window

The job states its own window through `runner.Windowed`; `ot` passes the zero `runner.WindowRule{}` (see `docs/contracts/jobs.md` for the job). It runs after the Tokyo close (see `docs/contracts/jobs.md`).

## Lines it reads

- The same stores, watch list and broker as `option_eod`; each JP stock entry of `watch.toml` adds one key after `N225` (`buildOptionEodJP`, `optioneod.NewJPWithWatch`).
- The `XTKS` session day of today from the session-day store.

## Left out

- Wired only while the `XTKS` calendar answers for today; otherwise one log line `calendar XTKS is not loaded` (no path) and the job is absent; the other jobs run as before.
- Broker not reachable at start: as `option_eod` (`startOff` of the family).
- The probe checks the XTKS session for today before the broker and returns `calendar XTKS is not loaded` while it fails; each rebuild of the reconciler re-probes an enabled `option_eod_jp` and builds without it for that reason (the owner loads the calendar and restarts).

## Notes

It makes no second call to a vendor on the same day: the job reads the rows of the day and writes none for an entry that has one (Decision 12 of `docs/design/event-sources.md`).

## Version

v12: introduced after `option_eod`; v23: built over the watch list; moved to its own document with the job registry (no behaviour change).
