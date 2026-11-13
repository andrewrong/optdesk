# ot job `compute_figures`

Order: 70

Family file: `cmd/ot/wire_computefigures.go`; registered with `registerJob` in an `init()` (`cmd/ot/jobreg.go`). The common rules of every job are in `docs/contracts/ot.md`.

## Window

The job states its own window through `runner.Windowed`; `ot` passes the zero `runner.WindowRule{}` (see `docs/contracts/jobs.md` for the job). `compute_figures` is never left out.

## Lines it reads

- `OT_PG_DSN_PAGES`: the catalog, option_eod, terms, names and series stores (and the bar reader of the HV step).
- `OT_PG_DSN_READINGS`: the readings store.
- The session days of `OT_PG_DSN_RUNNER`.

## Left out

- Never left out. Without `OT_PG_DSN_BAR` only the HV step is off (log line `HV step left out`).
- The tech step (`market.tech_state|market|NDX_SPX`) is wired with `computefigures.WithTech` when both readers exist, the bar reader (`OT_PG_DSN_BAR`) and the index-id reader (`OT_PG_DSN_PAGES`); with either missing the step is left out and the job is as before.

## Notes

It stores the readings of the figures, among them `macro.ease|market|WALCL` (v8).

## Version

v5: wired as the fourth job; v8: the ease reading; v21: the HV step reader; moved to its own document with the job registry (no behaviour change).
