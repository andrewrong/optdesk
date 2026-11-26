---
type: design
title: "buy price is a hint, not a block: a put whose strike is above the buy price stays ranked and shown"
status: accepted
date: "2026-11-26"
need: [none]
none_reason: "owner ask optdesk-6294 (2026-11-26), word a; no need record exists"
---

# buy price is a hint, not a block: a put whose strike is above the buy price stays ranked and shown

Author: the optdesk designer
Owner approved: (pending)
Revision: 4
Plan: P2

摘要：卖出看跌（sell put）候选的“行权价（保本点）高于买入价”不再是拦截，而是提示。这样的合约照常参与排名，显示在 /options/candidates 的“适合”或“待查”里（不变灰、不折叠），并带提示“行权价高于买入价”；没有买入价的行不变，其他闸门不变。做法：闸门放行并留下标记 above_buy_price；分档判断不再把它判灰，只加一条说明；页面补上文字并删掉“高于买入价”的灰档与公司状态；“适合/待查”的定义里去掉“保本点不高于买入价”。分三件事：先改接口注释（compute），再改 compute，再改页面。

Source: owner ask optdesk-6294 (2026-11-26), word "a": the sell-put gate "strike above buy price" becomes a hint, not a block. Read from origin/main 073ef977; the paths this design names are unchanged on origin/main c0b2cac6 (`git diff --stat` of `internal/compute`, `internal/pages/option_candidates_page` and `docs/contracts/compute` is empty).

Needs the owner: nothing. Contracts: unchanged.

## Frame

Today a short put whose break-even (strike minus bid) is above the buy price is stopped twice:

- `buyPriceGate` in `internal/compute/optioncand/put.go` returns `GateBlock` with reason `above_buy_price`, so `status` is `blocked` and `rank.go` never ranks it (only `ok` rows are ranked).
- The page builds its tiers from `ok` rows only (`builder.go`), and `greyReason` in `internal/compute/optiontier/grey.go` greys a row with the same test; a grey row is folded, and a company whose rows are all grey gets the state `above_buy`.

Result for the owner: such a put is ranked like any other ok row, sits in the fits or check tier of `/options/candidates` (not grey; like any fits or check row it is folded only when it is outside the band or past the per-company cap), and carries the visible hint "行权价高于买入价" (the sentence "Strike is above the buy price" in the Marks column). A put with no buy price is unchanged (it already passes with the mark `no_buy_price`); every other gate is unchanged.

### Decisions

#### Decision 1 (owner optdesk-6294): the gate passes and the stored mark carries the hint

`buyPriceGate` returns `GatePass` with an empty reason in every case, as it already does for no buy price. `evaluatePut` adds the mark `above_buy_price` to `Marks` when the break-even is above a usable buy price, the same way it adds `no_buy_price`. No new gate level: `GateStatus` stays pass, block, unknown, because a level other than pass keeps the row out of the ranking (`status` turns it blocked or unknown), and a pass with a reason would break the declared rule "Reason is empty when Status is pass".

#### Decision 2: the tier judge stops greying it and adds a note

`optiontier` drops `above_buy_price` from the grey reasons and from the grey order. A row with that break-even goes through `rule` like any other row and keeps its tier (fits or check by the other conditions); `rule` adds the note `above_buy_price` to `Verdict.Notes` (a note changes no tier, like `thin_premium`). The test is the same numbers as before: break-even (strike minus bid) above the buy price, both read. So the definition of fits and check changes with it: fits is "buy price set, out of the money, |delta| within the bound and cash enough", check is "the first two hold and a look-at badge is present"; "break-even not above the buy price" leaves the definition in the contract (`docs/contracts/compute/optiontier.md`), the code comment of `rule`, and every page text and golden that states it (issues 2 and 3 name them). The note is derived from the numbers, not from the stored mark, so a row stored before the change (its status `blocked`, not shown) needs no migration: it enters the tier at the next candidates run.

#### Decision 3: the company state `above_buy` is no longer produced

With no grey `above_buy_price`, `blocks.go` has no case for the state and no row is folded under that code. `optiontierapi.StateAboveBuy` stays declared for one more round so that the pages module, which names it, still builds when the interface issue lands; the pages issue stops using it; removing the constant is a later interface change (`## Left out`).

#### Decision 4: the page shows the hint twice

The note `above_buy_price` gets a label and gloss in `tierstrings.go` (label "行权价高于买入价"), shown with the tier row; the Marks column shows the mark with its English sentence through `markHints` (`above_buy_price · Strike is above the buy price`). The grey label, the grey title, the state verdict and the state sentence of `above_buy` are deleted from the page.

## Modules

compute (`internal/compute/optioncand`, `internal/compute/optiontier`, the interface files `internal/compute/api.go` and `internal/compute/optiontierapi/api.go`, `docs/contracts/compute/optiontier.md`) and pages (`internal/pages/option_candidates_page`). No contracts, no migration, no `modules.md` change.

## Interface

Comments only, owned by compute, landing first (issue 1):

- `internal/compute/api.go`: the `GateResult` code list says `buy_price: no_buy_price (block: no BuyPrice), above_buy_price (block)`; it becomes "buy_price: none (never blocks; no_buy_price and above_buy_price are marks of the row)". Types and constants unchanged.
- `internal/compute/optiontierapi/api.go`: the `Verdict.Grey` comment drops `above_buy_price` from its list of reasons; the `StateAboveBuy` comment says it is no longer produced; the `StateNoQuote` comment drops "(and the above_buy rule does not hold)"; the `Blocks` and `Tierer` text is unchanged. `StateAboveBuy` is not deleted (Decision 3).

`contracts/schemas/page.option_candidates_page.v1.json` keeps the values `grey_above_buy_price` and `above_buy` in its enums: a value that no longer occurs is not a change of shape, so `Contracts: unchanged`.

## Failure paths

- A stored row of an older run (gate `above_buy_price`, status blocked) stays in the blocked list with its sentence until the next candidates run stores it as ok (issue 3 acceptance 6).
- A row whose break-even cannot be read (buy price, strike or bid not a number) gets no note and keeps its grey reason (`no_buy_price`, `no_bid` or `bad_input`), as before.
- Carried decisions: Decision 1 in issue 2 (acceptances 1 to 3), Decision 2 in issues 2 and 3, Decision 3 in issues 1 and 2 (the constant stays; no entry removes it), Decision 4 in issue 3.

## Issues

### 1 buy-price-hint-interface

Module: compute. Size: S. Order: first. Design: docs/design/buy-price-hint.md#interface. Files: `internal/compute/api.go`, `internal/compute/optiontierapi/api.go`.

Acceptance:
1. The `GateResult` comment in `internal/compute/api.go` no longer says `above_buy_price (block)` and says that `buy_price` never blocks; no type, constant or signature of the file changes.
2. The `Verdict.Grey` comment in `internal/compute/optiontierapi/api.go` lists five grey reasons without `above_buy_price`, and the `StateAboveBuy` comment says it is no longer produced; the constant is still declared (`internal/compute/optiontierapi/api_test.go` still uses it and is not edited); the `StateNoQuote` comment has no "above_buy rule" clause.
3. Docs: docs/design/buy-price-hint.md#interface.

### 2 buy-price-hint-compute

Module: compute. Size: M. Order: after 1. Design: docs/design/buy-price-hint.md#decisions. Files: `internal/compute/optioncand/put.go`, `internal/compute/optioncand/optioncand.go`, `internal/compute/optioncand/put_test.go`, `internal/compute/optiontier/grey.go`, `internal/compute/optiontier/blocks.go`, `internal/compute/optiontier/tier.go`, `internal/compute/optiontier/badges.go`, `internal/compute/optiontier/grey_test.go`, `internal/compute/optiontier/blocks_test.go`, `internal/compute/optiontier/tier_test.go`, `docs/contracts/compute/optiontier.md`.

Acceptance:
1. A put with strike 100, bid 1.99 and buy price 98.00 has the gate `buy_price` with status pass and an empty reason, the row status `ok`, a rank, and the mark `above_buy_price`; at bid 2.00 the mark is absent: a new test `TestAboveBuyPriceIsMarkNotBlock` (new) in `put_test.go` pins both.
2. A put with no buy price still passes with the mark `no_buy_price` and no `above_buy_price`: `TestNoBuyPriceIsMarkNotBlock` passes unchanged. The two old tests `TestBuyPriceBoundary` and `TestBuyPriceSetStillBlocks` are removed in `put_test.go`; the first new test of item 1 takes their place.
3. Every other gate still blocks as before: a row with strike above buy price and a wide spread has status `blocked` by the quote gate only (`TestAboveBuyPriceDoesNotHideOtherBlocks` (new) in `put_test.go`).
4. The tier judge no longer greys the row: the case "above buy price" of `grey_test.go` expects an empty grey reason and the tier fits, with the note `above_buy_price` in `Verdict.Notes` (new test `TestAboveBuyPriceRowKeepsItsTierWithANote` (new) in `tier_test.go`); a row with the same break-even and a wide quote is tier check with the note.
5. The grey order has five reasons (`blocks_test.go` line 261 list drops `above_buy_price`), and a company whose priced rows are all above the buy price is `sellable` with those rows shown, not `above_buy` (new test `TestCompanyAboveBuyIsSellableNotFolded` (new) in `blocks_test.go`).
6. `docs/contracts/compute/optiontier.md` lists the five grey reasons, says the company state `above_buy` is not produced and names the note `above_buy_price`; its sentence "A row is green, fits ... when all hold" no longer has the condition "break-even (strike minus bid) is at most the buy price" and its check rule reads "when the first two hold"; the comment of `rule` in `tier.go` says the same.
7. Checked at origin/main c0b2cac6 by `grep -rn "above_buy\|AboveBuy" internal/compute`: outside `grey.go`, `blocks.go`, `put.go`, `api.go` and `internal/compute/optiontierapi/api.go` the only hits are `grey_test.go`, `blocks_test.go`, `put_test.go` (listed) and `internal/compute/optiontierapi/api_test.go` (the constant stays, not edited); `contract_test.go`, `jpy_test.go`, `order_test.go`, `tier_test.go` and every `testdata` file of `optioncand` and `optiontier` have none, so no file is missing from Files.
8. Docs: docs/design/buy-price-hint.md#decisions.

### 3 buy-price-hint-page

Module: pages. Size: M. Order: after 2. Design: docs/design/buy-price-hint.md#decisions. Files: `internal/pages/option_candidates_page/tierstrings.go`, `internal/pages/option_candidates_page/strings.go`, `internal/pages/option_candidates_page/zh_texts.go`, `internal/pages/option_candidates_page/bycompany.go`, `internal/pages/option_candidates_page/bycompany_test.go`, `internal/pages/option_candidates_page/tiers_test.go`, `internal/pages/option_candidates_page/zhstrip_test.go`, `internal/pages/option_candidates_page/reasons_test.go`, `internal/pages/option_candidates_page/tiersfakes_test.go`, `internal/pages/option_candidates_page/tierlegend_test.go`, `internal/pages/option_candidates_page/testdata/by_company.golden.json`, `internal/pages/option_candidates_page/testdata/jp_rows.golden.json`, `internal/pages/option_candidates_page/testdata/tiers_gaps.golden.json`, `internal/pages/option_candidates_page/testdata/shortlist_ok.golden.json`, `internal/pages/option_candidates_page/testdata/valueline.golden.json`, `internal/pages/option_candidates_page/testdata/tiers_ok.golden.json`. It is ordered after 2 because the page labels a note code that only issue 2 produces.

Acceptance:
1. A tier row whose verdict has the note `above_buy_price` shows the badge with the label "行权价高于买入价" and a gloss, in the tier row and not in a grey or folded block: `TestAboveBuyNoteIsShownOnTheRow` (new) in `tiers_test.go`.
2. The Marks column of a stored row with the mark `above_buy_price` shows `above_buy_price · Strike is above the buy price` (`TestAboveBuyMarkCarriesItsSentence` (new) in `reasons_test.go`).
3. The page has no grey title, grey label, state verdict or state sentence for `above_buy`, and no use of `optiontierapi.StateAboveBuy`; `by_company.golden.json` shows no `above_buy_price` fold and no `above_buy` block.
4. The definition of fits and check on the page no longer has the break-even condition: in `zh_texts.go` `explainFitsHead` and `legendFitsHead` read "有买入价；价外；|delta| 不高于 ..." without "保本点（行权价减 bid）不高于买入价", `explainCheck` reads "有买入价、价外这两项都满足，但有需要再看一眼的徽标。", `legendCheckText` reads "买入价、价外两项都满足，但有需要再看一眼的徽标。", `noneFits` reads "没有一行同时满足有买入价、价外和 delta 上限。", `zhFoldedLegendHead` reads "已判定但不能卖（价内，或没有 bid）..." without "或保本点高于买入价", and `noneCheck` reads "没有行满足有买入价、价外两项条件却带有需要再看一眼的徽标。".
5. The goldens `jp_rows`, `tiers_gaps`, `shortlist_ok`, `valueline`, `tiers_ok` and `by_company` show the new legend and explain texts of item 4, among them the empty-check explain line "无：没有行满足有买入价、价外两项条件却带有需要再看一眼的徽标。" in `shortlist_ok` and `tiers_gaps`; `by_company.golden.json` shows the above-buy rows of companies 7 and 8 as shown rows with the hint, not folded, and its fold titles, fold summaries, verdict texts and legend counts follow from that; the literals of `tierlegend_test.go` (lines 105, 112, 123) match the new texts. `markhints_test.go` lists no hint one by one for this mark, so it is not edited (checked).
6. A row of an older stored run with the gate `above_buy_price` (status blocked) is still listed with the sentence "Strike is above the buy price" in the blocked list: `reasonTexts` keeps its entry, and the existing code-coverage loop of `reasons_test.go` (line 38) still passes; the comment of that test (`TestEveryRuleCodeHasSentence`) that cites the code list of the `GateResult` comment is reworded to say the list is the reason codes the gates and the page still use.
7. Docs: docs/design/buy-price-hint.md#decisions.

## Checked by CI

- `go test ./internal/compute/... ./internal/pages/...`, `go vet ./...`, `golangci-lint` and `scripts/arch` pass.
- The tests are parallel with a temporary directory per file, a fake clock, no sleep, no live broker and no PostgreSQL.
- None of these is an acceptance item above.

## Left out

- `internal/strategy/shortput` is not edited, but it uses the same optioncand evaluator, so an above-buy put there also passes with the mark and keeps its rank; the owner chose this (2026-11-26, answer a on ticket 1315 of #4136): the buy price is a hint in shortput too.
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
- Likeliest slip: the page goldens. `by_company.golden.json` has two `above_buy_price` entries and the fake tierer rows of `bycompany_test.go` build them by hand; deleting the state changes the block order and counts there. The legend texts that define fits (six goldens, `tierlegend_test.go`) were the first review's finding; issue 3 now lists each file and text, and a missed one fails the issue loudly.
