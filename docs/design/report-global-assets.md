# report global assets, stage 1: the US close report shows five proxy assets and the 11 sector funds against SPY

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-13
Revision: 2

Source: owner 2026-11-13 "a" (the global-assets list as written); the advisor's definitions, section B (2026-11-13T05:38Z, origin/main 22f383da). The advisor found that plan v2.5 holds no proxy or sector list; the list used is the approved board v2.2 (option-trader `docs/design/v2/design-set-shadcn.html` lines 219-222, `src/option_trader/sectors/params/etfs_v1.toml`). The owner is told that.

Goal: the `global_assets` block of the US close report shows Stocks, Bonds, Gold, Commodities and Oil (SPY, TLT, GLD, DBC, USO) with the 1-session and 5-session change, a table of the 11 SPDR sector funds with their excess over SPY, and one sentence naming the leaders and the laggard. Figures come from stored daily bars, are computed when the page is built, and nothing new is stored. The definitions of the advisor are not changed here.

Existing design: none owns this. `report-page-producer.md` built the block as a `not_built` gap; `market-bars.md` stores the bars of four indexes and the watch list; `market-rv20.md` reads index bars through a name-to-id reader.

## Today

All paths at origin/main.

- `internal/pages/report_page/builder.go:28-35` `Config` has Actions, AccountMoves, MarketWatch and no global-assets func; `:112` writes `global_assets` as a `not_built` gap for every market and slot. The schema block is `GlobalAssetsBlock` of `contracts/schemas/page.report_page.v2.json` (proxies: list of {label, symbol, day, five_day} or Gap; sector: {heading, columns, rows of {sector, etf, d1, d5, d20}} or Gap; optional `rotation` {title, detail}; cell unit `percentage_points` exists; Gap reasons include `no_data`, `no_reader`, `missing_input`, `stale_input`).
- `cmd/ot/report.go:186-191` wires the three block funcs from `report_blocks.Config` (`reportBlocksConfig`, `:160`); `reportBlockReaders` opens only the series and account stores. The page is built on request (`buildReport`, `:197`), with day = today in `America/New_York` (`cmd/ot/day.go:27`); it is not a job and has no schedule (`report-page-producer.md` item 3).
- `internal/compute/pagefig/report.go:133-169` `moves` and `change`: the percent change of `closes[0]` over `closes[back]`, half-even 2 places; its stale rule is 3 weekdays behind the newest series of the request. `blockapi.PageFigures` (`internal/compute/blockapi/api.go:101-109`) is implemented by `pagefig.figures` and by fakes in test files of five packages.
- `internal/pages/home_companies/moves.go:30-43` is the precedent for reading stored bars into `blockapi.CloseIn` (`PriceBars(id, "trades", from, day)`, newest first).
- `store/bar.PriceBarReader.PriceBars` takes an instrument id (`internal/store/bar/api.go:26-32`). The only name-to-id readers are `instrument.IndexIDs` (SQL fixed to `instrument_type = 'index'`, `internal/store/instrument/index_ids.go:16`) and `symbolIDs` (`cmd/ot/companies_inputs.go:51`, through option_eod rows, so only underlyings with options).
- `internal/jobs/barsrefresh/job.toml` holds four `[[index]]` tables; `manifest.go:20-27` `Index` has `sec_type` but `job.go:107-111` builds every `[[index]]` as an instrument of type `index` under the key `index:<symbol>`; watch-list entries are type `stock` under their issuer key or `sym:<symbol>` (`job.go:96-104`). So a `stock` entry is not accepted by the manifest today: an ETF listed as `[[index]]` would be created as an index. The file is under `internal/jobs/**` (row jobs of `docs/design/modules.md`), not an owner-only path; `docs/design/**` and the register are owner-only.
- Schedule of bars_refresh: calendar XNYS, anchor `session_close`, offset 10m, run_bound 10m (`job.toml:41-45`); compute_figures runs at +20m, fx_refresh +15m. `backfill_names_per_run = 10` caps first pulls (400 days) per run; a daily pull asks 5 days (`run.go:98-108`). bars_refresh stores only basis `trades`.
- `docs/conventions/register.toml` is in the docs row (`docs/conventions/**`), not owner-only; #1664 and #1791 edited it. A figure entry needs its 15 keys and implemented_by names of the form `<pkg path>.<func>` that resolve in the package (`internal/compute/rules/rules_test.go` `splitName`, `ruleHash`); a figure with no Go convention keeps `source_hash = "pending-recompute"` and has no row in `rules.json` (`TestNoPendingForGoIds`). `report.market_move` v1 (`register.toml:2064`) says its gap_text: more than three weekdays behind the newest series of the request is `stale_input`.
- `cmd/ot/wire.go` is 793 lines of 800 and `cmd/ot/site.go` 639; `cmd/ot/depsext.go` lets a command file add its own deps without touching `wire.go`.

## Decisions

The numbering of the advisor's section B is kept in brackets.

1. [B0, B1] List and stages. Stage 1 is the US close report: proxies SPY (Stocks), TLT (Bonds), GLD (Gold), DBC (Commodities), USO (Oil); sectors XLB Materials, XLC Communication Services, XLE Energy, XLF Financials, XLI Industrials, XLK Technology, XLP Consumer Staples, XLRE Real Estate, XLU Utilities, XLV Health Care, XLY Consumer Discretionary, each against SPY. 16 symbols; SPY is both a proxy and the benchmark. The Japan report (4 ETFs, USDJPY, 17 sectors against 1306) is stage 2, a later design (Left out). The block is built only for market US and slot close; every other market or slot keeps today's `not_built` block.
2. [B2] Instruments: STK, exchange SMART, currency USD, type `stock`.
3. [B3] Prices are the daily `trades` close of `obs.price_bar`, unadjusted. Named limit: on an ex-dividend day a change is understated by the distribution (TLT monthly, about 0.3%; the SPDR funds quarterly), and a sector excess is off by the difference of the two distributions. Adjusted closes are left out.
4. [B4] Proxy cells use the formula of `report.market_move` v1 (the code `change`), but with the as-of rule of Decision 6, so they get their own figure id `report.proxy_move` v1 (Decision 14); `report.market_move` is not edited. The formula: day = c0/c1 - 1 and five_day = c0/c5 - 1, times 100, percent, half-even 2 places, c1 and c5 the first and fifth stored closes before c0 of the same symbol. The tone is the sign of the rounded value.
5. [B5] Sector cells d1, d5, d20 = change_n(ETF) - change_n(SPY), n = 1, 5, 20 sessions, each change unrounded (exact rational), the difference rounded half-even to 2 places, unit `percentage_points`, signed text. The ETF's and SPY's c0 must be the same day and their c_n the same day, else `missing_input`. d20 needs 21 stored closes.
6. [B6, revised by the advisor's review] As-of. The reference close c0 is the newest stored SPY close on or before the page day (the reader reads up to the page day, so a past `--day` gives that day). Its day is the day of the block (`data_times`, Decision 8). `stale_input` only when c0 is more than 3 weekdays behind the page day (`weekdaysBehind` and `staleWeekdays` of `pagefig/report.go`), so Friday's close is shown on Saturday, Sunday and Monday morning, and a holiday is no gap. Every other symbol, proxy or sector, must have the same c0 as SPY, else `missing_input` for that row; for a sector row also c_n of the ETF and of SPY are the same days, else `missing_input`. Order of the checks for one cell: SPY has no stored close is `no_data` for every cell; SPY c0 stale is `stale_input` for every cell; the symbol has no close is `no_data`; its c0 differs from SPY's is `missing_input`; fewer than n+1 closes is `no_data`; different days of c_n is `missing_input`; an unreadable or non-positive close is `missing_input`. A symbol the reader cannot find is `no_data`; two instruments of one name is `missing_input`. No bar reader, id reader, figures or asset list wired is `no_reader`. Texts stay under 200 characters.
7. [B6, order] Bars before the report. bars_refresh stores the close of D at close + 10m (XNYS) and the report is built on request, not on a schedule, so before that a request shows the figures of the session before (c0 is at most 3 weekdays behind), named by `data_times`; the request after it shows D. Nothing collects or retries on the page: it computes only from what is stored, and the owner's rule "fallback, not cascade" is about chained jobs.
8. [B7] Layout: `proxies` in the order SPY, TLT, GLD, DBC, USO with the labels above; `sector` heading `Sector excess over SPY`, columns `Sector`, `ETF`, `1d`, `5d`, `20d`, 11 rows sorted by d5 descending (ties in list order), rows whose d5 is a gap last in list order. The sorting is done by compute; the pages sort nothing. The `sector` and `etf` cells of a row are `Cell`s `{text: label or symbol, tone: "none"}` with no value and no unit (the schema makes them cells, not strings). Block texts, constants of `report_blocks/strings.go`: key `global_assets`, title `Global assets`, sub `Proxy assets and sector rotation.`, say `Stocks, bonds, gold, commodities and oil, and the sector funds against SPY, from stored closes.` (when the block is the whole-block gap: `Global assets data cannot be read now.`), `data_times` `Closes of ` followed by the day of SPY's c0 (`no data yet` when SPY has none), `chart_aria` `Day and five-day change of five proxy assets, and sector excess over SPY in percentage points`.
9. [B8] Rotation: `rotation` has title `Sector leaders` and detail `5-day leaders: A, B, C; laggard: Z -1.80 pt; 20-day leader: E` (A, B, C the first three rows by d5, Z the last row with a d5 and its d5, E the row with the greatest d20, a tie by list order; the names are the sector labels). Omitted when fewer than 6 rows have a d5 or no row has a d20. No word about defensive sectors or rotation is written (needs a classification and a threshold nobody gave: Left out).
10. [B9] Where. Computed at page build: compute `pagefig` (new `Global`), read by the block builder in `report_blocks` from `PriceBarReader`; nothing is stored. A new interface `blockapi.GlobalFigures` is separate from `PageFigures`, so no fake of `PageFigures` changes.
11. [B9] One list. `internal/jobs/barsrefresh/job.toml` gets 16 `[[stock]]` tables (symbol, label, group `proxy` or `sector`, sec_type `STK`, exchange `SMART`, currency `USD`, con_id 0) and `[global_assets] benchmark = "SPY"`; the manifest loader gives them as `Manifest.Stocks` and `Manifest.Benchmark`. Keys `stock:<symbol>`, planned after the index keys, instrument type `stock`. `cmd/ot` (which may import both) reads `barsrefresh.DefaultManifest()` and hands the list to `report_blocks.Config.Assets`, so the collector and the page cannot differ. If the owner also holds SPY in the watch list, the keys `sym:SPY` and `stock:SPY` pull the same instrument (same conid); the second insert is a no-op.
12. Name to id. A new reader `instrument.StockIDs` (type `stock`, by name, ordered by id, at most 500 names; the same shape and the same view `rd.instrument_r1` as `IndexIDs`). Its interface `report_blocks.StockIDReader` is declared in the consuming package, as `computefigures.IndexIDReader` is. `cmd/ot` binds and opens it with the pages line, through `extDeps`, with no field in `deps`.
13. Wiring. The wiring is in `cmd/ot/report.go` (the readers and the Config) and a new file `cmd/ot/report_global.go`; `cmd/ot/wire.go`, `cmd/ot/site.go` and `internal/compute/api.go` are not edited.
14. Register. `docs/conventions/register.toml` (docs row, not owner-only) gets two entries in its own issue (entry 6), after entry 1 so that the functions exist: `report.proxy_move` v1 (formula of Decision 4, inputs stored daily `trades` bars of a listed proxy, gap_text of Decision 6, implemented_by `internal/compute/pagefig.proxyMove` and `internal/compute/pagefig.change`) and `report.sector_excess` v1 (formula of Decision 5 with the sorting and the rotation facts of Decisions 8 and 9, unit percentage points, excludes adjusted closes and any judgement of rotation, implemented_by `internal/compute/pagefig.sectorExcess`, `internal/compute/pagefig.ratPercent` and `internal/compute/pagefig.rotationOf`). Both are page figures: `source_hash = "pending-recompute"`, no row in `rules.json`. `report.market_move` v1 stays as it is (it still serves the S&P 500 row).
15. Contracts unchanged: `page.report_page.v2.json` already has the block, the `percentage_points` unit and the optional `rotation`.

## Modules

No row of `docs/design/modules.md` changes. The entries touch: compute (`internal/compute/**`, `docs/contracts/compute.md`), docs (`docs/conventions/**`), jobs (`internal/jobs/**`, `docs/contracts/jobs/**`), store (`internal/store/**`), pages (`internal/pages/**`, `docs/contracts/pages.md`) and ot (`cmd/**`). `pages` imports `store/bar` (as `option_data_page` does), `store` and `compute`; `cmd/ot` already imports `internal/jobs/barsrefresh` and `internal/store/instrument`. No new import edge.

## Interface

Module compute, `internal/compute/blockapi/api.go`; existing declarations stay.

```go
// GlobalIn is the input of the global assets block. Day is the page day. Proxies, Sectors and Benchmark are closes
// newest first with their days (CloseIn), in list order; Name is the label.
type GlobalIn struct {
	Day       contracts.Day
	Proxies   []CloseIn
	Benchmark CloseIn
	Sectors   []CloseIn
}

// SectorLine is the excess over the benchmark in percentage points over 1, 5 and 20 sessions.
type SectorLine struct {
	Name, Symbol string
	D1, D5, D20  compute.FigureValue
}

// RotationLine is the facts of the rotation sentence: Leaders5 are the names of the first three rows by d5, Laggard
// and LaggardD5 the last row with a d5 and its decimal text, Leader20 the name with the greatest d20.
type RotationLine struct {
	Leaders5           []string
	Laggard, LaggardD5 string
	Leader20           string
}

// GlobalResult: DataDay is the day of SPY's c0 (empty when SPY has no close); Proxies in input order; Sectors sorted
// by d5 descending, gap rows last; Rotation nil when omitted.
type GlobalResult struct {
	DataDay  contracts.Day
	Proxies  []MoveLine
	Sectors  []SectorLine
	Rotation *RotationLine
}

// GlobalFigures evaluates the global assets block, no I/O, no clock.
type GlobalFigures interface {
	Global(GlobalIn) GlobalResult
}

// NewGlobalFiguresFunc constructs the implementation (internal/compute/pagefig.NewGlobal).
type NewGlobalFiguresFunc func() GlobalFigures
```

Module pages, `internal/pages/report_blocks/common.go` (`Config` gains `Bars bar.PriceBarReader`, `StockIDs StockIDReader`, `Assets []Asset`, `Global blockapi.NewGlobalFiguresFunc`) and `global_assets.go` (declares `Asset{Symbol, Label, Group string}`, `StockIDReader` with `StockIDs(ctx, names []string) ([]store.InstrumentName, error)`, and `GlobalAssets(c Config) BlockFunc`); `internal/pages/report_page/builder.go` (`Config` gains `GlobalAssets BlockFunc`). Only fields and a function are added; existing signatures stay.

`Contracts: unchanged`.

## Issues

Build order: every entry compiles and its tests pass against main. BASE is `git merge-base origin/main HEAD`. Entries 1, 2 and 3 are independent of each other. Where a result needs a test run, it is in `## Checked by CI`, not in an acceptance list.

### 1 report-global-assets-api (interface)

Module: compute. Size: M.
Order: first.
Files: `internal/compute/blockapi/api.go`, `internal/compute/blockapi/api_test.go`, `docs/contracts/compute.md`, `internal/compute/pagefig/pagefig.go`, `internal/compute/pagefig/global.go`, `internal/compute/pagefig/global_test.go`
Design: docs/design/report-global-assets.md#Issues (Decisions 3 to 6, 8, 9, 10; Interface)
Acceptance:
1. `api.go` declares `GlobalIn`, `SectorLine`, `RotationLine`, `GlobalResult`, `GlobalFigures` and `NewGlobalFiguresFunc` with the fields and the one method of the Interface section, no function body; the interface `PageFigures` has the same six methods as before, so `git diff BASE -- internal/pages internal/compute/pagefig/report.go internal/compute/pagefig/household.go` is empty.
2. `api_test.go` fills every new field through a fake `GlobalFigures` and reads each back (from both sides: the fake is called through `NewGlobalFiguresFunc`, and `pagefig.NewGlobal` is checked to be assignable to it), with `t.Parallel()`.
3. `pagefig.go` has `NewGlobal() blockapi.GlobalFigures` and `var _ blockapi.GlobalFigures` for the type; `global.go` is under 400 lines, uses `math/big` and no float, calls `change(` for the proxy cells and does not call `moves(`, and defines the functions `proxyMove`, `sectorExcess`, `ratPercent` and `rotationOf` (`git grep -n "^func \(proxyMove\|sectorExcess\|ratPercent\|rotationOf\)("` prints four lines). `docs/contracts/compute.md` lists `GlobalFigures`, `NewGlobalFiguresFunc` and the new types under Page figures, with the tests of item 4.
4. `global_test.go` names `TestProxyFormulaIsMarketMove` (closes 110, 100 ... gives the same D1 and D5 text as `New().Moves` for a series of the same closes), `TestProxyOneDayBeforeDayGivesFigures` (c0 one day before Day gives figures and `DataDay` is c0), `TestWeekendDayShowsFridayClose` (Day a Saturday, c0 Friday, figures), `TestProxyStaleAfterThreeWeekdays` (c0 four weekdays before Day gives `stale_input` in every cell), `TestProxyDifferentC0IsMissingInput` (a proxy whose c0 is not SPY's gives `missing_input` in D1 and D5), `TestSectorExcessUsesUnroundedChanges` (ETF +1.004 percent and SPY +0.004 percent gives 1.00, and a case where rounding each change first gives a different last digit), `TestSectorNeedsSameDays` (c5 days differ gives `missing_input`), `TestSectorStaleBenchmark` (SPY c0 four weekdays before Day gives `stale_input` in every row), `TestSectorC0DiffersFromSpy` (`missing_input`), `TestNoSpyIsNoDataEverywhere`, `TestSectorTwentyNeeds21Closes` (20 closes gives D20 `no_data` and D1, D5 numeric), `TestEmptySeriesIsNoData`, `TestNonPositiveCloseIsMissingInput`, `TestSectorsSortedByD5GapsLast` (ties keep input order), `TestRotationFacts` (11 rows give three leaders, the laggard with its text and the d20 leader), `TestRotationOmittedBelowSixRows` and `TestRotationOmittedWithoutD20`.
5. `git diff --name-only BASE` lists only paths of the Files line; the tests of `pagefig` that exist (`report_test.go`, `household_test.go`, `holdings_test.go`) are not edited.

### 2 report-global-assets-bars

Module: jobs. Size: M.
Order: independent (no entry before it).
Files: `internal/jobs/barsrefresh/job.toml`, `internal/jobs/barsrefresh/manifest.go`, `internal/jobs/barsrefresh/manifest_test.go`, `internal/jobs/barsrefresh/job.go`, `internal/jobs/barsrefresh/plan_test.go`, `internal/jobs/barsrefresh/run_test.go`, `docs/contracts/jobs/bars_refresh.md`
Design: docs/design/report-global-assets.md#Issues (Decisions 2, 11)
`types_test.go` is not edited: it does not use `Manifest`, `Indexes` or `DefaultManifest` (`git grep -c "Manifest\|Indexes" -- internal/jobs/barsrefresh/types_test.go` prints 0). No test of `cmd/ot` uses `DefaultManifest`, `barsrefresh.New` or the key list (`git grep -n "DefaultManifest\|barsrefresh\." -- 'cmd/ot/*_test.go'` prints nothing); `cmd/ot/held_underlying_rows.go:74` and `cmd/ot/underlyings_site.go:156` read only `m.Indexes`, which is unchanged.
Acceptance:
1. `job.toml` holds 16 `[[stock]]` tables, each with `sec_type = "STK"`, `exchange = "SMART"`, `currency = "USD"`, `con_id = 0`, and groups `proxy` (SPY, TLT, GLD, DBC, USO) and `sector` (XLB, XLC, XLE, XLF, XLI, XLK, XLP, XLRE, XLU, XLV, XLY), the labels of Decision 1, and `[global_assets]` with `benchmark = "SPY"`; the four `[[index]]` tables, the schedule and every other key are unchanged (`git diff BASE -- internal/jobs/barsrefresh/job.toml` has no removed line).
2. `Manifest` has `Stocks []Stock` (Symbol, Label, Group, SecType, Exchange, Currency, ConID) and `Benchmark string`; the type `Index` and its conversion are unchanged. `LoadManifest` refuses, with an error that names the key and never the value: a `stock` without `symbol`, `label`, `exchange` or `currency`, a `group` that is not `proxy` or `sector`, a repeated symbol, and a `benchmark` that is not a listed stock.
3. `New` adds one key `stock:<symbol>` per stock, after the index keys, whose instrument spec has type `stock` (`typeStock`); a watch entry with the symbol SPY and the key `sym:SPY` plus `stock:SPY` build without a duplicate-key error.
4. `plan_test.go` expects the keys of its two cases followed by the 16 `stock:` keys in the order of the file; `run_test.go` has a test that runs `stock:XLE` through the fake history with a `STK` contract and checks that the instrument is created with type `stock` and bars are stored under `trades`, and another that a `stock:` key whose contract cannot be resolved fails only its own key (the others still store); `manifest_test.go` names `TestManifestLoadsStocks`, `TestManifestRefusesBadStock` and `TestManifestRefusesUnknownBenchmark`, each with `t.Parallel()`.
5. `docs/contracts/jobs/bars_refresh.md` names the `[[stock]]` keys, the `benchmark` key, the refusals, and says the list is also read by the report block.
6. `git diff --name-only BASE` lists only paths of the Files line; `git diff BASE -- internal/jobs/barsrefresh/run.go internal/jobs/barsrefresh/types.go` is empty; every file stays under its size: `job.go` and `manifest.go` under 300 lines, none new.

### 3 report-global-assets-ids

Module: store. Size: S.
Order: independent (no entry before it).
Files: `internal/store/instrument/stock_ids.go`, `internal/store/instrument/stock_ids_test.go`, `internal/store/instrument/stock_ids_pg_test.go`
Design: docs/design/report-global-assets.md#Issues (Decision 12)
Acceptance:
1. `stock_ids.go` has `OpenStockIDs(ctx, dsn) (*StockIDs, error)`, `Close` and `StockIDs(ctx, names) ([]store.InstrumentName, error)`; its SQL reads `rd.instrument_r1` with `instrument_type = 'stock' AND name = ANY($1)`, ordered by instrument id; an empty `names` makes no database call; more than 500 names or an empty name is a Permanent error; it is under 100 lines; `index_ids.go` is not edited.
2. `stock_ids_test.go` (over the same database seam `index_ids_test.go` uses, `t.Parallel()`) names `TestStockIDsOrdersById`, `TestStockIDsEmptyMakesNoCall`, `TestStockIDsRefusesTooManyAndEmptyName` and `TestStockIDsTwoRowsOfOneName` (both rows come back).
3. `stock_ids_pg_test.go` carries the build tag `pg`, inserts a stock and an index of the same name and reads back only the stock; it runs only in GitHub CI.
4. `git diff --name-only BASE` lists only paths of the Files line.

### 4 report-global-assets-pages

Module: pages. Size: M.
Order: after 1.
Files: `docs/contracts/pages.md`, `internal/pages/report_blocks/common.go`, `internal/pages/report_blocks/global_assets.go`, `internal/pages/report_blocks/strings.go`, `internal/pages/report_blocks/global_assets_test.go`, `internal/pages/report_page/builder.go`, `internal/pages/report_page/blocks.go`, `internal/pages/report_page/builder_test.go`
Design: docs/design/report-global-assets.md#Interface and Decisions 1, 6 to 10
Acceptance:
1. `GlobalAssets(c)` returns the block of the schema: key `global_assets`, the texts of Decision 8 (title, sub, say, `data_times` from `GlobalResult.DataDay`, `chart_aria`) as constants of `strings.go`, `proxies` in the order of the assets of group `proxy`, `sector` with the heading and the five columns of Decision 8 and one row per asset of group `sector` in the order compute gives (`sector` and `etf` cells with tone `none`, no value, no unit), `rotation` only when `GlobalResult.Rotation` is set, title `Sector leaders`, detail built as in Decision 9. `docs/contracts/pages.md` no longer says `global_assets` is always `not_built`: it states the `GlobalAssets` func, the Config fields, and that only the US close report builds it.
2. A nil `Bars`, `StockIDs`, `Global` or empty `Assets` gives state `gap`, reason `no_reader` and no read; a reader error is an error of the call, never a gap; an id missing from the reader is `no_data` in that symbol's cells, two ids are `missing_input`; the bars of each symbol are read once (`PriceBars(id, "trades", day minus 50 calendar days, day)`), turned into newest-first `CloseIn` without arithmetic; no file of the package imports `math/big` or `sort`.
3. Cell text: a proxy cell is the signed value and `%` with unit `percent`; a sector cell is the signed value and ` pt` with unit `percentage_points`; the tone is the sign; a gap cell is the `Gap` of its reason with a text under 200 characters. The block state is `ok` with no gap cell, `partial` with some, `gap` when every proxy and every sector cell is a gap; `gaps` lists each reason once, at most 20.
4. `report_page.Config` has `GlobalAssets BlockFunc`; `Build` calls it only for market US and slot close and embeds its bytes; a nil func, market JP and slot open keep today's `not_built` block byte for byte; an error of the func is an error of the call, as for the other three.
5. `global_assets_test.go` (package `report_blocks_test`, a new fake of `GlobalFigures`, a fake bar reader, `t.Parallel()`) names `TestGlobalAssetsFullBlockValidatesAgainstSchema`, `TestGlobalAssetsNamesItsDay`, `TestGlobalAssetsNoReaderIsGap`, `TestGlobalAssetsReaderErrorIsError`, `TestGlobalAssetsUnknownSymbolIsNoData`, `TestGlobalAssetsAmbiguousNameIsMissingInput`, `TestGlobalAssetsReadsEachSymbolOnce`, `TestGlobalAssetsRotationOmitted`, `TestGlobalAssetsTextsAreEnglishAndShort` (every string of the block is ASCII and 1 to 200 characters). `builder_test.go` names `TestGlobalAssetsFuncReplacesBlockForUSClose` and `TestGlobalAssetsNotCalledForJPOrOpen`; the existing expectations of `not_built` for `global_assets` still pass without edit.
6. Within `report_blocks`, `git grep -n "math/big\|\"sort\"" -- internal/pages/report_blocks` prints nothing for the non-test files. `blocks_test.go`, `english_test.go` and `internal/pages/report_page/testdata/*` are not edited (their literals are keyed); `git diff --name-only BASE` lists only paths of the Files line; `global_assets.go` is under 350 lines.

### 5 report-global-assets-wire

Module: ot. Size: M.
Order: after 1, 2, 3, 4.
Files: `cmd/ot/report.go`, `cmd/ot/report_global.go`, `cmd/ot/report_global_test.go`, `cmd/ot/report_test.go`
Design: docs/design/report-global-assets.md#Issues (Decisions 7, 11, 12, 13)
Acceptance:
1. `report_global.go` registers its deps in `extDeps` under its own key (`reportGlobalDeps{openStockIDs}`, bound to `instrument.OpenStockIDs` the way `index_ids_bind.go` binds `OpenIndexIDs`, a nil interface on error); a missing key or a nil opener leaves `StockIDs` nil (`no_reader`) and never panics.
2. `reportBlockReaders` opens the bar store with `a.deps.openBar` and the stock id reader with the pages line for market US, closes them with the others, and keeps the early return only when no opener at all is set; in the demo profile the block is built from the existing demo readers or is the `no_reader` gap, never a connection.
3. `reportBlocksConfig` sets `Bars`, `StockIDs`, `Global: pagefig.NewGlobal` and `Assets` from `barsrefresh.DefaultManifest().Stocks`; `reportPageOver` sets `report_page.Config.GlobalAssets` to `report_blocks.GlobalAssets(bc)`; the list of symbols exists once: `git grep -n '"XLRE"' -- ':!*_test.go'` prints only `internal/jobs/barsrefresh/job.toml`.
4. `report_global_test.go` names `TestReportUsCloseGlobalFigures` (every stored close of the page day: numbers), `TestReportUsCloseShowsFridayOnSaturday` (the stored closes one weekday older still give figures and the block names that day), `TestReportUsCloseStaleAfterThreeWeekdays` (`stale_input`, Build returns no error), `TestReportNoStockIDsIsNoReader` (the block is the `no_reader` gap and the other blocks are unchanged) and `TestReportJpAndOpenKeepNotBuilt`; `report_test.go` is edited only where a test counts the opened readers or compares the whole page.
5. `git diff BASE -- cmd/ot/wire.go cmd/ot/site.go internal/compute/api.go docs/design` is empty; `git diff --name-only BASE` lists only paths of the Files line.

### 6 report-global-assets-register

Module: docs. Size: S.
Order: after 1.
Files: `docs/conventions/register.toml`
Design: docs/design/report-global-assets.md#Issues (Decision 14)
Acceptance:
1. `register.toml` has the entries `report.proxy_move` and `report.sector_excess`, each version 1, with all 15 keys of `TestRegisterEntriesComplete` (`id`, `version`, `title_zh`, `pages`, `formula`, `inputs`, `period_date`, `currency`, `excludes`, `gap_text`, `implemented_by`, `source_hash`, `agreed_on`, `agreed_in`, `history`), `pages = ["report_page.global_assets"]`, `source_hash = "pending-recompute"`, `agreed_in = "docs/design/report-global-assets.md"`.
2. The `implemented_by` names are exactly those of Decision 14, and each is a function defined in `internal/compute/pagefig` (`git grep -n "^func \(proxyMove\|change\|sectorExcess\|ratPercent\|rotationOf\)(" -- internal/compute/pagefig` prints five lines).
3. The `gap_text` of `report.proxy_move` states the rule of Decision 6 (more than three weekdays behind the page day is `stale_input`, another c0 than SPY's is `missing_input`); the `excludes` of `report.sector_excess` name adjusted closes and any judgement of rotation.
4. `git diff BASE -- docs/conventions/register.toml` removes no line and adds only the two entries; `git diff --name-only BASE` lists only `docs/conventions/register.toml`; `internal/compute/rules/rules.json` and `docs/design` are not edited.


## Checked by CI

`go test ./internal/compute/... ./internal/jobs/barsrefresh/... ./internal/store/instrument/... ./internal/pages/... ./cmd/ot/... -race -count=1` (the `pg` test of entry 3 runs only in GitHub CI), `go run ./scripts/arch --check`, `go build ./...`, `golangci-lint`, the register test of `internal/compute/rules` (entry 6: keys complete, names resolve, no pending hash for a Go id), and the contracts generation check (no change expected). Results that only a run shows: the block built by `GlobalAssets` validates against `page.report_page.v2.json` (entry 4, `TestGlobalAssetsFullBlockValidatesAgainstSchema`); the US close report built through `buildReport` holds numbers, Friday's figures on a weekend and `stale_input` after three weekdays (entry 5); `LoadManifest` refuses the listed bad stocks (entry 2). The review runs no test.

## Left out

- Stage 2, the Japan close report: 1321, 2510, 1540, 1699, USDJPY and the 17 TOPIX-17 funds 1617-1633 against 1306 (23 subjects). No Japan ETF or FX pair has a bar producer today, and IBKR daily bars of the TSE funds and of USDJPY have been verified by nobody. It is a second design, not dropped; until then the block of the Japan report stays as today.
- The words "defensive sectors are strengthening" and "a sign of rotation" of the board: they need a sector classification and a threshold nobody has given. The sentence states leaders and the laggard only.
- Adjusted closes (dividends); an exchange calendar for "5 days" (sessions of the symbol are used); the open slot of the report; history of the block (nothing is stored); a threshold or colour for rich or cheap sectors.
- A de-duplication of held names against the 16 stocks (Doubt 5), and per-market lists for stage 2 (`[global_assets] benchmark` is one scalar; stage 2 reworks the manifest shape).

## Doubts

1. Code contradicts a definition in two places. (a) B9 says the 16 symbols "enter bars_refresh the way the four indexes do". The manifest does not accept a `stock` entry: every `[[index]]` becomes an instrument of type `index` (`job.go:107-111`), and an ETF created so would be an index to every reader. This design adds a `[[stock]]` table and `Manifest.Stocks` (Decision 11). (b) B6 says the report "collects or retries". The report page is built on request and has no retry; Decision 7 settles it: the page shows the newest stored close within three weekdays and names its day. B6's "c0 must equal the report session day" was replaced by the advisor's review rule (Decision 6).
2. Unverified: that IBKR serves daily bars for DBC, USO and the SPDR funds on the owner's subscription, and the pacing of 16 more names with `run_bound` 10m. Result of the read-only live probe (2026-11-13, main account, ResolveContract then 40 days of daily `trades` bars, one name a second): all 16 names resolved to exactly one con_id and returned 40 bars each; no ambiguity, no error, no pacing error. Pacing inside `run_bound` with the existing names is still not measured.
3. STK/SMART ambiguity: `ResolveContract` (pyside `ibgateway.py:374-420`) asks `reqContractDetails` for STK, SMART, USD and raises `AmbiguousResolve` for more than one answer; the key then fails Permanent, and only that key (entry 2 has a test for it). Not verified for DBC, USO, XLC, XLRE. The fallback is the `con_id` literal of that `[[stock]]` table, which the manifest already carries.
4. First-pull starvation: `claimFirstPull` allows `backfill_names_per_run` = 10 first pulls (400 days) a day in key order, and the stock keys are planned last, after the watch entries and the indexes. New watch or held names can take the slots first, so the 16 names need two days at best and more when other names are new; until a name has 21 closes its cells read `no_data`. No cap is changed here.
5. Held names are pulled twice. `heldRows` (`cmd/ot/held_underlying_rows.go:74`) de-duplicates held names against the indexes only, so a held SPY, GLD, XLE and so on gets its own key (issuer or `sym:`) and `stock:<symbol>`. Both keys resolve the instrument by the same `ibkr_conid` identity (`run.go:78-91`, as optioneod, optionopen and dividendcapture do), so it is one instrument: the second key pulls 5 days and inserts nothing (stored days are skipped). It costs a request per name per day; it is not a wrong figure. Not de-duplicated here (Left out).
6. A decision of mine: the rotation sentence names sector labels (Energy), not ETF symbols (XLE). The sector cell text is `+1.25 pt`; the schema fixes only the unit. The table columns are five, since each row has five cells; B7 names the last three.
7. The block is built only for US close. B0 says the board lists it on the close reports only; the open report and the JP close keep `not_built`.
8. A name held by two stock instruments (two conids) is `missing_input` for that symbol, never a guess.

## Adversarial

Source: the author's adversarial notes (scratchpad), round 1, and the advisor's review r1.

1. Exact-day staleness blanks the block 16 hours a day and all weekend (also the advisor's blocking point): adopted. Decision 6 uses SPY's newest close on or before the page day, three weekdays of tolerance, equal c0 for every symbol, the day named in `data_times`; the old tests are replaced and Doubt 3 (the `--day` workaround) is gone. The Sessions reader the notes suggest was not used: the stored closes already say which day is the newest, and no reader of session days is opened in the report today.
2. Schema-required texts and the Cell shape of `sector` and `etf`: adopted (Decision 8, entry 4 item 1).
3. IBKR contract ambiguity and first-pull starvation: adopted as Doubts 3 and 4, plus a test in entry 2 that one unresolvable key fails alone. A `con_id` literal for each symbol was not added: the live probe resolved every symbol to one con_id, and the field is there when a symbol fails. The first-pull cap is not changed (the owner's file of rates and pacing; a change is another design).
4. Held names pulled twice, all 16 symbols not only SPY: adopted as Doubt 5 with the check (same `ibkr_conid` identity, one instrument). De-duplication was not adopted: it edits `cmd/ot/held_underlying_rows.go`, another module's concern, for a cost of one request a name.
5. Simpler way, drop `instrument.StockIDs` by widening `IndexIDs` or reusing `InstrumentNames`: not adopted. `IndexIDs` is fixed to type `index` and is used by compute_figures; `InstrumentNames` goes from id to name, not back. A new file of under 100 lines beside it changes neither. Merging entries 3 and 5 was not adopted: they are two modules (store, ot). The rotation sentence stays: the owner asked for it.
6. Lock-in: the scalar `benchmark` and the stage 2 rework: named in Left out. The plan hash uses the keys only (`plan.go`), so a label edit does not change it. Unadjusted closes fixed in the register: the figure ids carry a version, a change is a version bump. Stock-instrument readers: the only reader that lists stocks goes through option_eod rows, so it is unaffected.
7. Register edits: adopted as entry 6, in the docs module, after entry 1, with names that resolve (`<pkg path>.<func>`), `pending-recompute` and no `rules.json` row because they are page figures; `report.proxy_move` is its own id because its stale rule differs from the gap_text of `report.market_move`, which stays unchanged.
8. Files: `docs/contracts/compute.md` (entry 1) and `docs/contracts/pages.md` (entry 4) added. `types_test.go` and the `cmd/ot` tests were searched (`DefaultManifest`, `barsrefresh.`, `Indexes`, `index:`): none uses the changed identifiers, stated in entry 2. The PageFigures fakes are in five packages, not four; harmless as the interface is untouched (the count in Today is corrected).
9. Acceptance that needs a run (schema validation, behaviour of the built report) moved to `## Checked by CI`; "no file imports `math/big` or `sort`" is now a grep form.
