# Risk Greeks key: the option quotes are read by the instrument id of the underlying, held long legs are collected, and a JPY stock counts in Delta

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-24
Revision: 1

Source: owner bug 2026-11-24 on the risk page: "13 option structures not counted (missing quotes)", "No option structure counted in Vega", Delta covers stocks only (non-USD stocks excluded). Owner rule of the same day: data comes from standard sources (broker quotes and Greeks, stored collected rows); a quote or Greek is never computed or invented to fill a gap (no model-priced substitute); a missing value stays missing with its reason. Read on origin/main at c5543d94.

Needs the owner: nothing. Open points are under Left out.

## Goal

The risk page shows the Delta and Vega of every held option structure whose legs have a stored broker quote, and a leg with no stored quote stays a gap with its reason. A JPY stock is counted in Delta. The same wrong key is removed from `structures_page` and the `book_risk` job.

## Today

Facts checked on c5543d94 and on the read-only database of 2026-11-24.

- `risk_page`, `structures_page` and the job `bookrisk` read the quotes of a held underlying with `Eod.OptionEod(ctx, conIDs[symbol], ...)`, where `conIDs[symbol]` is the IB `con_id` of the stock row of the newest `account.position` capture (`internal/pages/risk_page/load.go` `quotes`, `internal/pages/structures_page/load.go` line 168, `internal/jobs/bookrisk/load.go` line 225). The store reader filters `obs.option_eod.underlying_id = $1` (`internal/store/snapshot/optioneod.go`), and that column holds the internal `master.instrument.instrument_id`. The IB id is only a `master.instrument_identifier` of scheme `ibkr_conid`.
- Proof: for TSM the capture gives con_id 6223250 and `obs.option_eod` has 0 rows with `underlying_id = 6223250` and 1701 rows with `underlying_id = 2080`, the instrument id of the stock TSM. The same holds for PDD (326398585 against 1819) and every other stock held. So every `OptionEod` call of the three readers answers an empty list.
- Seven held underlyings have an option and no stock row in the capture (CPSH, INTC, LMND, NOK, NVDA, ORCL, WDC), so `conIDs[symbol] == 0` and they are skipped before any read.
- The reader that maps a stock name to its instrument id already exists: `rd.stock_id_r1` through `store/instrument.StockIDs`, used by `positions_page`, `market_state_page` and `report_blocks` with a local `StockIDReader` interface of their own (`internal/pages/positions_page/bars.go`). With it the held underlyings resolve: NVDA 1640, INTC 11293, CPSH 11292, ORCL 11295, LMND 2404, WDC 11297, NOK 11294. `obs.option_eod` of 2026-11-23 has rows for these ids.
- Of the 18 held option legs of alias mine in the capture of 2026-11-23, 11 have `option_eod` rows with a delta and a vega (latest day 2026-11-18 to 2026-11-23) and 7 have none: the CPSH call 7.5, the INTC put 70, both LMND calls 60, the NOK calls 7 and 12 and the ORCL call 145. These are long legs. `heldOptions` in `internal/jobs/optioneod/held.go` keeps only a leg whose quantity is negative (`q.Sign() >= 0` is skipped), so the job collects short legs of a watched underlying and never a long one. `obs.option_open` holds no row of any held leg.
- `internal/compute/pagefig/risk.go` `risk` skips a stock whose currency is not `RiskIn.Currency` and makes the Delta total the gap `unsupported_currency`; `riskInput` in `internal/pages/risk_page/builder.go` leaves every structure not in USD out. `obs.fx_rate` holds USDJPY of 2026-11-24 (159.1905, `ibkr_midpoint`, status ok); `risk_page` reads no rate.
- The Delta of a stock is its share count and the Delta of a structure is the net option delta in shares; neither depends on the currency of the price (a move of one unit of the underlying changes the value by that many units of its own currency). Only Vega, money per volatility point, depends on the currency.

## Decisions

Owner rules this design is written against, each self-checked in `## Adversarial`: (1) standard sources only: a quote or Greek is read from the stored broker rows, never computed or invented; (2) a missing value stays missing with its reason; (3) the smallest fix; (4) the front end computes nothing; (5) files under 600 lines.

1. **The underlying id is the stock instrument id read by name.** A page or job resolves the symbols of the held underlyings with the stock-id reader (`StockIDs`, by name) and passes that id to `OptionEod` and to the bar source. The `con_id` of the capture is no longer used as a key of `option_eod`. A symbol that gives no row, or a name held by two rows, has no quotes and its structures keep the gap `missing_quote`. A nil reader gives no quotes, as a nil `Eod` does today. This reuses the reader that exists; no store change and no new reader.
2. **The three readers get the same fix.** `risk_page`, `structures_page` and `bookrisk` each have the bug, so each is changed; a pages package may not import another, so each declares its small `StockIDReader` interface as `positions_page` does.
3. **A held leg is collected whatever its side.** `heldOptions` stops skipping a leg with a positive quantity, so the job also collects the long leg, with the broker quote and Greeks the way it collects the short one. A leg the broker cannot quote stays without a row and the page shows the gap `missing_quote`. Nothing is estimated.
4. **A stock in JPY is counted in Delta by its share count.** `risk` no longer skips a stock of another currency; the Delta total has no `unsupported_currency` gap for it. A structure in JPY is no longer left out of the input: `GreekIn` gets `Currency` and `RiskIn` gets `UsdJpy`; Delta is added as it is, Vega of a JPY structure is divided by `UsdJpy`. Without a positive `UsdJpy` the Vega of that structure is the gap `unsupported_currency` (the line of its expiry), as `FamilyRisk` does. The rate is the stored `obs.fx_rate` of the day (`LatestFxRate`), never a default.
5. **Wiring is one issue.** `cmd/ot` gives the three consumers the stock-id reader and `risk_page` the FX reader from the readers the site already opens.

## Modules

No change to `docs/design/modules.md`: the rows compute, pages, jobs and ot exist and no depends cell changes (pages already depends on store, compute and contracts; ot imports pages, jobs and store).

| issue | module |
|---|---|
| rgk-compute | compute |
| rgk-risk-page | pages |
| rgk-structures-page | pages |
| rgk-bookrisk | jobs |
| rgk-held-long | jobs |
| rgk-wiring | ot |

## Interface

One interface change: `internal/compute/blockapi` (the interface entry of the compute row). Entry 1 `rgk-compute` is marked (interface): it adds the field `Currency string` to `GreekIn` and the field `UsdJpy string` to `RiskIn`, and rewrites the comment of `HoldingFigures.Risk`. Both new fields are decimal text; the empty value is "no rate" and "USD". The entries that use them (`rgk-risk-page`) come after it. Every other name is read on main as it is: `store.OptionEodReader`, `store.OptionTermsReader`, `bar.FxRateReader`, `bar.PriceBarReader`, `store/instrument.StockIDs`. No migration and no contract file change.

## Issues

Every test calls `t.Parallel()`, writes files only under a temporary directory, never sleeps, opens no port and touches no live broker or database. Acceptance is stated as results readable in the code; test runs are under the check of CI.

### 1 rgk-compute (interface)

Module: compute. Size: S.
Files: `internal/compute/blockapi/api.go`, `internal/compute/blockapi/api_test.go`, `internal/compute/pagefig/risk.go`, `internal/compute/pagefig/risk_test.go`
Design: docs/design/risk-greeks-key.md#1-rgk-compute
- Order: first.
Goal: Decision 4. The owner sees the Delta of a JPY stock and of a JPY structure on the risk page, and the Vega of a JPY structure in dollars.
Acceptance:
1. `GreekIn` has the field `Currency` and `RiskIn` the field `UsdJpy`, both decimal text, and the comment of `Risk` says what they mean.
2. `risk` adds the share count of a stock of any currency to its underlying, and the Delta total has no gap for a stock in another currency.
3. A `GreekIn` with `Currency` "JPY" has its Vega divided by `UsdJpy`; with no positive `UsdJpy` that line of Vega and the Vega total are the gap `unsupported_currency` and its Delta is added unchanged; a `GreekIn` with Currency "" or "USD" is as before.
4. The tests of `risk_test.go` that expected the gap `unsupported_currency` for a stock in JPY are changed to the new result, and a test `TestRiskJpyStockAndStructure` (new) shows a JPY stock in Delta, a JPY structure's Vega divided by the rate and its gap without a rate.
5. No file other than the Files changes.
6. The result matches docs/design/risk-greeks-key.md#1-rgk-compute.

### 2 rgk-risk-page

Module: pages. Size: M.
Files: `internal/pages/risk_page/page.go`, `internal/pages/risk_page/load.go`, `internal/pages/risk_page/builder.go`, `internal/pages/risk_page/strings.go`, `internal/pages/risk_page/builder_test.go`, `internal/pages/risk_page/testdata/ok.golden.json`
Design: docs/design/risk-greeks-key.md#2-rgk-risk-page
- Order: after 1 rgk-compute.
Goal: Decisions 1, 2 and 4 for the risk page. The owner sees the Delta and Vega of every structure whose legs have a stored quote, JPY ones included.
Acceptance:
1. `Config` has the fields `StockIDs` (a local interface `StockIDReader` with `StockIDs(ctx, names []string) ([]store.InstrumentName, error)`) and `Fx` (`bar.FxRateReader`); a nil `StockIDs` gives no quotes, as a nil `Eod`.
2. `quotes` in `load.go` resolves the symbols of the open positions by name through `StockIDs` and passes that id, not the `con_id` of the capture, to `OptionEod` and to the bar source; a symbol with no row or with two rows has no quotes. The map `conIDs` and its argument are removed.
3. `riskInput` in `builder.go` no longer leaves a structure in another currency out: it fills the currency of each greek input, and the USDJPY field of the risk input is the newest stored rate of the day when found and empty when not; the note `textForeign` and its gap no longer appear.
4. A structure whose Greek is a gap stays out of Delta and Vega and counted in the note, as before; no value is computed from a bid, an ask or a model.
5. The tests of `builder_test.go` that expected a foreign structure to be left out are changed, and `TestQuotesUseStockInstrumentID` (new) shows a quote stored under the instrument id of the stock is read while one stored under the con_id is not; `ok.golden.json` is regenerated to the new page.
6. `load.go` and `builder.go` stay under 600 lines.
7. The result matches docs/design/risk-greeks-key.md#2-rgk-risk-page.

### 3 rgk-structures-page

Module: pages. Size: S.
Files: `internal/pages/structures_page/page.go`, `internal/pages/structures_page/load.go`, `internal/pages/structures_page/load_test.go`, `internal/pages/structures_page/testdata`
Design: docs/design/risk-greeks-key.md#3-rgk-structures-page
- Order: first.
Goal: Decisions 1 and 2 for the structures page. The owner sees the quotes and Greeks of his structures there.
Acceptance:
1. `Config` has the field `StockIDs` (local interface `StockIDReader`, as in risk_page); a nil value gives no quotes.
2. The read of `option_eod` in `load.go` resolves the underlying symbols by name and passes that id; the `con_id` of the stock row is no longer used as a key, and a symbol with no or two rows has no quotes.
3. `TestStructuresQuotesUseStockInstrumentID` (new) shows a quote stored under the stock instrument id is read; the other tests and the golden files in `testdata` hold, or change only where a quote now appears.
4. The result matches docs/design/risk-greeks-key.md#3-rgk-structures-page.

### 4 rgk-bookrisk

Module: jobs. Size: S.
Files: `internal/jobs/bookrisk/job.go`, `internal/jobs/bookrisk/load.go`, `internal/jobs/bookrisk/load_test.go`
Design: docs/design/risk-greeks-key.md#4-rgk-bookrisk
- Order: first.
Goal: Decisions 1 and 2 for the job `book_risk`. The stored book risk has the Greeks of the held structures.
Acceptance:
1. `Stores` has the field `StockIDs` (local interface `StockIDReader`); a nil value gives no quotes.
2. `quotes` in `load.go` resolves the underlying symbols by name and passes that id to `OptionEod`; `conIDs` is no longer a key of it.
3. `TestBookRiskQuotesUseStockInstrumentID` (new) shows a quote stored under the stock instrument id is read; the other tests hold.
4. The result matches docs/design/risk-greeks-key.md#4-rgk-bookrisk.

### 5 rgk-held-long

Module: jobs. Size: S.
Files: `internal/jobs/optioneod/held.go`, `internal/jobs/optioneod/held_test.go`, `internal/jobs/optioneod/held_aliases_test.go`
Design: docs/design/risk-greeks-key.md#5-rgk-held-long
- Order: first.
Goal: Decision 3. The next run collects the broker quote and Greeks of the held long legs, so their structures are counted.
Acceptance:
1. `heldOptions` in `held.go` keeps a held option leg of positive quantity as well as negative; a leg of quantity zero or unreadable stays skipped; its comment and the comment of `NewWithHeld` say "every held option".
2. `TestHeldOptionsKeepsLongLeg` (new) shows a long call and a long put of a watched underlying in the list; the existing tests of `held_test.go` and `held_aliases_test.go` that expected a long leg to be left out are changed to the new result.
3. No quote is estimated: a leg the broker answers without a quote has no row, as before.
4. The result matches docs/design/risk-greeks-key.md#5-rgk-held-long.

### 6 rgk-wiring

Module: ot. Size: S.
Files: `cmd/ot/risk_site.go`, `cmd/ot/risk_site_test.go`, `cmd/ot/site_positions.go`, `cmd/ot/site_positions_test.go`, `cmd/ot/bookrisk.go`, `cmd/ot/bookrisk_test.go`
Design: docs/design/risk-greeks-key.md#6-rgk-wiring
- Order: after 2 rgk-risk-page, 3 rgk-structures-page and 4 rgk-bookrisk.
Goal: Decision 5. After deploy the risk page shows the Greeks of the held structures.
Acceptance:
1. `riskConfig` sets `StockIDs` and `Fx` from the readers of the site (the stock-id reader and the FX reader the positions and option data pages already open).
2. The structures page config of `site_positions.go` sets `StockIDs`, and the `bookrisk.Stores` of `bookrisk.go` sets `StockIDs`.
3. `TestRiskConfigHasStockIDsAndFx` (new) shows the config of the risk page has both readers non-nil in the live profile.
4. The result matches docs/design/risk-greeks-key.md#6-rgk-wiring.

## Checked by CI

- `ci-go`: `go test` of the packages of the entries, `golangci-lint` and `scripts/arch` pass.
- After the deploy and the next `option_eod` run the window reads the risk page (`/risk`) for the account mine and the rows of `obs.option_eod` of the held legs: the note "not counted (missing quotes)" lists only the legs the broker did not quote, Vega has rows, Delta has the JPY stocks.

## Adversarial

- Rule 1 (standard sources only): no entry computes a quote or Greek; entry 2 and 3 read stored rows, entry 5 only widens which contracts the broker is asked about, entry 1 divides a stored Vega by a stored FX rate.
- Rule 2 (missing stays missing): a name with no stock row, a leg without a stored row and a missing rate each keep their gap and reason.
- Rule 3 (smallest fix): the existing `StockIDs` reader is used; no store, migration or contract change.
- Rule 4: the page JSON carries finished figures; `web/` is not touched.
- The unit of Delta: shares are currency-independent, so no rate is needed for Delta; this is stated in Today and not a new rule.

## Left out

- A Delta in dollars of price (shares times price) is not built; the page keeps the share-count Delta it has.
- A held underlying that is not in the watch list of `option_eod` still has no quote for its legs; adding it to the watch list is the owner's choice.
- The option_open rows (`obs.option_open`) of held legs are not read by these pages.
