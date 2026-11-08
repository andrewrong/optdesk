# Option pages: bring option_page, stock_option_page, option_data_page and full_scan to boards 5, 17 and 18

Author: the optdesk designer (second draft, after the adversarial review)
Owner approved: (pending)
Revision: 4

Source: the page comparison of 2026-11-08 (design set v2.2, boards 5, 17, 18, plus the later owner changes #1377, #1758, #1789, #1797, #1799, #1803, #1804, PR1837, PR1843, PR1847). The owner chose to turn every deviation into a design.
Read on origin/main at 3ef73b1: `contracts/schemas/page.option_page.v1.json` (847 lines), `page.stock_option_page.v1.json` (1017 lines), `page.option_data_page.v1.json`, `page.full_scan.v1.json`, `contracts/fixtures/page.option_data_page.v1.*.json`, `contracts/fixtures/MANIFEST.synthetic-v1.json`,
`internal/contracts/fixtures_test.go`, `web/src/pages/options/OptionPage.tsx`, `web/src/pages/stockoption/*.tsx`, `web/src/pages/optiondata/*.tsx`, `web/src/pages/candidates/FullScanPage.tsx`, `web/fixtures/option-page`, `web/fixtures/full-scan`, `web/test/pages/stockoption/fixtures`,
`docs/design/option-page-serve.md`.

Goal: the option page, the full candidate table and the one-stock page show what boards 5, 17 and 18 show: on each opportunity card the contract line "sell put 62 · 11-13 周五 · 22 天 · Δ −0.15", the IV against the stock's own HV30 and IV percentile, the rich-premium flag and the E tag (a plain badge here; the red label is only on the option data page);
the candidate table with strike %, DTE, Δ, spread %, Vol, OI, IVP and E; a chain card that is always open; on the one-stock page Θ, Γ and Vega, IVP, HV30, the E text as a plain badge and a yield or premium toggle; on the option data page the earnings text with the red E (`earnings_tone`), HV30 and IVP and the JPY cell with its USD approximation. The option page and the one-stock page show E as a plain secondary badge until their v2.
Almost all of this is text that the payloads already carry, so only `option_data_page` gets three optional strings and one optional enum. The pages show "缺" for IVP and HV30 until the compute design gives them a producer. No page here has a route yet; `ot` serves `/` and `/options/data` only (`docs/design/option-page-serve.md`). Producer-written numbers (HV30, IVP, delta, the ease value, tile figures) wait for the compute design's register of metric ids: the live page shows 缺 until a producer exists, and the fixtures are the only source of their values. Schema field descriptions of such fields say "metric id: assigned by compute".

## What main holds today

- `option_page.v1` and `stock_option_page.v1` are carried files (in `carryover.sha256`); a field change would be a new file, and copying them breaks the 600-line rule (new files of 847 and 1017 lines). This design changes neither schema.
- `Opportunity` of `option_page.v1` has the free text `contract_line`, `iv_line`, `liquidity_line`, `premium_line` and `tags` (strings); `MarketPane.chain` is a `Chain` or null; the tables are driven by `columns` and `cells`, so Δ, strike %, DTE, spread %, IVP and E columns are data.
- `TopBar` of `stock_option_page.v1` has `iv` and `earnings` as nullable strings; `ContractsBlock` has 11 columns (合约 Bid Ask Mid Vol OI IV Δ Θ Γ Vega fit); `MatrixCell` has `yield_text` and `premium`; `Tables.tsx` shows `yield_text` with `premium` as a note and has no toggle.
- `internal/contracts/fixtures_test.go` maps `web/fixtures/option-page` to `page.option_page.v1.json` and `web/fixtures/full-scan` to `page.full_scan.v1.json`, validates every file there and expects each `bad-version.json` to fail on `schema_version`. A fixture edit that stays valid v1 passes; a version bump would not.
- `option_data_page.v1` is written for this repository and not carried; it takes optional fields without a version change (precedent: `underlyings`, `docs/design/option-page-serve.md#interface`, and its Go builder fills it). Its `Figure` and `Underlying` have `additionalProperties` false.
  `ChainTable.tsx` writes its column labels in English; `web/test/pages/optiondata/OptionDataPage.test.tsx` uses the contracts fixtures `ok`, `terms` (strikes), `gap` and `stale`. The fixture set on main is five: `ok`, `terms`, `gap`, `stale`, `choices` (`internal/contracts/optiondatapage_test.go:31`).
- Legal change: `docs/contracts/page-schemas.md:59` records that the optional field `underlyings` was added to `page.option_data_page.v1` "without a version change" and that the file is written for this repository and not in `carryover.sha256`. That line is the precedent that allows an optional field in place; it covers this file only. The carried files (`option_page`, `stock_option_page`) have no such line, so they would need a v2 and are not touched.
- Only `option_data_page` has a Go builder. Delta from a model value, HV30 and IVP have no producer on main.

## Decisions

1. No new version of `option_page` or `stock_option_page`. HV30, IVP, delta and DTE are written by the producer into `contract_line`, `iv_line` ("IV 38% · HV30 31% · IVP 72", with "HV30 缺" or "IVP 缺" when missing) and `iv`; the fixtures carry that text. The view prints it and computes nothing.
2. The red "E" label is chosen by a producer field, never by the text of a tag. `option_data_page.v1` gets the optional closed enum `Underlying.earnings_tone` (`in_term`, `outside`, `none`); the view switches on it: `in_term` destructive Badge, `outside` secondary, `none` outline. `option_page.v1` and `stock_option_page.v1` are carried and carry no such field, so their views draw the E tag and `TopBar.earnings` as a plain secondary Badge with the text as written; the red label there waits for the v2 of those pages with their builder design (Left out).
3. The rich-premium flag (PR1847) is one more tag text ("权利金偏高"); the 7-day scan lookback (PR1858, unmerged upstream) is scan-side and not part of a page.
4. The chain is never hidden (#1799): the pane shows `chain` in a shadcn Card (as `OptionPage.tsx` does today), open on first render with no collapse control; a pane whose `chain` is null shows the shell text "没有链数据" instead. Liquidity is shown beside the verdict and never folded into it.
5. `option_data_page.v1` gets three optional strings and one optional enum, in place: `Underlying.vol_line` ("IV 24.1% · HV30 缺 · IVP 缺"), `Underlying.earnings_text` ("E 11-20 · 来源 vendor", written with the vendor date, #1789), `Underlying.earnings_tone` (Decision 2) and `Figure.approx_usd_text` ("≈ US$ 123", the JpyAmount cell of PR1837). A per-row E column is not in board 18's table and is not designed.
6. Option terms stay English (sell put, IV, IVR, IVP, Δ, Θ, Γ, Vega); other shell text is Chinese. Plan rev 2.1: IV and Greeks only from IBKR model values; no pricing, no computation here.
7. Fixtures that stay valid v1 are edited in place (`web/fixtures/option-page/ok.json`, `web/fixtures/full-scan/ok.json`); the stock option fixtures are TypeScript under `web/test/pages/stockoption/fixtures` and are not validated by a Go test. Their `MANIFEST.synthetic-v1.json` lists file names only (`marker` and `files`), no hash, so editing `ok.ts` needs no manifest edit.

## Modules

None changed. `contracts/**` (schemas, fixtures, `docs/contracts/page-schemas.md`, `internal/contracts/**`, `web/src/generated/**`) belongs to contracts; `web/**` to web.

| issue | module | order |
|---|---|---|
| option-data-fields-schema | contracts (interface) | first |
| option-data-view | web | after option-data-fields-schema |
| option-page-view | web | independent |
| stock-option-view | web | independent |
| full-scan-fixtures | web | independent |

## Interface

Owner: contracts. Frozen by `option-data-fields-schema`, read by `option-data-view`.

- `contracts/schemas/page.option_data_page.v1.json`, in place, all optional: `Underlying.vol_line` (string 1..200; its `description` says "metric id: assigned by compute"), `Underlying.earnings_text` (string 1..200), `Underlying.earnings_tone` (enum `in_term`, `outside`, `none`), `Figure.approx_usd_text` (string 1..200). `required` is unchanged.
- The generated type `web/src/generated/option_data_page.v1.ts` is regenerated and committed.

## Issues

### option-data-fields-schema (interface)

- Module: contracts. Size: M. Depends on: none.
- Order: first of this design, and first of the three issues that edit `docs/contracts/page-schemas.md`: option-pages-v22, then tracking-household-reports, then missing-pages, each "after" the previous.
- Design: docs/design/option-pages-v22.md#Interface
- Files: contracts/schemas/page.option_data_page.v1.json, docs/contracts/page-schemas.md, internal/contracts/optiondatapage_test.go, contracts/fixtures/page.option_data_page.v1.jpy.json, contracts/fixtures/MANIFEST.synthetic-v1.json, web/src/generated/option_data_page.v1.ts

- Goal: the three optional strings and the enum of the Interface section (legal by `page-schemas.md:59`, the precedent `underlyings`, no version change); the type regenerated by `pnpm --dir web run generate:types` and committed; one new fixture `jpy` (a JPY underlying whose price, strike and bid carry `approx_usd_text`, a strike `Figure` on every chain row, no quote series, one row whose `bid` is a `Gap` cell with its `reason` and `text` (the gap is in `bid`, never in `strike`),
  `earnings_text` with a vendor date, `earnings_tone` `in_term`, `vol_line` with "HV30 缺"), listed in `MANIFEST.synthetic-v1.json` with the sha256 of the file and checked by `TestOptionDataPageFixtures`. `ok`, `terms`, `gap`, `stale` and `choices` are not edited and stay valid. `page-schemas.md` gets one sentence for the four fields and its fixture list of `page.option_data_page.v1` gains `jpy` (`{input,ok,terms,gap,stale,bad-version,choices,terms-input,jpy}`).

Acceptance:

1. `go test ./internal/contracts -run 'TestOptionDataPageFixtures|TestFixtureManifest|TestGeneratedTypes' -count=1` passes.
2. `TestOptionDataPageFixtures` validates `jpy`; a new case fails an `approx_usd_text` of 201 characters, an `earnings_tone` of `red` and an unknown key in `Underlying`; the five existing fixtures (`ok`, `terms`, `gap`, `stale`, `choices`) still validate; a payload without the four fields stays valid (the existing `ok` case).
3. `git diff $(git merge-base HEAD origin/main) -- contracts/fixtures/page.option_data_page.v1.ok.json contracts/fixtures/page.option_data_page.v1.terms.json contracts/fixtures/page.option_data_page.v1.gap.json contracts/fixtures/page.option_data_page.v1.stale.json contracts/fixtures/page.option_data_page.v1.choices.json` is empty; a case of `TestOptionDataPageFixtures` asserts that `jpy` has no quote series and that every `chain.rows[i].strike` is a Figure and that exactly one row has a `Gap` in `bid` with a non-empty `reason` and `text` (test assertions, not line counts).
4. `grep -c "page.option_data_page.v1.jpy.json" contracts/fixtures/MANIFEST.synthetic-v1.json` prints 1; `grep -c "@" contracts/fixtures/page.option_data_page.v1.jpy.json` prints 0; `grep -c "vol_line" web/src/generated/option_data_page.v1.ts` is at least 1.
5. `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the six paths of the Files line; `git diff $(git merge-base HEAD origin/main) -- internal/pages` is empty; `grep -c 'jpy' docs/contracts/page-schemas.md` is at least 1.

Checked by CI: ci-go; `pnpm install --frozen-lockfile`, then `pnpm --dir web run generate:types` leaves the committed file unchanged (drift check), tsc, eslint, vitest, build; hygiene (fixture manifest, privacy scan); the pull request body carries `Contracts: page.option_data_page.v1 (optional fields added)`; the design verdict (Decision 5).

### option-data-view

- Module: web. Size: M. Depends on: option-data-fields-schema.
- Order: after option-data-fields-schema.
- Design: docs/design/option-pages-v22.md#Decisions
- Files: web/src/pages/optiondata/OptionDataPage.tsx, web/src/pages/optiondata/ChainTable.tsx, web/src/pages/optiondata/QuoteChart.tsx, web/test/pages/optiondata/OptionDataPage.test.tsx

- Goal: the header shows `underlying.vol_line` and `underlying.earnings_text` when present; each header field is an element with `data-field="vol_line"` or `data-field="earnings_text"`; an absent `vol_line` prints "缺" once in its element, and an absent `earnings_text` prints "没有数据来源" once in its element. A price, strike or bid with `approx_usd_text` shows it beside the figure. The chain table header is Chinese for non-term labels (到期, 类型, 行权价, 状态, 买价, 卖价, 成交价, 成交量) and English for the terms (IV, Δ, Γ, Θ, Vega, OI).
  The quote chart caption is `QuoteSeries.caption`; the horizontal axis is `QuotePoint.slot` and the three lines are bid, ask and last (`QuotePoint.bid`, `ask`, `last`), named by shell text (买价, 卖价, 成交价); the payload has no axis-name field and none is added. The tests read the contracts fixtures `terms` and `jpy`; `terms` carries the quote series.
- 按 frontend-stack：用 @tanstack/react-table (`components/data-table/data-table.tsx`) 做 the chain table。
- 按 frontend-stack：用 recharts (`LineChart` through `components/ui/chart.tsx`) 做 the quotes by slot。
- 按 frontend-stack：用 shadcn card and badge (`components/ui`) 做 the header and the earnings label (variant by `earnings_tone`)。

Acceptance:

1. `pnpm --dir web run test -- web/test/pages/optiondata` passes.
2. With `jpy` the test checks that `approx_usd_text` is next to the price, that `earnings_text` and `vol_line` are in the header, that the earnings badge has `data-earnings="in_term"` (from `earnings_tone`, not from the text) and that the `vol_line` element holds the text as written; with `terms`, which carries none of the four fields, it checks that the `vol_line` element contains "缺" exactly once and the `earnings_text` element contains "没有数据来源" exactly once and that the `earnings_text` element has no `data-earnings` attribute, and that every row shows its strike text, that the chart caption is `QuoteSeries.caption` and that its x values are the `slot` values.
3. The test checks that the chain header contains "IV", "Δ" and "Θ" and does not contain "Expiry" or "Strike".
4. `grep -c "toFixed\|Intl.NumberFormat\|parseFloat" web/src/pages/optiondata/*.tsx` prints 0 for each file; `wc -l web/src/pages/optiondata/*.tsx` shows each under 600.
5. `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the four paths of the Files line.

Checked by CI: `pnpm install --frozen-lockfile`, tsc, eslint, vitest, build; `Contracts: unchanged`; the design verdict (Decisions 5 and 6).

### option-page-view

- Module: web. Size: M. Depends on: none.
- Order: independent. The payload stays `option_page` v1.
- Design: docs/design/option-pages-v22.md#Decisions
- Files: web/src/pages/options/OptionPage.tsx, web/test/pages/options/OptionPage.test.tsx, web/fixtures/option-page/ok.json

- Goal: Decisions 1 to 4. The card shows its lines as written; the E tag is a plain secondary Badge with the text as written (Decision 2); the chain is in a Card with no collapse control and a null `chain` shows "没有链数据". `ok.json` stays valid v1 and carries what board 5 prints: the 全部候选 columns 标的, 判定, 年化, strike %, DTE, Δ, Bid/Ask, spread %, Vol, OI, IVP, E;
  the contract line "sell put 62 · 11-13 周五 · 22 天 · Δ −0.15"; the iv line "IV 38% · HV30 31% · IVP 72"; the link "全部 6 个机会"; a JP tab; one card with "IVP 缺", one with the tag "E 11-12 在期内", one with the tag "E 无" and one with "权利金偏高". `gap.json`, `stale.json` and `bad-version.json` are not edited.
- 按 frontend-stack：用 shadcn tabs (`components/ui/tabs.tsx`) 做 the US and JP tabs。
- 按 frontend-stack：用 shadcn card and badge (`components/ui`) 做 the opportunity cards, their verdict and the E tag。
- 按 frontend-stack：用 @tanstack/react-table (`components/data-table/data-table.tsx`) 做 the 全部候选 table and the chain table。
- 按 frontend-stack：用 shadcn card (`components/ui`) 做 the chain that is always present。

Acceptance:

1. `pnpm --dir web run test -- web/test/pages/options/OptionPage.test.tsx` passes; `go test ./internal/contracts -run TestFixturesValidate -count=1` passes (the edited `ok.json` still validates as v1).
2. The test renders `ok` and checks that `contract_line` and `iv_line` of each card are on the page as written, that the "E 11-12 在期内" and "E 无" tags are on their cards as written with no variant chosen from their text, that the "权利金偏高" tag is on its card, and that the "IVP 缺" line is shown without a number.
3. The test checks that the candidate headers are the twelve of the board in that order (read from `ok`), that the link text with the count is on the page, that on first render, with no click, every row of `ok` `chain.rows` is in the DOM and passes `toBeVisible()`, and that a pane with `chain` null shows "没有链数据".
4. `grep -c "toFixed\|Intl.NumberFormat\|parseFloat\|accordion" web/src/pages/options/OptionPage.tsx` prints 0; `wc -l web/src/pages/options/OptionPage.tsx` shows under 600; `grep -c "@" web/fixtures/option-page/ok.json` prints 0.
5. `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the three paths of the Files line.

Checked by CI: `pnpm install --frozen-lockfile`, tsc, eslint, vitest, build; hygiene; `Contracts: unchanged`; the design verdict (Decisions 1 to 4).

### stock-option-view

- Module: web. Size: M. Depends on: none.
- Order: independent. The payload stays `stock_option_page` v1.
- Design: docs/design/option-pages-v22.md#Decisions
- Files: web/src/pages/stockoption/view.tsx, web/src/pages/stockoption/Cards.tsx, web/src/pages/stockoption/Tables.tsx, web/test/pages/stockoption/StockOptionView.test.tsx, web/test/pages/stockoption/fixtures/ok.ts

- Goal: Decisions 1, 2 and 6. The header prints `top.iv` as written ("IV 24.1% · HV30 缺 · IVP 38") and `top.earnings` as a secondary Badge with the text as written. The strike by expiry matrix gets a toggle between `yield_text` and `premium` of each cell (both in `MatrixCell` already; the choice is view state).
  The fixture contracts table carries Θ/天, Γ and Vega cells on every row, and the fixture text is Chinese with the English option terms. `gap.ts`, `stale.ts` and `bad-version.json` are not edited.
- 按 frontend-stack：用 shadcn toggle (`components/ui/toggle.tsx`) 做 the yield or premium switch of the matrix。
- 按 frontend-stack：用 shadcn card, badge and tabs (`components/ui`) 做 the header, the earnings text and the sections。
- 按 frontend-stack：用 @tanstack/react-table (`components/data-table/data-table.tsx`) 做 the 全部合约 table。

Acceptance:

1. `pnpm --dir web run test -- web/test/pages/stockoption` passes.
2. The test checks that `ok.top.iv` is on the page as written, that `top.earnings` is on the page as written and that the view has no branch on its text (`grep -c "startsWith" web/src/pages/stockoption/view.tsx` prints 0).
3. The test checks that each matrix cell shows `yield_text` before the click on the toggle and `premium` after it.
4. The test checks that the contracts table headers include "Θ", "Γ" and "Vega" and that every row of `ok.contracts.rows` shows its cells.
5. `grep -c "toFixed\|Intl.NumberFormat\|parseFloat" web/src/pages/stockoption/*.tsx` prints 0 for each file; `wc -l web/src/pages/stockoption/*.tsx` shows each under 600; `grep -c "@" web/test/pages/stockoption/fixtures/ok.ts` prints 0.
6. `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the five paths of the Files line; `git diff $(git merge-base HEAD origin/main) -- web/test/pages/stockoption/fixtures/MANIFEST.synthetic-v1.json` is empty (it lists names, not hashes).

Checked by CI: `pnpm install --frozen-lockfile`, tsc, eslint, vitest, build; hygiene; `Contracts: unchanged`; the design verdict (Decisions 1, 2, 6).

### full-scan-fixtures

- Module: web. Size: S. Depends on: none.
- Order: independent. Fixture and test only; the file stays valid `full_scan` v1.
- Design: docs/design/option-pages-v22.md#Decisions
- Files: web/fixtures/full-scan/ok.json, web/test/pages/candidates/FullScanPage.test.tsx

- Goal: board 17. The `short_put`, `covered_call` and `put_credit_spread` sections of `ok.json` carry the twelve columns (标的, 判定, 年化, strike %, DTE, Δ, Bid/Ask, spread %, Vol, OI, IVP, E), one row whose quotes are missing shown as "缺" with the cause once, and an E column with the vendor date as text ("E 10-28", "—").
  The covered-call section stays a plain call table.
- 按 frontend-stack：用 @tanstack/react-table (`components/data-table/data-table.tsx`) 做 the three tables (unchanged view)。

Acceptance:

1. `pnpm --dir web run test -- web/test/pages/candidates/FullScanPage.test.tsx` passes; `go test ./internal/contracts -run TestFixturesValidate -count=1` passes.
2. The test checks that the headers of each section are the twelve of the board in that order, that the missing row shows "缺" and its cause text once, and that no section has a collar or roll column.
3. `grep -c "@" web/fixtures/full-scan/ok.json` prints 0; `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the two paths of the Files line.

Checked by CI: `pnpm install --frozen-lockfile`, tsc, eslint, vitest, build; hygiene; `Contracts: unchanged`; the design verdict (Decision 7).

## Left out

- The producers of delta, HV30, IVP and the IV-against-HV30 text: delta and IV are IBKR model values and wait for the broker data path; HV30 and IVP wait for the compute design. Until then the text says "缺".
- A tone field on `Opportunity` and `TopBar` (the red E on option_page and stock_option_page), and a v2 of `option_page` and `stock_option_page`: not now; the text carries the values, and the shape is frozen with the Go builder design.
- The Go builders of `option_page`, `stock_option_page`, `full_scan` and their `ot` routes: each its own design. The 7-day scan lookback (PR1858): scan-side.
- Covered-call extras (calls on shares not held, call spreads, collars, rolling, tax, re-entry): dropped by the owner; the plain call table stays.
- A per-row E column on the option data page: not in board 18's table.
- Board 5's expandable chain matrix: the chain card, open on first render, is the chain. Rejected: a shadcn accordion for the chain, because a collapsible item can hide the chain, against #1799.
- Axis names in the payload of the option data page: not added; the chart uses `QuotePoint.slot` and shell text, so no schema field is needed.

## Adversarial

The review is kept by the author. Answers, by its numbering.

- Verdict 2 and 2.A size rule, top 1: adopted. Neither carried schema is copied; the two v2 files are gone. Verdict 3 and 3.1, 3.2: adopted, text carries delta, DTE, HV30 and IVP; the earnings tone is a field only where the schema can take it in place (`option_data_page`, Decision 2); on the carried files it waits for a v2 (a copy of an 847-line file).
- 1.2 PR1847: adopted as a tag text (Decision 3); the 7-day lookback is stated as not a page matter. 1.4 vendor date (#1789): adopted for the header as `earnings_text`; the per-row column is stated as not designed. 1.1, 1.3, 1.5, 1.6, 1.7: other files.
- 2.B guarded fixture directories, top 2: adopted. The option page and full scan fixtures stay valid v1 and their acceptance runs `TestFixturesValidate`; the stock option fixtures are TypeScript. `web/src/types/option_page.ts` is not edited, so nothing is orphaned.
- 2.C: not applicable. 2.D no route: adopted in the Goal.
- 3.3 merge parallel schema issues: adopted, one schema issue remains, so `page-schemas.md` is edited once. 3.4, 3.5, 3.6: other files.
- 4.3 required `facts` and `chain`: adopted, removed. 4.5: adopted (one in-place edit only). 4.1, 4.2, 4.4: other files.
- 5.5 `placeholderValidate` count 0: adopted, the check is deleted and `option_page.ts` is not touched. 5.6 `generate:types` acceptance: adopted, replaced by `go test ... TestGeneratedTypes` and a drift check in CI; the first run for a new schema would use `generate:types page.<name>.vN`, which does not occur here.
- 5.7 precedent: the precedent is kept (an own page whose builder exists), the field set is three strings. 5.1 to 5.4, 5.8: other files.
- 6.1 offline acceptance: adopted. 6.2 conflicts on `page-schemas.md`: adopted (one edit). 6.3 size rule: adopted. 6.4 fixtures guarded by Go: adopted. 6.5 permanently empty values: adopted, stated in the Goal.
- Per-file check OP: manifest sha256: adopted in `option-data-fields-schema`; the orphaned `option_page.v1.types.ts`: moot, not touched.
- Top 3: n/a here. Top 4: adopted. Top 5: adopted (E tone and PR1847 settled in Decisions 2 and 3).
- Design session verdict on rev 3, all adopted: E scope in the Goal limited to `earnings_tone` on the option data page; the chain is a Card open on first render with a visibility test; axis names dropped from the payload (slot and shell text); the stock fixture manifest lists names only, so it is not edited and a check says so; the `jpy` strike check is a test assertion, not `grep -c`; "E 无" in the option-page fixture list; `page-schemas.md` fixture list gains `jpy`. The `earnings_tone` case of `jpy` and the byte identity of the five existing fixtures are kept.
- Advisor opinion of 2026-11-08 (rev 3): (1) adopted, closed enum `earnings_tone`, no view branch on a text prefix; (2) adopted, five fixtures listed and kept valid; (3) adopted, `page-schemas.md:59` cited as the precedent, carried files untouched. Top 6: adopted (3 option schema issues became 1, no route stated).
