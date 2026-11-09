# Builders wave: the household page and the report data blocks that need no position marks, and the companies blocks that valuation leaves as gaps

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-09
Revision: 2 (slim scope after the adversarial read: only blocks that need account values, the executions and the option ledger, no position marks; 8 issues)

Status: draft for design review. No owner-only path is touched (no `contracts/proto`, no `.github`, no `ratchet.json`, no `services.toml`, no `transport.py`).

Source: owner 2026-11-08/09: options trading first and complete; nothing of plan v2.5 dropped silently; only the main IBKR account must work and the owner never pools accounts (one page per account, main by default); the margin cushion and the assignment cash never go to Telegram; the web renders data and computes nothing; own glue code sits behind an interface. Scope: `docs/design/alignment-v25.md` Decision 3 item 8 `builders-wave` and the rows "builder: household", "builders: report x4", "builder: companies" (the Owner column says: reports are computed per request and the US open report is built last).
Read on origin/main at c06e87d: `AGENTS.md`, `docs/design/modules.md`, `alignment-v25.md`, `report-page.md`, `report-page-producer.md` (Decisions 1, 2, 10, `## Interface`), `tracking-household-reports.md`, `page-schemas-v2.md`, `pages-builders.md`, `valuation.md` (Decisions 9, 12, 13, issues 9 and 16, `## Interface`), `option-positions.md` (Decisions 10, 11, 12), `account-data.md`, `contracts/schemas/page.household_page.v1.json`, `page.report_page.v2.json`, `page.companies_page.v2.json`, `contracts/datasets/account.{position,value,execution}.r1.json`, `internal/store/api.go` (`AccountReader`, `Position`, `OptionLedgerReader`, `ReadingViewReader`, `SeriesReader`, `QuoteSnapshotReader`), `internal/compute/api.go` (`OptionLedger`, `LedgerPosition`), `internal/pages/report_page/builder.go`, `internal/pages/risks_block/builder.go`, `internal/pages/api.go`, `scripts/arch/main.go` (`listedImports`, `deriveTree`), `docs/conventions/register.toml`.

On main: the schemas `page.household_page.v1`, `page.report_page.v2`, `page.companies_page.v2` and their web views; `store.AccountReader` (declared in `internal/store/api.go`), migration `0019_account.sql`; `compute.OptionLedger` and its rule (`internal/compute/optionledger`); `report_page.Build` with the risks block (`internal/pages/report_page`, `internal/pages/risks_block`), `cmd/ot/report.go`; the register ids `household.*` (13 entries, version 1) in `docs/conventions/register.toml`.
Not on main yet (named waits): the account store implementation and the job `account_capture` (`internal/store/account`, `internal/jobs/accountcapture` do not exist; `account-data.md`); the option-ledger store implementation (`internal/store/optionledger` does not exist); `docs/design/option-positions.md` (merged, not built: it adds the `LedgerPosition` terms to `internal/compute/api.go`); market-bars (PR 768, not merged: daily bars, HV30 `hv30_own`, daily USDJPY); valuation (`docs/design/valuation.md`, merged as design, `internal/pages/companies_page` not on main: its issues `companies-page-band` and `companies-page-sections`). Each entry that reads one of these says so in its Order line and builds on a fake of the interface until the real one lands.

## Goal

The owner opens `/household` and sees, for his main account, the blocks that need only the account summary values and the option ledger (overview, cash after assignment, stress, trend, obligations, income, month change, accounts row); opens `/report?market=us&slot=close` and sees what he did today (trades), the net-liquidation day change and the two meters per account, and the S&P 500 line of the market watch; and opens `/companies` and sees the header, the changes line and the glance tiles. A block that needs the price of a position is written as its own gap (`missing_quote`), named under `## Left out`. Every figure is computed in `compute` and stored nowhere.

## Decisions

1. Scope, the cut that the adversarial read asked for (`scratchpad/adv-builders-wave.md`, 1 to 3). `account-data.md` Decision 1 collects no position prices: `store.Position` has `Quantity` and `AvgCost` and no mark, and no resolver from a position to a quote snapshot exists. So this design builds only what needs `AccountReader.Values` (`NetLiquidation`, `TotalCashValue`, `ExcessLiquidity`, the margin tags), `AccountReader.Executions`, the position quantities (not their value) and the replayed option ledger. Every block that needs a mark is a gap now and a later design (`## Left out`).
2. Per request, no job, no stored page (`report-page-producer.md` Decision 3). Every block says its `data_times`; an input older than 3 session days is a `stale_input` gap.
3. Figures live in compute behind declarations of their own: a new package `internal/compute/blockapi` (`api.go`, declarations only), implemented by `internal/compute/pagefig`. Why not `internal/compute/api.go`: it is 596 lines, `option-positions.md` issue 1 takes it to at most 760 (its own acceptance), and the declarations here need about 100 lines even after the cut, so the sum passes 800; the file may not grow past the ceiling and an agent never raises one. Why not `internal/store/api.go`: 799 lines. The import rule for declarations-only packages is the one paragraph of `docs/design/modules.md` owned by market-bars (PR 768), with the list `declarationsOnly` in `scripts/arch`; this design copies none of it and changes neither. `compute/blockapi` is read only by sub-packages (`pages/household_page`, `pages/report_blocks`, `pages/companies_page` and `compute/pagefig`), never by a root component, so it needs no place in `declarationsOnly`; the pages edges follow the rule's sub-package part. The edge `compute/pagefig` to `compute/blockapi` stays inside the row of compute, which that part does not name: entry 2 reports it in its pull request description if `go run ./scripts/arch --check` refuses it. This pull request edits only the compute entry cell and the import list of `ot` in `docs/design/modules.md`, and regenerates `arch.toml` and `.go-arch-lint.yml` with `go run ./scripts/arch`. The figure type is `compute.FigureValue` (declared by `option-positions.md` issue 1 in `internal/compute/api.go`); `blockapi` imports `compute` for it and declares no figure type of its own.
4. Decimal text, no float: `math/big.Rat` as `optionledger` and `ease` do (no new dependency). The builders copy text and never round, add or sort a figure. Their tests fail on `float32`, `float64`, `math/big`, `sort` and any import of `internal/notify`.
5. One account per household page (owner: never pools accounts). `/household?account=<alias>`, main by default; an alias not in the list is a gap page "未知账户"; `scope` is the alias, `scopes` has that one entry; no function takes two aliases; a non-main alias without a capture shows its own gap and never touches the main page.
6. Household blocks built: `overview` and `accounts` (net liquidation and cash from `Values`; the stock and option cells of `accounts` are `missing_quote` gaps), `cash_after` and `stress` (`TotalCashValue` minus the obligations; stress is the cash left after assignment of all short puts; the three stepped rows need an underlying price, which neither `compute.LedgerPosition` nor `ObligationIn` carries, so they are always the gap `missing_quote` until a position-marks design adds it), `obligations` and `income` (from `compute.OptionLedger.Replay` over `store.OptionLedgerReader.FlexTradeRows`; the strike, right, expiry and open and close days come from the `LedgerPosition` terms `Right`, `Expiry`, `Strike`, `OpenedOn`, `ClosedOn` that `option-positions.md` issue 1 declares and issue 2 fills, a wait named in the Order line of entry 4: premium income of the month; a short put's strike times multiplier times contracts), `trend` (`NetLiquidation` of the last 90 session days) and `month_change` (the waterfall of net liquidation from the last session day of the previous month: premium, the rest as "其他"; the trading result needs marks and is the gap `missing_quote`). The formulas are those of the existing `household.*` register entries (version 1, unchanged, no entry added). Stress steps are constants of `pagefig`: 10, 20 and 30 percent (window default — owner may change).
7. JPY inside one account: `≈USD` only when a USDJPY close of the data day exists (market-bars: a wait); else the gap `unsupported_currency`, and no `approx_usd_total`.
8. Report blocks built: `actions` (`trades` from the day's executions of the main alias, each classified open, close, expire or exercise by compute from the signed quantity of the contract in the newest earlier capture; `candidates` null), `account_moves` (one row per configured alias, no total row: net liquidation, day amount and day percent from two `Values` captures; `biggest` and `movers` are `missing_quote` gaps; `meters` per account: `margin_buffer` = excess liquidity over net liquidation, `cash_after_assignment` = cash minus the short-put obligations), `market_watch` (`indices` row for the S&P 500 from the stored FRED series `SP500` read with `store.SeriesReader`, which is on main; `companies` the gap `missing_quote`). `global_assets` stays the `not_built` block (needs quotes: `## Left out`). The meters are on the web page only: the builder and its route import no `internal/notify` and no Telegram sender (ADR 0010; `option-positions.md` Decision 11).
9. The report keeps the shape of `report-page-producer.md`: `report_page.Config` gains `Actions`, `AccountMoves`, `MarketWatch` of the type `BlockFunc` (same shape as `RiskBlockFunc`); a nil function keeps the `not_built` block; the `Build` signature and the risks block are unchanged. `market=jp` builds the three blocks as gap blocks with the reason `jp_slice_missing` (window default — owner may change). The open-only blocks stay absent (the US open report is built last, alignment-v25 Owner column). The functions come from one new package `internal/pages/report_blocks`, which imports no `report_page` and no `risks_block`; `ot` converts.
10. Companies, after valuation `companies-page-sections`: `header` (`月-日 · 家庭持有 N 家`; N from the held set), `glance` tiles and `changes` (`pagefig` over the states the rows carry and the band readings of two days, no mark); the held set that valuation passes as nil is wired here: the stock positions of the main alias by quantity greater than zero, matched to `Subject.Symbol`, and `held.go` maps each symbol to its issuer (`cik:` or `edinet:`, the key of `companies_page.Config.Held`) through the watch entries (position quantity, no price: window default — owner may change). The held-company values and the volatility column need marks or bars and are left out (`## Left out`).
11. Frontend: no UI code and no schema change (`Contracts: unchanged`). Libraries: none new.

## Modules

New directories: `internal/compute/blockapi`, `internal/compute/pagefig` (compute), `internal/pages/household_page`, `internal/pages/report_blocks` (pages). This pull request edits `docs/design/modules.md` (compute entry cell, the import list of `ot`) and regenerates `arch.toml` and `.go-arch-lint.yml`. Entries 1, 2, 4 and 5 create or first use a component and edit `.go-arch-lint.yml` (regenerated with `go run ./scripts/arch`, never by hand), in that serial order, each from the merged file of the one before.

| # | issue | module | order |
|---|---|---|---|
| 1 | builders-wave-compute-interface (interface) | compute, plus repo (judged interface entry) | first; waits option-positions `option-positions-compute-interface` |
| 2 | page-figures-household | compute, plus repo (licensed pair) | after 1 |
| 3 | page-figures-report | compute, plus docs | after 2 |
| 4 | household-page-builder | pages, plus repo (licensed pair) | after 1, 2 |
| 5 | report-blocks | pages, plus repo (licensed pair) | after 1, 3, 4 |
| 6 | companies-blocks | pages | after 3; waits valuation `companies-page-sections` |
| 7 | builders-wave-wire | ot | after 4, 5, 6; waits valuation `valuation-wire-us` |

That is seven entries; the register ids of entry 3 are in it (licensed pair compute and docs), so no separate docs entry exists. Shared files: `internal/compute/pagefig/pagefig.go` is in 2 and 3 (3 after 2); `docs/contracts/pages.md` in 4 and 5 (5 after 4); `internal/pages/companies_page/builder.go` is in 6 and in the valuation entries (6 after them); `cmd/ot/wire.go`, `cmd/ot/site.go` and `docs/contracts/ot.md` in 7 only among these entries.

## Interface

Owner: compute owns `internal/compute/blockapi/api.go` (frozen by issue 1); pages own the constructors; ot owns the route constants.

```go
// Package blockapi declares the page figures of the household page, the report blocks and the companies blocks.
// Declarations only; the implementation is internal/compute/pagefig.
package blockapi

import (
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// Every figure is a compute.FigureValue (decimal text or a Gap; declared in internal/compute/api.go).

// ValueIn is one summary row of the account (tag NetLiquidation, TotalCashValue, ExcessLiquidity, ...).
type ValueIn struct{ Tag, Value, Currency string }

// DayValue is the net liquidation of one capture day.
type DayValue struct {
	Day            contracts.Day
	NetLiquidation string
	Currency       string
}

// ObligationIn is one open short put of the replayed ledger: strike, multiplier, contracts, premium income.
type ObligationIn struct{ Underlying, Currency, Strike, Multiplier, Contracts, PremiumIncome string }

// MoneyLine is one row of a money block.
type MoneyLine struct {
	Name, Currency, Note string
	Amount, Share        compute.FigureValue
}

// HouseholdInput is one account on one day (Alias is never the account id; UsdJpy is empty without a rate).
type HouseholdInput struct {
	Alias, Whose string
	Day          contracts.Day
	Values       []ValueIn
	MonthStart   []ValueIn
	History      []DayValue
	Obligations  []ObligationIn
	UsdJpy       string
}

// HouseholdResult holds the lines of the blocks this design builds, in display order.
type HouseholdResult struct {
	Overview, CashAfter, Stress, Obligations, Income, MonthChange, Accounts []MoneyLine
	Trend                                                                    []DayValue
}

// ExecIn is one execution; PriorQuantity is the signed quantity of the contract in the newest earlier capture.
type ExecIn struct{ Time, Symbol, SecType, Right, Strike, Expiry, Side, Quantity, Price, Currency, PriorQuantity string }

// TradeLine is one row of actions.trades; Kind is open, close, expire or exercise.
type TradeLine struct{ Account, Time, Kind, Instrument, Quantity, Price string }

// AccountMoveIn is one account on two capture days.
type AccountMoveIn struct {
	Alias         string
	Values, Prior []ValueIn
	Obligations   []ObligationIn
}

// AccountMoveOut is the row and the two meters of one account.
type AccountMoveOut struct {
	Net, DayAmount, DayPct, MarginBuffer, CashAfterAssignment compute.FigureValue
}

// CloseIn is a series: closes newest first with their days.
type CloseIn struct {
	Name, Symbol string
	Closes       []string
	Days         []contracts.Day
}

// MoveLine is the day and five-day percent change of a series.
type MoveLine struct {
	Name, Symbol string
	D1, D5       compute.FigureValue
	DataDay      contracts.Day
}

// CompanyState is one company row: State is band, owner override, awaiting approval, expired or unvalued.
type CompanyState struct{ Symbol, State, BandToday, BandThen string }

// ChangeLine is one line of the changes block.
type ChangeLine struct{ Symbol, Text string }

// PageFigures evaluates every figure of this design, no I/O, no clock.
type PageFigures interface {
	Household(HouseholdInput) HouseholdResult
	Trades(day contracts.Day, in []ExecIn) []TradeLine
	AccountMove(AccountMoveIn) AccountMoveOut
	Moves(in []CloseIn) []MoveLine
	Glance(rows []CompanyState) []MoneyLine
	Changes(rows []CompanyState) []ChangeLine
}

// NewPageFiguresFunc constructs the implementation (internal/compute/pagefig.New).
type NewPageFiguresFunc func() PageFigures
```

Pages: `internal/pages/household_page`: `type Config struct { RunID, BuildID string; Zone *time.Location; Aliases []string; Account store.AccountReader; Ledger store.OptionLedgerReader; Sessions store.SessionDayReader; Ledgers compute.NewOptionLedgerFunc; Figures blockapi.NewPageFiguresFunc }`, `func NewPage(c Config, alias string) pages.Page` with `Meta{Name: "household_page", Schema: "page.household_page.v1"}`. `internal/pages/report_blocks`: `type Config struct { Zone *time.Location; Aliases []string; Account store.AccountReader; Series store.SeriesReader; Sessions store.SessionDayReader; Figures blockapi.NewPageFiguresFunc }`, `type BlockFunc func(ctx context.Context, day contracts.Day, generatedAt time.Time) ([]byte, error)` and `Actions(c)`, `AccountMoves(c)`, `MarketWatch(c)` returning it. `report_page.Config` gains `Actions, AccountMoves, MarketWatch` of its own `BlockFunc`. `companies_page.Config` gains `Figures blockapi.NewPageFiguresFunc`. ot: `siteHouseholdRoute = "household"`, query `account`.

## Issues

Every test calls `t.Parallel()`, writes files only under `t.TempDir()`, uses a fake clock, never sleeps, opens no port, touches no live broker and no database. Acceptance items are decided by reading, grep or git; commands CI runs are on the `Checked by CI` line. `BASE=$(git merge-base HEAD origin/main)`. Each file stays under 600 lines.

### 1 builders-wave-compute-interface (interface)

Module: compute, plus repo (judged interface entry). Size: M.
Files: `internal/compute/blockapi/api.go`, `internal/compute/blockapi/api_test.go`, `docs/contracts/compute.md`, `.go-arch-lint.yml`
Design: docs/design/builders-wave.md#Interface
Order: first; waits `option-positions-compute-interface` of `docs/design/option-positions.md` (it declares `compute.FigureValue`), tested on a fake until it lands; waits market-bars (PR 768) for the declarations-only import rule (modules.md and `declarationsOnly`).
Goal: the declarations of `## Interface`, no function, with a contract test from both sides (a fake `PageFigures`; a caller that fills every input field and reads every output field), with `compute.FigureValue` as the one figure type. `scripts/arch` is not edited here.
Acceptance:
1. `api.go` declares `PageFigures`, `NewPageFiguresFunc`, `HouseholdInput`, `HouseholdResult`, `AccountMoveIn`, `AccountMoveOut` and no `func` and no `type FigureValue` (`grep -n "^func \|^type FigureValue" internal/compute/blockapi/api.go` prints nothing); it uses `compute.FigureValue`.
2. `git diff $BASE -- internal/compute/api.go internal/store/api.go` is empty.
3. `.go-arch-lint.yml` names `internal/compute/blockapi` and no root component lists it.
4. The api test has no `time.Sleep` and no `time.Now`; `git diff --name-only $BASE` lists exactly the Files paths.
Checked by CI: `go test ./internal/compute/blockapi -race -count=1`, `go run ./scripts/arch --check`, ci-go.

### 2 page-figures-household

Module: compute, plus repo (`.go-arch-lint.yml`). Size: M.
Files: `internal/compute/pagefig/pagefig.go`, `internal/compute/pagefig/household.go`, `internal/compute/pagefig/household_test.go`, `.go-arch-lint.yml`
Design: docs/design/builders-wave.md#Decisions (3, 4, 6, 7)
Order: after 1; entry 3 edits `pagefig.go` after it.
Goal: `pagefig.New()` returns a `blockapi.PageFigures`; `Household` fills overview, cash after, stress (constants 10, 20 and 30 percent), obligations, income, month change, accounts and trend from the input; a JPY line needs `UsdJpy`; the other methods return empty results until entry 3.
Acceptance:
1. `pagefig.go` has `var _ blockapi.PageFigures` and `New`; `household.go` has no `float32`, `float64` or `"sort"` and imports `math/big`; no file imports `internal/store`, `internal/pages`, `internal/jobs` or `internal/notify`.
2. `household_test.go` names `TestJpyNeedsRate`, `TestMonthChangeNeedsMonthStart`, `TestCashAfterIsCashMinusObligations`, `TestStressStepsAreConstants`, `TestTrendAscending`, `TestStockCellsAreMissingQuote`.
3. Each figure's comment names its register id (`household.overview`, `household.cash_after`, `household.stress`, `household.obligations`, `household.income`, `household.month_change`, `household.trend`, `household.accounts`).
4. `.go-arch-lint.yml` names `internal/compute/pagefig`; `git diff --name-only $BASE` lists only the Files paths.
Checked by CI: `go test ./internal/compute/pagefig -race -count=1`, `go run ./scripts/arch --check`, ci-go.

### 3 page-figures-report

Module: compute, plus docs (licensed pair). Size: M.
Files: `internal/compute/pagefig/pagefig.go`, `internal/compute/pagefig/report.go`, `internal/compute/pagefig/report_test.go`, `internal/compute/pagefig/company.go`, `internal/compute/pagefig/company_test.go`, `docs/conventions/register.toml`
Design: docs/design/builders-wave.md#Decisions (8, 10)
Order: after 2.
Goal: `Trades`, `AccountMove`, `Moves`, `Glance` and `Changes` as Decisions 8 and 10 give them; `register.toml` gets `report.trade_kind`, `report.account_day`, `report.margin_buffer` and `report.market_move`, version 1, `source_hash = "pending-recompute"`, a `history` line naming this design.
Acceptance:
1. `report.go` and `company.go` have no `float32`, `float64` or `"sort"`; neither imports `internal/store`, `internal/pages`, `internal/jobs` or `internal/notify`.
2. `report_test.go` names `TestSellFromZeroIsOpen`, `TestBuyOnShortIsClose`, `TestMeterCashAfterAssignment`, `TestNoPriorGivesGap`, `TestFiveDayNeedsSixCloses`, `TestStaleCloseIsGap`; `company_test.go` names `TestGlanceCountsEveryState`, `TestUnchangedRowIsSilent`.
3. `register.toml` has an `id = "report.margin_buffer"` entry and the four ids above each name `docs/design/builders-wave.md`; the `household.*` entries are unchanged in the diff.
4. `git diff --name-only $BASE` lists only the Files paths.
Checked by CI: `go test ./internal/compute/pagefig -race -count=1`, the `checks.yml` register check, ci-go.

### 4 household-page-builder

Module: pages, plus repo (`.go-arch-lint.yml`). Size: M.
Files: `internal/pages/household_page/page.go`, `internal/pages/household_page/builder.go`, `internal/pages/household_page/blocks.go`, `internal/pages/household_page/strings.go`, `internal/pages/household_page/builder_test.go`, `internal/pages/household_page/purity_test.go`, `docs/contracts/pages.md`, `.go-arch-lint.yml`
Design: docs/design/builders-wave.md#Decisions (2, 5, 6, 7)
Order: after 1, 2; entry 5 edits `docs/contracts/pages.md` and `.go-arch-lint.yml` after it. Waits (tested on fakes): `internal/store/account`, `internal/store/optionledger`, `option-positions-compute-interface` and `option-positions-ledger-terms` of `docs/design/option-positions.md` (the `LedgerPosition` terms `Right`, `Expiry`, `Strike`, `OpenedOn`, `ClosedOn`).
Goal: `NewPage(c, alias)` reads `Values`, the history, the month start and the replayed ledger, calls `Figures().Household`, and writes `household_page` v1: `scope` the alias, one `scopes` entry, each mark-needing block (composition, allocation, concentration, long term, movers) its `gap` with `missing_quote`; an unknown alias or a nil reader gives a gap page.
Acceptance:
1. `Meta{Name: "household_page", Schema: "page.household_page.v1"}` is in `page.go`; `builder.go` calls `Household(`; `git diff $BASE -- internal/pages/api.go` is empty.
2. The non-test files have no `float32`, `float64`, `math/big`, `"sort"`, `internal/notify`, `internal/jobs`, `database/sql` or `"os"` (a test in `purity_test.go` enforces it).
3. `builder_test.go` names `TestUnknownAliasIsGapPage`, `TestOneScopeNoPooling`, `TestSecondAccountFailureLeavesMainAlone`, `TestMarkBlocksAreMissingQuoteGaps`, `TestOutputValidatesAgainstSchema` (reads `page.household_page.v1.json`), `TestNilReadersGiveGaps`.
4. `docs/contracts/pages.md` names `household_page`; `.go-arch-lint.yml` names `internal/pages/household_page`; `git diff --name-only $BASE` lists only the Files paths.
Checked by CI: `go test ./internal/pages/household_page -race -count=1`, `go run ./scripts/arch --check`, ci-go.

### 5 report-blocks

Module: pages, plus repo (`.go-arch-lint.yml`). Size: L.
Files: `internal/pages/report_blocks/common.go`, `internal/pages/report_blocks/strings.go`, `internal/pages/report_blocks/actions.go`, `internal/pages/report_blocks/account_moves.go`, `internal/pages/report_blocks/market_watch.go`, `internal/pages/report_blocks/blocks_test.go`, `internal/pages/report_page/builder.go`, `internal/pages/report_page/blocks.go`, `internal/pages/report_page/builder_test.go`, `docs/contracts/pages.md`, `.go-arch-lint.yml`
Design: docs/design/builders-wave.md#Decisions (8, 9)
Order: after 1, 3, 4. Waits (fakes): `internal/store/account`.
Goal: `Actions`, `AccountMoves` and `MarketWatch` write the three blocks of Decision 8 as `report_page` v2 block JSON; `report_page.Config` gains the three `BlockFunc` fields, a non-nil function's bytes replace the `not_built` block, `market` `JP` gives `jp_slice_missing` gap blocks without calling them.
Acceptance:
1. `report_blocks` declares `BlockFunc`, `Actions`, `AccountMoves`, `MarketWatch`; its non-test files have no `float32`, `float64`, `math/big`, `"sort"`, `internal/notify`, `telegram`, `internal/jobs`, `database/sql`, `internal/pages/report_page` or `internal/pages/risks_block`.
2. `blocks_test.go` names `TestOneRowPerAliasNoTotal`, `TestSecondAccountMissingIsPartial`, `TestMetersPerAccount`, `TestNoExecutionsIsOkEmpty`, `TestIndexRowFromSeries`, `TestCompaniesCellIsMissingQuote`, `TestOutputValidatesAgainstReportSchema`.
3. `report_page/builder_test.go` names `TestNilBlockFuncKeepsNotBuilt`, `TestFuncBytesReplaceBlock`, `TestJpIsGapWithoutCall`, `TestRisksBlockUnchanged`; `Build` keeps its signature (`git diff $BASE -- internal/pages/report_page/builder.go` removes no `func Build` line).
4. `global_assets` is still produced by the constant of `blocks.go` (the diff adds no `global_assets` function).
5. `.go-arch-lint.yml` names `internal/pages/report_blocks`; `git diff --name-only $BASE` lists only the Files paths.
Checked by CI: `go test ./internal/pages/report_blocks ./internal/pages/report_page -race -count=1`, `go run ./scripts/arch --check`, ci-go.

### 6 companies-blocks

Module: pages. Size: S.
Files: `internal/pages/companies_page/header.go`, `internal/pages/companies_page/glance.go`, `internal/pages/companies_page/changes.go`, `internal/pages/companies_page/builder.go`, `internal/pages/companies_page/blocks_test.go`
Design: docs/design/builders-wave.md#Decisions (10)
Order: after 3, and after valuation `companies-page-band` and `companies-page-sections` (waits: they create the package and edit `builder.go`; this entry rebases on the merged file).
Goal: `Build` fills `header`, `glance` and `changes` (today `Gap`) through `Config.Figures`; `header.line` takes the held count from `Config.Held`, a null count when `Held` is nil.
Acceptance:
1. `glance.go` and `changes.go` call `Figures`; the four non-test files named here have no `float32`, `float64`, `math/big`, `"sort"` or `internal/jobs`.
2. `blocks_test.go` names `TestHeaderNilHeldHasNullCount`, `TestChangesSentenceWhenNone`, `TestGlanceTilesInOrder`.
3. `git diff $BASE -- internal/pages/api.go` is empty; `git diff --name-only $BASE` lists only the Files paths.
Checked by CI: `go test ./internal/pages/companies_page -race -count=1`, `go run ./scripts/arch --check`, ci-go.

### 7 builders-wave-wire

Module: ot. Size: M.
Files: `cmd/ot/household.go`, `cmd/ot/household_test.go`, `cmd/ot/held.go`, `cmd/ot/held_test.go`, `cmd/ot/report.go`, `cmd/ot/report_test.go`, `cmd/ot/site.go`, `cmd/ot/site_test.go`, `cmd/ot/wire.go`, `cmd/ot/wire_test.go`, `cmd/ot/pagelist.go`, `cmd/ot/pagelist_test.go`, `cmd/ot/page.go`, `cmd/ot/page_test.go`, `docs/contracts/ot.md`
Design: docs/design/builders-wave.md#Decisions (5, 8, 9, 10)
Order: after 4, 5, 6; waits valuation `valuation-wire-us` (it creates `newCompaniesPage` in `wire.go`), `internal/store/account` and `internal/store/optionledger` (until they land `ot` wires nil readers and the pages show `no_reader` gaps).
Goal: the route `/household` (query `account`, main by default) and `ot page --name household_page`; `buildReport` fills the three `BlockFunc` fields (a missing reader leaves one nil); `held.go` gives `Held` the issuers of the main alias's stock positions with a quantity above zero; `Figures` is `pagefig.New`; no `internal/notify` or Telegram import in the new files.
Acceptance:
1. `household.go` calls `household_page.NewPage`; `site.go` names `siteHouseholdRoute`; `pagelist.go` names `household_page`; `report.go` names `report_blocks.Actions`, `report_blocks.AccountMoves`, `report_blocks.MarketWatch`; `held.go` declares `heldIssuers`.
2. `household.go`, `held.go` and `report.go` have no `internal/notify` and no `telegram`.
3. Tests named `TestHouseholdDefaultsToMain`, `TestHouseholdUnknownAccountIsGapPage`, `TestReportNilReadersKeepNotBuilt`, `TestReportJpIsGapBlocks`, `TestHeldKeepsPositiveStockQuantityOnly`, `TestHeldIsMainAliasOnly` exist once each; `errReportQuery` is still used in `report.go`.
4. `docs/contracts/ot.md` names `household`; `git diff --name-only $BASE` lists only the Files paths.
Checked by CI: `go test ./cmd/ot -race -count=1`, `go run ./scripts/arch --check`, ci-go.

## Left out

Each line names the reason; none is dropped silently.

- Every block that needs a mark waits for a position-marks design (`account-data.md` Decision 1 collects no position prices). That design must provide a mark per position per capture (a stored price beside `account.position`), or an instrument resolver over the quote snapshots (symbol or contract to instrument id, since `QuoteSnapshotReader` is keyed by instrument id) and option marks (the mid of bid and ask of each held option contract; the held-contract capture of `option-ledger.md` is not on main). The blocks: household `composition`, `allocation`, `concentration`, `long_term`, `movers`, the stock and option cells of `accounts`, and the trading result of `month_change`; report `account_moves.movers`, the `biggest` cell, `market_watch.companies`, and the whole `global_assets` block (proxies and sector rows, which also need the daily bars of market-bars, and their symbol lists); companies held-company values. Each is written now as its own named gap (`missing_quote`), so nothing is hidden.
- The volatility column of the companies page (`hv30_own` of market-bars, PR 768, not merged): its own small entry after that design.
- `allocation.target` and `difference`: no owner targets and no settings table (plan section 11 week 11).
- The JP report slice (JP indices, companies, calendar days): no JP source; `jp_slice_missing` gaps. The US open report and the open-only blocks, `actions.candidates`: the US open report is built last.
- The news cell, a pooled household total, and Flex-only accounts (spouse and daughter through Flex): no source, never pooled, and the account-data follow-up.
- `home_page` members and the other pages of the wave (option pages, structures, tracking, market, boards 14 to 16): their own designs. The risks block: untouched.
- Alternatives not chosen: the types in `internal/compute/api.go` (596 plus 160 of `option-positions.md` plus about 100 here passes 800; its own acceptance caps the file at 760) or in `internal/store/api.go` (799); computing in the page builders (the `pages` row forbids it).

## Adversarial

Answers to the read-only attack (kept in the dispatcher window's scratch as adv-builders-wave.md), read against revision 1.

1. Household cannot value real positions: adopted. Decision 1 cuts every mark-needing block to a gap and `## Left out` names what the later design must provide.
2. Missing mark source and a chain on unbuilt stores: adopted for the marks; the stores stay waits and every entry is tested on fakes, said in the Order line of each.
3. Simpler, about 8 issues: adopted; 17 became 7 and one `pagefig` package replaces `householdfig` and `pagefig`, and the `.go-arch-lint.yml` chain is 1, 2, 4, 5.
4. Lock-in of `blockapi` and the import rule: not adopted as asked (a move into `compute/api.go`). The slim declarations are about 100 lines, the sum with `option-positions.md` passes 800, and an agent never raises a ceiling, so the separate package stays and the import rule is left to market-bars (PR 768) (Decision 3). Adopted in part: the proxy and sector lists are left out with the mark blocks; the stress steps stay constants (marked window default).
5. False claim that snapshots cover the positions: adopted; the claim is removed. The held set uses the quantity of stock positions only, and `--check` on the unchanged script was run (`arch: ok`).
6. Slips: adopted; the mark mapping is gone from the issues, exact-count greps are replaced by named symbols and named tests, and every file is held under 600 lines.
