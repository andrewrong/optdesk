# stock history from yfinance: the deep daily bars of a stock, raw and adjusted, come from yfinance and are re-pulled only when yfinance shows a new dividend or split

## Problem

IBKR is the live trading platform and its history farm shares a budget of 50 requests per 10 minutes with everything else. `internal/jobs/barsrefresh` spends it on blind work: the first 400-day backfill, the deepening of trades in pieces (`deepen.go`), the re-pull of the whole backfill and every older piece when a stored trades day is restated (`restate.go`, `olderThanWindow`), and `adjusted.go`, which every `adjusted_cadence_days` = 7 asks the whole backfill window of adjusted_last for 75 names, event or not. The owner decided on 2026-11-24: IBKR is used for option data and live (real-time) prices only and asks no stock daily bar at all; yfinance is the source of every daily bar of stocks and ETFs, the daily close included, raw and adjusted, as yfinance gives it; the adjusted history of a name is pulled again in full only when yfinance shows a new dividend or split since the last pull; no event, no history request. Stock bars matter less than option data, so the cut is small.

## Facts (read on main c5ecf29b, 2026-11-24)

- Adjusted_last in obs.price_bar goes back about 730 days although `backfill_days` = 400: `duration_text` in `pyside/src/optdesk/ibkr/history.py` turns more than 365 days into whole years rounded up, so 400 days is asked as "2 Y". IB takes no End day for adjusted_last and the rpc caps a request at 730 days (`_MAX_BAR_DAYS`, our own limit). Not probed on the gateway (owner: no long stock history from IBKR).
- The restatement path (`collect` in `run.go`): when a stored trades day differs from the pulled one by more than restate_tolerance, the whole backfill (400 days, so 2 Y) is pulled and `olderThanWindow` pulls every older piece (deepen_piece_days) down to the oldest stored day. So trades are re-pulled to full stored depth, not only 400 days; adjusted_last is not (only the weekly step, 2 Y).
- DB: 75 instruments; adjusted_last 482 to 523 rows each (oldest 2024-11-11 to 2024-11-29), trades 481 to 1519 rows (oldest 2020-11-17); every row has source ibkr, adjust_epoch 1. The columns open, high, low, close and volume are nullable.
- Readers: `computefigures` HV (`hv.go`, trades only, 70 calendar days back), `cmd/ot` strategy replay (`strategy_replay_market.go`, trades and adjusted_last closes of the entry and settle days of each lot), company page inputs and PE/PB (`companies_inputs.go`, `companies_pepb.go`, trades), option EOD index (`wire_optioneodindex.go`, trades), valuation site (`valuation_site.go`). Only the replay reads adjusted_last, and only its close.
- yfinance in the repo: `pyside/src/optdesk/refbars` (one-shot command, `python -m optdesk.refbars`, JSON in and out) and the Go adapter `internal/sources/yfinance` (`sources.BarSource`). Limits the code shows: at most 10 pairs per command run, 1 second between pairs (`GAP_SECONDS`), 10 seconds per pair (`TIMEOUT_SECONDS`), a `rate_limited` pair stops the later batches, error codes rate_limited / no_data / network / timeout / source_changed. Symbols: USD keeps the symbol (a dot becomes a dash), JPY appends `.T` (4063 is 4063.T), any other currency has no route (no_data). Route rule of this design: a key has a yfinance route when it is a stock or ETF (not an `[[index]]` table of `job.toml`) in USD or JPY; the indexes SPX, NDX, RUT and N225 have no route and stay on IB, because the Yahoo symbols of those names are not those indexes. The call is `Ticker.history(start, end, interval="1d", auto_adjust=False, actions=False)`; it already keeps Close raw and drops "Adj Close" and the Dividends and Stock Splits columns. No depth limit is in our code (the start day is the caller's); Yahoo's depth and rate are not probed here and are named as a risk below.
- Store rule today (`internal/store/barpg/store.go`): a yfinance row never replaces a stored ibkr row; an ibkr row replaces a yfinance row. The source check already allows ibkr and yfinance.

## Decisions

1. Source split (owner 2026-11-24). IBKR is for option data and live prices only: barsrefresh asks IB for no daily bar of a stock or ETF, not the recent window, not backfill, not deepening pieces, not restatement re-pulls, not adjusted_last. yfinance gives all of it: the daily close, the older days, raw and adjusted, dividends and splits. Today's live price stays on its existing IB path, which is not barsrefresh. A key with no route (an `[[index]]` key such as SPX, NDX, RUT, N225, or a stock in a currency other than USD or JPY) keeps its whole IB path as today (Decision 5).
2. The event test is a cheap yfinance call (a second one for the adjusted close of new days). For a name the job asks yfinance for the days from its last history pull (the session day of the newest stored yfinance row of the key, not its received_at) through today with actions: the answer carries the Dividends and Stock Splits rows of those days. A dividend or split dated after the last pull is an event; none is no event. No event: the new days are written, raw (auto_adjust off) and adjusted close (a second call with auto_adjust on, Close only, same days), and nothing else is asked. The test also brings the new daily bars, so a stock's daily close comes from it. It costs a yfinance request, never an IB one; it is made at most once per name per run day.
3. On an event, or for a name with no yfinance row, the job asks the full history twice in one pull: raw (auto_adjust off: Open, High, Low, Close, Volume as yfinance gives them) and adjusted (auto_adjust on, Close only is stored; Open, High, Low stay empty). yfinance's own adjusted numbers are stored; no factor is computed or applied in our code, and no adjusted row is derived from raw rows. Adjustment is standard practice: vendors back-adjust the whole series on each dividend or split, so one full re-pull on an event that replaces the stored adjusted series is the same thing. The full history starts at the first day yfinance has (listing); `deepen_days` and `backfill_days` no longer bound it.
4. Which source wins per day. yfinance rows are the stock series, trades and adjusted_last. The first full pull of a name, and every event pull, writes through a new store call `ReplacePriceBars` (store interface, optional method like `PriceBarUpserter`): the yfinance row replaces the stored row of any source, for every day of the pull, so the existing ibkr rows of a stock are replaced by yfinance rows and one series per name has one adjustment. A normal write of new days keeps the store rule (a yfinance row never replaces an ibkr row). No stored row is deleted; a replaced row keeps its key and the writer name says which job and day wrote it.
5. Hole and limit named: a dividend not in yfinance's actions gives no trigger. A key with no route (an `[[index]]` key, or a stock in a currency other than USD or JPY; FX rates are not keys of barsrefresh, `fxrefresh` owns them) keeps everything `runKey` in `run.go` runs today: the conid lookup, the IB daily window with its backfill and restatement re-pull, the deepening pieces and the weekly IB adjusted_last step; its stored rows do not change; it is named in Left out. Yahoo can refuse (rate_limited): the key stays as it is and is asked the next run, never filled from another model. Every key with an event and no answer is logged by name.
6. A stock with a yfinance route has no `daily_days` or `backfill_days` IB window, no deepening, no restatement re-pull and no adjusted_last step. The keys `adjusted_cadence_days`, `deepen_*`, `daily_days` and `backfill_days` of `job.toml` stay and apply only to the keys of Decision 5, whose weekly IB adjusted_last step stays as today. No wire entry: the job already gets its yfinance source from `WithFallback` in `cmd/ot/wire_barsfallback.go` and the new source methods come through the same `sources.BarSource` value.

## Interface

Landed first by the entry `stock-history-iface` (interface); the module entries use it.

- `pyside/src/optdesk/refbars/api.py`: Request gains `with_actions` and `adjusted` (bool); Bar gains `adj_close` (decimal text, empty when not asked); PairResult gains `dividends` and `splits` (lists of `{day, value}` as decimal text). Declarations only.
- `internal/sources/api.go`: `BarRequest` gains `Actions bool` and `Adjusted bool`; `RefBar` gains `AdjClose string`; `BarResult` gains `Actions []RefAction` with `RefAction{Pair, Day, Kind ("dividend" or "split"), Value}`. Fields are added, none changes.
- `internal/store/bar/api.go`: `PriceBarReplacer` (new, optional): `ReplacePriceBars(ctx, rows, writtenBy) (BarWriteCount, error)` replaces a stored row of the same key whatever its source; implemented by `barpg`.

## Owner-visible result

After the first full pull the adjusted_last series of a stock reaches back to its listing (a 10-year name has 10 years, not 400 or 730 days), the rows of a name carry one adjustment, and the replay and company pages read the same tables as before. IB history requests from barsrefresh for stocks drop to 0 (only the index keys, and a stock in a currency other than USD or JPY, still ask IB); the daily window, the weekly 75, the first backfills, the deepening pieces and the restatement re-pulls are gone for stocks. A split or dividend shows in the data the same day yfinance lists it.

## Issues

Build order: entry 1 (interface) lands first; entries 2 to 5 each depend on it only (entry 5 also on 3 and 4 through fakes of their interfaces, which entry 1 holds).

### 1 stock-history-iface (interface)

Modules: pyside, sources, store (interface files only). Size: S. Order: first.
Contracts: unchanged
Files: `pyside/src/optdesk/refbars/api.py`, `pyside/tests/contracts/test_refbars_api.py`, `internal/sources/api.go`, `internal/sources/api_test.go`, `internal/store/bar/api.go`, `internal/store/bar/api_test.go`
Design: docs/design/stock-history-yfinance.md#Interface
Acceptance: as the draft `stock-history-iface.md`; declarations only, no behaviour.

### 2 stock-history-refbars

Module: pyside. Size: M. Order: after 1 (stock-history-iface).
Contracts: unchanged
Files: `pyside/src/optdesk/refbars/fetch.py`, `pyside/src/optdesk/refbars/cli.py`, `pyside/tests/refbars/test_fetch.py`, `pyside/tests/refbars/test_cli.py`
Design: docs/design/stock-history-yfinance.md#Decisions
Acceptance: as the draft `stock-history-refbars.md`.

### 3 stock-history-store

Module: store. Size: S. Order: after 1 (stock-history-iface).
Contracts: unchanged
Files: `internal/store/barpg/store.go`, `internal/store/barpg/store_test.go`, `internal/store/barpg/upsert_pg_test.go`
Design: docs/design/stock-history-yfinance.md#Decisions
Acceptance: as the draft `stock-history-store.md`.

### 4 stock-history-sources

Module: sources. Size: M. Order: after 1 (stock-history-iface).
Contracts: unchanged
Files: `internal/sources/yfinance/source.go`, `internal/sources/yfinance/source_test.go`
Design: docs/design/stock-history-yfinance.md#Decisions
Acceptance: as the draft `stock-history-sources.md`.

### 5 stock-history-job

Module: jobs. Size: L. Order: after 1 (stock-history-iface).
Contracts: unchanged
Files: `internal/jobs/barsrefresh/adjusted.go`, `internal/jobs/barsrefresh/deepen.go`, `internal/jobs/barsrefresh/restate.go`, `internal/jobs/barsrefresh/run.go`, `internal/jobs/barsrefresh/manifest.go`, `internal/jobs/barsrefresh/job.toml`, and their tests in the package
Design: docs/design/stock-history-yfinance.md#Decisions
Acceptance: as the draft `stock-history-job.md`.

## Left out

- A yfinance probe of its depth and rate limit: not made here; the issue of entry 3 states what the adapter does on rate_limited, and the owner sees the first full pull in the log.
- A second source for dividends and splits (IB tick 456 is a forward projection and is not used).
- Intraday bars and option history.
- Keys with no yfinance route (the `[[index]]` keys, a stock in a currency other than USD or JPY): left as they are today, the whole IB path including the weekly adjusted_last step; no change.
- A dividend-adjusted row older than the first yfinance day: not made.
- The classification of the 47 names without a stored dividend row: no longer needed; the trigger is yfinance's own actions.

## Adversarial

1. Gap against the goal. The goal is a consistent deep series with no blind IB work. The cut leaves no IB history request for stocks and a once-a-day yfinance event test per name; the series is consistent only if yfinance's adjusted numbers are, which we take on trust. The first full pull replaces the stored ibkr rows of a stock.
2. Biggest unnamed risk. Yahoo is an unofficial scrape: a rate limit on 75 names with a 1 s gap, a changed column name (source_changed) or a cut of old depth could stop the event test, and then a dividend is silently late and the stock's daily close of the day is missing (no IB fallback for stocks). The job logs each name it could not test; nothing alerts the owner yet.
3. Simpler way. Keep IB and ask adjusted_last (2 Y) only on a split seen in trades: that misses dividends, which are the common event, so it is not enough; or drop adjusted_last and let the replay use raw closes plus dividends, which is a replay change outside this cut.
4. Lock-in. `ReplacePriceBars` lets one source overwrite stored rows of another; a later wrong yfinance answer would overwrite IB rows with no copy kept (rows are not versioned since adjust_epoch was frozen at 1). The replace is limited to the days of a full pull.
5. Claim most likely false. That yfinance lists a dividend or split by the day it happens and that "Adj Close" of the full pull equals the adjustment of the earlier pull plus the event; the depth to listing and the 4063.T history are assumed from the library, not checked in this repo.
6. Likeliest slip. The jobs entry (4) is large: it removes the IB steps for routed stocks and adds two sources of rows; a leftover caller of `deepen` or `adjusted` or the restate re-pull would keep IB requests alive. The acceptance names the removals and the issue lists every file.
