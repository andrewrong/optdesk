# Risk Delta per currency: the Delta total is one total for each currency, never a sum of dollars and yen

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-25
Revision: 1

Source: owner bug 2026-11-25 on the risk page: the Delta total, labelled "per $1 move", adds the share Delta of JPY names (a move of 1 JPY) to that of USD names. Dispatcher decision: one total for each currency ("per $1" for USD names, "per ¥1" for JPY names); no FX conversion. Read on origin/main at aa7a8c8c.

Needs the owner: nothing. Owner rule: gaps stay gaps with their reason.

## Goal

The risk page shows the Delta of a USD name as "per $1 move" and that of a JPY name as "per ¥1 move", and the Delta total as one figure for each currency in one cell, for example "+109.50 美元/每 $1；+3200.00 日元/每 ¥1". Nothing is converted with a rate, and a name of another currency is a gap with its reason.

## Today

Facts read on aa7a8c8c.

- `risk` in `internal/compute/pagefig/risk.go` adds the share count of every stock and the Delta of every structure into one bucket for each underlying, and `lines` adds all buckets into one `DeltaTotal`. The comments of `RiskIn` and `RiskResult` in `internal/compute/blockapi/api.go` say a stock of any currency counts; that is true for a row and false for the total.
- `riskInput` in `internal/pages/risk_page/builder.go` already hands the currency of each stock (`HoldingIn.Currency`) and of each structure (`GreekIn.Currency`; "" or "USD" is USD).
- `deltaCell` in `builder.go` writes the unit text `unitDelta` (" USD per $1 move", in `strings.go`) and `Currency: currency` ("USD") on every row and on the total, so a JPY row is labelled USD.
- The schema `contracts/schemas/page.household_risk.v1.json` has one `total` (a MoneyCell: value, text up to 200 characters, unit `usd_signed`, currency `USD` or `JPY`) and needs no change: a one-cell text total fits it.

## Decisions

1. A Delta line has a currency: the currency of its stock, or of its structures; "" is USD. A name whose stock and structures differ in currency is the gap `missing_input` for that line.
2. The Delta total is one total for each currency, in the order USD then JPY. A currency other than USD and JPY makes its lines the gap `unsupported_currency` and adds nothing to any total.
3. The total cell of the page carries the text of all totals, joined by "；" (one total for a single currency). Its `value` and `currency` are those of the first total. A line that is a gap is never dropped: the block carries `gap` with the reason of the first gap line (in line order) alongside `total`, and `total` is absent only when no currency has a counted line.
4. The unit text of a row follows the currency of the row: "美元/每 $1" for USD and "日元/每 ¥1" for JPY. The texts live in `zh_texts.go`.
5. No rate is read for Delta; the Vega rule (a JPY Vega divided by USDJPY) is unchanged.

## Modules

No change to `docs/design/modules.md`: the rows compute and pages exist, no depends cell changes (pages already depends on compute).

| issue | module | order |
|---|---|---|
| 1 rdc-compute | compute | first |
| 2 rdc-pages | pages | after 1 |

## Interface

One interface change: `internal/compute/blockapi` (the interface entry of the compute row). Entry 1 `rdc-compute` is marked (interface): it adds the field `Currency string` to `RiskLine` (the currency of a Delta line; empty on a Vega line), adds the type `RiskTotal` with the fields `Currency string` and `Value compute.FigureValue`, replaces the field `DeltaTotal` of `RiskResult` by `DeltaTotals []RiskTotal`, and corrects the comments of `RiskIn` and `RiskResult`. `rdc-pages` comes after it. No migration and no contract file change.

## Issues

Every test calls `t.Parallel()`, writes files only under a temporary directory, never sleeps, opens no port and touches no live broker or database. Acceptance is stated as results readable in the code; test runs are under the check of CI.

### 1 rdc-compute (interface)

Module: compute. Size: S.
Files: `internal/compute/blockapi/api.go`, `internal/compute/blockapi/api_test.go`, `internal/compute/pagefig/risk.go`, `internal/compute/pagefig/risk_test.go`
Design: docs/design/risk-delta-per-currency.md#1-rdc-compute
- Order: first.
Goal: Decisions 1, 2 and 5. The Delta total is no longer a sum over currencies.
Acceptance:
1. `RiskLine` has the field `Currency`, `RiskTotal` is declared in `api.go` with the fields `Currency` and `Value`, and `RiskResult` has the new field `DeltaTotals []RiskTotal` and no field `DeltaTotal`.
2. The comments of `RiskIn` and `RiskResult` no longer say that a stock "of any currency counts" in a total; they say a Delta line has the currency of its name and the total is one for each currency.
3. `risk` gives each Delta line the currency of its stock or structures ("" read as "USD"); a name with two currencies is the line gap `missing_input`; a currency other than USD and JPY is the line gap `unsupported_currency`.
4. `risk` returns one `RiskTotal` for each of the currencies USD and JPY that has at least one counted line, USD first, each the sum of the lines of that currency only; a gap line makes the gap of the total of its own currency only. `VegaTotal` is as before.
5. `TestRiskDeltaTotalsPerCurrency` (new, in `risk_test.go`) shows a USD name and a JPY name give two totals that are not added, and the existing tests that read `DeltaTotal` read `DeltaTotals` instead.
6. No file other than the Files changes.
7. The result matches docs/design/risk-delta-per-currency.md#1-rdc-compute.

### 2 rdc-pages

Module: pages. Size: S.
Files: `internal/pages/risk_page/builder.go`, `internal/pages/risk_page/strings.go`, `internal/pages/risk_page/zh_texts.go`, `internal/pages/risk_page/builder_test.go`, `internal/pages/risk_page/testdata/ok.golden.json`
Design: docs/design/risk-delta-per-currency.md#2-rdc-pages
- Order: after 1.
Goal: Decisions 3 and 4. The owner sees "per $1" for USD names, "per ¥1" for JPY names and one total for each currency.
Acceptance:
1. `deltaBlock` reads `DeltaTotals` (new, declared by entry 1) and no longer `DeltaTotal`; `deltaCell` takes the currency of the line and sets `Currency` of the cell to it, and the unit text of a USD row is "美元/每 $1" and that of a JPY row "日元/每 ¥1", both defined in `zh_texts.go`; `unitDelta` in `strings.go` is removed.
2. The total cell of the Delta block has the texts of all totals joined by "；", and `value` and `currency` of the first total; with one currency it is the cell of that total.
3. When any Delta line is a gap, `deltaBlock` sets the block `Gap` (reason of the first gap line, text by `gapText`) and still sets `Total` when a total exists; a gap line is never dropped without the block `Gap`; `Total` is nil only when no total exists.
4. `TestDeltaByUnderlyingVegaByExpiry` and the golden `ok.golden.json` read the new unit text, and `TestDeltaTotalPerCurrency` (new, in `builder_test.go`) shows a USD name and a JPY name give a JPY row with the text "日元/每 ¥1" and one total cell holding both totals.
5. `contracts/` and `web/` are not edited.
6. The result matches docs/design/risk-delta-per-currency.md#2-rdc-pages.

## Checked by CI

- `ci-go`: `go test` of `internal/compute/...` and `internal/pages/...`, `golangci-lint` and `scripts/arch` pass.
- After the deploy the window reads `/risk` of the main account: a JPY name shows "日元/每 ¥1" and the total cell names each currency.

## Adversarial

- Standard sources: nothing is computed or invented; no rate is applied to Delta, and a currency not supported stays a gap with its reason.
- The page JSON carries finished figures; `web/` is not touched and the schema is unchanged.
- Smallest fix: the total stays one cell, so no contract change; the block `gap` and `total` are both optional in the schema and may stand together.

## Left out

- A Delta total converted to one currency is not built (dispatcher decision: no FX conversion).
- A currency other than USD and JPY is a gap, not a third total.
- `web/` showing two totals as separate cells would need a schema change; not done.
