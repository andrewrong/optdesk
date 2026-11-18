# ot page `base_data_page`

Order: 135

Route: `base-data`. Name for `ot page`: `ot page --name base_data_page`. Family file: `cmd/ot/base_data_site.go`; registered with `registerPage` in an `init()` (`cmd/ot/pagereg.go`). The common rules of every page are in `docs/contracts/ot.md`. Design: `docs/design/base-data-page.md`, entry 10.

## Route

`ot serve` serves `/base-data`: GET and HEAD only, another method 405, a build error 500 with the fixed text `internal error`. The page is `page.base_data_page.v1`, built per request for the clock through `base_data_page.Build` (`internal/pages/base_data_page`) over the fact reader `basefact.Reader` (`basefactpg.Open`) and the coverage reader `coverage.Reader`, each opened once with the line `OT_PG_DSN_PAGES` through `siteOpeners` (openers in the ext deps under `base_data`, with `basecheck.Evaluate` as the injected verdict function). The wiring holds no logic.

## Gaps

A missing line `OT_PG_DSN_PAGES`, the demo profile, a failed open or a failed read never fails the page: the route answers 200 with a document that validates and one or more gaps in words:

- `facts_unavailable`: no fact reader or all four views failed; no row is judged.
- `view_unavailable`: one or more views failed; their rows are `unknown` and the other rows stay.
- `coverage_unavailable`: the coverage could not be read; the ledger rules are not judged.

## `/data-health`

The `/data-health` route (order 130) stays and gets no redirect: it keeps the repair button and the coverage grid of every dataset. The new page names it as the place of the repair (`repair_path`).

## Version

v1: introduced with this document (order 135).
