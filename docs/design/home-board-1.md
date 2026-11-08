# Home page: bring `home_page` to board 1 of design set v2.2

Author: the optdesk designer (second draft, after the adversarial review)
Owner approved: (pending)
Revision: 3

Source: the page comparison of 2026-11-08 (design set v2.2, owner-approved 2026-10-23, board 1, plus the later owner changes #1625, #1629, #1642, #1748, #1753, #1773, #1787, #1793, #1794, #1795). The owner chose to turn every deviation into a design.
Read on origin/main at 3ef73b1: `contracts/schemas/page.home_page.v2.json`, `page.household_summary.v2.json`, `page.market_summary.v3.json`, `page.options_summary.v3.json`, `page.macro_summary.v2.json`, `page.held_companies_summary.v2.json`,
`web/src/pages/home/*.tsx`, `web/test/pages/home/**`, `web/fixtures/v3/*`, `internal/contracts/fixtures_v3_test.go`, `docs/design/home-page.md`.

Goal: the home page reads as board 1: one headline, one column in the order macro, market, household, companies, options; every card has its header link; household shows the two-household total and its two part lines; the reports sit in a folded list;
the market card draws the SPX close with its 200-day line and band; the macro gauge reads -2 to +2; the options card labels its bars and shows what is already closed.
No schema changes: every value the board shows already has a text field. The page shows missing values as "缺" until the compute design gives them a producer. The page has no route yet: `/home` is served by `ot`, which is another design (`docs/design/option-page-serve.md`). Producer-written numbers (IVP, HV30, the 200-day line, the ease value) wait for the compute design's register of metric ids: the live page shows 缺 until a producer exists, and the fixtures are the only source of their values. Schema field descriptions of such fields say "metric id: assigned by compute".

Dropped by the owner and not brought back: the home detail view and the volatility table of the companies card.

## What main holds today

- `HomePage.tsx` shows a banner card with jump buttons and a two-column grid of the five cards in the order household, companies, macro, market, options. `home_page.v2` has `block_links` and `report_links` (each `BlockLink {href, text}`); it has `additionalProperties` false.
- The net-worth tile of `household_summary.v2` has `caption` and `explain` text; the part lines fit in `caption`.
- `Ease` of `macro_summary.v2` stores the marker on a 0 to 100 track (`PercentOfTrackFigure`, minimum 0, maximum 100) and 5 free-text `ticks`; the neutral band is two track percents with their own `text`.
- `SeriesPoint` of `options_summary.v3` has `premium` and `closed`, both a signed USD figure or a gap (`closed` is the amount already closed that month, not a flag); the card draws `premium` only and shows no value label.
- `web/fixtures/v3/market-summary/ok.json` already has `close`, `ma200`, `band_low` and `band_high` on four of five points and three `series_names`; a Go test (`internal/contracts/fixtures_v3_test.go`, `checkV3Tags`) refuses a `kind: gap` object in an `ok.json` of that directory, so a gap row goes to `gap.json`.

## Decisions

1. No new schema field and no Go change. Header links ride on the existing `block_links` in the board's card order (macro, market, household, companies, options): the view puts the link `i` above card `i`, and a missing link means no row. The report list rides on `report_links`: the first entry is visible, the rest sit in the fold "全部报告". Both are conventions of the view; the future Go builder of the home page (not designed) writes the lists in that order.
2. The daughter stays inside the household total; "女儿(单独计)" is a display split only (owner #1773, "daughter stays in totals; 单独计 = display split only"). The part lines are text in the net-worth tile `caption`: "家庭(本人+配偶) … · 女儿(单独计) … (已含在合计内)".
3. The macro scale is settled from the schema: the stored track is 0 to 100 and the page computes nothing. The mapping from the ease value to the track and to the labels -2..+2 lives in the producer (the Go macro summary, which waits for the compute design). The view draws `marker_percent` and prints `ticks` and `reading_text` as given; the fixture writes `ticks` "-2", "-1", "0", "+1", "+2", a marker and a neutral band as track percents whose `text` is "-0.5" and "+0.5".
4. 盘前/盘后: the cell is text (`EarningsCell.text`). The producer writes the qualifier when it knows it; the fixture shows it on the rows where the source says it and the date alone on one row, and the card note says once that the qualifier of that row has no source. Nothing is invented; the news column stays a gap with the cause.
5. Missing is shown as missing, once per card, with the cause. Fixture-only deviations (SPX 200-day line, NDX and RUT rows, wording "N 组") are fixed in the fixtures.
6. The options card draws two bar series: the month's `premium` and the amount already `closed`, each bar with a value label from the point's `text`, and the caption `premium_series_caption` under the chart (#1748). The view formats nothing.

## Modules

None changed. `web/**` belongs to web. No contracts or pages file is touched.

| issue | module | order |
|---|---|---|
| home-view-layout | web | independent |
| home-options-card-view | web | independent |
| home-market-macro-fixtures | web | independent |
| home-companies-household-fixtures | web | independent |

## Interface

None. No file of `contracts/` changes; the Contracts line of every pull request is `Contracts: unchanged`.

## Issues

### home-view-layout

- Module: web. Size: M. Depends on: none.
- Order: independent.
- Design: docs/design/home-board-1.md#Decisions
- Files: web/src/pages/home/HomePage.tsx, web/test/pages/home/HomePage.test.tsx, web/test/pages/home/home-v2/fixtures/ok.ts

- Goal: Decision 1. The headline is `title` and `say`, `gap_line` follows once. One column in the order macro, market, household, companies, options. Above card `i` a header row shows `block_links[i]` when it exists. After the cards a "报告" section shows `report_links[0]` and, in a fold "全部报告", the rest (no fold for a single link).
  The jump-button banner goes. The view has no route table and no `fetch(`. The five cards are imported, not edited. `ok.ts` has five block links and three report links.
- 按 frontend-stack：用 shadcn card, alert and button (`components/ui`) 做 the headline, the gap line and the header rows。
- 按 frontend-stack：用 shadcn accordion (`components/ui/accordion.tsx`) 做 the "全部报告" fold。

Acceptance:

1. `pnpm --dir web run test -- web/test/pages/home/HomePage.test.tsx` passes.
2. The test renders `ok` and checks by `compareDocumentPosition` on the five `data-card` markers that the order is macro, market, household, companies, options, and `grep -c "lg:grid-cols-2" web/src/pages/home/HomePage.tsx` prints 0.
3. The test checks that anchor `i` (`href` and `text` of `ok.block_links[i]`) sits above card `i`, that a payload with two block links renders two rows, that `ok.report_links[0]` is outside the fold and the other two are inside "全部报告", and that a payload with one report link has no fold.
4. `grep -c "fetch(" web/src/pages/home/HomePage.tsx` prints 0; `wc -l web/src/pages/home/HomePage.tsx` shows under 600; `grep -c "satisfies HomePage" web/test/pages/home/home-v2/fixtures/ok.ts` prints 1; `grep -c "@" web/test/pages/home/home-v2/fixtures/ok.ts` prints 0.
5. `git diff $(git merge-base HEAD origin/main) -- web/package.json web/src/shell web/src/pages/home/HouseholdCard.tsx web/src/pages/home/HeldCompaniesCard.tsx web/src/pages/home/MacroCard.tsx web/src/pages/home/MarketSummaryCard.tsx web/src/pages/home/OptionsSummaryCard.tsx contracts` is empty; `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the three paths of the Files line.

Checked by CI: `pnpm install --frozen-lockfile`, tsc, eslint, vitest, build; hygiene (fixture manifest, privacy scan); `Contracts: unchanged`; the design verdict (Decision 1).

### home-options-card-view

- Module: web. Size: S. Depends on: none.
- Order: independent.
- Design: docs/design/home-board-1.md#Decisions
- Files: web/src/pages/home/OptionsSummaryCard.tsx, web/test/pages/home/OptionsSummaryCard.test.tsx, web/fixtures/v3/options-summary/ok.json

- Goal: Decision 6. `spec.series` has two series: `premium` (named by `premium_series_text`) and `closed` (named "已了结"; a gap point is a null value with its text). Each point carries `label` set to its `text`, so the bar shows a value label; the caption `premium_series_caption` is shown under the chart. `ok.json` keeps its figures and README; its sold-tile
  text reads "本月卖出 5 组". The view formats nothing.
- 按 frontend-stack：用 recharts (`BarChart`, `Bar`) through `web/src/charts/Chart.tsx` and `components/ui/chart.tsx` 做 the two bar series and their value labels (`ChartPoint.label` of `web/src/charts/spec.ts`)。
- 按 frontend-stack：用 shadcn card (`components/ui`) 做 the tiles and the caption。

Acceptance:

1. `pnpm --dir web run test -- web/test/pages/home/OptionsSummaryCard.test.tsx` passes.
2. The test checks that the chart spec has two series, that one label per `premium_series_6m` point and one per `closed` point is rendered with the point's `text` (read from the fixture), and that the caption of the fixture is on the page.
3. The test checks that the sold tile text contains "组" and not "笔".
4. `grep -c "toFixed\|Intl.NumberFormat\|reduce(" web/src/pages/home/OptionsSummaryCard.tsx` prints 0; `wc -l web/src/pages/home/OptionsSummaryCard.tsx` shows under 600.
5. `go test ./internal/contracts -run TestFixtures -count=1` passes (the v3 fixture still validates, no gap object in `ok.json`); `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the three paths of the Files line.

Checked by CI: `pnpm install --frozen-lockfile`, tsc, eslint, vitest, build; hygiene; `Contracts: unchanged`; the design verdict (Decision 6).

### home-market-macro-fixtures

- Module: web. Size: M. Depends on: none.
- Order: independent. Fixture and test only; no view file changes.
- Design: docs/design/home-board-1.md#Decisions
- Files: web/fixtures/v3/market-summary/ok.json, web/fixtures/v3/market-summary/gap.json, web/fixtures/v3/market-summary/README.md, web/test/pages/home/MarketSummaryCard.test.tsx, web/test/pages/home/macro-v2/fixtures/ok.ts, web/test/pages/home/MacroCard.test.tsx

- Goal: Decisions 3 and 5. Market `ok.json`: at least 30 chart points each with `close`, `ma200`, `band_low` and `band_high` (the three `series_names` stay), and SPX, NDX and RUT rows (IV, RV20, IV−RV20). `gap.json`: the N225 row whose IV is a gap with its cause (a gap object never goes into `ok.json`).
  `README.md` states the new figures (it lists the hand-worked figures of `ok.json`). Macro `ok.ts`: `ticks` "-2", "-1", "0", "+1", "+2", a marker and a neutral band as track percents with texts "-0.5" and "+0.5", a prior value on every event and one event whose `previous` is missing and shown as missing.
- 按 frontend-stack：用 recharts through `web/src/charts/Chart.tsx` 做 the close, 200-day and band lines (unchanged view)。
- 按 frontend-stack：用 shadcn progress (`components/ui/progress.tsx`) 做 the gauge track (unchanged view)。

Acceptance:

1. `pnpm --dir web run test -- web/test/pages/home` passes; `go test ./internal/contracts -run TestFixtures -count=1` passes.
2. `MarketSummaryCard.test.tsx` checks that the chart has the three series named in `chart.series_names`, that the SPX, NDX and RUT rows are on the page with `ok`, and that with `gap` the N225 row shows the gap text.
3. `MacroCard.test.tsx` checks that the first tick is "-2" and the last "+2", that the neutral-band texts are on the page, that the `Progress` value is `ok.card.ease.marker_percent.value` (read, not computed) and that each event row shows its `previous` or the missing text.
4. `grep -c "kind\": \"gap" web/fixtures/v3/market-summary/ok.json` prints 0; `grep -c "@" web/fixtures/v3/market-summary/ok.json web/test/pages/home/macro-v2/fixtures/ok.ts` prints 0 for each file.
5. `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the six paths of the Files line.

Checked by CI: `pnpm install --frozen-lockfile`, tsc, eslint, vitest, build; hygiene (fixture manifests, privacy scan); `Contracts: unchanged`; the design verdict (Decisions 3 and 5).

### home-companies-household-fixtures

- Module: web. Size: S. Depends on: none.
- Order: independent. Fixture and test only.
- Design: docs/design/home-board-1.md#Decisions
- Files: web/test/pages/home/held-v2/fixtures/ok.ts, web/test/pages/home/HeldCompaniesCard.test.tsx, web/test/pages/home/household-v2/fixtures/ok.ts, web/test/pages/home/HouseholdCard.test.tsx

- Goal: Decisions 2 and 4. Held `ok.ts`: the `earnings` text carries 盘前 or 盘后 on the rows the source states and the date alone on one row; the card `notes` name that row's missing qualifier once; the news column is a gap stating the missing source. Household `ok.ts`: the net-worth tile `caption.text` reads
  "家庭(本人+配偶) … · 女儿(单独计) … (已含在合计内)" with amounts written as text.

Acceptance:

1. `pnpm --dir web run test -- web/test/pages/home/HeldCompaniesCard.test.tsx web/test/pages/home/HouseholdCard.test.tsx` passes.
2. The held test checks that the rows with a qualifier contain 盘前 or 盘后, that the other row shows the date alone and the note names it once, and that the news column shows the gap text.
3. The household test checks that the tile caption of the fixture, with both part labels and the words "已含在合计内", is on the page and that the total text is the fixture's `figure.text`.
4. `grep -c "@" web/test/pages/home/held-v2/fixtures/ok.ts web/test/pages/home/household-v2/fixtures/ok.ts` prints 0 for each file; `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the four paths of the Files line.

Checked by CI: `pnpm install --frozen-lockfile`, tsc, eslint, vitest, build; hygiene; `Contracts: unchanged`; the design verdict (Decisions 2 and 4).

## Left out

- The producers of the values the fixtures show: the 200-day line and its band (a compute figure; the SPX close is IBKR bars, #1793), RV20, IV−RV20 and IV percentile for NDX and RUT, the macro ease value and its mapping to -2..+2, the news text and the 盘前/盘后 flag: wait for the compute design and a news source. Until then they show "缺".
- The builder of the five summaries, a `Compose` call for `home_page` (it has no caller, and `ot` has no `home_page` constructor in `docs/design/modules.md`), and the `ot` route `/home`: a later design.
- A per-card link field and a `reports_more` field in the schema: not needed; the order convention of Decision 1 carries them. If the Go builder design finds the convention too weak it adds a field then, with a producer.
- The macro badge colour (#1794) and register tiles (#1753): found present by the comparison.
- The home detail view and the companies volatility table: dropped by the owner.

## Adversarial

The review is kept by the author. Answers, by its numbering.

- Verdict 2, 3, 4, 6 and section 6.3: adopted. No schema is edited, so there is no lock-in, no v2 file and no size problem here.
- 1.1 daughter in or out of the total: adopted, settled from owner #1773 in Decision 2 (inside the total, display split only); the old `whose` enum is gone.
- 1.2 PR1847 and the 7-day lookback: not a home matter; answered in option-pages-v22.
- 1.3, 1.4, 1.5, 1.6, 1.7: concern other files; answered there (1.7: no board 11, board 23 is the component sheet).
- 2.A size rule: adopted, no copy of any v1 file. 2.B guarded fixture directories: adopted; `home-options-card-view` keeps `ok.json` valid and its acceptance runs `TestFixtures`. 2.C v3 gap object in `ok.json`: adopted, the N225 gap goes to `gap.json` and `README.md` is in Files. 2.D no route: adopted, stated in the Goal and Left out.
- 3.4 `home-compose-links` has no caller: adopted, dropped with the schema change it served. Other 3.x points concern other files.
- 4.2 `Part.whose` enum hard-codes the family: adopted, the part lines are caption text. 4.5 many schema files for no producer: adopted for this file (none).
- 5.1 `closed` is an amount: adopted, `home-options-card-view` is rewritten (two series, value labels, caption). 5.2 series names are three, not four: adopted, the market acceptance reads `series_names`. 5.7 precedent and `additionalProperties`: moot, no field is added. 5.8 macro scale: adopted, Decision 3 (producer maps, view reads the stored track; fixture ticks -2..+2 with track percents).
- 6.1 offline acceptance: adopted, no `generate:types` line; the Go fixture tests run offline. 6.2 conflicts on `page-schemas.md`: adopted, this file does not edit it. 6.5 permanently empty values: adopted, said once in the Goal.
- Per-file check HB: no break found. "Owner approved: (pending)" stays until the review.
- Top 1: not applicable here. Top 2: adopted. Top 3: adopted for home-options-card-view. Top 4: adopted. Top 5: adopted (daughter, macro scale, 盘前/盘后 in Decision 4). Top 6: adopted, with the schema issues merged or removed.
