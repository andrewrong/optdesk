# sellput-jp-outcomes: a Japanese name with no candidate row says why, from the stored open rows

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-25
Revision: 1

Source: audit `sellput-audit-2026-11-25.md` item 2; owner rule (ask 5811): JP option markets are thin; missing data is never called an error without telling "the exchange has no quote" from "the fetch failed or was deferred". Order: after the registered draft `sellput-jpy-group-live` (it makes the yen group appear on the live page; without it this work is invisible) and after issue 1 of `sellput-gaps.md` (same schema file).
Read on origin/main (87f03903): `AGENTS.md`, `docs/design/modules.md`, `candidates-jp.md`, `internal/jobs/optioncandidates/jp.go` (line 254), `internal/jobs/optionopen/jp.go` (lines 312-326, `isNoQuoteRow`), `internal/jobs/optionopen/core.go` (statuses), `internal/store/openquote/api.go`, `internal/compute/optioncandapi/api.go`, `obs.option_open` of 2026-11-25.

## Goal

Each JP name that has no candidate row today shows one of: the exchange has no quote, the fetch failed, the fetch was deferred, or the name was not in today's plan at all, with counts.

## Today (measured, `obs.option_open`, session day 2026-11-25, currency JPY)

- 7 underlyings have open rows: status 1 (ok) 80 rows, 2 (partial) 9, 3 (no data) with code `NO_QUOTE` 39, 7 (deferred) with code none 10, and 7 with code `NO_DATA` 28.
- By underlying: 2413 has 11 `NO_QUOTE` rows and nothing else (the exchange has no quote); 2408 has 28 deferred rows with code `NO_DATA`: in `optionopen/core.go:238-243` status 7 with a code is written when the snapshot of the underlying gave no usable price (`spot()`, `NO_DATA` at line 343 or 369), so the 28 contracts were never asked; the market was not shown to have nothing for the options, the stock's price was not read; 2409 mixes 28 `NO_QUOTE`, 1 partial and 3 deferred; 2407, 2410, 2411, 2412 have usable rows.
- The JP candidate job names such a name only in its in-memory `Skipped()` ("no usable open quote, no row", `optioncandidates/jp.go:254`) and the log; a restart loses it and the page shows nothing. JP candidate rows today: 16 rows of 2 names.
- A JP entry of the watch list with no open row at all (the plan holds none, "no option chain", `jp.go:240`) has nothing to classify.

## Decisions

1. The page shows the outcome of each JP name that has no candidate row, read from the stored open rows of the chain day (they are stored, so a restart loses nothing), classified by a compute function; the page prints. A name with candidate rows shows no outcome.
2. The classifier reads the status definitions of `option-open-pull.md` Decision 8 and `optionopen/core.go`, row by row: status 3 with code `NO_QUOTE` counts as `no_quote` (the exchange has nothing for that contract); status 6, 8 and 9 count as `failed` (a fetch that did not finish); status 7 with code `BUDGET` or `TIME_CUT`, with no code, or outside the band counts as `deferred` (not asked: over a ceiling, a budget or the time bound, so "not yet reached", never a failure); status 7 with any other code (`NO_DATA`, `NO_SUBSCRIPTION`, `TIMEOUT`, a number) counts as `no_spot` (the stock's price was not read, so its contracts were not asked); any other not-ok status counts under `other` with its code. The name's outcome is the worst present: `failed`, then `no_spot`, then `deferred`, then `no_quote`, then `other`; fetch-failed always ranks above no-quote. The counts of every kind are printed, so "28 个合约取数失败（可重试），3 个未取" is visible, and 2413 reads "交易所无报价".
3. A JP entry with no open row at all has the outcome `no_rows`: "今日取数计划里没有这个名字的期权链（不是出错）". The names come from the same watch list the JP job reads.
4. The outcome is an open string with the documented set `no_quote`, `failed`, `no_spot`, `deferred`, `no_rows`, `other`, so a sixth kind later is not a schema change. Words, Chinese with the term beside it: `no_quote` "交易所无报价（NO_QUOTE：没有人挂单，不是出错）"; `failed` "取数失败（可重试）"; `no_spot` "没读到标的现价（<code>），所以没有询价"; `deferred` "未轮到取数（超出本次上限或时间，不是出错）"; `no_rows` as above; `other` "其他状态（<code>）".
5. The classifier lives in `compute` behind `optioncandapi` (AGENTS: other modules use a module through its interface); a pure function in the pages package was weighed and not taken, because the page computes nothing. It takes plain facts (status, code, outside band), not `contracts.OptionOpen`.
6. Nothing else changes: the JP job, its `Skipped()` and the yen group are not edited here.

## Modules

Rows used: contracts, compute, pages, web, ot. No row of `modules.md` is added or changed; no migration. The entry file `internal/compute/optioncandapi/api.go` is edited (an interface change; issue 2 carries the Design line).

## Interface

- `page.option_candidates_page.v1` gains the optional `Group.name_outcomes`: array of {`name` Text, `outcome` string (open; documented set in Decision 4), `text` Text, `counts` Text}; additive, `schema_version` stays 1. Owner: contracts (issue 1).
- `internal/compute/optioncandapi/api.go` gains `OpenFact` {`Status int`, `ErrorCode string`, `OutsideBand bool`}, `NameOpenFacts` {`Name string`, `Facts []OpenFact`}, `NameOpen` {`Outcome string`, `NoQuote, Failed, NoSpot, Deferred, Other int`}, the constants of Decision 4, `NameOpenRule` {`Classify(NameOpenFacts) NameOpen`} and `NewNameOpenRuleFunc`. Owner: compute (issue 2). Empty `Facts` gives `no_rows`.
- The page `Config` gains `OpenRows openquote.OptionOpenReader`, `NameOpen optioncandapi.NewNameOpenRuleFunc` and `JPNames []string`; with any of them nil the group has no `name_outcomes` and a gap with the reason "no_reader" (never a guess).

## Issues

All entries: `Design: docs/design/sellput-jp-outcomes.md#<decision>`. The order across two design files is not read by `crew issue check` (it reads `<!-- order -->` comments inside one design and `Depends on: #N` against existing issues), so issue 1 below carries `Depends on: #<issue 1 of sellput-gaps>` once that issue has a number. Issues 1 and 2 are independent and build in parallel; issues 3 and 4 follow their own interface only; issue 5 is the one ordered tail (it needs the page `Config` of 3).

### 1 name-outcomes-contract (interface)

Module: contracts. Size: S. Order: after the registered draft `sellput-jpy-group-live` and after issue 1 of `sellput-gaps.md`. Contracts: `page.option_candidates_page v1 additive`. Design: Decisions 1, 4.
Files: `contracts/schemas/page.option_candidates_page.v1.json`, `contracts/fixtures/MANIFEST.synthetic-v1.json`, `docs/contracts/page-schemas.md`, `internal/contracts/option_candidates_page_v1_test.go`, `internal/contracts/gen/records.go`, `web/src/generated/option_candidates_page.v1.ts`
- `Group` has the optional `name_outcomes` with `name`, `outcome` (a string, no enum), `text`, `counts`; it is in no `required` (tests `TestNameOutcomesValid` (new), `TestNameOutcomesOpenString` (new): an unknown outcome string validates).
- A document without it still validates (test `TestNoNameOutcomesStillValid` (new)); `schema_version` is 1; `web/src/generated/option_candidates_page.v1.ts` contains `name_outcomes` (grep).

### 2 name-open-rule (interface)

Module: compute. Size: M. Order: none. Design: Decisions 2, 3, 5.
Files: `internal/compute/optioncandapi/api.go`, `internal/compute/optioncandapi/api_test.go`, `internal/compute/optioncand/nameopen.go` (new), `internal/compute/optioncand/nameopen_test.go` (new), `internal/compute/optioncand/testdata/nameopen.json` (new)
- `api.go` declares the types, constants and `NameOpenRule` of `## Interface`, declarations only; `api_test.go` has a fake rule and a caller (the contract from both sides).
- `optioncand` implements the rule: all rows status 3 with `NO_QUOTE` give `no_quote`; one status 6, 8 or 9 gives `failed`; status 7 with a code other than `BUDGET` and `TIME_CUT` gives `no_spot` when none failed; status 7 with `BUDGET`, `TIME_CUT`, no code or outside the band gives `deferred` when neither of those is present; empty facts give `no_rows`; a mix gives the worst with all counts set (table cases in `testdata/nameopen.json`, test `TestNameOpenOutcomes` (new)).
- The case of 2026-11-25 is in the table: 11 `NO_QUOTE` rows give `no_quote` with `NoQuote` 11; 28 status 7 with code `NO_DATA` give `no_spot` with `NoSpot` 28; 28 `NO_QUOTE`, 1 partial and 3 deferred (no code) give `deferred` with the counts 28 and 3 (test `TestNameOpenLiveCases` (new)).
- The rule reads no store and no clock.

### 3 name-outcomes-page

Module: pages. Size: M. Order: after 1 and 2. Design: Decisions 1, 3, 4.
Files: `internal/pages/option_candidates_page/jp.go`, `internal/pages/option_candidates_page/jp_test.go`, `internal/pages/option_candidates_page/page.go`, `internal/pages/option_candidates_page/nameoutcome.go` (new), `internal/pages/option_candidates_page/nameoutcome_test.go` (new), `internal/pages/option_candidates_page/zh_jp_names.go` (new), `internal/pages/option_candidates_page/testdata/jp_names.golden.json` (new)
- The yen group has `name_outcomes` for every name of `JPNames` with no candidate row, with the Chinese words of Decision 4 and the counts as text; a name with candidate rows has none (golden `jp_names.golden.json`, test `TestJPNameOutcomes` (new)).
- A name with no open row has outcome `no_rows`; with a nil reader or rule the group has no `name_outcomes` and carries a gap "no_reader" (test `TestJPNameOutcomesNoReader` (new)).
- `grep -n "NO_QUOTE" internal/pages/option_candidates_page/zh_jp_names.go` finds the gloss of `no_quote`; the word 出错 is not in the `no_quote` text.

### 4 name-outcomes-web

Module: web. Size: S. Order: after 1. Design: Decisions 2, 4.
Files: `web/src/pages/candidates/CandidateGroup.tsx`, `web/src/pages/candidates/NameOutcomes.tsx` (new), `web/test/pages/optioncandidates/NameOutcomes.test.tsx` (new), `web/test/pages/optioncandidates/fixtures/nameoutcomes.ts` (new)
- Each entry of `name_outcomes` is drawn under the yen group with its text and counts; `no_quote` is not drawn in the error colour and `failed` is (test checks the class names from the fixture).
- A group without the field draws as before; the web computes nothing (no classification or counting in TypeScript); `tsc`, `eslint`, `vitest` pass in CI.

### 5 name-outcomes-ot

Module: ot. Size: S. Order: after 3. Design: Decisions 1, 3.
Files: `cmd/ot/candidates_site.go`, `cmd/ot/candidates_site_test.go`
- The candidates page `Config` gets the open-row reader of the serve wiring, `optioncand`'s `NewNameOpenRuleFunc` and the JP names of the watch list (the list the JP job reads); with no open-row reader the page is built without them (test `TestCandidatesSiteNameOutcomes` (new) in `candidates_site_test.go`).

## Owner-visible results

- /options/candidates, yen group: each JP name without rows reads "交易所无报价", "取数失败（可重试）", "没读到标的现价", "未轮到取数" or "今日取数计划里没有这个名字的期权链", with counts; today 2413 reads no quote and 2408 reads that its stock price was not read.

## Checked by CI

The tests named above in the `test` check and `vitest`.

## Left out

- The yen group being live, its currency handling and the spot day label: `sellput-jpy-group-live`.
- A change of the JP job, its `Skipped()` or `option_open`: the stored rows already carry the evidence.
- A retry button for `failed` names: a console action of its own.

## Adversarial

- A JP name with no open rows and a closed enum: both adopted (Decisions 3, 4).
- An interface for the classifier versus a pure function in the pages package: interface kept (Decision 5), at the price of one compute issue; the two interface issues build in parallel.
- The shared schema file with `sellput-gaps.md`: issue 1 is ordered after that design's issue 1.
