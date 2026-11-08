# One site: `ot serve` alone serves every page of the top bar, live where a source exists and sample data elsewhere

Author: the optdesk designer (draft, revision 2 after the adversarial review `unified-site-adversarial.md` of the scratchpad)
Owner approved: (pending)
Revision: 2

Source: owner request of 2026-11-08: the front-end pages are one site; macro_rates should have a web view; every page of the top bar is in the site, a page without
a live source shows sample data marked as such; the look stays (owner saw a local preview); trade-offs are the dispatcher's. It takes up `## Left out` of
`docs/design/option-page-serve.md` ("a single server for all pages", "a web view for macro_rates is its own design").
Read on main at 7326175: AGENTS.md, docs/design/modules.md, option-page-serve.md, macro-rates-view.md, macro-page-wiring.md, docs/contracts/ot.md,
cmd/ot/{serve,site,page,pagelist,wire,demo}.go, cmd/ot/{serve,site}_test.go, internal/pages/embed/{embed,inject,inject_test}.go, internal/pages/embed/dist/index.html,
internal/pages/macro_rates/*.go, internal/sources/fred/mapping.go, scripts/ratchet/main.go, web/vite.config.ts, web/package.json, web/src/shell/*, web/src/types/macro_rates.ts,
web/src/pages/macro_rates/view.tsx, web/fixtures/, web/test/pages/**/fixtures, contracts/schemas/page.macro_rates.v2.json, internal/contracts/macrorates_v2_test.go.

Goal: `ot serve` with no page flag serves every item of the top bar from the real web build. A page with a live source shows live data (`/macro`, with all seven stored FRED
series, and `/options/data[/<id>]`); a page without one (`/home`, `/household`, `/positions`, `/risk`, `/options`, `/advice`) shows sample data under one fixed banner.
One bar of eight items, no page hidden, no `--site` and no `--name` on `serve`.

## What main holds today

- The embedded page is a stub. `internal/pages/embed/dist/index.html` (committed since #42) is six lines: `<div id="root">` and an empty vm element, no script, no `#fail-closed`.
  `embed.go` is `//go:embed dist/index.html`, and `Handler` answers that one file for every path. Nothing copies `vite build` output into it: `web/vite.config.ts` has no `outDir`
  (output goes to `web/dist`, with absolute `/assets/...` URLs, ADR 0034), `pnpm build` is `tsc --noEmit && vite build`. So `ot serve` and `ot serve --site` answer a blank page today, and a
  request for `/assets/x.js` would get the same html. The option data page of `option-page-serve.md` has never been seen in a browser through `ot`.
- The macro rates view exists: `web/src/pages/macro_rates/view.tsx` registers `macro_rates` (v2 types, test `web/test/pages/macro_rates/view.test.tsx`): per series a card with the title,
  the latest text (or a gap alert), a recharts line through the shadcn chart and a tanstack table; above them title, sentence and one alert per gap. It computes nothing.
- The builder reads **one** series, `DGS10` (`seriesIDs`), 30 days, with stale after 4 days. DGS3MO, SP500, VIXCLS, VXNCLS, RVXCLS and VXVCLS are stored (`ot series-load`) and not read.
  The unit of a series is the stored unit of its points (`builder.go:101`). `internal/sources/fred/mapping.go` stores DGS10 and DGS3MO as `percent` and the other five as `index_points`;
  the v2 `unit` enum is `percent | ratio | ccy_amount | count`, so v2 cannot carry five of the seven.
- `macro_page` reads one indicator (DGS10) plus two permanent gaps. `ot serve` has `--name` and `--site`; `TopNav.tsx` has seven items; `/home`, `/household`, `/positions`, `/risk`, `/options`,
  `/advice` have views and schemas (home_page v2, household_page v1, positions_page v3, household_risk v1, option_page v1, advice_page v1) but no Go source (`home_page` has a composer only).
- `web/fixtures/` holds synthetic JSON per page (`household-page`, `positions-page`, `option-page`, `advice-page`, each with `ok.json`); `home_page` and `household_risk` have their fixtures as TS in
  `web/test/pages` (`home/home-v2/fixtures/ok.ts`, `risk/fixtures/ok.ts`). The size rule (`scripts/ratchet`) counts `.go .py .ts .tsx .js .mjs .sh` files only, so JSON data files are exempt; `.ts` data is not.
- `web/src/shell/mount.tsx` takes a render function; `App` is built in `web/src/main.tsx` and picks the view by `payload.schema`; `placeholderValidate` checks only an object and `schema_version`.

## Decisions

1. One command, no mode flag. `ot serve [--day DAY] [--addr ADDR]` serves the whole site; `--site` and `--name` are removed from `serve` (an unknown-flag error). `ot page --name macro_rates|macro_page` stays
   (it prints one page; it is not the site). Run it as `pnpm --dir web build` then `ot serve`.
2. The built web bundle is what `ot serve` serves. Vite writes to `internal/pages/embed/dist` (`build.outDir`, `emptyOutDir: false` so the guard file below survives; stale hashed files may pile up and are ignored by git).
   The pages module owns `dist`: it embeds `all:dist`, answers `/assets/*` from it (`Assets()`), writes the vm into `dist/index.html` for page routes, and answers 500 `web bundle not built: run pnpm --dir web build`
   when `index.html` is absent. The built files are not committed: `internal/pages/embed/dist/.gitignore` (`*` and `!.gitignore`) is the only committed file there, and the stub `index.html` is deleted. Generated
   bundles are not committed (unlike `contracts/` output) because hashed files would change on every web commit; the CI `build` step of ci-web already produces them. `go build` works without a bundle (the guard file keeps the embed pattern non-empty).
3. Routes: live `/macro`, `/options/data`, `/options/data/<id>` (rules of `option-page-serve.md`); sample `/home`, `/household`, `/positions`, `/risk`, `/options`, `/advice`; `/assets/*` the bundle; `/` is 302 to `/home`
   (the first bar item, kept). Any other path is 404, a method other than GET and HEAD is 405. The bar in order: `首页` /home, `宏观` /macro, `家庭资产` /household, `持仓` /positions, `风险` /risk, `期权` /options,
   `期权数据` /options/data, `建议` /advice.
4. No page is hidden. Sample pages: the web module carries the sample payloads and the server only marks the route as sample. The vm of a sample route is `{"schema":"sample_page","schema_version":1,"page":"<schema>"}`
   (written by `ot` as one constant per route; schema `page.sample_page.v1`, contracts). The shell, seeing `sample_page`, takes the payload of `page` from `web/src/samples/index.ts`, shows it through the normal registry and shows one
   fixed banner `示例数据，非实时数据 / Sample data, not live data` (a shell literal, never a payload field). So pages and `ot` write no sample payload, no element and no Go sample function. Cost: the samples go into the one bundle
   (about 4000 lines of JSON, no code); accepted for a local tool.
5. The sample files are existing data: `web/fixtures/{household-page,positions-page,option-page,advice-page}/ok.json` are imported as they are; two are added, `web/fixtures/home-page/ok.json` and
   `web/fixtures/household-risk/ok.json`, written from the TS `ok` fixtures with the `href` of the home links limited to bar routes (`/macro`, `/household`, `/options`) and `report_links` empty, so no link from a sample page
   leaves the site. JSON data is exempt from the size rule (checked in `scripts/ratchet`); the new `.ts` files stay small. A vitest renders every sample through the real `App` (the validator checks only the version).
6. v3 of macro_rates is the interface entry. `page.macro_rates.v3` is v2 with `schema_version` 3 and `unit` gaining `index_points`; it is the schema the builder writes for all seven series. Titles are text, so no field is added: the
   builder writes `10-year Treasury yield`, `3-month Treasury yield`, `S&P 500`, `VIX`, `VXN (Nasdaq-100 volatility)`, `RVX (Russell 2000 volatility)`, `VXV (3-month volatility)` for the seven ids, pinned by the golden
   fixtures. `index_points` text has two decimals and no suffix. v2 stays on disk, retired by never being read.
7. macro_page and macro_rates are folded into one route, `/macro` (macro_rates): same reader, macro_page's row is the `latest` of the first card, its other blocks are permanent gaps. macro_page leaves the site (its Chinese
   indicator name is not shown); retiring its code is a later cut. Series order: DGS10, DGS3MO, SP500, VIXCLS, VXNCLS, RVXCLS, VXVCLS. All seven are read; the demo reader gives points for all seven with the stored units (`percent` for the two
   rates, `index_points` for the five indices).
8. The site opens six stores once at start with the one line `OT_PG_DSN_PAGES` (the series store and the five option stores; a missing line names the variable, never a value). The demo profile and the sample routes open none.
9. The look stays (owner, after a local preview): shell, pages and styles are not edited; the macro view is reused with its types moved to v3. At each fork the simplest option: no collapse or grouping of the seven cards, a banner from the
   existing Alert, no new library (recharts, shadcn chart, @tanstack/react-table, shadcn Alert are in `web/`, ADR 0034). `web/src/main.tsx` and `mount.tsx` do not change: the shell decides the sample inside `App`.

## Modules

none. No row of `docs/design/modules.md` changes and no arch file: `cmd/ot` already imports `internal/pages/embed` and `internal/pages/macro_rates`. Entries: `contracts` (interface), `pages`, `web`, `ot`.

## Interface

Owner: contracts. Interface file(s): `contracts/schemas/page.macro_rates.v3.json`, `contracts/schemas/page.sample_page.v1.json` and their generated types `web/src/generated/macro_rates.v3.ts`, `web/src/generated/sample_page.v1.ts`.
Contracts line: `page.macro_rates v3 (new), page.sample_page v1 (new)`. Every cross-module contract of this design:

| contract | owner | writers and readers |
|---|---|---|
| `page.macro_rates.v3` (v2 plus `index_points`, `schema_version` 3; fixtures golden, gap, stale, bad-version) | contracts | `pages/macro_rates` writes; `web` reads (type, validator) |
| `page.sample_page.v1`: `{schema: "sample_page", schema_version: 1, page: enum home_page, household_page, positions_page, household_risk, option_page, advice_page}`, all required, closed | contracts | `ot` writes (one constant per sample route); `web` reads |
| `embed.Assets() http.Handler` (serves `/assets/*` of the bundle; 404 otherwise) and `embed.Handler(vm) http.Handler` (unchanged signature; now serves the built `index.html`, 500 when absent); `embed` is an implementation package, so these are declared in `docs/contracts/pages.md`, not an interface file | pages | `ot` calls both, only in `cmd/ot/wire.go` |
| the bundle path `internal/pages/embed/dist` (Vite `outDir`) | pages (dist) and web (config) | web builds into it, pages embeds it |

`pages.NewHandlerFunc`, `pages.Page` and `store.SeriesReader` are unchanged. Fixtures: `page.sample_page.v1.ok.json` (page `home_page`) and `page.sample_page.v1.bad-version.json`; the `ot` test builds each of the six constants and
compares the decoded result with the fixture's shape (the page name replaced).

## Issues

Order: `site-interface` first. After it merges, `site-pages` and `site-web` start together; `site-ot-serve` starts after `site-pages` (its `wire.go` calls `embed.Assets`). The pages tests use fake readers and an in-memory file system, the web
tests render the shell and read the fixtures, the `ot` tests use fake page, assets and readers. What is seen in the browser (blank page gone, eight pages, banner, seven cards) needs all four merged and is the owner's check, not an item.
Items under `Checked by CI` are decided by the workflows of the head commit; every other item is a fact read from the diff with an offline grep, `test -f` or `wc -l`.

### site-interface (interface)

The v3 schema of macro_rates and the schema of the sample marker, with generated types and fixtures
  Module: contracts
  Goal: both schemas, their generated TS types, fixtures and tests exist; nothing else changes.
  Files: contracts/schemas/page.macro_rates.v3.json, contracts/schemas/page.sample_page.v1.json, web/src/generated/macro_rates.v3.ts, web/src/generated/sample_page.v1.ts, contracts/fixtures/page.macro_rates.v3.golden.json, contracts/fixtures/page.macro_rates.v3.gap.json, contracts/fixtures/page.macro_rates.v3.stale.json, contracts/fixtures/page.macro_rates.v3.bad-version.json, contracts/fixtures/page.sample_page.v1.ok.json, contracts/fixtures/page.sample_page.v1.bad-version.json, contracts/fixtures/MANIFEST.synthetic-v1.json, internal/contracts/macrorates_v3_test.go, internal/contracts/samplepage_v1_test.go, internal/contracts/generated_ts_test.go, internal/contracts/carryover_test.go, docs/contracts/page-schemas.md
  Design: docs/design/unified-site.md#interface
  Contracts: page.macro_rates v3 (new), page.sample_page v1 (new)
  Depends on: none (merged work only).
  Acceptance:
  1. The v3 schema differs from `page.macro_rates.v2.json` only in `schema_version` const 3, the `unit` enum (`percent`, `ratio`, `ccy_amount`, `count`, `index_points`) and the schema id/title; `git diff --no-index contracts/schemas/page.macro_rates.v2.json contracts/schemas/page.macro_rates.v3.json | grep -c '^[+-] '` prints at most 8.
  2. `page.sample_page.v1.json` is closed (`additionalProperties: false` at the top), requires its three fields, has `schema` const `sample_page`, `schema_version` const 1 and the six-name enum of `## Interface` in that order.
  3. `TestMacroRatesV3Fixtures` validates golden, gap, stale (a series with unit `index_points` in golden) and rejects bad-version at `schema_version`; the golden holds the seven series ids in the order of Decision 7 with the seven titles of Decision 6; `TestSamplePageV1Fixtures` validates ok, rejects bad-version, rejects a page name outside the enum and an extra field.
  4. `web/src/generated/macro_rates.v3.ts` and `sample_page.v1.ts` are the output of `pnpm --dir web run generate:types` (banner kept); `git diff --stat` shows `web/package.json` and `pnpm-lock.yaml` absent and every v2 file and fixture byte-identical to base.
  5. `MANIFEST.synthetic-v1.json` gains the sha256 of the six new fixtures only; `docs/contracts/page-schemas.md` names both schemas and the field `index_points`; no file outside the sixteen listed is in the diff.
  Checked by CI: ci-go (`go build ./...`, `go test ./internal/contracts/...`), ci-web `tsc`, `eslint` and the type drift step; the PR body carries `Contracts: page.macro_rates v3 (new), page.sample_page v1 (new)` from the opening; the design verdict (an interface change).
  Size: M

### site-pages

macro_rates reads seven series as v3; embed serves the real web build
  Module: pages
  Goal: `Build` writes the v3 page for seven series; `Handler` and `Assets` serve the built bundle; no stub remains.
  Files: internal/pages/macro_rates/builder.go, internal/pages/macro_rates/text.go, internal/pages/macro_rates/page.go, internal/pages/macro_rates/builder_test.go, internal/pages/macro_rates/page_test.go, internal/pages/embed/embed.go, internal/pages/embed/inject_test.go, internal/pages/embed/assets_test.go, internal/pages/embed/dist/.gitignore, internal/pages/embed/dist/index.html, docs/contracts/pages.md
  Design: docs/design/unified-site.md#decisions
  Depends on: site-interface (merged: the v3 schema and fixtures).
  Acceptance:
  1. `grep -c 'DGS10", "DGS3MO", "SP500", "VIXCLS", "VXNCLS", "RVXCLS", "VXVCLS"' internal/pages/macro_rates/builder.go` prints 1; `Build` keeps its signature; `build(ctx, r, ids, day, generatedAt)` is unexported, `Build` calls it with the seven ids; `Page()` names `page.macro_rates.v3`; `SchemaVersion` is 3; titles are those of Decision 6; `index_points` text has two decimals and no suffix.
  2. Tests that change, by name: `TestBuildGolden`, `TestBuildFixtures`, `TestGapsAndStale` and the helper at `builder_test.go:166` and `:249` call `build(..., []string{"DGS10"})` and read the `page.macro_rates.v3.*` fixtures; `TestBuildDemo` and `TestPage` (and `page_test.go:22-26`, `p.Build` against `Build`) expect seven series; `TestText` gains the `index_points` case; `TestBuildErrors` is unchanged in meaning. New `TestBuildSevenSeries`: seven series in order, sentence `7 series on 2026-11-04.`, units equal to the stored units, and the output validates against `page.macro_rates.v3.json`; with only DGS10 stored, six series are `no_data` with `no points` and six `stale` gaps; a reader error on any id returns no bytes.
  3. `embed.go` reads `//go:embed all:dist`; `Handler(vm)` writes the vm into `dist/index.html`; `TestHandlerServesIndex` (replaces `TestHandlerServesStub`) and `TestHandlerBadViewModel` run on an in-memory `fstest.MapFS` through an unexported `newHandler(fsys, vm)`; `TestHandlerNoBundle500` (no `index.html`: 500, the fixed text, no path in the body); `TestAssets`: `/assets/a.js` answers the file with a JavaScript type, `/assets/missing` and `/index.html` and `/assets/../x` are 404, POST is 405.
  4. `git ls-files internal/pages/embed/dist` prints exactly `internal/pages/embed/dist/.gitignore` and the file holds `*` and `!.gitignore`.
  5. Every test calls `t.Parallel()`; `grep -c 'time.Sleep\|time.Now' internal/pages/embed/*.go internal/pages/macro_rates/*.go` prints 0; among `internal/...` imports only `internal/contracts` and `internal/store` appear; `docs/contracts/pages.md` names `Assets`, the bundle path, the 500 text and `page.macro_rates.v3`; no file outside the eleven listed is in the diff.
  Checked by CI: ci-go (`go build ./...` with no bundle, `go test ./internal/pages/...`, golangci-lint, go-arch-lint).
  Size: M

### site-web

The bar, the sample pages with their banner, the v3 types and the build output path
  Module: web
  Goal: `TopNav` has eight items; a `sample_page` payload shows the named page from the carried sample with the banner; the macro view reads v3; `pnpm build` writes into the embed directory.
  Files: web/src/shell/TopNav.tsx, web/src/shell/App.tsx, web/src/shell/SampleBanner.tsx, web/src/samples/index.ts, web/fixtures/home-page/ok.json, web/fixtures/household-risk/ok.json, web/fixtures/MANIFEST.synthetic-v1.json, web/src/types/macro_rates.ts, web/src/types/sample_page.ts, web/vite.config.ts, web/test/shell/TopNav.test.tsx, web/test/shell/sample.test.tsx, web/test/pages/macro_rates/view.test.tsx
  Design: docs/design/unified-site.md#decisions
  UI parts (frontend-stack, ADR 0034; no package is added, `frontend-whitelist.json` is unchanged; no page view is edited):
  - 按 frontend-stack：用 shadcn `Button` (`asChild`, `variant` `secondary` for the current item and `ghost` for the others) 做 the eight anchors of the bar, `aria-current="page"` on the current one.
  - 按 frontend-stack：用 shadcn `Alert` and `AlertDescription` (lucide `FlaskConical`) 做 the banner `示例数据，非实时数据 / Sample data, not live data`, `role="status"`, between `TopNav` and `main`.
  - 按 frontend-stack：用 recharts through the shadcn chart (`Chart`) and @tanstack/react-table through the shadcn data-table (`DataTable`) 做 the seven macro cards, unchanged in `view.tsx`; only its types move to v3.
  - 按 frontend-stack：用 vitest and testing-library 做 the tests; zustand is not touched.
  Depends on: site-interface (merged: the generated v3 and sample types).
  Acceptance:
  1. `TopNav.tsx` names exactly the eight paths of Decision 3 in order; `/options/data/9001` marks `期权数据` only, `/options` `期权` only, `/` `首页`, `/macro/` `宏观`, `/structure` nothing; `TopNav.test.tsx` (its two existing tests change: seven links become eight, and the marks) expects these.
  2. `web/src/samples/index.ts` maps the six names of `page.sample_page.v1` to a JSON file: the four existing `ok.json` of `web/fixtures` and the two new ones; `home-page/ok.json` has `block_links` hrefs within `/macro`, `/household`, `/options`, `report_links` `[]`, and every other field equal to the `ok` of `web/test/pages/home/home-v2/fixtures/ok.ts` (a test compares); `household-risk/ok.json` equals the `ok` of `web/test/pages/risk/fixtures/ok.ts`; `MANIFEST.synthetic-v1.json` lists the two files.
  3. `sample.test.tsx`: for each of the six names, `App` given `{"schema":"sample_page","schema_version":1,"page":<name>}` renders the page (a text of its fixture) and the banner once, after the nav; given the same page's own payload it shows no banner; an unknown `page` or version 2 throws (fail-closed notice); no `href` in the six samples points outside `/home /macro /household /positions /risk /options /options/data /advice`; the banner literal is in `SampleBanner.tsx` only (`grep -rl '示例数据，非实时数据' web/src` prints that one file).
  4. `types/macro_rates.ts` has `SUPPORTED_VERSIONS = [3]` and imports `@/generated/macro_rates.v3`; `view.test.tsx` reads the `page.macro_rates.v3.*` fixtures, still shows gap alerts and a chart spec with a null kept, and `refuses bad-version`; `view.tsx`, `main.tsx` and `mount.tsx` are not in the diff.
  5. `web/vite.config.ts` has `outDir` `../internal/pages/embed/dist` and `emptyOutDir: false`; offline: `grep -n "embed/dist" web/vite.config.ts` prints one line.
  6. Offline: `grep -n -E 'toFixed|toLocale|Intl\.|Math\.|fetch\(|XMLHttpRequest' web/src/shell/*.tsx web/src/samples/index.ts` prints nothing; `web/no-compute-allow.txt`, `web/package.json`, `pnpm-lock.yaml`, `web/src/generated` and `web/src/pages` are not in the diff; no test sleeps; each new `.ts` or `.tsx` file stays under 600 lines; no file outside the thirteen listed is in the diff.
  Checked by CI: ci-web `tsc`, `eslint`, `vitest`, the build (it writes `internal/pages/embed/dist/index.html` with a script tag).
  Size: L

### site-ot-serve

`ot serve` serves the live routes, the sample routes and the bundle from one process
  Module: ot
  Goal: the routes of Decision 3 on stores opened once, or on demo readers; `serve` has only `--day` and `--addr`.
  Files: cmd/ot/serve.go, cmd/ot/site.go, cmd/ot/sample.go, cmd/ot/wire.go, cmd/ot/demo.go, cmd/ot/serve_test.go, cmd/ot/site_test.go, cmd/ot/sample_test.go, docs/contracts/ot.md
  Design: docs/design/unified-site.md#decisions
  Depends on: site-interface (merged: `page.sample_page.v1` and its fixture), site-pages (merged: `embed.Assets`).
  Behaviour: `runServe` is `runSite` without the flag test. `siteHandler` gains `/macro` (builds `deps.pageList[0]` per request, written with `newHandler`), `/assets/` (`deps.assets`) and the six sample routes (`sample.go`: route to schema map, the body the constant of Decision 4 written with `newHandler`; no store read). `wire.go` gains `assets` (real value `embed.Assets()`). `openSiteReaders` also opens the series store. `demoReader` returns points for the seven ids with their stored units.
  Acceptance:
  1. `grep -c '"name"\|"site"' cmd/ot/serve.go` prints 0; `grep -c 'OT_PG_DSN\|DSNEnv' cmd/ot/site.go cmd/ot/sample.go` prints at most 1 in total and it is `store.DSNEnvPages`; `git diff cmd/ot/page.go cmd/ot/pagelist.go` is empty.
  2. Tests that change, by name: `TestServeUnknownNameBindsNoPort` and `TestServeMacroPage` (serve_test.go) are removed; `TestServeSiteFlags` becomes `TestServeFlags` (`serve --site` and `serve --name x` are unknown-flag errors and bind no port; `--day x` is a bad-day error); `TestSiteRoutes` expects `/` 302 to `/home`, `/macro` 200 with the fake macro page built once for the requested day, `/assets/a.js` answered by the fake assets handler, `/options/data` rules as before, `/home/x`, `/macro/x`, `/nope` 404, POST 405, build and catalog errors 500 with the fixed text; `TestSiteDemoOpensNoStore` expects `/macro` with `"schema":"macro_rates"` and seven series (all with a unit of the v3 enum) besides `/options/data`; `TestSiteMissingDSN` is unchanged in meaning (the variable name, no DSN text). `TestServeOnAnswersBuilderBytes` and `TestServeOnBuildError500` pass unchanged if `serveOn` stays, else are removed in the same diff.
  3. `TestSampleRoutes`: the six routes answer 200 with a vm equal in shape to `page.sample_page.v1.ok.json` and the schema name of Decision 4 (home to `home_page`, household to `household_page`, positions to `positions_page`, risk to `household_risk`, options to `option_page`, advice to `advice_page`); no opener is called (openers that fail the test); `/macro` and `/options/data` never produce a `sample_page` vm.
  4. `TestSiteStoresClosed`: a failed open of any of the six stores closes those already opened (a counter).
  5. `docs/contracts/ot.md` describes `ot serve [--day DAY] [--addr ADDR]`, the three kinds of route, `pnpm --dir web build` as the step before it, `OT_PG_DSN_PAGES` for the six stores, and no longer lists `--site` or `--name` for `serve` (lines 12-13, 29, 51, 74, 113-114 of today's text); no file outside the nine listed is in the diff.
  6. Every test calls `t.Parallel()`, uses httptest or a listener on port 0, `t.TempDir()`, no sleep; each new file under 600 lines and `site_test.go` under 800; `go run ./scripts/arch --check` exits 0.
  Checked by CI: ci-go (`go build ./...`, `go test ./cmd/ot/...`, golangci-lint, go-arch-lint).
  Size: M

## Left out

- Live data for the six sample pages: each is its own design (a source, a builder, then one line in the route table).
- Retiring `macro_page` (builder, view, schema, `ot page --name macro_page`) and the carried `option_page`.
- A lazy-loaded or smaller bundle (the samples ship in the one bundle), caching, authentication, TLS and another listen address: `serve` stays a local tool.
- Sample links beyond the bar: the samples are cut to bar routes, so no page of the site links to a 404.
- Clearing stale hashed files of `dist` (`emptyOutDir: false`): the owner may delete the directory contents by hand; git ignores them.
- A browser check: the owner's check after all four merge.

## Adversarial

Full answer of the review: `unified-site-adversarial.md` in the dispatcher window scratchpad (session 3a9912bb, not in the repository). Each point and fix, answered:

1. Gap (the stub): verified (dist is the six-line stub, no `outDir`, `Handler` ignores the path); adopted: Decisions 1-2, `site-pages` items 3-4, `site-web` item 5, the 500 text. Sample links that 404: adopted by cutting the samples to bar routes (Decision 5).
2. Biggest unnamed risk (units): verified in `mapping.go`; adopted, v3 with `index_points` as the interface issue, builder, demo reader and test validating against the schema. Sample payload risk: adopted in part, existing fixtures reused and rendered through `App` in `sample.test.tsx`; shape errors would still show only there, and home and risk JSON are new files checked by equality with their TS fixtures.
3. Simpler way: adopted in part, the sample machinery shrank to a marker schema plus web-side data (no Go sample code, no element); not adopted, a second design for samples, as the owner wants one. `ot page --name` stays (Decision 1), so "one command" means one site command.
4. Lock-in: adopted, `## Interface` names every cross-module contract and the interface issue lands first; the removal of `--site`/`--name` lists every line of `ot.md` to edit.
5. False claims: (a) units, adopted; (b) `site-ot-serve` now depends on `site-pages`; (c) `main.tsx` and `mount.tsx` need no edit because `App` handles the marker, and the sample test calls `App` only, so `shell.test.tsx` is untouched; (d) home fixtures: adopted, they are sources of the new JSON; (g)(h) every changed test is named by exact name in `site-pages` item 2 and `site-ot-serve` item 2; (i) the demo reader's units are stated.
6. Slip: the missing interface issue and the units are fixed; the module check is fixed by Files lines; the order is fixed by Depends; the size stop is gone (JSON exempt, `.ts` small); the blank page is fixed by Decision 2. Remaining risk: `resolveJsonModule` for the sample imports (default on with `moduleResolution: bundler`, not run here); the first `pnpm build` writing outside `web/` (a vite warning, not an error); the sixteen-file interface entry is large but one module.
