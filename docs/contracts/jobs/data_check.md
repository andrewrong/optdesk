# data_check

## data_check

`datacheck.New(datacheck.Stores, clock.Clock)` builds the job `data_check` (implementation: `internal/jobs/datacheck`); the design is `docs/design/data-check.md`, entry 6. It only reports: it repairs, deletes and overwrites nothing.

- Id `data_check`. Key: one `{datacheck.run, "check"}` per day; the plan hash is the hash of the id, the manifest version and the day.
- Manifest `job.toml`: `id`, `version`, `[schedule]` calendar `XNYS`, anchor `session_close`, zone `America/New_York`, offset `120m`, `run_bound = "10m"`, `restart_sensitive = false`.
- `Stores`: `Cov` (`coverage.Reader`), `Probe` (`finding.Probe`) and `Out` (`finding.Writer`) are required; `Expected` is the list of datasets a plan must exist for; `RuleVersions` (dataset id to the rule version its writer has now) may be nil, and `candidate_rule_version` then returns the error `no_expected_version`; `Announce` may be nil. `New` returns an error that names the missing key.
- Windows: for each calendar of `Expected` and of `XNYS` and `XTKS`, the last `CheckWindowSessions` (5) session days before the check day, read through `Cov.Sessions` over `coverage.LookbackDays`.
- Rules: `defaultRules()` returns nine rules in order: the six of `docs/design/data-check.md`, then three of `docs/design/data-check-2.md` (entry 4), all read-only:
  - `candidate_rule_version`: severity `warn`; the newest candidate run of each kind whose `RuleVersion` differs from `RuleVersions["candidates.option_candidate"]` (an empty version, rows that disagree, differs); dataset `candidates.option_candidate`, day of the run; evidence `rows`.
  - `earnings_coverage`: severity `warn`; per market (US on the XNYS window, JP on the XTKS window) and window day with check rows, any `error` row (evidence `names`, `listed`, `errors`), or, with no error, `listed` 0 where the previous window day with rows of that market had `listed` above 0 (evidence `names`, `listed`, `listed_before`); dataset `obs.earnings_date`. `absent` rows are no input.
  - `bar_ohlc_shape`: severity `error`; a day of the XNYS or XTKS window with trades bars that break low <= open, close <= high; dataset `obs.price_bar`; evidence `rows`, `bad`. A run with no rule returns the error `no_rules` (`Permanent`) and writes nothing. A rule that returns an error becomes one finding of severity `unknown`, evidence `read_failed` 1, with the rule id as rule and as dataset (a `Rule` names no dataset), on the check day.
- Dedupe: a `gap_unclosed` finding whose (dataset, day) equals a `capture_gap` finding of the same run is dropped; every other finding is kept.
- Store: one run through `Out.InsertRun` (`RulesTotal` = rules, `RulesUnknown` = rules that failed). `Returned` is the number of rules, `Usable` the rules that did not fail; all rules failing is `Usable` 0 with no error, which the runner treats as a failed run.
- Message: from the findings of the stored run with `IsNew`, one Chinese text: a header, at most 10 lines ordered `错误` before `缺口` before `无法判断` (rule label, dataset, day, counts), then `还有 N 条见页面` when more than 10. No new finding: no call to `Announce`. The text holds fixed words, datasets, days and digits only.
- Notice: after a successful send (or when nothing is new) `Out.MarkNoticed` is called once. A failed send returns a `Retryable` error and calls no `MarkNoticed`. With a nil `Announce` the job calls `MarkNoticed` never and logs one line.
- Retry classes: a session read, an insert or a notice error is `Retryable`; a run cut through its context writes nothing and is `Retryable`.

## Version history

- v1: the job with no rule.
- v2: the rules `candidate_rule_version`, `earnings_coverage` and `bar_ohlc_shape`; `Stores.RuleVersions`.
