# ot page `option_notices_page`

Order: 20

Route: `options/notices`. Name for `ot page`: `ot page --name option_notices_page`. Family file: `cmd/ot/notices_site.go`; registered with `registerPage` in an `init()` (`cmd/ot/pagereg.go`). The common rules of every page are in `docs/contracts/ot.md`.

## Window

The page is built per request (`ot serve`) or once per command (`ot page`) for the day of `--day` or else today.

## Lines it reads

The stores and `.env` lines are given in the paragraphs below; the page opens nothing itself, the readers of the site do.

## Left out

With no reader for a store the page shows its own gap (`no_reader`); a nil zone is an error. The demo profile opens no store.

## Contract

- `ot serve` serves the route `/options/notices` (`cmd/ot/notices_site.go`): GET and HEAD only, `/options/notices/x` is 404, another method 405, a build error 500 with the fixed text `internal error`. It builds `option_notices_page` per request for the day of `--day` or else today through `deps.optionNoticesPage` (run id `ot-serve`, scope `notices`, `Params` from `deps.noticeParams` = `compute.DefaultNoticeParamsFunc`, the rule from `deps.newOptionNotices`) from the account, candidate and earnings stores opened once per process with `OT_PG_DSN_PAGES` (the lines `OT_PG_DSN_ACCOUNT`, `OT_PG_DSN_CANDIDATE` and `OT_PG_DSN_EVENTS` are not read; closed on exit, so `ot serve` opens ten stores). `Config.Main` is the main label of the gateways file `OT_IBKR_GATEWAYS` and `Config.Aliases` the labels of `OT_ACCOUNT_SOCKETS` in sorted order, read as the account job reads them (an account id is not passed); a profile without them makes the build fail (500). The demo profile serves a fixed result (one notice of each severity for the alias `main`, a second alias `second` with `no_capture`) and opens no store. The link in the bar belongs to the web build, not to `cmd/ot`.

- `ot page --name option_notices_page [--day DAY]` builds the same page through `deps.optionNoticesPage` (run id `ot-page`, scope `notices`) in the zone of the profile (a nil zone is an error before any store is opened) from the three stores opened once with `OT_PG_DSN_PAGES`; a missing line gives no reader, so the page shows its gap `no_reader`; the demo profile serves the fixed result.

## Version

v15: introduced in `docs/contracts/ot.md` (see its version history); moved to this document with the page registry (no behaviour change).
