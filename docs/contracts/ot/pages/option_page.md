# ot page `option_page`

Order: 40

Route: `options`. Name for `ot page`: `ot page --name option_page`. Family file: `cmd/ot/option_page_site.go`; registered with `registerPage` in an `init()` (`cmd/ot/pagereg.go`). The common rules of every page are in `docs/contracts/ot.md`.

## Window

The page is built per request (`ot serve`) or once per command (`ot page`) for the day of `--day` or else today in the zone of the profile.

## Lines it reads

The candidate store and the option_eod store, both opened once with `OT_PG_DSN_PAGES`; the page opens nothing itself.

## Left out

With no reader for a store the page shows its own gap (`no_reader`); a nil zone is an error. The demo profile opens no store.

## Contract

- `ot page --name option_page [--day DAY]` builds the page through `option_boards.NewOptionPage` (run id `ot-page`, scope `options`) in the zone of the profile (a nil zone is an error before any store is opened) from the candidate and option_eod stores opened once with `OT_PG_DSN_PAGES`; a missing line gives nil readers, so the page shows the gap `no_reader`; the demo profile reads the demo candidates and the demo option data and opens no store.

- `ot serve` serves the route `/options` (`cmd/ot/option_page_site.go`): GET and HEAD only, another method 405, a build error 500 with the fixed text `internal error`. It builds `option_page` per request for the day of `--day` or else today in the zone of the profile (run id `ot-serve`, scope `options`) from the candidate and option_eod readers `ot serve` already holds. A missing reader is the page's own gap, never a failure. The route `/options` is no longer a sample route; the other sample routes are unchanged.

## Version

v1: the route `/options` moves from the sample pages to this page (`docs/design/option-page-builders.md`, Decision 10).
