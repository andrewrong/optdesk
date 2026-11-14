# ot job `macro_events`

Order: 220

Family file: `cmd/ot/wire_macro_events.go`; registered with `registerJob` in an `init()` (`cmd/ot/jobreg.go`). The common rules of every job are in `docs/contracts/ot.md`. Design: `docs/design/macro-events.md` (Decision 11).

## Window

The job takes the run day it is given; it has no window of its own, and `ot` passes the zero `runner.WindowRule{}`. The console row is any-day with Force.

## Lines it reads

- `FRED_API_KEY`: the key of the FRED release dates, the same value `fred_series` uses (`fredBaseURL` is the same host).
- `OT_PG_DSN_SERIES_FRED`: the series store, where the writer on `obs.macro_event` opens; it is closed through the closer set of the run. Without it the job is left out and one line says so.
- The macro page reads the stored dates through a reader on `OT_PG_DSN_PAGES` (`cmd/ot/macro_events_page.go`); an empty line or the demo profile leaves the page as it was.

## Left out

- An empty `FRED_API_KEY` leaves it out of `ot scheduler` (reason `FRED_API_KEY is not set`, the `startOff` of the family); the job is still built, held, so that the ticker knows its schedule.
- A deps without the macro-events ext deps leaves it out (tests).

## Notes

Order 210 is `book_risk`'s, so this is the next free multiple of ten. Its deps are ext deps (`extMacroEvents`); `deps` and `wire.go` are not edited.

## Version

v1: wired by `ot run-once`, `ot backfill` and `ot scheduler`, and by the macro page through `newMacroPageEvents`.
