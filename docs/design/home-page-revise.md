# Home page revise: a useful overview card, no jump chips, labelled charts, price and sharp-move marks, defined option bars

Author: the optdesk designer (first draft)
Owner approved: (pending)
Revision: 2 (after the adversarial read `home-revise-adversarial.md` of the scratchpad and the dispatcher's decisions)

Source: the owner's annotations of the `/home` preview with fixture data, 2026-11-08 (five points, below). It builds on `docs/design/home-page.md` (v2 compose) and
`docs/design/home-board-1.md` (board 1 layout; its Decision 1 put the jump links in `block_links` and its Decision 6 the two bar series).
Read on main at 36c4e50: AGENTS.md, docs/design/modules.md, home-page.md, home-board-1.md, docs/adr/0034-frontend-stack.md, contracts/schemas/page.home_page.v2.json,
page.held_companies_summary.v2.json, page.market_summary.v3.json, page.options_summary.v3.json, common.v1.json, internal/pages/home_page/compose.go,
web/src/pages/home/*.tsx, web/src/charts/{Chart.tsx,spec.ts}, web/src/components/data-table/data-table.tsx, web/test/pages/home/**/fixtures, web/fixtures/v3/*,
contracts/datasets/*, internal/store/snapshot/quotesnapshot.go, internal/compute/rules/. Also read, not edited: `docs/design/unified-site.md` (scratchpad `wt-site`, in progress).

Goal: the five points of the owner's walk through `/home` are answered in the schema, the Go that fills it and the view, with no number or flag computed in the browser.
Limit, stated so the goal is not read as a working page: no builder of the held, market or options summary and no caller of `Compose` exist, and under `unified-site.md` `/home` shows sample data until they do. So the owner will
see these changes only on fixture data; the real values need the builders (named in `## Left out`). Day and 5-day change, and the price, will come from stored quotes (`obs.quote_snapshot`) or be a Gap.

## What main holds today (the two "why" answers)

- Why "今日总览" shows nothing (point 1). The card is `title`, `say` and `gap_line` of `home_page.v2`, and the view prints exactly those three strings
  (`HomePage.tsx`, first `Card`). `Compose` (`internal/pages/home_page/compose.go`) does not write them: `Input.Title`, `Say` and `GapLine` are the caller's text,
  and no caller exists (nothing in `cmd/` or `internal/` imports `home_page` outside its tests). The preview strings `五张卡片，一页看完。` and `今天没有缺口`
  are literals of `web/test/pages/home/home-v2/fixtures/ok.ts`. So the text is not derived from the members: `gap_line` would say "no gap" even over a member that has gaps, and
  the five cards' own one-line summaries (`card.say`, household `card.subtitle`) are not carried to the top. The card is empty by design of v2, not by a data fault.
- Why four lines in 市场概览 (point 3). They are real series, not fixture artefacts: `market_summary.v3` `ChartPoint` has `close`, `ma200`, `band_low` and `band_high`,
  and `MarketSummaryCard.specOf` draws all four as lines, the last two under the one name `区间` (`series_names` has three names for four lines). The chart sets no
  `legend` (`ChartSpec.legend` exists, `Chart.tsx` supports it, the card never turns it on) and the lines differ only by the theme colours `--chart-1..4`, so nothing says
  which is which. The grey ones look straight because the fixture is a ramp (`ma200` 98.0, 98.1, 98.2, ..., `band_low` and `band_high` the same +0.1 a day): that part is a fixture artefact.
- The option bars (point 5): `OptionsSummaryCard` names series 2 with the literal `已了结`, a string in the view that no payload carries, and `SeriesPoint.closed` is "the amount already closed that month".
- Prices (point 4): `Row` of `held_companies_summary.v2` has no price. Stored sources on main: `obs.quote_snapshot` (`last`, `bid`, `ask` per instrument and session day,
  `QuoteSnapshotStore.QuoteSnapshots(instrumentID, from, to)`) and `obs.option_eod.underlying_price`. No dataset holds positions or fills (`contracts/datasets` has five files:
  `master.session_day`, `obs.option_eod`, `obs.quote_snapshot`, `obs.series_point`, `readings.reading`), and no Go builder of the held, market or options summary exists yet
  (Compose takes their JSON as bytes; the fixtures are synthetic).

## Decisions

1. The overview card is built from the members, not typed. `home_page.v3` gets `overview`: `items`, one per card in page order, each `{label, text, href}`: `label` is the card title, `text` is the card's own one-line summary,
   `card.say` when the card has one, else `card.subtitle` (macro, market and held carry `say`, `subtitle` is required on them too and `say` wins; options has an optional `say`; household has only `subtitle`), and `href` the card's page.
   `overview.gaps` holds every `card.gaps[].text` of the five members with the card title. `Compose` copies these strings and writes `gap_line` (`今天没有缺口`, else `N 处缺口，见下`). `title` and `say` of the page stay the caller's text.
   A member with neither `say` nor `subtitle` is an error naming the slot; nothing is invented.
2. The jump chips go. `block_links` is removed from v3 and `BlockRow` from the view. Each card title is the link to its page: `overview.items[i].href` is given to card `i` as `titleHref` (dispatcher decision); the overview
   label is a link too and does not replace the titles. `Input.CardLinks` is the one place the caller gives the five paths; `report_links` stays separate.
3. Market chart: the view turns the legend on and names the lines from `series_names` (`收盘`, `200日均线`, `区间`); the two band lines share one legend entry and are drawn dashed and thinner, so the grey-family lines differ by
   shape, not only by colour. No schema change. The fixture ramps (ma200 98.0, 98.1, ...) are replaced by curves.
4. Held companies: `held_companies_summary.v3` adds a column `price` (`价格`, after `company`) and per row `price`, a `PriceFigure` or a `Gap`. The source is the stored last quote only: the builder reads `obs.quote_snapshot.last`
   of the latest session day with status ok; the text carries its day when it is not today (`¥3,450 · 11-06`). Day and 5-day change come from the same store: the last quote of the latest session day against the last of the
   previous session day, and against the last of the fifth session before; a missing one makes that figure a `Gap` (`missing_quote`, text `无报价`), never a zero or a figure from another feed, so price and change cannot disagree.
5. Sharp moves are decided in Go with limits held as exported constants. New package `internal/compute/sharpmove`: `DayLimit = 3`, `FiveDayLimit = 8` (percent), `Limits()` for the builder's note, and `Mark(day, five *float64)`
   returning two bools, inclusive at the limit, false for a nil (gap). Thresholds: |day| >= 3% and |5-day| >= 8%; a later change edits the two constants. A row gets `day_sharp` and `five_day_sharp`, each a `SharpMark {kind: sharp, text}` or
   `NoMark {kind: none}` (text such as `大幅` from the builder; no `null`, as the v3 conventions require); the card gets `sharp_note`, one sentence naming the limits, written by the builder from `Limits()`. The view draws a mark where the mark is a `SharpMark`, none for a `NoMark`, and compares nothing.
6. Option bars, additive: `options_summary.v4` is v3 plus one required text `closed_series_text`; `premium` and `closed` keep their names. `premium_series_text` labels series 1 (`本月新开仓权利金`) and `closed_series_text`
   series 2 (`按完全了结所在月统计的权利金`); `premium_series_caption` defines both: `premium` = premium collected on positions opened in that month; `closed` = bucketed by the month the position fully settled (closed, expired or
   assigned), amount = the premium collected at open of those positions. The literal `已了结` leaves the view. The tile `本月权利金` is relabelled `本月新开仓权利金` (a payload text, same definition as series 1).
   Main has no positions dataset, so until one exists the builder writes `Gap` for both series with its cause; the labels and the legend show regardless.
7. The view keeps drawing with the one chart file `web/src/charts/Chart.tsx` (recharts through the shadcn chart component, owner decision 2026-11-03, as on main); `ChartSpec` series gain `dashed` and `legendHidden`.
8. Version line: `home_page` v3, `held_companies_summary` v3, `options_summary` v4; market (v3), household and macro (v2) stay. v2 files stay on disk, retired by never being read. The v3 options fixtures
   (`web/fixtures/v3/options-summary`) and `internal/contracts/fixtures_v3_test.go` are not touched; the v4 fixtures are new files under `contracts/fixtures` with their own Go test.

## Modules

None added to `docs/design/modules.md`; every path belongs to a row as it stands: `contracts` (schemas, `contracts/fixtures`, generated, `internal/contracts`, `docs/contracts/page-schemas.md`), `compute`, `pages`, `web`.
The new package `internal/compute/sharpmove` needs lint lines in `.go-arch-lint.yml` (row repo): a judged cross-module entry on the Files line of home-revise-sharpmove, as PR #519 did for talibind. `arch.toml` is generated by
`scripts/arch` (never edited by hand); the regenerated file is on the Files line of home-revise-sharpmove, because the arch check fails when it disagrees with `docs/design/modules.md`. `internal/compute/api.go` and `internal/pages/api.go` are not edited.

| issue | module | depends |
|---|---|---|
| home-revise-interface | contracts | site-interface (unified-site.md) |
| home-revise-sharpmove | compute | none |
| home-revise-compose | pages | home-revise-interface |
| home-revise-web | web | home-revise-interface, site-web (unified-site.md) |

## Interface

Owner: contracts. Frozen by `home-revise-interface`, read by `home-revise-compose` and `home-revise-web`.

- `page.home_page.v3.json`: v2 with `schema_version` 3, following the v3-line conventions (`TestPageV3Conventions`): shared defs by `$ref` to `common.v2.json` (not `common.v1`), `stale_after` required, a `kind` on every oneOf branch, no `null`. `Compose` writes `stale_after` as the earliest `stale_after` among the members that have one; when none has, the earliest of the members' `data_time` days plus 4 calendar days at 09:30:00 in `Input.Zone` (the freshness rule of `internal/pages/macro_page`, `freshnessLimitDays` 4), so it is always present; `block_links` removed from properties and `required`; new required `overview` `{items: OverviewLine[1..10], gaps: GapItem[0..20]}`;
  `OverviewLine {label, text, href}` (`href` starts with `/`), `GapItem {card, text}`, strings 1..200; `companies` refs held v3, `options` refs options v4.
- `page.held_companies_summary.v3.json`: v2 plus `Column.id` enum gains `price`; `Row` gains required `price` (oneOf `PriceFigure`, `Gap`), `day_sharp`, `five_day_sharp` (oneOf `SharpMark`, `NoMark`); `Card` gains required `sharp_note`.
  The file follows the v3-line conventions (`TestPageV3Conventions`): `Gap` is `common.v2.json#/$defs/Gap`, every oneOf branch requires a distinct `kind` const, no `null`.
  `PriceFigure {kind "value", value >= 0, text, unit "price"}`; `SharpMark {kind "sharp", text}`; `NoMark {kind "none"}`.
- `page.options_summary.v4.json`: v3 plus required `Card.closed_series_text` (string 1..200); nothing renamed.
- Generated: `web/src/generated/{home_page.v3,held_companies_summary.v3,options_summary.v4}.ts`, made once with `pnpm --dir web run generate:types page.home_page.v3 page.held_companies_summary.v3 page.options_summary.v4`
  (the script takes new targets from its arguments; `web/package.json` is not edited).
- Web-side types introduced by the web issues, not by contracts: `FigureCell.sharp?: { text: string }` in the shared Data Table, and the `dashed` and `legendHidden` fields of `ChartSeries`.

## Issues

### home-revise-interface (interface)

Module: contracts
Files: contracts/schemas/page.home_page.v3.json, contracts/schemas/page.held_companies_summary.v3.json, contracts/schemas/page.options_summary.v4.json, web/src/generated/home_page.v3.ts, web/src/generated/held_companies_summary.v3.ts, web/src/generated/options_summary.v4.ts, internal/contracts/home_page_v3_test.go, internal/contracts/held_options_v3v4_test.go, internal/contracts/fixtures_v4_test.go, internal/contracts/generated_ts_test.go, contracts/fixtures/page.options_summary.v4.ok.json, contracts/fixtures/page.options_summary.v4.gap.json, contracts/fixtures/page.options_summary.v4.stale.json, contracts/fixtures/MANIFEST.synthetic-v1.json, docs/contracts/page-schemas.md
Design: docs/design/home-page-revise.md#interface
Contracts: page.home_page v3 (new), page.held_companies_summary v3 (new), page.options_summary v4 (new)
Depends: site-interface of unified-site.md: both append to `page-schemas.md`, `generated_ts_test.go` and the contracts MANIFEST (`web/src/generated` has no index file). The row `"home-page": "page.home_page.v2.json"` that site-interface adds to `internal/contracts/fixtures_test.go` and the v2 sample `web/fixtures/home-page/ok.json` are left as they are, so web and contracts tests stay green after this issue; home-revise-web moves both together. The rows of `held-companies-summary` (v1) and `options-summary` (v2) map other directories and stay; the v3 options fixtures stay with `fixtures_v3_test.go`, the v4 ones with `fixtures_v4_test.go`.
Goal: the three schemas, generated types, v4 options fixtures and their Go tests exist; every v2 and v3 file stays byte-identical.
Acceptance:
1. `jq -e '(.required|index("block_links")==null) and (.required|index("overview")!=null) and (.required|index("stale_after")!=null) and .properties.schema_version.const==3' contracts/schemas/page.home_page.v3.json` exits 0, and its `companies` and `options` refs name `page.held_companies_summary.v3.json` and `page.options_summary.v4.json`.
2. `jq -e '.["$defs"].Row.required | (index("price")!=null) and (index("day_sharp")!=null) and (index("five_day_sharp")!=null)' contracts/schemas/page.held_companies_summary.v3.json` and `jq -e '.["$defs"].Card.required|index("closed_series_text")!=null' contracts/schemas/page.options_summary.v4.json` exit 0.
3. `go test ./internal/contracts/...` passes; the new tests accept a minimal instance of each schema and the three v4 fixtures, and reject a row without `price`, an options card without `closed_series_text`, a home page holding `block_links`, and a wrong `schema_version`.
4. The three generated `.ts` files are the output of `generate:types` for the three schemas, committed with their sources; no existing schema or fixture is modified or deleted, and `fixtures_v3_test.go` and `web/fixtures/v3` are untouched (the drift step and the diff against the merge base are under `Checked by CI`; no acceptance item runs json2ts offline).
5. `contracts/fixtures/MANIFEST.synthetic-v1.json` gains the sha256 of the three new fixtures only; `docs/contracts/page-schemas.md` names the three schemas; `git diff --name-only` against the merge base lists no file outside the fifteen above, and none of `internal/contracts/fixtures_test.go`, `web/fixtures` and `web/test`.
Checked by CI: ci-go (`go build ./...`, `go test ./internal/contracts/...`), ci-web `tsc`, `eslint` and the type drift step (`pnpm --dir web run generate:types && git diff --exit-code web/src/generated`); the diff against the merge base with `--diff-filter=MD` shows no file under `contracts/schemas` or `web/fixtures`; the design verdict (interface change); the PR body carries the Contracts line above from opening.
Size: M

### home-revise-sharpmove

Module: compute (plus `.go-arch-lint.yml` and `arch.toml`, a judged cross-module entry: the lint lines of the new component, as PR #519, and the output of `scripts/arch` for it)
Files: internal/compute/sharpmove/sharpmove.go, internal/compute/sharpmove/sharpmove_test.go, .go-arch-lint.yml, arch.toml
Design: docs/design/home-page-revise.md#decisions (5)
Depends: none
Goal: a pure function says whether a day change and a 5-day change are sharp, and the limits are exported for the builder's note.
Acceptance:
1. `go test ./internal/compute/sharpmove/...` passes with a table test: day 2.99 false, 3.0 true, -3.0 true, -2.99 false; 5-day 7.99 false, 8.0 true, -8.0 true; nil input false; the two results are independent; `Limits()` returns 3 and 8.
2. `go list -deps ./internal/compute/sharpmove | grep -c andrewrong/optdesk` prints 1 (the package itself); the package has no I/O, no clock and no embedded file.
3. `.go-arch-lint.yml` gains only the component `compute/sharpmove` in its two places (components and deps) as the diff of PR #519 shows; `arch.toml` is the unedited output of `scripts/arch` and differs from main only by the component `compute/sharpmove` and its edges; `go build ./...` passes.
4. Tests call `t.Parallel()`; no file outside the four listed is in the diff.
Checked by CI: ci-go (`go build ./...`, `go test ./internal/compute/...`, lint, the arch check that `arch.toml` and modules.md agree).
Size: S

### home-revise-compose

Module: pages
Files: internal/pages/home_page/compose.go, internal/pages/home_page/overview.go, internal/pages/home_page/compose_test.go, internal/pages/home_page/overview_test.go, internal/pages/home_page/testdata/overview_ok.json, internal/pages/home_page/testdata/overview_gaps.json
Design: docs/design/home-page-revise.md#decisions (1, 2)
Depends: home-revise-interface
Goal: `Compose` writes `home_page` v3: the members through as they came, the `overview` copied from them, and `gap_line` derived.
Acceptance:
1. `go test ./internal/pages/home_page/...` passes; on `testdata/overview_ok.json` the output validates against `page.home_page.v3.json` (the test loads the schema), `overview.items` has five lines in the order macro, market, household, companies, options, each `text` byte-equal to the member's `card.say` when present, else its `card.subtitle` (the data holds an options card with `say` and one without, and household with `subtitle` only), `overview.gaps` is empty and `gap_line` is `今天没有缺口`.
2. On `testdata/overview_gaps.json` (two members with `card.gaps`) `overview.gaps` holds those texts with the card titles in member order and `gap_line` is `2 处缺口，见下`; a member with neither `say` nor `subtitle` returns an error naming the slot.
3. `Input` has no `BlockLinks`, the output has no `block_links` key, the five hrefs come in as `Input.CardLinks` and appear as `overview.items[i].href`; an href not starting with `/` is an error.
4. `stale_after` is always present: with members that carry one the output equals the earliest of them, and on a test input where none has one it is the earliest `data_time` day plus 4 days at 09:30:00 in the test zone, and the output validates. The members' bytes in the output equal the input bytes; `schemaVersion` is 3 and the v2 expectations are gone from the tests.
5. `compose.go` stays under 600 lines (`wc -l`), tests call `t.Parallel()`, nothing reads a clock; no file outside the six listed is in the diff (`internal/pages/api.go` unchanged).
Checked by CI: ci-go (`go build ./...`, `go test ./internal/pages/...`, lint, arch check).
Size: M

### home-revise-web

Module: web
Files: web/src/charts/Chart.tsx, web/src/charts/spec.ts, web/src/components/data-table/data-table.tsx, web/src/pages/home/MarketSummaryCard.tsx, web/src/pages/home/HeldCompaniesCard.tsx, web/src/pages/home/OptionsSummaryCard.tsx, web/src/types/held_companies_summary.ts, web/src/types/options_summary.ts, web/test/pages/home/HeldCompaniesCard.test.tsx, web/test/pages/home/MarketSummaryCard.test.tsx, web/test/pages/home/OptionsSummaryCard.test.tsx, web/test/pages/home/HomePage.test.tsx, web/test/charts/Chart.test.tsx, web/test/components/data-table.test.tsx, web/test/pages/home/held-v2/fixtures/ok.ts, web/test/pages/home/held-v2/fixtures/gap.ts, web/test/pages/home/held-v2/fixtures/stale.ts, web/fixtures/v3/market-summary/ok.json, web/src/pages/home/HomePage.tsx, web/src/pages/home/OverviewCard.tsx, web/src/pages/home/MacroCard.tsx, web/src/pages/home/HouseholdCard.tsx, web/src/types/home_page.ts, web/test/pages/home/OverviewCard.test.tsx, web/test/pages/home/MacroCard.test.tsx, web/test/pages/home/HouseholdCard.test.tsx, web/test/pages/home/home-v2/fixtures/ok.ts, web/test/pages/home/home-v2/fixtures/bad-member.json, web/test/pages/home/home-v2/fixtures/bad-version.json, web/test/shell/sample.test.tsx, internal/contracts/fixtures_test.go, web/fixtures/home-page/ok.json, web/fixtures/MANIFEST.synthetic-v1.json
Design: docs/design/home-page-revise.md#decisions (1, 2, 3, 4, 5, 6, 7)
Depends: home-revise-interface, site-web of unified-site.md (it creates the v2 sample `web/fixtures/home-page/ok.json`, the MANIFEST line and the sample test this issue moves to v3). `internal/contracts/fixtures_test.go` is a contracts file, a licensed cross-module entry of this issue: its row `"home-page"` (added as v2 by site-interface of unified-site.md) becomes `page.home_page.v3.json`, in the same change as the sample, so each of the two issues leaves main green.
Goal: the top card lists the five card summaries with a link each and the real gaps; the chips between cards are gone; every card title is a link (`titleHref` on all five cards); the market chart has a legend, the companies table a price column and sharp marks, the option bars payload labels; the home sample is a v3 payload. One issue, so that the shared `titleHref` prop, `HomePage.test.tsx` and the home-v2 fixtures change together and nothing here needs the output of a sibling issue.
按 frontend-stack：用 shadcn chart (`ChartLegend`, `ChartLegendContent`) via the existing `Chart.tsx` 做图例 and dashed band lines (recharts `strokeDasharray`, `legendType="none"` for the second band line).
按 frontend-stack：用 shadcn `Badge` (destructive variant) and Lucide `TriangleAlert` in the shared Data Table (the new field `FigureCell.sharp`) 做急涨急跌标记; the table stays TanStack Table.
按 frontend-stack：用 shadcn `Button` (link variant, `asChild` anchor) 做卡片标题链接.
按 frontend-stack：用 shadcn `Card`, `Button` (link variant, `asChild` anchor) and `Alert` 做概览卡, Lucide `ChevronRight` and `TriangleAlert` for the icons.
按 frontend-stack：用 shadcn `Card`, `Button` (link variant, `asChild` anchor) and `Alert` 做概览卡, Lucide `ChevronRight` and `TriangleAlert` for the icons.
Acceptance:
1. `pnpm --dir web exec vitest run test/pages/home/MarketSummaryCard.test.tsx test/charts/Chart.test.tsx` passes (`Chart.test.tsx` is a new file; the card test mocks `@/charts/Chart` today, so the chart assertions live in `Chart.test.tsx`): the legend shows the three `series_names` once each and the band lines carry `stroke-dasharray`; the market card passes `legend: true` and `legendHidden` on the second band line (card test).
2. `pnpm --dir web exec vitest run test/pages/home/HeldCompaniesCard.test.tsx test/components/data-table.test.tsx` passes: header cells read `公司`, `价格`, `财报日`, `日涨跌`, `5日涨跌` in that order; a `price` Gap cell shows its text; a cell with a `day_sharp` of kind `sharp` shows a Badge with that text and a `NoMark` shows none; `sharp_note` is rendered. `held-v2/fixtures/ok.ts` imports the v3 type.
3. `pnpm --dir web exec vitest run test/pages/home/OptionsSummaryCard.test.tsx` passes with the v4 fixture of `contracts/fixtures`: the legend shows `premium_series_text` and `closed_series_text`, and `grep -n "已了结" web/src/pages/home/OptionsSummaryCard.tsx` prints nothing.
4. `grep -nE "Math\.abs|[0-9]+(\.[0-9]+)? *[<>]=? *[0-9]" web/src/pages/home/HeldCompaniesCard.tsx` prints nothing; the flag comes only from `day_sharp` and `five_day_sharp`.
5. `jq '[.card.chart.points[].ma200] | unique | length' web/fixtures/v3/market-summary/ok.json` prints more than 10 and the day-to-day differences of `ma200` are not all equal; the held fixtures have one sharp day mark, one sharp 5-day mark and one price Gap row; each of the three cards renders its title as `<a href>` from `titleHref`.
6. Macro and household render their title as `<a href>` from `titleHref` (tests `MacroCard.test.tsx`, `HouseholdCard.test.tsx`); `grep -rn "data-block-row\|block_links\|BlockRow" web/src` prints nothing.
7. `pnpm --dir web exec vitest run test/pages/home/HomePage.test.tsx test/pages/home/OverviewCard.test.tsx` passes: five overview lines render in payload order, each label is an `<a>` with its `href`, each text equals the payload text; with `overview.gaps` non-empty the Alert lists each gap text, with none it shows `gap_line`.
8. The `home-v2` fixtures are v3 payloads: `ok.ts` is typed by `generated/home_page.v3`, reads the options member from `contracts/fixtures/page.options_summary.v4.ok.json`, and its hrefs are bar routes only (`/macro` for macro and market, `/household`, `/positions`, `/options`); `bad-version.json` has `schema_version` 2 and is refused with the fail-closed notice; `bad-member.json` still fails on a member.
9. `web/fixtures/home-page/ok.json` equals `ok.ts` except `report_links` `[]` (the rule of unified-site Decision 5, compared by the existing sample test, which this issue updates), its sha256 is current in `web/fixtures/MANIFEST.synthetic-v1.json`, `grep -n '"home-page"' internal/contracts/fixtures_test.go` prints the row with `page.home_page.v3.json` and `grep -c 'home_page.v2' internal/contracts/fixtures_test.go` prints 0 (that edit is the only change of the file), `go test ./internal/contracts/...` passes, and `pnpm --dir web exec vitest run test/shell/sample.test.tsx` passes with `/home` rendering the v3 sample.
10. `src/types/home_page.ts` accepts version 3 only; each new or edited `.tsx` stays under 600 lines; a test renders with a changed overview text and sees it (nothing is computed or formatted in the view).
11. `pnpm --dir web exec vitest run test/pages/home/HomePage.test.tsx` passes with the types of `web/src/types/held_companies_summary.ts`, `options_summary.ts` and `home_page.ts` moved together to held v3, options v4 and home v3 (no step with mixed versions); `pnpm --dir web exec tsc --noEmit` and `pnpm --dir web exec eslint .` exit 0; no file outside the 33 listed is in the diff.
Checked by CI: ci-web (`tsc`, `eslint`, `vitest`, build, the home e2e spec).
Size: L

## Left out

- The builders of the held, market and options summaries and the caller of `Compose` (none exist in Go; the held builder, which writes price, change and the sharp marks from `obs.quote_snapshot` and `sharpmove`, is the next design). Until then `/home` shows sample data and
  `sharpmove.Mark` has no production caller; the schema, fixtures and tests pin what the builders write.
- A positions or fills dataset for the two option series (needs a broker-to-store design); both series are Gaps with a cause until then.
- `docs/adr/0034-frontend-stack.md` is stale on ECharts (the code and rules use recharts via the shadcn chart component); a separate docs issue fixes it.
- Rejected alternatives, weighed in `## Adversarial`: renaming `premium` and `closed` (breaks the v3 payload and its fixtures for a label that `closed_series_text` and the caption already define); embedded JSON for the sharp-move limits (more files for two constants); overview items without `href` (the view holds no route table, so `Input.CardLinks` stays the single source); numbering the held builder as an issue here (it needs its own design: broker positions, instrument map).
- Retiring `block_links` of v2, intraday prices, per-row thresholds, a news column, and any change to the household and macro cards beyond the title link.

## Adversarial

Answers to `home-revise-adversarial.md` of the scratchpad (blockers A to C, the six sections, the nine fixes), each adopted or not.
- A, fix 1 (v3 options fixtures break `fixtures_v3_test.go`): adopted. v4 fixtures are new files in `contracts/fixtures` with `fixtures_v4_test.go`, both in the interface issue; `web/fixtures/v3/options-summary` stays untouched (Decision 8).
- B, fix 2 (`card.say` missing): adopted as a rule, with one correction: options has `say`, optional, so "say, else subtitle" applies to all five and the compose test holds both an options card with and one without `say` (Decision 1, compose 1).
- C, fix 3 (interface acceptance 4): adopted, now `--diff-filter=MD` and the real jq expressions with `!= null`.
- Fix 4 (`web/package.json`): adopted, dropped; the script takes the schema names as arguments.
- Fix 5 (arch files, `Limits()`): adopted. `.go-arch-lint.yml` and the generated `arch.toml` (output of `scripts/arch`, no hand edit), named as a judged cross-module entry; `Limits()` is exported and tested.
- Section 3, simpler options path: adopted. No rename, one added text `closed_series_text`, definitions in the caption (Decision 6). Simpler sharpmove: adopted, constants instead of embedded JSON (Decision 5). Items without `href`: not adopted, because the view holds no route table; `Input.CardLinks` is the single caller input.
- Fix 6 (ordering with unified-site): adopted. home-revise-web depends on `site-web` and lists the sample, the MANIFEST line, the sample test and the one-row edit of `fixtures_test.go`; interface depends on `site-interface` and leaves the v2 sample and row alone. Moving them into the interface issue was rejected: the sample test and `HomePage` read v2 types until the web types move, so main would be red between the two issues.
- Fix 7 (goal gap): adopted in the Goal and `## Left out`; the source of price and change is stored quotes or a Gap (Decision 4). Not adopted: numbering the held builder as an issue here, because it needs its own design (broker positions, instrument map). Two Gaps in the option series stay until a positions dataset exists. `title` and `say` of the page: kept as the caller's text (Decision 1).
- Fix 8 (simpler paths): adopted as above. Fix 9 (false names): adopted. `Chart.test.tsx` is marked new, the field is `FigureCell.sharp` (not `flag`) and named in `## Interface`, `held-v2/fixtures/ok.ts` moves to the v3 type, and home-revise-web runs `HomePage.test.tsx`. After the design review the two web issues were merged into home-revise-web: the shared `titleHref` prop and the home-v2 fixtures cannot be split without a sibling dependency.
- Lock-in (section 4): accepted. Required `price`, `day_sharp`, `five_day_sharp`, `sharp_note` force every builder to fill them (a Gap or a `NoMark` is a fill); "adjustable" now means editing two constants and rebuilding.
- Unresolved: the sample hrefs use bar routes, so market points at `/macro` and companies at `/positions`; the owner may prefer other pages.
