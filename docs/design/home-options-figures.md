# home options figures: the three const-gap tiles of the home options card get compute figures from the replayed option ledger

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-13
Revision: 2

Source: the home page options card shows three tiles as fixed gaps (`internal/pages/home_options/member.go:259-262`: `noData(textSoldCount)`, `textOpen`, `textMaxLoss`). `docs/conventions/register.toml` registers them as `home.options.sold_count_month`, `home.options.open_premium` and `home.options.max_loss` (carried over from option-trader, `source_hash` pending) and no Go code implements them.

Goal: the tiles Sold this month, Open premium and Max loss show a figure computed in `internal/compute`, or a gap that names why. The front end and the page builder compute nothing: the builder copies text from compute and places the dollar sign, separators and captions.

Existing design: none owns this. `household-figures.md` is the model (compute figures behind `blockapi`, a pages builder that copies them) and does not touch the options card. `account-identity.md` (revision 5, in flight) decides the alias key (Decision 1).

## Today

All paths at origin/main.

- `home_options.replay` (`member.go:192-224`) reads every `StoredAliases`, replays each alias with `compute.OptionLedger.Replay` and keeps `Positions`, `Unresolved`, `Review`; it drops the replay's `Cycles`. `Unresolved` and `Review` are only appended and never read in `assemble`: no review line exists on the page. `compute.OptionResults.Summarise` gives only closed, sold/bought, per-currency sections.
- `compute.LedgerPosition` (`internal/compute/api.go:542`) carries `PositionID` (`alias:openTx:n`, `optionledger/premium.go:141`), `Status`, `Quantity` (signed, negative is short; a closed piece keeps the sign of its lot), `Multiplier`, `Currency`, `OpenedOn` (`YYYY-MM-DD`), `OpenPrice`. A lot closed in two steps is two positions with the same `alias:openTx` and a different `n` (`optionledger/lots.go:12`).
- `compute.OptionStructures.Evaluate` (`internal/compute/optionstruct/`) groups the open positions of one alias into `Structure`s and gives `Figures.MaxLoss`: a short put is `-(strike x qty x multiplier - premium)`, a credit vertical spread is `credit - width` (both negative for a loss), and `naked_call`, `covered_call`, `other` are the compute gap `no_rule`; a non-USD structure is the gap `unsupported_currency` (`optionstruct.go:65-67`, `risk.go:28-47`). A short call is `covered_call` only when the shares of the replay's `Cycles` (assignments in the ledger) cover it, otherwise `naked_call` (`recognise.go:226-234`); a debit pair and a position whose numbers do not parse are `other` (a parse failure gets zeroed numbers and the raw `Currency`, `recognise.go:70-85`). `MaxLoss` needs no quotes. `structures_page` and `risk_page` call `Evaluate`; `cmd/ot/site.go:352` sets `siteReaders.structures`.
- `contracts/schemas/page.options_summary.v4.json`: `sold_count_month` is a `CountTile` (`CountFigure`: integer, unit `structures`); `max_loss_usd` and `open_premium_usd` are `UsdTile` (lines 281-286: `common.v2.json` `UsdFigure`, unit `usd`, `value` minimum 0 with two decimals, `text` whole dollars); only `premium_month_usd` is a `SignedUsdTile`. A tile figure may instead be a `Gap`, whose `reason` is one of `missing_quote`, `no_data`, `unsupported_currency` (lines 171-176) and whose text is at most 200 characters; a tile has an optional `caption`.
- `member.go:256` starts the card gaps as `noData(textMonthly), noData(textMaxLoss)`; `textMaxLoss` (`member.go:52`) says the ledger does not give max loss.
- `member.go:233-236` `newestDay` is the newest opening, closing or fill day of the positions, not the fetch day; the page shows `Ledger as of MM-DD` and the stale label.
- `home_options/purity_test.go` forbids `math/big`, `math`, `strconv`, `sort`, `os` and others in the non-test files of the package; `member_test.go` `TestGoldenOK` builds `okCfg` (no structure reader) against `testdata/ok.golden.json`. Only `member.go`, its tests and golden, and `cmd/ot/home_site.go` mention `home_options` or the tile texts (`git grep`).

## Decisions

1. Alias key: the figures read only the Flex option ledger (`store.OptionLedgerReader.StoredAliases`, `FlexTradeRows`), as the Closed premium tile does. They read no `account.position`, so the gateway alias never enters. Before `account-identity.md` the stored alias is the Flex alias; after its migration `0037` it is the owner alias; the column is the same, the figures sum all stored aliases and show none, so neither order needs a change here. The tiles are all accounts combined (the sentence the page already uses in `noteClosed`) and every caption says so.
2. Source is the ledger, not the live snapshot. The register `home.options.max_loss` `inputs` name the current account snapshots; the ledger is the page's data day, needs no 24-hour snapshot rule, and reads no gateway. The figures are as of the ledger day in the caption (`Ledger as of MM-DD`); a position sold or assigned after the last Flex run is not in them. Deviation from the register; the owner updates `register.toml` after merge. For confirmation.
3. Sold this month = the number of sold opening lots opened in the calendar month of the page day, through it: distinct `PositionID` text before its last `:` over positions with negative `Quantity` and `OpenedOn` of that month and not after the day, of every status; a position whose `Quantity` does not parse is not counted. A roll's new sold lot counts; a buy-back does not; a vertical spread counts its sold leg once. The date is the Flex trade date, not converted to US Eastern. Deviation from the register (structures) and from the schema description of `CountFigure` ("count of option structures"): structures are recognised among open positions only, so a sold structure closed this month would be missed. The schema, the key and the unit `structures` stay (`Contracts: unchanged`); the caption says `lots opened this month`. For confirmation.
4. Open premium = the sum, over the open structures of USD that have a short leg (`short_put`, `put_spread`, `call_spread`, `covered_call`, `naked_call`, and `other` with a short leg), of the net opening credit of the part still open: over its legs `-Quantity x OpenPrice x Multiplier`. `long_put`, `long_call` and structures with no short leg are left out. A partly closed lot counts its remaining piece only (Left out); the caption says `remaining part of each lot`. It needs `Structures` for the net of a spread; Sold count needs `Positions` only. A total below zero is the gap `no_data` `Net paid, not shown`.
5. Max loss is shown as a positive amount under the label `Max loss` (the `UsdTile` has no sign): the magnitude of the sum of `Figures.MaxLoss` over the open USD structures that have a number. Three cases:
   - Every USD structure has a number: that amount, caption `Ledger as of MM-DD, all accounts combined`.
   - Some short call is `naked_call` in `Evaluate` (no assigned shares in the ledger; stock bought on the market or before the Flex history is not known to it, so this can be a call that is in fact covered): the tile is the gap `no_data` with the text `N calls without assigned shares in the ledger; other structures $X`. The words naked and unbounded are never used. `Held` shares of the capture are not given to `Evaluate` and nothing here depends on them.
   - Some USD structure has no number and no call is in the case above (`covered_call`, `other` including a debit spread): the amount of the others with the caption `At least: N structures have no loss number`.
6. Currency, USD only, as the page does today. A structure whose `Currency` is not `USD` (JPY, any other, or empty) is not summed and counted in `OtherCurrency`; caption text `N non-USD structures not counted` (it names no currency). When every open structure is non-USD, Open premium and Max loss are the gap `unsupported_currency` with the text `Only non-USD structures are open (N), not converted`, never `$0.00`. Sold count has no currency and counts every lot. No FX conversion in this cut (Left out). A USD structure with a leg whose numbers do not parse is counted in `Unreadable`, enters no sum, and the caption adds `N structures with unreadable rows not counted`.
7. Rounding: compute sums the exact decimal values `Evaluate` and the legs give (`math/big`, no per-structure rounding), rounds the total once to cents half away from zero for `value` and once to whole dollars half up from the same exact total for `text`; neither is rounded from the other. This is not the rule of the Closed premium tile (`textNotCents` refuses a sub-cent value; unchanged, that tile reads another field).
8. Zero and gaps:
   - No ledger reader or no row: the three tiles keep the existing gaps (`noText`).
   - Ledger rows, no open structure at all: Open premium and Max loss are the real figure `$0.00` with the caption `No open structures in the ledger as of MM-DD`. This deviates from the register gap text "Never a zero" of both figures, which is about accounts left out; here nothing is left out because the ledger is read. Sold count is its count, possibly 0. For confirmation; the alternative is the gap `no_data` with the same text.
   - `Unresolved` or `Review` rows from the replay: the figures still show and the card gets one new gap line `N ledger rows need review, figures may be incomplete` (reason `no_data`). This line is new work.
   - `Structures` or `Tiles` of the page Config nil (not wired): the three tiles keep today's gaps and Build returns no error, so the merge order of the entries cannot break the page.
9. Card-level gap: when `Structures` and `Tiles` are set, `noData(textMaxLoss)` leaves `k.Gaps`, whatever the tile shows (the tile carries its own gap text); it stays when either is nil. The two other texts are tile figures only, not card gaps.
10. `Evaluate` is called once per alias (`Alias` is part of the structure id) with `Alias`, `Positions`, `Cycles`, `AsOf`; no quotes, no held list, no session days (they feed marks and the capture diff, which `MaxLoss` does not use; a zero `StructureParams` is safe, `optionstruct/marks.go` guards it). `Structures` and `Other` of every alias go to compute with all positions.
11. Contract: `Contracts: unchanged`. The tiles use the existing `UsdTile`, `CountTile` and `Gap` shapes.

## Modules

No row of `docs/design/modules.md` changes. `internal/compute/blockapi`, `internal/compute/pagefig` and `docs/contracts/compute.md` are in the compute row; `internal/pages/**` is the pages row and `cmd/**` the ot row; pages already depends on compute and store, and ot on pages and compute.

## Interface

Module compute, `internal/compute/blockapi/api.go` (the interface file); the implementation is `internal/compute/pagefig/optiontiles.go`. Existing declarations stay; these are added.

```go
// OptionTilesIn is the input of the options card: the replayed positions of every stored alias and the structures
// that Evaluate recognised among their open positions (Structures and Other of every alias). Pure.
type OptionTilesIn struct {
	Day        contracts.Day
	Positions  []compute.LedgerPosition
	Structures []compute.Structure
}

// UsdText is one exact sum as text: Value has two decimals (half away from zero), Dollars is whole dollars
// (half up); both come from the same exact sum. Both are never negative.
type UsdText struct{ Value, Dollars string }

// OptionTilesOut holds the three figures and the counts the captions name. SoldCount is the sold opening lots
// of the month of Day. OpenPremiumGap and MaxLossGap are empty or "no_data" or "unsupported_currency"
// (page-schema reasons); with an empty gap the matching UsdText is the figure. With MaxLossGap "no_data"
// and Uncovered above 0, MaxLossUsd is still the sum of the other structures (empty when none). Uncovered
// counts calls recognised without assigned shares; Unnumbered the USD structures with no number that are not such calls;
// Unreadable the USD structures with a leg whose numbers do not parse; OtherCurrency the structures not
// in USD; OpenCount the open structures of every currency.
type OptionTilesOut struct {
	SoldCount                                                  int
	OpenPremium, MaxLossUsd                                    UsdText
	OpenPremiumGap, MaxLossGap                                 string
	OpenCount, Uncovered, Unnumbered, Unreadable, OtherCurrency int
}

// OptionTiles evaluates the three figures of the home options card, no I/O, no clock.
type OptionTiles interface{ Tiles(OptionTilesIn) OptionTilesOut }

// NewOptionTilesFunc constructs the implementation (internal/compute/pagefig.NewOptionTiles).
type NewOptionTilesFunc func() OptionTiles
```

`Contracts: unchanged`.

## Issues

Build order: each entry compiles and its tests pass against main. BASE is `git merge-base origin/main HEAD`.

### 1 home-options-figures-api (interface)

Module: compute. Size: M.
Order: first.
Files: `internal/compute/blockapi/api.go`, `internal/compute/blockapi/api_test.go`, `docs/contracts/compute.md`, `internal/compute/pagefig/optiontiles.go` (new), `internal/compute/pagefig/optiontiles_test.go` (new)
Design: docs/design/home-options-figures.md#interface (Decisions 3 to 7, 10)
Acceptance:
1. `api.go` declares `OptionTilesIn`, `UsdText`, `OptionTilesOut`, `OptionTiles` and `NewOptionTilesFunc` with the fields and method of the Interface section, no function body; `git diff BASE -- internal/compute/blockapi/api.go` has only added lines.
2. `docs/contracts/compute.md` gains a section `## Option tile figures` naming those five declarations, and a version line `v13: additions only` for them; no existing line is changed.
3. `api_test.go` has a fake `OptionTiles` and `NewOptionTilesFunc` and reads every new field back through it.
4. `optiontiles.go` has `func NewOptionTiles() blockapi.OptionTiles` and `var _ blockapi.OptionTiles = optionTiles{}`; its import list holds `math/big` and no `time`, `os`, `net`, `database/sql`, `math` or `strconv` (read the import block); it is under 400 lines.
5. `SoldCount` counts the distinct `PositionID` prefixes before the last `:` of positions with a negative `Quantity` whose `OpenedOn` has the year and month of `Day` and is not after it, of every `Status`; a `Quantity` that does not parse is not counted.
6. `OpenPremium` sums `-Quantity x OpenPrice x Multiplier` over the legs of every `USD` structure that has a leg of negative `Quantity`, kinds `long_put` and `long_call` excluded; a total below zero gives `OpenPremiumGap == "no_data"`; a structure with a leg that does not parse adds 1 to `Unreadable` and enters no sum.
7. `MaxLossUsd` is the magnitude of the sum of `Figures.MaxLoss.Value` of the USD structures that have a value; kind `naked_call` adds 1 to `Uncovered` and sets `MaxLossGap = "no_data"`; any other USD structure whose `MaxLoss` is a gap adds 1 to `Unnumbered`; a structure whose `Currency` is not `USD` adds 1 to `OtherCurrency` and enters no sum; when `OpenCount > 0` and every open structure is non-USD both gaps are `unsupported_currency`; `OpenCount` counts open structures of every currency.
8. Each total is one exact `math/big` sum rounded once: `Value` to cents half away from zero, `Dollars` to whole dollars half up from the same sum (Decision 7).
9. `optiontiles_test.go` has `TestSoldCountMonth` (a lot closed in two pieces counts once; the month before and a bought lot do not), `TestOpenPremiumSpreadNet`, `TestOpenPremiumRemainingPieceOnly` (the `OpenPrice` of a remaining piece is the price of its lot, so a partly closed lot is its remaining quantity times that price), `TestOpenPremiumNetPaid`, `TestMaxLossShortPutAndSpread`, `TestMaxLossUncoveredCallGap`, `TestMaxLossUnnumbered`, `TestOtherCurrencyNotSummed`, `TestOnlyNonUsdIsUnsupportedGap`, `TestUnreadableLegNotSummed`, `TestHalfCentRounding` (one exact sum, not a sum of rounded parts) and `TestEmptyInputZero`.
10. `git diff --name-only BASE` lists only paths of the Files line.

### 2 home-options-figures-pages-wire

Modules: pages and ot (one entry: the `cmd/ot` wiring sets the `Config` fields this entry adds, so neither half shows anything without the other). Size: M.
Order: after 1.
`cmd/ot/home_site.go` may also be edited by an account-identity entry; if one is open when this entry is cut, add "after" it.
Files: `internal/pages/home_options/member.go`, `internal/pages/home_options/tiles.go` (new), `internal/pages/home_options/tiles_test.go` (new), `internal/pages/home_options/member_test.go`, `internal/pages/home_options/testdata/figures.golden.json` (new), `cmd/ot/home_site.go`, `cmd/ot/home_site_test.go`
Design: docs/design/home-options-figures.md (Decisions 1, 3 to 6, 8 to 11)
Acceptance:
1. `home_options.Config` has `Structures compute.NewOptionStructuresFunc` and `Tiles blockapi.NewOptionTilesFunc`; with either nil the three tiles, the card gaps and `testdata/ok.golden.json` are as today and Build returns no error (`ok.golden.json` is not in Files and `TestGoldenOK` passes unedited, because `okCfg` sets neither field).
2. `replay` keeps the `Cycles` of each alias and calls `Evaluate` once per alias with `Alias`, `Positions`, `Cycles` and `AsOf` set and no other `StructureInput` field; `Structures` and `Other` of every alias, and all positions, are the `OptionTilesIn` of one `Tiles` call.
3. `tiles.go` holds the tile placing (the count figure, the `UsdFigure` with `$`, thousands separators and `Dollars` as `text`, `Value` as `value`, the captions and gap texts); its import list has no `math/big`, `math`, `strconv`, `sort`, `os` (`purity_test.go` passes unedited and is not in Files); it is under 400 lines; every text of Decisions 3 to 6 and 8 is a constant in `tiles.go`.
4. Tile shapes: `open_premium_usd` and `max_loss_usd` are `UsdFigure` (unit `usd`, never signed, `Max loss` positive) or a `Gap` with reason `no_data` or `unsupported_currency` only; `sold_count_month` is a `CountFigure` (unit `structures`) with the caption `lots opened this month`; every tile caption begins `Ledger as of MM-DD, all accounts combined` and the Open premium caption adds `remaining part of each lot`; the Max loss gap text for `Uncovered > 0` is `N calls without assigned shares in the ledger; other structures $X` and the strings `naked` and `unbounded` occur in no non-test file of the package.
5. With `Structures` and `Tiles` set, `k.Gaps` no longer holds `noData(textMaxLoss)`; with either nil it does. With `Review` or `Unresolved` rows, `k.Gaps` gets one line `N ledger rows need review, figures may be incomplete` (reason `no_data`); with none, no such line.
6. `tiles_test.go` and `member_test.go` cover: a fake ledger with a short put and a put spread opened this month gives the count, the open premium and the max loss of a fake `OptionTiles`; `Uncovered > 0` gives the Max loss gap and the bounded rest in its text; only non-USD open gives `unsupported_currency` for both and not `$0.00`; no open structure gives `$0.00` with its caption; a net paid total gives the `no_data` gap; the Review line appears and disappears; the unreadable and non-USD captions; the card gap leaves with the two fields set and stays with either nil; each output validates against `page.options_summary.v4.json`.
7. `testdata/figures.golden.json` is new, committed with its test (`TestGoldenFigures` compares the indented output byte for byte, as `TestGoldenOK` does; there is no update flag); it holds a figure in each of the three tiles and no text of the form `the ledger summary gives no`.
8. `cmd/ot/home_site.go` sets `Structures: rd.structures` and `Tiles: pagefig.NewOptionTiles` in the `home_options.Config`; `cmd/ot/home_site_test.go` builds the home page with the fakes and reads the options card tiles as figures.
9. `git diff --name-only BASE` lists only paths of the Files line.

## Checked by CI

`go test ./internal/compute/... ./internal/pages/... ./cmd/ot/... -race -count=1`, `go run ./scripts/arch --check`, `go build ./...`, `golangci-lint`, and the contracts generation check (no change expected). The review runs no test.

## Left out

- JPY and other non-USD structures in the sums (register converts with DEXJPUS): counted and named, not summed. A later design adds it when a rate reader reaches `home_options` (the stored `USDJPY` of `household-figures.md` entry 2).
- Structure-level Sold this month (the register counts structures): needs a recogniser for closed structures; the lot count stands in until then.
- The original opening net including the closed part of a partly closed lot (register open premium): the ledger piece holds the remaining quantity only; the caption says so.
- Giving the capture's held shares to `Evaluate` so a covered call on stock bought outside the ledger is not a call without assigned shares; per-account coverage and held-for-review rules of the register (the page has only the new review line); the 24-hour snapshot rule; synthetic-short detection (no such kind exists); stress and margin figures.
- Updating `docs/conventions/register.toml` (versions, `implemented_by`): owner-only, after merge.

## Doubts

1. Whether `Structure.Legs[].OpenPrice` is the price of the lot for every remaining piece (it is `LedgerPosition.OpenPrice`, read in `parseLeg`); acceptance 9 of entry 1 settles it with `TestOpenPremiumRemainingPieceOnly`.
2. A debit spread is `other` in `Evaluate`, so the `At least` caption shows for ordinary debit spreads although their max loss is the debit.
3. After account-identity, a ledger stored under two aliases for one account would be summed twice (the `PositionID` carries the alias); the rename of `0037` makes this impossible by its collision guard.
4. The Flex trade date can differ from the US Eastern date by a day around a month end (Decision 3).

## Adversarial

The author's adversarial notes (scratchpad, `design/adversarial-home-options.md`), with the advisor review r1:

1. Gap against the goal, `no_rule` is not a page gap reason: adopted (advisor A2; Decisions 5, 6 use `no_data` and `unsupported_currency`; entry 2 item 4).
2. Gap against the goal, `+$0.00` against the register "Never a zero": adopted as a named deviation (Decision 8, for confirmation); the gap alternative is stated.
3. Gap against the goal, lots under the schema unit `structures`: adopted as a named deviation (Decision 3), no schema change by the owner's rule of this round.
4. Gap against the goal, card gap `textMaxLoss` beside a figure: adopted (A3; Decision 9, entry 2 item 5).
5. Review rows and double rounding: adopted (the review line is new work with a test, Decision 8 and entry 2 items 5 and 6; one rounding rule, Decision 7, entry 1 item 8 and `TestHalfCentRounding`).
6. Unparsable leg and non-USD currency: adopted (Decision 6, `Unreadable`, `OtherCurrency`, entry 1 items 6, 7, 9).
7. Simpler way, Sold count and Open premium from `LedgerPosition` alone: partly adopted: Sold count reads positions only; Open premium stays on structures because the net of a spread needs its legs and `Evaluate` is called for Max loss anyway, so no code is removed. `NetPaid` is dropped (the gap `no_data` says it).
8. Claim most likely false, the review line and `load.go` constants: adopted (Today and Left out corrected; Decision 10 names the zero `StructureParams` as safe instead of constants).
9. Debit spread is `other`: adopted (Doubts 2, Decision 5 `At least` caption).
10. Alias double count after account-identity: adopted as Doubts 3.
11. Fragile greps and undecidable items: adopted (entry 1 item 4 and entry 2 item 3 read import lists, `purity_test.go` passes unedited; no grep for symbols in comments; the golden is a new committed file compared byte for byte, `ok.golden.json` unedited and out of Files; the `OptionTiles` fake is entry 1 item 3).
12. Same-package tests and `home_page`: adopted by `git grep`: only `member.go`, its tests and golden and `cmd/ot/home_site.go` use `home_options` or the tile texts; `home_page` does not import it; `purity_test.go` needs no edit, so it is not in Files.
13. Advisor A1, A4, A5 and notes (positive `UsdTile` amount, no naked/unbounded wording, `docs/contracts/compute.md` in entry 1, as-of and all-accounts captions, JPY-only gap, remaining-part caption): adopted (Decisions 1, 4 to 6; entry 1 Files and item 2; entry 2 item 4).
