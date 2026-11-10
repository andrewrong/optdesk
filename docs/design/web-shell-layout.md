# Web shell layout: the pages use the desktop width, show a missing value as a dash, collapse the bar on a phone and draw charts the way board 1 does

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-10
Revision: 1

Source: owner 2026-11-10: "the front end looks nothing like the design images, much uglier", then "especially the charts". The comparison is the page `compare2/index.html` of the owner window (screenshots `compare2/shots/*.png`, 1440 and 390 px) against the boards of design set v2.2 (owner-approved 2026-10-23; the board images are the PNGs of that set, `board1.png` is 720 px wide, `1. 首页 Home`; the text of board 1 is `docs/design/home-board-1.md`). The rules of the shell are `docs/adr/0034-frontend-stack.md` ("Accessibility and phone rules", "Staleness", "Missing is never zero"). Read on origin/main at 65780f9f.

Goal: the owner opens `/home`, `/market`, `/options` on a desktop and on a phone and sees pages that use the width, a table with dashes where a value is missing (the reason one tap away), a phone bar that does not scroll sideways, a grey data-date line unless the data is stale, and charts with an axis, straight lines, a shaded band and the last value labelled. No Go page-model string (`internal/pages/**`) changes: the front end renders the reason it is given.

## What main holds today (read at 65780f9f)

- `web/src/shell/App.tsx:14` and `:55`: both `<main className="p-4">`, no max width, no grid. `web/src/pages/home/HomePage.tsx:80` stacks the five cards in one `flex flex-col gap-4`. The shadcn table wraps itself in `overflow-x-auto` (`web/src/components/ui/table.tsx:8`).
- `web/src/shell/TopNav.tsx:41`: one `<nav className="flex gap-1 overflow-x-auto p-2">` of ten links and the `期权持仓` menu; at 390 px it scrolls sideways.
- `web/src/components/data-table/data-table.tsx:102-106`: a gap cell (`GapCell {reason, text}`, `:46`) shows its whole `text` in a `Badge` that wraps (`h-auto whitespace-normal`); every header gets a sort `Button` with `ArrowUpDown` (`:187-197`) whether or not the column has values. `MarketPage.tsx:35` shows a block gap the same way (a block, not a cell).
- `web/src/pages/home/StaleAlert.tsx:37` and `StaleNotice.tsx:8`: an `Alert` on every card and page, destructive only when stale (`StaleAlert`: the device clock is past `stale_after`, `:33`; `StaleNotice`: the producer's `is_stale`, `:8`), default variant otherwise (a bordered card box, not grey text). Fourteen files import them (`grep -rn "StaleAlert\|StaleNotice" web/src`); `web/src/pages/optiondata/OptionDataPage.tsx:53` does not, it draws its own `Alert` on `is_stale`.
- `web/src/charts/Chart.tsx` (recharts 3.10.1 through the shadcn chart, allowed by `frontend-whitelist.json:21`): default size `aspect-[2/1] w-full min-h-56` (`:50`), so 1100 px wide is about 550 px high; `<YAxis hide>` (`:69`); `type="monotone"` (`:100`); the band of the market card is two dashed series (`MarketSummaryCard.tsx:36-37`, `ChartSpec` has `dashed` and `legendHidden`, `spec.ts:22-27`); the legend is `ChartLegend` at the bottom; the tooltip shows `<key>_text` or the value (`web/src/components/ui/chart.tsx:94-95`). Callers with their own size: `HouseholdPage.tsx:148` and `:212`; `QuoteChart.tsx:28` draws its own `LineChart` and does not use `Chart`.
- Chart callers of `Chart`: `home/MarketSummaryCard.tsx`, `home/OptionsSummaryCard.tsx`, `household/HouseholdPage.tsx` (three), `indicator/view.tsx`, `ledger/SectionTables.tsx`, `macro_rates/view.tsx`, `market/MarketPage.tsx` (two), `report/global-assets.tsx`, `structuredetail/Payoff.tsx`; the only other file that draws with recharts is `optiondata/QuoteChart.tsx`, which entry 6 moves onto `Chart` (`grep -rln "recharts\|ChartContainer\|<Chart\b" web/src`).
- Popover and Sheet are not in `web/src/components/ui/`; `radix-ui` 1.8.1 is the one Radix dependency (`web/package.json:19`, no `@radix-ui/*`).

## Decisions

1. Width (owner decision 2026-11-10, in the window; it replaces the 720 px column of ADR 0034 "Accessibility and phone rules" for the desktop). The owner reads mostly on a desktop and wants to see everything: there is no max width. The shell keeps `<main className="p-4">` (16 px gutter, so a 390 px phone is one full-width column) and `App.tsx` is not edited. On wide screens the cards of a page may sit in a responsive grid: two columns from `lg` (1024 px) and three from `2xl` (1536 px), one below (the owner reads on a large desktop and wants to see more). Board 1 draws macro and market side by side (two cards in the first row, about 320 px each on its 720 px sheet) and household, held companies and options as full rows; the home page copies that: macro, market and household are one cell each at three columns; held companies and options, which are full rows on the board, are `col-span-full`; household is `lg:col-span-2 2xl:col-span-1`. A grid cell is `min-w-0`, so a wide table scrolls inside its card (`table.tsx:8`) and never widens the page. Tables are full width. The top bar sits outside the content (full width, as today): it is navigation of the site, not content of a page. Other pages stay one full-width stack until the owner has seen the home grid (Left out). Board source: `board1.png`; ADR source: the 390 px / no horizontal scroll half of "Accessibility and phone rules" stays, the 720 px half is replaced by this decision. The ADR itself is not edited: its sha256 is pinned in `web/copy-manifest.sha256`, which is owner-reserved (Left out).
2. Missing value in a table cell: the cell shows "—" only. The reason (the `text` Python sent) is in a shadcn Tooltip on hover and in a shadcn Popover on tap, on one trigger button, and stays in the DOM as `sr-only` text of the button, so a screen reader and the existing `toContain` tests of pages still read it. Colour is not the signal (the dash and the aria text are). Popover is added by `shadcn add popover` (previewed with `--dry-run`: the registry item `radix-nova/popover` imports `radix-ui` only, so no package is added). A block gap (a missing chart, a missing table) keeps its named sentence once (`MarketPage.tsx:35`, ADR "a data gap is named once"). Source: board 1 draws "没有缺口。" as one grey line and short cells; ADR 0034 "Missing is never zero".
3. Phone navigation: below 768 px (`md`) the bar becomes a 44 px menu button (Lucide `Menu`) that opens a shadcn Sheet from the left with the eleven items as a vertical list (the four position links under the heading `期权持仓`); from 768 px the bar is today's bar, without `overflow-x-auto`. Added by `shadcn add sheet` (registry item `radix-nova/sheet`, imports `radix-ui` and the existing `button`). Source: board 1's header has no 11-item bar; ADR "touch targets at least 44 px, no horizontal page scroll".
4. Data date and stale: grey small text (`text-sm text-muted-foreground`, no box, no icon) with the text Python sent, when the data is not stale; the destructive `Alert` only when the page's own rule says stale: `StaleAlert.tsx:33` (`Date.now()` past `stale_after`; the clock-skew alert stays as is) and `StaleNotice.tsx` (`is_stale` from the producer). Both files share one rendering (`StaleAlert` renders `StaleNotice` for its flag) and keep `data-stale` and `data-clock-skew`. Source: board 1 draws "数据时间" as grey text under each card and no red bar; the comparison names the red "数据日 11-05" bar as the loudest difference. When the data really is stale the bar stays red; a missing explanation there is the text of the page builder (Left out).
5. Charts, one component, every page (`web/src/charts/Chart.tsx` through the shadcn `ChartContainer`; `web/src/components/ui/chart.tsx` is not edited): (a) size by the box, not by the aspect: `h-[180px] md:h-[220px] w-full` (180 px phone, 220 px from 768 px); board 1's SPX line occupies about 160 px of height including the axis on a card of about 320 px, and 180 px / 220 px are the dispatcher's numbers (not the owner's, not measured from the board). The three callers that set their own size lose it (`HouseholdPage.tsx:148,212`, `QuoteChart.tsx`). (b) A visible y axis, `tickCount` 5, no axis line and no tick line, the tick label is the tick value exactly as recharts gives it, with the unit of the spec appended by string concatenation (`ChartSpec.unit`, text from Python, empty when none). The browser formats no number: `web/test/scripts/noCompute.test.ts:7` bans `toFixed`, `toLocale`, `Intl.` and `Math.` in `web/src`, so board 1's thousands separator ("5,400") is not drawn; a producer that wants one sends the tick texts in a later design (Left out). Bars start at zero (recharts' default domain); lines keep `domain={['auto', 'auto']}`, which `web/test/pages/optiondata/OptionDataPage.test.tsx:203-206` asserts on the quote chart and which stays true after entry 6 (no edit of that test for it). A series flagged `hole` (`ChartSeries.hole`) breaks its line at a null point (`connectNulls={false}`), as `QuoteChart.tsx:46` does today; every other series bridges, as `Chart.tsx` does today. (c) Lines are `type="linear"`: no smoothing, no shape between two points. (d) A band is a shaded `Area` of the series `ChartSpec.band` (`{ name, low: ChartPoint[], high: ChartPoint[] }`, numbers from Python) with `fillOpacity` 0.12 in `var(--chart-2)`, no line, in the tooltip not listed; the two dashed series of the market card are replaced by it. The chart of a line kind is a recharts `ComposedChart`. (e) The last point of every line series is labelled at the right edge with its `text` (right margin 56 px). (f) `ChartTooltip` shows the category (the date) as the header and one row per series with Python's `text` (falls back to the value, as `chart.tsx:95` does today). (g) Legend: compact, at the top (`verticalAlign="top"`, 12 px text), one entry per series name, a band one entry. (h) Gridlines horizontal only, 1 px, `var(--border)`; every colour is a token (`var(--chart-N)`, `var(--border)`, `var(--muted-foreground)`), no hex, so light and dark (`theme.css:103-107`, `:156-`) follow. (i) Axis tick and legend text are 12 px, as board 1 draws them. ADR 0034's "text at least 16 px" is the minimum for body text (paragraphs, table cells, labels a user must read to act); chart ticks and the legend annotate a figure whose values are in the tooltip and the aria text, so they are not body text. Body and table text stay at the ADR size. Source: `board1.png` (market and options charts), the owner's list of 2026-11-10.
6. Tables: sort arrows only on sortable columns. A column is sortable when at least one of its cells has a `value` (`FigureCell.value`, `data-table.tsx:25`); its header is a `Button` with `ArrowUp` or `ArrowDown` when sorted and a faint `ArrowUpDown` when not; any other header is plain text with no button. Numbers stay right-aligned with `tabular-nums` (`:243` today, kept). Compact rows: `py-1.5` on every `TableCell` (shadcn default `p-2`), same text size. Source: board 1's tables (header row, 29 px rows at 12 px text on the board) and ADR "colour never the only signal".
7. No payload field, no schema and no Go string is touched; `Contracts: unchanged` on every entry.
8. Charts are recharts through the shadcn chart, as on main: `web/eslint.config.js:8-9` forbids `echarts` and says "Charts go through recharts and the shadcn chart component"; `frontend-whitelist.json:21` lists `recharts` 3.10.1 with kind `charts`; `docs/design/home-page-revise.md:55` records the owner decision of 2026-11-03 and `:181` records that ADR 0034 "is stale on ECharts". This design follows them and does not edit the ADR (Left out).

## Modules

None changed. Every file is under `web/**` (the row web, whose tests are `web/test/**`; the row includes `docs/adr/0034-frontend-stack.md`). `docs/design/modules.md` is not edited.

## Interface

None. `ChartSpec` (`web/src/charts/spec.ts`) gains the optional fields `unit`, `band` and (on a series) `hole`; both are read only by `Chart.tsx` and written by the page mappers listed in entry 6 (`MarketSummaryCard.tsx` is the only mapper that fills `band`); no file under `contracts/` and no generated type changes, and an old spec still draws.

## Issues

Tests use no clock, no sleep, no file and no port. Acceptance items are decided by reading, grep or git; each prints one number. The commands CI runs are under `Checked by CI`. `BASE` is `git merge-base HEAD origin/main`. Every part below carries the line `按 frontend-stack：用 X 做 Y`.

### 1 ui-popover-sheet

Module: web. Size: S.
Files: `web/src/components/ui/popover.tsx` (new), `web/src/components/ui/sheet.tsx` (new), `web/src/components/ui/SOURCES.md`
Design: docs/design/web-shell-layout.md#Decisions
Order: first.
Goal: shadcn Popover and Sheet are in `web/src/components/ui/` like the other copied components; nothing uses them yet. The worker runs `pnpm dlx shadcn@4.22.1 add popover sheet --dry-run` first and reads the file list; it adds the two files, fixes the `cn` import to `@/lib/utils` as `SOURCES.md` says, and adds a row for each with the registry revision and licence MIT.
按 frontend-stack：用 shadcn `popover` (radix-ui Popover) 做 the tap-open reason of a missing value。
按 frontend-stack：用 shadcn `sheet` (radix-ui Dialog) 做 the phone menu panel。
Acceptance:
1. `grep -c 'from "radix-ui"' web/src/components/ui/popover.tsx web/src/components/ui/sheet.tsx | awk -F: '{s+=$2} END {print s}'` prints 2.
2. `grep -c '@radix-ui' web/src/components/ui/popover.tsx web/src/components/ui/sheet.tsx web/package.json | awk -F: '{s+=$2} END {print s}'` prints 0.
3. `grep -c '^| popover \|^| sheet ' web/src/components/ui/SOURCES.md` prints 2.
4. `git diff --name-only $(git merge-base HEAD origin/main) | grep -v -e '^web/src/components/ui/popover.tsx$' -e '^web/src/components/ui/sheet.tsx$' -e '^web/src/components/ui/SOURCES.md$' | wc -l` prints 0 (no `package.json`, no lockfile).
5. The result matches docs/design/web-shell-layout.md#1 ui-popover-sheet.

### 2 gap-display

Module: web. Size: M.
Files: `web/src/components/data-table/data-table.tsx`, `web/src/components/data-table/missing-value.tsx` (new), `web/test/components/missing-value.test.tsx` (new), `web/test/components/data-table.test.tsx`, `web/test/pages/candidates/FullScanPage.test.tsx`, `web/test/pages/home/HeldCompaniesCard.test.tsx`, `web/test/pages/home/MacroCard.test.tsx`, `web/test/pages/optiondata/OptionDataPage.test.tsx`, `web/test/pages/options/OptionPage.test.tsx`, `web/test/pages/optiontracking/OptionTrackingPage.test.tsx`, `web/test/pages/positions/PositionsPage.test.tsx` (these tests: their assertion on a gap text inside a table cell is updated to the dash plus the reason; found by running the suite with the dash in place: exactly these 11 tests in 8 files fail, no other test of `web/test` does), `web/src/pages/home/CardNotes.tsx`, `web/src/pages/home/OverviewCard.tsx` (the home notes show their gap list and the gap line as grey text instead of boxed alerts; board 1 draws "没有缺口。" as a grey line), `web/test/pages/macro/MacroPage.test.tsx` (its test "shows Python stale and page gap labels as alerts" fails with `role=alert` gone; found by the same run; `PositionsPage.test.tsx` fails the same way and is already listed)
Design: docs/design/web-shell-layout.md#Decisions
Order: after 1.
Goal: Decisions 2 and 6, and the boxed gap notes of the home cards. `CardNotes` renders its gap list as `<div data-gaps>` with a grey `text-xs` list (no `Alert`); `OverviewCard` renders its gap line as grey `text-sm` text with `data-gap-line` (no `Alert`); `MarketPage.tsx:35` `GapNote` keeps its Badge on purpose: it names the cause of a whole missing block once (ADR "a data gap is named once"), which a dash cannot carry. `missing-value.tsx` exports `MissingValue({ text })`: a `button` showing "—" with the `text` as `sr-only` content, wrapped in a `Tooltip` (hover) and a `Popover` (tap), each with the text. `data-table.tsx` shows it for a `GapCell` instead of the `Badge`; a column with no cell that has a `value` has no sort button and no arrow; a sorted column shows `ArrowUp` or `ArrowDown`, an unsorted sortable one a faint `ArrowUpDown`; every `TableCell` has `py-1.5`. A cell's textContent is now "—" plus the sr-only reason, so each listed test asserts the dash and the reason (`toContain`) where it asserted the bare text.
按 frontend-stack：用 shadcn `tooltip` and `popover` composed in `missing-value.tsx` 做 the reason of a missing value (hover and tap)。
按 frontend-stack：用 TanStack Table `enableSorting` per column in the shared Data Table 做 the sort arrows of sortable columns only。
按 frontend-stack：用 Tailwind `py-1.5` and `tabular-nums` on the shadcn `TableCell` 做 compact right-aligned figures。
Acceptance:
1. `grep -c 'Popover' web/src/components/data-table/missing-value.tsx` prints at least 2 and `grep -c 'Tooltip' web/src/components/data-table/missing-value.tsx` prints at least 2.
2. `grep -c 'sr-only' web/src/components/data-table/missing-value.tsx` prints at least 1 and `grep -c '—' web/src/components/data-table/missing-value.tsx` prints at least 1.
3. `grep -c 'MissingValue' web/src/components/data-table/data-table.tsx` prints at least 2 and `grep -c 'whitespace-normal text-left' web/src/components/data-table/data-table.tsx` prints 0.
4. `grep -c 'enableSorting' web/src/components/data-table/data-table.tsx` prints at least 1 and `grep -c 'ArrowUp\b\|ArrowDown\b' web/src/components/data-table/data-table.tsx` prints at least 2.
5. `grep -c 'py-1.5' web/src/components/data-table/data-table.tsx` prints at least 1 and `grep -c 'tabular-nums' web/src/components/data-table/data-table.tsx` prints at least 1.
6. `grep -c '@radix-ui' web/src/components/data-table/missing-value.tsx web/src/components/data-table/data-table.tsx | awk -F: '{s+=$2} END {print s}'` prints 0.
7. `wc -l < web/src/components/data-table/data-table.tsx` prints a number below 300 and `wc -l < web/src/components/data-table/missing-value.tsx` prints a number below 100.
8. `git diff --name-only $(git merge-base HEAD origin/main) | grep -v -e '^web/src/components/data-table/' -e '^web/test/components/data-table.test.tsx$' -e '^web/test/components/missing-value.test.tsx$' -e '^web/test/pages/candidates/FullScanPage.test.tsx$' -e '^web/test/pages/home/HeldCompaniesCard.test.tsx$' -e '^web/test/pages/home/MacroCard.test.tsx$' -e '^web/test/pages/optiondata/OptionDataPage.test.tsx$' -e '^web/test/pages/options/OptionPage.test.tsx$' -e '^web/test/pages/optiontracking/OptionTrackingPage.test.tsx$' -e '^web/test/pages/positions/PositionsPage.test.tsx$' -e '^web/src/pages/home/CardNotes.tsx$' -e '^web/src/pages/home/OverviewCard.tsx$' -e '^web/test/pages/macro/MacroPage.test.tsx$' | wc -l` prints 0.
9. `grep -c "MissingValue" web/test/components/missing-value.test.tsx` prints at least 1.
10. `grep -c "toBe('无数据')" web/test/components/data-table.test.tsx` prints 0.
11. `grep -c "toBe('缺')" web/test/pages/candidates/FullScanPage.test.tsx` prints 0.
12. `grep -c '<Alert' web/src/pages/home/CardNotes.tsx web/src/pages/home/OverviewCard.tsx | awk -F: '{s+=$2} END {print s}'` prints 0.
13. The result matches docs/design/web-shell-layout.md#2 gap-display.

### 3 phone-nav

Module: web. Size: S.
Files: `web/src/shell/TopNav.tsx`, `web/test/shell/TopNav.test.tsx`
Design: docs/design/web-shell-layout.md#Decisions
Order: after 1.
Goal: Decision 3. One list of items (`ITEMS`, `POSITION_LINKS`) renders twice: the desktop bar (`hidden md:flex`, no `overflow-x-auto`) and, below `md`, a 44 px `Menu` button that opens a `Sheet` with the same links, the current one `aria-current="page"`. The `nav` keeps `aria-label="主导航"`. Existing tests count `a` elements with the sheet closed, so they keep passing; new tests open the sheet.
按 frontend-stack：用 shadcn `sheet` with the shadcn `button` and Lucide `Menu` 做 the phone menu。
Acceptance:
1. `grep -c "from '@/components/ui/sheet'" web/src/shell/TopNav.tsx` prints 1 and `grep -c "Menu" web/src/shell/TopNav.tsx` prints at least 2.
2. `grep -c 'overflow-x-auto' web/src/shell/TopNav.tsx` prints 0.
3. `grep -c 'md:hidden' web/src/shell/TopNav.tsx` prints at least 1 and `grep -c 'hidden md:flex' web/src/shell/TopNav.tsx` prints at least 1.
4. `grep -c 'ITEMS.map' web/src/shell/TopNav.tsx` prints at most 2 (one list, two renderings) and `grep -c "path: '/" web/src/shell/TopNav.tsx` prints 14 (the same fourteen paths as at BASE).
5. `grep -c 'size-11\|min-h-11\|h-11' web/src/shell/TopNav.tsx` prints at least 1.
6. `grep -c "Sheet\|sheet" web/test/shell/TopNav.test.tsx` prints at least 1.
7. `git diff --name-only $(git merge-base HEAD origin/main) | grep -v -e '^web/src/shell/TopNav.tsx$' -e '^web/test/shell/TopNav.test.tsx$' | wc -l` prints 0.
8. The result matches docs/design/web-shell-layout.md#3 phone-nav.

### 4 stale-text

Module: web. Size: S.
Files: `web/src/pages/home/StaleNotice.tsx`, `web/src/pages/home/StaleAlert.tsx`, `web/src/pages/optiondata/OptionDataPage.tsx`, `web/test/pages/home/StaleNotice.test.tsx`, `web/test/pages/home/StaleAlert.test.tsx` (new)
Design: docs/design/web-shell-layout.md#Decisions
Order: none (no file in common with the other entries).
Goal: Decision 4. `StaleNotice` renders `<p data-stale="false" className="text-sm text-muted-foreground">{text}</p>` when not stale and today's destructive `Alert` (`data-stale="true"`, icon and text) when stale. `StaleAlert` keeps `useClock` and its two rules (past `stale_after`, 5-minute skew), renders `StaleNotice` with its flag, and keeps the `data-clock-skew` alert. The importers are not edited, except `OptionDataPage.tsx:53`, which draws its own `Alert` on `is_stale` and now renders `StaleNotice` (same `data-stale` and text, so `OptionDataPage.test.tsx` needs no edit for it).
按 frontend-stack：用 shadcn `Alert` (destructive variant) 做 the stale notice and Tailwind `text-muted-foreground` 做 the grey data date。
Acceptance:
1. `grep -c "text-muted-foreground" web/src/pages/home/StaleNotice.tsx` prints at least 1 and `grep -c "variant=\"destructive\"\|'destructive'" web/src/pages/home/StaleNotice.tsx` prints at least 1.
2. `grep -c "StaleNotice" web/src/pages/home/StaleAlert.tsx` prints at least 2 and `grep -c "<Alert variant={isStale" web/src/pages/home/StaleAlert.tsx` prints 0.
3. `grep -c "stale_after\|SKEW_MS\|data-clock-skew" web/src/pages/home/StaleAlert.tsx` prints at least 3 (the rules stay where they are).
4. `grep -c "data-stale" web/src/pages/home/StaleNotice.tsx` prints at least 1.
5. `grep -c "data-stale\|data-clock-skew" web/test/pages/home/StaleAlert.test.tsx` prints at least 2.
6. `grep -c 'StaleNotice' web/src/pages/optiondata/OptionDataPage.tsx` prints at least 2 and `grep -c '<Alert variant={stale' web/src/pages/optiondata/OptionDataPage.tsx` prints 0.
7. `git diff --name-only $(git merge-base HEAD origin/main) | grep -v -e '^web/src/pages/home/StaleNotice.tsx$' -e '^web/src/pages/home/StaleAlert.tsx$' -e '^web/src/pages/optiondata/OptionDataPage.tsx$' -e '^web/test/pages/home/StaleNotice.test.tsx$' -e '^web/test/pages/home/StaleAlert.test.tsx$' | wc -l` prints 0.
8. The result matches docs/design/web-shell-layout.md#4 stale-text.

### 5 home-grid

Module: web. Size: S.
Files: `web/src/pages/home/HomePage.tsx`, `web/test/pages/home/HomePage.test.tsx`
Design: docs/design/web-shell-layout.md#Decisions
Order: none.
Goal: Decision 1. The card list of `HomePage` (`:80`) becomes `grid grid-cols-1 gap-4 lg:grid-cols-2 2xl:grid-cols-3`; each card sits in a `min-w-0` cell: macro and market one cell each, household `lg:col-span-2 2xl:col-span-1`, held companies and options `col-span-full`. `App.tsx` is not edited and has no max width.
按 frontend-stack：用 Tailwind `grid lg:grid-cols-2 2xl:grid-cols-3` and `min-w-0` on the shadcn `Card` cells 做 the responsive card grid。
Acceptance:
1. `grep -c 'lg:grid-cols-2 2xl:grid-cols-3' web/src/pages/home/HomePage.tsx` prints 1 and `grep -c 'flex flex-col gap-4' web/src/pages/home/HomePage.tsx` prints 0.
2. `grep -c 'col-span-full' web/src/pages/home/HomePage.tsx` prints at least 2, `grep -c 'lg:col-span-2 2xl:col-span-1' web/src/pages/home/HomePage.tsx` prints 1 and `grep -c 'min-w-0' web/src/pages/home/HomePage.tsx` prints at least 5.
3. `git diff $(git merge-base HEAD origin/main) -- web/src/shell/App.tsx | wc -l` prints 0 and `grep -c 'max-w-' web/src/shell/App.tsx` prints 0.
4. `grep -c 'grid-cols' web/test/pages/home/HomePage.test.tsx` prints at least 1.
5. `grep -c 'p-4' web/src/shell/App.tsx` prints 2 (the 16 px gutter of both `<main>` elements stays).
6. `git diff --name-only $(git merge-base HEAD origin/main) | grep -v -e '^web/src/pages/home/HomePage.tsx$' -e '^web/test/pages/home/HomePage.test.tsx$' | wc -l` prints 0.
7. `grep -c 'grid-cols-1' web/src/pages/home/HomePage.tsx` prints 1 (one column below `lg`, so at 390 px the page is one column).
8. The result matches docs/design/web-shell-layout.md#5 home-grid.

### 6 chart-component

Module: web. Size: M.
Files: `web/src/charts/Chart.tsx`, `web/src/charts/spec.ts`, `web/src/pages/home/MarketSummaryCard.tsx`, `web/src/pages/household/HouseholdPage.tsx`, `web/src/pages/optiondata/QuoteChart.tsx`, `web/test/charts/Chart.test.tsx`, `web/test/pages/home/MarketSummaryCard.test.tsx` (the only tests that assert chart props: dashed lines `Chart.test.tsx:66-71`, the band series `MarketSummaryCard.test.tsx:131-137`, and the x-axis thinning test, which fails with a visible y axis; running the suite with the new size, linear lines and a visible axis fails only that file; `OptionDataPage.test.tsx` keeps `data-slots` and the `['auto', 'auto']` domain assertion at `:203-206` and is edited by entry 2 only)
Design: docs/design/web-shell-layout.md#Decisions
Order: after 2.
Goal: Decision 5 (a) to (i) in the one `Chart`. `spec.ts` gains `unit?: string` and `band?: { name: string; low: ChartPoint[]; high: ChartPoint[] }`. `MarketSummaryCard.tsx` fills `band`, drops its two dashed series, and shows a volatility index whose `level` is a gap (`:102-104`, a wrapping Badge sentence today) as `MissingValue` and a figure level as plain text. `HouseholdPage.tsx` drops its two `className` overrides. `QuoteChart.tsx` (the one chart outside `Chart`: bid, ask, last by slot) becomes a `Chart` of kind line (null for a missing cell, `text` from the cell), keeps the `data-slots` attribute of its wrapper and no longer imports `recharts`; its three series carry `hole: true` (a missing quote stays a hole, never a bridge, never zero); its `ariaText` is the caption of its tab (`one.caption`, `QuoteChart.tsx:56`) and its `name` is `option-quote-series`. The other callers that use the default (`OptionsSummaryCard`, `indicator/view`, `ledger/SectionTables`, `macro_rates/view`, `market/MarketPage`, `report/global-assets`, `structuredetail/Payoff`, and the rest of `HouseholdPage`) are not edited and take the new size and look from `Chart.tsx`. `Chart.tsx` stays under 250 lines.
按 frontend-stack：用 shadcn `ChartContainer`, `ChartTooltip`, `ChartLegend` (recharts) 做 every chart, with `ComposedChart`, `Area` for the band, `Line type="linear"`, `YAxis tickCount={5}`, `LabelList` for the last value。
Acceptance:
1. `grep -c 'type="linear"' web/src/charts/Chart.tsx` prints at least 1 and `grep -c 'monotone' web/src/charts/Chart.tsx` prints 0.
2. `grep -c '<YAxis' web/src/charts/Chart.tsx` prints at least 1 and `grep -c ' hide' web/src/charts/Chart.tsx` prints 0 and `grep -c 'tickCount={5}' web/src/charts/Chart.tsx` prints at least 1.
3. `grep -c '<Area' web/src/charts/Chart.tsx` prints at least 1 and `grep -c 'fillOpacity' web/src/charts/Chart.tsx` prints at least 1 and `grep -c 'ComposedChart' web/src/charts/Chart.tsx` prints at least 2.
4. `grep -c 'h-\[180px\] md:h-\[220px\]' web/src/charts/Chart.tsx` prints 1 and `grep -c 'aspect-\|min-h-56' web/src/charts/Chart.tsx web/src/pages/household/HouseholdPage.tsx | awk -F: '{s+=$2} END {print s}'` prints 0.
5. `grep -c 'verticalAlign="top"' web/src/charts/Chart.tsx` prints at least 1 and `grep -c '#[0-9a-fA-F]\{3,6\}' web/src/charts/Chart.tsx` prints 0.
6. `grep -c 'unit?:\|band?:' web/src/charts/spec.ts` prints 2.
7. `grep -c 'dashed: true' web/src/pages/home/MarketSummaryCard.tsx` prints 0 and `grep -c 'band' web/src/pages/home/MarketSummaryCard.tsx` prints at least 2.
8. `grep -c 'LabelList\|label=' web/src/charts/Chart.tsx` prints at least 2 (bar labels stay, the last-value label is added) and `grep -c 'ChartTooltip' web/src/charts/Chart.tsx` prints at least 2.
9. `grep -c "linear\|band\|tick" web/test/charts/Chart.test.tsx` prints at least 3 and `grep -c "band" web/test/pages/home/MarketSummaryCard.test.tsx` prints at least 1.
10. `wc -l < web/src/charts/Chart.tsx` prints a number below 250.
11. `git diff --name-only $(git merge-base HEAD origin/main) | grep -v -e '^web/src/charts/Chart.tsx$' -e '^web/src/charts/spec.ts$' -e '^web/src/pages/home/MarketSummaryCard.tsx$' -e '^web/src/pages/household/HouseholdPage.tsx$' -e '^web/src/pages/optiondata/QuoteChart.tsx$' -e '^web/test/charts/Chart.test.tsx$' -e '^web/test/pages/home/MarketSummaryCard.test.tsx$' | wc -l` prints 0.
12. `grep -c "from 'recharts'" web/src/pages/optiondata/QuoteChart.tsx` prints 0, `grep -c 'data-slots' web/src/pages/optiondata/QuoteChart.tsx` prints at least 1 and `grep -c 'aspect-' web/src/pages/optiondata/QuoteChart.tsx` prints 0.
13. `grep -c 'toLocale\|toFixed\|Intl\.\|Math\.' web/src/charts/Chart.tsx | awk '{print $1}'` prints 0.
14. `grep -c "domain={\['auto', 'auto'\]}" web/src/charts/Chart.tsx` prints at least 1 and `grep -c 'hole' web/src/charts/spec.ts web/src/charts/Chart.tsx | awk -F: '{s+=$2} END {print s}'` prints at least 2.
15. `grep -c 'MissingValue' web/src/pages/home/MarketSummaryCard.tsx` prints at least 2 and `grep -c 'whitespace-normal text-left' web/src/pages/home/MarketSummaryCard.tsx` prints 0.
16. The result matches docs/design/web-shell-layout.md#6 chart-component.

## Checked by CI

ci-web for every entry: `pnpm install --frozen-lockfile`, `tsc`, `eslint`, `vitest` (the test files named in the entry's Files, plus the whole `web/test` run), `pnpm build`, and the dependency check (`scripts/check-frontend-deps.mjs`: no package added, no `@radix-ui/*`). Entry 1 adds no test (copied source). Entries 2 to 6: the new and changed vitest cases of their Files, for example `data-table.test.tsx` (a gap cell shows "—" and keeps its text as `sr-only`; a text column has no sort button; a value column sorts), `missing-value.test.tsx` (the tooltip and the popover carry the text), `TopNav.test.tsx` (the sheet opens and lists eleven items), `StaleAlert.test.tsx` (grey text when not stale, destructive when past `stale_after`, clock skew), `HomePage.test.tsx` (five cards in a grid), `Chart.test.tsx` (linear, band `Area`, y-axis ticks, last-value label). Playwright layout at 390 px and 1440 px (ADR "Layout tests") is run by ci-web where the runner has Chromium. `Contracts: unchanged` on all six pull requests.

## Decided by the dispatcher (2026-11-10)

These were decided by the dispatcher, not by the owner; the owner may overturn them.

1. The chart heights 180 px / 220 px (Decision 5 a) are the dispatcher's numbers. Reason: board 1 measures about 160 px on a 320 px card; the desktop card is wider.
2. The top bar stays outside the content width. Reason: it is the site's navigation, and with no max width there is no column to be inside of.
3. Second pass, same day: 12 px chart text is allowed (Decision 5 i); the grid has 2 columns from `lg` and 3 from `2xl` (Decision 1); the ECharts sentences of the ADR are stale, not edited here (Decision 8, Left out).

## Left out

- Pages other than home keep one full-width stack (`space-y-6`, tables full width): a grid for market, household, positions, options waits until the owner has seen the home grid; then one entry per page.
- Go page-model strings (`internal/pages/**`, for example `internal/pages/market_page/strings.go`): not changed. The sentence in the tooltip is whatever Python sends; making it shorter is a Go design. The same holds for the red stale bar of `/market` ("数据日 11-05"): when the data is stale it stays red and its text is the builder's.
- The options page shows the issuer as `cik:0000320193` and "缺 spot" in cells (comparison finding): data of the builder (`docs/design/option-page-builders.md`), not of this layout.
- The sample shells (`cmd/ot/sample.go`, `SampleBanner.tsx`) and the slider scale (0 to 100 against board 8's -2 to +2): another design.
- What was not measured from the boards: the heights of the household, payoff, ledger and quote charts (only board 1's market and options charts were measured, by eye from the 720 px PNG: about 160 px and 180 px); the board's real card widths on a desktop (boards are 720 px sheets); the 390 px overflow cause (the screenshot at 390 px is cut on the right; the bar's `overflow-x-auto` and wide badges are suspects, not proven; entries 2 and 3 remove both, a layout test at 390 px confirms).
- The ADR's 720 px and ECharts sentences are stale; superseded by this design (width, owner 2026-11-10) and by the whitelist (recharts); the ADR amendment with its manifest (`web/copy-manifest.sha256`) is the owner's. No entry of this file edits `docs/adr/0034-frontend-stack.md`.
- Tick texts with a thousands separator: a producer would send them (`ChartSpec` field of ticks); none exists, so ticks are unformatted.
- Red date bars seen on the owner's screens (macro 10-23, market 11-05) are real-stale cases (`stale_after` past the device clock) and stay red after entry 4.
- Motion, a Storybook, a tooltip on touch for charts: not added.

## Adversarial

A fresh read-only reviewer attacked the file (revision of the commit 6c2b284d) with six questions.

1. Gap against the goal: adopted. The Badge sentence of `MarketSummaryCard.tsx:102` (entry 6), the inline Alert of `OptionDataPage.tsx:53` (entry 4) and the boxed notes of `CardNotes.tsx` and `OverviewCard.tsx` (entry 2, with the tests the suite showed failing) now have owners; `MarketPage.tsx:35` stays, with the reason in entry 2; red date bars of real-stale data stay (Left out).
2. Biggest unnamed risk (tick labels against `noCompute.test.ts:7`): adopted. The browser formats nothing: recharts' tick value plus the unit by concatenation, no thousands separator; entry 6 acceptance 13 greps the four banned patterns.
3. A simpler way (ship entries 2 to 5 and the 4-line chart change, defer band, last-value label and the QuoteChart migration; skip Popover): partly adopted. The band, the last-value label and the QuoteChart move stay because the owner named the charts as the ugliest part and they are one component; entries are separate pull requests, so entry 6 can be cut in two at the dispatcher's word. Popover stays only because a Radix Tooltip does not open on a touch tap and the owner reads on a phone too; if the owner reads on a desktop only, entry 1 and the Popover half of entry 2 are dropped and the Tooltip alone is used.
4. Lock-in: adopted in part. No new package; `ChartSpec` gains optional fields read only by `Chart.tsx`. The ADR edit is dropped (its hash is pinned in `web/copy-manifest.sha256`, owner-reserved); no entry edits the ADR.
5. Claim most likely false: adopted. `rules/frontend-stack.md` is replaced by in-repo evidence (`web/eslint.config.js:8-9`, `frontend-whitelist.json:21`, `docs/design/home-page-revise.md:55,181`); the `OptionDataPage.test.tsx:203-206` domain assertion is named (entry 6 keeps `['auto', 'auto']`); `QuoteChart` keeps its holes by `hole: true` and gets `ariaText` and `name` from its tab caption; `OptionDataPage` is no longer assumed to use `StaleAlert`.
6. Likeliest slip when driven: adopted. The ADR is out of Files (manifest); the malformed acceptance 9 of entry 2 is split into single commands (items 9 to 12); entry 5 item 7 is a grep; the one remaining non-command item of each entry is the last "result matches" line.
