# Go quant / time-series ecosystem for optdesk (research, 2026-11-08)

Stats via GitHub API on 2026-11-08 (stars, SPDX license, last push, latest tag). Dependency counts NOT measured (no go.mod audit done).
Web-search snippets were not opened one by one: treat descriptions of small libs as unverified.

## 1. Libraries per area

| Area | Library | Stars | License | Last push / release | Verdict |
|---|---|---|---|---|---|
| Stats / linalg | gonum/gonum | 8455 | BSD-3 | push 2026-10-25, v0.17.0 (2026-01-10) | Mature, de facto standard. Keep. |
| Plot | gonum/plot | 2970 | BSD-3 | 2026-04, v0.17.0 | fine if charts needed server-side |
| Indicators | markcheno/go-talib | 943 | MIT | push 2026-06-17, NO tags (pseudo-version) | Pure-Go port of TA-Lib funcs, float64 slices. Usable, unversioned; needs golden tests. |
| Indicators | cinar/indicator v2 | 1916 | AGPL-3.0 | 2026-09-14, v2.1.44 | Active, broad, but AGPL: avoid for a closed/private product unless AGPL is accepted. |
| Indicators / backtest | sdcoffey/techan | 904 | MIT | v0.14.0 (2026-09-04) | Active; indicator + strategy model, uses big.Float. |
| Dataframe | go-gota/gota | 3259 | unclear (NOASSERTION) | ARCHIVED, last push 2023, v0.12.0 (2021) | Dead. Do not use. |
| Dataframe | rocketlaunchr/dataframe-go | 1295 | unclear | last push 2022, no releases | Stale. |
| Dataframe | tobgu/qframe | 414 | MIT | push 2026-03, v0.4.0 | Small but alive; immutable columnar, filtering/groupby. |
| Columnar / compute | apache/arrow-go | 416 | Apache-2.0 | push 2026-11-06, v18.8.0 (2026-09-08) | Healthy (Apache project). Arrays, IPC, parquet, compute kernels (sum, cast, filter, arithmetic); NOT rolling windows/ewm. |
| Dataframe via FFI | pola-rs/polars (40k stars, MIT) | - | MIT | Python/Rust only | No official Go binding; none mature. Do not plan on it. |
| SQL compute | duckdb/duckdb-go (ex marcboeker/go-duckdb, old repo archived) | 313 | MIT | v2.20000.0 (2026-10-22) | Official-org driver, cgo, bundles libs. DuckDB SQL has window functions (rolling avg/stddev, ASOF join, quantiles) - the practical Go answer to "rolling windows/dataframes". Arrow interop exists. |
| SQL embedded | mattn/go-sqlite3 | 9271 | MIT | v1.14.53 (2026-11-05) | cgo; SQLite window functions work but no stats fns. Pure-Go alt: modernc.org/sqlite. |
| Decimal | shopspring/decimal | 7512 | MIT | v1.6.0 (2026-10-18) | Keep for money; not for numerics. |
| Option pricing / IV / Greeks | uscott/go-blackscholes (0 stars, MIT, 2025-02, v0.2.0); asargin-dev/gouant (1 star, MIT, 2024-11); onwsk8r/goption-pricing (4 stars, 2018, incomplete) | ~0 | MIT | stale / hobby | NO mature Go options library exists. Black-Scholes + Newton/Brent IV is ~100 lines on gonum (distuv.Normal CDF, gonum/optimize or own bisection). Write own, behind interface, test against QuantLib/py_vollib golden values. |
| Pricing, heavy | QuantLib (C++), QuantLib-SWIG | - | BSD | SWIG targets: Python, C#, Java, R; Go SWIG is not an official QuantLib target (not verified as maintained) | Only via cgo wrapper you maintain; not worth it for vanilla options. |
| Backtesting | gobacktest/gobacktest | 241 | MIT | ARCHIVED, 2023, v0.3.0 | Dead. |
| Backtesting | ZulferDev/backtest-go | 0 | MIT | 2026-08 | Too new/unproven. |
| Backtesting | techan (above), Backnrun | - | - | - | No mature Go backtester. People write event loops themselves; it is simple in Go and fits optdesk-specific logic (options legs, assignment). |
| Calendars | scmhub/calendar | 12 | MIT | push 2025-03, v0.1.1 | Only multi-exchange (MIC codes, early closes) Go lib found; tiny, one maintainer, stale-ish. |
| Calendars | ggoodwin/go-nyse-time (1 star, 2023), Knowckx/tradeday (1 star, bitmap 2015-2026 CN/US) | ~0 | MIT | | hobby grade |
| IBKR API in Go | scmhub/ibapi (81 stars, MIT, v10.34.2, push 2026-09), scmhub/ibsync (28 stars, MIT, v0.10.50), hadrianl/ibapi (162, stale 2023) | | MIT | | Go port of TWS API exists and is current; ibsync is the Go analogue of ib-async. Not production-proven at scale as far as I could see; one maintainer (scmhub, same as calendar). |

## 2. Patterns when no Go library fits

1. Pure-Go port of the reference algorithm + golden tests generated once from Python (TA-Lib, pandas, py_vollib, exchange_calendars) and committed as fixtures. Most common for indicators and Black-Scholes; zero runtime dependency; the generator is dev-only. This is what go-talib itself is (port of TA-Lib).
2. cgo to C libs (TA-Lib C, DuckDB). Works, fast; costs: cross-compile pain, CGO build in CI, no static pure-Go binary, memory ownership. DuckDB via cgo is production-used widely (embedded analytics); TA-Lib cgo wrappers are small/unmaintained.
3. Sidecar service over gRPC/HTTP (Python or other). Most common production fallback for pandas/QuantLib/ML. Costs: ops, process supervision, serialization. optdesk already does this for IBKR (pyside).
4. WASM (wazero, 6454 stars, Apache-2.0, v1.12.0): run a compiled C/Rust lib in-process without cgo. Good for a pure compute kernel (e.g. Rust `blackscholes` crate, TA-Lib compiled to wasm); needs a build pipeline for the wasm blob; I found no widely used Go quant project doing this, so treat as viable but unproven.
5. Embedded Python (DataDog/go-python3 archived 2021; gpython v0.2.0 2023, no numpy/C ext): not viable. Avoid.
6. SQL engine for time series: DuckDB (cgo) or PostgreSQL/TimescaleDB window functions; rolling stats written as SQL views. Production-common; keeps compute out of Go entirely.
7. Arrow as the interchange format at boundaries (arrow-go <-> pyarrow/polars) so the implementation behind it is swappable.

Evidence caveat: usage-in-production claims are from general knowledge of the ecosystem and repo metadata, not from surveying companies.

## 3. Recommendation for optdesk

Current Python deps (pyside/pyproject.toml): exchange_calendars, ib-async, grpcio, protobuf. Go side: gonum, go-talib, decimal, telegram bot.

Replace in Go now:
- Option math (BS price, IV solver, Greeks) when needed: own package on gonum, golden-tested against py_vollib/QuantLib values (dev-time fixtures). Not Python at runtime. No library to adopt.
- Indicators/rolling stats: keep gonum (stat, floats) + own rolling window types; go-talib only through the interface and with golden tests, because it has no tags. Avoid cinar (AGPL).
- Backtest loop, if/when: write own event loop; no Go framework is mature.
- Calendar: generate a static holiday/early-close table (XNYS, plus CBOE if needed) from exchange_calendars at dev time into a committed data file (contracts/datasets fits), and read it in Go. Removes the runtime Python dependency without trusting scmhub/calendar (12 stars). Yearly regeneration job. Fallback: scmhub/calendar behind same interface.
- Storage/rolling compute at scale: DuckDB (duckdb/duckdb-go) if SQL windows are wanted; accepts cgo. PostgreSQL is already in the project (pg tag), so window-function SQL there avoids new deps.

Cannot replace now (stay in Python behind gRPC):
- IBKR gateway: ib-async is mature and maintained by the community successor of ib_insync. Go options (scmhub/ibapi, ibsync) are current but young/single-maintainer; swapping is feasible later because the Go client already talks to a gRPC AccountService. Make the gateway proto the interface; a Go implementation can replace pyside server without touching callers. A pilot of scmhub/ibsync behind the same proto is the way to retire Python entirely; evaluate with paper account only.
- Anything needing pandas/scipy/QuantLib exotics: none required today.

Interfaces (owner rule: own glue code behind an interface too):
- `Indicators` (SMA/EMA/RSI/ATR... over []float64) - impls: talib adapter, own.
- `Rolling`/`Stats` (window stats) - impls: gonum, DuckDB SQL.
- `OptionMath` (Price, IV, Greeks) - impls: own BS now, QuantLib/other later; contract test with golden fixtures from both sides.
- `TradingCalendar` (IsSession, NextSession, CloseTime, EarlyClose) - impls: static-table, scmhub/calendar, exchange_calendars sidecar.
- `BrokerGateway` (the existing proto) - impls: pyside/ib-async, Go ibsync.
- `Clock` already via test kit.
Each with a golden-test suite in contracts so a new library must pass the same cases. Dev-only Python generator scripts for fixtures live under tools/, not in runtime deps.

## Uncertainties
- Dependency counts not measured; go.mod of each lib not inspected.
- Small repos (scmhub, uscott, gouant) were judged by metadata and search snippets only; code quality/correctness unverified (check holiday data for 2026-2030, e.g. Juneteenth, Good Friday, early closes).
- No evidence found of major firms publicly running Go-only quant stacks; the "port + golden tests" and "sidecar" patterns are inferred as the norm.
- Whether QuantLib has a maintained Go SWIG target: not found, assumed no.
- go-talib license MIT confirmed, but lacks releases; function-level numeric parity with TA-Lib (e.g. EMA seeding) must be tested.
