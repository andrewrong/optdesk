# Market bars: daily closes (trades and weekly adjusted_last), the daily USD.JPY rate, their store tables and the own HV30

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-09
Revision: 1

Status: draft, not yet a pull request. Builds on main at 28d0e86: `internal/compute` (`Figures.HistoricalVol`, `Rules`), `internal/jobs/computefigures` (the figures job, readings), `internal/jobs/earningsrefresh` and `jobs.WatchEntry` (#752), `internal/store/instrument` (`InstrumentResolver`), `internal/broker` (`ReferenceClient` as the pattern for a second client), `ibkr.v1.MarketDataService`, `docs/design/alignment-v25.md`. Not on main yet: the watch loader in `cmd/ot` (`docs/design/event-sources.md` entry `watch-loader-earnings-wire`), which entry 11 needs for `watch.toml`.
Needs the owner (owner-only paths): `contracts/proto/ibkr/v1/marketdata.proto` (entry 2) and `pyside/src/optdesk/ibkr/transport.py` (entry 6: the servicer method of the new rpc lives there; no other file can host it). No ratchet ceiling is needed: the store interface goes in the new package's own `internal/store/bar/api.go`, not in `internal/store/api.go` (799 lines on main).
Source: plan v2.5 section 11 rows W5-6 "daily bars (trades + weekly adjusted_last), Stooq" and "FX job", W8-9 "HV", W3-4 "store tables ... fx, bars"; the gap table of 2026-11-09 ("Daily bars / HV30 / Stooq / FX / hv30_own: NOT DESIGNED"); `docs/design/alignment-v25.md` Decision 3 item 3 and the rows "weekly adjusted_last re-pull with adjust_epoch", "stock history two bases". Owner 2026-11-09: Stooq coarse layer, scanner and the 15 s FX stream are dropped.
Scoring and the scanner are another design and are not here.

## Goal

After each XNYS close, `bars_refresh` stores the daily bars of every name on `watch.toml` and of four indices, and `fx_refresh` stores the day's USD.JPY mid; `compute_figures` then stores the own HV30 (`market.hv30_own`) of each underlying that has option rows. The pages that already print "HV30 缺" read a stored reading instead of a gap, and the JPY figures that say `unsupported_currency` get a stored rate to be read by their own designs.

## What main holds today

- The function exists and is tested: `Figures.HistoricalVol(id, closes, asOf)` (`internal/compute/api.go`, `internal/compute/figures/series.go`): sample stdev of the last N log returns times the square root of `TradingDays`, needs N+1 closes above zero, age check by `MaxAgeDays`. It has no rule in `internal/compute/rules/rules.json`, no register entry and no caller outside tests (`market.hv30_own` was dropped from the registered ids for want of a producer: `compute-figures.md` Decision 5, `compute-core.md` Left out).
- No dataset, table, rpc or job holds a daily close of a stock, index or currency: `MarketDataService` has only `Snapshot`; `internal/store` has series points (FRED), option rows and quote snapshots only.
- Consumers found by grep (all print or wait for a gap today):
  - `option_data_page` `Underlying.vol_line` and `option_page` / `stock_option_page` `iv_line` say "HV30 缺" (`docs/design/option-pages-v22.md` Decisions 1, 5; "HV30 and IVP wait for the compute design"); no Go builder writes them.
  - `held_companies_summary` v1 to v3 and `home_page` v1 carry the field `hv30` (`contracts/schemas/page.held_companies_summary.v3.json`, `page.home_page.v1.json`); the old register id `companies.volatility.hv30` names them.
  - `companies_page` v2 has `volatility_note` ("the own series and the data day"); market page rows `rv20` and `iv_minus_rv20` are Gaps `not_built` (`market-page-producer.md`).
  - `option-candidates.md` Left out names "the ITM probability, the volatility spread and IV rank (they need the bars design)"; FX conversion is also named there, in `option-positions.md` (gap `unsupported_currency`) and in `option-ledger.md` (no conversion).
  - The report page blocks (`global_assets` proxies, price lines, core, diagnosis) need closes; their builders are the builders-wave, not this design.
  This design stores the numbers and the readings. It does not edit any page, schema or builder (Left out).

## Decisions

1. Producers in this design: the store tables and their readers, `bars_refresh`, `fx_refresh`, the HV30 step of `compute_figures`. Pages, scoring, the scanner and the report blocks are other designs.
2. Source of the bars: IBKR historical data through pyside, one new rpc `Bars` on the existing `MarketDataService` (unary, one contract, one kind, daily bars). The plan names IBKR bars; the glossary defines the index closes as IBKR `TRADES` daily bars; Stooq is dropped by the owner (Left out). The rpc and the servicer method are owner-only paths (header). The Go side is a separate `broker.HistoryClient`, as `ReferenceClient` is, so `broker.Broker` and its fakes stay unchanged.
3. Two bases, never mixed in one series: `trades` (IBKR `TRADES`, split-adjusted by IBKR, not dividend-adjusted; this is the basis of HV(n) in `GLOSSARY.md`) refreshed every run; `adjusted_last` (IBKR `ADJUSTED_LAST`, split and dividend adjusted) refreshed when the newest stored one is 7 or more calendar days old. `adjusted_last` has no reader in this design (returns and the 200-day line come with the pages that need them); it is stored because IBKR restates it and the history cannot be rebuilt later. A third word `midpoint` is used only by the FX job and is not a bar basis in `obs.price_bar`.
4. `adjust_epoch` (alignment: "weekly adjusted_last re-pull with adjust_epoch"). Rows are insert-only. A row key is (instrument_id, basis, adjust_epoch, session_day). The epoch of a (instrument, basis) starts at 1; the winner rows (what readers return) are those of the highest epoch of the pair. Restatement rule, in one pure file of the job (`restate.go`): when a pulled day that is already stored differs from the stored winner close by more than `restate_tolerance` (relative, exact `math/big.Rat`; default 0.00001, window default), the job pulls the whole backfill window and inserts every row of that pull under epoch + 1 in one call (one transaction, so a half pull never wins); otherwise it inserts only the days not stored under the current epoch. A split (trades) or a dividend or split (adjusted_last) shows up as such a difference. No other state is kept.
5. The session day of a bar is the exchange date that IBKR gives (`formatDate` 1), stored as it comes; there is no time-zone conversion and no join to `master.session_day`. A JP name therefore has its own JP dates and a US holiday has no US row.
6. Universe: every `WatchEntry` of `watch.toml` (US and JP stocks; the owner puts held names there, the earnings job reads the same file) plus the job manifest tables `[[index]]` for SPX, NDX, RUT and N225 (the four rows of the market page; window default, see question B). The instrument id is the one `option_eod` uses: `ResolveInstrument` with the identifier `ibkr_conid`, so an underlying's bars and option rows share one id. An index entry whose `con_id` is 0 is resolved once per run through `broker.Broker.ResolveContract`; not listed is Permanent for that key.
7. Depth and pacing: the first pull of a name asks for `backfill_days` = 400 calendar days (about 275 sessions: more than the 200-day line and one year as ADR 0003; window default). A daily pull asks for 5 days. IBKR paces historical requests at 55 per 10 minutes and refuses an identical request within 15 seconds (ADR 0003): pyside keeps one pacer on the injected clock, waits up to 300 seconds and then answers `RESOURCE_EXHAUSTED`; the Go client maps that to Retryable and its call timeout `HistoryTimeout` is 330 seconds (a new `broker.Options` field). One key (one name) is one runner key, so a spill goes to the runner's retry and the pull is idempotent (insert, nothing replaced). Schedule: XNYS, `session_close` plus 10 minutes, `run_bound` 10 minutes per key (window default; `compute_figures` runs at plus 20 minutes and reads whatever is stored then, with its `data_day`).
8. Tables, schema `obs`, one migration, one store package `internal/store/bar`, one role `ot_w_bar` (line `OT_PG_DSN_BAR`): `obs.price_bar` (typed columns: `instrument_id`, `session_day`, `basis`, `adjust_epoch`, `open`, `high`, `low`, `close`, `volume`, `currency`, `source`, `received_at`; prices as decimal text, no float) and `obs.fx_rate` (`pair`, `rate_day`, `rate`, `source`, `received_at`). Views `rd.price_bar_r1` (winner rows) and `rd.fx_rate_r1` for the roles `ot_w_bar` (SELECT; the job reads through the view) and `ot_ro_pages` (SELECT; the figures job and pages read with the pages DSN, as the series store is read). No UPDATE, DELETE or TRUNCATE for anyone but the owner.
9. FX: pair `USDJPY` only (the plan's JPY exposure; no other pair is asked for). `fx_refresh` asks IBKR for the daily `MIDPOINT` bar of the IDEALPRO cash contract USD.JPY (resolved through `Broker.ResolveContract`) for 5 days after each XNYS close and inserts the close as the rate of the bar's date, `source` `ibkr_midpoint`, a conflicting key left as it is. There is no hard-coded or default rate (ADR 0004): `LatestFxRate(pair, asOf)` returns the newest row on or before `asOf` and a found flag; no row is "FX unknown", and what age is too old is the consumer's rule. Where IBKR cuts the FX day is IBKR's convention, unverified (as the glossary says for XAUUSD). The 15 s stream and the ADR 0004 shadow period and ECB cross-check are not built (Left out).
10. HV30 producer (the "producer feeding the existing HV function"): a step of `compute_figures`, after IV30 and before the market figures, with the reading name `market.hv30_own` (GLOSSARY.md "HV30", "HV(n)"), subject kind `underlying`, one reading per subject the job already lists (the underlyings with option rows on the day). Rule `market.hv30_own` v1: convention `hv30_v1`, unit `decimal` (an annual fraction: 0.31 is 31 percent), scale 6, `n` 30, `trading_days` 252, `max_age_days` 4, implemented by `HistoricalVol` (so no hashed body changes). Input: the winner `trades` rows of the underlying with `session_day` between `asOf` minus 70 calendar days and `asOf`, ascending, close text to float64 at the job's edge (the precedent of `ivinput.go`); fewer than 31 closes is a missing reading `short_history`, none is `no_input`, a stale newest close is `stale_input`, never a zero. IBKR's own HV30 (`Quote.HistoricalVol`) is not read here and is never a fallback. A new field `PriceBars bar.PriceBarReader` of `jobs.ComputeStores`; nil means the step is skipped and the job behaves as today.
11. Own code behind an interface: the broker client is `broker.HistoryClient` (a library or another gateway can replace it); the restatement rule and the pacer are small glue and live inside their package (`restate.go`, `history.py`), reached only through the job and the servicer. Library check: `math/big` of the standard library for exact compare (no decimal library in `go.mod`, as `option-candidates.md` Decision 1); no new direct dependency.
12. Window default — owner may change: items marked in `## Left out` and questions A to D at the end of this section: (A) the 7-day cadence of `adjusted_last`; (B) the four indices; (C) 400 days of first backfill; (D) the plus 10 minutes slot. Each is a manifest value, so a change is a one-line edit and not a redesign.

Questions for the owner (one word each), the window defaults above:
- A. `adjusted_last` refresh: a) weekly (7 days) b) daily.
- B. Indices beside the watch list: a) SPX NDX RUT N225 b) add TOPX and XAUUSD too.
- C. First backfill: a) 400 days b) 5 years (the probe of plan 11.1 decides the real depth).
- D. Slot: a) close plus 10 minutes b) close plus 50 minutes (after `compute_figures`; HV then lags one day).

## Modules

No new row; one changed cell: the entry cell of the store row gains `internal/store/bar` (a sub-package that jobs may import, as the entry cell of contracts lists `internal/contracts/genpb`; `docs/design/settings-form.md` Decision 6 makes the same kind of edit for `internal/store/ownerset`, so whichever pull request lands second merges the two names). The new directories belong to rows that exist: `internal/store/bar` and `migrations/**` to store, `internal/jobs/barsrefresh` and `internal/jobs/fxrefresh` to jobs, the new proto outputs to contracts, `pyside/src/optdesk/ibkr/history.py` to pyside (first matching row of each path). This pull request edits `docs/design/modules.md` in one sentence, the list of implementation packages `cmd/ot` may import, adding `internal/store/bar`, `internal/jobs/barsrefresh` and `internal/jobs/fxrefresh`, and regenerates `arch.toml` with `go run ./scripts/arch` (the precedent is `option-candidates.md`). `.go-arch-lint.yml` is tree-derived and appears with the first issue of each package. Interface files touched, each additive: `internal/contracts/api.go`, `internal/store/bar/api.go` (new), `internal/broker/api.go`, `internal/jobs/api.go`, `pyside/src/optdesk/ibkr/api.py`, and the generated types of `marketdata.proto`.

| # | issue | module | order |
|---|---|---|---|
| 1 | bar-datasets (interface) | contracts | first |
| 2 | bars-proto (interface) | contracts (owner-only proto) | first |
| 3 | bar-store-api (interface) | store, plus repo (licensed pair) | after 1 |
| 4 | bar-store-impl | store | after 1, 3 |
| 5 | broker-history (interface and implementation) | broker | after 2 |
| 6 | pyside-history (interface and implementation) | pyside (owner-only transport.py) | after 2 |
| 7 | bars-job | jobs, plus repo (licensed pair) | after 3, 4, 5 |
| 8 | fx-job | jobs, plus repo (licensed pair) | after 3, 4, 5, 7 |
| 9 | hv30-rule | compute, plus docs (judged: the register is under docs) | first |
| 10 | hv30-step | jobs | after 3, 7, 8, 9 |
| 11 | bars-wire | ot | after 4, 7, 8, 10 |

The three entries that edit `.go-arch-lint.yml` (3, 7, 8) are one serial chain in that order; 7, 8 and 10 share `internal/jobs/api.go`, `internal/jobs/api_test.go` and `docs/contracts/jobs.md` and name each other. Entry 11 also shares `cmd/ot` files with `docs/design/event-sources.md` `watch-loader-earnings-wire` and with the account wire; it rebases on whichever lands first.

## Interface

- contracts (entry 1): datasets `obs.price_bar` r1 (key instrument_id, basis, adjust_epoch, session_day; `basis` enum `trades`, `adjusted_last`; `adjust_epoch` integer from 1; `open`, `high`, `low`, `close` decimal text; `volume` decimal text or null; `currency`; `source` `ibkr`; `received_at`) and `obs.fx_rate` r1 (key pair, rate_day; `pair` pattern `^[A-Z]{6}$`; `rate` positive decimal text; `source` `ibkr_midpoint`; `received_at`); generated `contracts.PriceBar`, `contracts.FxRate`. Proto (entry 2), in `marketdata.proto`: `rpc Bars(BarsRequest) returns (BarsResponse)`; `enum BarKind { BAR_KIND_UNSPECIFIED = 0; BAR_KIND_TRADES = 1; BAR_KIND_ADJUSTED_LAST = 2; BAR_KIND_MIDPOINT = 3; }`; `BarsRequest { ContractSpec contract = 1; BarKind kind = 2; int32 duration_days = 3; }` (con_id required, duration 1 to 730, else INVALID_ARGUMENT; the unset kind is an error); `Bar { string day = 1; string open = 2; string high = 3; string low = 4; string close = 5; optional string volume = 6; }`; `BarsResponse { repeated Bar bars = 1; }` oldest first. No account id, token or address in any message.
- store, package `internal/store/bar`, file `api.go` (declarations only; entry 3): `PriceBarWriter.InsertPriceBars(ctx, []contracts.PriceBar) (inserted int, err error)` (one call is one transaction; a key that exists is left as it is); `PriceBarReader.PriceBars(ctx, instrumentID int64, basis string, from, to contracts.Day) ([]contracts.PriceBar, error)` (winner rows, ascending by day, rows carry `AdjustEpoch`); `FxRateWriter.InsertFxRates`; `FxRateReader.FxRates(ctx, pair, from, to)` and `LatestFxRate(ctx, pair string, asOf contracts.Day) (contracts.FxRate, bool, error)`; `BarStore` (all four and `Close`); `OpenBarFunc`; `bar.DSNEnvBar = "OT_PG_DSN_BAR"`.
- broker (entry 5): `type BarKind int` (`BarTrades`, `BarAdjustedLast`, `BarMidpoint`); `type Bar struct{ Day, Open, High, Low, Close string; Volume *string }`; `HistoryClient.Bars(ctx, c Contract, kind BarKind, days int) ([]Bar, error)` and `Close() error`; `NewHistoryFunc func(ctx, socketPath string, o Options) (HistoryClient, error)`; `Options.HistoryTimeout`.
- pyside (entry 6): in `ibkr/api.py` `BarKey`, `BarsRequest`, `Bar` (frozen dataclasses, prices as text) and `class History(Protocol)` with `async def bars(self, req) -> Sequence[Bar]`; implementation in `history.py` (pacer on the injected clock), the gateway adapter in `ibgateway.py`, the servicer method `Bars` in `transport.py`.
- jobs (entries 7, 8, 10): `NewBarsRefreshFunc func(broker.Broker, broker.HistoryClient, store.InstrumentResolver, bar.PriceBarWriter, bar.PriceBarReader, []WatchEntry, clock.Clock) (contracts.Job, error)`; `NewFxRefreshFunc func(broker.Broker, broker.HistoryClient, bar.FxRateWriter, bar.FxRateReader, clock.Clock) (contracts.Job, error)`; `ComputeStores.PriceBars bar.PriceBarReader`. Job ids `bars_refresh` and `fx_refresh`; run keys `{obs.price_bar, <issuer or index:SYMBOL>}` and `{obs.fx_rate, USDJPY}`; a key is present when its newest stored `trades` day (or rate day) is on or after the first row this run wrote or found, `Usable` counts such keys.
- compute (entry 9): no interface change; a rule row and a register entry.

## Issues

Every test calls `t.Parallel()` (Python: `pytest -n auto`), writes files only under a temporary directory, uses a fake clock, never sleeps, opens no port, and touches no live broker or database (a test that needs PostgreSQL carries the tag `pg`). Acceptance is checked offline by reading, grep and git; test runs are under "Checked by CI". `BASE` below is `git merge-base HEAD origin/main`.

### 1 bar-datasets (interface)

Module: contracts. Size: M.
Files: `contracts/datasets/obs.price_bar.r1.json`, `contracts/datasets/obs.fx_rate.r1.json`, `contracts/generate.sh`, `internal/contracts/gen/records.go`, `docs/contracts/datasets/obs.price_bar.md`, `docs/contracts/datasets/obs.fx_rate.md`, `contracts/fixtures/obs.price_bar.r1.json`, `contracts/fixtures/obs.fx_rate.r1.json`, `contracts/fixtures/MANIFEST.synthetic-v1.json`, `internal/contracts/schema_test.go`, `internal/contracts/api.go`, `internal/contracts/api_test.go`
Design: docs/design/market-bars.md#Interface
Order: first.
Goal: the two schemas of `## Interface`, the generated records exposed as `contracts.PriceBar` and `contracts.FxRate`, fixtures (price bar: one `trades` row epoch 1, one `adjusted_last` row epoch 2 with null volume; fx: one row), manifest hashes. `records.go` is generated and skipped by the ratchet.
Acceptance:
1. `grep -c "adjusted_last" contracts/datasets/obs.price_bar.r1.json` is at least 1 and `grep -c "adjust_epoch" contracts/datasets/obs.price_bar.r1.json` is at least 2; `grep -c "ibkr_midpoint" contracts/datasets/obs.fx_rate.r1.json` is at least 1.
2. `grep -c "obs.price_bar\|obs.fx_rate" internal/contracts/gen/records.go` is at least 2; `grep -c "PriceBar\|FxRate" internal/contracts/api.go` is at least 2.
3. `grep -c "obs.price_bar\|obs.fx_rate" contracts/generate.sh` is at least 4 and `grep -c "obs.price_bar\|obs.fx_rate" contracts/fixtures/MANIFEST.synthetic-v1.json` is at least 2.
4. `grep -c "float" contracts/datasets/obs.price_bar.r1.json contracts/datasets/obs.fx_rate.r1.json` prints 0 on both lines (prices are decimal text); `grep -c "account\|token" contracts/datasets/obs.price_bar.r1.json` prints 0.
5. `git diff BASE -- internal/contracts/api.go internal/contracts/schema_test.go | grep -c '^-[^-]'` prints 0 (additions only); `git diff --name-only BASE` lists no path outside the Files line.
Checked by CI: the generated-code check, `go test ./internal/contracts -race -count=1` with the fixture tests, `scripts/hygiene`.

### 2 bars-proto (interface)

Module: contracts. Size: S. The proto path is owner-only (the owner merges it).
Files: `contracts/proto/ibkr/v1/marketdata.proto`, `internal/contracts/genpb/marketdata.pb.go`, `internal/contracts/genpb/marketdata_grpc.pb.go`, `pyside/src/optdesk/contracts/genpb/ibkr/v1/marketdata_pb2.py`, `pyside/src/optdesk/contracts/genpb/ibkr/v1/marketdata_pb2.pyi`, `pyside/src/optdesk/contracts/genpb/ibkr/v1/marketdata_pb2_grpc.py`, `pyside/src/optdesk/contracts/genpb/ibkr/v1/marketdata_pb2_grpc.pyi`, `pyside/src/optdesk/contracts/api.py`, `pyside/tests/contracts/test_contracts_api.py`
Design: docs/design/market-bars.md#Interface (contracts, proto)
Order: first. It shares `pyside/src/optdesk/contracts/api.py` and `pyside/tests/contracts/test_contracts_api.py` with entries of other designs that add messages; it rebases on whichever lands first.
Goal: the rpc, enum and three messages of `## Interface`, the generated output with its source, and the re-export of `BarsRequest`, `BarsResponse`, `Bar` and `BarKind` in the contracts `api.py` (pyside imports generated code only there).
Acceptance:
1. `grep -c "rpc Bars(BarsRequest) returns (BarsResponse)" contracts/proto/ibkr/v1/marketdata.proto` prints 1; `grep -c "BAR_KIND_TRADES\|BAR_KIND_ADJUSTED_LAST\|BAR_KIND_MIDPOINT" contracts/proto/ibkr/v1/marketdata.proto` prints 3.
2. `grep -ci "account_id\|password\|token\|float\|double" contracts/proto/ibkr/v1/marketdata.proto` prints 0 (decimal text only).
3. `git diff BASE -- contracts/proto | grep -c '^-[^-]'` prints 0 (additive: `Snapshot` and its messages are unchanged).
4. `grep -c "BarsRequest" internal/contracts/genpb/marketdata.pb.go pyside/src/optdesk/contracts/genpb/ibkr/v1/marketdata_pb2.pyi pyside/src/optdesk/contracts/api.py` prints three counts, each at least 1.
5. `git diff --name-only BASE` lists no path outside the Files line.
Checked by CI: the generated-code check (buf lint and generate), `uv sync --locked`, `pytest -n auto` in `pyside` (`tests/contracts`), ci-go.

### 3 bar-store-api (interface)

Module: store, plus repo (the licensed pair: `.go-arch-lint.yml`, regenerated for the new package). Size: S.
Files: `internal/store/bar/api.go`, `internal/store/bar/api_test.go`, `docs/contracts/store.md`, `.go-arch-lint.yml`
Design: docs/design/market-bars.md#Interface (store). `internal/store/api.go` is not edited (799 lines on main); the interface file of this package is `internal/store/bar/api.go`, listed in the entry cell of the store row by this pull request.
Order: after 1 (`contracts.PriceBar`, `contracts.FxRate`); the `.go-arch-lint.yml` chain is 3, 7, 8 in that order.
Goal: the writer, reader, store and open types of `## Interface` and `DSNEnvBar` in package `bar`, declarations only, with doc comments that fix the promises: insert-only, one call one transaction, readers return the winner rows (highest epoch of the pair), ascending by day; `LatestFxRate` has no fallback. The contract test has a fake store and a caller that checks the winner rule on a two-epoch fake and the not-found answer.
Acceptance:
1. `grep -c "^type PriceBarWriter interface\|^type PriceBarReader interface\|^type FxRateWriter interface\|^type FxRateReader interface\|^type BarStore interface\|^type OpenBarFunc " internal/store/bar/api.go` prints 6; `grep -c 'DSNEnvBar = "OT_PG_DSN_BAR"' internal/store/bar/api.go` prints 1.
2. `grep -c "^func " internal/store/bar/api.go` prints 0 (declarations only); `git diff BASE -- internal/store/api.go | grep -c .` prints 0.
3. `grep -c "^func TestBarStoreContract\|^func TestBarWinnerIsHighestEpoch\|^func TestLatestFxRateNotFound" internal/store/bar/api_test.go` prints 3; `grep -c "time.Sleep\|time.Now()" internal/store/bar/api_test.go` prints 0.
4. `grep -c "winner\|epoch\|no fallback" docs/contracts/store.md` is at least 3 (0 on main); `grep -c "internal/store/bar" .go-arch-lint.yml` is at least 1; `git diff --name-only BASE` lists exactly the Files line.
Checked by CI: `go test ./internal/store/bar -race -count=1`, `go run ./scripts/arch --check`, `go run ./scripts/arch && git diff --exit-code arch.toml .go-arch-lint.yml`, ci-go.

### 4 bar-store-impl

Module: store. Size: M. Second opinion: a new database role `ot_w_bar`.
Files: `migrations/NNNN_bars.sql` (NNNN the next free number at cut time; migrations apply out of order, so no number is claimed here), `internal/store/bar/doc.go`, `internal/store/bar/store.go`, `internal/store/bar/read.go`, `internal/store/bar/store_test.go`, `internal/store/bar/store_pg_test.go` (tag `pg`), `internal/store/setup/setup.go`, `internal/store/setup/setup_test.go`, `internal/store/setup/bar_pg_test.go` (tag `pg`)
Design: docs/design/market-bars.md#Decisions (4, 8, 9)
Order: after 1, 3. It shares `setup.go`, `setup_test.go` and the next free migration number with the store entries of other designs and rebases on whichever lands first.
Goal: tables `obs.price_bar` and `obs.fx_rate` with the dataset keys and CHECKs (`basis` in the two words, `adjust_epoch >= 1`, `pair` six capitals, rate and prices parse as positive decimals), views `rd.price_bar_r1` (rows of the highest epoch of each instrument and basis) and `rd.fx_rate_r1`, role `ot_w_bar` (INSERT on the tables, SELECT on the views, USAGE on `obs` and `rd`; line `OT_PG_DSN_BAR`) and SELECT on both views for `ot_ro_pages`; `bar.Open(ctx, dsn)` is the `bar.OpenBarFunc`; readers read the views only; inserts are `ON CONFLICT DO NOTHING`, one call one transaction.
Acceptance:
1. `grep -c "CREATE TABLE obs.price_bar\|CREATE TABLE obs.fx_rate\|CREATE VIEW rd.price_bar_r1\|CREATE VIEW rd.fx_rate_r1" migrations/*_bars.sql` prints 4; `grep -ci "GRANT.*\(UPDATE\|DELETE\|TRUNCATE\)" migrations/*_bars.sql` prints 0.
2. `grep -c "GRANT SELECT ON rd.price_bar_r1 TO ot_ro_pages\|GRANT SELECT ON rd.fx_rate_r1 TO ot_ro_pages\|GRANT SELECT ON rd.price_bar_r1 TO ot_w_bar\|GRANT SELECT ON rd.fx_rate_r1 TO ot_w_bar" migrations/*_bars.sql` prints 4; `grep -c "GRANT INSERT ON obs.price_bar TO ot_w_bar\|GRANT INSERT ON obs.fx_rate TO ot_w_bar" migrations/*_bars.sql` prints 2.
3. `grep -c "obs.price_bar\|obs.fx_rate" internal/store/bar/read.go` prints 0 and `grep -c "rd.price_bar_r1\|rd.fx_rate_r1" internal/store/bar/read.go` is at least 2; `grep -c "ON CONFLICT DO NOTHING" internal/store/bar/store.go` is at least 2; `grep -rl "os.Getenv" internal/store/bar` prints nothing.
4. `grep -c "ot_w_bar\|OT_PG_DSN_BAR" internal/store/setup/setup.go` is at least 2; `grep -c "^func TestInsertQueryText\|^func TestWinnerQueryReadsView\|^func TestOneCallOneTransaction\|^func TestBarRoleSpec" internal/store/bar/store_test.go internal/store/setup/setup_test.go` prints counts that add up to 4 (new, no database needed).
5. `head -1 internal/store/bar/store_pg_test.go` prints `//go:build pg`; `grep -c "^func TestBarRoundTrip\|^func TestNewEpochWins" internal/store/bar/store_pg_test.go` prints 2; `git diff --name-only BASE` lists no path outside the Files line.
Checked by CI: `go test ./internal/store/bar ./internal/store/setup -race -count=1`; `go test -tags pg ./internal/store/...` (GitHub CI only); `go run ./scripts/arch --check`.

### 5 broker-history (interface and implementation)

Module: broker. Size: M.
Files: `internal/broker/api.go`, `internal/broker/api_test.go`, `internal/broker/grpcbroker/history.go`, `internal/broker/grpcbroker/history_test.go`, `internal/broker/grpcbroker/dial.go`, `docs/contracts/broker.md`
Design: docs/design/market-bars.md#Decisions (2, 7, 11). `Broker` is left unchanged on purpose: other types implement it outside these Files.
Order: after 2 (the generated client).
Goal: `HistoryClient`, `NewHistoryFunc`, `BarKind`, `Bar` and `Options.HistoryTimeout` (default 330 s in `dial.go`), and `grpcbroker.DialHistory` over `MarketDataServiceClient.Bars`; a day that is not `YYYY-MM-DD`, a price that is not a decimal, an unset kind and `RESOURCE_EXHAUSTED` (Retryable) are typed errors; the contract test has a fake client and a caller.
Acceptance:
1. `grep -c "^type HistoryClient interface\|^type NewHistoryFunc \|^type BarKind int\|^type Bar struct" internal/broker/api.go` prints 4; `grep -c "HistoryTimeout" internal/broker/api.go internal/broker/grpcbroker/dial.go` prints two counts, each at least 1.
2. `grep -c "^type Broker interface" internal/broker/api.go` prints 1 and `git diff BASE -- internal/broker/api.go | grep -A12 "^ type Broker interface" | grep -c '^[-+]'` prints 0 (`Broker` is unchanged).
3. `grep -rhE "^func (TestBarsContract|TestBarsBadDayIsError|TestBarsBadPriceIsError|TestBarsPacedIsRetryable|TestBarsUnsetKindIsError|TestHistoryTimeoutDefault)\(" internal/broker | awk 'END{print NR}'` prints 6 (new).
4. `grep -c "genpb\|grpcbroker" internal/broker/api.go` prints 0 (the interface imports no implementation); `grep -c "time.Sleep" internal/broker/grpcbroker/history_test.go` prints 0.
5. `git diff --name-only BASE` lists no path outside the Files line.
Checked by CI: `go test ./internal/broker/... -race -count=1`, `go run ./scripts/arch --check`, ci-go.

### 6 pyside-history (interface and implementation)

Module: pyside. Size: M. `pyside/src/optdesk/ibkr/transport.py` is owner-only (the servicer method of the new rpc lives there); the owner merges this pull request.
Files: `pyside/src/optdesk/ibkr/api.py`, `pyside/src/optdesk/ibkr/history.py`, `pyside/src/optdesk/ibkr/ibgateway.py`, `pyside/src/optdesk/ibkr/transport.py`, `pyside/tests/contracts/test_ibkr_api.py`, `pyside/tests/ibkr/test_history.py`, `pyside/tests/ibkr/test_transport.py`
Design: docs/design/market-bars.md#Decisions (2, 3, 7)
Order: after 2 (messages in the contracts `api.py`). It shares `api.py`, `ibgateway.py`, `transport.py` and `test_ibkr_api.py` with `docs/design/event-sources.md` `pyside-dividends` and the account-data pyside entries and rebases on whichever lands first.
Goal: `History` protocol and the three dataclasses in `api.py` (declarations only); `history.py` holds the pacer (at most 55 requests per 10 minutes, no identical request within 15 seconds, a wait of at most 300 seconds on the injected clock, then a paced answer) and the mapping of `duration_days` to a TWS duration string and of the three kinds to `TRADES`, `ADJUSTED_LAST`, `MIDPOINT` with bar size 1 day and `formatDate` 1; `ibgateway.py` adapts it to the gateway; `transport.py` adds `MarketDataServicer.Bars` (a missing `con_id` or a duration outside 1 to 730 is INVALID_ARGUMENT; the paced answer is RESOURCE_EXHAUSTED; no account id in a log line). The test file defines its own fake gateway and uses the fake clock of the kit.
Acceptance:
1. `grep -c "class BarKey\|class BarsRequest\|class Bar\b\|class History(Protocol)" pyside/src/optdesk/ibkr/api.py` prints 4; `grep -c "return " pyside/src/optdesk/ibkr/api.py` equals `git show origin/main:pyside/src/optdesk/ibkr/api.py | grep -c "return "` (no logic added).
2. `grep -c "async def Bars" pyside/src/optdesk/ibkr/transport.py` prints 1; `grep -c "RESOURCE_EXHAUSTED\|INVALID_ARGUMENT" pyside/src/optdesk/ibkr/transport.py` is at least the count on main plus 2.
3. `grep -c "def test_pacer_limits_to_55_per_10_minutes\|def test_identical_request_waits_15_seconds\|def test_wait_over_300_seconds_is_paced\|def test_kind_maps_to_tws_word\|def test_duration_days_maps_to_tws_string\|def test_missing_con_id_is_invalid" pyside/tests/ibkr/test_history.py` prints 6.
4. `grep -c "sleep(" pyside/tests/ibkr/test_history.py` prints 0; `grep -c "TRADES\|ADJUSTED_LAST\|MIDPOINT" pyside/src/optdesk/ibkr/history.py` is at least 3; each new file is under 600 lines (`awk 'END{print NR}' pyside/src/optdesk/ibkr/history.py` is below 600).
5. `git diff --name-only BASE` lists no path outside the Files line.
Checked by CI: the pyside workflow (`uv sync --locked`, `ruff check`, `mypy --strict`, `pytest -n auto`).

### 7 bars-job

Module: jobs, plus repo (the licensed pair: `.go-arch-lint.yml`). Size: L.
Files: `.go-arch-lint.yml`, `internal/jobs/api.go`, `internal/jobs/api_test.go`, `internal/jobs/barsrefresh/doc.go`, `internal/jobs/barsrefresh/job.toml`, `internal/jobs/barsrefresh/manifest.go`, `internal/jobs/barsrefresh/plan.go`, `internal/jobs/barsrefresh/run.go`, `internal/jobs/barsrefresh/restate.go`, `internal/jobs/barsrefresh/manifest_test.go`, `internal/jobs/barsrefresh/plan_test.go`, `internal/jobs/barsrefresh/run_test.go`, `internal/jobs/barsrefresh/restate_test.go`, `docs/contracts/jobs.md`
Design: docs/design/market-bars.md#Decisions (3 to 7, 11)
Order: after 3, 4, 5; the `.go-arch-lint.yml` chain is 3, 7, 8; it shares `internal/jobs/api.go`, `internal/jobs/api_test.go` and `docs/contracts/jobs.md` with 8 and 10 (before 8, 10).
Goal: `barsrefresh.New` (`NewBarsRefreshFunc`): one key per `WatchEntry` (key = its `Issuer`) and per `[[index]]` (key = `index:<symbol>`); `Run` for a key resolves the instrument id (Decision 6), pulls `trades` (5 days, or `backfill_days` when nothing is stored or after a restatement), applies the rule of Decision 4 and inserts; pulls `adjusted_last` when none is stored or the newest is 7 or more calendar days before the run day (`adjusted_cadence_days`), with the same rule; a source error for one key is that key's failure, the other keys are unaffected; a name with no bars is `Usable` 0, never an invented close. Manifest: `id`, `backfill_days = 400`, `daily_days = 5`, `restate_tolerance = "0.00001"`, `adjusted_cadence_days = 7`, `[[index]]` tables, `[schedule]` (XNYS, `session_close`, `10m`, `run_bound` `10m`). Money is `math/big.Rat`, never float.
Acceptance:
1. `grep -rhE "^func (TestPlanOneKeyPerEntryAndIndex|TestFirstPullBacksFill|TestDailyPullInsertsOnlyNewDays|TestRestatedDayRepullsUnderNextEpoch|TestWithinToleranceIsNotRestated|TestAdjustedPulledWhenSevenDaysOld|TestAdjustedSkippedWhenFresh|TestOneKeyFailureDoesNotStopOthers|TestIndexConIDResolvedOnce|TestNoBarsIsUnusableNotZero|TestPacedIsRetryable)\(" internal/jobs/barsrefresh | awk 'END{print NR}'` prints 11 (new); `TestRestatedDayRepullsUnderNextEpoch` has a stored close 100.0000 and a pulled 50.0000 for one day and requires every pulled row to carry epoch 2; `TestWithinToleranceIsNotRestated` has 100.0000 and 100.0005 and requires no epoch 2 row (the difference is 5e-6 relative, below the tolerance 1e-5).
2. `grep -c 'id = "bars_refresh"' internal/jobs/barsrefresh/job.toml` prints 1; `grep -c "backfill_days\|restate_tolerance\|adjusted_cadence_days" internal/jobs/barsrefresh/job.toml` prints 3; `grep -c "NewBarsRefreshFunc" internal/jobs/api.go` prints 1; `git diff BASE -- internal/jobs/api.go | grep -c '^-[^-]'` prints 0.
3. `grep -c "float32\|float64\|time.Sleep\|time.Now()" internal/jobs/barsrefresh/restate.go internal/jobs/barsrefresh/run.go` prints 0 on both; `grep -c "big.Rat" internal/jobs/barsrefresh/restate.go` is at least 1; `grep -c "t.Parallel()" internal/jobs/barsrefresh/run_test.go` is at least the count of `^func Test` there.
4. `grep -c "bars_refresh\|restate" docs/contracts/jobs.md` is at least 2 (0 on main); `grep -c "internal/jobs/barsrefresh" .go-arch-lint.yml` is at least 1; every new file is under 600 lines (`awk 'END{print NR}'` of each is below 600); `git diff --name-only BASE` lists no path outside the Files line.
Checked by CI: `go test ./internal/jobs/... -race -count=1`, `go run ./scripts/arch --check`, `go run ./scripts/arch && git diff --exit-code arch.toml .go-arch-lint.yml`, ci-go, golangci-lint.

### 8 fx-job

Module: jobs, plus repo (the licensed pair: `.go-arch-lint.yml`). Size: M.
Files: `.go-arch-lint.yml`, `internal/jobs/api.go`, `internal/jobs/api_test.go`, `internal/jobs/fxrefresh/doc.go`, `internal/jobs/fxrefresh/job.toml`, `internal/jobs/fxrefresh/run.go`, `internal/jobs/fxrefresh/run_test.go`, `docs/contracts/jobs.md`
Design: docs/design/market-bars.md#Decisions (9)
Order: after 3, 4, 5, 7 (`.go-arch-lint.yml` chain 3, 7, 8; `internal/jobs/api.go`, `api_test.go`, `jobs.md` shared with 7 and 10, before 10).
Goal: `fxrefresh.New` (`NewFxRefreshFunc`), one key `{obs.fx_rate, USDJPY}`; `Run` resolves the USD.JPY cash contract once through `Broker.ResolveContract` (exchange IDEALPRO, currency JPY; not listed is Permanent), asks `MIDPOINT` for 5 days, inserts one row per bar day not stored (`rate` = the close text, `source` `ibkr_midpoint`), leaves a stored day as it is, and fabricates nothing when the answer is empty; no default rate exists in the package.
Acceptance:
1. `grep -rhE "^func (TestOneKeyUSDJPY|TestInsertsOnlyNewDays|TestStoredDayLeftAsIs|TestEmptyAnswerWritesNothing|TestNotListedIsPermanent|TestPacedIsRetryable|TestNoDefaultRate)\(" internal/jobs/fxrefresh | awk 'END{print NR}'` prints 7 (new).
2. `grep -c 'id = "fx_refresh"' internal/jobs/fxrefresh/job.toml` prints 1; `grep -c "NewFxRefreshFunc" internal/jobs/api.go` prints 1; `grep -c "fx_refresh" docs/contracts/jobs.md` is at least 1.
3. `grep -c "float32\|float64\|time.Sleep\|time.Now()" internal/jobs/fxrefresh/run.go` prints 0; `grep -ci "fallback" internal/jobs/fxrefresh/run.go` prints 0 (`TestNoDefaultRate` reads the package source with `go/parser` and fails on a numeric literal assigned to a rate).
4. `grep -c "internal/jobs/fxrefresh" .go-arch-lint.yml` is at least 1; `git diff BASE -- internal/jobs/api.go | grep -c '^-[^-]'` prints 0; `git diff --name-only BASE` lists no path outside the Files line.
Checked by CI: `go test ./internal/jobs/... -race -count=1`, `go run ./scripts/arch --check`, `go run ./scripts/arch && git diff --exit-code arch.toml .go-arch-lint.yml`, ci-go.

### 9 hv30-rule

Module: compute, plus docs (judged two-module entry: the register is under docs; precedent `compute-rules` of `docs/design/compute-figures.md`). Size: S.
Files: `internal/compute/rules/rules.json`, `internal/compute/rules/rules_test.go`, `docs/conventions/register.toml`
Design: docs/design/market-bars.md#Decisions (10)
Order: first. It shares no file with another entry of this design.
Goal: a rule `market.hv30_own` v1 (`convention` `hv30_v1`, `unit` `decimal`, `scale` 6, `n` 30, `trading_days` 252, `max_age_days` 4, `compare` empty, the other numbers 0) with the `hash` the existing hash test computes for `HistoricalVol`, and the `[[figure]]` entry with all 15 keys: `pages = []` (the pages that show HV30 cite it in their own designs), `implemented_by = ["internal/compute/figures.(*calc).HistoricalVol"]`, the `currency` sentence "Unit decimal, stored as decimal text, rounded half-even to 6 places.", `excludes` naming IBKR's HV30, dividend adjustment and any fill, `agreed_on` and `agreed_in` carried by the issue text (the owner's approval of this design; a worker writes no agreement of its own). The old entry `companies.volatility.hv30` is left as it is.
Acceptance:
1. `grep -c '"id":"market.hv30_own"' internal/compute/rules/rules.json` prints 1; that line has `"n":30`, `"trading_days":252` and `"max_age_days":4` (`grep -c '"id":"market.hv30_own".*"n":30.*"trading_days":252.*"max_age_days":4' internal/compute/rules/rules.json` prints 1).
2. `grep -c 'id = "market.hv30_own"' docs/conventions/register.toml` prints 1 and `grep -c 'id = "companies.volatility.hv30"' docs/conventions/register.toml` prints 1 (unchanged); `git diff BASE -- docs/conventions/register.toml | grep -c '^-[^-]'` prints 0.
3. In `rules_test.go` the id appears in the existing tests' expected lists (`grep -c "market.hv30_own" internal/compute/rules/rules_test.go` is at least 1) and no test is deleted (`git diff BASE -- internal/compute/rules/rules_test.go | grep -c '^-func'` prints 0).
4. `git diff BASE -- docs/conventions/register.toml | grep -c '^+source_hash = "pending-recompute"'` prints 0 (the entry carries the real 64-hex hash of the other Go ids' method); `git diff --name-only BASE` lists exactly the Files line.
Checked by CI: `go test ./internal/compute/rules -race -count=1` (`TestHashesMatchSource`, `TestRulesMatchRegister`, `TestNoPendingForGoIds`, `TestRegisterEntriesComplete`), ci-go.

### 10 hv30-step

Module: jobs. Size: M.
Files: `internal/jobs/api.go`, `internal/jobs/api_test.go`, `internal/jobs/computefigures/job.go`, `internal/jobs/computefigures/job.toml`, `internal/jobs/computefigures/hv.go`, `internal/jobs/computefigures/hv_test.go`, `docs/contracts/jobs.md`
Design: docs/design/market-bars.md#Decisions (10)
Order: after 3, 7, 8, 9; it shares `internal/jobs/api.go`, `internal/jobs/api_test.go` and `docs/contracts/jobs.md` with 7 and 8, and `job.go` with no other entry of this design.
Goal: `ComputeStores.PriceBars` (nil: the step is skipped and `Plan` and `Run` are byte for byte as today); with it set, the manifest key `hv30 = "market.hv30_own"` plans one key `market.hv30_own|underlying|<name>` per subject of the day, `Run` reads the winner `trades` rows (Decision 10), builds the `compute.Series` at the edge and stores the figure of `Figures.HistoricalVol` as a reading with the existing writer (state ok or missing with the figure's reason; the rows hash and the parameters `data_day`, `observations`, `window`).
Acceptance:
1. `grep -rhE "^func (TestHv30StoredForSubject|TestHv30ShortHistoryIsMissing|TestHv30NoBarsIsNoInput|TestHv30StaleIsMissing|TestNilPriceBarsSkipsStep|TestHv30UsesTradesBasisOnly|TestHv30ClosesAscendingLast31)\(" internal/jobs/computefigures | awk 'END{print NR}'` prints 7 (new); `TestHv30StoredForSubject` feeds 31 closes alternating 100 and 101 and requires the stored value to equal the hand-worked `stdev(30 log returns) x sqrt(252)` at 6 places half-even, written in the test as a literal.
2. `grep -c "PriceBars" internal/jobs/api.go` is at least 1 and `git diff BASE -- internal/jobs/api.go | grep -c '^-[^-]'` prints 0; `grep -c 'hv30 = "market.hv30_own"' internal/jobs/computefigures/job.toml` prints 1.
3. `grep -c "HistoricalVol" internal/jobs/computefigures/hv.go` is at least 1; `grep -c "historical_vol\|HistoricalVol \*float64" internal/jobs/computefigures/hv.go` prints 0 (IBKR's HV30 is never read); `grep -c "adjusted_last" internal/jobs/computefigures/hv.go` prints 0.
4. `grep -c "hv30\|PriceBars" docs/contracts/jobs.md` is at least 2; each new file is under 600 lines; `git diff --name-only BASE` lists no path outside the Files line.
Checked by CI: `go test ./internal/jobs/... -race -count=1`, `go run ./scripts/arch --check`, ci-go.

### 11 bars-wire

Module: ot. Size: M.
Files: `cmd/ot/wire.go`, `cmd/ot/wire_test.go`, `cmd/ot/runonce.go`, `cmd/ot/runonce_test.go`, `cmd/ot/env.go`, `cmd/ot/preflight.go`, `cmd/ot/preflight_test.go`, `docs/contracts/ot.md`
Design: docs/design/market-bars.md#Decisions (6, 7, 8, 10)
Order: after 4, 7, 8, 10 and after `docs/design/event-sources.md` `watch-loader-earnings-wire` (the `watch.toml` loader, `runonce.go`, `wire.go`, `env.go`, `preflight.go` are shared with it and with the account wire; this entry rebases on whichever lands first).
Goal: register `bars_refresh` and `fx_refresh` in `run-once` and the scheduler with the `WatchEntry` list of the loader, open `bar.Open` with `OT_PG_DSN_BAR`, dial `grpcbroker.DialHistory`, give `computefigures` the bar reader opened with the pages DSN as `ComputeStores.PriceBars`; a missing DSN or an unreachable broker disables only these jobs (and the HV step) with a named line, values never printed; `OT_PG_DSN_BAR` joins the preflight list.
Acceptance:
1. `grep -rhE "^func (TestBarsRefreshRegistered|TestFxRefreshRegistered|TestMissingBarDSNDisablesOnlyBars|TestComputeFiguresGetsPriceBars|TestBrokerDownDisablesBarJobs)\(" cmd/ot | awk 'END{print NR}'` prints 5 (new).
2. `grep -c "OT_PG_DSN_BAR\|bars_refresh\|fx_refresh" docs/contracts/ot.md` is at least 3; `grep -c "OT_PG_DSN_BAR=[A-Za-z0-9]" cmd/ot/*_test.go | grep -vc ":0"` prints 0.
3. `grep -c "barsrefresh\|fxrefresh\|store/bar" cmd/ot/wire.go` is at least 3 and each of the three names is in the list of `docs/design/modules.md` that this pull request already edited (`grep -c "internal/jobs/barsrefresh\|internal/jobs/fxrefresh\|internal/store/bar" docs/design/modules.md` prints 3).
4. `git diff --name-only BASE` lists no path outside the Files line; each new file is under 600 lines and `awk 'END{print NR}' cmd/ot/runonce.go` is below 800.
Checked by CI: `go test ./cmd/ot -race -count=1`, `go run ./scripts/arch --check`, ci-go. The first real pull (that the gateway answers `ADJUSTED_LAST` and the index and FX contracts, the depth it gives, whether 55 requests per 10 minutes holds) is not checkable offline; it is read from the first `ot run-once bars_refresh` by the owner.

## Left out

- Stooq source and coarse layer, the 15 s FX stream, the scanner and composite scoring: dropped by the owner 2026-11-09, `alignment-v25.md` (Decision 3 and its Left out). Only daily FX is built. No second source of closes; a name IBKR gives no history for has an HV30 gap `short_history` or `no_input`.
- Permanent intraday quote storage (listed with "bars" in `alignment-v25.md` Decision 3): not designed here; `obs.quote_snapshot` is insert-only on main and no retention job was found by grep, but what the owner means by "permanent" is not stated. Asked: it is its own small design if the owner wants a rule.
- Indices TOPX and XAUUSD (glossary "Index closes (RUT, TOPX)", "Spot gold"): display-only rows with no designed consumer; question B.
- Held names that are not on `watch.toml`: no bars; the owner adds them to the file.
- The 20-year history and Timescale (plan 11.1 probes) and the Historical rpc for the live index IV (`alignment-v25.md`): this rpc has `BarKind` words for bars only; IV bars are a later additive enum value after the CBOE probe.
- RV20, IV minus RV20, beta, the 200-day line on IBKR closes, IV rank and the ITM probability: they read these tables but belong to the compute and page designs that name them (`market-page-producer.md`, `option-candidates.md`).
- Reconciliation of own HV30 against IBKR's (`Quote.HistoricalVol`, glossary: "recorded, never shown"): not recorded here; a mismatch record is a later step of the figures job.
- FX: the 15 s stream (owner, dropped), decision-grade observations, ECB cross-check, `fx_override` and the internal 16:27 fix of ADR 0004; pairs other than USDJPY; the consumers (funding gates, JPY structures, P and L in USD) which keep their gaps until their designs read `LatestFxRate`.
- Pages, schemas and builders that print HV30 (`vol_line`, `iv_line`, `hv30` fields, `volatility_note`): not edited; the builders-wave cites `market.hv30_own` by id and version.
- Scoring and the scanner: another design.
