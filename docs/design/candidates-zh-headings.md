# candidates zh headings: the candidates page gets its Chinese subtitle and the Chinese headings of the company cards and the contract tiers, from the payload

Author: the optdesk designer (draft)
Owner ground: ask optdesk-5206 (owner 2026-11-19 08:00Z "开始吧" started P7, the Chinese-first site; 09:15Z approved the Chinese-first mockup of the candidates page; issues 3228 and 3229 were cut from it); this design is pending
Date: 2026-11-19

Source: the live check of `/options/candidates` against the approved mockup, 2026-11-19 (after pull requests 3230 and 3231): the mockup has a Chinese subtitle under the title and two section headings, "公司" (with a one-line gloss) and "合约分档"; the live page has none of the three because the payload has no field for them. Owner rules carried: `web/src` is English-only and the Chinese comes from the Go payload (`internal/pages/option_candidates_page/zh_texts.go`, checked by `TestZhTextsDoor`); the front end renders data and computes nothing; payload fields are added, never changed. Built on `docs/design/candidates-zh.md` (its `head_labels`, `strip_labels`, `card_note` pattern).

Read on origin/main: `AGENTS.md`, `docs/design/modules.md`, `docs/design/candidates-zh.md`, `docs/design/site-look.md`, `contracts/schemas/page.option_candidates_page.v1.json`, `web/src/generated/option_candidates_page.v1.ts`, `internal/pages/option_candidates_page/{builder,tiers,zh_texts}.go`, `web/src/pages/candidates/{OptionCandidatesPage,TierView,TierRows}.tsx`, the mockup and its `labels.md`.

## What you see

On `/options/candidates`, under the title "期权候选", one grey sentence: "先卖出看跌期权（short put）的候选合约，按公司分组；只有你设了买入价的公司才会被评判。" Above the company cards a heading "公司" with the gloss "每家公司两行：上行看价格，下行看估值和合约数量". Above the tier tables a heading "合约分档" (no gloss: the subtitle already says "先卖出看跌" and the chain-day badge already shows the data day). The Japanese tab's tier view carries the same two headings.

## Modules

`docs/design/modules.md` is not changed. The paths below match the rows contracts (`contracts/**`, `web/src/generated/**`, `internal/contracts/**`, `docs/contracts/page-schemas.md`), pages (`internal/pages/**`) and web (`web/**`); the first matching row owns each path. No store, dataset or migration change; nothing holds the auto-deploy.

- contracts: `contracts/schemas/page.option_candidates_page.v1.json`, `web/src/generated/option_candidates_page.v1.ts`, `internal/contracts/option_candidates_page_v1_test.go`, `docs/contracts/page-schemas.md` (entry 1).
- pages: `internal/pages/option_candidates_page/{builder,tiers,zh_texts}.go`, the new test file `zhheads_test.go` and the five golden files that print a tier view (`testdata/tiers_ok.golden.json`, `tiers_gaps.golden.json`, `jp_rows.golden.json`, `shortlist_ok.golden.json`, `valueline.golden.json`) (entry 2).
- web: `web/src/pages/candidates/{OptionCandidatesPage,TierView}.tsx`, `web/test/pages/optioncandidates/OptionCandidatesPage.test.tsx`, `TierView.test.tsx` and the fixtures `fixtures/ok.ts`, `fixtures/tiers.ts`, `fixtures/tiers-null.ts`, `fixtures/jp.ts` (entry 3); `fixtures/MANIFEST.synthetic-v1.json` there lists file names only and no file is added, so it is unchanged.

## Interface

Owner of the shape: contracts. `contracts/schemas/page.option_candidates_page.v1.json`, v1 gains two optional properties (`additionalProperties: false` kept on the new object, texts only, no JSON number, every `required` list unchanged):

    Page      += subtitle?: Text
    TierView  += heads?:    { companies, companies_note, tiers: Text }   // optional and not nullable

`Text` is `#/$defs/Text` (1 to 200 characters). The pages module fills them; the web module prints them as given. Texts:

    subtitle        先卖出看跌期权（short put）的候选合约，按公司分组；只有你设了买入价的公司才会被评判。
    companies       公司
    companies_note  每家公司两行：上行看价格，下行看估值和合约数量
    tiers           合约分档

The pattern is that of `strip_labels` and `card_note` (a labels object filled in `fillStrips`, `tiers.go`). Unlike them `heads` is a pointer in Go with `omitempty`, and optional and non-null in the schema: a page with no tier view, or a stored page read by an older reader, has no `heads` key and never `"heads": null`. There is no `tiers_note`: the chain-day badge already shows the data day and the subtitle already says "先卖出看跌".

## Decisions

1. The page shows the Chinese subtitle under the title, before the badges and the bid note; it is one sentence, plain text, visible without a tooltip.
2. The tier view shows a heading "公司" with its gloss above the company cards and a heading "合约分档" above the first tier section. Both are headings of the page outline (second level), and the gloss of "公司" sits beside it in the smaller muted style of the mockup.
3. Every one of these words comes from the Go payload; the web files carry no Chinese and no new English word.
4. A payload without `subtitle` or `heads` (an older stored page) draws nothing for them and no error; the old fields are kept and the page reads as it does today.
5. The Japanese tier view gets the same headings through the same builder step.
6. The table column heads are not touched (see Left out).

## Issues

Build order: entry 1 first; entries 2 and 3 after entry 1 and in parallel (entry 3's tests type-check against the regenerated types and read their own TypeScript fixtures, so they need nothing of entry 2). Entries are not merged (three modules). Each merge leaves main green: the fields are optional.

### 1 candidates-zh-headings-contract (interface)

Module: contracts. Size: S.
Order: first.
Files: `contracts/schemas/page.option_candidates_page.v1.json`, `web/src/generated/option_candidates_page.v1.ts`, `internal/contracts/option_candidates_page_v1_test.go`, `docs/contracts/page-schemas.md`
Contracts: changed (`contracts/schemas/page.option_candidates_page.v1.json`)
Design: docs/design/candidates-zh-headings.md#Interface
Acceptance:
1. The schema has optional `subtitle` on `Page` and optional, non-null `heads` on `TierView`, with the three texts of the Interface section, each `$ref: #/$defs/Text`; `heads` lists its three names in `required` and has `additionalProperties: false`; no `required` list outside `heads` differs from main; nothing is removed; no new property is a JSON number.
2. `web/src/generated/option_candidates_page.v1.ts` is regenerated in the same change and holds the new optional members and nothing removed.
3. `option_candidates_page_v1_test.go` has `TestHeadingsFieldsValidate` (new: an instance with `subtitle` and `heads` passes), `TestHeadingsOldShapeStillValidates` (new: an instance without them passes) and `TestHeadsNeedAllThree` (new: `heads` missing `tiers` fails) and `TestHeadsNullFails` (new: `"heads": null` fails); the existing tests still pass.
4. `docs/contracts/page-schemas.md` names the two new fields in one sentence under the page entry.
Checked by CI: `go test ./internal/contracts/...`; `tsc` on the regenerated type.
Left out: removing any old field.

### 2 candidates-zh-headings-pages

Module: pages. Size: S.
Order: after 1.
Files: `internal/pages/option_candidates_page/builder.go`, `internal/pages/option_candidates_page/tiers.go`, `internal/pages/option_candidates_page/zh_texts.go`, `internal/pages/option_candidates_page/zhheads_test.go`, `internal/pages/option_candidates_page/testdata/tiers_ok.golden.json`, `internal/pages/option_candidates_page/testdata/tiers_gaps.golden.json`, `internal/pages/option_candidates_page/testdata/jp_rows.golden.json`, `internal/pages/option_candidates_page/testdata/shortlist_ok.golden.json`, `internal/pages/option_candidates_page/testdata/valueline.golden.json`
Contracts: unchanged
Design: docs/design/candidates-zh-headings.md#Decisions
Acceptance:
1. `zh_texts.go` holds the four texts of the Interface section as string constants and nothing else new; `TestZhTextsDoor` still passes.
2. The page struct in `builder.go` has `Subtitle` with the JSON name `subtitle`, set on every built page; the tier view struct in `tiers.go` has `Heads` (a pointer, tag `json:"heads,omitempty"`) set by `fillStrips` beside `StripLabels`.
3. Every tier view of the page, the Japanese one included, carries `heads`; a page with no tier view has no `heads`.
4. `zhheads_test.go` (new file) has `TestSubtitleAndHeads` (new): the built page has the subtitle text, and a built tier view has `heads` with the three texts, and a page with no tier view marshals with no `heads` key.
5. The five golden files named in Modules carry the new fields; no existing field changed.
6. `tiers.go` (466 lines) and `builder.go` (260) stay under 600 lines; `zhheads_test.go` is a new file under 600; no ceiling in `docs/design/ratchet.json` changes.
Checked by CI: `go test ./internal/pages/...`.

### 3 candidates-zh-headings-web

Module: web. Size: S.
Order: after 1.
Files: `web/src/pages/candidates/OptionCandidatesPage.tsx`, `web/src/pages/candidates/TierView.tsx`, `web/test/pages/optioncandidates/OptionCandidatesPage.test.tsx`, `web/test/pages/optioncandidates/TierView.test.tsx`, `web/test/pages/optioncandidates/fixtures/ok.ts`, `web/test/pages/optioncandidates/fixtures/tiers.ts`, `web/test/pages/optioncandidates/fixtures/tiers-null.ts`, `web/test/pages/optioncandidates/fixtures/jp.ts`
Contracts: unchanged
Design: docs/design/candidates-zh-headings.md#Decisions
Acceptance:
1. `OptionCandidatesPage.tsx` prints `payload.subtitle` in a paragraph right after the title heading and before the badges, only when the field is present.
2. `TierView.tsx` prints `heads.companies` with `heads.companies_note` as a second-level heading above the company cards, and `heads.tiers` as a second-level heading above the tier sections, only when `heads` is present.
3. Neither file has a new string literal that is visible text; the files compute, join and sort nothing.
4. `OptionCandidatesPage.test.tsx` and `TierView.test.tsx` each have one new test (new) that renders the fixture (`ok.ts` for the subtitle; `tiers.ts` for the tier view, `jp.ts` for the Japanese one, each with `subtitle` or `heads` added) and finds the subtitle and the headings with the note of "公司", and one (new) that renders a payload without the fields (`tiers-null.ts` among them) and finds none of them and no error.
5. Table head code (`TierRows.tsx`, `ShortListRows.tsx`, `table.tsx`) is unchanged.
Checked by CI: `tsc`, `eslint`, `vitest`, the build.

## Adversarial

Answer to `adversarial-2026-11-19-two-designs.md` (section A) and the advisor opinion of the same day (`advisor-2026-11-19-two-designs.md`, A1 to A3).

- A.1 `tiers_note` repeats the chain-day badge and the subtitle: adopted, `tiers_note` is dropped (Interface, Left out). The gloss of "公司" describes the card layout and can turn false: not adopted, kept as in the approved mockup; a later card change edits that one constant.
- A.2 `heads` as a plain struct could emit `"heads": null`: adopted, pointer with `omitempty`, optional and non-null in the schema, with `TestHeadsNullFails`.
- A.3 reuse `strip_labels` or `card_note`: adopted as the pattern (same builder step, same file), not as the field: `strip_labels` is required and belongs to the strips; `heads` stays its own optional object.
- A.4 lock-in of a required-all object: accepted, v1 adds only optional objects and a later heading is one more optional field.
- A.5 size line "under its size today" impossible: adopted, entry 2 item 6 states the limits. Entry 3 Files and fixtures: adopted (`tiers.ts`, `tiers-null.ts`, `jp.ts`; the manifest lists names only, so unchanged). Golden glob: adopted, five exact files. `TestZhTextsDoor` allows const strings only: kept, all four texts are constants.
- Advisor A1: this section. A2: adopted as above. A3: the owner ground is in the header; the texts 公司 and 合约分档 are kept.

## Left out

- The capital letters of the table heads: `docs/design/site-look.md` decides them (the shared `TableHead` in `web/src/components/ui/table.tsx`, used by `TierRows.tsx` and `ShortListRows.tsx`); this design leaves that as it is.
- A heading for the short list and the stored-tables accordion, the summary strip look, the card left border, the doubled words "待查 待查" and "折叠 折叠" of the live check: other gaps of the same check, not part of these three texts.
- A gloss under "合约分档" (the mockup's "先卖出看跌 · 数据日 <day>"): the day is the badge's and the first words are the subtitle's; adding it back is one optional field.
- The heading wording "公司分档" for the cards: the approved mockup says "公司" for the cards (the tier of a company is its pill) and "合约分档" for the tables; the design follows the mockup.
- Removing old fields; any theme, font or size change.
