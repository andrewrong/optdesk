# jobs contract

## Responsibility

The jobs the binary runs. `fredseries.New` builds the `fred_series` job: one FRED series (DGS10) fetched through a `sources.Source` and written through a `store.SeriesWriter`. The job implements `contracts.Job`; the constructor has the type `jobs.NewFredSeriesFunc`, and `jobs.Registry` lists the jobs the binary runs.

`New` returns an error when the source, the writer or the clock is nil, and when the embedded manifest is invalid.

## Inputs

- `internal/jobs/fredseries/job.toml`, embedded: `version`, `id`, `series`, and one `[window]` table (`rule = "after_close"`, `lateness_hours = 96`, `calendar = "XNYS"`), read with github.com/pelletier/go-toml/v2. The fields are `version`, `id`, `series` (the list of series ids) and the `[window]` table with `rule`, `lateness_hours` and `calendar`. A manifest without an id or without a series list fails with the error "fredseries: manifest needs an id and a series list".
- A `sources.Source`, a `store.SeriesWriter` and a `clock.Clock`, passed to `New`.
- A `contracts.Day` and, for `Run`, the keys of `Plan`.

## Outputs

- `Plan(day)`: one `ExpectedKey{obs.series_point, <series id>}` per series, sorted by id, and the plan hash: the lowercase hex SHA-256 of the job id, the manifest version, the day and the sorted series ids, joined with a newline.
- `Run`: a `JobResult`; `Returned` is the points the source gave, `Usable` those with a value. Points are written in one `InsertSeriesPoints` call; a `no_value` point keeps its nil value and status.
- Errors of `Run` are `contracts.TypedError`, through `asTyped`, which returns an error unchanged when it is already typed and wraps it with a kind otherwise:
  - a fetch error that is a `contracts.TypedError` is returned unchanged, else it is `Permanent`; nothing is written.
  - a failed `InsertSeriesPoints` is `Retryable` unless the writer's error is already typed (`asTyped`), in which case its own kind is kept; `Usable` is 0 then.

## Invariants

- The source gets a `FetchPlan` with the series ids of the keys and `From` and `To` equal to the day.
- No write happens when the fetch fails or returns no points.
- The `[window]` keys: `rule` is required and `after_close` is the only accepted value; `lateness_hours` is an integer of at least 0 (absent means 0) and becomes the close offset in hours; `calendar` is an optional string and becomes the calendar id (absent means empty). `New` fails on a bad table; the error names the key and no value.
- The job answers the optional method `Window() (calendarID string, openOffset, closeOffset time.Duration)` with those values; the open offset is always 0. `contracts.Job` and `internal/jobs/api.go` are unchanged.
- The window rule is carried, not applied; the runner hookup belongs to the follow-up design.
- The package imports contracts, clock, sources, store and the `jobs` root only.

## option_eod

`optioneod.New(broker.Broker, store.InstrumentResolver, store.OptionEodWriter, clock.Clock)` builds the job `option_eod` (type `jobs.NewOptionEodFunc`) from its embedded `job.toml`; the design is `docs/design/option-jobs.md`. `Plan` gives one key `{obs.option_eod, SYMBOL}` per `[[underlying]]`, sorted, and a hash of the day and every manifest setting. A `Run` of one key: a `day` other than the day of the clock in the manifest `zone` is `NoValue` with "wrong day" and nothing else happens; before `freeze_after` (default 1615) it answers an empty result with a nil error and calls no port. Otherwise it takes the underlying's price (`Last`, else `Close`, else `UndPrice` of one `Snapshot(KindSnapshot)`; none is `Retryable`), selects from the chain the nearest `max_expiries` expirations on or after the day and the strikes inside the `strike_band` with at most `max_strikes_per_side` per side (an empty selection is `Permanent`), resolves each pair with `ResolveContract` (a pair not listed gets no row), resolves the instrument ids (the option with the 21-character `osi` identifier) and asks `Snapshot(KindOptionEod)` in chunks of `chunk`, writing the rows of a chunk before the next. The clock is read before every broker call; once `budget_minutes` (default 8) have passed the run stops with `Retryable` and keeps the rows written. All rows of a key carry one deterministic `capture_id`, `uuid.NewSHA1(uuid.NameSpaceOID, day|dataset|subject)`, and `snapshot_kind` `eod`. Status: OK and PARTIAL 1, NO_DATA 2, NO_SUBSCRIPTION, TIMEOUT, ERROR and DISCONNECTED 5 with the IBKR code or the status name as `error_code`. `Returned` is the rows written, `Usable` those of status 1; a writer error is `Retryable` with `Usable` 0. Errors of the broker and the resolver keep their kind, an untyped one is `Permanent`. The `[window]` lateness defaults to 5 hours and is answered by `Window()`.

`optioneod.NewWithWatch(broker.Broker, store.InstrumentResolver, store.OptionEodWriter, []jobs.WatchEntry, clock.Clock)` (type `jobs.NewOptionEodWatchFunc`) builds the same job with the underlyings of the US stock entries of the watch list (`Market` US, `Contract.SecType` STK; the `[[underlying]]` tables of `job.toml` are not used; an empty result or a repeated symbol is an error). Three optional manifest keys apply only to it: `min_dte` and `max_dte` (calendar days from the run day, both inclusive; with `max_dte` set the expiries are all those with DTE in `min_dte..max_dte` instead of the nearest `max_expiries`; `job.toml` sets 21 and 45) and `max_strikes` (default 24). The strikes of an entry with a `BuyPrice` run from `0.80 x BuyPrice` to `max(1.05 x BuyPrice, spot)`, exact decimal arithmetic; when more than `max_strikes` fall in the range the nearest `BuyPrice` are kept, a tie keeping the lower strike; `strike_band` and `max_strikes_per_side` do not apply to it. An entry without a `BuyPrice` keeps the `strike_band`. `optioneod.New` ignores the three keys and behaves as before; a manifest without them has the same plan hash as before. `optioneod.NewWithWatchWindow(broker.Broker, store.InstrumentResolver, store.OptionEodWriter, []jobs.WatchEntry, jobs.OptionEodWindow, clock.Clock)` (type `jobs.NewOptionEodWatchWindowFunc`) behaves as `NewWithWatch` and replaces `min_dte`, `max_dte` and `max_strikes` of the manifest with each non-zero field of `jobs.OptionEodWindow{MinDTE, MaxDTE, MaxStrikes int}`; a zero field keeps the manifest value, and a window with `min_dte` above `max_dte` gives the manifest validation error. `NewWithWatch` is unchanged. `jobs.NewOptionEodHeldFunc` (the `store.AccountReader` comes after the watch list, before the clock) is declared only; it has no implementation yet.

`optioneod.NewJP(broker.Broker, store.InstrumentResolver, store.OptionEodWriter, clock.Clock)` (type `jobs.NewOptionEodFunc`) builds the job `option_eod_jp` from its embedded `job_jp.toml`; the design is `docs/design/option-capture-jp.md#Decisions`. It has one underlying, N225 (`IND`, `OSE.JPN`, JPY, `con_id` 14087999), zone `Asia/Tokyo`, window calendar `XTKS` with lateness 2 hours, slot `time` 15h50m in `Asia/Tokyo` on `XTKS`, `freeze_after` 1550. Three optional top-level manifest keys, absent from `job.toml` (whose plan hash is unchanged); each is added to the plan hash only when set:

- `trading_class`: when set, the chain is the first of that trading class on the underlying's exchange, none is an empty selection, `Permanent`; `job_jp.toml` sets `225`.
- `ledger_dataset`: the dataset id of the plan keys, default `obs.option_eod`; `job_jp.toml` sets `obs.option_eod_jp`; the table written stays `obs.option_eod`.
- `stop_after`: HHMM in the zone; a run at or after it, with the right day, is `NoValue` with "after session" and calls no port; `job_jp.toml` sets 1700.

## Schedule

Each of the three jobs holds a `[schedule]` table in its embedded `job.toml`, read with go-toml/v2 beside `[window]`, and answers three optional plain-value methods (standard-library types only; no job package imports `internal/runner`, and a job has no preflight, which `cmd/ot` owns):

- `Schedule() (calendarID, anchor, zone string, offset time.Duration)`, from the keys `calendar`, `anchor`, `zone`, `offset`. `anchor` is `session_open`, `session_close` (`zone` must be empty) or `time` (`zone` is required and `offset`, in `[0, 24h)`, is the wall time since midnight in it). `offset` is a Go duration string such as `-30m`; absent means 0.
- `RunBound() time.Duration`, from `run_bound` (a duration, at least 0; absent means 0).
- `RestartSensitive() bool`, from `restart_sensitive` (default false).

A manifest without the table has no schedule: `Schedule()` returns empty strings and `RunBound()` 0. A table that is present needs `calendar` and a valid `anchor`. `New` fails on a bad table; the error names the key (`schedule.anchor`, `schedule.zone`, `schedule.offset`, `schedule.run_bound`, `schedule.calendar`) and no value.

Proposed slots, confirmed by the owner at review: `option_eod` `session_close` `0s`, bound 16m (8 minutes plus its budget), restart-sensitive; `fred_series` `time` 07:30 in `America/New_York`, bound 5m, not restart-sensitive; `quote_snapshot` `session_close` `-30m`, bound 16m, restart-sensitive.

## compute_figures

`computefigures.New(jobs.ComputeStores, compute.Figures, compute.Rules, compute.Units, compute.SeriesOps, clock.Clock)` builds the job `compute_figures` (type `jobs.NewComputeFiguresFunc`); the design is `docs/design/compute-figures.md`. `jobs.ComputeStores` holds the seven store interfaces the job uses: `Catalog` (`store.OptionEodCatalog`), `OptionEod` (`store.OptionEodReader`), `Terms` (`store.OptionTermsReader`), `Names` (`store.InstrumentNameReader`), `SessionDays` (`store.SessionDayReader`), `Series` (`store.SeriesReader`) and `Readings` (`store.ReadingStore`). The job is declared here; its implementation is a later issue.

- Ease: `computefigures.NewWithEase` (type `jobs.NewComputeFiguresEaseFunc`) takes a `compute.Ease` after the `compute.SeriesOps` and before the clock; `New` is `NewWithEase` with nil. Only a job built with an `Ease` plans the key `{readings.reading, "macro.ease|market|WALCL"}`, and its `Postcondition` is a reading row of the as-of day, ok or missing, like every other key; a job built with nil has no `macro.ease` key and runs no ease step. The job reads the seven series of `[ease]` in `job.toml` and stores the reading of `docs/design/ease-v2.md` Decision 7. The ease is stored after the market figures and after IVP and IVR, so its failure (a series read that fails leaves the key without a row, named by the postcondition) stops no other key.
- Keys: one `{readings.reading, "name|subject_kind|subject"}` per (rule, subject). `subject_kind` is `underlying` (subject is the symbol of `InstrumentNames`, else `underlying:<id>`) or `market` (subject is the FRED id of the first input).
- Steps, in order: IV30, then the market figures, then IVP and IVR from the stored IV30 readings, so a missing IV30 gives a missing IVP with `no_input`. A step that fails does not stop the others; its keys are written missing or stay absent, and the postcondition names them.
- As-of day: the slot day. `Postcondition`: every planned key has a reading row for the as-of day, ok or missing.
- Iv30 input rules (Decision 10): subjects are the underlyings of `OptionEodCatalog.OptionEodLatest(ctx, day)` that have a row of the day itself in `OptionEod(ctx, id, day, day)`, named by `InstrumentNames`. Of one `instrument_id` the row with the latest `received_at` is used, and only `status` 1. `OptionTerms` is asked in chunks of 500 ids; a row without a current terms version is dropped and counted. The mid is (bid+ask)/2 when both are present, `ask >= bid` and `ask > 0`, else that side is absent; an IV absent or not above 0 is 0. `T` runs from the close of the as-of day's session row (calendar `XNYS`) to the `close_at` of the expiration day's session row, or its `open_at` for an `AM` settlement; an expiration day that is not a session day drops that expiry; minutes are floored. The rate is the newest `DGS3MO` point not after the as-of day; none gives a missing `no_input`; a point more than 4 calendar days before the as-of day (`max_age_days` of `market.iv30_own`) gives a missing `stale_input` (stored `missing_input`, `reason_code` 3); a point of an earlier day within 4 calendar days is used and the parameter `stale_rate` is true.
- Retry classes: a store error is `Retryable`; a bad rule (an unknown id or version, a rule the core rejects) is `Permanent`. A figure the core marks missing is stored `missing_input` with its reason, never skipped.

## SeriesHistoryLoader

`jobs.SeriesHistoryLoader` has `Load(ctx, sources.FetchPlan) ([]jobs.SeriesLoaded, error)`; `jobs.SeriesLoaded` is `{SeriesID string; Fetched int}`; the constructor type is `jobs.NewSeriesHistoryFunc` (`serieshistory.New(sources.Source, store.SeriesWriter)`). The design is `docs/design/compute-fred-series.md#series-history-loader`; `ot series-load` only wires it.

- `Load` refuses an empty series list, `From` after `To` and a range over 5 years as `Permanent`, before any fetch.
- One `Fetch` per series over `From`..`To`; every series is fetched before the first write, so a source error writes nothing.
- The points of a series are written once, with one `InsertSeriesPoints` call (idempotent), and `Load` returns the count fetched per series.
- A source or writer error is passed on by class only: a `TypedError` keeps its kind, an untyped fetch error is `Permanent`, an untyped write error `Retryable`; the text is not passed on. Only `contracts.TypedError` values are returned.
- No clock and no environment are read.

## calendar_refresh

`jobs.NewCalendarRefreshFunc(clock.SessionSource, store.SessionDayStore, clock.SessionDayDiffer, clock.Clock, jobs.CalendarObserver)` builds the job `calendar_refresh` (implementation: `internal/jobs/calendarrefresh.New`, a later issue); the design is `docs/design/compute-calendar.md`. `jobs.CalendarObserver` is `func(clock.SessionDayChange)`.

- Id `calendar_refresh`. Key: one `{master.session_day, "XNYS"}`.
- Span: from the day of the injected clock minus 7 to that day plus 400, inclusive. `Run` reads the stored rows and the fresh rows of the source over the span, takes the diff with the differ, upserts only added and changed rows through the store and deletes none.
- Retry classes: an empty fresh answer is `Permanent` (nothing is overwritten with nothing). A source error keeps the class the source gave: exit 1, exit 2 and any other nonzero exit of the command are `Permanent`, a failure to start or wait for the process (a kill at the run bound included) is `Retryable`. A store error is `Retryable` unless already typed.
- `Returned` and `Usable` are the fresh rows read and checked.
- `Postcondition`: the stored rows cover every day from the as-of day to the span end; false until the first successful refresh, true again on a no-change day.
- Observer: called once per changed day, in day order, with the `clock.SessionDayChange`; the wiring writes one log line per call (`component=calendar_refresh`, day, kind added or changed, before and after). It is the only trace of the diff; no table holds it.

## earnings_refresh

`jobs.NewEarningsRefreshFunc(map[string]sources.EarningsSource, store.EarningsDateWriter, store.EarningsDateReader, []jobs.WatchEntry, clock.Clock)` builds the job `earnings_refresh` (implementation: `internal/jobs/earningsrefresh.New`); the design is `docs/design/event-sources.md#Decisions` (1, 4, 5, 7, 12). The map holds one source per market, keyed `US` or `JP`.

`jobs.WatchEntry{Issuer, Market, ReportDateOverride, BuyPrice, SalePrice string; Contract broker.Contract}` is one `[[underlying]]` of `watch.toml`. `buy_price` (`BuyPrice`) and `sale_price` (`SalePrice`) are decimal text in the currency of the contract, empty is unset, and the owner's word wins over any automatic value. `New` refuses an entry with an empty or duplicate issuer or a market other than `US` or `JP`.

- Id `earnings_refresh`; schedule `XNYS`, `session_close`, offset 30 minutes; key `{obs.earnings_date, <issuer>}` per entry.
- One row per entry, attempt 1: `listed` (earliest date on or after the day) or `absent` from the source of its market; `unsupported` with source `none` for a market with no source; for an entry with a `report_date_override` only a `listed` row with source `owner` and no vendor row.
- One `Fetch` per source per run. A market that has a vendor row of the day (`error` included) is not fetched; a second run of the day writes no row for an entry that has one. A source error writes an `error` row (`error_code` of the source, `unknown` when it gives none) for each entry of that market only, the other market's rows are still written, and the run returns a `Permanent` error with a fixed text; a second run returns the same error. No retry the same day.
- `Returned` is the rows written, `Usable` those that are not `error`. `received_at` comes from the injected clock.

## Version history

- v1: `fred_series`, manifest version 1.
- v1: `option_eod`, manifest version 1.
- v1: `compute_figures`, interface only (`jobs.ComputeStores`, `jobs.NewComputeFiguresFunc`).
- v1: `compute_figures` stores `macro.ease` when built with an `Ease` (`jobs.NewComputeFiguresEaseFunc`, additive).
- v1: `SeriesHistoryLoader`, with `internal/jobs/serieshistory`.
- v1: `calendar_refresh`, interface only (`jobs.CalendarObserver`, `jobs.NewCalendarRefreshFunc`).
- v1: `option_eod` with a watch list, `min_dte`, `max_dte`, `max_strikes` (additive; `jobs.NewOptionEodWatchFunc`, `jobs.NewOptionEodHeldFunc`).
- v1: `jobs.OptionEodWindow`, `jobs.NewOptionEodWatchWindowFunc`, `optioneod.NewWithWatchWindow` (additive).
- v1: `option_eod_jp`, `trading_class`, `ledger_dataset`, `stop_after` (additive; `optioneod.NewJP`).
- v1: `earnings_refresh`, `jobs.WatchEntry`, `jobs.NewEarningsRefreshFunc`.
- v1: `option_ledger`, interface only (`jobs.OptionLedgerStores`, `jobs.OptionLedgerConfig`, `jobs.FlexRun`, `jobs.NewOptionLedgerJobFunc`, `jobs.ImportOptionTraderFunc`, `jobs.ImportResult`, `jobs.FlexAttributesSHA256Func`), with `optionledger.AttributesSHA256`.
- v1: `jobs.OptionLedgerStores.Sections` (`flexsection.Writer`, additive; `docs/design/account-flex-fx.md`): nil switches the Flex section step off, and the other fields and `jobs.NewOptionLedgerJobFunc` are unchanged. Delivery rule (Decision 10): nothing the section step handles reaches notify; a job result or `job_event` carries a job id, a status and an alias only, never a cash amount, balance, net value or position. Test: `internal/jobs/flex_section_api_test.go` `TestOptionLedgerStoresSectionsSet`, `TestOptionLedgerStoresSectionsNil`.

## Contract tests

- `internal/jobs/api_test.go`: `TestRegistryContract`, `TestNewComputeFiguresFuncContract`, `TestNewComputeFiguresEaseFuncContract`, `TestNewSeriesHistoryFuncContract`, `TestCalendarRefreshContract`, `TestCalendarRefreshErrorClasses`, `TestWatchEntryPrices`, `TestOptionEodWindowContract`, the interface.
- `internal/jobs/earningsrefresh/run_test.go`: `TestListedTakesEarliestOnOrAfterDay`, `TestMarketWithoutSourceIsUnsupported`, `TestOwnerOverrideWins`, `TestSourceErrorWritesErrorRows`, `TestOneFetchPerSourcePerRun`, `TestSecondRunSameDayMakesNoCall`, `TestErrorDayNotRetried`, `TestRateLimitedRecordedNoRetry`, `TestSecondRunWritesNoRow`, `TestSecondRunAfterErrorReturnsPermanent`.
- `internal/jobs/fredseries/plan_test.go`: `TestPlanHash`, `TestEmbeddedManifest`.
- `schedule_test.go` of each of the three packages: `TestEmbeddedSchedule`, `TestBadScheduleTable`.
- `internal/jobs/fredseries/window_test.go`: `TestWindowOf`, `TestWindowErrors`, `TestEmbeddedWindow`.
- `internal/jobs/fredseries/run_test.go`: `TestRunWritesOnce`, `TestPlanLateRunRecovery`.
- `internal/jobs/optioneod`: `TestPlan`, `TestManifestDefaults`, `TestSelect`, `TestRunWritesRows`, `TestCaptureIDStable`, `TestNotListed`, `TestBudget`, `TestGuards`, `TestErrors`, `TestWatchUnderlyingsUSOnly`, `TestDTEWindowSelectsMonthlies`, `TestNoWindowKeepsNearestExpiries`, `TestStrikeRangeFromBuyPrice`, `TestNoBuyPriceKeepsBand`, `TestMaxStrikesCap`, `TestJPManifestSlotAndWindow`, `TestJPPlanKeysUseLedgerDataset`, `TestPairsFiltersTradingClass`, `TestPairsWithoutTradingClassUnchanged`, `TestStopAfterReturnsNoValue`, `TestJPRowsCurrencyJPY`, `TestJPWrongDayNoRows`, `TestUSPlanHashPinned`, `TestWindowOverridesManifest`, `TestZeroWindowKeepsManifest`, `TestWindowMinAboveMaxFails`.

There is no test yet: the wrapping of an untyped fetch error as `Permanent` and the `Retryable` kind of a failed `InsertSeriesPoints` have no test (the fake writer never fails and no test gives the source an untyped error).

## Jobs and slots

`fred_series`, manifest version 1, window rule `after_close` with 96 hours lateness on calendar `XNYS`. Slots are assigned by the runner.

## Forbidden

- Importing another jobs package or pgx.
- `time.Sleep`, or reading the system time.
- Writing a missing value as 0.

## Option ledger

- `jobs.FlexAttributesSHA256Func` (implemented by `internal/jobs/optionledger.AttributesSHA256`, shared by the `option_ledger` job and the option-trader import): remove the keys `accountId` and `acctAlias`; encode the rest as one JSON object, keys ascending by byte order, no whitespace, strings in UTF-8 with only `"`, `\` and control characters below 0x20 escaped and no HTML escaping; the hash is the lower-case hex SHA-256 of that text. Control characters are written `\b \t \n \f \r` or `\u00xx`. `internal/jobs/optionledger/testdata/attrs_hash.json` lists `{attrs, canonical, sha256}` entries.
- `jobs.FlexRun` carries `Required`; a login whose token variable is empty reaches the job with a nil `Source`. The constructor accepts it and never fails for a missing token; `Run` returns a failed result when the login is `Required`, else skips it with the `degraded` log record.
- Contract tests: `internal/jobs/optionledger_api_test.go` `TestOptionLedgerJobContract`, `TestOptionLedgerImportContract`; `internal/jobs/optionledger/hash_test.go` `TestAttributesSHA256Fixture`, `TestAttributesSHA256Ignores`, `TestAttributesSHA256KeyOrder`.

## Cut-over

- `jobs.ImportAccountFunc`, `jobs.VerifyAccountFunc`, `jobs.CompareFiguresFunc` (implemented by `internal/jobs/cutover.ImportAccount`, `VerifyAccount`, `CompareFigures`; design `docs/design/cut-over.md`), with `jobs.VerifyResult`, `jobs.FigureStatus` (`checked`, `explained`, `unexplained`, `new`, `unchecked`), `jobs.FigureResult`, `jobs.FigureReport`; an import reuses `jobs.ImportResult`.
- `--verify` writes nothing: `VerifyAccountFunc` takes a `store.AccountReader`, which has no write method, and reports `Missing` and `Different` only.
- `internal/jobs/cutover` shares `jsonl.go` (JSON Lines reader whose errors carry the line number and never the content; canonical SHA-256 of a line with keys sorted, no space) between the import, the verify and the comparison. It holds no float type.
- Contract tests: `internal/jobs/api_test.go` `TestImportAccountFuncContract`, `TestVerifyAccountFuncContract`, `TestCompareFiguresFuncContract`; `internal/jobs/cutover/jsonl_test.go` `TestCanonicalHashSortsKeys`.

## Option candidates

- `jobs.CandidateStores` and `jobs.NewOptionCandidatesJobFunc` (implemented by `internal/jobs/optioncandidates.New`; design `docs/design/option-candidates.md`, Decisions 1, 4, 5, 9, 13, 15). The job `option_candidates` is anchored at `session_close` plus 30 minutes on `XNYS`.
- For each US stock entry of the watch list it reads the newest chain day of `option_eod`, the terms (in chunks of 500 ids), the earnings rows, the newest account captures of the aliases `A` and `B` and the series `DGS3MO`, converts floats to decimal text (shortest round-trip text, then half-even to 4 places), calls `Evaluate` with `compute.KindShortPut` and writes the rows through `store.CandidateWriter` under a new run id (one UUID text per `Run`).
- A nil `Earnings`, `Accounts` or `Series` reader gives `unknown` gates, not an error. A store error is a failed run whose error text starts `optioncandidates: <code>` (`read_session_days_failed`, `read_chain_catalog_failed`, `read_rate_failed`, `read_chain_failed`, `read_earnings_failed`, `read_account_failed`, `write_candidates_failed`). A JP entry gives no row; the job names it, and a US entry with no chain, through `Skipped() []string`.
- `CandidateStores` carries a `Catalog store.OptionEodCatalog` besides the stores of the design: it is how the job finds the instrument id and the newest chain day of each symbol (with `Names`).
- `New` holds no dividend reader and imports no `notify`: the margin cushion and the assignment cash never go to Telegram.
- Contract tests: `internal/jobs/api_test.go` `TestOptionCandidatesJobFuncContract`; `internal/jobs/optioncandidates` `TestRerunIsNewRun`, `TestOnlyUSEntries`, `TestStoreErrorFailsRun`, `TestJPEntryNamedNotSilent`, `TestNilEarningsIsUnknown`, `TestAliasesAandBAreSeparate`, `TestTermsChunkedAt500`, `TestStaleChainDay`, `TestSessionDaysSupplied`, `TestScheduleIsCloseAfterEod`, `TestFloatToDecimal`, `TestRateRatio`.
- `jobs.NewOptionCandidatesCallsFunc` (implemented by `internal/jobs/optioncandidates.NewWithCalls`; design `docs/design/option-candidates.md#Interface`, entry 11) is the same job as `New` (same `job.toml`, schedule, plan and `Skipped`; `NewOptionCandidatesJobFunc` and `New` are unchanged) plus a `dividend.Reader` of `internal/store/dividend`. A nil dividend reader is a constructor error; a nil `Earnings`, `Accounts` or `Series` still gives `unknown` gates.
- In one run it writes the `short_put` rows as `New` does, then the `covered_call` and `held_call_guard` rows under the same run id, in one `InsertCandidates` call. It reads the chain day for both rights, calls `Dividends(ctx, id, from, day)` with `from` 14 calendar days before the run day, takes the last row (no row gives a nil `Dividend`, so the guard is `unknown`), and sets `next_ex_date` and `ex_amount` of a call or guard row from a `present` row. `OpenShortIDs` are the sorted ids of the short `OPT` calls on the underlying in the newest capture of aliases A and B.
- A dividend read error is a failed run with the new code `read_dividend_failed`; nothing is written. Ranks are per kind: `rank` is set on the `ok` rows of `short_put` among puts and on the `ok` rows of `covered_call` among covered calls; `held_call_guard` rows have none.
- Contract tests: `internal/jobs/api_test.go` `TestOptionCandidatesCallsFuncContract`; `internal/jobs/optioncandidates` `TestCallsNeedDividendReader`, `TestPutsUnchangedByCalls`, `TestHeldCallGuardRows`, `TestRanksArePerKind`.

## bars_refresh

- `jobs.NewBarsRefreshFunc` (implemented by `internal/jobs/barsrefresh.New`; design `docs/design/market-bars.md`, Decisions 4, 6, 7 and 12): `func(broker.Broker, broker.HistoryClient, store.InstrumentResolver, bar.PriceBarWriter, bar.PriceBarReader, []WatchEntry, clock.Clock) (contracts.Job, error)`. The job body is a later entry.
- `barsrefresh.LoadManifest(data)` and `barsrefresh.DefaultManifest()` read `internal/jobs/barsrefresh/job.toml`. Keys: `version`, `id` (`bars_refresh`), `backfill_days` (400), `daily_days` (5), `restate_tolerance` (`"0.00001"`, a decimal text read as an exact `*big.Rat`), `adjusted_cadence_days` (7), `backfill_names_per_run` (10), `deepen_days` (1825), `deepen_piece_days` (730), `deepen_names_per_run` (0 is no cap), four `[[index]]` tables (SPX, NDX, RUT, N225: `symbol`, `sec_type`, `exchange`, `currency`, `con_id` 0). The index exchanges and symbols are proposals the owner sets; the loader does not check them against IBKR.
- Refusals name the key and never the value: a non-positive `backfill_days`, `daily_days`, `adjusted_cadence_days`, `backfill_names_per_run`, `deepen_days` or `deepen_piece_days`; a negative `deepen_names_per_run`; a `restate_tolerance` that is not a decimal; an `[[index]]` without `symbol`, `exchange` or `currency`; a schedule without `calendar`.
- Schedule: calendar `XNYS`, anchor `session_close`, offset `10m`, `run_bound` `10m`.
- Contract tests: `internal/jobs/api_test.go` `TestBarsRefreshContract`; `internal/jobs/barsrefresh/manifest_test.go` `TestManifestLoads`, `TestManifestRefusesBadDays`, `TestManifestRefusesBadTolerance`, `TestManifestRefusesIncompleteIndex`, `TestManifestDeepenZeroIsNoCap`.
