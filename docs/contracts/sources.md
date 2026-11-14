# Contract: sources

## Responsibility

Fetch series from external data sources and return typed records with provenance. The first client is FRED (`internal/sources/fred`, series DGS10). It opens no database and knows neither the runner nor pages.

## Inputs

- `sources.FREDConfig`: `BaseURL`, `APIKey` (a string passed in by the caller; the package reads no environment), `Clock` (`clock.Clock`).
- `sources.FetchPlan`: series ids and an inclusive date range.
- FRED `series/observations` JSON: `{realtime_start, realtime_end, date, value}` per observation.

## Outputs

- `[]contracts.SeriesPoint` (`obs.series_point r1`), never a hand-written copy. A FRED value `"."` becomes a nil value with status `no_value`; other rows have status `present`. The unit and currency of each series come from the table `seriesUnits` in `mapping.go` only. Credit and funding series (pinned, read on the FRED pages on 2026-11-13): `BAMLC0A0CM`, `BAA10Y` and `SOFR` are `percent` (FRED units "Percent"); `WRESBAL` is `ccy_amount` `USD` (FRED units "Millions of U.S. Dollars", shift 6). Funding and financial-conditions series (pinned, liquidity-view Decision 5): `IORB` and `SOFR99` are `percent` ("Percent"); `NFCI` and `STLFSI4` are `index_points` ("Index"); `TLAACBW027SBOG` is `ccy_amount` `USD` ("Billions of U.S. Dollars", shift 9); `WLCFLPCL` is `ccy_amount` `USD` ("Millions of U.S. Dollars", shift 6). A FRED series of unit `ccy_amount` is stored in whole units of its currency (`currency` set, e.g. `USD`): the decimal point of the FRED text is shifted on the text before it becomes a float64 (millions 6 places, billions 9). A pinned series (`pinVintage`) keeps the first value seen: `realtime_start`, `realtime_end` and `source_revision` are the observation date, so a second fetch of the same observation is the same store key. `realtime_start`, `realtime_end` and `published_at` (the fetch instant on the injected clock) are kept, so the vintage is preserved; `source` is `fred`, `source_revision` is `realtime_start`.
- Errors are `contracts.TypedError` only: HTTP 429, 5xx, transport failures and cancellation are `Retryable`; other non-200 statuses, undecodable bodies and bad configuration are `Permanent`.

## Invariants

- At most one request per second per client (`requestInterval`), measured on the injected clock; the wait is a cancellable timer, never `time.Sleep`.
- The API key never appears in an error text or log line.
- Pure `net/http`; no `internal/` import other than contracts, clock and sources.

## Version history

- r1: FRED client for one series (issue 28).

## Contract tests

`internal/sources/fred`: `TestMapping`, `TestFixtureMapping`, `TestShiftDecimalText`, `TestPinnedVintageSameKeyAcrossFetchDays`, `TestPacing`, `TestErrorKinds`, `TestRedaction`; the wiring `var _ sources.NewFREDFunc = New`.

## Jobs and slots

None. A job (`internal/jobs/fredseries`) calls `Source.Fetch`; this module schedules nothing.

## Forbidden

Opening the database; knowing the runner or pages; reading the environment; a live call in a test.

## Flex (IBKR) declarations

Declared in `internal/sources/api.go` (declarations only; the client and loader are `internal/sources/flex`): `FlexSource`, `FlexRequest`, `FlexReport`, `FlexStatement`, `FlexConfig`, `NewFlexFunc`, `FlexFile`, `FlexLogin`, `LoadFlexFileFunc`, `ErrFlexNotConfigured`, `ErrFlexConfig`. The package reads no environment: the wiring reads the variables `flex.toml` names and passes the values.

### File shape of `$OT_HOME/flex.toml` (Decision 3)

- The file is outside any Git work tree and has no group or other permission bit.
- `[[login]]`: `name` (`[a-z][a-z0-9_-]{0,31}`, unique), `token_env` (the NAME of the environment variable holding the token), `trade_queries` (a non-empty list of unique query ids, digits), `required` (boolean, default false: the login that serves the main account).
- The loader (`internal/sources/flex`) refuses a file with no required login, or with more than one.
- Tokens and account numbers live only in the environment; they never appear in a log, error, fixture or row.

### Flex sections (container rule, Decision 3)

`FlexStatement.Sections` is keyed by row element name; each row is a text map of every attribute as sent (`accountId` and `acctAlias` included; the job drops them). `Trades` and `OtherLevels` are unchanged. The container rule: for each direct child of a statement other than `Trades`, if it has child elements, each child is a row under the child's element name (`<CashReport>` holds `<CashReportCurrency>` rows: key `CashReportCurrency`); if it has none, its own attributes are one row under its own name (`<ChangeInNAV .../>`: key `ChangeInNAV`, one row). A section the report lacks has no key.

### Error kinds (Decision 4)

- `LoadFlexFileFunc`: `ErrFlexNotConfigured` (no file) and `ErrFlexConfig` (bad or unsafe file); texts name entry and field only, never a value.
- `FlexSource.Fetch`: `contracts.TypedError` values naming the kind `network`, `http_status`, `flex_error`, `in_progress`, `too_large` or `not_report` and the code; none holds the token, the query id or an address. A login whose token variable is empty fails alone as `not_configured`.

## Earnings (Alpha Vantage) declarations

Declared in `internal/sources/api.go`: `EarningsSource`, `EarningsConfig` (`BaseURL`, `APIKey`, `Symbols`, `Doer`, `Sleep`), `EarningsListing`, `NewEarningsFunc`, `HTTPDoer`, `CodedError`. The client is `internal/sources/alphavantage` (`New`). The package reads no environment: the wiring reads `ALPHA_VANTAGE_API_KEY` and passes it in. `Symbols` are the JP codes of the J-Quants source; Alpha Vantage ignores them.

- `Fetch(ctx)` makes exactly one `GET /query?function=EARNINGS_CALENDAR&horizon=3month&apikey=...` over the injected `HTTPDoer` (nil: an `http.Client` whose `CheckRedirect` returns `http.ErrUseLastResponse`). It never retries and follows no redirect (a 3xx is a `http_status` error); Decision 12 of `docs/design/event-sources.md` limits the job to one `Fetch` a day.
- The body is read through `io.LimitReader` (16 MiB); only parsed rows (`EarningsListing`) are returned; the raw csv is never logged or kept. Columns are found by header name (`symbol`, `reportDate` required; `fiscalDateEnding`, `estimate`, `timeOfTheDay` optional); a row with no symbol or no date is skipped.
- Errors are `CodedError` values (`contracts.TypedError` plus `ErrorCode()`): a JSON notice body (wrong key or used-up quota) and HTTP 429 are `rate_limited`, never an empty result; a body that is not csv (or lacks the required header names) is `not_csv`; fewer than 500 data rows is `short_calendar`; other statuses are `http_status` (code only); transport failure and cancellation are `network`. The key is only in the query and never in an error: a transport error is replaced by a fixed message.
- Contract tests (`internal/sources/alphavantage`): `TestParseByHeaderName`, `TestNoticeBodyIsError`, `TestShortCalendarIsError`, `TestKeyNeverInError`, `TestOneCallPerFetch`, `TestNoticeIsRateLimitedNoRetry`; `TestEarningsSourceContract` in `internal/sources`; the wiring `var _ sources.NewEarningsFunc = New`.

## Earnings (J-Quants) declarations

Declared in `internal/sources/api.go`: `NewJQuantsFunc` (same `EarningsSource` and `EarningsConfig`). The client is `internal/sources/jquants` (`New`). The package reads no environment: the wiring reads `JQUANTS_API_KEY` and passes it in.

- `Fetch(ctx)` makes one `GET /v2/fins/earnings-date?code=<4 digits>0` per entry of `EarningsConfig.Symbols`, with the key only in the header `x-api-key` (never with `Authorization`), over the injected `HTTPDoer` (nil: an `http.Client` that follows no redirect). Before every call after the first it waits 12 s (`callGap`: at most 5 calls in any 60 s, the Free plan). A 429 waits 61 s (`retryWait`) and asks the same symbol once more, not spaced by the 12 s gap; a second 429 returns `rate_limited` of kind `Retryable`. 5xx, `network`, other 4xx and bad bodies are not retried. The first-failure rule stays: the first failed symbol returns at once and no later symbol is called. Both waits go through `EarningsConfig.Sleep` (nil: a timer and `ctx.Done()`, returning `ctx.Err()`); a `Sleep` error is the `network` code (`jquants: request cancelled`, `Retryable`). Alpha Vantage ignores `Sleep`.
- Every row with a `SchDate` is one `EarningsListing` (no choice of "the day" here; the job picks the earliest on or after its session day). The 5-digit `Code` is cut to 4. A row with an empty `SchDate` and a symbol with no row yield no listing, which the job records as `absent`. The body is read through `io.LimitReader`; only parsed rows are returned.
- Errors are `CodedError` values: 429 is `rate_limited`; other statuses (400, 401, 403, 5xx) are `http_status` (code only); a `pagination_key` in a body is `unexpected_pagination`; a body that is not JSON is `not_json`; transport failure and cancellation are `network`. The key is never in an error: a transport error is replaced by a fixed message.
- Contract tests (`internal/sources/jquants`): `TestJQuantsParse`, `TestJQuantsReturnsEveryDatedRow`, `TestJQuantsFiveDigitCodeSent`, `TestJQuantsEmptyBodyIsAbsent`, `TestJQuantsEmptyDateIsAbsent`, `TestJQuantsKeyNeverInError`, `TestJQuantsAuthErrorIsTyped`, `TestJQuantsBadRequestIsTyped`, `TestJQuantsPaginationKeyIsError`, `TestJQuantsOneCallPerSymbol`, `TestJQuantsRateLimitedRetriesOnce`, `TestJQuantsRateLimitedTwiceFails`, `TestJQuantsServerErrorNotRetried`, `TestJQuantsPacesCalls`, `TestJQuantsSleepCancelled`, `TestJQuantsRealSleepCancelled`; `TestNewJQuantsFuncContract` in `internal/sources`; the wiring `var _ sources.NewJQuantsFunc = New`.

## Backup source (NY Fed)

`internal/sources/nyfed` (`New(Config)`, a `sources.Source`) is a backup for the series `SOFR` only: it fills gaps and never replaces a value (the job writes its points with `FillSeriesPoints`). It needs no key; the one outside host is `markets.newyorkfed.org` (read-only), a constant default `DefaultBaseURL` that a test replaces with a local server through `Config.BaseURL`.

- `Fetch` makes one `GET /api/rates/secured/sofr/search.json?startDate=&endDate=`. Any other series id is a `Permanent` error naming it. Each record of `refRates` becomes a point: `source` `nyfed`, unit `percent`, status `present`, `value` the parsed `percentRate`, `realtime_start`, `realtime_end` and `source_revision` the `effectiveDate`; dates outside `From` and `To` are dropped. A missing day has no point.
- Errors are `contracts.TypedError` only: 429, 5xx, transport failures and cancellation are `Retryable`; other statuses and a body that does not parse are `Permanent`. A record whose date or rate is missing, empty or not a number makes the whole answer `Permanent` and returns no point.
- The package imports no other source package. Contract tests: `TestMapping`, `TestUnknownIDIsPermanent`, `TestMalformedBodyIsPermanent`, `TestServiceUnavailableIsRetryable`, `TestTransportFailureIsRetryable`, `TestBadRecordFailsWholeAnswer`. The fixture `testdata/sofr.json` is hand-written from the reading of the API's answer, not a recorded response.
