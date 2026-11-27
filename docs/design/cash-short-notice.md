---
type: design
title: "cash short notice: the option notices page raises a cash_short notice when the cash left after the assignments due by a day is below zero"
status: accepted
date: "2026-11-27"
need: [none]
none_reason: "owner ask optdesk-4723 2026-11-27: the page showed no notice while cash left was -$30,553 on a day; no need record exists"
---

# cash short notice: the option notices page raises a cash_short notice when the cash left after the assignments due by a day is below zero

Author: the optdesk designer
Owner approved: (pending)
Date: 2026-11-27
Plan: P2

Source: owner ask optdesk-4723 (the page `/options/notices` had a day with cash left -$30,553 and no notice); `docs/design/option-notices.md` (landed; Decisions 5 and 6); `docs/design/sellput-gaps.md` (Left out, "Shortfall notice on /options/notices", which suggests this as a separate compute issue and a separate page issue). Every fact below was read on origin/main `a160d98d`.

Owner-visible result: on `/options/notices`, an account block whose cash left (see Decision 1) is below zero on some expiry day shows one notice "现金不足 Cash is short" per currency, with the first day it is short as its expiry (a later day can show a larger shortfall in the table below), the cash available, the cash all the short puts up to that day would need, the cash left (for example "-$30,553"), the number of days that are short, and one Chinese sentence saying how "left" is counted. A block whose cash left is zero or above on every day shows no such notice. A currency with no cash row shows no notice and no figure (the table already shows no "left" there); nothing is estimated.

## Frame

- `internal/compute/optionnotice/cash.go` (`cashByDate`) already returns, per expiry day and currency, `Amount` (the cash the short puts of that day need), `Available` (the stored `TotalCashValue` of that currency) and `Left` (`Available` minus the `Amount` of this day and all earlier days from the build day on; negative is a shortfall). `Available` and `Left` are empty when no `TotalCashValue` row has that currency. Nothing reads `Left` except the page table (`option_notices_page/blocks.go`, `okBlock`).
- `Evaluate` (`notice.go`) builds the notices from `expiryNotices`, `guardNotices`, `earningsNotices`, `assignmentNotices`, `cushionNotice` and `staleNotice`; `CashByDate` is built after them and is not an input of any of them.
- The page turns a `compute.Notice` into a row in `noticeRowOf` (`blocks.go`): title from `noticeTitles` (`text.go`, texts in `zh_texts.go`), facts through `factLabels`, a fact labelled `reason` through `reasonTexts`. The schema `contracts/schemas/page.option_notices_page.v1.json` types `code` as the pattern `^[a-z_]{1,32}$` ("a new code needs no schema change") and `facts` as label/text pairs. `web/src/pages/notices/` holds no code or label of any notice (`grep -rn cushion_low web/src` finds nothing): it renders the payload fields.
- `docs/design/option-notices.md` Decision 6 says the cash by date is "display only". This design amends that sentence for the shortfall case, see Decision 7; this file is the record of the amendment and `option-notices.md` is not edited.

## Owner rules and how this design meets them

- No made-up data, a gap stays a gap with its reason: a day with no cash row has an empty `Left`, so no notice is raised and no figure is shown (Decision 2). The notice never uses a cash figure of another currency, and never a figure the page cannot also show in its table.
- Chinese hints, terms explained next to the term: the notice carries a Chinese `reason` sentence that explains "left" and "Assigned" where the figures are (Decision 5).
- The front end renders and computes nothing: the web is unchanged and the schema is unchanged (Decisions 6 and 8).
- Cushion and assignment cash never go to Telegram (`option-notices.md` Decision 1): this notice is built by the pure rule and shown by the page; no `Sender` is reached (Decision 7).
- No migration, no new table, no role: none is needed.

## Decisions

1. **What is short.** A `CashDay` row of `CashByDate` is short when `Left` parses and is below zero. `Left` is cumulative (cash minus the dues of every earlier day too), so once a day is short, every later day with a short put is short unless a later row has no cash. The rule is exactly the number the table shows to the owner; no second formula.
2. **A day with no cash is not short.** `Left` empty (no `TotalCashValue` of that currency) or not parsable raises nothing; the notice reads `Left` only, and `Available` and `Left` empty is the existing gap shown by the table.
3. **One notice per currency, for the first short day.** Because `Left` is cumulative, one notice per short row would repeat one shortfall on every later day. The notice is for the earliest short row of the currency: `Code` `cash_short`, `Symbol` and `Contract` empty (it belongs to the account, as `cushion_low` does), `Expiry` that day. Its facts, in this order: `currency` (as the row writes it), `available` (the row's `Available`), `due` (`Available` minus `Left`, exact `big.Rat` arithmetic, written with `formatTrim`), `left` (the row's `Left`), `short_days` (the count of rows of this currency with `Left` below zero, as text), and one `reason` fact with the code `cash_short_cumulative`. The deepest figure is not repeated: the table below the notices shows every day.
4. **Severity.** `act` when the first short day is at most `ExpiryDays` (5, the existing parameter) calendar days after the build day, otherwise `warn`; `warn` when the build day does not parse. No new parameter and no new threshold. The ordering of `Evaluate` (severity, expiry, contract, code) is unchanged and places this notice among the others.
5. **Page text.** The title is "现金不足 Cash is short". The labels are "币种 Currency", "可用现金 Cash available", "所需现金 Cash needed", "剩余现金 Cash left" and "不足的天数 Days short". The `reason` sentence for `cash_short_cumulative` is: "剩余现金 = 可用现金 - 到这一天为止所有到期的空头 put 被行权（Assigned，买方要求你按行权价买入股票）需要的现金；是累计的，不是只算这一天。" The page formats the facts `available`, `due` and `left` with `amountText` and the currency of the fact `currency`, so a negative USD amount reads "-$30,553". An unknown fact label and an unknown reason code keep today's fallback (shown as written).
6. **Schema, web: unchanged.** The code is a pattern, the facts are label/text pairs and the reasons are texts, so `contracts/` and `web/` do not change; the entries state `Contracts: unchanged`. Each visible string is a payload field.
7. **Amends Decision 6 of `option-notices.md`.** The cash by date stays a table of what an assignment would need; the rule now also reads its `Left` to raise one notice. It is still no forecast: it says "if all the short puts up to that day are Assigned, the cash on hand is not enough". The notice goes to the page only; no notify type is imported (the test of `option-notices.md` Decision 1 stays green).
8. **Rule version.** `ruleVersion` of `optionnotice/params.go` goes from 1 to 2, as its comment asks when a code is added. No consumer reads the number besides the result type.
9. **No data from another currency or account.** Each currency has its own cash row and its own notice; the JPY and USD cash are never added.

## Modules

- compute: `internal/compute/optionnotice` only (entry 1). `internal/compute/api.go` is not edited: `Notice` and `CashDay` already carry everything.
- pages: `internal/pages/option_notices_page` only (entry 2). It reads `compute.Notice` through the existing interface.
- No change to `docs/design/modules.md`, contracts, store, ot or web.

## Interface

No interface file is edited. The two modules share the existing `compute.Notice` and `compute.NoticeFact`, and now one more agreed text between them: the code `cash_short`, the fact labels `currency`, `available`, `due`, `left`, `short_days` and the reason code `cash_short_cumulative` of Decision 3. Entry 1 writes them, entry 2 reads them; no entry carries `(interface)`. Both entries name the same strings in their acceptance so a drift fails a test on one side.

## Issues

Build order: entries 1 and 2 are independent (the page tests build a `compute.Notice` by hand); the owner sees the notice when both have landed. Every entry carries `Contracts: unchanged`.

### 1 cashshort-compute

Module: compute. Size: S.
Order: first.
Files: `internal/compute/optionnotice/notice.go`, `internal/compute/optionnotice/params.go`, `internal/compute/optionnotice/notice_test.go`
Files: `internal/compute/optionnotice/cashshort.go` (new), `internal/compute/optionnotice/cashshort_test.go` (new)
Contracts: unchanged
Design: docs/design/cash-short-notice.md#Decisions (1, 2, 3, 4, 8)
Acceptance:
1. `cashshort.go` (new) declares `cashShortNotices(rows []compute.CashDay, day int, dayOK bool, p compute.NoticeParams) []compute.Notice`; `Evaluate` calls `cashByDate` once, before `order`, appends `cashShortNotices` of those rows to the notices, and returns the same rows as `CashByDate`.
2. `TestCashShortFirstDayPerCurrency` (new) shows rows USD day 1 `Left` "5000", USD day 2 `Left` "-30553", USD day 3 `Left` "-40000" give one notice with `Code` "cash_short", `Expiry` the day 2, empty `Symbol` and `Contract`, and the facts of Decision 3 in order with `left` "-30553" and `short_days` "2".
3. `TestCashShortDue` (new) shows `available` "100000" and `left` "-30553.5" give `due` "130553.5".
4. `TestCashShortNoCashNoNotice` (new) shows rows with empty `Available` and `Left`, and a row with a `Left` that does not parse, raise no notice; `TestCashShortZeroOrAbove` (new) shows `Left` "0" raises none.
5. `TestCashShortCurrenciesApart` (new) shows a short USD row and a non-short JPY row give one notice with `currency` "USD", and two short currencies give two notices.
6. `TestCashShortSeverity` (new) shows a first short day 5 days after the build day is `act`, 6 days after is `warn`, and an unparsable build day gives `warn`.
7. `TestEvaluateCashShort` (new) shows `Evaluate` with a short put whose strike times multiplier is above the `TotalCashValue` row gives a result whose `Notices` hold one `cash_short` and whose `CashByDate` equals the rows of a direct call of `cashByDate` on the same input, and the notice is placed by `order` (an `act` notice before a `warn` one); `ruleVersion` in `params.go` is 2 and `notice_test.go` expects 2 where it expects the version.
8. `grep -c "t.Parallel()" internal/compute/optionnotice/cashshort_test.go` is at least its count of `^func Test`; `cashshort.go` (new) stays under 600 lines.

### 2 cashshort-page

Module: pages. Size: S.
Order: first.
Files: `internal/pages/option_notices_page/text.go`, `internal/pages/option_notices_page/zh_texts.go`, `internal/pages/option_notices_page/blocks.go`, `internal/pages/option_notices_page/blocks_test.go`, `internal/pages/option_notices_page/text_test.go`, `internal/pages/option_notices_page/builder_test.go`
Files: `internal/pages/option_notices_page/testdata/cash_short.json` (new)
Contracts: unchanged
Design: docs/design/cash-short-notice.md#Decisions (5, 6, 9)
Acceptance:
1. `noticeTitles` has `cash_short` with the title "现金不足 Cash is short", `factLabels` has `currency`, `available`, `due`, `left` and `short_days` with the labels of Decision 5, and `reasonTexts` has `cash_short_cumulative` with the sentence of Decision 5; the three texts live in `zh_texts.go`; the label `short_days` and the reason code `cash_short_cumulative` are new names, written by entry 1.
2. `noticeRowOf` shows the fact `currency` as an ordinary row and writes the facts `available`, `due` and `left` through `amountText` with the value of the fact `currency` of the same notice; `TestCashShortRow` (new) shows a notice with `currency` "USD" and `left` "-30553" gives a row with the title, the fact text "-$30,553", the reason sentence in `Reasons`, and the `Expiry` of the notice.
3. `TestCashShortJPY` (new) shows `currency` "JPY" and `left` "-1200000" gives the text "-¥1,200,000".
4. `TestCashShortUnknownCurrencyKeepsFallback` (new) shows a notice with no `currency` fact writes the figures as the producer wrote them and raises no panic.
5. `TestCashShortPageValidates` (new) shows a built page with one `cash_short` notice equals the golden `testdata/cash_short.json` and validates against the schema of the page.
6. `git diff --name-only $(git merge-base HEAD origin/main)` lists no path under `contracts/` and none under `web/`; `grep -rn cash_short web/src` prints nothing.
7. The new tests call `t.Parallel()`: `grep -c "t.Parallel()"` of `blocks_test.go` and `text_test.go` is at least their count of `^func Test`; no changed source file passes 600 lines.

## Checked by CI

`go test ./...` (entries 1 and 2: the named tests and `go vet`), `go run ./scripts/arch` and go-arch-lint (no new import edge), the schema test of the page (the golden validates), `ci-web` (nothing under `web/` changes), `go run ./scripts/precheck`.

## Left out

- **A message to Telegram.** Not done: `option-notices.md` Decision 1 keeps cash and cushion off Telegram; a morning-report line for a shortfall is an owner choice, not made here.
- **One notice per short day.** Not done: with a cumulative `Left` it repeats one shortfall (Decision 3); the table shows every day.
- **Cash that is not `TotalCashValue`** (available funds, margin, cash of the candidates page `free` in `candidates-tiers.md`). Not changed: the notice uses the same number as the table.
- **A page line saying a currency has no cash row.** Not done: the table already shows no "left" for it; a "cash unknown" notice would be a new gap line, an owner choice.
- **The demo data of `cmd/ot/demo.go`.** Not changed; it is a different module.
- **Converting JPY to USD or netting currencies.** Not done (Decision 9).

## Doubts

1. `TotalCashValue` is the account's total cash in that currency; with a margin loan it is itself negative, and then every day is short and the notice is true but the first short day is the first row. Read, not changed.
3. The code name `cash_short` is also the name of a badge of the candidates page (`internal/compute/optiontier`) with another formula; two meanings on two pages, the owner may compare them (adopted from the adversarial review as a named risk, not unified).
4. `TotalCashValue` is not the cash free for assignment: margin or a base-currency row can make `Left` negative with nothing wrong; the notice repeats the number the table shows, and the reason sentence says how it is counted.
5. `act` for a shortfall within five days reuses the expiry window; whether the owner wants a different window is for him to say.

## Adversarial

Answers to `state/optdesk-drafts/designs/cash-short-adversarial.md` (a fresh sub-agent that read the design and the repo), by its numbers:

1. Gap vs goal: adopted. The owner-visible result now says the notice is for the first short day and a later day can be larger. The amendment of `option-notices.md` Decision 6 is recorded in this file, which is stated in the Frame; no edit of that file. A negative balance with no put due raises nothing: not adopted, it follows from "due by a day".
2. Unnamed risk: adopted as Doubts 3 and 4 (the badge name `cash_short` of `optiontier`, and `TotalCashValue` is not cash free for assignment). Not unified.
3. Simpler way (only `left`, `currency`, reason): not adopted. The owner reads the notice without scrolling to the table, and `due` shows why `left` is below zero; the cost is one subtraction with `big.Rat` already used in the package.
4. Lock-in: accepted and named in Interface (code, labels, reason code, `ruleVersion` 2); both entries name the same strings in tests.
5. Claim most likely false: adopted. `cashByDate` is called inside the return after `order`; entry 1 item 1 now says it is called once and hoisted before `order`.
6. Likeliest slip: adopted. Entry 1 item 7 now compares `CashByDate` with a direct call and checks the order; entry 2 item 2 says the `currency` fact is an ordinary row.

## 摘要

期权提醒页（/options/notices）上，当某一天"剩余现金"（可用现金减去截至该日所有到期的空头 put 被行权所需现金，累计）小于零时，多一条"现金不足"提醒。每个币种一条，取最早不足的那一天，显示可用现金、所需现金、剩余现金（如 -$30,553）、不足的天数和一句中文解释；没有现金记录的币种不出提醒，也不估算。做法：compute 的 optionnotice 读已有的 Left 并产出 cash_short；页面只补标题、标签和中文说明并格式化金额；schema 和前端不变，没有迁移。
