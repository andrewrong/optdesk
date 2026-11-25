# nasdaq-drawdown: a Nasdaq-100 drawdown card beside the S&P 500 one on the market state page

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-25
Revision: 1

Source: owner ask optdesk-6218 (2026-11-25); owner decision of the same day: Nasdaq-100 (NDX) from the stored IB bars, no new collection; the schema follows the page version policy.

Goal: on `/market-state` the owner sees, right after the card "S&P 500 drawdown", a card "Nasdaq-100 drawdown" with the same method: the newest close against the highest of the newest 60 closes ("Nasdaq-100 is X% below its 60-session high of <day>"), the Chinese window line, the state (normal, caution, alarm; trial) with the two cut lines, "A notice only; nothing is blocked.", and a Chinese hint. The two cuts are percentiles (20th and 5th) of the Nasdaq-100's own stored history; the web renders and computes nothing.

## Today

All at origin/main (checked against 4b28845e).
- Exists: the S&P card is the row `sp500_drawdown`: `internal/compute/marketstate/marketstate.go` (`drawdown`, `drawdownAt`, `Params.HighSessions` 60, `MaxAgeDays` 4), the history line in `internal/compute/marketstate/history.go` (`ChartYears` 3, `ChartMax` 800), `internal/compute/marketstateapi/api.go` (`Input.SP500`, `Result.Drawdown`, `History.Drawdown`), `internal/pages/market_state_page/rows.go` (`drawdownRow`, `buildRows` makes five rows), `strings.go` (`labelDrawdown`), `zh_texts.go` (`windowDrawdown`), `ranges.toml` (row `sp500_drawdown`, percentile cuts 20 and 5, the Chinese hint) and `ranges.go` (state and cuts from the row's own history). The S&P series is FRED `SP500` (`fred_series`, `rd.series_point_r1`, 799 days since 2023-11-01), read by `readSeries` in `builder.go`.
- Exists for Nasdaq: the index NDX (instrument 2415, type index) is collected daily by `bars_refresh` (`[[index]] NDX`, IB, basis `trades`): 1511 sessions, 2020-11-17 to 2026-11-23, in `rd.price_bar_r1`. `fred_series` collects no Nasdaq index (only VXNCLS, the volatility index). QQQ is not an instrument and the Nasdaq Composite is not stored.
- Missing: any Nasdaq series in `marketstateapi.Input`; a reader of index ids in `market_state_page` (its `StockIDReader` reads type stock only, so NDX is not found through it; `instrument.OpenIndexIDs` and `store.InstrumentName` exist and `cmd/ot/index_ids_bind.go` opens it for compute_figures); the sixth row in the schema (`page.market_state_page.v2.json` fixes the row id enum and 5 rows).
- Page version policy: `docs/contracts/00-protocol.md` freezes a page schema at its first consumer and a later change is a new version; `docs/contracts/page-schemas.md` records the rule's practice for a change nothing can reject: `page.positions_page.v4` took the optional `accounts` "without a version change" and `page.market_state_page.v2` took `range`, `hint`, `price_chart`, `vol_gap` and `bars_source` in place (#3449, #3565, #3407).

## Decisions

1. Series: Nasdaq-100, NDX, from the stored daily `trades` closes of `rd.price_bar_r1`. No new collection, no change of `bars_refresh` or `fred_series`. Why not the others: the Nasdaq Composite and QQQ are not stored; FRED NASDAQ100 would be a new series in `fred_series`, `internal/sources/fred/mapping.go`, a migration of the series registry and a backfill, for the same index. If the owner later wants the Composite, it is one more row of the same shape.
2. Method is the S&P card's, by the same function: `drawdownAt(series, last, HighSessions)` and the same age rule (`MaxAgeDays`), over a `compute.Series` of the NDX closes. The history line is built by the same `line()` over the same window (`ChartYears` 3, `ChartMax` 800), so the cuts are the 20th and 5th percentile of the Nasdaq-100's own history in that window, by `ranges.go` unchanged. Nothing is shared with the S&P row: its series, history and cuts stay as they are.
3. Compute interface (entry 2): `marketstateapi.Input` gets `NDX compute.Series`; `Result` gets `NasdaqDrawdown liquidityapi.Num` (same meaning as `Drawdown`; Aux is the day of the high); `History` gets `NasdaqDrawdown Line`. An empty `NDX` gives the Gap `no_input`, a stale one `stale_input`, fewer than 60 closes `short_history`, exactly as the S&P row.
4. The page reads the NDX closes itself, as it reads the sector funds: a new `IndexIDReader` interface in `market_state_page` (`IndexIDs(ctx, names) ([]store.InstrumentName, error)`, satisfied by `instrument.IndexIDs`), a new `Config.IndexIDs`, then `Bars.PriceBars(id, basis trades, from, to)` for `seriesBackDays` (1500) days, ascending, close text read to a number as `seriesOf` reads a FRED value. No `IndexIDs` or no `Bars` gives the gap `no_reader`; no instrument named NDX gives `no_data`; two instruments of one name give `missing_input` (as the funds). The bar rows read are included in the page's `bars_source` as the funds' are.
5. Wiring in `cmd/ot/market_state_site.go`: a second site opener (as `openMarketStateIDs`) opens `instrument.OpenIndexIDs` with the pages line; no opener or line gives no reader and the card is the gap `no_reader`. This is folded into the pages entry, entry 4 (as in `docs/design/market-state-chart-ma-ivrv.md` entry 4), so nothing waits on a sibling's unmerged work.
6. Payload: the schema `page.market_state_page.v2` stays v2 and is edited in place: the row id enum gains `nasdaq_drawdown`, `market.rows` goes from exactly 5 to 5 to 6 items (minimum stays 5, so every stored v2 payload stays valid and no consumer's input is rejected), and the generated type gains the one union member. This follows the practice of `page-schemas.md` for a change that rejects nothing old (the protocol rule's "new version" applies to a change that does); `Contracts:` of entry 1 says `page.market_state_page.v2 additive`. The row order is `sp500_vs_200d`, `sp500_drawdown`, `nasdaq_drawdown`, `vix`, `vix_term`, `hy_spread`; the page always emits six.
7. Texts. English: label `Nasdaq-100 drawdown`; "Nasdaq-100 is X% below its 60-session high of <day>" and "Nasdaq-100 is at its 60-session high of <day>". Chinese: the window line is the S&P card's `windowDrawdown` (60 个交易日水平：最新收盘价对比最近 60 个收盘价中的最高值); the hint in `ranges.toml` is "纳斯达克 100 距近 60 个交易日最高点的回撤。区间按它自己的历史分位给出（越低越差），试用，只是提示。"; the row has `direction = "low_bad"`, cuts caution 20 and alarm 5, `unit_text = "%"`, `decimals = 2`.
8. The row's day joins the page's data days under the source name `NDX`, so a stale NDX shows in the freshness line as a stale S&P does. Source of the number in the card is the same wording as the S&P row's.
9. Web: the rows are rendered by `id` from the payload (`MarketRows.tsx` maps `block.rows`; the chart, badge, cuts and hint are generic), so no source file changes; entry 5 adds the test that six rows render the Nasdaq card. `按 frontend-stack：用 现有 MarketRows 行组件（shadcn 卡片与 chart，按 id 渲染）做 第六张纳斯达克回撤卡，只加测试`.

## Modules

contracts (entry 1), compute (entries 2, 3), pages and ot (entry 4, a judged two-module entry: the opener in `cmd/ot` is folded in), web (entry 5). No row of `docs/design/modules.md` changes. Entries 1 and 2 are the (interface) entries.

## Interface

- `page.market_state_page.v2.json` (contracts to pages and web): row id `nasdaq_drawdown` and up to 6 rows (Decision 6), with `web/src/generated/market_state_page.v2.ts`.
- `marketstateapi` (compute to pages): `Input.NDX`, `Result.NasdaqDrawdown`, `History.NasdaqDrawdown` (Decision 3).
- `market_state_page.IndexIDReader` and `Config.IndexIDs`: inside the pages module, used by `cmd/ot` through `Config`.

## Issues

### 1 nasdaq-drawdown-1-schema (interface)

Module: contracts. Size: S.
Order: first. Depends on: none.
Plan: P2.
Files: `contracts/schemas/page.market_state_page.v2.json`, `web/src/generated/market_state_page.v2.ts`, `docs/contracts/page-schemas.md`, `internal/contracts/market_state_nasdaq_test.go`
Design: Decision 6.
Acceptance: the schema accepts a payload with the six rows in the order of Decision 6 and still accepts the stored five-row `contracts/fixtures/page.market_state_page.v2.ok.json`; the generated type holds `nasdaq_drawdown` and is regenerated by the generator; `TestMarketStateNasdaqRow` (new) builds the six-row payload from the ok fixture and validates it; `docs/contracts/page-schemas.md` names the change and `Contracts:` reads `page.market_state_page.v2 additive`.

### 2 nasdaq-drawdown-2-api (interface)

Module: compute. Size: S.
Order: first (may run beside 1). Depends on: none.
Plan: P2.
Files: `internal/compute/marketstateapi/api.go`, `internal/compute/marketstateapi/api_test.go`
Design: Decision 3.
Acceptance: `Input` has `NDX compute.Series`, `Result` has `NasdaqDrawdown liquidityapi.Num` and `History` has `NasdaqDrawdown Line`; the contract test (extended) compiles against all three and a `marketstate.Compute` result still decodes.

### 3 nasdaq-drawdown-3-compute

Module: compute. Size: S.
Order: after 2. Depends on: nasdaq-drawdown-2-api.
Plan: P2.
Files: `internal/compute/marketstate/marketstate.go`, `internal/compute/marketstate/history.go`, `internal/compute/marketstate/nasdaq_test.go`
Design: Decisions 2, 3.
Acceptance: `Compute` fills `NasdaqDrawdown` and `History.NasdaqDrawdown` from `Input.NDX` with `drawdownAt` and the S&P age rule, and leaves `Drawdown` unchanged for the same input; `TestNasdaqDrawdown` (new) shows, on a fixed 80-close series, the percent and the high's day, and the gaps `no_input`, `stale_input` and `short_history`.

### 4 nasdaq-drawdown-4-page

Module: pages and ot (a judged two-module entry: the opener in `cmd/ot` is folded in, so no issue waits on a sibling's unmerged work). Size: M.
Order: after 1, 2. Depends on: nasdaq-drawdown-1-schema, nasdaq-drawdown-2-api.
Plan: P2.
Files: `internal/pages/market_state_page/page.go`, `internal/pages/market_state_page/builder.go`, `internal/pages/market_state_page/rows.go`, `internal/pages/market_state_page/ndx.go`, `internal/pages/market_state_page/strings.go`, `internal/pages/market_state_page/ranges.toml`, `internal/pages/market_state_page/ndx_test.go`, `internal/pages/market_state_page/rows_test.go`, `internal/pages/market_state_page/builder_test.go`, `internal/pages/market_state_page/ranges_test.go`, `cmd/ot/market_state_site.go`, `cmd/ot/market_state_site_test.go`
Design: Decisions 4, 5, 6, 7, 8.
Acceptance: the page payload has six rows in the order of Decision 6 and validates against entry 1; the Nasdaq row's figure, window line, state, two cut lines, notice and Chinese hint are those of Decision 7, the cuts being the 20th and 5th percentile of the NDX history the fake compute returns; with no index-id reader, no bar reader, no NDX instrument or two of them the card is the named gap (`no_reader`, `no_data`, `missing_input`) and the other five rows are unchanged; the page tests use a fake `marketstateapi.Func`, not `marketstate.Compute`; `marketStateConfig` sets `IndexIDs` from the new opener and `TestMarketStateConfigIndexIDs` (new) finds it set when the pages line is set and nil when it is not.

### 5 nasdaq-drawdown-5-web

Module: web. Size: S.
Order: after 1. Depends on: nasdaq-drawdown-1-schema.
Plan: P2.
Files: `web/test/pages/market_state/MarketRows.test.tsx`
Design: Decision 9.
Acceptance: `按 frontend-stack：用 现有 MarketRows 行组件（shadcn 卡片与 chart，按 id 渲染）做 第六张纳斯达克回撤卡，只加测试`; a payload with six rows renders a card with `data-market-row="nasdaq_drawdown"` right after `sp500_drawdown`, showing its figure text, state, cuts, notice and hint as given; the test finds no arithmetic on the values.

## Checked by CI

- `ci-go` for entries 1, 2, 3 and 4 (`go test`, the contract tests, the generation check) and `ci-web` for entries 1 and 5 (`tsc`, `eslint`, `vitest`, build).

## After merge (checked by the dispatcher, not by review)

- The window renders `/market-state` and shows the Nasdaq-100 card right after the S&P 500 one, with a screenshot (owner rule of 2026-11-14); the figure is read against the newest stored NDX close and its 60-session high, and the two cut values against the stored NDX history.

## Left out

- The Nasdaq Composite and QQQ: not stored; a further row of the same shape if the owner asks.
- A new FRED series or any change of collection: the stored NDX closes suffice.
- A Nasdaq chart on the card: the S&P drawdown row has the same history chart; this card has what that row has, nothing more.
- A shared drawdown for both indices, or a change of the S&P row, its series or its cuts.
- A new page version v3: not needed, nothing a consumer holds is rejected (Decision 6); if the reviewer reads the protocol rule as requiring v3, entry 1 becomes the bump and entries 4 and 5 follow it, with no other change.

## Adversarial

1. Gap vs goal: the owner sees the card next to the S&P one with the same wording and Chinese hints; the series is stored, the page reads it by a new index-id reader (the stock reader cannot find NDX), and the opener is part of entry 4, so the card is not a permanent `no_reader` on the live site.
2. Wrong source or window: the cuts are of the NDX closes in the same 3-year window as the S&P cuts, up to 800 points; stored NDX starts 2020-11-17, so the 250-point floor of `ranges.go` is met.
3. Simpler way: the same `drawdownAt` and `line()` are reused; a FRED series or a new collection would be larger for the same index (Decision 1).
4. Lock-in and contract: the schema edit is additive and keeps every stored v2 payload valid; the cost of the opposite reading is stated in Left out.
5. "No web change" false: checked, `MarketRows.tsx` renders `block.rows` by `id` with generic text, chart and hint; the one test is entry 5.
6. Modules refusal: every Files path is of its entry's module; the pages entry names `cmd/ot` as a judged fold-in, as `docs/design/market-state-chart-ma-ivrv.md` entry 4 did; entries 3, 4 and 5 touch no common file and run in parallel after 1 and 2.
