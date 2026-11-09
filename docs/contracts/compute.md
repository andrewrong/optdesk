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

## Ease

`EaseSeries`, `EaseBand`, `EasePart`, `EaseFigure`, `Ease` and `NewEaseFunc` are in `internal/compute/api.go`; the implementation (`internal/compute/ease`, a stub until its issue lands) is passed in by `cmd/ot`. Design: `docs/design/ease-v2.md#Interface`.

- Arithmetic. `Ease` of `macro.ease` is exact (no float drift in the sums); the value and each part's score are rounded half-even to 2 places.
- Parts. `EaseFigure.Parts` always has four entries, in the order `net_liquidity`, `us10y`, `fed_funds`, `hy_oas`; a part without a figure is missing with a reason and an empty `DataDay`.
- Band. `Band` is `tighter`, `steady` or `looser`, and empty when the figure is missing.
- Unknown id. A missing figure with reason `bad_input`, as `Figures`. Pure: no I/O, no clock.

## Option candidates

`CandidateKind`, `GateStatus`, `GateResult`, `OptionQuote`, `EarningsInfo`, `DividendInfo`, `AliasFunds`, `CandidateInput`, `Candidate`, `CandidateParams`, `OptionCandidates` and `NewOptionCandidatesFunc` are in `internal/compute/api.go`; the implementation (`internal/compute/optioncand`, a stub until its issue lands) is passed in by `cmd/ot`. Design: `docs/design/option-candidates.md#Interface`.

- Arithmetic. Exact (`math/big.Rat` in the implementation), no float. Money, rates, prices and the `DGS3MO` rate are decimal text.
- Float to decimal. The caller converts each stored float with `strconv.FormatFloat(v, 'f', -1, 64)` and rounds half-even to 4 places before it reaches `CandidateInput`, so 1.99 is 199/100 and 0.1 is 1/10.
- Rate. `DGS3MO` is percent: the text "4.20" is the ratio 0.042 (`value / 100`).
- Days. `CandidateInput` carries the run day, the chain day and the XNYS session days as ascending day texts (oldest input date to the run day), so every age in sessions is a difference of positions and the rule needs no calendar and no clock. A chain day that is not the latest session day on or before the run day gives `unknown` `stale_chain`.
- Status. Each gate is `pass`, `block` or `unknown`; unknown is not a pass. The row is `blocked` if any gate blocked, else `unknown` if any is unknown, else `ok`. Gate results come in the fixed order of the rule of the kind (see the doc comment of `GateResult`).
- Rank. Only `ok` rows are ranked: annualized yield descending, then DTE, strike and instrument id ascending. A blocked or unknown row has rank 0.
- Formulas. Annualized yield is `(bid / strike) x (365 / DTE)` for a put and `(bid / spot) x (365 / DTE)` for a call, stored to 6 places. Break-even is `strike - bid` for a put and `strike + bid` for a call. Pure: no I/O, no clock.

## Option ledger

`LedgerTrade`, `LedgerMark`, `LedgerInput`, `LedgerParams`, `LedgerPosition`, `LedgerCycle`, `LedgerResult`, `OptionLedger` and `NewOptionLedgerFunc` are in `internal/compute/api.go`; the implementation (`internal/compute/optionledger`, a stub until its issue lands) is passed in by `cmd/ot`. Design: `docs/design/option-ledger.md#Interface`.

- Kind of a row (Decision 6). An option (`assetCategory` OPT) with `openCloseIndicator` `O` is `open`; `C` with notes holding `Ep` is `expire`, `A` is `assign`, `Ex` is `exercise`, otherwise `close`; stock (STK) is `stock_buy` or `stock_sell` by the sign of `quantity`. A cancel or correction row is not entered in a figure: it is listed in `Review`, never a guess.
- Matching (Decision 8). Per alias and contract (`conid`, else `symbol:expiry:strike:put_call`) a closing event matches opens first in, first out. A close with no open before it is listed in `Unresolved`, never a negative lot and never in a figure. Long and short are one mechanism (the sign of the quantity). A lot closed in two steps is two positions. Spreads are not grouped. An assignment before the expiry date is `early`. An `assign` or `exercise` closes only the option position; shares come only from the stock row of the delivery.
- Premium income (Decision 9). Per position, in the currency of the contract, before tax: `premium_income` is the cash of the opens plus the cash of the closes (`proceeds`), `commissions` the sum of `ibCommission` of its rows, `realized_pnl = premium_income - commissions` once the position is closed, expired or assigned; the premium is never moved into a share basis. For an open short, `unrealized_pnl = premium_income - commissions - mark x quantity x multiplier` (the mirror for a long), `mark` the mid of bid and ask of the newest mark. No mark, or one older than `MarkMaxAgeSessions` session days, gives an empty `unrealized_pnl` and `no_mark`; nothing is estimated.
- Wheel cycle (Decision 10). A cycle starts when shares appear (a `stock_buy`, an assignment shows as one) and ends when flat; shares are first in, first out. `cost_basis_per_share` is the buy price, the strike for an assignment; stock commissions are apart in `stock_commissions`; no premium is subtracted from a basis. `stock_realized_pnl = proceeds of the sale - basis of the shares sold - commissions of the sale`; `unrealized_stock_pnl = (stock_mark - cost_basis_per_share) x shares`. The cycle shows `stock_realized_pnl`, `premium_income` and the commissions as separate numbers; no total hides them.
- Numbers (Decision 11). Exact (`math/big.Rat` in the implementation), decimal text at every boundary, rounded half-even to 4 places in printed figures. No float. Every figure carries its currency; no function adds two currencies and there is no FX conversion.
- Input. One alias per call; `Trades` holds the latest revision of each transaction; `SessionDays` are ascending XNYS day texts supplied by the caller, so the rule needs no calendar and no clock. Pure: no I/O, no clock.

## Option structures and results

`LedgerFill`, `StructureQuote`, `HeldOption`, `StructureInput`, `StructureParams`, `Structure`, `StructureLeg`, `StructureFigures`, `FigureValue`, `PayoffPoint`, `GreekSet`, `StructureLevel`, `StructureResult`, `CaptureDiff`, `OptionStructures`, `NewOptionStructuresFunc`, `ResultInput`, `ResultSummary`, `ResultSection`, `ResultMonth`, `ResultEnding`, `ResultReliability`, `OptionResults` and `NewOptionResultsFunc` are in `internal/compute/api.go`, and `LedgerPosition` gains `Right`, `Expiry`, `Strike`, `OpenedOn`, `ClosedOn`, `OpenPrice`, `Fills` and `BrokerRealized`. Design: `docs/design/option-positions.md#Interface`.

- `FigureValue` is a `Value` or a `Gap` (`missing_quote`, `stale_input`, `unsupported_currency`, `no_rule`, `no_data`, `missing_input`); exactly one is set.
- Money and shares are decimal text rounded half-even to 4 places; `KeptShare` is a ratio of 6 places or empty. Pure: no I/O, no clock.

## Option notices

`NoticePosition`, `NoticeGuard`, `NoticeEarnings`, `NoticeValue`, `NoticeInput`, `NoticeParams`, `Notice`, `NoticeFact`, `CashDay`, `NoticeResult`, `OptionNotices`, `NewOptionNoticesFunc` and `DefaultNoticeParamsFunc` are in `internal/compute/api.go`. Design: `docs/design/option-notices.md#Interface`.

- Plain values and decimal text only; no float. Severities are `info`, `warn`, `act`. Pure: no I/O, no clock.

## Rule parameters and settings form

`RuleParamSpec`, `RuleParams`, `NewRuleParamsFunc`, `FormToken`, `FormRequest`, `FormField`, `FormState`, `FormChange`, `SettingsForm` and `NewSettingsFormFunc` are declared in the package `internal/compute/settingsapi` (declarations only, imports `time` only), not in `internal/compute/api.go`. `RuleParams` is the registry of the twelve owner parameters; `SettingsForm` issues the form token, guards a POST and parses a posted form. Design: `docs/design/settings-form.md#Interface`.

- Text only; no float. Pure: the clock is passed in.

## Version history

- v1: the seven interfaces, the types and the constructor function types.
- v2: additions only: the alert rule types and function types above.
- v3: additions only: the ease types and `Ease` above.
- v4: additions only: the option candidate types and `OptionCandidates` above.
- v5: additions only: the ledger types and `OptionLedger` above.
- v6: additions only: the option structure and result types, `OptionStructures`, `OptionResults` and the eight `LedgerPosition` terms above.
- v7: additions only: the option notice types, `OptionNotices` and the two function types above.
- v8: additions only: the package `internal/compute/settingsapi` above.

## Contract tests

- `internal/compute/api_test.go` tests the contract from both sides: a fake of each interface and a caller through the interface only; `TestAlertContract` does so for `EvaluateAlertsFunc` and `LoadAlertRulesFunc`.
- `internal/compute/settingsapi/api_test.go` tests `RuleParams` and `SettingsForm` from both sides: a fake of each and a caller that resolves a stored map and parses a posted form.
- The golden suites per interface (`internal/compute/contracttest`) come with later issues.
