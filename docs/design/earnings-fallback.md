---
type: design
title: "earnings fallback: when Alpha Vantage or J-Quants has no next earnings date for a name, yfinance is asked, and each stored date names its source"
status: accepted
date: "2026-11-26"
need: [none]
none_reason: "owner word 2026-11-25: earnings dates must use more sources; no need record exists"
---

# earnings fallback: when Alpha Vantage or J-Quants has no next earnings date for a name, yfinance is asked, and each stored date names its source

Author: the optdesk designer
Owner approved: (pending)
Revision: 1
Plan: none — owner word 2026-11-25, a gap of the event sources (docs/design/event-sources.md), not a plan item

摘要：美股用 Alpha Vantage、日股用 J-Quants 查“下一个财报日”，但两者只列近期要报的公司，远期的公司查不到（状态 absent）。现在这类公司在同一次 earnings_refresh 里再向 yfinance 问一次（yfinance 已在 pyside 里固定为 1.8.0，只用于日线，不需要账号和密钥）。问到日期就按 listed 存，来源写 yfinance；问不到就保持 absent，不编日期。每个日期的来源本来就存在行里（source 一栏），这里只是多一个取值 yfinance。页面不改，仍然只显示库里的日期。分八件事：先改契约取值，再加迁移、pyside 接口、pyside 命令、sources 注释、sources 适配器、job、接线。

Source: owner word 2026-11-25 ("earnings dates must use more sources"). Survey 2026-11-25 by the dispatcher: Alpha Vantage (US) and J-Quants (JP) work but list only near-term reporters; yfinance 1.8.0 is pinned in `pyside/pyproject.toml` and needs no key; FMP free tier unclear (not used); the IBKR Wall Street Horizon feed is a paid subscription and is not used. Read from origin/main 076ebfac. The other earnings bug (readers following only the newest `earnings_check`) is closed: issues 4008, 4009, 4011 and 4038 landed (`5c930c1b`, `faff191a`, `0d42639f`, `67fabe44`); this design does not touch a reader.

Needs the owner: nothing. The fallback is yfinance, which is already a dependency (Apache-2.0, no account, no key), so no outside service or account is added and no second opinion is called for. Contracts: changed (the `source` enum of `obs.earnings_date` gains `yfinance`; migration 0072).

## Frame

Where the next earnings date comes from today:

- `internal/jobs/earningsrefresh/run.go` (`fetchAndWrite`, line 251): one row per watch entry and session day. An entry with `report_date_override` is `listed`/`owner` (line 271); a market with no source is `unsupported`/`none` (line 280); the others go to `fetchPending` (line 314), one `Fetch` per market.
- `listed` (line 367) takes the earliest listing on or after the day for the entry, else the row is `absent` (lines 394 to 397). Alpha Vantage lists only the next few months and J-Quants only the coming weeks, so a name that reports later is `absent`.
- `cmd/ot/runonce.go` `openEarnings` (line 527) builds the US source (`alphaKey`) and the maker of the JP source (`jquantsKey`); `cmd/ot/wire_earnings.go` `earningsRefresh` (line 41) hands them to the job.
- `source` is checked as `alphavantage|jquants|owner|none` in `migrations/0020_events.sql` line 28 and `migrations/0046_fundamentals_keys.sql` line 54, and in `contracts/datasets/obs.earnings_date.r1.json`.
- The bars job already has a yfinance fallback (`docs/design/bars-fallback.md`): `pyside/src/optdesk/refbars` is a one-shot command, `internal/sources/yfinance` is its Go adapter and runs it.

Result for the owner: a followed US or JP name that Alpha Vantage or J-Quants does not list now gets its next earnings date from yfinance when Yahoo has one, stored on the same row with `source = yfinance`; every page that shows the stored date shows it as before. A name Yahoo has no date for stays `absent`, with its primary source on the row: a gap, never an invented date. A date from the primary source always wins; an owner override always wins.

### Decisions

#### Decision 1: the fallback asks only for entries the primary left without a date

After the primary fetches, an entry with no `listed` row and no owner override (status `absent`, `error`, or `unsupported` because that market has no source) is asked of yfinance, once per market per run, with the symbols of those entries only. A `listed` primary row is never replaced. One run still writes one row per entry (key issuer and day), so the fallback row replaces the primary row of that entry in the same upsert; no second attempt row. The data row is keyed `(issuer, period_key)` (`internal/store/events/earnings_upsert.go` line 61: the fiscal period end for an alphavantage row that has one, else the report date), so a yfinance row keys on its report date; the answer row of the day (`earnings_check`, key issuer and day) names the `period_key` of the date the day found, and the readers follow that answer row (issues 4008 to 4038). A Yahoo date that moves, or an Alpha Vantage listing of the same quarter that arrives later, therefore adds a data row under another key and the newest answer row points to it; the older data row stays as history and no reader picks it. The store is not changed.

#### Decision 2: a fallback row is `listed` with source `yfinance`; a failed fallback leaves the primary row

A date on or after the day gives `status listed`, `source yfinance`, `report_date`, empty `time_of_day`, no `fiscal_period_end`, no `estimate` (Yahoo gives a date, sometimes a range of two, from which the earliest on or after the day is taken). No date, or an error of the command (`rate_limited`, `no_data`, `network`, `timeout`, `source_changed`), leaves the primary row as it is, `absent` or `error` or `unsupported`, so the gap keeps its reason (absent: the source has no date; error: its `error_code`; unsupported: no source). The fallback error codes go to one log line of the run, as in the bars fallback. A failed fallback never fails the job: the existing result rules for the primary stay.

#### Decision 3: yfinance runs through its own one-shot command

A new pyside package `optdesk.refearn` (command `python -m optdesk.refearn --deadline SECONDS`, one JSON `Request` on stdin, one JSON `Answer` on stdout, at most 10 symbols per request) reads `Ticker.calendar` for each symbol and returns its dates (in yfinance 1.8.0, `scrapers/quote.py` `_fetch_calendar`, it is a dict: `"Earnings Date"` is a list of zero, one or two `datetime.date` and the key is absent when Yahoo has no earnings block; the dict is empty when the request gave nothing); it reuses `optdesk.refbars.symbols.map_pair` for the Yahoo symbol (USD keeps the symbol with a dot changed to a dash; JPY appends `.T`). The Go side is `yfinance.NewEarnings` in `internal/sources/yfinance`, which implements the existing `sources.EarningsSource` and splits a request into batches of 10 and stops at the first `rate_limited`. Own implementation, because no mainstream library covers a Yahoo earnings date other than yfinance itself, which is used; the glue is behind `sources.EarningsSource`, so a library can replace it (owner 2026-11-08).

#### Decision 4: the wiring is a decorator option, as in the bars job

`earningsrefresh.WithFallback(job, maker)` returns the job with a maker `func(market string, symbols []string) (sources.EarningsSource, error)`; the job calls it once per market with the entries still without a date. `cmd/ot` passes the yfinance maker whenever the refbars command can run (no key, no new setting); without it the job behaves as today.

#### Decision 5: no page, web or compute change

The pages read the stored rows and show the stored date; the source column is already in the row and in `rd.earnings_date_r1`. The web renders and computes nothing. The only reader that names a source is `internal/pages/option_data_page/earnings.go` line 50 (`owner`), unchanged.

## Modules

contracts (`contracts/datasets/obs.earnings_date.r1.json`, its fixture, `internal/contracts/gen/records.go`, `internal/contracts/schema_test.go`, `docs/contracts/datasets/obs.earnings_date.md`), store (`migrations/0072_earnings_source_yfinance.sql`), pyside (`pyside/src/optdesk/refearn`, `pyside/tests/refearn`, `pyside/tests/contracts/test_refearn_api.py`), sources (`internal/sources/api.go`, `internal/sources/yfinance`), jobs (`internal/jobs/earningsrefresh`) and ot (`cmd/ot`). `docs/design/modules.md` gains in this pull request, in the docs module, the entry `pyside/src/optdesk/refearn/api.py` and the test `pyside/tests/contracts/test_refearn_api.py` in the pyside row; no dependency between modules changes (pyside depends on contracts, jobs already depends on sources).

## Interface

Four interface changes, each its own entry and landing before the issues that use it:

- contracts: the `source` enum of `obs.earnings_date` r1 gains `yfinance` (an additive value; schema, fixture and generated type together).
- store: the two `source` checks accept `yfinance` (migration 0072; no table shape change).
- pyside: `pyside/src/optdesk/refearn/api.py` declares `Request`, `SymbolResult`, `Answer` and the error codes of the command (new file, declarations only).
- sources: the comment of `EarningsConfig` says `Symbols` are the symbols the source is asked for (JP codes for J-Quants, the symbols of the entries without a date for yfinance). No type or signature changes. The command runner is not part of `EarningsConfig`: the yfinance constructor takes `yfinance.CommandFunc` (declared in `internal/sources/yfinance/command.go`) as its own argument, `NewEarnings(market string, run CommandFunc, cfg sources.EarningsConfig)`.

## Failure paths

- Yahoo has no calendar for a name, or the command answers `no_data`: the primary row stays; the page shows what it showed.
- Yahoo rate limit: the rest of the batches of that market are not asked; the names asked so far keep their dates; the log line names the code.
- The command cannot run (no `uv` environment, deadline): same as a failed fallback, the primary rows stand.
- With no Alpha Vantage key `earningsOff` leaves the whole job out (unchanged), so the fallback runs for neither market then.
- A stored row of an earlier day is not rewritten; a forced re-run of a stored day (`-force`) asks the fallback again.
- Carried decisions: Decisions 1 and 2 in issue 7, Decision 3 in issues 3, 4 and 6, Decision 4 in issues 7 and 8, Decision 5 has no entry.

## Issues

### 1 earnings-fallback-contracts

Module: contracts. Size: S. Order: first. Design: docs/design/earnings-fallback.md#interface. Files: `contracts/datasets/obs.earnings_date.r1.json`, `contracts/fixtures/obs.earnings_date.r1.json`, `internal/contracts/gen/records.go`, `internal/contracts/schema_test.go`, `docs/contracts/datasets/obs.earnings_date.md`.

Acceptance:
1. The `source` enum of `contracts/datasets/obs.earnings_date.r1.json` lists `alphavantage`, `jquants`, `yfinance`, `owner`, `none`, and its description says `yfinance` is a date read from Yahoo when the primary source had none; no other property changes.
2. `internal/contracts/gen/records.go` declares the constant `EarningsDateSourceYfinance` with value `yfinance` and lists it in the values of `EarningsDateSource`, as the generator writes it from the schema (source and output in one change).
3. `contracts/fixtures/obs.earnings_date.r1.json` holds one listed row with source `yfinance`, and `internal/contracts/schema_test.go` has the case `{"listed", "yfinance", true}` next to the `jquants` cases (new case).
4. `docs/contracts/datasets/obs.earnings_date.md` names `yfinance` in its list of sources.
5. Docs: docs/design/earnings-fallback.md#interface.

### 2 earnings-fallback-migration

Module: store. Size: S. Order: after 1. Design: docs/design/earnings-fallback.md#interface. Files: `migrations/0072_earnings_source_yfinance.sql`.

Acceptance:
1. `migrations/0072_earnings_source_yfinance.sql` (new) drops and adds `earnings_date_source_ck` on `obs.earnings_date` and `earnings_check_source_ck` on `obs.earnings_check`, each as `CHECK (source IN ('alphavantage', 'jquants', 'yfinance', 'owner', 'none'))`, and nothing else; the number 0072 follows `0071_instrument_conid_repair.sql`.
2. The file has no `DROP TABLE`, no `DELETE` and no `UPDATE`, and keeps `earnings_date_unsupported_ck` and `earnings_check_unsupported_ck` as they are.
3. Docs: docs/design/earnings-fallback.md#interface.

### 3 earnings-fallback-pyside-api

Module: pyside. Size: S. Order: after 1. Design: docs/design/earnings-fallback.md#decisions. Files: `pyside/src/optdesk/refearn/api.py`, `pyside/src/optdesk/refearn/__init__.py`, `pyside/tests/contracts/test_refearn_api.py`.

Acceptance:
1. `pyside/src/optdesk/refearn/api.py` (new) is declarations only (`TypedDict`s and a `Protocol`, no function body with logic): `SymbolSpec` (`symbol`, `currency`), `Request` (`symbols` list, at most 10, and `from_day`), `SymbolResult` (`symbol`, `currency`, `error_code`, `dates` as `YYYY-MM-DD` text), `Answer` (`results`, one per requested symbol in request order) and `ERROR_CODES` equal to those of `optdesk.refbars.api` (`rate_limited`, `no_data`, `network`, `timeout`, `source_changed`).
2. `pyside/tests/contracts/test_refearn_api.py` (new) is the contract test from both sides, as `test_refbars_api.py` is: a fake that implements the `Protocol` and a caller that builds a `Request` and reads an `Answer`.
3. Docs: docs/design/earnings-fallback.md#decisions.

### 4 earnings-fallback-pyside

Module: pyside. Size: M. Order: after 3. Design: docs/design/earnings-fallback.md#decisions. Files: `pyside/src/optdesk/refearn/__main__.py`, `pyside/src/optdesk/refearn/cli.py`, `pyside/src/optdesk/refearn/fetch.py`, `pyside/tests/refearn/test_cli.py`, `pyside/tests/refearn/test_fetch.py`.

Acceptance:
1. `python -m optdesk.refearn --deadline SECONDS` reads one `Request` from stdin and writes one `Answer` with one result per requested symbol in order, and exits 0 whenever it wrote the answer, as `optdesk.refbars.cli` does.
   Built: `fetch.py` exposes `make_fetcher(calendar_of)`, where `calendar_of(yahoo_symbol)` returns `Ticker.calendar`; `cli.py` builds that function over the `yfinance` module it is given (as `optdesk.refbars.cli` does) and keeps the one-second gap, the deadline (`timeout` for the symbols not reached) and the stop at the first `rate_limited`. Exceptions are classified by class name (`YFRateLimitError`, `*Timeout*`, `*ConnectionError*`, `OSError`), so `fetch.py` imports no yfinance.
2. `fetch.py` maps the symbol with `optdesk.refbars.symbols.map_pair` (a pair with no route gives `no_data`), reads the dates of the Yahoo calendar of that symbol through a fetcher passed in (the real one wraps `yfinance.Ticker(...).calendar`), returns them as `YYYY-MM-DD` text in ascending order with none before `from_day`, and maps a rate-limit exception to `rate_limited`, a timeout to `timeout`, a connection error to `network`, an empty calendar, a missing `"Earnings Date"` key or an empty list to `no_data` and any other exception or an unexpected shape (a value that is not a list of `datetime.date`) to `source_changed`.
3. The tests use fixtures only, a fake fetcher that stands in for yfinance, and a fake clock for the deadline: `test_fetch.py` covers a one-date calendar, a two-date range, a date before `from_day`, an empty calendar, a rate limit and an unexpected shape; `test_cli.py` covers the one-request, one-answer round trip and the exit code. No test imports `yfinance` or reaches the network.
4. The new files are each under 200 lines and `pyside/pyproject.toml` and `pyside/uv.lock` are unchanged (yfinance is already pinned).
5. Docs: docs/design/earnings-fallback.md#decisions.

### 5 earnings-fallback-sources-interface

Module: sources. Size: S. Order: first. Design: docs/design/earnings-fallback.md#interface. Files: `internal/sources/api.go`.

Acceptance:
1. The comment of `EarningsConfig` in `internal/sources/api.go` says `Symbols` are the symbols the source is asked for: the JP codes for J-Quants, the symbols of the entries without a date for the yfinance source; Alpha Vantage ignores them.
2. No type, constant or signature of the file changes (`git diff` of the file is comment lines only).
3. Docs: docs/design/earnings-fallback.md#interface.

### 6 earnings-fallback-sources

Module: sources. Size: M. Order: after 4 and 5. Design: docs/design/earnings-fallback.md#decisions. Files: `internal/sources/yfinance/earnings.go`, `internal/sources/yfinance/earnings_test.go`, `internal/sources/yfinance/testdata/earnings_answer.json`.

Acceptance:
1. `internal/sources/yfinance/earnings.go` (new) declares `NewEarnings(market string, run CommandFunc, cfg sources.EarningsConfig) (sources.EarningsSource, error)`, where `CommandFunc` is the type the yfinance package already declares (a test passes a fake, the wiring passes `Exec`); the returned source runs the `optdesk.refearn` command through `run`, one request of at most 10 symbols per batch, and stops asking at the first answer with `rate_limited`. The argv is fixed in the package (`uv run --locked --project pyside python -m optdesk.refearn --deadline N`, since `NewEarnings` takes no command), `from_day` is the UTC day of the call, and the currency is `USD` for market `US` and `JPY` for `JP` (another market is an error).
2. `Fetch` returns one `sources.EarningsListing` per date of a result with an empty `error_code` (symbol as asked, `ReportDate` the date, the other fields empty); a result with an error code gives no listing; if every batch fails it returns a `CodedError` with the first code (`rate_limited`, `network`, `timeout`, `source_changed`, or `no_data` when all results say so), and a partly answered request returns the listings it has with no error.
3. `earnings_test.go` (new) uses `testdata/earnings_answer.json` and a fake `CommandFunc`: one batch, 25 symbols split into batches of 10, a rate limit in the second batch that stops the third, a non-zero exit code, and a bad JSON answer. No test starts a process or reaches the network, and none sleeps.
4. The error returned and the source hold no address and no environment value, as the other sources.
5. Docs: docs/design/earnings-fallback.md#decisions.

### 7 earnings-fallback-job

Module: jobs. Size: M. Order: after 1, 2 and 6. Design: docs/design/earnings-fallback.md#decisions. Files: `internal/jobs/earningsrefresh/fallback.go`, `internal/jobs/earningsrefresh/run.go`, `internal/jobs/earningsrefresh/fallback_test.go`.

Acceptance:
1. `fallback.go` (new) declares `FallbackMaker` (`func(market string, symbols []string) (sources.EarningsSource, error)`) and `WithFallback(j contracts.Job, mk FallbackMaker) contracts.Job`; a job that is not from this package is returned unchanged; `vendor` in `run.go` gains `yfinance` only as the source text the fallback writes.
2. In `fetchAndWrite`, after `fetchPending`, an entry whose row is not `listed` and has no owner override is asked of the fallback with the other such entries of its market in one `Fetch`; a listing on or after the day (earliest wins; the symbol is matched against the symbol asked, not translated through the Alpha Vantage vendor mapping that `listed` applies) turns the row into `listed`, `source yfinance`, the `report_date`, with `time_of_day`, `fiscal_period_end` and `estimate` empty; a `listed` primary row and an owner row are never changed.
3. A fallback with no listing or with an error leaves the primary row unchanged (`absent`, `error` with its code, or `unsupported` with source `none`); its error code is logged once per market and never changes the result of the run or the `failed` map of the primary markets.
4. Without `WithFallback` the job behaves as before: the existing tests of `run_test.go` pass unchanged.
5. `fallback_test.go` (new) uses fake sources and a fake clock, one case per row of Decision 1 and 2 (the JP case uses an entry whose vendor symbol differs from its code): an absent US entry listed by the fallback (`TestFallbackFillsAbsent`), an error entry listed by the fallback, an unsupported JP entry listed, a primary `listed` row kept (`TestFallbackNeverReplacesListed`), an owner row kept, a fallback with no date leaving `absent`, a fallback error leaving the primary row with the run result unchanged, and one fallback `Fetch` per market. No test sleeps, uses a live service or a fixed path.
6. `run.go` stays under 600 lines.
7. Docs: docs/design/earnings-fallback.md#decisions.

### 8 earnings-fallback-wire

Module: ot. Size: S. Order: after 7. Design: docs/design/earnings-fallback.md#decisions. Files: `cmd/ot/wire_earnings.go`, `cmd/ot/wire_earnings_test.go`.

Acceptance:
1. In `cmd/ot/wire_earnings.go`, `earningsRefresh` wraps the job it builds with `earningsrefresh.WithFallback`, giving a maker that builds `yfinance.NewEarnings(market, run, sources.EarningsConfig{Symbols: symbols})` with `run` the `yfinance.Exec` of `internal/sources/yfinance/command.go` (the real `CommandFunc` the bars source also runs; `earningsRefresh` calls `earningsRefreshVia(run)`, the seam a test gives a fake runner); `earningsOff` is unchanged, so with no Alpha Vantage key the job is still left out as before.
2. `wire_earnings_test.go` has a case (new) that builds the job with a fake command runner and a fake primary source that lists nothing, and sees the row written with source `yfinance`; and a case with a failing runner that sees the primary row written.
3. No setting, environment name or key is added, and `cmd/ot/runonce.go` is not edited.
4. Docs: docs/design/earnings-fallback.md#decisions.

## Checked by CI

- `go test ./internal/contracts/... ./internal/sources/... ./internal/jobs/earningsrefresh/... ./cmd/ot/...`, `go vet ./...`, `golangci-lint` and `scripts/arch` pass; the regeneration check shows `internal/contracts/gen/records.go` equal to its schema.
- `uv sync --locked`, `ruff check`, `mypy --strict` and `pytest -n auto` pass in `pyside/`.
- The tests are parallel with a temporary directory per file, a fake clock, no sleep, no live service and no PostgreSQL; the migration is applied in the `pg` CI job.
- None of these is an acceptance item above.

## Left out

- Fiscal period, estimate and time of day for a Yahoo date: Yahoo's calendar gives a date (or a two-date range) and an estimate of unknown currency; storing it would mean a made-up period. Not cut.
- Showing the source on a page ("from Yahoo"): the source is stored and in `rd.earnings_date_r1`; the pages are unchanged (Decision 5). Whether the owner wants a mark on the row is a separate word.
- FMP: free-tier limits not established; a key and an account would be a second-opinion case. Not cut.
- IBKR Wall Street Horizon: a paid subscription, not used (owner).
- A second fallback after yfinance, and a cross-check of two sources that disagree: the primary wins and yfinance only fills a gap; reconciling is not cut.
- A Yahoo date that is an estimate and later moves: the daily run asks again each day an entry has no primary date, so the newest answer row follows Yahoo (see Decision 1: the older data row stays as history under its own key); no other history of moves is kept.
- Rewriting earlier days: not done; `-force` of a stored day asks again.
- The earlier reader bug (newest `earnings_check` hides a stored future date): closed by 4008, 4009, 4011, 4038.

## Adversarial

Full review: `the dispatcher's drafts folder, earnings-fallback-adversarial.md` (a local file, not committed). The earlier self-attack of the author is kept below it.

- Gap (stale primary date, no source mark, no job without an Alpha Vantage key): not adopted as a change; the last is now a Failure path.
- Unnamed risk and claim most likely false (key of the data row, a moved date): adopted. Decision 1 states the `(issuer, period_key)` key, that the newest answer row points to the new date and the old row stays as history; no store change.
- Simpler way: none; lock-in: accepted.
- Slip (symbol matching in `listed()`): adopted; issue 7 acceptance 2 and 5 say the fallback matches the symbol asked and tests a JP entry whose vendor symbol differs.

Self-attack by the author, no agent spawned.
- Gap against the goal: "more sources" is met for names the primary lists nothing for; names the primary lists wrongly (a stale date) are not corrected, since the primary wins.
- Biggest unnamed risk: Yahoo's date is often a company-projected estimate, and the row says only `yfinance`, not "estimate". The owner reads a date on a page with no flag. Not adopted: a new column or flag (a contract change); the source on the row is the record.
- Simpler way: store the Yahoo date without a migration by writing it as `source none`. Rejected: it would break the check `unsupported` rule and hide the source the owner asked to record.
- Claim most likely false: that `Ticker.calendar` returns a list under `Earnings Date` for both US and `.T` symbols in yfinance 1.8.0. The shape is now read from the 1.8.0 wheel (`_fetch_calendar`: dict, list of `datetime.date`, key absent without an earnings block), but whether Yahoo returns data for `.T` symbols was not called live. Issue 4 acceptance 2 treats any other shape as `source_changed` and an empty one as `no_data`, so a wrong shape shows as a gap, not as a wrong date.
- Likeliest slip: migration number 0072 if another migration lands first; the issue names the number by origin/main 076ebfac and the check fails loudly on a clash.
