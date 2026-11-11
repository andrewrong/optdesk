# Options premium series: the home options card shows the six month premium by close month, computed in Go

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-11
Revision: 2

Source: owner 2026-11-10 ("前段几乎没什么真实数据"); follow-up of `pages-real-data.md` entry 16 (home-member-options, merged, PR 1400), whose `## Left out` says a later compute issue adds the premium by month and a one-file pages issue replaces the gap.
Read on origin/main at ae9e442f: `AGENTS.md`, `docs/design/modules.md`, `docs/design/pages-real-data.md` (entries 16 and 17, Decisions 1, 3 and 4), `internal/compute/api.go` (`ResultInput`, `ResultSection`, `ResultMonth`, `OptionResults`), `internal/compute/optionresult/{months,optionresult}.go` and tests, `internal/pages/home_options/member.go`, `contracts/schemas/page.options_summary.v4.json`, `docs/design/ratchet.json`.

Goal: the output of `home_options.Build` shows, for each of the six calendar months ending with the month of the day, the premium of the sold USD positions closed in that month, a figure built by Go, with a card text that says so; a month with no closed sold position is the gap `no_data` (a hole, never zero). The web renders and computes nothing. The card reaches `/home` through entry 17 of `pages-real-data.md` (ot-home-route, issue #1419, in the queue); this design does not wait for it and adds no wiring.

## What main holds today

- `home_options.Build` returns `page.options_summary.v4`; `series(d)` makes six points whose `premium` and `closed` are the gap `no_data`, because `compute.ResultMonth` is `{Month, Realized, Running string; Over bool}`. The texts say "尚未计算": `textMonthly`, `seriesText`, `closedText`, `seriesCaption` (member.go lines 45-52) and `k.Gaps` always holds `noData(textMonthly)`. The web shows `premium_series_text` as the legend and the caption under the chart, and the gaps as notes, so figures alone would sit under a false text.
- The monthly figures live in `internal/compute/optionresult` (`months(rows, asOf)`), not `optionledger`: its `row.premium` is the opening premium (`openingPremium` takes the absolute value of each open fill, so it is never negative) and `ResultSection.Premium` is the sum of those rows, so the tile and the series use one rule.
- `internal/compute/api.go` is 835 lines with a ceiling of 840 in `ratchet.json` (owner only); the next field there needs the owner.

## Decisions

1. Meaning: the month of a bar is the close month. `ResultMonth.Premium` is the sum of the opening premium of the rows of the section closed in that month (the grouping of `months`, by `ClosedOn`), decimal text of four places, the same rule as `ResultSection.Premium`. It is not the premium sold in the month: a put sold in June and expired in September is in September. The bars do not sum to the tile, which is the all-time sum.
2. The field `Premium` is added on the existing line `Month, Realized, Running string` (no new line, so the ceiling of `api.go` holds); no function or interface type is added, `OptionResults.Summarise` already returns `ResultMonth`. Months are per section (side and currency); a month with no row is absent.
3. `home_options` copies `Premium` of the month whose `Month` equals the point month of the sold USD section through its existing `money` (text only, no arithmetic); a point with no such month, or no sold USD section, keeps the gap `no_data`. The schema is unchanged (a point allows a figure or a gap). The `closed` series stays the gap `no_data` (Left out) and `closedText` says the monthly closed result is not given, not that it is "not yet calculated".
4. The card text is part of entry 3: `seriesText`, `seriesCaption` and the per-point gap text say 按平仓月统计的已了结权利金 (close month), the `k.Gaps` entry for the series is dropped when at least one month has a figure, and the gap text of a month with no closed sold position says there was none (没有当月平仓的卖出记录), not "尚未计算".
5. Own implementation, because no library fits a rule of this ledger; it sits behind `compute.OptionResults`.

## Modules

No row changes. compute and pages keep their rows; every file below is inside the paths of its row and no depends cell changes (pages already names compute).

## Interface

- `internal/compute/api.go`: `ResultMonth` gains `Premium string` (decimal text, four places, the opening premium of the positions closed in the month). Owning module: compute. Entry 1 is the interface entry; entries 2 and 3 wait on it.

## Issues

### 1 result-month-premium (interface)

Module: compute. Size: S.
Files: `internal/compute/api.go`, `internal/compute/api_test.go`
Design: docs/design/options-premium-series.md#Interface
Order: first.
Goal: `compute.ResultMonth` has the field `Premium` of Interface on the existing field line, with its meaning (close month) in the doc comment; the contract test sets it in its `ResultMonth` literal and reads it back. `api.go` stays at or under 840 lines.
Acceptance:
1. `grep -cE 'Month, Realized, Running, Premium string' internal/compute/api.go` prints 1, and `wc -l < internal/compute/api.go` prints at most 840.
2. `grep -c 'Months:.*Premium:' internal/compute/api_test.go` prints 1.
3. `git diff --name-only $(git merge-base HEAD origin/main) | grep -vxF -e internal/compute/api.go -e internal/compute/api_test.go | wc -l` prints 0.
4. The result matches docs/design/options-premium-series.md#Interface

### 2 optionresult-month-premium

Module: compute. Size: S.
Files: `internal/compute/optionresult/months.go`, `internal/compute/optionresult/months_test.go`, `internal/compute/optionresult/testdata/mixed_endings.json`
Design: docs/design/options-premium-series.md#Decisions
Order: after 1.
Goal: `months` fills `Premium` with the sum of `row.premium` of the rows closed in the month (Decision 1), through the existing `fixed(.., 4)`. In `mixed_endings.json` the expected months are: JPY sold 2026-03 `2000.0000`; USD sold 2026-01 `200.0000` and 2026-02 `300.0000` (sum `500.0000`, the section `Premium`); USD bought 2026-03 `110.0000` (positive, as `openingPremium` takes the absolute value). `months_test.go` adds a case of two positions closed in one month (the sum) and one in another month, and that the months of a section sum to its `Premium`.
Acceptance:
1. `grep '"Month":' internal/compute/optionresult/testdata/mixed_endings.json | grep -c '"Premium"'` prints 4, and the four values are the ones of the Goal.
2. `grep -c 'Premium' internal/compute/optionresult/months.go` prints at least 2, and `grep -cE 'time\.Now|net/http|database/sql' internal/compute/optionresult/months.go` prints 0.
3. `git diff --name-only $(git merge-base HEAD origin/main) | grep -vxF -e internal/compute/optionresult/months.go -e internal/compute/optionresult/months_test.go -e internal/compute/optionresult/testdata/mixed_endings.json | wc -l` prints 0.
4. The result matches docs/design/options-premium-series.md#Decisions

### 3 home-options-premium-series

Module: pages. Size: M.
Files: `internal/pages/home_options/member.go`, `internal/pages/home_options/member_test.go`, `internal/pages/home_options/testdata/ok.golden.json`
Design: docs/design/options-premium-series.md#Decisions
Order: after 2.
Goal: the output of `home_options.Build` (not the `/home` route, see the header) fills each point's `premium` from the sold USD section's `ResultMonth.Premium` by month (Decisions 3 and 4) and changes the series texts: the constants `textMonthly`, `seriesText`, `closedText`, `seriesCaption` (lines 45-52) and the `k.Gaps` entry (line 248) say 按平仓月统计的已了结权利金 and no longer "尚未计算" for the premium. `TestSeriesIsNoDataGap` is rewritten (a month with a figure, a month without) and the helper `sold()` gets a `Months` argument. In `ok.golden.json` the section has `Months` 2026-09 premium `200.0000` and 2026-11 premium `300.0000`: the series is gap, gap, gap, `+$200.00`, gap, `+$300.00` for 2026-06 to 2026-11. No arithmetic is added in pages (`big.`, `math.`, `Sprintf` still print 0).
Acceptance:
1. Reading `internal/pages/home_options/testdata/ok.golden.json`: the six points of the series, in the order 2026-06 to 2026-11, have a premium of kind gap, gap, gap, value, gap, value, and the text of the 4th is +$200.00 and of the 6th +$300.00 (every point is read, not a window of lines).
2. `grep -c '按平仓月' internal/pages/home_options/testdata/ok.golden.json` prints at least 1, and `grep -c '尚未计算' internal/pages/home_options/testdata/ok.golden.json` prints 2 less than at the merge base (the series text and the series gap).
3. `grep -cE 'big\.|math\.|Sprintf' internal/pages/home_options/member.go` prints 0, and `grep -c 'TestSeriesIsNoDataGap' internal/pages/home_options/member_test.go` prints 0.
4. `git diff --name-only $(git merge-base HEAD origin/main) | grep -vxF -e internal/pages/home_options/member.go -e internal/pages/home_options/member_test.go -e internal/pages/home_options/testdata/ok.golden.json | wc -l` prints 0.
5. The result matches docs/design/options-premium-series.md#Decisions

## Checked by CI

Every test run is CI, not acceptance: ci-go runs `go test ./internal/compute/... ./internal/pages/home_options/... -race -count=1` (two positions closed in one month add up, a month with no row is absent and a gap on the page), the contract test of `internal/compute/api_test.go`, `go run ./scripts/arch` and the size ratchet. Tests are parallel with `t.TempDir()` and the fake clock; no test sleeps or connects to a broker. `Contracts: unchanged` for all three.

## Left out

- The `closed` series by month (stays the gap `no_data`): needs a count or result by close month in `ResultMonth`, and `api.go` has 5 lines of room under its ceiling; ask the owner whether the card should show it.
- Premium by the month it was sold (open month): a second field in `api.go`, needs the owner; this series is by close month on purpose.
- Other currencies than USD (the card already gaps them `unsupported_currency`).
- The route: entry 17 of `pages-real-data.md` (#1419).

## Adversarial

Answer on revision 1; full answer: the dispatcher's scratchpad file `adversarial-options-premium-series.md`.

1. Stale texts stay under the figures: adopted, entry 3 changes the constants, the gap and the golden (Decision 4).
2. The card does not reach `/home` before entry 17: adopted, the Goal is narrowed to the member output and names #1419; no Depends line, the member output does not need it.
3. Close-month meaning is unnamed: adopted, Decision 1 and the caption text.
4. A negative premium case cannot exist (`openingPremium` takes Abs): adopted, dropped; the bought row is `110.0000`.
5. `mixed_endings.json` has four month lines, not three: adopted, acceptance 1 of entry 2 prints 4, with values.
6. `grep -A70` misses later points: adopted, replaced by a `jq` read of all six points of the golden.
7. `TestSeriesIsNoDataGap` and `sold()` must change: adopted, named in the Goal of entry 3.
8. Acceptance as grep shapes: adopted in part, results are stated as values; the "matches the design" item stays as the format requires.
9. A simpler way: none for compute (the field is the smallest change); not adopted otherwise.
