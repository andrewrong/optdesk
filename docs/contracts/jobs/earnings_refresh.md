# earnings_refresh

## earnings_refresh

`earningsrefresh.NewEarningsRefreshFunc(map[string]sources.EarningsSource, store.EarningsDateWriter, store.EarningsDateReader, []jobs.WatchEntry, clock.Clock)` builds the job `earnings_refresh` (implementation: `internal/jobs/earningsrefresh.New`); the design is `docs/design/event-sources.md#Decisions` (1, 4, 5, 7, 12). The map holds one source per market, keyed `US` or `JP`.

`jobs.WatchEntry{Issuer, Market, ReportDateOverride, BuyPrice, SalePrice string; Contract broker.Contract}` is one `[[underlying]]` of `watch.toml`. `buy_price` (`BuyPrice`) and `sale_price` (`SalePrice`) are decimal text in the currency of the contract, empty is unset, and the owner's word wins over any automatic value. `New` refuses an entry with an empty or duplicate issuer or a market other than `US` or `JP`.

- Id `earnings_refresh`; schedule `XNYS`, `session_close`, offset 30 minutes; key `{obs.earnings_date, <issuer>}` per entry.
- One row per entry, attempt 1: `listed` (earliest date on or after the day) or `absent` from the source of its market; `unsupported` with source `none` for a market with no source; for an entry with a `report_date_override` only a `listed` row with source `owner` and no vendor row.
- `Run` asserts its writer to `fundkey.EarningsUpserter` and returns a `Retryable` error naming the missing `UpsertEarnings` when it is absent. It upserts all rows of the run in one call with `writtenBy` `earnings_refresh:<day>`, through `ReplayEarnings` when the day is before the clock's date in the manifest zone (`America/New_York`). A re-run replaces what the source now says; the store keeps an answer an `error` row would replace.
- The day given names the answer rows and the filter on the report date; there is no development-mode override of the day. `Attempt` is 1.
- One `Fetch` per source per Plan: the job memoises per instance and day that it fetched and wrote, and a later `Run` of the same instance and day (the next key) returns the first result without a fetch. When the first key fails, the memo holds the failure and the later keys return it. A new instance (a forced console run plans again) fetches once more; the job cannot tell it from a first run.
- The job does not skip an entry that has a row of the day. A key that is present is kept from a second scheduled run by the runner, not by the job.
- A source error writes an `error` row (`error_code` of the source, `unknown` when it gives none) for each entry of that market only, the other market's rows are still written, and the run returns a `Permanent` error with a fixed text; with one key only that entry's market counts.
- `Returned` is the rows written, `Usable` those that are not `error`. `received_at` comes from the injected clock.

## Contract tests

- `internal/jobs/earningsrefresh/run_test.go`: `TestListedTakesEarliestOnOrAfterDay`, `TestMarketWithoutSourceIsUnsupported`, `TestOwnerOverrideWins`, `TestSourceErrorWritesErrorRows`, `TestOneFetchPerSourcePerRun`, `TestSecondRunSameDayMakesNoCall`, `TestRunWithoutUpserterIsRetryable`, `TestPastDayIsAnswerDayAndUsesReplay`, `TestSecondKeyOfOneInstanceDoesNotFetch`, `TestNewInstanceFetchesAgain`, `TestErrorAnswerForFailedMarketAsBefore`, `TestFailedFirstKeyIsMemoised`.

## Version history

- v1: `earnings_refresh`, `jobs.WatchEntry`, `earningsrefresh.NewEarningsRefreshFunc`.
