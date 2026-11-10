# ot job `calendar_refresh`

Order: 80

Family file: `cmd/ot/wire_calendar.go`; registered with `registerJob` in an `init()` (`cmd/ot/jobreg.go`). The common rules of every job are in `docs/contracts/ot.md`.

## Window

The job states its own window through `runner.Windowed`; `ot` passes the zero `runner.WindowRule{}` (see `docs/contracts/jobs.md` for the job). `calendar_refresh` is never left out.

## Lines it reads

- `OT_PG_DSN_LOADER`: the only line of the store it opens (role `ot_loader`).
- `OT_CALENDAR_UV` (optional, default `uv`): it runs `uv run --no-sync --project pyside python -m optdesk.calendar` through `calendarcli.Exec`.

## Left out

- Never left out.

## Notes

Each changed day is logged as `calendar change` (`status` carries day, before and after).

## Version

v6: introduced as the fifth job; moved to its own document with the job registry (no behaviour change).
