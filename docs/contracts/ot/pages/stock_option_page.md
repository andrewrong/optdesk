# ot page `stock_option_page`

Order: none (no name in the known-pages text)

Route: `options/stock` and `options/stock/<key>`. No name: `ot page --name stock_option_page` is an unknown page, because the page needs a key. Family file: `cmd/ot/stock_option_site.go`; registered with `registerPage` in an `init()` through the field `buildKey` of `pageFamily` (`cmd/ot/pagereg.go`). The common rules of every page are in `docs/contracts/ot.md`.

## Window

The page is built per request (`ot serve`) for the day of `--day` or else today.

## Lines it reads

The page opens nothing itself; it reads the candidate, option_eod and instrument-name readers of the site, and `vol_view` through `rd.view` (the reading reader the site already holds; nothing is opened anew). A nil `rd.view` gives every row of `vol_view` a gap.

## What is shown

The company name is the ticker, read from the instrument-name reader; the key of the page is the ticker (`/options/stock/AAPL`), and a stored issuer text still matches as a fallback. The page shows the chain day: the newest day on or before the data day that holds option_eod rows, up to 14 days back, and its note names it.

## Left out

With no candidate reader the page shows its own gap (`no_reader`); a nil zone is an error. There is no `ot page` form.

## Contract

- `ot serve` serves `/options/stock` and `/options/stock/<key>` (`cmd/ot/stock_option_site.go`): GET and HEAD only, another method 405, a build error 500 with the fixed text `internal error`. The key is the issuer text of a stored candidate row and must match `^[A-Za-z0-9_-]{1,64}$`; another key, an empty key (`/options/stock/`) or a further slash is 404. The bare `/options/stock` shows the issuer of the first ok row of the data day, else of the first row. A key with no stored row gives an empty page with the gap text of the builder, status 200. The page is built through `option_boards.NewStockPage` (run id `ot-serve`, scope `stock`) from `rd.cands`, `rd.eod`, `rd.names` and `rd.view`; see `docs/design/option-page-builders.md` Decisions 8 and 10.

## Version

v1: introduced with the keyed page family (`docs/design/option-page-builders.md`, entry 6).

v3: schema `page.stock_option_page.v3`; `vol_view` is built from the stored readings read through `rd.view` (`docs/design/vol-edge.md`, entry 5, Decisions 5, 7 and 9).
