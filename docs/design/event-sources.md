# Event sources: earnings dates and ex-dividend dates with amounts, stored and readable per underlying

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-09
Revision: 2 (adversarial review `events-adversarial.md` answered; revision 1 was 7e88e93)

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

1. One owner-edited list, `$OT_HOME/watch.toml`: one `[[underlying]]` per company with `issuer` (`cik:<10 digits>` for a US company, `edinet:<code>` for a Japanese one), `market` (`US` or `JP`), and the IBKR contract fields (`symbol`, `sec_type`, `exchange`, `currency`, `con_id`) and an optional `report_date_override` (`YYYY-MM-DD`, the owner's own word for one company, see Decision 5). Typed `jobs.WatchEntry{Issuer, Market, ReportDateOverride string; Contract broker.Contract}` in `internal/jobs/api.go`, read by a loader in `cmd/ot`. The valuation design reuses the same file and type for its subjects (a revision of that draft, not made here). Every row below is keyed by `issuer`, with the symbol kept as the vendor or IBKR sent it, so share classes (BRK.B, BRK-B) do not split a company.
2. Dividends: IBKR tick 456 through `pyside`, as the old system and plan v2.5 row W5-6 have it; no new account. The tick is four comma-separated values (past 12 months, next 12 months, next ex-date `YYYYMMDD`, next amount). IBKR does not say declared or projected, so the guard treats every ex-date as projected.
3. Earnings: Alpha Vantage `EARNINGS_CALENDAR` (`horizon=3month`), ported from the old source (header found by name; a notice body for a wrong key or used-up quota is an error, never `absent`; fewer than 500 rows is refused). Confirmed by the owner 2026-11-09; it is the only source the old system had; IBKR gives no earnings date through `pyside`. A new outside service for optdesk: `second-opinion case: new outside service`. The key `ALPHA_VANTAGE_API_KEY` is read by the `.env` loader and passed in; it never enters an error, log or row.
4. What a gate may do with these rows (the gates are the next design; this is the contract they get). A vendor earnings date is never a company confirmation. Until a design ports the SEC estimated window, a `listed` date can only refuse or mark a candidate, never clear it; the date is `unconfirmed` unless its `time_of_day` is `pre-market` or `post-market` and the row is at most 2 days old (the old rule's two tests, without the window). `absent`, `unsupported` and `error` all mean unknown, never "no results before expiry", and an unknown or missing date gates as blocked (the candidate waits; it is not cleared); with `horizon=3month` an expiry beyond three months cannot be judged either.
5. Japan, earnings: Alpha Vantage does not list Japanese companies. The first JP route is the JPX 決算発表予定日 Excel list (`jpx.co.jp/listing/event-schedules/financial-announcement/`, one xlsx per fiscal period, refreshed each business day about 17:00 for the next business day's items and less often otherwise): JPX's own list, no account, no key, a plain download once a day, rows `source` `jpx_xlsx`. Chosen first because it needs no outside account; J-Quants `/fins/earnings-date` (also JPX's, free plan) is the better feed but needs a free J-Quants account and API key, a new outside account, so it waits for the owner (issue 17) and replaces the Excel route only when the owner opens the account. The Excel's terms for automated download were not found (the site refused the research fetch): issue 15 begins by reading the JPX site policy and stops, reporting, if it forbids automated download; the answer is then issue 17 and the owner's account. A JP company missing from the list gets `absent` (the date is "announced to TSE", companies can be missing for weeks), and the gate sees unknown, so blocked, per Decision 4. The hand-typed date is no longer a route: `report_date_override` is only an owner override for one company, stored as a `listed` row with `source` `owner` and winning over the vendor's row for that day. For dividends the probe covers JP symbols (plan 11.1 item 4 asks for them); a market whose probe fails gets `unsupported` dividend rows.
6. Probe first, checkable: `ot probe dividends` asks IBKR for a fixed list: three US payers, one US non-payer, one US ETF, two JP payers. Per symbol: `ok` (parsable next ex-date), `no_value`, `timeout` (nothing in 10 s) or `error`. Per market the last lines are `probe: US pass|fail` and `probe: JP pass|fail`; a market passes when every payer is `ok`; for the non-payer and the ETF `no_value` and `timeout` are both fine, `error` is not. The owner runs it on the gateway machine (plan 11.1) and commits the output as `docs/design/event-sources-probe.txt`. `dividend-datasets` and later dividend issues Depend on `dividends-probe` and check that file in Acceptance 1 (`grep -c "^probe: US pass" ...` prints 1); if it prints 0 the worker stops and reports, it does not edit the file. Plan 11.1 stop rule: a failed market is not scheduled.
7. Shapes (append-only; an error is a row, a later success is a new attempt):
   - `obs.earnings_date` r1: key `(observed_on, issuer, attempt)`; `symbol`, `market`, `status` (`listed`, `absent`, `unsupported`, `error`), `report_date` (earliest listed date on or after `observed_on`), `fiscal_period_end`, `time_of_day`, `estimate` (currency not stored), `error_code`, `source` (`alphavantage`, `jpx_xlsx`, `owner`; later `jquants`), `received_at`.
   - `obs.dividend_expectation` r1: key `(session_day, instrument_id, attempt)`; `issuer`, `status` (`present`, `no_value`, `unsupported`, `error`), `past_12m`, `next_12m`, `next_amount` (per share), `next_ex_date`, `currency` (of the contract asked, not a claim of the tick), `error_code`, `source` `ibkr_tick_456`, `received_at`.
   - Readers: `EarningsDates(ctx, issuer, from, to)` and `Dividends(ctx, instrumentID, from, to)`, inclusive, ordered by day, only the highest `attempt` of a day, empty slice for none.
8. Role `ot_w_events` (INSERT only on both tables) is added in `internal/store/setup/setup.go` with `store.DSNEnvEvents = "OT_PG_DSN_EVENTS"`, read in `cmd/ot/runonce.go` and listed in the preflight. `ot_ro_pages` reads. `ot setup pg` and `ot migrate` for the new role are the owner's, run before the jobs (the job is off with a named line while the DSN line is missing).
9. The short-call guard needs the next ex-date and amount per share and `received_at` freshness; its rule (extrinsic value against the dividend) is the next design.
10. Order against `origin/design/account-data`, which edits the same files. Rule: an events issue that edits a file named here is `after` the account-data issue that edits it, and both keep a Files-bounded diff. Shared files and the account-data issues: `generate.sh`, `records.go`, docs/contracts datasets (`account-proto`); `internal/broker/api.go` (`account-broker-api`); `internal/store/api.go` (`account-store-api`); `setup.go` (`account-store-migration`, which also takes migration 0016); `cmd/ot/runonce.go`, `wire.go` (`account-wire`); `transport.py` (`account-guard-id`); `server.py` (`account-pyside-executions`). Migration numbers are written "next free at cut time" everywhere. `store/api.go` is 490 lines plus account-data's: the store issues check it stays under 600 and, if not, report instead of splitting.
11. Cut order: earnings first (issues 1 to 6 and 15, 16, no probe), the probe chain (7 to 10) in parallel with it, dividends (11 to 14) after the probe. Seventeen issues: the watch list, the dividend side and the two Japanese sources have their own issues; the gate is what stops wasted dividend work.

## Modules

| issue | module | order |
|---|---|---|
| 1 earnings-datasets (interface) | contracts | after account-proto |
| 2 earnings-store (interface and implementation) | store | after 1, account-store-api, account-store-migration |
| 3 earnings-source (interface and implementation) | sources | after 1 |
| 4 earnings-job (interface and implementation) | jobs | after 2, 3 |
| 5 watch-loader-earnings-wire | ot | after 4, account-wire |
| 6 events-docs-modules | docs | after 4 |
| 7 reference-proto (interface) | contracts | after 1 |
| 8 pyside-dividends (interface and implementation) | pyside | after 7, account-guard-id, account-pyside-executions |
| 9 broker-dividends (interface and implementation) | broker | after 7, account-broker-api |
| 10 dividends-probe | ot | after 5, 8, 9 |
| 11 dividend-datasets (interface) | contracts | after 10 and the probe file |
| 12 dividend-store (interface and implementation) | store | after 11, 2 |
| 13 dividend-job (interface and implementation) | jobs | after 12, 4, 6 |
| 14 dividend-wire | ot | after 13, 5, 16 |
| 15 jpx-earnings-source (interface and implementation) | sources | after 3 |
| 16 jpx-earnings-wire | ot | after 15, 5 |
| 17 jquants-earnings-source (waits for owner) | sources | after 15 |

## Interface

- contracts: datasets of Decision 7 (generated Go, docs); `reference.proto`: `service ReferenceService { rpc Dividends(stream ContractKey) returns (stream DividendProjection); }` as plan v2.5 line 170 (the plan's `TradingHours` rpc is not part of this design); `ContractKey` (symbol, sec_type, exchange, currency, con_id) is new in this file; `DividendProjection` has the key, `status`, the four values as optional text decimals, `error_code`, `received_at`.
- store: `EarningsDateWriter/Reader`, `DividendWriter/Reader`, `store.DSNEnvEvents`. sources: `EarningsConfig{BaseURL, APIKey, Clock}`, `EarningsRow`, `EarningsSource`, `NewAlphaVantageFunc`; `NewJPXFunc` (issue 15, same `EarningsSource`). broker: `Broker.Dividends(ctx, []Contract) ([]DividendProjection, error)`. jobs: `WatchEntry`, `NewEarningsRefreshFunc`, `NewDividendCaptureFunc`, each taking `[]WatchEntry`.
- pyside: `ReferenceService` servicer registered in `transport.py`; the Protocol in `ibkr/api.py`.

## Issues

Each entry: Module, Goal, Files, Design, Order, Acceptance, Checked by CI. "the diff line" is: `git diff --name-only $(git merge-base HEAD origin/main)` lists exactly the Files paths. Tests: `t.Parallel()`, `t.TempDir()`, fake clock, no sleep, no network, no live broker.

### 1 earnings-datasets (interface)
- Module: contracts. Size: M. Depends on: none.
- Files: contracts/datasets/obs.earnings_date.r1.json, contracts/generate.sh, internal/contracts/gen/records.go, docs/contracts/datasets/obs.earnings_date.md, contracts/fixtures/obs.earnings_date.r1.json, contracts/fixtures/MANIFEST.synthetic-v1.json, internal/contracts/schema_test.go
- Design: docs/design/event-sources.md#Decisions (7). Contracts: obs.earnings_date r1 (new). `records.go` is generated and skipped by the ratchet; nothing to report.
- Order: after `account-proto` (same generated files).
- Goal: schema with the status enum and the key of Decision 7, generated record, fixtures `listed`, `absent`, `unsupported`, `error` (synthetic), manifest hashes.
- Acceptance: 1. `grep -c "unsupported" contracts/datasets/obs.earnings_date.r1.json` is at least 1 and `grep -c "obs.earnings_date" internal/contracts/gen/records.go` is at least 1. 2. `go test ./internal/contracts -race -count=1` passes with `TestEarningsDateFixtureValues` once (a row per status; `absent`, `unsupported` and `error` have null `report_date`). 3. the diff line.
- Checked by CI: the generated-code check; `TestFixtureManifest`; `scripts/hygiene`.

### 2 earnings-store (interface and implementation)
- Module: store. Size: L. Depends on: earnings-datasets.
- Files: internal/store/api.go, internal/store/api_events_test.go, migrations/NNNN_events.sql, internal/store/events/earnings.go, internal/store/events/earnings_test.go, internal/store/events/earnings_pg_test.go, internal/store/setup/setup.go, internal/store/setup/setup_test.go, docs/contracts/store.md
- Design: docs/design/event-sources.md#Decisions (7, 8, 10). The contract test is a new file (`api_test.go` is 800 lines). NNNN is the next free at cut time.
- Order: after 1, `account-store-api`, `account-store-migration`.
- Goal: `EarningsDateWriter/Reader`, `DSNEnvEvents`, table and view `rd.earnings_date_r1`, role `ot_w_events` INSERT only, reader returns the highest attempt per day.
- Acceptance: 1. `grep -c "EarningsDateWriter\|EarningsDateReader\|DSNEnvEvents" internal/store/api.go` is at least 3 and `wc -l < internal/store/api.go` is under 600. 2. `go test ./internal/store/... -race -count=1` passes with `TestReaderTakesHighestAttempt` once; `grep -c "go:build pg" internal/store/events/earnings_pg_test.go` prints 1; `grep -c "ot_w_events" internal/store/setup/setup.go` is at least 1. 3. `grep -c "GRANT.*\(UPDATE\|DELETE\|TRUNCATE\)" migrations/*_events.sql` prints 0; the diff line.
- Checked by CI: ci-go; `go test -tags pg ./internal/store/...` (GitHub CI only); `go run ./scripts/arch --check`; the size rule.

### 3 earnings-source (interface and implementation)
- Module: sources. Size: M. Depends on: earnings-datasets. Second-opinion case: new outside service.
- Files: internal/sources/api.go, internal/sources/api_test.go, internal/sources/alphavantage/client.go, internal/sources/alphavantage/parse.go, internal/sources/alphavantage/client_test.go, internal/sources/alphavantage/testdata/calendar.csv, internal/sources/alphavantage/testdata/notice.json, docs/contracts/sources.md
- Design: docs/design/event-sources.md#Decisions (3)
- Order: after 1.
- Goal: `EarningsSource.Fetch(ctx)` over recorded bodies in tests; the key only in the query; notice, non-csv and short bodies are typed errors.
- Acceptance: 1. `go test ./internal/sources/... -race -count=1` passes with `TestParseByHeaderName`, `TestNoticeBodyIsError`, `TestShortCalendarIsError`, `TestKeyNeverInError` once each. 2. `grep -c "time.Sleep\|http.Get\|os.Getenv" internal/sources/alphavantage/*.go` prints 0; `grep -c "EarningsSource" internal/sources/api.go` is at least 1. 3. the diff line.
- Checked by CI: ci-go; `go run ./scripts/arch --check`. No new module dependency (`encoding/csv`).

### 4 earnings-job (interface and implementation)
- Module: jobs. Size: M. Depends on: earnings-store, earnings-source.
- Files: internal/jobs/api.go, internal/jobs/api_test.go, internal/jobs/earningsrefresh/job.toml, internal/jobs/earningsrefresh/run.go, internal/jobs/earningsrefresh/run_test.go, docs/contracts/jobs.md
- Design: docs/design/event-sources.md#Decisions (1, 4, 5, 7). The job takes one `EarningsSource` per market; a market without a source gets `unsupported`.
- Order: after 2, 3.
- Goal: `WatchEntry`, `NewEarningsRefreshFunc`; once per `XNYS` session day after the close, one fetch, one row per entry: `listed` (earliest date on or after the day) or `absent` from the source of its market, `unsupported` for a market with no source, and a `report_date_override` as a `listed` row with `source` `owner` that wins; a source error writes an `error` row per entry (the next run gets the next attempt) and fails the run.
- Acceptance: 1. `go test ./internal/jobs/... -race -count=1` passes with `TestListedTakesEarliestOnOrAfterDay`, `TestMarketWithoutSourceIsUnsupported`, `TestOwnerOverrideWins`, `TestSourceErrorWritesErrorRows`, `TestRerunNextAttempt` once each. 2. `grep -c 'id = "earnings_refresh"' internal/jobs/earningsrefresh/job.toml` prints 1; `grep -c "time.Sleep\|time.Now()" internal/jobs/earningsrefresh/*.go` prints 0. 3. the diff line.
- Checked by CI: ci-go; `go run ./scripts/arch --check`; the size rule.

### 5 watch-loader-earnings-wire
- Module: ot. Size: M. Depends on: earnings-job.
- Files: cmd/ot/watch.go, cmd/ot/watch_test.go, cmd/ot/runonce.go, cmd/ot/runonce_test.go, cmd/ot/wire.go, cmd/ot/wire_test.go, cmd/ot/env.go, docs/contracts/ot.md
- Design: docs/design/event-sources.md#Decisions (1, 3, 8)
- Order: after 4, `account-wire`.
- Goal: load `$OT_HOME/watch.toml` (an unknown market, a duplicate issuer or a malformed issuer or a malformed `report_date_override` is a named refusal), register `earnings_refresh` in `run-once` and the scheduler, read `OT_PG_DSN_EVENTS` and `ALPHA_VANTAGE_API_KEY`; a missing key or DSN disables only this job with a named line; values never printed.
- Acceptance: 1. `go test ./cmd/ot -race -count=1` passes with `TestWatchLoad`, `TestWatchRefusals` (covers the override),  `TestEarningsRefreshRegistered`, `TestMissingKeyDisablesOnlyEarnings` once each. 2. `grep -c "watch.toml\|OT_PG_DSN_EVENTS\|ALPHA_VANTAGE_API_KEY" docs/contracts/ot.md` is at least 3; `grep -c "ALPHA_VANTAGE_API_KEY=[A-Za-z0-9]" cmd/ot/*_test.go | grep -vc ":0"` prints 0. 3. the diff line.
- Checked by CI: ci-go; `go run ./scripts/arch --check`.

### 6 events-docs-modules
- Module: docs. Size: S. Depends on: earnings-job.
- Files: docs/design/modules.md
- Design: docs/design/event-sources.md#Modules
- Order: after 4 (and after the other designs' edits of the same sentence).
- Goal: add `internal/sources/alphavantage`, `internal/sources/jpx`, `internal/jobs/earningsrefresh`, `internal/store/events` to the one-constructor and implementation lists (the dividend package follows in 13).
- Acceptance: 1. `grep -c "alphavantage\|earningsrefresh\|store/events" docs/design/modules.md` is at least 3. 2. the diff line.
- Checked by CI: the modules-table check of `crew issue check`.

### 7 reference-proto (interface)
- Module: contracts. Size: S. Depends on: earnings-datasets.
- Files: contracts/proto/ibkr/v1/reference.proto, contracts/generate.sh, internal/contracts/genpb/ibkr/v1/reference.pb.go, internal/contracts/genpb/ibkr/v1/reference_grpc.pb.go, pyside/src/optdesk/contracts/genpb/ibkr/v1/reference_pb2.py, pyside/src/optdesk/contracts/genpb/ibkr/v1/reference_pb2.pyi, pyside/src/optdesk/contracts/genpb/ibkr/v1/reference_pb2_grpc.py
- Design: docs/design/event-sources.md#Interface. The proto path is owner-only (the owner merges it).
- Order: after 1.
- Goal: the service and messages of the Interface section, generated output with its source.
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
- Design: docs/design/event-sources.md#Interface
- Order: after 7, `account-broker-api`.
- Goal: `Broker.Dividends` and `DividendProjection` (pointer numbers for unset); the client over an in-process fake server; a status the server did not set is an error.
- Acceptance: 1. `go test ./internal/broker/... -race -count=1` passes with `TestDividendsContract`, `TestDividendsUnsetStatusIsError` once each. 2. `go list -deps ./internal/broker | grep -c "grpcbroker\|genpb"` prints 0. 3. the diff line.
- Checked by CI: ci-go; `go run ./scripts/arch --check`.

### 10 dividends-probe
- Module: ot. Size: S. Depends on: watch-loader-earnings-wire, pyside-dividends, broker-dividends.
- Files: cmd/ot/probe.go, cmd/ot/probe_test.go, docs/contracts/ot.md
- Design: docs/design/event-sources.md#Decisions (6)
- Order: after 5, 8, 9. The owner runs it and commits `docs/design/event-sources-probe.txt`; the dividend issues wait for that file.
- Goal: `ot probe dividends [--symbols ...]` prints one line per symbol and `probe: US pass|fail`, `probe: JP pass|fail`; exit code 0 only when every market in the run passes; no key, DSN or account text; real broker by the wiring, fake in the test.
- Acceptance: 1. `go test ./cmd/ot -race -count=1 -run TestProbeDividends` passes with `TestProbeDividendsPass`, `TestProbeDividendsTimeoutOnNonPayerPasses`, `TestProbeDividendsErrorOnNonPayerFails`, `TestProbeDividendsMissingDateFails`, `TestProbeDividendsPerMarket`, `TestProbeDividendsPrintsNoSecret` once each. 2. `grep -c "probe dividends" docs/contracts/ot.md` is at least 1. 3. the diff line.
- Checked by CI: ci-go. The real run is the owner's.

### 11 dividend-datasets (interface)
- Module: contracts. Size: M. Depends on: dividends-probe.
- Files: contracts/datasets/obs.dividend_expectation.r1.json, contracts/generate.sh, internal/contracts/gen/records.go, docs/contracts/datasets/obs.dividend_expectation.md, contracts/fixtures/obs.dividend_expectation.r1.json, contracts/fixtures/MANIFEST.synthetic-v1.json, internal/contracts/schema_test.go
- Design: docs/design/event-sources.md#Decisions (6, 7). Contracts: obs.dividend_expectation r1 (new).
- Order: after 10, 7 and the probe file.
- Goal: schema, generated record, one fixture per status.
- Acceptance: 1. `grep -c "^probe: US pass" docs/design/event-sources-probe.txt` prints 1 (else stop and report). 2. `go test ./internal/contracts -race -count=1` passes with `TestDividendFixtureValues` once (`no_value`, `unsupported`, `error` rows have null `next_ex_date` and `next_amount`). 3. the diff line.
- Checked by CI: the generated-code check; `TestFixtureManifest`.

### 12 dividend-store (interface and implementation)
- Module: store. Size: M. Depends on: dividend-datasets, earnings-store.
- Files: internal/store/api.go, internal/store/api_dividends_test.go, migrations/NNNN_dividends.sql, internal/store/events/dividends.go, internal/store/events/dividends_test.go, internal/store/events/dividends_pg_test.go, docs/contracts/store.md
- Design: docs/design/event-sources.md#Decisions (7, 8). NNNN is the next free at cut time.
- Order: after 11, 2.
- Goal: `DividendWriter/Reader`, table and view `rd.dividend_expectation_r1`, INSERT for `ot_w_events`, SELECT for `ot_ro_pages`, highest attempt per day.
- Acceptance: 1. `grep -c "DividendWriter\|DividendReader" internal/store/api.go` is at least 2 and `wc -l < internal/store/api.go` is under 600. 2. `go test ./internal/store/... -race -count=1` passes; `grep -c "go:build pg" internal/store/events/dividends_pg_test.go` prints 1; `grep -c "GRANT.*\(UPDATE\|DELETE\|TRUNCATE\)" migrations/*_dividends.sql` prints 0. 3. the diff line.
- Checked by CI: ci-go; `go test -tags pg ./internal/store/...`; the size rule.

### 13 dividend-job (interface and implementation)
- Module: jobs. Size: M. Depends on: dividend-store, earnings-job.
- Files: internal/jobs/api.go, internal/jobs/api_test.go, internal/jobs/dividendcapture/job.toml, internal/jobs/dividendcapture/run.go, internal/jobs/dividendcapture/run_test.go, docs/contracts/jobs.md, docs/design/modules.md
- Design: docs/design/event-sources.md#Decisions (2, 5, 6, 7)
- Order: after 12, 4, 6 (modules.md) and the probe file.
- Goal: `NewDividendCaptureFunc`; once per session day after the close, ask `broker.Dividends` for the entries of the markets whose probe passed (`unsupported` rows for the others), write `present`, `no_value`, and a `timeout` as `error` with `error_code`, per stock; one stock failing does not stop the others.
- Acceptance: 1. `grep -c "^probe: US pass" docs/design/event-sources-probe.txt` prints 1 (else stop and report). 2. `go test ./internal/jobs/... -race -count=1` passes with `TestPresentRow`, `TestNoValueRow`, `TestTimeoutIsErrorRow`, `TestUnpassedMarketIsUnsupported`, `TestOneStockFailsOthersWritten` once each. 3. `grep -c "dividendcapture" docs/design/modules.md` is at least 1; the diff line.
- Checked by CI: ci-go; `go run ./scripts/arch --check`.

### 14 dividend-wire
- Module: ot. Size: S. Depends on: dividend-job, watch-loader-earnings-wire.
- Files: cmd/ot/runonce.go, cmd/ot/runonce_test.go, cmd/ot/wire.go, cmd/ot/wire_test.go, docs/contracts/ot.md
- Design: docs/design/event-sources.md#Decisions (6, 8)
- Order: after 13, 5, 16 (same files), `account-wire`.
- Goal: register `dividend_capture` beside `option_eod` with the same broker and the watch list; a down broker disables it by the existing preflight rule.
- Acceptance: 1. `go test ./cmd/ot -race -count=1` passes with `TestDividendCaptureRegistered`, `TestDividendCaptureOffWhenBrokerDown` once each. 2. `grep -c "dividend_capture" docs/contracts/ot.md` is at least 1. 3. the diff line.
- Checked by CI: ci-go; `go run ./scripts/arch --check`.

### 15 jpx-earnings-source (interface and implementation)
- Module: sources. Size: M. Depends on: earnings-source.
- Files: internal/sources/jpx/client.go, internal/sources/jpx/parse.go, internal/sources/jpx/client_test.go, docs/contracts/sources.md
- Design: docs/design/event-sources.md#Decisions (5). Own code is the page parse; the xlsx read uses `github.com/xuri/excelize/v2` (BSD-3-Clause, widely used, stable API, few dependencies; a new direct dependency, named in the pull request description with its license). The tests build a small xlsx in `t.TempDir()`, no binary fixture.
- Order: after 3 (same `docs/contracts/sources.md`).
- Goal: `NewJPXFunc` returns an `EarningsSource`: find the current period's xlsx link on the JPX page, download it once, parse code, company, date by header name, normalise the 5-digit code to 4; a changed layout, a short list or a non-xlsx body is a typed error. First read the JPX site policy; if it forbids automated download, stop and report in the pull request and change nothing.
- Acceptance: 1. `go test ./internal/sources/... -race -count=1` passes with `TestJPXParseByHeader`, `TestJPXCodeNormalised`, `TestJPXLayoutChangeIsError`, `TestJPXShortListIsError` once each. 2. `grep -c "time.Sleep\|http.Get\|os.Getenv" internal/sources/jpx/*.go` prints 0; `grep -c "jpx" docs/contracts/sources.md` is at least 1. 3. the diff line.
- Checked by CI: ci-go; `go run ./scripts/arch --check`.

### 16 jpx-earnings-wire
- Module: ot. Size: S. Depends on: jpx-earnings-source, watch-loader-earnings-wire.
- Files: cmd/ot/runonce.go, cmd/ot/runonce_test.go, cmd/ot/wire.go, cmd/ot/wire_test.go, docs/contracts/ot.md
- Design: docs/design/event-sources.md#Decisions (5)
- Order: after 15, 5 (same files), `account-wire`. Issue 14 is after 16 for the same files.
- Goal: pass `NewJPXFunc` as the `JP` source of `earnings_refresh`; no key, no new env name; a JPX failure writes `error` rows for JP entries only.
- Acceptance: 1. `go test ./cmd/ot -race -count=1` passes with `TestJPSourceWired`, `TestJPXFailureOnlyJPRows` once each. 2. `grep -c "jpx" docs/contracts/ot.md` is at least 1. 3. the diff line.
- Checked by CI: ci-go; `go run ./scripts/arch --check`.

### 17 jquants-earnings-source (waits for owner: J-Quants account; second-opinion case)
- Module: sources. Size: M. Depends on: jpx-earnings-source. Second-opinion case: new outside account (a free J-Quants account and API key). Not cut until the owner opens the account and says so.
- Files: internal/sources/jquants/client.go, internal/sources/jquants/parse.go, internal/sources/jquants/client_test.go, internal/sources/jquants/testdata/earnings_date.json, docs/contracts/sources.md
- Design: docs/design/event-sources.md#Decisions (5). The free plan's data window and delay for `/fins/earnings-date` are unchecked; the first step reads the plan page and stops if the window cannot cover the next quarter.
- Order: after 15 (same `docs/contracts/sources.md`). The wire (env name, `source` `jquants`) is a later issue cut with the owner's word.
- Goal: an `EarningsSource` over recorded bodies; the key only in a header, never in an error, log or row.
- Acceptance: 1. `go test ./internal/sources/... -race -count=1` passes with `TestJQuantsParse`, `TestJQuantsKeyNeverInError`, `TestJQuantsAuthErrorIsTyped` once each. 2. `grep -c "time.Sleep\|http.Get\|os.Getenv" internal/sources/jquants/*.go` prints 0. 3. the diff line.
- Checked by CI: ci-go; `go run ./scripts/arch --check`.

## Checked by CI

- `go test ./internal/... ./cmd/ot -race -count=1`; `go test -tags pg ./internal/store/...` (GitHub CI only); `go run ./scripts/arch --check`; the generated-code check; `TestFixtureManifest` and `scripts/hygiene`; pyside `uv sync --locked`, `ruff check`, `mypy --strict`, `pytest -n auto`.

## What the owner gets

- You keep one short file, `watch.toml`, of the companies you watch, US and Japanese. The valuation work reads the same file.
- Once a day, for each US company, the system stores the next results date the vendor lists, or "not listed", with the vendor's time text; a Japanese company's date comes from JPX's published list, and a company missing from it is "not listed", which holds the candidate back. A date is a vendor's word, not the company's: it can make a candidate wait or carry a mark, never approve it.
- After one command, `ot probe dividends`, run on your gateway machine, you know whether IBKR gives ex-dividend dates for US and for Japanese stocks. Only a market that passes gets a dividend schedule. Then per stock and day the next ex-date and amount per share are stored; the guard for short calls reads them as projected, never as declared.
- Dates you type in `watch.toml` as `report_date_override` win over any vendor. Before the jobs run you do two things: add `OT_PG_DSN_EVENTS` and `ALPHA_VANTAGE_API_KEY` to `.env`, and run `ot migrate` and `ot setup pg`. No Telegram text, no order touched.

## Open questions for the owner

1. Is the old Alpha Vantage key still valid, and is the free quota (about 25 calls a day by my knowledge) enough? (The source itself is settled, 2026-11-09.)
2. J-Quants (issue 17): do you open a free J-Quants account for the JP earnings feed, or stay on the JPX Excel? Waits for your word. If a market's dividend probe fails: a hand-kept file, or no guard for it?

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
