# Calendar against IBKR hours: a daily early warning for ad-hoc closures and early closes (layer 1 only)

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-13
Revision: 1

Source: owner decision 2026-11-13, "layer 1 only": each day, after `calendar_refresh`, compare our exchange calendar of the next 7 days with IBKR's own hours of one reference contract per market (SPY for XNYS, 7203 Toyota for XTKS); every differing day logs one line and raises a notice the home page shows; our calendar is never changed by it; read-only broker calls only; the after-the-fact layer 2 is dropped. Owner rules: own glue code sits behind an interface, prefer a mainstream library, the smallest design, development mode (no time guard). Read on origin/main at e69b8d10: `AGENTS.md`, `docs/design/modules.md`, `docs/design/compute-calendar.md` (Decision 2 left the cross-check "a later design"), `contracts/proto/ibkr/v1/resolve.proto`, `internal/broker/{api.go,grpcbroker/dial.go}`, `pyside/src/optdesk/ibkr/{ibgateway.py,model.py,transport.py}`, `internal/clock/{api.go,calendar/diff.go}`, `internal/jobs/calendarrefresh/*`, `internal/pages/home_page/*`, `migrations/{0004,0012,0015}*.sql`. The IBKR string format is quoted from the owner's ask ("20261127:0930-20261127:1300;20261128:CLOSED"), not from a probe. "[unverified]" marks a claim not checked in code or a probe.

Goal: when IBKR's hours of SPY or Toyota differ from our `master.session_day` on any of the next 7 days (open vs closed, open time, close time), a line is logged and the home page's gap line names the day, within one daily run.

## Today

| Piece | On main |
|---|---|
| hours | `tradingHours`, `liquidHours` and `timeZoneId` of `ContractDetails` are read by nothing: `ibgateway.py` (`resolve`, ~:424) takes `conId`, `exchange`, `tradingClass`, `multiplier` from the details and drops the rest; `ResolveContractResponse` carries `status`, `contract`, `error_code`, `error_text` |
| calendar | `calendar_refresh` (XNYS anchor, `-30m`) refreshes `master.session_day` from the pinned library and logs one line per changed day (`CalendarObserver`); it has no broker and no IBKR comparison (`compute-calendar.md` Decision 2) |
| notices | there is no notice table. The home page shows a `gap_line` text built by `home_page.gapLine` from the gaps of its five cards; `ops.job_gap` (`0012_scheduler.sql`) holds scheduler gaps for notify and is written by the runner only, with no CHECK on `reason` |
| modules | the depends cell of `jobs` already names `broker` (`modules.md` row jobs), so a jobs package may import `internal/broker`; #1929 concerns only the `ot` component and is not needed here |

## Decisions

1. Carrier: three optional strings on `ResolveContractResponse`, `trading_hours = 5`, `liquid_hours = 6`, `time_zone_id = 7`, filled on FOUND and empty otherwise. Not a new rpc, because the resolve already holds the `ContractDetails` and a new rpc would add a handler, a client method, a fake and a service entry for the same one request; added fields keep old clients working (proto3). Python, `Resolved` in Go and `grpcbroker` pass them through unchanged.
2. Reference contracts: SPY (stock, SMART, USD) for XNYS and 7203 (stock, TSEJ, JPY) for XTKS, constants of the job package, one `ResolveContract` each per run. A gateway or resolve failure of one market fails only that market (error names market and code, as `earnings_refresh` does); the other market still compares. A failed market leaves its notices as they were.
3. Which string: `liquid_hours` (the regular session), not `trading_hours` (it includes extended hours). A day's IBKR open is the first segment's start and its close the last segment's end, so the Tokyo lunch break is ignored, as our calendar has one open and one close per day. `CLOSED` is a closed day. Times are read in `time_zone_id`, then compared as instants with our `open_at` and `close_at`.
4. Compare: for each of the next 7 days (today first) and each market, ours = the `master.session_day` row, IBKR's = the parsed day. It differs when `IsSession` differs, or both are open and `open_at` or `close_at` differ by one minute or more. A day IBKR's string does not list is not compared (IBKR lists about a week, [unverified]); a day beyond the string never raises a notice.
5. Parser behind an interface: `clock.IBKRHours` (`Parse(liquidHours, timeZoneID string) ([]clock.HoursDay, error)`; `HoursDay{Day, IsSession, OpenAt, CloseAt}`), implementation `internal/clock/ibkrhours`. Own implementation, because no maintained Go library parses IBKR's session strings [unverified]; about 60 lines, and a library may replace it behind the interface. A string it cannot read is an error naming the market, never a guess.
6. Output: one `slog` line per differing day, `calendar hours differ market=XNYS day=2026-11-27 ours=0930-1300 ibkr=CLOSED`; and one row per differing day in a new table `master.session_hours_notice` (market, day, ours, ibkr, first_seen_at, cleared_at), written by the role `ot_w_master` (which the loader line already uses). A row of a day that no longer differs, or is past, is cleared by the same run; an open row is one per (market, day). The job never writes `master.session_day`.
7. Home page: the page reads open rows through a view `rd.session_hours_notice_r1` (grant to `ot_ro_pages`) and `gapLine` appends them to the `gap_line` text, for example `1 gap, shown in its card; calendar: XNYS 2026-11-27 ours 0930-1300, IBKR closed`. No page schema, no generated TypeScript and no web file changes (the field is a string already); a typed list is a later change (Left out). Rows go in `master`, not in `ops.job_gap`, because that table is the runner's and a row there would also be sent to Telegram by notify, which the owner did not ask for.
8. Schedule: a new job `calendar_hours`, XNYS anchor `session_open`, offset `-20m` (10 minutes after `calendar_refresh` starts, so a refreshed calendar is read; its run bound 5m bounds the wait), `restart_sensitive = false`. It runs from `cmd/ot` with the broker and the session-day reader already wired; it makes read-only calls (`ResolveContract`) only, the AGENTS.md rule on no order and no write rpc holds. Reading both markets in one run means a XTKS notice is raised on a US session day; a US holiday means no run that day, and the next session day's run finds it (as `compute-calendar.md` says for refresh).
9. Migration: one file adds the table, the view and the grants; no CHECK of an existing table changes (there is no notice table). It is a migration, so the owner's window runs `setup pg` and `migrate` after the merge (memory: window runs migrate).

## Modules

contracts (proto and generated output), broker (`Resolved` and `grpcbroker`), clock (the parser interface and implementation), pyside (fill the fields), store (the notice table and its reader and writer), jobs (the new job), pages (the gap line), ot (wiring and the manifest). Depends cells: unchanged (jobs names broker, clock and store; pages names store).

## Interface

Owning modules, in this order: contracts (entry 1: the three fields), broker (entry 2: `Resolved.TradingHours`, `LiquidHours`, `TimeZoneID` strings), clock (entry 3: `IBKRHours` and `HoursDay` in `internal/clock/api.go`), store (entry 5: `HoursNoticeStore`, `HoursNoticeReader` and `HoursNotice{Market, Day, Ours, IBKR string}` in `internal/store/api.go`). Python `api.py` of ibkr declares no resolve call (resolve lives in `model.py`), so `pyside/src/optdesk/ibkr/api.py` and `test_ibkr_api.py` stay unchanged; the pyside interface is the proto.

## Issues

Order: 1 first; 2, 3, 4, 5 after 1 (3 and 5 have no other dependency); 6 after 2, 3, 5; 7 after 5; 8 after 6 and 7. BASE is `git merge-base origin/main HEAD`.

### 1 resolve-hours-fields (interface)

Module: contracts. Order: first.
Files: `contracts/proto/ibkr/v1/resolve.proto`, `internal/contracts/genpb/resolve.pb.go`, `pyside/src/optdesk/contracts/genpb/ibkr/v1/resolve_pb2.py`, `pyside/src/optdesk/contracts/genpb/ibkr/v1/resolve_pb2.pyi`, `pyside/tests/contracts/test_contracts_api.py`
Design: docs/design/calendar-ibkr-hours.md#decisions (Decision 1)
Acceptance:
1. `git grep -n "string trading_hours = 5;\|string liquid_hours = 6;\|string time_zone_id = 7;" -- contracts/proto/ibkr/v1/resolve.proto` prints three lines, and the comment of `ResolveContractResponse` says they are empty unless FOUND.
2. The Go and Python generated files carry the three fields and `git diff --name-only BASE` lists only the Files paths; the existing fields keep their numbers.
3. `test_contracts_api.py` builds a response with the three fields and reads them back, and one without them reads empty strings.

### 2 broker-resolved-hours

Module: broker. Order: after 1.
Files: `internal/broker/api.go`, `internal/broker/api_test.go`, `internal/broker/grpcbroker/dial.go`, `internal/broker/grpcbroker/client_test.go`, `docs/contracts/broker.md`
Acceptance:
1. `Resolved` declares `TradingHours, LiquidHours, TimeZoneID string` with a doc line that they are empty unless `Listed`; no other declaration changes.
2. `dial.go` copies the three response fields on FOUND and leaves them empty on NOT_LISTED; `client_test.go` has one test of each from a fake `ContractsService`.
3. `docs/contracts/broker.md` names the three fields.

### 3 clock-ibkr-hours-parser (interface)

Module: clock. Order: after 1 (no code dependency; lands before 6).
Files: `internal/clock/api.go`, `internal/clock/api_test.go`, `internal/clock/ibkrhours/parse.go`, `internal/clock/ibkrhours/parse_test.go`, `docs/contracts/clock.md`
Design: docs/design/calendar-ibkr-hours.md#interface (Decisions 3, 5)
Acceptance:
1. `api.go` declares `IBKRHours` and `HoursDay` only; its test checks the contract with a fake from both sides.
2. `Parse` returns, per listed day, the first start and the last end of its segments as UTC instants of `time_zone_id`, and `IsSession=false` for `CLOSED`; tests cover a one-segment day, a two-segment day (the Tokyo lunch break), an early close (`0930-1300`), a `CLOSED` day, a zone with daylight saving on a changeover day, and refuse an empty zone, an unknown zone, a segment end before its start and a malformed token, each naming the token.
3. The package imports no other package of the module besides `internal/clock` and `internal/contracts`.

### 4 pyside-fill-hours

Module: pyside. Order: after 1.
Files: `pyside/src/optdesk/ibkr/model.py`, `pyside/src/optdesk/ibkr/ibgateway.py`, `pyside/src/optdesk/ibkr/transport.py`, `pyside/tests/ibkr/test_ibgateway.py`, `pyside/tests/ibkr/test_transport.py`
Acceptance:
1. `ResolveResult` gains `trading_hours`, `liquid_hours`, `time_zone_id` strings with default `""`; `ibgateway.resolve` fills them from `details[0].tradingHours`, `liquidHours`, `timeZoneId` on the found path only.
2. `transport.py` copies them into `pb.ResolveContractResponse` on found; the not-listed response leaves them empty; one test of each in `test_transport.py`, and `test_ibgateway.py` checks a fake `ContractDetails` carrying the three strings.
3. No call is added: the one `reqContractDetailsAsync` of the resolve is the only broker call (read in `ibgateway.py`).

### 5 store-hours-notice (interface)

Module: store. Order: after 1.
Files: `internal/store/api.go`, `internal/store/api_test.go`, `internal/store/hoursnotice/store.go`, `internal/store/hoursnotice/store_test.go`, `migrations/0039_session_hours_notice.sql`, `docs/contracts/store.md`
Design: docs/design/calendar-ibkr-hours.md#decisions (Decisions 6, 7, 9)
Acceptance:
1. `api.go` declares `HoursNotice`, `HoursNoticeReader` (open rows, ordered by market and day) and `HoursNoticeStore` (`Replace(ctx, markets []string, rows []HoursNotice)`: opens the rows given, clears the open rows of those markets not given); its test checks the contract with a fake from both sides.
2. The migration creates `master.session_hours_notice`, the view `rd.session_hours_notice_r1` and grants to `ot_w_master` (INSERT, SELECT, UPDATE of `cleared_at`) and `ot_ro_pages` (SELECT on the view only), with a Down; it changes no CHECK and no other table, and its number is the next free one at merge time.
3. Store tests with the build tag `pg` open, repeat (one open row per market and day) and clear; a test without a database reads the SQL text for the grants.

### 6 job-calendar-hours

Module: jobs. Order: after 2, 3, 5.
Files: `internal/jobs/calendarhours/job.go`, `internal/jobs/calendarhours/job.toml`, `internal/jobs/calendarhours/types.go`, `internal/jobs/calendarhours/job_test.go`, `docs/contracts/jobs/calendar_hours.md`
Acceptance:
1. `New` takes a `broker.Broker`, a `clock.SessionSource`, a `clock.IBKRHours`, a `store.HoursNoticeStore`, a `clock.Clock` and a logger; it imports no other jobs package and `scripts/arch --check` passes.
2. Tests with fakes and a fake clock: equal days write no row and log nothing; an IBKR `CLOSED` against our open day, an early close and a differing open time each log one line of Decision 6 and write one row; a row of a day that no longer differs is cleared; a failed `ResolveContract` of one market leaves that market's rows and still compares the other, and the error names market and code; an unparsable string fails its market only.
3. The only broker method the job calls is `ResolveContract` (read in `job.go`), and it never writes `master.session_day` (the job holds no `SessionDayWriter`).
4. `job.toml` has the schedule of Decision 8 and `docs/contracts/jobs/calendar_hours.md` states Decisions 2 to 4.

### 7 pages-home-hours-notice

Module: pages. Order: after 5.
Files: `internal/pages/home_page/overview.go`, `internal/pages/home_page/compose.go`, `internal/pages/home_page/gapline_test.go`, `internal/pages/home_page/compose_test.go`
Acceptance:
1. `gapLine` takes the open `[]store.HoursNotice` as well and, with at least one, appends `; calendar: <market> <day> ours <ours>, IBKR <ibkr>` per row after the card text; with none, its text is unchanged (the existing `gapline_test.go` cases stay as they are).
2. A test with two rows shows both in the line; the page still validates against its schema with no schema or web file changed.

### 8 ot-wire-calendar-hours

Module: ot. Order: after 6 and 7.
Files: `cmd/ot/wire.go`, `cmd/ot/wire_calendar.go`, `cmd/ot/runonce.go`, `cmd/ot/jobconsole_jobs.go`, `cmd/ot/home_site.go` (the caller of `home_page.Compose`, line 158), `cmd/ot/wire_test.go`
Acceptance:
1. `calendar_hours` is wired with the broker, the session-day reader, `ibkrhours` and the `hoursnotice` store of the `ot_w_master` line, registered in the job console as today-only with force allowed.
2. The home site reads `HoursNoticeReader` and passes the rows to `Compose`'s caller; a nil reader gives the old text.
3. A wiring test lists `calendar_hours` and checks it is registered after `calendar_refresh`.

## Checked by CI

`go test ./... -race -count=1`, `go run ./scripts/arch --check`, `go build ./...`, `buf lint` and the generated-output check, `uv sync --locked`, `ruff check`, `mypy --strict`, `pytest -n auto` in `pyside/`, the `pg`-tag store tests in GitHub CI.

## Left out

- Layer 2 (a check after the day, the IBKR hours against what happened), any automatic change of `master.session_day`, and a notice by Telegram.
- A typed notice list on the home page (a page schema and web change): the first version words the gap line; the owner may ask for it.
- More reference contracts, other markets, extended hours (`trading_hours`), the Tokyo lunch break as a separate check, and a tolerance setting.
- A `ListContracts`-style new rpc or a stand-alone hours rpc (Decision 1).
- A time-of-day guard: development mode, the job runs when triggered.

## Doubts

1. The `liquid_hours` of 7203 may list the lunch break as two segments and a half day around the year end; Decision 3 reads first start and last end, so a half day shows as an early close, which is the intended warning.
2. The exact set of days IBKR lists, and whether `timeZoneId` of Toyota is `Japan` or `Asia/Tokyo`, are not probed; entry 3's tests use both names and the owner's window may run one read-only probe of SPY and 7203 before entry 6.
3. `Compose` builds `gapLine` itself, so entry 7 adds an `Input` field for the rows and entry 8 fills it in `home_site.go`.
