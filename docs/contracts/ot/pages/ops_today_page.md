# ot page `ops_today_page`

Order: 110

Route: `options/today`. Name for `ot page`: `ot page --name ops_today_page`. Family file: `cmd/ot/ops_today_site.go`; registered with `registerPage` in an `init()` (`cmd/ot/pagereg.go`). The common rules of every page are in `docs/contracts/ot.md`. Design: `docs/design/option-open-gap-jp.md#Same-morning-visibility`.

## Window

Built per request (`ot serve`) or once per command (`ot page`) for the day of `--day` or else today.

## Lines it reads

`OT_PG_DSN_PAGES`: the run store (`ops.OpenRuns`, through `deps.openRuns`) is opened once per process by the opener of the page in `siteOpeners` and handed to the page as a `runview.Reader`, so the page cannot write. The page reads the runs and reasons of the day and the `not_wired` set; it computes nothing.

## Left out

With no opener, no line or the demo profile the page has no reader and shows no row.

## Contract

The rows come from `internal/pages/ops_today_page.Build`. The `not_wired` rows are written by `ot scheduler` at each start (`cmd/ot/not_wired.go`): the set of that start replaces the earlier one, so a row never outlives its fix.

## Version

v1: introduced with issue #1972.
