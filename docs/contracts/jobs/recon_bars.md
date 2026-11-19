# recon_bars

## recon_bars

`reconbars.NewReconBarsFunc(sources.BarSource, store.InstrumentResolver, bar.PriceBarReader, reconstore.Writer, reconapi.Func, []jobs.WatchEntry, clock.Clock)` builds the job `recon_bars` (implementation: `internal/jobs/reconbars.New`); the design is `docs/design/base-data-page.md`, entry 21 (pair R6). The compare function is `recon.Compare`, wired by entry 23; the job imports no implementation package of compute.

- Id `recon_bars`. Key: one `{recon.run, "R6"}` per day.
- Manifest `job.toml`: `id`, `version`, `[schedule]` calendar `XNYS`, anchor `session_close`, offset `30m` (after `bars_refresh`, whose slot is 10m with a 10m bound), `run_bound = "30m"` (must be longer than 2m); `[source] id` (`yfinance`, the only source setting; any value is returned unchanged, the wiring picks the source by it); `[compare]` `close_rel` (`0.001`) and `tolerance_text`, stored on the run. The job makes no threshold and no level.
- Span: the window of the run day and the 6 days before it (5 weekdays). Pairs are the watch entries of market US and JP by symbol and currency of the contract, each once; an entry with no con id is left out. The job resolves each instrument with scheme `ibkr_conid`, reads the winner rows of basis `trades` of the window with `PriceBars`, makes ONE `DailyBars` call with a context deadline of `run_bound` minus 2 minutes counted from the clock, and no batch of its own.
- Compare: keys are `symbol|currency|day`; `Skip` holds the keys of our rows of each unanswered pair with its code, `OtherMissingSkipped` is true. One run is written at the end through `reconstore.Writer` with `pair_id` `R6`, `PairsAsked`, `PairsCompared`, the counts per kind, the items that are not `match` (most severe first, at most `reconstore.RunItemsMax`, `Company` the symbol) and the source id of the manifest.
- Status: all pairs answered is `ok`; some unanswered is `partial` with the reason code of the first unanswered pair other than `no_data` (empty when all are `no_data`); a source error is `failed` with a code (`rate_limited`, `network`, `timeout` or `source_changed`, taken from a `Code() string` of the error, `timeout` for a deadline, else `network`) and no items, never a pass.
- Retry classes: a source error keeps the class the source gave (`Retryable` when untyped) and a failed run is written first; a resolver error is `Permanent` unless typed; a reader or writer error is `Retryable` unless typed; a run cut through its context writes nothing and returns `Retryable`.
- `Returned` is the compared and skipped keys, `Usable` the compared ones (`N`).
- Running twice for the same day writes two runs; there is no force and no time guard, any past day can be run.
- `Postcondition`: the job has no method of its own; the key is present when `Run` returned no error, that is, when its run row was inserted.

## Version history

- v1: `recon_bars`, R6 on the yfinance source.
