---
type: design
title: "tracking shows opening credit, closing cost, realized P&L and the broker's realized apart on a closed option"
status: accepted
date: "2026-11-26"
need: [none]
none_reason: "UI data check, record state/optdesk-ui-check.md lines 93-94; no need record exists"
---

# tracking shows opening credit, closing cost, realized P&L and the broker's realized apart on a closed option

Author: the optdesk designer
Owner approved: (pending)
Revision: 1
Plan: P2

摘要：/options/tracking 的已平仓表只有一列“净权利金”（开仓现金加平仓现金），买回价高于卖出价时为负，看起来像“收到的权利金是负数”。改为分开显示：开仓收入（收到的权利金）、平仓成本（买回花的钱）、佣金、已实现盈亏，再加券商自己报的已实现盈亏，方便对账。分三件事：先加接口字段（trackapi），再在 optiontrack 里由成交行算出，再改页面。

Source: UI data check round 3 (state/optdesk-ui-check.md lines 93-94, against ee13f42), read against origin/main 076ebfac.

Needs the owner: nothing. Contracts: unchanged.

## Frame

Owner-visible result: on /options/tracking the closed table of a short option shows, for each row, "Opening credit" (the premium collected when it was sold, never negative for a short), "Closing cost" (what the buy-back cost; zero for an expired option), "Commissions", "Realized P&L" (opening credit minus closing cost minus commissions) and "Broker realized" (the broker's own figure, a gap when the broker reports none). A short put sold for 20 and bought back for 55 reads credit 20, cost 55, realized -35 less commission, not "premium -35".

Cause. `position` in `internal/compute/optionledger/premium.go` (line 150) prints `PremiumIncome` as `p.premium`, which `internal/compute/optionledger/lots.go` (line 69) builds as the cash of the opens plus the cash of the closes; the interface comment of `LedgerPosition` and `docs/contracts/compute.md` (Decision 9) say so. For a closed short whose buy-back cost more than the credit the sum is negative: the checked rows (GOOG puts 320, CEG put 250 and 255) are such rows, and `realized = premium - commission` (premium.go line 163) is the same sum, so the realized figure is arithmetically right and only the split is missing. The page showed that sum under "Premium collected" on 2026-11-17; since #2724 the closed table says "Net premium" (`internal/pages/option_tracking_page/tables.go`, line 78) and `moneyCell` prints the U+2212 minus, so the ASCII hyphen of the six rows is already gone on main. What is still missing: the credit and the cost are not apart, and `LedgerPosition.BrokerRealized` (already filled, premium.go line 154) is not carried to tracking. /options/result reads the same ledger and already lists the trades that differ from the broker.

### Decisions

#### Decision 1: tracking derives the two halves from the position's own fills; the ledger interface does not change

`internal/compute/api.go` has 840 lines, the most `docs/design/ratchet.json` allows it, so `LedgerPosition` gets no field; deriving in `optiontrack` is a workaround for that, not the first choice. `LedgerPosition.Fills` holds the rows of the piece in order, the first being the opening row, with the signed `Quantity` of the piece (the part of the row the piece owns, so a partial close splits both halves alike) and the `Price` text. In `optiontrack` (`baseRow`), both halves are gross trade money from the fill prices: `OpeningCredit` is minus the opening quantity times its price times `Multiplier`; `ClosingCost` is the sum over the closing fills (`Action` other than the open) of their quantity times price times `Multiplier`, and "0.0000" when the piece has no closing fill price (expired, or an assignment row with no price). `PremiumIncome` stays the ledger's figure from Flex `proceeds` and `RealizedPnL` stays `PremiumIncome - Commissions`, so /options/result, /household, `cmd/ot` and `optionledger` are unchanged. The identity `OpeningCredit - ClosingCost = PremiumIncome` is therefore not forced: it holds to the rounding of the price text, and the page shows the ledger's realized figure as it is, never a recomputed one. An open piece has an empty `ClosingCost`.

#### Decision 2: tracking carries the three fields and the page only prints them

`trackapi.TrackedOption` gets `OpeningCredit`, `ClosingCost` and `BrokerRealized`; `optiontrack` fills them in `baseRow` (the broker figure is copied from `LedgerPosition.BrokerRealized`). The page does no arithmetic: it prints the strings with `moneyCell` / `moneyHalfUp`. The closed table columns become company, contract, quantity, status, closed day, opening credit, closing cost, commissions, realized, broker realized; the "Net premium" column and its hint go. The open table is unchanged (its "Premium collected" is the credit of an open lot, no close exists).

#### Decision 3: the broker figure is the broker's own lot pairing and may differ per row

Checked read-only against the stored Flex rows (GOOG put 320, expiry 2026-10-30; the broker's figure includes the commission, as does the net cash below): a sell of 1 at 4.00 (net cash 399.44) and a sell of 1 at 3.89 (net cash 387.95) were open; one buy of 1 at 39.66 (net cash -3965.81) closed one of them. The broker's `fifoPnlRealized` for that close is -3577.86, which is 387.95 - 3965.81, so the broker paired the close with the 3.89 lot; the ledger paired it with the 4.00 lot (-3566.36). The 11.50 gap is the choice of lot, not a wrong fill; the totals over all the lots agree. So "Broker realized" is shown beside "Realized P&L" for the owner to read, and a per-row difference is not an error.

## Modules

compute (`internal/compute/optiontrack`, the interface file `internal/compute/trackapi/api.go`) and pages (`internal/pages/option_tracking_page`). No contracts, no migration, no `modules.md` change. `internal/compute/api.go` and `internal/compute/optionledger` are not edited.

## Interface

Owned by compute, landing first (issue 1):

- `internal/compute/trackapi/api.go`: `TrackedOption` gains `OpeningCredit`, `ClosingCost` and `BrokerRealized` (decimal text, 4 places; `ClosingCost` empty while open; `BrokerRealized` empty when the broker reports none), with a comment that `PremiumIncome` is `OpeningCredit` minus `ClosingCost`.

An addition of fields; no signature changes. The page JSON schema (`contracts/schemas/page.option_tracking_page.v1.json`) is not touched, because columns are rows of generic cells.

## Failure paths

- A position with no broker figure has an empty `BrokerRealized`; the cell is the gap `no_value`, not zero.
- A long option (bought to open) has a negative `OpeningCredit`; it is printed as it is. Tracking holds short options, so nothing special is built for it.
- An expired or assigned option whose closing fill has no price has `ClosingCost` "0.0000" and prints $0.00.
- A piece with no opening fill (not expected) gets empty `OpeningCredit` and `ClosingCost`, printed as gaps.

## Issues

### 1 ledger-realized-interface

Module: compute. Size: S. Order: first. Design: docs/design/ledger-realized.md#interface. Files: `internal/compute/trackapi/api.go`.

Acceptance:
1. `TrackedOption` in `internal/compute/trackapi/api.go` declares `OpeningCredit`, `ClosingCost` and `BrokerRealized` as strings, with a comment saying `PremiumIncome` is `OpeningCredit` minus `ClosingCost`; no other declaration of the file changes.
2. Docs: docs/design/ledger-realized.md#interface.

### 2 ledger-realized-compute

Module: compute. Size: S. Order: after 1. Design: docs/design/ledger-realized.md#decisions. Files: `internal/compute/optiontrack/rows.go`, `internal/compute/optiontrack/rows_test.go`, `internal/compute/trackapi/api_test.go`.

Acceptance:
1. A closed short put whose opening fill has quantity -1, price 20.00 and multiplier 100 and and one closing fill with quantity 1 and price 55.00 has `OpeningCredit` "2000.0000" and `ClosingCost` "5500.0000" in the row `baseRow` builds: a new test `TestOpeningCreditAndClosingCostApart` (new) in `rows_test.go` pins both, and that `RealizedPnL` is still the ledger's figure.
2. An expired short put (one fill) has `ClosingCost` "0.0000"; an open piece has an empty `ClosingCost`; a piece with no fills has both empty: `TestOpeningCreditAndClosingCostApart` (new) covers the three.
3. `baseRow` copies `BrokerRealized` from the ledger position, empty stays empty; `TestRowCopiesLedgerTerms` is extended to compare it.
4. `internal/compute/api.go` and the files of `internal/compute/optionledger` are not edited, and `rows.go` stays under 600 lines.
5. Docs: docs/design/ledger-realized.md#decisions.

### 3 ledger-realized-page

Module: pages. Size: S. Order: after 2. Design: docs/design/ledger-realized.md#decisions. Files: `internal/pages/option_tracking_page/tables.go`, `internal/pages/option_tracking_page/tables_test.go`, `internal/pages/option_tracking_page/zh_texts.go`, `internal/pages/option_tracking_page/zh_texts_test.go`, `internal/pages/option_tracking_page/cells_test.go`.

Acceptance:
1. `recentColumns` in `tables.go` has the ids company, contract, qty, status, closed_day, opening_credit, closing_cost, commissions, realized, broker_realized in this order, with the labels "Opening credit", "Closing cost" and "Broker realized"; the column "premium" and `netPremiumHint` are gone from the closed table.
2. The new cells read `OpeningCredit`, `ClosingCost` and `BrokerRealized` of the row through `moneyCell` or `moneyHalfUp`; the page file does no arithmetic on them. An empty value is the gap `no_value`.
3. A row with `OpeningCredit` "2000", `ClosingCost` "5500" in USD prints "$2,000.00" and "$5,500.00" and a realized of -3,500 prints with the U+2212 minus: a new test `TestClosedTableShowsCreditAndCostApart` (new) in `tables_test.go` pins the texts; `want` of the closed-column test (line 82 of `tables_test.go`) is updated to the new ids.
4. The Chinese hint of "Closing cost" says it is what the buy-back cost, zero for an expired option, and the hint of "Broker realized" says it is the broker's own figure, paired by the broker's own lot order, so it may differ from the realized figure of the row; `zh_texts_test.go` checks both hints are non-empty Chinese.
5. Docs: docs/design/ledger-realized.md#decisions.

## Checked by CI

- `go test ./internal/compute/... ./internal/pages/...`, `go vet ./...`, `golangci-lint` and `scripts/arch` pass.
- The tests are parallel with a temporary directory per file, a fake clock, no sleep, no live broker and no PostgreSQL.
- None of these is an acceptance item above.

## After merge (checked by the dispatcher, not by review)

- On /options/tracking the closed rows of GOOG puts 320, CEG put 250 and 255 show a positive opening credit, a closing cost, and realized as credit minus cost minus commission; the broker cell shows the broker's figure or a gap; a per-row difference from realized is the broker's lot pairing (Decision 3), not an error.

## Left out

- Changing /options/result, /household or `cmd/ot`: they read `PremiumIncome` and `RealizedPnL`, which keep their meaning.
- A difference column (realized minus broker realized): the page computes nothing and /options/result already lists the trades that differ.
- Showing the individual fills (price, quantity per fill) on tracking: `LedgerPosition.Fills` exists; not needed for the split.
- Why a GOOG put cost 55 to buy back is market data, not a ledger fault; a per-row gap to the broker figure is lot pairing (Decision 3); a wrong stored closing cash would show in the totals, and fixing the import is a separate issue.
- The empty-quote notes of tracking (no two-sided quote, `obs.option_eod`): a data-collection matter, a different design.
- Contracts: no file under `contracts/schemas`, `contracts/datasets` or `contracts/proto` changes.

## Adversarial

Full review: `the dispatcher's drafts folder, ledger-realized-adversarial.md` (a local file, not committed).

- Gap (broker figure vs commission): not adopted as a change; Decision 3 now says the broker figure includes the commission and the hint says it may differ.
- Unnamed risk (gross or net, source of the halves): adopted. Decision 1 names the fill prices as the source of both halves and no longer forces the identity; the closing cost is summed from closing fills.
- Simpler way (carry halves from the ledger): not adopted, `api.go` is at its ceiling; Decision 1 says it is a workaround.
- Lock-in: not adopted; accepted.
- Claim likely false (identity and zero cost on expiry): adopted, as above.
- Slip (test fixtures outside Files): adopted; `trackapi/api_test.go` and `cells_test.go` are in the Files lines.
