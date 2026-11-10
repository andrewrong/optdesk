# ot page `option_tracking_page`

Order: 30

Route: `options/tracking`. Name for `ot page`: `ot page --name option_tracking_page`. Family file: `cmd/ot/tracking_site.go`; registered with `registerPage` in an `init()` (`cmd/ot/pagereg.go`). The common rules of every page are in `docs/contracts/ot.md`.

## Window

The page is built per request (`ot serve`) or once per command (`ot page`) for the day of `--day` or else today.

## Lines it reads

The stores and `.env` lines are given in the paragraphs below; the page opens nothing itself, the readers of the site do.

## Left out

With no reader for a store the page shows its own gap (`no_reader`); a nil zone is an error. The demo profile opens no store.

## Contract

- `ot page --name option_tracking_page [--day DAY]` builds the same page through `deps.optionTrackingPage` (run id `ot-page`, scope `tracking`) in the zone of the profile (a nil zone is an error before any store is opened) from the stores opened once with `OT_PG_DSN_PAGES`; a missing line gives no reader, so the page shows its gaps; the demo profile reads the demo ledger and account.

- `ot serve` serves the route `/options/tracking` (`cmd/ot/tracking_site.go`): GET and HEAD only, `/options/tracking/x` is 404, another method 405, a build error 500 with the fixed text `internal error`. It builds `option_tracking_page` per request for the day of `--day` or else today in the zone of the profile through `deps.optionTrackingPage` (run id `ot-serve`, scope `tracking`; `compute/optiontrack` and `compute/optionledger` and the parameters are wired in `cmd/ot/wire.go`) from the readers `ot serve` already holds plus the option ledger store opened once with `OT_PG_DSN_PAGES` (role `ot_ro_pages`, the views `rd.flex_trade_row_r1` and `rd.flex_fetch_r1`, through `deps.openOptionLedgerRO`; `OT_PG_DSN_OPTION_LEDGER` is not read). The page reads the session days through `rd.session_day_r1` (the session-day store opened once with `OT_PG_DSN_PAGES`, through `deps.openSessionDay`); the demo profile opens no store and keeps the gap `no_sessions`; until migration 0028 is applied the page answers 500 `internal error`. A missing reader is the page's own gap, never a failure. The demo profile reads a fixed demo ledger (two open short options and one recent assignment) and a fixed demo account (one position) and opens no store. The bar item that links to it belongs to the web build, not to `cmd/ot`.

## Version

v18: introduced in `docs/contracts/ot.md` (see its version history); moved to this document with the page registry (no behaviour change).
