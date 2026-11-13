# household movers: the "What moved today" block gets its figure, and the "long-term account" block is dropped

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-13
Revision: 1

Source: the advisor's definitions 1 to 3 for household movers (2026-11-13T06:20Z, kept word for word as Decisions 1 to 10), and the owner's decision of 2026-11-13 (window, "a"): drop the household page's "long-term account" block.

Goal: after a session the owner sees which positions moved his account and by how much, the same day, from the stored marks of two capture days; the "long-term account" block is gone from the page, the web view and the register (the schema keeps it as an optional property). The front end computes nothing.

Existing design: `household-figures.md` and `report-global-assets.md` (merged) give the shape; no design owns movers. `docs/conventions/register.toml` has `household.movers` (written for the old Python page) and `household.long_term`.

## Today

All paths at origin/main.

- `internal/pages/household_page/builder.go:287` sets `p.Movers = moversGap(no_data, textNoMovers)` always; `moversGap` (`:328`) and `moversBlock` (`blocks.go:64`) have only `Rows`, `Totals` and two `[]emptyRow`. `builder.go:286` and `:364` set `p.LongTerm` (`blocks.go:186`); `strings.go:20,34,38,76` hold its texts; `:309` lists its gap in the run-status list. `moneyRow` (`blocks.go:27`) has no `scope`. The web view (`web/src/pages/household/AccountsMovers.tsx:129-132`) shows `rows` of the scope whose key is the alias, and `block.totals` only when the scope key is `household`; the page's scope key is the alias (`builder.go:61`), so `totals` would never show. The schema's `MoneyRow` has an optional `scope`.
- The schema (`contracts/schemas/page.household_page.v1.json`) already has `MoversBlock` (`columns` four heads, `stocks` and `options` of `MoverRow`, at most 60 each, `totals` at most 2 `MoneyCell`, `scope_gaps`) and `MoverRow` (`previous` and `current` are `PriceCell` or `Gap`, `amount` is `MoneyCell` or null, `note`, `scope`, `account`). `long_term` is defined at line 1696 and required at line 1830; the root has `additionalProperties: false`, so an emitted `long_term` fails a schema without it.
- Page schemas are not generated into Go: `contracts/generate.sh` generates `internal/contracts/gen/records.go` from `contracts/datasets` only, and `household_page` writes its own structs (`blocks.go`). `web/src/types/household_page.v1.types.ts` is a hand-written placeholder (its first line says so). `internal/contracts/fixtures_test.go:13,22` validates `web/fixtures/household-page/*.json` against the schema, so a fixture and the schema must agree at every merge. `contracts/schemas/carryover.sha256:6` pins the sha256 of the schema file and `internal/contracts/carryover_test.go` (`TestCarryoverManifest`) fails on any other hash; `docs/contracts/page-schemas.md:105` says the carried copies are unchanged except `x-timezone` and lists no required key.
- `store.AccountReader.Positions` and `Values` (`internal/store/api.go:783-786`, `internal/store/account/read.go:15-60`) return the rows of every capture of a session day, ordered by capture id; `positionmarks.newestCapture` (`internal/jobs/positionmarks/plan.go:28`) cuts them to the newest capture. `mark.Reader.Marks` (`internal/store/mark/api.go:51`) returns the rows of the newest capture of the day that has marks (highest `Attempt` per `ConID`), each with its `CaptureID`; a day without marks gives an empty slice.
- `readHeld` (`household_page/holdings.go:40-76`) reads positions, marks and the USDJPY rate for the capture day only. `HoldingFigures` (`blockapi/api.go:170`) is built from `Config.Holdings`, which `cmd/ot/household_site.go:35` already sets (`pagefig.NewHoldings`); only `blockapi/api_test.go:156` implements it outside `pagefig`. `docs/contracts/compute.md:116,140,152` document `HoldingFigures` (methods, version line, the both-sides test) and `docs/contracts/pages.md:59,78` document `household_page` and its tests.
- `docs/conventions/register.toml:606` (`household.movers`) and `:858` (`household.long_term`) name Python functions; both keep `source_hash = "pending-recompute"`. `internal/compute/rules/rules_test.go` reads the register: `TestHashesMatchSource` covers only the ids in `rules.json`, and `TestNoPendingForGoIds` (`:437`) demands `pending-recompute` for every other figure. No test reads `implemented_by` of a figure that is not in `rules.json`, so the register edit need not travel with the code.

## Decisions

Decisions 1 to 10 are the advisor's definitions, unchanged. Decisions 11 to 16 are this design's own.

1. Register id `household.movers` is kept. Its formula is the rule: line = q_prev x multiplier x (Price(D1) - Price(D0)), converted at the page's rate, half-even to 2 places; Other = the change of net liquidation less the listed lines. The entry's `implemented_by` is edited in entry 5 (see Doubt 1 for `source_hash`).
2. The two days are not the page day. D1 = the newest day on or before the page day that has a mark capture for the alias; D0 = the newest day before D1 that has a mark capture. No D1, or no D0: Gap `no_data` (the text of `strings.go:37` stays). D1 more than 5 calendar days before the page day, or D0 more than 5 calendar days before D1: Gap `stale_input` naming the day. The block names both days (`columns` heads `Previous (11-11)` and `Last (11-12)`, and data_times).
3. Inputs, all stored, no new collector: `account.position` of D0 and D1; `account.position_mark` through `Marks(alias, D0)` and `Marks(alias, D1)` (field `Price`); the USDJPY rate the holdings blocks use (`LatestFxRate` on or before D1); NetLiquidation of D0 and D1 from the ledger the builder already reads (`builder.go:145`).
4. A line: one per contract (conid) held at D0 with q_prev not 0 and still held at D1. amount = q_prev x multiplier x (Price(D1) - Price(D0)) in the position's currency, then to USD at the rate of Decision 3, half-even to 2 places, unit `usd_signed`. q_prev is signed. Stocks have multiplier 1. `previous` and `current` are the two prices to 4 places in the position's own currency. A quantity change between D0 and D1 does not change the line; its effect lands in Other.
5. Left to Other, never a line: a position opened after D0 or closed before D1, cash, interest, fees, deposits and withdrawals, the rate change on yen balances.
6. Tables `stocks` and `options`, each sorted by the absolute amount, largest first, at most 60 rows; the rest stays in Other. `note` = the position in words (`100 sh`, `short 2 put 240 11-20`), built by rule, no news text. `scope` = the page's account only.
7. Missing price: a line whose mark is absent on D0 or D1 has `amount` null and a Gap `missing_quote` in the price cell; it is not in the sum, so its effect is in Other. A currency other than USD or JPY: `unsupported_currency`.
8. Totals: total = NetLiquidation(D1) - NetLiquidation(D0), both in USD as the ledger stores them; Other = total - the sum of the lines with an amount. NetLiquidation missing on either day: the total and Other are Gap `missing_input`, the lines stay. A total is never built by adding the lines. (Decisions 15 and 17 add: a JPY value converts at the D1 rate of the lines; Total and Other are two rows of `rows`.)
9. Where: computed at page build in `internal/compute/pagefig` beside the household holdings figures, behind the interface the page already uses; nothing stored. The builder starts to emit `columns` and the `MoverRow` fields. `householdConfig` feeds both `ot page` and `ot serve`.
10. Left out, named: news reasons; household and daughter scopes; a split of an option's move into delta, theta and volatility; intraday moves; adjusted prices (an ex-dividend drop shows as a loss on the stock line, the dividend arrives in Other).
11. The figure is a method `Movers(MoversIn) MoversResult` of `blockapi.HoldingFigures`, not of `PageFigures`: `Config.Holdings` is already wired in `householdConfig`, so `cmd/ot` is not edited (`wire.go` is at 793 of 800 lines and is not touched), and no fake of `PageFigures` in another package changes. A `Config` with `Holdings == nil` keeps the block as it is today (Gap `no_data`, `textNoMovers`); with `Marks == nil` the same.
12. The pages module finds the days and the rows; `pagefig` does every sum, the sort and the rounding (`household_page/purity_test.go` forbids `math/big`, float and `sort` in the package). Days: the session days of `sessionDays` (150 calendar days back, at most 90 sessions) are read newest first, `Marks` per day, until D1 and then D0 are found; a day with no mark row is skipped. No new reader.
13. The row set of a day is the positions whose `CaptureID` is the `CaptureID` of that day's mark rows, and the NetLiquidation value is the row of that capture too, so a day with two captures never mixes them. A day whose mark capture has no NetLiquidation row gives the gap of Decision 8.
14. A JPY position with no stored rate has `amount` null and the Gap `missing_input` naming `USDJPY`; the rate is the one of D1 for both prices of the line.
15. NetLiquidation is stored in the account's currency (`household.go:109-125` shows a JPY value with a rate), so a JP alias holds yen. A value in JPY is converted to USD with the same D1 USDJPY rate as the lines (divide, half-even to 2 places); no rate: total and Other are the Gap `missing_input` naming `USDJPY`; a currency other than USD and JPY on either day: `unsupported_currency`. The lines stay in every case. Other holds the quantity, open, close and cash effects; a rate change on yen balances is in it only as far as one rate converts both values (Decision 5 stands: it is never a line).
16. Drop `long_term` (owner, 2026-11-13): the page, the web view and the register lose the block. Contracts change: `long_term` leaves `required` of `contracts/schemas/page.household_page.v1.json` and stays an optional property (the advisor's definition D: the schema field stays optional); entry 2 changes the schema and its hash line together. Deleting the property is Left out. Order: schema, then web (it no longer reads the key), then page (it no longer emits it), so the live page never reads a missing block.
17. Total and Other are two `MoneyRow` of `rows`, scope = the alias, `totals` stays empty (the web shows `rows` for the alias scope and `totals` only for a `household` scope). Names: `Change in net liquidation` and `Other`. `moneyRow` gains `Scope` in entry 3. A gap of Decision 8 or 15 is the block gap and the two rows are absent.
18. Marks: a line is built only when the D0 and D1 prices have the same `Basis` (mid, last or close) and neither is `none`; otherwise the price cells show the gap `missing_quote`. An option with an empty multiplier has `amount` null and the Gap `missing_input`, never 1; a stock's empty multiplier is 1 as `positionsIn` does.

## Modules

No row of `docs/design/modules.md` changes. compute (`internal/compute/**`, interface file `internal/compute/blockapi`), pages (`internal/pages/**`), contracts (`contracts/**`), web (`web/**`) and docs (`docs/**`; `docs/conventions/register.toml` is in the last row, it is not owner-only). `pages` already depends on `store`, `compute` and `contracts`; `web` on `contracts`.

## Interface

Module compute, `internal/compute/blockapi/api.go`; existing fields and signatures stay, types and one method are added.

```go
// MoverIn is one contract held on both days, all numbers decimal text. Quantity is q_prev, signed; Multiplier is
// 1 for a stock and empty only when the store has none; a price is empty when that day has no mark; BasisPrev and
// BasisLast are the mark bases of D0 and D1. Right is "put", "call" or "" (a stock).
type MoverIn struct {
	Name, SecType, Right, Strike, Expiry, Currency, Quantity, Multiplier string
	PricePrev, PriceLast, BasisPrev, BasisLast                          string
}

// MoversIn is the input of household.movers. Prev and Last are D0 and D1. NetPrev and NetLast are the
// NetLiquidation of each day, in their own Currency; a Value of "" is missing. UsdJpy is the D1 rate, empty
// without one.
type MoversIn struct {
	Prev, Last     contracts.Day
	UsdJpy         string
	NetPrev, NetLast ValueIn
	Rows           []MoverIn
}

// MoverLine is one line. Amount is USD, 2 places, signed, or a gap; PricePrev and PriceLast are 4 places in
// Currency, or the gap missing_quote. Note is the position in words.
type MoverLine struct {
	Name, SecType, Currency, Note string
	PricePrev, PriceLast, Amount  compute.FigureValue
}

// MoversResult holds the two tables, each by absolute amount (a line without an amount last), at most 60 lines
// each, and the total and Other of Decisions 8 and 15, both in USD.
type MoversResult struct {
	Stocks, Options []MoverLine
	Total, Other    compute.FigureValue
}
```

- `HoldingFigures` gains `Movers(MoversIn) MoversResult`; the contract text states Decisions 4 to 8, 14, 15 and 18. `HoldingsIn`, `RiskIn` and the others are unchanged.

`Contracts:` changed (entry 2 edits `contracts/schemas/page.household_page.v1.json`; no dataset or proto file; no generated Go changes, see Today).

## Issues

Build order: every entry compiles and its tests pass against main, and no entry needs an unmerged sibling. BASE is `git merge-base origin/main HEAD`. Entries 1 and 2 are independent; 4 follows 2; 3 follows 1, 2 and 4.

### 1 household-movers-figure (interface)

Module: compute. Size: M.
Order: first.
Files: `internal/compute/blockapi/api.go`, `internal/compute/blockapi/api_test.go`, `internal/compute/pagefig/holdings_new.go`, `internal/compute/pagefig/movers.go`, `internal/compute/pagefig/movers_test.go`, `docs/contracts/compute.md`
Design: docs/design/household-movers.md#Interface (Decisions 4 to 8, 11, 14, 15, 18)
Acceptance:
1. `api.go` declares `MoverIn`, `MoversIn`, `MoverLine`, `MoversResult` with the fields of the Interface section and `HoldingFigures.Movers`, no function body.
2. `api_test.go`: `fakeHoldings` implements `Movers`, and the new test `TestMoversInOutRoundTrip` fills every field of the four types through the fake and reads each back; every old keyed literal in the package tests compiles unedited.
3. `movers.go` holds the arithmetic with `math/big`, no float, no `time`, no I/O, under 400 lines; `holdings_new.go` has `holdingFigures.Movers` calling it; `holdings.go`, `household.go` and `exposure.go` are unchanged.
4. A line is q_prev x multiplier x (PriceLast - PricePrev), converted to USD with `UsdJpy` for JPY (divide) and as is for USD, rounded half-even to 2 places; a short put with q_prev -2, multiplier 100, price 3.10 to 2.40 gives 140.00; a JPY row without a rate has the Amount gap `missing_input`; any other currency has `unsupported_currency`; an empty price, a `none` basis, or two different bases (`BasisPrev` not equal to `BasisLast`) give the gap `missing_quote` in the price cells and in Amount; an option with an empty Multiplier has the Amount gap `missing_input`, a stock with an empty Multiplier uses 1.
5. `Total` is NetLast minus NetPrev, each converted to USD as Decision 15 says (JPY divided by `UsdJpy`), and `Other` is Total minus the sum of the Amounts that are numbers (lines cut by the 60-row limit are in Other); an empty NetPrev or NetLast, or a JPY value without a rate, gives `missing_input` for both; a currency other than USD and JPY in either gives `unsupported_currency` for both; the lines stay in all cases.
6. Each table is sorted by the absolute Amount, largest first, ties by Name, a gap last, and holds at most 60 lines; `Note` is `100 sh` for a stock and `short 2 put 240 11-20` for an option (the strike without trailing zeros, the expiry as month-day).
7. `movers_test.go` is new and names `TestMoverLineShortPut`, `TestMoverMissingQuoteLeavesOther`, `TestMoverBasisMismatchIsGap`, `TestMoverOptionEmptyMultiplierIsGap`, `TestMoverJpyNeedsRate`, `TestMoversTotalIsNetChange`, `TestMoversNetJpyConverted` (yen 15000000 and 15150000 at 150 give 100000.00 and 101000.00, Total 1000.00), `TestMoversNetMissingGap`, `TestMoversNetOtherCurrencyGap`, `TestMoversSortAndLimit` (61 stock lines: 60 returned, the smallest in Other) and `TestMoverNotes`; every test calls `t.Parallel()`.
8. `docs/contracts/compute.md` names `Movers` in the `HoldingFigures` line (116), the version line (140) and the both-sides test line (152), one line each.
9. `git diff --name-only BASE` lists only paths of the Files line; `git diff BASE -- internal/pages cmd` is empty.

### 2 household-long-term-optional (interface)

Module: contracts. Size: S.
Order: first.
Files: `contracts/schemas/page.household_page.v1.json`, `contracts/schemas/carryover.sha256`
Design: docs/design/household-movers.md (Decision 16)
Acceptance:
1. In `contracts/schemas/page.household_page.v1.json` the `required` array has no `long_term`, the `properties` still define `long_term` as `MoneyBlock`, and no other line of the file changes.
2. Line 6 of `contracts/schemas/carryover.sha256` is the only line of that file that changes and holds the sha256 of the new schema file under the same path (checked by `TestCarryoverManifest`, see Checked by CI).
3. A document with and a document without `long_term` both validate (`internal/contracts/fixtures_test.go` reads the unchanged fixtures, which still have the key).
4. `docs/contracts/page-schemas.md` lists no required key of this schema (`git grep -n long_term -- docs/contracts` prints nothing at BASE), so it is not edited.
5. `git diff --name-only BASE` lists only the Files paths; `git diff BASE -- internal web` is empty.

### 3 household-page-movers

Module: pages. Size: L.
Order: after 1, 2 and 4.
Files: `internal/pages/household_page/blocks.go`, `internal/pages/household_page/builder.go`, `internal/pages/household_page/movers.go`, `internal/pages/household_page/strings.go`, `internal/pages/household_page/builder_test.go`, `internal/pages/household_page/holdings_test.go`, `internal/pages/household_page/movers_test.go`, `internal/pages/household_page/testdata/holdings.golden.json`, `docs/contracts/pages.md`
Design: docs/design/household-movers.md (Decisions 2, 3, 6, 9, 11 to 13, 16 to 18)
Acceptance:
1. `movers.go` (new, under 400 lines) reads, through `Config.Marks`, `Config.Account`, `Config.Fx` and `Config.Holdings` only, the days of Decision 12 and the rows of Decision 13, and calls `Holdings().Movers` once; `git grep -n "Account.Positions" -- internal/pages/household_page` counts 2 over the non-test files (the capture day in `holdings.go`, unchanged, and one helper in `movers.go` that reads a mark-capture day and is called for D1 and for D0); `holdings.go` is not edited.
2. D1 and D0 follow Decision 2: no mark row on any session day, or only one mark day, gives Gap `no_data` with `textNoMovers`; a D1 more than 5 calendar days before the page day, or a D0 more than 5 calendar days before D1, gives Gap `stale_input` whose text names that day; otherwise `columns` is `Name`, `Previous (MM-DD of D0)`, `Last (MM-DD of D1)`, `Moved (USD)`, and `data_times` holds two entries more, sources `position_mark_prev` and `position_mark_last`, each the newest `MarkedAt` of that day's mark rows.
3. A contract held on D0 only or D1 only, or with q_prev 0, has no line (it is in Other); a reader error is an error of Build, never a gap; `Holdings == nil` or `Marks == nil` keeps today's `no_data` movers block. An option row is passed with its stored `Multiplier` as it is (empty stays empty), and each row carries the `Basis` of both days' marks.
4. `moneyRow` in `blocks.go` gains `Scope string` (`json:"scope,omitempty"`, so no other block changes). `moversBlock` has `Columns`, `Stocks` and `Options` of a `moverRow` (name, previous and current as a price or a gap, amount or null, note, scope = the alias, account tag), `Rows` of two `moneyRow` named `Change in net liquidation` and `Other` with `Scope` = the alias (the total first), and `Totals` empty (`[]`, not omitted); with the gap of Decision 8 or 15 the block carries that Gap and `Rows` is empty. `Stocks` and `Options` are copied in the order `pagefig` returns; the package imports no `sort` (`purity_test.go` holds unchanged).
5. The builder text for a movers gap, the column heads, the two row names and the day names are defined in `strings.go`, English only; `git grep -n -P "[^\x00-\x7F]" -- internal/pages/household_page/strings.go internal/pages/household_page/movers.go` prints nothing.
6. `long_term` is gone from the package: `git grep -n -i "long_\?term" -- internal/pages/household_page` prints nothing; `pageJSON` has no `LongTerm`; the run-status list at `builder.go:309` has no long-term entry, so a page whose blocks all have no gap is `ok`.
7. Tests, each edit named: (a) `holdings_test.go`: `fakeMarks` (`:36-45`) gains `byDay map[string][]mark.PositionMark`, a day in `byDay` answers its rows and any other day answers `rows`; the golden test gives `byDay` rows for 2026-11-10 (the rows it uses today) and for the previous session day of its fixture with other prices, so the golden movers block has lines; (b) the assertion at `holdings_test.go:88` (one `Marks` read and one `Positions` read, both `main 2026-11-10`) becomes an exact ordered list of the reads of `marks.asks` and `acc.asked`: the capture-day read first, then D1, then D0; (c) `holdings_test.go:81` and `builder_test.go:260` no longer expect `long_term: not_built`, and `holdings_test.go:81-83` expects the movers block of the golden page to have no gap; (d) the loop at `builder_test.go:193` (gap page for an unknown alias) is unchanged, its `movers` expectation `Unknown account` still holds; (e) every other test Config in `builder_test.go` that sets `Marks` and `Holdings` is run against a fake that answers no day (`byDay` empty and `rows` nil), and its movers block is expected to be `no_data` with `textNoMovers`.
8. `movers_test.go` is new and names `TestMoversDaysNotPageDay` (page day 11-13, marks on 11-10 and 11-12: D1 is 11-12, D0 is 11-10), `TestMoversNoDataOneDay`, `TestMoversStaleD1`, `TestMoversStaleD0`, `TestMoversHeldBothDaysOnly`, `TestMoversCaptureNotMixed` (two captures on D0: only the rows of the capture of the marks are used), `TestMoversMissingMarkGap`, `TestMoversBasisMismatch`, `TestMoversJpyRate`, `TestMoversRowsHaveAliasScope` (two rows, scope = the alias, `totals` is `[]`), `TestMoversDataTimesNameBothDays` and `TestMoversPageValidatesSchema` (the built page validates against `page.household_page.v1`, the `long_term` key absent); every test calls `t.Parallel()` and uses a fake store and `t.TempDir()` where it writes.
9. `testdata/holdings.golden.json` is regenerated by its test and has no `long_term` key and a `movers` block with `columns`, two `rows` and the stocks of the second fake day.
10. `docs/contracts/pages.md` line 59 (the `household_page` entry) says movers is built from two mark days and `long_term` is not emitted, and line 78 names the new tests (`movers_test.go`), each marked new.
11. `git diff --name-only BASE` lists only paths of the Files line; `cmd`, `web` and `contracts` are not edited.

### 4 household-web-long-term

Module: web. Size: S.
Order: after 2.
Files: `web/src/pages/household/HouseholdPage.tsx`, `web/src/types/household_page.v1.types.ts`, `web/fixtures/household-page/ok.json`, `web/fixtures/household-page/gap.json`, `web/fixtures/household-page/stale.json`, `web/fixtures/household-page/bad-version.json`, `web/test/pages/household/householdPage.test.tsx`
Design: docs/design/household-movers.md (Decision 16)
Acceptance:
1. `git grep -n -i "long_\?term" -- web` prints nothing.
2. `HouseholdPage.tsx` renders no cell named `long_term` and the grid has no empty place left by it; the type file has no `long_term` field; the four fixtures have no `long_term` key and still validate against the schema of entry 2, where the property is optional (`internal/contracts/fixtures_test.go` reads them); the expected-block list in `householdPage.test.tsx:43` has no `long_term`.
3. The entry builds and renders while Go still emits `long_term`: the web does no runtime schema validation (`git grep -n -i "ajv" -- web/src` prints nothing), so the extra key in a live payload is ignored.
4. `web/src/pages/household/AccountsMovers.tsx` is not edited: it shows `rows` of the scope whose key is the alias, which is where entry 3 puts Total and Other; no web test is added, because `web/fixtures/household-page/ok.json` already has a `rows` entry with a scope (Other).
5. `git diff --name-only BASE` lists only paths of the Files line.

### 5 household-register

Module: docs. Size: S.
Order: after 3.
Files: `docs/conventions/register.toml`
Design: docs/design/household-movers.md (Decisions 1, 15, 16)
Acceptance:
1. The `household.movers` entry (version 1 kept) has a formula that states Decisions 2, 4, 8 and 15 (the two days, q_prev x multiplier x price change, Other = total - lines, a JPY NetLiquidation converted at the D1 rate of the lines), inputs, `period_date`, `excludes` and `gap_text` in the words of this design, `implemented_by` listing `internal/compute/pagefig.movers` and `internal/pages/household_page.moversBlock` and no `option_trader.` name, and a new `history` line dated 2026-11-13.
2. `source_hash` of `household.movers` is still `pending-recompute` (`TestNoPendingForGoIds` in `internal/compute/rules/rules_test.go` demands it for a figure that is not in `rules.json`).
3. The entry `household.long_term` is removed with its `[[figure]]` header; `git grep -n "household.long_term\|household_page.long_term"` prints nothing.
4. `git diff --name-only BASE` lists only the Files path.

## Checked by CI

`go test ./internal/compute/... ./internal/pages/... ./internal/contracts/... -race -count=1` (this includes `TestCarryoverManifest`, `TestNoPendingForGoIds` and the fixture validation), `go run ./scripts/arch --check`, `go build ./...`, `golangci-lint`, the contracts generation check (no output changes expected), and the web workflow (`pnpm install --frozen-lockfile`, `tsc`, `eslint`, `vitest`, build). The review runs no test.

## Left out

- The items of Decision 10 (news reasons, household and daughter scopes, option move split, intraday moves, adjusted prices).
- Deleting the `long_term` property from the schema: the advisor's definition keeps it optional, and a delete costs a second hash edit and an ordering edge; if the owner wants it gone, one contracts entry after entries 3 and 4 does it, with `carryover.sha256`.
- Index and other sec types: marks exist, but there is no table for them; a position of such a type has no line (its effect is in Other).
- Days that are not XNYS session days: `sessionDays` reads the XNYS calendar, so a mark capture on a US holiday is never found.
- `cmd/ot`: no reader is missing (`Marks`, `Account`, `Fx`, `Holdings` are set in `householdConfig`), so `wire.go` and `household_site.go` are not edited.
- `report_blocks` and `report_page` movers (other pages with their own `movers` gap), `home_household`, `positions_page`.
- A faster way to find a mark day than reading `Marks` per session day (a new reader); the scan is at most 90 reads.

## Doubts

1. Definition 1 says `source_hash` is edited. `TestNoPendingForGoIds` fails any figure that is not in `rules.json` and has a hash, and a hash is only computed for a `rules.json` id. The design keeps `pending-recompute`; the definition's wording needs the advisor's or the owner's word only if `household.movers` should become a rule.
2. Definition 8 says NetLiquidation is "in USD as the ledger stores them". The code stores the account's currency (`household.go:109-125`), so a JP alias holds yen. The design keeps Decision 8 for USD and converts a JPY value at the D1 rate of the lines (Decision 15, advisor review r1 point 4). The ledger may hold one NetLiquidation row per currency; `tagValue` takes the first, as the page does, and this is not verified.
3. Decision 3 reads D0 and D1 "from the ledger the builder already reads (builder.go:145)". That read is the newest capture of the page's own days only (`capture`, `builder.go:140-175`); movers needs `Values` of D0 and D1 again (a second read of two days, entry 3). `Positions` and `Values` return every capture of a day, so the design pins both to the `CaptureID` of the marks (Decision 13).
4. Verified from code: (a) `account.position` is insert-only (`internal/store/account/store.go:23-36`, no UPDATE or DELETE on `account.*` anywhere), so D0's rows stay as stored, but a day may hold several captures, hence Decision 13; (b) no job captures marks on a Flex-only day: `account.position` is written only by `accountcapture` (gateway) and `cutover/accountimport`, and `positionmarks` marks the newest capture of the clock's market day and logs and skips an alias without a capture (`run.go:200-210`); a Flex import writes only `FlexTradeRow`s, so such a day has no mark row and is skipped by D1 and D0; (c) NetLiquidation is stored in the account currency, not always USD (Doubt 2).
5. A stale D1 is found only inside the session list (150 calendar days, 90 sessions); older than that the block says `no_data`, not `stale_input`.
6. The schema's `totals` takes `MoneyCell` only and the web shows it for a `household` scope only, so Total and Other are `rows` (Decision 17); a gap of Decision 8 or 15 is the block gap and the two rows are absent, not two cell gaps.
7. A JP holiday or Asian-day capture: which calendar `master.session_day` uses and which market day `positionmarks` stamps is not verified (see Left out, non-XNYS days).

## Adversarial

Sources: adversarial-household-movers.md in the scratchpad `design` folder (and the advisor review r1, household-movers-review-r1.md).

1. Total and Other not shown by the web (`AccountsMovers.tsx:132`), and `totals` cells all labelled with the block title: adopted. Both go to `rows` with scope = the alias, distinct names, `moneyRow.Scope` added (Decision 17, entries 1 and 3); `totals` stays empty and the web file is not edited.
2. `moneyRow` has no `scope`, `moneyCell` no `tone`: `scope` adopted (entry 3 item 4); `tone` not adopted, the schema's `tone` is optional and the figure's sign shows in the text.
3. `data_times` names both days with no test: adopted (entry 3 items 2 and 8, `TestMoversDataTimesNameBothDays`).
4. Entries 2 and 5 fail `TestCarryoverManifest` (`carryover.sha256`): adopted for entry 2 (Files and item 2); entry 5 is dropped.
5. `page-schemas.md:105` needs a line: not adopted, it lists no required key and the carried schema keeps its path; named in entry 2 item 4.
6. Entry 3 before entry 4 crashes the live page (`HouseholdPage.tsx:252`): adopted, entry 3 is after 1, 2 and 4; entry 4 builds with Go still emitting the key (entry 4 item 3).
7. Mixed mark bases (mid against close): adopted (Decision 18, entry 1 items 4 and 7, entry 3 item 3).
8. Empty option multiplier read as 1: adopted (Decision 18, entry 1 item 4).
9. Other dominates on expiry and assignment days, no check shows it: not adopted, Decision 5 names it and the definition says it stays in Other; a check would need a second figure.
10. JPY-base NetLiquidation gives a permanent gap: adopted (Decision 15, entry 1 item 5).
11. Index and other sec types, and non-XNYS days: adopted as Left out.
12. Drop the schema-property delete (entry 5): adopted, Left out names it.
13. Advisor r1: entry 1 Files miss `docs/contracts/compute.md`: adopted (entry 1 item 8); `docs/contracts/pages.md` goes to entry 3 (item 10).
14. Advisor r1 and review of tests: false cites at `builder_test.go:193` and `holdings_test.go:88`, fakes and golden per day: adopted (entry 3 item 7).
