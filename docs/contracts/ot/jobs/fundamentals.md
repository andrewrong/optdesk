# ot job `fundamentals`

Order: 160

Family file: `cmd/ot/wire_fundamentals.go`; registered with `registerJob` in an `init()` (`cmd/ot/jobreg.go`). The common rules of every job are in `docs/contracts/ot.md`. The job itself is `internal/jobs/fundamentals` (see `docs/contracts/jobs.md`).

## Key and reading

The job fetches the SEC company facts of the `cik:` entries of Market `US` in `watch.toml` through one `FactSource.Fetch` per run and writes them through one `FactWriter.InsertFacts` call. The source is `sec.New` built by `newSEC` over `https://data.sec.gov`; the EDINET source is nil (JP entries are skipped with the reason `edinet_not_fetched`).

## Lines it reads

- `OT_PG_DSN_FUNDAMENTAL`: the fact store, opened once per run with `store/fundamental.Open` (role `ot_w_fundamental`). `compute_figures` reads the same store for its valuation step.
- `SEC_USER_AGENT`: the user agent of the SEC requests; never printed.
- `$OT_HOME/watch.toml`.

## Left out

- A missing `OT_PG_DSN_FUNDAMENTAL` or `SEC_USER_AGENT` leaves only this job out, with one log line `job left out` naming `fundamentals` and the line (`... is not set`). Without `OT_PG_DSN_FUNDAMENTAL` the valuation step of `compute_figures` is left out too, and `compute_figures` is built as before, with the ease figure.
- Absent for a deps without the `fundamentals` ext deps.

## Notes

With the fact store open, `compute_figures` is built through `computefigures.NewWithValuation` (`ValuationDeps`: the fact store, the watch list as `Entries`, and the compute functions of `internal/compute/valuation`); that constructor has no ease figure, so `ot` joins the job it builds with the ease job `compute_figures` has without the fact store: the plan is the union of the two and a run hands each key to the job that planned it, so the ease reading `macro.ease|market|WALCL` is still stored. Its wiring order is 160, after `compute_figures` (70): the figures job reads the facts the job wrote on an earlier run.

## Version

v1: new family `fundamentals` (valuation, entry 17).
