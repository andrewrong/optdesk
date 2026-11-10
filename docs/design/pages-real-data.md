# Pages real data: home, household, positions, risk and advice read stored data and show a gap where there is none

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-10
Revision: 1

Source: owner 2026-11-10: "前段几乎没什么真实数据" (the front end has almost no real data). Read on origin/main at 3c91d89d: `AGENTS.md`, `docs/design/modules.md`, `account-data.md` (Decisions 1, 4, 9, 10), `position-marks.md`, `option-positions.md` (Left out: `positions_page` stays, no pooled household view, per account only), `option-page-builders.md`, `pages-registry.md`, `ot-wire-split.md`, `unified-site.md`, `home-page.md`, `web-shell-layout.md` (format), `internal/pages/api.go`, `internal/pages/household_page/*`, `internal/pages/home_page/*`, `internal/pages/structures_page/config`, `internal/compute/blockapi/api.go`, `internal/store/api.go` (`AccountReader`, `Position`, `AccountValue`, `Execution`), `internal/store/mark/api.go`, `cmd/ot/{sample,site,pagereg,companies_site,site_positions}.go`, `contracts/schemas/page.{home_page.v3,household_page.v1,positions_page.v3,household_risk.v1,advice_page.v1,*_summary}.json`, `web/fixtures/*`.

Goal: the owner opens `/home`, `/household`, `/positions`, `/risk` and `/advice` of `ot serve` and every number on them is a stored value built by Go, or a dash with its reason. No page shows a sample number. The web renders what the payload holds and computes nothing; no schema is edited.

## What main holds today (read at 3c91d89d)

- `cmd/ot/sample.go` is 24 lines: `sampleSchemas` maps `home`, `household`, `positions`, `risk`, `advice` to a schema name and `sampleVM` returns `{"schema":"sample_page","schema_version":1,"page":"<schema>"}`; the web bundle then shows `web/fixtures/*/ok.json` under the banner of `unified-site.md` Decision 4. `cmd/ot/site.go` reaches `sampleVM` in the `default` case, after the registered families (`extraPageBuilder`, `pagereg.go`), so a route registered with `registerPage` is never sampled. Verified: the five routes are the only entries.
- Stored and read: `store.AccountReader` (`Positions`, `Values`, `Executions`, views `rd.account_*_r1`, `account-data.md` Decision 10) and its captures; `store.mark.Reader.Marks(alias, day)` (one mark per open position, `position-marks.md`); `store.OptionLedgerReader`; `store.SessionDayReader`; `store.OptionEodReader`; the series and reading-view readers. `siteReaders` already opens `account` and the ledger (`openOptionalStores`), used by the structures pages.
- Builders: `household_page.Build` (internal/pages/household_page) is written and tested and called by no one in `cmd/ot`; it builds overview, cash after, stress, obligations, income, month change, trend and accounts from the daily capture and the replayed ledger, and writes composition, concentration, long term, allocation and movers as the gap `missing_quote`. `home_page.Compose` composes five member documents and nothing builds any member (`home-page.md` Limits). There is no builder for `positions_page`, `household_risk` or `advice_page`.
- Compute: `blockapi.PageFigures` has `Household`, `Trades`, `AccountMove`, `Moves`, `Glance`, `Changes`; `trades` classifies executions as open, close, expire or exercise from the prior quantity. `compute.Structure.Greeks` (`GreekSet`) holds the net greeks of an option structure from `option_eod`. No figure of a held stock's value share, of unrealised profit, of delta by underlying or of vega by expiry exists.
- Page registry: a page is one `registerPage(pageFamily{name, route, order, build, named})` in its own `cmd/ot/<page>_site.go` (`companies_site.go` is the pattern); the builder lives in `internal/pages/<page>`; the readers come from `siteReaders` or from `a.deps`.

## Decisions

1. Same pattern as `/options` and `/companies`: a builder in its own package of `internal/pages` (`func Build(ctx, Config, ...)`, no clock, no store write, no figure computed in it), a registered family in `cmd/ot`, and `ot page --name <page>` for the same builder. A sample route stops being served the moment its family is registered (`site.go` order, above); `sample.go` is cleaned in the last issue, not in each.
2. Which day and which account. The day is the day of the request (`--day` or the clock), as in every builder. Each page shows one account, chosen by the query `?account=<alias>` (the key `siteAccountQuery` of `site_positions.go`), default the first alias of `siteAliases`; accounts are never pooled (owner 2026-11-08/09, `option-positions.md`). The capture used is the newest capture on or before the day within `staleSessions` sessions (the rule `household_page` already has); an older capture gives the page gap `stale_input`, none gives `no_data`, a nil reader gives `no_reader`. A gap page keeps its title and the envelope and has no row. The mark of a position is the one of that capture (`position-marks.md`).
3. A missing value is a gap, never a zero and never a sample: a figure that needs a price the mark row lacks (`Basis` none, empty `Price`) is `missing_quote`; a block whose source no job stores is `not_built` and its text says what is missing. The web already shows a gap as a dash with the reason (`web-shell-layout.md` Decision 2).
4. Compute stays in `compute`. The household blocks that need a mark, the positions figures and the risk figures are new functions of `blockapi.PageFigures` (one interface issue, then the implementation), so `pages` builds JSON and formats nothing it did not receive (`household_page/purity_test.go` is the model and is copied for each new package).
5. Per-page sources and blocks (the table is the contract of the issues):

| page | schema | filled from stored data | stays a gap (Left out) |
|---|---|---|---|
| household | `household_page` v1 | overview, cash after, stress, obligations, income, month change, trend, accounts: account capture values and the ledger (already built). Composition, allocation, concentration, movers: positions with marks (after the figures issue) | long term (no holding-period source) |
| positions | `positions_page` v3 | per account view: stocks and options rows (code, name, quantity, cost from `AvgCost`, price from the mark, unrealised); `trades` from `Executions` classified by `Trades`; options role and remaining premium from the replayed ledger | day profit (needs two marks; gap until the previous session has one); the pooled view `household` |
| risk | `household_risk` v1 | delta by underlying (shares of stock plus option net delta, valued at the mark) and vega by expiry, from positions and `Structure.Greeks` | beta weighted delta (no beta source), sigma, standard metrics (no stored source) |
| advice | `advice_page` v1 | nothing: no store holds yesterday's advice | all rows: the page is the schema's `gap` with no row |
| home | `home_page` v3 | the five members, each built from the stored data of its own page: household tiles (capture net liquidation, cash, day and month change), held companies (reading view, as `companies_page`), macro (series), market (as `market_page`), options (option ledger premium series) | any tile or series of a member with no source is that member's own gap |

6. Home is composed of members that are built by their own builders; a member whose source is absent is a card with its `gaps` filled and no tile (the required `gaps` and `notes` of `page.*_summary` carry it). A member builder never calls another page's builder; it reads the stores and uses the same figure functions, so no page waits for another to be correct.
7. No new job, no new table, no new dataset, no schema or generated type is edited (`Contracts: unchanged` on every entry). The web is untouched: its views and fixtures exist; the sample banner is a shell literal that shows only for a `sample_page` payload.

## Modules

Edited rows: `ot` (`cmd/**`), `pages` (`internal/pages/**`), `compute` (`internal/compute/**`). `docs/design/modules.md` is not edited (every new package is under `internal/pages/**` or `cmd/**`). An issue edits only the files of its Files line. A needed file outside it is reported in the pull request.

## Interface

`internal/compute/blockapi/api.go` (compute row, entry `blockapi`) gains, in issue 3, three methods of `PageFigures` with their input and output types: `Holdings(HoldingsIn) HoldingsResult` (per position value, share of the account, unrealised profit and the totals; every figure a `compute.FigureValue`), `Risk(RiskIn) RiskResult` (delta by underlying, vega by expiry) and nothing else. Existing methods and types are unchanged, so `household_page` and the report blocks compile as they are. `internal/pages/api.go` is not edited.

## Issues

Tests use no clock, no sleep, no file at a fixed place and no port; a builder test reads a golden file under its own `testdata` and a fake reader. Acceptance items are decided by reading, grep or git; each prints one number. The commands CI runs are under `Checked by CI`. `BASE` is `git merge-base HEAD origin/main`. A wiring issue lists `cmd/ot/sample.go` nowhere: it is not touched until issue 17.

### 1 ot-household-route

Module: ot. Size: M.
Files: `cmd/ot/household_site.go` (new), `cmd/ot/household_site_test.go` (new)
Design: docs/design/pages-real-data.md#Decisions
Order: first.
Goal: `/household` and `ot page --name household_page` serve `household_page.Build` of the account named by `?account=`, with `Account`, `Ledger`, `Sessions` from `siteReaders` and the constructors `Ledgers` and `Figures` the structures pages already get from `withPositionsRules` (the demo profile gets the demo readers and a payload with the demo account). The family is registered with `route: "household"`, `name: "household_page"`, an `order` not used by another family. Decision 2 gaps hold: with no reader the page is the gap page `no_reader`.
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
Goal: the methods and types of the Interface section, declarations only; `api_test.go` gains a fake that implements the new `PageFigures` and a caller that uses `Holdings` and `Risk` (the contract tested from both sides). Input types carry decimal text: `HoldingIn{Underlying, SecType, Currency, Quantity, Multiplier, Cost, Mark string}`, `GreekIn{Underlying string, Expiry contracts.Day, Delta, Vega string}`; `Mark` is empty when the position has no price, which gives the gap `missing_quote` in the result and never a zero. The note of the type says the unit of delta (dollars per one dollar move of the underlying) and of vega (dollars per volatility point), the text the web shows as `meaning`.
Acceptance:
1. `grep -c 'Holdings(\|Risk(' internal/compute/blockapi/api.go` prints at least 2.
2. `git diff $(git merge-base HEAD origin/main) -- internal/compute/blockapi/api.go | grep -c '^-[^-]'` prints 0 (additive).
3. `grep -c 'Holdings\|Risk' internal/compute/blockapi/api_test.go` prints at least 2 and `grep -c 't.Parallel()' internal/compute/blockapi/api_test.go` prints at least 1.
4. `grep -c 'Holdings\|Risk' docs/contracts/compute.md` prints at least 2.
5. `git diff --name-only $(git merge-base HEAD origin/main) | grep -v -e '^internal/compute/blockapi/api.go$' -e '^internal/compute/blockapi/api_test.go$' -e '^docs/contracts/compute.md$' | wc -l` prints 0.

### 4 pagefig-holdings-risk

Module: compute. Size: M.
Files: `internal/compute/pagefig/holdings.go` (new), `internal/compute/pagefig/holdings_test.go` (new), `internal/compute/pagefig/risk.go` (new), `internal/compute/pagefig/risk_test.go` (new), `internal/compute/pagefig/pagefig.go`
Design: docs/design/pages-real-data.md#Decisions
Order: after 3.
Goal: `pagefig.New()` implements `Holdings` and `Risk`: value is quantity times mark times multiplier, unrealised is value minus quantity times cost times multiplier, share is value over the account total of the same currency (a currency pair with no rate stays two blocks, `unsupported_currency`, as `Household` does), stock delta is the quantity, option delta and vega are summed from the `GreekIn` rows by underlying and by expiry. A position with an empty mark gives `missing_quote` for its value, unrealised and share and is left out of every total, the totals then carry the gap `missing_quote` (a total that omitted a position is never shown as complete). Decimal arithmetic as the existing `household.go` does (the library it uses, no float).
Acceptance:
1. `grep -c 'func (.*) Holdings(\|func (.*) Risk(' internal/compute/pagefig/*.go | awk -F: '{s+=$2} END {print s}'` prints 2.
2. `grep -c 'missing_quote' internal/compute/pagefig/holdings.go` prints at least 2.
3. `grep -c 'float64' internal/compute/pagefig/holdings.go internal/compute/pagefig/risk.go | awk -F: '{s+=$2} END {print s}'` prints 0.
4. `grep -c 't.Parallel()' internal/compute/pagefig/holdings_test.go internal/compute/pagefig/risk_test.go | awk -F: '{s+=$2} END {print s}'` prints at least 2.
5. `git diff --name-only $(git merge-base HEAD origin/main) | grep -v '^internal/compute/pagefig/' | wc -l` prints 0.

### 5 household-holdings-blocks

Module: pages. Size: M.
Files: `internal/pages/household_page/holdings.go` (new), `internal/pages/household_page/holdings_test.go` (new), `internal/pages/household_page/builder.go`, `internal/pages/household_page/page.go`, `internal/pages/household_page/testdata/holdings.golden.json` (new)
Design: docs/design/pages-real-data.md#Decisions
Order: after 4.
Goal: `household_page.Config` gains `Marks store.MarkReader`; with it, composition, allocation, concentration and movers are built from `Figures().Holdings` of the capture's positions and marks and no longer the constant gap; without it (nil) they stay `missing_quote` as today. `long_term` stays the gap `not_built`. `Positions`, `Values` and marks are read for the capture day only. The page `run_status` is `ok` when every block is built, else `partial` as today.
Acceptance:
1. `grep -c 'Marks' internal/pages/household_page/page.go` prints at least 1 and `grep -c 'Holdings(' internal/pages/household_page/holdings.go` prints at least 1.
2. `grep -c 'missingQuote()' internal/pages/household_page/builder.go` prints at most 2 (long term and the nil-reader path).
3. `grep -c 'float64\|math\.' internal/pages/household_page/holdings.go` prints 0.
4. `git diff --name-only $(git merge-base HEAD origin/main) | grep -v '^internal/pages/household_page/' | wc -l` prints 0.

### 6 ot-household-marks

Module: ot. Size: S.
Files: `cmd/ot/household_site.go`, `cmd/ot/household_site_test.go`, `cmd/ot/site.go`
Design: docs/design/pages-real-data.md#Decisions
Order: after 1 and 5.
Goal: the mark reader (`store/mark`, role of `OT_PG_DSN_PAGES` through the view, as the account reader) is opened once with the site readers and set as `Config.Marks`; with no opener the field is nil and the blocks stay gaps.
Acceptance:
1. `grep -c 'Marks' cmd/ot/household_site.go` prints at least 1.
2. `grep -c 'mark\.' cmd/ot/site.go` prints at least 1.
3. `git diff --name-only $(git merge-base HEAD origin/main) | grep -v -e '^cmd/ot/household_site.go$' -e '^cmd/ot/household_site_test.go$' -e '^cmd/ot/site.go$' | wc -l` prints 0.

### 7 positions-page-builder

Module: pages. Size: L.
Files: `internal/pages/positions_page/page.go` (new), `internal/pages/positions_page/builder.go` (new), `internal/pages/positions_page/rows.go` (new), `internal/pages/positions_page/trades.go` (new), `internal/pages/positions_page/strings.go` (new), `internal/pages/positions_page/builder_test.go` (new), `internal/pages/positions_page/purity_test.go` (new), `internal/pages/positions_page/testdata/ok.golden.json` (new)
Design: docs/design/pages-real-data.md#Decisions
Order: after 4.
Goal: `positions_page.NewPage(Config, alias)` builds `page.positions_page.v3`: one view per configured alias only, `key` the alias, no pooled view. The `Config` is `RunID, BuildID, Zone, Aliases, Account store.AccountReader, Marks store.MarkReader, Ledger store.OptionLedgerReader, Sessions store.SessionDayReader, Ledgers compute.NewOptionLedgerFunc, Figures blockapi.NewPageFiguresFunc`. Stock rows: code, name, quantity, cost (`AvgCost`), price (mark), unrealised from `Holdings`; option rows add expiry, days left (from the day, as the structures page does), role and remaining premium from the replayed ledger (a position with no ledger match has the role cell as a gap). The day profit column is a gap in every row. `trades` is the executions of the last 30 session days through `Figures().Trades`. The column ids are those of `web/fixtures/positions-page/ok.json`. A nil reader gives the page gap `no_reader`; no capture gives `no_data`.
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
Goal: `/positions` and `ot page --name positions_page` serve issue 7's builder with the readers of issue 6.
Acceptance:
1. `grep -c 'registerPage' cmd/ot/positions_site.go` prints 1 and `grep -c 'positions_page\.' cmd/ot/positions_site.go` prints at least 1.
2. `grep -c '"positions"' cmd/ot/positions_site.go` prints at least 1.
3. `git diff --name-only $(git merge-base HEAD origin/main) | grep -v -e '^cmd/ot/positions_site.go$' -e '^cmd/ot/positions_site_test.go$' | wc -l` prints 0.

### 9 risk-page-builder

Module: pages. Size: M.
Files: `internal/pages/risk_page/page.go` (new), `internal/pages/risk_page/builder.go` (new), `internal/pages/risk_page/strings.go` (new), `internal/pages/risk_page/builder_test.go` (new), `internal/pages/risk_page/purity_test.go` (new), `internal/pages/risk_page/testdata/ok.golden.json` (new)
Design: docs/design/pages-real-data.md#Decisions
Order: after 4.
Goal: `risk_page.NewPage(Config, alias)` builds `page.household_risk.v1`: the `delta` block (rows per underlying) and the `vega` block (rows per expiry) from `Figures().Risk` over the capture's positions, the marks and the `Greeks` of the structures (`compute.NewOptionStructuresFunc`, as `structures_page` loads them). The `beta_delta`, `sigma` and `standard_metrics` blocks are the schema's gap with reason `not_built` and a text naming the missing source; their rows and metrics are empty. `run_status` is `partial`. The `Config` mirrors issue 7 plus `Eod`, `Terms` and `Structures` as `structures_page.Config`.
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
Goal: `/risk` and `ot page --name household_risk` serve issue 9's builder.
Acceptance:
1. `grep -c 'registerPage' cmd/ot/risk_site.go` prints 1 and `grep -c 'risk_page\.' cmd/ot/risk_site.go` prints at least 1.
2. `grep -c '"risk"' cmd/ot/risk_site.go` prints at least 1.
3. `git diff --name-only $(git merge-base HEAD origin/main) | grep -v -e '^cmd/ot/risk_site.go$' -e '^cmd/ot/risk_site_test.go$' | wc -l` prints 0.

### 11 home-member-household

Module: pages. Size: M.
Files: `internal/pages/home_members/household/member.go` (new), `internal/pages/home_members/household/member_test.go` (new), `internal/pages/home_members/household/testdata/ok.golden.json` (new)
Design: docs/design/pages-real-data.md#Decisions
Order: after 3.
Goal: `household.Build(ctx, Config, day, at)` returns `page.household_summary.v2`: tiles net worth, cash, day change and month to date of the main account from the capture values (`Figures().Household` as `household_page`), each a gap when its input capture is missing. Member identity (`run_id`, `build_id`, `scope`) is the `Config`'s, which the composer checks.
Acceptance:
1. `grep -c 'household_summary' internal/pages/home_members/household/member.go` prints at least 1.
2. `grep -c 'Household(' internal/pages/home_members/household/member.go` prints at least 1.
3. `grep -c 'float64\|math\.\|time\.Now' internal/pages/home_members/household/member.go | awk '{print $1}'` prints 0.
4. `git diff --name-only $(git merge-base HEAD origin/main) | grep -v '^internal/pages/home_members/household/' | wc -l` prints 0.

### 12 home-member-companies

Module: pages. Size: M.
Files: `internal/pages/home_members/companies/member.go` (new), `internal/pages/home_members/companies/member_test.go` (new), `internal/pages/home_members/companies/testdata/ok.golden.json` (new)
Design: docs/design/pages-real-data.md#Decisions
Order: first.
Goal: `companies.Build` returns `page.held_companies_summary.v3`: one row per held company with the price, the band and the hand price from the same reading view and subjects as `companies_page` (it imports `companies_page`'s exported `Subject` and `Config`, not its output); the sharp-move note is a gap while `sharpmove` readings are absent.
Acceptance:
1. `grep -c 'held_companies_summary' internal/pages/home_members/companies/member.go` prints at least 1.
2. `grep -c 'companies_page\.' internal/pages/home_members/companies/member.go` prints at least 1.
3. `git diff --name-only $(git merge-base HEAD origin/main) | grep -v '^internal/pages/home_members/companies/' | wc -l` prints 0.

### 13 home-member-macro

Module: pages. Size: M.
Files: `internal/pages/home_members/macro/member.go` (new), `internal/pages/home_members/macro/member_test.go` (new), `internal/pages/home_members/macro/testdata/ok.golden.json` (new)
Design: docs/design/pages-real-data.md#Decisions
Order: first.
Goal: `macro.Build` returns `page.macro_summary.v2` from `store.SeriesReader` and the `ease` figure of `internal/compute/ease` as `macro_page` reads them; `events` is empty with a gap `not_built` (no event source is stored).
Acceptance:
1. `grep -c 'macro_summary' internal/pages/home_members/macro/member.go` prints at least 1.
2. `grep -c 'not_built' internal/pages/home_members/macro/member.go` prints at least 1.
3. `git diff --name-only $(git merge-base HEAD origin/main) | grep -v '^internal/pages/home_members/macro/' | wc -l` prints 0.

### 14 advice-page-builder

Module: pages. Size: S.
Files: `internal/pages/advice_page/page.go` (new), `internal/pages/advice_page/builder.go` (new), `internal/pages/advice_page/builder_test.go` (new), `internal/pages/advice_page/testdata/gap.golden.json` (new)
Design: docs/design/pages-real-data.md#Decisions
Order: first.
Goal: `advice_page.NewPage(Config)` builds `page.advice_page.v1` with `rows` empty and `gap` set to reason `not_built` and the text "no record of earlier advice is stored yet"; `day` is the day before the request's day.
Acceptance:
1. `grep -c 'advice_page' internal/pages/advice_page/page.go` prints at least 1.
2. `grep -c 'not_built' internal/pages/advice_page/builder.go` prints at least 1.
3. `git diff --name-only $(git merge-base HEAD origin/main) | grep -v '^internal/pages/advice_page/' | wc -l` prints 0.

### 15 home-member-market

Module: pages. Size: M.
Files: `internal/pages/home_members/market/member.go` (new), `internal/pages/home_members/market/member_test.go` (new), `internal/pages/home_members/market/testdata/ok.golden.json` (new)
Design: docs/design/pages-real-data.md#Decisions
Order: first.
Goal: `market.Build` returns `page.market_summary.v3` (the index chart, the index table and the volatility indices) from the same readers and figures as `market_page` (`internal/pages/market_page`'s exported config), a gap per index with no series.
Acceptance:
1. `grep -c 'market_summary' internal/pages/home_members/market/member.go` prints at least 1.
2. `grep -c 'market_page\.' internal/pages/home_members/market/member.go` prints at least 1.
3. `git diff --name-only $(git merge-base HEAD origin/main) | grep -v '^internal/pages/home_members/market/' | wc -l` prints 0.

### 16 home-member-options

Module: pages. Size: M.
Files: `internal/pages/home_members/options/member.go` (new), `internal/pages/home_members/options/member_test.go` (new), `internal/pages/home_members/options/testdata/ok.golden.json` (new)
Design: docs/design/pages-real-data.md#Decisions
Order: first.
Goal: `options.Build` returns `page.options_summary.v4`: tiles and the six month premium series from the replayed option ledger (`store.OptionLedgerReader` and `compute.NewOptionLedgerFunc`, as `option_tracking_page`); a month with no ledger row is a hole, not zero; the closed series text is built only from ledger rows.
Acceptance:
1. `grep -c 'options_summary' internal/pages/home_members/options/member.go` prints at least 1.
2. `grep -c 'OptionLedgerReader' internal/pages/home_members/options/member.go` prints at least 1.
3. `git diff --name-only $(git merge-base HEAD origin/main) | grep -v '^internal/pages/home_members/options/' | wc -l` prints 0.

### 17 ot-home-route

Module: ot. Size: M.
Files: `cmd/ot/home_site.go` (new), `cmd/ot/home_site_test.go` (new), `cmd/ot/sample.go`, `cmd/ot/sample_test.go`, `cmd/ot/site.go`
Design: docs/design/pages-real-data.md#Decisions
Order: after 2, 8, 10, 11, 12, 13, 15 and 16 (the last issue of the cut).
Goal: `/home` and `ot page --name home_page` call the five member builders and `home_page.Compose` (identity and texts as `home-page.md` Decision 6; `gap_line` is "No gap today" or the count of members with a gap, from `Compose`'s overview). `sample.go`, `sample_test.go` and the `sampleVM` branch of `site.go` are deleted, `/` stays the redirect to `/home`; the constant `sampleHome` that the redirect uses becomes `siteHomeRoute` in `home_site.go`. The web samples (`web/src/samples`) stay for a later cut.
Acceptance:
1. `grep -c 'registerPage' cmd/ot/home_site.go` prints 1 and `grep -c 'home_page\.Compose' cmd/ot/home_site.go` prints 1.
2. `grep -c 'home_members/' cmd/ot/home_site.go` prints at least 5.
3. `git ls-files cmd/ot/sample.go cmd/ot/sample_test.go | wc -l` prints 0 and `grep -rc 'sampleVM\|sampleSchemas' cmd/ot/site.go | awk '{print $1}'` prints 0.
4. `git diff --name-only $(git merge-base HEAD origin/main) | grep -v -e '^cmd/ot/home_site.go$' -e '^cmd/ot/home_site_test.go$' -e '^cmd/ot/sample.go$' -e '^cmd/ot/sample_test.go$' -e '^cmd/ot/site.go$' | wc -l` prints 0.

## Checked by CI

ci-go for every entry: `go build ./...`, `go vet`, `golangci-lint`, `scripts/arch`, `go test ./... -race` (parallel tests, `t.TempDir()`, no sleep). Entries 3 and 4 also run the contract tests of `blockapi` from both sides and the golden suites of `internal/compute`. Entries 5, 7, 9, 11 to 16: the golden test of the package and its purity test. Entry 17: `TestSampleRoutes` is gone and `cmd/ot` tests serve `/home`, `/household`, `/positions`, `/risk`, `/advice` through `httptest` with the demo profile and with no reader (the page is a gap page, status 200). `ci-web` is not triggered by any entry (no path under `web/`). Contracts: unchanged on every entry.

## Left out

- Day profit of positions (needs two marks of consecutive sessions; the mark job has run for days only; the column is a gap, no job added).
- Long term block of `household_page` (no holding-period source: executions are collected for the current day only, `account-data.md` Decision 4).
- Beta weighted delta, sigma and standard metrics of `household_risk` (no stored beta, no stored per-stock volatility used by this block, no metric source; the three blocks are the gap `not_built`; the stored IV bars of `iv_history` are not wired here and a design must say how sigma is defined first).
- Advice rows (no store of advice and no job writes it; a recorded-advice store is a new job, which this design does not invent; the page is an honest gap).
- The sharp-move note of held companies and the events of macro (no stored events).
- A pooled household view of the positions page (owner: per account only).
- Removing `web/src/samples`, the `sample_page` schema and `web/fixtures` (a web and contracts cut after issue 17).
- Account B or any second alias: only the main account must work (`account-data.md` Decision 8); `siteAliases` is not changed.
- Telegram or any other delivery of these figures (ADR 0010: account figures never reach notify).
