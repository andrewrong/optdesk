# market state chart: the S&P 500 chart shows the close, its 200-day average and a range band, and a volatility block shows implied minus realised

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-21
Revision: 2

Source: owner UI doc rows 4.2 and 4.3 (ask optdesk-5618); owner rule 2026-11-21: computation in Go with the existing library on raw data; nothing is computed in web or in the page.

Goal: on `/market-state` the owner opens the S&P 500 row and sees the index line, its 200-day average and a shaded range band, and under the market rows a block "Implied minus realised volatility" for SPX, NDX and RUT, each with a Chinese gloss. The chart series come from one pass over the stored daily closes, the volatility block from the stored `market.iv_rv20`.

## Today

All at origin/main.
- Exists: `internal/compute/marketstate/history.go` builds the five row histories in one pass over the raw series (`line()`, window `ChartYears` 3, `ChartMax` 800, `marketstateapi.History`); the page calls it as `Config.Market` and reads the raw `SP500` series back 1500 days (`builder.go:216`); the library `talibind.New()` (`internal/compute/talibind/indicators.go:25`, `compute.Indicators` with `SMA` and `BBands`, `internal/compute/api.go:109-117`), imported by nothing but its own package; the stored `market.iv_rv20` (rule `market.iv_rv20`, `ivrv.go` #2119, newest ok reading of five days #3437), read by `home_market` (`ivrv.go`, subjects SPX, NDX, RUT).
- Missing: the 200-day average level and a band per day; `marketstate` has no `Indicators` (`marketstateapi.Input` has Figures, Units, Rules only; `figures.New` takes none); `market_state_page` never reads `market.iv_rv20` (`git grep iv_rv20 -- internal/pages/market_state_page` prints nothing). Whether a live ok iv_rv20 row exists is not verified here; the block shows a Gap when none.

## Decisions

1. No stored per-day reading and no backfill. `marketstate.Compute` adds four lines to `History` in the same one pass as `Sp500Vs200d`: `Close`, `Ma200`, `BandLow`, `BandHigh`, over the whole raw `SP500` series, cut by `line()` to the chart window. The past shows at once.
2. The average is `Indicators.SMA(closes, 200)`, the band `Indicators.BBands(closes, 20, 2, 2, SMA)` (Bollinger, "Bollinger on Bollinger Bands", 2001): lower and upper, 20-day simple average, 2 standard deviations. TA-Lib uses the population standard deviation (divisor n), not the sample one of `Stats.StdDev`; the page text says so. A day before the window is full (NaN) has no point for that line, never a zero. Texts: 2 decimals, `Units.RoundHalfEven`. These numbers are display series, not registered figures: no rules.json or register.toml entry (as `Drawdown` and `VIX` histories have none). `market.sp500_vs_200d` still divides by `Rolling.Mean`; the two averages agree to float noise, equal at 2 decimals.
3. Wiring: `Indicators compute.Indicators` is a new field of `marketstateapi.Input` (interface change, entry 2) because a `compute` sub-package imports no sibling (modules.md); `market_state_page.Config` gains the same field and copies it into `Input` (entry 4); `cmd/ot/market_state_site.go` sets `talibind.New()` (entry 5). A nil `Indicators` leaves the three lines empty, like a nil `Figures`.
4. The band is the Bollinger band, the sourced band of the library; the owner's "range band" is read as this and asked back once (Left out).
5. Payload: `page.market_state_page.v2.json` gains optional fields (v2 payloads stay valid): on the row `sp500_vs_200d` an optional `price_chart` (caption, aria_text, unit, points of day, close, optional ma200, band_low, band_high, each with its text); a top-level optional `vol_gap` (title, hint, rows of three indices, each a Figure in vol points or a Gap, with its day text). The old `history` stays.
6. The page joins the five lines by day (copying text, computing nothing) and builds `price_chart` over the days of `Close`. A day without an average or band point has no such field.
7. `vol_gap` rows read the stored `market.iv_rv20` of SPX, NDX, RUT as `home_market` does (newest ok within `max_age_days` 5, day from the `data_day` parameter, not the row day); figure = fraction times 100, one decimal, half even, signed, day shown when not the page day. This read is new code in `market_state_page`; no readings window is read, and no new readings id exists. Formula in the gloss: implied (VIX, VXN, RVX over 100) minus 20-day realised volatility (21 closes, log returns, sample sd, times the square root of 252).
8. Chinese gloss in a new `zh_texts.go`: `vol_gap.hint` says what the number is (positive: options priced above actual swings; negative: below) and the unit (volatility points); the chart gets one line: 黄线为 200 日简单均线，阴影为布林带（20 日，2 倍标准差，总体标准差）。
9. Web: `按 frontend-stack：用 shadcn chart (recharts) 的 line 和 band 画价格、200 日均线和阴影带`; `按 frontend-stack：用 shadcn table 和 tooltip 画波动率差一块和中文说明`. `ChartSpec` has `band` and `dashed` (`web/src/charts/spec.ts`).

## Modules

contracts (entry 1), compute (entries 2, 3), pages (entry 4), ot (entry 5), web (entry 6).

## Interface

`page.market_state_page.v2.json` (contracts to pages and web, entry 1); `marketstateapi` `Input.Indicators` and `History` lines (compute to pages, entry 2).

## Issues

### 1 ms-chart-1-schema (interface)

Module: contracts. Size: S.
Order: first. Depends on: none.
Plan: P1.
Files: `contracts/schemas/page.market_state_page.v2.json`, `contracts/fixtures/page.market_state_page.v2.ok.json`, `web/src/generated/market_state_page.v2.ts`, `docs/contracts/page-schemas.md`, `internal/contracts/market_state_chart_test.go`
Design: Decisions 5, 7.
Acceptance: optional `price_chart` and `vol_gap` in the schema, not in `required`; generated types regenerated by the generator; the ok fixture holds both and validates; the old v2 fixture without them still validates.

### 2 ms-chart-2-api (interface)

Module: compute. Size: S.
Order: first (may run beside 1). Depends on: none.
Plan: P1.
Files: `internal/compute/marketstateapi/api.go`, `internal/compute/marketstateapi/api_test.go`
Design: Decisions 1, 2, 3.
Acceptance: `Input` has `Indicators compute.Indicators`; `History` has `Close`, `Ma200`, `BandLow`, `BandHigh` of type `Line`; the contract test compiles against both and `marketstate.Compute` still returns a Result.

### 3 ms-chart-3-series

Module: compute. Size: M.
Order: after 2. Depends on: ms-chart-2-api.
Plan: P1.
Files: `internal/compute/marketstate/chart.go`, `internal/compute/marketstate/chart_test.go`, `internal/compute/marketstate/history.go`
Design: Decisions 1, 2.
Acceptance: `History` carries the four lines from one pass over `SP500` through `Indicators`; a series of 250 closes gives 51 average points and the first band point at the 20th close; nil `Indicators` gives empty lines; the band equals population-sd Bollinger on a fixed series in `chart_test.go`.

### 4 ms-chart-4-page

Module: pages. Size: M.
Order: after 1 and 3. Depends on: ms-chart-1-schema, ms-chart-3-series.
Plan: P1.
Files: `internal/pages/market_state_page/page.go`, `internal/pages/market_state_page/builder.go`, `internal/pages/market_state_page/chart.go`, `internal/pages/market_state_page/volgap.go`, `internal/pages/market_state_page/zh_texts.go`, `internal/pages/market_state_page/chart_test.go`, `internal/pages/market_state_page/volgap_test.go`
Design: Decisions 3, 6, 7, 8.
Acceptance: `Config.Indicators` is copied into `Input`; the payload carries `price_chart` joined by day from `History` and `vol_gap` from the stored `market.iv_rv20` (a new read here), validates against entry 1, gaps are named, texts are Chinese in `zh_texts.go`.

### 5 ms-chart-5-wire

Module: ot. Size: S.
Order: after 4. Depends on: ms-chart-4-page.
Plan: P1.
Files: `cmd/ot/market_state_site.go`, `cmd/ot/market_state_site_test.go`
Design: Decision 3.
Acceptance: `marketStateConfig` sets `Indicators` to `talibind.New()`; a test builds the page over a 250-close series and finds `price_chart` with an average point.

### 6 ms-chart-6-web

Module: web. Size: M.
Order: after 1. Depends on: ms-chart-1-schema.
Plan: P1.
Files: `web/src/pages/market_state/MarketRows.tsx`, `web/src/pages/market_state/VolGap.tsx`, `web/src/pages/market_state/MarketStatePage.tsx`, `web/test/pages/market_state/MarketRows.test.tsx`, `web/test/pages/market_state/VolGap.test.tsx`
Design: Decision 9.
Acceptance: the row chart draws close, average and band from the payload; the block shows the figure, its day and the gloss; no arithmetic.

## Left out

- A band other than Bollinger (percent envelope, the alarm cut as a price): ask the owner once if the shaded look is not what he meant.
- Stored readings for the average and band, and any backfill: not needed, the series is computed in one pass.
- The old `/market` page's chart; why the bars end on 11-18 (row 4.1): separate.

## Adversarial

Full answer: kept in the dispatcher draft store (adversarial-answer of this design), not committed.
1. Gap vs goal (backfill, join by day, window read): adopted. No per-day readings, one pass (Decision 1); iv_rv20 joins by `data_day` (7); the new read is named.
2. Rule hash circularity: adopted by removal. No rules or register entry exists.
3. Simpler way: adopted in the form of the `history.go` pass, not as a figure function.
4. Lock-in (band, population sd): adopted; sd stated (2); no stored contract.
5. "No new code" false: adopted. Wiring is entries 2, 4, 5 (3).
6. Modules refusal of register.toml: adopted by removal; every Files path is of its entry's module.
