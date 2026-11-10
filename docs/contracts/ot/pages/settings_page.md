# ot page `settings_page`

Order: none (no name in the known-pages text)

Route: `settings`. Name for `ot page`: none: the page is a route only, `ot page --name settings_page` is an unknown name and `settings_page` is not in the list of known pages. Family file: `cmd/ot/settings.go`; registered with `registerPage` in an `init()` (`cmd/ot/pagereg.go`). The common rules of every page are in `docs/contracts/ot.md`.

## Window

The page is built per request (`ot serve`) or once per command (`ot page`) for the day of `--day` or else today.

## Lines it reads

The stores and `.env` lines are given in the paragraphs below; the page opens nothing itself, the readers of the site do.

## Left out

With no reader for a store the page shows its own gap (`no_reader`); a nil zone is an error. The demo profile opens no store.

## Contract

- `ot serve` serves the route `settings` (`cmd/ot/settings.go`): GET and HEAD only, `settings/x` is 404, another method 405, a build error 500 with the fixed text `internal error`. It builds `settings_page` per request through `deps.settingsPage` over the catalog and name readers of the site, the registry of `deps.newRuleParams` and an owner reader opened once per process with `deps.openOwnerSet` (`ownersetpg.Open`) and `OT_PG_DSN_PAGES` (no new `.env` line; closed on exit). Without `--owner-edit` the page is read-only: `Form` and `Confirm` are nil, so `form` is null and its gap text says editing is off; with `--owner-edit` (and both `.env` lines) it carries the token of the form and the result line of `?saved=<n>`, and `POST /owner-settings` is served as the `ot serve` paragraph above describes; history reads 20 rows, run id `ot-serve`, scope `settings`. With no opener (the demo profile) the reader is nil and the page shows the gap `no_reader`, status 200. `deps` gains `newRuleParams`, `openOwnerSet`, `settingsPage` and `newSettingsForm`.

## Version

v1: moved to this document with the page registry from `docs/contracts/ot.md` (no behaviour change); the route was introduced before the version list.
