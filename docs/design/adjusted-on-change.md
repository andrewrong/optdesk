# adjusted on change: bars_refresh re-pulls adjusted_last only for a name whose adjustment changed, found without an IB request

## Problem

`internal/jobs/barsrefresh/adjusted.go` asks IB for the whole backfill window (400 days) of adjusted_last for every key every `adjusted_cadence_days` = 7 and upserts it, whether or not a dividend or split happened (75 names, one request each per week, blind). The owner wants: refresh only when a dividend or split changed the adjustment, the decision costing no IB request, and no short or half-consistent adjusted series.

Facts read on 2026-11-24 (main c5ecf29b):

- IB takes no End day for adjusted_last and 1 to 730 days (`internal/broker/api.go` BarQuery; `pyside/src/optdesk/ibkr/transport.py` `_MAX_BAR_DAYS`, "adjusted_last bars take no end_day"). The deepest adjusted series one request gives is 730 days, and it cannot be asked in older pieces. Trades go deeper (deepen_days=1825).
- obs.price_bar: 75 instruments, adjusted_last 482 to 523 rows each, oldest day 2024-11-11 to 2024-11-29; trades 481 to 1519 rows, oldest 2020-11-17.
- IB TRADES bars are split-adjusted, not dividend-adjusted. A split therefore already shows as a restated trades close in the daily trades pull (`restated`, `collect` in `run.go`, which today re-pulls the whole backfill when it fires). A dividend does not show in trades.
- Dividends stored: `rd.dividend_expectation_r1` (store/dividend Reader, rows per instrument and session day with the projected next ex-date from IB tick 456, filled by the dividend job): 44 instruments, from 2026-11-10. yfinance is wired in barsrefresh only as a daily-bar fallback (`sources.BarSource`); it has no dividend or split call, so using it would need new code in sources and in the yfinance command. IB `Dividends` (reference interface) is a forward projection and costs a request.

## Decisions

1. Trigger, no IB request. A key's adjusted_last is re-pulled when (a) the daily trades pull of the run found a restated stored trades day (a split; the existing `again` of `collect`), or (b) a stored dividend answer of the key has a next ex-date after the day its newest adjusted row was received and on or before the run day. No trigger, no adjusted request: new days of adjusted_last are not asked at all (nothing reads them without a change; adjusted equals trades after the newest ex-date). Chosen over an IB probe of the newest bars because the owner asked for no request in the decision, and over yfinance actions because that needs a new call in sources and the yfinance command; it stays the follow-up if the owner wants coverage beyond IB tick 456.
2. Refresh = one request of `adjusted_days` = 730 days (new manifest key, 1 to 730, replaces backfill_days in this step) and an upsert of every answered row by key, so all stored days inside the window carry one adjustment. A key with no adjusted row yet is pulled once the same way. A key whose oldest adjusted day is more than 30 days after both its oldest trades day and the run day minus 730 is pulled once (a short series is fixed once; a name that lists later does not repeat).
3. Rows older than the 730-day answer are never deleted, rewritten or computed: IB gives no older adjusted day, and an adjusted row is not derived from trades (owner rule). Such a row keeps its `received_at`, which says under which adjustment it was written. This is the stated limit; the pull request of the jobs issue names it.
4. Known hole: a name with no stored dividend answer, a dividend IB tick 456 did not project, and a special dividend give no trigger (no timer replaces it). Before this ships the owner is told the count of adjusted names with no dividend answer (47 of 75 have no `obs.dividend_expectation` row today).
5. The deepening of trades (`deepen.go`) no longer waits for the weekly adjusted gate; it keeps its own once per `adjusted_cadence_days` limit (`askedRecently`), the key stays for that use only.
6. Wiring: the job gets the dividend Reader through an option `WithDividends` (as `WithFallback`), declared in the job package, no change to `internal/jobs/api.go`. `cmd/ot` opens the dividend store read-only with the pages role line and attaches it; a job without it behaves as decision 1 (a) only.

## Interface

- `barsrefresh.WithDividends(j contracts.Job, r dividend.Reader) (contracts.Job, error)`: nil reader or a job of another package is an error. Declared in `internal/jobs/barsrefresh`, which already depends on store.

## Issues

Build order: entry 1 lands before entry 2.

### 1 adjusted-on-change-job

Module: jobs. Size: M. Order: first.
Contracts: unchanged
Files: `internal/jobs/barsrefresh/adjusted.go`, `internal/jobs/barsrefresh/deepen.go`, `internal/jobs/barsrefresh/run.go`, `internal/jobs/barsrefresh/job.go`, `internal/jobs/barsrefresh/manifest.go`, `internal/jobs/barsrefresh/job.toml`, `internal/jobs/barsrefresh/adjusted_test.go`, `internal/jobs/barsrefresh/deepen_test.go`, `internal/jobs/barsrefresh/manifest_test.go`, `internal/jobs/barsrefresh/run_test.go`
Design: docs/design/adjusted-on-change.md#Decisions (1, 2, 3, 5, 6)
Acceptance: as the draft `adjusted-on-change.md` of the same name.

### 2 adjusted-on-change-wire

Module: ot. Size: S. Order: after entry 1.
Contracts: unchanged
Files: `cmd/ot/wire_barsdividends.go`, `cmd/ot/wire_barsdividends_test.go`, `cmd/ot/wire.go`
Design: docs/design/adjusted-on-change.md#Decisions (6)
Acceptance: as the draft `adjusted-on-change-wire.md`.
