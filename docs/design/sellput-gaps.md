# sellput-gaps: the candidates tiers grouped by company, and earnings dates that are not forgotten

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-25
Revision: 2

Source: audit `sellput-audit-2026-11-25.md` items 3 and 5. Items 2 (JP reasons) is in `sellput-jp-outcomes.md`; items 6 (structure on /positions) and 8 (cash notice) are taken out, see `## Left out`. Owner rules honoured: the page computes nothing; gaps stay gaps with a reason, never an invented value; Chinese gloss next to terms; no new outside service or account; a source already in the repo before new code.
Read on origin/main (87f03903): `AGENTS.md`, `docs/design/modules.md`, `candidates-tiers.md`, `internal/jobs/optioncandidates/input.go`, `internal/pages/option_candidates_page/tiers.go` (`companiesOf`, `splitIdle`), `contracts/schemas/page.option_candidates_page.v1.json`, the live page of 2026-11-25, `candidates.option_candidate`, `obs.earnings_check`, and the public Alpha Vantage demo-key example (below).

## Goal

The grey tiers (2386 rows) stop being a wall: every tier is listed company by company, folded. A company with an earnings date once seen keeps it; a company with none is said to have "no date from the source".

## Today (measured)

- Earnings, which side drops the date (probe 2026-11-25, `function=EARNINGS_CALENDAR&horizon=3month&apikey=demo`): the bulk file has 671 rows from 2026-11-24 to 2027-02-22 (Dec 267, Jan 144, Feb 116). It has COST (2026-12-10) and does not have AAPL, MSFT, NVDA, GOOG, AMZN, META or WMT, whose reports fall inside that window. So Alpha Vantage's bulk file is missing them; our parse and match are not dropping them. The per-symbol form (`symbol=IBM`) answers a demo key with a notice body ("I,n,f,o,..." when read as CSV), so a per-symbol lookup cannot be checked without the owner's key and is out of this design. The store holds 88 `listed` and 367 `absent` Alpha Vantage rows; 4 of 33 US names on the run carry a date.
- Kept date: `internal/jobs/optioncandidates/input.go:179-189` takes the newest row only; an `absent` row of today hides a `listed` date of an earlier day that is still in the future.
- Tier wall: tiers of the live run are fits 0, check 285, `grey_no_buy_price` 407, `grey_in_the_money` 347, `grey_no_bid` 291, `grey_above_buy_price` 1341. The rows of a tier are one flat list.
- The strip shows 24 of the 33 US names on purpose: `splitIdle` (`internal/pages/option_candidates_page/tiers.go:450`) keeps only companies with a buy price or a valuation figure and writes the other 9 into the one line `idle_note` ("9 家公司没有买入价和估值档案：BABA、OXY、PDD、MP、NET、NOW、BE、INTU、LMND"). So the 24 shown all have a buy price, the 9 are named in the line, and their 407 rows are the tier `grey_no_buy_price`. Nothing is hidden; it is one line, not a list.

## Decisions

1. Every tier (fits, check and each grey tier) lists its rows grouped by company: one folded line per company with its count ("AAPL 125"), the rows open under it. The grey tier `grey_no_buy_price` grouped by company is the owner's "names without a buy price as their own short list"; no second structure is made for it. The tier counts and the sum of all rows are unchanged ("no row is missing", `candidates-tiers.md`).
2. One schema addition, `Tier.companies` (optional, array of {`company` Text, `count` integer, `row_ids` array of string}), in `page.option_candidates_page.v1`. No other type. The text of `idle_note` stays and gains the pointer "（它们的行在“没有买入价”一组，按公司折叠）".
3. Earnings: nothing is fetched afresh. The next date is the newest `listed` row with a report date not before the chain day, even when a newer row is `absent` or `error`; an owner override still wins (`input.go`). Reason: a bulk calendar that stops listing a company has not cancelled the report, and the source misses large caps, so a date seen once is worth more than the newest silence.
4. The strip's earnings text: a date, or "来源暂无日期（未查到，不等于没有）". The page never prints "3个月内无财报" because the bulk file cannot tell "none within 3 months" from "not in the file" (the probe above). The old "未知" is replaced by this text.
5. The 21 of 24 names with no date stay without one: no source in the repo has them (Alpha Vantage bulk lacks them; J-Quants is JP only). Which other source to use is the owner's choice and is under `## Left out`.

## Modules

Rows used: contracts, jobs, pages, web. No row of `modules.md` is added or changed; no migration; `internal/sources/api.go` and the other entry files are not edited.

## Interface

- `page.option_candidates_page.v1` gains the optional `Tier.companies` (Decision 2); additive, `schema_version` stays 1. Owner: contracts (issue 1). Generated Go and TypeScript come from the schema, never by hand.
- Nothing else is shared: the kept date is inside `optioncandidates` and the earnings words are the page's own.

## Issues

All entries: `Design: docs/design/sellput-gaps.md#<decision>`. Issues 2, 3 and 4 are independent of each other and build in parallel after issue 1; issue 3 does not use the new field and needs no wait.

### 1 tier-companies-contract (interface)

Module: contracts. Size: S. Order: first. Contracts: `page.option_candidates_page v1 additive`. Design: Decisions 1, 2.
Files: `contracts/schemas/page.option_candidates_page.v1.json`, `contracts/fixtures/MANIFEST.synthetic-v1.json`, `docs/contracts/page-schemas.md`, `internal/contracts/option_candidates_page_v1_test.go`, `internal/contracts/gen/records.go`, `web/src/generated/option_candidates_page.v1.ts`
- The schema's `Tier` has the optional property `companies` (items with `company`, `count`, `row_ids`); it is in no `required`, and an item without `count` fails (tests `TestTierCompaniesValid` (new), `TestTierCompaniesNeedsCount` (new)).
- A document without `companies` still validates (test `TestNoTierCompaniesStillValid` (new)); `schema_version` is 1.
- `web/src/generated/option_candidates_page.v1.ts` contains `companies` (grep); the generated Go and TypeScript are produced from the schema.

### 2 tier-companies-page

Module: pages. Size: M. Order: after 1. Design: Decisions 1, 2, 4.
Files: `internal/pages/option_candidates_page/tiers.go`, `internal/pages/option_candidates_page/tiers_test.go`, `internal/pages/option_candidates_page/tiercompanies.go` (new), `internal/pages/option_candidates_page/tiercompanies_test.go` (new), `internal/pages/option_candidates_page/zh_texts.go`, `internal/pages/option_candidates_page/tierstrings.go`, `internal/pages/option_candidates_page/testdata/ok_rows.json`, `internal/pages/option_candidates_page/testdata/tiers_ok.golden.json`
- Every tier of the answer carries `companies`, one entry per company in the order of its first row, with the count and the row ids of that company in that tier; the counts of a tier's entries sum to the tier's row count and the tiers still sum to the rows read (tests `TestTierCompaniesSumToRows` (new), golden `tiers_ok.golden.json`).
- `idle_note` carries the pointer text of Decision 2 (grep `zh_texts.go`); the strip still lists only the companies with a buy price or a valuation figure.
- The strip's earnings text is a date or "来源暂无日期（未查到，不等于没有）"; the string "未知" for earnings and any text with "3个月内无财报" are not in `zh_texts.go` (grep, test `TestEarningsTextUnknown` (new)).

### 3 earnings-keep-last-date

Module: jobs. Size: S. Order: none. Design: Decision 3.
Files: `internal/jobs/optioncandidates/input.go`, `internal/jobs/optioncandidates/input_test.go`, `internal/jobs/optioncandidates/testdata/earnings_keep.json` (new)
- When the newest earnings row of a name is `absent` or `error` and an older row in the window is `listed` with a report date not before the chain day, `earnings()` returns that date as `Next` (tests `TestKeepLastKnownDate` (new), `TestKeepIgnoresPastDate` (new)).
- An owner override still wins over the kept date (test `TestOverrideWins` (new)); a name with no `listed` row in the window behaves as before (the existing tests pass unchanged).
- No `migrations/` file and no change of `internal/jobs/earningsrefresh/` is made.

### 4 tier-companies-web

Module: web. Size: S. Order: after 1. Design: Decisions 1, 2.
Files: `web/src/pages/candidates/TierView.tsx`, `web/src/pages/candidates/TierRows.tsx`, `web/test/pages/optioncandidates/TierView.test.tsx`, `web/test/pages/optioncandidates/fixtures/bycompany.ts`
- Each tier draws one folded line per entry of `companies` with its count, and opens that company's rows under it; a tier without `companies` draws its rows as before (test checks both from the fixture).
- The web computes nothing: no grouping, count or sum in TypeScript; `tsc`, `eslint`, `vitest` pass in CI.

## Owner-visible results

- /options/candidates: each tier is a list of companies with counts, folded; the 407 rows of "没有买入价" read as nine company lines.
- A company whose date was once listed keeps it when the calendar goes quiet.
- Earnings with no date say "来源暂无日期（未查到，不等于没有）".

## Checked by CI

The tests named above in the `test` check and `vitest`; no live run is needed.

## Left out

- Per-symbol earnings lookup: dropped. Alpha Vantage's bulk file lacks large caps that report inside the window (probe above); whether its per-symbol answer has them cannot be shown with the public demo key and costs a call per name on the owner's key. If wanted, it is a later design with a probe on the owner's key first.
- Another date source (yfinance exists in the repo; `event-sources.md` records that its dates drifted): an owner choice, not made here.
- J-Quants: already one call per symbol; JP earnings wording follows the same page text.
- /positions structure kind: taken out of this design. Suggested separate single-module issue (pages): a link from each option row of `/positions` to `/options/structures` of the alias, with no recognition on the positions page.
- Shortfall notice on /options/notices: taken out. It is a new feature, not a bug: `option-notices.md` Decisions 5 and 6 define the notice codes (`expiry_today`, `expiry_near`, guard, earnings, `assignment_seen`, `cushion_low`) and the cash by date as display only; `internal/compute/optionnotice/cash.go` computes `Left` and no code reads it. Suggested separate single-module issue (compute): a notice `cash_short` per day and currency with `Left` below zero, and its title in `option_notices_page/zh_texts.go` as its own page issue.
- P&L page for calls and long puts (skipped by design, audit "DECIDE"): the owner decides; not here.
- Items 1, 4 of the audit (JPY group live, spot day label): other drafts.

## Adversarial

Answers to `sellput-gaps-adversarial.md`, by its sections:
- (1) JP goal rests on the yen group being live: adopted; JP is `sellput-jp-outcomes.md`, ordered after the registered draft `sellput-jpy-group-live` (its Decision 6). A JP name with no open rows: adopted, `no_rows` there (Decision 3).
- (1) Earnings goal may not be met: adopted. Decisions 3 to 5 cut the goal to a kept date and an honest text; the probe shows the bulk file lacks AAPL, MSFT, NVDA while holding COST, so the cause is the source, not our parse (no code fix exists in our modules).
- (1) Cash notice cause: adopted by removal; verified as a missing feature, not a bug (`## Left out`).
- (2) Alpha Vantage quota (about 25 calls a day): adopted by removing the per-symbol lookup.
- (3) Simpler way: adopted for all four: no positions recognition (taken out, link-only suggestion), one change for the tier wall with `Tier.companies` and no `NoBuyPrice` type, the kept date alone (issue 3), and no lookup. Its idea of a pure function in the pages package fed by `contracts.OptionOpen` is weighed in `sellput-jp-outcomes.md` Decision 5.
- (4) Closed enum of three: adopted in the JP design (open string with a documented set).
- (5.1) positions builder lacks the structure input: adopted by removal. (5.2) "AV has no date yet": adopted, same as the probe. (5.3) cause of the 9 missing names unknown: adopted and found: `splitIdle` (tiers.go:450) moves them to `idle_note` on purpose (`## Today`). (5.4) nonexistent `positions_page_v4_test.go`: adopted, gone with positions; every other listed path exists on origin/main, `(new)` marks the rest. (5.5) the kept date reads `obs.earnings_date` rows through `EarningsDates`; the fixture of issue 3 supplies the date rows and the `earnings_check` status.
- (6) Slip risk: adopted: the three-page, 25-file pages issue is gone; the pages issue touches one page and two new files carry the logic (`tiers.go` is 515 lines, so it does not grow past 600).
- Parallelism: adopted: issues 2, 3 and 4 are independent; Order lines match. Compute split: the cash notice is a separate suggested issue.
