# ot page `option_candidates_page`

Order: 10

Route: `options/candidates`. Name for `ot page`: `ot page --name option_candidates_page`. Family file: `cmd/ot/candidates_site.go`; registered with `registerPage` in an `init()` (`cmd/ot/pagereg.go`). The common rules of every page are in `docs/contracts/ot.md`.

## Window

The page is built per request (`ot serve`) or once per command (`ot page`) for the day of `--day` or else today.

## Lines it reads

The stores and `.env` lines are given in the paragraphs below; the page opens nothing itself, the readers of the site do.

## What is shown

The company name of a row is the ticker, read from the instrument-name reader (`store.InstrumentNameReader`) by the underlying id; a row whose underlying has no name keeps the stored issuer text. `ot serve` passes `rd.names`; `ot page` opens the instrument-name store with `OT_PG_DSN_PAGES` (closed with the page; a missing line gives a nil reader, the demo profile uses the demo names).

## Left out

With no reader for a store the page shows its own gap (`no_reader`); a nil zone is an error. The demo profile opens no store.

## Contract

- `ot serve` serves the route `/options/candidates` (`cmd/ot/candidates_site.go`): GET and HEAD only, `/options/candidates/x` is 404, another method 405, a build error 500 with the fixed text `internal error`. It builds `option_candidates_page` per request for the day of `--day` or else today through `deps.optionCandidatesPage` (run id `ot-serve`, scope `candidates`) from the candidate store opened once per process with `OT_PG_DSN_PAGES` (role `ot_ro_pages`, through `candidate.Open`; closed on exit, so `ot serve` opens the stores of the pages line); the lookback over earlier days is the builder's. The demo profile reads fixed demo rows (two ok and one blocked row of each of the three kinds) and opens no store. The ninth item of the bar that links to it belongs to the web build, not to `cmd/ot`.

- `ot page --name option_candidates_page [--day DAY]` builds the same page through `deps.optionCandidatesPage` (run id `ot-page`, scope `candidates`) in the zone of the profile (a nil zone is an error before any store is opened) from the candidate store opened once with `OT_PG_DSN_PAGES`; a missing line gives no reader, so the page shows its gap `no_reader`; the demo profile reads the demo rows.

## Version

v11: introduced in `docs/contracts/ot.md` (see its version history); moved to this document with the page registry (no behaviour change).
