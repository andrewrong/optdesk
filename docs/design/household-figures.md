# household figures: JPY holdings convert to USD with the stored rate, and marked positions fill the Stock value and Option value cells

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-13
Revision: 2

Source: UI check round 4 of `/household` (deployed e560294), rows 3.3, 3.10, 3.11 (yen amounts labelled USD; allocation a gap) and row 3.13 (the Stock and Option cells of accounts are always `missing_quote`). Row 3.5 (stress rows) is split off (see Left out).

Goal: a JPY holding shows USD with the stored USD/JPY, or stays in JPY labelled JPY with a gap that names the rate; a position with a mark fills the Stock value and Option value cells; an unmarked position is named in the gap. The front end computes nothing.

Existing design: none owns this. `builders-wave.md` Decisions 6 and 7 built the cells as gaps "until a position-marks design adds it"; `position-marks.md` (merged) stores the marks; `pages-real-data.md` wired the stock-only holdings blocks. No design reads a rate into the household builder.

## Today

All paths at origin/main.

- `internal/compute/pagefig/holdings.go:28-31`: a row whose `Currency` differs from `HoldingsIn.Currency` keeps its unconverted `Value` and `Unrealised`, `Share` is the gap `unsupported_currency`, and both totals carry the gap. `blockapi.HoldingsIn` (`internal/compute/blockapi/api.go:105`) has no rate, and `HoldingLine` has no currency.
- `internal/pages/household_page/holdings.go:91` labels every row cell with the one `currency` argument (`holdings.go:70-72`, the currency of `NetLiquidation`), so a JPY row shows `$3,474,600.00` (4063). `allocationOf` (`holdings.go:115-128`) is a gap because `TotalValue` is a gap.
- `HouseholdInput.UsdJpy` exists (`api.go:39`) but no caller sets it: `internal/pages/household_page/builder.go:83-87` and `internal/pages/home_household/member.go:91-93` leave it empty. "household_page already reads that rate" does not hold. Only `internal/pages/option_data_page/usd.go:23` reads a rate (`bar.FxRateReader.LatestFxRate`, `internal/store/bar/api.go:49`, pair `USDJPY`, newest rate day on or before the asked day).
- `pagefig/household.go:79-91` `line` does not convert: a JPY amount stays yen, with the note `≈USD n` when a rate is given and the gap `unsupported_currency` when not. It stays as it is.
- `pagefig/household.go:150-151` (`accounts`: Stock value, Option value) are hardcoded `gapLine(..., "missing_quote")`; `:183` does the same for the stress rows (not changed here). `HouseholdInput` (`api.go:32-40`) carries no positions or marks.
- `household_page/holdings.go:26-65` reads stock positions (`c.Account.Positions`) and marks (`c.Marks.Marks`, `internal/store/mark/api.go:51`, basis other than `none` with a price); options are skipped at `:53`. `builder.go:88` reads the positions a second time. `accountsOf` (`blocks.go:343-358`, `fig` at `:348-352`) turns a gap line into `gapText(lineGap(l))` and drops the line's `Note`.
- `home_household` uses only the net and cash lines (`member.go:177-178`) with a USD-only `lineGap` (`member.go:344`); it is not touched.
- Wiring is `cmd/ot/household_site.go:30-35` (`Marks: rd.marks` set). `siteReaders.bars` is a `bar.PriceBarReader` (`cmd/ot/site.go:127`, `openBarReader` at `:625`, assigned at `:275`); no `FxRateReader` is held in `cmd/ot`.

## Decisions

1. The rate is the stored `USDJPY` rate of the capture day or the newest before it (`LatestFxRate`, no age limit: dev mode; `fx-cash.md` owns the job). It is read only when a value, mark or position of the capture is JPY. It travels as text; no page file parses it. The page shows the rate's day (`FxRate.RateDay`, copied as text) in the text of a converted row's block.
2. A rate not found, or a read error, is a JPY-only gap: the JPY rows are `unsupported_currency` naming the missing rate (decision 4) and everything else on the page renders. Never an error of Build.
3. A JPY holding row with a rate is converted by compute: `Value`, `Unrealised` and `Share` are USD (`yen / rate`, `money` rounding), the row's `Currency` is `USD`, the totals include it, the allocation block shows the stock total.
4. A JPY row without a rate stays in JPY: `Value` and `Unrealised` are yen, `Currency` is `JPY`, `Share` is the gap `unsupported_currency`, both totals carry that gap (today's rule). `HoldingLine` carries `Currency`; `household_page` labels each cell by it; the block gap text is `<symbols>: no USDJPY rate on or before <day>`. No new gap code.
5. Only a JPY row against a USD denominator converts. Every other pair (a USD position in a JPY-net account, a third currency) is `unsupported_currency`, never summed.
6. `HouseholdInput` gains `Positions []PositionIn` and `PositionsRead`. `PositionsRead` false (no Marks reader) keeps both cells `missing_quote`, as today. True with no positions gives `0.00`.
7. Stock value is the sum of `quantity x multiplier x mark` over stocks, Option value over options (signed: short is negative), in the NetLiquidation currency, by decisions 3 to 5. One unmarked position of a kind (empty mark) makes that cell the gap `missing_quote` and the `Note` of the line names it (`no mark: AAPL, TSLA 260116P00150000`); no partial sum. A JPY position without a rate makes the cell `unsupported_currency` with the note `no USDJPY rate`. A position of another sec type (for example `index`) makes both cells `missing_input` with the note `unsupported type: <symbol>`; it is never skipped in a sum.
8. The page shows the note: `accountsOf` puts the `Note` of a gap line into the gap text of the cell (`blocks.go`, in Files).
9. `household_page` reads positions and marks once and feeds both `Household` and `Holdings` (the second `Positions` read at `builder.go:88` is dropped). `home_household` is not changed: it renders only net and cash.
10. Wiring is module ot, entry 3. Until it lands `Fx` is nil, so the JPY rows show the gap of decision 4 and the entries merge in order without a half state.

## Modules

No row of `docs/design/modules.md` changes. `internal/compute/blockapi` is in the compute entry cell, `internal/pages/**` and `cmd/**` are the pages and ot paths; `pages` already depends on `store` (for `store/bar`, as `option_data_page` does) and `compute`.

## Interface

Module compute, `internal/compute/blockapi/api.go`; existing fields and signatures stay, fields are added.

```go
// PositionIn is one held stock or option of the capture, all numbers decimal text. SecType is the store's value
// ("stock", "option"; any other value is a gap). Mark is the price per unit, empty when the position has none.
// Multiplier is 1 for a stock. Quantity is signed.
type PositionIn struct {
	Underlying, SecType, Currency, Quantity, Multiplier, Mark string
}
```

- `HouseholdInput` gains `Positions []PositionIn` and `PositionsRead bool`; the contract text states decisions 5 to 7.
- `HoldingsIn` gains `UsdJpy string` (empty without a rate).
- `HoldingLine` gains `Currency string` (currency of `Value` and `Unrealised`: `USD` when converted, the row's own when not).
- `HoldingsResult`, `HoldingFigures`, `PageFigures` and `HouseholdResult` are unchanged.

`Contracts: unchanged` (no schema, dataset or proto file; the JSON shape is unchanged, JPY is already in the schema).

## Issues

Build order: every entry compiles and its tests pass against main. BASE is `git merge-base origin/main HEAD`.

### 1 household-figures-api (interface)

Module: compute. Size: M.
Order: first.
Files: `internal/compute/blockapi/api.go`, `internal/compute/blockapi/api_test.go`, `internal/compute/pagefig/holdings.go`, `internal/compute/pagefig/holdings_test.go`, `internal/compute/pagefig/household.go`, `internal/compute/pagefig/household_test.go`, `internal/compute/pagefig/exposure.go` (new), `internal/compute/pagefig/exposure_test.go` (new)
Design: docs/design/household-figures.md#interface (Decisions 3 to 7)
Acceptance:
1. `api.go` declares `PositionIn` with the six fields of the Interface section, `HouseholdInput.Positions`, `HouseholdInput.PositionsRead`, `HoldingsIn.UsdJpy` and `HoldingLine.Currency`, no function body.
2. `api_test.go` fills every new field through a fake and reads each back; every old keyed literal in the package tests compiles unedited.
3. In `holdings.go` a `JPY` row with `in.Currency == "USD"` and a positive `UsdJpy` has `Currency == "USD"`, numeric `Value`, `Unrealised`, `Share`, and is in both totals; with an empty or non-positive rate it keeps decision 4 with `Currency == "JPY"`; any other mismatched pair is the gap `unsupported_currency`; every matching row has `Currency == in.Currency`.
4. `holdings_test.go` names `TestJpyRowConvertedWithRate` (yen 3474600 at 150 gives 23164.00), `TestJpyRowKeepsYenWithoutRate` and `TestUnsupportedPairStaysGap`; the case at `holdings_test.go:59` still expects the gap for an empty rate.
5. `exposure.go` holds the value arithmetic with `math/big`, no float; `accounts` in `household.go` returns the two gap lines only when `PositionsRead` is false; `stressSteps` and `stress` are unchanged; `household.go` stays under 300 lines, `exposure.go` under 400.
6. `exposure_test.go` names `TestStockAndOptionValueFromMarks` (a short put with mark 2.50, multiplier 100, quantity -1 gives -250.00), `TestUnmarkedPositionNamedInNote` (Stock value is the gap `missing_quote`, the `Note` is `no mark: ` and the symbol), `TestJpyPositionNeedsRate`, `TestOtherPairIsGap`, `TestIndexPositionIsNamedGap` and `TestPositionsNotReadKeepsMissingQuote`.
7. `git diff --name-only BASE` lists only paths of the Files line, and `git diff BASE -- internal/pages internal/compute/pagefig/risk.go` is empty.

### 2 household-figures-pages

Module: pages. Size: M.
Order: after 1.
Files: `internal/pages/household_page/page.go`, `internal/pages/household_page/builder.go`, `internal/pages/household_page/holdings.go`, `internal/pages/household_page/blocks.go`, `internal/pages/household_page/strings.go`, `internal/pages/household_page/builder_test.go`, `internal/pages/household_page/holdings_test.go`, `internal/pages/household_page/testdata/holdings.golden.json`
Design: docs/design/household-figures.md (Decisions 1, 2, 4, 7 to 9)
Acceptance:
1. `household_page.Config` has a field `Fx bar.FxRateReader`; nil means no rate.
2. `git grep -c "Account.Positions" -- internal/pages/household_page` counts 1 over the non-test files (read once); that one read feeds `Household` (`Positions`, `PositionsRead`) and `Holdings` (`UsdJpy`, row currencies).
3. `LatestFxRate(ctx, "USDJPY", day)` is called only when a value, mark or position of the capture is `JPY`; a found rate is copied as text into both `UsdJpy` fields and its `RateDay` into the page text; not found, a nil `Fx` and an error all leave `UsdJpy` empty and Build returns no error for them.
4. `holdingRows` labels each cell by `HoldingLine.Currency`; `git grep -n "Currency: currency" -- internal/pages/household_page/holdings.go` prints nothing.
5. The gap text for `unsupported_currency` on a holdings block holds `no USDJPY rate on or before` and the day, defined in `strings.go`; the text of that code for other blocks is unchanged.
6. `accountsOf` in `blocks.go` appends the `Note` of a gap line to the gap text of its cell, so the Stock value and Option value cells show the named unmarked positions.
7. `builder_test.go` and `holdings_test.go` cover: a JPY stock with a rate is a USD amount with a share and the allocation block has a row; without a rate it is yen labelled JPY with the gap; a fx read error does the same and Build returns no error; Stock value and Option value are numbers with marks and a gap whose text names the symbol when a mark is empty; with `Marks == nil` the old `missing_quote` results stay (`builder_test.go:125-131`, `:255-265`); every test Config that sets `Marks` is checked for changed expectations.
8. `testdata/holdings.golden.json` is regenerated by its test and holds no yen amount labelled USD; `purity_test.go` still holds (no `math/big`, float, `sort`).
9. `git diff --name-only BASE` lists only paths of the Files line; `internal/pages/home_household` and `cmd/ot` are not edited.

### 3 household-figures-wire

Module: ot. Size: S.
Order: after 2. `cmd/ot/site.go` is also edited by account-identity entry 6 (issue 2030 was open on 2026-11-13); if it is not merged when this entry is cut, add "after" it (read-only check: `gh issue list --repo andrewrong/optdesk --search "account-identity"`).
Files: `cmd/ot/site.go`, `cmd/ot/household_site.go`, `cmd/ot/household_site_test.go`
Design: docs/design/household-figures.md (Decision 10)
Acceptance:
1. `siteReaders` in `site.go` has a field `fx bar.FxRateReader` assigned beside `bars` at `site.go:275` from the same opened bar store, nil when `bars` is nil (no opener).
2. `householdConfig` sets `Fx: rd.fx`; `site.go` stays under 800 lines.
3. `household_site_test.go` builds the page with a fake `LatestFxRate` and a JPY position: the amount is labelled USD; with `fx` nil the yen stay labelled JPY with the gap.
4. `git diff --name-only BASE` lists only paths of the Files line.

## Checked by CI

`go test ./internal/compute/... ./internal/pages/... ./cmd/ot/... -race -count=1`, `go run ./scripts/arch --check`, `go build ./...`, `golangci-lint`, and the contracts generation check (no change expected). The review runs no test.

## Left out

- Stress rows (`household.stress` steps, `household.go:183`): they stay `missing_quote`. Reasons: an option on an unheld underlying needs the spot of that underlying and no household reader gives it (marks exist per held ConID only; most of the owner's options are short puts on unheld stocks); and the model (intrinsic with time value held, a one-for-one bound, or delta from `obs.option_eod`) is the owner's choice. A later design decides both.
- Cash after assignment and obligations of a JPY short put (`unsupported_currency` at `household.go:168-169`, yen with a note at `:79-91`): unchanged.
- Greeks, margin, the Movers and long-term blocks, option rows in the holdings blocks (stock-only).
- A rate age limit or fallback rate; pairs other than USD/JPY; `home_household`; `positions_page` (it leaves `UsdJpy` empty, keeps its yen values and its own `p.Currency`).

## Doubts

1. Positions and marks are read as the newest capture each; from different captures the conids miss and every cell reads "unmarked" (named, not wrong). The holdings blocks have the same property.
2. The rate has no age limit; a missed `fx_refresh` converts with an old rate, visible only by the shown rate day.
3. `HoldingLine.Currency` lets `positions_page` disagree with it if it later passes a rate.
4. The brief said `household_page` already reads the rate; it does not (see Today), so entry 3 exists.

## Adversarial

Full answers: scratchpad household-figures-adversarial.md of the dispatcher (with the advisor review r1).

1. Gap against the goal (stress stays gaps for an option seller): adopted by the split; stress is in Left out with the reason. Cash after assignment of a JPY put: named in Left out. The note not reaching the page: adopted (Decision 8, entry 2 item 6). The stress share: dropped with stress.
2. A fx read error fails the whole page: adopted (Decision 2, JPY-only gap).
3. Simpler way: adopted in part (stress split off; the rate stays a `HoldingsIn` field because `HouseholdInput.UsdJpy` serves the money lines and holdings is a separate input).
4. Lock-in: `PositionIn` has no market-data field beyond the mark; the duplicate read is gone with `home_household`; the `positions_page` currency point is Doubt 3.
5. Entry 3 claim false (no `FxRateReader` in `cmd/ot`): adopted, `site.go` is in Files and acceptance 1 is rewritten; `noBarStore` is dropped.
6. Likeliest slip: Files and acceptance fixed (note on the page, every test Config with `Marks` checked); `home_household` dropped (advisor 3); other pairs and `index` positions decided (Decisions 5, 7); new files marked, field count fixed.
