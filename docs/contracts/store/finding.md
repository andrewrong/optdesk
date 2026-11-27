# store/finding

Package `internal/store/finding` declares the findings of the daily data check (`docs/design/data-check.md`, Interface, entry 1). It holds declarations only; `internal/store/findingpg` implements `Writer`, `Reader` and `Probe`. The writer and the probe use the line `OT_PG_DSN_DATACHECK` (role `ot_w_datacheck`), the reader the pages line.

## Severities

`error`: wrong data stored as good. `warn`: a gap, data missing or late. `unknown`: the rule could not decide, so the run says nothing about the data it covers.

## Rule ids

`reading_repeat`, `option_open_late_quote`, `candidates_older_than_settings`, `capture_gap`, `plan_absent`, `gap_unclosed`, `candidate_rule_version`, `earnings_coverage`, `bar_ohlc_shape`. The ids are fixed strings; a new rule adds one and never changes one.

## Evidence vocabulary

`ok`, `same`, `rows`, `late`, `hours_behind`, `keys`, `missing`, `expired`, `cells`, `age_hours`, `read_failed`, `names`, `listed`, `errors`, `listed_before`, `bad`. Each is a `Count{Name, N}`; a new name is added, never changed.

## What a finding holds

A finding holds counts and fixed words, never free text, an id or a secret. It is keyed by (rule, dataset, day, subject), not by its counts; `IsNew` is set by the writer and ignored on input.

## Probe additions (data check 2)

`CandidateRun.RuleVersion` is the `rule_version` of the rows of the newest run, empty when they disagree. `Probe.EarningsChecks(from, to)` returns one `EarningsCheck{Market, Day, Names, Listed, Errors}` per market and day that has rows in `rd.earnings_date_r1` (`Listed` those with status `listed`, `Errors` those with status `error`). `Probe.BarShapes(from, to)` returns one `BarShape{Day, Rows, Bad}` per day of `trades` bars in `rd.price_bar_r1`; a bar is bad when low > high, or open or close lies outside [low, high], compared as numbers.
