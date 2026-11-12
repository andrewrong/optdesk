# earnings_refresh

## earnings_refresh

`earningsrefresh.NewEarningsRefreshFunc(map[string]sources.EarningsSource, store.EarningsDateWriter, store.EarningsDateReader, []jobs.WatchEntry, clock.Clock)` builds the job `earnings_refresh` (implementation: `internal/jobs/earningsrefresh.New`); the design is `docs/design/event-sources.md#Decisions` (1, 4, 5, 7, 12). The map holds one source per market, keyed `US` or `JP`.

`jobs.WatchEntry{Issuer, Market, ReportDateOverride, BuyPrice, SalePrice string; Contract broker.Contract}` is one `[[underlying]]` of `watch.toml`. `buy_price` (`BuyPrice`) and `sale_price` (`SalePrice`) are decimal text in the currency of the contract, empty is unset, and the owner's word wins over any automatic value. `New` refuses an entry with an empty or duplicate issuer or a market other than `US` or `JP`.

- Id `earnings_refresh`; schedule `XNYS`, `session_close`, offset 30 minutes; key `{obs.earnings_date, <issuer>}` per entry.
- One row per entry, attempt 1: `listed` (earliest date on or after the day) or `absent` from the source of its market; `unsupported` with source `none` for a market with no source; for an entry with a `report_date_override` only a `listed` row with source `owner` and no vendor row.
- One `Fetch` per source per run. A market that has a vendor row of the day (`error` included) is not fetched; a second run of the day writes no row for an entry that has one. A source error writes an `error` row (`error_code` of the source, `unknown` when it gives none) for each entry of that market only, the other market's rows are still written, and the run returns a `Permanent` error with a fixed text; a second run returns the same error. No retry the same day.
- The vendors answer for today only: in development mode (owner decision 2026-11-12) a day other than the clock's date in the manifest zone (`America/New_York`) runs with the clock's date as its day. Test: `TestPastDayRunsClockDay`.
- `Returned` is the rows written, `Usable` those that are not `error`. `received_at` comes from the injected clock.

## Contract tests

- `internal/jobs/earningsrefresh/run_test.go`: `TestListedTakesEarliestOnOrAfterDay`, `TestMarketWithoutSourceIsUnsupported`, `TestOwnerOverrideWins`, `TestSourceErrorWritesErrorRows`, `TestOneFetchPerSourcePerRun`, `TestSecondRunSameDayMakesNoCall`, `TestErrorDayNotRetried`, `TestRateLimitedRecordedNoRetry`, `TestSecondRunWritesNoRow`, `TestSecondRunAfterErrorReturnsPermanent`.

## Version history

- v1: `earnings_refresh`, `jobs.WatchEntry`, `earningsrefresh.NewEarningsRefreshFunc`.
