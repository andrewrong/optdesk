# market state ranges: each market row shows a trial normal, caution or alarm range with its source, or the row's own percentile band, and a Chinese hover hint

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-20

Source: owner UI doc row 8 (ask optdesk-5479), decided by optdesk-advisor 2026-11-20 as "3a": the market-state page shows, for each indicator it lists, a normal, caution or alarm range taken from professional sources (each source cited, the numbers marked "trial"); where no published reference exists, the indicator's own percentile band over its stored history; a red reading is a notice only, never a block; the range numbers live in configuration, not code; each indicator has a hover hint in Chinese saying what it is and what the range means. The front end computes nothing; the producer computes the bands.

Goal: on `/market-state` each of the five market rows (S&P 500 against its 200-day line, S&P 500 drawdown from its 60-session high, VIX, VIX against VIX3M, high-yield spread) carries a range badge (Normal, Caution or Alarm, with the rule and its source in words) and a Chinese hint. The owner sees at a glance whether a reading is inside a professional range or unusual against its own past, and why.

Existing design: `docs/design/market-state.md` built the page and, in its Decision 1, said "no state word, no tone or colour, no threshold". The owner's row 8 replaces that Decision for the five market rows only; the sector table keeps it. This design does not edit `market-state.md`; it is read as the later word.

## Today

All paths at origin/main 83720816.

- The producer is `internal/pages/market_state_page` (`builder.go:69` `marketRow`: `id`, `label`, `figure`, `window`, `day`, `percentile`, `history`). `buildRows` (`rows.go:250`) builds the five rows and fills each `history` from `marketstateapi` lines (`internal/compute/marketstate/history.go`), at most 800 daily points over 3 years (`marketstateapi/api.go` `ChartYears` 3, `ChartMax` 800).
- The schema is `contracts/schemas/page.market_state_page.v2.json`: `MarketRow` has `additionalProperties` false and the required `id`, `label`, `figure`, `window`, `day`, `history`, optional `percentile`; the generated type is `web/src/generated/market_state_page.v2.ts`.
- The web page is `web/src/pages/market_state/MarketRows.tsx` (`RowText` prints label, figure text, percentile, window and day); a tooltip component exists at `web/src/components/ui/tooltip.tsx`.
- A precedent for embedded TOML configuration with `go:embed` and `go-toml/v2` is `internal/compute/alertrules/rules.toml`.
- FRED's page for `BAMLH0A0HYM2` (fetched 2026-11-20) states the series holds only three years of observations since April 2026, so a percentile of the high-yield spread is over about three years.

## Decisions

1. Scope. The five market rows of the market block get a range. The sector table gets none (it is a ranking of funds, not a level with a published range).
2. A row's range is an ordered list of cuts, each a `caution` or `alarm` line, in one direction per row (`high_bad`: the reading is worse the higher it is; `low_bad`: the lower). A cut is either a published value, with its source, or a percentile of the row's own history, with the source "own stored history". The state is Alarm when the reading is beyond the alarm cut, Caution when beyond the caution cut only, Normal otherwise.
3. The numbers are trial and live in `internal/pages/market_state_page/ranges.toml`, embedded with `go:embed` and read once with `go-toml/v2`; the Go code holds no band number. The file holds, per row, the direction, the cuts, the Chinese hint and the source ids, and a `[[source]]` table with the full citation text of each source. A row or cut missing from the file gives the row no range, never a default.
4. The first values (all trial; the owner may change them in the file):
   - `vix` (high_bad): caution at 20, alarm at 30, published (source A and B below). Plain meaning: below 20 calm, 20 to 30 elevated, 30 and above stress.
   - `sp500_vs_200d` (low_bad): caution at 0 percent (the index is under its 200-day line), published (source C, whose rule is the 10-month average, read as the 200-day line); alarm at the 5th percentile of the row's history, own history (C gives no second line).
   - `sp500_drawdown` (low_bad): caution at the 20th and alarm at the 5th percentile of the row's history, own history (no citable published line for a drawdown from a 60-session high was found; the 10 and 20 percent "correction" and "bear" words are for the all-time high and are not borrowed).
   - `vix_term` (high_bad): caution at the 80th and alarm at the 95th percentile, own history (no citable published line found).
   - `hy_spread` (high_bad): caution at the 80th and alarm at the 95th percentile, own history (the FRED page and the ICE series give no band; the history is about three years).
5. Sources, cited in the design and, by `[[source]]`, in the file and on the page:
   - A. Cboe, "Inside Volatility Trading: More Chaos Ahead - Or a Bottom? What Does the VIX Index Say?", cboe.com/insights/posts/inside-volatility-trading-more-chaos-ahead-or-a-bottom-what-does-the-vix-index-say (long-term median 17.6; the index tends to revert toward its long-term level).
   - B. S&P Dow Jones Indices, "A Practitioner's Guide to Reading VIX", spglobal.com/spdji/en/documents/education/education-a-practitioners-guide-to-reading-vix.pdf (how the level is read).
   - C. M. T. Faber, "A Quantitative Approach to Tactical Asset Allocation", Journal of Wealth Management 9(4), Spring 2007, 69-79, SSRN 962461 (hold the index while its price is above the 10-month average, leave it below).
   - D. FRED, series BAMLH0A0HYM2 (ICE BofA US High Yield Index Option-Adjusted Spread), fred.stlouisfed.org/series/BAMLH0A0HYM2 (what the series is; three years of data).
   The 20 and 30 of the VIX are the common reading of the level in educational material (A, B and the sources the web search of 2026-11-20 returned); neither is a rule of the Cboe, so the page text says "trial reading", not "official".
6. A percentile cut is computed by the producer from the `history.points` it already builds for the row: the cut value is the nearest-rank value of the points at that percentile (sorted ascending, rank `ceil(p/100*n)`), and a history of fewer than 250 points gives the row the Gap `no_data` for the percentile cuts. The page shows the cut value in the row's unit and the number of days ("the 80th percentile of 780 days, 3.9%"). The reading compared is `figure.value` in the unit of the figure (percent, index points, ratio); a Gap figure gives a Gap range.
7. The payload. `MarketRow` gets two optional fields (v2 in place, as `market_summary.v3` took `index_iv`): `range`, an object `state` (`normal`, `caution`, `alarm`), `state_text` (English, ends "trial"), `cuts` (array of `level`, `text`, `basis` `published` or `percentile`, `source`), and `notice` ("A notice only; nothing is blocked."), or the `common.v2.json` Gap; and `hint`, the Chinese text, 1 to 600 characters. The page's `notes` are unchanged. A range Gap is not a page gap: it does not change `run_status` and is not listed in the page `gaps`.
8. The web page prints `state_text` as a badge with a tone by `state` (normal neutral, caution amber, alarm red; the tone is never a block, no control is disabled or hidden by it), the `cuts` texts and `notice` under the row text, and `hint` in the tooltip of the row label (with a visible text-free mark, so a touch user can open it). It computes no band, no percentile and no comparison.
9. Terms glossed nearby. Every hint says in Chinese what the figure is, what the three ranges mean, and that the range is a trial reading. The hints (first text, the owner may reword in the file):
   - `vix`: "VIX 是标普 500 期权隐含的未来 30 天波动率，常被叫做恐慌指数。20 以下一般视为平静，20 到 30 为偏高，30 以上为紧张；这是试用读法，只是提示，不会阻止任何操作。"
   - `vix_term`: "VIX 与 VIX3M 的比值，即 30 天隐含波动率除以 3 个月隐含波动率。大于 1 说明近期比远期更不安（期限结构倒挂）。区间按它自己的历史分位给出，试用，只是提示。"
   - `sp500_vs_200d`: "标普 500 相对 200 日均线的偏离。低于 0 表示指数在均线下方，趋势策略通常据此转为防守（Faber 2007）；报警线取自身历史的第 5 分位，试用，只是提示。"
   - `sp500_drawdown`: "标普 500 距近 60 个交易日最高点的回撤。区间按它自己的历史分位给出（越低越差），试用，只是提示。"
   - `hy_spread`: "高收益债信用利差（ICE BofA 美国高收益指数期权调整利差），越大代表市场对信用风险要求的补偿越高。区间按约三年自身历史的分位给出，试用，只是提示。"
10. Owner rules kept: results visible on the page; terms glossed beside the term; every published number cited; numbers marked trial; no ticker of the owner's watch list and no JP stock code appears (the page's rows are indexes and FRED series); the producer computes, the web prints.

## Modules

Three modules: contracts (`contracts/schemas/page.market_state_page.v2.json`, `web/src/generated/market_state_page.v2.ts`, `docs/contracts/page-schemas.md`, `internal/contracts/**`), pages (`internal/pages/market_state_page/**`) and web (`web/src/pages/market_state/**`, `web/test/pages/market_state/**`, `web/fixtures/**`). No row of `docs/design/modules.md` changes. Entry 1 is the (interface) entry: the schema is what entries 2 and 3 read.

## Interface

One shared item changes: the schema `page.market_state_page.v2` gains the optional `MarketRow.range` and `MarketRow.hint` (Decision 7), and with it the generated type. No Go declaration changes (`internal/pages/api.go`, `marketstateapi` keep theirs). Entry 1 `Contracts: changed`; entries 2 and 3 `Contracts: unchanged`.

## Issues

### 1 market-state-ranges-schema (interface)

Module: contracts. Size: S.
Order: none.
Files: `contracts/schemas/page.market_state_page.v2.json`, `web/src/generated/market_state_page.v2.ts`, `docs/contracts/page-schemas.md`, `internal/contracts/market_state_ranges_test.go`
Acceptance: `MarketRow` has optional `range` (object or Gap) and `hint`; not in `required`; the generated type has both; a new test shows an instance without and with the fields; no other file changes.

### 2 market-state-ranges-producer

Module: pages. Size: M.
Order: after 1; no file shared with entry 3.
Files: `internal/pages/market_state_page/ranges.toml`, `internal/pages/market_state_page/ranges.go`, `internal/pages/market_state_page/ranges_test.go`, `internal/pages/market_state_page/builder.go`, `internal/pages/market_state_page/rows.go`, `internal/pages/market_state_page/rows_test.go`
Acceptance: the file holds the Decision 4 numbers, sources and hints; `ranges.go` embeds and reads it and computes the state and percentile cuts (Decision 6); `marketRow` gets `Range` and `Hint`; no band number in Go; tests for each row, the Gap cases and the schema.

### 3 market-state-ranges-web

Module: web. Size: S.
Order: after 1 (no file shared with entry 2; it may run beside it).
Files: `web/src/pages/market_state/MarketRows.tsx`, `web/test/pages/market_state/MarketRows.test.tsx`
Acceptance: badge, cuts, notice and hint printed as Decision 8; no computation; tests.

## Checked by CI

- `ci-go` for entries 1 and 2 and `ci-web` for entries 1 and 3 (`tsc`, `eslint`, `vitest`, build).

## After merge (checked by the dispatcher, not by review)

- The window renders `/market-state` and compares each row's badge, cuts and hint with the figures, with a screenshot (owner rule of 2026-11-14), and reads the five cut values against the stored history.

## Left out

- Ranges for the sector table, for the home Market card and for the macro page.
- A published band for the drawdown, the VIX term ratio and the high-yield spread: none was found citable; they use the own-history percentile and may move to a published value by editing `ranges.toml` once the owner names a source.
- Any block, hiding or disabling by a reading: a range is a notice only.
- A page where the owner edits the numbers; the file is the configuration.
- A change of `docs/design/market-state.md` Decision 1 itself.

## Doubts

- Sources A and B were found by web search on 2026-11-20 and read as search summaries, not line by line; the 20 and 30 of the VIX are a common reading, not a Cboe rule. The implementer states only what each citation supports.
- The 10-month average of C is used as the 200-day line (the same idea, not the same window).
- FRED keeps only three years of the high-yield spread since April 2026, so its percentile window is short.

## Adversarial

- A red reading blocks a trade: no. No code path reads `range`; the field has no consumer except the web print, and `run_status` ignores a range Gap (Decision 7).
- A band number hides in Go: a test in entry 2 greps the Go files of the package for the literals 20, 30 and the percentile numbers outside `ranges.toml` and the tests.
- Percentile of a short history looks sharp: the 250-point floor gives the Gap `no_data`, and the cut text shows the day count.
- The web computes a state: entry 3 forbids any comparison; the state comes in the payload.
- A direction mistake (a low reading marked high): each row's direction is in the file and tested with a reading on both sides of each cut.
