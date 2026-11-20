# bars fallback: when the history farm of the gateway is broken, bars_refresh fills the daily trades bars from yfinance and every page that shows bars says where they came from

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-20. Checked head: `origin/main` `cd3ccf96`. The next free migration at that head is `0064` (the last is `0063_job_run.sql`); the number is the next free one at the cut of the issue.

Owner words: 2026-11-20 (a broken farm shows on the page and waits; the bars fallback to yfinance is its own design, `docs/design/gateway-health.md` `## Left out`). Plan: none, incident repair. Advisor constraints of 2026-11-20 are carried: the upsert has a WHERE on the source, so a yfinance row never overwrites an IB row and an IB row replaces a yfinance row, proved by a `pg` Go test; the next free migration number; the source of each bar is shown on the pages that show bars; the adjustment basis is stated.

Standing owner rules this design is written against, each self-checked in `## Self-check`: (1) chained jobs collect live when stored input is bad: fallback, not cascade; (2) every job is runnable on its own and idempotent; (3) the same job runs one at a time; (4) dev mode: no time guards; (5) the front end renders data and computes nothing; (6) prefer a mainstream library; (7) own glue code sits behind an interface so a library can replace it; (8) no ticker and no stock code in the text.

## Goal

When the history farm of the `main` gateway is broken (the incident of 2026-11-19, 20 hours of bars calls that ended Retryable), the daily run of `bars_refresh` still stores the daily trades bars of every name of its list, taken from yfinance. The row says `source = yfinance`. When the farm is back, the next IB pull replaces those rows and the series is IB again. The owner sees on market state, company data and positions, in Chinese, whether the bars under the page are IB, Yahoo or mixed. Nothing is written by Yahoo over an IB row.

## Today

- `internal/jobs/barsrefresh/run.go`: `runKey` resolves the contract, reads the stored trades bars, pulls `DailyDays` (5) or `BackfillDays` (400) through `broker.HistoryClient.Bars`, upserts by key with `writtenBy "bars_refresh:<day>"`, then deepens and runs the weekly `adjusted_last` step. In the incident the `Bars` call of a key waited its `HistoryTimeout` (330 s) and ended Retryable (deadline exceeded, or INTERNAL); the key failed, the run went on and each next key waited the whole time again. The runner retries a Retryable day until the close of its calendar.
- The signal `history_unavailable` of `docs/design/gateway-health.md` (Decision 6) is not on main yet and `grpcbroker` drops the message of an `UNAVAILABLE` answer; this design does not depend on it (Left out).
- yfinance is already in the repo and is not a new dependency: `pyside/pyproject.toml` pins `yfinance==1.8.0` (Apache-2.0, mainstream, the module map names it for the pyside row); `optdesk.refbars` is the one-shot read-only command and `internal/sources/yfinance` is the Go `sources.BarSource` over it (used by `recon_bars` through `cmd/ot/wire_barsource.go`). It returns the close only (`sources.RefBar{Pair, Day, Close}`, python `Bar{session_day, close}`) and asks `auto_adjust=False`, `actions=False`.
- `obs.price_bar` (migration `0041`) is keyed `(instrument_id, basis, session_day)`; `barpg.upsertPriceBarSQL` replaces a row whose prices, volume, currency, source or status differs. The check `price_bar_source_ck` (migration `0021`) still allows only `source = 'ibkr'`, and so do the dataset `obs.price_bar.r1` enum and the generated `PriceBarSource`. The dataset r1 still requires `adjust_epoch` and lists it in `x-primary-key`, which migration `0041` removed from the key.
- Pages that read trades bars: `market_state_page` (has a `bars_caption`), `company_data_page`, `positions_page` (v3); the report global-assets block reads them too (Left out).

## Decisions

1. Trigger, per key. The `Bars` call of a key that ends with a Retryable error (what main emits today: the call's deadline or INTERNAL, after the 330 s wait) sends that key to the fallback. Only Retryable classes fall back: an unknown key, a bad bar day or a Permanent error never does. One dead symbol does not flip the others; but when a second key fails the same way in the run, `histDown` is set and the rest of the run skips the IB pull (each would wait the full time) and goes to the fallback. The fallback is inside `bars_refresh`; no other job reads yfinance for it (fallback, not cascade: downstream jobs read the stored bars as before).
2. Both sources failing. A key whose fallback also fails (Yahoo rate limit, network, timeout, an empty answer, source changed) is a named Retryable gap with both codes in its message; the runner retries it as today. Pairs Yahoo cannot route (the main indexes and any currency other than USD and JPY in `refbars/symbols.py`) are not sent: no fallback, the key fails Retryable as today, and the market pages show the gap.
3. What is written. Only basis `trades`, only the daily step (the window of `DailyDays`, or `BackfillDays` for a first pull, a past-day repair or a restated window, as today). Volume is written NULL: IB volume units (US stocks are often in lots of 100) and Yahoo shares are not shown equal, and no figure may read a fallback volume. Not written by the fallback: the weekly `adjusted_last` step and the deepening of older history; both wait for the IB farm (they tolerate a gap and nothing reads them within days). A pair Yahoo answers with no data stays unfilled for that day (code `no_data`, Retryable); no new symbol mapping is added here.
4. Full bars. A fallback row needs open, high, low and close. `refbars` and `sources.RefBar` are extended additively with those fields and `volume` (empty text when Yahoo gives none; the job drops the volume, Decision 3). A bar missing any of the four prices is not written and is counted as incomplete; the key's result is Retryable. Prices are decimal text of 4 places as `refbars` already makes them.
5. Adjustment basis. Sources read on 2026-11-20: the IBKR page https://interactivebrokers.github.io/tws-api/historical_bars.html says "TRADES data is adjusted for splits, but not dividends" and "ADJUSTED_LAST data is adjusted for splits and dividends"; yfinance `Ticker.history(auto_adjust=...)` (https://github.com/ranaroussi/yfinance, `yfinance/scrapers/history.py`) adjusts the price columns when `auto_adjust` is true, and refbars passes `auto_adjust=False`, so `Close` is the price before that adjustment (Yahoo applies a split with some delay after the day, which is why the comparison below exists). The two `trades` series are taken as the same basis and the fallback writes basis `trades` only; `adjusted_last` is never taken from `Adj Close`. Consistency is kept in three steps. (a) Before a name's rows are written the fallback compares the days of its window that have a stored IB row with the Yahoo day; when any close differs by more than `fallback_tolerance` nothing is written for that name, its result is a Retryable error `fallback_basis_differs`, and the day stays a gap. The tolerance is a trial value, `0.005` (0.5 percent), a new key of `job.toml` the owner can change; it is looser than `close_rel` 0.001 of `recon_bars` because it only detects a broken basis (a split not yet applied is a factor of 2 or more), not the quality of a close, and Yahoo's close and IB's last trade differ by cents. The largest observed difference of the run is printed in the run note. With no overlapping IB day nothing can be compared: the rows are written, marked `yfinance`, and the note says "no overlap". (b) The recovery pull of Decision 7 is IB's basis for the whole window, so a split between a fallback day and the recovery is covered by it. (c) Volume is not written (Decision 3).
6. Precedence in the store. The upsert statement gains `AND (EXCLUDED.source = 'ibkr' OR obs.price_bar.source <> 'ibkr')` to its `WHERE`: a yfinance row never replaces a stored ibkr row (it counts as unchanged); an ibkr row replaces a yfinance row because the source differs (existing `IS DISTINCT FROM` already covers `source`), even when the prices are equal, so the source label heals. `TestUpsertSourcePrecedence` (tag `pg`, CI only) proves the four cases ibkr over ibkr, yfinance over ibkr (kept), ibkr over yfinance (replaced), yfinance over yfinance. Migration `0064_price_bar_source.sql` replaces `price_bar_source_ck` by `source IN ('ibkr', 'yfinance')`; its down drops it and restores the old one only when no yfinance row exists. The dataset becomes `obs.price_bar.r2` (`docs/contracts/00-protocol.md` states the additive rule for the proto package only, not for a dataset enum, so a new value is a new revision): `source` enum `ibkr`, `yfinance`, and `required` and `x-primary-key` follow migration `0041` (`adjust_epoch` out of both).
7. Healing. A normal IB run whose stored trades rows include any with source `yfinance` pulls `BackfillDays` instead of `DailyDays` (the same widening `restated` already does), so the yfinance days older than the 5-day window are replaced too, and after a recovered run no yfinance row remains for a day IB serves in the pulled window. A name dropped from the list or not pulled because of the first-pull cap keeps its yfinance rows, still marked. No other state is kept: the source column is the memory, so the job is idempotent and a re-run of any day gives the same rows.
8. Rows carry `writtenBy "bars_refresh:<day>"` as today; the log line of a run names how many keys went to the fallback, how many rows it wrote and the codes of the unanswered pairs (`rate_limited`, `no_data`, `network`, `timeout`, `source_changed`, `fallback_basis_differs`, `incomplete`). The time of the fallback is bounded by the context of the run, as `sources.BarSource` already promises (it stops 10 s before the deadline and reports the rest `timeout`). There is no time guard and no time anchor: the fallback runs whenever the call fails, on any day, forced or scheduled. The bar is filtered by data instead: the job drops any yfinance bar whose session day is later than the last closed session of the pair's calendar (`clock.Calendar.Session`, `CloseAt` and `EarlyClose`, so half days count; the US and the JP calendars the repo already has), so a console run in mid-session writes no partial day.
9. Wiring. The job gets its source by decoration, as `withFxSecondary` does for `fx_refresh`: `barsrefresh.WithFallback(job, src)` returns the job with the source set; `New` keeps its signature. `cmd/ot` builds the source with the existing registry (`barSource("yfinance")`, same command as `recon_bars`) in a new file; no source built (the command missing) leaves the job as today and the log says so.
10. Source on the pages. Readers of stored bars other than these pages (`computefigures` hv, tech and vol, `buyprice`, `optionopen`, `optionledger`, `valuation`) are not given the source (Left out); the note therefore also says that figures of the days it names may rest on a Yahoo bar. `internal/pages/shared` gets `BarSourceOf(rows []contracts.PriceBar) (kind, note string)`: kind is `ibkr`, `yfinance` or `mixed`; note is empty for `ibkr` and a Chinese sentence otherwise (for example "日线含 Yahoo 数据，当日指标可能基于 Yahoo 价格；IB 历史数据恢复后自动改回 IB 数据" for `mixed`). `market_state_page`, `company_data_page` and `positions_page` v3 gain the optional fields `bars_source` and `bars_source_note`; the web shows the note beside the table when it is not empty and computes nothing.
11. Library and own code. No new dependency: yfinance 1.8.0 (Apache-2.0) is the library; our glue is `refbars` and `internal/sources/yfinance` behind `sources.BarSource`, so another library or source replaces it by one registry line. Own code of this design is the decision rule and the precedence SQL, because no library does either.

## Modules

No row of `docs/design/modules.md` changes and no new edge: `jobs` already depends on `sources`, `clock` and `broker`, `pages` on `contracts` and `store`, `ot` on all of them. A path belongs to the first row whose glob matches.

| paths | row |
|---|---|
| `contracts/datasets/obs.price_bar.r2.json` (r1 file kept), `contracts/schemas/page.market_state_page.v2.json`, `contracts/schemas/page.company_data_page.v2.json`, `contracts/schemas/page.positions_page.v4.json`, `contracts/fixtures/**`, `internal/contracts/gen/records.go`, `web/src/generated/**`, `docs/contracts/datasets/obs.price_bar.md`, `docs/contracts/page-schemas.md` | contracts |
| `internal/sources/api.go`, `internal/sources/api_test.go`, `internal/sources/yfinance/**` | sources |
| `internal/store/bar/**`, `internal/store/barpg/**`, `migrations/0064_price_bar_source.sql` | store |
| `internal/jobs/barsrefresh/**`, `docs/contracts/jobs/bars_refresh.md` | jobs |
| `internal/pages/**` | pages |
| `cmd/ot/**` | ot |
| `web/src/pages/**`, `web/src/components/**`, `web/test/pages/**` | web |
| `pyside/src/optdesk/refbars/**`, `pyside/tests/refbars/**`, `pyside/tests/contracts/test_refbars_api.py` | pyside |

## Interface

1. contracts: the dataset becomes `obs.price_bar.r2` (Decision 6); the three page schemas get a new version each (`00-protocol.md`: a page schema changes by a new version after its first consumer) with the optional `bars_source` (enum `ibkr`, `yfinance`, `mixed`) and `bars_source_note` (string), both not in `required`; the generated Go, TypeScript and fixtures follow.
2. sources (`internal/sources/api.go`, additive): `RefBar` gains `Open, High, Low, Volume string` (decimal text; empty when the source gave none); `Close` is unchanged; the doc of `BarSource` says an implementation fills them when it can.
3. store (`internal/store/bar/api.go`, doc only): the doc of `PriceBarUpserter` adds "a row whose source is not ibkr never replaces a stored row whose source is ibkr; it counts as unchanged".
4. pyside (`pyside/src/optdesk/refbars/api.py`, additive): `Bar` gains `open`, `high`, `low`, `volume` (decimal text of 4 places; empty string when none); rule 4 of the module docstring covers them.
5. Job (not an interface file): `barsrefresh.WithFallback(job contracts.Job, src sources.BarSource) (contracts.Job, error)` and the manifest key `fallback_tolerance`.

## Issues

Build order: every entry compiles and its tests pass against main; BASE is `git merge-base origin/main HEAD`. Entry 7 carries a migration and holds the auto-deploy; the window runs the migration before entries 8 and 11 merge.

### 1 bars-fallback-contracts (interface)

Module: contracts. Size: M. Order: first.
Contracts: `obs.price_bar r1 -> r2` (source enum, key per migration 0041), `page.market_state_page v1 -> v2`, `page.company_data_page v1 -> v2`, `page.positions_page v3 -> v4`.
Files: `contracts/datasets/obs.price_bar.r2.json`, `contracts/schemas/page.market_state_page.v2.json`, `contracts/schemas/page.company_data_page.v2.json`, `contracts/schemas/page.positions_page.v4.json`, `contracts/fixtures/page.market_state_page.v2.ok.json`, `contracts/fixtures/page.company_data_page.v2.ok.json`, `internal/contracts/gen/records.go`, `web/src/generated/market_state_page.v2.ts`, `web/src/generated/company_data_page.v2.ts`, `web/src/generated/positions_page.v4.ts`, `docs/contracts/datasets/obs.price_bar.md`, `docs/contracts/page-schemas.md`
Design: docs/design/bars-fallback.md#Interface (1)
Acceptance:
1. `obs.price_bar.r2.json` has the `source` enum `["ibkr", "yfinance"]`; its `required` and `x-primary-key` match migration `0041` (`instrument_id`, `basis`, `session_day`; `adjust_epoch` in neither); `PriceBarSourceYfinance` is produced by the generator (no hand edit).
2. Each of the three schemas declares `bars_source` and `bars_source_note` and leaves them out of `required`; the generated TypeScript of each has both as optional.
3. `TestPriceBarSourceEnum` (new) accepts `yfinance` and rejects `tiingo`; the fixtures `ok` of the three new versions validate.
4. `docs/contracts/datasets/obs.price_bar.md` names both sources and the rule of Decision 6 in one line.

### 2 bars-fallback-sources-api (interface)

Module: sources. Size: S. Order: first.
Contracts: unchanged
Files: `internal/sources/api.go`, `internal/sources/api_test.go`
Design: docs/design/bars-fallback.md#Interface (2)
Acceptance:
1. `RefBar` has the fields `Open, High, Low, Volume string` after `Close`; no existing field changes.
2. `TestRefBarKeyedLiteral` (new) builds a `RefBar` by keyed fields without the new ones and compares equal to one with them empty (no fake of `reconbars` or `yfinance` breaks).

### 3 bars-fallback-store-api (interface)

Module: store. Size: S. Order: first.
Contracts: unchanged
Files: `internal/store/bar/api.go`, `internal/store/bar/api_test.go`
Design: docs/design/bars-fallback.md#Interface (3)
Acceptance:
1. Only the doc comment of `PriceBarUpserter` changes; `git diff BASE -- internal/store/bar/api.go` shows comment lines only.
2. `TestPriceBarUpserterSourceDoc` (new) calls the interface on a fake that applies the rule and shows an ibkr row kept against a yfinance row and a yfinance row replaced by an ibkr one.

### 4 bars-fallback-refbars-api (interface)

Module: pyside. Size: S. Order: first.
Contracts: unchanged
Files: `pyside/src/optdesk/refbars/api.py`, `pyside/tests/contracts/test_refbars_api.py`
Design: docs/design/bars-fallback.md#Interface (4)
Acceptance:
1. `api.py` declares `Bar` with `session_day`, `open`, `high`, `low`, `close`, `volume`, all `str`, declarations only; rule 4 of the docstring covers the new fields.
2. `test_refbars_api.py` gets `test_bar_has_ohlcv_keys` (new), which reads the `TypedDict` annotations.

### 5 bars-fallback-refbars

Module: pyside. Size: S. Order: after 4.
Contracts: unchanged
Files: `pyside/src/optdesk/refbars/fetch.py`, `pyside/tests/refbars/test_fetch.py`
Design: docs/design/bars-fallback.md#Decisions (3, 4)
Acceptance:
1. `fetch.py` fills `open`, `high`, `low`, `volume` next to `close` from the same frame (`auto_adjust=False`, `actions=False` unchanged), 4 places, `ROUND_HALF_UP` through the existing `close_text`; a NaN open, high or low gives an empty string for that field and the bar is kept; a NaN close skips the bar as today.
2. `test_fetch.py` gets `test_bar_carries_ohlcv` and `test_nan_open_is_empty_text` (new) over a fake `yf` module; no network, no sleep.

### 6 bars-fallback-yfinance

Module: sources. Size: S. Order: after 2 and 5.
Contracts: unchanged
Files: `internal/sources/yfinance/source.go`, `internal/sources/yfinance/source_test.go`
Design: docs/design/bars-fallback.md#Decisions (4)
Acceptance:
1. `wireBar` reads `open`, `high`, `low`, `volume` and `source.go` copies them to `RefBar`; an answer without them (an older command) gives empty fields and no error.
2. `TestDailyBarsOhlcv` and `TestOldCommandAnswerHasEmptyOhlc` (new) run over the fake command of `command_test.go`.
3. `internal/sources/yfinance/source.go` stays under 600 lines.

### 7 bars-fallback-store

Module: store. Size: M. Order: after 1 and 3. Holds the auto-deploy: it carries a migration.
Contracts: unchanged (the dataset change is entry 1)
Files: `migrations/0064_price_bar_source.sql`, `internal/store/barpg/store.go`, `internal/store/barpg/store_test.go`, `internal/store/barpg/upsert_pg_test.go`
Design: docs/design/bars-fallback.md#Decisions (6)
Acceptance:
1. The migration (the next free number at the cut; `0064` at the checked head) drops `price_bar_source_ck` and adds it again as `source IN ('ibkr', 'yfinance')`; its down section restores the old check and fails when a yfinance row exists. No grant changes.
2. `upsertPriceBarSQL` gains the condition of Decision 6 in its `WHERE`; the argument list and the `RETURNING` are unchanged; `store_test.go` pins the statement text.
3. `TestUpsertSourcePrecedence` (new, build tag `pg`, in `upsert_pg_test.go`): the four cases of Decision 6 over a temporary database, with the written and unchanged counts of each.
4. `TestUpsertIbkrReplacesYfinanceEqualPrices` (new, `pg`): an ibkr row with the same prices replaces the yfinance row and the source reads `ibkr`.

### 8 bars-fallback-job

Module: jobs. Size: L. Order: after 2, 3, 6 and 7.
Contracts: unchanged
Files: `internal/jobs/barsrefresh/fallback.go`, `internal/jobs/barsrefresh/fallback_test.go`, `internal/jobs/barsrefresh/run.go`, `internal/jobs/barsrefresh/job.go`, `internal/jobs/barsrefresh/manifest.go`, `internal/jobs/barsrefresh/job.toml`, `internal/jobs/barsrefresh/manifest_test.go`, `docs/contracts/jobs/bars_refresh.md`
Design: docs/design/bars-fallback.md#Decisions (1, 2, 3, 4, 5, 7, 8, 9)
Acceptance:
1. `fallback.go` (new, under 600 lines) holds `WithFallback`, the pair mapping from the key's contract (symbol and currency), the basis comparison of Decision 5, the session filter of Decision 8 and the mapping of `RefBar` to `contracts.PriceBar` (basis `trades`, source `yfinance`, volume NULL); `run.go` gets only the hook calls and stays under 300 lines.
2. `TestFallbackAfterRetryableBars` (new, fake clock, fake `HistoryClient` returning a Retryable error, fake `BarSource`): the failing key goes to the fallback; after a second key fails the same way the rest make no `Bars` call; one `DailyBars` call carries the pairs; the rows written have source `yfinance` and basis `trades`.
3. `TestOneDeadSymbolDoesNotFlipOthers` (new): one key's Retryable failure with the others served by `Bars` sends only that key to the fallback.
4. `TestNoFallbackForPermanent` (new): an unknown key, a bad bar day and a Permanent error make no `DailyBars` call.
5. `TestBothSourcesFail` (new): Yahoo rate limit, network, timeout and an empty answer each give a Retryable error naming both codes; an unmapped pair (an index, a currency other than USD and JPY) is not sent and fails Retryable.
6. `TestFallbackNoWeeklyNoDeepen` (new): in fallback no `adjusted_last` row is written and no deepening pull is made.
7. `TestFallbackBasisDiffers` (new): a stored IB close and a Yahoo close that differ by more than `fallback_tolerance` write nothing for the name and give Retryable `fallback_basis_differs`; a difference within it writes the rows and the run note carries the largest difference; with no overlapping IB day the rows are written and the note says "no overlap".
8. `TestFallbackIncompleteBar` (new): a bar with an empty open, high, low or close is not written and is counted `incomplete`; a written row has NULL volume.
9. `TestFallbackDropsOpenSession` (new, fake `clock.Calendar`): a console run in mid-session of a US day and of a JP day, and on a half day, writes no bar of the session in progress.
10. `TestRecoveryLeavesNoYfinanceRow` (new): with stored yfinance rows older than `DailyDays` and a working farm, the pull asks `BackfillDays` and afterwards no yfinance row remains for the days IB serves in the window; with none stored it asks `DailyDays` as today.
11. `TestDroppedNameKeepsYfinanceRows` (new): a name not in the list, and a name past the first-pull cap, keep their yfinance rows with source `yfinance`.
12. `TestFallbackIdempotent` (new): the same run twice over a fake store that applies the precedence rule gives the same rows and the second reports them unchanged.
13. `job.toml` gains `fallback_tolerance = "0.005"`; `manifest_test.go` parses it as an exact `big.Rat`, and a missing or non-positive value is an error; `WithFallback` on a job of another package is an error.
14. `docs/contracts/jobs/bars_refresh.md` states the trigger, the basis rule, the volume rule and that the weekly and deepening steps are not part of the fallback.

### 9 bars-fallback-pages

Module: pages. Size: M. Order: after 1 and 7.
Contracts: unchanged
Files: `internal/pages/shared/bar_source.go`, `internal/pages/shared/bar_source_test.go`, `internal/pages/market_state_page/sectors.go`, `internal/pages/market_state_page/page.go`, `internal/pages/company_data_page/rows.go`, `internal/pages/company_data_page/builder.go`, `internal/pages/positions_page/bars.go`, `internal/pages/positions_page/page.go`, `docs/contracts/pages.md`
Design: docs/design/bars-fallback.md#Decisions (10)
Acceptance:
1. `TestBarSourceOf` (new, `t.Parallel()`): no rows and all-ibkr rows give kind `ibkr` and an empty note; all-yfinance gives `yfinance`; both give `mixed`; every note contains Chinese text and no Latin letters except `IB` and `Yahoo`.
2. Each of the three builders sets `bars_source` and `bars_source_note` from the rows it already reads (no extra store call); a test per page (`TestBarsSourceMixed`, new) builds the payload from a fake reader with one yfinance row and validates it against the schema of entry 1.
3. A payload from all-ibkr rows validates and the note is empty or absent.
4. No new file is over 600 lines.

### 10 bars-fallback-web

Module: web. Size: S. Order: after 1 and 9.
Contracts: unchanged
Files: `web/src/pages/market_state/MarketStatePage.tsx`, `web/src/pages/company_data/CompanyDataPage.tsx`, `web/src/pages/positions/PositionsPage.tsx`, `web/src/components/BarsSourceNote.tsx`, `web/test/pages/market_state/BarsSourceNote.test.tsx`
Design: docs/design/bars-fallback.md#Decisions (10)
Acceptance:
1. `BarsSourceNote.tsx` (new) renders `bars_source_note` as text when it is a non-empty string and renders nothing otherwise; it computes nothing and reads no `bars_source` kind itself.
2. The three pages render `BarsSourceNote` with the payload fields; a payload without them renders as before.
3. `BarsSourceNote.test.tsx` (new) renders a mixed fixture note and an empty one.

### 11 bars-fallback-wire

Module: ot. Size: S. Order: after 8, and after entry 7 is merged and its migration is applied (the window runs migrate), because the first fallback write needs the widened check.
Contracts: unchanged
Files: `cmd/ot/wire_barsfallback.go`, `cmd/ot/wire_barsfallback_test.go`, `cmd/ot/wire.go`
Design: docs/design/bars-fallback.md#Decisions (9)
Acceptance:
1. `wire_barsfallback.go` (new) holds `withBarsFallback(j)`: it builds the source through `jobFactory.barSource("yfinance")` and returns `barsrefresh.WithFallback(j, src)`; an error from the registry returns the job unchanged and logs one line.
2. `wire.go` changes by at most 5 lines (one call in `barsRefresh`) and stays under 800 lines.
3. `TestBarsRefreshHasFallback` (new): the built `bars_refresh` job has a source set; `TestBarsRefreshWithoutCommand` (new): with no registry entry the job is built without one and the build does not fail.
4. No line of `cmd/ot` holds a rule: the decision is in `barsrefresh`.

## Checked by CI

- `ci-go`: build, vet, tests of the packages, the architecture checks, the generation check and schema checks for entry 1; the `pg`-tagged tests of entry 7 run only in GitHub CI.
- `ci-pyside` for entries 4 and 5: `uv sync --locked`, `ruff check`, `mypy --strict`, `pytest -n auto`.
- `ci-web` for entries 1 and 10: `tsc`, `eslint`, `vitest`, build.
- No test sleeps, none connects to a gateway or to Yahoo: every wait goes through the fake clock; the `BarSource`, the `HistoryClient` and the calendar are fakes.

## Self-check

1. Fallback, not cascade: bars_refresh collects live from the second source when its own source fails; nothing downstream waits for it (Decision 1).
2. Runnable on its own and idempotent: no new job, no state but the `source` column (entry 8 item 12, entry 7 items 3 and 4).
3. One at a time: the fallback is inside `bars_refresh` under its existing run fence; no second run of the job, no new goroutine.
4. Dev mode, no time guards: the trigger is the error, the filter is the session calendar's data, never an hour or elapsed time (Decision 8); `fallback_tolerance` is a ratio.
5. The front end computes nothing: the kind and the Chinese note come from `shared.BarSourceOf` (entry 10 item 1).
6. Mainstream library: yfinance 1.8.0, Apache-2.0, already pinned in `pyside/pyproject.toml`.
7. Own glue behind an interface: `refbars` and `internal/sources/yfinance` behind `sources.BarSource`; the job behind `WithFallback`.
8. No ticker or stock code in this text: checked by search.
9. Size: the largest new file is `fallback.go` (under 600); `cmd/ot/wire.go` is 789 lines with at most 5 added; `run.go` is 241 lines.

## Left out

- The signal `history_unavailable` and `broker.HistoryDown` of `docs/design/gateway-health.md`: not used here; HistoryDown may later shorten the wait before the fallback (one more trigger in Decision 1).
- `adjusted_last` and the deepening of old history from Yahoo: IB only (Decision 3).
- Indexes and currencies other than USD and JPY: Yahoo has no route in `refbars/symbols.py`; no fallback, their days stay gaps and the market pages show them. A mapping is its own change.
- Figures read from stored bars (`computefigures` hv, tech and vol, `buyprice`, `optionopen`, `optionledger`, `valuation`): they are not given the source and are not recomputed after healing, because a figure is a function of the stored bars of its day and the next run of each reads the healed bars; the note of the pages says figures of a fallback day may rest on a Yahoo bar.
- `fx_refresh`, `recon_bars` and the report global-assets block: not changed (`recon_bars` keeps its close-only use of `RefBar`; `fx_refresh` has its own secondary; the report block gets the note only if the owner asks).
- Health state and notice of the broken farm: `docs/design/gateway-health.md`.
- A per-name or per-day source switch, a history table of sources, a second fallback provider: not adopted.

## Doubts

1. Yahoo egress from the service host is not verified: the window checks it read-only (one `refbars` command run) before entry 11 is cut.
2. yfinance 1.8.0 has so far been used only by `recon_bars`, not for a write path; the basis comparison (Decision 5) is its guard.
3. Contracts versions: if the owner or advisor reads `00-protocol.md` as allowing an optional page field without a bump, entry 1 shrinks to the dataset r2 and the page schema files keep their names; the code that reads `AdjustEpoch` of `contracts.PriceBar` (r2 drops it from `required`) is not enumerated here and entry 1 must list it.
4. The JP calendar id the session filter uses is not named here; entry 8 takes the one `calendar_refresh` already stores.

## Adversarial

Source: the full adversarial answer is kept by the dispatcher outside the repository.

1. Gap against the goal: adopted in part. The weekly and deepening steps and the unrouted pairs stay gaps by design and are named (Left out); a basis difference stays a named Retryable gap (Decision 5), kept because a mixed-basis series is worse than a gap. The trigger no longer depends on an absent signal (Decision 1). The report block stays Left out.
2. Unnamed risk, the trigger: adopted. The trigger is what main emits today and the broker entries are dropped; the exact-string contract is gone. The first-key wait is reduced by `histDown` after the second failing key.
3. Simpler way: not adopted as cut to 5 slices (the one-module-per-entry rule and interface-first order in AGENTS.md stay); the broker entries are dropped (11 entries). The tolerance gate is kept as trial, with its reason (Decision 5), because write-only-where-no-row leaves a wrong basis in the series after a split.
4. Lock-in: adopted. The dataset is a new revision r2; derived figures are named in Left out with the reason; the page note says figures may rest on a Yahoo bar.
5. Claim likely false: adopted. Both sources are cited with their date; volume is written NULL; the split delay of Yahoo is the reason for the comparison.
6. Likeliest slip: adopted. The gateway-health implementation is no longer a dependency; the migration is held by entry 7 and entry 11 waits for it and for the applied migration.
