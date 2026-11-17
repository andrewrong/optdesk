# ot page `option_pnl_page`

Order: 35

Route: `options/pnl`. Name for `ot page`: `ot page --name option_pnl_page`. Family file: `cmd/ot/pnl_site.go`; registered with `registerPage` in an `init()` (`cmd/ot/pagereg.go`). The common rules of every page are in `docs/contracts/ot.md`.

## Window

The page is built per request (`ot serve`) or once per command (`ot page`) for the day of `--day` or else today.

## Lines it reads

The stores and `.env` lines are those of the readers of the site (`OT_PG_DSN_PAGES`): the option ledger, the session days, the option day rows, the daily bars, the stock id reader of `siteMarketStateIDsKey` and the series reader (the rates of `DGS3MO`). The page opens nothing itself.

## Left out

With no reader for a store the page shows its own gap; a nil zone is an error. The setting `max_rate_age_days` is not read: `cmd/ot` passes the constant `pnlMaxRateAgeDays` (7), its default.

## Contract

- `ot page --name option_pnl_page [--day DAY]` builds the page through `deps.optionPnlPage` (run id `ot-page`, scope `pnl`) in the zone of the profile (a nil zone is an error before any store is opened) from the stores opened once with `OT_PG_DSN_PAGES`; a missing line gives no reader, so the page shows its gaps. The `Build` of the page gets the series reader of the stores.
- `ot serve` serves `/options/pnl` (`cmd/ot/pnl_site.go`): GET and HEAD only, a build error is 500 `internal error`. The page is built per request through `deps.optionPnlPage` (run id `ot-serve`, scope `pnl`). `pnlConfigOf` passes `optionattr.New(bsm.New())` as attributor, `optionattr.NewBooker()` as booker, `optionledger.New()` as replay, mark age 3 sessions, 30 recent days, the rate age 7 days and the calendar `XNYS`. The ledger is wrapped in `aliasedLedger`, as for the tracking page.

## Version

Introduced with `docs/design/options-pnl-page.md` (entry 5).
