---
type: design
title: "option P&L totals: /options/pnl shows, per account and currency, the cash you would have to pay if every open short put were assigned"
status: accepted
date: "2026-11-27"
need: [none]
none_reason: "plan P2 (sell-put desk) step 6 of docs/design/option-attribution.md Left out; owner goal 2026-11-13 (option seller: know the cash a put can call); no need record exists"
---

# option P&L totals: /options/pnl shows, per account and currency, the cash you would have to pay if every open short put were assigned

Author: the optdesk designer
Owner approved: (pending)
Revision: 1
Plan: P2

摘要：`docs/design/option-attribution.md` 的 Left out 把"月度块、按公司合计、被行权需付现金、券商未实现盈亏对账"列为后面的设计。读 origin/main 之后：月度块和按公司合计已经落地（`docs/design/options-pnl-ledger.md`，页面 `Account.ledger`，`buildLedger`），所以这份设计不再做它们，只做缺的那一件：每个账户、每个币种一行"若全部被行权需付现金"（每个未平仓卖出看跌期权的 行权价 x 合约乘数 x 张数，相加，币种之间不换算）。这个数是计算，放在 compute（`optionattrapi.Booker` 加一个方法），页面只写文字，前端只显示。第三层对账（券商未实现盈亏对账我们的中价盈亏）不做，写进 Left out。

Source: `docs/design/option-attribution.md` Left out lines 1 and 2 (the monthly block, per-company sums, cash due if assigned, layer 3); `docs/design/options-pnl-ledger.md` (the ledger block, landed); `internal/compute/optionnotice/cash.go` `cashByDate` (the same sum by expiry day on the notice page: this design does not reuse it, see Decision 2); the plan "卖 put 桌面（P2）" 持仓期间 section. Read from origin/main 93dd0f49.

Needs the owner: nothing. No outside service, key or account, no migration, no write.

## What you see

On `/options/pnl`, under each account's book cards, a line "若全部被行权需付现金 (cash due if assigned)" with one figure per currency, for example `$38,000.00 · 2 张合约 · 1 个仓位`. A short Chinese hint next to it says what the number is: "卖出的看跌期权（put）若全部在今天被行权，你要按行权价买入股票，需要的现金；不含已收的权利金，不换算币种". An account with no open short put shows no line. The ledger block, the book cards and the lot cards are unchanged.

## Frame

What is on origin/main today:

- The monthly block and the per-company sums exist: `Account.ledger` has the views `by_month` and `by_company` (`internal/pages/option_pnl_page/ledger.go` `buildLedger`, design `options-pnl-ledger.md`). The line of `option-attribution.md` that lists them as later designs is out of date; this design records that and builds nothing for them.
- No figure of cash due exists on the page. `grep -rn "cash due" internal/pages/option_pnl_page` prints nothing; the schema `page.option_pnl_page.v1.json` has no such field.
- The sum exists once, inside the notice page: `optionnotice.cashByDate` sums `exposure()` (strike x multiplier x contracts) of the short puts per expiry day and currency. It is private to that package and is keyed by day.
- `pnlLot` carries `compute.LedgerPosition` with `Strike`, `Multiplier`, `Quantity` (decimal text, negative for a short), `Currency`, `Right`, `Status`; `card` is true for every open lot, so every open lot the page selects (short puts; bought puts are skipped and counted) is a candidate. `Config.Booker` is the injected `optionattrapi.Booker`; `cmd/ot` passes `optionattr.NewBooker()` already.
- Pages must not compute (modules.md "pages: computing" is forbidden), so the sum cannot be written in `option_pnl_page`.

## Decisions

### Decision 1: one new line per account and currency, not a new block

`Account` gains the optional `cash_due`: an array of the existing `Line` (id, label, text, hint, tone), one per currency in order of first appearance, empty or absent when the account has no open short put. Reasons: `Line` already carries label, final text, a hint and a tone, so the web renders it with the component that renders lines; no new def is needed; the figure is a total of the account, not of one currency book of the ledger, so it does not go in `Book` (which the ledger groups also use and which would then repeat the figure under every month).

### Decision 2: a new method on `Booker`, own sum

`Booker` gains `CashIfAssigned(puts []OpenPut) ([]CashDue, error)`. `OpenPut` has `Currency`, `Strike`, `Multiplier`, `Quantity` (decimal text; the sign of a short is dropped). `CashDue` has `Currency`, `Lots int`, `Contracts` (exact decimal text) and `Cash` (exact decimal text, not rounded): the sums per currency of 1, of the absolute quantity and of strike x multiplier x absolute quantity, in order of first appearance. A text that is not a positive number returns `ErrBadInput`. A new method on the interface the page already holds needs no `cmd/ot` change and no new `Config` field; the cost is that fakes of `Booker` in tests of compute and pages gain the method. Own implementation, because `optionnotice.cashByDate` is private, is keyed by expiry day and takes the notice module's input type; compute packages do not import each other's implementation, and the rule is five lines on `shopspring/decimal`, which `optionattr` already uses.

Every implementation and fake of `Booker` on origin/main 93dd0f49 (searched with `git grep -n "Combine(rs"`): the real one, `booker` in `internal/compute/optionattr/book.go` (the new method goes in the new file `cashdue.go` of the same package); the fakes `fakeBooker` and `allBooker` in `internal/compute/optionattrapi/api_test.go`; the fakes `fixedBooker` (`builder_test.go`), `sumBooker` (`context_test.go`) and `allBooker` (`ledger_test.go`, which embeds `sumBooker`) in `internal/pages/option_pnl_page`. Nothing else outside compute and pages implements it (`cmd/ot` tests use `optionattr.NewBooker()`), so entry 1's Files cover both sides (the compute fakes and the three page fakes, each gaining a stub, so entry 1 compiles and passes CI alone); no narrow separate interface is needed. Reuse of the notice computation was looked at: it lives in compute (`optionnotice.cashByDate`, private, over `NoticePosition` rows from the broker, result `compute.CashDay` per expiry day and currency). The page would have to add its day rows to get one figure per currency, and adding is computing, which pages may not do; extending it would put a second output on `NoticeResult` for the notice page's input, which the P&L page does not hold (it holds ledger lots). So the sum is repeated in `optionattr` on the page's own input, five lines.

Other ways, not taken: a new `Config` field and a new interface (a second constructor to wire in `cmd/ot`, an entry in module ot); exporting `cashByDate` (an interface change of `optionnotice` and its Booker-unrelated types); computing in pages (forbidden).

### Decision 3: the figure is the cash for the open lots the page selects, nothing else

The page maps every open lot of the account with `Right` P and a negative `Quantity` to an `OpenPut`, whatever its mark: lots with no mark (`no_mark` gap cards, for example a JP contract with no quote) count too, since the rule needs no mark. The multiplier is the stored one, `LedgerPosition.Multiplier` (as the notice page takes it from the held position); no constant 100 or other default is in the page or in compute, and an empty or non-numeric multiplier is `ErrBadInput`, which Decision 3's muted line shows. Premium received is not subtracted and nothing is converted between currencies. Expiry day, the account's cash and a shortfall are not shown here: the notice page has them by date. A lot whose figures `CashIfAssigned` refuses makes that account's `cash_due` one line with the text "无法计算：个别仓位的行权价或张数不是数字" and tone `muted`; the page is still built.

### Decision 4: Chinese label and hint, English beside the label

The label reads "若全部被行权需付现金 (cash due if assigned)", the hint is the Chinese gloss of "What you see"; both are constants in `zh_texts.go`, the door for Chinese in page code. The web shows text as given and computes nothing.

## Modules

Five modules, the four of `options-pnl-ledger.md`: compute (`internal/compute/optionattrapi/**`, `internal/compute/optionattr/**`, `docs/contracts/compute/optionattr.md`), contracts (`contracts/schemas/page.option_pnl_page.v1.json`, `web/src/generated/option_pnl_page.v1.ts`, `internal/contracts/option_pnl_page_v1_test.go`, `docs/contracts/page-schemas.md`), pages (`internal/pages/option_pnl_page/**`, `docs/contracts/pages.md`) web (`web/src/pages/optionpnl/**`, `web/test/pages/optionpnl/**`) and docs (`docs/design/option-attribution.md`, entry 5; no draft or issue in the drafts directory or in `crew ask optdesk list` is open on that file: the only issue draft that names it in Files is `design-p2-compute-issue.md`, an old design-fix entry that is not listed as open). No row of `docs/design/modules.md` changes and no dependency is added: pages already imports `optionattrapi`, web imports generated types only.

## Interface

Entry 1 (compute, interface `internal/compute/optionattrapi/api.go`, declarations only; the sum is a compute rule, so the interface entry lands first):

    // OpenPut is one open short put lot: decimal texts; Quantity may carry the sign of a short.
    type OpenPut struct{ Currency, Strike, Multiplier, Quantity string }

    // CashDue is the sum for one currency: Lots puts, Contracts = sum of |Quantity|,
    // Cash = sum of Strike x Multiplier x |Quantity|; exact, not rounded.
    type CashDue struct {
        Currency  string
        Lots      int
        Contracts string
        Cash      string
    }

    // on Booker:
    // CashIfAssigned adds the puts per currency in order of first appearance; no puts gives a
    // non-nil empty slice; a text that is not a positive number gives an error wrapping ErrBadInput.
    CashIfAssigned(puts []OpenPut) ([]CashDue, error)

Entry 2 (contracts): `Account` gains the optional `cash_due` (array of `#/$defs/Line`, not in `required`); `additionalProperties: false` stays; `schema_version` stays 1. `web/src/generated/option_pnl_page.v1.ts` is regenerated with the schema, never edited by hand (`cash_due?: Line[]`).

Entries 3, 4 and 5 declare nothing public.

## Failure paths

- No open short put in the account: no `cash_due` key, no line.
- Two currencies: two lines, never added.
- An unparsable figure: the muted line of Decision 3; other accounts are unaffected.
- No stored mark for a lot: the lot is still in the sum.
- The ledger or the lot cards fail to build: as before; the cash line is built apart and does not change their outcome.

## Issues

Build order: entries 1 and 2 are interface entries and land first, in parallel (no shared file); entry 3 needs both; entry 4 needs entry 2; entry 5 (docs) follows entry 3. No `ot` entry (Decision 2), no migration, nothing holds the auto-deploy. Companies in tests and fixtures are AAA, BBB, CCC with invented numbers; a test names a Chinese constant and never repeats its text.

### 1 option-cash-due-booker (interface)

Module: compute, plus the three fake `Booker` test files of pages (a judged two-module interface entry: the interface in `optionattrapi` and its implementation in `optionattr`, one package pair, the contract test runs against the fake and the real `NewBooker`; the stub `CashIfAssigned` in the page fakes lands with the interface so that `internal/pages/option_pnl_page` still compiles).
Size: S.
Order: first.
Files: `internal/compute/optionattrapi/api.go`, `internal/compute/optionattrapi/api_test.go`, `internal/compute/optionattr/cashdue.go` (new), `internal/compute/optionattr/cashdue_test.go` (new), `internal/compute/optionattr/testdata/cashdue.json` (new), `docs/contracts/compute/optionattr.md`, `internal/pages/option_pnl_page/builder_test.go`, `internal/pages/option_pnl_page/context_test.go`, `internal/pages/option_pnl_page/ledger_test.go`
Contracts: unchanged
Design: docs/design/option-pnl-totals.md#Interface
Plan: P2
Acceptance:
1. `api.go` declares `OpenPut`, `CashDue` and the method `CashIfAssigned` of `Booker` with docs and no function body (all three names are new).
2. `api_test.go` has `TestFakeBookerCashIfAssigned` (new) (every fake `Booker` of the file answers `CashIfAssigned`, and a caller reads a `CashDue` of two currencies) and the existing fakes still compile.
3. `cashdue.go` (new) implements `CashIfAssigned` on `NewBooker()`: per currency in order of first appearance, `Lots` the number of puts, `Contracts` the exact sum of the absolute quantity, `Cash` the exact sum of strike x multiplier x absolute quantity, no rounding, no `float64`, no clock; no puts gives a non-nil empty slice; a strike, multiplier or quantity that is not a positive number returns an error wrapping `ErrBadInput` and no partial answer.
4. `cashdue_test.go` (new) reads `internal/compute/optionattr/testdata/cashdue.json` (new) with invented numbers and has `TestCashIfAssignedGolden` (new) (one currency, multiplier 100, a lot of quantity "-2" at strike 190 and a lot of "-3" at strike 38 give `Cash` "49400", `Contracts` "5" and `Lots` 2), `TestCashIfAssignedTwoCurrencies` (new) (USD and JPY lines, never added, order of first appearance), `TestCashIfAssignedShortSignDropped` (new) (quantity "-3" and "3" give the same line), `TestCashIfAssignedExactDecimals` (new) ("0.1" and "0.2" contracts give "0.3"), `TestCashIfAssignedNoPuts` (new) and `TestCashIfAssignedBadInput` (new) (each of strike, multiplier and quantity "abc", "0" and "" gives `ErrBadInput`); each calls `t.Parallel()`.
5. `docs/contracts/compute/optionattr.md` gains one paragraph for `CashIfAssigned`: what it adds, that the sign of a short is dropped, that premium and conversion are not part of it, and that the result is exact and unrounded. The text named here is new.
6. No file outside the Files line changes (the three page test files only by acceptance item 7), and the existing cases of the files `book.json` and `golden.json` in `internal/compute/optionattr/testdata` keep their expected output.
7. In `builder_test.go`, `context_test.go` and `ledger_test.go` of `internal/pages/option_pnl_page`, each fake `Booker` (`fixedBooker`, `sumBooker`, `allBooker`) gains a stub `CashIfAssigned` that returns no lines and no error, and nothing else in those files changes.
8. The result matches docs/design/option-pnl-totals.md#Interface.

### 2 option-pnl-cash-due-schema (interface)

Module: contracts.
Size: S.
Order: first.
Files: `contracts/schemas/page.option_pnl_page.v1.json`, `web/src/generated/option_pnl_page.v1.ts`, `internal/contracts/option_pnl_page_v1_test.go`, `docs/contracts/page-schemas.md`
Contracts: `contracts/schemas/page.option_pnl_page.v1.json` changes; the source and the generated output land in the same pull request.
Design: docs/design/option-pnl-totals.md#Interface
Plan: P2
Acceptance:
1. `Account` in `contracts/schemas/page.option_pnl_page.v1.json` has the optional property `cash_due`, an array whose items are `#/$defs/Line`; `cash_due` is not in the `required` of `Account`, `additionalProperties` stays false and `schema_version` stays 1.
2. `web/src/generated/option_pnl_page.v1.ts` is the regenerated output of that schema (`cash_due` appears as an optional array of `Line`) and is not edited by hand.
3. `option_pnl_page_v1_test.go` has `TestCashDueAccepted` (new) (a page document with `cash_due` of one `Line` validates), `TestCashDueOptional` (new) (the existing sample without the key still validates) and `TestCashDueRejectsUnknownKey` (new) (a `cash_due` item with an extra key is refused); each calls `t.Parallel()`, and the existing tests of the file keep their meaning.
4. `docs/contracts/page-schemas.md` says, in the entry of `page.option_pnl_page.v1`, that `Account.cash_due` is the optional list of lines "cash due if assigned", one per currency, absent when the account has no open short put. The text named here is new.
5. No file outside the Files line changes.
6. The result matches docs/design/option-pnl-totals.md#Interface.

### 3 option-pnl-cash-due-page

Module: pages.
Size: M.
Order: after 1 and 2.
Files: `internal/pages/option_pnl_page/cashdue.go` (new), `internal/pages/option_pnl_page/cashdue_test.go` (new), `internal/pages/option_pnl_page/builder.go`, `internal/pages/option_pnl_page/zh_texts.go`, `internal/pages/option_pnl_page/testdata/ok.golden.json`, `internal/pages/option_pnl_page/testdata/ledger.golden.json`, `internal/pages/option_pnl_page/testdata/lot_gap.golden.json`, `internal/pages/option_pnl_page/testdata/no_quotes.golden.json`, `docs/contracts/pages.md`
Contracts: unchanged
Design: docs/design/option-pnl-totals.md#Decisions
Plan: P2
Acceptance:
1. For an account with open short put lots, `Build` writes `cash_due` with one line per currency, in order of first appearance, whose text carries the money of `Cash` (through the page's `money` function), the contracts and the lots; the figures come from `Config.Booker.CashIfAssigned` and the page adds, divides and rounds nothing itself.
2. An open lot with no stored mark (a JP contract with no quote) is in the sum, and a lot whose multiplier is not the usual 100 is summed with its stored multiplier; an ended lot, a bought put and a lot of another account are not; two currencies give two lines.
3. An account with no open short put has no `cash_due` key.
4. A refused figure (`ErrBadInput`) gives the one muted line of Decision 3 for that account, the page is still built and the other accounts and the ledger are unchanged.
5. The label and the hint are string constants in `zh_texts.go` (new names), the label with the English "cash due if assigned" beside it; `TestCashDueLineConstants` (new) names them and never repeats their text.
6. `cashdue_test.go` (new) has `TestCashDueOneCurrency` (new), `TestCashDueTwoCurrencies` (new), `TestCashDueNoMarkLotCounts` (new), `TestCashDueNoOpenPut` (new), `TestCashDueBadInputMuted` (new) and `TestCashDueFromBooker` (new) (a fake `Booker` answers a `CashDue` and the line shows exactly that `Cash`, so the page does not sum); each calls `t.Parallel()`.
7. `ok.golden.json` gains the `cash_due` of the sample (AAA and CCC open) and the other three goldens are unchanged apart from what their open lots give; every figure of the existing cards and of the ledger is as before.
8. `docs/contracts/pages.md` gains the line of version v23 for `cash_due` (design `docs/design/option-pnl-totals.md`). The text named here is new.
9. The result matches docs/design/option-pnl-totals.md#Decisions.

### 4 option-pnl-cash-due-web

Module: web.
Size: S.
Order: after 2.
Files: `web/src/pages/optionpnl/OptionPnlPage.tsx`, `web/src/pages/optionpnl/CashDueBlock.tsx` (new), `web/test/pages/optionpnl/OptionPnlPage.test.tsx`, `web/test/pages/optionpnl/CashDueBlock.test.tsx` (new), `web/test/pages/optionpnl/fixtures/ok.ts`
Contracts: unchanged
Design: docs/design/option-pnl-totals.md#Decisions
Plan: P2
Acceptance:
1. `CashDueBlock.tsx` (new) renders each line of `account.cash_due` with its label, text and hint exactly as given, and renders nothing when `cash_due` is absent or empty; it computes no figure (no addition, no formatting of numbers).
2. `OptionPnlPage.tsx` shows the block under the book cards of each account, before the ledger block.
3. `CashDueBlock.test.tsx` (new) has `shows label text and hint as given`, `renders nothing without cash_due` and `one row per currency`; `OptionPnlPage.test.tsx` has `shows the cash due block under the books` (all four new); the `ok.ts` fixture gains a `cash_due` of one USD line.
4. The result matches docs/design/option-pnl-totals.md#Decisions.

### 5 option-attribution-left-out-fix

Module: docs.
Size: S.
Order: after 3.
Files: `docs/design/option-attribution.md`
Contracts: unchanged
Design: docs/design/option-pnl-totals.md#Decisions
Plan: P2
Acceptance:
1. The first bullet of `## Left out` in `docs/design/option-attribution.md` no longer lists "the page `/options/pnl`", "the monthly block" or "cash due if assigned" as later designs: it says that the page landed with `docs/design/options-pnl-page.md`, the monthly and per-company ledger block with `docs/design/options-pnl-ledger.md` and cash due if assigned with `docs/design/option-pnl-totals.md`, and keeps the rest of the bullet (quote coverage on the tracking page, the reader of `obs.option_day`, display rounding, the reconciliation lines) as it is.
2. The bullet "The sample sentence ... and monthly or per-company sums: the page's." keeps its meaning and gains the pointer to `docs/design/options-pnl-ledger.md`; the bullet for layer 3 is unchanged.
3. No other line of the file changes and no other file is touched; `grep -n "option-pnl-totals" docs/design/option-attribution.md` prints the pointer (the design file is new in its own pull request).
4. The result matches docs/design/option-pnl-totals.md#Decisions.

## Checked by CI

`go test ./internal/compute/... ./internal/contracts/... ./internal/pages/...` (entries 1 to 3), `go run ./scripts/arch` and go-arch-lint (no import edge is added), the regeneration check of `web/src/generated`, the contract-docs check, and for entry 4 `tsc`, `eslint`, `vitest` and the web build. Tests are parallel, use fakes and fixed data, no sleep, no live service and no PostgreSQL.

## Checked by owner

On `/options/pnl` after the deploy: each account with open sell-put lots shows the line "若全部被行权需付现金" with one figure per currency; the figure equals the sum of strike x multiplier x contracts of the open lots listed on the page; an account without open puts shows no line; the Chinese hint reads clearly.

## Left out

- Layer 3 of the plan's reconciliation (the account page's IBKR unrealised P&L against our mid P&L): IBKR's pricing basis is not read by any store the page uses and the figure would be a second source with its own gaps; not trivial, so not done. A later design needs the owner's word on which IBKR figure counts.
- The monthly block and per-company sums: already landed with `docs/design/options-pnl-ledger.md`; nothing to do here. The stale first bullet of `docs/design/option-attribution.md` Left out is corrected by entry 5.
- Cash due by expiry day, the account's cash and the shortfall: the notice page shows them (`optionnotice.cashByDate`); showing them here is a new design.
- Currency conversion and a household total of cash due: no FX is read; each currency stays apart.
- Covered or short calls and long puts: the page selects short puts only.
- Premium received netted against the cash due, and the cash due of a stock position after assignment: not the question this line answers.
- A chart or an export of the figure: none asked.

## Adversarial

Self-written, no agent spawned.

- Gap against the goal ("know the cash the open puts can call"): the figure assumes all lots are assigned at once and at the strike; by-day detail and available cash live on the notice page. Accepted and named in Left out; the hint says "all assigned".
- Biggest unnamed risk: `Booker` gains a method, so every fake of it in the repository stops compiling. The search of origin/main finds fakes only in `optionattrapi/api_test.go` and the pages tests; entry 1 names those files. A fake elsewhere is found by `go vet ./...` in CI and reported, not edited outside the Files line.
- Simpler way: reuse `optionnotice.cashByDate`. It is private and keyed by expiry day on another input type; exporting it is an interface change in a module the page does not depend on. Rejected in Decision 2.
- Claim most likely false: that every open lot is a `card` lot and so reaches the page's lot list. It is read from `lots.go` (`card` true for open lots); if the page's selection drops an open lot for another reason (a long put), the cash line misses it, which is right for short puts only. Entry 3 test `TestCashDueNoOpenPut` and the bought-put case in acceptance item 2 pin it.
- Likeliest slip: showing the figure rounded twice or summed in the page. Entry 3 `TestCashDueFromBooker` fixes the page to the booker's text; the web has no arithmetic by acceptance item 1 of entry 4.
- Stale claim risk: the schema key list of `Account` changed after origin/main 93dd0f49; entry 2 is a plain optional addition and the contract test keeps the existing sample valid.
