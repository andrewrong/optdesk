# ot page `option_candidates_page`

Order: 10

Route: `options/candidates`. Name for `ot page`: `ot page --name option_candidates_page`. Family file: `cmd/ot/candidates_site.go`; registered with `registerPage` in an `init()` (`cmd/ot/pagereg.go`). The common rules of every page are in `docs/contracts/ot.md`.

## Window

The page is built per request (`ot serve`) or once per command (`ot page`) for the day of `--day` or else today.

## Lines it reads

The stores and `.env` lines are given in the paragraphs below; the page opens nothing itself, the readers of the site do.

## What is shown

The company name of a row is the ticker, read from the instrument-name reader (`store.InstrumentNameReader`) by the underlying id; a row whose underlying has no name keeps the stored issuer text. `ot serve` passes `rd.names`; `ot page` opens the instrument-name store with `OT_PG_DSN_PAGES` (closed with the page; a missing line gives a nil reader, the demo profile uses the demo names).

## Evidence readers

`ot serve` passes the readers of the site in `candidateReaders` (`cmd/ot/evidence_site.go`): the candidate reader `rd.cands`, the name reader `rd.names`, the reading reader `rd.view`, the option terms reader `rd.terms` and the option day reader `rd.days`. The route is the live path of the evidence cells. `ot page --name option_candidates_page` passes only the candidate and name readers, so there the evidence cells are gaps `no_reader`; the demo profile builds with the evaluator and no day reader, so its evidence cells are gaps `no_reader` or figures of the demo rows.

The ATM row of the skew is chosen by `newATMFor` (`cmd/ot/evidence_site.go`): one `ChainDay` per underlying and day, `OptionTerms` in chunks of at most 500 ids, one `ChainQuote` per chain row that has terms, and the picker `optionevidence.NewATMPicker()` called with the Source `option_eod`, the capture of the candidates' own quote. A nil day reader, terms reader or picker gives no function, and the page shows the gap `no_reader`. `Evidence` is `optionevidence.New` over the `bsm` pricer with `DefaultParams()`.

## Tier view readers

`ot serve` also passes the readers of the tier view in `candidateReaders`: `Tiers` from `newTierer()` (`cmd/ot/tiers_site.go`, `optiontier.New` over `newEvaluator()`, returned as `optiontierapi.Tierer`), `Account` from `rd.account`, `Aliases` from `rd.aliasList()`, `Owner` from `rd.owner` and `RuleParams` from `deps.newRuleParams()`; they fill the same fields of `option_candidates_page.Config`. The demo profile passes none of them. `ot page --name option_candidates_page` passes none of the five, opens no store beyond the candidate and name stores, and the field `short_put_tiers` is `null`. The new file is `cmd/ot/tiers_site.go`.

## Valuation line readers

`ot serve` passes the valuation reader of the site, `valuationReaderOf(rd)` (nil without the pages line), in `candidateReaders.Valuation`; `candidatesConfig` sets `Valuation` from it and `ValueLines` from `newValueLines()` (`cmd/ot/valline_site.go`, `valuelines.New()` returned as `valuelinesapi.Calculator`), so the calculator is set on every path. The demo profile builds a nil `Valuation`. `ot page --name option_candidates_page` passes no valuation reader (nil `Valuation`), opens no store for it, and the `valuation` block of the company strips is `null` there.

## Left out

With no reader for a store the page shows its own gap (`no_reader`); a nil zone is an error. The demo profile opens no store.

## Contract

- `ot serve` serves the route `/options/candidates` (`cmd/ot/candidates_site.go`): GET and HEAD only, `/options/candidates/x` is 404, another method 405, a build error 500 with the fixed text `internal error`. It builds `option_candidates_page` per request for the day of `--day` or else today through `deps.optionCandidatesPage` (run id `ot-serve`, scope `candidates`) from the candidate store opened once per process with `OT_PG_DSN_PAGES` (role `ot_ro_pages`, through `candidate.Open`; closed on exit, so `ot serve` opens the stores of the pages line); the lookback over earlier days is the builder's. The demo profile reads fixed demo rows (two ok and one blocked row of each of the three kinds) and opens no store. The ninth item of the bar that links to it belongs to the web build, not to `cmd/ot`.

- `ot page --name option_candidates_page [--day DAY]` builds the same page through `deps.optionCandidatesPage` (run id `ot-page`, scope `candidates`) in the zone of the profile (a nil zone is an error before any store is opened) from the candidate store opened once with `OT_PG_DSN_PAGES`; a missing line gives no reader, so the page shows its gap `no_reader`; the demo profile reads the demo rows.

## Version

v11: introduced in `docs/contracts/ot.md` (see its version history); moved to this document with the page registry (no behaviour change).
