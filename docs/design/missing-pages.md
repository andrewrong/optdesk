# Missing pages: market, price lines, core and diagnosis

Author: the optdesk designer (second draft, after the adversarial review)
Owner approved: (pending)
Revision: 3

Source: the page comparison of 2026-11-08: boards 7 (市场页), 14 (价格线), 15 (Core) and 16 (Diagnosis · 10 years) of the owner-approved design set v2.2 have no page and no schema in optdesk. The owner chose to turn every missing page into a design.
Board 22 (Backtest) is in `## Left out`. Read on origin/main at 3ef73b1: `contracts/schemas/common.v2.json`, `page.market_summary.v3.json`, `page.market_weather.v1.json`, `page.settings_page.v1.json` (the form precedent), `page.option_data_page.v1.json`,
`internal/contracts/home_page_v2_test.go`, `web/src/types/home_page.ts`, `web/src/shell/pages.tsx`, `web/src/shell/TopNav.tsx`, `web/src/charts/Chart.tsx`, `web/src/components/data-table/data-table.tsx`, `web/test/pages/home/home-v2/fixtures/MANIFEST.synthetic-v1.json`, `docs/design/home-page.md`.

Goal: each of the four boards is a registered page of the web shell with its own page schema, its generated type and a view that renders a fixture. Each page is a render of numbers a Go function will give it; this design adds the schema and the view and no producer.
None of the four pages has a route yet: `/market`, `/plan/price-lines`, `/core` and `/diagnosis` answer 404 after these issues merge, because `ot` serves routes and `TopNav.tsx` lists live routes only. The pages are real in vitest and in the schema, not yet reachable by the owner; the route issue is a later `ot` design. No page is served until a producer lands: no route, no TopNav item. Until the compute design, a page shows "缺" for numbers without a producer. Producer-written numbers (HV30, IVP, delta, the ease value, tile figures) wait for the compute design's register of metric ids: the live page shows 缺 until a producer exists, and the fixtures are the only source of their values. Schema field descriptions of such fields say "metric id: assigned by compute".

## What main holds today

- No `page.market_page`, `page.price_lines_page`, `page.core_page` or `page.diagnosis_page` file in `contracts/schemas`; no directory for them under `web/src/pages`. The market weather exists only as the home card (`page.market_weather.v1.json`, `web/src/pages/home/WeatherCard.tsx`) and the market summary card (`page.market_summary.v3.json`).
- A page of the shell is one file `web/src/pages/<dir>/*.tsx` that exports `registration = { schema, page }`; the shell finds it by one glob (`web/src/shell/pages.tsx`), so a new page needs no line in a list.
- A new page schema of this repository is a file with its own `$id` and needs no hash line (`classifyPages` in `internal/contracts/carryover_test.go` derives it); its generated type is the new `.ts` file in `web/src/generated` and nothing else.
- `settings_page.v1` is the precedent for a page that posts a form: the payload carries `post_path`, `expires` and `mac`; the view renders a plain HTML form and runs no request of its own.
- `TopNav.tsx` lists live routes only; `cmd/ot/site.go` serves `/` and `/options/data` and the other routes answer 404 (`docs/design/option-page-serve.md`). No page here has a route yet.
- No producer exists for any number these pages show: the 200-day line, RV20 and IV−RV20 of the indices, the 3-year rank, VIX3M and the term curve, breadth, the return at today's price under four assumptions, and the ten-year ratios of the annual reports.

## Decisions

1. Four new page schemas, each `schema_version` 1, each with `$ref` to `common.v2.json` for `Id`, `Timestamp`, `RunStatus`, `SourceTime`, `StaleLabel` and `Gap`, and the envelope of `option_data_page` (`schema`, `schema_version`, `run_id`, `build_id`, `scope`, `generated_at`, `data_times`, `run_status`, `title`, `sentence`,
   optional `stale_after`, `stale`, `notes`, `gaps`). Everything the board prints is a string or a figure object `{kind "value", value, text, unit}` or a `Gap`; the view prints `text`. A producer-written `marker_percent` (0 to 100) places a dot on a bar as `Ease.marker_percent` does today.
2. Missing is shown as missing: a figure the producer cannot give is a `Gap` with its cause, named once at the top of its block (standing rule, "missing data named once").
3. No computation in the view or in the schema: the market page does not rank, the core page does not compute a return, the diagnosis page does not rank years. The text "3/10" and the return percentages are produced.
4. The page does not give a decision: core and diagnosis say "不给目标价 · 不进规则" and "只看体质 · 不给价格 · 不进任何规则" as `basis_note`, and nothing here feeds a rule.
5. The price-line form posts like `settings_page`: `post_path` is the constant `/plan/price-lines`; the token fields are `expires` and `mac`; saving needs the owner's plan-edit mode, which is a property of the route and not of the page. The view writes no `fetch(`.
6. Shapes are loose where a producer will decide: `ReturnPoint` lists 1 to 6 points (the board shows four assumptions), `years` at most 12; no enum or exact count freezes the compute design.
7. The views are split by page and registered by their own `registration`; no TopNav item and no route is added here (the routes are served by `ot`, a later design).

## Modules

None changed. `contracts/**`, `internal/contracts/**`, `docs/contracts/page-schemas.md`, `web/src/generated/**` belong to contracts; `web/**` to web.

| issue | module | order |
|---|---|---|
| page-schemas-four (interface) | contracts | first |
| market-page-view | web | after page-schemas-four |
| price-lines-view | web | after page-schemas-four |
| core-page-view | web | after page-schemas-four |
| diagnosis-page-view | web | after page-schemas-four |

## Interface

Owner: contracts. All four files are frozen by `page-schemas-four` and each is read by its view issue (the generated type) and, later, by a builder.

- `contracts/schemas/page.market_page.v1.json` (title `MarketPage`): `weather` (`{headline, sentence, chart}`; `chart` is a `Chart {aria_text, series_names, points}` of `ChartPoint {label, close, ma200}` or a `Gap`), `indices` (`{title, sentence, columns, rows, note}`; `IndexRow {name, iv, rv20, iv_minus_rv20, rank_3y}`, each figure a `Figure` or `Gap`),
  `term` (`{title, sentence, points}` of `TermPoint {label, today, five_days_ago}`, or a `Gap`) and `breadth` (`{title, rows}` of `BreadthRow {label, figure}`, or a `Gap`).
- `contracts/schemas/page.price_lines_page.v1.json` (title `PriceLinesPage`): `intro` (string), `saved` (string or null: "已保存 AMD 的价格线是 130.00。"), `error` (string or null), `form` (`PriceLineForm {post_path const "/plan/price-lines", expires, mac}`), `rows` (`PriceLineRow {symbol, market enum US JP, price string or null, note string or null}`),
  `add` (`{markets [US, JP], hint}`), `unset` (`UnsetRow {symbol, market}`: held companies with no price line, each with a prefilled add form), `footnote`.
- `contracts/schemas/page.core_page.v1.json` (title `CorePage`): `basis_note`, `scale` (`{low_text, high_text, treasury {text, marker_percent}}`), `rows` (`CoreRow {symbol, price_text, cyclical boolean, flag (string or null: "高过国债 10-14"), returns (1 to 6 `ReturnPoint {assumption, text, marker_percent}` or a `Gap`)}`),
  `jp_note`, `legend` (strings), `snapshot_text`.
- `contracts/schemas/page.diagnosis_page.v1.json` (title `DiagnosisPage`): `basis_note`, `groups` (`DiagGroup {key enum cyclical financial, title, columns, rows, legend}`; `DiagRow {symbol, latest_fy, latest, rank_text, extra, usual, years}` of `YearRow {fy, revenue, margin, extra, bar_percent}` (at most 12), or a `Gap` for "只有 4 年" and "没有 US GAAP 年报"), `notes`.
- Generated types: `web/src/generated/market_page.v1.ts`, `price_lines_page.v1.ts`, `core_page.v1.ts`, `diagnosis_page.v1.ts`, each a new file.

## Issues

### page-schemas-four (interface)

- Module: contracts. Size: L. Depends on: tracking-indicator-schemas (tracking-household-reports.md).
- Order: first of this design, "after" tracking-household-reports `tracking-indicator-schemas` (page-schemas.md order: option-pages-v22, tracking-household-reports, missing-pages). One issue for the four schemas, so `docs/contracts/page-schemas.md` is edited once.
- Design: docs/design/missing-pages.md#Interface
- Files: contracts/schemas/page.market_page.v1.json, contracts/schemas/page.price_lines_page.v1.json, contracts/schemas/page.core_page.v1.json, contracts/schemas/page.diagnosis_page.v1.json, docs/contracts/page-schemas.md, internal/contracts/market_page_v1_test.go, internal/contracts/price_lines_page_v1_test.go, internal/contracts/core_page_v1_test.go, internal/contracts/diagnosis_page_v1_test.go, web/src/generated/market_page.v1.ts, web/src/generated/price_lines_page.v1.ts, web/src/generated/core_page.v1.ts, web/src/generated/diagnosis_page.v1.ts

- Goal: write the four schemas as the Interface section gives them, each with `$id` `https://optdesk.invalid/schemas/page.<name>.v1.json`, `schema` const the page name, `schema_version` const 1 and `additionalProperties` false on every object; each file stays under 600 lines. `page-schemas.md` gets four rows. The four tests (new; the pattern of `home_page_v2_test.go` with `minimal` and `readRaw`) check, per page:
  market: the smallest instance validates; version 2 fails; an `IndexRow` with `iv` as a `Gap` validates; `weather.chart`, `term` and `breadth` as a `Gap` validate; a `ChartPoint` without `ma200` fails. Price lines: the smallest instance validates; version 2 fails; `post_path` other than `/plan/price-lines` fails; `mac` that is not 64 lower-case hex digits fails; `market` of `EU` fails.
  Core: the smallest instance validates; version 2 fails; `returns` as a `Gap` validates; `marker_percent` of 101 fails. Diagnosis: the smallest instance validates; version 2 fails; a `DiagGroup` of `key` `other` fails; `years` with 13 entries fails; a `DiagRow` given as a `Gap` validates.

Acceptance:

1. `go test ./internal/contracts -race -count=1` passes; `grep -c "t.Parallel()" internal/contracts/market_page_v1_test.go internal/contracts/price_lines_page_v1_test.go internal/contracts/core_page_v1_test.go internal/contracts/diagnosis_page_v1_test.go` is at least the number of `func Test` lines in each file; `grep -c "time.Sleep"` on the same four files prints 0 for each.
2. `go test ./internal/contracts -run 'TestGeneratedTypes|TestNewSchemasAreOurs' -count=1` passes: each generated file has the banner, its schema and an exported interface named by the schema title; the four new schemas need no hash line.
3. `wc -l contracts/schemas/page.market_page.v1.json contracts/schemas/page.price_lines_page.v1.json contracts/schemas/page.core_page.v1.json contracts/schemas/page.diagnosis_page.v1.json` shows each under 600; `git diff $(git merge-base HEAD origin/main) -- contracts/schemas/carryover.sha256` is empty.
4. `grep -c "export interface MarketPage" web/src/generated/market_page.v1.ts`, and the same with `PriceLinesPage`, `CorePage` and `DiagnosisPage` on their files, print 1 each.
5. `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the thirteen paths of the Files line.

Checked by CI: ci-go; `pnpm install --frozen-lockfile`, then `generate:types` leaves the committed files unchanged, tsc, eslint, vitest, build; the pull request body carries `Contracts: page.market_page.v1, page.price_lines_page.v1, page.core_page.v1, page.diagnosis_page.v1 (new)`; the design verdict (the schemas match the Interface section and Decisions 1 to 3 and 6).

### market-page-view

- Module: web. Size: M. Depends on: page-schemas-four.
- Order: after page-schemas-four.
- Design: docs/design/missing-pages.md#Decisions
- Files: web/src/types/market_page.ts, web/src/pages/market/MarketPage.tsx, web/test/pages/market/MarketPage.test.tsx, web/test/pages/market/fixtures/ok.ts, web/test/pages/market/fixtures/gap.ts, web/test/pages/market/fixtures/stale.ts, web/test/pages/market/fixtures/bad-version.json, web/test/pages/market/fixtures/MANIFEST.synthetic-v1.json

- Goal: The page `MarketPage` shows, in the board's order, the weather sentence and chart, the index table, the VIX term block and the breadth table; a `Gap` block shows its cause once and no empty table. `web/src/types/market_page.ts` takes `Payload` from `MarketPage` of `@/generated/market_page.v1` with `SUPPORTED_VERSIONS = [1]` (the pattern of `web/src/types/home_page.ts`). `MarketPage.tsx` exports `registration = { schema: 'market_page', page: page<Payload>(validate, MarketPage) }`, shows `stale` and `gaps` with the `StaleAlert` and `CardNotes` of `web/src/pages/home` (imported, not edited), prints every text from the payload, and compares, sums and formats nothing.
  `ok.ts` is typed `satisfies MarketPage`; `gap.ts` carries a missing figure with its cause; `stale.ts` a stale label; `bad-version.json` a `schema_version` of 2. The page has no route (Goal of this design); no TopNav item is added.
- 按 frontend-stack：用 recharts through `web/src/charts/Chart.tsx` 做 the SPX close with its 200-day line over 120 sessions and the VIX term curve (today against 5 days ago)。
- 按 frontend-stack：用 @tanstack/react-table (`web/src/components/data-table/data-table.tsx`) 做 the index table and the breadth table。
- 按 frontend-stack：用 shadcn card, alert and badge (`components/ui`) 做 the four blocks, a gap's cause and the weather label。

Acceptance:

1. `pnpm --dir web run test -- web/test/pages/market web/test/shell` passes (the registration finds `market_page` once).
2. The test renders `ok` and checks that `ok.weather.headline`, the index names and the figure texts of every `IndexRow`, the term sentence and the breadth rows are on the page, in the order weather, indices, term, breadth (`compareDocumentPosition`).
3. The test renders `gap` and checks that the N225 and TOPX rows show the cause text of their `Gap`, that a `term` gap shows its text and no chart, and that `registration.page.validate` refuses `bad-version.json`.
4. `grep -c "export const registration" web/src/pages/market/MarketPage.tsx` prints 1; `grep -c "SUPPORTED_VERSIONS = \[1\]" web/src/types/market_page.ts` prints 1; `grep -c "fetch(\|toFixed\|parseFloat\|Intl.NumberFormat\|reduce(" web/src/pages/market/MarketPage.tsx` prints 0; `wc -l web/src/pages/market/MarketPage.tsx` shows under 600.
5. `grep -c "satisfies MarketPage" web/test/pages/market/fixtures/ok.ts` prints 1; the manifest has `marker` `synthetic-v1` and lists exactly the four fixture files; `grep -c "@" web/test/pages/market/fixtures/ok.ts` prints 0.
6. `git diff $(git merge-base HEAD origin/main) -- web/package.json web/src/shell web/src/generated` is empty; `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the eight paths of the Files line.

Checked by CI: `pnpm install --frozen-lockfile`, tsc, eslint, vitest, build; hygiene (the manifest of the new `fixtures` directory, the privacy scan); `Contracts: unchanged`; the design verdict (Decisions 2 to 4).

### price-lines-view

- Module: web. Size: M. Depends on: page-schemas-four.
- Order: after page-schemas-four.
- Design: docs/design/missing-pages.md#Decisions
- Files: web/src/types/price_lines_page.ts, web/src/pages/pricelines/PriceLinesPage.tsx, web/test/pages/pricelines/PriceLinesPage.test.tsx, web/test/pages/pricelines/fixtures/ok.ts, web/test/pages/pricelines/fixtures/gap.ts, web/test/pages/pricelines/fixtures/stale.ts, web/test/pages/pricelines/fixtures/bad-version.json, web/test/pages/pricelines/fixtures/MANIFEST.synthetic-v1.json

- Goal: The page `PriceLinesPage` shows the intro, the saved message, one row per saved line with price and note and a save and a delete button, the add form (code, market, price, note) and, below, the held companies with no price line, each with a prefilled add form. Every form is a plain `<form method="post" action=...>` with hidden `expires` and `mac` fields (the precedent of `settings_page`). `web/src/types/price_lines_page.ts` takes `Payload` from `PriceLinesPage` of `@/generated/price_lines_page.v1` with `SUPPORTED_VERSIONS = [1]` (the pattern of `web/src/types/home_page.ts`). `PriceLinesPage.tsx` exports `registration = { schema: 'price_lines_page', page: page<Payload>(validate, PriceLinesPage) }`, shows `stale` and `gaps` with the `StaleAlert` and `CardNotes` of `web/src/pages/home` (imported, not edited), prints every text from the payload, and compares, sums and formats nothing.
  `ok.ts` is typed `satisfies PriceLinesPage`; `gap.ts` carries a missing figure with its cause; `stale.ts` a stale label; `bad-version.json` a `schema_version` of 2. The page has no route (Goal of this design); no TopNav item is added.
- 按 frontend-stack：用 shadcn card, table, button and alert (`components/ui`) 做 the saved lines, the held-but-unset list, the save and delete buttons and the `saved` and `error` messages; the inputs are native `input` and `select` styled with the classes of `button.tsx` (no new `components/ui` file)。

Acceptance:

1. `pnpm --dir web run test -- web/test/pages/pricelines web/test/shell` passes (the registration finds `price_lines_page` once).
2. The test renders `ok` and checks that each form has `method="post"` and `action="/plan/price-lines"`, carries hidden `expires` and `mac` inputs equal to `ok.form`, that every `ok.rows` symbol, price and note are on the page, and that each `ok.unset` symbol has its own form.
3. The test renders a payload with `error` set and checks that the text is on the page in an alert, that with `saved` null no saved message is rendered, and that `registration.page.validate` refuses `bad-version.json`.
4. `grep -c "export const registration" web/src/pages/pricelines/PriceLinesPage.tsx` prints 1; `grep -c "SUPPORTED_VERSIONS = \[1\]" web/src/types/price_lines_page.ts` prints 1; `grep -c "fetch(\|toFixed\|parseFloat\|Intl.NumberFormat\|reduce(" web/src/pages/pricelines/PriceLinesPage.tsx` prints 0; `wc -l web/src/pages/pricelines/PriceLinesPage.tsx` shows under 600.
5. `grep -c "satisfies PriceLinesPage" web/test/pages/pricelines/fixtures/ok.ts` prints 1; the manifest has `marker` `synthetic-v1` and lists exactly the four fixture files; `grep -c "@" web/test/pages/pricelines/fixtures/ok.ts` prints 0.
6. `git diff $(git merge-base HEAD origin/main) -- web/package.json web/src/shell web/src/generated` is empty; `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the eight paths of the Files line.

Checked by CI: `pnpm install --frozen-lockfile`, tsc, eslint, vitest, build; hygiene (the manifest of the new `fixtures` directory, the privacy scan); `Contracts: unchanged`; the design verdict (Decisions 2 to 4).

### core-page-view

- Module: web. Size: M. Depends on: page-schemas-four.
- Order: after page-schemas-four.
- Design: docs/design/missing-pages.md#Decisions
- Files: web/src/types/core_page.ts, web/src/pages/core/CorePage.tsx, web/test/pages/core/CorePage.test.tsx, web/test/pages/core/fixtures/ok.ts, web/test/pages/core/fixtures/gap.ts, web/test/pages/core/fixtures/stale.ts, web/test/pages/core/fixtures/bad-version.json, web/test/pages/core/fixtures/MANIFEST.synthetic-v1.json

- Goal: The page `CorePage` shows the sentence, the `basis_note`, the dot bar scale with the treasury line, one row per company (price, the first assumption's return, the flag, the cyclical mark) that opens to its returns, a `Gap` row ("估值数据不足") with its cause, the JP note and the legend. `web/src/types/core_page.ts` takes `Payload` from `CorePage` of `@/generated/core_page.v1` with `SUPPORTED_VERSIONS = [1]` (the pattern of `web/src/types/home_page.ts`). `CorePage.tsx` exports `registration = { schema: 'core_page', page: page<Payload>(validate, CorePage) }`, shows `stale` and `gaps` with the `StaleAlert` and `CardNotes` of `web/src/pages/home` (imported, not edited), prints every text from the payload, and compares, sums and formats nothing.
  `ok.ts` is typed `satisfies CorePage`; `gap.ts` carries a missing figure with its cause; `stale.ts` a stale label; `bad-version.json` a `schema_version` of 2. The page has no route (Goal of this design); no TopNav item is added.
- 按 frontend-stack：用 shadcn accordion (`components/ui/accordion.tsx`) 做 the row fold that opens the assumptions of one company。
- 按 frontend-stack：用 shadcn progress (`components/ui/progress.tsx`) 做 the 0 to 15% dot bar, with the dot at the producer's `marker_percent` and the treasury line as a second marker。
- 按 frontend-stack：用 shadcn card and badge (`components/ui`) 做 the header, the "高过国债" flag and the cyclical mark。

Acceptance:

1. `pnpm --dir web run test -- web/test/pages/core web/test/shell` passes (the registration finds `core_page` once).
2. The test renders `ok` and checks that each company's `returns[i].text` are in its accordion item, that the dot of each return has the style `left` equal to its `marker_percent.value` (read from the fixture, no arithmetic) and the treasury marker `scale.treasury.marker_percent`, and that a row with `returns` as a `Gap` shows the gap text and no dot.
3. The test checks that `jp_note`, `legend` and `snapshot_text` are on the page and that `registration.page.validate` refuses `bad-version.json`.
4. `grep -c "export const registration" web/src/pages/core/CorePage.tsx` prints 1; `grep -c "SUPPORTED_VERSIONS = \[1\]" web/src/types/core_page.ts` prints 1; `grep -c "fetch(\|toFixed\|parseFloat\|Intl.NumberFormat\|reduce(" web/src/pages/core/CorePage.tsx` prints 0; `wc -l web/src/pages/core/CorePage.tsx` shows under 600.
5. `grep -c "satisfies CorePage" web/test/pages/core/fixtures/ok.ts` prints 1; the manifest has `marker` `synthetic-v1` and lists exactly the four fixture files; `grep -c "@" web/test/pages/core/fixtures/ok.ts` prints 0.
6. `git diff $(git merge-base HEAD origin/main) -- web/package.json web/src/shell web/src/generated` is empty; `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the eight paths of the Files line.

Checked by CI: `pnpm install --frozen-lockfile`, tsc, eslint, vitest, build; hygiene (the manifest of the new `fixtures` directory, the privacy scan); `Contracts: unchanged`; the design verdict (Decisions 2 to 4).

### diagnosis-page-view

- Module: web. Size: M. Depends on: page-schemas-four.
- Order: after page-schemas-four.
- Design: docs/design/missing-pages.md#Decisions
- Files: web/src/types/diagnosis_page.ts, web/src/pages/diagnosis/DiagnosisPage.tsx, web/test/pages/diagnosis/DiagnosisPage.test.tsx, web/test/pages/diagnosis/fixtures/ok.ts, web/test/pages/diagnosis/fixtures/gap.ts, web/test/pages/diagnosis/fixtures/stale.ts, web/test/pages/diagnosis/fixtures/bad-version.json, web/test/pages/diagnosis/fixtures/MANIFEST.synthetic-v1.json

- Goal: The page `DiagnosisPage` shows the sentence, the `basis_note`, the cyclical group and the financial group, one fold per company with the latest year, the rank text ("3/10"), the extra ratio, the usual value and, opened, the year table with its mini bars; a company with too few years or no US GAAP report is a `Gap` row with its cause. `web/src/types/diagnosis_page.ts` takes `Payload` from `DiagnosisPage` of `@/generated/diagnosis_page.v1` with `SUPPORTED_VERSIONS = [1]` (the pattern of `web/src/types/home_page.ts`). `DiagnosisPage.tsx` exports `registration = { schema: 'diagnosis_page', page: page<Payload>(validate, DiagnosisPage) }`, shows `stale` and `gaps` with the `StaleAlert` and `CardNotes` of `web/src/pages/home` (imported, not edited), prints every text from the payload, and compares, sums and formats nothing.
  `ok.ts` is typed `satisfies DiagnosisPage`; `gap.ts` carries a missing figure with its cause; `stale.ts` a stale label; `bad-version.json` a `schema_version` of 2. The page has no route (Goal of this design); no TopNav item is added.
- 按 frontend-stack：用 shadcn accordion (`components/ui/accordion.tsx`) 做 one fold per company that opens its year table。
- 按 frontend-stack：用 @tanstack/react-table (`web/src/components/data-table/data-table.tsx`) 做 the per-year table (年, 收入, margin, 库存 ÷ 收入 or ROA)。
- 按 frontend-stack：用 shadcn card and badge (`components/ui`) 做 the two groups and the rank text; the per-year mini bars are divs whose width is the producer's `bar_percent`。

Acceptance:

1. `pnpm --dir web run test -- web/test/pages/diagnosis web/test/shell` passes (the registration finds `diagnosis_page` once).
2. The test renders `ok` and checks that for every `DiagRow` the `latest`, `rank_text`, `extra` and `usual` texts are on the page, that the opened year table has one row per `years` entry with the mini bar style `width` equal to `bar_percent` of the fixture, and that the two group titles appear in the order cyclical, financial.
3. The test checks that a `Gap` row shows its cause text and no year table, and that `registration.page.validate` refuses `bad-version.json`.
4. `grep -c "export const registration" web/src/pages/diagnosis/DiagnosisPage.tsx` prints 1; `grep -c "SUPPORTED_VERSIONS = \[1\]" web/src/types/diagnosis_page.ts` prints 1; `grep -c "fetch(\|toFixed\|parseFloat\|Intl.NumberFormat\|reduce(" web/src/pages/diagnosis/DiagnosisPage.tsx` prints 0; `wc -l web/src/pages/diagnosis/DiagnosisPage.tsx` shows under 600.
5. `grep -c "satisfies DiagnosisPage" web/test/pages/diagnosis/fixtures/ok.ts` prints 1; the manifest has `marker` `synthetic-v1` and lists exactly the four fixture files; `grep -c "@" web/test/pages/diagnosis/fixtures/ok.ts` prints 0.
6. `git diff $(git merge-base HEAD origin/main) -- web/package.json web/src/shell web/src/generated` is empty; `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the eight paths of the Files line.

Checked by CI: `pnpm install --frozen-lockfile`, tsc, eslint, vitest, build; hygiene (the manifest of the new `fixtures` directory, the privacy scan); `Contracts: unchanged`; the design verdict (Decisions 2 to 4).

## Left out

- Board 22 (Backtest, short put): where the options are priced is undecided until the backtest starts, and plan rev 2.1 allows no option pricing; no pure-render part stands alone. It waits for that decision.
- Every producer of the new pages: the 200-day line, RV20, IV−RV20 and the 3-year rank, VIX3M and the term curve, breadth (market); the returns at today's price and the treasury comparison (core); the ten-year ratios, ranks and the cyclical or financial class (diagnosis): wait for the compute design. The price-line values are the owner's own input and need no compute.
- The saving of price lines (the POST handler in `cmd/ot`, where the lines are kept, the rewrite of the file and its snapshot, the plan-edit switch): an `ot` and store design of its own; this design renders the form and posts to the constant path only.
- The `ot` routes `/market`, `/plan/price-lines`, `/core` and `/diagnosis` and the TopNav items (`cmd/ot` belongs to the `ot` module): a route design after the producers. Until then no page here is reachable.
- Builders (`Compose` functions) of the four pages: with no producer there is nothing to compose. Links from the company page to core and diagnosis: unchanged.

## Adversarial

The review is kept by the author. Answers, by its numbering.

- Verdict 2 and 3, 2.D: adopted. The Goal says the four pages are unreachable (no route) and show "缺" until the compute design; Left out names the route design. Verdict 4 and 4.4, 4.5: adopted, the shapes are loose (`ReturnPoint` 1 to 6, `years` at most 12, no exact counts) and the schemas are one issue; the lock-in that remains is the four new files, which a producer design may supersede by a new version.
- 3.3 merge the four schema issues: adopted, `page-schemas-four` (13 files, Size L, one module). 5.6 `generate:types` acceptance, top 4: adopted, replaced by `TestGeneratedTypes` and `TestNewSchemasAreOurs` offline, a CI drift check, and the first-run command that names the four schemas. 5.5 `placeholderValidate` count: adopted, the check is deleted.
- 6.1 offline acceptance: adopted. 6.2 conflicts on `page-schemas.md`: adopted (one edit, one issue). 6.3 size rule: adopted, `wc -l` under 600 for each schema. 6.5 permanently empty pages: adopted, said in the Goal.
- Per-file check MP, top 6 format break at lines 154 to 160: adopted, all four view issues are regenerated with acceptance items 1 to 6 in order and complete; the stray item is gone.
- 1.7 boards 11 and 23: no board 11 (merged into 23), board 23 is the component sheet; no question. 1.1 to 1.6, 2.A to 2.C, 3.1, 3.2, 3.4 to 3.6, 4.1 to 4.3, 5.1 to 5.4, 5.7, 5.8, 6.4: concern other files and are answered there. 2.B: new web fixture directories are not Go-validated and hold no copy of an old one.
- Top 1: not applicable (new files only, each under 600 lines). Top 2, 3, 5: other files. Top 4, 6: adopted.
