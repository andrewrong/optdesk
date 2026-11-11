# FX cash: USD/JPY through an IB CASH contract, a decision-grade rate once per XNYS session day

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-11
Revision: 2

Source: the `fx_refresh` job failed 3 of 3 runs. Learned from option-trader (its advisor, 2026-11-11, from its origin/main): one pair USD/JPY, an IB Forex contract (secType CASH, exchange IDEALPRO, symbol USD, currency JPY) qualified by contract details and identity-checked; a streaming quote with no generic ticks; a reading is decision-grade only with a live data type, 0 < bid < ask, a spread policy, the FX market open and a tick age of at most 180 s, otherwise "unknown" with no hard-coded fallback; MYR is not on IDEALPRO.
Read on origin/main at 9a95c180: `AGENTS.md`, `docs/design/modules.md`, `docs/design/market-bars.md` (Decision 9), `docs/design/option-open-pull.md` (format model), `internal/jobs/fxrefresh/*`, `internal/broker/api.go`, `internal/broker/grpcbroker/convert.go`, `contracts/proto/ibkr/v1/{contract,quote_types}.proto`, `contracts/datasets/obs.fx_rate.r1.json`, `migrations/0021_bars.sql`, `internal/jobs/fredseries/job.toml`, `pyside/src/optdesk/ibkr/{spec,transport,ibgateway}.py`, `pyside/tests/live/`, `pyside/src/optdesk/contracts/api.py`, `cmd/ot/wire.go`.

## Requirement

optdesk gets a decision-grade USD/JPY rate from IB: one stored row per XNYS session day, written only from a live, valid, fresh quote of the IDEALPRO cash contract, and no row (so "FX unknown") otherwise.

## What main holds today

- `internal/jobs/fxrefresh/run.go:142` calls `ResolveContract` with `SecType: "CASH"`. `internal/broker/grpcbroker/convert.go` (`secTypeText`) maps only STK, IND and OPT, so `toSpec` returns Permanent "unknown sec type" and no call leaves the process. `contracts/proto/ibkr/v1/contract.proto` `SecType` has UNSPECIFIED, STOCK, INDEX, OPTION. `pyside/src/optdesk/ibkr/transport.py` (`_SEC_TYPES`) has the same three; `ibgateway.py` builds an `ib_async` `Contract` from the text of the sec type, so CASH needs no new code there once the text maps.
- After the resolve the job asks for the daily MIDPOINT bar (`HistoryClient.Bars`) and stores each bar close as the rate of the bar's day (`source` `ibkr_midpoint`, key `(pair, rate_day)`, insert only). A daily bar is a history value; it carries no bid, ask, data type or age, so it cannot be decision-grade.
- `broker.Snapshot` with `KindSnapshot` is already the quote asked for: a short streaming subscription with no generic ticks, released at bid and ask (`quote_types.proto`), returning `MarketDataType`, `Generation`, `ReceivedAt` (the pyside clock at the end of the wait, a receipt time), `EventTs` (the tick time, `ticker.time`, nil when the line gave none; `internal/broker/api.go`, `quote.proto` field 27, wanted by `spec.SNAPSHOT`), `Bid`, `Ask`.
- `ibgateway.py` reads `int(getattr(ticker, "marketDataType", default))` and `ib_async` defaults `Ticker.marketDataType` to 1, so a quote whose type tick has not arrived reads as type 1 (live). The resolve in `transport.py` copies the requested contract into the answer and sets only `con_id`, `exchange`, `trading_class` and `multiplier` from IB.

## Decisions

1. Add `SEC_TYPE_CASH = 4` to `SecType` in `contract.proto` and its generated Go and Python code (entry 1). Map it to the text `CASH` in grpcbroker (entry 3) and in pyside `transport.py` (entry 4). One value, no new enum, no pair or forex message: a pair is the existing `symbol` (base) and `currency` (quote) of `ContractSpec`.
2. Keep the scheduled-job model. `fx_refresh` stays a job of the `XNYS` calendar, `session_close` anchor plus 15 m, one key `USDJPY`. A permanent streaming service (option-trader's one line on its own client id) is not built: optdesk stores one reading a day, a second long-lived connection would need its own client id, pacer and generation handling, and nothing in optdesk reads the rate faster than daily.
3. The job reads a quote, not a bar: `ResolveContract`, then `Snapshot(KindSnapshot, [resolved contract])`. No `reqHistoricalData`, no generic ticks, no snapshot flag, no `whatToShow`: `KindSnapshot` is exactly option-trader's `reqMktData(contract, "", False, False)`. The job no longer calls `HistoryClient.Bars` and no longer backfills bars (the daily-bar backfill is the history grade of Decision 5 of Left out); the constructor signature stays as it is (Left out: the unused argument).
4. Identity check, after the resolve and before the quote: the answer is listed, and its contract has `SecType` `CASH`, `Symbol` `USD`, `Currency` `JPY`, `Exchange` `IDEALPRO` and `ConID` above 0. Otherwise the run fails Permanent ("identity"), nothing is stored. Honest scope: pyside echoes the request for `SecType`, `Symbol` and `Currency`, so only `ConID` and `Exchange` are IB's own answer; those two are the real check, the other three guard against a wiring slip in our code. The quote is asked for the resolved contract (with its `ConID`), and the answer's `Contract.ConID` must equal the resolved one (it catches a mixed-up row, not a wrong contract).
5. Validity rule, all of: quote `Status` is `StatusOK`; `MarketDataType` is 1 (a necessary check, not proof: `ib_async` defaults the type to 1 when no type tick arrived, so it rejects a reported frozen or delayed type 2, 3 or 4 and nothing more; the proof of a live tick is the tick-time rule below); `0 < Bid < Ask`; spread `(Ask-Bid)/mid` is at most `max_spread_ratio` (`job.toml`, 0.001, a value tuned without a design); `EventTs` is not nil (a quote without a tick time is "no valid quote", check `event_ts`); the FX market is open at `EventTs` (closed from Friday 17:00 to Sunday 17:00 America/New_York); `EventTs` is at most 180 s before `clock.Now()` and not after it. The age is measured on `EventTs`, the tick time, never on `ReceivedAt`, which is the end of the call and is always recent; a frozen or delayed line carries an old or no tick time and fails here. The quote's own `Generation` must be above 0 (one quote is one generation; the resolve answer carries none, so no cross-call generation compare is possible and none is claimed). Anything else is "no valid quote".
6. The stored rate is the mid `(Bid+Ask)/2`, rounded half-even to 3 decimals, as decimal text, computed on exact decimal values of the two floats with `math/big` (no new dependency; the arithmetic is ten lines and a library would add a decimal package for it). It is stored as `contracts.FxRate{Pair "USDJPY", RateDay the run's day, Source "ibkr_midpoint", ReceivedAt the quote's}`: the source word stays so the dataset, the table CHECK and the readers do not change. Its meaning widens: before, the close of the IB daily MIDPOINT bar; from entry 5, the IDEALPRO bid/ask mid at about 16:15 ET. Rows already stored stay and mix with the new ones, and nothing marks which is which (the rows differ by a few hours of the day, both are IB mids); a distinct grade is Left out. A row for that day that already exists is left as it is (a rerun inserts nothing).
7. No valid quote on a run is Retryable: the result has `Returned` 0 and a typed error whose text starts `no_valid_quote` and names the failed check (the first that failed), so the runner's retry window applies; a Permanent error is only the identity check, a bad day or a bad manifest. A failed weekday run leaves no row and the next row is a day later; there is no catch-up. A weekday session close is inside FX hours, so a failure there is a gateway or data problem, not a closed market. No hard-coded, default or last-known rate is ever written (ADR 0004).
8. Weekend and holiday: `XNYS` has no session, so the job does not run and writes nothing; Friday's row stays the newest. The reading rule belongs to the consumer, as in `market-bars.md` Decision 9: `LatestFxRate(pair, asOf)` returns the newest row on or before `asOf`, and a consumer that shows a rate accepts a row of at most 10 days before the display day, beyond that it shows "no rate". XNYS holidays that fall on a weekday are FX-open days with no row, so a gap of 4 days (Friday to Tuesday) is normal and is inside the look-back. This design changes no reader.
9. MYR: `fx_refresh` asks only for USDJPY today, so there is nothing to drop from IB. IDEALPRO has no USD.MYR (MYR is non-deliverable), so MYR never comes from IB. The source, when a design needs it, is FRED `DEXMAUS` (daily, reference grade): `fred_series` takes its series from the `series` list of its `job.toml`, so it carries `DEXMAUS` (and `DEXJPUS`, the USD/JPY history reference) by adding the id to that list. Not done here: nothing reads MYR, and household totals never convert (Left out).
10. Supersedes `docs/design/market-bars.md` Decision 9 (the `fx_refresh` sentence that asks the daily `MIDPOINT` bar): from the merge of this design, `fx_refresh` reads a quote as Decisions 3 to 7 say; the pair, the 10-day reading rule and "FX unknown" of that decision stay. This design edits no other file; the text of that decision is changed by the owner in the merge of this design (owner-only path), not by an issue.
11. Evidence before the job: the read-only live check of entry 4 (`pyside/tests/live/`, `OT_LIVE_PYSIDE=1`) is a required step, not optional: entry 5 does not start until the window has run it against the deployed pyside and it showed a USD.JPY quote with `0 < bid < ask`, a non-nil `EventTs` within 180 s and market data type 1. If it shows no `EventTs`, this design is revised before entry 5.
12. Not adopted from option-trader because optdesk's shape does not need it: rotating or fixed client ids per service (the existing pyside connection serves the call), the 15 s sampling and the state-change log (no stream), a switch to delayed data on error 10197 or 1100, and any re-request during error 2110 (a quote with a non-OK status is a failed run, retried by the runner's window, never re-requested inside the run).

## Modules

No row of `docs/design/modules.md` changes. `contracts`, `broker`, `pyside` and `jobs` keep their paths and interface files; the broker interface file `internal/broker/api.go` is not edited (`KindSnapshot` and the `Quote` fields exist). `pyside/src/optdesk/ibkr/transport.py` is owner-only (the owner merges entry 4); `contracts/proto/` is owner-only (the owner merges entry 1). Each entry stays inside one module row (1 and 2 contracts, 3 broker, 4 pyside, 5 jobs).

## Interface

- Proto (owner merges): `enum SecType` in `contracts/proto/ibkr/v1/contract.proto` gets `SEC_TYPE_CASH = 4;` with the comment "a cash pair of IDEALPRO: symbol is the base currency, currency the quote currency". Nothing else in `contracts/proto` changes. The generated Go (`internal/contracts/genpb/contract.pb.go`, `pb.SecType_SEC_TYPE_CASH`) and Python (`pyside/src/optdesk/contracts/genpb/**`) come with it, in the same pull request.
- Go: `internal/broker/grpcbroker/convert.go` `secTypeText` gets `pb.SecType_SEC_TYPE_CASH: "CASH"`; `secTypeOfText` is its inversion, so `toSpec` and `fromSpec` both handle it. `broker.Contract.SecType` is a string and stays one.
- Python: `optdesk.contracts.api` imports `SEC_TYPE_CASH` from the generated `contract_pb2` and lists it in `__all__`; `transport.py` `_SEC_TYPES` gets `pb.SEC_TYPE_CASH: "CASH"` (its inverse `_SEC_TYPE_VALUES` follows).
- Job: `fxrefresh.New` keeps its signature (`NewFxRefreshFunc`); `job.toml` loses `duration_days` and gains `max_spread_ratio` and `max_age = "180s"` (the age of the tick time). No dataset, table, migration or page schema changes.

## Issues

### 1 sec-type-cash-proto (interface)

Module: contracts. Size: S.
Files: `contracts/proto/ibkr/v1/contract.proto`, `internal/contracts/genpb/**`, `pyside/src/optdesk/contracts/genpb/**`
Design: docs/design/fx-cash.md#Interface
Order: first.
Sequence: the pull request is the owner's merge (`contracts/proto`); entries 2, 3 and 4 follow it.
Goal: `SEC_TYPE_CASH = 4` exists in `SecType` with its comment, and the generated Go and Python output.
Acceptance:
1. `grep -c "SEC_TYPE_CASH = 4" contracts/proto/ibkr/v1/contract.proto` prints 1.
2. `grep -rl "SEC_TYPE_CASH" internal/contracts/genpb | wc -l` prints at least 1.
3. `grep -rl "SEC_TYPE_CASH" pyside/src/optdesk/contracts/genpb | wc -l` prints at least 1.
4. `git diff --name-only $(git merge-base HEAD origin/main) -- contracts/proto | wc -l` prints 1, and `git diff $(git merge-base HEAD origin/main) -- contracts/proto | grep -c "^-[^-]"` prints 0 (no existing value changes).
Checked by CI: ci-go, ci-python, the buf lint and generated-drift check; `Contracts: changed (proto)`.

### 2 pyside-contracts-sec-type-cash (interface)

Module: contracts. Size: S.
Files: `pyside/src/optdesk/contracts/api.py`, `pyside/tests/contracts/test_contracts_api.py`
Design: docs/design/fx-cash.md#Interface
Order: after 1.
Sequence: the generated `contract_pb2` holds the name (entry 1); entry 4 follows this one (`transport.py` imports `optdesk.contracts.api as pb` and nothing else of contracts).
Goal: `optdesk.contracts.api` exports `SEC_TYPE_CASH`, and the contract test asserts its value.
Acceptance:
1. `grep -c "SEC_TYPE_CASH" pyside/src/optdesk/contracts/api.py` prints 2 (the import and `__all__`).
2. `grep -c "SEC_TYPE_CASH == 4" pyside/tests/contracts/test_contracts_api.py` prints 1.
3. `git diff --name-only $(git merge-base HEAD origin/main) | grep -vxF -e pyside/src/optdesk/contracts/api.py -e pyside/tests/contracts/test_contracts_api.py` prints nothing.
Checked by CI: ci-python (`uv sync --locked`, `ruff check`, `mypy --strict`, `pytest -n auto`); `Contracts: unchanged`.

### 3 broker-cash-sec-type

Module: broker. Size: S.
Files: `internal/broker/grpcbroker/convert.go`, `internal/broker/grpcbroker/convert_test.go`
Design: docs/design/fx-cash.md#Interface
Order: after 1.
Goal: a `broker.Contract` with `SecType` `CASH` converts to `SEC_TYPE_CASH` and back, so `ResolveContract` and `Snapshot` of the USD.JPY contract reach the service instead of failing Permanent in `toSpec`.
Acceptance:
1. `grep -c "SEC_TYPE_CASH" internal/broker/grpcbroker/convert.go` prints 1 and `grep -c '"CASH"' internal/broker/grpcbroker/convert.go` prints 1.
2. `grep -c "CASH" internal/broker/grpcbroker/convert_test.go` prints at least 2 (the round trip of a contract with symbol USD, exchange IDEALPRO, currency JPY, and the conversion back of the proto value), and `grep -c "unknown sec type" internal/broker/grpcbroker/convert_test.go` prints at least 1 (an unmapped text is still refused; no such string is in `convert_test.go` today, so the issue's own test adds it).
3. `git diff $(git merge-base HEAD origin/main) -- internal/broker/grpcbroker/convert.go | grep -c "^-[^-]"` prints 0 (no existing mapping changes).
4. `git diff --name-only $(git merge-base HEAD origin/main) | grep -vxF -e internal/broker/grpcbroker/convert.go -e internal/broker/grpcbroker/convert_test.go` prints nothing.
Checked by CI: ci-go; `go test ./internal/broker/... -race -count=1`.

### 4 pyside-transport-cash

Module: pyside. Size: S.
Files: `pyside/src/optdesk/ibkr/transport.py`, `pyside/tests/ibkr/test_transport_cash.py`, `pyside/tests/live/test_live_cash.py`
Design: docs/design/fx-cash.md#Interface
Order: after 1, 2.
Sequence: `transport.py` is owner-only, so the owner merges this pull request; it follows entries 1 and 2 (the name through `optdesk.contracts.api`).
Goal: a `ContractSpec` with `SEC_TYPE_CASH` reaches `ib_async` as `Contract(secType="CASH", symbol="USD", exchange="IDEALPRO", currency="JPY")` for the resolve and for the quote, and a resolve answer of a CASH contract carries `SEC_TYPE_CASH` back; the quote kind is the existing `SNAPSHOT`, unchanged; a read-only live test of the USD.JPY quote exists for the window to run.
Acceptance:
1. `grep -c "SEC_TYPE_CASH" pyside/src/optdesk/ibkr/transport.py` prints 1 and `grep -c '"CASH"' pyside/src/optdesk/ibkr/transport.py` prints 1.
2. `git diff $(git merge-base HEAD origin/main) -- pyside/src/optdesk/ibkr/transport.py | grep -c "^-[^-]"` prints 0.
3. `grep -c "SEC_TYPE_CASH" pyside/tests/ibkr/test_transport_cash.py` prints at least 2 (the request path and the resolve answer path, over the fake gateway of `pyside/tests/ibkr`); `grep -c "sleep\|connect(" pyside/tests/ibkr/test_transport_cash.py` prints 0.
4. `grep -c "OT_LIVE_PYSIDE\|skip" pyside/tests/live/test_live_cash.py` prints at least 1, `grep -c "event_ts\|EventTs" pyside/tests/live/test_live_cash.py` prints at least 1, and `grep -ci "order\|place" pyside/tests/live/test_live_cash.py` prints 0 (read-only: resolve and snapshot of USD.JPY, asserts `0 < bid < ask`, a tick time and data type 1; follows `test_live_pyside.py`).
5. `git diff --name-only $(git merge-base HEAD origin/main) | grep -vxF -e pyside/src/optdesk/ibkr/transport.py -e pyside/tests/ibkr/test_transport_cash.py -e pyside/tests/live/test_live_cash.py` prints nothing.
Checked by CI: ci-python (`uv sync --locked`, `ruff check`, `mypy --strict`, `pytest -n auto`); the live test is skipped there. Run by the window, not CI, before entry 5 starts (Decision 11): `OT_LIVE_PYSIDE=1`, read-only, no write rpc.

### 5 fxrefresh-quote-rate

Module: jobs. Size: M.
Files: `internal/jobs/fxrefresh/run.go`, `internal/jobs/fxrefresh/run_test.go`, `internal/jobs/fxrefresh/quote.go`, `internal/jobs/fxrefresh/quote_test.go`, `internal/jobs/fxrefresh/job.toml`, `internal/jobs/fxrefresh/doc.go`
Design: docs/design/fx-cash.md#Decisions
Order: after 3, 4, and after the live run of Decision 11.
Sequence: after 3 (Go side) and 4 (the service side) and after the window's live read-only run of entry 4's test; `types.go`, `types_test.go` and `cmd/ot` are not touched, so the constructor signature stays.
Goal: `fx_refresh` resolves the USD.JPY cash contract, checks its identity, asks `Snapshot(KindSnapshot)`, applies the validity rule of Decision 5 (tick-time age) and stores the half-even 3-decimal mid of Decision 6 as the rate of the run's day; with no valid quote it stores nothing and fails Retryable `no_valid_quote`; it no longer calls `Bars`.
Acceptance:
1. `grep -c "KindSnapshot" internal/jobs/fxrefresh/run.go` prints at least 1 and `grep -c "Bars(" internal/jobs/fxrefresh/run.go internal/jobs/fxrefresh/quote.go` prints 0 on both lines.
2. `grep -c "max_spread_ratio\|max_age" internal/jobs/fxrefresh/job.toml` prints 2 and `grep -c "duration_days" internal/jobs/fxrefresh/job.toml internal/jobs/fxrefresh/run.go` prints 0 on both lines.
3. `grep -c "IDEALPRO" internal/jobs/fxrefresh/run.go internal/jobs/fxrefresh/quote.go` prints at least 1 in total, `grep -c "no_valid_quote" internal/jobs/fxrefresh/quote.go` prints at least 1, and `grep -c "ibkr_midpoint" internal/jobs/fxrefresh/run.go` prints at least 1.
4. `grep -c "^func Test" internal/jobs/fxrefresh/quote_test.go` prints at least 9, one per rule: identity mismatch, frozen or delayed type, `bid >= ask`, a zero or negative side, wide spread, FX market closed (a Saturday `EventTs` from the fake clock), `EventTs` nil, `EventTs` older than 180 s (with a fresh `ReceivedAt`), and the half-even mid of 151.2345 and 151.2355 at 3 dp.
5. `grep -c "time.Sleep\|time.Now()" internal/jobs/fxrefresh/run_test.go internal/jobs/fxrefresh/quote_test.go` prints 0 on both lines; `git diff --name-only $(git merge-base HEAD origin/main) | grep -vc "^internal/jobs/fxrefresh/"` prints 0, and `wc -l < internal/jobs/fxrefresh/run.go` prints under 600.
Checked by CI: ci-go; `go test ./internal/jobs/fxrefresh/... -race -count=1` (a valid quote stores one row; an invalid one stores none and returns Retryable; a rerun with the day stored inserts nothing; the identity check is Permanent).

## Checked by CI

Every test run named above runs in CI, not in acceptance. No test connects to a live broker; tests use the fake broker of the test kit and the fake clock, `t.TempDir()` and `t.Parallel()`.

## Left out

- A permanent USD/JPY service with its own client id and a 15 s sampling (option-trader's shape): optdesk reads the rate once a day.
- The daily-bar history of FX (what `fx_refresh` stored through `HistoryClient.Bars`) and any backfill of missed days: rows already stored stay; a missed day is "FX unknown", as `market-bars.md` Decision 9 already says. FRED `DEXJPUS` as the history reference is a manifest change of `fred_series` (add the id to `series`) and its own later design if a figure needs it.
- The unused `broker.HistoryClient` argument of `fxrefresh.New` and the history dial that `fx_refresh` shares in `cmd/ot/wire.go`: removing them changes `types.go`, `types_test.go` and `cmd/ot` (module ot), a later wiring issue.
- Marking or migrating the old `ibkr_midpoint` bar rows: they stay, mixed with the new mids.
- A distinct stored grade (a `source` word for the quote mid next to `ibkr_midpoint`, a dataset r2, a migration) and a stored `no_valid_quote` row: the table has one source and no status column; today the absence of a row is the signal.
- Proof of a live type from pyside (a type-tick flag in `Quote`, a proto change in `ibgateway.py` and `quote.proto`): the data type is a weak check, the tick-time age carries the freshness; a later design if the live run shows a stale tick can pass.
- A cross-call connection-generation check between the resolve and the quote (the resolve answer carries no generation).
- MYR: `DEXMAUS` through `fred_series`, or display-only; nothing reads MYR, and household totals never convert.
- Other pairs (EUR, CNH, XAUUSD) and an FX position or order: no cash order is placed.
- The display rule for the 10-day look-back and the "no rate" note: pages and `LatestFxRate` consumers, not this design.

## Adversarial

1. Gap against the goal (stored value mislabeled, `market-bars.md` Decision 9 false, missing-day policy, no `no_valid_quote` row): adopted in part. The widened meaning of `ibkr_midpoint` and the mixing of old and new rows are now stated (Decision 6, Left out); Decision 10 supersedes Decision 9 of `market-bars.md`, whose text the owner changes in the merge because this issue set edits only this file; the 4-day holiday gap is stated (Decisions 7, 8); a stored `no_valid_quote` row stays Left out (no status column).
2. Unnamed risk (a) live type passes by default: holds (`ib_async` defaults the type to 1, `ibgateway.py:516`); adopted as an explicit weakness in Decision 5, with proof moved to the tick-time rule, and a proto-level type flag Left out.
3. Unnamed risk (b) age measures receipt: holds (`ReceivedAt` is `clock.now()` at the end of the wait); adopted and fixed more cheaply than a proto change, because `Quote.EventTs` already exists (`api.go`, `quote.proto` 27, `convert.go`): the age, the open-market check and a nil check now use `EventTs`; the review's claim that Go has no tick-time field is wrong.
4. Unnamed risk (c) identity is tautological: holds (resolve echoes the request, `transport.py:433-434`); adopted as an honest scope statement in Decision 4, checks kept.
5. Simpler way (FRED DEXJPUS as the replacement): not adopted, a noon reference rate cannot be decision-grade; it stays a later history reference.
6. Slip risk (owner-only chain, live check optional) and the failing `unknown sec type` grep: adopted; the live read-only run is required before entry 5 (Decision 11, entry 4 Files and acceptance, entry 5 Order), and entry 3 states that its test adds the string.
