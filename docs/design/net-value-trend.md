# net value trend: the family page draws the family's daily net value over time, with one line per account

Author: the optdesk designer (draft 2026-11-21)
Owner ground: owner UI doc, household pages row "家庭净值走势" (daily family net value over time, per account optional)
Date: 2026-11-21

Source: the main of 2026-11-21. The page `/household` of a book of two or more accounts is built by `household_page.BuildFamily` (`internal/pages/household_page/family.go`). Its block `trend` is today the gap `not_built` (`detailGaps`). The page of one account already draws a trend: `Config.capture` reads the stored account values of each of the last 90 session days (`store.AccountReader.Values`) and keeps the `NetLiquidation` row of each day as `capture.history`; `trendOf` turns it into a `trend` block; the web (`web/src/pages/household/HouseholdPage.tsx`, `trendToSpec`) draws `trend.series` as lines with recharts. `readBook` already reads that capture (with its history) for every account of the book and throws the history away. The schema `page.household_page.v1` already says `trend.series` holds "one series for each scope, the household first" (at most 4 series), so the payload needs no change.

Rules carried: the front end renders data and computes nothing; Go computes; charts are recharts through the shadcn chart component (`web/src/charts/Chart.tsx`); hints and labels are Chinese and a term is explained next to it; no ticker and no account id in this file; a page states the result the owner sees.

## Today

Daily net value IS stored: one `NetLiquidation` row per account and capture day in the account values (written by the account summary capture), read by `store.AccountReader.Values(alias, day)`. So no store, migration or job entry is needed. What is missing is only the sum over accounts.

Owner-visible today: on `/household` (the family view) the block "走势" shows the grey sentence that it is not built. Choosing one account shows that account's 90-day line.

Not stored and not claimed: a day on which an account has no capture has no value for it (the Flex statement net values exist for some days but are another source and are not mixed in here; see Left out). How many days the account values hold for each account is not known from the code; the page prints the first and the last day it draws, so the owner sees the real span.

## What the owner sees

On `/household`, the block "走势" (title kept) shows one chart. Example; all names and numbers are made up:

    走势 Trend
    全家净值每天的变化；细线是各账户。净值 = 账户里的现金加所有持仓按市价折算的总价值（美元）。
    [chart: thick line "全家合计", thin lines "主账户", "配偶账户", "女儿账户"; x = day, y = USD]
    全家合计：2026-08-03 至 2026-11-20，共 78 天，最后一天 341,200 美元。
    有 12 天没有画点：当天至少一个账户没有净值记录。
    日元账户按当天的美元兑日元汇率折成美元。

Rules the owner can read off the page:

1. A family point exists only on a day on which every account counted in the family total has a net value. Other days are left out, never filled and never zero. The sentence "有 N 天没有画点" says how many session days of the window were left out and why.
2. The family line is the sum of the account lines of the same day, so at any point the thin lines add up to the thick line. The thick line is first in the legend.
3. A yen account is converted with the stored USDJPY rate of that day (latest rate on or before the day). A yen day without a rate has no point for that account, so it leaves the family out that day. The sentence "日元账户按当天汇率折成美元" appears only when a yen account is counted.
4. An account the family total does not count (no data, stale, read error) is not in the trend either; the page already names it in a lead note.
5. With 4 or more counted accounts the thin lines are left out (the schema holds 4 series: the family and at most 3 accounts) and the sentence "账户超过 3 个，只画全家合计" says so. The family line is always drawn.
6. Fewer than 2 family points: the block is the gap `no_data` with "全家净值记录不足 2 天，画不出走势". Never an empty chart.

## Decisions

1. Source is the stored account values (`NetLiquidation`), the same source as the one-account trend, so the family line at the data day equals the family net total of the page. Reason: one source, one number; Flex nav rows are the later statement and would show two different numbers for one day.
2. Go computes the sums (a new figure `FamilyTrendFigure`, exact decimal arithmetic, half-even 2 places); the page builder copies its text; the web draws `trend.series` with the existing `trendToSpec`. No web logic change.
3. Dollars, not percent. The per-account trend is already in dollars and the web already draws it; the register figure `household.trend` (percent since first point) is a separate later row. Reason: smallest step that closes the owner row.
4. Per-account lines are lines on the same axis, not a stacked area. `ChartSpec` stacks bars only; a stacked area is a new chart kind and a change of the web chart module. The thin lines plus the thick sum show the same facts. A stacked area is Left out.
5. The window is the 90 session days that `BuildFamily` already reads (`historyDays`). No new reader call per account.
6. No contract change: `series[0].scope` is `household` (`familyAliasKey`), the account series use the alias as scope key like the one-account page, `label` is `familyAccountLabel(alias)`. `Contracts: unchanged`.
7. A `Config.FamilyTrend` constructor is injected like `Config.Family`; nil keeps today's gap `not_built`. The field and its wiring in the site land in one entry (2).
8. Daily USDJPY rate comes from the existing `Config.Fx` reader (`LatestFxRate(pair, day)`), read once per history day that a counted yen account has, none when no counted account is in yen. No new store.

## Modules

`docs/design/modules.md` is not changed. Paths match the rows compute, pages, ot and web; the first matching row owns each path. No store, dataset, migration or `contracts/` change; the auto-deploy is not held. `cmd/ot/household_site.go` is small and is the only ot file touched.

- compute (entry 1): `internal/compute/blockapi/api.go` (470 lines), `internal/compute/blockapi/api_test.go`, new `internal/compute/pagefig/family_trend.go` and `family_trend_test.go`, `docs/contracts/compute.md`.
- pages and ot (entry 2, one judged two-module entry, because the site wiring is two lines and must land with the field it sets): in `household_page`, new `family_trend.go` and `family_trend_test.go`; edited `family.go` (290 lines), `page.go` (Config), `zh_texts.go`, `docs/contracts/pages.md`; in ot, edited `cmd/ot/household_site.go` and `cmd/ot/household_site_test.go`.
- web (entry 3): edited `web/fixtures/household-page/family.json`, `web/test/pages/household/householdFamily.test.tsx`; `householdToSpec.ts` is read, not changed unless the test shows the thick line cannot be told from the thin ones (then one `color` or `dashed` field of the payload-independent spec, still no computation).

No file listed is over 600 lines after the change; none is in `docs/design/ratchet.json`.

## Interface

In `internal/compute/blockapi/api.go`, declarations only (the implementation is `pagefig.NewFamilyTrend`):

    // FamilyTrendAccountIn is one counted account and its net liquidation of each day it has one.
    type FamilyTrendAccountIn struct {
        Alias string
        Days  []DayValue // ascending by day; Currency "USD" or "JPY"; a negative or malformed value is no point
    }
    // FamilyTrendIn: the counted accounts and the USDJPY rate of each day a yen account has a value (decimal text).
    type FamilyTrendIn struct {
        Accounts []FamilyTrendAccountIn
        UsdJpy   map[contracts.Day]string
        Window   []contracts.Day // the session days asked, ascending
    }
    type FamilyTrendPoint struct {
        Day contracts.Day
        Net string // USD, 2 decimal places, half-even
    }
    type FamilyTrendAccountOut struct {
        Alias  string
        Points []FamilyTrendPoint
    }
    // FamilyTrendResult: Family holds only days on which every account has a point. LeftOut is the number of
    // Window days without a family point. Gap is "" or "no_data" (fewer than 2 family points).
    type FamilyTrendResult struct {
        Family   []FamilyTrendPoint
        Accounts []FamilyTrendAccountOut // input order
        LeftOut  int
        YenUsed  bool
        Gap      string
    }
    type FamilyTrendFigure interface {
        // FamilyTrend sums the accounts of in per day. Pure: no I/O, no clock, no float.
        FamilyTrend(in FamilyTrendIn) FamilyTrendResult
    }
    type NewFamilyTrendFigureFunc func() FamilyTrendFigure

It is not a method of `FamilyFigure` or `PageFigures`, so their callers are unchanged.

## Issues

Build order: entry 1 first (interface); entries 2 and 3 depend only on it (3 needs 2 only to see the line on a live page, not to compile). Every entry says `Contracts: unchanged`.

### 1 nvt-1-figure (interface)

Module: compute. Order: first. Files: `internal/compute/blockapi/api.go`, `internal/compute/blockapi/api_test.go`, `internal/compute/pagefig/family_trend.go`, `internal/compute/pagefig/family_trend_test.go`, `docs/contracts/compute.md`.

### 2 nvt-2-page

Module: pages and ot. Order: after 1. Files: `internal/pages/household_page/family_trend.go`, `internal/pages/household_page/family_trend_test.go`, `internal/pages/household_page/family.go`, `internal/pages/household_page/page.go`, `internal/pages/household_page/zh_texts.go`, `docs/contracts/pages.md`, `cmd/ot/household_site.go`, `cmd/ot/household_site_test.go`.

### 3 nvt-3-web

Module: web. Order: after 1. Files: `web/fixtures/household-page/family.json`, `web/test/pages/household/householdFamily.test.tsx`.

## Left out

- Percent change since the first day and the "最后一点比第一点" badge of register figure `household.trend`: a later row; the payload fields `change` stay unused.
- A stacked area of the accounts: needs a new chart kind in `web/src/charts`; the lines show the same data.
- Filling days from the Flex statement net values, and days before the first capture: another source, two numbers for one day.
- A toggle to hide the thin lines: a display choice that needs a payload hint; the lines are thin and the legend names them.
- A daughter-only or wife-only scope line set by the group file (register `household.trend` scopes): not in this row.
- Benchmarks (market lines) on the trend: separate row.
- A window longer than 90 session days: `historyDays` is shared with the one-account page.

## Checked by CI

- `ci-go`: `go build ./...`, `go vet ./...`, `go test ./internal/compute/... ./internal/pages/... ./cmd/ot/...`.
- `ci-web`: `pnpm install --frozen-lockfile`, `tsc`, `eslint`, `vitest`, build.

## After merge (checked by the dispatcher, not by review)

On the live `/household` the thick line ends at the family net total of the page's data day, and the thin lines add up to it on a day read off the tooltip.

## Adversarial

Run as a fresh reader on 2026-11-21.

1. Gap vs goal. The owner row says "daily family net value over time, per account stack optional". Built: sum line plus per-account lines, no stack. Not adopted as a stack: a new chart kind costs a web chart change and shows no extra fact (Decision 4). Named in Left out so the owner can ask for it.
2. Biggest unnamed risk: history length. The design does not know how many days of account values exist per account; the family line starts only when every counted account has a value, so a recently added account (or one captured only on some days) shortens or fragments the line. Adopted: the page prints first day, last day, count and the left-out count (rule 1, 6), so the shortness is visible instead of silent. Not adopted: fall back to Flex nav rows to fill (two sources, two numbers).
3. Simpler way: drop yen conversion and per-day rates. Not adopted: the family total of the page counts yen accounts, so a trend that omitted them would end at a different number than the total beside it. Adopted instead: rate read only when a counted yen account exists (Decision 8).
4. Lock-in: the interface fixes the shape `Family` + `Accounts` per day in USD text. A later percent or stacked view reads the same points, so it adds a field, not a rewrite. The 4-series schema cap is the real lock-in: rule 5 degrades to the family line alone rather than change the schema now; raising the cap is a contracts change for a later issue.
5. Claim most likely false: "the family line at the data day equals the family net total" (Decision 1). The total uses the newest capture of each account and may use an earlier day for an account (`familyLineEarlierDay`), while the trend needs the same day for all. They can differ when accounts' newest captures are on different days. Adopted: the after-merge check compares only on a day when all captures share the day, and the page's window sentence names the last trend day, which can be earlier than the data day. Acceptance of entry 2 tests the differing-day case.
6. Likeliest slip: the page builder summing by itself because "it is only addition" (the package test forbids `math/big`, so the slip would show as string arithmetic or float). Adopted: acceptance of entry 2 requires the builder to call the figure and the purity test to stay unchanged. Second slip: entry 3 growing a web computation (percent, sum); acceptance 4 of entry 3 forbids it.
