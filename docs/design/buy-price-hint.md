# buy price is a hint, not a block: a put whose strike is above the buy price stays ranked and shown

Author: the optdesk designer
Owner approved: (pending)
Date: 2026-11-26
Plan: P2

Source: owner ask optdesk-6294 (2026-11-26), word "a": the sell-put gate "strike above buy price" becomes a hint, not a block. Read from origin/main 073ef977.

## Frame

Today a short put whose break-even (strike minus bid) is above the buy price is stopped twice:

- `buyPriceGate` in `internal/compute/optioncand/put.go` returns `GateBlock` with reason `above_buy_price`, so `status` is `blocked` and `rank.go` never ranks it (only `ok` rows are ranked).
- The page builds its tiers from `ok` rows only (`builder.go`), and `greyReason` in `internal/compute/optiontier/grey.go` greys a row with the same test; a grey row is folded, and a company whose rows are all grey gets the state `above_buy`.

Result for the owner: such a put is ranked like any other ok row, sits in the fits or check tier of `/options/candidates` (not grey, not folded), and carries the visible hint "行权价高于买入价" (the sentence "Strike is above the buy price" in the Marks column). A put with no buy price is unchanged (it already passes with the mark `no_buy_price`); every other gate is unchanged.

## Decisions

### Decision 1 (owner optdesk-6294): the gate passes and the stored mark carries the hint

`buyPriceGate` returns `GatePass` with an empty reason in every case, as it already does for no buy price. `evaluatePut` adds the mark `above_buy_price` to `Marks` when the break-even is above a usable buy price, the same way it adds `no_buy_price`. No new gate level: `GateStatus` stays pass, block, unknown, because a level other than pass keeps the row out of the ranking (`status` turns it blocked or unknown), and a pass with a reason would break the declared rule "Reason is empty when Status is pass".

### Decision 2: the tier judge stops greying it and adds a note

`optiontier` drops `above_buy_price` from the grey reasons and from the grey order. A row with that break-even goes through `rule` like any other row and keeps its tier (fits or check by the other conditions); `rule` adds the note `above_buy_price` to `Verdict.Notes` (a note changes no tier, like `thin_premium`). The test is the same numbers as before: break-even (strike minus bid) above the buy price, both read. The note is derived from the numbers, not from the stored mark, so a row stored before the change (its status `blocked`, not shown) needs no migration: it enters the tier at the next candidates run.

### Decision 3: the company state `above_buy` is no longer produced

With no grey `above_buy_price`, `blocks.go` has no case for the state and no row is folded under that code. `optiontierapi.StateAboveBuy` stays declared for one more round so that the pages module, which names it, still builds when the interface issue lands; the pages issue stops using it; removing the constant is a later interface change (`## Left out`).

### Decision 4: the page shows the hint twice

The note `above_buy_price` gets a label and gloss in `tierstrings.go` (label "行权价高于买入价"), shown with the tier row; the Marks column shows the mark with its English sentence through `markHints` (`above_buy_price · Strike is above the buy price`). The grey label, the grey title, the state verdict and the state sentence of `above_buy` are deleted from the page.

## Modules

compute (`internal/compute/optioncand`, `internal/compute/optiontier`, the interface files `internal/compute/api.go` and `internal/compute/optiontierapi/api.go`, `docs/contracts/compute/optiontier.md`) and pages (`internal/pages/option_candidates_page`). No contracts, no migration, no `modules.md` change.

## Interface

Comments only, owned by compute, landing first (issue 1):

- `internal/compute/api.go`: the `GateResult` code list says `buy_price: no_buy_price (block: no BuyPrice), above_buy_price (block)`; it becomes "buy_price: none (never blocks; no_buy_price and above_buy_price are marks of the row)". Types and constants unchanged.
- `internal/compute/optiontierapi/api.go`: the `Verdict.Grey` comment drops `above_buy_price` from its list of reasons; the `StateAboveBuy` comment says it is no longer produced; the `Blocks` and `Tierer` text is unchanged. `StateAboveBuy` is not deleted (Decision 3).

`contracts/schemas/page.option_candidates_page.v1.json` keeps the values `grey_above_buy_price` and `above_buy` in its enums: a value that no longer occurs is not a change of shape, so `Contracts: unchanged`.

## Issues

### 1 buy-price-hint-interface

Module: compute. Size: S. Order: first. Design: docs/design/buy-price-hint.md#interface. Files: `internal/compute/api.go`, `internal/compute/optiontierapi/api.go`.

Acceptance:
1. The `GateResult` comment in `internal/compute/api.go` no longer says `above_buy_price (block)` and says that `buy_price` never blocks; no type, constant or signature of the file changes.
2. The `Verdict.Grey` comment in `internal/compute/optiontierapi/api.go` lists five grey reasons without `above_buy_price`, and the `StateAboveBuy` comment says it is no longer produced; the constant is still declared.
3. Docs: docs/design/buy-price-hint.md#interface.

### 2 buy-price-hint-compute

Module: compute. Size: M. Order: after 1. Design: docs/design/buy-price-hint.md#decisions. Files: `internal/compute/optioncand/put.go`, `internal/compute/optioncand/optioncand.go`, `internal/compute/optioncand/put_test.go`, `internal/compute/optiontier/grey.go`, `internal/compute/optiontier/blocks.go`, `internal/compute/optiontier/tier.go`, `internal/compute/optiontier/badges.go`, `internal/compute/optiontier/grey_test.go`, `internal/compute/optiontier/blocks_test.go`, `internal/compute/optiontier/tier_test.go`, `docs/contracts/compute/optiontier.md`.

Acceptance:
1. A put with strike 100, bid 1.99 and buy price 98.00 has the gate `buy_price` with status pass and an empty reason, the row status `ok`, a rank, and the mark `above_buy_price`; at bid 2.00 the mark is absent: a new test `TestAboveBuyPriceIsMarkNotBlock` in `put_test.go` pins both, and `TestBuyPriceBoundary` and `TestBuyPriceSetStillBlocks` are replaced by it.
2. A put with no buy price still passes with the mark `no_buy_price` and no `above_buy_price`: `TestNoBuyPriceIsMarkNotBlock` passes unchanged.
3. Every other gate still blocks as before: a row with strike above buy price and a wide spread has status `blocked` by the quote gate only (new test `TestAboveBuyPriceDoesNotHideOtherBlocks` in `put_test.go`).
4. The tier judge no longer greys the row: the case "above buy price" of `grey_test.go` expects an empty grey reason and the tier fits, with the note `above_buy_price` in `Verdict.Notes` (new test `TestAboveBuyPriceRowKeepsItsTierWithANote` in `tier_test.go`); a row with the same break-even and a wide quote is tier check with the note.
5. The grey order has five reasons (`blocks_test.go` line 261 list drops `above_buy_price`), and a company whose priced rows are all above the buy price is `sellable` with those rows shown, not `above_buy` (new test `TestCompanyAboveBuyIsSellableNotFolded` in `blocks_test.go`).
6. `docs/contracts/compute/optiontier.md` lists the five grey reasons and says the company state `above_buy` is not produced; it names the note `above_buy_price`.
7. Docs: docs/design/buy-price-hint.md#decisions.

### 3 buy-price-hint-page

Module: pages. Size: M. Order: after 2. Design: docs/design/buy-price-hint.md#decisions. Files: `internal/pages/option_candidates_page/tierstrings.go`, `internal/pages/option_candidates_page/strings.go`, `internal/pages/option_candidates_page/zh_texts.go`, `internal/pages/option_candidates_page/bycompany.go`, `internal/pages/option_candidates_page/bycompany_test.go`, `internal/pages/option_candidates_page/tiers_test.go`, `internal/pages/option_candidates_page/zhstrip_test.go`, `internal/pages/option_candidates_page/reasons_test.go`, `internal/pages/option_candidates_page/tiersfakes_test.go`, `internal/pages/option_candidates_page/testdata/by_company.golden.json`.

Acceptance:
1. A tier row whose verdict has the note `above_buy_price` shows the badge with the label "行权价高于买入价" and a gloss, in the tier row and not in a grey or folded block: a new test `TestAboveBuyNoteIsShownOnTheRow` in `tiers_test.go`.
2. The Marks column of a stored row with the mark `above_buy_price` shows `above_buy_price · Strike is above the buy price` (new test `TestAboveBuyMarkCarriesItsSentence` in `reasons_test.go`).
3. The page has no grey title, grey label, state verdict or state sentence for `above_buy`, and no use of `optiontierapi.StateAboveBuy`; `by_company.golden.json` carries no `above_buy_price` fold and no `above_buy` block; the other goldens are unchanged.
4. A row of an older stored run with the gate `above_buy_price` (status blocked) is still listed with the sentence "Strike is above the buy price" in the blocked list: `reasonTexts` keeps its entry, and the existing code-coverage loop of `reasons_test.go` (line 38) still passes.
5. Docs: docs/design/buy-price-hint.md#decisions.

## Checked by CI

- `go test ./internal/compute/... ./internal/pages/...`, `go vet ./...`, `golangci-lint` and `scripts/arch` pass.
- The tests are parallel with a temporary directory per file, a fake clock, no sleep, no live broker and no PostgreSQL.
- None of these is an acceptance item above.

## Left out

- `internal/strategy/shortput` is unchanged: its check list (`buy_price` block with `above_buy_price` in its own test, `shortput_test.go` line 117) was read only at that test line, and neither compute nor pages is changed to import it, so it keeps its own rule; making it a hint too is a separate owner word, not cut here.
- Deleting `optiontierapi.StateAboveBuy`, the `above_buy` and `grey_above_buy_price` values of `page.option_candidates_page.v1.json` and the generated type: a second interface change plus a contract change (`Contracts: changed`), after the pages issue; not cut, the values are harmless.
- A new `GateStatus` level (warn/hint): rejected, see Decision 1.
- Old stored rows: not rewritten; the next candidates run stores them as ok with the mark. A forced re-run of a stored day (`-force`) does it at once; that is the owner's call.
- Moving the buy-price test of the tier judge to read the stored mark instead of the numbers: the numbers are the single rule today; not changed.

## Adversarial

Self-attack by the author on six questions, no agent spawned.

- Gap against the goal: "ranked and usable" needs the compute gate and the tier judge to agree; they are two copies of one test (`buyPriceGate` and `greyReason`). Adopted: both change in issue 2 with a test each. Not adopted: one shared helper, since the two sit behind different interfaces and would need a new one.
- Biggest unnamed risk: ranking. An above-buy put now competes in `rank.go` with the rest and, with a high yield, may rank first. The owner asked for "usable", not "ranked last". Not adopted: demoting it; the owner can say so after seeing the page, and the hint is on the row.
- Simpler way: only change the gate level. Not enough: the tier judge would still grey and fold the row, so the page would not change. Another simpler way, a new warn level, needs a change of the `GateStatus` type and keeps the row out of the ranking.
- Lock-in: low. The hint is a mark and a note, both free words; `StateAboveBuy` stays declared, so reverting is two small edits.
- Claim most likely false: "no migration". A page showing yesterday's stored rows has no above-buy puts in the tier until the next candidates run stores them as ok; I did not check when that run is. The `Contracts: unchanged` claim rests on the enum values staying valid, which I read in the schema but did not run through the contract test.
- Likeliest slip: the page goldens. `by_company.golden.json` has two `above_buy_price` entries and the fake tierer rows of `bycompany_test.go` build them by hand; deleting the state changes the block order and counts there. Issue 3 lists the file and says the other goldens are unchanged, which fails the issue loudly if wrong.
