# ot page `company_data_page`

Order: 190

Route: `company-data`. Name for `ot page`: `ot page --name company_data_page`. Family file: `cmd/ot/company_data_site.go`; registered with `registerPage` in an `init()` (`cmd/ot/pagereg.go`). The common rules of every page are in `docs/contracts/ot.md`. Design: `docs/design/data-collection.md`, entry 8 (Decision 8).

## Route

`ot serve` serves `/company-data`: GET and HEAD only, another method 405, a path below the route 404, a build error 500 with the fixed text `internal error`. The page is built per request for the day of the clock through `company_data_page.Build` (`internal/pages/company_data_page`); `ot page --name company_data_page` builds the same document.

## Readers

- The underlying reader `rd.underlying`, the bar reader `rd.bars` and the earnings reader `rd.earnings` of the site readers.
- The fact reader of `siteFactsKey`, opened once through `siteOpeners` by `fundamentalsOf(a.deps).openFacts` with the pages line `OT_PG_DSN_PAGES` (role `ot_ro_pages`), asserted to `companydataapi.PeriodFactReader`.
- The calculator is `companydata.Calc` of `internal/compute/companydata`; the site file does no arithmetic.

Every reader is opened with the page DSN; none with a write role. With the demo profile `ot page` opens no store and the page lists no company.

## Left out

Left out: no repair button, no form. The page only reads; its data-health link is a plain path.

## Version

v1: introduced with this document (order 190).
