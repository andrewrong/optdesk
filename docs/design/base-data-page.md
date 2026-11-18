# base data page: one page that shows only base data and says, per dataset, whether it is correct; version 1 checks values, version 2 reconciles against the broker and a second source

Author: the optdesk designer (draft, fix round 2026-11-18 after adversarial.md)
Date: 2026-11-18

Owner decision: route a, 2026-11-18, asks optdesk-4921 and 4924 (plan item P1): version 1 now, version 2 designed in full here and cut into issues after the owner's source word. The design review of this file is the crew's; the owner decides only the second source (Version 2, owner question).

Source: `state/optdesk-data-health-audit-2026-11-18.md` (gaps 1 to 9) and the read-only review `state/optdesk-drafts/designs/base-data-page/adversarial.md` (answered in `## Adversarial`). Owner principles used: start from how he uses the page; a technical word is explained next to it in Chinese; the front end computes nothing; dev mode has no time guards; every job runs alone and twice with the same result; own glue sits behind an interface (2026-11-08); an outside source or a new account is the owner's word (2026-11-18). Every path, view, table and column below was read on origin/main `c16ebc95` after `git fetch`; nothing was run and no database was read.

## Goal

The owner builds strategies on base data (stock bars, quotes, option chains, rates, fundamentals, FX, events, the account's own facts). Today `/data-health` proves that a job ran and stored the keys it planned (`rd.coverage_day_r1`); it does not look at a value. This design adds one page, `/base-data`, that answers one question per base dataset: "is what is stored right, and how fresh is it?", with one verdict (pass, warn, fail, unknown), the newest value with its date and age, and the checks that failed. Version 1 runs value-level checks (ordered, plausible, fresh, complete against the expected universe). Version 2 reconciles against the broker's own statement and against a second market source.

## Page

How he uses it: opens `/base-data` once a day before looking at candidates. He reads the top line, scans one column of verdicts, and opens the one red row to see which check failed, with a Chinese sentence saying what it means and a link to `/data-health` where the repair button lives.

Sketch (labels English, hints Chinese; the numbers are invented):

    Base data                                  as of 2026-11-18 (last closed session US 2026-11-17, JP 2026-11-18)
    ┌──────────────────────────────────────────────────────────────────────────────────────────┐
    │ FAIL   12 datasets: 8 pass, 2 warn, 1 fail, 1 unknown      基础数据共 12 项：8 项正常 ...      │
    └──────────────────────────────────────────────────────────────────────────────────────────┘
    Market data                         Verdict  Newest value        Date        Age     Failed checks
    Stock bars, US and JP (日线)         WARN     NVDA close 181.42   2026-11-17  0 s     coverage_gap 3 days
    Quotes         (报价快照)            PASS     SPY last 590.10     2026-11-17  0 s     -
    Option chains end of day (期权链)    FAIL     rows 112,004        2026-11-16  1 s     age; crossed 2.1%
    ...
    Rates and events                     (FRED, FX USDJPY, earnings dates, dividends, macro events, calendar)
    Company facts                        (fundamentals)
    Account facts from the broker        (positions, cash and value tags, Flex statement)
    Derived data (readings, candidates, strategy, plan) is not checked here: 在「采集记录」页查看.

    Click a row: the checks of that dataset, one line each: rule id, English label, what it means in Chinese,
    rows examined, rows bad, share, threshold used, one example (symbol and day).
    Below the table: a glossary (term, Chinese explanation) and the thresholds table (each marked "trial value"
    or its source). Age "s" means sessions (交易日).

The 12 rows, their groups and where each reads (the one table, owned by the interface: `basecheckapi.Rows` in `internal/compute/basecheckapi/api.go`, entry 2; the builder, compute and the tests read it from there). "Ledger datasets" are the dataset ids of the coverage ledger (`dataHealthManifests` in `cmd/ot/data_health_site.go`); the builder sums the cells of these ids into one cell named by the row id. "Subjects" are the values the SQL puts in `Fact.Subject` for `newest`; compute maps a subject to its calendar for ages.

| row id | group | ledger datasets | subjects and calendar | facts read |
|---|---|---|---|---|
| bars | Market data | `obs.price_bar` (one ledger row for US and JP; the ledger cannot split it, so the row is one) | `USD` XNYS, `JPY` XTKS | `rd.price_bar_r1` with `master.instrument` (symbol, currency) |
| quotes | Market data | `obs.quote_snapshot` | `USD` XNYS | `rd.quote_snapshot_r1` |
| chains | Market data | `obs.option_eod`, `obs.option_eod_jp`, `obs.option_eod_index` | `USD` XNYS (stock and index chains), `JPY` XTKS | `rd.option_eod_r1` |
| open | Market data | `obs.option_open`, `obs.option_open.jp` | `USD` XNYS, `JPY` XTKS | `rd.option_open_r1` |
| contracts | Market data | `master.option_list`, `master.option_list.jp`, `master.option_contract`, `master.option_contract.jp`, `master.option_contract.gap` | none (calendar days) | `rd.option_terms_r1` |
| rates | Rates and events | `obs.series_point` | none | `rd.series_point_r1`, `obs.series_definition` |
| fx | Rates and events | `obs.fx_rate` | `USDJPY` XNYS | `rd.fx_rate_r1` |
| events | Rates and events | `obs.earnings_date`, `obs.dividend_expectation`, `obs.macro_event` | none | `rd.earnings_date_r1`, `rd.dividend_expectation_r1`, `obs.macro_event` (table) |
| calendar | Rates and events | `master.session_day`, `master.session_hours_notice` | none | `rd.session_day_r1` |
| facts | Company facts | `obs.fundamental_fact` | none | `rd.fundamental_fact_r2` |
| account | Account facts | `account.capture`, `account.position_mark` | `USD` XNYS | `rd.account_capture_r1`, `rd.account_position_r1`, `rd.account_value_r1` |
| flex | Account facts | `optionledger.flex_trade_row` | none | `rd.flex_trade_row_r1`, `rd.flex_section_status_r1` |

Not rows, on purpose: `settings.underlying` (the owner's own input, shown and edited on `/underlyings`, not base data), and the derived datasets `readings.reading`, `candidates.option_candidate`, `strategy.result`, `plan.option_open_item(.jp)`. `account.execution` is read by version 2 only.

Decisions:

1. Keep `/data-health`; add `/base-data` as a new page. Why: `/data-health` holds the repair button (`POST /owner-job-console`) and the coverage grid of every dataset including derived ones (`cmd/ot/data_health_site.go`, route `data-health`, order 130); replacing it would drop both. The owner's "one page" is about correctness: the new page is the only place that says correct or not and names `/data-health` as the place of the repair. `/data-health` gets no change in version 1.
2. Base and derived are separated by page: this page has the 12 base rows only; the page says in one line that derived data is on `/data-health`.
3. Where checks run: in the page builder of each request, not in a job and not stored. SQL views count bad rows per rule and return one fact row per (dataset, rule, subject). Cost, stated honestly: the keys of the big tables are not led by the day (`obs.price_bar` key is `(instrument_id, basis, session_day)`, `obs.fx_rate` `(pair, rate_day)`), so the bars view scans the last 45 calendar days with a window function; its bound is (instruments with bars) x about 31 sessions x 2 bases (`trades`, `adjusted_last`), and the number of instruments is the size of the watch list, which I have not measured. The chain, quote and open tables are keyed from `session_day`. Page time on production is unverified: it is read once after the deploy; if it is slow the fix is an index or a narrower window in a later migration, and a job with a stored result is the version 2 shape anyway. One failing view must not blank the page: `Facts` reads the four views one by one, each under its own timeout (10 seconds, trial); a view that fails or times out makes only its rows `not judged` (level `unknown`, check `not_read`) and puts one gap in words on the page; the other rows stay. The view groups and their rows: bars (bars); option (quotes, chains, open); other (contracts, rates, fx, events, calendar, facts); account (account, flex). The four views are in four migrations, 0056 to 0059, one view each, all landed by one issue (entry 5), so the owner has one deploy hold and no window in which a view exists without its reader.
4. The arithmetic is in two modules: SQL views (store) count bad rows by a rule; `internal/compute/basecheck` (compute, no I/O) turns counts, thresholds and the sessions into verdicts, ages and the top line; the page builder (pages) only lays out and adds Chinese text from `zh_texts.go`. The web shows the payload and computes nothing.
5. Verdicts: a check is `pass`, `warn`, `fail` or `unknown` (no fact for it). A row takes the worst of its checks in the order fail, unknown, warn, pass. A row with no usable row at all is `fail` (rule `rows`). The top line takes the worst row and says "pass" only when no row is warn, fail or unknown.
6. Age is in sessions of the subject's calendar, counted from the newest stored day to the last closed session of that calendar. The builder gives compute `Days` (calendar to last closed session) and `Sessions` (calendar to its session days from `coverage.Reader.Sessions`). The last closed session of a calendar is its newest session day strictly before the current date in the calendar's zone (America/New_York for XNYS, Asia/Tokyo for XTKS; the day of the close counts from the next morning; trial). Constants for the two calendar ids and zones live in the page package (`calUS`, `calJP` are in `cmd/ot/data_health_site.go`, package main, and cannot be imported). A row with several subjects takes the worst age of its subjects. Pass 0, warn 1, fail 2 or more sessions (trial). A newest day later than the last closed session (JP data on a US morning, a quote of the running session) gives age 0, never a negative number. Both calendars are always in `Days` and `Sessions`; a row whose subjects have no calendar uses calendar days. Rows without a calendar use calendar days and the limit named in their rule.
7. One row per contract, by the documented status (`docs/contracts/datasets/obs.option_eod.md`: 1 ok, 2 no quote, 3 not listed, 4 stale, 5 error; `statusOf` in `internal/jobs/optioneod/map.go` sets `error_code` only for 5 and maps an all-empty quote to 2; the quote snapshot uses the same codes, `internal/jobs/quotesnapshot/map.go`; the option open table has its own list, 1 ok, 2 partial, 3 no_data, 4 not_listed, 5 no_subscription, 6 deadline, 7 deferred, 8 error, 9 disconnected, `docs/contracts/datasets/obs.option_open.md`). For chains, quotes and open: the newest `session_day` of the subject's currency; per `instrument_id` the picked row is the newest `received_at` among the rows with status 1 (`DISTINCT ON (instrument_id)`). N of `crossed` and `bad_iv` is the picked rows. A contract with no status-1 row that day is classified by its newest row: no quote (status 2 for chains and quotes, 3 for open) is counted in `empty`; error (5 for chains and quotes, 5 to 9 for open) in `errors`; not listed and stale (3 and 4 for chains and quotes, 4 for open, 2 partial of open) are counted in the fact `not_judged` (a number shown on the row, in no verdict). `empty` and `errors` use all contracts of the day as N. `non_live` is the share of picked rows whose `market_data_type` is not 1 (2 frozen, 3 delayed, 4 delayed frozen; the column is in `rd.option_eod_r1`, `rd.quote_snapshot_r1` and `rd.option_open_r1`).
8. Language: labels, column names and numbers English; every hint, risk text and glossary entry Chinese, as Go constants in `internal/pages/base_data_page/zh_texts.go`.
9. Ids cannot drift: every SQL block that emits a rule carries a comment line `-- rule <row> <rule>`; `basecheckapi.Rules` (entry 2) marks each rule `sql`, `ledger`, `derived` or `note` (a note is shown on the row and never judged); a test in `cmd/ot` (the one place that sees migrations, compute and pages together) requires the set of `-- rule` lines of the four migrations to equal the `sql` rules of `basecheckapi.Rules`, and the page package tests require a Chinese hint for every rule id and row id of the same table.

## Checks

Notation: N = rows examined, Bad = rows that break the rule, share = Bad / N. A threshold pair is (warn at, fail at): warn when Bad (or share) reaches the first, fail when it reaches the second; "any" means Bad >= 1. Source of a threshold: "def" = definitional (a price cannot break it), "trial" = our choice, tuned on live numbers after the first reads, in `rules.go` only. Window: the one row per contract of Decision 7 for chains, quotes and open; the last 45 calendar days for bars. "Day" in the views is `(now() at time zone 'America/New_York')::date`.

Rules that a table constraint already enforces are not repeated: positive open, high, low, close of a bar (`price_bar_positive_ck`, migration 0041; it excepts rows of status `no_value`, which `rd.price_bar_r1` hides with `WHERE status = 'ok'`); a positive FX rate (`fx_rate_rate_ck`, 0039, same exception); the number shape of a fundamental value (`fundamental_fact_value_ck`, 0046, added NOT VALID, so only new rows). `status` codes of the option tables are read (Decision 7). Dropped as not worth a rule: strike and multiplier of terms, `nonpositive` quotes, `bad_delta`, `no_oi`, `outside_band`, `position_zero`, `exec_bad`, `series_null` (void points are hidden by the view), `filed_early`.

Generic rules:

- `rows` (sql) N = usable rows of the subject; fail when 0 (def).
- `newest` (sql) not a check: the newest day, subject and value shown on the row.
- `age` (derived by compute from `newest`): Decision 6.
- `coverage_gap` (ledger, W30): from the ledger cells of the row (`coverage.Reader.Cells`, 30 session days): Bad = days with missing > 0; warn any, fail 5 days (trial). Days before 2026-11-07 are shown apart as the fact `history_lost` (audit gap 6).
- `coverage_latest` (ledger): Bad = missing + expired keys of the newest closed session of the row's first subject calendar (XNYS if none); N = keys; warn any, fail share 0.05 (trial). This is gap 5.
- `bar_gap` is not a rule: the ledger rules already say which keys are missing, and the table status that would tell a no_value day apart is not readable by the pages role.
- Silent drops (gap 7): `crossed` and `empty` counts are the rule results; the `no_value` keys of the newest cell are shown as a fact from the ledger.

Rules per row (dataset ids as in the table above; thresholds in the last column):

| row | rule (source) | Bad is | (warn, fail) |
|---|---|---|---|
| bars | `ohlc_order` (sql) | high < low, or high < max(open, close), or low > min(open, close), on `rd.price_bar_r1` rows of basis trades in the window | (any, any) def |
| bars | `bar_jump` (sql) | close differs from the previous stored close of the same instrument and basis by more than 40 percent (no split is stored anywhere, so a split looks exactly like this) | (any, none): warn only, never fail, until splits are stored; the hint says "a split looks like this" |
| quotes | `crossed` (sql) | bid > ask, both not null and above 0 | (share 0.001, 0.02) trial |
| quotes | `empty` (sql) | contracts whose newest row has no status-1 row that day and is status 2 (no quote); N = all contracts of the day | (share 0.1, 0.4) trial |
| quotes | `errors` (sql) | contracts with no status-1 row whose newest row is status 5; N = all contracts | (share 0.05, 0.3) trial |
| quotes | `non_live` (sql) | picked rows with market_data_type not 1 (frozen or delayed) | (share 0.1, 0.5) trial |
| chains | `crossed` (sql) | as quotes (crossed quotes of the snapshot instant are expected at a small share; a normal day should be pass) | (share 0.01, 0.05) trial |
| chains | `bad_iv` (sql) | iv_decimal not null and (<= 0 or > 5) | (any, share 0.01) trial |
| chains | `empty` (sql) | as quotes (far strikes have no quote) | (share 0.5, 0.9) trial |
| chains | `errors` (sql) | as quotes | (share 0.05, 0.3) trial |
| chains | `non_live` (sql) | as quotes (an off-session snapshot is frozen or delayed by design: warn, read the hint) | (share 0.1, 0.5) trial |
| open | `crossed`, `bad_iv`, `empty`, `errors`, `non_live` (sql) | as chains, with the open status list of Decision 7 (empty = status 3, errors = 5 to 9) | as chains |
| contracts | `newest` (sql), `age` (derived) | newest `valid_from` of `rd.option_terms_r1` | `age` 7 days warn, 30 fail (trial) |
| rates | `series_age` (sql) | N = active series (`obs.series_definition.active`); Bad = series whose newest observation_date is older than 4 days (daily), 14 (weekly), 45 (monthly), 120 (quarterly) from Day (the first two are the trial values of `liquidityapi.Params`) | (any, share 0.1) trial |
| fx | `fx_band` (sql) | rate not in 80..250 (JPY per USD; wide) | (any, any) trial |
| fx | `fx_jump` (sql) | moves more than 5 percent from the previous rate_day | (any, any) trial |
| events | `earnings_past` (sql) | N = issuers whose newest check has status listed; Bad = those whose report_date is before Day (the NVDA 2026-11-17 case of ui-check round 7) | (any, share 0.1) trial |
| events | `dividend_past` (sql) | the same on next_ex_date of `rd.dividend_expectation_r1` | (any, share 0.1) trial |
| events | `macro_exhausted` (sql) | N = kinds of `obs.macro_event`; Bad = kinds with no event_date on or after Day (columns kind, event_date, run_day, held_day; no observed_on) | (any, any) trial |
| events | `age` (derived) | newest `observed_on` of `rd.earnings_date_r1`, 3 days warn, 10 fail | trial |
| calendar | `calendar_horizon` (sql) | N = calendars (XNYS, XTKS); Bad = those whose last stored session_day is less than 30 days after Day | (any, any) trial |
| facts | `period_order` (sql) | period_start not null and period_end < period_start (`rd.fundamental_fact_r2`) | (any, any) def |
| facts | `subject_stale` (sql) | N = subjects; Bad = subjects whose newest `filed` is older than 135 days (a 10-Q is due 45 days after a quarter, plus one quarter) | (share 0.2, 0.5) trial |
| facts | `age` (derived) | newest `filed`, 30 days warn, 100 fail | trial |
| account | `capture_age` (derived) | newest `rd.account_capture_r1`, `age` in sessions | (1, 3) trial |
| account | `no_netliq` (sql) | N = aliases; Bad = aliases whose newest capture has no `rd.account_value_r1` row with tag NetLiquidation | (any, any) def |
| account | `position_expired` (sql) | sec_type option with expiry (text; if it is not an ISO date the fact is absent and the check `unknown`) before Day | (any, any) def |
| flex | `flex_age` (derived) | newest `recorded_at` of `rd.flex_trade_row_r1`, 3 days warn, 7 fail | trial |
| flex | `flex_section_absent` (sql, note) | N = sections wanted by version 2 (OpenPosition, EquitySummaryByReportDateInBase, ConversionRate); Bad = those with state absent in the newest fetch (`rd.flex_section_status_r1`) | a setup note on the row, never in the verdict |

Where results live: nowhere. Each request recomputes. `symbol` in an example comes from `master.instrument.symbol` joined on `instrument_id` inside the view (the migration runs as the owner; the `rd` views carry only `instrument_id`).

The first live read tunes `rules.go`; before the issues are cut the dispatcher may run one read-only query per rule on the production database (owner allows schema reads) to see that the first page is not red from a bad trial value.

Gap table (audit gaps): 1 value checks: the rules above; 2 cross-source: version 2; 3 latest value, date, age: `newest` and Decision 6; 4 base against derived and one verdict: Decisions 2 and 5; 5 expected universe: `coverage_latest`; 6 old gaps: `coverage_gap` with `history_lost` apart; 7 silent drops: `crossed`, `empty`, `no_value` fact; 8 earnings and close plausibility: `earnings_past`, `bar_jump`; 9 Chinese hints: Decision 8.

## Version 2

Principle: a number is trusted when two routes that do not share a fault agree. Version 2 reconciles pairs (ours, other) inside a tolerance, stores each run's result, and shows on the dataset row of this page the worst result of its pairs next to the version 1 checks.

Pairs (id, ours, other, key, tolerance; every tolerance is a trial value to be tuned after two weeks of live runs):

| id | ours | other route | key and compared value | tolerance |
|---|---|---|---|---|
| R1 positions | `account.position` of the newest capture of the session day | Flex `OpenPosition` rows (`rd.flex_section_row_r1`, section OpenPosition, newest revision, report date = session day) | (alias, symbol, sec_type, expiry, strike, right); quantity | quantity exact; avg_cost 0.1 percent (R1 to R5: any item of kind `diff`, `missing_ours` or `missing_other` is a fail) |
| R2 trades | `rd.account_execution_r1` | `rd.flex_trade_row_r1` | (alias, executed day, symbol, expiry, strike, right, side); sum of quantity and mean price; the exec id spaces differ, so ids are not used | quantity exact; price 0.0001 absolute; commission 0.01 |
| R3 cash and value | `rd.account_value_r1` tags NetLiquidation and TotalCashValue of the capture nearest after the close | Flex `EquitySummaryByReportDateInBase` (total and cash) at the report date | (alias, tag, currency); amount | max(1 unit, 0.05 percent): the API value is taken after the close, the statement at the end of day |
| R4 FX | `rd.fx_rate_r1` USDJPY | Flex `ConversionRate` JPY to USD at the report date | (rate_day); rate | 0.3 percent (fixing time differs) |
| R5 live positions | stored positions | the live gateway `broker.AccountReader.Account` (port 4001 read only), run once after each capture | the same key as R1 | quantity exact; detects a capture that dropped or doubled a line |
| R6 bars | `rd.price_bar_r1` basis trades (winner rows, highest `adjust_epoch`), last 5 sessions, USD and JPY, for the stocks of `watch.toml` (the universe of bars_refresh) | the second source yfinance through a one-shot pyside command (below), `Close` of `auto_adjust=False` | (symbol, currency, session_day); close | 0.1 percent pass, above warn, above 1 percent fail; volume is not compared (venues differ); a pair the source did not answer is kind `skipped`, never a verdict (S5) |
| R7 chain parity | `rd.option_eod_r1`, one call and one put of the same strike and expiry | put-call parity inside the stored chain: abs(C - P - (S - K)) <= (spread of C + spread of P) + 0.02 x S, with S = underlying_price; no outside source | (expiry, strike); the gap | the bound itself; share of pairs broken warn 0.02, fail 0.1 |
| R8 bar against quote | `rd.price_bar_r1` close of the newest session | `rd.quote_snapshot_r1` last of the newest slot after the close | (instrument); close against last | 0.5 percent |
| R9 FRED refetch | `rd.series_point_r1` newest 5 points of each active series | a fresh fetch through `sources.Source` (FRED) | (series, observation_date); value | exact; a difference is a vintage revision and is shown as warn, never fail |

R1 to R4 need no new source: the Flex statement is the broker's back-office route, independent of the API route that wrote our rows, and the Flex fetch and its sections already exist (`internal/jobs/optionledger/registry.go`, `optionledger.flex_section_row`, `optionledger.flex_section_status`). They need the sections OpenPosition, EquitySummaryByReportDateInBase and ConversionRate in the Flex query of the account; version 1's `flex_section_absent` tells whether they are there. If absent, the owner adds them in the IBKR Flex query screen (an owner action, no code).

### Second source for R6

Owner word, 2026-11-18T13:07Z (ask optdesk-4965): the second source for daily bars is yfinance, for US stocks and Tokyo stocks (symbol plus `.T`); "if it has problems I switch to Tiingo". The numbered decisions S1 to S7 settle it; each has an entry in Issues. The first design round chose a gRPC service; the adversarial review `adversarial-raw.md` showed a one-shot command is smaller (`## Adversarial`), so there is no proto, no socket and no service.

Facts of yfinance, read on PyPI and its sources on 2026-11-18 (workers have no web egress, so they are written here): version 1.8.0 (uploaded 2026-11-03), license Apache-2.0, an unofficial open-source client of Yahoo's public pages, no key and no account; its README says the Yahoo API is intended for personal use only, which is our case (one owner; the page shows our own verdicts, not Yahoo's data to others). Direct dependencies: pandas, numpy, curl_cffi, requests, lxml, beautifulsoup4, peewee, platformdirs, protobuf (pyside already has it), pytz, websockets; heavy, so the library lives in pyside only and Go imports none of it. It ships no `py.typed` and pandas has no stubs in the lock, so `mypy --strict` needs `ignore_missing_imports` overrides for `yfinance` and `pandas`. `history(start=, end=, interval="1d", auto_adjust=False, actions=False, timeout=10)` returns a frame with Open, High, Low, Close, Adj Close, Volume indexed by the exchange-local date; `end` is exclusive; the README says prices are adjusted for dividends by default (`auto_adjust=True`), so the call passes `False`: then `Close` is split-adjusted only and `Adj Close` is also dividend-adjusted. A failed call is hidden by default and gives an empty frame; `yfinance.config.debug.hide_exceptions = False` makes it raise (`YFRateLimitError`, `YFPricesMissingError` and others of `yfinance.exceptions`), and the command sets it, because an empty frame must never be read as "no bars". yfinance keeps a time-zone cache in a sqlite file under the user cache directory; the command points it to a temporary directory it removes at exit (`yfinance.set_tz_cache_location`), so it writes nothing that stays. Tokyo symbols are `<code>.T` (7203.T); a US class share uses a dash (BRK-B for BRK.B). Floats come out of pandas with artefacts (187.1999969482422): the command rounds to 4 places with `Decimal` and writes decimal text. Egress checked: from the service host on 2026-11-18T13:50Z (not from inside the deployed pyside process) `GET https://query1.finance.yahoo.com/v8/finance/chart/AAPL?range=5d&interval=1d` answered 200 in 0.07 s and the same for `7203.T` answered 200 in 0.08 s. The exact names are read against the 1.8.0 source by the pyside entry; a name that differs is reported, not guessed.

Decisions:

S1. Where it runs: a one-shot command, `python -m optdesk.refbars`, run by the Go adapter as `<uv> run --locked --project <pyside dir> python -m optdesk.refbars --deadline <seconds>`, exactly as `calendarCommand` runs `optdesk.calendar` (`cmd/ot/wire_calendar.go`; the uv program and the project directory come from the same `.env` lines). It reads one JSON request on stdin, `{"pairs":[{"symbol","currency"}],"from_day","to_day"}` (days `YYYY-MM-DD`, inclusive; at most 10 pairs), and writes one JSON answer on stdout, `{"results":[{"symbol","currency","error_code","bars":[{"session_day","close"}]}]}` (empty `error_code` is ok; codes `rate_limited`, `no_data`, `network`, `timeout`, `source_changed`); exit 0 whenever it wrote the answer, any other exit is a failed run. No gateway, no database, no socket, no service, nothing that stays running: the process lives only during the job. The calendar command's `calendarcli.Exec` discards output, so the Go side has its own exec with stdin and stdout (S3).
S2. The dependency lands first, in a separate entry written by the dispatcher window, because a worker has no network to resolve the lock: `yfinance==1.8.0` in `pyside/pyproject.toml` with the new `pyside/uv.lock` and the mypy overrides for `yfinance` and `pandas`; the entry holds only these and is green alone (the import-linter contract line for `optdesk.refbars` comes with the package, in entry 17, because `pyside/tests/guards/test_layers.py` runs the linter on `pyproject.toml` and a contract naming a missing package fails). Every other pyside entry (17 and 26) is ordered after it. Deploy: pyside runs with `--no-sync` and its build step is `uv sync --locked --no-dev --project pyside` (`docs/deploy/pyside.md`), so after the lock changes that build step must run on each host that runs the command (the ot host); the line is written into `docs/deploy/pyside.md` by entry 25 and the owner or window runs it after the merge of entry 12. The command of S1 runs with `uv run --locked` as calendar does. The owner's word covers the outside service (AGENTS.md libraries rule: a library that touches the network).
S3. Behind the interface: Go sees only `sources.BarSource`; the adapter `internal/sources/yfinance` runs the command of S1 through its own `CommandFunc` (argv, stdin bytes in, stdout bytes and exit code out; own glue, because yfinance is a Python library and no Go library reads Yahoo reliably; it sits behind the interface so a library or another source can replace it). Switching to Tiingo is: one new package `internal/sources/tiingo` (an HTTP adapter of `BarSource`), the config value `[source] id = "tiingo"` in `internal/jobs/reconbars/job.toml`, and one registry line `"tiingo": tiingo.New` in `cmd/ot/wire_barsource.go`; the job, the compare function, the store and the page do not change. The registry line is the composition root's, as `attachFxSecondary` is. Tiingo's key would come from the runner's service secrets (agent-crew arranges the line), through `BarConfig.Key`; no key is needed for yfinance.
S4. Raw against adjusted, the real model: `docs/design/market-bars.md` Decisions 3 and 4 say our `trades` bars are IBKR `TRADES`, split-adjusted by IBKR and not dividend-adjusted, insert-only with `adjust_epoch`, and bars_refresh re-pulls 5 days each run and restates a changed day as a new epoch. So both sides are split-adjusted and neither is dividend-adjusted: the job reads the winner rows (highest epoch) with `bar.PriceBarReader.PriceBars(instrumentID, "trades", from, to)` and compares them with yfinance `Close`. The split skip of the first round is not needed and is dropped: `recon_bars` runs after bars_refresh, which has restated the last 5 sessions, so a split inside the window is already in both; a difference that remains is a real `diff`, and its hint says to look at a split not yet restated. `Adj Close` is never used; the command does not read `Stock Splits`. The session day is the date of the frame index in the exchange's own zone (America/New_York, Asia/Tokyo).
S5. Failure is an unknown, a missing bar is a finding: a pair the source did not answer, and a day we have that the source lacks (Yahoo lags or drops days), are items of kind `skipped` with a reason (the error code or `source_lacks`), not counted; a day the source has and our store lacks is `missing_ours`, a finding counted with `diff` as bad (a whole session absent from our store is what this check exists to find); `missing_other` is not produced for R6. The pair level is made by `recon.Verdict` (entry 19) from the run's counts: compared N (matched + diff + missing_ours) of 0 is `unknown`; bad share (diff + missing_ours over N) above 0.001 is `warn`, above 0.01 `fail`; fewer than half of the asked pairs compared (the run stores `PairsAsked` and `PairsCompared`) is at best `warn`. (The package `internal/store/recon` is imported as `reconstore` and `internal/compute/recon` as `recon` in every file of entries 21, 22 and 23.) A run stores the count of each kind and only the items that are not `match`, so the 20 the page shows are the findings, listed per company (the item's `company` is the symbol). The page shows "compared N of M" with M = N + skipped. The level is made at page time from stored counts, so tuning a threshold never rewrites a run.
S6. No retry storm, partial results kept, one side batches: only the adapter batches. One `DailyBars` call splits its pairs into batches of 10 (one command run each) under the deadline of the context it is given. A batch is bounded by the smaller of 15 seconds per pair plus 15 (165 s for 10) and the time left of the context, and the adapter passes the command `--deadline` of that bound minus 10 seconds, after which the command stops and answers every pair it did not reach `timeout` with the bars it already has; a command killed at its bound loses its batch only, the earlier batches stay. When the time left of the context is 10 seconds or less before a batch, the adapter asks no further batch and marks every pair not asked `timeout`. Inside the command: one pair at a time, a gap of 1 second (from an injected clock, so a test never sleeps), the per-call `timeout=10` of yfinance, no retry; the first `YFRateLimitError` stops the batch at once, the remaining pairs are `rate_limited`, and the adapter asks no further batch and marks the pairs not asked `rate_limited`. `DailyBars` returns the partial result with the reason of each unanswered pair in `Unanswered`, and an error only when no batch answered. The job makes ONE `DailyBars` call with a context deadline of its `run_bound` minus 2 minutes (read from its own manifest, time from the injected clock); it knows no batch and no batch timeout; it writes one run at the end with status `partial` and, as its reason, the code of the first unanswered pair other than `no_data` (empty when every unanswered pair is `no_data`; a `no_data` pair is a `skipped` item with that reason). The whole run is bounded by `run_bound = "30m"` in `job.toml` (six batches of 165 s at most, for a watch list of up to 60 names); a job killed at its bound writes no run and the page reads `unknown`. A rerun is the owner's console action.
S7. Run time and the verdict row: `recon_bars` runs after bars_refresh, so the newest session is closed and restated. The level of R6 is fed to the row: `basecheckapi.Input` gains `Pairs []PairLevel` and `basecheck.Evaluate` takes the worst of the version 1 checks and the pair levels of that dataset row (entries 14, 19, 22); a day with no run gives the level `unknown`, so the bars row reads `unknown`, not `pass`, until the first run (honest, as the Version 2 principle says).


### Compare, storage and jobs

Compare logic: a pure function in `internal/compute/recon` (module compute): input two keyed lists and a tolerance, output items (key, ours, other, diff, kind) with kind `match`, `diff`, `missing_ours`, `missing_other`, `skipped` (not compared, with a reason; never counted in N; `Input.OtherMissingSkipped` turns a key only on our side into `skipped`, used by R6). One function serves R1 to R9, so tolerances are data (`rules.go` of `recon`), not code.

Storage: migration `0060_recon.sql`: `recon.run (run_id, pair_id, session_day, started_at, finished_at, status, reason, n, matched, diff, missing_ours, missing_other, skipped, pairs_asked, pairs_compared, tolerance_text, source_id)` and `recon.item (run_id, key, company, ours, other, diff, kind, reason)` (the same lists as entry 20 acceptance 1 and Interface 6), insert-only; a rerun is a new run_id and the view `rd.recon_latest_r1` takes the newest finished run per (pair_id, session_day), as `strategy.result` does. A role `ot_w_recon` (INSERT, SELECT on both tables, SELECT on the rd views the jobs read) and the line `OT_PG_DSN_RECON` follow `internal/store/setup`; the pages role gets SELECT on the view only. The line `OT_PG_DSN_RECON` is the owner's or `ot setup pg`'s to write, never the window's. The owner runs setup pg and migrate after the merge (owner rule 2026-11-09).

Jobs (each runnable alone for any day, idempotent: running twice leaves one newest run per pair and day, dev mode no time guard, one run of the same job at a time):

| job id | pairs | schedule (manifest) | table row in `jobTable` |
|---|---|---|---|
| recon_flex | R1, R2, R3, R4 | XNYS, after option_ledger finishes | anyDay, force allowed |
| recon_live | R5 | XNYS, 20 minutes after account_capture | todayOnly, no force (the gateway shows the present only; the owner can still run it from the console on the day) |
| recon_bars | R6 | XNYS, after bars_refresh | anyDay, force allowed |
| recon_internal | R7, R8, R9 | XNYS, after option_eod and fred_series | anyDay, force allowed |

Page rows: the row of version 1 gains the optional payload field `cross_check` (already in the v1 schema, Interface 3, so no version bump) shown as a column "Cross-check" with the worst pair verdict and the pair ids; opening a row shows, per pair: verdict, compared N, matched, differences, missing on either side, tolerance in words, source id, run time, and the first 20 items as a table (key, ours, other, difference, tolerance, kind in Chinese). A pair that has not run for the day is `unknown`, never pass.

How a mismatch is shown: a `diff` or `missing` item is a red line with both numbers side by side and the difference in the unit and in percent; the Chinese line says which side to trust first: for R1 to R4 and R5 "券商对账单为准，检查采集" (the statement wins, check our capture); for R6 "两个来源不一致，先看是否有拆股尚未重述；两边都已按拆股调整，都不含分红调整"; for R7 "期权链内部自相矛盾，检查报价时间"; for R9 "FRED 修订值，非错误". The pair verdict: R1 to R5 are exact on quantity, so any item that is not `match` is fail; R6 is made by `recon.Verdict` (S5): warn when the share of `diff` and `missing_ours` of the compared is above 0.001 and fail above 0.01, `unknown` when none is compared, at best warn when fewer than half of the pairs are compared; R7 warn 0.02, fail 0.1; R8 warn 0.005, fail 0.02; R9 never fails, any `diff` is warn. The row verdict is the worst of the version 1 checks and the pair verdicts.

Order of work after the owner's word: (a) R1 to R4 and R5 (no source needed); (b) R7, R8, R9; (c) R6 on yfinance (owner's word of 2026-11-18, cut below as entries 12 to 26). Cut at that time into issues: sources interface and adapter, compute recon, store recon migration and reader, jobs, pages and web columns, ot wiring; each under the same modules as below.

Owner question: answered 2026-11-18: yfinance (S1 to S7); Tiingo is the switch path if yfinance has problems (S3).

## Modules

`docs/design/modules.md` changes in two cells:

- `store` row, entry cell: add `internal/store/basefact` (declarations only; implementation `internal/store/basefactpg`, a sub-package component). Paths unchanged: `migrations/**` and `internal/store/**` cover entries 4 to 6.
- `compute` row, entry cell: add `internal/compute/basecheckapi`. Paths unchanged.

Version 2 (entries 12 to 26) changes three cells of `docs/design/modules.md`: the `pyside` row's entry cell adds `pyside/src/optdesk/refbars/api.py` and its tests cell `pyside/tests/contracts/test_refbars_api.py` (both landed by entry 26), and its purpose cell names the Yahoo daily bars through yfinance in a one-shot read-only command; the `store` entry cell adds `internal/store/recon`; the `compute` entry cell adds `internal/compute/reconapi`. The amended file is `modules.md` beside this design. No dependency edge is added.

Other paths are in rows that exist: `contracts/**`, `web/src/generated/**`, `internal/contracts/**`, `docs/contracts/page-schemas.md` are contracts (entry 3); store (entries 1, 4, 5, 6); compute (entries 2, 7, 8); pages `internal/pages/**` and `docs/contracts/pages.md` (entry 9); ot `cmd/**` and `docs/contracts/ot/**` (entry 10); web (entry 11). No dependency edge is added: pages already depends on contracts, store and compute.

## Interface

Contracts wording: new schema `page.base_data_page.v1` (`schema_version` 1; a new page, no bump of an existing id).

1. (interface) `internal/store/basefact/api.go`, declarations only: `type Fact struct { Dataset, Rule, Subject string; N, Bad int64; Day contracts.Day; Value string }` (one row per row-id, rule and subject; `Dataset` is the row id of the table in Page; `Subject` is empty for a rule of the whole row; `Day` and `Value` are set for the rule id `newest`; `rows` uses N), `type Result struct { Facts []Fact; Failed []string }` (`Failed` are the names of the view groups that failed or timed out: `bars`, `option`, `other`, `account`), `type Reader interface { Facts(ctx context.Context) (Result, error); Close() error }` (no day parameter: each view works out its own newest day and uses the New York date as Day; `Facts` slice non-nil; the error is returned only when all four views failed; the reader runs as `ot_ro_pages`), `type OpenFunc func(ctx context.Context, dsn string) (Reader, error)`. The reader selects the columns `(dataset text, rule text, subject text, n bigint, bad bigint, day date, value text)` from the four views, one query each, `rd.base_fact_bars_r1`, `rd.base_fact_option_r1`, `rd.base_fact_other_r1`, `rd.base_fact_account_r1`. Owner: store.
2. (interface) `internal/compute/basecheckapi/api.go`, declarations only (the declared data `Rules` and `Rows` included, so the package stays declarations-only: `api.go` is its one non-test file, modules.md package kinds): `type Level string` (`pass`, `warn`, `fail`, `unknown`); `type Fact` (the same fields as 1; the store type is the source type, this one is a copy because compute imports no store, and `convert.go` of entry 9 follows the store type; a field change is made in both and in `convert.go`); `type Cell struct { Day contracts.Day; Dataset string; Keys, Missing, Expired int }` (`Dataset` is the row id); `type Input struct { Facts []Fact; NotRead []string; Cells []Cell; Days map[string]contracts.Day; Sessions map[string][]contracts.Day }` (`NotRead` are the row ids of a view that failed: their rows are `unknown`; `Days` and `Sessions` by calendar id XNYS and XTKS, both always present); `type Check struct { Rule string; Level Level; N, Bad int64; Share string; Example string; WarnAt, FailAt string; ThresholdSource string }` (decimal text written by compute); `type Row struct { Dataset, Group string; Level Level; Newest, NewestValue, NewestSubject string; AgeDays, AgeSessions int; Checks []Check }`; `type Output struct { Level Level; Counts map[Level]int; Rows []Row }`; `type Func func(Input) Output`. Also in `api.go`: `type RuleInfo struct { Row, Rule, Source string }` (Source `sql`, `ledger`, `derived` or `note`), `type RowInfo struct { ID, Group, ViewGroup string; LedgerDatasets []string; Calendars map[string]string }` (`Calendars` maps a subject to its calendar id), `var Rules []RuleInfo` and `var Rows []RowInfo`: the rules of the Checks table and the 12 rows of the Page table, ids and marks and ledger datasets only, no threshold. The table is the single owner of the 12 rows and the rule marks; entries 7, 8, 9 and 10 read it and none keeps a second copy. Owner: compute.
3. (interface) `contracts/schemas/page.base_data_page.v1.json` with `web/src/generated/base_data_page.v1.ts`: `schema` const `base_data_page`, `schema_version` const 1; `run_id`, `built_at`, `as_of`, `last_sessions` {XNYS, XTKS}, `top` {level, text_zh, counts}, `groups[]` {name, rows[]}, row {dataset, label, label_zh, level, newest_value, newest_day, newest_subject, age_text, failed_summary, checks[], optional `cross_check`}, check {rule, label, hint_zh, level, n, bad, share, warn_at, fail_at, threshold_source, example}, `glossary[]`, `derived_note`, `repair_path` const `/data-health`, `gaps[]` {kind gap, reason enum `facts_unavailable` (all four views failed), `view_unavailable` (one or more), `coverage_unavailable`, text}, and the version 2 slot: optional `cross_check` on a row {pair_id, level, n, matched, diff, missing, tolerance_text, source_id, run_at, optional `reason` (enum `no_run`, `rate_limited`, `network`, `timeout`, `source_changed`; absent on a run of status `ok` or `partial` without a reason) and optional `reason_zh` (its Chinese line), items[] {key, ours, other, diff, kind, hint_zh}} and optional top-level `cross_checks[]` of the same object; version 1 never writes them. Text maximum 600. Owner: contracts.

4. (interface) `internal/sources/api.go` gains: `type BarPair struct { Symbol, Currency string }`, `type BarRequest struct { Pairs []BarPair; From, To contracts.Day }`, `type RefBar struct { Pair BarPair; Day contracts.Day; Close string }`, `type BarUnanswered struct { Pair BarPair; Code string }`, `type BarResult struct { Bars []RefBar; Unanswered []BarUnanswered }`, `type BarSource interface { DailyBars(ctx context.Context, req BarRequest) (BarResult, error) }`, `type BarConfig struct { Command []string; Key string; PerPair time.Duration }` (a source reads the fields it needs: yfinance the command and the timing, Tiingo the key), `type NewBarSourceFunc func(BarConfig) (BarSource, error)` and the codes `BarCodeRateLimited`, `BarCodeNoData`, `BarCodeNetwork`, `BarCodeTimeout`, `BarCodeSourceChanged`. Owner: sources.
5. (interface) `internal/compute/reconapi/api.go`: `type Kind string` (`match`, `diff`, `missing_ours`, `missing_other`, `skipped`); `type Value struct { Key, Amount string }`; `type Tolerance struct { Abs, Rel string }` (decimal text); `type Item struct { Key, Ours, Other, Diff, Reason string; Kind Kind }`; `type Input struct { Ours, Other []Value; Skip map[string]string; OtherMissingSkipped bool; Tol Tolerance }` (`Skip` maps a key to the reason it is not compared); `type Output struct { Items []Item; N, Matched, Diff, MissingOurs, MissingOther, Skipped int }`; `type Func func(Input) Output`; `type VerdictFunc func(pair string, o Output, pairsAsked, pairsCompared int) basecheckapi.Level` (pairs asked and pairs with at least one answer). And in `internal/compute/basecheckapi/api.go`: `type PairLevel struct { Dataset, Pair string; Level Level }` and the field `Pairs []PairLevel` of `Input`. Owner: compute.
6. (interface) `internal/store/recon/api.go`: the constant `DSNEnvRecon = "OT_PG_DSN_RECON"`, `LatestItemsMax = 20` (the items per run `Latest` returns) and `RunItemsMax = 500` (the items per run `InsertRun` accepts); `type Item struct { Key, Company, Ours, Other, Diff, Kind, Reason string }`; `type Run struct { PairID string; SessionDay contracts.Day; Status, Reason string; StartedAt, FinishedAt time.Time; N, Matched, Diff, MissingOurs, MissingOther, Skipped, PairsAsked, PairsCompared int; ToleranceText, SourceID string; Items []Item }` (`Status` is `ok`, `partial` or `failed`; `Reason` is empty or the code `rate_limited`, `network`, `timeout`, `source_changed`; the counts per kind are stored on the run, and `Items` holds only the items that are not `match`, at most `RunItemsMax` per run); `type Writer interface { InsertRun(ctx context.Context, r Run) (int64, error); Close() error }`; `type Reader interface { Latest(ctx context.Context) ([]Run, error); Close() error }` (newest finished run per pair and day, at most `LatestItemsMax` items each, ordered by severity diff, missing_ours, missing_other, skipped, then company and key); `OpenWriterFunc` and `OpenReaderFunc` over a dsn. Owner: store.
7. (interface) The JSON of the command of S1, declared in `pyside/src/optdesk/refbars/api.py` (typed dicts of the request and the answer, the fetcher protocol, the clock protocol) and landed by entry 26 with its contract test and one golden answer file, `pyside/tests/refbars/answer.json`. The command test (entry 17) and the adapter test of `internal/sources/yfinance` (entry 18, which reads that file by the relative path `../../../pyside/tests/refbars/answer.json`) read this one file; there is no second copy. Owner: pyside.
8. (interface) `contracts/schemas/page.base_data_page.v1.json` gains the optional property `skipped` (integer, at least 0) of `CrossCheck` and the optional `company` (string) of its items, with the generated type; `n` is the compared count (matched, diff and missing_ours), `missing` is missing_ours plus missing_other, M of "compared N of M" is `n` plus `skipped`. No version bump (the slot was optional and never written). Owner: contracts.

## Issues

Each entry is one module; none spans two. `(interface)` entries land first; the numbers name entries of this design. All entries: `Plan: P1`, `Design: docs/design/base-data-page.md#<n>`. Migration numbers 0056 to 0059 are free on origin/main `c16ebc95` (last is 0055) and no open pull request claims one. The nav entry of `docs/design/option-pages-r7.md` (its 13, pull request #2909) is merged at that head, so entry 11 has no order against it.

### 1 basefact-api (interface)

Module: store. Size: S. Order: first.
Files: `internal/store/basefact/api.go`, `internal/store/basefact/api_test.go`
Contracts: unchanged
The declarations of Interface 1 and a test of the contract from both sides (a fake reader and a caller).
Acceptance:
1. `api.go` declares `Fact`, `Result` (`Facts` and `Failed`), `Reader` (`Facts(ctx)` without a day, returning `Result`, and `Close`) and `OpenFunc` as Interface 1 gives them, no function body, imports only `context` and `internal/contracts`.
2. `api_test.go` has `TestFakeReaderServesCaller` (new), `TestFactsNonNilWithoutRows` (new) and `TestFailedNamesAreTheFourGroups` (new: the constants for `bars`, `option`, `other`, `account`).
3. The result matches docs/design/base-data-page.md#1.

### 2 basecheck-api (interface)

Module: compute. Size: S. Order: first.
Files: `internal/compute/basecheckapi/api.go`, `internal/compute/basecheckapi/api_test.go`, `internal/compute/basecheckapi/table_test.go`
Contracts: unchanged
The declarations of Interface 2 and the declared table of rules and rows.
Acceptance:
1. `api.go` declares `Level` with the four constants, `Fact`, `Cell`, `Input` (with `NotRead`, `Days` and `Sessions` by calendar id), `Check`, `Row`, `Output`, `Func`, no function body; the only variables with a value are `Rules` and `Rows` of item 2; imports only `internal/contracts`.
2. `api.go` also declares `RuleInfo`, `RowInfo`, and `Rules` and `Rows` with the rules (ids, source marks) and the 12 rows (ids, groups, view groups, ledger datasets, subject calendars) of the Checks and Page tables; no threshold number is in the file; the package has no non-test `.go` file other than `api.go`.
3. `api_test.go` has `TestFuncServesCaller` (new) and `TestLevelValues` (new: the four strings); `table_test.go` has `TestRuleIDsUnique` (new), `TestEveryRuleHasSource` (new), `TestRowsAreTheTwelveOfTheDesign` (new: ids and groups equal the Page table), `TestCheckedRulesAreNotTableConstraints` (new: none of the dropped rule ids is in `Rules`) and `TestEveryRowHasAViewGroup` (new).
4. The result matches docs/design/base-data-page.md#2.

### 3 base-data-contract (interface)

Module: contracts. Size: M. Order: first.
Files: `contracts/schemas/page.base_data_page.v1.json`, `web/src/generated/base_data_page.v1.ts`, `contracts/fixtures/page.base_data_page.v1.ok.json`, `contracts/fixtures/page.base_data_page.v1.gap.json`, `contracts/fixtures/MANIFEST.synthetic-v1.json`, `internal/contracts/base_data_page_v1_test.go`, `docs/contracts/page-schemas.md`
Contracts: page.base_data_page v1 (new)
The schema of Interface 3 including the optional version 2 slot (`cross_check` on a row and `cross_checks`), the generated type and two fixtures with invented numbers.
Acceptance:
1. The schema has the properties and the `required` list of Interface 3; `schema` and `schema_version` are const; `repair_path` is const `/data-health`; `gaps.reason` is the enum of three values `facts_unavailable`, `view_unavailable`, `coverage_unavailable`; `cross_check` and `cross_checks` are optional and not in `required`.
2. `web/src/generated/base_data_page.v1.ts` is generated from the schema; `git diff --name-only BASE` lists it with the schema. The names and texts named here are new.
3. The fixtures `ok` and `gap` are listed in `MANIFEST.synthetic-v1.json` with their sha256 as the other fixtures are; `gap` has no groups and one gap.
4. `base_data_page_v1_test.go` has `TestBaseDataPageOkValid` (new), `TestBaseDataPageGapValid` (new), `TestBaseDataPageRejectsBadLevel` (new), `TestBaseDataPageAcceptsCrossCheckSlot` (new: a row with a `cross_check` validates, one without it validates).
5. `page-schemas.md` lists `page.base_data_page.v1 | 1` in the table and one paragraph naming `docs/design/base-data-page.md#Interface`.
6. `git diff --name-only BASE -- contracts/schemas/carryover.sha256 internal/contracts/common_test.go` prints nothing.
7. The result matches docs/design/base-data-page.md#3.

### 4 basefact-reader

Module: store. Size: M. Order: after 1.
Files: `internal/store/basefactpg/reader.go`, `internal/store/basefactpg/reader_test.go`
Contracts: unchanged
The implementation of `basefact.Reader` over the four views, as the page role, each view on its own.
Acceptance:
1. `reader.go` declares `Open` of the type `basefact.OpenFunc` and a reader whose `Facts` runs one query per view (`rd.base_fact_bars_r1`, `rd.base_fact_option_r1`, `rd.base_fact_other_r1`, `rd.base_fact_account_r1`, columns `dataset, rule, subject, n, bad, day, value`, no parameter), each under its own context timeout of 10 seconds from one constant; a view that fails or times out adds its group name to `Result.Failed` and its facts are left out; the other views still give their facts; the error is returned only when all four fail; the slices are non-nil; a null `day` gives the empty `contracts.Day`; results are merged in dataset, rule and subject order.
2. `reader_test.go` has `TestFactsMergeFourViews` (new, with a fake row source), `TestFactsOrder` (new), `TestFactsNullDay` (new), `TestOneViewFailsOthersStay` (new: the failed group is in `Failed`, the other facts arrive), `TestOneViewTimesOut` (new, a fake source that waits for the context), `TestAllViewsFailReturnsError` (new); each uses `t.Parallel()` and no sleep.
3. The result matches docs/design/base-data-page.md#4.

### 5 basefact-migration

Module: store. Size: L. Order: after 1.
Files: `migrations/0056_base_fact_bars.sql`, `migrations/0057_base_fact_option.sql`, `migrations/0058_base_fact_other.sql`, `migrations/0059_base_fact_account.sql`
Contracts: unchanged
Four migrations, one view each, landed by this one issue so the owner has one deploy hold and the reader never meets a missing view: `rd.base_fact_bars_r1` (0056), `rd.base_fact_option_r1` (0057), `rd.base_fact_other_r1` (0058), `rd.base_fact_account_r1` (0059), with the rules of the Checks table, the grants and the Down of each. The split is decided here, so no file is expected to reach 600 lines.
Acceptance:
1. Each migration creates its one view with the columns `dataset, rule, subject, n, bad, day, value` owned by `ot_owner`, grants SELECT on it to `ot_ro_pages`, has a Down that revokes and drops it, and changes no table.
2. Each SQL block carries the comment line `-- rule <row> <rule>` for the rules marked `sql` in the Checks table, including `rows` and `newest` of each row, and none for the rules marked `derived` or `ledger` (`age`, `capture_age`, `flex_age`, `coverage_gap`, `coverage_latest`); the block of `flex_section_absent` has the line `-- note flex flex_section_absent`.
3. The bars view reads `rd.price_bar_r1` rows of basis trades of the last 45 calendar days and joins `master.instrument` for `symbol`; it has no rule `bar_gap`; the option view picks per `instrument_id` the newest `received_at` among the rows of status 1 of the newest `session_day` of each currency and classifies a contract without such a row by its newest row (Decision 7); `no_netliq` reads the tag `NetLiquidation` of `rd.account_value_r1`.
4. `git diff --name-only BASE` lists exactly the four files named in Files and no other file.
5. The result matches docs/design/base-data-page.md#5.

### 6 basefact-views-pg

Module: store. Size: M. Order: after 4 and 5.
Files: `internal/store/basefactpg/bars_pg_test.go`, `internal/store/basefactpg/option_pg_test.go`, `internal/store/basefactpg/other_pg_test.go`, `internal/store/basefactpg/account_pg_test.go`, `internal/store/basefactpg/reader_pg_test.go`
Contracts: unchanged
The pg tests of the four views and of the reader against them; they run in GitHub CI only.
Acceptance:
1. Each file carries the build tag `pg`, uses `t.Parallel()`, its own database and invented rows.
2. `bars_pg_test.go` has `TestBarsViewOhlcOrder` (new), `TestBarsViewJumpAcrossStoredDays` (new), `TestBarsViewNewestPerCurrency` (new); `option_pg_test.go` has `TestOptionViewCountsOneRowPerContract` (new: two captures of one contract count once), `TestOptionViewPicksStatusOne` (new: a status-2 row newer than a status-1 row does not replace it), `TestOptionViewEmptyOnlyWithoutStatusOne` (new), `TestOptionViewNonLiveShare` (new), `TestOptionViewCrossedShare` (new).
3. `other_pg_test.go` has `TestOtherViewSeriesAge` (new), `TestOtherViewEarningsPast` (new), `TestOtherViewMacroExhausted` (new); `account_pg_test.go` has `TestAccountViewNoNetLiq` (new), `TestAccountViewPositionExpired` (new), `TestAccountViewFlexSectionAbsentIsNote` (new).
4. `reader_pg_test.go` has `TestReaderReadsTheFourRealViews` (new: after the migration of entry 5 `Facts` returns facts of every group and an empty `Failed`).
5. The result matches docs/design/base-data-page.md#6.

### 7 basecheck-rules

Module: compute. Size: S. Order: after 2.
Files: `internal/compute/basecheck/rules.go`, `internal/compute/basecheck/rules_test.go`
Contracts: unchanged
The thresholds of the rules, keyed by the ids of `basecheckapi.Rules`: warn at, fail at (a fail threshold may be none) and the threshold source `def` or `trial`, as the Checks table gives them.
Acceptance:
1. `rules.go` declares one table of thresholds keyed by `<row>.<rule>`; every rule of `basecheckapi.Rules` marked `sql`, `ledger` or `derived` has an entry (an `age` rule has its limit) and a rule marked `note` has none; no threshold number is written anywhere else in the package; `bars.bar_jump` has no fail threshold.
2. `rules_test.go` has `TestEveryRuleHasThresholds` (new), `TestNoteRuleHasNoThreshold` (new), `TestBarJumpHasNoFailThreshold` (new), `TestEveryThresholdHasSource` (new: `def` or `trial`).
3. The result matches docs/design/base-data-page.md#7.

### 8 basecheck-eval

Module: compute. Size: M. Order: after 7.
Files: `internal/compute/basecheck/eval.go`, `internal/compute/basecheck/age.go`, `internal/compute/basecheck/eval_test.go`, `internal/compute/basecheck/age_test.go`, `internal/compute/basecheck/testdata/facts.json`
Contracts: unchanged
`Evaluate` of the type `basecheckapi.Func`: levels, ages per subject calendar, the top line.
Acceptance:
1. `Evaluate` gives each check the level by the thresholds (any, share; a rule without a fail threshold is at most warn), `unknown` for a rule without a fact, a row listed in `Input.NotRead` level `unknown` with the one check `not_read`, a row the worst level in the order fail, unknown, warn, pass, a row whose `rows` N is 0 fail, a `note` rule never in the level, and `Output.Level` the worst row; shares are decimal text made with `math/big`.
2. `age.go` counts the sessions of `Input.Sessions[calendar]` between the newest day of a subject and `Input.Days[calendar]`, the calendar of a subject being `Calendars[subject]` of its row in `basecheckapi.Rows`; 0 pass, 1 warn, 2 or more fail; a newest day after `Days` gives 0; a row with several subjects takes the worst.
3. `git grep -n 'time.Now\|net/http\|database/sql' -- internal/compute/basecheck` prints nothing.
4. `eval_test.go` has `TestWorstLevelWins` (new), `TestNoRowsIsFail` (new), `TestMissingFactIsUnknown` (new), `TestTopLinePassOnlyWhenAllPass` (new), `TestShareThresholds` (new), `TestBarJumpNeverFails` (new), `TestNoteDoesNotDriveVerdict` (new), `TestNotReadRowIsUnknown` (new); `age_test.go` has `TestAgeInSessions` (new), `TestAgeAcrossHoliday` (new), `TestJpAgeUsesJpCalendar` (new: JP newest 2026-11-18 with `Days` XTKS 2026-11-18 and XNYS 2026-11-17 gives age 0), `TestAgeNeverNegative` (new: a newest day after the last closed session gives 0); the testdata holds invented numbers.
5. The result matches docs/design/base-data-page.md#8.

### 9 base-data-page

Module: pages. Size: L. Order: after 1, 2, 3.
Files: `internal/pages/base_data_page/page.go`, `internal/pages/base_data_page/builder.go`, `internal/pages/base_data_page/rows.go`, `internal/pages/base_data_page/convert.go`, `internal/pages/base_data_page/text.go`, `internal/pages/base_data_page/zh_texts.go`, `internal/pages/base_data_page/purity_test.go`, `internal/pages/base_data_page/builder_test.go`, `internal/pages/base_data_page/rows_test.go`, `internal/pages/base_data_page/convert_test.go`, `internal/pages/base_data_page/zh_texts_test.go`, `internal/pages/base_data_page/testdata/ok.json`, `internal/pages/base_data_page/testdata/gap.json`, `docs/contracts/pages.md`
Contracts: unchanged
The builder: reads `basefact.Reader` and `coverage.Reader`, converts the facts, sums the ledger cells per row, derives `Days` and `Sessions`, maps `Result.Failed` to the row ids of those view groups, calls the injected `basecheckapi.Func`, lays out groups, rows, checks and glossary with Chinese texts from `zh_texts.go`.
Acceptance:
1. `Build` takes an `Input` with `Now`, `Facts basefact.Reader`, `Cov coverage.Reader` and `Eval basecheckapi.Func`; the page package declares its own constants for the calendar ids XNYS and XTKS and their zones; `Days` of a calendar is its newest session day before the current date in its zone, from `Cov.Sessions`, and both calendars are always in `Days` and `Sessions`; a failed view group gives its rows in `NotRead` and one gap `view_unavailable` that names the group in words, the other rows stay; all four views failing gives the gap `facts_unavailable` and no rows; a failed `Cov` read gives the gap `coverage_unavailable` with the other rules still evaluated.
2. `convert.go` converts `basefact.Fact` to `basecheckapi.Fact` field by field (`Dataset, Rule, Subject, N, Bad, Day, Value`) in one function, and sums the `coverage.Cell` values of the ledger datasets of a row into one `basecheckapi.Cell` named by the row id, using the table of ledger datasets of the Page section; `convert_test.go` has `TestFactConversionKeepsEveryField` (new: every field set to a distinct value arrives unchanged) and `TestCellsSumLedgerDatasetsPerRow` (new: `obs.option_eod`, `obs.option_eod_jp` and `obs.option_eod_index` sum into the row `chains`).
3. `zh_texts.go` holds string constants only: one hint per rule of `basecheckapi.Rules` (id `<row>.<rule>`) and one per row of `basecheckapi.Rows`, the glossary and the `derived_note`; `Hint(id string) (string, bool)` in `builder.go` is the exported lookup of them; labels in `text.go` are English; the hint of `bar_jump` says that a split looks like this. The builder takes the ledger datasets of a row from `basecheckapi.Rows` and keeps no table of its own.
4. `zh_texts_test.go` has `TestEveryRuleAndRowHasChineseHint` (new: each id of `basecheckapi.Rules` and each id of `basecheckapi.Rows` has a text with a CJK rune through `Hint`) and `TestGlossaryExplainsEveryTerm` (new).
5. `purity_test.go` is as the other pages (no clock, no database); `builder_test.go` has `TestBuildGroupsAreFour` (new), `TestBuildPayloadMatchesFixture` (new: the output of a fake `Eval` equals `testdata/ok.json` and validates against the schema of entry 3), `TestBuildOneViewFailsOthersStay` (new), `TestBuildAllViewsFailGap` (new), `TestBuildGivesBothCalendars` (new), `TestBuildStepTwoFromStepOnePayload` (new: the check list of a row is built from the row payload of the table).
6. `docs/contracts/pages.md` gains one line for `base_data_page.Build(ctx, in Input) ([]byte, error)` with the schema name, as the line of `data_health_page.Build` does.
7. The result matches docs/design/base-data-page.md#9.

### 10 base-data-wire

Module: ot. Size: M. Order: after 4, 5, 8 and 9.
Files: `cmd/ot/base_data_site.go`, `cmd/ot/base_data_site_test.go`, `cmd/ot/base_data_ids_test.go`, `cmd/ot/pagereg_test.go`, `cmd/ot/pagelist_test.go`, `cmd/ot/valuation_site_test.go`, `docs/contracts/ot/pages/base_data_page.md`
Contracts: unchanged
The route `base-data` (name `base_data_page`, order 135): openers for `basefactpg.Open` and the coverage reader, `basecheck.Evaluate` injected; and the tests that ids agree.
Acceptance:
1. `base_data_site.go` registers the page family with `registerPage` (route `base-data`, name `base_data_page`, order 135) and puts the openers in `extDeps` and `siteOpeners` as `data_health_site.go` does; no logic beyond wiring.
2. `pagereg_test.go` lists `{"base-data", "base_data_page", 135}` and the name in the registered names; both `known pages:` lists of `pagelist_test.go` name `base_data_page`; `valuation_site_test.go` keeps its assertion that the valuation pages and the two company pages are the last names true with the new page in order 135.
3. `base_data_ids_test.go` has `TestSqlRuleCommentsEqualRulesTable` (new: the `-- rule <row> <rule>` lines of `migrations/0056_base_fact_bars.sql`, `0057_base_fact_option.sql`, `0058_base_fact_other.sql` and `0059_base_fact_account.sql` equal the rules marked `sql` of `basecheckapi.Rules`, both ways), `TestEveryRuleAndRowHasZhText` (new: every id of `basecheckapi.Rules` and `basecheckapi.Rows` has a hint through `base_data_page.Hint`) and `TestEveryLedgerDatasetIsInARowOrNamedOut` (new: every dataset id of `dataHealthManifests` is in the `LedgerDatasets` of one row of `basecheckapi.Rows` or in the fixed list of derived and owner-input datasets of the Page section).
4. `base_data_site_test.go` has `TestBaseDataRouteServesGetAndHead` (new), `TestBaseDataRouteWithoutDSNIsGap` (new).
5. `docs/contracts/ot/pages/base_data_page.md` states route, order, the three gaps and that the `/data-health` route stays without a redirect.
6. The result matches docs/design/base-data-page.md#10.

### 11 base-data-web

Module: web. Size: M. Order: after 3.
Files: `web/src/pages/base_data/BaseDataPage.tsx`, `web/src/pages/base_data/DatasetTable.tsx`, `web/src/pages/base_data/CheckList.tsx`, `web/src/types/base_data_page.ts`, `web/src/shell/TopNav.tsx`, `web/test/shell/TopNav.test.tsx`, `web/test/pages/base_data/base_data.test.tsx`, `web/test/pages/base_data/fixtures/ok.ts`, `web/test/pages/base_data/fixtures/gap.ts`
Contracts: unchanged
The page: top verdict, one table of rows by group, an opened row shows the checks, glossary and thresholds under it, gaps in words; menu entry `Base data` under System before `Data health`, written against the System group of `c16ebc95` (Settings, Job console, Data health, Option data).
UI:
- 按 frontend-stack：用 shadcn `Alert` 与 `Badge` 做 顶部结论与每行的判定色（pass、warn、fail、unknown）。
- 按 frontend-stack：用 @tanstack/react-table 与 shadcn `Table` 做 数据集表（按判定排序，展开行）。
- 按 frontend-stack：用 shadcn `Accordion` 做 每个数据集的检查明细。
- 按 frontend-stack：用 Tailwind `text-muted-foreground` 做 中文说明的次要文字。
Acceptance:
1. `BaseDataPage.tsx` is registered with `registration = { schema: 'base_data_page', page: page(validate, View) }`; `types/base_data_page.ts` follows `types/data_health_page.ts` with `SUPPORTED_VERSIONS = [1]`.
2. The table has the columns Dataset, Verdict, Newest value, Date, Age, Failed checks, in the order of the payload groups; every text shown is a payload field; `git grep -n 'reduce\|parseFloat\|Number(' -- web/src/pages/base_data` prints nothing.
3. `TopNav.tsx` lists `{ label: 'Base data', path: '/base-data' }` before `Data health` in System; `TopNav.test.tsx` lists `['Base data', '/base-data']` in the System tuple and `'Base data'` in the label list that holds `'Data health'`.
4. `base_data.test.tsx` has `renders one row per dataset with its verdict` (new), `opens a row to show its checks` (new), `renders a view gap and the other rows` (new), `renders the gap payload` (new).
5. The screenshot of the page against the sketch of the design is taken by the `ui-shots` check (UI rule).
6. The result matches docs/design/base-data-page.md#11.


### Version 2 entries (12 to 26, R6 on yfinance)

Cut on 2026-11-18 after the owner's word (ask optdesk-4965), in the second round after `adversarial-raw.md`. Entry 12 is written by the dispatcher window and lands first; entries 13 to 16 are interface entries and land first too; entry 26 (interface of the command) follows 12 and lands before 17 and 18; each later entry names its order. Migration 0060 is free on origin/main `c517086c` (last is 0059); job order 230 is free. Coverage: S1 entries 26, 17, 18; S2 entries 12 and 25; S3 entries 13, 18, 23; S4 entries 19, 21; S5 entries 19, 21, 22; S6 entries 17, 18 (batching and the context deadline in 18 alone), 21 (one call); S7 entries 14, 19, 21, 22; the cross-check slot's `skipped` count entry 16; R6 tolerance and verdict entry 19; the page and the reader entries 22, 23; the web column entry 24. R1 to R5 and R7 to R9 and their jobs `recon_flex`, `recon_live`, `recon_internal`: no entry, text only (the owner gave only the source word).

### 12 yfinance-dependency (window)

Module: pyside. Size: S. Order: first. Written by the dispatcher window.
Files: `pyside/pyproject.toml`, `pyside/uv.lock`
Contracts: unchanged
Written by the dispatcher window, not by a worker (a worker has no network to resolve the lock). New direct dependency yfinance 1.8.0, Apache-2.0, an outside library that touches the network; transitive pandas, numpy, curl_cffi, requests, lxml, beautifulsoup4, peewee, platformdirs, pytz, websockets (Version 2, Second source for R6). The owner's word is of 2026-11-18, ask optdesk-4965.
Acceptance:
1. `pyproject.toml` lists `yfinance==1.8.0` in `dependencies` and no other new direct dependency; `uv.lock` holds the package `yfinance` at version `1.8.0`. The names and texts named here are new.
2. `pyproject.toml` has `[[tool.mypy.overrides]]` with `module = ["yfinance", "pandas"]` and `ignore_missing_imports = true`, and changes no import-linter contract. The names and texts named here are new.
3. The result matches docs/design/base-data-page.md#12.

### 13 barsource-api (interface)

Module: sources. Size: S. Order: first.
Files: `internal/sources/api.go`, `internal/sources/api_test.go`, `docs/contracts/sources.md`
Contracts: unchanged
`internal/sources/api.go` gains the declarations of Interface 4 and a test of the contract from both sides. Editing the interface file is an interface change; this lands before the adapter and the job.
Acceptance:
1. `api.go` gains `BarPair`, `BarRequest`, `RefBar`, `BarUnanswered`, `BarResult`, `BarSource` (the one method `DailyBars(ctx, BarRequest) (BarResult, error)`), `BarConfig`, `NewBarSourceFunc` and the five code constants `BarCodeRateLimited`, `BarCodeNoData`, `BarCodeNetwork`, `BarCodeTimeout`, `BarCodeSourceChanged`; no function body; `git diff` of the file shows added lines only. The names and texts named here are new.
2. The doc comment of `BarSource` says that one call makes at most the requests its service allows, never retries, returns `contracts.TypedError` values only with no key or address in a text, and returns an error only when the whole source failed (a pair it cannot answer goes in the `Unanswered` list of the result), may split one call into batches and stops at the deadline of its context, and then returns the partial result with the unanswered pairs and their code. The names and texts named here are new.
3. `api_test.go` has `TestFakeBarSourceServesCaller` (new: a fake source and a caller) and `TestBarCodesAreFive` (new). The names and texts named here are new.
4. `docs/contracts/sources.md` lists `BarSource` with one paragraph naming the design section Second source for R6. The names and texts named here are new.
5. The result matches docs/design/base-data-page.md#13.

### 14 recon-api (interface)

Module: compute. Size: S. Order: after 2.
Files: `internal/compute/reconapi/api.go`, `internal/compute/reconapi/api_test.go`, `internal/compute/basecheckapi/api.go`, `internal/compute/basecheckapi/api_test.go`
Contracts: unchanged
The declarations of Interface 5: the compare function type, its data and the verdict function type, and the pair levels on `basecheckapi.Input`. Editing the interface file of basecheckapi is an interface change; it lands before entry 19.
Acceptance:
1. `reconapi/api.go` declares `Kind` with the five constants, `Value`, `Tolerance`, `Item`, `Input` (with `Skip` and `OtherMissingSkipped`), `Output`, `Func` and `VerdictFunc`, no function body; the package has no non-test `.go` file other than `api.go`. The names and texts named here are new.
2. `basecheckapi/api.go` gains `PairLevel` and the field `Pairs` of `Input`, and no other line of the file changes (`git diff` shows added lines only). The names and texts named here are new.
3. `reconapi/api_test.go` has `TestFuncServesCaller` (new) and `TestKindValues` (new: the five strings); `basecheckapi/api_test.go` has `TestInputCarriesPairs` (new). The names and texts named here are new.
4. The result matches docs/design/base-data-page.md#14.

### 15 recon-store-api (interface)

Module: store. Size: S. Order: first.
Files: `internal/store/recon/api.go`, `internal/store/recon/api_test.go`
Contracts: unchanged
The declarations of Interface 6: the writer of a run with its items and the reader of the newest run per pair.
Acceptance:
1. `api.go` declares the constant `DSNEnvRecon`, `LatestItemsMax` (20), `RunItemsMax` (500), `Item` (with `Company`), `Run` (with `Reason`, `Skipped`, `PairsAsked` and `PairsCompared`), `Writer` (`InsertRun`, `Close`), `Reader` (`Latest`, `Close`), `OpenWriterFunc` and `OpenReaderFunc` as Interface 6 gives them, no function body, imports only `context`, `time` and `internal/contracts`. The names and texts named here are new.
2. `api_test.go` has `TestFakeWriterAndReader` (new), `TestLatestItemsMaxIsTwenty` (new: `LatestItemsMax` and `RunItemsMax` have the values 20 and 500) and `TestDSNEnvName` (new: `OT_PG_DSN_RECON`). The names and texts named here are new.
3. The result matches docs/design/base-data-page.md#15.

### 16 crosscheck-schema (interface)

Module: contracts. Size: S. Order: after 3.
Files: `contracts/schemas/page.base_data_page.v1.json`, `web/src/generated/base_data_page.v1.ts`, `contracts/fixtures/page.base_data_page.v1.cross.json`, `contracts/fixtures/MANIFEST.synthetic-v1.json`, `internal/contracts/base_data_page_v1_test.go`
Contracts: page.base_data_page v1 additive
Interface 8: the optional integer `skipped` on `CrossCheck` and the optional string `company` on its items, with the generated type and a fixture that carries a `cross_check`.
Acceptance:
1. The schema's `CrossCheck` has the properties `skipped` (integer, minimum 0), `reason` (string, enum `no_run`, `rate_limited`, `network`, `timeout`, `source_changed`) and `reason_zh` (string), its item definition has the property `company` (string), none is in a `required` list, and no other property changes. The names and texts named here are new.
2. `web/src/generated/base_data_page.v1.ts` is generated from the schema; `git diff --name-only BASE` lists it with the schema. The names and texts named here are new.
3. The fixture `page.base_data_page.v1.cross.json` is listed in `MANIFEST.synthetic-v1.json` with its sha256 as the other fixtures are; it has one bars row with a `cross_check` of pair `R6` with `n`, `matched`, `diff`, `missing`, `skipped`; the manifest lines of the existing fixtures are unchanged. The names and texts named here are new.
4. `base_data_page_v1_test.go` has `TestCrossFixtureValid` (new) and `TestSkippedNotNegative` (new). The names and texts named here are new.
5. The result matches docs/design/base-data-page.md#16.

### 17 refbars-command

Module: pyside. Size: M. Order: after 12 and 26.
Files: `pyside/pyproject.toml`, `pyside/src/optdesk/refbars/__main__.py`, `pyside/src/optdesk/refbars/cli.py`, `pyside/src/optdesk/refbars/fetch.py`, `pyside/src/optdesk/refbars/symbols.py`, `pyside/tests/refbars/test_cli.py`, `pyside/tests/refbars/test_fetch.py`, `pyside/tests/refbars/test_symbols.py`
Contracts: unchanged
The one-shot command of S1: JSON in on stdin, JSON out on stdout, the yfinance call, the symbol mapping, the rate stop and the deadline. The dependency and the lock are entry 12's; this entry edits `pyproject.toml` only for the import-linter lines. The package files `__init__.py` and `api.py` and the golden `pyside/tests/refbars/answer.json` are entry 26's; this entry imports and reads them.
Notes for the worker: a name of `yfinance` that the 1.8.0 source spells otherwise is used as spelled there and named in the pull request description.
Acceptance:
1. `fetch.py` sets `yfinance.config.debug.hide_exceptions` to `False` before it calls `history`, calls `history` with `interval="1d"`, `auto_adjust=False`, `actions=False` and `timeout=10`, reads `Close` and never `Adj Close` or `Stock Splits`, writes each close as decimal text rounded to 4 places with `Decimal`, and maps `YFRateLimitError` to `rate_limited`, an empty frame or `YFPricesMissingError` to `no_data`, a connection error to `network`, a call over its timeout to `timeout` and an unexpected column set to `source_changed`. The names and texts named here are new.
2. `symbols.py` maps (symbol, currency) to the Yahoo symbol: USD keeps the symbol with a dot changed to a dash, JPY appends `.T`; any other currency is the code `no_data` for that pair; the session day is the date of the index in America/New_York for USD and Asia/Tokyo for JPY. The names and texts named here are new.
3. `cli.py` reads the request from stdin, refuses more than 10 pairs with exit 2, calls yfinance one pair at a time with a gap of 1 second from one constant taken through an injected clock, stops at the first `rate_limited` and answers the pairs not reached `rate_limited`, stops at `--deadline` seconds and answers the pairs not reached `timeout` keeping the bars it has, never retries, points the yfinance time-zone cache to a temporary directory it removes at exit, writes only the one JSON answer to stdout and exits 0 when it wrote it; `git grep -n 'time.sleep\|import requests' -- pyside/src/optdesk/refbars` prints nothing. The names and texts named here are new.
4. `pyproject.toml` lists `optdesk.refbars` in the `modules` of the independence contract of calendar and ibkr and in the `source_modules` of the contract on the generated package, and no dependency line changes; the module imports `optdesk.refbars.api` and edits no line of it; it imports no module of `optdesk.ibkr`, opens no database and no socket. The names and texts named here are new.
5. `test_fetch.py` has `test_auto_adjust_false_and_close_read` (new), `test_decimal_text_has_no_float_artefact` (new), `test_rate_limit_stops_batch` (new), `test_empty_frame_is_no_data_not_empty_bars` (new) and `test_hide_exceptions_restored_after_test` (new), all with a fake yfinance object and a fake clock; `test_symbols.py` has `test_us_dot_class_share_gets_dash` (new), `test_tokyo_gets_dot_t` (new) and `test_other_currency_is_no_data` (new); `test_cli.py` has `test_answer_matches_golden` (new, reading `pyside/tests/refbars/answer.json` of entry 26), `test_deadline_answers_rest_timeout_keeping_bars` (new) and `test_over_ten_pairs_exit_2` (new); all with `pytest -n auto` and no sleep. The names and texts named here are new.
6. The result matches docs/design/base-data-page.md#17.

### 18 yfinance-adapter

Module: sources. Size: M. Order: after 13 and 26.
Files: `internal/sources/yfinance/source.go`, `internal/sources/yfinance/command.go`, `internal/sources/yfinance/doc.go`, `internal/sources/yfinance/source_test.go`, `internal/sources/yfinance/command_test.go`
Contracts: unchanged
The adapter of `sources.BarSource` that runs the command of S1 (S3), copying the shape of `internal/clock/calendarcli`. No Python and no Yahoo address is in Go.
Acceptance:
1. `command.go` declares `CommandFunc` (argv and stdin in; stdout and exit code out) and `Exec`, the real one with `os/exec`, a context deadline and stdin and stdout; `source.go` declares `New` of the type `sources.NewBarSourceFunc` whose source is the only batching in the system (S6): one `DailyBars` call asks in batches of 10 pairs, one command run per batch, each bounded by the smaller of 15 seconds per pair plus 15 and the time left of the context, with `--deadline` of that bound minus 10; it keeps the answers of earlier batches when a later one fails, stops after the first batch with a `rate_limited` pair and marks the pairs not asked `rate_limited`, and when the time left of the context is 10 seconds or less before a batch it asks no more and marks the pairs not asked `timeout`, returning the partial result with each reason in `Unanswered`. The names and texts named here are new.
2. Each result with an `error_code` becomes one `BarUnanswered` with that code; each bar becomes a `RefBar` with the close text unchanged (no float is made); a nonzero exit, unreadable JSON or a batch killed at its timeout makes that batch `timeout` or `source_changed` for its pairs, and an error is returned only when no batch gave an answer; no error text holds a path or an address. The names and texts named here are new.
3. `git grep -n 'net/http\|yahoo' -- internal/sources/yfinance ':!internal/sources/yfinance/testdata'` prints nothing. The names and texts named here are new.
4. `source_test.go` has `TestBatchesOfTen` (new), `TestEarlierBatchesKept` (new), `TestRateLimitStopsBatches` (new), `TestErrorCodeBecomesUnanswered` (new), `TestStopsAtContextDeadline` (new, a fake clock; the pairs not asked are `timeout`), `TestAnswerMatchesGolden` (new, reading `pyside/tests/refbars/answer.json` of entry 26 by the relative path `../../../pyside/tests/refbars/answer.json`) and `TestConformsToBarSource` (new); `command_test.go` has `TestExecStdinStdout` (new, a fake child through `os.Args[0]` in `t.TempDir()`) and `TestExecDeadlineKillsChild` (new, a context cancelled by a fake clock); each uses `t.Parallel()` and no sleep. The names and texts named here are new.
5. The result matches docs/design/base-data-page.md#18.

### 19 recon-compare

Module: compute. Size: M. Order: after 8, 14.
Files: `internal/compute/recon/compare.go`, `internal/compute/recon/verdict.go`, `internal/compute/recon/rules.go`, `internal/compute/recon/compare_test.go`, `internal/compute/recon/verdict_test.go`, `internal/compute/recon/testdata/pairs.json`, `internal/compute/basecheck/eval.go`, `internal/compute/basecheck/eval_test.go`
Contracts: unchanged
The pure compare function of the Version 2 section Compare logic as a `reconapi.Func`, the tolerance and verdict table as data (`rules.go`), `Verdict` as a `reconapi.VerdictFunc` (S5), and the fold of the pair levels into the row level in `Evaluate` (S7).
Acceptance:
1. `compare.go` declares `Compare` of the type `reconapi.Func`: keys on both sides give `match` inside the tolerance or `diff` with the difference as decimal text made with `math/big`; a key in `Input.Skip` gives `skipped` with its reason and is not counted in N; a key only on the `Ours` side gives `missing_other`, or `skipped` with reason `source_lacks` when `OtherMissingSkipped` is true; a key only on the `Other` side gives `missing_ours`; items are in key order. The names and texts named here are new.
2. `rules.go` holds the tolerances of R1 to R9 as data and the verdict shares of R6 (0.001 warn, 0.01 fail, half of the asked pairs) and `OtherMissingSkipped` true for R6 only; `verdict.go` declares `Verdict` of the type `reconapi.VerdictFunc`: for R6 compared N of 0 is `unknown`, the share of `Diff` plus `MissingOurs` over N decides `pass`, `warn`, `fail`, fewer than half of `pairsAsked` compared (`pairsCompared`) caps the level at `warn`, and any pair of R1 to R5 with a `Diff` or a missing count above 0 is `fail`. The names and texts named here are new.
3. `eval.go` gives a row the worst of its check levels and the levels of the `Input.Pairs` whose `Dataset` is the row, in the order fail, unknown, warn, pass, and adds no check to the row; a row without a pair is unchanged (`eval_test.go` has `TestPairLevelFoldsIntoRow` (new), `TestRowWithoutPairUnchanged` (new) and `TestUnknownPairMakesRowUnknown` (new)). The names and texts named here are new.
4. `git grep -n 'time.Now\|net/http\|database/sql\|float64' -- internal/compute/recon` prints nothing. The names and texts named here are new.
5. `compare_test.go` has `TestMatchInsideTolerance` (new), `TestDiffOutsideTolerance` (new), `TestSkipKeysAreNotCounted` (new), `TestOursOnlyKeyIsSkippedForR6` (new), `TestOursOnlyMissingOtherR1` (new), `TestOtherOnlyKeyIsMissingOurs` (new) and `TestPairsGolden` (new, reading `testdata/pairs.json`: one R6 case with a diff, an unanswered pair, a day the source lacks and a day our store lacks); `verdict_test.go` has `TestR6NothingComparedIsUnknown` (new), `TestR6MissingOursCountsAsBad` (new), `TestR6HalfAskedCapsAtWarn` (new) and `TestR1AnyDiffFails` (new). The names and texts named here are new.
6. The result matches docs/design/base-data-page.md#19.

### 20 recon-store

Module: store. Size: L. Order: after 15.
Files: `migrations/0060_recon.sql`, `internal/store/setup/setup.go`, `internal/store/setup/setup_test.go`, `internal/store/reconpg/writer.go`, `internal/store/reconpg/reader.go`, `internal/store/reconpg/writer_test.go`, `internal/store/reconpg/recon_pg_test.go`
Contracts: unchanged
Migration `0060_recon.sql` with the two tables and the view of the Storage paragraph, the role `ot_w_recon` with the line `OT_PG_DSN_RECON`, and the implementation of `reconstore.Writer` and `reconstore.Reader`. The window runs setup pg and migrate after the merge (owner rule 2026-11-09).
Acceptance:
1. `0060_recon.sql` creates `recon.run (run_id, pair_id, session_day, started_at, finished_at, status, reason, n, matched, diff, missing_ours, missing_other, skipped, pairs_asked, pairs_compared, tolerance_text, source_id)` and `recon.item (run_id, key, company, ours, other, diff, kind, reason)` (no `match` item is stored) insert-only (no UPDATE or DELETE grant), the view `rd.recon_latest_r1` with the newest finished run per (pair_id, session_day), SELECT on the view for `ot_ro_pages`, and a Down that drops them. The names and texts named here are new.
2. `setup.go` creates the role `ot_w_recon` with INSERT and SELECT on both tables and writes the line `OT_PG_DSN_RECON` (`reconstore.DSNEnvRecon`) to the service secrets as `ot setup pg` writes the others; `setup_test.go` has `TestReconRoleAndDsnLine` (new). The names and texts named here are new.
3. `writer.go` declares `OpenWriter` of the type `reconstore.OpenWriterFunc`; `InsertRun` inserts the run and its items in one transaction and returns the new `run_id`, a second call for the same pair and day is a new run, never an update; `reader.go` declares `OpenReader` of the type `reconstore.OpenReaderFunc`; `Latest` reads `rd.recon_latest_r1` as the page role with at most `reconstore.LatestItemsMax` items per run in the severity order of Interface 6, slices non-nil; `InsertRun` refuses more than `reconstore.RunItemsMax` items and any `match` item. The names and texts named here are new.
4. `writer_test.go` has `TestInsertRunUsesOneTransaction` (new, fake executor); `recon_pg_test.go` carries the build tag `pg` and has `TestRerunNewRunLatestNewest` (new) and `TestItemsCapAtTwenty` (new) and `TestItemsOrderedBySeverity` (new). The names and texts named here are new.
5. The result matches docs/design/base-data-page.md#20.

### 21 recon-bars-job

Module: jobs. Size: L. Order: after 13, 14, 15 and 19.
Files: `internal/jobs/reconbars/job.go`, `internal/jobs/reconbars/job.toml`, `internal/jobs/reconbars/types.go`, `internal/jobs/reconbars/manifest.go`, `internal/jobs/reconbars/run.go`, `internal/jobs/reconbars/doc.go`, `internal/jobs/reconbars/manifest_test.go`, `internal/jobs/reconbars/run_test.go`, `internal/jobs/reconbars/testdata/bars_ours.json`, `internal/jobs/reconbars/testdata/source_answer.json`, `docs/contracts/jobs/recon_bars.md`
Contracts: unchanged
The job of R6: it reads our winner rows of the last 5 sessions for the watch list, asks the `BarSource` for the same pairs, compares and stores one run for the day. `types.go` declares its constructor type in its package (AGENTS.md): the source, `store.InstrumentResolver`, `bar.PriceBarReader`, `reconstore.Writer`, the compare function `reconapi.Func`, `[]jobs.WatchEntry` and the clock, as `barsrefresh.NewBarsRefreshFunc` does.
Acceptance:
1. `types.go` declares `NewReconBarsFunc(sources.BarSource, store.InstrumentResolver, bar.PriceBarReader, reconstore.Writer, reconapi.Func, []jobs.WatchEntry, clock.Clock) (contracts.Job, error)`; `job.toml` has `id = "recon_bars"`, the schedule XNYS after `bars_refresh` finishes, and a `[source]` table with `id = "yfinance"` as the only source setting; `manifest.go` returns the source id unchanged for any value (`manifest_test.go` has `TestSourceIdIsData` (new, with `tiingo`)). The names and texts named here are new.
2. `job.toml` has `run_bound = "30m"`; `run.go` takes the pairs from the watch entries of market US and JP (symbol and currency of the contract), resolves each instrument as bars_refresh does, reads the winner rows of basis `trades` of the last 5 sessions with `PriceBars`, makes ONE `DailyBars` call with a context deadline of `run_bound` minus 2 minutes (the bound parsed by `manifest.go`, time from the clock) and no batch of its own, builds `reconapi.Input` with `Skip` for each unanswered pair (reason the code) and `OtherMissingSkipped` true, calls the injected `reconapi.Func` (wired to `recon.Compare` by entry 23; the job imports no implementation package of compute) and writes one run at the end through `reconstore.Writer` with `pair_id` `R6`, `PairsAsked`, `PairsCompared`, the counts per kind, only the items that are not `match` (each with the symbol as `Company`) and the manifest's source id; no threshold and no level is made in the job. The names and texts named here are new.
3. A run in which some pairs were unanswered has status `partial` and the reason code of the first unanswered pair other than `no_data`, empty when every unanswered pair is `no_data`; a source error (no batch answered) writes status `failed` with that reason code (`rate_limited`, `network`, `timeout` or `source_changed`) and no items, never a pass; a job killed at `run_bound` writes nothing; running twice for the same day writes two runs; no force is needed for a past day and no time guard is in the job; `docs/contracts/jobs/recon_bars.md` states id, key, span, retry classes and the postcondition as the other job documents do. The names and texts named here are new.
4. `git grep -n 'tiingo\|yfinance\|time.Now\|time.Sleep' -- internal/jobs/reconbars ':!internal/jobs/reconbars/*_test.go' ':!internal/jobs/reconbars/job.toml'` prints nothing. The names and texts named here are new.
5. `run_test.go` has `TestReadsWinnerRows` (new), `TestUnansweredPairSkipped` (new), `TestSourceOnlyDayMissingOurs` (new), `TestDayOnlyOursIsSkipped` (new), `TestSourceDownFailedReason` (new), `TestPartialRunHasReason` (new), `TestOneDailyBarsCallWithRunBoundDeadline` (new, a fake source records the deadline), `TestMatchItemsNotStored` (new) and `TestTwoRunsSameDay` (new), with a fake source, a fake bar reader, a fake resolver, a fake writer and a fake clock, `t.Parallel()`, no sleep. The names and texts named here are new.
6. The result matches docs/design/base-data-page.md#21.

### 22 recon-cross-check-page

Module: pages. Size: M. Order: after 9, 14, 15, 16, 19 and 20.
Files: `internal/pages/base_data_page/crosscheck.go`, `internal/pages/base_data_page/crosscheck_test.go`, `internal/pages/base_data_page/builder.go`, `internal/pages/base_data_page/builder_test.go`, `internal/pages/base_data_page/page.go`, `internal/pages/base_data_page/convert.go`, `internal/pages/base_data_page/convert_test.go`, `internal/pages/base_data_page/testdata/cross_check_r6.json`
Contracts: unchanged
The page reads the newest runs, makes the pair levels with the injected `Verdict` function, hands them to `Eval` as `Input.Pairs` (so the bars row level includes R6) and fills the optional `cross_check` of the bars row.
Acceptance:
1. `page.go` gains the fields `Recon reconstore.Reader` (nil means no reader) and `Verdict reconapi.VerdictFunc` of `Input` (wired to `recon.Verdict` by entry 23; the pages package imports no implementation package of compute, as `Eval basecheckapi.Func` is injected); `builder.go` calls `Recon.Latest` once, makes each pair level by calling `Input.Verdict` with `PairsAsked` and `PairsCompared` of the run, passes them as `basecheckapi.Input.Pairs` with the row id `bars` for pair `R6`, and when the reader fails leaves `Pairs` and `cross_check` out and adds the gap `view_unavailable`. The names and texts named here are new.
2. `crosscheck.go` builds the `cross_check` object of the `bars` row from the newest run: pair id, `level` as made by the injected `Verdict`, `n`, `matched`, `diff`, `missing`, `skipped`, `tolerance_text`, `source_id`, `run_at` and the first 20 items, each with its `company` and `hint_zh` for R6 "两个来源不一致，先看是否有拆股尚未重述；两边都已按拆股调整，都不含分红调整"; the text carries "compared N of M" in Chinese from `n` and `skipped`; `convert.go` carries the object to the payload. The names and texts named here are new.
3. A day with no run gives the pair level `unknown`, never `pass`, with `reason` `no_run` and `reason_zh` "尚未运行"; a run of status `failed` gives `unknown` with `reason` the run's code and `reason_zh` "未能比对：" followed by the Chinese name of the code (`rate_limited` 被限流, `network` 网络错误, `timeout` 超时, `source_changed` 数据源格式变了); a `partial` run with a reason carries it the same way; the page code computes no level beyond that rule (a failed run is `unknown` without a call) and calling `Input.Verdict`. The names and texts named here are new.
4. `crosscheck_test.go` has `TestNoRunIsUnknown` (new, asserts `reason` `no_run`), `TestFailedRunIsUnknown` (new, asserts `reason` and `reason_zh` of a `rate_limited` run), `TestSkippedItemsShownWithReason` (new), `TestFirstTwentyItems` (new) and `TestR6Golden` (new, `testdata/cross_check_r6.json`, valid against `page.base_data_page.v1`); `builder_test.go` has `TestBarsRowLevelIncludesR6` (new, a fail pair over passing checks gives a fail row) and `TestReaderFailureAddsGap` (new) and `TestVerdictIsTheInjectedFunction` (new, a fake `VerdictFunc` decides the level). The names and texts named here are new.
5. The result matches docs/design/base-data-page.md#22.

### 23 recon-bars-wire

Module: ot. Size: M. Order: after 10, 17, 18, 20, 21 and 22.
Files: `cmd/ot/wire_barsource.go`, `cmd/ot/wire_reconbars.go`, `cmd/ot/wire_barsource_test.go`, `cmd/ot/runonce.go`, `cmd/ot/runonce_test.go`, `docs/contracts/ot/jobs/recon_bars.md`, `cmd/ot/jobconsole_jobs.go`, `cmd/ot/jobconsole_jobs_test.go`, `cmd/ot/data_health_site.go`, `cmd/ot/data_health_site_test.go`, `cmd/ot/base_data_site.go`, `cmd/ot/base_data_site_test.go`
Contracts: unchanged
The composition: the source registry (the one line a new source adds), the job family `recon_bars` at order 230 (free at `c517086c`; `registerJob` panics on a duplicate), its `jobTable` row and the tests and lists that count jobs, the compare function `recon.Compare` handed to the job, and the recon reader and the `recon.Verdict` function of the base data page (both its builders).
Acceptance:
1. `wire_barsource.go` holds the registry `map[string]sources.NewBarSourceFunc` with the one entry `"yfinance": yfinance.New`, builds the source from the manifest's `[source] id` with the command `<uv> run --locked --project <dir> python -m optdesk.refbars` made from the same uv program and project directory as `calendarCommand`, and an id not in the registry is an error that names the id and lists the known ids. The names and texts named here are new.
2. `runonce.go` gains the field `reconDSN` of `runOnceConfig`, read in `loadConfig` from `recon.DSNEnvRecon` (family DSNs come only from there); `wire_reconbars.go` passes `recon.Compare` to `NewReconBarsFunc` and registers the family `recon_bars` with `registerJob` at order 230 and a `startOff` that returns the reason "OT_PG_DSN_RECON is not set (run setup pg)" while `reconDSN` is empty, so the job is held with that reason and is not an error loop when the code deploys before the owner's setup pg (`runonce_test.go` has `TestReconDSNLoaded` (new) and `TestReconBarsHeldWithoutDSN` (new)); with the line it opens the writer; `jobconsole_jobs.go` gains the `recon_bars` row of `jobTable`, anyDay with force allowed; `jobconsole_jobs_test.go` and `data_health_site_test.go` count 34 jobs where they counted 33. The names and texts named here are new.
3. `data_health_site.go` names `recon_bars` in `dataHealthNoLedger` with the reason that it stores a reconciliation run and no ledger dataset, so `TestEveryJobHasRowOrReason` holds. The names and texts named here are new.
4. `base_data_site.go` registers a site opener for the recon reader (`reconpg.OpenReader` as the page role) beside the facts and coverage openers, and both `buildBaseData` and `baseDataPageNamed` pass it as `Input.Recon` and pass `recon.Verdict` as `Input.Verdict`; a missing `OT_PG_DSN_PAGES` line gives no reader as for the others (`base_data_site_test.go` has `TestBaseDataPassesReconReader` (new) and `TestBaseDataPassesVerdictFunc` (new)). The names and texts named here are new.
5. `docs/contracts/ot/jobs/recon_bars.md` has the line `Order: 230`, names the family file `cmd/ot/wire_reconbars.go`, the held reason and a `## Version` heading as the other documents of that directory do. The names and texts named here are new.
6. `wire_barsource_test.go` has `TestRegistryHasYfinance` (new), `TestUnknownSourceIdIsError` (new) and `TestSwitchIsRegistryEntry` (new: a fake `tiingo` entry added in the test builds a source from `[source] id = "tiingo"` with no other change); `jobconsole_jobs_test.go` has `TestReconBarsRowIsAnyDay` (new). The names and texts named here are new.
7. The result matches docs/design/base-data-page.md#23.

### 24 cross-check-web

Module: web. Size: S. Order: after 11, 16 and 22.
Files: `web/src/pages/base_data/CrossCheck.tsx`, `web/src/pages/base_data/DatasetTable.tsx`, `web/test/pages/base_data/cross_check.test.tsx`, `web/test/pages/base_data/fixtures/cross_check.ts`, `web/test/pages/base_data/fixtures/MANIFEST.synthetic-v1.json`
Contracts: unchanged
A column Cross-check on the dataset table and the opened pair with its items; the front end renders payload fields and computes nothing.
UI:
- 按 frontend-stack：用 shadcn `Badge` 做 交叉核对列的判定色（pass、warn、fail、unknown）。
- 按 frontend-stack：用 shadcn `Table` 做 展开后的差异明细表。
Acceptance:
1. `DatasetTable.tsx` gains the column `Cross-check` showing the row's `cross_check.level` and pair id, empty when the field is absent; `CrossCheck.tsx` renders the opened pair: level, `reason_zh` when present, compared, matched, differences, skipped, the items grouped by `company` in payload order, tolerance text, source id, run time and the items as a table (key, ours, other, difference, kind, hint); every text is a payload field; `git grep -n 'reduce\|parseFloat\|Number(' -- web/src/pages/base_data/CrossCheck.tsx` prints nothing. The names and texts named here are new.
2. `web/test/pages/base_data/fixtures/MANIFEST.synthetic-v1.json` lists `cross_check.ts` as it lists `ok.ts` and `gap.ts`; the lines of those two are unchanged. The names and texts named here are new.
3. `cross_check.test.tsx` has `renders the column with the pair verdict` (new), `renders skipped items with their reason` (new) and `renders no column text when cross_check is absent` (new) and `renders the reason line of a failed run` (new). The names and texts named here are new.
4. The screenshot of the page against the sketch of the design is taken by the `ui-shots` check (UI rule). The names and texts named here are new.
5. The result matches docs/design/base-data-page.md#24.

### 25 pyside-deploy-note

Module: docs. Size: S. Order: after 12.
Files: `docs/deploy/pyside.md`
Contracts: unchanged
A line in the pyside runbook: after the lock changes, the build step must run on every host that runs the refbars command (S2).
Acceptance:
1. `docs/deploy/pyside.md` says that after a change of `pyside/uv.lock` the build step `uv sync --locked --no-dev --project pyside` is run on every host that runs `python -m optdesk.refbars` (pyside itself runs with `--no-sync`), and records the Yahoo egress check of the design (200 for AAPL and 7203.T from the service host on 2026-11-18, not from inside the deployed process); no other line of the file changes. The names and texts named here are new.
2. The result matches docs/design/base-data-page.md#25.

### 26 refbars-api (interface)

Module: pyside. Size: S. Order: after 12.
Files: `pyside/src/optdesk/refbars/__init__.py`, `pyside/src/optdesk/refbars/api.py`, `pyside/tests/contracts/test_refbars_api.py`, `pyside/tests/refbars/answer.json`
Contracts: unchanged
The declarations of Interface 7: the typed dicts of the request and the answer of the command, the fetcher protocol and the clock protocol, with the one golden answer that entry 17 (the command) and entry 18 (the adapter) both read. Editing the interface file is an interface change; this lands before 17 and 18.
Acceptance:
1. `api.py` declares the typed dicts of the request (`pairs` of symbol and currency, `from_day`, `to_day`) and of the answer (`results` with `symbol`, `currency`, `error_code`, `bars` of `session_day` and `close`), the fetcher protocol and the clock protocol, and no logic; `__init__.py` holds no statement other than a docstring; the module imports no module of `optdesk.ibkr`, opens no database and no socket. The names and texts named here are new.
2. `pyside/tests/refbars/answer.json` is one answer of S1 with one pair ok with two bars, one pair with `error_code` `rate_limited` and one with `no_data`, closes as decimal text of 4 places; it is the only golden answer file of the command in the repository. The names and texts named here are new.
3. `test_refbars_api.py` has `test_refbars_api_is_declarations_only` (new) and `test_golden_answer_has_the_declared_keys` (new, reading `answer.json`), with `pytest -n auto`. The names and texts named here are new.
4. The result matches docs/design/base-data-page.md#26.

## Checked by CI


`ci-go`: `go build ./...`, `go test` of the packages of each Go entry, `golangci-lint`, the arch derivation; the `pg` tests run in GitHub CI only. `ci-web`: `tsc`, `eslint`, `vitest`, build. `ui-shots`: the screenshot of `/base-data` at both widths. After merge of entry 5 (four migrations, one hold) the window runs migrate (owner rule 2026-11-09).

## Left out

- Version 2 is cut for R6 on yfinance (entries 12 to 26). No entry now, text only: R1 to R5 and R7 to R9 and their jobs `recon_flex`, `recon_live`, `recon_internal` (the owner gave only the source word; they are cut later in the same shape on entries 14, 19, 20 and 22).
- Tiingo is the later alternative adapter (`internal/sources/tiingo`, config value `[source] id`, one registry line, S3); not built. A switch to Tiingo most likely loses the Japanese pairs (Tiingo end-of-day prices cover US stocks; Tokyo is unverified): its adapter answers `no_data`, every JP pair is `skipped`, the pair level counts only what was compared and the page shows the skipped count, so the loss is visible, not silent. Its key comes from the runner's service secrets (agent-crew arranges the line). The newest-run view mixes sources over a switch; the page shows `source_id` of the run it reads.
- A table of stock splits: none is stored; not needed for R6 (S4).
- A retry of yfinance inside a run and a cache of its answers: none (S6). A process or service for yfinance that stays running: none (S1).
- `settings.underlying` is not a base row (owner input, on `/underlyings`); `account.execution` is read by version 2 only.
- A hint line on `/data-health` pointing to `/base-data` (a change of a merged design's page; later, small).
- A stored history of verdicts and a chart over time: needs a job and a table.
- A job in place of the page-time views if the page is slow: the same `Evaluate`, a job and a table.
- `no_value` and not-refetchable history is shown as a fact, not repaired (the repair is `/data-health`).
- JP bars cross-check: by yfinance with `.T` symbols (S1, S3); JP volume is not compared.
- Not rows and not judged in version 1, with the reason: `master.instrument` (a registry of ids and symbols, no value to judge; its `symbol` is only joined for examples); `valuation.company_profile` (the owner's own valuation input, edited on its page, not base data); stock splits (no table stores them, so `bar_jump` is warn only until one does); `adjusted_last` bars (a second basis for which no page reads a value yet; R6 of version 2 compares `trades` only).
- Thresholds are trial values; tuning is a later change of `rules.go` (thresholds) only, never of the table in `basecheckapi`.

## Adversarial

Reviews: `state/optdesk-drafts/designs/base-data-page/adversarial.md` (read on `8ce1059b`, answered in round one) and the five blocking points of optdesk-advisor against `c16ebc95` (round two, answered here). Re-verified on `c16ebc95`. One line per point.

Round one, six questions:
1. Gap against the goal: adopted. Merged `bars` row and the ledger table of Page; `obs.option_eod_index` in `chains`, `master.option_*` in `contracts`; `bar_gap` dropped; per-calendar `Days` and `Sessions`; `settings.underlying` in Left out.
2. Biggest risk (views unspecified, unbounded cost): adopted in part. Four views, one migration; the cost is stated in Decision 3 with the bars bound (instruments x 31 sessions x 2 bases, instruments not measured). Page time on production is unverified and is read once after the deploy; a cache or job is not adopted now.
3. Simpler way: adopted (table-constraint rules dropped with their constraint names; 12 rows). A page without a migration is not adopted: the `rd` views hide symbol, the bar status and the per-contract pick.
4. Lock-in: adopted (`-- rule` lines, rule marks `sql`, `ledger`, `derived`, `note`, tests in entries 7 and 10, the v2 slot in the schema, migration order).
5. False claims: adopted (`calUS`/`calJP` in `cmd/ot`, `price_bar_positive_ck`, keys of bars and fx, `macro_event` columns, symbol from `master.instrument`).
6. Likeliest slip (a view failing in CI): adopted in part. The first split into four migrations is replaced in round two by one migration entry (5) plus one pg-test entry (6); a failing view no longer blanks the page (point 3 below).

Round one, blocking defects 1 to 9: all adopted as in round one; the one change since: defect 8 (size of the SQL) is now answered by one migration entry with the worker told to report, not split, a file over 600 lines, and by pg tests in a separate entry.

Round two, advisor points:
1. Status: adopted. The codes are read from `docs/contracts/datasets/obs.option_eod.md` and `statusOf`; the pick is the newest status-1 row; `empty` counts no-quote contracts only when no status-1 row exists; errors have their rule; not listed and stale are the fact `not_judged`; `non_live` is added for chains, quotes and open. The sentence "I have not read the meaning of the codes" is deleted. For the option open table I used its own code list from its dataset doc.
2. Entry 4 pg test: adopted. `reader_pg_test.go` moved to entry 6, ordered after 4 and 5.
3. One failing view must not blank the page: adopted (Decision 3; `Result.Failed`; `NotRead`; gap `view_unavailable`; entries 4, 8, 9, 11).
4. Entry 3 Contracts grammar: adopted (`page.base_data_page v1 (new)`).
5. (a) `bar_jump` warn only: adopted. (b) `flex_section_absent` a setup note, source `note`, never in the verdict: adopted. (c) Left out names `master.instrument`, `valuation.company_profile`, splits and `adjusted_last` bars with reasons: adopted.
Also: one migration with four views: adopted. Nav order against the r7 nav entry: dropped, since #2909 is merged at `c16ebc95`. Age never negative, both calendars always present, rows against `dataHealthManifests`: adopted as tests in entries 8, 9 and 10. Not adopted: none.

Round three, design session on pull request 2911 (head 01973220), three blocking points, all adopted: (1) entry 9 no longer needs entry 7: the rule and row ids it covers are `basecheckapi.Rules` and `Rows`, declared in the interface (entry 2); `RuleList` and `basecheck.RuleIDs` are removed. (2) Entry 10's tests read the same table (id, row, source mark, ledger datasets) from the interface and the hints through the exported `base_data_page.Hint` of entry 9, so no test needs unmerged sibling output beyond entries 8 and 9, which it is ordered after. (3) The SQL is four migrations, 0056 to 0059, one view each, landed by one issue (entry 5), one deploy hold. Notes taken: the store `Fact` is the source type and `convert.go` follows it; the 12-row table has one owner (the interface); `capture_age`, `flex_age` and the `age` rules of contracts, events and facts are marked derived.

## Adversarial (version 2 round 2, `adversarial-raw.md`)

The read-only review `state/optdesk-drafts/designs/base-data-v2/adversarial-raw.md` (origin/main c517086c) made six points; each is answered here.

1. Gap against the goal (the cross-check never reaches the page; the row verdict ignores R6; nobody owns the level function; the schema has no `skipped`; a missing bar is hidden): adopted. Entry 22 edits `builder.go`, `page.go` and `convert.go`; entry 23 opens the recon reader in both `buildBaseData` and `baseDataPageNamed`; `recon.Verdict` makes the pair level (S5, entry 19); `basecheckapi.Input.Pairs` and the fold in `Evaluate` put it in the row level (S7, entries 14, 19); the schema gains `skipped` (entry 16); a bar the source has and we lack is `missing_ours`, counted as bad. R1 to R5 and R7 to R9 stay "no entry: text only" by the owner's scope.
2. Biggest risk (entry 15 needs the network for the lock; mypy; import-linter; a second service with no health check; timeouts; global state; tz cache): adopted. The dependency, the lock and the mypy overrides are entry 12, written by the window (the import-linter line comes with the package in entry 17, so entry 12 is green alone); every other pyside entry is after it. There is no service now, so no deploy file and no "not running" state. Batches of 10 with a deadline and partial results (S6). The test that sets `hide_exceptions` restores it (entry 17). The tz cache goes to a temporary directory (Facts).
3. Simpler way (one-shot command like `calendar_refresh`): adopted in full. No proto, no genpb, no generator edit, no socket, no owner-only merge. The Go adapter has its own exec with stdin and stdout because `calendarcli.Exec` discards output (S3, entry 18). `BarSource` stays, so the Tiingo switch is the same.
4. Lock-in (Python mapping, Tiingo semantics, JP loss, key plumbing, mixed history): partly adopted. The symbol mapping stays in Python by design and a Tiingo adapter does its own in Go; that is the cost of an adapter and is said in Left out. The JP loss, the key from the runner's service secrets and the mixed source history are in Left out; the page shows `source_id` and the skipped count. Not adopted: a source column in the view's key.
5. Claims most likely false: adopted. S4 is rewritten on the real model (IBKR `trades` is split-adjusted and restated by `adjust_epoch`; the winner rows are read; the split skip is dropped, with the reason). The socket path is gone. The job entry now carries the files that its row breaks (`data_health_site.go`, the two counting tests, order 230, the job document `docs/contracts/jobs/recon_bars.md`). The generator script problem is gone with the proto. Float artefacts: decimal text rounded to 4 places (entry 17). Order lines now name the entries they need.
6. Likeliest slip (entries 15 and 12 would bounce on Files and on the lock): adopted; entry 12 (window) holds `pyproject.toml` and the lock (with the mypy overrides), the deploy note is entry 25, entry 17 edits `pyproject.toml` for the import-linter lines only, and the owner-only entry is gone.
