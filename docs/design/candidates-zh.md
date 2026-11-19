# candidates zh: the tier view of the candidates page reads in Chinese, two short lines per company card, with the tier and the three counts taken from the payload

Author: the optdesk designer (draft)
Owner approved: the mockup and the field map, 2026-11-19 09:15Z (word "a"); the design itself is pending
Date: 2026-11-19

Source: owner decisions of 2026-11-19 09:08Z (ask 5223: Chinese-first labels, two short lines per card, full-width pages) and 09:15Z (the mockup `candidates-zh-mockup.html` with its screens at 1280 and 390 px, and the field map `labels.md`, both in the designs folder of the drafts state). Owner rules carried: `web/src` is English-only and the Chinese comes from the Go payload (`internal/pages/option_candidates_page/zh_texts.go` and the string files beside it, checked by `TestZhTextsDoor`); the front end computes nothing, so the tier pill and the three counts come from the payload; jargon is glossed next to it as visible text, not only in a tooltip (owner 2026-11-14). Plan P7. The theme (14 px body, palette, fonts) is `docs/design/site-look.md` (pull request 3210); this design uses the theme as it is and edits no theme file.

Read on origin/main at 80c9a01e: `AGENTS.md`, `docs/design/modules.md`, `docs/design/candidates-page-v2.md`, `docs/design/buy-price-source.md`, `docs/design/candidates-valuation-line.md`, `docs/design/candidates-tiers.md`, `internal/pages/option_candidates_page/{tiers,tierstrings,tierlegend,tierrows,valueline,strings,shortlisttexts,zh_texts}.go`, `contracts/schemas/page.option_candidates_page.v1.json`, `web/src/generated/option_candidates_page.v1.ts`, `web/src/pages/candidates/{CompanyStrip,TierView}.tsx`, `web/test/pages/optioncandidates/*`, `internal/contracts/{option_candidates_page_v1_test,fixtures_test}.go`, `cmd/ot/zhdoor_test.go`.

## What you see

`/options/candidates`, the tier view, in the look of the approved mockup (companies, prices and counts below are invented placeholders).

The page head: title "期权候选", the labelled badges "数据日 <day>", "运行状态 <部分完成|完成>", "规则版本 <id>", the groups "先卖出看跌", "备兑看涨", "持仓看涨保护", the legend "✓ 适合 n", "! 待查 n", "– 未判定 n", "账户现金", each with its one Chinese sentence.

The company strip, one card per company, two short lines:

    line 1   <ticker>  [适合|待查|未判定]  买入价 <price> [手填|估值]  现价 <price> 存档 <mm-dd>  下次财报 <date|未知>
    line 2   估值买入线 <price> <note>                                  合约 <n> 适合 · <n> 待查 · <n> 折叠   (折叠 = 未判定)

- The cards come before the tier table, right under the legend, as in the mockup. The pill at the head of line 1 is the verdict of the company, with its glyph (✓ 适合, ! 待查, – 未判定): 适合 when it has a fitting contract, else 待查 when it has one to check, else 未判定.
- A company with no buy price shows "没有买入价" in place of the price and the pill 未判定; the card draws no separate "Not judged" source badge (the short list keeps its own badge, unchanged).
- The valuation note says one of three things: the valuation is used by the latest run ("复核于 <day>，运行就用它"); it is shown but the latest run did not use it ("复核于 <day>，最近一次运行没用到": the case of a valuation with no buy price on the stored rows); or there is none ("没有找到估值档案" with the action "导入估值，或在“公司 > 关注列表”手填买入价").
- The old third line "Buy price used by the gate" is gone. One visible sentence under the cards (not a tooltip) says it once: the gate uses the buy price shown above, the valuation zone when there is one, else the hand price; the spot is the price the job took for that day, not the close.
- Beside "折叠" the card says "折叠 = 未判定" as visible text.

The tier table under the cards: column heads "详情 / 档位 / 合约 / 安全边际 / 距现价 / 权利金 / 年化 / 徽标", each with a one-line visible subtitle; the tiers "适合 n", "待查 n", "<n> 行：没有买入价"; a contract reads "看跌 <strike>, <day> · <n> 天"; the sub texts read "保本点 <p> / 买入价 <p>", "<x> 个预期波动*", "<y>% / 年"; badges read "权利金薄" and the like. The pages stay full width (the owner's rule of ADR 0034; no column cap), and at phone width the two lines wrap.

## Goal

The tier view of the candidates page has no English word of ours left in it, every Chinese text comes from the Go payload, and the card is two lines. A card tells the verdict, the buy price with its source, the valuation line with the state of its use, and the three counts, without a tooltip and without the owner computing which counts mean what.

## Today

- English words of ours live in the web: `Spot`, `rows of`, `Buy price`, `Next earnings` (`web/src/pages/candidates/CompanyStrip.tsx:53-59`).
- `counts` is one joined string, `"<n> fit, <n> check, <n> collapsed"`, built in Go (`internal/pages/option_candidates_page/tiers.go:232`, parts `tierstrings.go:34-36`); the web prints it (`CompanyStrip.tsx:60`). The three numbers cannot be put apart from it.
- The page head badges print the bare values with no label, and "Rule <id>" and "Stored tables" are English (`web/src/pages/candidates/OptionCandidatesPage.tsx:62,86`); the column subtitle is a tooltip (`TierRows.tsx:80,108`); the card is drawn after the tier table (`TierView.tsx:44-52`).
- The company has no verdict field; `Company` is key, ticker, spot, spot_day, buy_price, buy_price_note, next_earnings, counts, valuation, buy_source (`tiers.go:15-26`, generated `web/src/generated/option_candidates_page.v1.ts:397-408`).
- The valuation block is label "Valuation buy line B_hi", text "<price> (review <day>)", a long Chinese hint, a reason, and three gate fields `gate_label`, `gate_text`, `gate_hint` (`valueline.go:11-19`, label constants `valueline.go:22-25`, text `valueline.go:100`). `gate_text` is the company `buy_price` or "not judged" (`valueline.go:73-75`), so it is the same value as the buy price on line 1.
- The card has no state for "the valuation is shown but the latest run did not use it". The page knows it from the stored marks: `buySourceCode` (`buysource.go`) says which source a stored row carries, and the valuation figure comes from the resolver (`valueline.go:87-90`); the two are not compared.
- The tier words, titles, legend titles, column labels and sub texts are English in `tierstrings.go` (words `:6-8`, count and contract suffixes `:34-43`, columns `:75-81`, legend `:24-27`, titles `:18-19`), the page title and the group title in `strings.go:5,28`, the source labels in `shortlisttexts.go:69-71`.
- The explanations and glosses are already Chinese (`zh_texts.go`, `tierlegend.go:55-60`, `buy_source.gloss` at `zh_texts.go:142-144`); `spot_hint` and `buy_hint` of the tier view are Chinese (`tiers.go:62-63`, `zh_texts.go:105-106`).
- The Chinese door: `zh_texts.go` is const-only (`cmd/ot/zhdoor_test.go`, `TestZhTextsDoor`).
- The web fixtures of this page are TypeScript files in `web/test/pages/optioncandidates/fixtures/`; there is no `web/fixtures/option-candidates-page/` directory, so `internal/contracts/fixtures_test.go` has no row for this page and needs none.
- The goldens that hold the English texts are `internal/pages/option_candidates_page/testdata/{evidence_ok,evidence_gaps,jp_rows,shortlist_ok,shortlist_gaps,tiers_ok,tiers_gaps,valueline}.golden.json`.

## Decisions

1. Add, never drop. Three entries in a fixed order: the schema (contracts), the Go payload (pages), the page (web). The schema only gains optional properties; every old property stays as it is and stays required where it is required today, so the generated type changes only by additions and the web type-check stays green after entry 1. The pages entry fills the new fields and keeps sending every old field with its old value, so the short list and every other reader of the old fields keep working. Dropping the old fields is a later clean-up (Left out).
2. Interface change, additive only, on `page.option_candidates_page.v1`; no `v2` (the page JSON is built on request, nothing stored holds the old shape, and the page registry and the web import the v1 name).
3. New optional payload fields, all text (no JSON number, as the rest of the tier view). A schema text is `Text` (`$defs.Text`, 1 to 200 characters; `Text1` is only the generated TypeScript name); a count part `"0"` is valid.
   - `Company.tier`: `{ id: "fits" | "check" | "grey", word: Text, glyph: Text }`; word "适合" / "待查" / "未判定", glyph "✓" / "!" / "–". The page builder derives it from the three counts of the resolver answer (fits above zero, else check above zero, else grey).
   - `Company.count_parts`: `{ fits, check, grey }`, three texts of whole numbers.
   - `Company.spot_text`: the spot price text, or null when there is no spot (the card draws no spot then). `Company.spot_note`: "存档 <mm-dd>" (the stored day without its year, cut as text, no date arithmetic), null when there is no spot.
   - `Company.price_note`: "没有买入价" when the company has no buy price, else null. `Company.earnings_text`: the next earnings day, or "未知".
   - `Company.source_badge`: a badge `{ code, label, gloss, tone }` with label "手填" (hand) or "估值" (valuation) and the existing glosses; null when the source is none (the pill and `price_note` say it). The old `buy_source` is sent unchanged.
   - `ValueLine.line_label` "估值买入线"; `ValueLine.line_text`: the figure alone (no review day), or, when `reason` is non-null, the sentence that `text` holds today ("无估值档案" for `no_profile`); `ValueLine.run_state` `"used" | "not_used" | "none"`; `ValueLine.note`: one visible line, Chinese: "复核于 <day>，运行就用它" for `used`, "复核于 <day>，最近一次运行没用到" for `not_used`, the action "导入估值，或在“公司 > 关注列表”手填买入价" for reason `no_profile`, null for the other reasons. `used` means a figure is shown and the company's source code (`buySourceCode` of the stored rows, kept as a value before any badge is made) is the valuation code; `not_used` means a figure is shown and it is not; `none` means `reason` is non-null.
   - `TierView.strip_labels`: `{ spot "现价", buy_price "买入价", next_earnings "下次财报", contracts "合约", fits "适合", check "待查", grey "折叠", grey_hint "折叠 = 未判定" }`.
   - `TierView.card_note`: the one sentence under the cards: "现价是任务为这一天运行时取到的价格，不是当天收盘价。“闸门”用的买入价就是上面的买入价：有估值买入区间就用它，没有再用手填价格。" (the old `spot_hint` and `buy_hint` are sent unchanged).
   - `Page.head_labels`: `{ data_day "数据日", run_status "运行状态", run_status_text, rule_version "规则版本" }`; `run_status_text` is "完成" for `ok` and "部分完成" for `partial`. The existing `TierView.stored_title` is filled with "已存表格" by the pages entry, so the web fallback `Stored tables` is never used.
4. Old fields, all sent unchanged with their old values: `Company.counts`, `buy_price_note`, `next_earnings`, `spot`, `spot_day`, `buy_source`; `ValueLine.label`, `text`, `hint`, `gate_label`, `gate_text`, `gate_hint`; `TierView.spot_hint`, `buy_hint`. In particular `tierGreyWord`, `notJudged` and the `buySourceTexts` labels are not changed: the card uses new strings, the short list keeps its own badge ("Not judged") and its English source labels.
5. Texts of the tier table, the legend, the page and the groups change in place (same fields, same shape, only the words): they are plain strings read by the web and by tests only. New const strings carry the Chinese; the old English consts that the short list or the old fields still use stay.
   - page title "期权候选"; groups "先卖出看跌", "备兑看涨" (Covered call), "持仓看涨保护" (Held call guard); the two page notes and the estimate note get a Chinese sentence in `zh_texts.go`, in the words of the map's "needs Chinese" rows (a yen group is separate and in yen when a yen reader is set; the cash rate for the bid against the forecast price is DGS3MO for every currency and is a trial; the bid is the closing bid, sold on the next trading day, not a traded price).
   - legend titles "✓ 适合 <n>", "! 待查 <n>", "– 未判定 <n>", "账户现金"; `tiers[].word` "适合", "待查", "未判定" (from new consts, not from `tierGreyWord`); tier titles "适合 <n>", "待查 <n>", "<n> 行：没有买入价" (the grey tails: "没有现价", "价内", "没有买价", "保本点高于买入价", "数字读不出" after the same "<n> 行：" head; "没有买入价" for the no_buy_price tail).
   - columns "详情", "档位", "合约", "安全边际", "距现价", "权利金 / 年化", "徽标"; each column `hint` is one line of at most 40 characters before the data day, and the web prints it as visible text under the head.
   - row texts: contract "看跌 <strike>, <day> · <n> 天"; margin sub "保本点 <p> / 买入价 <p>"; below-spot sub "<x> 个预期波动*"; premium sub "<y>% / 年".
   - badge labels, exactly: no_open_interest "无未平仓量", low_open_interest "未平仓量低", one_sided_quote "单边报价", wide_quote "报价价差大", stale_quote "报价过时", earnings_before_expiry "到期前有财报", cash_short "现金不足", delta_above_bound "Delta 超上限", delta_unknown "Delta 未知", thin_premium "权利金薄", earnings_unconfirmed "财报日未确认", earnings_beyond_horizon "财报日在期限之外", earnings_unknown "财报日未知", cash_unknown "现金未知", no_jpy_rate "无日元现金利率", no_jpy_funds "无日元账户". Their glosses stay.
6. The web prints payload texts and decides nothing. Order on the page: head, legend, cards, `card_note`, tier table. Card line 1: the pill (`tier.glyph` and `tier.word`, `data-tier` from `tier.id`), the ticker, the buy price or `price_note` with `source_badge`, `spot_text` with `spot_note`, `strip_labels.next_earnings` with `earnings_text`. Line 2: `line_label`, `line_text`, `note`, then the counts from `count_parts` with the words of `strip_labels` and `grey_hint` as visible text. A card with `valuation` null draws line 1 and the counts only. A card with a valuation whose `reason` is `no_profile` draws `line_label` and the `note` action (the mockup's "估值买入线 无估值档案" is `line_text` "无估值档案"). The gate line is not drawn. The head draws `head_labels.data_day` before `chain_day`, `run_status` with `run_status_text`, `rule_version` before the rule id, each with its label from the payload.
7. Web English-only rule: the words are payload texts; the web keeps no Chinese literal. The glyphs come from the payload. The web fallback strings that this page owns (`Stored tables`, `Rule`) are replaced by payload texts; the old labels `Spot`, `rows of`, `Buy price`, `Next earnings` are removed from the card.
8. No new design token and no theme file is edited, so the web entry needs no order after `site-look`; if it finds it needs a token that does not exist it stops and says so.
9. Not in this design (named in Left out): the expanded-row labels and lines of the main table ("IV (this contract)", "Bid per share", "fits: none", the cash lines: `tierstrings.go:31-73`) and the English nav word `Watch list` that the Chinese path "公司 > 关注列表" points at (`web/src/shell/TopNav.tsx:56`). The first belongs to the 23-column table, which has no mockup; the second is the shell nav, a file of another page.
10. Failure behaviour: with a resolver read error every strip has reason `read_failed`, `run_state` "none" and `note` null, as today (`valueline.go:55-57`); no text is a reason to fail the page.

## Modules

`docs/design/modules.md` is not changed: the paths below match the rows contracts (`contracts/**`, `web/src/generated/**`, `internal/contracts/**`, `docs/contracts/page-schemas.md`), pages (`internal/pages/**`) and web (`web/**`), and the first row whose glob matches owns each path (`web/src/generated/**` is contracts, so the generated type is in the contracts entry).

- contracts: `contracts/schemas/page.option_candidates_page.v1.json`, `web/src/generated/option_candidates_page.v1.ts`, `internal/contracts/option_candidates_page_v1_test.go`, `docs/contracts/page-schemas.md` (entry 1).
- pages: `internal/pages/option_candidates_page/**` (entry 2).
- web: `web/src/pages/candidates/{CompanyStrip,TierView,TierRows,OptionCandidatesPage}.tsx`, `web/test/pages/optioncandidates/**` (entry 3).

No store, dataset or migration change; nothing holds the auto-deploy.

## Interface

`contracts/schemas/page.option_candidates_page.v1.json`, v1 gains optional properties (`additionalProperties: false` kept on every object, texts only, no JSON number, `required` lists unchanged):

    Page      += head_labels?:  { data_day, run_status, run_status_text, rule_version: Text }
    Company   += tier?:         { id: "fits"|"check"|"grey", word: Text, glyph: Text }
                 count_parts?:  { fits: Text, check: Text, grey: Text }
                 spot_text?:    Text | null
                 spot_note?:    Text | null
                 price_note?:   Text | null
                 earnings_text?: Text
                 source_badge?: Badge | null
    ValueLine += line_label?:   Text
                 line_text?:    Text
                 run_state?:    "used"|"not_used"|"none"
                 note?:         Text | null
    TierView  += strip_labels?: { spot, buy_price, next_earnings, contracts, fits, check, grey, grey_hint: Text }
                 card_note?:    Text | null

Generated type `web/src/generated/option_candidates_page.v1.ts` (never edited by hand; regenerate with `pnpm --dir web run generate:types`, and run `sh contracts/generate.sh` for the Go records, as CI does): the types gain the optional members and nothing else changes, so `CompanyStrip.tsx` and the other readers compile unchanged. The contract test `internal/contracts/option_candidates_page_v1_test.go` builds its instance by hand (`tiersRow`, `tiersView`, the valuation map) and is the test of the contract from both sides. `docs/contracts/page-schemas.md` (the page entry) gets one sentence on the new fields.

## Issues

Build order: entry 1 first, entry 2 after entry 1 (it emits the new fields), entry 3 after entry 2 (the page needs the fields in the payload). Entries are not merged (contracts, pages and web are three modules). The old fields are kept by all three, so each merge leaves main green and the site readable.

### 1 candidates-zh-contract (interface)

Module: contracts. Size: S.
Order: first.
Files: `contracts/schemas/page.option_candidates_page.v1.json`, `web/src/generated/option_candidates_page.v1.ts`, `internal/contracts/option_candidates_page_v1_test.go`, `docs/contracts/page-schemas.md`
Contracts: changed (`contracts/schemas/page.option_candidates_page.v1.json`)
Design: docs/design/candidates-zh.md#Interface
Plan: P7
Acceptance:
1. The schema has the optional properties of the Interface section on `Page`, `Company`, `ValueLine` and `TierView`, each text typed `$ref: #/$defs/Text` (no reference to `Text1`), and every `required` list is exactly what it is on main today; no property is removed and `additionalProperties` is false on each new object.
2. `run_state` is an enum of exactly `used`, `not_used`, `none`; `tier.id` is an enum of exactly `fits`, `check`, `grey`; no new property has a JSON number type.
3. `web/src/generated/option_candidates_page.v1.ts` is regenerated with `pnpm --dir web run generate:types` (and the Go records with `sh contracts/generate.sh`) in the same change and holds the new optional members and nothing removed, so `tsc` passes with no change to `web/src`.
4. `internal/contracts/option_candidates_page_v1_test.go` has `TestTiersNewFieldsValidate` (new: an instance with all new fields passes), `TestTiersOldShapeStillValidates` (new: an instance with only the old fields passes), `TestRunStateEnum` (new: `"maybe"` fails) and `TestTierIDEnum` (new: `"fit"` fails); the existing tests still pass.
5. `docs/contracts/page-schemas.md` names the new fields in one sentence under the page entry.
6. The result matches docs/design/candidates-zh.md#Interface.
Checked by CI: `go test ./internal/contracts/...`; `tsc` on the regenerated type.
Adversarial: docs/design/candidates-zh.md#Adversarial, points 2, 3, 5.3, 6.3
Left out: removing the old fields.

### 2 candidates-zh-pages

Module: pages. Size: L.
Order: after 1.
Files: `internal/pages/option_candidates_page/tiers.go`, `internal/pages/option_candidates_page/tierstrings.go`, `internal/pages/option_candidates_page/tierlegend.go`, `internal/pages/option_candidates_page/tierrows.go`, `internal/pages/option_candidates_page/valueline.go`, `internal/pages/option_candidates_page/strings.go`, `internal/pages/option_candidates_page/builder.go`, `internal/pages/option_candidates_page/buysource.go`, `internal/pages/option_candidates_page/zh_texts.go`, `internal/pages/option_candidates_page/tiers_test.go`, `internal/pages/option_candidates_page/tierrows_test.go`, `internal/pages/option_candidates_page/tierlegend_test.go`, `internal/pages/option_candidates_page/valueline_test.go`, `internal/pages/option_candidates_page/buysource_test.go`, `internal/pages/option_candidates_page/page_test.go`, `internal/pages/option_candidates_page/builder_test.go`, `internal/pages/option_candidates_page/cells_test.go`, `internal/pages/option_candidates_page/evcells_test.go`, `internal/pages/option_candidates_page/evidence_test.go`, `internal/pages/option_candidates_page/jp_test.go`, `internal/pages/option_candidates_page/reasons_test.go`, `internal/pages/option_candidates_page/shortlist_test.go`, `internal/pages/option_candidates_page/shortlistfacts_test.go`, `internal/pages/option_candidates_page/tiersfakes_test.go`, `internal/pages/option_candidates_page/zhstrip_test.go`, `internal/pages/option_candidates_page/testdata/evidence_ok.golden.json`, `internal/pages/option_candidates_page/testdata/evidence_gaps.golden.json`, `internal/pages/option_candidates_page/testdata/jp_rows.golden.json`, `internal/pages/option_candidates_page/testdata/shortlist_ok.golden.json`, `internal/pages/option_candidates_page/testdata/shortlist_gaps.golden.json`, `internal/pages/option_candidates_page/testdata/tiers_ok.golden.json`, `internal/pages/option_candidates_page/testdata/tiers_gaps.golden.json`, `internal/pages/option_candidates_page/testdata/valueline.golden.json`
Contracts: unchanged
Design: docs/design/candidates-zh.md#Decisions
Plan: P7
Acceptance:
1. Each company of `short_put_tiers.companies` has `tier` with `id`, `word` and `glyph` from the three counts of the answer: fits above zero gives `fits`, "适合", "✓"; else check above zero gives `check`, "待查", "!"; else `grey`, "未判定", "–" (new test `TestCompanyTierFromCounts`, the three cases and an all-zero company).
2. Each company has `count_parts` of three whole-number texts equal to the answer's fit, check and grey counts, and the old `counts` string is still sent with its old value (new test `TestCompanyCountParts`).
3. `spot_text` is the spot price and `spot_note` is "存档 " plus the stored day without its year; both are null when the company has no spot (new test `TestSpotNote`). `price_note` is "没有买入价" when there is no buy price, else null; `earnings_text` is the day or "未知" (new test `TestCompanyZhTexts`).
4. `source_badge` has label "手填" for a hand price, "估值" for a valuation, and is null for none; the old `buy_source` of every company and the short list badge for none ("Not judged") are unchanged (new test `TestStripSourceBadge`; the existing `buysource_test.go` cases pass unchanged).
5. `short_put_tiers.strip_labels` has the eight words of Decision 3 and `card_note` has the sentence of Decision 3, all from `zh_texts.go` (new test `TestStripLabelsChinese`, in the new file `zhstrip_test.go`, which also holds `TestStripSourceBadge`).
6. `valuation.run_state` is `used` when a figure is shown and the company's `buySourceCode` is the valuation code, `not_used` when a figure is shown and it is not, `none` when `reason` is non-null; `line_label` is "估值买入线", `line_text` is the figure alone (or the reason sentence), and `note` is the sentence of Decision 3 for each state (new tests `TestValueRunStateUsed`, `TestValueRunStateNotUsed`, `TestValueRunStateNone`; the not-used case is a figure with a company whose stored rows carry no buy price, and `note` contains "最近一次运行没用到"). The old `label`, `text`, `hint` and `gate_*` have the values they have on main (existing `TestValueLineFromCalculator` and `TestValueLineTextsFromZhConsts` pass unchanged).
7. `head_labels` has the four words of Decision 3, `run_status_text` is "完成" for `ok` and "部分完成" for `partial`, and `short_put_tiers.stored_title` is "已存表格" (new test `TestHeadLabels`).
8. The page title is "期权候选"; the group titles are "先卖出看跌", "备兑看涨", "持仓看涨保护"; the legend titles, `tiers[].word`, tier titles, column labels, column hints (one line of at most 40 characters before the day), row sub texts and the sixteen badge labels are the Chinese of Decision 5 (new test `TestTierViewZhWords`: each word of the list is compared to its exact string; no label or title of the tier view has an ASCII letter except the allowed terms `Delta`, `DGS3MO`, `IV`, a price, a day or a symbol).
9. Every new Chinese string is a const in `zh_texts.go` or in `tierstrings.go` (const-only, no `+`; a text with a day is made in `valueline.go` or `tiers.go` from two consts), and `TestZhTextsDoor` of `cmd/ot` passes.
10. The short list rows keep every text they have on main (new test `TestShortListTextsUnchanged`: the none badge reads "Not judged" and the source labels read "Hand price" and "Valuation buy zone").
11. The eight goldens hold the new fields and words and the old fields as before. There is no update flag: the tests only compare, so the goldens are edited by hand from a dump of `Build` output printed by a throwaway program outside the repository, and the dump is read before it is pasted; `TestEvidenceValidatesSchema` and the golden comparison pass on the result.
12. No file is over 600 lines that was not before and no file over 800 lines grows.
13. The result matches docs/design/candidates-zh.md#Decisions.
Checked by CI: `go test ./internal/pages/... ./cmd/ot/...`.
Adversarial: docs/design/candidates-zh.md#Adversarial, points 5.1, 5.2, 5.5 to 5.8, 6.4 to 6.6
Left out: the 23-column main table, its expanded-row labels and lines, the evidence cells, the reasons, the short list texts, tap hints, removing the old fields.

### 3 candidates-zh-web

Module: web. Size: M.
Order: after 2.
Files: `web/src/pages/candidates/CompanyStrip.tsx`, `web/src/pages/candidates/TierView.tsx`, `web/src/pages/candidates/TierRows.tsx`, `web/src/pages/candidates/OptionCandidatesPage.tsx`, `web/test/pages/optioncandidates/CompanyStrip.test.tsx`, `web/test/pages/optioncandidates/TierView.test.tsx`, `web/test/pages/optioncandidates/OptionCandidatesPage.test.tsx`, `web/test/pages/optioncandidates/fixtures/tiers.ts`, `web/test/pages/optioncandidates/fixtures/tiers-null.ts`, `web/test/pages/optioncandidates/fixtures/valueline.ts`, `web/test/pages/optioncandidates/fixtures/ok.ts`, `web/test/pages/optioncandidates/fixtures/jp.ts`
Contracts: unchanged
Design: docs/design/candidates-zh.md#What-you-see
Plan: P7
Acceptance:
1. `TierView` draws the legend, then the company cards, then the sentence `card_note` as visible text, then the tier table (new test `TestCardsBeforeTable`: the cards come before the table in document order).
2. `CompanyStrip.tsx` draws two lines: line 1 is the pill (`tier.glyph` and `tier.word`, `data-tier` from `tier.id`), the ticker, the buy price or `price_note` with `source_badge`, `spot_text` with `spot_note`, `strip_labels.next_earnings` with `earnings_text`; line 2 is `line_label`, `line_text` and `note`, then the three counts with the words of `strip_labels` and `grey_hint` as visible text (new test `TestStripTwoLines`).
3. `CompanyStrip.tsx`, `TierView.tsx`, `TierRows.tsx` and `OptionCandidatesPage.tsx` hold no label of our own: no `Spot`, `rows of`, `Buy price`, `Next earnings`, `Rule `, `Stored tables` and no Chinese literal; every word is a payload text (new test `TestStripWordsFromPayload`: a fixture with other words shows exactly those words; `web/test/english.test.ts` still passes).
4. The gate line is not drawn: `[data-gate]` is absent for every fixture (existing `TestStripShowsValueLine` and `TestStripNoValuationUnchanged` updated to the new texts).
5. A company with `run_state` `not_used` shows its note beside the figure, `used` and `none` show theirs (new test `TestStripRunStateNotes`, three fixtures).
6. The pill is the payload word and glyph, never derived from the counts in the web (new test `TestPillIsPayloadWord`: counts say fits and `tier` says grey, the pill shows grey).
7. The page head shows `head_labels.data_day` with `chain_day`, `head_labels.run_status` with `run_status_text`, and `head_labels.rule_version` with the rule id, each label from the payload; the accordion title is `stored_title` (updated tests in `OptionCandidatesPage.test.tsx`).
8. The column subtitles of the tier table are visible text under each head (updated test in `TierView.test.tsx`), and the page still has no `max-w-` class under `web/src/pages/candidates/` (new test `TestNoWidthCap`).
9. The fixtures `tiers.ts`, `tiers-null.ts`, `valueline.ts`, `ok.ts` and `jp.ts` carry the new fields.
10. No theme file and no `web/src/index.css` is edited and no new token is used.
11. The result matches docs/design/candidates-zh.md#What-you-see.
Checked by CI: `pnpm vitest`, `tsc`, `eslint`, the web build. A UI pass counts only with a screenshot of the live page beside the mockup at 1280 and 390 px, taken by the window after the deploy (owner rule 2026-11-14).
Adversarial: docs/design/candidates-zh.md#Adversarial, points 1.1 to 1.5, 6.1
Left out: the 23-column main table and its expanded row, the short list, the English nav words, tap hints, the theme.

## Checked by CI

`go test` for contracts, pages and `cmd/ot`; `tsc`, `eslint`, `vitest` and the web build for the web entry. The screenshot against the mockup is the window's check after the deploy, not a CI check.

## Adversarial

Read: `docs/design/candidates-zh.md` at head acd5078a against `adversarial.md` in the designs folder of the drafts state (`designs/candidates-zh/adversarial.md`). Every blocking point is answered; the dispatcher took the simpler way of its section 3.

- 1.1 cards after the table: adopted. Decision 6 and web acceptance 1 fix the order (cards, `card_note`, table) with a test.
- 1.2 page head words: adopted. `head_labels` (entries 1, 2) and `OptionCandidatesPage.tsx` in the web Files (acceptance 7); the `Stored tables` fallback is replaced by `stored_title`.
- 1.3 column subtitles in `TierRows.tsx`: adopted. `TierRows.tsx` is in the web Files; web acceptance 8.
- 1.4 pill glyph: adopted. `tier.glyph` in the payload, printed by the web.
- 1.5 sentence under the cards: adopted. New `TierView.card_note` drawn as visible text; the old `spot_hint` stays unchanged.
- 1.6 English nav words in the action text: not adopted as a change: `TopNav.tsx` belongs to the shell and the web English-only rule, so it is named in Left out and Decision 9; the check of the tier view words is `TestTierViewZhWords`, which compares exact strings and covers the groups and the badges.
- 1.7 expanded row English: not adopted, named in Left out with the reason (part of the 23-column table, which has no mockup); Goal reads for the card and table of the mockup.
- 1.8 AAA card line 2: adopted. A valuation with reason `no_profile` draws `line_label` and `line_text` "无估值档案" (Decision 6).
- 1.9 small wording differences: adopted where visible (tier titles, contract line as the map gives them); no change else.
- 2 red main at entry 1: adopted by the dispatcher's simpler way. No `required` list changes and no old field changes, so the generated type changes by additions only and the web type-check stays green; no web file is in entry 1.
- 3 simpler way: adopted. Three entries; the drop entries are gone; old fields stay (Left out: a later clean-up). The pages entry stays one entry (size L) as the dispatcher decided; the golden fixes are the same change.
- 4 lock-in: accepted as low. Day-bearing sentences are made from two consts in a non-door file (acceptance 9).
- 5.1 short list badge changes with `tierGreyWord`: adopted. Decision 4: `tierGreyWord`, `notJudged` and the `buySourceTexts` labels are unchanged; the card uses new consts and the new `source_badge`; `TestShortListTextsUnchanged`.
- 5.2 `gate_text` changes with `notJudged`: adopted, same change; old fields keep their values, and `valueline_test.go:195` keeps passing.
- 5.3 `Text1` in the schema: adopted. The schema uses `$ref Text`; 200-character cap noted in Decision 3.
- 5.4 web type-check: adopted (see 2).
- 5.5 no regenerate command for goldens: adopted. Pages acceptance 11 says the goldens are edited by hand from a dump of `Build` output.
- 5.6 `run_state` after `buy_source` null: adopted. `buy_source` is no longer nulled; `buySourceCode` is the value used, and `buysource.go` is in the Files.
- 5.7 `valueFigure` signature: adopted. The old `text` is not changed; `line_text` is a new value built beside it, so the old function stays and `TestValueLineTextsFromZhConsts` passes unchanged.
- 5.8 `zhstrip_test.go` not marked new: adopted. Pages acceptance 5 says the file is new and names the tests in it.
- 6.1 missing Files: adopted: `TierRows.tsx`, `OptionCandidatesPage.tsx`, `tiers-null.ts` added.
- 6.2 diff-shape acceptance: adopted. No `git diff` item remains; every item states a result. The ASCII whitelist is named (`Delta`, `DGS3MO`, `IV`, a price, a day, a symbol).
- 6.3 generated type command: adopted: `pnpm --dir web run generate:types` and `sh contracts/generate.sh`.
- 6.4 golden command: adopted (5.5).
- 6.5 badge words invented by the worker: adopted. The sixteen badge labels are given exactly (Decision 5).
- 6.6 modules touched twice and the door: adopted. The drop pair is gone, so pages is one entry; `TestNoArithmeticInValueLine` is kept (the day cut is string slicing, in `valueline.go` and `tiers.go`).

## Left out

- Removing the old fields (`Company.counts`, `buy_price_note`, `next_earnings`, `spot`, `buy_source`, `ValueLine.label`, `text`, `gate_*`, the `spot_hint`): a later clean-up pair of issues.
- The 23-column main table, its expanded-row labels and lines (`IV (this contract)`, `Bid per share`, `fits: none`, the cash lines), the evidence cells and the reasons; the short list texts (its badge, its English source labels).
- The English nav word `Watch list` in `web/src/shell/TopNav.tsx` that the action text "公司 > 关注列表" points at: the shell is another page.
- Tap hints on touch screens (the visible glosses replace the tooltip for the card; a tap hint for the rest is its own design).
- The theme, fonts and sizes (`docs/design/site-look.md`); the page width (full width stays).
- A "used by the run" state for sources other than the valuation; the tier verdict for the other groups.
- Translating the names of option terms (put, delta, IV, DGS3MO stay as the terms they are, glossed beside).
