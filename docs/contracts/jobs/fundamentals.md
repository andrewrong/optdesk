# fundamentals

## fundamentals

- `fundamentals.NewFundamentalsFunc(sources.FactSource, store.FactWriter, []jobs.WatchEntry, clock.Clock)` (implemented by `internal/jobs/fundamentals.New`; design `docs/design/valuation.md#fundamentals-job`) builds the job `fundamentals`. A nil source, writer or clock is a constructor error.
- `job.toml`, embedded: `version`, `id`, `lookback_days` (14), a `[window]` table (`rule = "after_close"`, `calendar = "XNYS"`, `lateness_hours = 96`) and a `[schedule]` table (anchor `time`, zone `America/New_York`, calendar `XNYS`, with `run_bound`); the table shapes and error texts are those of `fredseries` with the package name `fundamentals`.
- `Plan(day)` gives one key `{obs.fundamental_fact, <issuer>}` per entry of Market `US` whose Issuer starts with `cik:`, sorted, and a hash of the job id, version, day, lookback and the sorted issuers.
- `Run(day, keys)` makes one `FactSource.Fetch` with `FactPlan{Subjects, From: day - lookback_days, To: day}` and writes the facts in one `InsertFacts` call. `Returned` and `Usable` are the facts fetched. An untyped fetch error is `Permanent`, a typed one keeps its kind, a writer error is `Retryable` with `Usable` 0; no facts means no write. A second run of the day adds no row (the store key is the eight-column key). The job keeps no cursor file.
- Skips: an entry of Market `US` with an empty or non-`cik:` Issuer, or of any market with an empty Issuer, is skipped with reason `no_cik`; one with an `edinet:` Issuer of any market (a real one is `JP`) with reason `edinet_not_fetched`. The job implements `fundamentals.FundamentalsSkips`: `Skipped() []fundamentals.FundamentalsSkip{Subject, Reason}` in watch-list order, Subject the Issuer or, when empty, `Contract.Symbol`. No CIK is guessed.
- Contract tests: `internal/jobs/fundamentals/types_test.go` `TestNewFundamentalsFuncContract`; `internal/jobs/fundamentals/run_test.go` `TestRunTwiceAddsZero`, `TestNoCikSkippedAndNamed`, `TestEdinetSkippedWhileNil`, `TestPlanOneKeyPerCik`, `TestFetchErrorKinds`, `TestWriterErrorRetryable`, `TestEmbeddedManifest`.
- v1: `fundamentals`, manifest version 1.
