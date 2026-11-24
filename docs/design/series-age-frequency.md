# series age by frequency: one late limit per FRED frequency, the same in the base-data rates row and the macro page

Author: the optdesk designer
Owner approved: (pending)
Date: 2026-11-25

Source: bug sweep 2026-11-24 entry 6 (rates row `series_age` 50 percent failed; macro page late rows). Owner rule: FRED data lags, tolerate. Cites checked at origin/main 074991c7. The numbers are trial values.

## 你会看到什么

- /base-data 的"利率"一行："序列过期"只在序列比下面的天数更旧时才算：日频 4 天、周频 14 天、月频 100 天、季频 220 天。月度和季度数据本来就晚一两个月才发布，不再每月被算成过期。
- /macro 的"指标"卡：月频序列（日本 10 年国债）超过 100 天才标"Late"，周频序列超过 14 天才标"Late"（原来 45 和 10）。日频仍是 6 天（一个周末加一个节假日），不变。
- /base-data 的说明文字写出这几个天数；/macro 的说明文字不变。

## Today

- `migrations/0058_base_fact_other.sql:110-117` (view `rd.base_fact_other_r1`, rule `series_age`): Bad = newest `observation_date` older than 4 (daily), 14 (weekly), 45 (monthly), 120 (quarterly, the `ELSE`) days from the New York day.
- `internal/compute/basecheck/rules.go:86` holds only the verdict limits `pair(anyBad, share("0.1"), "trial")`; `internal/compute/basecheckapi/api.go:155` holds only the rule name. Neither holds a day count, so neither changes.
- `internal/pages/base_data_page/zh_texts.go:19` `hintSeriesAge` names no number. `internal/pages/macro_page/indicators.go:18` `monthlyFreshnessDays = 45`, `:20` `weeklyFreshnessDays = 10`, `builder.go:32` `dailyFreshnessDays = 6`.
- Open draft `bug-G6-basedata-age` (migration with monthly 100, quarterly 215) is replaced by this design: its quarterly number becomes 220 and its page text is part of entry 2 here.

## Decisions

1. Limits in days from the page day: daily 4, weekly 14, monthly 100, quarterly 220. Reason: a monthly figure is dated the first day of its month and published up to about 35 days after the month ends, a quarterly one up to about 130 days after the quarter starts plus the revision lag; both are trial and are tuned on observed rows.
2. A new migration (the next number after the newest file of `migrations`) replaces the view with the body of 0058, changing only the limits of the `series_age` block. A series with no frequency is still not counted.
3. The macro page uses the same weekly and monthly numbers (14 and 100). Its daily limit stays 6, because the macro card judges one published day, not a count of series. The macro page has no quarterly series in its card, so no quarterly constant is added.
4. /base-data states the numbers in Chinese next to the rule; the Chinese texts of /macro do not change.

## Supersedes

- `docs/design/base-data-page.md:103` (table row rates `series_age`): replace "older than 4 days (daily), 14 (weekly), 45 (monthly), 120 (quarterly)" with "4 (daily), 14 (weekly), 100 (monthly), 220 (quarterly)".
- `docs/design/macro-page-v2.md:129` (risk bullet): replace "`status` `late` after 45 days" with "`late` after 100 days".

## Modules

| issue | module | order |
|---|---|---|
| store-series-age-view | store | first |
| pages-series-age-texts | pages | first |

No change to `docs/design/modules.md`: no new package and no new import edge.

## Issues

### store-series-age-view

Module: store. Files: `migrations/<next>_base_fact_series_age.sql`, `internal/store/basefactpg/other_pg_test.go`. Order: first.

Result for the owner: after the migration, /base-data rates row counts a monthly series late only after 100 days and a quarterly one after 220.

Acceptance:
1. The new migration replaces `rd.base_fact_other_r1` with the body of `0058_base_fact_other.sql`; in its `series_age` block the `CASE s.frequency` reads `WHEN 'daily' THEN 4 WHEN 'weekly' THEN 14 WHEN 'monthly' THEN 100 ELSE 220 END`, and no other block differs from 0058.
2. In `other_pg_test.go` the series table of `TestOtherViewSeriesAge` has a monthly series 90 days old (not late), a monthly one 110 days old (late), a quarterly one 200 days old (not late) and a quarterly one 230 days old (late), and the expected counts are written as literals.
3. New test `TestOtherViewSeriesAgeMonthlyQuarterly` (new) holds the four rows of item 2.

Checked by CI: `ci-pg` runs the `pg` tests of `internal/store/basefactpg`.

### pages-series-age-texts

Module: pages. Files: `internal/pages/base_data_page/zh_texts.go`, `internal/pages/macro_page/indicators.go`, `internal/pages/macro_page/indicators_test.go`, `internal/pages/macro_page/builder_test.go`. Order: first (any order with entry 1).

Result for the owner: /base-data says how many days each frequency may lag; /macro marks a monthly series Late only after 100 days and a weekly one after 14.

Acceptance:
1. `hintSeriesAge` in `zh_texts.go` contains the text "日频 4 天、周频 14 天、月频 100 天、季频 220 天".
2. In `indicators.go` `monthlyFreshnessDays` is 100 and `weeklyFreshnessDays` is 14; `dailyFreshnessDays` in `builder.go` is not edited.
3. In the tests of `macro_page` that read the monthly or weekly limit, a monthly series 60 days old is "On time" and one 110 days old is "Late"; a weekly one 12 days old is "On time" and one 16 days old is "Late"; the days are written as literals.
4. New test `TestMonthlyWeeklyLimits` (new) holds the cases of item 3.
5. `rules.go` and `basecheckapi/api.go` are not in the diff.

Checked by CI: `ci-go` runs `go test ./internal/pages/macro_page/... ./internal/pages/base_data_page/...`.

## Left out

- Series with an empty frequency (IORB, NFCI, SOFR99, STLFSI4, TLAACBW027SBOG, WLCFLPCL) stay uncounted: the frequency is the owner's entry in `obs.series_definition`.
- The macro daily limit 6 is not changed to 4.
- Per-series limits (a slow series such as JOLTS) are not added; one limit per frequency.
- `lookbackDays = 60` of the macro lead is not changed: the lead reads daily series only.
