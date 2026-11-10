# Pages real data: home, household, positions, risk and advice read stored data and show a gap where there is none

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-10
Revision: 3 (after the design review of 90aae1ef)

Source: owner 2026-11-10: "前段几乎没什么真实数据" (the front end has almost no real data). Read on origin/main at 3c91d89d: `AGENTS.md`, `docs/design/modules.md`, `account-data.md` (Decisions 1, 4, 9, 10), `position-marks.md`, `option-positions.md` (Left out: `positions_page` stays, no pooled household view, per account only), `option-page-builders.md`, `pages-registry.md`, `ot-wire-split.md`, `unified-site.md`, `home-page.md`, `web-shell-layout.md` (format), `internal/pages/api.go`, `internal/pages/household_page/*`, `internal/pages/home_page/*`, `internal/pages/structures_page/config`, `internal/compute/blockapi/api.go`, `internal/store/api.go` (`AccountReader`, `Position`, `AccountValue`, `Execution`), `internal/store/mark/api.go`, `cmd/ot/{sample,site,pagereg,companies_site,site_positions}.go`, `contracts/schemas/page.{home_page.v3,household_page.v1,positions_page.v3,household_risk.v1,advice_page.v1,*_summary}.json`, `web/fixtures/*`.

Goal: the owner opens `/home`, `/household`, `/positions`, `/risk` and `/advice` of `ot serve` and every number on them is a stored value built by Go, or a dash with its reason. No page shows a sample number. The web renders what the payload holds and computes nothing; no schema is edited.

## What main holds today (read at 3c91d89d)

- `cmd/ot/sample.go` is 24 lines: `sampleSchemas` maps `home`, `household`, `positions`, `risk`, `advice` to a schema name and `sampleVM` returns `{"schema":"sample_page","schema_version":1,"page":"<schema>"}`; the web bundle then shows `web/fixtures/*/ok.json` under the banner of `unified-site.md` Decision 4. `cmd/ot/site.go` reaches `sampleVM` in the `default` case, after the registered families (`extraPageBuilder`, `pagereg.go`), so a route registered with `registerPage` is never sampled. Verified: the five routes are the only entries.
- Stored and read: `store.AccountReader` (`Positions`, `Values`, `Executions`, views `rd.account_*_r1`, `account-data.md` Decision 10) and its captures; `store.mark.Reader.Marks(alias, day)` (one mark per open position, `position-marks.md`); `store.OptionLedgerReader`; `store.SessionDayReader`; `store.OptionEodReader`; the series and reading-view readers. `siteReaders` already opens `account` and the ledger (`openOptionalStores`), used by the structures pages.
- Builders: `household_page.Build` (internal/pages/household_page) is written and tested and called by no one in `cmd/ot`; it builds overview, cash after, stress, obligations, income, month change, trend and accounts from the daily capture and the replayed ledger, and writes composition, concentration, long term, allocation and movers as the gap `missing_quote` (its test `TestMarkBlocksAreMissingQuoteGaps` pins all five). `home_page.Compose` composes five member documents and nothing builds any member (`home-page.md` Limits). There is no builder for `positions_page`, `household_risk` or `advice_page`.
- Compute: `blockapi.PageFigures` has `Household`, `Trades`, `AccountMove`, `Moves`, `Glance`, `Changes`; `trades` classifies executions as open, close, expire or exercise from the prior quantity. `compute.Structure.Greeks` (`GreekSet`) holds the net greeks of an option structure from `option_eod`. No figure of a held stock's value share, of unrealised profit, of delta by underlying or of vega by expiry exists.
- Page registry: a page is one `registerPage(pageFamily{name, route, order, build, named})` in its own `cmd/ot/<page>_site.go` (`companies_site.go` is the pattern); the builder lives in `internal/pages/<page>`; the readers come from `siteReaders` or from `a.deps`.

## Decisions

1. Same pattern as `/options` and `/companies`: a builder in its own package of `internal/pages` (`func Build(ctx, Config, ...)`, no clock, no store write, no figure computed in it), a registered family in `cmd/ot`, and `ot page --name <page>` for the same builder. A sample route stops being served the moment its family is registered (`site.go` order, above); `sample.go` is cleaned in the last issue, not in each.
2. Which day and which account. The day is the day of the request (`--day` or the clock), as in every builder. Each page shows one account, chosen by the query `?account=<alias>` (the key `siteAccountQuery` of `site_positions.go`), default the first alias of `siteAliases`; accounts are never pooled (owner 2026-11-08/09, `option-positions.md`). The capture used is the newest capture on or before the day within `staleSessions` sessions (the rule `household_page` already has); an older capture gives the page gap `stale_input`, none gives `no_data`, a nil reader gives `no_reader` (one exception: the advice page reads no capture, issue 14). A gap page keeps its title and the envelope and has no row. The mark of a position is the one of that capture (`position-marks.md`).
3. A missing value is a gap, never a zero and never a sample: a figure that needs a price the mark row lacks (`Basis` none, empty `Price`) is `missing_quote`; a block whose source no job stores is `not_built` and its text says what is missing. The web already shows a gap as a dash with the reason (`web-shell-layout.md` Decision 2).
4. Compute stays in `compute`. The household blocks that need a mark, the positions figures and the risk figures are new functions of a new interface `blockapi.HoldingFigures` (one interface issue, then the implementation), so `pages` builds JSON and formats nothing it did not receive (`household_page/purity_test.go` is the model and is copied for each new package).
5. Per-page sources and blocks (the table is the contract of the issues):

| page | schema | filled from stored data | stays a gap (Left out) |
|---|---|---|---|
| household | `household_page` v1 | overview, cash after, stress, obligations, income, month change, trend, accounts: account capture values and the ledger (already built). Composition, allocation, concentration: positions with marks (after the figures issue) | movers (a day change needs two marks, as the day profit of positions), long term (no holding-period source) |
| positions | `positions_page` v3 | per account view: stocks and options rows (code, name, quantity, cost from `AvgCost`, price from the mark, unrealised); `trades` from `Executions` classified by `Trades`; options remaining premium from the replayed ledger; the role cell is the account label | day profit (needs two marks; gap until the previous session has one); the pooled view `household` |
| risk | `household_risk` v1 | delta by underlying (shares of stock plus option net delta, valued at the mark) and vega by expiry, from positions and `Structure.Greeks` | beta weighted delta (no beta source), sigma, standard metrics (no stored source) |
| advice | `advice_page` v1 | nothing: no store holds yesterday's advice | all rows: the page is the schema's `gap` with no row |
| home | `home_page` v3 | the five members, each built from the stored data of its own page: household tiles (capture net liquidation, cash, day and month change), held companies (reading view, as `companies_page`), macro (series), market (as `market_page`), options (option ledger premium series) | any tile or series of a member with no source is that member's own gap |

6. Home is composed of members that are built by their own builders; a member whose source is absent is a card with its `gaps` filled and no tile (the required `gaps` and `notes` of `page.*_summary` carry it). A member builder never calls another page's builder; it reads the stores and uses the same figure functions, so no page waits for another to be correct.
7. The readers a page gets in `cmd/ot` are the real fields of `siteReaders` (`site.go`), not the ones the structures pages declare: `rd.noticeAccount` (the opened account store; `rd.account` is always nil), `rd.optionLedger`, `rd.sessions`, the constructors `rd.ledgers` and `rd.structures` (set by `withPositionsRules`), `pagefig.New` and `pagefig.NewHoldings` imported by the page's own `cmd/ot` file (as `report.go` does for `pagefig.New`), and, for marks, `deps.openMark` below. The demo profile has no session reader (`openSiteReaders` sets `series`, `view`, `noticeAccount`, `optionLedger` and the option demos only), so under `--profile demo` the builders that need `Sessions` (household, positions, risk) show their `no_reader` gap page. `/advice` shows its `not_built` gap (it needs no capture, issue 14). `/home` is not a `no_reader` page: the members companies, macro, market and options are built from the demo readers (`demoView`, `demoReader`, `demoTrackingLedger`), and the household member, which needs `Sessions`, is a card with its gap. No demo reader is added.
8. Marks. `migrations/0026_position_mark.sql` grants SELECT on `rd.account_position_mark_r1` to `ot_ro_pages` (and to `ot_w_posmark`, the writer of `account.position_mark`), and `markpg.Marks` reads that view. So `ot serve` opens the mark store with `deps.openMark` (`mark.OpenFunc`, `markpg.Open`) and the pages line `OT_PG_DSN_PAGES` (`store.DSNEnvPages`, the `dsn` that `openSiteReaders` already holds), never with `OT_PG_DSN_POSMARK`, so the server holds no write role. Only `Marks` is called. An open that fails or a nil `deps.openMark` leaves the reader nil and the blocks that need a mark are gaps.
9. No new job, no new table, no new dataset, no schema or generated type is edited (`Contracts: unchanged` on every entry). The web is untouched: its views and fixtures exist; the sample banner is a shell literal that shows only for a `sample_page` payload.

## Modules

Edited rows: `ot` (`cmd/**`), `pages` (`internal/pages/**`), `compute` (`internal/compute/**`). `docs/design/modules.md` is not edited (every new package is under `internal/pages/**` or `cmd/**`). An issue edits only the files of its Files line. A needed file outside it is reported in the pull request.

## Interface

`internal/compute/blockapi/api.go` (compute row, entry `blockapi`) gains, in issue 3, a new interface `HoldingFigures` (with `NewHoldingFiguresFunc` and the input and output types) of two methods: `Holdings(HoldingsIn) HoldingsResult` and `Risk(RiskIn) RiskResult`, with these fields (decimal text in, `compute.FigureValue` out): `HoldingsIn{Currency, NetLiquidation string; Rows []HoldingIn}` (`NetLiquidation` is the `NetLiquidation` tag of the capture values in `Currency`, the denominator of every share; empty gives the gap `no_data` for shares); `HoldingsResult{Rows []HoldingLine; TotalValue, TotalUnrealised FigureValue}` with `HoldingLine{Underlying, SecType string; Value, Unrealised, Share FigureValue}`; `RiskIn{Currency string; Stocks []HoldingIn; Greeks []GreekIn}`; `RiskResult{Delta, Vega []RiskLine; DeltaTotal, VegaTotal FigureValue}` with `RiskLine{Key string; Value FigureValue}` (`Key` is the underlying for delta and the expiry day for vega). `PageFigures` is not edited, so the test fakes of `household_page` and `report_blocks`, which implement it without embedding, still compile; only the new code uses the new interface. `internal/pages/api.go` is not edited.

## Issues

Tests use no clock, no sleep, no file at a fixed place and no port; a builder test reads a golden file under its own `testdata` and a fake reader. Acceptance items are decided by reading, grep or git; each prints one number. The commands CI runs are under `Checked by CI`. `BASE` is `git merge-base HEAD origin/main`. A wiring issue lists `cmd/ot/sample.go` nowhere: it is not touched until issue 17.

### 1 ot-household-route

Module: ot. Size: M.
Files: `cmd/ot/household_site.go` (new), `cmd/ot/household_site_test.go` (new)
Design: docs/design/pages-real-data.md#Decisions
Order: first.
Goal: `/household` and `ot page --name household_page` serve `household_page.Build` of the account named by `?account=`, with `Account: rd.noticeAccount`, `Ledger: rd.optionLedger`, `Sessions: rd.sessions`, `Ledgers: rd.ledgers` and `Figures: pagefig.New` (Decision 7; the demo profile has no session reader, so it gets the `no_reader` page). The family is registered with `route: "household"`, `name: "household_page"`, an `order` not used by another family. Decision 2 gaps hold: with no reader the page is the gap page `no_reader`.
Acceptance:
1. `grep -c 'registerPage' cmd/ot/household_site.go` prints 1 and `grep -c 'route: *siteHouseholdRoute\|siteHouseholdRoute *= *"household"' cmd/ot/household_site.go` prints at least 1.
2. `grep -c 'household_page\.' cmd/ot/household_site.go` prints at least 1.
3. `grep -c 'sampleVM\|sampleSchemas' cmd/ot/household_site.go` prints 0.
4. `grep -c 'no_reader\|unknown account\|siteAccountQuery' cmd/ot/household_site_test.go` prints at least 2.
5. `git diff --name-only $(git merge-base HEAD origin/main) | grep -v -e '^cmd/ot/household_site.go$' -e '^cmd/ot/household_site_test.go$' | wc -l` prints 0.

### 2 ot-advice-route

Module: ot. Size: S.
Files: `cmd/ot/advice_site.go` (new), `cmd/ot/advice_site_test.go` (new)
Design: docs/design/pages-real-data.md#Decisions
Order: after 14.
Goal: `/advice` and `ot page --name advice_page` serve the builder of issue 14.
Acceptance:
1. `grep -c 'registerPage' cmd/ot/advice_site.go` prints 1 and `grep -c 'advice_page\.' cmd/ot/advice_site.go` prints at least 1.
2. `grep -c '"advice"' cmd/ot/advice_site.go` prints at least 1.
3. `git diff --name-only $(git merge-base HEAD origin/main) | grep -v -e '^cmd/ot/advice_site.go$' -e '^cmd/ot/advice_site_test.go$' | wc -l` prints 0.

### 3 blockapi-holdings-risk (interface)

Module: compute. Size: S.
Files: `internal/compute/blockapi/api.go`, `internal/compute/blockapi/api_test.go`, `docs/contracts/compute.md`
Design: docs/design/pages-real-data.md#Interface
Order: first. The issues 4 to 11 wait for it.
Goal: the methods and types of the Interface section, declarations only; `api_test.go` gains a fake that implements `HoldingFigures` and a caller that uses `Holdings` and `Risk` (the contract tested from both sides). Input types carry decimal text: `HoldingIn{Underlying, SecType, Currency, Quantity, Multiplier, Cost, Mark string}`, `GreekIn{Underlying string, Expiry contracts.Day, Delta, Vega string}` (no underlying price is needed: the stock delta is the share count and the option delta is already in dollars per one dollar move; `Delta` and `Vega` are the net values of `compute.GreekSet` in the unit its note gives, which the type note of this issue restates); `Mark` is empty when the position has no price, which gives the gap `missing_quote` in the result and never a zero. The note of the type says the unit of delta (dollars per one dollar move of the underlying) and of vega (dollars per volatility point), the text the web shows as `meaning`.
Acceptance:
1. `grep -c 'Holdings(\|Risk(' internal/compute/blockapi/api.go` prints at least 2.
2. `git diff $(git merge-base HEAD origin/main) -- internal/compute/blockapi/api.go | grep -c '^-[^-]'` prints 0 (additive).
3. `grep -c 'Holdings\|Risk' internal/compute/blockapi/api_test.go` prints at least 2 and `grep -c 't.Parallel()' internal/compute/blockapi/api_test.go` prints at least 1.
4. `grep -c 'Holdings\|Risk' docs/contracts/compute.md` prints at least 2.
5. `git diff --name-only $(git merge-base HEAD origin/main) | grep -v -e '^internal/compute/blockapi/api.go$' -e '^internal/compute/blockapi/api_test.go$' -e '^docs/contracts/compute.md$' | wc -l` prints 0.

### 4 pagefig-holdings-risk

Module: compute. Size: M.
Files: `internal/compute/pagefig/holdings.go` (new), `internal/compute/pagefig/holdings_test.go` (new), `internal/compute/pagefig/risk.go` (new), `internal/compute/pagefig/risk_test.go` (new), `internal/compute/pagefig/holdings_new.go` (new)
Design: docs/design/pages-real-data.md#Decisions
Order: after 3.
Goal: `pagefig.NewHoldings()` returns a `HoldingFigures` implementing `Holdings` and `Risk`: value is quantity times mark times multiplier, unrealised is value minus quantity times cost times multiplier, share is value over `NetLiquidation` of the same `Currency` (a row in another currency is `unsupported_currency`, as `Household` does), stock delta is the quantity, option delta and vega are summed from the `GreekIn` rows by underlying and by expiry. A position with an empty mark gives `missing_quote` for its value, unrealised and share and is left out of every total, the totals then carry the gap `missing_quote` (a total that omitted a position is never shown as complete). Decimal arithmetic as the existing `household.go` does (the library it uses, no float).
Acceptance:
1. `grep -c 'func (.*) Holdings(\|func (.*) Risk(' internal/compute/pagefig/*.go | awk -F: '{s+=$2} END {print s}'` prints 2.
2. `grep -c 'missing_quote' internal/compute/pagefig/holdings.go` prints at least 2.
3. `grep -c 'float64' internal/compute/pagefig/holdings.go internal/compute/pagefig/risk.go | awk -F: '{s+=$2} END {print s}'` prints 0.
4. `grep -c 't.Parallel()' internal/compute/pagefig/holdings_test.go internal/compute/pagefig/risk_test.go | awk -F: '{s+=$2} END {print s}'` prints at least 2.
5. `git diff --name-only $(git merge-base HEAD origin/main) | grep -v '^internal/compute/pagefig/' | wc -l` prints 0.

### 5 household-holdings-blocks

Module: pages. Size: M.
Files: `internal/pages/household_page/holdings.go` (new), `internal/pages/household_page/holdings_test.go` (new), `internal/pages/household_page/builder.go`, `internal/pages/household_page/page.go`, `internal/pages/household_page/strings.go`, `internal/pages/household_page/builder_test.go`, `internal/pages/household_page/testdata/holdings.golden.json` (new)
Design: docs/design/pages-real-data.md#Decisions
Order: after 4.
Goal: `household_page.Config` gains `Marks mark.Reader` (`internal/store/mark`) and `Holdings blockapi.NewHoldingFiguresFunc`; with it, composition, allocation and concentration are built from `Holdings().Holdings` of the capture's positions and marks; without it (nil) they stay `missing_quote` as today (Decision 3: a missing mark is `missing_quote`). `movers` becomes the gap `no_data` (a day change needs two marks) and `long_term` the gap `not_built` (no source); `TestMarkBlocksAreMissingQuoteGaps` is updated to these reasons. `Positions`, `Values` and marks are read for the capture day only. The page `run_status` is `ok` when every block is built, else `partial` as today.
Acceptance:
1. `grep -c 'Marks' internal/pages/household_page/page.go` prints at least 1 and `grep -c 'Holdings(' internal/pages/household_page/holdings.go` prints at least 1.
2. `grep -c 'missingQuote()' internal/pages/household_page/builder.go` prints at most 3 (composition, concentration, allocation on the nil path) and `grep -c 'not_built' internal/pages/household_page/builder.go` prints at least 1 (long term); `grep -c 'no_data' internal/pages/household_page/builder.go` prints at least 2 (movers; the existing no-capture path); the texts are in `strings.go`.
3. `grep -c 'float64\|math\.' internal/pages/household_page/holdings.go` prints 0.
4. `git diff --name-only $(git merge-base HEAD origin/main) | grep -v '^internal/pages/household_page/' | wc -l` prints 0.

### 6 ot-household-marks

Module: ot. Size: S.
Files: `cmd/ot/household_site.go`, `cmd/ot/household_site_test.go`, `cmd/ot/site.go`
Design: docs/design/pages-real-data.md#Decisions
Order: after 1 and 5.
Goal: the mark store is opened once with the site readers by `deps.openMark` with the pages line `OT_PG_DSN_PAGES` (Decision 8; not the write role line), stored as the field `marks mark.Reader` of `siteReaders`, closed with the others, and set as `Config.Marks` together with `Holdings: pagefig.NewHoldings`; a failed open or a nil opener leaves it nil and the blocks stay gaps. Only `Marks` is ever called.
Acceptance:
1. `grep -c 'Marks' cmd/ot/household_site.go` prints at least 1.
2. `grep -c 'openMark' cmd/ot/site.go` prints at least 1 and `grep -c 'OT_PG_DSN_POSMARK\|DSNEnvPositionMark' cmd/ot/site.go` prints 0.
3. `git diff --name-only $(git merge-base HEAD origin/main) | grep -v -e '^cmd/ot/household_site.go$' -e '^cmd/ot/household_site_test.go$' -e '^cmd/ot/site.go$' | wc -l` prints 0.

### 7 positions-page-builder

Module: pages. Size: L.
Files: `internal/pages/positions_page/page.go` (new), `internal/pages/positions_page/builder.go` (new), `internal/pages/positions_page/rows.go` (new), `internal/pages/positions_page/trades.go` (new), `internal/pages/positions_page/strings.go` (new), `internal/pages/positions_page/builder_test.go` (new), `internal/pages/positions_page/purity_test.go` (new), `internal/pages/positions_page/testdata/ok.golden.json` (new)
Design: docs/design/pages-real-data.md#Decisions
Order: after 4.
Goal: `positions_page.NewPage(Config, alias)` builds `page.positions_page.v3`: one view per configured alias only, `key` the alias, no pooled view. The `Config` is `RunID, BuildID, Zone, Aliases, Account store.AccountReader, Marks mark.Reader, Ledger store.OptionLedgerReader, Sessions store.SessionDayReader, Ledgers compute.NewOptionLedgerFunc, Figures blockapi.NewPageFiguresFunc, Holdings blockapi.NewHoldingFiguresFunc`. Stock rows: code, name, quantity, cost (`AvgCost`), price (mark), unrealised from `Holdings`; option rows add expiry, days left (from the day, as the structures page does), remaining premium from the replayed ledger (a position with no ledger match has that cell as a gap). The `role` cell of every row, stock or option, is the account label (the text `household_page` gives the account), as the fixture does. The day profit column is a gap in every row. `trades` is the executions of the last 30 session days through `Figures().Trades`. The column ids are those of `web/fixtures/positions-page/ok.json`. A nil reader gives the page gap `no_reader`; no capture gives `no_data`.
Acceptance:
1. `grep -c 'page.positions_page.v3\|"positions_page"' internal/pages/positions_page/page.go` prints at least 1.
2. `grep -c 'Holdings(\|Trades(' internal/pages/positions_page/*.go | awk -F: '{s+=$2} END {print s}'` prints at least 2.
3. `grep -c 'float64\|math\.\|time\.Now' internal/pages/positions_page/*.go | awk -F: '{s+=$2} END {print s}'` prints 0.
4. `wc -l internal/pages/positions_page/*.go | sort -n | tail -2 | head -1 | awk '{print ($1<600)}'` prints 1.
5. `git diff --name-only $(git merge-base HEAD origin/main) | grep -v '^internal/pages/positions_page/' | wc -l` prints 0.

### 8 ot-positions-route

Module: ot. Size: S.
Files: `cmd/ot/positions_site.go` (new), `cmd/ot/positions_site_test.go` (new)
Design: docs/design/pages-real-data.md#Decisions
Order: after 6 and 7.
Goal: `/positions` and `ot page --name positions_page` serve issue 7's builder with the readers of Decision 7 and the mark reader of issue 6 (`Account: rd.noticeAccount`, `Ledger: rd.optionLedger`, `Sessions: rd.sessions`, `Ledgers: rd.ledgers`, `Marks: rd.marks`, `Figures: pagefig.New`, `Holdings: pagefig.NewHoldings`).
Acceptance:
1. `grep -c 'registerPage' cmd/ot/positions_site.go` prints 1 and `grep -c 'positions_page\.' cmd/ot/positions_site.go` prints at least 1.
2. `grep -c '"positions"' cmd/ot/positions_site.go` prints at least 1.
3. `git diff --name-only $(git merge-base HEAD origin/main) | grep -v -e '^cmd/ot/positions_site.go$' -e '^cmd/ot/positions_site_test.go$' | wc -l` prints 0.

### 9 risk-page-builder

Module: pages. Size: M.
Files: `internal/pages/risk_page/page.go` (new), `internal/pages/risk_page/builder.go` (new), `internal/pages/risk_page/strings.go` (new), `internal/pages/risk_page/builder_test.go` (new), `internal/pages/risk_page/purity_test.go` (new), `internal/pages/risk_page/testdata/ok.golden.json` (new)
Design: docs/design/pages-real-data.md#Decisions
Order: after 4.
Goal: `risk_page.NewPage(Config, alias)` builds `page.household_risk.v1`: the `delta` block (rows per underlying) and the `vega` block (rows per expiry) from `Holdings().Risk` over the capture's positions, the marks and the `Greeks` of the structures (`compute.NewOptionStructuresFunc`, as `structures_page` loads them). The `beta_delta`, `sigma` and `standard_metrics` blocks are the schema's gap with reason `not_built` and a text naming the missing source; their rows and metrics are empty. `run_status` is `partial`. The `Config` mirrors issue 7 plus `Eod`, `Terms` and `Structures` as `structures_page.Config`.
Acceptance:
1. `grep -c 'household_risk' internal/pages/risk_page/page.go` prints at least 1.
2. `grep -c 'Risk(' internal/pages/risk_page/builder.go` prints at least 1 and `grep -c 'not_built' internal/pages/risk_page/builder.go` prints at least 3.
3. `grep -c 'float64\|math\.\|time\.Now' internal/pages/risk_page/*.go | awk -F: '{s+=$2} END {print s}'` prints 0.
4. `git diff --name-only $(git merge-base HEAD origin/main) | grep -v '^internal/pages/risk_page/' | wc -l` prints 0.

### 10 ot-risk-route

Module: ot. Size: S.
Files: `cmd/ot/risk_site.go` (new), `cmd/ot/risk_site_test.go` (new)
Design: docs/design/pages-real-data.md#Decisions
Order: after 6 and 9.
Goal: `/risk` and `ot page --name household_risk` serve issue 9's builder with the same readers as issue 8 plus `Structures: rd.structures`, `Eod: rd.eod` and `Terms: rd.terms`.
Acceptance:
1. `grep -c 'registerPage' cmd/ot/risk_site.go` prints 1 and `grep -c 'risk_page\.' cmd/ot/risk_site.go` prints at least 1.
2. `grep -c '"risk"' cmd/ot/risk_site.go` prints at least 1.
3. `git diff --name-only $(git merge-base HEAD origin/main) | grep -v -e '^cmd/ot/risk_site.go$' -e '^cmd/ot/risk_site_test.go$' | wc -l` prints 0.

### 11 home-member-household

Module: pages. Size: M.
Files: `internal/pages/home_household/member.go` (new), `internal/pages/home_household/member_test.go` (new), `internal/pages/home_household/purity_test.go` (new), `internal/pages/home_household/testdata/ok.golden.json` (new)
Design: docs/design/pages-real-data.md#Decisions
Order: after 3.
Goal: `home_household.Build(ctx, Config, day, at)` returns `page.household_summary.v2`: tiles net worth, cash, day change and month to date of the main account from the capture values (`Figures().Household` as `household_page`), each a gap when its input capture is missing. Member identity (`run_id`, `build_id`, `scope`) is the `Config`'s, which the composer checks.
Acceptance:
1. `grep -c 'household_summary' internal/pages/home_household/member.go` prints at least 1.
2. `grep -c 'Household(' internal/pages/home_household/member.go` prints at least 1.
3. `grep -c 'float64\|math\.\|time\.Now' internal/pages/home_household/member.go | awk '{print $1}'` prints 0.
4. `git diff --name-only $(git merge-base HEAD origin/main) | grep -v '^internal/pages/home_household/' | wc -l` prints 0.

### 12 home-member-companies

Module: pages. Size: M.
Files: `internal/pages/home_companies/member.go` (new), `internal/pages/home_companies/member_test.go` (new), `internal/pages/home_companies/purity_test.go` (new), `internal/pages/home_companies/testdata/ok.golden.json` (new)
Design: docs/design/pages-real-data.md#Decisions
Order: first.
Goal: `home_companies.Build` returns `page.held_companies_summary.v3`: one row per held company with the price, the band and the hand price from the same reading view and subjects as `companies_page` (`Config` carries its own `Rows []Row{Issuer, Market, Symbol, HandPrice string}` that `cmd/ot` fills from `companiesSubjects`, and `Read store.ReadingViewReader`; the package imports no `companies_page`, because a pages sub-package may import only the roots and the declarations-only packages of its depends cell (`scripts/arch/main.go`)); the sharp-move note is a gap while `sharpmove` readings are absent.
Acceptance:
1. `grep -c 'held_companies_summary' internal/pages/home_companies/member.go` prints at least 1.
2. `grep -c 'internal/pages/companies_page' internal/pages/home_companies/member.go` prints 0.
3. `git diff --name-only $(git merge-base HEAD origin/main) | grep -v '^internal/pages/home_companies/' | wc -l` prints 0.

### 13 home-member-macro

Module: pages. Size: M.
Files: `internal/pages/home_macro/member.go` (new), `internal/pages/home_macro/member_test.go` (new), `internal/pages/home_macro/purity_test.go` (new), `internal/pages/home_macro/testdata/ok.golden.json` (new)
Design: docs/design/pages-real-data.md#Decisions
Order: first.
Goal: `home_macro.Build` returns `page.macro_summary.v2` from `Config{Read compute.ReadingFunc}` the way `macro_page` reads `macro.ease` (no import of `internal/compute/ease` or `macro_page`, arch rule as issue 12); `events` is empty with a gap `not_built` (no event source is stored).
Acceptance:
1. `grep -c 'macro_summary' internal/pages/home_macro/member.go` prints at least 1.
2. `grep -c 'not_built' internal/pages/home_macro/member.go` prints at least 1 and `grep -c 'compute/ease\|pages/macro_page' internal/pages/home_macro/member.go` prints 0.
3. `git diff --name-only $(git merge-base HEAD origin/main) | grep -v '^internal/pages/home_macro/' | wc -l` prints 0.

### 14 advice-page-builder

Module: pages. Size: S.
Files: `internal/pages/advice_page/page.go` (new), `internal/pages/advice_page/builder.go` (new), `internal/pages/advice_page/builder_test.go` (new), `internal/pages/advice_page/purity_test.go` (new), `internal/pages/advice_page/testdata/gap.golden.json` (new)
Design: docs/design/pages-real-data.md#Decisions
Order: first.
Goal: `advice_page.NewPage(Config)` builds `page.advice_page.v1` with `rows` empty and `gap` set to reason `not_built` and the text "no record of earlier advice is stored yet"; `day` is the session day before the request's day, read with `Config.Sessions` (a `store.SessionDayReader`); the page needs no reader to be correct, so it is the one exception to Decision 2: with no reader (the demo profile) or no earlier session `day` is the calendar day before and the page still shows its `not_built` gap, never `no_reader`.
Acceptance:
1. `grep -c 'advice_page' internal/pages/advice_page/page.go` prints at least 1.
2. `grep -c 'not_built' internal/pages/advice_page/builder.go` prints at least 1.
3. `git diff --name-only $(git merge-base HEAD origin/main) | grep -v '^internal/pages/advice_page/' | wc -l` prints 0.

### 15 home-member-market

Module: pages. Size: M.
Files: `internal/pages/home_market/member.go` (new), `internal/pages/home_market/member_test.go` (new), `internal/pages/home_market/purity_test.go` (new), `internal/pages/home_market/testdata/ok.golden.json` (new)
Design: docs/design/pages-real-data.md#Decisions
Order: first.
Goal: `home_market.Build` returns `page.market_summary.v3` (the index chart, the index table and the volatility indices) from `Config{Readings compute.ReadingFunc, ...}`, the same reading function `market_page.Config` takes, filled by `cmd/ot`; it imports no `market_page` (arch rule as issue 12), and a gap per index with no series.
Acceptance:
1. `grep -c 'market_summary' internal/pages/home_market/member.go` prints at least 1.
2. `grep -c 'internal/pages/market_page' internal/pages/home_market/member.go` prints 0.
3. `git diff --name-only $(git merge-base HEAD origin/main) | grep -v '^internal/pages/home_market/' | wc -l` prints 0.

### 16 home-member-options

Module: pages. Size: M.
Files: `internal/pages/home_options/member.go` (new), `internal/pages/home_options/member_test.go` (new), `internal/pages/home_options/purity_test.go` (new), `internal/pages/home_options/testdata/ok.golden.json` (new)
Design: docs/design/pages-real-data.md#Decisions
Order: first.
Goal: `home_options.Build` returns `page.options_summary.v4`: tiles and the six month premium series from the replayed option ledger (`store.OptionLedgerReader` and `compute.NewOptionLedgerFunc`, as `option_tracking_page`); a month with no ledger row is a hole, not zero; the closed series text is built only from ledger rows.
Acceptance:
1. `grep -c 'options_summary' internal/pages/home_options/member.go` prints at least 1.
2. `grep -c 'OptionLedgerReader' internal/pages/home_options/member.go` prints at least 1.
3. `git diff --name-only $(git merge-base HEAD origin/main) | grep -v '^internal/pages/home_options/' | wc -l` prints 0.

### 17 ot-home-route

Module: ot. Size: M.
Files: `cmd/ot/home_site.go` (new), `cmd/ot/home_site_test.go` (new), `cmd/ot/sample.go`, `cmd/ot/sample_test.go`, `cmd/ot/site.go`
Design: docs/design/pages-real-data.md#Decisions
Order: after 2, 8, 10, 11, 12, 13, 15 and 16 (the last issue of the cut).
Goal: `/home` and `ot page --name home_page` call the five member builders and `home_page.Compose` (identity and texts as `home-page.md` Decision 6; `gap_line` is "No gap today" or the count of members with a gap, from `Compose`'s overview). `sample.go`, `sample_test.go` and the `sampleVM` branch of `site.go` are deleted, `/` stays the redirect to `/home`; the constant `sampleHome` that the redirect uses becomes `siteHomeRoute` in `home_site.go`. The web samples (`web/src/samples`) stay for a later cut.
Acceptance:
1. `grep -c 'registerPage' cmd/ot/home_site.go` prints 1 and `grep -c 'home_page\.Compose' cmd/ot/home_site.go` prints 1.
2. `grep -c 'internal/pages/home_' cmd/ot/home_site.go` prints at least 5.
3. `git ls-files cmd/ot/sample.go cmd/ot/sample_test.go | wc -l` prints 0 and `grep -rc 'sampleVM\|sampleSchemas' cmd/ot/site.go | awk '{print $1}'` prints 0.
4. `git diff --name-only $(git merge-base HEAD origin/main) | grep -v -e '^cmd/ot/home_site.go$' -e '^cmd/ot/home_site_test.go$' -e '^cmd/ot/sample.go$' -e '^cmd/ot/sample_test.go$' -e '^cmd/ot/site.go$' | wc -l` prints 0.

## Checked by CI

ci-go for every entry: `go build ./...`, `go vet`, `golangci-lint`, `scripts/arch`, `go test ./... -race` (parallel tests, `t.TempDir()`, no sleep). Entries 3 and 4 also run the contract tests of `blockapi` from both sides and the golden suites of `internal/compute`. Entries 5, 7, 9, 11 to 16: the golden test of the package and its purity test; `scripts/arch` for the new packages (flat `internal/pages/<package>`, the one level the tool reads). Entry 17: `TestSampleRoutes` is gone and `cmd/ot` tests serve `/home`, `/household`, `/positions`, `/risk`, `/advice` through `httptest` with the demo profile and with no reader (the page is a gap page, status 200). `ci-web` is not triggered by any entry (no path under `web/`). Contracts: unchanged on every entry.

## Left out

- Day profit of positions (needs two marks of consecutive sessions; the mark job has run for days only; the column is a gap, no job added).
- Movers of `household_page` (a day change needs two marks of consecutive sessions, as the day profit of positions; the block is the gap `no_data`).
- Long term block of `household_page` (no holding-period source: executions are collected for the current day only, `account-data.md` Decision 4).
- Beta weighted delta, sigma and standard metrics of `household_risk` (no stored beta, no stored per-stock volatility used by this block, no metric source; the three blocks are the gap `not_built`; the stored IV bars of `iv_history` are not wired here and a design must say how sigma is defined first).
- Advice rows (no store of advice and no job writes it; a recorded-advice store is a new job, which this design does not invent; the page is an honest gap).
- The sharp-move note of held companies and the events of macro (no stored events).
- A pooled household view of the positions page (owner: per account only).
- Removing `web/src/samples`, the `sample_page` schema and `web/fixtures` (a web and contracts cut after issue 17).
- Account B or any second alias: only the main account must work (`account-data.md` Decision 8); `siteAliases` is not changed.
- Rejected alternatives. Extending `PageFigures` with the new methods: the non-embedding test fakes of `household_page` and `report_blocks` stop compiling and are outside the issue's files. Home with only the household member and four gap cards: the owner asked for real data on each page and the other four members have stored sources. Pooling accounts in positions or household: owner decision, per account only. Opening the mark store with the write role line `OT_PG_DSN_POSMARK`: the pages role already has SELECT on the mark view, so the server needs no write role.
- Telegram or any other delivery of these figures (ADR 0010: account figures never reach notify).

## Adversarial

Read by a fresh agent with no context; its answer is `adversarial-pages-real-data.md` of the scratchpad.

1. Gap against the goal (partly): not adopted as a change. Advice, sigma, beta, day profit, long term and events have no stored source; Left out names them and no job is invented (the brief forbids it). Adopted in part: Checked by CI is not a deployment check, so the owner's first look at `/household` needs the mark job to have run; the issue 6 text now says a missing mark opener leaves the blocks as gaps.
2. Biggest unnamed risk (adopted): extending `PageFigures` breaks test fakes outside issue 3. The new methods are now a separate interface `HoldingFigures`; `PageFigures` is not edited.
3. A simpler way (not adopted): home with only the household member. The owner asked for real data on each page, and the other four members have stored sources; merging issues across modules (1 and 6, 9 and 10) is not allowed by one-module issues. Kept the cut.
4. Lock-in (adopted as a note): the unit text and the new interface are the commitment; it is a separate interface, so a replacement implements two small interfaces and `PageFigures` is untouched.
5. Claim most likely false (adopted, corrected in revision 2): `store.MarkReader` does not exist; configs use `mark.Reader`. Revision 1 then named `openMarkReaders`, which opens the option_eod readers and no mark reader; revision 2 names `deps.openMark` with `OT_PG_DSN_PAGES` (Decision 8).
6. Likeliest slip (adopted, corrected in revision 2): issue 5 acceptance and goal now agree (nil path `missing_quote`, movers `no_data`, long term `not_built`). The imports of the home members are decided, not left to the worker: `scripts/arch/main.go` lets a pages sub-package import only roots and declarations-only packages, so the members take plain config and the packages are flat `internal/pages/home_<member>` (the tool reads one level).

## Review round 2 (design review of 3d548ea8)

All eleven points are fixed in revision 2: issue 6 (mark opener and DSN line, Decision 8, with the pages line, which already has the grant on the mark view); issues 12, 13, 15 (no import of an implementation package); issue 5 (one reason per path, movers moved to the gap column and Left out); issue 1 and Decision 7 (the real `siteReaders` fields, demo gives `no_reader`); purity tests in the Files of issues 11 to 16; the fields of the four types in Interface; the `role` text of issue 7; issue 14 (session day); Left out names the rejected alternatives. The merge of origin/main is clean.

## Review round 3 (design review of 90aae1ef)

Fixed in revision 3: Decision 7 limits the demo `no_reader` sentence to household, positions and risk and says what `/advice` and `/home` show; issue 14 and Decision 2 agree on the advice exception; Decision 8, issue 6 and Left out use the pages line (the pages role has SELECT on the mark view); movers is in Left out; `GreekIn.Spot` is dropped.
