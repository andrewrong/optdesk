# candidates zh: the tier view of the candidates page reads in Chinese, two short lines per company card, with the tier and the three counts taken from the payload

Author: the optdesk designer (draft)
Owner approved: the mockup and the field map, 2026-11-19 09:15Z (word "a"); the design itself is pending
Date: 2026-11-19

Source: owner decisions of 2026-11-19 09:08Z (ask 5223: Chinese-first labels, two short lines per card, full-width pages) and 09:15Z (the mockup `candidates-zh-mockup.html` with its screens at 1280 and 390 px, and the field map `labels.md`, both in the designs folder of the drafts state). Owner rules carried: `web/src` is English-only and the Chinese comes from the Go payload (`internal/pages/option_candidates_page/zh_texts.go` and the string files beside it, checked by `TestZhTextsDoor`); the front end computes nothing, so the tier pill and the three counts come from the payload; jargon is glossed next to it as visible text, not only in a tooltip (owner 2026-11-14). Plan P7. The theme (14 px body, palette, fonts) is `docs/design/site-look.md` (pull request 3210); this design uses the theme as it is and edits no theme file.

Read on origin/main at 80c9a01e: `AGENTS.md`, `docs/design/modules.md`, `docs/design/candidates-page-v2.md`, `docs/design/buy-price-source.md`, `docs/design/candidates-valuation-line.md`, `docs/design/candidates-tiers.md`, `internal/pages/option_candidates_page/{tiers,tierstrings,tierlegend,tierrows,valueline,strings,shortlisttexts,zh_texts}.go`, `contracts/schemas/page.option_candidates_page.v1.json`, `web/src/generated/option_candidates_page.v1.ts`, `web/src/pages/candidates/{CompanyStrip,TierView}.tsx`, `web/test/pages/optioncandidates/*`, `internal/contracts/{option_candidates_page_v1_test,fixtures_test}.go`, `cmd/ot/zhdoor_test.go`.

## What you see

`/options/candidates`, the tier view, in the look of the approved mockup (companies, prices and counts below are invented placeholders).

The page head: title "期权候选", the group "先卖出看跌", the legend "✓ 适合 n", "! 待查 n", "– 未判定 n", "账户现金", each with its one Chinese sentence.

The company strip, one card per company, two short lines:

    line 1   <ticker>  [适合|待查|未判定]  买入价 <price> [手填|估值]  现价 <price> 存档 <mm-dd>  下次财报 <date|未知>
    line 2   估值买入线 <price> <note>                                  合约 <n> 适合 · <n> 待查 · <n> 折叠   (折叠 = 未判定)

- The pill at the head of line 1 is the verdict of the company: 适合 when it has a fitting contract, else 待查 when it has one to check, else 未判定.
- A company with no buy price shows "没有买入价" in place of the price and the pill 未判定; the old "Not judged" badge is not drawn on the card.
- The valuation note says one of three things: the valuation is used by the latest run ("复核于 <day>，运行就用它"); it is shown but the latest run did not use it ("复核于 <day>，最近一次运行没用到": the case of a valuation with no buy price on the stored rows); or there is none ("没有找到估值档案" with the action "导入估值，或在“公司 > 关注列表”手填买入价").
- The old third line "Buy price used by the gate" is gone. One sentence under the cards says it once: the gate uses the buy price shown above, the valuation zone when there is one, else the hand price; the spot is the price the job took for that day, not the close.
- Beside "折叠" the card says "折叠 = 未判定" as visible text.

The tier table under the cards: column heads "详情 / 档位 / 合约 / 安全边际 / 距现价 / 权利金 / 年化 / 徽标", each with a one-line visible subtitle; the tiers "适合 n", "待查 n", "<n> 行：没有买入价"; a contract reads "看跌 <strike>, <day> · <n> 天"; the sub texts read "保本点 <p> / 买入价 <p>", "<x> 个预期波动*", "<y>% / 年"; badges read "权利金薄" and the like. The pages stay full width (the owner's rule of ADR 0034; no column cap), and at phone width the two lines wrap.

## Goal

The tier view of the candidates page has no English word of ours left in it, every Chinese text comes from the Go payload, and the card is two lines. A card tells the verdict, the buy price with its source, the valuation line with the state of its use, and the three counts, without a tooltip and without the owner computing which counts mean what.

## Today

- English words of ours live in the web: `Spot`, `rows of`, `Buy price`, `Next earnings` (`web/src/pages/candidates/CompanyStrip.tsx:53-59`).
- `counts` is one joined string, `"<n> fit, <n> check, <n> collapsed"`, built in Go (`internal/pages/option_candidates_page/tiers.go:232`, parts `tierstrings.go:34-36`); the web prints it (`CompanyStrip.tsx:60`). The three numbers cannot be put apart from it.
- The company has no verdict field; `Company` is key, ticker, spot, spot_day, buy_price, buy_price_note, next_earnings, counts, valuation, buy_source (`tiers.go:15-26`, generated `web/src/generated/option_candidates_page.v1.ts:397-408`).
- The valuation block is label "Valuation buy line B_hi", text "<price> (review <day>)", a long Chinese hint, a reason, and three gate fields `gate_label`, `gate_text`, `gate_hint` (`valueline.go:11-19`, label constants `valueline.go:22-25`, text `valueline.go:100`). `gate_text` is the company `buy_price` or "not judged" (`valueline.go:73-75`), so it is the same value as the buy price on line 1.
- The card has no state for "the valuation is shown but the latest run did not use it". The page knows it from the stored marks: `buySourceCode` (`buysource.go`) says which source a stored row carries, and the valuation figure comes from the resolver (`valueline.go:87-90`); the two are not compared.
- The tier words, titles, legend titles, column labels and sub texts are English in `tierstrings.go` (words `:6-8`, count and contract suffixes `:34-43`, columns `:75-81`, legend `:24-27`, titles `:18-19`), the page title and the group title in `strings.go:5,28`, the source labels in `shortlisttexts.go:69-71`.
- The explanations and glosses are already Chinese (`zh_texts.go`, `tierlegend.go:55-60`, `buy_source.gloss` at `zh_texts.go:142-144`); `spot_hint` and `buy_hint` of the tier view are Chinese (`tiers.go:62-63`, `zh_texts.go:105-106`).
- The Chinese door: `zh_texts.go` is const-only (`cmd/ot/zhdoor_test.go`, `TestZhTextsDoor`).
- The web fixtures of this page are TypeScript files in `web/test/pages/optioncandidates/fixtures/`; there is no `web/fixtures/option-candidates-page/` directory, so `internal/contracts/fixtures_test.go` has no row for this page and needs none.
- The goldens that hold the English texts are `internal/pages/option_candidates_page/testdata/{evidence_ok,evidence_gaps,jp_rows,shortlist_ok,shortlist_gaps,tiers_ok,tiers_gaps,valueline}.golden.json`.

## Decisions

1. The three amounts of work are three modules in a fixed order: the schema first (contracts), then the Go payload (pages), then the page (web); the old fields go in a last pair of entries so the site never reads a field that is gone.
2. Interface change, additive first (entry 1) and removal last (entry 5), on `page.option_candidates_page.v1`; no `v2`: the page JSON is built on request, nothing stored holds the old shape, and the generated type and the page change in the same sequence.
3. New payload fields, all text (no JSON number, as the rest of the tier view):
   - `Company.tier`: `{ id: "fits" | "check" | "grey", word: Text }`; the word is "适合", "待查" or "未判定". The page builder derives it from the three counts of the resolver answer (fits above zero, else check above zero, else grey); the web prints `word` and sets `data-tier=id`.
   - `Company.count_parts`: `{ fits, check, grey }`, three texts of whole numbers.
   - `Company.spot_note`: the text "存档 <mm-dd>" (the day of the stored rows without its year), or null when there is no spot; the page cuts the stored day text, no date arithmetic.
   - `TierView.strip_labels`: one object of the visible card words, once for the page: `spot` ("现价"), `buy_price` ("买入价"), `next_earnings` ("下次财报"), `contracts` ("合约"), `fits` ("适合"), `check` ("待查"), `grey` ("折叠"), `grey_hint` ("折叠 = 未判定").
   - `ValueLine.run_state`: `"used" | "not_used" | "none"`, the payload state of the valuation line. `used`: a valuation figure is shown and the stored rows carry the valuation as the buy price source (`buy_source.code` of the company is the valuation code); `not_used`: a figure is shown and they do not (the "最近一次运行没用到" case); `none`: no figure, `reason` is non-null. The page sets it from values it already has (the resolver figure and `buySourceCode`), compares nothing else, and the web only prints.
   - `ValueLine.note`: the one visible line after the figure, Chinese, from `zh_texts.go`: "复核于 <day>，运行就用它" for `used`, "复核于 <day>，最近一次运行没用到" for `not_used`, and for `reason` `no_profile` the action sentence "导入估值，或在“公司 > 关注列表”手填买入价"; null for the other reasons (their sentence is `text`, as today).
4. Changed values (no schema shape change): `ValueLine.label` becomes "估值买入线" (no symbol); `ValueLine.text` is the figure alone, the review day moves to `note`; `ValueLine.hint` keeps the long Chinese gloss as a tooltip; `Company.buy_price_note` is "没有买入价"; `Company.next_earnings` is "未知" when unknown; the spot text is "无现价" when none; `Company.buy_source` is null when the source is none (the pill and "没有买入价" say it; the short list keeps its own badge for none through `buySourceBadge`, which is not changed).
5. Removed (entry 5): `Company.counts` (the joined string), `ValueLine.gate_label`, `ValueLine.gate_text`, `ValueLine.gate_hint`. The gate sentence goes into the existing `TierView.spot_hint` text ("现价是任务为这一天运行时取到的价格，不是当天收盘价。“闸门”用的买入价就是上面的买入价：有估值买入区间就用它，没有再用手填价格。"); `buy_hint` stays the action text of the page (as today). `Company.spot_day` stays (the stored day, kept for tests and for `spot_note`).
6. Chinese labels from the owner-approved map, carried in the pages entry (the English text, then the Chinese):
   - page: "Option candidates" → "期权候选"; group "Short put" → "先卖出看跌" (the groups "Covered call" and "Held call guard" are not in the approved map and stay as they are: Left out); the two page notes (`strings.go:7,110`) are given a Chinese sentence in `zh_texts.go` by the pages entry in the words of the map's "needs Chinese" rows (the yen group is a separate group in yen when a yen reader is set; the cash rate used for the bid against the forecast price is DGS3MO for every currency and is a trial).
   - legend titles "✓ 适合 <n>", "! 待查 <n>", "– 未判定 <n>", "账户现金" (the count has no brackets); tier words "适合", "待查", "未判定"; tier titles "适合 <n>", "待查 <n>", "<n> 行：没有买入价" (the grey tail of each grey tier keeps its reason in Chinese).
   - columns: "详情", "档位", "合约", "安全边际", "距现价", "权利金 / 年化", "徽标"; the column hint becomes a one-line visible subtitle in the head (the payload field `hint` is kept; the web prints it as text under the head, not as a tooltip).
   - row texts: contract "看跌 <strike>, <day> · <n> 天"; margin sub "保本点 <p> / 买入价 <p>"; below-spot sub "<x> 个预期波动*"; premium sub "<y>% / 年"; badge labels in Chinese ("权利金薄" and the other stored labels of `tierstrings.go`).
   - source labels: "手填" (hand), "估值" (valuation); their glosses stay.
7. The web prints payload texts and decides nothing: the pill word and `data-tier` come from `tier`, the counts from `count_parts` with the words of `strip_labels`, the note from `ValueLine.note`, the line of the gate is not drawn. A card with `valuation` null (no valuation reader) draws line 1 and the counts only. A company with `tier.id` grey and `buy_price` null draws `buy_price_note` as the price. `web/src` keeps no Chinese literal and no English label of ours (the rule: web English-only), checked by the web entry's test.
8. Two short lines: line 1 is pill, ticker, buy price with its source, spot with its note, next earnings; line 2 is the valuation line with its note and the counts with the visible gloss. The wrap at phone width is by the existing flex classes; no new design token, no theme file is edited, so the web entry needs no order after `site-look` (entry 1 of that design is the font and size tokens; if the web entry finds it needs a token that does not exist, it stops and says so).
9. The page keeps its other groups as they are (the covered call and held call groups, the short list, the 23-column main table, the evidence cells, the reasons): their English is Left out and gets a mockup of its own when the owner asks.
10. Failure behaviour: `run_state` is derived with the valuation figure; with a resolver read error every strip has reason `read_failed`, `run_state` "none" and `note` null, as today (`valueline.go:55-57`); no text is a reason to fail the page.

## Modules

`docs/design/modules.md` is not changed: the paths below match the rows contracts (`contracts/**`, `web/src/generated/**`, `internal/contracts/**`, `docs/contracts/page-schemas.md`), pages (`internal/pages/**`) and web (`web/**`), and the first row whose glob matches owns each path (`web/src/generated/**` is contracts, so the generated type is in the contracts entry, not the web entry).

- contracts: `contracts/schemas/page.option_candidates_page.v1.json`, `web/src/generated/option_candidates_page.v1.ts`, `internal/contracts/option_candidates_page_v1_test.go`, `docs/contracts/page-schemas.md` (entries 1, 5).
- pages: `internal/pages/option_candidates_page/**` (entries 2, 4).
- web: `web/src/pages/candidates/{CompanyStrip,TierView}.tsx`, `web/test/pages/optioncandidates/**` (entry 3).

Imports against the `depends` column: pages already imports contracts, store, compute; web imports the generated types only. No store, dataset or migration change; nothing holds the auto-deploy.

## Interface

`contracts/schemas/page.option_candidates_page.v1.json`, v1 changed in two steps (`additionalProperties: false` kept everywhere, texts only, no JSON number).

Entry 1 adds, all optional, and makes the old fields optional (not removed):

    Company       += tier?:        { id: "fits"|"check"|"grey", word: Text1 }
                     count_parts?: { fits: Text1, check: Text1, grey: Text1 }
                     spot_note?:   Text1 | null
                     counts?       (was required)
    ValueLine     += run_state?:   "used"|"not_used"|"none"
                     note?:        Text1 | null
                     gate_label?, gate_text?, gate_hint?   (were required)
    TierView      += strip_labels?: { spot, buy_price, next_earnings, contracts, fits, check, grey, grey_hint: Text1 }

Entry 5 removes `Company.counts`, `ValueLine.gate_label`, `ValueLine.gate_text`, `ValueLine.gate_hint` and makes `tier`, `count_parts`, `spot_note`, `run_state`, `note` and `strip_labels` required (a field that may be empty stays `null`, never absent).

Generated type `web/src/generated/option_candidates_page.v1.ts` (never edited by hand): `Company` gains `tier`, `count_parts`, `spot_note`; `ValueLine` gains `run_state`, `note`; `TierView` gains `strip_labels`; after entry 5 `counts` and the `gate_*` fields are gone from the type. The contract test `internal/contracts/option_candidates_page_v1_test.go` builds its instance by hand (`tiersRow`, `tiersView`, the valuation map at line 255) and is the test of the contract from both sides. `docs/contracts/page-schemas.md` (line 133, the page entry) gets one sentence on the new fields.

## Issues

Build order: entry 1 first; entry 2 and entry 3 need only landed entry 1 and are ordered 1, 2, 3 so the site never shows a card without the new fields; entry 4 needs entry 3 landed (the web no longer reads the old fields); entry 5 needs entry 4 landed (no page emits them). Entries are not merged.

### 1 candidates-zh-contract (interface)

Module: contracts. Size: S.
Order: first.
Files: `contracts/schemas/page.option_candidates_page.v1.json`, `web/src/generated/option_candidates_page.v1.ts`, `internal/contracts/option_candidates_page_v1_test.go`, `docs/contracts/page-schemas.md`
Contracts: changed (`contracts/schemas/page.option_candidates_page.v1.json`)
Design: docs/design/candidates-zh.md#Interface
Plan: P7
Acceptance:
1. The schema has the optional properties `Company.tier`, `Company.count_parts`, `Company.spot_note`, `ValueLine.run_state`, `ValueLine.note` and `TierView.strip_labels` of the Interface section, and `Company.counts`, `ValueLine.gate_label`, `ValueLine.gate_text`, `ValueLine.gate_hint` are no longer in a `required` list; no property is removed and `additionalProperties` is false on each object.
2. `run_state` is an enum of exactly `used`, `not_used`, `none`; `tier.id` is an enum of exactly `fits`, `check`, `grey`; no new property has a JSON number type.
3. `web/src/generated/option_candidates_page.v1.ts` is regenerated from the schema in the same change (the generator, not a hand edit) and `git diff --name-only BASE` lists only the four files.
4. `internal/contracts/option_candidates_page_v1_test.go` has `TestTiersNewFieldsValidate` (new: an instance with all new fields passes), `TestTiersOldShapeStillValidates` (new: an instance with only the old fields passes), `TestRunStateEnum` (new: `"maybe"` fails) and `TestTierIDEnum` (new: `"fit"` fails); the existing tests are unchanged.
5. `docs/contracts/page-schemas.md` names the new fields in one sentence under the page entry.
6. The result matches docs/design/candidates-zh.md#Interface.
Checked by CI: `go test ./internal/contracts/...`; the web type-check on the regenerated type.
Adversarial: to be filled
Left out: removing the old fields (entry 5).

### 2 candidates-zh-pages

Module: pages. Size: L.
Order: after 1.
Files: `internal/pages/option_candidates_page/tiers.go`, `internal/pages/option_candidates_page/tierstrings.go`, `internal/pages/option_candidates_page/tierlegend.go`, `internal/pages/option_candidates_page/tierrows.go`, `internal/pages/option_candidates_page/valueline.go`, `internal/pages/option_candidates_page/strings.go`, `internal/pages/option_candidates_page/shortlisttexts.go`, `internal/pages/option_candidates_page/zh_texts.go`, `internal/pages/option_candidates_page/tiers_test.go`, `internal/pages/option_candidates_page/tierrows_test.go`, `internal/pages/option_candidates_page/tierlegend_test.go`, `internal/pages/option_candidates_page/valueline_test.go`, `internal/pages/option_candidates_page/buysource_test.go`, `internal/pages/option_candidates_page/page_test.go`, `internal/pages/option_candidates_page/builder_test.go`, `internal/pages/option_candidates_page/cells_test.go`, `internal/pages/option_candidates_page/evcells_test.go`, `internal/pages/option_candidates_page/evidence_test.go`, `internal/pages/option_candidates_page/jp_test.go`, `internal/pages/option_candidates_page/reasons_test.go`, `internal/pages/option_candidates_page/shortlist_test.go`, `internal/pages/option_candidates_page/shortlistfacts_test.go`, `internal/pages/option_candidates_page/tiersfakes_test.go`, `internal/pages/option_candidates_page/zhstrip_test.go`, `internal/pages/option_candidates_page/testdata/evidence_ok.golden.json`, `internal/pages/option_candidates_page/testdata/evidence_gaps.golden.json`, `internal/pages/option_candidates_page/testdata/jp_rows.golden.json`, `internal/pages/option_candidates_page/testdata/shortlist_ok.golden.json`, `internal/pages/option_candidates_page/testdata/shortlist_gaps.golden.json`, `internal/pages/option_candidates_page/testdata/tiers_ok.golden.json`, `internal/pages/option_candidates_page/testdata/tiers_gaps.golden.json`, `internal/pages/option_candidates_page/testdata/valueline.golden.json`
Contracts: unchanged
Design: docs/design/candidates-zh.md#Decisions
Plan: P7
Acceptance:
1. Each company of `short_put_tiers.companies` has `tier` with `id` and `word` from the three counts of the answer: fits above zero gives `fits` and "适合", else check above zero gives `check` and "待查", else `grey` and "未判定" (new test `TestCompanyTierFromCounts`, table of the three cases and an all-zero company).
2. Each company has `count_parts` of three whole-number texts equal to the answer's fit, check and grey counts, and `counts` is still sent (new test `TestCompanyCountParts`).
3. `spot_note` is "存档 " plus the stored day without its year, null when the spot text is "无现价" (new test `TestSpotNote`).
4. `short_put_tiers.strip_labels` has the eight words of Decision 3 in Chinese from `zh_texts.go` (new test `TestStripLabelsChinese`: no ASCII letter in any value).
5. `valuation.run_state` is `used` when a figure is shown and the company's `buy_source.code` is the valuation code, `not_used` when a figure is shown and it is not, `none` when `reason` is non-null; `valuation.note` carries the sentence of Decision 3 for each state (new tests `TestValueRunStateUsed`, `TestValueRunStateNotUsed`, `TestValueRunStateNone`: the not-used case is a figure with a company whose stored rows carry no buy price, and `note` contains "最近一次运行没用到").
6. `valuation.label` is "估值买入线", `valuation.text` holds the figure alone with no review day, and the old `gate_*` fields are still sent unchanged (existing `TestValueLineFromCalculator` updated).
7. `buy_source` of a company is null when the source is none and the short list's badge for none is unchanged (new test `TestStripNoBadgeForNone`, existing `buysource_test.go` cases for the short list still pass).
8. The page title is "期权候选", the short put group title "先卖出看跌", the legend titles, tier words, tier titles, column labels, row sub texts and the hand and valuation source labels are the Chinese of Decision 6, and no label of the tier view is English (new test `TestTierViewNoEnglishLabels`: every `word`, `title`, column `label` and legend `title` of the golden answer has no ASCII letter except in a price, a day or a symbol).
9. Every new Chinese string is a const in `zh_texts.go` or in `tierstrings.go` (const-only), and `TestZhTextsDoor` of `cmd/ot` still passes.
10. The eight goldens are regenerated and `git diff` of each shows only text changes of Decision 6 and the added fields.
11. No file is over 600 lines that was not before and no file over 800 lines grows.
12. The result matches docs/design/candidates-zh.md#Decisions.
Checked by CI: `go test ./internal/pages/... ./cmd/ot/...`.
Adversarial: to be filled
Left out: the groups Covered call and Held call guard, the 23-column main table, the evidence cells, the reasons, the short list texts other than the source labels, tap hints.

### 3 candidates-zh-web

Module: web. Size: M.
Order: after 2.
Files: `web/src/pages/candidates/CompanyStrip.tsx`, `web/src/pages/candidates/TierView.tsx`, `web/test/pages/optioncandidates/CompanyStrip.test.tsx`, `web/test/pages/optioncandidates/TierView.test.tsx`, `web/test/pages/optioncandidates/OptionCandidatesPage.test.tsx`, `web/test/pages/optioncandidates/fixtures/tiers.ts`, `web/test/pages/optioncandidates/fixtures/valueline.ts`, `web/test/pages/optioncandidates/fixtures/ok.ts`, `web/test/pages/optioncandidates/fixtures/jp.ts`
Contracts: unchanged
Design: docs/design/candidates-zh.md#What-you-see
Plan: P7
Acceptance:
1. `CompanyStrip.tsx` draws two lines: line 1 is the pill (`tier.word`, `data-tier` from `tier.id`), the ticker, the buy price or `buy_price_note` with the source badge, the spot with `spot_note`, the next earnings; line 2 is the valuation label, figure or reason text and `note`, then the three counts with the words of `strip_labels` and `grey_hint` as visible text (new test `TestStripTwoLines`).
2. `CompanyStrip.tsx` and `TierView.tsx` hold no label of our own: no `Spot`, `rows of`, `Buy price`, `Next earnings` and no Chinese literal; every word is a payload text (new test `TestStripWordsFromPayload`: a fixture with other words shows exactly those words).
3. The gate line is not drawn: `[data-gate]` is absent for every fixture (existing `TestStripShowsValueLine` and `TestStripNoValuationUnchanged` updated to the new text).
4. A company with `run_state` `not_used` shows its note, "最近一次运行没用到", beside the figure; `used` and `none` show theirs (new test `TestStripRunStateNotes`, three fixtures).
5. The pill of a company is the payload word, never derived from the counts in the web (new test `TestPillIsPayloadWord`: counts say fits and `tier` says grey, the pill shows grey).
6. The column subtitles of the tier table are visible text, not only a tooltip (updated test in `TierView.test.tsx`), and the page still has no `max-w-` class under `web/src/pages/candidates/` (new test `TestNoWidthCap`).
7. The fixtures `tiers.ts`, `valueline.ts`, `ok.ts` and `jp.ts` carry the new fields and no `counts` string or `gate_*` field in a company or a valuation (the type-check of entry 5 then passes).
8. `tsc`, `eslint`, `vitest` and the build pass; no theme file and no `web/src/index.css` is edited, and no new token is used, so the entry has no order after `docs/design/site-look.md`.
9. The result matches docs/design/candidates-zh.md#What-you-see.
Checked by CI: `pnpm vitest`, `tsc`, `eslint`, the web build. A UI pass counts only with a screenshot of the live page beside the mockup at 1280 and 390 px, taken by the window after the deploy (owner rule 2026-11-14).
Adversarial: to be filled
Left out: the 23-column main table, the short list, tap hints (a tooltip on touch), the theme.

### 4 candidates-zh-pages-drop

Module: pages. Size: S.
Order: after 3.
Files: `internal/pages/option_candidates_page/tiers.go`, `internal/pages/option_candidates_page/tierstrings.go`, `internal/pages/option_candidates_page/valueline.go`, `internal/pages/option_candidates_page/zh_texts.go`, `internal/pages/option_candidates_page/tiers_test.go`, `internal/pages/option_candidates_page/valueline_test.go`, `internal/pages/option_candidates_page/tiersfakes_test.go`, `internal/pages/option_candidates_page/testdata/tiers_ok.golden.json`, `internal/pages/option_candidates_page/testdata/tiers_gaps.golden.json`, `internal/pages/option_candidates_page/testdata/valueline.golden.json`
Contracts: unchanged
Design: docs/design/candidates-zh.md#Decisions
Plan: P7
Acceptance:
1. No company JSON has `counts`, and no valuation JSON has `gate_label`, `gate_text` or `gate_hint` (new test `TestOldStripFieldsNotSent`: the keys are absent from the tiers golden answer).
2. The constants only the dropped fields used (`countFit`, `countCheck`, `countCollapsed`, `gateLabel`, `gateHintZh` and the English `notJudged` of the gate) are removed; the gate sentence is in `spot_hint` (new test `TestSpotHintCarriesGateSentence`: the text contains "闸门").
3. The three goldens are regenerated and show only the removed keys and the new `spot_hint` text.
4. The result matches docs/design/candidates-zh.md#Decisions.
Checked by CI: `go test ./internal/pages/... ./cmd/ot/...`.
Adversarial: to be filled
Left out: any other field.

### 5 candidates-zh-contract-drop (interface)

Module: contracts. Size: S.
Order: after 4.
Files: `contracts/schemas/page.option_candidates_page.v1.json`, `web/src/generated/option_candidates_page.v1.ts`, `internal/contracts/option_candidates_page_v1_test.go`, `docs/contracts/page-schemas.md`
Contracts: changed (`contracts/schemas/page.option_candidates_page.v1.json`)
Design: docs/design/candidates-zh.md#Interface
Plan: P7
Acceptance:
1. `Company.counts`, `ValueLine.gate_label`, `ValueLine.gate_text` and `ValueLine.gate_hint` are not properties of the schema, and `tier`, `count_parts`, `spot_note`, `run_state`, `note` and `strip_labels` are in the `required` lists of their objects.
2. The generated type is regenerated in the same change and `Company` has no `counts`, `ValueLine` has no `gate_*`.
3. `internal/contracts/option_candidates_page_v1_test.go` builds its instance with the new fields only; `TestTiersOldShapeStillValidates` is replaced by `TestTiersOldShapeFails` (new: an instance with `counts` fails) and `TestTiersMissingRunStateFails` (new).
4. `docs/contracts/page-schemas.md` no longer names the removed fields.
5. The result matches docs/design/candidates-zh.md#Interface.
Checked by CI: `go test ./internal/contracts/... ./internal/pages/...`; the web type-check.
Adversarial: to be filled
Left out: nothing else is removed.

## Checked by CI

`go test` for contracts, pages and `cmd/ot`; `tsc`, `eslint`, `vitest` and the web build for the web entry. The screenshot against the mockup is the window's check after the deploy, not a CI check.

## Adversarial

To be filled.

## Left out

- The 23-column main table, the evidence cells and the reasons of the page (their English and their layout); the covered call and held call groups; the short list text other than the two source labels.
- Tap hints on touch screens (the visible glosses of this design replace the tooltip for the card; a tap hint for the rest is its own design).
- The theme, fonts and sizes (`docs/design/site-look.md`); the page width (full width stays).
- A "used by the run" state for sources other than the valuation; the tier verdict for the other groups.
- Translating the symbol names of option terms (put, delta, IV, DGS3MO stay as the terms they are, glossed beside).
