# bars_refresh

## bars_refresh

- `barsrefresh.NewBarsRefreshFunc` (implemented by `internal/jobs/barsrefresh.New`; design `docs/design/market-bars.md`, Decisions 4, 6, 7 and 12): `func(broker.Broker, broker.HistoryClient, store.InstrumentResolver, bar.PriceBarWriter, bar.PriceBarReader, []WatchEntry, clock.Clock) (contracts.Job, error)`. The job body is a later entry.
- `barsrefresh.LoadManifest(data)` and `barsrefresh.DefaultManifest()` read `internal/jobs/barsrefresh/job.toml`. Keys: `version`, `id` (`bars_refresh`), `backfill_days` (400), `daily_days` (5), `restate_tolerance` (`"0.00001"`, a decimal text read as an exact `*big.Rat`), `adjusted_cadence_days` (7), `backfill_names_per_run` (10), `deepen_days` (1825), `deepen_piece_days` (730), `deepen_names_per_run` (0 is no cap), four `[[index]]` tables (SPX, NDX, RUT, N225: `symbol`, `sec_type`, `exchange`, `currency`, `con_id` 0). The index exchanges and symbols are proposals the owner sets; the loader does not check them against IBKR.
- Refusals name the key and never the value: a non-positive `backfill_days`, `daily_days`, `adjusted_cadence_days`, `backfill_names_per_run`, `deepen_days` or `deepen_piece_days`; a negative `deepen_names_per_run`; a `restate_tolerance` that is not a decimal; an `[[index]]` without `symbol`, `exchange` or `currency`; a schedule without `calendar`.
- Schedule: calendar `XNYS`, anchor `session_close`, offset `10m`, `run_bound` `10m`.
- Contract tests: `internal/jobs/barsrefresh/types_test.go` `TestBarsRefreshContract`; `internal/jobs/barsrefresh/manifest_test.go` `TestManifestLoads`, `TestManifestRefusesBadDays`, `TestManifestRefusesBadTolerance`, `TestManifestRefusesIncompleteIndex`, `TestManifestDeepenZeroIsNoCap`.
