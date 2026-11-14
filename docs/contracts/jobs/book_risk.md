# book_risk

## Book risk

- `bookrisk.Stores` and `bookrisk.New(Stores, clock.Clock) (contracts.Job, error)` (`internal/jobs/bookrisk`; design `docs/design/strategy-risk.md`, Decisions 4, 5, 8, 9, 11 and 12). The job `book_risk` is anchored at `session_close` plus 45 minutes on `XNYS`, after `option_candidates`.
- The job imports the root package `internal/strategy` only. `Stores` holds a `strategy.RiskStresser` and a `strategy.RiskFitter`; `cmd/ot` builds and injects them. `OptionEod`, `Terms`, `Marks` and `Series` may be nil (no option quote, no share mark, no rate); every other field is required.

## Manifest (`job.toml`)

- `id = "book_risk"`, `version = 1`; `[schedule]` calendar `XNYS`, anchor `session_close`, zone `America/New_York`, offset `45m`, `run_bound = "10m"`, `restart_sensitive = false`.
- `[params] max_mark_age_sessions = 5`: the value `risk_page` uses [O].
- `[[class]]` rows `symbol`, `class`: SPX, XSP, NDX `broad_high`; RUT `broad_other`; every other symbol is `equity`. Each number carries a tag comment. There is no `[[need]]` row: the job needs no capture of its own.

## Reads

Per alias of `Stores.Aliases`, in the sequence of `internal/pages/risk_page/load.go`: the ledger rows (`FlexTradeRows`), `Replay` with `MarkMaxAgeSessions` and no marks, the session days of the last 60 calendar days, the newest capture of the last ten session days (OPTION and STOCK rows), the newest `option_eod` row of each contract of the held underlyings through `store.OptionTerms`, then `compute.OptionStructures` over the input. An alias with no ledger row and no capture has no book and no row.

- Option holdings: one per structure leg; `Mark` is the leg's mid, `Spot` the `UnderlyingClose` of the leg's quote, both days the quote's. `RiskLeg.CashSettled` is true when `ExerciseStyle` is European and `SettlementType` cash, or the underlying's class is `broad_high` or `broad_other`; empty or `unknown` terms are assignable unless the class table says otherwise.
- Share holdings: the STOCK rows of the capture, price from the `UnderlyingClose` of a quote of the symbol, else from `store/mark` (`Marks(alias, day)`, joined by `ConID`; usable when `Status` is ok, `Basis` is not `none` and a price exists). An alias with no capture has `HasCapture` false and the share lines its ledger cycles carry, priced only when a quote gives a spot.
- A capture option the ledger holds no open position for is excluded as `not_in_ledger` in the currency of its capture row; the currency then has no total.
- `Funds`: `NetLiquidation`, `AvailableFunds`, `ExcessLiquidity` of the newest `account.value` capture of the run day, else of the session day before, with the currency of the NetLiquidation row; nil when neither day has one.
- `Rate`: the newest `DGS3MO` point of the last 14 days as stored (percent text), empty without a reader or a point.
- The ok rows of `Reader.Results(day, "short_put")`; a failed read of them fails the run with nothing written. No broker and no network source is read.

## Rows

All rows go through `strategyresult.Writer.InsertResults` in one call under one new run id (one UUID per run); a rerun writes a new run id. `legs` and `checks` of a `book_risk` row are `[]`; `facts` is a JSON object with sorted keys; an absent figure is an absent key. Facts of `docs/contracts/strategy.md` ("Book risk, Rows").

- `book_risk` `<alias>:<currency>`, one per currency of the alias's holdings and of what the stress returned; `underlying_id` 0; status `ok` when the currency's total is complete (the outcome has the total and nothing of the currency is excluded), `unknown` otherwise. Facts: `worst_loss_<currency>` (complete only), `ladder_<class>`, `worst_<class>`, `oldest_spot_day_<class>`, `assignment_cash` (sum of the class worsts' assignment cash), `excluded` (underlying, reason, count), `not_stressed_<currency>` (holdings of another currency not stressed), `range_basis` (`finra_equity_applied` for a non-USD currency), `basis_note`, `equity_loss_vs_excess_liquidity` (`{"worst_loss", "excess_liquidity"}`, complete total in the funds currency only), `margin_change_not_modelled` `true`.
- A failed read of one alias is a row `<alias>:USD` (USD: no holding says another currency), status `unknown`, fact `reason` `read_failed`; the other aliases run.
- `book_risk` `household:<currency>`, one per currency: the sum of the complete aliases' totals (`worst_loss_<currency>`, two decimals) when every alias row of the currency is complete, status `ok`; else status `unknown`, no total, fact `incomplete_aliases` (JSON list). Fact `basis_note`.
- `fit` `<candidate key>:<alias>`, one per ok `short_put` row and alias with funds; `underlying_id` of the candidate row; `legs` copied from it; `checks` the one `fit_capital` check as `{"gate","status","reason"}` (the form of the candidate rows); status `ok` for pass, `blocked` for block, `unknown` otherwise; facts those of the fitter. The candidate's underlying is the `issuer` fact of its row, its currency USD, its class from the table; `spot` and `iv` come from its facts. With no candidate rows only book rows are written; with no book at all nothing is written and the run succeeds.
- Rule versions `risk_v1` and `fit_v1`; `param_hash` is the SHA-1 hex of the sorted `key=value` lines of `max_mark_age_sessions` and the class table.
- Failed run codes: `read_candidates_failed`, `write_results_failed`; the error text starts `bookrisk: <code>`.

## Tests

`internal/jobs/bookrisk` `job_test.go`: `TestBookRiskNewWritesOneRowPerAliasAndHousehold`, `TestBookRiskNewHouseholdIsSumOfCompleteAliasTotalsPerCurrency`, `TestBookRiskNewIncompleteAliasGivesHouseholdNoTotal`, `TestBookRiskNewAliasWithoutCaptureIsNoCapture`, `TestBookRiskNewBookRowHasUnderlyingIdZeroAndStatusByCompleteness`, `TestBookRiskNewFitRowsOnlyForOkCandidates`, `TestBookRiskNewAliasWithoutValuesGetsNoFitRow`, `TestBookRiskNewOneAliasReadFailureKeepsTheOthers`, `TestBookRiskNewRerunWritesANewRunId`; `load_test.go`: `TestBookRiskNewIndexClassLegIsCashSettledWithEmptyTerms`, `TestBookRiskNewCashSettledFromTermsFields`, `TestBookRiskNewCaptureOnlyPositionIsNotInLedger`, `TestBookRiskNewLegsEqualRiskPageStructureLegs`.
