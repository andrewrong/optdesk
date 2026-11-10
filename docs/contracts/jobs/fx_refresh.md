# fx_refresh

## fx_refresh

- `fxrefresh.NewFxRefreshFunc` (implemented by `internal/jobs/fxrefresh.New`; design `docs/design/market-bars.md`, Decision 9): `func(broker.Broker, broker.HistoryClient, bar.FxRateWriter, bar.FxRateReader, clock.Clock) (contracts.Job, error)`.
- Id `fx_refresh`; one key `{obs.fx_rate, USDJPY}`; schedule `XNYS`, `session_close`, offset 15 minutes, `run_bound` 10 minutes; `job.toml` also holds `duration_days` (5).
- `Run` resolves the USD.JPY cash contract once through `Broker.ResolveContract` (sec type `CASH`, symbol `USD`, exchange `IDEALPRO`, currency `JPY`; not listed is Permanent), asks `MIDPOINT` for 5 days, and inserts one `obs.fx_rate` row per bar day not stored (`rate` is the close text, `source` `ibkr_midpoint`). A stored day is left as it is. An empty answer writes nothing and fabricates nothing; no default rate exists in the package. A history or store error is Retryable unless it is already typed; a close that is not a positive decimal is Permanent.
- Contract tests: `internal/jobs/fxrefresh/types_test.go` `TestFxRefreshContract`; `internal/jobs/fxrefresh` `TestOneKeyUSDJPY`, `TestInsertsOnlyNewDays`, `TestStoredDayLeftAsIs`, `TestEmptyAnswerWritesNothing`, `TestNotListedIsPermanent`, `TestPacedIsRetryable`, `TestNoDefaultRate`.
