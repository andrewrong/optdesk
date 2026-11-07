# Option data page served: `ot serve --site` builds `option_data_page` per underlying and the web links to it

Author: the optdesk dispatcher (revision 1)
Owner approved: (pending)
Revision: 1

Source: owner request of 2026-11-07: frontend work runs in parallel with the backend, so the option data page must be reachable and
usable in the web without waiting for option collection. It is the `ot` wiring and the shell link that `docs/design/option-page.md`
(`## Left out`) put in a follow-up cut.
Read on origin/main at 3f2c13a (the one sha every fact below is read at).

Goal: `ot serve --site` serves `option_data_page` for a chosen underlying at `/options/data` and `/options/data/<id>`; the page links
to the other underlyings that have rows; the top bar of the shell has a link to it; the demo profile needs no database; and the chain
shows strike, expiry and right once the terms table has rows. Nothing waits for collection.

## What main holds today

- The builder exists: `option_data_page.NewPage(Config)` (`internal/pages/option_data_page/page.go`), `Config` is `RunID`, `BuildID`,
  `Scope`, `Zone`, `UnderlyingID`, `Label`, `Eod`, `Quotes`, `Notes` (`builder.go:37-48`). It writes `symbol`, `expiry` and `right` as
  null and `strike` as a `Gap` (`builder.go:171`); a read of an empty window gives a `partial` page with a gap, not an error. The fixtures
  `ok`, `terms`, `gap`, `stale` and `bad-version` exist in `contracts/fixtures/`; `terms.json` is the page with `symbol`, `expiry`,
  `right` and `strike` filled.
- The web view exists and is registered (`web/src/shell/pages.tsx`, key `option_data_page`); it shows no link and has no field for a
  choice of underlying. `web/src/shell/TopNav.tsx` has six fixed items (one `期权`, path `/options`, the carried `option_page`), and an
  item is active on an exact path match only.
- `cmd/ot/wire.go` has `pageList` (`macro_rates` only) and `macroPage`; `cmd/ot/pagelist.go` `pageNamed` names `macro_rates` and
  `macro_page`. `runServe` (`cmd/ot/serve.go`) serves one page for every path: `serveOn(ctx, ln, build, newHandler)` builds the view
  model per request and `embed.Handler` writes it into `<script id="vm">` whatever the path is (`internal/pages/embed/embed.go`). The
  page makes no request (`web/src/vm.ts`), so a choice of underlying is made by the server, on the route.
- Stores: `snapshot.OpenOptionEod`, `snapshot.OpenQuoteSnapshot`, `snapshot.OpenOptionEodCatalog` (`OptionEodLatest(ctx, since)`, one entry
  per underlying with its greatest `session_day`), `instrument.OpenOptionTerms` (`OptionTerms(ctx, ids)`, at most 500 ids, current
  versions only). Each has an `Open...Func` type in `internal/store/api.go`. The role `ot_ro_pages` has `SELECT` on `rd.option_eod_r1`,
  `rd.quote_snapshot_r1` and `rd.option_terms_r1` (migrations 0006, 0007, 0009), and `OpenOptionTerms` documents the line
  `OT_PG_DSN_PAGES`. `internal/store/snapshot`, `internal/store/instrument` and `internal/pages/option_data_page` are on the import list of
  the row `ot` in `docs/design/modules.md` already, so no arch file changes.
- `page.macro_rates.v1` is served by default (`--name` default `macro_rates`) and has no web view: no file of `web/src` or `web/test`
  names it, so the shell shows its hello page for that schema (`App.tsx` falls back to `Hello` on an unknown schema).
- The shell is a static page: a link is an anchor with a full load, which is what the embedded-vm model wants (the server answers each
  route with its own vm).

## Decisions

1. Route, not query. `ot serve --site` is a new mode with three routes: `/options/data` (the underlying with the greatest latest day in
   the catalog, or the empty page when the catalog is empty), `/options/data/<id>` (a positive integer id, else 404) and `/` (302 to
   `/options/data`). Any other path is 404. `--site` and `--name` together are an error. `--name` alone is unchanged, so the default
   `ot serve` keeps its behaviour and every existing test.
2. The links come from the page, not from the shell. The v1 schema gains one optional field `underlyings` (a list of `{id, label, href,
   current}`); Go fills it, the web renders it. `href` is written by `ot` (one route string in one Go file), so a later change of route
   touches no web file. The field is optional, so `ok.json`, `gap.json`, `stale.json` and `bad-version.json` are unchanged.
3. The label of an underlying is `Underlying <id>` (written by `ot`), because no store the page role reads holds a symbol of an underlying
   yet. `Config.Symbol` exists for the day a reader does (the terms fixture needs it too); `ot` leaves it empty.
4. The terms reader is wired now, because `strike`, `expiry` and `right` are what makes a chain readable and the reader, the grant and
   the `terms.json` fixture all exist. A nil `Config.Terms` keeps today's null and gap values.
5. `macro_rates` is left out of the site (reason in `## Left out`); it stays at `ot serve --name macro_rates`.
6. The demo profile (`OT_PROFILE` unset) serves the site from in-process demo readers and opens no store, like `demoReader` does for
   the series.

## Modules

No row of `docs/design/modules.md` is added or changed (and no arch file): every path below belongs to the first matching row as
listed in each entry, and the three packages `ot` imports are already on its list. No new Go or web dependency.

The interface entry is a licensed cross-module entry (contracts and pages), the second of its kind after `option-page-schema` of
`option-page.md`: the schema, its generated TS type and the Go `Config` fields that fill it are one contract, and the fields must
exist before the three entries that use them start in parallel. Its pages part is declarations and one file of types; no logic.

## Interface

Owner: contracts (schema, generated type, fixtures) with pages (`Config` and one type). Contracts line:
`page.option_data_page v1 (changed: optional underlyings)`.

Schema `contracts/schemas/page.option_data_page.v1.json` gains, and nothing else changes:

| field | type |
|---|---|
| `underlyings` | optional, list of `UnderlyingChoice` (no `maxItems`, see `option-page.md` on tuple size; the builder keeps at most 50) |

`UnderlyingChoice` is `{id: integer, label: text, href: string matching ^/[A-Za-z0-9/_-]{1,100}$, current: boolean}`,
`additionalProperties: false`. The `href` pattern forbids a scheme, so a payload cannot make the page link off site. The field is absent
when the builder was given no choices.

Go, `internal/pages/option_data_page`:

```go
// Choice is one underlying the page links to. Href is a path of this server; the builder checks it against the schema pattern.
type Choice struct {
	ID    int64
	Label string
	Href  string
}

// Config gains three fields; the zero values keep today's output byte for byte.
type Config struct {
	// ... the fields of today ...
	Symbol  string                  // empty: symbol is null
	Terms   store.OptionTermsReader // nil: expiry and right null, strike a no_data gap
	Choices []Choice                // empty: no underlyings field
}
```

Behaviour the builder entry implements (the interface entry only declares the fields and fixes the fixtures):

- `underlyings`: the first 50 choices in the given order; `current` is true for the choice whose `ID` equals `UnderlyingID`; a `Href`
  that does not match the pattern is an error of `Build`.
- terms: one `Terms.OptionTerms(ctx, ids)` call with the instrument ids of the kept rows (at most 500, the row cap); `Right` `C` and `P` give
  `call` and `put`; `ExpirationDate` gives `expiry`; `Strike` (decimal text) gives the `Figure` with 2 decimals under the price rule of
  `option-page.md`; a row without a current version keeps the null and gap values. A reader error is returned, never a half page.
  `Symbol` non-empty gives `underlying.symbol`.

Fixtures (in the interface entry, listed in `MANIFEST.synthetic-v1.json`): `page.option_data_page.v1.choices.json` is `ok.json` with an
`underlyings` list of three choices (ids 9000, 9010, 9020; labels `Underlying 9000`, ...; hrefs `/options/data/9000`, ...; `current`
true for 9000 only), and `page.option_data_page.v1.terms-input.json` holds the six terms records (`OptionTerms` fields) whose result is
`terms.json` (symbol `SYNTH` comes from `Config.Symbol`). The two builder goldens and the web tests read them.

## Issues

Which entries run when. The day this design merges: `option-serve-interface` only. As soon as it merges, `option-serve-builder`,
`option-serve-web` and `option-serve-ot` start together (width 3, disjoint modules, each buildable from the interface alone). The `ot`
tests use a fake page (a recording fake that sees the `Config`), the web tests read the fixtures, the builder tests use fake readers:
no entry reads the output of another. What is shown end to end in the browser needs all four merged and is the owner's check, not an item.

Items under `Checked by CI` are decided by the workflows of the head commit; every other item is a fact read from the diff, with an
offline grep, `test -f` or `wc -l`.

### option-serve-interface (interface)

The optional `underlyings` field, its generated type, the `Config` fields and the two fixtures
  Goal: schema, generated type, fixtures and the declared Go fields exist; every existing contract test and fixture stays unchanged.
  Files: contracts/schemas/page.option_data_page.v1.json, web/src/generated/option_data_page.v1.ts, contracts/fixtures/page.option_data_page.v1.choices.json, contracts/fixtures/page.option_data_page.v1.terms-input.json, contracts/fixtures/MANIFEST.synthetic-v1.json, internal/contracts/optiondatapage_test.go, docs/contracts/page-schemas.md, internal/pages/option_data_page/choice.go, internal/pages/option_data_page/builder.go
  Design: docs/design/option-page-serve.md#interface
  Contracts: page.option_data_page v1 (changed: optional underlyings)
  Acceptance:
  1. The schema has `underlyings` and `UnderlyingChoice` as in `## Interface`, `underlyings` is not in `required`, `additionalProperties: false` on the new def, the `href` pattern is exactly the one above; `wc -l` of the schema is at most 420 and `grep -c maxItems` still prints 1.
  2. `web/src/generated/option_data_page.v1.ts` is the output of `pnpm --dir web run generate:types` (banner kept); `wc -l` of it is at most 400; `git diff --stat` shows `web/package.json` and `pnpm-lock.yaml` absent.
  3. `ok.json`, `gap.json`, `stale.json`, `terms.json` and `bad-version.json` are byte-identical to the base (`git diff --stat` does not list them); `MANIFEST.synthetic-v1.json` gains the sha256 of the two new files only.
  4. `choices.json` validates, has three `underlyings` with the ids and `current` of `## Interface`, and no other difference from `ok.json`; `terms-input.json` holds six records for the instrument ids of `terms.json` with `Right` C and P, the expiries and the strikes of `terms.json`. Ids are 9000 and above; no symbol of a real company.
  5. `TestOptionDataPageFixtures` validates `choices.json` too, and a case with `href` `https://x/y` (an inline mutation of `choices.json` in the test) fails at the `href` keyword. `docs/contracts/page-schemas.md` names the new field.
  6. `choice.go` declares `Choice` and nothing else (no function); `builder.go` gains the three `Config` fields with the comments of `## Interface` and no other line changes (`git diff internal/pages/option_data_page/builder.go` shows added lines in the `Config` struct only).
  7. No file outside the nine listed is in the diff.
  Checked by CI: ci-go (`go build ./...`, `go test ./internal/contracts/... ./internal/pages/...`, hygiene scan), ci-web `tsc`, `eslint` and the type drift step; the PR body carries `Contracts: page.option_data_page v1 (changed: optional underlyings)` from the opening; the design verdict (an interface change).
  Depends on: none (merged work only).
  Size: M

### option-serve-builder

The builder fills `underlyings`, the terms and the symbol
  Goal: with the new `Config` fields set, `Build` writes the pages `choices.json` and `terms.json` from fake readers; with them unset the output is the same as today.
  Files: internal/pages/option_data_page/builder.go, internal/pages/option_data_page/terms.go, internal/pages/option_data_page/choices.go, internal/pages/option_data_page/terms_test.go, internal/pages/option_data_page/choices_test.go, docs/contracts/pages.md
  Design: docs/design/option-page-serve.md#interface
  Behaviour: as the two bullets of `## Interface`. Terms are asked once, after the rows are chosen and capped; the id list is the kept rows' instrument ids in row order.
  Acceptance:
  1. `TestBuildChoicesGolden`: the input of `page.option_data_page.v1.input.json` with `Choices` of the three entries builds a page whose decoded JSON equals the decoded `choices.json`; `TestBuildTermsGolden`: the same input, a fake `Terms` returning `terms-input.json` and `Symbol` `SYNTH` equals `terms.json`. Neither test writes a file.
  2. `TestBuildNoNewFields`: with `Choices`, `Terms` and `Symbol` zero, the output decodes equal to `ok.json` and has no `underlyings` key.
  3. `TestBuildChoices`: 60 choices give 50; a bad `Href` (`https://x`, `javascript:x`, empty, with a space) is an error and returns no bytes; `current` is true for the `UnderlyingID` choice only and for none when no choice matches.
  4. `TestBuildTerms`: the fake sees one call with the kept ids in row order and never more than 500; a row without a terms record keeps null `expiry` and `right` and the `no_data` strike; a terms error is returned and no bytes; an empty row set makes no terms call.
  5. Every test calls `t.Parallel()`; `grep -c 'time.Sleep\|time.Now' internal/pages/option_data_page/*.go` prints 0; no zone literal and no `pgx`; among `internal/...` imports only `internal/pages`, `internal/contracts` and `internal/store` appear; each new file stays under 600 lines and `builder.go` stays under 800.
  6. `docs/contracts/pages.md` names the three fields and the two tests in its version history; no file outside the six listed is in the diff.
  Checked by CI: ci-go (`go build ./...`, `go test ./internal/pages/...`, golangci-lint, go-arch-lint).
  Depends on: option-serve-interface (merged).
  Size: M

### option-serve-web

The web shows the links to the other underlyings and the top bar links to the page
  Goal: the page renders `underlyings` as links, the top bar has the item, and both are tested on the fixtures; no fetch, no computation.
  Files: web/src/pages/optiondata/UnderlyingLinks.tsx, web/src/pages/optiondata/OptionDataPage.tsx, web/src/shell/TopNav.tsx, web/test/pages/optiondata/UnderlyingLinks.test.tsx, web/test/pages/optiondata/OptionDataPage.test.tsx, web/test/shell/TopNav.test.tsx
  Design: docs/design/option-page-serve.md#interface
  UI parts (frontend-stack, ADR 0034; no package is added, `frontend-whitelist.json` is unchanged):
  - 按 frontend-stack：用 shadcn `Card`, `CardHeader`, `CardTitle` 做 the frame of the underlying choices, placed above the chain card.
  - 按 frontend-stack：用 shadcn `Button` (`asChild`, `variant` `secondary` for the current one and `ghost` for the others) 做 one anchor per `underlyings` item, with `aria-current="page"` on the current one; the anchor text is `label` and the target is `href` as given.
  - 按 frontend-stack：用 shadcn `Button` (`asChild`, as the six items of `TopNav` do) 做 the new top bar item `期权数据` with path `/options/data`.
  - 按 frontend-stack：用 shadcn `Badge` 做 the `id` text beside the label, so two underlyings with one label stay apart.
  - 按 frontend-stack：用 vitest and testing-library 做 the tests; zustand and recharts are not touched.
  Acceptance:
  1. `UnderlyingLinks` is rendered by `OptionDataPage` only when `payload.underlyings` is present; it writes `label` and `href` as given and keeps the order of the payload. Offline: `grep -n -E 'toFixed|toLocale|Intl\.|Math\.|\.sort\(|fetch\(|XMLHttpRequest' web/src/pages/optiondata/*.tsx` prints nothing; `web/no-compute-allow.txt` is unchanged.
  2. `TopNav.tsx` gains the seventh item `期权数据` at `/options/data` and a per-item flag `prefix` set on that item only; an item is active on an exact match, and an item with `prefix` also when the path starts with its path and a slash; so `/options/data/9001` marks `期权数据` and not `期权`, and `/options` marks `期权` only.
  3. `TopNav.test.tsx` expects seven links with the fixed labels and paths, the active mark for `/options/data`, `/options/data/9001` and `/options`, and the old six cases unchanged.
  4. `UnderlyingLinks.test.tsx` and `OptionDataPage.test.tsx` read `choices.json` from `contracts/fixtures/`: three anchors with the hrefs of the fixture, `aria-current` on the one with `current` true only; `ok.json` shows no choices block; `validate` rejects a payload whose `href` is not a string and still accepts `ok.json`.
  5. No test uses network, clock or sleeps (`grep -c 'setTimeout\|sleep'` over the three test files prints 0); each new file stays under 600 lines; `git diff --stat` shows `web/package.json`, `pnpm-lock.yaml` and `web/src/generated` absent.
  6. No file outside the six listed is in the diff.
  Checked by CI: ci-web `tsc`, `eslint`, `vitest`, the build, and the type drift step.
  Depends on: option-serve-interface (merged: the generated type has `underlyings`).
  Size: M

### option-serve-ot

`ot serve --site` serves the option data page per underlying, from the stores or from demo readers
  Goal: the three routes of `## Decisions`, built per request, on the stores opened with `OT_PG_DSN_PAGES` or on demo readers; the existing `serve` and `page` behaviour is unchanged.
  Files: cmd/ot/site.go, cmd/ot/site_test.go, cmd/ot/demo_option.go, cmd/ot/serve.go, cmd/ot/wire.go, docs/contracts/ot.md
  Design: docs/design/option-page-serve.md#decisions
  Behaviour:
  - `runServe` gains `--site` (bool). With `--name` given as well it is an error that names both flags. Without `--site` the function is as today.
  - `serveOn` is split into `serveHandler(ctx, ln, h http.Handler)` (the listen, shutdown and error part) and `serveOn`, which calls it; `serveOn` keeps its signature and behaviour.
  - `site.go` holds the handler: `/` answers 302 to `/options/data`; `/options/data` and `/options/data/<id>` build per request like `serveOn` (the day from `--day` or the clock, `generatedAt` from the clock in UTC, a build or catalog error answers 500 with the fixed text and logs `component=ot status=failed` only); an id that is not a positive integer, a longer path, or another method than GET and HEAD answers 404 or 405.
  - Per request: `OptionEodLatest(ctx, since)` with `since` the day 7 days before the build day (the window the builder reads, a comment names that); `/options/data` takes the entry with the greatest `SessionDay`, lowest id on a tie, or id 0 when none; the `Choices` are all entries in catalog order, `Label` `Underlying <id>`, `Href` `/options/data/<id>`.
  - `wire.go` gains `deps` fields `openOptionEod`, `openQuoteSnapshot`, `openOptionCatalog`, `openOptionTerms` (the `store.Open...Func` types, real values from `snapshot` and `instrument`) and `optionDataPage func(...) pages.Page` that fills `Config` (`RunID` `ot-page`, `Scope` `option`, `BuildID` from `buildIDOf(version)`, `Zone` of the profile, `Symbol` empty) and calls `NewPage`; only `wire.go` imports the implementation package.
  - The four stores are opened once at start with the `.env` line `OT_PG_DSN_PAGES` only (a missing line is an error that names the variable and no value) and closed on exit; with the demo profile none is opened and `demo_option.go` supplies in-process readers: a catalog of two underlyings, eod rows and quote rows for each (session day taken from the requested `to`), and terms for the eod instruments.
  Acceptance:
  1. `grep -c 'OT_PG_DSN\|DSNEnv' cmd/ot/site.go` prints at most 1 and that use is `store.DSNEnvPages`; no other DSN name appears in the added lines; `git diff cmd/ot/wire.go` shows no import added other than `snapshot`, `instrument` (if not yet imported) and `option_data_page`.
  2. `TestSiteRoutes` (`httptest`, fake `optionDataPage` that records the `Config` and returns a fixed page): `/options/data/9010` builds `UnderlyingID` 9010; `/options/data` builds the greatest-day entry (tie to the lowest id) and id 0 on an empty catalog; `/` is 302 to `/options/data`; `/options/data/0`, `/options/data/-1`, `/options/data/x`, `/options/data/1/2` and `/nope` are 404; POST is 405; a catalog error and a build error are 500 with the fixed text and the response holds no error text.
  3. `TestSiteChoices`: the recorded `Config.Choices` list every catalog entry in order with the labels and hrefs above, and the catalog was asked with `since` seven days before the build day.
  4. `TestServeSiteFlags`: `--site --name macro_page` is an error naming both flags; `--name option_data_page` is still an unknown page; every test of `serve_test.go` passes unchanged (that file is not in the diff).
  5. `TestSiteDemoOpensNoStore`: with the demo profile and fake openers that fail the test when called, `/options/data` answers 200 with HTML whose `vm` JSON has `"schema":"option_data_page"`; `TestSiteMissingDSN`: a real profile without the line fails before listening with the variable name in the error and no DSN text.
  6. `docs/contracts/ot.md` names `serve --site`, the routes, and `OT_PG_DSN_PAGES` for the four stores.
  7. Every test calls `t.Parallel()`, uses `httptest` or a listener on port 0, `t.TempDir()` for files, and no sleep; each new file stays under 600 lines; no file outside the six listed is in the diff; `go run ./scripts/arch --check` exits 0.
  Checked by CI: ci-go (`go build ./...`, `go test ./cmd/ot/...`, golangci-lint, go-arch-lint).
  Depends on: option-serve-interface (merged: `Choice` and the `Config` fields). Needs no output of the builder: its tests use a fake page.
  Size: L

## Left out

- `macro_rates` in the site: it has no web view, so a link to it would show the hello page of the shell; a web view for it is its own
  design (a schema-bound page with a table or chart, ADR 0034). It stays reachable as `ot serve --name macro_rates`, the default of `--name`.
  `macro_page` has a view but no owner request here; adding `/macro` is one route and one test in a later cut.
- `ot page --name option_data_page` and a `--underlying` flag: not needed to see the page, and `pageNamed` has no underlying parameter.
- A symbol or name for an underlying: `Config.Symbol` is ready; the reader (instrument store or a catalog column) is the collection design's.
- The other items of the top bar (`/home`, `/household`, ...) are routes of an older listener; `ot serve --site` answers 404 for them.
  Making them live, a single server for all pages, and retiring the carried `option_page` and its `期权` item are left to later designs.
- A catalog or window longer than 7 days: a choice older than the window never appears (that is the builder's window, see
  `option-page.md`); a longer window needs the builder and the catalog `since` to change together.
- Caching, authentication, TLS and the listen address of the site: `serve` is a local tool bound as today.
- Any collection job, broker or schedule: the option-jobs design.

## Adversarial

1. Gap against the goal: adopted in part. The goal says usable without waiting for collection; the demo readers give a populated page with
   no database, the empty real store gives the empty page with its gap text, and terms are wired so that real rows are readable the day
   they exist. Not adopted: a browser check in the items (not offline); it is named as the owner's check after all four merge.
2. Biggest unnamed risk: the licensed cross-module interface entry (contracts and pages) is refused by the issue check or the design
   review, as it edits the `Config` struct of a module that owns it. Adopted as a named fallback: if refused, cut it into a contracts entry
   and a pages entry, the `ot` entry then waits for the pages entry. Not done now, because the precedent of `option-page.md` was merged and the
   split costs one serial step. Second risk, adopted: the schema was 366 lines under a 400 line bound; the new bound is 420, measured.
3. A simpler way: add `option_data_page` to `pageList` with an `--underlying` flag and one nav link. Not adopted: one process would show one
   underlying, the nav link would need the flag at start, and the owner asked for selection in the web. The simplest part adopted: no new
   route table beyond three paths, and `--site` is a flag, so the default `ot serve` is untouched.
4. Lock-in: the route `/options/data/<id>` and the `underlyings` field. Adopted in part: `href` is written by Go, so a route change is one
   Go file; the field is optional in v1, so no version bump. Not mitigated: the label `Underlying <id>` and the `期权数据` item are visible
   text that the owner may rename; both are one literal.
5. Claim most likely false: that `OpenOptionTerms` works with the `OT_PG_DSN_PAGES` line against a real database. Read, not run: the grant
   is in migration 0009 and the opener's comment names the line, but no `pg` test was run here and none runs in this design. The `ot` entry
   does not depend on it offline (fake openers); a wrong grant would show only on the owner's first real run. Also unverified: that the
   fixture `terms.json` is reproducible from `terms-input.json` plus `Symbol` without more fields (the interface entry's item 4 and the
   builder's golden are where that is found).
6. Likeliest cause of a slip: the interface entry, since it touches the schema, the generated file, the fixtures manifest and `builder.go`
   at once, and the drift step runs only in CI. Adopted: the item list names each file and the byte-identical fixtures, and the generated file
   comes from the command CI runs. Second: the 7 day window is written in `ot` as well as in the builder; not removed (the builder does not
   export it), named in a comment and in `## Left out`.
