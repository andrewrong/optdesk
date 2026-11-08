# Tracking, indicator, reports, risk, ledger, structures and macro pages: close the board deviations

Author: the optdesk designer (second draft, after the adversarial review)
Owner approved: (pending)
Revision: 3

Source: the page comparison of 2026-11-08 (design set v2.2, boards 2, 3, 4, 8, 9, 20, 21, 24, 25, plus the later owner changes #1457, #1458, #1544, #1644, #1741 to #1761, #1766, #1774, #1778, #1844, #1845, #1854). The owner chose to turn every deviation into a design.
Read on origin/main at 3ef73b1: `contracts/schemas/page.tracking.v1.json` (331 lines), `page.indicator_detail.v1.json` (443 lines), `page.report_page.v2.json`, `page.ledger_page.v1.json`, `page.structures_list.v1.json`, `page.macro_page.v1.json`, `page.macro_summary.v2.json`,
`internal/contracts/fixtures_test.go`, `web/src/pages/options/Tracking.tsx`, `web/src/pages/indicator/view.tsx`, `web/src/pages/report/frame.tsx`, `web/src/pages/report/global-assets.tsx`, `web/src/pages/report/open-blocks.tsx`, `web/src/pages/ledger/SectionTables.tsx`, `web/src/pages/structure/StructuresList.tsx`,
`web/src/pages/structuredetail/Payoff.tsx`, `web/src/charts/spec.ts`, `web/src/pages/macro/MacroPage.tsx`, `web/fixtures/**`, `web/test/pages/**`, `web/src/types/tracking.ts`, `web/src/types/indicator_detail.ts`.

Goal: the tracking page groups by rule version with the board's summary tiles, the covered-call section and "My trades"; the indicator page has its last-12-periods table and its source line; the reports lose their skeleton text and name their bar series; the ledger draws its cumulative line; the structures list shows its compare row;
the macro page reads -2 to +2; the risk page says its result in plain words. Pages show "缺" for numbers without a producer until the compute design. Producer-written numbers (HV30, IVP, delta, the ease value, tile figures) wait for the compute design's register of metric ids: the live page shows 缺 until a producer exists, and the fixtures are the only source of their values. Schema field descriptions of such fields say "metric id: assigned by compute". No page here gets a route in this design; each is served by `ot` in a later design (`docs/design/option-page-serve.md`).

## What main holds today

- `tracking.v1` and `indicator_detail.v1` are carried files; both fit under 600 lines when copied. `tracking.v1` has one flat `rows` list and the text `pending` ("暂未迁移"); `indicator_detail.v1` has `latest`, `points`, `revision`, `reference_line` and `data_times`, no change column and no source line.
- `report_page.v2` blocks already carry `data_times` text and `state`; `frame.tsx` renders a fold of the data times and a badge for `state !== 'ok'` with the raw word ("partial"); `open-blocks.tsx` already has the `rest` accordion titled `rest_title`. What is left: `frame.tsx` still renders `design_note`, `global-assets.tsx` names its series `day` and `five_day`, and the open fixture holds close blocks as well.
- `SectionTables.tsx` of the ledger renders only `chart.summary` as a paragraph; no chart is drawn from `chart.points` (`ChartPoint {day, text, value}`).
- `ChartSpec` (`web/src/charts/spec.ts`) has `kind` line or bar, categories and series, with no reference line or area; `Payoff.tsx` shows the break-even and the band as badges.
- `Ease` of `macro_page.v1` and `macro_summary.v2` stores the marker on a 0 to 100 track with free-text `ticks`.
- `web/fixtures/macro-page`, `structures-list` and `household-page` are validated by `internal/contracts/fixtures_test.go` against their v1 schemas; an edit that stays valid passes. The household fixture already has the title "家庭资产" and every board 12 block, including the 女儿 scope and the waterfall (`month_change`).
- None of these pages has a Go builder on main.

## Decisions

1. Carried pages that need a field get a new file: `page.tracking.v2.json` and `page.indicator_detail.v2.json`, both written in one contracts issue so `page-schemas.md` is edited once. Everything else is a fixture, test or view change.
2. Tracking v2 is grouped by rule version, with loose shapes so the compute design is not pre-empted: `versions` (1 to 10 `RuleVersion`); `RuleVersion {label, expired_text, groups (1 to 6 RuleGroup), covered_calls (CoveredCalls or null), my_trades (MyTrades or Gap)}`; `RuleGroup {key (Id string), title, summary (a list of 1 to 8 `{label, text}` tiles, or a Gap), note (string or null), rows (TrackingRow list)}`;
   `CoveredCalls {title, sub, groups, comparison (a table {columns, rows of strings}), note}`; `MyTrades {title, sub, columns, rows of strings, note}`. The tile labels (给出, 已到期, 跌破, 平均结果, 比国债) are the producer's text, not an enum; the view prints and computes nothing. Older versions are later entries of `versions`, folded. `my_trades` stays a Gap with its cause until a producer exists.
3. Indicator v2 adds `meta_line` (string: "来源 FRED · 数据截至 2026-10-21 · 及时") and `recent` (a `RecentBlock {title, columns, rows}` of at most 12 rows of strings `{period, released, value, change}`, or a Gap). The 较前 text is the producer's.
4. Reports: the badge words below present the closed enum `state` of the schema, not a decision of the view on free text. `design_note` is never rendered (the field stays in the schema); the badge words for `state` are 延迟 (`partial`) and 缺 (`gap`), shell text; the bar series are named 当日 and 五日; the open fixture holds only the open blocks and the close fixture only the close blocks, with 系统状态 once and the links 账户波动 to `/household` and 风险点 to `/positions` (#1766, #1778).
5. Ledger: the view draws the cumulative realized line from `section.chart.points`; the summary text stays.
6. The macro scale is settled from the schema: the stored track is 0 to 100 and the page computes nothing; the mapping from the ease value to the track and to the labels -2..+2 lives in the producer (the Go macro page builder, which waits for the compute design). The view draws `marker_percent` and prints `ticks` and `reading_text`; the fixture writes `ticks` "-2" to "+2" and a neutral band as track percents with the texts "-0.5" and "+0.5".
7. The daughter stays inside the household total (owner #1773); no issue of this design splits it. The household page already shows the 女儿 scope and the waterfall (#1760, #1761), so it needs no issue.
8. Missing is shown as missing, with the cause once. JPY carries ≈USD. Option terms stay English.

## Modules

None changed. `contracts/**` belongs to contracts; `web/**` to web.

| issue | module | order |
|---|---|---|
| tracking-indicator-schemas | contracts (interface) | first |
| tracking-view | web | after tracking-indicator-schemas |
| indicator-view | web | after tracking-indicator-schemas |
| report-view-fixes | web | independent |
| risk-fixtures | web | independent |
| ledger-chart-view | web | independent |
| structures-list-fixtures | web | independent |
| macro-page-fixtures | web | independent |

## Interface

Owner: contracts. Frozen by `tracking-indicator-schemas`, read by `tracking-view` and `indicator-view`.

- `contracts/schemas/page.tracking.v2.json` (`$id` `https://optdesk.invalid/schemas/page.tracking.v2.json`, title `Tracking`, `schema` const `tracking`, `schema_version` const 2, `additionalProperties` false): the envelope of v1 (`run_id`, `build_id`, `scope`, `generated_at`, `data_times`, `run_status`, `stale`, `title`, `sentence`, `how`, `note`) and `versions` as Decision 2 gives it, in place of `rows` and `pending`.
  `TrackingRow` is copied from v1.
- `contracts/schemas/page.indicator_detail.v2.json` (title `IndicatorDetail`, `schema_version` const 2): the v1 content plus the required `meta_line` and `recent` of Decision 3.
- Generated types: `web/src/generated/tracking.v2.ts` and `web/src/generated/indicator_detail.v2.ts`, both new; `indicator_detail.v1.ts` stays.

## Issues

### tracking-indicator-schemas (interface)

- Module: contracts. Size: M. Depends on: option-data-fields-schema (option-pages-v22.md).
- Order: first of this design, "after" option-pages-v22 `option-data-fields-schema`; the three interface issues that edit `docs/contracts/page-schemas.md` land in the order option-pages-v22, tracking-household-reports, missing-pages.
- Design: docs/design/tracking-household-reports.md#Interface
- Files: contracts/schemas/page.tracking.v2.json, contracts/schemas/page.indicator_detail.v2.json, docs/contracts/page-schemas.md, internal/contracts/tracking_v2_test.go, internal/contracts/indicator_detail_v2_test.go, web/src/generated/tracking.v2.ts, web/src/generated/indicator_detail.v2.ts

- Goal: write the two schemas of the Interface section; `page.tracking.v1.json` and `page.indicator_detail.v1.json` stay byte-identical. Neither new file is over 600 lines. `page-schemas.md` gets the two rows. The generated types are produced by `pnpm --dir web run generate:types page.tracking.v2 page.indicator_detail.v2` (the first run of a new schema names it) and committed.
  The tests (new, pattern of `home_page_v2_test.go`): the smallest instance validates; version 1 fails at `/schema_version`; `versions` empty fails; a `summary` given as a Gap validates; `my_trades` as a Gap validates; `recent` with 13 rows fails and `recent` as a Gap validates.

Acceptance:

1. `go test ./internal/contracts -race -count=1` passes; `grep -c "t.Parallel()" internal/contracts/tracking_v2_test.go internal/contracts/indicator_detail_v2_test.go` is at least the number of `func Test` lines in each file; `grep -c "time.Sleep" internal/contracts/tracking_v2_test.go internal/contracts/indicator_detail_v2_test.go` prints 0 for each.
2. `go test ./internal/contracts -run 'TestGeneratedTypes|TestNewSchemasAreOurs' -count=1` passes (banner, schema and exported interface of each generated file; the new schemas need no hash line).
3. `wc -l contracts/schemas/page.tracking.v2.json contracts/schemas/page.indicator_detail.v2.json` shows each under 600; `git diff $(git merge-base HEAD origin/main) -- contracts/schemas/page.tracking.v1.json contracts/schemas/page.indicator_detail.v1.json contracts/schemas/carryover.sha256 web/src/generated/indicator_detail.v1.ts` is empty.
4. `grep -c "interface RuleGroup" web/src/generated/tracking.v2.ts` prints 1; `grep -c "meta_line" web/src/generated/indicator_detail.v2.ts` is at least 1.
5. `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the seven paths of the Files line.

Checked by CI: ci-go; `pnpm install --frozen-lockfile`, then `generate:types` leaves the committed files unchanged, tsc, eslint, vitest, build; the pull request body carries `Contracts: page.tracking.v2, page.indicator_detail.v2 (new)`; the design verdict (Decisions 2 and 3).

### tracking-view

- Module: web. Size: L. Depends on: tracking-indicator-schemas.
- Order: after tracking-indicator-schemas.
- Design: docs/design/tracking-household-reports.md#Decisions
- Files: web/src/types/tracking.ts, web/src/pages/options/Tracking.tsx, web/test/pages/options/Tracking.test.tsx, web/test/pages/options/tracking-v2/fixtures/ok.ts, web/test/pages/options/tracking-v2/fixtures/gap.ts, web/test/pages/options/tracking-v2/fixtures/stale.ts, web/test/pages/options/tracking-v2/fixtures/bad-version.json, web/test/pages/options/tracking-v2/fixtures/MANIFEST.synthetic-v1.json

- Goal: Decision 2. `web/src/types/tracking.ts` takes `Payload` from `Tracking` of `@/generated/tracking.v2` with `SUPPORTED_VERSIONS = [2]`. The page shows the headline, "怎么算的", then for the current version its label and `expired_text`, each group's tiles (a Gap summary prints its text once; a `note` is printed under the title), the group rows in a fold, the covered-call section with its groups and comparison table,
  and the My trades table or its gap; older versions are folded under "更早的规则版本". The v2 fixtures are new TypeScript files typed `satisfies Tracking` and carry the board's numbers as text (31, 24, 3, +0.9%, +0.2%; the group with "太早，不下结论。"; the My trades rows).
  `web/fixtures/tracking` (v1) is not edited; the Go fixture test keeps validating it. Nothing is averaged or compared in the view.
- 按 frontend-stack：用 shadcn card and badge (`components/ui`) 做 the group tiles and the version heads。
- 按 frontend-stack：用 shadcn accordion (`components/ui/accordion.tsx`) 做 the folded rows and the older versions。
- 按 frontend-stack：用 @tanstack/react-table (`components/data-table/data-table.tsx`) 做 the rows, comparison and My trades tables。

Acceptance:

1. `pnpm --dir web run test -- web/test/pages/options/Tracking.test.tsx` passes; `go test ./internal/contracts -run TestFixturesValidate -count=1` passes (the v1 fixtures are untouched).
2. The test renders `ok` and checks that every tile label and text of each group of `ok.versions[0]` is on the page, that "太早，不下结论。" appears for the group that carries it, and that the covered-call comparison rows and the My trades rows are on the page.
3. The test checks that a `my_trades` gap renders its text and no table, that older versions sit in a closed accordion item, and that `registration.page.validate` refuses `bad-version.json` and accepts `ok`.
4. `grep -c "SUPPORTED_VERSIONS = \[2\]" web/src/types/tracking.ts` prints 1; `grep -c "reduce(\|toFixed\|parseFloat\|Intl.NumberFormat" web/src/pages/options/Tracking.tsx` prints 0; `wc -l web/src/pages/options/Tracking.tsx` shows under 600; `grep -c "satisfies Tracking" web/test/pages/options/tracking-v2/fixtures/ok.ts` prints 1; `grep -c "@" web/test/pages/options/tracking-v2/fixtures/ok.ts` prints 0.
5. `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the eight paths of the Files line; `git diff $(git merge-base HEAD origin/main) -- web/fixtures/tracking` is empty.

Checked by CI: `pnpm install --frozen-lockfile`, tsc, eslint, vitest, build; hygiene (the manifest of the new `fixtures` directory, the privacy scan); `Contracts: unchanged`; the design verdict (Decision 2).

### indicator-view

- Module: web. Size: M. Depends on: tracking-indicator-schemas.
- Order: after tracking-indicator-schemas.
- Design: docs/design/tracking-household-reports.md#Decisions
- Files: web/src/types/indicator_detail.ts, web/src/pages/indicator/view.tsx, web/test/pages/indicator/view.test.tsx, web/test/pages/indicator/fixtures/ok.ts, web/test/pages/indicator/fixtures/gap.ts, web/test/pages/indicator/fixtures/stale.ts, web/test/pages/indicator/fixtures/bad-version.json

- Goal: Decision 3 on board 9. Types come from `@/generated/indicator_detail.v2` with `SUPPORTED_VERSIONS = [2]`. The page shows the latest value with its release date, the line chart on the release-date axis, `meta_line`, then the "最近 12 期" table (期, 公布日, 值, 较前) from `recent`, or its gap text. The fixtures are v2 and in Chinese; the explanation block stays.
- 按 frontend-stack：用 recharts through `web/src/charts/Chart.tsx` 做 the line over release dates。
- 按 frontend-stack：用 @tanstack/react-table (`components/data-table/data-table.tsx`) 做 the last-12-periods table。
- 按 frontend-stack：用 shadcn badge and card (`components/ui`) 做 the header。

Acceptance:

1. `pnpm --dir web run test -- web/test/pages/indicator` passes.
2. The test checks that the four column headers are 期, 公布日, 值 and 较前, that every row of `ok.recent.rows` shows its four texts, that `meta_line` is on the page, and that a `recent` gap shows its text and no table.
3. `grep -c "SUPPORTED_VERSIONS = \[2\]" web/src/types/indicator_detail.ts` prints 1; `grep -c "toFixed\|parseFloat\|Intl.NumberFormat" web/src/pages/indicator/view.tsx` prints 0; `wc -l web/src/pages/indicator/view.tsx` shows under 600; `grep -c "@" web/test/pages/indicator/fixtures/ok.ts` prints 0.
4. `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the seven paths of the Files line.

Checked by CI: `pnpm install --frozen-lockfile`, tsc, eslint, vitest, build; hygiene (the fixture manifest keeps its file set); `Contracts: unchanged`; the design verdict (Decision 3).

### report-view-fixes

- Module: web. Size: S. Depends on: none.
- Order: independent.
- Design: docs/design/tracking-household-reports.md#Decisions
- Files: web/src/pages/report/frame.tsx, web/src/pages/report/global-assets.tsx, web/test/pages/report/view.test.tsx, web/test/pages/report/fixtures/ok-open.ts, web/test/pages/report/fixtures/ok-close.ts

- Goal: Decision 4. `frame.tsx` never renders `design_note` and writes the state badge as 延迟 or 缺; `global-assets.tsx` names its bar series 当日 and 五日. `ok-open.ts` holds the open blocks only (market line, tiles, accounts, candidates by sector, watch list, reminders, system status); `ok-close.ts` holds the five close blocks and `system_status` once,
  with `account_moves.link.href` `/household` and `risks.link.href` `/positions`. Neither fixture has `design_note`. The data-times fold and the reminder fold already exist and are not edited.
- 按 frontend-stack：用 recharts (`BarChart` through `components/ui/chart.tsx`) 做 the global-assets bars。
- 按 frontend-stack：用 shadcn badge and card (`components/ui`) 做 the state badges and blocks。

Acceptance:

1. `pnpm --dir web run test -- web/test/pages/report` passes.
2. The test renders `ok-close` and checks that the five block titles appear in the board order, that the two links have the hrefs above, that "系统状态" appears once, and that neither "day" nor "five_day" is on the page.
3. The test renders `ok-open` and checks that no close-only block is rendered (`data-block` of `account_moves`, `risks` and `global_assets` count 0) and that a block with `state` `partial` shows 延迟 and not "partial".
4. The test renders a payload whose `design_note` is set and checks that the note text is not on the page; `grep -c "design_note" web/src/pages/report/frame.tsx` prints 0; `grep -c "design_note" web/test/pages/report/fixtures/ok-open.ts web/test/pages/report/fixtures/ok-close.ts` prints 0 for each file.
5. `grep -c "toFixed\|parseFloat\|Intl.NumberFormat" web/src/pages/report/frame.tsx web/src/pages/report/global-assets.tsx` prints 0 for each file; `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the five paths of the Files line.

Checked by CI: `pnpm install --frozen-lockfile`, tsc, eslint, vitest, build; hygiene; `Contracts: unchanged`; the design verdict (Decision 4).

### risk-fixtures

- Module: web. Size: S. Depends on: none.
- Order: independent. Fixture and test only.
- Design: docs/design/tracking-household-reports.md#Decisions
- Files: web/test/pages/risk/fixtures/ok.ts, web/test/pages/risk/riskPage.test.tsx

- Goal: board 25. The `sentence` of the fixture says what the numbers say in plain words (the largest delta underlying, the worst sigma move) instead of generic text; every block keeps its `meaning` sentence (owner feedback 7); the standard-metrics block holds at least four metrics each with its explanation. Board 25 has no waterfall; the floating waterfall is board 12's block on the household page.

Acceptance:

1. `pnpm --dir web run test -- web/test/pages/risk/riskPage.test.tsx` passes.
2. The test checks that `ok.sentence`, the `meaning` text of each of the delta, beta delta, sigma and vega blocks and every metric `explanation` are on the page.
3. `grep -c "@" web/test/pages/risk/fixtures/ok.ts` prints 0; `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the two paths of the Files line.

Checked by CI: `pnpm install --frozen-lockfile`, tsc, eslint, vitest, build; hygiene; `Contracts: unchanged`; the design verdict (board 25, owner feedback 7).

### ledger-chart-view

- Module: web. Size: S. Depends on: none.
- Order: independent.
- Design: docs/design/tracking-household-reports.md#Decisions
- Files: web/src/pages/ledger/SectionTables.tsx, web/test/pages/ledger/view.test.tsx, web/test/pages/ledger/fixtures/ok.ts

- Goal: Decision 5, board 20. `SectionTables.tsx` draws a line chart from `section.chart.points` (category `day`, value `value`, tooltip `text`) above the summary text, and nothing when `chart` is null. `ok.ts` carries a chart of at least 10 points per USD section, the strategy columns Strategy, Closed, Realized, Ann. and the n<30 mark, and for the JPY section `approx_usd` text and the FX note of the header.
- 按 frontend-stack：用 recharts (`LineChart` through `web/src/charts/Chart.tsx`, `ChartSpec` of `web/src/charts/spec.ts`) 做 the cumulative realized line。

Acceptance:

1. `pnpm --dir web run test -- web/test/pages/ledger/view.test.tsx` passes.
2. The test checks that a section with a chart renders one line chart (`data-chart` marker) with as many points as `ok` has, shows `chart.summary`, that a section without `chart` renders none, that the header "Ann." is on the page and that the FX note text is on the page.
3. `grep -c "toFixed\|parseFloat\|Intl.NumberFormat" web/src/pages/ledger/SectionTables.tsx` prints 0; `wc -l web/src/pages/ledger/SectionTables.tsx` shows under 600; `grep -c "@" web/test/pages/ledger/fixtures/ok.ts` prints 0.
4. `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the three paths of the Files line.

Checked by CI: `pnpm install --frozen-lockfile`, tsc, eslint, vitest, build; hygiene; `Contracts: unchanged`; the design verdict (board 20).

### structures-list-fixtures

- Module: web. Size: S. Depends on: none.
- Order: independent. Fixture and test only; `ok.json` stays valid `structures_list` v1.
- Design: docs/design/tracking-household-reports.md#Decisions
- Files: web/fixtures/structures-list/ok.json, web/test/pages/structure/StructuresList.test.tsx

- Goal: the list fixture has the "本周要决定" group first and the later group second, and one row with `compare` filled (captured, close cost, exit DTE, roll net, cash after, commission; #1854), as board 6 and owner feedback 6 describe.

Acceptance:

1. `pnpm --dir web run test -- web/test/pages/structure/StructuresList.test.tsx` passes; `go test ./internal/contracts -run TestFixturesValidate -count=1` passes.
2. The test checks that the group titles appear in the payload's order and that the `compare.sentence` of the filled row is on the page.
3. `grep -c "@" web/fixtures/structures-list/ok.json` prints 0; `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the two paths of the Files line.

Checked by CI: `pnpm install --frozen-lockfile`, tsc, eslint, vitest, build; hygiene; `Contracts: unchanged`; the design verdict (owner feedback 6, #1854).

### macro-page-fixtures

- Module: web. Size: S. Depends on: none.
- Order: independent. Fixture and test only; `ok.json` stays valid `macro_page` v1.
- Design: docs/design/tracking-household-reports.md#Decisions
- Files: web/fixtures/macro-page/ok.json, web/test/pages/macro/MacroPage.test.tsx

- Goal: Decision 6, board 8. `ease` carries `ticks` "-2", "-1", "0", "+1", "+2", the marker and neutral band as track percents with their texts, and an `aria_text` that names the scale -2 to +2; the page keeps the gap sentence "没有缺口。" (or the named gap) below the indicator table; every indicator row has a status text and its `detail_href` (#1458);
  one indicator row is missing with its cause once.

Acceptance:

1. `pnpm --dir web run test -- web/test/pages/macro/MacroPage.test.tsx` passes; `go test ./internal/contracts -run TestFixturesValidate -count=1` passes.
2. The test checks that the first and last tick texts are "-2" and "+2", that the `Progress` value is `ok.ease.marker_percent.value`, that every `status` text is on the page, that each detail link has the fixture's `href`, and that the missing row shows its cause once.
3. `grep -c "@" web/fixtures/macro-page/ok.json` prints 0; `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the two paths of the Files line.

Checked by CI: `pnpm install --frozen-lockfile`, tsc, eslint, vitest, build; hygiene; `Contracts: unchanged`; the design verdict (board 8).

## Left out

- The builders of tracking, indicator, report, ledger, structure and risk pages and every number the new text carries (tracking averages and the comparison with the treasury, My trades, the ease value): they wait for the compute design and their own builder designs.
- The structure detail price chart with the break-even mark and band (board 24): `ChartSpec` has a category axis and no reference line or area, so it needs a numeric axis in the chart layer (`web/src/charts/**`); a design of its own. The badges stay.
- `household_page` (board 12): the fixture already carries the title and the twelve blocks, the 女儿 scope and the waterfall; nothing to design. `option_result_page` (board 19): matches.
- `positions_page` (board 6): the owner decided on 2026-11-08 that it stays as it is. `macro_rates`: the owner decided that it stays English. `advice_page` against board 10 (复盘): the owner chose not now; it is decided at strategy.
- Macro weather marks that follow the gauge limit (#1544): the comparison found no weather marks on `macro_page` to verify; not covered.
- Boards 11 and 23: there is no board 11 (merged into 23) and board 23 is the component sheet, not a page; nothing to compare.
- `companies_page` (board 13): its extra 波动 column stays; the owner dropped the volatility table on the home card only.
- Close-report valuation sections and covered-call extras: dropped by the owner. The `ot` routes of the pages here: later designs.

## Adversarial

The review is kept by the author. Answers, by its numbering.

- Verdict 2, 3, 4, 6: adopted. Two copies of carried files stay but are under 600 lines (331 and 443); the loose tile and group shapes remove the freeze of five tiles and three groups (4.1).
- 1.1 daughter: adopted, Decision 7 and the household fixture needs no issue; the old "never part of the total" text is gone. 1.3 `macro_rates` and #1544: macro_rates stays English by the owner; #1544 stated as not covered. 1.5 floating waterfall: adopted, it is board 12 on the household page (confirmed by the advisor). 1.6 ledger and structure detail are view gaps: adopted; ledger got a view change, the structure detail chart is left out with its reason.
  1.7 boards 11 and 23: no board 11 (merged into 23), board 23 is the component sheet; the owner question is removed. 1.2, 1.4: option-pages-v22.
- 2.B guarded fixtures, top 2: adopted. Tracking v2 fixtures are new TypeScript under `tracking-v2/fixtures`, `web/fixtures/tracking` is untouched; macro, structures-list and the others stay valid v1. 2.A size rule: both copies are under 600 lines, acceptance checks it. 2.C: home file. 2.D no route: adopted in the Goal.
- 3.3 merge schema issues: adopted (two became one). 3.5 `report-view-fixes` mostly done: adopted, cut to five files and four changes. 3.6 `household-fixtures` complete on main: adopted, the issue is removed. 3.4: home file.
- 4.1 exact tile counts: adopted. 4.2 `Part.whose`: home file (gone). 4.5: adopted, two new files only.
- 5.3 ledger draws no chart: adopted, `ledger-chart-view`. 5.4 no reference line in `ChartSpec`: adopted, left out. 5.5 `placeholderValidate` check: adopted, deleted. 5.6 `generate:types` acceptance: adopted, `TestGeneratedTypes` plus a CI drift check, first run names the schemas. 5.8 macro scale: adopted, Decision 6. 5.1, 5.2, 5.7: other files.
- 6.1 offline acceptance: adopted. 6.2 conflicts on `page-schemas.md`: adopted (one edit). 6.3 size: adopted. 6.4: adopted. 6.5 permanently empty: adopted, said in the Goal.
- Per-file check THR: `close-blocks.tsx` removed from Files; `web/src/types/tracking.v1.types.ts` becomes orphaned when `tracking.ts` moves to the generated v2 type; this is accepted and the file is not touched. The enum `state` to 延迟/缺 mapping in the view is presentation of a closed enum.
- Top 3: adopted (household, ledger, structures, report rewritten or dropped). Top 4: adopted. Top 5: adopted. Top 6: adopted. Top 1: not needed here (copies are small). Top 2: adopted.
