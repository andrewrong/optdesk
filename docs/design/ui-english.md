# UI English: every user-visible text of the pages and the site is English first

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-11
Revision: 2

Source: owner decision 2026-11-11, ask optdesk-2529, the owner's words "英文为主吧" (the user interface is English-first); window decisions the same day: a bilingual text keeps only its English half, weekdays are `Sun`..`Sat`, a missing cell is `n/a`. Read on origin/main (12e7241b): `AGENTS.md`, `docs/design/modules.md`, `docs/contracts/00-protocol.md`, `internal/pages/**`, `internal/compute/pagefig/**`, `cmd/ot/**`, `web/src/**`, `web/fixtures/**`, `web/test/**`, `contracts/schemas/**`, `contracts/fixtures/**`, `internal/contracts/**`.

Goal: a page, a card, a gap and an empty state read in English. Today they mix Chinese and English ("家庭资产", "这个账户还没有采集", "期权账本暂无记录", "松紧度暂无数据", "隐含波动率暂无数据：还没有存储"), so one screen speaks two languages. After this design the greps of the issues below print nothing: no CJK character and no full-width punctuation is left in a file an issue owns, except the frozen values named in Left out.

Method: published schemas are not edited. `docs/contracts/00-protocol.md:3` freezes a version at its first consumer and an incompatible change bumps the version, and a changed `const` or enum value is incompatible. So the page builders emit English wherever a schema takes free text (almost every label, title, sentence and gap text), and the few schema values that are Chinese are left as published and listed in Left out with the reason. No schema, no `carryover.sha256` line and no generated file changes.

## What main holds today

Every count is a command run on origin/main (12e7241b); `C` is `[\x{4e00}-\x{9fff}]` (the ideographs; the acceptance below also catches `\x{3000}-\x{303f}\x{ff00}-\x{ffef}`, the full-width punctuation). Line counts are `git grep -h -cP C -- <paths>` summed, files are the same command's rows.

| module (modules.md row) | path (command pathspec) | lines | files |
|---|---|---|---|
| pages, source | `internal/pages ':!internal/pages/**/*_test.go' ':!internal/pages/**/testdata/**'` | 669 | 53 |
| pages, tests | `internal/pages/**/*_test.go` | 218 | 40 |
| pages, golden and input files | `internal/pages/**/testdata/**` | 845 | 30 |
| compute | `internal/compute` (`pagefig/household.go` 24, `household_test.go` 6, `company_test.go` 4) | 34 | 3 |
| ot (`cmd/**`) | `cmd` | 7 | 6 |
| web, source | `web/src ':!web/src/generated' ':!web/src/types'` | 150 | 34 |
| web, entry page | `web/index.html` | 2 | 1 |
| web, tests | `web/test` | 1384 | 85 |
| web, fixtures | `web/fixtures` (86 tracked files in all, 84 hold CJK) | 3217 | 84 |
| contracts, fixtures | `contracts/fixtures` (`page.options_summary.v4.{ok,gap,stale}.json`, `page.option_data_page.v1.jpy.json`) | 65 | 4 |
| contracts, Go tests | `internal/contracts` | 19 | 9 |
| contracts, schemas (not edited) | `contracts/schemas` | 70 | 17 |
| contracts, generated and hand types (not edited) | `web/src/generated` 20 lines in 9 files; `web/src/types` 36 lines in 10 files | 56 | 19 |
| jobs, sources (data, not edited) | `internal/jobs internal/sources` | 5 | 2 |

`git grep -lP C -- . ':!docs' ':!*.md' | wc -l` prints 377 (all rows above). `git grep -lP C -- docs | wc -l` prints 49. `internal/notify`, `internal/runner`, `internal/store`, `pyside` hold none: no Telegram text is Chinese on main.

Pages packages that hold CJK, files with CJK each (`git grep -lP C -- internal/pages | awk -F/ '{print $3}' | sort | uniq -c`, 128 files with the wide class): `option_boards` 28, `structures_page` 14, `home_page` 11, `option_result_page` 8, `risks_block` 7, `household_page` 6, `option_candidates_page` 6, `option_tracking_page` 5, `report_page` 5, `macro_page` 4, `option_data_page` 4, `positions_page` 4, `risk_page` 4, `home_household` 3, `home_market` 3, `home_options` 3, `market_page` 3, `option_notices_page` 3, `report_blocks` 3, `advice_page` 2, `home_macro` 2. `companies_page` is already English (`internal/pages/companies_page/builder.go:20`, `noMoatText = "moat record not entered"`): the English half of this design is partly done.

Where a text lives:

- Go page builders: `internal/pages/home_market/strings.go:5-10` (`cardTitle = "市场"`), `internal/pages/household_page/strings.go:5` (`pageTitle = "家庭资产"`), `internal/pages/home_page/overview.go:94` (`"今天没有缺口"`, inline, not in a strings file), `internal/pages/option_boards/eodtext.go:11` (`gapText = "缺"`), `internal/pages/option_boards/decimal.go:24` (`weekdayNames`, `周日`..`周六`).
- Full-width punctuation in code, no ideograph on the line: `internal/pages/home_household/member.go:206,241`, `home_macro/member.go:165`, `home_market/member.go:339`, `home_options/member.go:287,357`, `household_page/blocks.go:253`, `builder.go:244`, `holdings.go:102,121` (`"："`, `"；"`, `"，"`, `"。"`, `"、"` joined into a text). The wide class catches them.
- Figures: `internal/compute/pagefig/household.go:128-136` (`净值`, `净清算值`).
- Site: `cmd/ot/home_site.go:108` (`今日总览`), `cmd/ot/site_positions.go:25-26` (`未知账户`, `演示数据中没有 Flex 持仓`), `cmd/ot/wire.go:365` (`10年期美债收益率`).
- Web: `web/src/shell/TopNav.tsx:13-31` (14 menu labels), `web/src/shell/SampleBanner.tsx:9` (`示例数据，非实时数据 / Sample data, not live data`), `web/index.html:15,17` (`page did not load, do not act<br />页面未加载,请勿据此操作`), `web/src/pages/**` (10 files under `home`, 3 under `optiondata`).
- Golden files that pin the texts: `internal/pages/**/testdata/*.json` (30 files), `cmd/ot/testdata/macro_page_demo.golden.json`; regenerated with `-update` (`cmd/ot/page_test.go`, `internal/pages/option_boards/stock_page_test.go`, `positions_page/builder_test.go`, `risk_page/builder_test.go`) or `UPDATE_GOLDEN=1` (`internal/pages/home_household/member_test.go`, `internal/pages/household_page/holdings_test.go`).
- Designs that prescribe exact Chinese texts: 49 files under `docs/`.

## Decisions

1. English first: every text a user reads in a page payload, a figure label, a site title, a web component, an error or empty state and the fallback notice of `web/index.html` is English. A bilingual text keeps only the English half (`Sample data, not live data`, `page did not load, do not act`). No language switch, no locale files. A text stays where it lives (a `strings.go`, an inline literal, a component); only its language changes.
2. Tone: short, plain, sentence case, no full stop on a label, a full stop on a sentence; an empty state is `<Subject>: <state>` ("Implied volatility: no data yet"), a gap sentence says what is missing and why ("no data: not stored yet"). ASCII punctuation only (`,` `:` `;` `.`); the Chinese `，` `：` `；` `。` `、` and full-width parentheses are not used in code that builds a text.
3. Glossary, one English word per term, used by every issue. A term not listed is translated plainly and added to the table in the pull request description; a reviewer compares the two tables.

| 中文 | English | | 中文 | English |
|---|---|---|---|---|
| 家庭资产 | Household | | 缺口 | Gap |
| 首页 | Home | | 缺 (a missing cell) | n/a |
| 今日总览 | Today | | 净值 | Net value |
| 宏观 | Macro | | 净清算值 | Net liquidation |
| 持仓 | Positions | | 隐含波动率 | Implied volatility (IV) |
| 风险 | Risk | | 松紧度 | Ease |
| 建议 | Advice | | 财报日期 / 财报日 | Earnings date |
| 设置 | Settings | | 护城河 | Moat |
| 期权 | Options | | 我 / 配偶 / 家庭 | Me / Spouse / Household |
| 期权数据 | Option data | | 市场 | Market |
| 期权候选 | Option candidates | | 收盘 / 盘中 | Close / Intraday |
| 期权账本 | Option ledger | | 示例数据 | Sample data |
| 结构 / 盈亏 / 提示 / 追踪 | Structures / Result / Notices / Tracking | | 周日..周六 | Sun..Sat |
| 暂缺 / 暂无数据 | no data yet | | 数据日 | Data day |
| 账户 | Account | | 行权价 | Strike |
| 到期 / 到期日 | Expiry | | 权利金 | Premium |
| 盈亏平衡 | Breakeven | | 年化 | Annualised |
| 标的 | Underlying | | 合约 | Contract |
| 最大盈利 / 最大亏损 | Max profit / Max loss | | 现价 | Last price |
| 买价 / 卖价 | Bid / Ask | | 成交量 | Volume |

The table is built from the most frequent Chinese words of the source files (`git grep -hoP '[\x{4e00}-\x{9fff}]{2,}' -- internal/pages ':!*_test.go' ':!*testdata*' web/src ':!web/src/generated' ':!web/src/types' cmd internal/compute | sort | uniq -c | sort -rn| head -70`), the nav labels of `web/src/shell/TopNav.tsx` and the titles of the page builders.

4. Number, date and time formats are unchanged (`%d`, `+$310`, `2026-06-29`, ET and JST stamps). Only a weekday name and a text around a number change (`近 7 天` becomes `last 7 days`).
5. Data values stay as the source gives them. On main no file these issues own holds such a value: the Japanese filing names (`第1四半期`) are in `internal/sources/jquants/testdata/earnings_date.json`, which no issue edits. A fixture or a test that carries a company name or a record is synthetic and becomes English.
6. Schemas are not edited (Method). The builders emit English into free text; the three frozen value sets that stay Chinese (market_weather v1 labels, positions_page v1 and v2 enums, the `default` of report_page) are in Left out with the reason and the way they are skipped by a grep.
7. Designs that prescribe Chinese texts are superseded by this file for text only: every exact Chinese string in `docs/design/*.md` reads as its English form by decision 3; their structure, fields and rules stay in force. The old files are not edited (`docs/design` is owner-only).
8. A text and the golden files, fixtures and tests that pin it change in the same issue; the worker regenerates the golden files (`-update` or `UPDATE_GOLDEN=1`) and commits them. An issue that changes a text without its pinned files is not done. The glossary guard of each issue (acceptance) names the glossary words that must be in the regenerated golden files, so a golden file is read, not only blessed.
9. No new test sleeps, no shared state: only expected strings change.

## Modules

None added. Rows touched: contracts (fixtures and Go tests only), compute, pages, web, ot. Row depends cells are unchanged. One issue stays in one row: Files of the web issue name `web/src/components/**`, `web/src/pages/**` and `web/src/shell/**`, never `web/src/generated/**` (contracts row, listed first in `modules.md`) nor `web/src/types/**` (not edited).

## Interface

No Go, Python or TypeScript declaration changes, no schema, no generated output: `Contracts: unchanged` in every pull request. No issue is `(interface)`: the interface files (`internal/contracts/api.go`, `internal/pages/api.go`, `web/src/generated/*.ts`, `contracts/schemas/**`) hold no change.

## Issues

Every `Files:` is one module. Order: issues 1, 2 and 8 touch separate paths and run first, together; 3 to 6 follow 2 (the household labels come from `pagefig`) and may run together; 7 (ot) follows 3 to 6 because `cmd/ot/testdata/macro_page_demo.golden.json` is built from the pages. Each grep below spells the class out: ideographs `\x{4e00}-\x{9fff}` and full-width punctuation `\x{3000}-\x{303f}\x{ff00}-\x{ffef}`.

### 1 ui-english-contracts-fixtures

Module: contracts. Size: S.
Files: `contracts/fixtures/page.options_summary.v4.ok.json`, `contracts/fixtures/page.options_summary.v4.gap.json`, `contracts/fixtures/page.options_summary.v4.stale.json`, `contracts/fixtures/page.option_data_page.v1.jpy.json`, `contracts/fixtures/MANIFEST.synthetic-v1.json`, `internal/contracts/held_options_v3v4_test.go`, `internal/contracts/indicator_detail_v2_test.go`, `internal/contracts/market_page_v1_test.go`, `internal/contracts/option_candidates_page_v1_test.go`, `internal/contracts/optiondatapage_test.go`, `internal/contracts/pagev3_test.go`, `internal/contracts/positions_page_v3_test.go`, `internal/contracts/settings_page_v2_test.go`, `internal/contracts/tracking_v2_test.go`
Order: first, with 2 and 8.
Design: docs/design/ui-english.md#Decisions
Goal: the four contract fixtures and the Go tests of `internal/contracts` carry English texts; the sha256 lines of the manifest follow the files. No schema changes.
Acceptance:
1. `git grep -nP '[\x{4e00}-\x{9fff}\x{3000}-\x{303f}\x{ff00}-\x{ffef}]' -- contracts/fixtures internal/contracts` prints nothing.
2. `git diff --name-only origin/main...HEAD -- contracts/fixtures/MANIFEST.synthetic-v1.json | wc -l` prints 1 (the manifest changes with the four fixtures; the sha256 values are checked by the hygiene check of CI, below).
3. `git grep -h -c '"n/a"\|HV30 n/a' -- contracts/fixtures/page.option_data_page.v1.jpy.json` prints 1 (the glossary cell `缺` is `n/a`) and `git grep -h -c 'Sample data' -- contracts/fixtures/page.options_summary.v4.ok.json` prints 1.
4. `git diff --name-only origin/main...HEAD | grep -vcE '^(contracts/fixtures/(page\.options_summary\.v4\.(ok|gap|stale)|page\.option_data_page\.v1\.jpy|MANIFEST\.synthetic-v1)\.json|internal/contracts/[a-z0-9_]+_test\.go)$'` prints 0, and `git diff --name-only origin/main...HEAD -- contracts/schemas web/src/generated | wc -l` prints 0 (design docs/design/ui-english.md).
Checked by CI: `go test ./internal/contracts/... -count=1` and `go run ./scripts/hygiene`, which compares each fixture with its sha256 line of `contracts/fixtures/MANIFEST.synthetic-v1.json`.

### 2 ui-english-compute

Module: compute. Size: S.
Files: `internal/compute/pagefig/household.go`, `internal/compute/pagefig/household_test.go`, `internal/compute/pagefig/company_test.go`
Order: first, with 1 and 8.
Design: docs/design/ui-english.md#Decisions
Goal: figure labels and the tests that pin them are English.
Acceptance:
1. `git grep -nP '[\x{4e00}-\x{9fff}\x{3000}-\x{303f}\x{ff00}-\x{ffef}]' -- internal/compute` prints nothing.
2. Glossary guard: `git grep -h -cE '"Net value"|"Net liquidation"' -- internal/compute/pagefig/household.go` prints at least 3, and `git grep -nE '"(Net worth|Equity|NLV)"' -- internal/compute` prints nothing.
3. `git diff --name-only origin/main...HEAD | grep -vc '^internal/compute/pagefig/\(household\|household_test\|company_test\)\.go$'` prints 0 (design docs/design/ui-english.md).
Checked by CI: `go test ./internal/compute/... -count=1`.

### 3 ui-english-pages-home

Module: pages. Size: M.
Files: `internal/pages/home_page/**`, `internal/pages/home_household/**`, `internal/pages/home_macro/**`, `internal/pages/home_market/**`, `internal/pages/home_options/**`
Order: after 2 (the home household member uses the figure labels).
Design: docs/design/ui-english.md#Decisions
Goal: the five home members, their golden and input files and tests are English; the inline texts (`overview.go:94`) too.
Acceptance:
1. `git grep -nP '[\x{4e00}-\x{9fff}\x{3000}-\x{303f}\x{ff00}-\x{ffef}]' -- internal/pages/home_page internal/pages/home_household internal/pages/home_macro internal/pages/home_market internal/pages/home_options` prints nothing.
2. Glossary guard: `git grep -h -cP 'cardTitle\s+= "Market"' -- internal/pages/home_market/strings.go` prints 1; `git grep -h -c '"Market"' -- internal/pages/home_market/testdata/ok.golden.json` prints at least 1 (the golden file was regenerated, not edited around); `git grep -h -cF 'Card: "Home"' -- internal/pages/home_page/overview.go` prints 1; `git grep -nP '"(Family|Home page|Overview of the day)"' -- internal/pages/home_page internal/pages/home_household internal/pages/home_macro internal/pages/home_market internal/pages/home_options` prints nothing.
3. `git grep -nP '\x{ff1a}|\x{ff1b}|\x{ff0c}|\x{3002}' -- internal/pages/home_household internal/pages/home_macro internal/pages/home_market internal/pages/home_options` prints nothing (the joins of `member.go` use `: ` `; ` `, ` `. `).
4. `git diff --name-only origin/main...HEAD | grep -vc '^internal/pages/home_\(page\|household\|macro\|market\|options\)/'` prints 0 (design docs/design/ui-english.md).
Checked by CI: `go test ./internal/pages/... -count=1` (golden suites).

### 4 ui-english-pages-option-boards

Module: pages. Size: M.
Files: `internal/pages/option_boards/**`
Order: after 2.
Design: docs/design/ui-english.md#Decisions
Goal: the option and stock option boards, their 28 files with CJK, are English; weekdays are `Sun`..`Sat`, a missing cell is `n/a`.
Acceptance:
1. `git grep -nP '[\x{4e00}-\x{9fff}\x{3000}-\x{303f}\x{ff00}-\x{ffef}]' -- internal/pages/option_boards` prints nothing.
2. Glossary guard: `git grep -h -cF 'const gapText = "n/a"' -- internal/pages/option_boards/eodtext.go` prints 1; `git grep -h -cF '{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}' -- internal/pages/option_boards/decimal.go` prints 1; `git grep -h -c '"n/a"' -- internal/pages/option_boards/testdata/option_page_ok.golden.json` prints at least 1; `git grep -nP '"(N/A|NA|-|--|missing)"' -- internal/pages/option_boards` prints nothing.
3. `git diff --name-only origin/main...HEAD | grep -vc '^internal/pages/option_boards/'` prints 0 (design docs/design/ui-english.md).
Checked by CI: `go test ./internal/pages/option_boards/... -count=1`.

### 5 ui-english-pages-option-pages

Module: pages. Size: M.
Files: `internal/pages/option_candidates_page/**`, `internal/pages/option_data_page/**`, `internal/pages/option_notices_page/**`, `internal/pages/option_result_page/**`, `internal/pages/option_tracking_page/**`, `internal/pages/structures_page/**`
Order: after 2.
Design: docs/design/ui-english.md#Decisions
Goal: the six option pages are English.
Acceptance:
1. `git grep -nP '[\x{4e00}-\x{9fff}\x{3000}-\x{303f}\x{ff00}-\x{ffef}]' -- internal/pages/option_candidates_page internal/pages/option_data_page internal/pages/option_notices_page internal/pages/option_result_page internal/pages/option_tracking_page internal/pages/structures_page` prints nothing.
2. Glossary guard: `git grep -h -c 'Breakeven' -- internal/pages/structures_page/detail.go` prints at least 3 (盈亏平衡 appears 3 times today); `git grep -h -c 'Premium' -- internal/pages/option_tracking_page/tables.go` prints at least 2; `git grep -nP '"[^"]*(Exercise price|Execution price|Cost of option|Profit and loss)' -- internal/pages/option_candidates_page internal/pages/option_data_page internal/pages/option_notices_page internal/pages/option_result_page internal/pages/option_tracking_page internal/pages/structures_page` prints nothing; the pull request description lists the terms added to the glossary.
3. `git diff --name-only origin/main...HEAD | grep -vc '^internal/pages/\(option_candidates_page\|option_data_page\|option_notices_page\|option_result_page\|option_tracking_page\|structures_page\)/'` prints 0 (design docs/design/ui-english.md).
Checked by CI: `go test ./internal/pages/... -count=1`.

### 6 ui-english-pages-rest

Module: pages. Size: M.
Files: `internal/pages/advice_page/**`, `internal/pages/household_page/**`, `internal/pages/macro_page/**`, `internal/pages/market_page/**`, `internal/pages/positions_page/**`, `internal/pages/report_blocks/**`, `internal/pages/report_page/**`, `internal/pages/risk_page/**`, `internal/pages/risks_block/**`
Order: after 2.
Design: docs/design/ui-english.md#Decisions
Goal: the nine other page builders are English, including the default sentences of `report_page` (`builder.go`) that its schema shows as `"default": "没有缺口。"`: the builder writes the English sentence, the schema is not touched.
Acceptance:
1. `git grep -nP '[\x{4e00}-\x{9fff}\x{3000}-\x{303f}\x{ff00}-\x{ffef}]' -- internal/pages/advice_page internal/pages/household_page internal/pages/macro_page internal/pages/market_page internal/pages/positions_page internal/pages/report_blocks internal/pages/report_page internal/pages/risk_page internal/pages/risks_block` prints nothing.
2. Glossary guard: `git grep -h -cP 'pageTitle\s+= "Household"' -- internal/pages/household_page/strings.go` prints 1; `git grep -h -c '"Household"' -- internal/pages/household_page/testdata/holdings.golden.json` prints at least 1; `git grep -h -cP 'pageTitle\s+= "Positions"' -- internal/pages/positions_page/strings.go` prints 1; `git grep -nP '"(Family assets|Holdings page)"' -- internal/pages/household_page internal/pages/positions_page` prints nothing.
3. `git diff --name-only origin/main...HEAD -- contracts internal/contracts web/src/generated | wc -l` prints 0 (the schemas stay as published) and `git diff --name-only origin/main...HEAD | grep -vc '^internal/pages/\(advice_page\|household_page\|macro_page\|market_page\|positions_page\|report_blocks\|report_page\|risk_page\|risks_block\)/'` prints 0 (design docs/design/ui-english.md).
Checked by CI: `go test ./internal/pages/... -count=1`, the `report_page` schema validation of the builder output.

### 7 ui-english-ot

Module: ot. Size: S.
Files: `cmd/ot/home_site.go`, `cmd/ot/site_positions.go`, `cmd/ot/wire.go`, `cmd/ot/site_positions_test.go`, `cmd/ot/stock_option_site_test.go`, `cmd/ot/testdata/macro_page_demo.golden.json`
Order: after 2 to 6 (the macro golden file is a page builder output).
Design: docs/design/ui-english.md#Decisions
Goal: the site titles, the unknown-account text, the demo series name and the macro demo golden file are English.
Acceptance:
1. `git grep -nP '[\x{4e00}-\x{9fff}\x{3000}-\x{303f}\x{ff00}-\x{ffef}]' -- cmd` prints nothing.
2. Glossary guard: `git grep -h -cF 'Title: "Today"' -- cmd/ot/home_site.go` prints 1; `git grep -o 'Macro\|Ease' -- cmd/ot/testdata/macro_page_demo.golden.json | wc -l` prints at least 2 (宏观 and 松紧度 of the regenerated file); `git grep -nP '"(Home overview|Unknown acct)"' -- cmd` prints nothing.
3. `git diff --name-only origin/main...HEAD | grep -vc '^cmd/ot/\(home_site\|site_positions\|wire\|site_positions_test\|stock_option_site_test\)\.go$\|^cmd/ot/testdata/macro_page_demo\.golden\.json$'` prints 0 (design docs/design/ui-english.md).
Checked by CI: `go test ./cmd/... -count=1`.

### 8 ui-english-web

Module: web. Size: L (one issue, so `tsc`, `vitest` and the build are green at every merge: the sources, the tests and the fixtures that pin them change together).
Files: `web/src/components/**`, `web/src/pages/**`, `web/src/shell/**`, `web/index.html`, `web/test/**`, `web/fixtures/**`
Order: first, with 1 and 2 (it reads no Go output; the fixtures are synthetic).
Design: docs/design/ui-english.md#Decisions
Goal: every menu label, card title, table header, empty state and notice of the web front end, the fallback of `web/index.html`, and the tests and fixtures that pin them, are English. `web/src/types/**` and `web/src/generated/**` are not edited (Left out).
Note: the only exception of the greps below is a line holding, as a whole quoted value, one of the four published `market_weather.v1` values `收盘`, `盘中试算`, `盘中`, `盘中·延迟` (the negative look-ahead of grep 1).
Acceptance:
1. `git grep -nP '^(?!.*["\x27](收盘|盘中试算|盘中|盘中·延迟)["\x27]).*[\x{4e00}-\x{9fff}\x{3000}-\x{303f}\x{ff00}-\x{ffef}]' -- web/src/components web/src/pages web/src/shell web/index.html web/test web/fixtures ':!*.md' ':!web/test/pages/positions/PositionsPage.test.tsx'` prints nothing.
2. `git grep -nP '^(?!.*["\x27](我|配偶|家庭)["\x27]).*[\x{4e00}-\x{9fff}\x{3000}-\x{303f}\x{ff00}-\x{ffef}]' -- web/test/pages/positions/PositionsPage.test.tsx` prints nothing (the one test file that may name the published `positions_page` v1 and v2 enum values `我`, `配偶`, `家庭`; if the worker needs none, the grep still prints nothing).
3. Glossary guard: `git grep -h -cP "label: '(Home|Macro|Household|Positions|Risk|Options|Option data|Option candidates|Advice|Settings|Structures|Result|Notices|Tracking)'" -- web/src/shell/TopNav.tsx` prints 14; `git grep -h -c 'Sample data, not live data' -- web/src/shell/SampleBanner.tsx` prints 1; `git grep -h -c 'page did not load, do not act' -- web/index.html` prints 2; `git grep -h -c '"Household"' -- web/fixtures/household-page/ok.json` prints 1.
4. `git diff --name-only origin/main...HEAD | grep -vc '^web/\(src/\(components\|pages\|shell\)/\|index\.html$\|test/\|fixtures/\)'` prints 0 (design docs/design/ui-english.md).
5. `git diff --name-only --diff-filter=AD origin/main...HEAD | wc -l` prints 0 (no file is added or removed, so the fixture manifest stays valid).
Checked by CI: `tsc`, `eslint`, `vitest` and the web build.

## Checked by CI

`go test ./...`, `ci-go`, `tsc`, `eslint`, `vitest`, the web build and the hygiene check of the fixture manifests, on every pull request of the issues above.

## Left out

- Published schema values, frozen at their first consumer (`docs/contracts/00-protocol.md:3`; `contracts/schemas/carryover.sha256` pins `page.market_weather.v1.json`, `page.report_page.v1.json`, `page.report_page.v2.json` and `internal/contracts/carryover_test.go` checks it):
  - `page.market_weather.v1.json:13,51,129-130`: `const "收盘"`, `const "盘中试算"`, enum `盘中`, `盘中·延迟`; no Go producer on main (`git grep -n market_weather -- internal cmd ':!internal/contracts'` finds no builder), consumers are `web/src/pages/home/WeatherCard.tsx`, `WeatherIntraday.tsx` and the hand-written `web/src/types/market_weather.v1.types.ts:14,17,28`. They stay until a producer exists; then a `market_weather` v2 carries English values in its own issue. The web grep skips only those four quoted values.
  - `page.positions_page.v1.json:343-345,422-423,430-431` and `v2.json:552-554,631-632,639-640`: enum `我`, `配偶`, `家庭`. No builder produces v1 or v2 (`positions_page` builds v3, whose role cell is an account label); `web/src/types/positions_page.v1.types.ts` and `v2.types.ts` mirror them.
  - `"default": "没有缺口。"` in `page.report_page.v1.json` (5 lines) and `v2.json` (11 lines, from line 24): the builder writes every sentence itself, so the default only documents the schema.
- `web/src/types/**` (hand-written types, 36 lines in 10 files, comments and the frozen values above) and `web/src/generated/**` (20 lines in 9 files, comments copied from schema descriptions): not edited; `web/src/generated` is the contracts row.
- The other Chinese in `contracts/schemas` (about 55 lines) is `description` text, a comment of the schema.
- Data values: `internal/sources/jquants/testdata/earnings_date.json` (Japanese filing names), `internal/jobs/optionledger/testdata/attrs_hash.json` (`naïve 期权 put`, a hash input that must not change).
- Telegram and notify messages: none is Chinese on main; if one is added, English first applies by decision 1.
- Chinese in `docs/**`, `*.md`, ADRs and commit messages: unchanged; the old designs are superseded for text by decision 7, not edited.
- A language switch or locale files: not wanted (English first, not English only); the inline texts are not moved into locale files, so a second language later repeats this change. The owner chose English first; no hedge is added.
- Changing a number, a date or a currency format.

## Adversarial

An adversarial review of revision 1 (full answer: `adv-ui-answer.md` in the scratchpad of the designer session 90b6cbb3-4175-4778-86fc-beec4cdae79d (`.../90b6cbb3-4175-4778-86fc-beec4cdae79d/scratchpad/adv-ui-answer.md`)) raised six points and two settled facts. Settled facts: a published schema is not edited in place (adopted, it is the Method), and a Files line with `except` prose fails `crew issue check` (adopted: no prose, globs of the web row only).

1. Gap against the goal: adopted. The `prints only comment lines` and `data that mirrors a source` exceptions are gone: comments in the owned files are translated too, and the one exception is a quoted frozen value, a grep pattern, or a named file. The glossary grew from 16 to about 40 terms (decision 3) with every nav label, and each issue has a glossary guard that names golden-file terms. Not adopted: scope for `pyside` and `docs`-served text (none holds CJK; counted above).
2. Hash pins: adopted. No schema is edited, so `carryover.sha256` is untouched; issue 1 lists `MANIFEST.synthetic-v1.json` and acceptance 2 checks its four sha256 lines by `shasum`. The web fixtures are synthetic and have a manifest of names only (no hashes), unchanged.
3. Simpler way: adopted. Published schemas are not touched; the builders emit English; issue 1 shrank to four fixtures, the manifest and nine tests. Issues 1 and 2 are not merged (different rows, `Files:` must stay in one row).
4. Lock-in: not adopted as a hedge, the false claim is corrected: texts are partly inline (`overview.go:94`), so a second language later would touch the same files again; the owner chose English first, so no locale seam is built.
5. False counts: adopted. Every count is a command rerun on origin/main (12e7241b); `web/test` is 1384 lines in 85 files, `web/fixtures` 3217 lines in 84 files, `home` is 21 files (the `19` is gone), `internal/contracts/gen` holds dataset records only, no page output, and no market_weather builder exists: neither is claimed as a generated output.
6. Likeliest slip: adopted. (a) no schema, no carryover; manifest in Files. (b) `Files:` lists globs of the web row only. (c) The hand-written market_weather types are not edited, so `tsc` cannot go red. (d) Web sources, tests and fixtures are one issue (8). (e) Not adopted as a split: the web issue stays one large issue to keep CI green; the glossary guard and a regenerated-golden check reduce the risk of blessed wrong strings. (f) Acceptance items are commands with an exact output (`prints nothing`, `prints N`), and each issue ends with a scope check and the design file name.
