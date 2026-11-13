# statement_capture

## Version history

- v1: `statement_capture`: `statementcapture.New(cfg statementcapture.Config) (contracts.Job, error)` builds the job (docs/design/statement-account.md#2 statement-capture-job). `Config` holds `Aliases` (the statement-sourced aliases), `Sections` (`flexsection.Reader`), `Account` (`store.AccountWriter`), `Marks` (`mark.Writer`), `Calendar` and `Clock`; `New` fails when one is missing.

## Key

One capture per alias and report day, id `<alias>:<YYYY-MM-DD>:flex`, `SessionDay` the report date, `Status` `ok`, `At` the newest `RecordedAt` of the `OpenPosition` rows and the `EquitySummaryByReportDateInBase` row of that day. The plan has one key (`account.capture`, `flex`).

## Guard

The job writes only for the aliases of `Config.Aliases`, because readers take every row of (alias, day) and two captures of one day would be mixed (Decision 3). It never writes for another alias; keeping the gateway job away from those aliases is the wiring's work.

## Day rules

- A day is written when `OpenPosition` and `EquitySummaryByReportDateInBase` are both `present` in the newest statuses of the alias, the EquitySummary has a row of the report date and the OpenPosition has a row of it. Otherwise nothing is written and the log line names the reason, never a value.
- The job considers every stored report date up to the run day, newest first. A report date that `Calendar.Session` does not give as an XNYS session day is skipped and counted, not failed. A capture whose id exists is left as it is (the writer is insert-only), so a revised statement is not applied.
- A report date of another form than the two below refuses all days of the alias for that run, as the day it belongs to is unknown.

## Dates

`reportDate` and `expiry` are read as `YYYYMMDD` or `YYYY-MM-DD` and written as `YYYY-MM-DD`; any other text refuses the day.

## Mapping

- Per conid the `SUMMARY` row is used alone when it exists. A conid with only `LOT` rows is combined: `quantity` is the sum of `position`, the unit cost the average of `costBasisPrice` weighted by `position`, rounded to 6 places; a sum of zero skips the conid (counted); the other attributes come from the first lot row in `RowKey` order; lots that differ in `currency` or `markPrice` refuse the day. A row of another level is skipped and counted.
- `assetCategory` `STK` gives `stock`, `OPT` gives `option`; any other category is skipped and counted.
- Symbol rule: `symbol` of a stock is the attribute `symbol`; `symbol` of an option is the attribute `underlyingSymbol` (the readers take `Position.Symbol` as the underlying); an option without it refuses the day.
- `putCall` `P` and `C` give `put` and `call`; `quantity` = `position`; `avg_cost` = unit cost for a stock and unit cost times `multiplier` for an option, in exact decimal arithmetic; `multiplier` of a stock is 1; `exchange` is empty.
- Values: `NetLiquidation` = `total` and `TotalCashValue` = `cash` of the EquitySummary row, in the one `currency` of the `ChangeInNAV` rows; none or two different ones refuse the day.
- A required attribute that is missing or not a decimal refuses the day: nothing is written for it and the result carries a Permanent error whose text names the day and the attribute, never a value.
- Marks: after a capture, one mark per written position through `Marks.WriteMarks`: `Attempt` 1, `Basis` `close`, `Price` and `Close` = `markPrice`, `Currency` the position's, `Status` `ok`, `MarkedAt` = `At`, `SessionDay` the report date. A store error of either writer is Retryable.

## Roles

- Reads the Flex sections with the role `ot_w_optionledger` (`OT_PG_DSN_OPTION_LEDGER`, SELECT on the section views).
- Writes the capture with `ot_w_account` (`OT_PG_DSN_ACCOUNT`, INSERT and SELECT).
- Writes the marks with `ot_w_posmark` (`OT_PG_DSN_POSMARK`, INSERT and SELECT).

## Schedule

`job.toml`: `version = 1`, `id = "statement_capture"`, `[schedule]` calendar `XNYS`, anchor `session_close`, offset `95m`, `run_bound` `10m`, `restart_sensitive` false.
