# Settings form: owner rule parameters and option exclusions in the store, one POST route on `ot serve`

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-09
Revision: 5
Revision note: revision 5 moves the compute interfaces (`RuleParams`, `SettingsForm`, their constructors and the `Form*` types) out of `internal/compute/api.go` into the new declarations-only package `internal/compute/settingsapi` (owner decision of 2026-11-09, answer "a"; Decision 15): the file is 835 lines on main under a ceiling of 840 in `docs/design/ratchet.json`, which an agent never raises; revisions 2 to 4 below said that no ratchet file existed, and that is stale. Nothing else changes. Revision 4 and earlier: revision 2 answers the adversarial read at `adv-settings-form.md` (section `## Adversarial`): a confirm step and a history list (Decision 10), realistic registry bounds (Decision 3), the Host and Origin check by loopback names (Decision 5), the 800-line text of AGENTS.md instead of a ratchet file, the grants for `ot_runner` and the one consumer on main (Decisions 7, 14, issue 11), an optional settings DSN (issue 10) and issue 9 split in two. Rebased on origin/main at adeca98. Revision 3 records the owner's answer of 2026-11-09 that the settings version is stored with each candidate run (Decision 13, issues 12 to 14). Revision 4 answers the design verdict at cbdecf1 (main merged at 16588f8): the declarations (`internal/store/ownerset`) and the implementation (`internal/store/ownersetpg`) are two packages under the one modules.md rule shared with `market-bars.md` (Decision 6, issue 2 carries the `scripts/arch` change), entry 9 waits for entry 6 and carries the setup-literal test, entry 10 carries `wire.go`, entry 13 states its cap precondition, `SettingsVersionFunc` sits in the jobs paragraph, and the acceptance items named in the verdict are fixed.

Source: plan v2.5 section 11 row W11 "settings form (rule_param)" and plan 1.3 (owner settings versioned in the database, settings-as-of; only the gateway and the credentials live in files), as `docs/design/alignment-v25.md` cites them: the settings form is "design exists, not built" (schema, no POST handler, no persistence), the settings POST route of W1 is "NOT DESIGNED", and the tables for owner settings and exclusions are "NOT DESIGNED". Owner decisions 2026-11-08/09: options trading first and complete; `watch.toml` is hand-edited for `buy_price` and `sale_price` until valuation lands; the web renders data and computes nothing; own glue code sits behind an interface; prefer a mainstream library.
Read on origin/main at 28d0e86 and again at adeca98: `AGENTS.md`, `docs/design/modules.md`, `docs/design/alignment-v25.md`, `docs/design/option-candidates.md` (Decision 15, the keys of the candidates job), `docs/design/option-positions.md` (Decision 13, the exit parameters), `docs/design/missing-pages.md` (the form precedent and the plan-edit switch), `docs/design/macro-page-wiring.md` and `docs/design/store-readers.md` (the owner-settings store with a code default), `contracts/schemas/page.settings_page.v1.json`, `internal/store/api.go`, `internal/compute/api.go`, `cmd/ot/serve.go`, `cmd/ot/site.go`, `web/src/pages/pricelines/PriceLinesPage.tsx`, `web/src/components/ui/SOURCES.md`, `frontend-whitelist.json`.

Needs the owner: nothing in an owner-only path. One owner action after the store issue: `ot migrate`, `ot setup pg`, a new `.env` line `OT_PG_DSN_SETTINGS` and a new `.env` line `OT_FORM_KEY` (Decision 5).

## What main holds today, and what does not

- On main: `contracts/schemas/page.settings_page.v1.json` (`OptionExclusionRow {instrument_id, symbol, excluded}`, `OptionExclusionForm {post_path "/owner-settings", expires, mac}`), carried from option-trader; no view, no builder, no route, no table. `web/src/pages/pricelines/PriceLinesPage.tsx` is a precedent for rendering a form only (a plain `<form method="post">` with hidden `expires` and `mac`, no `fetch`); it has no POST handler behind it. `ot serve` (`cmd/ot/serve.go`, `cmd/ot/site.go`) is read-only: every method but GET and HEAD answers 405, and its stores are opened with the role `ot_ro_pages`.
- `store.OptionEodCatalog` (underlyings with option_eod rows) and `store.InstrumentNameReader` (names by id) exist in `internal/store/api.go`.
- `internal/store/api.go` holds 799 lines on main: one more declaration passes 800, and AGENTS.md says a file over 800 lines is listed with a ceiling that an agent never raises. Decision 6 keeps the new interface out of that file.
- On main, a consumer of two of the keys: the `option_eod` job (`internal/jobs/optioneod`, `NewWithWatch`) reads `min_dte`, `max_dte` and `max_strikes` from its embedded `job.toml`; `compute.CandidateParams` and `internal/compute/optioncand` (the put rule) and `internal/store/candidate` are on main, but no job feeds them yet and `cmd/ot` does not call `NewWithWatch` yet. Not on main: the candidates job (`option-candidates.md` issue 7), the positions builders (`option-positions.md`, two constants), any owner-settings table, any HTTP write.

## Goal

The owner opens `/settings` on `ot serve`, sees the rule parameters of the option rules with their defaults and the underlyings he can exclude from option candidates, changes some, presses save, and the change is stored, versioned, and readable as of any instant. Nothing computes in the page; the stored value is text the rule packages read through one resolver with a code default. The write is one new, narrow, loopback-only route.

## Decisions

1. Form or CLI. Window default — owner may change: a) a form at `/settings` (plan v2.5 names a settings form) b) CLI only: a read-only `/settings` page and `ot settings set <key> <value>` through `ownerset.Writer`, with no POST route, no token and no flag (issues 5, 9 part, 10 and the route guards drop; the store, registry, page and consumers stay). The rest of this design is written for a).
   What the form edits: (a) the twelve rule parameters of Decision 2 and (b) the option exclusions (one switch per underlying) that `page.settings_page.v1` already names. The form does not edit `watch.toml`, the credentials, the gateway, the zone or the alert rules (`## Left out`).
2. The rule parameters (`rule_param`), each with a code default taken from the design that owns it; the owner may change the value, never the key. From `option-candidates.md` Decision 15: `min_dte` 21, `max_dte` 45, `min_bid_size` 1, `max_spread` 0.10, `min_open_interest` 10, `div_guard_factor` 1.0, `max_rate_age_days` 7, `max_event_age_sessions` 2, `min_report_gap_days` 60, `max_strikes` 24. From `option-positions.md` Decision 13: `capture_target` 0.50, `exit_dte` 21. Window default (owner may change): this list of twelve; a key is added by a change of the registry file of issue 4, never by the form.
3. A parameter has a kind, `int` or `decimal`, a minimum and a maximum; the registry rejects text outside them. The bounds are realistic, so a slipped digit is refused and does not widen a screen silently: `min_dte` 7 to 120, `max_dte` 14 to 180 and not below `min_dte`, `min_bid_size` 1 to 100, `max_spread` 0.01 to 0.30, `min_open_interest` 1 to 1000, `div_guard_factor` 0.5 to 3, `max_rate_age_days` 1 to 30, `max_event_age_sessions` 1 to 10, `min_report_gap_days` 30 to 100, `max_strikes` 4 to 60, `capture_target` 0.10 to 0.95, `exit_dte` 1 to 45 (every default lies inside; the table is the one place a bound is changed, by a reviewed issue). The canonical stored text has no sign, no exponent and no trailing zero, made with `math/big` of the standard library (exact, the way `option-candidates.md` Decision 1 does it); there is no float.
4. watch.toml. Window default (owner may change): the form edits neither the file nor the store for `buy_price` and `sale_price`. Owner 2026-11-09: they stay hand-edited in `$OT_HOME/watch.toml` (`WatchEntry.BuyPrice`, `WatchEntry.SalePrice` of `internal/jobs/api.go`) until valuation lands. Reasons: a web route that rewrites a file the jobs read has no snapshot and no history; two places for one price would let the file and the store disagree; the owner's word in the file already wins by `option-candidates.md` Decision 3. When valuation lands the price moves to the store as a new key kind in its own design, and this design's tables and route take it without a change of shape.
5. Who may call the POST route (new write; second-opinion case). The handler is new code in `cmd/ot/site.go` (today every method but GET and HEAD answers 405 there). The route is `POST /owner-settings` (the constant `post_path` of the schema). It exists only when `ot serve` is started with `--owner-edit` (off by default; window default, owner may change) and the line `OT_PG_DSN_SETTINGS` in `.env` (the route is absent when the line or `OT_FORM_KEY` is unset, so no existing command or test needs the line) and the listen address is a loopback address; with another address and `--owner-edit`, `ot serve` refuses to start and names the flag. A request is accepted only when: the connection's remote address is loopback; the `Host` header is one of `127.0.0.1`, `localhost` or `[::1]` with the listen port; an `Origin` header, if present, is `http://` plus one of the same three names with the same port; the content type is `application/x-www-form-urlencoded`; the body is at most 64 KiB; the hidden `expires` and `mac` verify (HMAC-SHA256 of the expiry and the post path with the key `OT_FORM_KEY` of `$OT_HOME/.env`, standard library `crypto/hmac`, no new dependency) and `expires` is not in the past and not more than 15 minutes ahead (window default, owner may change). The token is a freshness and cross-site-request token, not a login: any process of the owner's machine that can fetch `/settings` can obtain one, so the route trusts the machine, as the rest of the site does. Anything wider (a bind to a LAN address, a tunnel, a reverse proxy, a second user) is a change of who may act and is a second-opinion case for the advisor; it is not built here and not allowed by any flag. The new database role of Decision 7 is a second second-opinion case. Both are named in issues 6 and 10.
6. Where the interface lives, and the implementation. A declarations-only package `internal/store/ownerset` (`api.go` and its contract test, nothing else), listed in the entry cell of the store row next to `internal/store/api.go` by the import rule for declarations-only sub-packages that `market-bars.md` owns (landed in #768); this design writes none of its own; the implementation is the separate package `internal/store/ownersetpg` (`Open`, the queries, pgx), which is not an entry and which only `cmd/ot` imports, so `pages/settings_page` and the jobs import declarations and never a database. Window default (owner may change): this split, because `internal/store/api.go` is 799 lines and one more declaration would pass the 800-line rule of AGENTS.md (an agent never raises a ceiling); the other way is that the owner allows a ceiling for `internal/store/api.go` and the declarations go there, which this design does not ask for. `scripts/arch` follows the rule once `bar-store-api` of `market-bars.md` has landed; this design's entry 2 only adds `internal/store/ownerset` to the `declarationsOnly` list there.
7. Tables, one schema `settings`, insert-only, versioned: `settings.rule_param (id bigint identity, param_key text, value text, set_at timestamptz default now())` and `settings.option_exclusion (id bigint identity, instrument_id bigint, excluded boolean, set_at timestamptz default now())`, primary key `id`, index on `(param_key, set_at desc, id desc)` and `(instrument_id, set_at desc, id desc)`. `set_at` comes from the database's `now()`, so the writer holds no clock (the precedent is `alerts.sent`). The current value of a key is its newest row; settings-as-of is the newest row with `set_at` not after the instant asked (ties by `id`). An empty `value` is a row that means "use the code default"; no row is ever updated or deleted, so a reset is one more row. Views `rd.rule_param_r1` and `rd.option_exclusion_r1` pass every column; the role `ot_ro_pages` reads the views and no table. The new role `ot_w_settings` holds INSERT and SELECT on the two tables and USAGE on schema `rd` and SELECT on the two views; no UPDATE, DELETE or TRUNCATE for anyone but the owner role. `ot_runner` gets USAGE on schema `rd` and SELECT on the two views and no table, so a job opened with the runner DSN reads settings through `ownersetpg.Open`, the value of an `ownerset.OpenFunc` (Decision 13). These are not account-derived data, so no ADR 0010 question arises. No dataset file under `contracts/datasets` is made: the tables hold mutable owner state with a versioned history, not an observation envelope; window default (owner may change).
8. Writes. `Writer.Apply(ctx, Change)` inserts, in one transaction, one row for each parameter value and each exclusion flag that differs from the current one; an unchanged field inserts nothing, so saving an untouched form writes zero rows and answers "nothing changed". Keys are checked against the registry and instrument ids against the catalog the page showed before `Apply` is called (a key or an id not on the page is 400, nothing is written).
9. Reading. The `/settings` page builder reads `Reader.Params(ctx, at)` and `Reader.Exclusions(ctx, at)` at the request instant, the names from `store.InstrumentNameReader`, the underlyings from `store.OptionEodCatalog` (window default: the exclusion list is the underlyings that have chain rows, plus any instrument with an exclusion row so a flag never disappears; the alternative is the watch list, which needs the watch-list loader of the candidates design), and fills one row per parameter with the key, label, unit, current value, default, minimum and maximum as text from the registry. Jobs resolve a value with `RuleParams.Resolve` (stored text, else the code default); the consumers are Decision 13.
10. Confirm, history, revert. A save is two posts. The first (no `confirm` field) is parsed and checked but writes nothing: it answers 200 with the page in confirm mode, a table of old value to new value for every differing field (built by the server, texts only) and a form that carries the same values plus `confirm=1` and a fresh token. The second (`confirm=1`) is parsed again against the then-current values and applied; success is a 303 to `/settings?saved=<n>` (n digits, rows written, `0` is "nothing changed"). A first post with no difference answers the page with the notice `nothing_changed`. A refused value is 422 and the page is built again with the posted values and one `error` text naming the key; a failed token or guard is 403 with a fixed text and no page; a store error is 500 with a fixed text and a log line `component ot status failed` that never carries a value. The page lists the newest 20 history rows (both tables, newest first: instant in the owner's zone, key or symbol, value or flag; window default, owner may change the number) read-only; a revert is the owner pressing "use this value" on a history row, which posts that row's value as a first post, so it goes through the same confirm step. A GET of `/settings` without `--owner-edit` shows the page with `form` null (read-only) and a gap text saying the edit switch is off.
11. UI (ADR 0034, no new package): the view is a plain `<form method="post" action="/owner-settings">` with hidden `expires` and `mac`, no `fetch`, no form library. A reset is an empty field. `frontend-whitelist.json` and `web/deps-allowlist.txt` do not change; three shadcn components that add no package (`input`, `label`, `switch`, over the installed `radix-ui`) are copied in the shadcn way and recorded in `web/src/components/ui/SOURCES.md`.
12. Own code, behind interfaces (owner 2026-11-08): the registry and the form logic are small and have no library that fits (a table of twelve keys; an HMAC token over two fields), so they are own implementations: "own implementation, because no mainstream library covers a typed key registry with exact decimal text, and the token is `crypto/hmac` of the standard library". Both sit behind `settingsapi.RuleParams` and `settingsapi.SettingsForm` so a library can replace them. A form-decoding or CSRF library (for example `gorilla/csrf`) was rejected: it keeps a session cookie the site does not have and adds transitive dependencies.
13. Consumers. Jobs resolve a value with `RuleParams.Resolve` over `Reader.Params(ctx, at)`. The consumer on main today, `option_eod`, takes `min_dte`, `max_dte` and `max_strikes` from the settings through an additive constructor of issue 11 (the embedded `job.toml` values stay the fallback and equal the registry defaults); the call from `cmd/ot` belongs to the wiring issue of the candidates design that first calls `NewWithWatch` (`option-candidates.md` issue 12 `candidates-wire`), which passes the resolved window. The other keys have no consumer on main; each is named in `## Left out` with the design that will read it. Settings-as-of: `Reader.Version(ctx, at)` returns the newest row id of both tables not after `at`, so a job can record which settings a run used; owner 2026-11-09: the version is stored with each candidate run. The column `settings_version` (nullable bigint, insert-only like the rest of the row) is added to `candidates.option_candidate` and to `store.Candidate` (issues 12 and 13, an interface change of `store/candidate` that the candidates design did not carry), and the candidates job reads `Reader.Version(ctx, at)` once at the start of a run, with the same instant it resolves the parameters at, and writes it on every row of the run (issue 14; the job takes a function `jobs.SettingsVersionFunc`, so the jobs root imports nothing of the store). A job with no settings function, or one whose call fails, writes null and logs `component jobs status degraded`; an old row has null.
14. Size and tests. Every new file stays under 600 lines. Every test calls `t.Parallel()`, writes only under `t.TempDir()`, takes time from the fake clock, opens no port (handlers run under `httptest`), touches no live broker and no database; the tests that need PostgreSQL carry the build tag `pg` and run in GitHub CI only.
15. Where the compute interfaces live (owner decision 2026-11-09, answer "a"). `RuleParamSpec`, `RuleParams`, `NewRuleParamsFunc`, `FormToken`, `FormRequest`, `FormField`, `FormState`, `FormChange`, `SettingsForm` and `NewSettingsFormFunc` are declared in the new package `internal/compute/settingsapi` (`api.go`, declarations only, and its contract test), not in `internal/compute/api.go`. Reason: that file is 835 lines on main under the ratchet ceiling of 840 (`docs/design/ratchet.json`, owner-only; an agent never raises it), and these declarations are about 45 lines, so they cannot be added to it. The package imports the standard library only (`time`), so it needs no import from `internal/compute`. The import rule is the paragraph on declarations-only packages of `docs/design/modules.md` (main's text), as in Decision 6; this design adds `settingsapi` to the entry cell of the compute row, to the two lists that paragraph and the `depends` paragraph name, and to the `ot` import list (the routes build a `settingsapi.FormRequest` from the HTTP request); entry 3 adds it to `declarationsOnly`. `docs/design/option-tracking.md` makes the same move with `internal/compute/trackapi`, and `docs/design/builders-wave.md` (open) with `internal/compute/blockapi`; they share the compute entry cell and the lists, and the later pull request rebases that text. The implementations `ruleparam` and `settingsform` stay in `internal/compute/` and import `settingsapi`; `pages/settings_page` and `cmd/ot` import it too.

## Modules

New directories: `internal/store/ownerset` (declarations), `internal/store/ownersetpg` (implementation), `internal/compute/ruleparam`, `internal/compute/settingsform`, `internal/pages/settings_page`, `web/src/pages/settings`. No new row. Every path above already belongs to a row: `internal/store/**` and `migrations/**` to store, `internal/compute/**` to compute, `internal/pages/**` to pages, `web/**` to web, `cmd/**` to ot, `internal/contracts/**` and `contracts/**` to contracts. Changed cells of `docs/design/modules.md` (edited in this pull request, with `arch.toml` regenerated by `go run ./scripts/arch`; `.go-arch-lint.yml` is tree-derived and appears with the first issue of each package):

- store, entry cell: gains `internal/store/ownerset`, a declarations-only sub-package (Decision 6); the rule for such a sub-package is `market-bars.md`'s, landed in #768 and already in `docs/design/modules.md`; this design adds only this row's entry cell and the `ot` list.
- compute, entry cell: gains `internal/compute/settingsapi`, a declarations-only sub-package (Decision 15); the "today" list and the `declarationsOnly` list of the declarations-only paragraph and the entry-listed sub-packages of the `depends` paragraph name it, and no other sentence of that rule changes.
- ot, the exact import list: gains `internal/compute/settingsapi` (declarations, Decision 15), `internal/store/ownerset` (declarations: `openOwnerSet` has type `ownerset.OpenFunc`, `ownerset.DSNEnvSettings`, `ownerset.Change`; entries 9 and 10) and `internal/store/ownersetpg` (the implementation), as `internal/store/bar` and `internal/store/barpg` stand in the same list for market-bars, `internal/compute/ruleparam`, `internal/compute/settingsform`, `internal/pages/settings_page`.

| # | issue | module | order |
|---|---|---|---|
| 1 | settings-page-schema (interface) | contracts | first |
| 2 | ownerset-api (interface) | store, plus repo (licensed pair) | first of the `.go-arch-lint.yml` chain |
| 3 | settings-compute-api (interface) | compute, plus repo (licensed pair) | after 1, 2; after `declarationsOnly` exists |
| 4 | ruleparam-impl | compute, plus repo (licensed pair) | after 3, 2 |
| 5 | settingsform-impl | compute, plus repo (licensed pair) | after 3, 4, 2 |
| 6 | ownerset-store-impl (package `ownersetpg`) | store, plus repo (licensed pair) | after 2, 4, 5 |
| 7 | settings-page-builder | pages, plus repo (licensed pair) | after 1, 2, 3, 5, 6 |
| 8 | settings-view | web | after 1 |
| 9 | settings-get-route | ot | after 2, 4, 6, 7, 8 |
| 10 | settings-post-route | ot | after 5, 6, 9 |
| 11 | option-eod-settings | jobs | after 4 |
| 12 | candidate-settings-version-dataset (interface) | contracts | first |
| 13 | candidate-settings-version-store (interface change of store/candidate) | store | after 12, 2, 6 |
| 14 | candidate-job-settings-version | jobs | after 11, 13; waits for docs/design/option-candidates.md entry 7 |

Interface files touched: `internal/store/ownerset/api.go` (new), `internal/compute/settingsapi/api.go` (new), `internal/jobs/api.go` (additive, issues 11 and 14), `internal/store/api.go` (one field of `store.Candidate`, issue 13, which waits on the cap as its entry says), `contracts/datasets/candidates.option_candidate.r1.json` (one nullable column, issue 12), the schema `contracts/schemas/page.settings_page.v2.json` (new; v1 stays as carried). The one-entry-one-module rule holds for entries 6, 8 to 14 and the implementation entries; entries 2, 4, 5 and 7 are the licensed pairs of the earlier designs (`.go-arch-lint.yml` is repo's).

## Interface

Contracts: the schema `contracts/schemas/page.settings_page.v2.json` (title `SettingsPage`): the envelope of v1 (`schema` const `settings_page`, `schema_version` const 2, `run_id`, `build_id`, `scope`, `generated_at`, `data_times`, `run_status`, `title`, `as_of`, optional `gap`, no extra field) plus `form` (`SettingsForm {post_path const "/owner-settings", expires, mac}` or null: null is read-only), `confirm` (`{rows: [ConfirmRow {label, old, new}], form: SettingsForm}` or null: set on the preview answer, its `form` carries the token of the `confirm=1` post), `history` (at most 20 `HistoryRow {at, kind (param|exclusion), name, value, key (or null), instrument_id (or null), excluded (or null)}`, newest first; the key or id and the value let the view post a revert), `params` (list of `ParamRow {key (^[a-z_]{1,48}$), label, unit, kind (int|decimal), value, default, min, max}`, all text; `value` empty means the default is in force), `rows` (`OptionExclusionRow` as in v1) and `notice` (`{kind (saved|nothing_changed|error), text}` or null). Generated type `web/src/generated/settings_page.v2.ts` (title `SettingsPage`), committed with the schema.

store, package `internal/store/ownerset`, file `api.go` (declarations only; the implementation `internal/store/ownersetpg` has `func Open(ctx context.Context, dsn string) (ownerset.Store, error)`, the value of an `ownerset.OpenFunc`):

```go
type Param struct { Key, Value string; SetAt time.Time }
type Exclusion struct { InstrumentID int64; Excluded bool; SetAt time.Time }
type Change struct { Params []ParamValue; Exclusions []ExclusionValue }
type ParamValue struct { Key, Value string }
type ExclusionValue struct { InstrumentID int64; Excluded bool }
const DSNEnvSettings = "OT_PG_DSN_SETTINGS"
type Reader interface {
	Params(ctx context.Context, at time.Time) ([]Param, error)         // newest row per key not after at, sorted by key
	Exclusions(ctx context.Context, at time.Time) ([]Exclusion, error) // newest row per instrument not after at, sorted by id
	History(ctx context.Context, n int) ([]HistoryRow, error) // newest n rows of both tables, newest first
	Version(ctx context.Context, at time.Time) (int64, error) // newest row id of both tables not after at, 0 when none
	Close() error
}
type HistoryRow struct { Kind, Key string; InstrumentID int64; Value string; Excluded bool; SetAt time.Time }
type Writer interface {
	Apply(ctx context.Context, c Change) (int, error) // rows inserted; one transaction; unchanged fields insert nothing
}
type Store interface { Reader; Writer }
type OpenFunc func(ctx context.Context, dsn string) (Store, error)
```

A DSN of the role `ot_ro_pages` opens a `Store` whose `Apply` returns a Permanent error from the database (no grant); the page process reads with that DSN and writes only with `OT_PG_DSN_SETTINGS`.

compute, new declarations-only package `internal/compute/settingsapi`, file `api.go` (`package settingsapi`; imports `time` only; not in `internal/compute/api.go`, Decision 15):

```go
type RuleParamSpec struct { Key, Label, Unit, Kind, Default, Min, Max string }
type RuleParams interface {
	Specs() []RuleParamSpec                               // the twelve keys in a fixed order
	Normalize(key, text string) (string, error)           // canonical text; "" is valid and means the default
	Resolve(stored map[string]string) map[string]string   // every key: the stored text, else the default
}
type NewRuleParamsFunc func() RuleParams
type FormToken struct { Expires, MAC string }
type FormRequest struct { RemoteAddr, Host, Origin, Listen, ContentType string; BodySize int64; Token FormToken }
type FormField struct { Key, Text string }
type FormState struct { Params map[string]string; Exclusions map[int64]bool }
type FormChange struct { Params []FormField; Exclusions map[int64]bool; Confirm bool }
type SettingsForm interface {
	Token(now time.Time) FormToken
	Check(r FormRequest, now time.Time) error                    // guard of Decision 5 and the token; a fixed-text error
	Parse(values map[string][]string, shown []int64, cur FormState) (FormChange, error) // only differing fields; Confirm is the `confirm=1` field; first refused key named
}
type NewSettingsFormFunc func(key []byte, ttl time.Duration, p RuleParams) SettingsForm
```

pages, package `internal/pages/settings_page`: `type Config struct { RunID, BuildID, Scope string; Zone *time.Location; Params settingsapi.RuleParams; Owner ownerset.Reader; Catalog store.OptionEodCatalog; Names store.InstrumentNameReader; Form *settingsapi.FormToken; Confirm *Confirm; HistoryN int; Notice *Notice }` (`Confirm` is the old-to-new table and its token, built by the caller; the builder only lays it out); `func NewPage(c Config) pages.Page` with `Meta{Name: "settings_page", Schema: "page.settings_page.v2"}`; `func Build(ctx context.Context, c Config, r store.SeriesReader, day contracts.Day, at time.Time) ([]byte, error)` (the `pages.Builder` shape; `r` is unused; nil `Owner` gives the gap `no_reader`; nil `Form` is read-only).

ot: route `GET settings` (issue 9) and `POST owner-settings` (issue 10) in `cmd/ot/site.go` through one constructor in `cmd/ot/settings.go`; flag `--owner-edit` in `cmd/ot/serve.go`; deps `newRuleParams`, `newSettingsForm`, `openOwnerSet`, `settingsPage`.

candidates (interface change of `store/candidate`, owner 2026-11-09): dataset column `settings_version` (integer or null, additive in r1); `store.Candidate` gains `SettingsVersion *int64` (nil is null); the table gains `settings_version bigint` and the view `rd.option_candidate_r1` passes it; the function type is declared by jobs, below.

jobs, additive in `internal/jobs/api.go`: `type OptionEodWindow struct { MinDTE, MaxDTE, MaxStrikes int }` and `type NewOptionEodWatchWindowFunc func(broker.Broker, store.InstrumentResolver, store.OptionEodWriter, []WatchEntry, OptionEodWindow, clock.Clock) (contracts.Job, error)` (a zero field keeps the manifest value); and, issue 14, `type SettingsVersionFunc func(ctx context.Context, at time.Time) (int64, error)`.

## Issues

Every test calls `t.Parallel()`, writes files only under `t.TempDir()`, uses a fake clock, never sleeps, opens no port, and touches no live broker or database. Acceptance items are decided by reading, grep or git; the commands run by CI are on the `Checked by CI` line of each entry. The pull request body of each entry carries a `Contracts:` line (`Contracts: unchanged` except entry 1).

### 1 settings-page-schema (interface)

Module: contracts. Size: M.
Files: `contracts/schemas/page.settings_page.v2.json` (new), `web/src/generated/settings_page.v2.ts` (new), `internal/contracts/settings_page_v2_test.go` (new), `docs/contracts/page-schemas.md`
Design: docs/design/settings-form.md#Interface
Order: first; it shares no file with another entry.
Goal: the schema of `## Interface` and its generated type, with a Go test of the contract in the style of `internal/contracts/price_lines_page_v1_test.go`. `page.settings_page.v1.json` and `contracts/schemas/carryover.sha256` are not edited.
Acceptance:
1. `grep -c '"const": "settings_page"' contracts/schemas/page.settings_page.v2.json` prints 1; `grep -c '"const": 2' contracts/schemas/page.settings_page.v2.json` is at least 1; `grep -c '"const": "/owner-settings"' contracts/schemas/page.settings_page.v2.json` prints 1; `grep -c '"additionalProperties": false' contracts/schemas/page.settings_page.v2.json` is at least 6.
2. `grep -c "export interface SettingsPage" web/src/generated/settings_page.v2.ts` prints 1 and its first line is the generated banner; `grep -c "settings_page" docs/contracts/page-schemas.md` is at least 2.
3. `grep -c "^func Test" internal/contracts/settings_page_v2_test.go` is at least 5 and `grep -c "t.Parallel()" internal/contracts/settings_page_v2_test.go` is at least that count; the test names cover a minimal valid page, `schema_version` 1, a `post_path` other than `/owner-settings`, a `mac` that is not 64 lower-case hex digits and a `ParamRow` with `kind` `float` (`grep -c "float" internal/contracts/settings_page_v2_test.go` is at least 1).
4. `git diff $(git merge-base HEAD origin/main) -- contracts/schemas/page.settings_page.v1.json contracts/schemas/carryover.sha256 | grep -c .` prints 0; `grep -c '' contracts/schemas/page.settings_page.v2.json` prints less than 600.
5. `git diff --name-only $(git merge-base HEAD origin/main)` lists no path outside the Files line.
Checked by CI: `go test ./internal/contracts -race -count=1`; `pnpm --dir web run generate:types` leaves `web/src/generated` unchanged; the pull request body carries `Contracts: page.settings_page.v2 (new)`.
Frontend stack lines: none (no UI code in this entry).

### 2 ownerset-api (interface)

Module: store, plus repo (the licensed pair: `.go-arch-lint.yml`). Size: S.
Files: `internal/store/ownerset/api.go` (new), `internal/store/ownerset/api_test.go` (new), `docs/contracts/store.md`, `scripts/arch/main.go`, `scripts/arch/main_test.go`, `arch.toml`, `.go-arch-lint.yml`
Design: docs/design/settings-form.md#Interface
Order: first of the `.go-arch-lint.yml` chain; entries 3, 4, 5, 6 and 7 share that file and come after it. It comes after `bar-store-api` of `docs/design/market-bars.md` (design landed in #768; the issue is still to land), which makes the `scripts/arch` change for the import rule of declarations-only packages; this entry adds `internal/store/ownerset` to the `declarationsOnly` list of `scripts/arch/main.go` that market-bars' entry creates (the paragraph of modules.md stays identical), with a case in `main_test.go`. It shares `docs/contracts/store.md` with 13, which comes after it.
Goal: the declarations of `## Interface` for package `ownerset`, with the contract test from both sides: a fake `Store` on one side, a caller on the other that applies a change twice and reads settings-as-of before and after it. `internal/store/api.go` is not edited (799 lines on main). `arch.toml` and `.go-arch-lint.yml` are regenerated by `go run ./scripts/arch` and nothing else changes in them; the rule that lets `ot`, the jobs and the pages import the listed package is the one `bar-store-api` put in `scripts/arch`, and the only change here is that its `declarationsOnly` list gains `internal/store/ownerset`.
Acceptance:
1. `grep -c "type Reader interface\|type Writer interface\|type Store interface\|type OpenFunc" internal/store/ownerset/api.go` prints 4; `grep -c "func " internal/store/ownerset/api.go` prints 0 (declarations only); `grep -c "pgx\|database/sql" internal/store/ownerset/api.go` prints 0.
2. `grep -c "OT_PG_DSN_SETTINGS" internal/store/ownerset/api.go` prints 1; `grep -c "ownerset" docs/contracts/store.md` is at least 2.
3. `grep -c "^func Test" internal/store/ownerset/api_test.go` is at least 2; `grep -c "time.Sleep\|time.Now()" internal/store/ownerset/api_test.go` prints 0.
4. `git diff $(git merge-base HEAD origin/main) -- internal/store/api.go go.mod go.sum | grep -c .` prints 0; `grep -c "store/ownerset" .go-arch-lint.yml` is at least 1.
5. `grep -c "^func TestRootImportsListedInterfacePackage\|^func TestGeneratedSubPackageStaysSubOnly" scripts/arch/main_test.go` prints 2 on main already (the cases `bar-store-api` adds), and `grep -c "internal/store/ownerset" scripts/arch/main.go scripts/arch/main_test.go` is at least 2 in total; `grep -c '' internal/store/ownerset/api.go` prints less than 600.
6. `git diff --name-only $(git merge-base HEAD origin/main)` lists no path outside the Files line.
Checked by CI: `go test ./internal/store/ownerset ./scripts/arch -race -count=1`, `go run ./scripts/arch --check`, `go run ./scripts/arch && git diff --exit-code arch.toml .go-arch-lint.yml`, ci-go.
Frontend stack lines: none (no UI code in this entry).

### 3 settings-compute-api (interface)

Module: compute, plus repo (the licensed pair: `scripts/arch`, `.go-arch-lint.yml`). Size: S.
Files: `internal/compute/settingsapi/api.go` (new), `internal/compute/settingsapi/api_test.go` (new), `docs/contracts/compute.md`, `scripts/arch/main.go`, `scripts/arch/main_test.go`, `arch.toml`, `.go-arch-lint.yml`
Design: docs/design/settings-form.md#Interface
Order: after 1, 2; it comes after the first entry that creates the list `declarationsOnly` in `scripts/arch/main.go` (`bar-store-api` of `docs/design/market-bars.md` or `position-marks-store-api` of `docs/design/position-marks.md`, whichever lands first) and adds `internal/compute/settingsapi` to it with a case in `main_test.go`. It shares `scripts/arch/main.go`, `scripts/arch/main_test.go` and `.go-arch-lint.yml` with entry 2 and with the same step of `docs/design/option-tracking.md` entry 2 and `docs/design/builders-wave.md` entry 1; the later pull request rebases (one added name in a list). Entries 4 and 5 add packages beside it. It no longer edits `internal/compute/api.go` or `internal/compute/api_test.go`.
Goal: the declarations of `## Interface` for package `settingsapi` (`RuleParamSpec`, `RuleParams`, `NewRuleParamsFunc`, `FormToken`, `FormRequest`, `FormField`, `FormState`, `FormChange`, `SettingsForm`, `NewSettingsFormFunc`), no function. The contract test is `internal/compute/settingsapi/api_test.go`: a fake `RuleParams` and `SettingsForm` on one side, a caller that resolves a stored map and parses a posted form on the other. `arch.toml` and `.go-arch-lint.yml` are regenerated by `go run ./scripts/arch` (the `declarationsOnly` rule puts `internal/compute/settingsapi` into `arch.toml`) and nothing else changes in them.
Acceptance:
1. `grep -c "type RuleParams interface\|type SettingsForm interface\|type NewRuleParamsFunc\|type NewSettingsFormFunc" internal/compute/settingsapi/api.go` prints 4; `grep -c "^func " internal/compute/settingsapi/api.go` prints 0; `grep -c '"time"' internal/compute/settingsapi/api.go` is at least 1.
2. `grep -c "float64\|net/http\|optdesk/internal" internal/compute/settingsapi/api.go` prints 0 (no float, no I/O type, no import of another package of this module).
3. `grep -c "^func Test" internal/compute/settingsapi/api_test.go` is at least 2; `grep -c "time.Sleep\|time.Now()" internal/compute/settingsapi/api_test.go` prints 0; `grep -c "RuleParams\|SettingsForm" docs/contracts/compute.md` is at least 2.
4. `git diff $(git merge-base HEAD origin/main) -- internal/compute/api.go internal/compute/api_test.go` prints nothing; `grep -c "internal/compute/settingsapi" scripts/arch/main.go scripts/arch/main_test.go .go-arch-lint.yml` is at least 1 for each file; `grep -c '' internal/compute/settingsapi/api.go` prints less than 600.
5. `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the Files line.
Checked by CI: `go test ./internal/compute/settingsapi ./scripts/arch -race -count=1`, `go run ./scripts/arch --check` (prints `arch: ok`), `go run ./scripts/arch && git diff --exit-code arch.toml .go-arch-lint.yml`, ci-go.
Frontend stack lines: none (no UI code in this entry).

### 4 ruleparam-impl

Module: compute, plus repo (the licensed pair: `.go-arch-lint.yml`). Size: M.
Files: `internal/compute/ruleparam/doc.go` (new), `internal/compute/ruleparam/registry.go` (new), `internal/compute/ruleparam/normalize.go` (new), `internal/compute/ruleparam/registry_test.go` (new), `internal/compute/ruleparam/normalize_test.go` (new), `.go-arch-lint.yml`
Design: docs/design/settings-form.md#Decisions (Decisions 2, 3, 12)
Order: after 3, 2; it shares `.go-arch-lint.yml` with 2, 5, 6 and 7 (2 before it, 5, 6 and 7 after it).
Goal: `ruleparam.New` (the type `settingsapi.NewRuleParamsFunc`) holds the twelve keys with kind, default, minimum, maximum, label and unit as one table; `Normalize` checks the kind and the range and returns the canonical text with `math/big`; `Resolve` returns every key with the stored text or the default. The defaults are the values of `option-candidates.md` Decision 15 and `option-positions.md` Decision 13; the bounds are those of Decision 3.
Acceptance:
1. `grep -c "min_dte\|max_dte\|min_bid_size\|max_spread\|min_open_interest\|div_guard_factor\|max_rate_age_days\|max_event_age_sessions\|min_report_gap_days\|max_strikes\|capture_target\|exit_dte" internal/compute/ruleparam/registry.go` is at least 12; `grep -c "float64\|ParseFloat" internal/compute/ruleparam/registry.go internal/compute/ruleparam/normalize.go` prints 0 for both files; `grep -c "math/big" internal/compute/ruleparam/normalize.go` is at least 1.
2. `grep -rhE "^func (TestTwelveKeysInOrder|TestDefaultsMatchDesign|TestNormalizeCanonicalText|TestNormalizeRefusesOutOfRange|TestMaxDteNotBelowMinDte|TestEmptyMeansDefault|TestResolveFillsDefaults|TestUnknownKeyRefused)\(" internal/compute/ruleparam | wc -l` prints 8 (new); `TestNormalizeCanonicalText` feeds "021", "0.10", "1e1" and " 7 " and requires "21" (or the refusal named in the test), "0.1", a refusal and "7".
3. `grep -rc "net/http\|database/sql\|os\.\|time.Now" internal/compute/ruleparam/registry.go internal/compute/ruleparam/normalize.go` prints 0 for both files (no I/O, no clock); `grep -rh "optdesk/internal/" internal/compute/ruleparam/*.go | sort -u` prints only lines naming `internal/compute`, `internal/compute/settingsapi` and `internal/contracts` (and no `internal/store` or `internal/pages`).
4. `grep -c "compute/ruleparam" .go-arch-lint.yml` is at least 1; each new file has fewer than 600 lines (`grep -c '' internal/compute/ruleparam/registry.go` and the other four print less than 600).
5. `git diff --name-only $(git merge-base HEAD origin/main)` lists no path outside the Files line; `git diff $(git merge-base HEAD origin/main) -- go.mod go.sum | grep -c .` prints 0.
Checked by CI: `go test ./internal/compute/ruleparam -race -count=1`, `go run ./scripts/arch --check`, ci-go.
Frontend stack lines: none (no UI code in this entry).

### 5 settingsform-impl

Module: compute, plus repo (the licensed pair: `.go-arch-lint.yml`). Size: M.
Files: `internal/compute/settingsform/doc.go` (new), `internal/compute/settingsform/token.go` (new), `internal/compute/settingsform/guard.go` (new), `internal/compute/settingsform/parse.go` (new), `internal/compute/settingsform/token_test.go` (new), `internal/compute/settingsform/guard_test.go` (new), `internal/compute/settingsform/parse_test.go` (new), `.go-arch-lint.yml`
Design: docs/design/settings-form.md#Decisions (Decisions 5, 8, 10, 12)
Order: after 3, 4, 2; it shares `.go-arch-lint.yml` with 2 and 4 (before it) and 6 and 7 (after it).
Goal: `settingsform.New` (the type `settingsapi.NewSettingsFormFunc`): `Token` signs `expires` and the post path with HMAC-SHA256 (`crypto/hmac`, hex lower case); `Check` applies the guard of Decision 5 in a fixed order (remote address, host, origin, content type, body size, token format, MAC with `hmac.Equal`, expiry) and returns a fixed-text error that never names the key or the expected MAC; `Parse` reads `param.<key>` fields and the `row` and `excluded` fields, refuses a key or an id not shown, normalizes through the `settingsapi.RuleParams` it was given and returns only the fields that differ from the current state. It reads no clock (time is an argument), no environment and no file.
Acceptance:
1. `grep -rhE "^func (TestTokenRoundTrip|TestTokenExpired|TestTokenTooFarAhead|TestTokenBadMAC|TestCheckRefusesNonLoopbackRemote|TestCheckRefusesForeignOrigin|TestCheckRefusesWrongHost|TestCheckRefusesBigBody|TestCheckRefusesWrongContentType|TestParseOnlyDifferingFields|TestParseRefusesUnknownKey|TestParseRefusesUnshownInstrument|TestParseUncheckedSwitchMeansIncluded|TestParseEmptyIsReset|TestErrorTextNamesNoSecret)\(" internal/compute/settingsform | wc -l` prints 15 (new).
2. `grep -c "crypto/hmac" internal/compute/settingsform/token.go` is at least 1 and `grep -c "hmac.Equal" internal/compute/settingsform/guard.go` is at least 1; `grep -rc "==.*mac\|mac.*==" internal/compute/settingsform/guard.go` prints 0 (no direct comparison of the MAC); `grep -rc "time.Now\|os.Getenv\|net/http\|http.Request" internal/compute/settingsform/token.go internal/compute/settingsform/guard.go internal/compute/settingsform/parse.go` prints 0 for each file.
3. `git diff $(git merge-base HEAD origin/main) -- go.mod go.sum | grep -c .` prints 0 (the standard library only); `grep -c "compute/settingsform" .go-arch-lint.yml` is at least 1.
4. The tests use a fixed 32-byte key written in the test file and the fake clock of the test kit (`grep -c "time.Sleep" internal/compute/settingsform/token_test.go` prints 0); each new file has fewer than 600 lines.
5. `git diff --name-only $(git merge-base HEAD origin/main)` lists no path outside the Files line.
Checked by CI: `go test ./internal/compute/settingsform -race -count=1`, `go run ./scripts/arch --check`, ci-go.
Frontend stack lines: none (no UI code in this entry).

### 6 ownerset-store-impl

Module: store, plus repo (the licensed pair: `.go-arch-lint.yml`). Size: M.
Second opinion: a new database role, `ot_w_settings`, and the first table the owner's own changes go into: `ot_w_settings` has INSERT and SELECT on `settings.rule_param` and `settings.option_exclusion`, USAGE on schema `rd` and SELECT on `rd.rule_param_r1` and `rd.option_exclusion_r1`; `ot_ro_pages` and `ot_runner` have USAGE on `rd` and SELECT on the two views and nothing on the tables; no UPDATE, DELETE or TRUNCATE for any role but the owner role.
Files: `migrations/NNNN_settings.sql` (NNNN the next free number at cut time; migrations apply out of order, #693; other designs write their numbers the same way, so none is claimed here), `internal/store/ownersetpg/store.go` (new), `internal/store/ownersetpg/read.go` (new), `internal/store/ownersetpg/store_test.go` (new), `internal/store/ownersetpg/store_pg_test.go` (new, tag `pg`), `internal/store/setup/setup.go`, `internal/store/setup/setup_test.go`, `internal/store/setup/settings_pg_test.go` (new, tag `pg`), `arch.toml`, `.go-arch-lint.yml`
Design: docs/design/settings-form.md#Decisions (Decisions 6, 7, 8)
Order: after 2, 4, 5; it is a new package beside the package of 2 and edits none of its files. It shares `.go-arch-lint.yml` with 2, 4 and 5 (before it) and 7 (after it). It shares the next free migration number with 13 (after it) and `setup.go` and the number with other designs (it rebases on whichever lands first and takes the next free number at cut time); it shares no file with another entry of this design.
Goal: the two tables, the two views and the role `ot_w_settings` (made in `internal/store/setup/setup.go` with the line `OT_PG_DSN_SETTINGS`, as `ot_w_candidate` is); `ownersetpg.Open(ctx, dsn)` implements `ownerset.Store` (the package imports `internal/store/ownerset` for the types). `Params` and `Exclusions` read the views only, with `DISTINCT ON (key) ... WHERE set_at <= $1 ORDER BY key, set_at DESC, id DESC`; `Apply` reads the current values in the same transaction and inserts one row for each field that differs; `History` and `Version` read the views only. The literal `OT_PG_DSN_SETTINGS` in `setup.go` equals `ownerset.DSNEnvSettings` (the check is the named test of entry 9, because `setup` is a sibling sub-package; `ot` sees both).
Acceptance:
1. `grep -c "CREATE TABLE settings.rule_param\|CREATE TABLE settings.option_exclusion" migrations/*_settings.sql` prints 2; `grep -ci "GRANT.*\(UPDATE\|DELETE\|TRUNCATE\)" migrations/*_settings.sql` prints 0; `grep -c "GRANT SELECT ON rd.rule_param_r1 TO ot_ro_pages\|GRANT SELECT ON rd.option_exclusion_r1 TO ot_ro_pages" migrations/*_settings.sql` prints 2; `grep -c "GRANT USAGE ON SCHEMA rd TO ot_w_settings" migrations/*_settings.sql` prints 1; `grep -c "GRANT SELECT ON rd.rule_param_r1 TO ot_runner\|GRANT SELECT ON rd.option_exclusion_r1 TO ot_runner" migrations/*_settings.sql` prints 2, and no GRANT line names both `ot_runner` and a table of schema `settings`.
2. `grep -c "settings.rule_param\|settings.option_exclusion" internal/store/ownersetpg/read.go` prints 0 and `grep -c "rd.rule_param_r1\|rd.option_exclusion_r1" internal/store/ownersetpg/read.go` is at least 2 (the reader names the views and no base table); `grep -rl "os.Getenv" internal/store/ownersetpg` prints nothing.
3. `grep -rhE "^func (TestParamsAsOf|TestExclusionsAsOf|TestApplyWritesOnlyDifferences|TestApplyUnchangedWritesNothing|TestApplyIsOneTransaction|TestEmptyValueIsDefaultRow|TestHistoryNewestFirst|TestVersionIsNewestId|TestSettingsRoleSpec)\(" internal/store/ownersetpg internal/store/setup | wc -l` prints 9 (new; the first eight use a fake of the query layer, `TestSettingsRoleSpec` pins the role `ot_w_settings` and the line `OT_PG_DSN_SETTINGS` in the role list of `setup.go`).
4. `grep -c "^func TestSettingsRoundTrip\|^func TestSettingsRolesCannotUpdate" internal/store/ownersetpg/store_pg_test.go internal/store/setup/settings_pg_test.go` prints 2 in total, and both files start with `//go:build pg`; `grep -c '' internal/store/setup/setup.go` prints less than 800.
5. `grep -rl "store/ownersetpg" internal/notify internal/runner internal/pages internal/jobs` prints nothing; `git diff --name-only $(git merge-base HEAD origin/main)` lists no path outside the Files line; each new file has fewer than 600 lines; `grep -c "store/ownersetpg" .go-arch-lint.yml` is at least 1.
Checked by CI: `go test ./internal/store/ownersetpg ./internal/store/setup -race -count=1`; the two `pg` tests run only in GitHub CI; `go run ./scripts/arch --check`, `go run ./scripts/arch && git diff --exit-code arch.toml .go-arch-lint.yml`. The owner runs `ot migrate`, `ot setup pg` and adds `OT_PG_DSN_SETTINGS` to `.env` before the first save.
Frontend stack lines: none (no UI code in this entry).

### 7 settings-page-builder

Module: pages, plus repo (the licensed pair: `.go-arch-lint.yml`). Size: M.
Files: `internal/pages/settings_page/page.go` (new), `internal/pages/settings_page/builder.go` (new), `internal/pages/settings_page/rows.go` (new), `internal/pages/settings_page/page_test.go` (new), `internal/pages/settings_page/builder_test.go` (new), `internal/pages/settings_page/testdata/` (new), `docs/contracts/pages.md`, `.go-arch-lint.yml`
Design: docs/design/settings-form.md#Decisions (Decisions 9, 10, 11) and #Interface
Order: after 1, 2, 3, 5, 6; it shares `.go-arch-lint.yml` with 2, 4, 5 and 6 (all before it).
Goal: `Build` reads the parameters and the exclusions through `Config.Owner` at the request instant, the underlyings through `Config.Catalog`, the names through `Config.Names`, fills one `ParamRow` per `RuleParams.Specs()` key (the stored text, else empty; the default, the bounds and the unit as the registry gives them) and one `OptionExclusionRow` per underlying, in sorted order, sets `form` from `Config.Form` (null is read-only, with the gap text of Decision 10), lays out `Config.Confirm` as `confirm`, lists `Config.HistoryN` rows of `Reader.History` as `history`, maps `Config.Notice` to `notice`, and validates against the schema of entry 1. The builder compares nothing, formats no number and signs nothing: the token arrives made.
Acceptance:
1. `grep -rhE "^func (TestBuildFillsOneRowPerKey|TestBuildShowsDefaultsAndBounds|TestBuildExclusionRowsSorted|TestBuildKeepsFlaggedInstrumentWithoutChain|TestBuildReadOnlyHasNullForm|TestBuildConfirmLaidOut|TestBuildHistoryNewestFirst|TestBuildNilOwnerIsGap|TestBuildNoticeKinds|TestBuildValidatesAgainstSchema|TestNewPageMeta)\(" internal/pages/settings_page | wc -l` prints 11 (new).
2. `grep -rc "sort\.\|big\.\|float64\|hmac\|strconv.Parse" internal/pages/settings_page/builder.go internal/pages/settings_page/rows.go` prints 0 for both files (the sort order comes from the readers; no arithmetic, no MAC); `grep -c "database/sql\|pgx\|INSERT\|Apply(" internal/pages/settings_page/builder.go` prints 0 (the page never writes).
3. `grep -c "settings_page" docs/contracts/pages.md` is at least 1; `grep -c "settings_page" .go-arch-lint.yml` is at least 1; `grep -c 'Schema: "page.settings_page.v2"' internal/pages/settings_page/page.go` prints 1.
4. `grep -rc "time.Sleep\|time.Now()" internal/pages/settings_page/builder_test.go internal/pages/settings_page/page_test.go` prints 0 for both files; each new file has fewer than 600 lines.
5. `git diff --name-only $(git merge-base HEAD origin/main)` lists no path outside the Files line.
Checked by CI: `go test ./internal/pages/settings_page -race -count=1`, `go run ./scripts/arch --check`, `go run ./scripts/arch && git diff --exit-code arch.toml .go-arch-lint.yml`, ci-go.
Frontend stack lines: none (no UI code in this entry).

### 8 settings-view

Module: web. Size: M.
Files: `web/src/pages/settings/SettingsPage.tsx` (new), `web/src/types/settings_page.ts` (new), `web/src/components/ui/input.tsx` (new), `web/src/components/ui/label.tsx` (new), `web/src/components/ui/switch.tsx` (new), `web/src/components/ui/SOURCES.md`, `web/src/shell/TopNav.tsx`, `web/test/pages/settings/SettingsPage.test.tsx` (new), `web/test/pages/settings/fixtures/` (new), `web/test/shell/TopNav.test.tsx`
Design: docs/design/settings-form.md#Decisions (Decision 11)
Order: after 1; it shares `TopNav.tsx` and its test with the nav items other designs add (it rebases on whichever lands first); it shares no file with another entry of this design.
Goal: the page `SettingsPage` shows the parameter table (label, unit, current value or the default, bounds), one switch per underlying and one save button, all texts from the payload; the form is a plain `<form method="post" action="/owner-settings">` with hidden `expires` and `mac` from `payload.form`; with `form` null every control is disabled and the gap text shows; `confirm` shows the old-to-new table with a second form (the same fields as hidden inputs, `confirm=1` and the token of `confirm.form`) and a cancel link; `history` shows as a table with a small form per row that posts that row's value ("use this value"); `notice` shows as an alert; the view writes no `fetch(`, computes and compares nothing; the registration `export const registration` is found by the glob of `web/src/shell/pages.tsx`. `TopNav.tsx` gets the item `设置` at `/settings` (the last item).
Acceptance:
1. `grep -c 'method="post"' web/src/pages/settings/SettingsPage.tsx` is at least 1; `grep -c 'action="/owner-settings"\|payload.form.post_path' web/src/pages/settings/SettingsPage.tsx` is at least 1; `grep -c 'name="expires"\|name="mac"' web/src/pages/settings/SettingsPage.tsx` is at least 2; `grep -c "fetch(" web/src/pages/settings/SettingsPage.tsx` prints 0.
2. `grep -c "export const registration" web/src/pages/settings/SettingsPage.tsx` prints 1; `grep -c "settings_page.v2" web/src/types/settings_page.ts` is at least 1; `grep -c "/settings" web/src/shell/TopNav.tsx` is at least 1.
3. `grep -c "settings_page\|owner-settings" web/test/pages/settings/SettingsPage.test.tsx` is at least 2; the test renders a fixture with `form` and a fixture with `form` null and checks the hidden inputs equal the payload, one `name="param.<key>"` input per `params` row and one switch per `rows` entry, and that every control is disabled when `form` is null (`grep -c "disabled" web/test/pages/settings/SettingsPage.test.tsx` is at least 1).
4. `grep -c "input.tsx\|label.tsx\|switch.tsx" web/src/components/ui/SOURCES.md` is at least 3; `git diff $(git merge-base HEAD origin/main) -- frontend-whitelist.json web/package.json pnpm-lock.yaml | grep -c .` prints 0 (no package change).
5. `git diff --name-only $(git merge-base HEAD origin/main)` lists no path outside the Files line; each new file has fewer than 600 lines.
Checked by CI: `pnpm --dir web run typecheck`, `pnpm --dir web run lint`, `pnpm --dir web run test`, `pnpm --dir web run build`, `pnpm install --frozen-lockfile`.
Frontend stack lines:
- 按 frontend-stack：用 shadcn input (`components/ui/input.tsx`, no new package) 做 the text field of each rule parameter (`name="param.<key>"`, `inputMode` from `kind`, the default as placeholder)。
- 按 frontend-stack：用 shadcn switch (`components/ui/switch.tsx`, over the installed `radix-ui`; inside a form it submits a hidden checkbox input with its name) 做 the exclusion switch of each underlying (`name="excluded"`, `value` the instrument id; a hidden `row` input carries every shown id)。
- 按 frontend-stack：用 shadcn label (`components/ui/label.tsx`, over `radix-ui`) 做 the label of every field。
- 按 frontend-stack：用 shadcn table (`components/ui/table.tsx`) 做 the parameter table, the exclusion table, the confirm table and the history table。
- 按 frontend-stack：用 shadcn card, button and alert (`components/ui`) 做 the page frame, the save button (`asChild` is not used: a submit button) and the notice and read-only gap messages。
- 按 frontend-stack：用 lucide-react 做 the icons of the notice states; recharts, tanstack table, tabs and accordion are not used: no chart, no sort, no filter。
- 按 frontend-stack：用 shadcn Button (`asChild`, `components/ui/button.tsx`) 做 the `设置` anchor of the bar in `TopNav.tsx`, like the other items。

### 9 settings-get-route

Module: ot. Size: S.
Files: `cmd/ot/settings.go` (new), `cmd/ot/settings_test.go` (new), `cmd/ot/site.go`, `cmd/ot/site_test.go`, `cmd/ot/wire.go`, `cmd/ot/wire_test.go`, `docs/contracts/ot.md`
Design: docs/design/settings-form.md#Decisions (Decisions 9, 10)
Order: after 2, 4, 6, 7, 8 (6 creates `ownersetpg.Open`, which `openOwnerSet` calls); entry 10 comes after it and shares `settings.go`, `settings_test.go`, `site.go`, `site_test.go`, `wire.go`, `wire_test.go` and `ot.md` with it; entry 11 shares no file with it. Designs outside this one edit `site.go`, `wire.go` and `ot.md` too (candidates and positions wiring); it rebases on whichever lands first.
Goal: `GET /settings` builds the page per request with `settings_page.NewPage` over the readers the site already holds plus an `ownerset.Reader` opened by `ownersetpg.Open` with the pages DSN; `form` is null and the gap text says the edit switch is off (the POST route is entry 10). A missing pages DSN or reader gives the page gap `no_reader`, never a 500. The deps `newRuleParams`, `openOwnerSet` and `settingsPage` are added to `wire.go`; no `.env` line becomes required, so the tests that build `OT_PG_DSN_*` strings (`wire_test.go` is the only one edited, for the new deps) need no other change.
Acceptance:
1. The tests `TestGetSettingsReadOnly`, `TestGetSettingsNoReaderIsGap` and `TestSettingsRouteIsGetOnly` exist in `cmd/ot/settings_test.go`, and `TestSetupLineEqualsOwnersetConst` (the literal `OT_PG_DSN_SETTINGS` of `internal/store/setup/setup.go` equals `ownerset.DSNEnvSettings`) exists in `cmd/ot/wire_test.go`.
2. `settingsPage` and `openOwnerSet` are fields of the deps struct in `cmd/ot/wire.go`; `cmd/ot/settings.go` has fewer than 600 lines and holds no `sort.`, `big.` or `float64`.
3. `docs/contracts/ot.md` names `/settings`; `grep -c '' cmd/ot/site.go cmd/ot/wire.go` shows each under 800.
4. `git diff --name-only $(git merge-base HEAD origin/main)` lists no path outside the Files line.
Checked by CI: `go test ./cmd/ot -race -count=1`, golangci-lint, `go run ./scripts/arch --check`.
Frontend stack lines: none (no UI code in this entry).

### 10 settings-post-route

Module: ot. Size: M.
Second opinion: the first HTTP write of the site and a change of who may act: before it no request changed anything; after it a request of a process on the owner's machine can insert settings rows. Questions: (a) the route exists only with `--owner-edit`, on a loopback listen address, with `OT_PG_DSN_SETTINGS` and `OT_FORM_KEY` both set, and `ot serve` refuses `--owner-edit` with another address; (b) every guard of Decision 5 holds on every request, and a failed guard writes nothing; (c) the key and the DSN are read from `.env` only and never logged; (d) no flag or setting widens the bind or the allowed callers.
Files: `cmd/ot/settings.go`, `cmd/ot/settings_test.go`, `cmd/ot/site.go`, `cmd/ot/site_test.go`, `cmd/ot/serve.go`, `cmd/ot/serve_test.go`, `cmd/ot/env.go`, `cmd/ot/env_test.go`, `cmd/ot/wire.go`, `cmd/ot/wire_test.go`, `docs/contracts/ot.md`
Design: docs/design/settings-form.md#Decisions (Decisions 5, 8, 10)
Order: after 5, 6, 9; it edits `settings.go`, `settings_test.go`, `site.go`, `site_test.go`, `wire.go`, `wire_test.go` and `ot.md`, which entry 9 created or edited (it adds the dep `newSettingsForm` to `wire.go`), and no later entry of this design touches them. Other designs edit `site.go` and `env.go` too; it rebases on whichever lands first.
Goal: the POST handler is new code in `site.go`, an adapter of calls. `POST /owner-settings`: `Check`, `ParseForm` under `http.MaxBytesReader` (64 KiB), `Parse`; without `confirm` it answers the confirm page (200) and writes nothing; with `confirm=1` it parses again, maps the `FormChange` to an `ownerset.Change`, `Apply`, 303 to `/settings?saved=<n>`; a refused value is 422 with the page built again; a failed `Check` is 403 with a fixed text and no page; another method on that path is 405 with `Allow: POST`. `GET /settings` now carries a token and the history. `runServe` refuses `--owner-edit` with a non-loopback address and names the flag. An unset or short `OT_FORM_KEY` (under 32 bytes) or an unset `OT_PG_DSN_SETTINGS` leaves the route out with one named log line; no existing command lists the two lines as required, so no other `cmd/ot` test changes.
Acceptance:
1. The tests `TestPostPreviewWritesNothing`, `TestPostConfirmSavesAndRedirects`, `TestPostBadTokenIs403AndWritesNothing`, `TestPostRefusedValueIs422`, `TestPostRouteAbsentWithoutFlag` and `TestPostHostNamesAndOrigin` exist in `cmd/ot/settings_test.go`.
2. `TestOwnerEditRefusesNonLoopbackListen` exists in `cmd/ot/serve_test.go` and `TestUnsetOrShortKeyLeavesRouteOut` in `cmd/ot/env_test.go`.
3. `cmd/ot/settings.go` names `127.0.0.1`, `localhost` and `[::1]` and calls `http.MaxBytesReader`; `cmd/ot/serve.go` calls `IsLoopback`; neither file holds `0.0.0.0`.
4. `newSettingsForm` is a field of the deps struct in `cmd/ot/wire.go`; `grep -c '' cmd/ot/site.go cmd/ot/wire.go cmd/ot/env.go` shows each under 800; `docs/contracts/ot.md` names `OT_FORM_KEY`, `OT_PG_DSN_SETTINGS` and `--owner-edit`; `cmd/ot/settings.go` holds no `sort.`, `big.`, `float64` or `hmac.` and has fewer than 600 lines.
5. `git diff --name-only $(git merge-base HEAD origin/main)` lists no path outside the Files line.
Checked by CI: `go test ./cmd/ot -race -count=1`, golangci-lint, `go run ./scripts/arch --check`. The owner adds `OT_PG_DSN_SETTINGS` and `OT_FORM_KEY` (32 random bytes as hex) to `.env`, runs `ot migrate` and `ot setup pg`, and starts `ot serve --owner-edit` when he wants to edit.
Frontend stack lines: none (no UI code in this entry).

### 11 option-eod-settings

Module: jobs. Size: S.
Files: `internal/jobs/api.go`, `internal/jobs/api_test.go`, `internal/jobs/optioneod/watch.go`, `internal/jobs/optioneod/watch_test.go`, `docs/contracts/jobs.md`
Design: docs/design/settings-form.md#Decisions (Decision 13)
Order: after 4 (the keys and bounds it takes); it shares `internal/jobs/api.go`, `internal/jobs/api_test.go` and `docs/contracts/jobs.md` with 14, which comes after it. The call from `cmd/ot` is not here: it belongs to `candidates-wire` of `option-candidates.md`, which first calls `NewWithWatch`.
Goal: `OptionEodWindow` and `NewOptionEodWatchWindowFunc` (additive in `jobs/api.go`) and `optioneod.NewWithWatchWindow`, which behaves as `NewWithWatch` and overrides `MinDTE`, `MaxDTE` and `MaxStrikes` of the manifest with each non-zero field of the window; a window that breaks the manifest's own rule (`min_dte` above `max_dte`) gives the same error `validate` gives today. `NewWithWatch` is not edited.
Acceptance:
1. `OptionEodWindow` and `NewOptionEodWatchWindowFunc` are declared in `internal/jobs/api.go`, and `git diff $(git merge-base HEAD origin/main) -- internal/jobs/api.go | grep '^-' | grep -vc '^---'` prints 0 (additive).
2. The tests `TestWindowOverridesManifest`, `TestZeroWindowKeepsManifest` and `TestWindowMinAboveMaxFails` exist in `internal/jobs/optioneod/watch_test.go`, and `TestOptionEodWindowContract` exists in `internal/jobs/api_test.go`.
3. `git diff $(git merge-base HEAD origin/main) -- internal/jobs/optioneod/watch.go | grep '^-' | grep -vc '^---'` prints 0 (`NewWithWatch` unchanged).
4. `docs/contracts/jobs.md` names `NewWithWatchWindow`; `grep -c '' internal/jobs/api.go` shows under 800.
5. `git diff --name-only $(git merge-base HEAD origin/main)` lists no path outside the Files line.
Checked by CI: `go test ./internal/jobs ./internal/jobs/optioneod -race -count=1`, ci-go, `go run ./scripts/arch --check`.
Frontend stack lines: none (no UI code in this entry).

### 12 candidate-settings-version-dataset (interface)

Module: contracts. Size: S.
Files: `contracts/datasets/candidates.option_candidate.r1.json`, `internal/contracts/gen/records.go`, `docs/contracts/datasets/candidates.option_candidate.md`, `contracts/fixtures/candidates.option_candidate.r1.json`, `contracts/fixtures/MANIFEST.synthetic-v1.json`
Design: docs/design/settings-form.md#Interface (owner 2026-11-09; an interface change of the dataset of `option-candidates.md` entry 4, which is on main)
Order: first; it shares no file with another entry of this design.
Goal: one nullable integer column `settings_version` in the dataset r1, additive (no existing field changes, the primary key is unchanged), with the generated Go record and the document committed together; the fixture gains the field on one row and omits it on another.
Acceptance:
1. The dataset file names `settings_version` once, and `x-primary-key` is unchanged (`["session_day","run_id","kind","instrument_id","alias"]`).
2. `internal/contracts/gen/records.go` names `SettingsVersion` and its first line is the generated banner; the dataset document names `settings_version`.
3. The fixture holds one row with `settings_version` and one without it.
4. `git diff --name-only $(git merge-base HEAD origin/main)` lists no path outside the Files line.
Checked by CI: `go test ./internal/contracts -race -count=1`, the generate check of `checks.yml` (`bash contracts/generate.sh && git diff --exit-code internal/contracts/gen`); the pull request body carries `Contracts: candidates.option_candidate.r1 (settings_version added)`.
Frontend stack lines: none (no UI code in this entry).

### 13 candidate-settings-version-store (interface change of store/candidate)

Module: store. Size: S.
Second opinion: none beyond entry 6 (no new role, no new grant: `ot_w_candidate` already holds INSERT and SELECT on the table, and the view is replaced with one more column).
Files: `migrations/NNNN_candidate_settings_version.sql` (NNNN the next free number at cut time; none is claimed here), `internal/store/api.go`, `internal/store/api_candidate_test.go`, `internal/store/candidate/store.go`, `internal/store/candidate/read.go`, `internal/store/candidate/store_test.go`, `internal/store/candidate/store_pg_test.go` (tag `pg`), `docs/contracts/store.md`
Design: docs/design/settings-form.md#Decisions (Decision 13)
Order: after 12, 2, 6; it shares `docs/contracts/store.md` with 2 and the next free migration number with 6 (both before it). Precondition on the cap: `internal/store/api.go` is 799 lines on main, so one added field is the last line the 800-line rule of AGENTS.md allows, and another design that adds a line there first makes this entry impossible to cut. Choice: this entry is cut only while `internal/store/api.go` has at most 799 lines at cut time; otherwise it waits for the owner's ceiling for that file (owner-only) and the pull request description reports it, and the entry does not split the file. Why not move the `Candidate` declarations into a second file or package: that changes the import path or the entry cell for every reader of `store.Candidate` (the candidates page, the job, `candidate_text`), an interface change of three other designs, for one field.
Goal: the migration adds `settings_version bigint` (nullable) to `candidates.option_candidate` and replaces the view `rd.option_candidate_r1` with the same columns plus it; `store.Candidate` gains `SettingsVersion *int64`; the insert and the reader carry it. Insert-only is kept (no UPDATE, no DELETE; old rows hold null). This is an interface change of `store/candidate`: `store.Candidate` is a type of the interface file.
Acceptance:
1. `migrations/*_candidate_settings_version.sql` names `ADD COLUMN settings_version`, `CREATE OR REPLACE VIEW rd.option_candidate_r1` and no `UPDATE`, `DELETE` or `GRANT`.
2. `SettingsVersion *int64` is a field of `store.Candidate` in `internal/store/api.go`; `git diff $(git merge-base HEAD origin/main) -- internal/store/api.go | grep '^-' | grep -vc '^---'` prints 0 and `grep -c '' internal/store/api.go` prints at most 800.
3. `internal/store/candidate/read.go` names the view `rd.option_candidate_r1` and no base table; `internal/store/candidate/store.go` names `settings_version`.
4. The tests `TestSettingsVersionRowMapping` and `TestNilSettingsVersionIsNull` exist in `internal/store/candidate/store_test.go`, `TestCandidateContractSettingsVersion` in `internal/store/api_candidate_test.go` and `TestSettingsVersionRoundTrip` in `internal/store/candidate/store_pg_test.go`.
5. `git diff --name-only $(git merge-base HEAD origin/main)` lists no path outside the Files line.
Checked by CI: `go test ./internal/store ./internal/store/candidate -race -count=1`; the `pg` test runs only in GitHub CI; `go run ./scripts/arch --check`. The owner runs `ot migrate`.
Frontend stack lines: none (no UI code in this entry).

### 14 candidate-job-settings-version

Module: jobs. Size: S.
Files: `internal/jobs/api.go`, `internal/jobs/api_test.go`, `internal/jobs/optioncandidates/job.go`, `internal/jobs/optioncandidates/job_test.go`, `docs/contracts/jobs.md`
Design: docs/design/settings-form.md#Decisions (Decision 13)
Order: after 11, 13; it waits for docs/design/option-candidates.md entry 7 (`candidates-job-puts`, which creates `internal/jobs/optioncandidates`) and, for the covered-call rows, entry 11 of that design. It shares `internal/jobs/api.go`, `internal/jobs/api_test.go` and `docs/contracts/jobs.md` with 11 (before it).
Goal: `jobs.SettingsVersionFunc` (additive) and an option of the candidates job that takes it: at the start of a run the job calls it once with the run's instant and writes the value as `SettingsVersion` on every row of that run, for every kind it writes; no function, or an error, writes nil and logs one line. The job resolves no parameter here (that is the wiring of `option-candidates.md` entry 12 together with `NewWithWatchWindow`); the call from `cmd/ot` passes `ownerset.Reader.Version` and is Left out with it.
Acceptance:
1. `SettingsVersionFunc` is declared in `internal/jobs/api.go` and `git diff $(git merge-base HEAD origin/main) -- internal/jobs/api.go | grep '^-' | grep -vc '^---'` prints 0 (additive).
2. The tests `TestRunWritesOneSettingsVersionOnEveryRow`, `TestNoSettingsFuncWritesNil`, `TestSettingsFuncErrorWritesNilAndRuns` exist in `internal/jobs/optioncandidates/job_test.go`, and `TestSettingsVersionFuncContract` in `internal/jobs/api_test.go`.
3. `internal/jobs/optioncandidates/job.go` calls the function once per `Run` (the test counts calls) and imports no `store/ownerset`.
4. `docs/contracts/jobs.md` names `SettingsVersionFunc`; `grep -c '' internal/jobs/api.go` shows under 800.
5. `git diff --name-only $(git merge-base HEAD origin/main)` lists no path outside the Files line.
Checked by CI: `go test ./internal/jobs ./internal/jobs/optioncandidates -race -count=1`, ci-go, `go run ./scripts/arch --check`.
Frontend stack lines: none (no UI code in this entry).

## Left out

- Editing `buy_price` and `sale_price` (Decision 4): stays hand-edited in `watch.toml` until valuation lands; shown nowhere on the settings page. Why: owner 2026-11-09, and one place per price. Window default; the owner may ask for a read-only column or a form field later.
- The alert rules (`internal/compute/alertrules`, the file `$OT_HOME/alerts.toml` or the embedded rules): stay in the file. The gap table row "owner settings versioned in DB ...; alert rules file conflicts with plan 1.3" is "altered, pending owner". Why: moving them is a decision of its own; this design gives the table and route they would use. Asked.
- The zone (`OT_ZONE`), the gateway and the credentials: stay in the process environment and `.env` (plan 1.3 keeps the gateway and credentials in files; `profile.Load` needs the zone before any database is open).
- Consumers with no code on main, each reads `RuleParams.Resolve` in its own design: `min_bid_size`, `max_spread`, `min_open_interest`, `div_guard_factor`, `max_rate_age_days`, `max_event_age_sessions`, `min_report_gap_days` by the candidates job (`option-candidates.md` issue 7 `candidates-job-puts`, which fills `compute.CandidateParams`); `min_dte`, `max_dte`, `max_strikes` are also passed to `option_eod` by `candidates-wire` (issue 12 of the same design, with entry 11 here); `capture_target` and `exit_dte` by the positions builders (`option-positions.md` Decision 13, `StructureParams`); the macro default window (`store-readers.md` Left out) has no key yet. Until a consumer lands its key is stored and shown but changes nothing; the page says so in its gap text only if the owner asks.
- The `cmd/ot` calls that hand the settings to the jobs: `ownerset.Reader.Version` as the `SettingsVersionFunc` of the candidates job, and the resolved window to `NewWithWatchWindow`; both belong to `candidates-wire` (`option-candidates.md` entry 12), which waits for the job entry 7 there. The settings version is not shown on the candidates page.
- Consuming the exclusions (a gate `excluded` in the candidates job, a note on the candidates page): the next step of `option-candidates.md`; the flag is stored and readable now.
- Remote access, a login, a second user, per-field permissions: not built and not allowed by a flag (Decision 5, second-opinion case).
- Rejected: a form library or CSRF library (session cookie, transitive dependencies); one JSON blob row for all settings (no per-key history); updating rows in place (no history, the role would need UPDATE); the interface in `internal/store/api.go` (800-line rule of AGENTS.md).

## Adversarial

The full answer of the fresh read is `adv-settings-form.md` in the scratchpad of the drafting session (`scratchpad/adv-settings-form.md`). Points 1 to 6, answered:

1. Nothing reads the settings, `ot_runner` has no grant, no settings-as-of, exclusions not consumed: adopted (the exclusion consumer stays Left out). `ot_runner` gets SELECT on the two views (Decision 7, issue 6); the one consumer on main, `option_eod`, gets issue 11; the other consumers are named under Left out with the design that reads each, because their code is not on main (`optioncandidates` does not exist; `cmd/ot` does not call `NewWithWatch`). `Reader.Version` gives settings-as-of; storing it with a candidate run is adopted on the owner's word of 2026-11-09 (Decision 13): the dataset column (12), the store change (13, an interface change of `store/candidate`) and the job side (14, which waits for `option-candidates.md` entry 7). The exclusion consumer stays Left out for the same reason.
2. A typo passes the bounds, no preview, no history, no revert, a write path in a read-only process: adopted. Bounds are realistic (Decision 3); a confirm step shows old to new, server-built (Decision 10); the page lists the last 20 history rows and a revert goes through the same confirm step; `--owner-edit` stays an explicit switch and the route is absent without it and without both `.env` lines (Decision 5).
3. Drop the HTTP write for a CLI: not adopted as the default, because plan v2.5 names a settings form; kept as Decision 1 b), "window default — owner may change", with what it drops.
4. Lock-in (twelve fixed keys, a v2 schema, a new role, a special sub-package): noted, no change; each is a named window default or a reviewed registry file (Decisions 2, 6, 7).
5. Host check breaks on `localhost` and `[::1]`; no ratchet file; `PriceLinesPage` is no POST precedent: adopted. The check accepts the three loopback names with the listen port (Decision 5); the 800-line text of AGENTS.md replaces the ratchet claim; `PriceLinesPage` is cited as a render precedent only and the POST handler is stated as new code in `site.go`.
6. Issue 9 shares files and misses tests that build `OT_PG_DSN_*`, fragile greps, 13 named tests: adopted. The settings DSN and key are optional (route absent when unset), so `backfill_test.go`, `runonce_test.go` and `runonce_option_test.go` need no change; the issue is split into 9 (GET) and 10 (POST) with the shared files named both ways; acceptance items name a symbol or a test that exists, no pattern greps beyond a pattern-free count.

Revision 5 addition (the move out of `internal/compute/api.go`; Decision 15). The full answer of a fresh read-only agent is the file `scratchpad/adv-compute-pkgs.md` of the drafting session (not in the repository; the pull request body gives its full path). Points 1 to 6:

1. Gap. Adopted: the open pull request of the old interface issue (PR 830 for issue 786) puts 98 lines into `internal/compute/api.go` and must be closed and its issue re-cut after this design lands (named in the pull request body). Not adopted here: five other designs (`option-candidates.md`, `option-ledger.md`, `option-notices.md`, `option-positions.md`, `home-page-revise.md`) also write to that file and `docs/design/ratchet.json` leaves 5 lines; each needs its own owner decision and is not changed here (named in the pull request body).
2. Risk. Adopted as named: the same four places of `docs/design/modules.md` are edited by `builders-wave.md` (PR 775); whichever lands second rebases and checks that all three packages are in the lists (`go run ./scripts/arch --check` catches a missing entry-cell name, not a missing list name).
3. Simpler way. Not adopted: one shared package for several designs; the owner chose one package per design (answer "a") and unrelated interfaces would share a package.
4. Lock-in. Accepted and named: consumers name the new package; `compute.X` is no longer the one place to look; the next interface that does not fit forces another package.
5. False claim. Adopted: line counts are stated with their source (835 on main, ceiling 840 in `docs/design/ratchet.json`); the check "`internal/compute/api.go` unchanged" is read against the merge base of the entry's own pull request.
6. Slip. Adopted: the three arch files of the interface entries are shared with the sibling entries; each Order line says the later pull request rebases and reruns `go run ./scripts/arch`; `.go-arch-lint.yml` is regenerated, never hand-edited.
