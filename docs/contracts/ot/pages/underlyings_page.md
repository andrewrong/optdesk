# ot page `underlyings`

Order: none (no name in the known-pages text)

Route: `underlyings`. Name for `ot page`: none: the page is a route only, `ot page --name underlyings` is an unknown name and `underlyings` is not in the list of known pages. Post route: `owner-underlyings` (only with `--owner-edit`). Family files: `cmd/ot/underlyings_site.go` (the page, registered with `registerPage` in an `init()`) and `cmd/ot/underlyings_post.go` (the post). The common rules of every page are in `docs/contracts/ot.md`.

## Window

The page is built per request (`ot serve`) for the day of `--day` or else today. It reads the current rows and the last 20 versions of the underlying store, and the held stock symbols of the account aliases (the newest positions within six days).

## Lines it reads

The page reads through `deps.openUnderlying` (`underlyingpg.Open`) opened once per process with `OT_PG_DSN_PAGES`; no new `.env` line. The post opens a writer per confirmed post through the same opener with the `OT_PG_DSN_SETTINGS` line that `ownerEditSecrets` returns, and closes it.

## Left out

With no reader (no opener, the demo profile, a failed open) the page shows the gap `no_reader` and no rows, status 200; a nil zone is an error. Without `--owner-edit` the page is read-only (`form` null) and `POST /owner-underlyings` is 404. With `--owner-edit` and `OT_PG_DSN_SETTINGS` empty the route is left out with the logged reason of `/owner-settings`, as `/owner-settings` itself is.

## Contract

- `GET` and `HEAD /underlyings`: the page `page.underlyings.v1` built with `underlyings.Build`. With `--owner-edit` it carries the form token (`form`) and the result line of `?saved=<n>` (digits only, as `/settings`); without it `form` is null.
- `POST /owner-underlyings` (`--owner-edit` only; another method 405) has the guards of `/owner-settings`, `guardOwnerPost` called as it is: loopback `Host`, `Origin`, the HMAC token (`expires`, `mac`) and a 64 KiB body. A refused guard is 403 with the text `forbidden`, no page and no write.
- Form fields: a post with `key` edits that row (the fields `status`, `name`, `issuer`, `exchange`, `currency`, `buy_price`, `sale_price`, `report_date_override`, `note` are written when they differ; `options` and `fundamentals` are on when present, so an absent switch is off); a post with no `key` and a `symbol` adds a name (`symbol`, `market`, `kind` and the same fields). `confirm=1` marks the second step.
- Two steps. First post (no `confirm`): nothing is written; the answer is 200 with the page in confirm mode: `confirm.changes` lists old and new, `confirm.stops` the lines `<group>: stopped for <symbol>` of each group that stops for the change (a group a held name keeps is not named) and the line that days missed while a group is off are not refilled. A post that changes nothing answers 200 with the notice `Nothing changed.`. Second post (`confirm=1`): `Apply` is called once and the answer is 303 to `/underlyings?saved=<n>`, `n` the rows written (0 and 303 when nothing changed, with no write).
- Codes: 303 saved; 403 guard refused; 404 without `--owner-edit`; 422 a value the store refuses with an `underlying.InvalidError` (the page with a notice that names the field), a key not on the list, or a save that leaves no active name (field `status`); 500 for any other error, with the fixed text `internal error` and one log line that carries no field value.
- The page has no name for `ot page`.

## Version

v1: the route `underlyings` and the post route `owner-underlyings`.
