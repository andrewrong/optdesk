# Calendar against IBKR hours: a daily early warning for ad-hoc closures and early closes (layer 1 only)

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-13
Revision: 2

Source: owner decision 2026-11-13, "layer 1 only": each day, after `calendar_refresh`, compare our exchange calendar with IBKR's own hours of one reference contract per market (SPY for XNYS, 7203 Toyota for XTKS); every differing day logs one line and raises a notice the home page shows; our calendar is never changed by it; read-only broker calls only; the after-the-fact layer 2 is dropped. Owner rules: own glue code sits behind an interface, prefer a mainstream library, the smallest design, development mode (no time guard). Read on origin/main at e69b8d10: `AGENTS.md`, `docs/design/modules.md`, `docs/design/compute-calendar.md` (Decision 2 left the cross-check "a later design"), `contracts/proto/ibkr/v1/resolve.proto`, `internal/broker/{api.go,grpcbroker/dial.go}`, `pyside/src/optdesk/ibkr/{ibgateway.py,model.py,transport.py}`, `internal/clock/{api.go,calendar/diff.go}`, `internal/jobs/calendarrefresh/*`, `internal/pages/home_page/*`, `migrations/{0004,0012,0015}*.sql`. The IBKR string format was probed read-only by the adversarial read (answer file `cih-adversarial.md`, 2026-11-13): SPY zone `US/Eastern`, 7203 zone `Japan`, IBKR lists only about 6 calendar days (SPY yesterday to +4, 7203 today to +5), every token and every segment of one day is separated by `;` (a day is a run of tokens with one start date), 7203 ends 15:30. "[unverified]" marks a claim not checked in code or a probe.

Goal: when IBKR's hours of SPY or Toyota differ from our `master.session_day` on any day IBKR lists (about 4 to 5 days ahead, not 7), a line is logged and the home overview lists the day, within one run of each market's session day.

## Today

| Piece | On main |
|---|---|
| hours | `tradingHours`, `liquidHours` and `timeZoneId` of `ContractDetails` are read by nothing: `ibgateway.py` (`resolve`, ~:424) takes `conId`, `exchange`, `tradingClass`, `multiplier` from the details and drops the rest; `ResolveContractResponse` carries `status`, `contract`, `error_code`, `error_text` |
| calendar | `calendar_refresh` (XNYS only, anchor `session_open`, `-30m`) refreshes `master.session_day` from the pinned library and logs one line per changed day; it has no broker and no IBKR comparison (`compute-calendar.md` Decision 2). XTKS rows are refreshed by no job: the generator supports XTKS (`SUPPORTED_CALENDARS`) and the rows land by `ot calendar load` of a generated file (`calendarrefresh` for XTKS is Left out of `option-capture-jp-stocks.md`) |
| notices | there is no notice table. The home overview has a typed list `overview.gaps` of `{card, text}` (`internal/pages/home_page/overview.go:25-28`, page schema v3, rendered by the web overview card), empty today by comment; `gap_line` is a count text from the cards; `ops.job_gap` (`0012_scheduler.sql`) holds scheduler gaps for notify and is written by the runner only, with no CHECK on `reason` |
| modules | the depends cell of `jobs` already names `broker` (`modules.md` row jobs), so a jobs package may import `internal/broker`; #1929 concerns only the `ot` component and is not needed here |

## Decisions

1. Carrier: three optional strings on `ResolveContractResponse`, `trading_hours = 5`, `liquid_hours = 6`, `time_zone_id = 7`, filled on FOUND and empty otherwise. Not a new rpc, because the resolve already holds the `ContractDetails` and a new rpc would add a handler, a client method, a fake and a service entry for the same one request; added fields keep old clients working (proto3). Python, `Resolved` in Go and `grpcbroker` pass them through unchanged.
2. Reference contracts: SPY (stock, SMART, USD) for XNYS and 7203 (stock, TSEJ, JPY) for XTKS, constants of the job package, one `ResolveContract` each per run. Each job has one market, so a gateway or resolve failure fails that job only (the error names market and code, as `earnings_refresh` does) and leaves its rows as they were: a stale notice stays until the next good run, and the owner sees "could not compare" as the failed run on the ops page.
3. Which string: `liquid_hours` (the regular session), not `trading_hours` (it includes extended hours). A day's IBKR open is the first segment's start and its close the last segment's end, so the Tokyo lunch break is ignored, as our calendar has one open and one close per day. `CLOSED` is a closed day. Times are read in `time_zone_id`, then compared as instants with our `open_at` and `close_at`.
4. Compare: for each day IBKR lists (about 4 to 5 days; the horizon is IBKR's, not 7) and each market, ours = the `master.session_day` row, IBKR's = the parsed day. It differs when `IsSession` differs, or both are open and `open_at` or `close_at` differ by one minute or more. A day with no row of ours (XTKS rows are loaded by hand, no job refreshes them) is "not compared", never a notice; a day IBKR does not list is not compared either. Our XTKS rows must say 15:30 for the close (IBKR's 7203 ends 15:30), else every Tokyo day differs; this is read from the rows and not changed here.
5. Parser behind an interface: `clock.IBKRHours` (`Parse(liquidHours, timeZoneID string) ([]clock.HoursDay, error)`; `HoursDay{Day, IsSession, OpenAt, CloseAt}`), implementation `internal/clock/ibkrhours`. The parser splits on `;` and groups tokens by start date (one day may hold two segments). Own implementation, because no maintained Go library parses IBKR's session strings [unverified]; about 60 lines, and a library may replace it behind the interface. A string it cannot read is an error naming the market, never a guess.
6. Output: one `slog` line per differing day, `calendar hours differ market=XNYS day=2026-11-27 ours=0930-1300 ibkr=CLOSED`; and one row per differing day in a new table `master.session_hours_notice` (market, day, ours, ibkr, seen_at), no `cleared_at`: each run deletes its market's rows and inserts the differing days in one transaction, so a day that no longer differs vanishes. The writer is `internal/store/hoursnoticepg` on the master line (`OT_PG_DSN_MASTER`, role `ot_w_master`, the instrument writer, opened in `runonce.go` and `wire_optionopen.go`), not the loader line (`ot_loader`, which `calendar_refresh` uses). The job's session reader is the loader store already wired for `calendar_refresh`, a second store. The job never writes `master.session_day`.
7. Home page: the page reads the rows through a view `rd.session_hours_notice_r1` (grant to `ot_ro_pages`; read through `hoursnotice.Reader`, implemented in `hoursnoticepg`) and fills the typed `overview.gaps` list, one `{card: "calendar", text: "XNYS 2026-11-27 ours 0930-1300, IBKR closed"}` per row; `gapLine` and its tests stay unchanged, and no schema or web file changes. Rows go in `master`, not in `ops.job_gap`, because that table is the runner's and notify would send a row there to Telegram, which the owner did not ask for.
8. Schedule: two jobs of one package, `calendar_hours_us` (reference SPY, calendar XNYS) and `calendar_hours_jp` (7203, calendar XTKS), each anchored on its own calendar's `session_open` with offset `-20m`, so a US holiday does not blind the Tokyo check and a Tokyo holiday does not blind the US one (one job on one calendar would skip the other market's days). A market's closed day needs no warning that day: its next session day compares again. `restart_sensitive = false`, run bound 5m. Read-only calls (`ResolveContract`) only. Neither job is force-allowed in the job console, and each has its own unique `order` number: 85 for `calendar_hours_us` and 86 for `calendar_hours_jp` (free in the registry and in `docs/contracts/ot/jobs`, right after `calendar_refresh`, 80). Each is a `todayOnly` row of the job console with the reason code `not_proved` (a run of a stored day is not proved to leave the notice rows right).
9. Migration: one file (the next free number at merge time, `migrations/0040_session_hours_notice.sql` today, since main has 0039_fx_rate_repair.sql) adds the table, the view and the grants; no CHECK of an existing table changes (there is no notice table). It is a migration, so the owner's window runs `setup pg` and `migrate` after the merge (memory: window runs migrate).

## Modules

contracts (proto and generated output), broker (`Resolved` and `grpcbroker`), clock (the parser interface and implementation), pyside (fill the fields), store (the notice table and its reader and writer), jobs (the new job), pages (the gap line), ot (wiring and the manifest). Depends cells: unchanged (jobs names broker, clock and store; pages names store).

## Interface

Owning modules, in this order: contracts (entry 1: the three fields), broker (entry 2: `Resolved.TradingHours`, `LiquidHours`, `TimeZoneID` strings), clock (entry 3: `IBKRHours` and `HoursDay` in `internal/clock/api.go`), store (entry 5: `Store`, `Reader` and `Notice{Market, Day, Ours, IBKR string}` in `internal/store/hoursnotice/api.go`, a declarations-only package like `internal/store/jobrequest`, with its Postgres implementation in `internal/store/hoursnoticepg` (as `bar` and `barpg`), because `internal/store/api.go` is at its 800 lines; the store interface cell of `docs/design/modules.md` names the package); entries 1 and 3 do not depend on each other and come first. Python `api.py` of ibkr declares no resolve call (resolve lives in `model.py`), so `pyside/src/optdesk/ibkr/api.py` and `test_ibkr_api.py` stay unchanged; the pyside interface is the proto.

## Issues

Order: 1 and 3 first; 2, 4, 5 after 1; 6 after 2, 3, 5; 7 after 5; 8 after 6 and 7. BASE is `git merge-base origin/main HEAD`.

### 1 resolve-hours-fields (interface)

Module: contracts. Order: first. Owner-only: `contracts/proto/**` is an owner-only path, so the gate needs the owner's word and the owner merges this pull request.
Files: `contracts/proto/ibkr/v1/resolve.proto`, `internal/contracts/genpb/resolve.pb.go`, `pyside/src/optdesk/contracts/genpb/ibkr/v1/resolve_pb2.py`, `pyside/src/optdesk/contracts/genpb/ibkr/v1/resolve_pb2.pyi`, `internal/contracts/genpb/resolve_proto_test.go`, `pyside/tests/contracts/test_contracts_api.py`
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

Module: clock. Order: first.
Files: `internal/clock/api.go`, `internal/clock/api_test.go`, `internal/clock/ibkrhours/parse.go`, `internal/clock/ibkrhours/parse_test.go`, `docs/contracts/clock.md`
Design: docs/design/calendar-ibkr-hours.md#interface (Decisions 3, 5)
Acceptance:
1. `api.go` declares `IBKRHours` and `HoursDay` only; its test checks the contract with a fake from both sides.
2. `Parse` returns, per listed day, the first start and the last end of its segments as UTC instants of `time_zone_id`, and `IsSession=false` for `CLOSED`; tests cover a one-segment day, a two-segment day (the Tokyo lunch break, tokens joined by `;` and grouped by start date), an early close (`0930-1300`), a `CLOSED` day, the zones `US/Eastern` (with a daylight-saving changeover day) and `Japan`, a Tokyo day ending 15:30, and refuse an empty zone, an unknown zone, a segment end before its start and a malformed token, each naming the token.
3. The package imports no other package of the module besides `internal/clock` and `internal/contracts`.

### 4 pyside-fill-hours

Module: pyside. Order: after 1. Owner-only: `transport.py` is an owner-only path, so the gate needs the owner's word and the owner merges this pull request.
Files: `pyside/src/optdesk/ibkr/model.py`, `pyside/src/optdesk/ibkr/spec.py`, `pyside/src/optdesk/ibkr/ibgateway.py`, `pyside/src/optdesk/ibkr/transport.py`, `pyside/tests/ibkr/test_ibgateway_hours.py` (new), `pyside/tests/ibkr/test_transport.py`
Acceptance:
1. `ResolveResult` gains `trading_hours`, `liquid_hours`, `time_zone_id` strings with default `""`; `ibgateway.resolve` fills them from `details[0].tradingHours`, `liquidHours`, `timeZoneId` on the found path only.
2. `transport.py` copies them into `pb.ResolveContractResponse` on found; the not-listed response leaves them empty; one test of each in `test_transport.py`, and the new `test_ibgateway_hours.py` checks a fake `ContractDetails` carrying the three strings (`test_ibgateway.py`, 793 lines, is not edited).
3. No call is added: the one `reqContractDetailsAsync` of the resolve is the only broker call (read in `ibgateway.py`).
4. Size: `ibgateway.py` (785 lines on main at this writing) gains one line, a call of a field helper written in `spec.py` (`model.py` holds declarations only); `transport.py` (791 lines on main at this writing) and `ibgateway.py` each stay at most 800 lines (the ratchet limit is a file over 800), read by `wc -l`; neither is split here.

### 5 store-hours-notice (interface)

Module: store. Order: after 1.
Files: `internal/store/hoursnotice/api.go`, `internal/store/hoursnotice/api_test.go`, `internal/store/hoursnoticepg/store.go`, `internal/store/hoursnoticepg/store_test.go`, `migrations/0040_session_hours_notice.sql`, `docs/contracts/store.md` (the store interface cell of `docs/design/modules.md` names `internal/store/hoursnotice` in this design pull request)
Design: docs/design/calendar-ibkr-hours.md#decisions (Decisions 6, 7, 9)
Acceptance:
1. `internal/store/hoursnotice/api.go` (declarations only, as `bar` is to `barpg`; `internal/store/api.go` is not edited, it is at 800 lines; the Postgres implementation is the package `internal/store/hoursnoticepg`, so no jobs or pages package imports an implementation package) declares `Notice`, `Reader` (rows ordered by market and day) and `Store` (`Replace(ctx, market string, rows []Notice)`: deletes the market's rows and inserts the given ones in one transaction); its test checks the contract with a fake from both sides.
2. The migration creates `master.session_hours_notice`, the view `rd.session_hours_notice_r1` and grants to `ot_w_master` (INSERT, DELETE, SELECT) and `ot_ro_pages` (SELECT on the view only), with a Down; it changes no CHECK and no other table, and its number is the next free one at merge time (0040 today); the `pg` test reads the file by that name.
3. Store tests with the build tag `pg` replace, repeat and replace with fewer rows; a test without a database reads the SQL text for the grants.

### 6 job-calendar-hours

Module: jobs. Order: after 2, 3, 5.
Files: `internal/jobs/calendarhours/job.go`, `internal/jobs/calendarhours/job_us.toml`, `internal/jobs/calendarhours/job_jp.toml`, `internal/jobs/calendarhours/types.go`, `internal/jobs/calendarhours/job_test.go`, `docs/contracts/jobs/calendar_hours.md`, `docs/contracts/jobs.md` (the index)
Acceptance:
1. `New` takes a market (`XNYS` or `XTKS`), a `broker.Broker`, a `clock.SessionSource`, a `clock.IBKRHours`, a `hoursnotice.Store`, a `clock.Clock` and a logger; it imports no other jobs package and `scripts/arch --check` passes.
2. The job writes no dataset: its `Plan` returns one key of its own, `{master.session_hours_notice, <calendar id>}`, never `{master.session_day, ...}`, so `calendar_refresh` marking its key present at -30m cannot make the runner skip these jobs; the manifest ids are `calendar_hours_us` and `calendar_hours_jp`.
3. Tests with fakes and a fake clock: equal days write no row and log nothing; an IBKR `CLOSED` against our open day, an early close and a differing open time each log one line of Decision 6 and write one row; a day with no row of ours is not compared; a day that no longer differs is gone after the next run; a failed `ResolveContract` keeps the rows and the error names market and code; an unparsable string fails the run.
4. The only broker method the job calls is `ResolveContract`, and it holds no `SessionDayWriter` (read in `job.go`).
5. The manifests have the schedule of Decision 8 and `docs/contracts/jobs/calendar_hours.md` states Decisions 2 to 4 and is listed in the index `docs/contracts/jobs.md`.

### 7 pages-home-hours-notice

Module: pages. Order: after 5.
Files: `internal/pages/home_page/overview.go`, `internal/pages/home_page/compose.go`, `internal/pages/home_page/overview_test.go`, `internal/pages/home_page/compose_test.go`
Acceptance:
1. `Input` gains the rows (`[]hoursnotice.Notice`); `overview.Gaps` holds one `{card: "calendar", text}` per row (the comment of `overview.go` that says "overview.gaps holds only gaps no card shows" is updated), and `gapLine` and `gapline_test.go` are unchanged.
2. A test with two rows shows both in `overview.gaps`, and the page validates against its schema with no schema or web file changed; with no rows the list is empty as today.

### 8 ot-wire-calendar-hours

Module: ot. Order: after 6 and 7, and after entry 4 index-wire of `docs/design/option-capture-need.md` and entry 5 statement-wire of `docs/design/statement-account.md`, which edit the same console table `cmd/ot/jobconsole_jobs.go` and the same pinned counts in `jobconsole_jobs_test.go`. The issue of this entry carries a `Depends on` line for each of those two issues, and its counts are the ones after both: the table has 31 rows (27 at main, 28 after index-wire, 29 after statement-wire, 31 here), and every number below is read from the file after both have merged.
Files: `cmd/ot/wire_calendar.go`, `cmd/ot/runonce.go`, `cmd/ot/jobconsole_jobs.go`, `cmd/ot/jobconsole_jobs_test.go`, `cmd/ot/jobreg_test.go`, `cmd/ot/home_site.go` (the caller of `home_page.Compose`), `cmd/ot/site.go` (declares `siteReaders` and `openSiteReaders`), `cmd/ot/site_hours_test.go` (new), `cmd/ot/wire_test.go`, `docs/contracts/ot/jobs/calendar_hours_us.md` (new), `docs/contracts/ot/jobs/calendar_hours_jp.md` (new), `docs/contracts/ot.md`
Acceptance:
1. `calendar_hours_us` and `calendar_hours_jp` are registered with the broker, the loader session reader, `ibkrhours` and the `hoursnotice` store of the master line, the broker being the default socket as `earnings_refresh` reads it (`runonce.go:113`), never the account resolver, so the jobs stay on gateway 4001; each with its own unique `order` number (85 and 86, Decision 8); neither is `Force` in the job console, so `TestForceYesIsExactlyCalendarRefresh` stays as it is, and `jobconsole_jobs_test.go` has its row count, `TestTodayOnlyIsExactlyTheFourteen` (the name as at main; the entry keeps it and does not rename it; the today-only map gains `calendar_hours_us` and `calendar_hours_jp` and has 17 ids), `TestOnlyOptionEodJpIsInTheTokyoZone` and `TestEveryForceNoJobHasReason` (19 force-no jobs become 21, each new row with the reason code `not_proved` and its text) updated. The constructors are called from the new code in `wire_calendar.go`; `wire.go` (793 lines) is not edited, and `runonce.go`, `jobconsole_jobs.go` and `wire_calendar.go` each stay under 800 lines, read by `wc -l`.
2. `docs/contracts/ot/jobs/calendar_hours_us.md` and `calendar_hours_jp.md` each have the Order line and Version heading that `TestJobDocsExist` reads for its id, and `docs/contracts/ot.md` lists the jobs.
3. `siteReaders` gains the `hoursnotice.Reader` and `openSiteReaders` opens it with the pages line; the home site fills the `Input` field from it; a nil reader gives an empty list. Both are tested in the new `cmd/ot/site_hours_test.go` (`site_test.go`, 796 lines, is not edited).

## Checked by CI

`go test ./... -race -count=1` (it includes `TestJobDocsExist` and `TestJobOrderUnique`), `go run ./scripts/arch --check`, `go build ./...`, `buf lint` and the generated-output check, `uv sync --locked`, `ruff check`, `mypy --strict`, `pytest -n auto` in `pyside/`, the `pg`-tag store tests in GitHub CI.

## Left out

- Layer 2 (a check after the day, the IBKR hours against what happened), any automatic change of `master.session_day`, and a notice by Telegram.
- Refreshing XTKS rows daily (no job does it; the generator supports XTKS) and a horizon beyond what IBKR lists.
- More reference contracts, other markets, extended hours (`trading_hours`), the Tokyo lunch break as a separate check, and a tolerance setting.
- A `ListContracts`-style new rpc or a stand-alone hours rpc (Decision 1).
- A time-of-day guard: development mode, the job runs when triggered.

## Doubts

1. The year-end half days of 7203 show as an early close, the intended warning.
2. `contracts/fixtures/page.job_console.v1.golden.json` lists job rows, but no Go, TypeScript or Python file reads it by name (grep), so it is a fixture of the page schema and not of `jobTable`; entry 8 does not edit it.

## Adversarial

The answer file is `cih-adversarial.md` (the read of revision 1, with a live read-only probe).

1. Horizon of 7 days: adopted. IBKR lists about 4 to 5 days; the goal and Decision 4 say so and a day with no row of ours is "not compared".
2. Unnamed risk (who keeps XTKS fresh; US holidays blind the Tokyo check): adopted. XTKS rows are loaded by hand and this design says so; one job per market on its own calendar (Decision 8); the job has its own key `{master.session_hours_notice, <calendar id>}` (entry 6).
3. Simpler way (typed `overview.gaps`, no `gapLine` change): adopted (Decision 7, entry 7); the table has no `cleared_at` and is replaced per market (Decision 6).
4. Lock-in (field numbers, `r1` names, job ids, order numbers): accepted as low; the order numbers are unique (entry 8).
5. False claims: adopted. The role sentence is fixed (master line, not loader), the string uses `;` for days and segments, fixtures use `US/Eastern` and `Japan`, and the migration is one rule: the next free number at merge time, 0040 today.
6. Files misses: adopted. Entry 1 adds `resolve_proto_test.go`, entry 3 is first, entry 8 adds the job-console and registry tests and the three docs, and no job is force-allowed.
