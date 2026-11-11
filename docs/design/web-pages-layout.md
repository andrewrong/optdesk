# Web pages layout: market, household, positions, options, risk and advice use the desktop width in blocks and fix two real defects

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-11
Revision: 2

Source: the owner boards `docs/design/v2/design-set-shadcn.html` of the option-trader repository (design set v2.2; every board is a 390 px mobile frame, the reports 720 px; there is no desktop board, so the desktop grid is our own, ADR 0034 full width and #1338); the board-against-page screenshot comparison of 2026-11-10 20:41 local (`compare2/index.html` in the owner window's scratchpad `b60ca8e6-5671-42e5-9b0f-4962be3cf288`; its findings fed `web-shell-layout.md`; it is cited here and not redone); `docs/design/web-shell-layout.md` `## Left out` ("per-page layout comes one entry per page once the owner has seen the home grid"); `docs/design/pages-real-data.md` (landed). Read on origin/main at ae9e442f: `web/src/pages/{market,household,positions,options,risk,advice,home}/**`, `web/src/components/ui/tabs.tsx`, `web/src/charts/{Chart.tsx,spec.ts}`, `contracts/schemas/page.{market_page.v1,household_page.v1,positions_page.v3,household_risk.v1,advice_page.v1}.json`, `docs/design/alignment-v25.md`, `frontend-whitelist.json`, `docs/adr/0034-frontend-stack.md`.

Goal: six pages that now show real data use the desktop width in blocks (a grid from `lg`, one column below), and two real defects go: a tab list that cannot wrap and a duplicate title. The front end computes nothing. No schema, no Go string and no shell file changes: `Contracts: unchanged` on every entry.

## What main holds today (read at ae9e442f)

Board numbers are in `alignment-v25.md`: market 7, positions 6, option 5, household 12, risk 25; advice has no board. On a phone every page is one column like its board. Chart content already matches the boards for market and household; no entry adds a chart.

- market (board 7): board order weather chart, index table, VIX term chart, breadth table, one column. `MarketPage.tsx` stacks them in `space-y-6`; a missing block is a wrapping `Badge` (`GapNote`) where shell Decision 2 wants a dash and a note.
- household (board 12): `HouseholdPage.tsx` stacks 13 blocks in one column, full width on a desktop; the income chart overrides the shell height with `className="h-44 w-full"`; `householdToSpec.ts` builds the trend, income and month-change specs; the trend spec has no `hole`.
- positions (board 6, "stays as is", owner 2026-11-08; owner word 5: stocks first, options after): `Tabs` over one to eight account views (positions_page v3); `TabsList` is `w-fit` and `h-8` (`web/src/components/ui/tabs.tsx:27`), so eight tabs overflow on a phone.
- options (board 5): `OptionPage.tsx` has the same tab list; opportunity cards are `grid-cols-1 sm:grid-cols-2` where the board has one column. Tracking is board 21, another page, not covered.
- risk (board 25): board order delta, vega, beta-weighted, sigma, standard metrics, each with its sentence; `risk/view.tsx` renders delta, beta, sigma, vega, metrics in one column.
- advice (no board): `AdvicePage.tsx` shows the title twice (`h1` and `CardTitle`) and wraps the table in `overflow-x-auto`, which `ui/table.tsx` already provides. Its gap alert already exists and is not touched.
- home (board 1): the grid `grid-cols-1 lg:grid-cols-2 2xl:grid-cols-3` already exists; no entry.

## Modules

None. All entries edit the `web` module's paths `web/src/pages/**` and `web/test/pages/**` (`docs/design/modules.md`, web rows).

## Interface

None. The page schemas and `ChartSpec` are not edited.

## Decisions

1. Desktop grid (market, household, risk): the page root keeps `min-w-0`; blocks sit in `grid grid-cols-1 gap-4 lg:grid-cols-2`; a wide block is wrapped in a cell with `lg:col-span-2`. Each cell is a `div` with `data-cell="<block name>"` so a test reads the classes from the DOM (jsdom has no layout). Six pages repeating the class string is accepted; a shared `BlockGrid` helper is possible later as its own first entry and is not added now (no page needs it to be correct).
2. Block order is the page's order today, except risk, which takes board 25's order (a page choice; the blocks are separate fields). No other page reorders.
3. Tab list (positions, options): `TabsList` gets `className="group-data-horizontal/tabs:h-auto w-full flex-wrap"`. `cn` (tailwind-merge) replaces the same-variant `group-data-horizontal/tabs:h-8` by `h-auto` and `w-fit` by `w-full`, so `web/src/components/ui/tabs.tsx` does not change; a plain `h-auto` would not beat the variant class and is wrong. Each page entry tests the rendered class names of the list.
4. Charts: no page sets a chart height; the `Chart.tsx` default applies. `hole: true` on a line series that can have a null point. No `unit` is invented: the payload has no display unit for the trend.
5. A missing figure is the shared dash cell (shell Decision 2); a whole missing block keeps an `Alert` with the producer's text.
6. An acceptance item on a file that holds CJK text is never a byte or line diff of that file; it is a test, or a grep of a class name or identifier.

## Issues

Tests use no clock, no sleep, no file and no port; each runs with `vitest`. Each acceptance item is a test or a grep that prints one number or nothing. The existing tests of a page stay green and are extended. Each entry carries `Contracts: unchanged`.

### 1 market-layout

Module: web. Size: S.
Files: `web/src/pages/market/MarketPage.tsx`, `web/test/pages/market/MarketPage.test.tsx`.
Design: docs/design/web-pages-layout.md#Decisions
Order: first.
Goal: on a desktop the weather block and the breadth block are full width and the index table and the VIX term chart sit side by side; below `lg` the order and the one column of board 7 are unchanged.
按 frontend-stack：用 recharts（经 `web/src/charts/Chart.tsx`）做 the close line with its 200-day line and the two term lines。
按 frontend-stack：用 shadcn `popover` 与 `tooltip` 做 the reason of a missing figure（共用单元格）。
Acceptance:
1. A test renders the `ok` fixture and asserts the `data-cell` nodes are `weather`, `indices`, `term`, `breadth` in that order, `weather` and `breadth` have the class `lg:col-span-2`, and `indices` and `term` do not.
2. `grep -c 'space-y-6' web/src/pages/market/MarketPage.tsx` prints 0 and `grep -c 'lg:grid-cols-2' web/src/pages/market/MarketPage.tsx` prints 1.
3. `grep -c 'GapNote' web/src/pages/market/MarketPage.tsx` prints 0; the test with the `gap` fixture asserts a missing weather chart, term and breadth each render an `Alert` with the `Gap.text` and `data-gap` equal to the reason.
4. A test with a null `today` point asserts that series of the term spec has `hole` true and that point `value` null.
5. `grep -nE 'parseFloat|Number\(|toFixed|\.reduce\(' web/src/pages/market/MarketPage.tsx` prints nothing.

### 2 household-layout

Module: web. Size: M.
Files: `web/src/pages/household/HouseholdPage.tsx`, `web/src/pages/household/householdToSpec.ts`, `web/src/pages/household/MoneySections.tsx`, `web/test/pages/household/householdPage.test.tsx`.
Design: docs/design/web-pages-layout.md#Decisions
Order: first.
Goal: the household page keeps its payload order and puts two half blocks side by side from `lg`; one column below `lg`. Wide cells (`lg:col-span-2`): overview, accounts, composition, movers, income, obligations, concentration. Half cells: trend, stress, month_change, cash_after, allocation, long_term.
按 frontend-stack：用 recharts（经 `web/src/charts/Chart.tsx`）做 the trend line, the income bars per currency and the month-change bars。
按 frontend-stack：用 shadcn `card` 与 `table` 做 the money blocks。
Acceptance:
1. A test renders the fixture and asserts the `data-cell` values equal, in order, the block names listed in `householdPage.test.tsx` on main (the test holds that list), with `lg:col-span-2` on exactly the seven wide cells.
2. `grep -c 'lg:grid-cols-2' web/src/pages/household/HouseholdPage.tsx` prints 1.
3. `grep -c 'h-44' web/src/pages/household/HouseholdPage.tsx` prints 0.
4. In `householdToSpec.ts` the trend series set `hole: true`; a test with a null trend point asserts it. The count `grep -c 'unit' web/src/pages/household/householdToSpec.ts` equals the count on main.
5. `grep -nE 'parseFloat|Number\(|toFixed|\.reduce\(' web/src/pages/household/HouseholdPage.tsx web/src/pages/household/householdToSpec.ts` prints nothing.
6. `wc -l` of each listed source file prints under 600; `MoneySections.tsx` is changed only if the cell wrapper needs a prop.

### 3 positions-tabs

Module: web. Size: S.
Files: `web/src/pages/positions/PositionsPage.tsx`, `web/test/pages/positions/PositionsPage.test.tsx`.
Design: docs/design/web-pages-layout.md#Decisions
Order: first.
Goal: with one to eight account views the tab list wraps instead of overflowing; stocks, then options, then trades stay stacked, as board 6 and the owner's word have them.
按 frontend-stack：用 shadcn `tabs`（radix-ui Tabs）做 the account views。
Acceptance:
1. A test renders a payload with eight views and asserts eight `tab` nodes and that the `tablist` class contains `group-data-horizontal/tabs:h-auto`, `w-full` and `flex-wrap`.
2. `git diff --name-only $(git merge-base HEAD origin/main)` does not list `web/src/components/ui/tabs.tsx`.
3. A test asserts that in each view the `data-table` nodes appear in the order `stocks`, `options` and, when present, `trades`.
4. `grep -c 'xl:grid-cols' web/src/pages/positions/PositionsPage.tsx` prints 0.

### 4 options-layout

Module: web. Size: S.
Files: `web/src/pages/options/OptionPage.tsx`, `web/test/pages/options/OptionPage.test.tsx`.
Design: docs/design/web-pages-layout.md#Decisions
Order: first.
Goal: the market tab list wraps, and the opportunity cards are one column below `lg` (board 5) and two from `lg`, never more.
按 frontend-stack：用 shadcn `tabs` 做 the market panes。
Acceptance:
1. A test asserts the `tablist` class contains `group-data-horizontal/tabs:h-auto`, `w-full` and `flex-wrap`.
2. `grep -c 'sm:grid-cols-2' web/src/pages/options/OptionPage.tsx` prints 0, `grep -c 'lg:grid-cols-2' web/src/pages/options/OptionPage.tsx` prints 1 and `grep -c 'grid-cols-3' web/src/pages/options/OptionPage.tsx` prints 0.
3. The existing test of `data-chain` and `data-chain-empty` stays green.
4. `git diff --name-only $(git merge-base HEAD origin/main)` does not list `web/src/pages/options/Tracking.tsx`.

### 5 risk-layout

Module: web. Size: S.
Files: `web/src/pages/risk/view.tsx`, `web/test/pages/risk/riskPage.test.tsx`.
Design: docs/design/web-pages-layout.md#Decisions
Order: first.
Goal: the risk page shows its blocks in board 25's order, delta, vega, beta, sigma, standard metrics; delta and vega side by side from `lg`, beta and sigma side by side, standard metrics full width. No chart.
按 frontend-stack：用 shadcn `card` 与 `table` 做 the blocks and tables。
Acceptance:
1. A test renders the fixture and asserts the `data-cell` values in order `delta`, `vega`, `beta`, `sigma`, `standard-metrics`, with `lg:col-span-2` on `standard-metrics` only.
2. `grep -c 'lg:grid-cols-2' web/src/pages/risk/view.tsx` prints 1.
3. `grep -c 'Chart' web/src/pages/risk/view.tsx` prints 0.
4. `wc -l web/src/pages/risk/view.tsx` prints under 600.

### 6 advice-title

Module: web. Size: S.
Files: `web/src/pages/advice/AdvicePage.tsx`, `web/test/pages/advice/advicePage.test.tsx`.
Design: docs/design/web-pages-layout.md#Decisions
Order: first.
Goal: the advice page shows its title once and its table scrolls once.
按 frontend-stack：用 shadcn `table` 做 the advice table。
Acceptance:
1. `grep -c 'payload.title' web/src/pages/advice/AdvicePage.tsx` prints 1.
2. `grep -c 'overflow-x-auto' web/src/pages/advice/AdvicePage.tsx` prints 0.
3. A test with rows asserts exactly one node with the title text; the existing gap test stays green.

## Left out

- Home: the grid exists; no entry.
- Charts: no new chart. Risk bars of delta and vega are deferred: the payload gives `MoneyCell.unit` as an enum and a `currency`, not a display unit, and rows can mix currencies on one axis; they need an interface design with the owner first. A trend `unit` is likewise not invented.
- A shared `BlockGrid` component: optional, later, as its own first entry.
- Tracking (board 21), the option card text (`option-pages-v22.md`) and pages without real data: not here.
- A desktop board: none exists; if the owner draws one, an entry follows.

## Checked by CI

Every entry: `pnpm install --frozen-lockfile`, `tsc`, `eslint`, `vitest` and the build of `web/` pass, and `scripts/check-frontend-deps.mjs` finds no package outside `frontend-whitelist.json`.

## Adversarial

Full answer: the dispatcher's scratchpad file `adversarial-web-pages-layout.md`.

- Gap: boards are 390 px only and were unread: adopted; named in Source, the desktop grid is declared our own.
- Gap: risk order and bars not on board 25: adopted; board order, bars deferred.
- Gap: positions side by side against the owner: adopted; stacked, tab wrap only.
- Gap: options three columns: adopted; at most two from `lg`.
- Wrap fix loses to `group-data-horizontal/tabs:h-8`: adopted (Decision 3); `ui/tabs.tsx` does not change, tested by class.
- Simpler way, drop single-column grids: partly; market keeps its desktop pair (own grid), positions and option cards do not grow; shared helper not added.
- Lock-in on class strings: partly; tests read `data-cell` classes, greps stay for removed strings and one grid class.
- Risk missing-figure test impossible: adopted; item removed.
- Household trend `unit`/`hole` and `householdToSpec.ts` missing from Files: adopted.
- Household jsdom layout check and "prints no line not on main": adopted; replaced by `data-cell` tests and greps that print nothing.
- Advice no-op items: adopted; only title and scroll remain.
- Options CJK diff check: adopted (Decision 6); item removed, Tracking left out.
