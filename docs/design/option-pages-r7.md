# option pages r7: the fixes of the round-7 evaluation of every option page, the overview merged into the candidates page, Advice out of the menu, the chain browser under System

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-18

Source: the evaluation `state/optdesk-option-pages-eval-2026-11-18.md` (round 7, deployed 23500cab) and the owner's answer "a" on 2026-11-18 (ask optdesk-4888): fix the data errors and the listed fixes of candidates, pnl, notices, structures, tracking and result; merge `/options` into `/options/candidates`; drop `/advice` from the menu; move `/options/data` under System. Checked on origin/main 23500cab. No feature is added; the fixes are the report's, with three that differ from its wording and are named in Decisions 2, 9 and 11.

## Goal

After this design each data error of the report has a cause named in code and a fix that is checked by reading code; the candidates page opens on its tables with one line for the companies that have no buy price; the notices, tracking, result and pnl pages explain themselves in Chinese where they explain and in English where they label; the Options menu holds only pages with content; and `/options` leads to the candidates page.

## Decisions

Numbers in brackets are the numbered fixes of the report.

1. pnl header count [15]. Cause: `internal/pages/option_pnl_page/builder.go:196` counts `ended` only inside the loop after `if !l.card { continue }` (builder.go:189), and a card lot is a lot that is open or ended within `RecentDays` (`context.go:119-144`); the ledger months use every ended lot (`ledger.go:162`). So the header counts recent ended lots (9) and the months count all (61). Fix: count over `all` and print both the ended and the split count; the 20-lot rule counts split lots only, since only split lots tell what pays (entry 4, `builder.go:212-214`).
2. pnl entry vs mid on late-entry lots [16]. Cause: by the attribution identity the whole difference between the fill and the first stored mid is "Entry" (`internal/compute/optionattr/attr.go:316` sets `LateEntry`), so a lot sold in May and first quoted on 2026-11-07 carries months of market movement in it. The report asks to exclude it from the total; that would break the identity that the split adds up to the ledger figure (the `Difference` of the golden in `optionattr/testdata/golden.json`). Decision: keep figure and total, mark the line in Chinese as movement before the first quote, and put on the lot, book and month or company Entry vs mid lines the count `N of M lots entered before the first stored quote (quotes start 2026-11-07)`, counted from the outcomes by the page (entries 4 and 5).
3. pnl months without a split lot [17]: folded into one group (entry 5).
4. pnl "no stored quotes" on every lot: expected data, not a bug. The quote archive starts 2026-11-07 (`option_pnl_page/zh_texts.go:6-7` says so) and `reasonOf` (`lots.go:75-81`) gives a gap to every lot without stored quotes in its life. No fix; Decision 3 removes the repetition.
5. Stale quote on every row [3]. Cause: `chainFresh` (`internal/compute/optioncand/events.go:20-23`) is true only when the chain day equals the latest session day on or before the run day; the job runs for session day D after the close (`internal/jobs/optioncandidates/job.go:1-3`) but the chain it reads is the newest stored one (`input.go:127`, `chain[id]`), which is D-1 until the end-of-day chain of D is stored. So `stale_chain` goes into the events gate and `stale_quote` into the badges of every row on every normal day (`optiontier/badges.go:50`). Fix: the chain is fresh when its day is the run day or the XNYS session just before it (Monday gives Friday, holidays are skipped because only session days are in `SessionDays`); a chain two or more sessions old, which is a collector that missed a session, stays stale and keeps its flag, so the flag is not hidden when it matters (entry 3; the comment in `internal/compute/api.go:410-416` changes by at most one net line, the file has 839 lines against a ceiling of 840). The earnings and ex-dividend comparisons against `ChainDay` in `events.go` are unchanged. I did not read the schedule of the option_eod collector; if the chain of D is stored later than the job, running the job after it is the other fix and is not decided here. Rows already stored keep their flag until the job runs again for the day; that is a run of the existing job, not part of this design.
6. Candidates page [1, 2, 4, 5 and the earnings and spot contradictions]:
   a. Companies with neither buy price nor valuation file go into one line `idle_note` and get no strip; the repeated valuation sentence disappears with them (`tiers.go:203-230` writes a strip for every company of the answer).
   b. One Chinese line `buy_hint` says where the buy price is typed (Companies > Watch list, Buy column); the web file links it to `/underlyings`.
   c. An empty Fits or Check tier prints `None: ...` with its reason and no bare table header (the mockup's wording).
   d. Past earnings date: `companiesOf` copies the stored `NextEarnings` (`tiers.go:224-226`) with no comparison with the data day, so NVDA 2026-11-17 shows on 2026-11-18. A date before the data day is shown as unknown. The tracking page has the same fault (Decision 8).
   e. Spot day: `tiers.go:212` writes `SpotDay` as the group day (the run day), while the spot is the price of the chain rows read at the job run. No reader is added (the catalog has no "on or before" bound and is keyed by id): `SpotDay` stays the day of the stored rows, the strip reads `Spot <spot> (rows of <day>)` and a Chinese `spot_hint` says the spot is the price taken when the job ran, never that it is the spot's day. The tracking page dates its stock close by `MarkDay` (Decision 8). I did not find why the two numbers differ (candidates 153.57 against tracking 156.05 for NVDA).
   f. The tiers come before the company strips in the web file (the strips were 2400px above the tables).
   g. `TierView.companies` now lists only companies with a buy price or a valuation figure. Its consumers: `web/src/pages/candidates/TierView.tsx` and `CompanyStrip.tsx` (the strips, with their `counts` text), the fillers of `valueline.go` (they loop over the listed companies), and the fixtures and tests of `web/test/pages/optioncandidates`. The legend counts (Fits, Check, grey) come from the tiers, not from the strips, and are unchanged; the per-company counts of an idle company (all grey) are not shown, which is the point of the line.
7. Notices [8, 9, 10]. Causes: the explanation sentences are English constants in `option_notices_page/text.go:25-26,40-60` although the comment at line 14 says other text is Chinese, and the cash gloss is an English constant in `web/src/pages/notices/AliasBlock.tsx:16-17`. The web file may not hold Chinese (`docs/design/ui-english.md`, decision 1), so the gloss moves to the payload: the optional text `cash_gloss` on `AliasBlock` and, on each cash row, `available_text` and `left_text` (entry 1, interface). Per due date the rule computes the account cash and the cash left after the cumulative dues up to that date (entry 16, compute, `big.Rat`; the page may not compute and the web only shows). The `TotalCashValue` row used is the one of the alias currency, as the cash basis of `docs/design/candidates-tiers.md`; a due in another currency, or an account with no such row, shows no figure rather than a mixed one. A negative `Left` is a shortfall. Put lines get the labels `Strike` and `Qty` in the web file (English labels).
8. Tracking [11, 12, 13, 14]. Causes: the stock close is `UnderlyingPrice` of the newest option_eod row of the contract (`context.go:282`), shown with no day (`tables.go:53`) while `MarkDay` is known; the next earnings date is copied (`cells.go:293-295`); coverage counts from the opening day (`coverage.go:70`, `sessionsFrom`) although the archive starts 2026-11-07, which gives the 98 percent; the note names a page that does not exist (`strings.go:10`). Fix: dated stock close, past date as a dash, coverage from the first stored quote in one short text without the gap list, `Kept` and `Kept %` right after `Premium`, and the note text `see Held options > Structures`. The report suggests dropping the Ex-dividend and Early assignment columns; they are the call rows' cells, so they stay and move right (nothing is removed).
9. Structures [21, 22]. Cause of "No data": `structures_page/load.go:141` skips every underlying with no `STOCK` row in the capture (`stocks[p.Underlying] == 0`, filled at `load.go:110-111`), and a cash-secured seller holds no shares, so no quote is read and every figure of the page is a gap. The tracking page finds the underlying through the catalog and the instrument names (`option_tracking_page/context.go:185-209`); the structures page does the same through two new optional readers in its `Config` (entry 9) and `cmd/ot` hands them over (entry 15, after entry 9 because the fields are declared there). "Unidentified" [22]: the cause is the rule, not the page: `related` in `internal/compute/optionstruct/recognise.go` ties a leg to any opposite leg of the same right and to any leg of the other right at the same expiry, so a short put with a long put of another expiry, or a short put with a short call of one expiry, becomes one `other` unit (the comment above `entangled` says a diagonal, calendar, straddle or collar is `other`). That is Decision 4 of `docs/design/option-positions.md`; changing it is not a page fix, so it is in Left out. The page only says in one Chinese line which tie made a multi-leg unit (legs of one right on opposite sides at different expiries, or a put and a call at one expiry).
10. Result [18, 19, 20]. Causes: `sections.go:81,83` write the two "note is empty" lines for the owner as placeholders; `reliabilityOf` (`reliability.go:37`) is called for each of the three sections with the same account-wide `Unresolved` and `Review` lists (`optionresult.go:91`), so the ids repeat three times; the page shows one account (the query `account`, default the first alias, `cmd/ot/site_positions.go:92-96`) and has no word about the others. Fix: delete the two lines, the ids once, a Chinese scope note naming the account and the others. The cause of the 27 differing trades is not found in code (the compute counts them in `optionresult/reliability.go`); the page only explains what the two figures compare, without a cause.
11. Menu [6, 7 as a whole, Advice, Data]. `web/src/shell/TopNav.tsx:39-52`: Overview and Advice leave the Options menu; `Data` moves to the System menu with the label `Option data`. No page code is deleted: the routes stay reachable by address, and the pages are not "rebuilt as a status page" as the report offered, because the candidates page already shows the data day, the chain day and the run time.
12. `/options` (route `options`, `cmd/ot/option_page_site.go:25`) answers a 302 to `/options/candidates` (entry 14): GET and HEAD answer 302, other methods as today. Of the overview only the data day line was useful and the candidates page has it. The live-quote Refresh button of the overview is not moved (Left out).
13. Owner rules bound into every entry: explanation and risk texts Chinese only as constants in a `zh_texts.go` of the page package (`ui-english.md` decision 1); labels and numbers English; jargon glossed beside it; the web files render payload texts and compute nothing; no time guard; no new write; no new file over 600 lines and no ceiling raised (a file that would pass its ceiling gets its new code in a new file of the package, said in the pull request).
14. Notices cash per date: see Decision 7 (entries 1, 16, 6, 7); the report's fix 9 asked for cash beside each due date and that is what is built.

## Modules

`docs/design/modules.md`: no row is added or changed. Paths and the first row whose glob matches: `contracts/schemas/**`, `web/src/generated/**` (web row first, see below), `internal/contracts/**` and `docs/contracts/page-schemas.md` are the contracts row (entries 1, 2); `internal/compute/**`, `docs/contracts/compute.md` and `docs/contracts/compute/**` compute (entries 3, 16); `internal/pages/**` pages (entries 4 to 6, 8 to 11); `web/**` web (entries 7, 12, 13); `cmd/**` ot (entries 14, 15). `web/src/generated/*.ts` is generated code that the interface issue edits together with its schema, as `docs/design/options-pnl-ledger.md#Interface` did; the modules check for entries 1 and 2 follows that precedent.

## Interface

Four entries change an interface and land first; the contracts wording is `<id> v1 additive` (`docs/contracts/00-protocol.md`). All are additive; no `schema_version` changes (the schemas of these two pages are edited in place, as `page.positions_page.v3` and `option_notices_page` were before).

1. (interface) `page.option_notices_page.v1`: `AliasBlock.cash_gloss`, `CashRow.available_text` and `CashRow.left_text`, optional `NullableText` (entry 1).
2. (interface) `page.option_candidates_page.v1`: `TierView.idle_note`, `buy_hint` and `spot_hint`, optional `NullableText`; and the meaning of `TierView.companies` (Decision 6g) (entry 2).
3. (interface) `internal/compute/api.go`: the comment of `CandidateInput` states the new chain-freshness rule; no declaration changes (entry 3).
4. (interface) `internal/compute/api.go`: `CashDay` gains `Available` and `Left` in the line of `Amount` (entry 16).

## Issues

Each entry is one module (`docs/design/modules.md`); no entry spans two modules. `(interface)` marks the entries that change a schema or the compute interface file; they land before the entries that use them (`Order:`). The numbers below name entries of this design only.

### 1 notices-contract (interface)

Module: contracts. Size: S.
Order: first.
Files: `contracts/schemas/page.option_notices_page.v1.json`, `web/src/generated/option_notices_page.v1.ts`, `internal/contracts/option_notices_page_v1_test.go`, `docs/contracts/page-schemas.md`
Contracts: page.option_notices_page v1 additive
Design: docs/design/option-pages-r7.md#1
Plan: P2
The notices page gets one optional text on `AliasBlock` (the cash gloss) and two optional texts on `CashRow` (the account cash and the cash left after the dues up to that date), so the Chinese gloss and the figures come from the payload and the web file holds no English sentence and no sum (Decisions 7 and 14).
Acceptance:
1. The schema `page.option_notices_page.v1.json` gives `AliasBlock` the optional property `cash_gloss` and `CashRow` the optional properties `available_text` and `left_text`, each `NullableText`; `required` of both is unchanged and `schema_version` const is still 1. The names and texts named here are new.
2. `web/src/generated/option_notices_page.v1.ts` has both fields and `git diff --name-only BASE` lists it together with the schema (never edited by hand).
3. `internal/contracts/option_notices_page_v1_test.go` gains `TestOptionNoticesPageCashFieldsValid` (new: a block with the gloss and a cash row with both texts validates) and `TestOptionNoticesPageWithoutCashFieldsValid` (new: a block without them validates, the additive rule).
4. `docs/contracts/page-schemas.md` gains one sentence in the `page.option_notices_page.v1` paragraph: the three optional fields were added by `docs/design/option-pages-r7.md#Interface` as `v1 additive`, without a version change. The text named here is new.
5. `git diff --name-only BASE -- contracts/schemas/carryover.sha256 internal/contracts/common_test.go` prints nothing.
6. The result matches docs/design/option-pages-r7.md#1.

### 2 candidates-contract (interface)

Module: contracts. Size: S.
Order: after 1.
Files: `contracts/schemas/page.option_candidates_page.v1.json`, `web/src/generated/option_candidates_page.v1.ts`, `internal/contracts/option_candidates_page_v1_test.go`, `docs/contracts/page-schemas.md`
Contracts: page.option_candidates_page v1 additive
Design: docs/design/option-pages-r7.md#2
Plan: P2
The tier view gets three optional texts: one line for the companies that have neither buy price nor valuation file, one line telling where the buy price is typed, and one hint saying what the spot of a strip is (Decision 6). It follows entry 1 because both edit `docs/contracts/page-schemas.md`.
Acceptance:
1. The schema `page.option_candidates_page.v1.json` gives `TierView` the optional properties `idle_note`, `buy_hint` and `spot_hint`, each `NullableText` (the same `oneOf` of `Text` and null as `kept_note`); `required` of `TierView` is unchanged and `schema_version` const is still 1. The names and texts named here are new.
2. `web/src/generated/option_candidates_page.v1.ts` has both fields and `git diff --name-only BASE` lists it together with the schema (never edited by hand).
3. `internal/contracts/option_candidates_page_v1_test.go` gains `TestOptionCandidatesTierViewIdleFieldsValid` (new: a tier view with the three texts validates) and `TestOptionCandidatesTierViewWithoutIdleFieldsValid` (new: a tier view without them validates).
4. `docs/contracts/page-schemas.md` gains one sentence in the `page.option_candidates_page.v1` paragraph: `TierView.idle_note`, `buy_hint` and `spot_hint` were added by `docs/design/option-pages-r7.md#Interface` as `v1 additive`, without a version change; and `TierView.companies` lists only the companies that have a buy price or a valuation figure, the others being named in `idle_note`. The text named here is new.
5. `git diff --name-only BASE -- contracts/schemas/carryover.sha256 internal/contracts/common_test.go` prints nothing.
6. The result matches docs/design/option-pages-r7.md#2.

### 3 chain-fresh-rule (interface)

Module: compute. Size: S.
Order: first.
Files: `internal/compute/optioncand/events.go`, `internal/compute/optioncand/events_test.go`, `internal/compute/api.go`, `docs/contracts/compute.md`
Contracts: unchanged
Design: docs/design/option-pages-r7.md#3
Plan: P2
The chain counts as fresh when its day is the run day or the XNYS session just before it, so a normal overnight run no longer writes `stale_chain` on every row; a chain two or more sessions old stays stale (Decision 5).
Acceptance:
1. `chainFresh` in `events.go` is true when `ChainDay` is the run day's position in `SessionDays`, or the position just before it (Monday gives Friday, a holiday is skipped because only session days are in the list); a `ChainDay` two or more positions before, after `RunDay`, or not in `SessionDays` is false. No clock and no other input is read; the comparisons of earnings and ex-dividend dates against `ChainDay` in the same file are unchanged.
2. The comment of `CandidateInput` in `internal/compute/api.go` (the sentence about a `ChainDay` that is not the latest session day on or before `RunDay`) states the rule of item 1; the file has at most 840 lines after the change (ceiling in `docs/design/ratchet.json`, net change of the comment at most one line) and no declaration changes (`git diff BASE -- internal/compute/api.go` shows only comment lines).
3. `events_test.go` gains `TestChainFreshPreviousSessionIsFresh` (new: run day Wednesday, chain day Tuesday gives no `stale_chain`), `TestChainFreshWeekendRun` (new: run day Monday, chain day the Friday before is fresh) and `TestChainFreshTwoSessionsBehindIsStale` (new: run day Wednesday, chain day Monday gives `stale_chain`); the existing cases of the file keep their expectations except the one that named the previous session as stale, which is changed in the same file.
4. `docs/contracts/compute.md` states the same rule in the paragraph that names `stale_chain`. The text named here is new.
5. The result matches docs/design/option-pages-r7.md#3.

### 4 pnl-count-late-entry

Module: pages. Size: S.
Order: first.
Files: `internal/pages/option_pnl_page/builder.go`, `internal/pages/option_pnl_page/builder_test.go`, `internal/pages/option_pnl_page/cells.go`, `internal/pages/option_pnl_page/cells_test.go`, `internal/pages/option_pnl_page/card.go`, `internal/pages/option_pnl_page/zh_texts.go`, `internal/pages/option_pnl_page/testdata/ledger.golden.json`, `internal/pages/option_pnl_page/testdata/ok.golden.json`, `internal/pages/option_pnl_page/testdata/lot_gap.golden.json`, `internal/pages/option_pnl_page/testdata/no_quotes.golden.json`, `internal/pages/option_pnl_page/strings.go`
Contracts: unchanged
Design: docs/design/option-pages-r7.md#4
Plan: P2
The header of an account prints both the ended and the split lot counts and the 20-lot rule counts split lots; the Entry vs mid line of a late-entry lot and of a book says in Chinese how many lots entered before the first stored quote (Decisions 1 and 2).
Acceptance:
1. `buildAccount` in `builder.go` counts `ended` over `all` (every lot, the ledger lots included; a status other than `open` counts once) and `split` as the ended lots that are not gap lots; the sample note reads `N ended lots, M split lots: ...` (English, both numbers), so it equals the number of ended lots in the ledger months of the account. The names and texts named here are new.
2. The 20-lot threshold of the same function is applied to `split`, not to `ended`: with 30 ended lots of which 5 split the note says fewer than 20 split lots and no conclusion; the sentences `sampleFew` and `sampleEnuf` in `strings.go` say `split lots` instead of `ended lots`. The names and texts named here are new.
3. `zh_texts.go` declares the Chinese pieces `lateEntryLotZh` and `lateEntryBookZh` (new, string constants only): the hint of a lot says that its Entry vs mid line is market movement from the fill to the first stored quote, not slippage; the hint of a book reads, in Chinese, `N of M lots entered before the first stored quote (quotes start 2026-11-07)` with N and M written by the page.
4. The page takes N from the outcomes of the book (the lots whose `res.LateEntry` is true) and M from the lots of the book; it writes them with `strconv` and adds no figure (a count, `purity_test.go` stays green); the hint is the hint of the Entry vs mid line of each lot card with `LateEntry` and of each book card in `bookCards`; the figures and totals are unchanged because the split must add up to the ledger.
5. `builder_test.go` gains `TestSampleNoteCountsEndedAndSplit` (new: two ended lots outside the 30-day card window, one of them a gap lot, give `2 ended lots, 1 split lots`) and `TestSampleThresholdCountsSplit` (new: 25 ended lots of which 5 split say fewer than 20 split lots); `cells_test.go` gains `TestLateEntryHintOnLotAndBook` (new: a book with 2 late of 5 lots carries `2` and `5` in its hint, a lot without `LateEntry` has none).
6. The four golden files under `testdata/` are changed only where the sample note or a hint appears.
7. The result matches docs/design/option-pages-r7.md#4.

### 5 pnl-fold-months

Module: pages. Size: S.
Order: after 4.
Files: `internal/pages/option_pnl_page/ledger.go`, `internal/pages/option_pnl_page/ledger_test.go`, `internal/pages/option_pnl_page/zh_texts.go`, `internal/pages/option_pnl_page/testdata/ledger.golden.json`, `internal/pages/option_pnl_page/cells.go`
Contracts: unchanged
Design: docs/design/option-pages-r7.md#5
Plan: P2
The ledger months in which no lot could be split are folded into one group, and the Entry vs mid line of every month and company group carries the late-entry count hint (Decisions 2 and 3). The no-quotes text is expected data and stays (Decision 4).
Acceptance:
1. `buildLedger` in `ledger.go` puts every month bucket whose lots are all gap lots (no `Combine` input) into one bucket, in the view `by_month`, written once per currency and labelled by its first and last month and its lot count; the buckets with split lots stay one per month. The folded bucket is still given to `Booker.Combine` once and `ledger.go` adds no figure and no count itself (`purity_test.go` stays green).
2. The label and the one-line explanation of the folded group are Chinese constants in `zh_texts.go` (new constants `foldedMonthsZh` and `foldedNoteZh`, string constants only); the explanation says that no quote is stored before 2026-11-07 so only the ledger figure exists for these lots.
3. Each group of `buildLedger` (month, company and the folded one) has the Entry vs mid hint of entry 4 (`lateEntryBookZh`, new in entry 4) with N the late-entry lots and M the lots of that group, taken from the outcomes of the group by counting; `ledger.go` adds no figure and no count by arithmetic beyond `len` and counting a flag. The names and texts named here are new.
4. `ledger_test.go` gains `TestLedgerFoldsMonthsWithoutSplit` (new: three months with no split lot and one with a split lot give two groups), `TestLedgerFoldedGroupSumsMonths` (new: the folded group's lot count and ledger figure equal the sums over the three months, read from the `Booker` result of the group) and `TestLedgerGroupLateEntryCount` (new: a month group with 2 late of 4 lots carries 2 and 4 in its hint).
5. `testdata/ledger.golden.json` is changed only in the groups of the folded months; `ok.golden.json`, `lot_gap.golden.json` and `no_quotes.golden.json` are not in `git diff --name-only BASE`.
6. The files `web/src/pages/optionpnl/*` and `contracts/schemas/page.option_pnl_page.v1.json` are not in `git diff --name-only BASE`: the schema's `Group` already carries a title and a summary.
7. The result matches docs/design/option-pages-r7.md#5.

### 6 notices-page

Module: pages. Size: M.
Order: after 1 and 16.
Files: `internal/pages/option_notices_page/zh_texts.go`, `internal/pages/option_notices_page/text.go`, `internal/pages/option_notices_page/blocks.go`, `internal/pages/option_notices_page/blocks_test.go`, `internal/pages/option_notices_page/text_test.go`, `internal/pages/option_notices_page/testdata/no_rows.json`, `internal/pages/option_notices_page/testdata/second_missing.json`, `internal/pages/option_notices_page/testdata/severities.json`
Contracts: unchanged
Design: docs/design/option-pages-r7.md#6
Plan: P2
The explanation texts of the notices page become Chinese in a `zh_texts.go`, the cash gloss moves into the payload, and each account block carries its snapshot cash beside the cash-due table (Decision 7).
Acceptance:
1. `zh_texts.go` is new in `internal/pages/option_notices_page` and holds string constants only: the Chinese forms of the no-guard hint (now `noteHint` in `text.go`), the cushion hint sentence (now built in `cushionHintText`, keeping the thresholds as the numbers it is given), the reason texts of `reasonTexts`, and the new cash gloss (`cashGlossZh` (new), the sentence now written in `web/src/pages/notices/AliasBlock.tsx` as `CASH_GLOSS`, in Chinese, with `strike × multiplier × quantity` and the word Assigned glossed). The titles, labels and the note labels in `text.go` stay English.
2. `okBlock` in `blocks.go` sets `CashGloss` to `cashGlossZh` for a block with status `ok` or `stale`, and for each cash row `AvailableText` and `LeftText` from `compute.CashDay.Available` and `.Left` written by `amountText` (labels English); a row whose texts are empty has both null, and so has a `no_capture` block. The page does no arithmetic: both figures come from entry 16. The names and texts named here are new.
3. The comment `other text is Chinese` above the constants of `text.go` is true: no explanation sentence left in `text.go` is English.
4. `blocks_test.go` gains `TestBlockCarriesCashGlossAndPerDateCash` (new: cash days with `Available` and `Left` give both texts per row, empty ones give null) and `text_test.go` gains `TestExplanationsAreChinese` (new: `noteHint`, the cushion hint and the cash gloss each contain a CJK rune).
5. The three files under `testdata/` change only where the new fields or the Chinese texts appear.
6. The result matches docs/design/option-pages-r7.md#6.

### 7 notices-web

Module: web. Size: S.
Order: after 1.
Files: `web/src/pages/notices/AliasBlock.tsx`, `web/src/pages/notices/CashPuts.tsx`, `web/test/pages/optionnotices/OptionNoticesPage.test.tsx`, `web/test/pages/optionnotices/fixtures/ok.ts`, `web/test/pages/optionnotices/fixtures/second-missing.ts`
Contracts: unchanged
Design: docs/design/option-pages-r7.md#7
Plan: P2
The cash table shows the gloss and the account cash from the payload and labels each put's strike and quantity (Decision 7).
UI:
- 按 frontend-stack：用 shadcn `Table` 与 `Alert` 做 现金表的说明行（仍用现有组件，不加新库）。
- 按 frontend-stack：用 Tailwind `text-muted-foreground` 做 账户现金与标签的次要文字。
Acceptance:
1. `AliasBlock.tsx` has no `CASH_GLOSS` constant and no English sentence of explanation left: the gloss under the title `Cash due if assigned` is `block.cash_gloss` when it is a string, and each cash row shows `available_text` under the label `Account cash` and `left_text` under the label `Left after dues` when they are strings; `git grep -n 'reduce\|parseFloat\|Number(' -- web/src/pages/notices` prints nothing. The names and texts named here are new.
2. The file CashPuts.tsx writes the labels `Strike` and `Qty` before `strike_text` and `quantity_text` of each put (English labels, the values as sent), so a line reads `TSM 2026-12-11 Strike 340 P Qty 1`. The names and texts named here are new.
3. `OptionNoticesPage.test.tsx` gains `renders the cash gloss and the per-date cash from the payload` (new) and `labels strike and quantity of each put` (new); `fixtures/ok.ts` and `fixtures/second-missing.ts` carry the new fields, invented numbers.
4. The result matches docs/design/option-pages-r7.md#7.

### 8 tracking-page

Module: pages. Size: M.
Order: first.
Files: `internal/pages/option_tracking_page/tables.go`, `internal/pages/option_tracking_page/tables_test.go`, `internal/pages/option_tracking_page/cells.go`, `internal/pages/option_tracking_page/cells_test.go`, `internal/pages/option_tracking_page/coverage.go`, `internal/pages/option_tracking_page/coverage_test.go`, `internal/pages/option_tracking_page/strings.go`, `internal/pages/option_tracking_page/zh_texts.go`, `internal/pages/option_tracking_page/builder.go`, `internal/pages/option_tracking_page/builder_test.go`, `internal/pages/option_tracking_page/context.go`, `internal/pages/option_tracking_page/context_test.go`
Contracts: unchanged
Design: docs/design/option-pages-r7.md#8
Plan: P2
The tracking page dates the stock close, shows no earnings date that has passed, counts quote coverage only from the first stored quote, keeps Kept % in view and names an existing page in its note (Decision 8).
Acceptance:
1. The stock close cell of an open row (column `spot` of `openColumns` in `tables.go`) carries the day of the figure, `MarkDay` of the tracked option, as part of the cell (the day written `2026-11-17` beside the price, or the column hint when every row has the same day); a row with no `MarkDay` shows the price without a day.
2. `resultsCell` in `cells.go` shows the dash, not the date, when `NextEarnings` is before the page day (`AsOf`), and the page day reaches the function through the table builder, not through a clock; a date on or after the page day is shown as before.
3. `coverageOf` in `coverage.go` counts the trading days from the later of the opening day and the first stored two-sided or one-sided quote day of that contract's underlying that the page read (not from the opening day alone), and the row's cell holds one short text of the form n of m days (new); the list of gap days is not written on the row; the footer note counts over the same span.
4. The columns of `openColumns` are ordered so that `kept` and `kept_percent` come directly after `premium` and before `delta`; no column is removed, so call rows keep their Ex-dividend and Early assignment cells.
5. `leftOutTail` in `strings.go` names an existing page: `long legs not listed, see Held options > Structures` (new text); the words `option positions page` appear nowhere in the package.
6. `tables_test.go` gains `TestStockCloseCarriesDay` (new) and `TestKeptPercentBeforeDelta` (new); `cells_test.go` gains `TestPastEarningsShowsDash` (new); `coverage_test.go` gains `TestCoverageCountsFromFirstStoredQuote` (new: a position opened ten sessions before the first stored quote counts only the sessions from that quote); `builder_test.go` changes the assertion on the long-legs note to the new text.
7. The result matches docs/design/option-pages-r7.md#8.

### 9 structures-quotes

Module: pages. Size: M.
Order: first.
Files: `internal/pages/structures_page/page.go`, `internal/pages/structures_page/load.go`, `internal/pages/structures_page/load_test.go`, `internal/pages/structures_page/list.go`, `internal/pages/structures_page/list_test.go`, `internal/pages/structures_page/testdata/list_ok.json`, `internal/pages/structures_page/testdata/list_gaps.json`
Contracts: unchanged
Design: docs/design/option-pages-r7.md#9
Plan: P2
The structures page finds the underlying of a held option the way the tracking page does, so a seller who holds no shares still gets quotes, moneyness and floating P/L (Decision 9).
Acceptance:
1. `Config` in `page.go` gains two optional readers, `Catalog store.OptionEodCatalog` and `Names store.InstrumentNameReader`, each nil giving the old behaviour.
2. `quotes` in `load.go` takes the instrument id of an underlying from the catalog and the instrument names (the symbol of a held option equals the name, as `underlyingIDs` in `option_tracking_page/context.go` does) when `Catalog` and `Names` are set, and from the `STOCK` rows of the capture only when they are nil; a held underlying with no stock row in the capture still gets its quotes.
3. `list.go` gives every multi-leg unit of kind `other` a one-line Chinese reason in the group note that says which tie made the unit: `legs of one right on opposite sides at different expiries`, or `a put and a call at one expiry` (two constants in a new `zh_texts.go`, string constants only; the page decides by comparing the stored `Right` and `Expiry` of the legs, no new rule); the recognised kinds do not change.
4. `load_test.go` gains `TestQuotesFoundWithoutStockRow` (new: a capture with option rows and no stock row, a catalog and a name reader give the quote of the held put) and `TestQuotesFromStockRowWhenNoCatalog` (new: the old path still works with nil readers); `list_test.go` gains `TestUnidentifiedHasReason` (new).
5. `testdata/list_ok.json` and `list_gaps.json` change only where the moneyness, the distance, the earned ratio or the new reason text appear.
6. The result matches docs/design/option-pages-r7.md#9.

### 10 result-page

Module: pages. Size: S.
Order: first.
Files: `internal/pages/option_result_page/sections.go`, `internal/pages/option_result_page/sections_test.go`, `internal/pages/option_result_page/reliability.go`, `internal/pages/option_result_page/reliability_test.go`, `internal/pages/option_result_page/build.go`, `internal/pages/option_result_page/build_test.go`, `internal/pages/option_result_page/zh_texts.go`, `internal/pages/option_result_page/testdata/result_ok.json`, `internal/pages/option_result_page/testdata/result_empty.json`
Contracts: unchanged
Design: docs/design/option-pages-r7.md#10
Plan: P2
The result page drops the two lines that say a note is empty, writes the unpaired and review ids once for the whole account, and says which account it shows (Decision 10).
Acceptance:
1. `sections.go` no longer writes `Assigned note is empty` or `Protection note is empty`: no `FigureNotes` entry says that a note is empty, and the string `is empty` is in no non-test file of the package.
2. `reliabilityOf` in `reliability.go` writes the lists `Unresolved` and `Review` (account-wide, not per section) in one reliability block of the page, once; the per-section reliability lines keep the three counts and no id list. A page with three sections has the id list once.
3. `zh_texts.go` is new and holds string constants only: a Chinese gloss of the reconciliation block (what `Same` and `Differs` compare, without naming a cause), written once under the heading, and a scope note `本页只含账户 <alias>` followed by the other aliases of `Config.Aliases` when there are any.
4. `reliability_test.go` gains `TestIdListsOnceForThreeSections` (new); `sections_test.go` gains `TestNoEmptyNoteLines` (new); `build_test.go` gains `TestScopeNoteNamesOtherAccounts` (new).
5. `testdata/result_ok.json` and `result_empty.json` change only where the notes, the id lists or the new texts appear; `contracts/schemas/page.option_result_page.v1.json` is not in `git diff --name-only BASE`.
6. The result matches docs/design/option-pages-r7.md#10.

### 11 candidates-page

Module: pages. Size: L.
Order: after 2.
Files: `internal/pages/option_candidates_page/tiers.go`, `internal/pages/option_candidates_page/tiers_test.go`, `internal/pages/option_candidates_page/valueline.go`, `internal/pages/option_candidates_page/valueline_test.go`, `internal/pages/option_candidates_page/zh_texts.go`, `internal/pages/option_candidates_page/builder.go`, `internal/pages/option_candidates_page/builder_test.go`, `internal/pages/option_candidates_page/testdata/tiers_ok.golden.json`, `internal/pages/option_candidates_page/testdata/tiers_gaps.golden.json`, `internal/pages/option_candidates_page/testdata/valueline.golden.json`
Contracts: unchanged
Design: docs/design/option-pages-r7.md#11
Plan: P2
The candidates page puts the companies without buy price and valuation on one line, says where the buy price is typed, states what an empty tier means, shows no past earnings date and says what the spot of a strip is (Decision 6).
Acceptance:
1. `companiesOf` and `buildTiers` give `Companies` only the companies that have a buy price or a valuation figure; the others are named in `TierView.IdleNote`, one Chinese line of the form `N 家公司没有买入价和估值档案：<tickers>` (constant in `zh_texts.go`, the tickers joined by the page, cut with an ellipsis before 200 characters); with no such company `IdleNote` is null. The repeated per-company valuation sentence therefore appears at most for companies that have a figure. The names and texts named here are new.
2. `TierView.BuyHint` is set to a Chinese line (constant in `zh_texts.go`) saying that no buy price is set and where to set it (the Buy column of Companies > Watch list) when at least one company has none; otherwise null. The names and texts named here are new.
3. An empty Fits or Check tier has `Explain` starting with `None:` in Chinese (constant in `zh_texts.go`) followed by the reason that applies, set in `buildTiers` of `tiers.go` after the rows are added (`explainFits` of `tierlegend.go` and `tierrows.go` are read, not edited); a tier with rows keeps `explainFits` and `explainCheck`.
4. `NextEarnings` of a company is `earningsUnknown` when the stored date is before the data day (the day `Build` was called for), not the stored date; a date on or after it is shown as before.
5. `SpotDay` of a company stays the day of the stored rows (`gr.day`, no new reader and no `Config` field); `TierView.SpotHint` is a Chinese constant of `zh_texts.go` saying that the spot is the price taken when the job ran for that day, not the close of that day; the field `SpotDay` is never described as the spot's day in a text of the page. The names and texts named here are new.
6. The stale-quote mark is not edited here: `tierstrings.go` and the `common` marks stay as they are, because entry 3 changes the rule that writes `stale_chain`.
7. `tiers_test.go` gains `TestIdleCompaniesOneLine` (new), `TestEmptyTierSaysNone` (new), `TestPastEarningsIsUnknown` (new) and `TestSpotHintSet` (new); `valueline_test.go` keeps its cases for a company with a valuation line.
8. The three golden files under `testdata/` change only where the idle line, the hint, the tier explanations, the earnings text or the spot day appear. No new source file is over 600 lines and no existing file of the Files line passes its ceiling in `docs/design/ratchet.json`; a file that would is reported in the pull request description and its new code goes to a new file of the package.
9. The result matches docs/design/option-pages-r7.md#11.

### 12 candidates-web

Module: web. Size: M.
Order: after 2.
Files: `web/src/pages/candidates/TierView.tsx`, `web/src/pages/candidates/CompanyStrip.tsx`, `web/src/pages/candidates/TierRows.tsx`, `web/test/pages/optioncandidates/TierView.test.tsx`, `web/test/pages/optioncandidates/CompanyStrip.test.tsx`, `web/test/pages/optioncandidates/fixtures/tiers.ts`, `web/test/pages/optioncandidates/fixtures/tiers-null.ts`
Contracts: unchanged
Design: docs/design/option-pages-r7.md#12
Plan: P2
The candidates page shows the buy-price hint with a link, the idle-companies line, and no empty table headers; the tiers come before the strips (Decision 6).
UI:
- 按 frontend-stack：用 shadcn `Alert` 做 买入价提示行（带去 `underlyings` 的链接）。
- 按 frontend-stack：用 shadcn `Accordion` 做 空档位与未设买入价公司的折叠说明（沿用现有组件，不加新库）。
Acceptance:
1. `TierView.tsx` renders, in this order: `buy_hint` as a link to the route `underlyings` when it is a string, the legend, the tiers, then `idle_note` and the company strips; no strip is written for a company the payload does not list. A string field that is null renders nothing. The names and texts named here are new.
2. `CompanyStrip.tsx` writes the spot as `Spot <spot> (rows of <spot_day>)` and shows `spot_hint` of the tier view as the hint of the word Spot when it is a string; the day is never labelled as the day of the spot. The names and texts named here are new.
3. `TierRows.tsx` renders for a tier with no rows only its title and its `explain` text, not the table header row; a tier with rows renders as before.
4. The three files render texts as sent and compute, count and compare nothing.
5. `TierView.test.tsx` gains `renders the buy hint as a link to the watch list` (new), `renders the idle line once` (new) and `renders no table header for an empty tier` (new); `fixtures/tiers.ts` and `fixtures/tiers-null.ts` carry the two new fields, invented numbers.
6. The result matches docs/design/option-pages-r7.md#12.

### 13 nav-options

Module: web. Size: S.
Order: first.
Files: `web/src/shell/TopNav.tsx`, `web/test/shell/TopNav.test.tsx`
Contracts: unchanged
Design: docs/design/option-pages-r7.md#13
Plan: P2
The Options menu loses Overview and Advice, and the option chain browser moves under System (Decision 11). The `Lightbulb` import of `TopNav.tsx` and the test rows of the removed links go with it.
UI:
- 按 frontend-stack：用 shadcn `DropdownMenu`（现有 TopNav 组件）做 菜单项的移动，不加新库。
Acceptance:
1. `TopNav.tsx` has no link with the path `options` or `advice`; the Options menu lists, in this order, `Candidates`, the section `Held options` with `Structures`, `Result`, `Notices`, `Tracking`, `P&L split`.
2. The System menu lists `Settings`, `Job console`, `Data health` and `Option data` (path `options/data`, `prefix: true` so a sub-route marks it); the Options menu has no `Data` link.
3. `TopNav.test.tsx` expects the lists of item 1 and item 2; the rows of `TestTopNavMarksGroup` for the routes options, advice and options/data/9001 are removed or moved (options/data/9001 expects `System`); `marks Data in the Options menu at options/data/9001` is renamed `marks Option data in the System menu at options/data/9001` (new name).
4. `TopNav.tsx` no longer imports `Lightbulb` (`git grep -n Lightbulb -- web/src/shell` prints nothing).
5. The pages `web/src/pages/advice` and `web/src/pages/options` and their routes are not in `git diff --name-only BASE` (the routes stay reachable by address).
6. The result matches docs/design/option-pages-r7.md#13.

### 14 ot-options-redirect

Module: ot. Size: S.
Order: first.
Files: `cmd/ot/site.go`, `cmd/ot/option_page_site_test.go`, `cmd/ot/options_redirect_test.go`
Contracts: unchanged
Design: docs/design/option-pages-r7.md#14
Plan: P2
The site sends the route `options` to the candidates page (Decision 12).
Acceptance:
1. `siteHandler` in `site.go` answers a GET or a HEAD of the route `options` (no sub-path) with a 302 to the route `options/candidates` before the page-family lookup; any other method on `options` answers as it does today; `options/data`, `options/candidates` and the other sub-routes are served as before, and the page family `option_page` stays registered for the command ot page option_page and the serve check.
2. `option_page_site_test.go` keeps the tests that build the page directly (`buildOptionPage`, the named page, the readers, the refresh state) and changes each test that fetched the route `options` through the handler to assert that GET and HEAD answer 302 with the location options/candidates, the body assertions (scope, refresh state) moving to a call of the page's build function.
3. `options_redirect_test.go` is new and holds `TestOptionsRouteRedirectsToCandidates` (new: GET and HEAD of `options` are 302 with that location, POST is not 302, GET of `options/data` is not 302); `site_test.go` is not in `git diff --name-only BASE` (796 lines).
4. `cmd/ot/pagereg_test.go` and `cmd/ot/check_serve.go` are not in `git diff --name-only BASE`.
5. The result matches docs/design/option-pages-r7.md#14.

### 15 ot-structures-wiring

Module: ot. Size: S.
Order: after 9.
Files: `cmd/ot/site_positions.go`, `cmd/ot/site_positions_test.go`
Contracts: unchanged
Design: docs/design/option-pages-r7.md#15
Plan: P2
The site hands the structures page the catalog and names readers (Decision 9). It follows entry 9 because the two `Config` fields it sets are declared there.
Acceptance:
1. The `structures_page.Config` built in `site_positions.go` sets `Catalog` and `Names` from the readers of the site (the ones the tracking page already uses, `rd.catalog` and `rd.names`); in the gap case (`rd = siteReaders{}`) they stay nil.
2. `site_positions_test.go` gains `TestStructuresConfigHasCatalogAndNames` (new: the config the handler passes to `structuresListPage` has both readers non-nil with stored readers, nil in the gap case).
3. `cmd/ot/candidates_site.go` and `cmd/ot/evidence_site.go` are not in `git diff --name-only BASE` (the candidates page gets no new reader).
4. The result matches docs/design/option-pages-r7.md#15.

### 16 notices-cash-compute (interface)

Module: compute. Size: S.
Order: first.
Files: `internal/compute/api.go`, `internal/compute/optionnotice/cash.go`, `internal/compute/optionnotice/cash_test.go`, `internal/compute/optionnotice/notice.go`, `docs/contracts/compute/optionnotice.md`
Contracts: unchanged
Design: docs/design/option-pages-r7.md#16
Plan: P2
The notices rule gives each cash-due row the account cash and the cash left after the dues up to that date, so the page and the web only show them (Decision 14). The two fields are written in the line of the existing `Amount` field, so `internal/compute/api.go` keeps its line count (839 of a ceiling of 840).
Acceptance:
1. `CashDay` in `internal/compute/api.go` declares `Currency, Amount, Available, Left string` in the one line that now declares `Currency, Amount string`; `git diff --numstat BASE -- internal/compute/api.go` shows one line added and one removed, plus comment lines of at most one net line, and the file stays at most 840 lines. The names and texts named here are new.
2. `cashByDate` in `cash.go` takes the values of the input (the `Values` of the input) and, for each row, sets `Available` to the `TotalCashValue` value whose currency is the currency of the alias (the cash basis of `docs/design/candidates-tiers.md`: one row, the alias currency) when the row's currency equals it, and `Left` to `Available` minus the sum of `Amount` of this row and of the earlier rows of the same currency (a negative `Left` is a shortfall, written with its sign); a row of another currency, and every row when no such cash row exists, has both empty; arithmetic uses `big.Rat` and `formatTrim` as the rest of the file.
3. `notice.go` passes `in.Values` to `cashByDate`; `docs/contracts/compute/optionnotice.md` states the two fields and the currency rule in one paragraph. The text named here is new.
4. `cash_test.go` gains `TestCashLeftAfterCumulativeDues` (new: cash 50000 USD and dues 34000 and 14000 on two dates give `Left` 16000 and 2000), `TestCashLeftNegativeIsShortfall` (new: cash 10000 and a due of 14000 give `Left` -4000) and `TestCashOtherCurrencyHasNoFigure` (new: a JPY due with a USD cash row has empty `Available` and `Left`).
5. The result matches docs/design/option-pages-r7.md#16.

## Left out

- Structures "Unidentified" rule [22] (its own design after a listing of the legs, with the tie named per unit by entry 9): a change of `related` in `recognise.go` is a change of the structure rule (Decision 4 of `docs/design/option-positions.md`), not a page fix. Ask: should a short put with a long put of another expiry be a put spread-like unit or stay `other`? Not decided here.
- Why 27 trades differ from the broker [20]: no cause found in code; the page explains what is compared. A cause needs a read of the broker rows and the ledger of one day.
- Why candidates spot and tracking stock close differ for the same company: not found (Decision 6e); both are dated now.
- What only the overview showed and becomes unreachable from the menu: the opportunity cards (rank-1 row per underlying, `RankInUnderlying`), the verdict column with its tones, the annualised, IVP and E columns and the per-market tabs; the candidates page has no counterpart and the owner has not asked for one.
- Live-quote Refresh button of the overview: live quotes are off today; the button returns to the candidates page when the owner turns them on.
- Deleting the overview page code, its producer and `/advice` page code: the routes stay for a week behind the redirect and the menu change; deleting them is a separate clean-up after the owner confirms nothing links there.
- Report items outside "a": `/options/data` default underlying and Chinese notes [23, 24], `/underlyings` [25, 26], the earnings dates known for 8 of 35 companies, JP names, tracking sort and roll candidates.
- Re-running the candidates job for the current day so the stale flag clears on stored rows (a run of the existing job; with `-force` for a stored day).
- Entry vs mid excluded from the total [16]: kept in the total on purpose (Decision 2).

## Adversarial

Read: `state/optdesk-drafts/designs/option-pages-r7-adversarial.md` (against 357c585b).

1. Gap against the goal: partly adopted. Cash beside each due date is now built, per currency, by the rule (entries 16, 1, 6, 7; Decisions 7 and 14). "Unidentified" and the 27 differing trades stay in Left out with the reason, named in the answer to the owner; the structures reason line now names the tie (entry 9).
2. Redefining "fresh" hides a missing chain: adopted. The rule keeps a chain two or more sessions old stale, the earnings comparisons are untouched, the collector schedule is named as unread, the stored rows keep their flag until the job re-runs, and `internal/compute/api.go` is held to its ceiling (Decision 5, entry 3).
3. Simpler way: adopted for the redirect split (entries 14 and 15), for the spot label without a catalog reader (Decision 6e, entry 11) and for the web edges 7<-6 and 12<-11, which are dropped. Not adopted: dropping the redirect itself (the owner asked that `/options` merge into candidates). Edge 15<-9 is kept because the `Config` fields it sets are declared in entry 9; the pages entry 11 no longer waits for entry 3.
4. Lock-in: adopted. The Contracts wording is `<id> v1 additive`; the consumers of the changed meaning of `companies` are named (Decision 6g, entry 2).
5. Claim likely false (catalog day): adopted. The catalog reader and its wiring are removed; `SpotDay` is the stored rows' day with a Chinese hint.
6. Likeliest slip: points 1 and 2 adopted (candidates wiring dropped, redirect test in a new file); 3 adopted (ceiling kept, `CashDay` fields in one line); 4 adopted (split count rule); 5 not adopted for the closing item "The result matches ..." because the `design` rule of `crew issue rules` requires it as the last item; the judged-by-reading items got greps or named tests where cheap (entries 7, 13); 6 adopted; 8 adopted (the empty-tier text is set in `tiers.go`, `tierlegend.go` and `tierrows.go` are read only).
