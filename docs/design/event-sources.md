# Event sources: earnings dates and ex-dividend dates with amounts, stored and readable per underlying

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-09
Revision: 4 (adversarial review `events-adversarial.md` answered in revision 2; revision 3 added the free-plan call budget; revision 4 settles the J-Quants request shape, gives `watch.toml` one owner with the option price fields, and fixes the issue order; revision 1 was 7e88e93)

Source: advisor 2026-11-08: earnings dates and ex-dividend dates are blocking dependencies of the option-candidate gates, and an early-assignment and ex-dividend guard for short calls is a missing seller need. Restart plan v2.5 section 1 and 11, row W5-6 (`Reference.Dividends`, tick 456, "scheduled only if the probe passes") and 11.1 item 4 (tick 456 on Japanese stocks, 日股). The owner dropped the scanner and Stooq.
Read on main at 3aac781: `AGENTS.md`, `docs/design/modules.md`, `docs/design/ease-v2.md`, `docs/design/account-data.md` (on `origin/design/account-data`), `internal/{sources,broker,jobs,store}/api.go`, `contracts/datasets/obs.quote_snapshot.r1.json`, `migrations/0007_quote_snapshot.sql`, `pyside/src/optdesk/ibkr/{api,spec}.py`.
Read in option-trader (read only; no secret or data copied): `GLOSSARY.md` (vendor results date, dividends), `README.md`, `refdata/datasets.toml` (`dividend_expectation`), `sources/alphavantage/earnings_calendar.py`, `docs/adr/0015-reference-data.md` (Dividend expectations), `candidates/call_inputs.py`.

Goal: two stored, readable facts per underlying: the next results date and the next ex-dividend date with its amount per share. The candidate gates and the short-call guard read them through store Readers; this design builds the sources, the tables and the jobs, not the gates.

## What main holds today

- Nothing: no dataset, table, source or job for either fact. `pyside` asks IBKR for generic ticks `100,101` and `104,106` only (`ibkr/spec.py`); `contracts/proto/ibkr/v1` has no reference service and no `ContractKey` message.
- No watch list: the only underlyings are `[[underlying]]` of `internal/jobs/optioneod/job.toml` (SPY, a manifest struct, not a string list). The valuation draft (`docs/design/valuation.md`) takes its subjects from the caller and defines no list either, so this design defines the one shared list (Decision 1).
- Roles are in `internal/store/setup/setup.go`, DSN names in `internal/store/api.go` (`DSNEnvQuoteSnapshot` and the others), read in `cmd/ot/runonce.go`. `internal/store/api_test.go` is 800 lines. `internal/contracts/gen/records.go` is generated and `scripts/ratchet/main.go` skips generated files, so it needs no ceiling and no report.
- Old system (option-trader): dividends from IBKR tick 456 per stock, stored only (`dividend_observation`, no currency in the tick; it waited about 10 s and raised a timeout when a non-payer never answered); earnings from the Alpha Vantage `EARNINGS_CALENDAR` csv, one call a day, `listed` or `absent` per issuer (SEC CIK), used by short put gate 4 only when the time text was `pre-market` or `post-market`, the source at most 2 days old and the date inside an SEC-filing estimated window. FMP was never used. Owner 2026-11-09: "先用alpha vantage" for earnings; the old FMP and yfinance dates drifted, so every vendor date is inherently uncertain.

## Decisions

1. One owner-edited list, `$OT_HOME/watch.toml`: one `[[underlying]]` per company with `issuer` (`cik:<10 digits>` for a US company, `edinet:<code>` for a Japanese one), `market` (`US` or `JP`), and the IBKR contract fields (`symbol`, `sec_type`, `exchange`, `currency`, `con_id`) and three optional fields: `report_date_override` (`YYYY-MM-DD`, the owner's own word for one company, see Decision 5), `buy_price` and `sale_price` (decimal text in the currency of the contract, positive, at most 4 places, `sale_price` not below `buy_price`; empty is unset; read by the option-candidates design, which adds no field of its own). Typed `jobs.WatchEntry{Issuer, Market, ReportDateOverride, BuyPrice, SalePrice string; Contract broker.Contract}` in `internal/jobs/api.go`. One owner of the format: this design. The `cmd/ot` loader (issue 5) is the only parser, and its contract in `docs/contracts/ot.md` is the format's definition; a consumer takes `[]jobs.WatchEntry` from it. The valuation design, which reads the same file through its own loader and type (a module below `jobs` in the layers may not import it), follows that contract; a revision of that draft, not made here. `watch.toml` is the one list and the only place for these prices. Every row below is keyed by `issuer`, with the symbol kept as the vendor or IBKR sent it, so share classes (BRK.B, BRK-B) do not split a company.
2. Dividends: IBKR tick 456 through `pyside`, as the old system and plan v2.5 row W5-6 have it; no new account. The tick is four comma-separated values (past 12 months, next 12 months, next ex-date `YYYYMMDD`, next amount). IBKR does not say declared or projected, so the guard treats every ex-date as projected.
3. Earnings: Alpha Vantage `EARNINGS_CALENDAR` (`horizon=3month`), ported from the old source (header found by name; a notice body for a wrong key or used-up quota is an error, never `absent`; fewer than 500 rows is refused). Confirmed by the owner 2026-11-09; it is the only source the old system had; IBKR gives no earnings date through `pyside`. A new outside service for optdesk: `second-opinion case: new outside service`. The key `ALPHA_VANTAGE_API_KEY` is read by the `.env` loader and passed in; it never enters an error, log or row.
4. What a gate may do with these rows (the gates are the next design; this is the contract they get). A vendor earnings date is never a company confirmation. Until a design ports the SEC estimated window, a `listed` date can only refuse or mark a candidate, never clear it; the date is `unconfirmed` unless its `time_of_day` is `pre-market` or `post-market` and the row is at most 2 days old (the old rule's two tests, without the window). `absent`, `unsupported` and `error` all mean unknown, never "no results before expiry", and an unknown or missing date gates as blocked (the candidate waits; it is not cleared); with `horizon=3month` an expiry beyond three months cannot be judged either.
5. Japan, earnings: Alpha Vantage does not list Japanese companies. The JP route is J-Quants `GET https://api.jquants.com/v2/fins/earnings-date` (JPX's own API, all listed companies with the date history, fields `PubDate` and `SchDate`; the `/v2/equities/earnings-calendar` endpoint lists only March and September fiscal year companies, so it is not used). Owner 2026-11-09: "a, 我应该有账户，我之前注册过" (an account exists). Auth of the current V2 API is one header `x-api-key` and nothing else (V2 refuses `x-api-key` with `Authorization`; the V1 mail, password and refresh token flow is gone). The env key is `JQUANTS_API_KEY`, read by the `.env` loader and passed in, never in an error, log or row; the owner puts the value from the J-Quants dashboard in the service `.env`. option-trader never used J-Quants (no match in its code), so no earlier key name exists to reuse. The entry's `symbol` is the 4-digit securities code of a JP stock, matched to the vendor's code after the 5-digit code is cut to 4. Rows have `source` `jquants`; the pagination key is followed until empty or the 5-page cap of Decision 12 (then `error` `page_cap`); an empty `SchDate` (undecided) is `absent`. A new outside service: `second-opinion case: new outside service; owner's word 2026-11-09 given`. The JPX 決算発表予定日 Excel is left out, not a fallback: it is the same JPX data a day behind the API, its terms for automated download were not found, and it adds a second source, an xlsx dependency and a layout that changes per period for a case the gate already covers (a failed or missing J-Quants answer writes `error` or `absent`, which gates as blocked per Decision 4, never as cleared). The J-Quants account is the free plan (owner 2026-11-09: "免费版，所以要省着调用"), so calls are budgeted (Decision 12) and the plan's data delay and window shape the gate (Decision 12). A JP company without a date gets `absent` (companies can be missing for weeks), and the gate sees unknown, so blocked. `report_date_override` is only an owner override for one company, stored as a `listed` row with `source` `owner` and winning over the vendor's row for that day. For dividends the probe covers JP symbols (plan 11.1 item 4 asks for them); a market whose probe fails gets `unsupported` dividend rows.
6. Probe first, checkable: `ot probe dividends` asks IBKR for a fixed list: three US payers, one US non-payer, one US ETF, two JP payers. Per symbol: `ok` (parsable next ex-date), `no_value`, `timeout` (nothing in 10 s) or `error`. Per market the last lines are `probe: US pass|fail` and `probe: JP pass|fail`; a market passes when every payer is `ok`; for the non-payer and the ETF `no_value` and `timeout` are both fine, `error` is not. The owner runs it on the gateway machine (plan 11.1) and commits the output as `docs/design/event-sources-probe.txt`. `dividend-datasets` and later dividend issues Depend on `dividends-probe` and check that file in Acceptance 1 (`grep -c "^probe: US pass" ...` prints 1); if it prints 0 the worker stops and reports, it does not edit the file. Plan 11.1 stop rule: a failed market is not scheduled.
7. Shapes (append-only; an error is a row, a later success is a new attempt):
   - `obs.earnings_date` r1: key `(observed_on, issuer, attempt)`; `symbol`, `market`, `status` (`listed`, `absent`, `unsupported`, `error`), `report_date` (earliest listed date on or after `observed_on`), `fiscal_period_end`, `time_of_day`, `estimate` (currency not stored), `error_code`, `source` (`alphavantage`, `jquants`, `owner`, `none`: a status `unsupported` row has source `none`, since no source produced it; `source` is never null), `received_at`.
   - `obs.dividend_expectation` r1: key `(session_day, instrument_id, attempt)`; `issuer`, `status` (`present`, `no_value`, `unsupported`, `error`), `past_12m`, `next_12m`, `next_amount` (per share), `next_ex_date`, `currency` (of the contract asked, not a claim of the tick), `error_code`, `source` (`ibkr_tick_456`, or `none` for a status `unsupported` row; never null), `received_at`.
   - Readers: `EarningsDates(ctx, issuer, from, to)` and `Dividends(ctx, instrumentID, from, to)`, inclusive, ordered by day, only the highest `attempt` of a day, empty slice for none.
8. Role `ot_w_events` (INSERT on both tables, plus SELECT on the views `rd.earnings_date_r1` and `rd.dividend_expectation_r1`, so the job can read the rows of the day for the once-a-day rule of Decision 12; no UPDATE, DELETE or TRUNCATE) is added in `internal/store/setup/setup.go` with `store.DSNEnvEvents = "OT_PG_DSN_EVENTS"`, read in `cmd/ot/runonce.go` and listed in the preflight. `ot_ro_pages` reads too. `ot setup pg` and `ot migrate` for the new role are the owner's, run before the jobs (the job is off with a named line while the DSN line is missing).
9. The short-call guard needs the next ex-date and amount per share and `received_at` freshness; its rule (extrinsic value against the dividend) is the next design.
10. Order against `origin/design/account-data`, which edits the same files. Rule: an events issue that edits a file named here is `after` the account-data issue that edits it, and both keep a Files-bounded diff. Shared files and the account-data issues: `generate.sh`, `records.go`, docs/contracts datasets (`account-proto`); `internal/broker/api.go` (`account-broker-api`); `internal/store/api.go` (`account-store-api`); `setup.go` (`account-store-migration`, which also takes migration 0016); `cmd/ot/runonce.go`, `wire.go` (`account-wire`); `transport.py` (`account-guard-id`); `server.py` (`account-pyside-executions`). Migration numbers are written "next free at cut time" everywhere. `store/api.go` is 490 lines plus account-data's: the store issues check it stays under 600 and, if not, report instead of splitting.
11. Cut order: earnings first (issues 1 to 6 and 15, 16, no probe), the probe chain (7 to 10) in parallel with it, dividends (11 to 14) after the probe. Sixteen issues: the watch list, the dividend side and the two Japanese sources have their own issues; the gate is what stops wasted dividend work.
12. Call budget (owner 2026-11-09: the J-Quants account is the FREE plan, "免费版，所以要省着调用"; the Alpha Vantage key is the free tier, about 25 calls a day). The rule, the same for both earnings sources:
   - At most one outside call per job run. `earnings_refresh` makes one Alpha Vantage call (the one `EARNINGS_CALENDAR` csv covers all US issuers) and one J-Quants call, never one per symbol.
   - At most one run per day: the job is keyed by the `XNYS` session day and does nothing when a non-`error` row of the day exists for the source (an `unsupported` row has source `none` and counts toward no source, so it never stops a call); a manual `run-once` on the same day does not call again either.
   - No automatic retry loop, inside a run or across runs of a day. A failure is recorded as an `error` row (attempt 1) and gates as blocked (Decision 4) until the next day's run. A 429 or any limit answer (J-Quants 429, the Alpha Vantage notice body) is recorded as `error` with `error_code` `rate_limited` and is not retried.
   - J-Quants, one request: the source makes one bulk request without `code` (all listed companies, pages followed by the `pagination_key` cursor; pages count as one logical call and are capped at 5 per run, then `error` `page_cap`). There is no per-symbol fallback: the source takes no issuer list and the job has no `refetch_days` or `max_calls` key. The v2 parameters could not be read offline, so the owner confirms the bulk request from his own dashboard docs and commits `docs/design/event-sources-jquants.txt` with the line `request: bulk` (and a line `delay_days: N` or `delay_days: unknown`); issues 15 and 16 check that file in Acceptance 1 and stop and report when it is absent. If the owner's docs show the endpoint takes `code` only, the design is revised before issue 15 is cut; a worker never chooses a request shape.
   - Free plan delay and window: the J-Quants free plan is known to serve data with a delay (about 12 weeks for the price data; the delay for this endpoint is unverified) and a limited history. The gate rule: a J-Quants date is `unconfirmed` (Decision 4) whenever `received_at` minus the vendor's own publication stamp is not known to be under 2 days; with the free plan that is every JP date until `delay_days` in the owner's file is a number under 2 (Open question 3), so JP dates can only refuse or mark a candidate, never clear it, and a JP company whose date is missing because of the lag is `absent`, which gates as blocked. The job stores the date as listed and leaves that judgement to the gate (`source` `jquants` is the signal).

## Modules

| issue | module | order |
|---|---|---|
| 1 earnings-datasets (interface) | contracts | after account-proto |
| 2 earnings-store (interface and implementation) | store | after 1, account-store-api, account-store-migration |
| 3 earnings-source (interface and implementation) | sources | after 1, 2 |
| 4 earnings-job (interface and implementation) | jobs | after 2, 3 |
| 5 watch-loader-earnings-wire | ot | after 4, 6, account-wire |
| 6 events-docs-modules | docs | after 4, 15 |
| 7 reference-proto (interface) | contracts | after 1 |
| 8 pyside-dividends (interface and implementation) | pyside | after 7, account-guard-id, account-pyside-executions |
| 9 broker-dividends (interface and implementation) | broker | after 7, account-broker-api |
| 10 dividends-probe | ot | after 5, 8, 9, account-wire |
| 11 dividend-datasets (interface) | contracts | after 10 and the probe file |
| 12 dividend-store (interface and implementation) | store | after 11, 2 |
| 13 dividend-job (interface and implementation) | jobs | after 12, 4, 6 |
| 14 dividend-wire | ot | after 13, 5, 16, account-wire |
| 15 jquants-earnings-source (interface and implementation) | sources | after 3 and the jquants file |
| 16 jquants-earnings-wire | ot | after 15, 5, 6, 10, account-wire |

## Interface

- contracts: datasets of Decision 7 (generated Go, docs); `reference.proto`: `service ReferenceService { rpc Dividends(stream ContractKey) returns (stream DividendProjection); }` as plan v2.5 line 170 (the plan's `TradingHours` rpc is not part of this design); `ContractKey` (symbol, sec_type, exchange, currency, con_id) is new in this file; `DividendProjection` has the key, `status`, the four values as optional text decimals, `error_code`, `received_at`.
- store: `EarningsDateWriter/Reader`, `DividendWriter/Reader`, `store.DSNEnvEvents`. sources: `EarningsConfig{BaseURL, APIKey, Clock}`, `EarningsRow`, `EarningsSource`, `NewAlphaVantageFunc`; `NewJQuantsFunc` (issue 15, same `EarningsSource`, declared in `internal/sources/api.go` by issue 15). broker: a separate `ReferenceClient` interface with `Dividends(ctx, []Contract) ([]DividendProjection, error)` and its own dial type `NewReferenceFunc` in `internal/broker/api.go`; `Broker` is unchanged, so `unreachableBroker` and the test fakes keep compiling. jobs: `WatchEntry`, `NewEarningsRefreshFunc`, `NewDividendCaptureFunc`, each taking `[]WatchEntry`.
- ot: owns the `watch.toml` format (Decision 1), defined in `docs/contracts/ot.md` by issue 5; the loader in `cmd/ot/watch.go` is its only parser and the valuation design follows the contract.
- pyside: `ReferenceService` servicer registered in `transport.py`; the Protocol in `ibkr/api.py`.

## Issues

Each entry: Module, Goal, Files, Design, Order, Acceptance, Checked by CI. "the diff line" is: `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the Files paths. Tests: `t.Parallel()`, `t.TempDir()`, fake clock, no sleep, no network, no live broker.

### 1 earnings-datasets (interface)
- Module: contracts. Size: M. Depends on: none.
- Files: contracts/datasets/obs.earnings_date.r1.json, contracts/generate.sh, internal/contracts/gen/records.go, docs/contracts/datasets/obs.earnings_date.md, contracts/fixtures/obs.earnings_date.r1.json, contracts/fixtures/MANIFEST.synthetic-v1.json, internal/contracts/schema_test.go, internal/contracts/api.go, internal/contracts/api_test.go
- Design: docs/design/event-sources.md#Decisions (7). Contracts: obs.earnings_date r1 (new). `records.go` is generated and skipped by the ratchet; nothing to report.
- Order: after `account-proto` (same generated files).
- Goal: schema with the status enum and the key of Decision 7, generated record exposed as `contracts.EarningsDate` in `internal/contracts/api.go` (the store interface names it), `source` enum `alphavantage`, `jquants`, `owner`, `none` (not nullable; the `unsupported` fixture has `none`), fixtures `listed`, `absent`, `unsupported`, `error` (synthetic), manifest hashes.
- Acceptance: 1. `grep -c "unsupported" contracts/datasets/obs.earnings_date.r1.json` is at least 1 and `grep -c "obs.earnings_date" internal/contracts/gen/records.go` is at least 1. 2. `go test ./internal/contracts -race -count=1` passes with `TestEarningsDateFixtureValues` once (a row per status; `absent`, `unsupported` and `error` have null `report_date`; the `unsupported` row has `source` `none`). 3. the diff line.
- Checked by CI: the generated-code check; `TestFixtureManifest`; `scripts/hygiene`.

### 2 earnings-store (interface and implementation)
- Module: store. Size: L. Depends on: earnings-datasets.
- Files: internal/store/api.go, internal/store/api_events_test.go, migrations/NNNN_events.sql, internal/store/events/earnings.go, internal/store/events/earnings_test.go, internal/store/events/earnings_pg_test.go, internal/store/setup/setup.go, internal/store/setup/setup_test.go, docs/contracts/store.md, arch.toml, .go-arch-lint.yml
- Design: docs/design/event-sources.md#Decisions (7, 8, 10). `arch.toml` and `.go-arch-lint.yml` are regenerated for the new `internal/store/events` package (a module plus repo-file pair). The contract test is a new file (`api_test.go` is 800 lines). NNNN is the next free at cut time.
- Order: after 1, `account-store-api`, `account-store-migration`.
- Goal: `EarningsDateWriter/Reader`, `DSNEnvEvents`, table and view `rd.earnings_date_r1`, role `ot_w_events` with INSERT and SELECT on the view `rd.earnings_date_r1` (the job reads the rows of the day), reader returns the highest attempt per day.
- Acceptance: 1. `grep -c "EarningsDateWriter\|EarningsDateReader\|DSNEnvEvents" internal/store/api.go` is at least 3 and `wc -l < internal/store/api.go` is under 600. 2. `go test ./internal/store/... -race -count=1` passes with `TestReaderTakesHighestAttempt` once; `grep -c "go:build pg" internal/store/events/earnings_pg_test.go` prints 1; `grep -c "ot_w_events" internal/store/setup/setup.go` is at least 1. 3. `grep -c "GRANT.*\(UPDATE\|DELETE\|TRUNCATE\)" migrations/*_events.sql` prints 0 and `grep -c "GRANT SELECT ON rd.earnings_date_r1 TO ot_w_events" migrations/*_events.sql` prints 1; the diff line.
- Checked by CI: ci-go; `go test -tags pg ./internal/store/...` (GitHub CI only); `go run ./scripts/arch --check`; the size rule.

### 3 earnings-source (interface and implementation)
- Module: sources. Size: M. Depends on: earnings-datasets, earnings-store. Second-opinion case: new outside service.
- Files: internal/sources/api.go, internal/sources/api_test.go, internal/sources/alphavantage/client.go, internal/sources/alphavantage/parse.go, internal/sources/alphavantage/client_test.go, internal/sources/alphavantage/testdata/calendar.csv, internal/sources/alphavantage/testdata/notice.json, docs/contracts/sources.md, arch.toml, .go-arch-lint.yml
- Design: docs/design/event-sources.md#Decisions (3, 12). `arch.toml` and `.go-arch-lint.yml` are regenerated for the new package (a module plus repo-file pair).
- Call budget (Decision 12): `Fetch` makes exactly one HTTP request, follows no redirect loop and never retries; a notice body is `error` with `error_code` `rate_limited`.
- Order: after 1, 2 (both regenerate `arch.toml` and `.go-arch-lint.yml`; issue 15 follows through 3).
- Goal: `EarningsSource.Fetch(ctx)` over recorded bodies in tests; the key only in the query; notice, non-csv and short bodies are typed errors.
- Acceptance: 1. `go test ./internal/sources/... -race -count=1` passes with `TestParseByHeaderName`, `TestNoticeBodyIsError`, `TestShortCalendarIsError`, `TestKeyNeverInError`, `TestOneCallPerFetch`, `TestNoticeIsRateLimitedNoRetry` once each (a fake HTTP client counts calls: exactly 1 per `Fetch`, also after a notice body or a 5xx). 2. `grep -c "time.Sleep\|http.Get\|os.Getenv" internal/sources/alphavantage/*.go` prints 0; `grep -c "EarningsSource" internal/sources/api.go` is at least 1. 3. the diff line.
- Checked by CI: ci-go; `go run ./scripts/arch --check`. No new module dependency (`encoding/csv`).

### 4 earnings-job (interface and implementation)
- Module: jobs. Size: M. Depends on: earnings-store, earnings-source.
- Files: internal/jobs/api.go, internal/jobs/api_test.go, internal/jobs/earningsrefresh/job.toml, internal/jobs/earningsrefresh/run.go, internal/jobs/earningsrefresh/run_test.go, docs/contracts/jobs.md, arch.toml, .go-arch-lint.yml
- Design: docs/design/event-sources.md#Decisions (1, 4, 5, 7, 12). `WatchEntry` carries every `watch.toml` field, the option prices included, so no other design edits it. `arch.toml` and `.go-arch-lint.yml` are regenerated for the new package (a module plus repo-file pair).
- Call budget (Decision 12): one `Fetch` per source per run, at most one run per session day, no retry; a day with a non-`error` row for a source is skipped without a call, and an `error` row of the day is not retried until the next day. The job takes one `EarningsSource` per market (a map keyed by `US` or `JP`); a market without a source gets `unsupported` with source `none`.
- Order: after 2, 3.
- Goal: `WatchEntry` (with `BuyPrice` and `SalePrice`, doc comments: decimal text, currency of the contract, empty unset, the owner's word wins over any automatic value; the contract test shows a caller reading both from a fake loader) and `NewEarningsRefreshFunc`; once per `XNYS` session day after the close, one fetch per market source, one row per entry: `listed` (earliest date on or after the day) or `absent` from the source of its market, `unsupported` (source `none`) for a market with no source, and a `report_date_override` as a `listed` row with `source` `owner` that wins; a source error writes an `error` row for each entry of that source's market only (attempt 1 of the day; no retry the same day, the next day's run starts again) and fails the run after the other market's rows are written.
- Acceptance: 1. `go test ./internal/jobs/... -race -count=1` passes with `TestListedTakesEarliestOnOrAfterDay`, `TestMarketWithoutSourceIsUnsupported` (the row has source `none`), `TestOwnerOverrideWins`, `TestSourceErrorWritesErrorRows`, `TestOneFetchPerSourcePerRun`, `TestSecondRunSameDayMakesNoCall`, `TestErrorDayNotRetried`, `TestRateLimitedRecordedNoRetry`, `TestWatchEntryPrices` once each, on fake sources that count calls (never more than 1 per source per run, 0 on a second run of the same day, 0 after an `error` row of that day). 2. `grep -c 'id = "earnings_refresh"' internal/jobs/earningsrefresh/job.toml` prints 1; `grep -c "BuyPrice\|SalePrice" internal/jobs/api.go` is at least 2 and `grep -c "buy_price\|BuyPrice" docs/contracts/jobs.md` is at least 1; `grep -c "time.Sleep\|time.Now()" internal/jobs/earningsrefresh/*.go` prints 0. 3. the diff line.
- Checked by CI: ci-go; `go run ./scripts/arch --check`; the size rule.

### 5 watch-loader-earnings-wire
- Module: ot. Size: M. Depends on: earnings-job, events-docs-modules.
- Files: cmd/ot/watch.go, cmd/ot/watch_test.go, cmd/ot/runonce.go, cmd/ot/runonce_test.go, cmd/ot/wire.go, cmd/ot/wire_test.go, cmd/ot/env.go, cmd/ot/preflight.go, cmd/ot/preflight_test.go, docs/contracts/ot.md
- Design: docs/design/event-sources.md#Decisions (1, 3, 8, 12). The loader is the one parser of `watch.toml`.
- Order: after 4, 6 (the `ot` import list of modules.md), `account-wire`.
- Goal: load `$OT_HOME/watch.toml` (an unknown market, a duplicate issuer, a malformed issuer, a malformed `report_date_override`, a `buy_price` or `sale_price` that is not a positive decimal with at most 4 places, or a `sale_price` below the `buy_price` of the same entry is a named refusal that names the issuer, never a path; empty price is unset), register `earnings_refresh` in `run-once` and the scheduler, read `OT_PG_DSN_EVENTS` and `ALPHA_VANTAGE_API_KEY` and list them in the preflight; the scheduler runs `earnings_refresh` once per session day and `run-once` makes no second call the same day (Decision 12); a missing key or DSN disables only this job with a named line; values never printed.
- Acceptance: 1. `go test ./cmd/ot -race -count=1` passes with `TestWatchLoad`, `TestWatchRefusals` (covers the override and the prices), `TestWatchPricesLoaded`, `TestEarningsRefreshRegistered`, `TestMissingKeyDisablesOnlyEarnings`, `TestEarningsRefreshOncePerDayScheduled` once each (fake source counting calls: one scheduler tick pair on one day gives 1 call). 2. `grep -c "watch.toml\|OT_PG_DSN_EVENTS\|ALPHA_VANTAGE_API_KEY" docs/contracts/ot.md` is at least 3 and `grep -c "buy_price\|sale_price" docs/contracts/ot.md` is at least 2; `grep -c "ALPHA_VANTAGE_API_KEY=[A-Za-z0-9]" cmd/ot/*_test.go | grep -vc ":0"` prints 0. 3. the diff line.
- Checked by CI: ci-go; `go run ./scripts/arch --check`.

### 6 events-docs-modules
- Module: docs. Size: S. Depends on: earnings-job, jquants-earnings-source.
- Files: docs/design/modules.md, arch.toml, .go-arch-lint.yml
- Design: docs/design/event-sources.md#Modules. `arch.toml` and `.go-arch-lint.yml` are derived from the `ot` import list of modules.md (a docs plus repo-file pair): with the packages present the generator writes the new `ot` dependencies into both, so `go run ./scripts/arch --check` fails if either is left out.
- Order: after 4 and 15 (the packages must exist), and after the other designs' edits of the same sentence.
- Goal: add `internal/sources/alphavantage`, `internal/sources/jquants`, `internal/jobs/earningsrefresh`, `internal/store/events` to the one-constructor and implementation lists (the dividend package follows in 13).
- Acceptance: 1. `grep -o "internal/sources/alphavantage\|internal/sources/jquants\|internal/jobs/earningsrefresh\|internal/store/events" docs/design/modules.md | sort -u | wc -l` prints 4. 2. `go run ./scripts/arch --check` passes. 3. the diff line.
- Checked by CI: the modules-table check of `crew issue check`; `go run ./scripts/arch --check`.

### 7 reference-proto (interface)
- Module: contracts. Size: S. Depends on: earnings-datasets.
- Files: contracts/proto/ibkr/v1/reference.proto, contracts/generate.sh, internal/contracts/genpb/ibkr/v1/reference.pb.go, internal/contracts/genpb/ibkr/v1/reference_grpc.pb.go, pyside/src/optdesk/contracts/genpb/ibkr/v1/reference_pb2.py, pyside/src/optdesk/contracts/genpb/ibkr/v1/reference_pb2.pyi, pyside/src/optdesk/contracts/genpb/ibkr/v1/reference_pb2_grpc.py, pyside/src/optdesk/contracts/api.py, pyside/tests/contracts/test_contracts_api.py
- Design: docs/design/event-sources.md#Interface. The proto path is owner-only (the owner merges it).
- Order: after 1.
- Goal: the service and messages of the Interface section, generated output with its source, and the re-export of the `ReferenceService` messages in the contracts `api.py` (the pyside servicer of issue 8 imports them only there).
- Acceptance: 1. `grep -c "rpc Dividends(stream ContractKey) returns (stream DividendProjection)" contracts/proto/ibkr/v1/reference.proto` prints 1; `grep -ci "account_id\|password\|token" contracts/proto/ibkr/v1/reference.proto` prints 0. 2. `git diff $(git merge-base HEAD origin/main) -- contracts/proto | grep -c '^-[^-]'` prints 0; `cd pyside && uv run pytest tests/contracts -q` passes. 3. the diff line.
- Checked by CI: the generated-code check; pyside and ci-go workflows.

### 8 pyside-dividends (interface and implementation)
- Module: pyside. Size: M. Depends on: reference-proto.
- Files: pyside/src/optdesk/ibkr/api.py, pyside/src/optdesk/ibkr/dividends.py, pyside/src/optdesk/ibkr/spec.py, pyside/src/optdesk/ibkr/ibgateway.py, pyside/src/optdesk/ibkr/transport.py, pyside/src/optdesk/ibkr/server.py, pyside/tests/contracts/test_ibkr_api.py, pyside/tests/ibkr/test_dividends.py, pyside/tests/ibkr/test_spec.py, pyside/tests/ibkr/test_transport.py
- Design: docs/design/event-sources.md#Decisions (2, 6)
- Order: after 7, `account-guard-id` (transport.py), `account-pyside-executions` (server.py).
- Goal: `dividends.py` requests tick 456 per contract with the existing release rule, waits at most 10 s on the injected clock (nothing: `timeout`), parses four values (empty is `no_value`, never zero), and `ReferenceService` is registered in `transport.py`; the test file defines its own fake gateway (the kit holds only `fakeclock.py`).
- Acceptance: 1. `cd pyside && uv run pytest tests/ibkr tests/contracts -q -n auto` passes with `test_parse_four_values`, `test_empty_tick_is_no_value`, `test_malformed_ex_date_is_no_value`, `test_silent_ticker_is_timeout`, `test_subscription_released`, `test_tick_456_in_spec` once each. 2. `cd pyside && uv run ruff check && uv run mypy --strict src tests` pass; `grep -c "sleep(" pyside/tests/ibkr/test_dividends.py` prints 0. 3. the diff line.
- Checked by CI: the pyside workflow (`uv sync --locked`, `ruff check`, `mypy --strict`, `pytest -n auto`).

### 9 broker-dividends (interface and implementation)
- Module: broker. Size: M. Depends on: reference-proto.
- Files: internal/broker/api.go, internal/broker/api_test.go, internal/broker/grpcbroker/dividends.go, internal/broker/grpcbroker/dividends_test.go, docs/contracts/broker.md
- Design: docs/design/event-sources.md#Interface. `Broker` is left unchanged on purpose: other types implement it outside these Files.
- Order: after 7, `account-broker-api`.
- Goal: the `ReferenceClient` interface with `Dividends`, `NewReferenceFunc` and `DividendProjection` (pointer numbers for unset); the client over an in-process fake server; a status the server did not set is an error.
- Acceptance: 1. `go test ./internal/broker/... -race -count=1` passes with `TestDividendsContract`, `TestDividendsUnsetStatusIsError` once each. 2. `go list -deps ./internal/broker | grep -c "grpcbroker\|genpb"` prints 0. 3. the diff line.
- Checked by CI: ci-go; `go run ./scripts/arch --check`.

### 10 dividends-probe
- Module: ot. Size: S. Depends on: watch-loader-earnings-wire, pyside-dividends, broker-dividends.
- Files: cmd/ot/probe.go, cmd/ot/probe_test.go, cmd/ot/wire.go, cmd/ot/wire_test.go, docs/contracts/ot.md
- Design: docs/design/event-sources.md#Decisions (6). `cmd/ot` builds its parts through the `deps` struct in `wire.go`, which tests fill with fakes; this issue adds the `deps` field for `NewReferenceFunc`, so the probe test needs no package global.
- Order: after 5, 8, 9, `account-wire` (same `wire.go`). Issue 16 is after 10 for `docs/contracts/ot.md` and `wire.go`. The owner runs it and commits `docs/design/event-sources-probe.txt`; the dividend issues wait for that file.
- Goal: `ot probe dividends [--symbols ...]` prints one line per symbol and `probe: US pass|fail`, `probe: JP pass|fail`; exit code 0 only when every market in the run passes; no key, DSN or account text; a `deps` field for `NewReferenceFunc`, wired to grpcbroker in `wire.go` and filled with a fake in the test (issue 14 reuses that field).
- Acceptance: 1. `go test ./cmd/ot -race -count=1 -run TestProbeDividends` passes with `TestProbeDividendsPass`, `TestProbeDividendsTimeoutOnNonPayerPasses`, `TestProbeDividendsErrorOnNonPayerFails`, `TestProbeDividendsMissingDateFails`, `TestProbeDividendsPerMarket`, `TestProbeDividendsPrintsNoSecret` once each. 2. `grep -c "probe dividends" docs/contracts/ot.md` is at least 1; `grep -c "NewReferenceFunc" cmd/ot/wire.go` is at least 1. 3. the diff line.
- Checked by CI: ci-go. The real run is the owner's.

### 11 dividend-datasets (interface)
- Module: contracts. Size: M. Depends on: dividends-probe.
- Files: contracts/datasets/obs.dividend_expectation.r1.json, contracts/generate.sh, internal/contracts/gen/records.go, docs/contracts/datasets/obs.dividend_expectation.md, contracts/fixtures/obs.dividend_expectation.r1.json, contracts/fixtures/MANIFEST.synthetic-v1.json, internal/contracts/schema_test.go, internal/contracts/api.go, internal/contracts/api_test.go
- Design: docs/design/event-sources.md#Decisions (6, 7). Contracts: obs.dividend_expectation r1 (new).
- Order: after 10, 7 and the probe file.
- Goal: schema, generated record exposed as `contracts.DividendExpectation` in `internal/contracts/api.go`, one fixture per status.
- Acceptance: 1. `grep -c "^probe: US pass" docs/design/event-sources-probe.txt` prints 1 (else stop and report). 2. `go test ./internal/contracts -race -count=1` passes with `TestDividendFixtureValues` once (`no_value`, `unsupported`, `error` rows have null `next_ex_date` and `next_amount`). 3. the diff line.
- Checked by CI: the generated-code check; `TestFixtureManifest`.

### 12 dividend-store (interface and implementation)
- Module: store. Size: M. Depends on: dividend-datasets, earnings-store.
- Files: internal/store/api.go, internal/store/api_dividends_test.go, migrations/NNNN_dividends.sql, internal/store/events/dividends.go, internal/store/events/dividends_test.go, internal/store/events/dividends_pg_test.go, docs/contracts/store.md
- Design: docs/design/event-sources.md#Decisions (7, 8). NNNN is the next free at cut time.
- Order: after 11, 2.
- Goal: `DividendWriter/Reader`, table and view `rd.dividend_expectation_r1`, INSERT and SELECT on the view for `ot_w_events`, SELECT for `ot_ro_pages`, highest attempt per day.
- Acceptance: 1. `grep -c "DividendWriter\|DividendReader" internal/store/api.go` is at least 2 and `wc -l < internal/store/api.go` is under 600. 2. `go test ./internal/store/... -race -count=1` passes; `grep -c "go:build pg" internal/store/events/dividends_pg_test.go` prints 1; `grep -c "GRANT.*\(UPDATE\|DELETE\|TRUNCATE\)" migrations/*_dividends.sql` prints 0 and `grep -c "GRANT SELECT ON rd.dividend_expectation_r1 TO ot_w_events" migrations/*_dividends.sql` prints 1. 3. the diff line.
- Checked by CI: ci-go; `go test -tags pg ./internal/store/...`; the size rule.

### 13 dividend-job (interface and implementation)
- Module: jobs. Size: M. Depends on: dividend-store, earnings-job, events-docs-modules.
- Files: internal/jobs/api.go, internal/jobs/api_test.go, internal/jobs/dividendcapture/job.toml, internal/jobs/dividendcapture/run.go, internal/jobs/dividendcapture/run_test.go, docs/contracts/jobs.md, docs/design/modules.md, arch.toml, .go-arch-lint.yml
- Design: docs/design/event-sources.md#Decisions (2, 5, 6, 7). This entry spans two modules, jobs and docs (`modules.md`), plus the regenerated `arch.toml` and `.go-arch-lint.yml` for the new package: a declared module pair.
- Order: after 12, 4, 6 (modules.md) and the probe file.
- Goal: `NewDividendCaptureFunc`; once per session day after the close, ask `ReferenceClient.Dividends` for the entries of the markets listed in the `passed_markets` key of `internal/jobs/dividendcapture/job.toml` (the owner copies the passed markets from the probe file; this is the run-time input) (`unsupported` rows for the others), write `present`, `no_value`, and a `timeout` as `error` with `error_code`, per stock; one stock failing does not stop the others.
- Acceptance: 1. `grep -c "^probe: US pass" docs/design/event-sources-probe.txt` prints 1 (else stop and report). 2. `go test ./internal/jobs/... -race -count=1` passes with `TestPresentRow`, `TestNoValueRow`, `TestTimeoutIsErrorRow`, `TestUnpassedMarketIsUnsupported`, `TestOneStockFailsOthersWritten` once each. 3. `grep -c "dividendcapture" docs/design/modules.md` is at least 1; the diff line.
- Checked by CI: ci-go; `go run ./scripts/arch --check`.

### 14 dividend-wire
- Module: ot. Size: S. Depends on: dividend-job, watch-loader-earnings-wire, jquants-earnings-wire, dividends-probe.
- Files: cmd/ot/runonce.go, cmd/ot/runonce_test.go, cmd/ot/wire.go, cmd/ot/wire_test.go, docs/contracts/ot.md
- Design: docs/design/event-sources.md#Decisions (6, 8)
- Order: after 13, 5, 10, 16 (same files), `account-wire`. It reuses the `NewReferenceFunc` field of `deps` that issue 10 adds.
- Goal: register `dividend_capture` beside `option_eod` with the same broker and the watch list; a down broker disables it by the existing preflight rule.
- Acceptance: 1. `go test ./cmd/ot -race -count=1` passes with `TestDividendCaptureRegistered`, `TestDividendCaptureOffWhenBrokerDown` once each. 2. `grep -c "dividend_capture" docs/contracts/ot.md` is at least 1. 3. the diff line.
- Checked by CI: ci-go; `go run ./scripts/arch --check`.

### 15 jquants-earnings-source (interface and implementation)
- Module: sources. Size: M. Depends on: earnings-source. Second-opinion case: new outside service; owner's word 2026-11-09 given.
- Files: internal/sources/jquants/client.go, internal/sources/jquants/parse.go, internal/sources/jquants/client_test.go, internal/sources/jquants/testdata/earnings_date.json, internal/sources/jquants/testdata/earnings_date_page2.json, internal/sources/api.go, internal/sources/api_test.go, docs/contracts/sources.md, arch.toml, .go-arch-lint.yml
- Design: docs/design/event-sources.md#Decisions (5, 12). Own code over `net/http` and `encoding/json`, no new dependency. `arch.toml` and `.go-arch-lint.yml` are regenerated for the new package (a module plus repo-file pair). The plan is settled (free, owner 2026-11-09) and the request shape is the owner's committed file, so no network step or choice is left to the worker.
- Call budget (Decision 12): one bulk request per `Fetch` (pages by cursor capped at 5, then `error` `page_cap`), never a request per symbol and no issuer input; no retry on any status; a 429 is `error` with `error_code` `rate_limited`.
- Order: after 3 (same `docs/contracts/sources.md` and `internal/sources/api.go`) and after the owner commits `docs/design/event-sources-jquants.txt`.
- Goal: `NewJQuantsFunc` returns an `EarningsSource` over recorded bodies: the key only in the `x-api-key` header, pages followed by the pagination key, the earliest `SchDate` on or after the day per company (Decision 7), 5-digit code normalised to 4, an empty `SchDate` is `absent`; a 401 or 403, a 429 and a non-JSON body are typed errors, and no error is retried.
- Acceptance: 1. `go test ./internal/sources/... -race -count=1` passes with `TestJQuantsParse`, `TestJQuantsEarliestFutureDate`, `TestJQuantsPagination`, `TestJQuantsEmptyDateIsAbsent`, `TestJQuantsKeyNeverInError`, `TestJQuantsAuthErrorIsTyped`, `TestJQuantsOneCallPerFetch`, `TestJQuantsRateLimitedNoRetry`, `TestJQuantsPageCap` once each, with a fake client that counts calls (one call for a one-page body, at most 5 for pages, exactly 1 after a 429 or 5xx, `page_cap` on a sixth page), on the recorded files in `testdata` and no network. 2. `grep -c "^request: bulk" docs/design/event-sources-jquants.txt` prints 1 (else stop and report, do not edit the file). 3. `grep -c "time.Sleep\|http.Get\|os.Getenv" internal/sources/jquants/*.go` prints 0; `grep -c "jquants" docs/contracts/sources.md` is at least 1. 4. the diff line.
- Checked by CI: ci-go; `go run ./scripts/arch --check`.

### 16 jquants-earnings-wire
- Module: ot. Size: S. Depends on: jquants-earnings-source, watch-loader-earnings-wire, events-docs-modules, dividends-probe.
- Files: cmd/ot/runonce.go, cmd/ot/runonce_test.go, cmd/ot/wire.go, cmd/ot/wire_test.go, cmd/ot/env.go, docs/contracts/ot.md
- Design: docs/design/event-sources.md#Decisions (5, 12)
- Order: after 15 and the jquants file, 5, 6 (same files, and the `ot` import list of modules.md), 10 (`wire.go`, `docs/contracts/ot.md`), `account-wire`. Issue 14 is after 16 for the same files.
- Goal: pass `NewJQuantsFunc` as the `JP` source of `earnings_refresh`, reading `JQUANTS_API_KEY`; a missing key disables only the JP source with a named line (JP entries get `unsupported`, source `none`) and the value is never printed; a J-Quants failure writes `error` rows for JP entries only; the J-Quants source is called at most once per day (no second call from a manual `run-once` or a scheduler tick the same day, no retry after an `error`).
- Acceptance: 1. `go test ./cmd/ot -race -count=1` passes with `TestJPSourceWired`, `TestJQuantsFailureOnlyJPRows`, `TestMissingJQuantsKeyNamedLine`, `TestJQuantsOneCallPerDay`, `TestJQuantsErrorDayNotRetried` once each (fake client counting calls: 1 for two ticks on one day, 0 further after an `error`). 2. `grep -c "JQUANTS_API_KEY" docs/contracts/ot.md` is at least 1. 3. `grep -c "^request: bulk" docs/design/event-sources-jquants.txt` prints 1 (else stop and report). 4. the diff line.
- Checked by CI: ci-go; `go run ./scripts/arch --check`.

## Checked by CI

- `go test ./internal/... ./cmd/ot -race -count=1`; `go test -tags pg ./internal/store/...` (GitHub CI only); `go run ./scripts/arch --check`; the generated-code check; `TestFixtureManifest` and `scripts/hygiene`; pyside `uv sync --locked`, `ruff check`, `mypy --strict`, `pytest -n auto`.

## What the owner gets

- You keep one short file, `watch.toml`, of the companies you watch, US and Japanese, with an optional `buy_price` and `sale_price` per company that the option-candidates work reads. The valuation work reads the same file. This design is the file's one owner.
- Once a day, for each US company, the system stores the next results date the vendor lists, or "not listed", with the vendor's time text; a Japanese company's date comes from J-Quants (JPX's own API), and a company without a date is "not listed", which holds the candidate back. A date is a vendor's word, not the company's: it can make a candidate wait or carry a mark, never approve it.
- After one command, `ot probe dividends`, run on your gateway machine, you know whether IBKR gives ex-dividend dates for US and for Japanese stocks. Only a market that passes gets a dividend schedule. Then per stock and day the next ex-date and amount per share are stored; the guard for short calls reads them as projected, never as declared.
- Dates you type in `watch.toml` as `report_date_override` win over any vendor. Before the jobs run you do three things: write `docs/design/event-sources-jquants.txt` from your J-Quants dashboard docs (`request: bulk`, `delay_days`), add `OT_PG_DSN_EVENTS`, `ALPHA_VANTAGE_API_KEY` and `JQUANTS_API_KEY` (the key from your J-Quants dashboard) to `.env`, and run `ot migrate` and `ot setup pg`. No Telegram text, no order touched.

## Open questions for the owner

1. Is the old Alpha Vantage key still valid? The free tier (about 25 calls a day) is enough: the job makes one call per day (Decision 12). Settled 2026-11-09: J-Quants is the free plan, so one call per run, no retry.
2. If a market's dividend probe fails: a hand-kept file, or no guard for it?
3. J-Quants free plan, `/v2/fins/earnings-date`: how many days behind is it (`delay_days` in the file of Decision 12)? Until you answer with a number under 2, every JP date is `unconfirmed` and can only hold a candidate back; the gates design inherits that as its decided input.

## Left out

- The gates and the guard rule: the candidate-gates design and a guard design read the Readers of Decision 7. The SEC estimated window and past results moves: stage 2.
- History load of past earnings or dividends: the tables start on the first run.
- Rejected: Yahoo or scraped pages (no stable contract); a Go IBKR client (plan v2.5 removed the row); IBKR `CalendarReport` (needs a Reuters subscription, not probed); the review's two simpler routes: an owner-typed TOML of dates only (covers Japan but is manual upkeep; kept only as `report_date_override`, Decision 5) and a dividend kind inside the quote path (reshapes `obs.quote_snapshot` and its frozen contract, more risk than a service the plan already names).
- Lock-ins: the keys with `attempt`, the status words, the `source` texts, `issuer` text forms, the proto field numbers once the owner merges `reference.proto`, and `watch.toml` as the one list.

## Adversarial

Review: `events-adversarial.md` (dispatcher scratchpad, not in the repository), checked at origin/main 02c784e. Each point:

- 1 goal gap, no watch list: adopted (Decision 1, issue 5; one shared `watch.toml`, valuation reuses it). JP: adopted (Decision 5, `unsupported`). Weak vendor date: adopted (Decision 4: refuse or mark only, `unconfirmed` without the window). Symbol identity: adopted (rows keyed by `issuer`).
- 2 watch list, DSN role, setup step, non-payer timeout: adopted (Decision 8, issues 2 and 5; probe outcome `timeout`, Decision 6, `test_silent_ticker_is_timeout`).
- 3 simpler ways: the hand-typed TOML only: not adopted as the main route (manual upkeep; kept only as the owner override of Decision 5). A dividend kind in the quote path: not adopted (Left out, reason there). Earnings first: adopted (Decision 11, issues 1 to 6 need no probe).
- 4 lock-ins: `unsupported` and `error` in the enums, issuer key, `attempt`: adopted (Decision 7); the plan's rpc shape `stream ContractKey`: adopted (Interface); Alpha Vantage isolation: noted, no change.
- 5 false claims: `roles.go` (now `setup.go`), hard-coded 0017 (now "next free at cut time"), the option_eod list, pyside `transport.py` and `ibgateway.py` in Files, `ReferenceService` naming, the nonexistent fake (the test file holds its own), plan 11.1 quote (日股), `ratchet.json` and `records.go` size (revision 3: `records.go` is generated and skipped by the ratchet, claim removed): all adopted (Decisions 8, 10; issues 2, 8; Source line).
- 6 slip: order against account-data for each shared file: adopted (Decision 10, Modules table). The probe gate: adopted as a checkable file `event-sources-probe.txt` read by Acceptance 1 of issues 11 and 13. The `t.Parallel()` count grep: not adopted as a grep, kept as the test rule in the Issues preface. The size remark on the datasets issue: adopted, split into issues 1 and 11 and the dividend store (12).
