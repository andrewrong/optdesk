# compute contract

## Responsibility

Pure figures. `internal/compute` turns typed series into `Figure`s through seven interfaces declared in `internal/compute/api.go`: `Indicators`, `Stats`, `Rolling`, `SeriesOps`, `Units`, `Rules` and `Figures`. It stores nothing, reads nothing and has no clock: every figure method takes the as-of day. The package holds declarations only; implementations live in sub-packages behind the constructor function types (`NewIndicatorsFunc`, `NewStatsFunc`, `NewRollingFunc`, `NewSeriesOpsFunc`, `NewUnitsFunc`, `NewRulesFunc`, `NewFiguresFunc`).

## Inputs

- `Series` of `Point{Day, Value}`, ascending, one point per day.
- `[]contracts.SeriesPoint` (vintages and `no_value` rows) for `SeriesOps.FromSeriesPoints`.
- `[]ExpiryQuotes` of IBKR model values and a rate `Point` for `Iv30`.
- The as-of day (`contracts.Day`) of every figure, and a register `id` whose numbers come from `Rules`.

## Outputs

- `Figure{ID, Version, Subject, Day, DataDay, Value, Status, Reason, Observations, StaleRate}`. `ID` and `Version` come from `Rules`; `Day` is the as-of day the caller passed; `DataDay` is the day of the newest input point.
- Series functions return `[]float64` of the input's length.
- Errors are `contracts.TypedError`: `Permanent` for a bad period, unequal lengths, an empty input, NaN, Inf or a negative scale.

## Invariants

- Missing is marked, never faked. `Status` is `ok` or `missing`; a missing figure has a `Reason` (`no_input`, `short_history`, `stale_input`, `zero_range`, `bad_input`) and `Value` 0. No method returns NaN or Inf, a default, a previous value or a recomputed one. IV and Greeks come only from IBKR model values; a missing one is never recomputed.
- Day semantics. A figure whose newest input is older than the rule's `MaxAgeDays` against the as-of day is missing with `stale_input`. `StaleRate` of `Iv30` is true when `rate.Day != asOf`.
- Reading a stored series. `FromSeriesPoints` keeps, per observation date, the point of the latest realtime_start whose status is `present` and whose value is not null; a duplicate day cannot reach a figure.
- Warm-up. A series function returns a slice of the input's length with NaN before the first defined value; `Figures` turns an undefined entry into `missing`, with `short_history` when the history is too short, never a NaN value.
- Parameters are fixed per figure id by `Rules`, not per call; `Compare` (`Below`, `AtOrBelow`) is fixed per id.
- Standard library plus `contracts` only in this package; no I/O.
- A new field of `Rule` or a new `Figures` method is an interface change.

## Alert rules

`AlertRule`, `AlertRules`, `AlertResult`, `ReadingFunc`, `EvaluateAlertsFunc`, `LoadAlertRulesFunc` and `DefaultAlertRulesFunc` are plain types and function types in `internal/compute/api.go`; the implementation (`internal/compute/alertrules`, a stub until its issue lands) is passed in by `cmd/ot`, so notify and pages hold it as a value. Design: `docs/design/risk-alerts.md#Interface`.

- States. `AlertResult.State` is `fired`, `clear` or `no_data`.
- Reasons. `Reason` is empty, or `no_reading`, `missing_input`, `unit_mismatch`, `stale_data` or `no_data_day`; a reason goes with state `no_data` only.
- Window. For each rule `read` is called for the days `today - 5` to `today` and the newest row returned is used; evaluation does not choose a winner, the `ReadingFunc` passed in applies the winner rule of the report page.
- Data-day rule. The row day is not the day of the data. The parameter `data_day` of the row is the day of its newest input: absent or not a day gives `no_data_day`; row day minus `data_day` above the rule's `MaxDataAgeDays` calendar days gives `stale_data`.
- Stale and missing data. `stale_data` and `no_data_day` are counted and logged by rule id, since they mean the data or the rule file is wrong.
- Comparison. Exact on decimal text (no float); `Detail` is built from fixed words and the stored value, never from free text of the rule file. `Public` is true when the subject is in `PublicSubjects`.
- Errors. A reader error is an error of the call, never `no_data`. A loader error names the key and the rule id and never a value.

## Version history

- v1: the seven interfaces, the types and the constructor function types.
- v2: additions only: the alert rule types and function types above.

## Contract tests

- `internal/compute/api_test.go` tests the contract from both sides: a fake of each interface and a caller through the interface only; `TestAlertContract` does so for `EvaluateAlertsFunc` and `LoadAlertRulesFunc`.
- The golden suites per interface (`internal/compute/contracttest`) come with later issues.
