# bugsweep company rows: the company data page reads bars by instrument id, and a held fund without an issuer stays on the home card

Author: the optdesk designer
Owner approved: (pending)
Date: 2026-11-24

Source: bug sweep 2026-11-24, `optdesk-bugsweep-2026-11-24/causes.md` groups A and B (entries 1, 9 and the CIK part of 11 of `owner-page.md`). Cites checked at origin/main c5543d94. The empty-name part of group A is already fixed by #3679 and is not touched here.

## 你会看到什么

- /company-data：每家公司的"日线根数"和"价格到哪天"不再是 0 和"—"。有日线的公司（例如 AAPL）显示库里存的根数和最新一根的日期，与估值页、基础数据页一致。
- 首页"持仓公司"卡：持有的 ETF 和没有 CIK 的公司（SGOV、JEPQ、VGT、GOOGL、IBKR）也有一行，数量与持仓公司页一致（14 变成 19）。这些行的"财报日"一格写"earnings date: no issuer"，不再误写"none listed"；价格和涨跌照常从日线读。
- /companies 页不变：它仍只列有发行人（CIK）的公司，因为估值和财报需要发行人。给 11 个没有 CIK 的名字填 CIK 是你在 /underlyings 页的录入，不在本设计里。

## Goal

Two readers resolve a company to the wrong key. `company_data_page` reads bars with the IBKR contract id as if it were the instrument id, so all 54 rows read no bar. The home Companies card builds its rows from the Companies subjects, which drop every underlying without an issuer, so held funds and uncovered names vanish from the card. Each fix is the smallest one: the company data page resolves the symbol the way `positions_page` does; the home rows are built from the held underlyings, and a row with no issuer reads no earnings.

## Today

- `companyRow` (`internal/pages/company_data_page/rows.go`) sets `HasInstrument: u.ConID != 0` and calls `in.Bars.PriceBars(ctx, u.ConID, "trades", ...)`. `PriceBars` takes the `instrument_id` of `master.instrument` (`internal/store/barpg/read.go`), a generated identity, not the contract id (AAPL: con_id 265598). No bar has that id, so `barDays` is empty and Summarize adds `no_bars`. The test fake `fakeBars` (`builder_test.go`) is keyed by whatever id it is given and sets `ConID: 1`, so no test saw it.
- `positions_page` declares `StockIDReader` (`StockIDs(ctx, names []string) ([]store.InstrumentName, error)`, `bars.go`): a name returned once is one instrument, a name returned twice is shared and gives no id. `cmd/ot` holds a reader of this shape under `siteMarketStateIDsKey` and passes it to positions, pnl and market state.
- `companiesSubjects` (`cmd/ot/companies_site.go`) skips every row with `Issuer == ""`. `homeRows` (`cmd/ot/home_site.go`) takes its held symbols only from those subjects. The five held names missing on the home card are five of the 11 underlyings with an empty issuer; 14 + 5 = 19.
- `home_companies.Row.Issuer` is also the reading subject: `earningsCell` calls `EarningsDates(ctx, issuer, ...)`. An empty issuer would read the key "".
- `earnings_refresh` skips an entry with `Issuer == ""` (`internal/jobs/earningsrefresh/run.go`), so such a name never has an earnings row.

## Decisions

1. The company data page resolves a row's instrument by name: its `Symbol` through a `StockIDReader` of its own package (same method as `positions_page`), one call per Build for all rows. A symbol returned once is the instrument; a symbol not returned, returned twice, or no reader gives `HasInstrument` false and no bars read. `u.ConID` is no longer read for bars.
2. The key of a fund row is its underlying key `Market:Symbol` (for example `US:SGOV`). A fund has no filer, so no issuer text is made up for it (no `etf:SGOV`): `Issuer` stays empty for funds and for names the owner has not given a CIK, and everything keyed by an issuer (earnings, facts, the Companies page) stays absent for them. Nothing is guessed from the symbol.
3. The home card lists every held underlying that is active, paused or retired, with or without an issuer. The Companies subjects are not changed: that page needs the issuer.
4. A home row with an empty issuer reads no earnings: the cell is the gap `no_issuer` with the text "earnings date: no issuer". The price and change cells read through `InstrumentID` as before.
5. Group B wiring is two modules because `Row.Issuer` is read in `internal/pages` and built in `cmd/ot`. The pages entry lands first so that the card never reads `EarningsDates` with an empty issuer.

## Modules

No change to `docs/design/modules.md`, `arch.toml` or `.go-arch-lint.yml`: no new package and no new import edge (`ot` already imports `pages`; `company_data_page` declares its reader, imports no `positions_page`).

| issue | module | order |
|---|---|---|
| pages-company-data-ids | pages | first |
| ot-company-data-ids | ot | after 1 |
| pages-home-no-issuer | pages | first |
| ot-home-held-rows | ot | after 3 |

## Interface

No interface entry. `internal/pages/api.go` and `internal/jobs/api.go` are not edited. `company_data_page.Input` gains the field `StockIDs` and `home_companies.Row` keeps its fields; both are package types, not interface files. No schema, dataset or proto changes: `Contracts: unchanged`.

## Issues

### pages-company-data-ids

Module: pages. Files: `internal/pages/company_data_page/builder.go`, `internal/pages/company_data_page/rows.go`, `internal/pages/company_data_page/builder_test.go`, `internal/pages/company_data_page/rows_test.go`. Order: first.

Result for the owner: /company-data shows the stored bar count and the newest bar date for each company that has bars.

Acceptance:
1. `builder.go` declares `StockIDReader` (new name) with the one method `StockIDs(ctx context.Context, names []string) ([]store.InstrumentName, error)` and `Input` gains the field `StockIDs StockIDReader`.
2. `rows.go` does not read `u.ConID`: `grep -n "ConID" internal/pages/company_data_page/rows.go` finds nothing. `PriceBars` is called with the instrument id that `StockIDs` returned once for `u.Symbol`.
3. A symbol that `StockIDs` does not return, returns twice, or a nil `Input.StockIDs`, gives `HasInstrument` false and no `PriceBars` call for that row.
4. `Build` calls `StockIDs` once for all rows (batches of at most 500 names), not once per row.
5. New test `TestBarsReadByResolvedInstrumentID` (new) in `builder_test.go`: the fake reader answers `StockIDs` with an id different from the row's `ConID` and the fake `PriceBars` has bars only under that id; the row's `bar_count` is the number of those bars. New test `TestSharedSymbolReadsNoBars` (new): a symbol returned twice gives `bar_count` "0" and no `PriceBars` call.
6. Tests that built `Input` without `StockIDs` and expect bars (`newInput` in `builder_test.go`, `rows_test.go`) pass the new reader.

Checked by CI: `ci-go` runs `go test ./internal/pages/company_data_page/...`.

### ot-company-data-ids

Module: ot. Files: `cmd/ot/company_data_site.go`, `cmd/ot/company_data_site_test.go`. Order: after 1.

Result for the owner: after deploy, /company-data shows non-zero bar counts for the held and watched stocks.

Acceptance:
1. `buildCompanyData` and the `build` closure of `companyDataPageNamed` set `StockIDs` of `company_data_page.Input` from `siteReaderOf[market_state_page.StockIDReader](rd, siteMarketStateIDsKey)`, as `positions_site.go` does; the demo profile leaves it nil.
2. No other file of `cmd/ot` changes; `siteMarketStateIDsKey` and its opener are not edited.
3. New test `TestCompanyDataPassesStockIDs` (new) in `company_data_site_test.go`: with a site reader under `siteMarketStateIDsKey` that returns an id for one symbol and a bar reader that has bars only under that id, the built page's row for the symbol has a `bar_count` above 0.

Checked by CI: `ci-go` runs `go test ./cmd/ot/...`.

### pages-home-no-issuer

Module: pages. Files: `internal/pages/home_companies/member.go`, `internal/pages/home_companies/earnings.go`, `internal/pages/home_companies/member_test.go`. Order: first.

Result for the owner: a held fund or a name without a CIK shows "earnings date: no issuer" in the home card instead of an earnings read of an empty key.

Acceptance:
1. `earningsCell` returns, for `issuer == ""`, `mkGap("no_issuer", "earnings date: no issuer")` before any call of `Earnings.EarningsDates`; a nonempty issuer is read as before.
2. The doc comment of `Row` states that `Issuer` may be empty (a fund or a name without a CIK) and that the row is identified by `Market` and `Symbol`.
3. New test `TestEarningsCellNoIssuerReadsNothing` (new) in `member_test.go`: a row with empty `Issuer` and a counting earnings reader gives the gap text above and zero reader calls.
4. `testdata/ok.golden.json` is not edited: `git diff --name-only BASE` does not list it.

Checked by CI: `ci-go` runs `go test ./internal/pages/home_companies/...`.

### ot-home-held-rows

Module: ot. Files: `cmd/ot/home_site.go`, `cmd/ot/home_site_test.go`. Order: after 3.

Result for the owner: the home "持仓公司" count equals the held companies page (19, not 14); SGOV, JEPQ, VGT, GOOGL and IBKR are rows.

Acceptance:
1. `homeRows` outside the demo profile builds one `home_companies.Row` per stored underlying with status active, paused or retired whose symbol is held (`heldSymbols`), whatever its `Issuer`; the row carries `Issuer` as stored (possibly empty), `Market`, `Symbol`, `BuyPrice` as `HandPrice` and the id of `homeInstrumentID`. The demo profile still uses `companiesSubjects`.
2. `companiesSubjects` and `companies_site.go` are not in `git diff --name-only BASE`.
3. New test `TestHomeCompaniesRowsIncludeHeldFundWithoutIssuer` (new) in `home_site_test.go`: an underlying of kind `etf` with empty issuer that is held gives a row with empty `Issuer`; an unheld underlying without issuer gives none.

Checked by CI: `ci-go` runs `go test ./cmd/ot/...`.

## Left out

- Entering the 11 missing CIKs (CPSH, GOOGL, IBKR, INTC, JEPQ, NOK, ORCL, SGOV, TMUS, VGT, WDC): the owner's entry on /underlyings. CIK lookup through SEC `company_tickers.json` stays "not adopted" (`docs/design/symbol.md`). JEPQ, SGOV and VGT never get one.
- Next report empty for names with a CIK, and PDD "FY2020 Q4": causes not established (needs stored `obs.earnings_date` and fact rows); a separate sweep entry.
- Two stock rows named 4063 (group C): a symbol returned twice gives no bars here, as on /positions; the resolver of the three pages is its own design.
- The Companies page and `earnings_refresh` still skip rows without an issuer.
- Web: nothing changes; it prints the texts as given.
