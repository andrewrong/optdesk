# spot from bars: every held name on /positions shows a price and a day result, read from the daily bars

Author: the optdesk designer
Owner approved: (pending)
Date: 2026-11-17

Source: advisor decision 2026-11-17 (plan item P1, "bars cover every followed name, JP included; the pages only read them"). Cites checked at origin/main 4d3ea026. Revised after the adversarial read (see `## Adversarial`).

## 你会看到什么

- /positions：每只持仓股票（包括日股）都有 "Last price" 和 "Day result"。以前 "Day result" 整列是空缺（gap），现在股票行显示：数量 × （现价 − 前一根日线的收盘价）。
- 价格来源：账户快照里已有的标记价（mark）照旧优先；没有标记价的股票改用日线价（obs.price_bar，由 bars_refresh 写入）。用了日线价的股票，Value、Unrealised 和 Share 以及合计也随之有数（以前这些是 gap）。
- 页面 Notes 只加一行，写明这些股票用的是 "last stored close"（最近一次存下的收盘价）以及各自的日期；不声称那是完整交易日。
- 前收盘价 = 同一只股票、所示价格那一天之前的那根日线的收盘价。期权行没有日线，"Day result" 仍是 gap，并说明原因。
- 没有日线的股票、日线币种和持仓币种不一致的股票：仍是 gap，不猜。
- 只在关注名单里、没有持仓的名字不在 /positions 上；它们的价格在 Companies 页，那是另一张单（见 Left out）。

## Goal

Every name the owner holds shows a price and a day result on `/positions`, JP names included, using the daily bars `obs.price_bar` (written by `bars_refresh`) for the price where the account has no mark and for the previous close. Names that are watched and not held are not rows of `/positions` (it lists the positions of the newest capture only); their spot on the companies pages is a separate issue. No new job, table or collector: this is a reader use only.

## Today

- Bars exist for every followed name: `internal/jobs/barsrefresh/job.go:73-115` builds one key per watch entry (market and exchange per row) and `cmd/ot/followfilter.go:15-47` (`followedBy`) gives the group `bars` to every active row and to paused or retired rows that are held; past days are repairable (`run.go:16-19`); `internal/pages/home_companies/moves.go:28-45` already builds the day change from `PriceBars`.
- `internal/pages/positions_page/builder.go:245` and `strings.go:22` put one gap line on the page ("Day result needs the previous trading day price") and `rows.go:347` gives every row `Day: dayGap`. `builder.go:150-163` (`prices`) feeds `HoldingIn.Mark`; `Holdings` (`internal/compute/blockapi/api.go`, `HoldingIn`/`HoldingLine`) has no previous price and no day figure, and the page computes nothing, so the figure must come from compute. A row with an empty `Mark` has the gap `missing_quote` in Value, Unrealised and Share, and a total that left a position out carries a gap.
- The reader of stocks by name exists: `StockIDs(ctx, names)` (`internal/store/instrument/stock_ids.go`, at most 500 names per call, two rows for a shared name), opened by `siteOpeners[siteMarketStateIDsKey]` (`cmd/ot/market_state_site.go:26-31`) and read with `siteReaderOf[market_state_page.StockIDReader]`. No reader finds an instrument by its `ibkr_conid` identifier (`Identifier` reads the other way, id to value), so the conid route is not available without a store change. The bars job names an instrument by its symbol (`specOf`: `Name: c.Symbol`).
- The positions_page v3 schema allows `day` as a money cell or a gap and `notes` as at most 20 strings of at most 200 characters (`contracts/schemas/page.positions_page.v3.json`).
- Context for the separate issue on the companies pages: `spotOf` (`cmd/ot/companies_inputs.go`) reads `quote_snapshot`, SPY only, and `symbolIDs` finds an instrument only through option_eod rows.

## Decisions

1. Reader use only. No job, migration, schema or `contracts/` file changes; the day of the prices goes into one note.
2. Price. `/positions` keeps the stored mark as the price where one exists (`builder.go:150-163`). A stock with no mark takes the close of its newest `trades` bar up to the capture day (look-back 14 days, as `home_companies` `barLookbackDays`). One `PriceBars` call per stock row gives both the price and the previous close.
3. Previous close = the close of the bar before the shown price's day, for the same instrument: for a bar price, the bar before that bar; for a mark, the newest bar before the capture day. A name with fewer than two usable bars, or whose bar currency differs from the position currency, has no previous close and the day result is the gap `no_data`. Assumption, named: a mark is taken at or after the close of the capture day; if a capture was taken before the close, mark and previous close can be equal and the day result is then a true zero of that moment, not a gap (Checked by owner).
4. The day result is computed in compute: `HoldingIn` gains `Prev` (previous close per unit, empty when none) and `HoldingLine` gains `Day` (stock rows: quantity x multiplier x (Mark - Prev), converted to USD like `Unrealised` for a converted row; gap `missing_quote` without a Mark, `no_data` without a Prev; option rows: always the gap `no_data`). No total of the day result.
5. Value, Unrealised, Share and both totals now fill in for a row priced from a bar, because the bar close feeds `Mark`; that is intended (a held name with a bar is no longer a gap), and the page says so in its one note.
6. Resolving a position to an instrument: one batched `StockIDs` call with every stock symbol of the account (at most 500 names, else the extra rows are gaps); a symbol returned once is used, none or twice is a gap, never a pick. Conid is not used (no reader, Today).
7. Notes. A fallback adds at most one line to `notes` for the whole account, naming up to eight symbols with their bar day and a count for the rest, at most 200 characters ("Last price of AAA, BBB is the last stored close (2026-11-13, 2026-11-17); the close of a day still open may change"), so `notes` stays within its 20 lines; the line says "last stored close" and does not claim a finished session.
8. JP session day. The bar day is the exchange date: `pyside/src/optdesk/ibkr/history.py:24-25` asks `BAR_SIZE = "1 day"` with `FORMAT_DATE = 1` ("exchange date, as IBKR gives it"), so a bar of a TSE name carries its Tokyo session day; the pull's end date `YYYYMMDD-23:59:59` (UTC) cannot cut a Tokyo day. No fixture or test under `internal/jobs/barsrefresh/` carries a JP stock bar (`job.toml` holds only the JPX index with exchange `OSE.JPN`), so this is read from code and not yet seen on a stored row. The design relies on it only so: the previous close is the previous bar of the same instrument (Decision 3), so a JP name is compared with its own Tokyo days and a JP holiday just gives an older price day. A read of one JP `obs.price_bar` row is Checked by owner.

## Modules

- compute (`internal/compute/**`): issue 1, edits the interface file `internal/compute/blockapi/api.go`.
- pages (`internal/pages/**`): issue 2.
- ot (`cmd/**`): issue 3.
(compute, pages, ot) are three issues of one module each. No row of `docs/design/modules.md` changes; no new dependency between modules (pages already depends on store and compute; ot on both).

## Interface

`internal/compute/blockapi/api.go`: `HoldingIn` gains the field `Prev string`; `HoldingLine` gains the field `Day compute.FigureValue`. Additions only, no existing field changes. `positions_page.Config` (`internal/pages/positions_page/page.go`, not an interface file) gains `Bars bar.PriceBarReader` and `StockIDs` (a reader with `StockIDs(ctx, names []string) ([]store.InstrumentName, error)`, the same method as `market_state_page.StockIDReader`; a page cannot import another page, so the page declares its own); both nil keeps today's page. Nothing else.

## Issues

### 1 holding-day-figure

Module: compute. Size: S.
Order: first.
Files: `internal/compute/blockapi/api.go`, `internal/compute/blockapi/api_test.go`, `internal/compute/pagefig/holdings.go`, `internal/compute/pagefig/holdings_new.go`, `internal/compute/pagefig/holdings_test.go`
Contracts: unchanged
Design: docs/design/spot-from-bars.md#Decisions (Decision 4)
Acceptance:
1. `HoldingIn.Prev` and `HoldingLine.Day` are declared in `blockapi/api.go` with doc comments; `api_test.go` has the new test `TestHoldingDayContract` (new) with a fake on one side and a caller on the other.
2. `holdings.go` gives a stock row `Day` = quantity x multiplier x (Mark - Prev) with the rounding of money, in USD with the rate for a converted JPY row; new tests `TestHoldingDayStock` (new) and `TestHoldingDayJpyConverted` (new).
3. New test `TestHoldingDayGaps` (new): empty Mark gives `missing_quote`, empty or unreadable Prev gives `no_data`, an option row gives `no_data` even with a Prev, never a zero.
4. `Value`, `Unrealised`, `Share` and both totals are unchanged: the tests of today pass without edit; `holdings_new.go` changes only if the constructor needs it.

### 2 positions-day-from-bars

Module: pages. Size: M.
Order: after 1.
Files: `internal/pages/positions_page/page.go`, `internal/pages/positions_page/builder.go`, `internal/pages/positions_page/rows.go`, `internal/pages/positions_page/strings.go`, `internal/pages/positions_page/bars.go`, `internal/pages/positions_page/builder_test.go`, `internal/pages/positions_page/bars_test.go`, `internal/pages/positions_page/testdata/bars.golden.json`
Contracts: unchanged
Design: docs/design/spot-from-bars.md#Decisions (Decisions 2, 3, 5, 6, 7)
Acceptance:
1. `Config` gains `Bars` and `StockIDs`; `ready()` does not require them; with both nil the output equals today's, and `testdata/ok.golden.json` and `testdata/jpy_fractional.golden.json` are unchanged (`TestGolden`, `TestGoldenJPYFractional` pass without edit).
2. The new `bars.go` makes one `StockIDs` call for all stock symbols and one `PriceBars` call per resolved row; a row without a mark takes the newest bar close as price, a row with a mark keeps it; the previous close is the bar before the price day; a bar currency that differs from the position currency, one bar only, a shared symbol or no id leaves the gap; new tests `TestPriceFallsBackToBarClose` (new), `TestPreviousCloseIsTheBarBeforeThePriceDay` (new, a US name, a JP name with an older last bar, a one-bar name) and `TestSharedSymbolAndCurrencyMismatchAreGaps` (new).
3. `rowsOf` and `holdingsOf` pass the price and `Prev` to `Holdings` and show its `Day` as a money cell `amount_signed` for a stock row; option rows keep the gap; Value, Unrealised and Share of a bar-priced row now carry numbers; the page gap line names only what is still missing; new test `TestBarPricedRowFillsFigures` (new).
4. One note for the account lists the bar-priced symbols (at most eight and a count) with their bar days and the words "last stored close"; new test `TestFallbackNoteIsOneLineWithin200Characters` (new) builds 25 bar-priced names plus the skipped and statement notes and asserts `notes` has at most 20 entries, each at most 200 characters, and the page validates against the v3 schema.
5. New golden `testdata/bars.golden.json` (new) is written by the update path of the golden tests and validated by `validate`; new test `TestGoldenBars` (new) compares it.
6. A reader error is returned like `TestReaderErrorIsAnError`; `purity_test.go` passes; each new file is under 600 lines and no edited file passes 600 lines.

### 3 positions-bars-wiring

Module: ot. Size: S. This is the composition-root step of the design: it only wires readers that issue 2 adds to `positions_page.Config`, so it is ordered after issue 2 and is written against main once issue 2 has merged; no module issue waits for it.
Order: after 2.
Files: `cmd/ot/positions_site.go`, `cmd/ot/positions_site_test.go`
Contracts: unchanged
Design: docs/design/spot-from-bars.md#Decisions (Decisions 2, 6)
Acceptance:
1. `positionsPageConfig` sets `Bars` from `rd.bars` and `StockIDs` from `siteReaderOf[market_state_page.StockIDReader](rd, siteMarketStateIDsKey)`, each only when non-nil; the named path `positionsPageNamedOf` uses the same function, so it gets both when its readers open.
2. New test `TestPositionsPageGetsBarsAndStockIDs` (new): a site with a fake bar reader and a fake stock reader yields a `positions_page.Config` whose `Bars` and `StockIDs` are those readers, the same through `positionsPageNamedOf`; a site with neither yields a `Config` with both nil. The page behaviour itself is checked in issue 2.

## Checked by CI

`ci-go`: `go build ./...`, `go vet ./...`, `go test ./internal/compute/... ./internal/pages/positions_page/... ./cmd/ot/...`.

## Checked by owner

At the first deploy: one JP row of `obs.price_bar` (session day is the Tokyo date); /positions with a JP holding showing its price, the day of its last stored close and the day result; whether the account capture is taken after the close (Decision 3).

## Left out

- Intraday freshness: the price is the last stored close and the note says its day.
- A name with no bar yet, and paused or retired names that are not held: gaps.
- Option day result (no option bars) and a total day result.
- Watched names that are not held on `/positions`: not rows there.
- Spot of the companies pages: a separate issue.
- Resolving by conid: no reader by identifier exists.

## Adversarial

Answers to the window's file adversarial-spot-from-bars-2026-11-17.md (outside the repository).

1. Gap against the goal (watched names): adopted. Goal and the user text say held names on `/positions`; watched-not-held is in Left out.
2. Entry 4 still in the design: adopted. Removed from Goal, Modules, Decisions, Issues, user text and Left out; one Left-out line remains, and the `symbolIDs` and `spotOf` facts stay under Today as context only.
3. Fallback changes Value, Unrealised, Share and totals: adopted (Decision 5, issue 2 item 3).
4. Issue 2 items 1 and 4 contradict on the goldens: adopted. The two old goldens stay unchanged; a new `bars.golden.json` carries the new output (item 5).
5. Risk 2, false zero when the capture precedes the close: partly adopted. The previous close is taken relative to the price shown (Decision 3); a mark does not carry its own day, so the capture-time assumption is named and moved to Checked by owner, not coded.
6. Partial bar of an open day and the word "close": adopted. The note says "last stored close" and that an open day may change (Decision 7). What `bars_refresh` writes for today is not settled by the code read here.
7. Notes limit of 20: adopted. One summary line (Decision 7, issue 2 item 4, a 25-name test).
8. Simpler way, conid: not adopted, no reader by identifier exists (Today). One batched `StockIDs` call instead (Decision 6). One `PriceBars` call per row serves price and previous close (Decision 2).
9. Lock-in, bar currency: adopted. A currency mismatch is a gap (Decision 3, issue 2 item 2). The second `StockIDs` interface copy: not changed, a page cannot import another page.
10. Name equals symbol: kept as a named assumption of the bars job; a shared symbol is a gap.
11. Claim likely false, `ot page` keeps no bars: adopted. Removed; issue 3 covers the named path.
12. Slip risks: `holdings_new.go` added to issue 1 Files; "size ceilings" became "under 600 lines"; goldens fixed; issue 2 kept at six items (it did not need splitting).
