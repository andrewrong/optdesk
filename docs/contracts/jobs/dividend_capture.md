# dividend_capture

## dividend_capture

`dividendcapture.NewDividendCaptureFunc(broker.ReferenceClient, store.InstrumentResolver, dividend.Writer, dividend.Reader, []jobs.WatchEntry, clock.Clock)` builds the job `dividend_capture` (implementation: `internal/jobs/dividendcapture.New`); the design is `docs/design/event-sources.md#Decisions` (2, 5, 6, 7).

- Id `dividend_capture`; schedule `XNYS`, `session_close`, offset 30 minutes, `run_bound` 10 minutes; key `{obs.dividend_expectation, <issuer>}` per entry. `New` refuses a nil argument, an entry with an empty or duplicate issuer and a market other than `US` or `JP`.
- `job.toml` holds `passed_markets`, the markets whose probe passed in `docs/design/event-sources-probe.txt`; the owner copies them there. It is the run-time input: an entry of a market not in it gets an `unsupported` row with source `none` and is not asked.
- `Run` resolves each entry to its `instrument_id` with the identifier scheme `ibkr_conid`, makes one `ReferenceClient.Dividends` call for the entries of the passed markets that have no row of the day, and writes one row per stock through `dividend.Writer`, attempt 1: `present` (parsable next ex-date and amount; decimal text from the tick), `no_value`, or `error` with `error_code` `timeout` (tick timeout), `ibkr_<code>` or `error_status` (error status), `request_failed` (the call failed) or `bad_answer` (wrong number of answers). Source is `ibkr_tick_456`, except `unsupported` and `resolve_failed` rows, which have `none`.
- A failed resolve is an `error` row `resolve_failed` for that stock only; the row is keyed by the `con_id` of the entry, since the writer refuses instrument id 0, and the stock is not asked. One stock failing does not stop the others. A stock with a row of the day gets no second row and no call. A writer error is Retryable, after the other rows are tried.
- In development mode (owner decision 2026-11-12) a day other than the date of the clock in the manifest zone (`America/New_York`) runs with the clock's date as its day: the tick has no history, so every row carries today's `session_day`. Test: `TestPastDayRunsClockDay`.
- Contract tests: `internal/jobs/dividendcapture/run_test.go` `TestEmbeddedManifest` (the constructor type), `TestPresentRow`, `TestNoValueRow`, `TestTimeoutIsErrorRow`, `TestUnpassedMarketIsUnsupported`, `TestOneStockFailsOthersWritten`, `TestRowKeyedByResolvedInstrument`, `TestSecondRunWritesNoRow`, `TestWholeCallFailureIsErrorRows`.

## Version history

- v1: `dividend_capture`, `dividendcapture.NewDividendCaptureFunc`, `dividendcapture.New` (`passed_markets` in `job.toml`).
