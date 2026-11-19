# ot job `recon_bars`

Order: 230

Family file: `cmd/ot/wire_reconbars.go`; registered with `registerJob` in an `init()` (`cmd/ot/jobreg.go`). The bar source comes from the registry of `cmd/ot/wire_barsource.go` by the manifest's `[source] id` (`yfinance`). The common rules of every job are in `docs/contracts/ot.md`.

## Window

The job states its own schedule in `internal/jobs/reconbars/job.toml` (once per XNYS session day, after `bars_refresh`); `ot` passes the zero `runner.WindowRule{}`. It compares our bars with the second source and stores one reconciliation run (pair R6). The job console lists it as an any-day row with force allowed.

## Lines it reads

- `OT_PG_DSN_RECON`: the recon writer (`reconpg.OpenWriter`, role `ot_w_recon`); read only through `loadConfig`.
- The bar store of `bars_refresh` (`OT_PG_DSN_BAR`) and the instrument store, as `bars_refresh` gets them; the bars names of the stored list.
- The command `<uv> run --locked --project <dir> python -m optdesk.refbars`, with the uv program and the project directory of `calendar_refresh` (`OT_CALENDAR_UV`, `OT_CALENDAR_PROJECT`).

## Left out

- While `OT_PG_DSN_RECON` is empty the job is held with the reason `OT_PG_DSN_RECON is not set (run setup pg)` (the `startOff` of the family): no job is built, no writer is opened and the scheduler does not loop on an error, so the code may deploy before `setup pg`.
- With an empty `OT_PG_DSN_BAR` the job is built and held with the reason of `bars_refresh`.
- An id in `[source]` that the registry lacks fails the build with an error that names the id and lists the known ids.

## Version

v1: introduced with issue 3101 (docs/design/base-data-page.md#23); the family is registered at order 230.
