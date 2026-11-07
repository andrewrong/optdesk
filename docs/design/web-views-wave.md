# Web: a wave of self-registering page views

Author: advisor of optdesk
Source: owner request of 2026-11-07, run front-end work in parallel with the backend, as many independent issues as possible
(the queue runs up to 8 at once). Read on main at 3f2c13a: `web/src/shell/pages.tsx`, `web/src/shell/page.ts`,
`web/src/types/*.ts`, `web/src/generated/*`, `web/package.json` (`generate:types`), `scripts/hygiene/main.go`,
`internal/contracts/generated_ts_test.go`, `contracts/schemas/page.*.json`, `docs/design/web-v2-types.md`,
`docs/design/web-fixtures-vn1.md`, `docs/design/modules.md`.

Not covered: `option_data_page` and `macro_rates` (the design `option-page-serve` of branch `design/option-page-serve` owns the serving
and navigation of option data and any macro rates view).

## What main holds today

- Every page renders from one JSON in `<script id=vm>` (`web/src/vm.ts`); the shell dispatches on `payload.schema` through
  `pages` of `web/src/shell/pages.tsx`. There is no `fetch`.
- `pages.tsx` registers eleven page views by hand in the object `explicit`. Home cards are found by the glob
  `../pages/home/*Card.tsx` and need no line there (`discoverCards`: a module that exports `registration = { schema, page }` is added; a
  schema registered twice throws). So a new view that adds a line to `explicit` edits a shared file: two such issues overlap and run one after the other.
- Page schemas with no view: `indicator_detail`, `settings_page`, `household_risk`, `ledger_page`, `report_page` (v1, v2), `home_page`,
  `stock_option_page`, `option_result_page`, `structure_detail` (v1 to v3), and `market_weather` (a card only).
- Generated types: `web/src/generated/**` belongs to contracts (modules.md); `web/package.json` holds the one `generate:types` script that lists
  every schema (five today), `web/src/generated/index.ts` re-exports and `TestGeneratedSummaryTypes` in `internal/contracts/generated_ts_test.go`
  lists the files. ci-web runs `generate:types` and fails on drift. A view reads its type through a small file `web/src/types/<schema>.ts`
  (web) that wraps the generated type and `placeholderValidate` (`web/src/types/market_summary.ts` is the pattern).
- Fixtures: `web/fixtures/<dir>/{ok,gap,stale,bad-version}.json` for 13 directories, all listed in one `web/fixtures/MANIFEST.synthetic-v1.json`.
  `scripts/hygiene` reads one manifest per directory named `fixtures` and fails for a file the manifest does not list or a `fixtures`
  directory with no manifest. So a new fixture in `web/fixtures/` edits the shared manifest (the second shared file of the cut); a new
  `fixtures` directory with its own manifest edits nothing shared.

## Shared files found, and how each is split

| shared file | what is shared | split |
|---|---|---|
| `web/src/shell/pages.tsx` | the object `explicit`, one line and one import per page | the interface entry moves each page's registration into its own page file and replaces the cards glob by one glob over `../pages/*/*.tsx`; `explicit` keeps only `option_data_page` until `option-page-serve` lands (its page file is under that design) |
| `web/src/generated/index.ts`, the `generate:types` script of `web/package.json`, the list in `internal/contracts/generated_ts_test.go` | one line per generated schema | the interface entry adds all six schemas at once (the six views need no line there); a later schema is a one-line addition by its own interface entry |
| `web/fixtures/MANIFEST.synthetic-v1.json` | one list of every fixture file | each new page has its own `fixtures` directory with its own manifest (Decision 4); the old manifest is not touched |
| `web/test/shell/pages.test.tsx` | assertions about several pages | the interface entry keeps it for the registry rules only; each view has its test in its own `web/test/pages/<dir>/` |
| `web/src/types/*.ts` | a directory, not a file | one new file per page, named by the schema; no index file exists |
| `web/no-compute-allow.txt`, `web/deps-allowlist.txt`, `web/copy-manifest.sha256` | lists that a page could need to extend | not edited by a view; a view that needs an entry is stopped and reported (acceptance 3) |

## Decisions

1. Which pages. A page is in the cut when its schema is a single page (not a composition of other pages), is under 1100 lines, and a fixture can be written from the schema alone. That
   gives six: `indicator_detail` (443 lines), `option_result_page` (482), `household_risk` (894), `ledger_page` (851), `structure_detail` v3 (865) and
   `stock_option_page` (1017). The reasons for the others are in `## Left out`.
2. Self-registration. The interface entry replaces the cards glob by one glob `../pages/*/*.tsx` (home cards included). A page file exports the same
   `registration` as a card; the new views use `view.tsx`. The one glob feeds the duplicate check with `explicit` as `taken`. The entry also adds
   `export const registration` to the ten existing explicit page files and deletes their lines and imports from `pages.tsx`; `option_data_page` stays explicit.
   After it lands no page issue, old or new, edits `pages.tsx`.
3. Types for all six come from one interface entry. The generator script, `index.ts` and the Go list are shared files, so the interface entry adds all six
   schemas to them at once (the way `web-v2-types.md` made `generated-summary-types` a two-module entry of contracts and web). The six views then need no contracts path.
   Splitting `pages.tsx` also at the ten old pages makes the interface entry size M, and it stays serial before the six; the alternative leaves a shared file for every later page issue.
4. Fixtures go in a `fixtures` directory of the page's own test directory, `web/test/pages/<dir>/fixtures/`, each with its own manifest. This is
   not a way round hygiene: the check applies to the new directory and each manifest lists its four files. The old shared `web/fixtures/` is left as it is.
5. Directory names are new and one per page (`indicator`, `risk`, `ledger`, `optionresult`, `structuredetail`, `stockoption`), so no entry shares a directory with another or with an existing page (`pages/household/` and `pages/options/` already have files from other issues).
6. A view is read-only and computes nothing (AGENTS.md); `settings_page` has a form that cannot be sent without a request, so it is left out.

## Modules

No row is added or changed in `docs/design/modules.md`, so `arch.toml` is unchanged (`scripts/arch` derives it from the table). Paths used:
`web/**` and `web/test/**` belong to web; `web/src/generated/**` and `internal/contracts/**` belong to contracts.

| issue | module | paths |
|---|---|---|
| pages-self-registering (interface) | web, plus contracts (judged two-module entry, Decision 3) | `web/src/shell/pages.tsx`, `web/test/shell/pages.test.tsx`, `web/package.json`, `web/src/generated/**`, `internal/contracts/generated_ts_test.go`, and the ten existing page files named in its What |
| indicator-detail-view | web | `web/src/pages/indicator/**`, `web/src/types/indicator_detail.ts`, `web/test/pages/indicator/**` |
| option-result-view | web | `web/src/pages/optionresult/**`, `web/src/types/option_result_page.ts`, `web/test/pages/optionresult/**` |
| household-risk-view | web | `web/src/pages/risk/**`, `web/src/types/household_risk.ts`, `web/test/pages/risk/**` |
| ledger-view | web | `web/src/pages/ledger/**`, `web/src/types/ledger_page.ts`, `web/test/pages/ledger/**` |
| structure-detail-view | web | `web/src/pages/structuredetail/**`, `web/src/types/structure_detail.ts`, `web/test/pages/structuredetail/**` |
| stock-option-view | web | `web/src/pages/stockoption/**`, `web/src/types/stock_option_page.ts`, `web/test/pages/stockoption/**` |

Order: `pages-self-registering` first; then the six views, independent of each other (up to six at once, with no shared file). Needing a file
outside an issue's Files is reported in the pull request description, not edited.

## Interface

Owner: web (`pages.tsx`), contracts (the generated types). Frozen by `pages-self-registering`, read by the six views.

- `view.tsx` of `web/src/pages/<dir>/` exports `registration: { schema: string; page: Page }` (the shape `CardRegistration` already declares in `pages.tsx`), built as
  `page(validate, View)` with `validate` and `Payload` from `@/types/<schema>`. A file that exports no `registration` is skipped; a schema registered twice, or already in `explicit` or the cards, throws at load.
- `pages.tsx` holds one `import.meta.glob('../pages/*/*.tsx', { eager: true })`; `discoverCards` is reused unchanged (its name stays so that the existing test keeps working).
- `web/src/generated/` gains `indicator_detail.v1.ts` (`IndicatorDetail`), `option_result_page.v1.ts` (`OptionResultPage`), `household_risk.v1.ts` (`HouseholdRisk`),
  `ledger_page.v1.ts` (`LedgerPage`), `structure_detail.v3.ts` (`StructureDetail`) and `stock_option_page.v1.ts` (`StockOptionPage`), each made by `json2ts` with the banner
  of the existing files; `index.ts` re-exports the six top-level names; `generate:types` lists the six.

## Issues

### pages-self-registering (interface)

- Module: web and contracts. Size: M. Depends on: none.
- Design: Decisions 2 and 3.
- Files: `web/src/shell/pages.tsx`, `web/test/shell/pages.test.tsx`, `web/package.json`, `web/src/generated/**`, `internal/contracts/generated_ts_test.go`, `web/src/pages/macro/MacroPage.tsx`, `web/src/pages/household/HouseholdPage.tsx`, `web/src/pages/companies/CompaniesPage.tsx`, `web/src/pages/advice/AdvicePage.tsx`, `web/src/pages/home/MacroCard.tsx`, `web/src/pages/candidates/FullScanPage.tsx`, `web/src/pages/options/OptionPage.tsx`, `web/src/pages/options/Tracking.tsx`, `web/src/pages/positions/PositionsPage.tsx`, `web/src/pages/structure/StructuresList.tsx`.
- What: replace the cards glob by the one glob over `../pages/*/*.tsx`; add `export const registration = { schema, page: page(validate, Component) }` to each of the ten page files above and remove its import and its line from `explicit` (only `option_data_page` stays); make the duplicate check span `explicit` and the glob; add the six `json2ts` calls to `generate:types`; commit the six generated files and the `index.ts` lines; extend `TestGeneratedSummaryTypes` with the six rows. The existing license of `json-schema-to-typescript` (whitelisted 2026-11-06) is used, no new package.
- 按 frontend-stack：用 vite 的 `import.meta.glob` 做 页面视图的自注册。

Acceptance:

1. `grep -c "pages/\*/\*.tsx" web/src/shell/pages.tsx` prints 1, `grep -c "page<" web/src/shell/pages.tsx` prints 1 (only `option_data_page`), and `grep -l "export const registration" web/src/pages/*/*.tsx | wc -l` prints at least 15 (five cards, ten pages).
2. `test -f` holds for each of the six files named under Interface in `web/src/generated/`, and `grep -c "Code generated from contracts/, DO NOT EDIT" web/src/generated/<file>` prints 1 for each.
3. `grep -c "export type" web/src/generated/index.ts` prints 9 (three before, six after), and `grep -c "json2ts" web/package.json` is unchanged in kind: `grep -o "page\.[a-z_]*\.v[0-9]*\.json" web/package.json | sort -u | wc -l` prints 11.
4. `grep -c "stock_option_page.v1.ts" internal/contracts/generated_ts_test.go` prints 1 and `git diff --name-only origin/main -- contracts/schemas contracts/proto contracts/datasets` prints nothing (the pull request says `Contracts: unchanged`).
5. `web/test/shell/pages.test.tsx` holds a test that a module map with a `registration` under `../pages/x/view.tsx` is found and that a schema named twice throws; `wc -l web/src/shell/pages.tsx` stays under 800.
6. The result matches the Interface section of this design: nothing is added to `explicit`, and a view needs no other shared file.

Checked by CI: `pnpm install --frozen-lockfile`, `pnpm --dir web run generate:types` with `git diff --exit-code web/src/generated` (ci-web drift check), tsc, eslint, vitest, the build; ci-go (`go test ./internal/contracts`); checks (the `Contracts:` line).

### indicator-detail-view

- Module: web. Size: S. Depends on: `pages-self-registering` only.
- Files: `web/src/pages/indicator/view.tsx`, `web/src/pages/indicator/*.tsx`, `web/src/types/indicator_detail.ts`, `web/test/pages/indicator/**` (the test and the fixtures directory `web/test/pages/indicator/fixtures/` with its own `MANIFEST.synthetic-v1.json`).
- What: a read-only view of `page.indicator_detail.v1` that shows the title, sentence and stale label as text; the `latest` figure or its gap; the `points` as one line chart with the optional `reference_line`; the `revision` note, the `explanation` lines and the `gaps`. Every string and figure is a field of the payload; the view computes and formats nothing and has no `fetch`. `web/src/types/indicator_detail.ts` follows the pattern of `web/src/types/market_summary.ts` (`Payload` is `IndicatorDetail` of `@/generated/indicator_detail.v1`, `SUPPORTED_VERSIONS = [1]`, `validate` from `placeholderValidate`). Four fixtures `ok`, `gap`, `stale` and `bad-version` are written from the schema alone (`schema` is `indicator_detail`, `schema_version` is 1; `bad-version` carries another number), each typed with `satisfies IndicatorDetail` in the test so `tsc` checks its shape.

- 按 frontend-stack：用 recharts, through `web/src/charts/Chart.tsx` 做 the `points` line chart and its reference line。
- 按 frontend-stack：用 shadcn card, badge and alert (`components/ui`) 做 the header, the latest figure, the stale label and the gap lines。

Acceptance:

1. `test -f web/src/pages/indicator/view.tsx && test -f web/src/types/indicator_detail.ts`, and the four files `ok.json`, `gap.json`, `stale.json`, `bad-version.json` exist under `web/test/pages/indicator/fixtures/`.
2. `grep -c "export const registration" web/src/pages/indicator/view.tsx` prints 1 and `grep -c "schema: 'indicator_detail'" web/src/pages/indicator/view.tsx` prints 1; `git diff --name-only origin/main` lists no path outside the Files line (in particular not `web/src/shell/pages.tsx`, `web/package.json` or `web/fixtures/MANIFEST.synthetic-v1.json`).
3. `wc -l web/src/pages/indicator/*.tsx` shows every file under 600 lines; `grep -c "fetch(" web/src/pages/indicator/*.tsx` prints 0 for each; `git diff origin/main -- web/no-compute-allow.txt` is empty.
4. `web/test/pages/indicator/fixtures/MANIFEST.synthetic-v1.json` has `marker` `synthetic-v1` and lists exactly the four fixture files and nothing else; `grep -c "indicator_detail" web/test/pages/indicator/fixtures/ok.json` is at least 1; `grep -c "@" web/test/pages/indicator/fixtures/*.json` prints 0 for each file.
5. `web/test/pages/indicator/` holds a vitest file that reads the four fixtures, renders `ok` and `stale` and checks that a text read from the fixture is on the page, renders `gap` and checks that the gap text is on the page, and checks that `validate` refuses `bad-version`; it reads no clock and opens no port.
6. The result matches the part of this design named `indicator-detail-view`: the view shows exactly the fields listed under What, from code and this file (not a rendering).

Checked by CI: `pnpm install --frozen-lockfile`, then `pnpm --dir web run typecheck` (tsc), `pnpm --dir web run lint` (eslint), `pnpm --dir web run test` (vitest) and `pnpm --dir web run build`; hygiene (the manifest of the new `fixtures` directory and the privacy scan).

### household-risk-view

- Module: web. Size: M. Depends on: `pages-self-registering` only.
- Files: `web/src/pages/risk/view.tsx`, `web/src/pages/risk/*.tsx`, `web/src/types/household_risk.ts`, `web/test/pages/risk/**` (the test and the fixtures directory `web/test/pages/risk/fixtures/` with its own `MANIFEST.synthetic-v1.json`).
- What: a read-only view of `page.household_risk.v1` that shows the `sentence` and stale label; the `delta`, `beta_delta`, `sigma` and `vega` blocks (each a titled block of rows or a gap); the `standard_metrics` table; the `notes` and `gaps`. Every string and figure is a field of the payload; the view computes and formats nothing and has no `fetch`. `web/src/types/household_risk.ts` follows the pattern of `web/src/types/market_summary.ts` (`Payload` is `HouseholdRisk` of `@/generated/household_risk.v1`, `SUPPORTED_VERSIONS = [1]`, `validate` from `placeholderValidate`). Four fixtures `ok`, `gap`, `stale` and `bad-version` are written from the schema alone (`schema` is `household_risk`, `schema_version` is 1; `bad-version` carries another number), each typed with `satisfies HouseholdRisk` in the test so `tsc` checks its shape.

- 按 frontend-stack：用 @tanstack/react-table, through `components/data-table` 做 the `standard_metrics` and the rows of the delta, sigma and vega blocks。
- 按 frontend-stack：用 shadcn card, badge and alert 做 the titled blocks, the stale label and the gap lines。

Acceptance:

1. `test -f web/src/pages/risk/view.tsx && test -f web/src/types/household_risk.ts`, and the four files `ok.json`, `gap.json`, `stale.json`, `bad-version.json` exist under `web/test/pages/risk/fixtures/`.
2. `grep -c "export const registration" web/src/pages/risk/view.tsx` prints 1 and `grep -c "schema: 'household_risk'" web/src/pages/risk/view.tsx` prints 1; `git diff --name-only origin/main` lists no path outside the Files line (in particular not `web/src/shell/pages.tsx`, `web/package.json` or `web/fixtures/MANIFEST.synthetic-v1.json`).
3. `wc -l web/src/pages/risk/*.tsx` shows every file under 600 lines; `grep -c "fetch(" web/src/pages/risk/*.tsx` prints 0 for each; `git diff origin/main -- web/no-compute-allow.txt` is empty.
4. `web/test/pages/risk/fixtures/MANIFEST.synthetic-v1.json` has `marker` `synthetic-v1` and lists exactly the four fixture files and nothing else; `grep -c "household_risk" web/test/pages/risk/fixtures/ok.json` is at least 1; `grep -c "@" web/test/pages/risk/fixtures/*.json` prints 0 for each file.
5. `web/test/pages/risk/` holds a vitest file that reads the four fixtures, renders `ok` and `stale` and checks that a text read from the fixture is on the page, renders `gap` and checks that the gap text is on the page, and checks that `validate` refuses `bad-version`; it reads no clock and opens no port.
6. The result matches the part of this design named `household-risk-view`: the view shows exactly the fields listed under What, from code and this file (not a rendering).

Checked by CI: `pnpm install --frozen-lockfile`, then `pnpm --dir web run typecheck` (tsc), `pnpm --dir web run lint` (eslint), `pnpm --dir web run test` (vitest) and `pnpm --dir web run build`; hygiene (the manifest of the new `fixtures` directory and the privacy scan).

### ledger-view

- Module: web. Size: M. Depends on: `pages-self-registering` only.
- Files: `web/src/pages/ledger/view.tsx`, `web/src/pages/ledger/*.tsx`, `web/src/types/ledger_page.ts`, `web/test/pages/ledger/**` (the test and the fixtures directory `web/test/pages/ledger/fixtures/` with its own `MANIFEST.synthetic-v1.json`).
- What: a read-only view of `page.ledger_page.v1` that shows the `header`, the `overview`, each of the `sections` as a titled table, the `shares`, the `nothing_closed` line when nothing closed, the `notes`, the `footer` and the `gaps`. Every string and figure is a field of the payload; the view computes and formats nothing and has no `fetch`. `web/src/types/ledger_page.ts` follows the pattern of `web/src/types/market_summary.ts` (`Payload` is `LedgerPage` of `@/generated/ledger_page.v1`, `SUPPORTED_VERSIONS = [1]`, `validate` from `placeholderValidate`). Four fixtures `ok`, `gap`, `stale` and `bad-version` are written from the schema alone (`schema` is `ledger_page`, `schema_version` is 1; `bad-version` carries another number), each typed with `satisfies LedgerPage` in the test so `tsc` checks its shape.

- 按 frontend-stack：用 @tanstack/react-table, through `components/data-table` 做 each of the `sections`。
- 按 frontend-stack：用 shadcn card, tabs and alert 做 the header, the overview, the shares and the nothing-closed and gap lines。

Acceptance:

1. `test -f web/src/pages/ledger/view.tsx && test -f web/src/types/ledger_page.ts`, and the four files `ok.json`, `gap.json`, `stale.json`, `bad-version.json` exist under `web/test/pages/ledger/fixtures/`.
2. `grep -c "export const registration" web/src/pages/ledger/view.tsx` prints 1 and `grep -c "schema: 'ledger_page'" web/src/pages/ledger/view.tsx` prints 1; `git diff --name-only origin/main` lists no path outside the Files line (in particular not `web/src/shell/pages.tsx`, `web/package.json` or `web/fixtures/MANIFEST.synthetic-v1.json`).
3. `wc -l web/src/pages/ledger/*.tsx` shows every file under 600 lines; `grep -c "fetch(" web/src/pages/ledger/*.tsx` prints 0 for each; `git diff origin/main -- web/no-compute-allow.txt` is empty.
4. `web/test/pages/ledger/fixtures/MANIFEST.synthetic-v1.json` has `marker` `synthetic-v1` and lists exactly the four fixture files and nothing else; `grep -c "ledger_page" web/test/pages/ledger/fixtures/ok.json` is at least 1; `grep -c "@" web/test/pages/ledger/fixtures/*.json` prints 0 for each file.
5. `web/test/pages/ledger/` holds a vitest file that reads the four fixtures, renders `ok` and `stale` and checks that a text read from the fixture is on the page, renders `gap` and checks that the gap text is on the page, and checks that `validate` refuses `bad-version`; it reads no clock and opens no port.
6. The result matches the part of this design named `ledger-view`: the view shows exactly the fields listed under What, from code and this file (not a rendering).

Checked by CI: `pnpm install --frozen-lockfile`, then `pnpm --dir web run typecheck` (tsc), `pnpm --dir web run lint` (eslint), `pnpm --dir web run test` (vitest) and `pnpm --dir web run build`; hygiene (the manifest of the new `fixtures` directory and the privacy scan).

### option-result-view

- Module: web. Size: S. Depends on: `pages-self-registering` only.
- Files: `web/src/pages/optionresult/view.tsx`, `web/src/pages/optionresult/*.tsx`, `web/src/types/option_result_page.ts`, `web/test/pages/optionresult/**` (the test and the fixtures directory `web/test/pages/optionresult/fixtures/` with its own `MANIFEST.synthetic-v1.json`).
- What: a read-only view of `page.option_result_page.v1` that shows the `title`, the `facts` pairs, the `sections`, the `reliability` block, the `nothing` line when there is no result, the `notes` and the stale label. Every string and figure is a field of the payload; the view computes and formats nothing and has no `fetch`. `web/src/types/option_result_page.ts` follows the pattern of `web/src/types/market_summary.ts` (`Payload` is `OptionResultPage` of `@/generated/option_result_page.v1`, `SUPPORTED_VERSIONS = [1]`, `validate` from `placeholderValidate`). Four fixtures `ok`, `gap`, `stale` and `bad-version` are written from the schema alone (`schema` is `option_result_page`, `schema_version` is 1; `bad-version` carries another number), each typed with `satisfies OptionResultPage` in the test so `tsc` checks its shape.

- 按 frontend-stack：用 shadcn card, badge, separator and alert 做 the facts pairs, the reliability block and the nothing and stale lines。
- 按 frontend-stack：用 @tanstack/react-table, through `components/data-table` 做 each of the `sections`。

Acceptance:

1. `test -f web/src/pages/optionresult/view.tsx && test -f web/src/types/option_result_page.ts`, and the four files `ok.json`, `gap.json`, `stale.json`, `bad-version.json` exist under `web/test/pages/optionresult/fixtures/`.
2. `grep -c "export const registration" web/src/pages/optionresult/view.tsx` prints 1 and `grep -c "schema: 'option_result_page'" web/src/pages/optionresult/view.tsx` prints 1; `git diff --name-only origin/main` lists no path outside the Files line (in particular not `web/src/shell/pages.tsx`, `web/package.json` or `web/fixtures/MANIFEST.synthetic-v1.json`).
3. `wc -l web/src/pages/optionresult/*.tsx` shows every file under 600 lines; `grep -c "fetch(" web/src/pages/optionresult/*.tsx` prints 0 for each; `git diff origin/main -- web/no-compute-allow.txt` is empty.
4. `web/test/pages/optionresult/fixtures/MANIFEST.synthetic-v1.json` has `marker` `synthetic-v1` and lists exactly the four fixture files and nothing else; `grep -c "option_result_page" web/test/pages/optionresult/fixtures/ok.json` is at least 1; `grep -c "@" web/test/pages/optionresult/fixtures/*.json` prints 0 for each file.
5. `web/test/pages/optionresult/` holds a vitest file that reads the four fixtures, renders `ok` and `stale` and checks that a text read from the fixture is on the page, renders `gap` and checks that the gap text is on the page, and checks that `validate` refuses `bad-version`; it reads no clock and opens no port.
6. The result matches the part of this design named `option-result-view`: the view shows exactly the fields listed under What, from code and this file (not a rendering).

Checked by CI: `pnpm install --frozen-lockfile`, then `pnpm --dir web run typecheck` (tsc), `pnpm --dir web run lint` (eslint), `pnpm --dir web run test` (vitest) and `pnpm --dir web run build`; hygiene (the manifest of the new `fixtures` directory and the privacy scan).

### structure-detail-view

- Module: web. Size: L. Depends on: `pages-self-registering` only.
- Files: `web/src/pages/structuredetail/view.tsx`, `web/src/pages/structuredetail/*.tsx`, `web/src/types/structure_detail.ts`, `web/test/pages/structuredetail/**` (the test and the fixtures directory `web/test/pages/structuredetail/fixtures/` with its own `MANIFEST.synthetic-v1.json`).
- What: a read-only view of `page.structure_detail.v3` that shows the `sentence` and `subline`; the `tiles`; the `legs`, `greeks` and `fills` tables with their titles and `legs_total`; the `payoff` as one line chart; the `key_levels`; the `timeline`; the `gaps`. Every string and figure is a field of the payload; the view computes and formats nothing and has no `fetch`. `web/src/types/structure_detail.ts` follows the pattern of `web/src/types/market_summary.ts` (`Payload` is `StructureDetail` of `@/generated/structure_detail.v3`, `SUPPORTED_VERSIONS = [3]`, `validate` from `placeholderValidate`). Four fixtures `ok`, `gap`, `stale` and `bad-version` are written from the schema alone (`schema` is `structure_detail`, `schema_version` is 3; `bad-version` carries another number), each typed with `satisfies StructureDetail` in the test so `tsc` checks its shape.

- 按 frontend-stack：用 recharts, through `web/src/charts/Chart.tsx` 做 the `payoff` chart。
- 按 frontend-stack：用 @tanstack/react-table, through `components/data-table` 做 the `legs`, `greeks` and `fills` tables。
- 按 frontend-stack：用 shadcn card, badge, accordion and alert 做 the tiles, key levels, timeline, stale label and gap lines。

Acceptance:

1. `test -f web/src/pages/structuredetail/view.tsx && test -f web/src/types/structure_detail.ts`, and the four files `ok.json`, `gap.json`, `stale.json`, `bad-version.json` exist under `web/test/pages/structuredetail/fixtures/`.
2. `grep -c "export const registration" web/src/pages/structuredetail/view.tsx` prints 1 and `grep -c "schema: 'structure_detail'" web/src/pages/structuredetail/view.tsx` prints 1; `git diff --name-only origin/main` lists no path outside the Files line (in particular not `web/src/shell/pages.tsx`, `web/package.json` or `web/fixtures/MANIFEST.synthetic-v1.json`).
3. `wc -l web/src/pages/structuredetail/*.tsx` shows every file under 600 lines; `grep -c "fetch(" web/src/pages/structuredetail/*.tsx` prints 0 for each; `git diff origin/main -- web/no-compute-allow.txt` is empty.
4. `web/test/pages/structuredetail/fixtures/MANIFEST.synthetic-v1.json` has `marker` `synthetic-v1` and lists exactly the four fixture files and nothing else; `grep -c "structure_detail" web/test/pages/structuredetail/fixtures/ok.json` is at least 1; `grep -c "@" web/test/pages/structuredetail/fixtures/*.json` prints 0 for each file.
5. `web/test/pages/structuredetail/` holds a vitest file that reads the four fixtures, renders `ok` and `stale` and checks that a text read from the fixture is on the page, renders `gap` and checks that the gap text is on the page, and checks that `validate` refuses `bad-version`; it reads no clock and opens no port.
6. The result matches the part of this design named `structure-detail-view`: the view shows exactly the fields listed under What, from code and this file (not a rendering).

Checked by CI: `pnpm install --frozen-lockfile`, then `pnpm --dir web run typecheck` (tsc), `pnpm --dir web run lint` (eslint), `pnpm --dir web run test` (vitest) and `pnpm --dir web run build`; hygiene (the manifest of the new `fixtures` directory and the privacy scan).

### stock-option-view

- Module: web. Size: L. Depends on: `pages-self-registering` only.
- Files: `web/src/pages/stockoption/view.tsx`, `web/src/pages/stockoption/*.tsx`, `web/src/types/stock_option_page.ts`, `web/test/pages/stockoption/**` (the test and the fixtures directory `web/test/pages/stockoption/fixtures/` with its own `MANIFEST.synthetic-v1.json`).
- What: a read-only view of `page.stock_option_page.v1` that shows the `top` block, the `cards` (or the `no_card` line), the `matrix`, the `maps`, the `contracts` table, the `held` block, the `notice`, the `shared_gap` and the `footer`. Every string and figure is a field of the payload; the view computes and formats nothing and has no `fetch`. `web/src/types/stock_option_page.ts` follows the pattern of `web/src/types/market_summary.ts` (`Payload` is `StockOptionPage` of `@/generated/stock_option_page.v1`, `SUPPORTED_VERSIONS = [1]`, `validate` from `placeholderValidate`). Four fixtures `ok`, `gap`, `stale` and `bad-version` are written from the schema alone (`schema` is `stock_option_page`, `schema_version` is 1; `bad-version` carries another number), each typed with `satisfies StockOptionPage` in the test so `tsc` checks its shape.

- 按 frontend-stack：用 @tanstack/react-table, through `components/data-table` 做 the `matrix` and the `contracts` table。
- 按 frontend-stack：用 shadcn card, tabs, badge and alert 做 the top block, the cards, the maps, the held block and the notice and gap lines。

Acceptance:

1. `test -f web/src/pages/stockoption/view.tsx && test -f web/src/types/stock_option_page.ts`, and the four files `ok.json`, `gap.json`, `stale.json`, `bad-version.json` exist under `web/test/pages/stockoption/fixtures/`.
2. `grep -c "export const registration" web/src/pages/stockoption/view.tsx` prints 1 and `grep -c "schema: 'stock_option_page'" web/src/pages/stockoption/view.tsx` prints 1; `git diff --name-only origin/main` lists no path outside the Files line (in particular not `web/src/shell/pages.tsx`, `web/package.json` or `web/fixtures/MANIFEST.synthetic-v1.json`).
3. `wc -l web/src/pages/stockoption/*.tsx` shows every file under 600 lines; `grep -c "fetch(" web/src/pages/stockoption/*.tsx` prints 0 for each; `git diff origin/main -- web/no-compute-allow.txt` is empty.
4. `web/test/pages/stockoption/fixtures/MANIFEST.synthetic-v1.json` has `marker` `synthetic-v1` and lists exactly the four fixture files and nothing else; `grep -c "stock_option_page" web/test/pages/stockoption/fixtures/ok.json` is at least 1; `grep -c "@" web/test/pages/stockoption/fixtures/*.json` prints 0 for each file.
5. `web/test/pages/stockoption/` holds a vitest file that reads the four fixtures, renders `ok` and `stale` and checks that a text read from the fixture is on the page, renders `gap` and checks that the gap text is on the page, and checks that `validate` refuses `bad-version`; it reads no clock and opens no port.
6. The result matches the part of this design named `stock-option-view`: the view shows exactly the fields listed under What, from code and this file (not a rendering).

Checked by CI: `pnpm install --frozen-lockfile`, then `pnpm --dir web run typecheck` (tsc), `pnpm --dir web run lint` (eslint), `pnpm --dir web run test` (vitest) and `pnpm --dir web run build`; hygiene (the manifest of the new `fixtures` directory and the privacy scan).

## Left out

- `settings_page`: its `form` (option exclusion) is an input form. With no `fetch` and no write path the form cannot be sent; a read-only table of `rows` is not the page's job. It waits for a write path (a design of its own).
- `report_page` v1 and v2: v2 is 3875 lines with 57 definitions of blocks, chips and market parts; a fixture written from the schema alone would not be honest and the view is too large for one issue. It needs its own design with real golden payloads.
- `home_page` (2446 lines): it composes the five cards and links to reports and blocks; its view depends on which pages exist and on navigation, which `option-page-serve` touches. Later.
- `market_weather` as a page: the card exists (`WeatherCard.tsx`) and no second view is asked for.
- `household_summary` v2 and `held_companies_summary` v2: cards with v1 views today; a version move is a separate change.
- Moving `option_data_page` out of `explicit`: its page file belongs to the design in flight; a one-line follow-up after it lands.
- Fixtures in the shared `web/fixtures/` and a schema check of fixtures (see Adversarial 2 and 5).

## Adversarial

1. Gap against the goal: the cut gives six parallel page entries, the owner asked for as many as possible. Adopted as a limit: the schemas that remain are either large or need writes (Left out). The interface entry is one serial step before them; it is small and has no view code in it.
2. Biggest unnamed risk: a fixture written from the schema alone may satisfy `tsc` (`satisfies`) and still break a constraint the type cannot say (`maxLength`, `pattern`, `x-rounding`); no schema validator is whitelisted. Not removed: a follow-up may add a Go test in `internal/contracts` that validates `web/test/pages/*/fixtures` against the schemas, the way `fixtures_test.go` does for `web/fixtures`. Named in the entries' risk, not built here.
3. Simpler way: put each `registration` line in `explicit` and accept serial runs. Not adopted: it makes the six entries overlap on one file, the thing the goal rules out. The glob costs one entry.
4. Lock-in: `view.tsx` in `web/src/pages/*/` becomes the page convention, and the per-page `fixtures` directories under `web/test/pages` split fixtures from `web/fixtures`. Adopted knowingly; a later issue can move the old ones, and each is one directory.
5. Claim most likely false: "after the interface entry each view touches only its own paths". Two things can break it: a view that needs a `components/ui` piece that is not there (the list has no table, select or dialog beyond `table.tsx`; a missing piece is reported, not added) and a schema whose `generate:types` output does not compile under `tsc --strict` (the json2ts output for `structure_detail` and `stock_option_page` is not seen until the interface entry runs; if it fails, that entry stops and reports). Both are named as the stop rule.
6. Likeliest cause of a slip: the two L entries (`structure-detail-view`, `stock-option-view`) exceed 600 lines in one file; the rule says report, not split, so the agent must split the view over several files inside its own directory (the Files line allows `web/src/pages/<dir>/*.tsx`). Adopted: the Files line says so and acceptance 3 checks each file. The second cause: the `Contracts:` line; the body of this change and of the interface entry says `Contracts: unchanged` from the start.
