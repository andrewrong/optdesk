# ot page `companies_page`

Order: 50

Route: `companies`. Name for `ot page`: `ot page --name companies_page`. Family file: `cmd/ot/companies_site.go`; registered with `registerPage` in an `init()` (`cmd/ot/pagereg.go`). The common rules of every page are in `docs/contracts/ot.md`.

## Window

The page is built per request (`ot serve`) or once per command (`ot page`) for the day of `--day` or else today in the market zone, through `companies_page.Build` (`internal/pages/companies_page`), with the zone of the profile.

## Lines it reads

- `$OT_HOME/watch.toml`, through `loadWatch`: one row per entry (issuer, market, contract symbol); `HandPrice` is the entry's `buy_price` text, empty when none. A broken file is a build error, as in `ot run-once`; a missing file gives no rows.
- `OT_PG_DSN_PAGES`: the reading view (`rd.reading_r1`), the view reader of the site; `ot page` opens it once, a missing line gives no reader.

## Left out

`Series`, `Spot`, `Offers`, `Facts`, the compute functions, `Held`, `Text` and `Override` are not set, so every block but the price line is a gap and no row has a band: the price line cell shows the hand price (`hand_price`) beside the gap text `no band: reader not set`. Nothing is computed in `cmd/ot`.

## Contract

- `ot serve` serves the route `/companies`: GET and HEAD only, `/companies/x` is 404, another method 405, a build error 500 with the fixed text `internal error`.
- `ot page --name companies_page [--day DAY]` builds the same page; the demo profile reads two fixed demo subjects, each with a hand price, and opens no store.

## Version

v1: introduced with this document (order 50).
