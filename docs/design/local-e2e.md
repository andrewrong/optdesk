# Local check: `ot check scheduler|serve|pyside`, a read-only check per service that the owner runs on this machine

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-10
Revision: 1

Source: owner 2026-11-10: every service needs a local integration / end-to-end test, run by the owner on this machine to confirm features, not living in CI. The advisor agreed on the shape (A: `ot check`, read-only against the live deployment; B: a throwaway-database chain test, left out as its own later design).

Read on origin/main at ca8ce50: `AGENTS.md`, `docs/design/modules.md`, `docs/design/scheduler.md`, `cmd/ot/setup.go`, `cmd/ot/schedstatus.go`, `cmd/ot/page.go`, `cmd/ot/pagelist.go`, `cmd/ot/site.go`, `internal/store/api.go` (`GapReader`, `Expectation`, `Gap`, `SeriesReader`, `QuoteSnapshotReader`, `DSNEnvPages`, `DSNEnvRunner`), `migrations/0002`, `0003`, `0012`, `0013`, `0019`, `0026`, `pyside/tests/live/`, and the hand-run end-to-end of 2026-11-10 (the dispatcher scratchpad: e2e-plan.md, e2e-pages-diagnosis.md, cursor-diagnosis-2026-11-10.md; not in the repository).

Needs the owner: nothing. No new role, no permission change, no migration, no contract file.

## Goal

One command per service, `ot check scheduler`, `ot check serve`, `ot check pyside`, that the owner runs on this machine against the deployed system and that answers "does this feature work today" with one line per check, without writing anything. The checks must catch what the hand-run of 2026-11-10 found by reading code for hours: stale FRED days, expired gaps with no reason, report blocks never wired (4 of 5 `not_built`), a page 500 from a missing migration, an `-1` quote sentinel, a quote that is not from this session.

## Decisions

1. Command and output. `ot check <service> [--net]` in `cmd/ot`, service one of `scheduler`, `serve`, `pyside`. The first line prints `OT_HOME=<path>` and `pg=<host>/<database>` taken from the pages DSN (no user, no password, no query). Then one line per check: `PASS|FAIL|EXPLAINED <check> <as-of day> <reason>`. The exit code is 1 when any line is FAIL, else 0. A check that cannot run (no `OT_HOME`, no database, no socket) is a FAIL line, never a silent skip. Own implementation, because no library renders this contract; the result type is one small struct behind a function `report(w, Result)` so a library can replace it.
2. One database role. `ot check` connects only as the pages role `ot_ro_pages` (`internal/store/setup/setup.go:33`, `roleRoPages`) through `store.DSNEnvPages`; it never reads a runner, writer, loader or admin DSN. Acceptance 1 of issue 1 greps the non-test files `cmd/ot/check*.go` for DSN names. `ot scheduler status` reads the ledger as the runner and stays as it is; `ot check scheduler` does not call it.
3. No SQL of its own. Every database read goes through an existing store reader interface (`store.GapReader` for `Expectations` and `OpenGaps`, `store.SeriesReader`, `store.QuoteSnapshotReader`, `store.AccountReader`, `store.OptionEodReader`, `store.ReadingViewReader`, and the pages' own builders). A fact no reader gives is reported `EXPLAINED <check> - no view: <what is missing>`, and the design lists it under "Views" below. Adding a view needs a migration and a grant to `ot_ro_pages`; that is a permission change, so it is left out and goes to the owner as a separate design.
4. Anti-lie rules, each one a test.
   a. Expected datasets and days come from the scheduler's own windows (`rd.expectation_r1`, one row per dataset window, produced from the plan of the day), read through `GapReader.Expectations`; no dataset id and no key list is written in the check code. A dataset is PASS when `PresentDay` equals `ExpectedDay` (the view counts a day present only when every expected key is present or no_value). A day with no planned dataset prints `PASS (nothing planned) <day>` only when the view returns a row for that day with `Closed` true (a day without a session); a day with no row at all means the scheduler never walked it and is FAIL (`no window rows`), as is an empty result for the last 7 calendar days (a dead or never started scheduler writes no rows, so it must not look green). The check never prints a bare PASS for an empty plan. The range is the last 7 calendar days through today, in the zone of `OT_ZONE`.
   b. EXPLAINED is allowed only for a row that carries one of the reasons this design names: `window_open` (the window of the dataset has not closed: `DueBy` is after now; the window of the FRED job already includes its 96 hour lateness, so a FRED day missing inside it is `window_open` and the cursor holds by design, `scheduler.md` cursor rule), `closed_day` (the view's `Closed` is true), and `no_view` (Decision 3). A gap row with any other reason, an empty or older `PresentDay` after the window closed (the stale case: `PresentDay` is the latest complete day on or before `ExpectedDay`, so older than `ExpectedDay` is the usual failure), or a gap with an empty reason is FAIL. An unexplained empty is FAIL.
   c. Every PASS and EXPLAINED line carries the as-of day: the data day the line is about (`ExpectedDay`, the page's data day, the quote's session day), not the day the command ran.
   d. Pages: for each page in the table "Blocks that must be built" the check reads each block's `not_built` and `gap` state and compares it with that table. A block the table marks `built` and that is `not_built` or has any gap is FAIL (today four of the five report blocks: the report blocks exist since #1022 but `cmd/ot/wire.go` and `report.go` pass only the risks block); a block the table marks `permanent` is EXPLAINED with the reason named in the table; a block in neither list is FAIL ("unknown block"), so a new block cannot hide.
   e. Serve: each route answers 200; the page body, captured by the check's own `NewHandlerFunc` handed to `siteHandler`, is valid against the schema version the page declares in its meta, loaded from `contracts/schemas` of the repository checkout (flag `--repo-dir`, default `.`; `santhosh-tekuri/jsonschema/v6` is a direct dependency used so far only in tests, so this is its first non-test use and is named in the pull request; the binary embeds no schema); the number of blocks equals the number the table "Blocks that must be built" gives for that page (the schema has no required-block list); a build error is a FAIL line that names the page and the error text (a 500 from a missing migration reads as that error, not as a diagnosis); the report route needs `OT_ZONE` and the demo profile is refused, both FAIL; one freshness line per page compares the page's data day with the latest `ExpectedDay` of a closed window and is FAIL when older by more than the window's lateness. Routes are called on the in-process handler (`siteHandler`); no port is opened, so a check never collides with a running `ot serve`.
   f. Pyside: `ot check pyside` runs `uv run pytest tests/live --junitxml=<temp>` in `pyside/` with `OT_LIVE_PYSIDE=1` and `OT_HOME` set, reads the junit file (not the text), and prints one line per test: passed is PASS, failed or error is FAIL, skipped is FAIL (a skipped live test means the switch, `OT_HOME` or the gateway health port is wrong; issue 6 makes the existing health test fail with a message instead of skipping, and the command passes `OT_IBKR_HEALTH_PORT` through from the owner's environment), with the test's own as-of day from its recorded property. The tests under `pyside/tests/live` assert fresh data: the quote timestamp is inside the latest session of its market, the bars end on the latest trading day, and the index quote has no `-1` sentinel (this is the pyside module's issue). The tests stay read-only, as `AGENTS.md` allows.
   g. FRED: with `--net` only, `ot check scheduler` compares the latest stored point of each FRED series (`SeriesReader`) with the latest public observation from the FRED API (`internal/sources` FRED source, key from `FRED_API_KEY` as the scheduler loads it); a stored day older than the public day is FAIL, unless the dataset's window is still open, which is EXPLAINED `window_open`. Without `--net` the command makes no network call, and prints one line `EXPLAINED fred.public <day> offline: pass --net`. `--net` only reads.
5. Time. The command takes `now` from the existing `a.deps.newFwClock()` clock and the zone of `OT_ZONE`; tests use the fake clock.
6. The check is a command, not a service: `ot check` starts no server, writes no file outside a temporary directory (the junit file), and is run by the owner by hand. It is not run in CI and `AGENTS.md` is not changed; its unit tests use fakes for every reader and never touch the deployment.

## Modules

No new row of `docs/design/modules.md`. All of `ot` is `cmd/**` (row ot, depends cell already holds store, pages, sources, clock, contracts), the pyside tests are `pyside/**` (row pyside). New files are under 600 lines; `cmd/ot/check_scheduler.go` and `cmd/ot/check_blocks.go` are the likeliest to grow, so each has its own file. `cmd/ot/site_test.go` is 796 lines and is not in `docs/design/ratchet.json`, so it cannot grow: tests of the serve check live in `cmd/ot/check_serve_test.go`, and `site.go` is not edited. `cmd/ot/wire.go` (426 lines) gets only the one registration line in `main.go` (95 lines).

## Interface

No interface changes: no `api.go`, no `contracts/` file, no `docs/contracts/store.md` entry. The check reads through the interfaces that exist.

Views (checked against `migrations/` on main). Readable by `ot_ro_pages` today: `rd.job_gap_r1` and `rd.expectation_r1` (0012), `rd.series_point_r1` (0002), `rd.reading_r1` (0013), `rd.quote_snapshot_r1`, `rd.option_eod_r1`, `rd.price_bar_r1`, `rd.account_capture_r1`, `rd.account_position_r1`, `rd.account_value_r1` (0019), `rd.account_position_mark_r1` (0026), `rd.rule_param_r1`, `rd.earnings_date_r1`. Missing, each reported `EXPLAINED ... no view` until the owner decides:

- scheduler cursor (`ops.scheduler_state`, runner only): the check cannot print the cursor day; it prints the oldest open `expected_day` of `Expectations` instead.
- the expected key list of a day (`ops.capture_expected_key`, runner only): the check is dataset-level through `rd.expectation_r1`, which already requires every key present or no_value; it cannot name which key is missing (`ot scheduler status` and run-once outputs do).
- run rows and reasons (`ops.run`): a gap's cause (why `account_capture` expired on 2026-11-09) is not visible; only the gap reason.
- `master.session_day` (granted to the runner only, 0004): the calendar check is read from the windows' `Closed` flag, not from the calendar.
- the settings POST (`/owner-settings`) is not built; the check of `/settings` is GET only.

### Blocks that must be built

Source: `docs/design/builders-wave.md`, `market-page-producer.md`, `report-page-producer.md` and the diagnosis of 2026-11-10. The file `cmd/ot/check_blocks.json` (embedded, read by `check_blocks.go`) is this table; the table is the design's list, the file is a copy that the acceptance checks against the names here.

| Page | built (a gap is FAIL) | permanent (EXPLAINED, reason) |
|---|---|---|
| report_page (4 variants: us/jp, open/close) | the five blocks, today only `risks` is wired | jp `actions`: `jp_slice_missing` (builders-wave Decision 9); `global_assets`: `no_producer` |
| market_page | VIX term cells, SPY IV rank | index IV, rv20 and iv-rv20 cells: `not_built_by_design` (market-page-producer); 200 day chart, breadth, N225 volatility: `permanent_gap` |
| macro_page | rates series, ease | events: `no_producer` |
| option_candidates, option_notices, option_tracking, option_data, settings | all blocks | none |

A block name in this table that the page does not produce, or a block of the page that the table does not name, is FAIL; so the table is edited in the same pull request as any block change.

## Issues

Every test calls `t.Parallel()` (Python: `pytest -n auto`), writes files only under a temporary directory, uses the fake clock, never sleeps, opens no port, and touches no live broker, socket or database. Acceptance is checked offline by reading files and `git`; runs are under `## Checked by CI`.

### 1 local-check-core

Module: ot. Size: M.
Files: `cmd/ot/check.go`, `cmd/ot/check_test.go`, `cmd/ot/main.go`, `docs/contracts/ot.md`
Design: docs/design/local-e2e.md#Decisions
Order: first.
Goal: the command `ot check <service> [--net]`, the `Result` line `PASS|FAIL|EXPLAINED <check> <as-of day> <reason>`, the header line, the exit code 1 on any FAIL, the pages-DSN-only opener, and the rule that an EXPLAINED line must name one of the three reasons of Decision 4b.
Acceptance:
1. `ls cmd/ot/check*.go | grep -v _test.go | xargs grep -ho "DSNEnv[A-Za-z]*\|OT_PG_[A-Z_]*" | sort -u` prints only `DSNEnvPages`, and `grep -c "envFile\|openSchedulerLedger\|OT_PG_ADMIN" cmd/ot/check_scheduler.go cmd/ot/check_serve.go cmd/ot/check_net.go cmd/ot/check_pyside.go` is 0 for each file (one helper in `check.go` reads the pages DSN; no other file opens a store by a helper that reads another role).
2. `grep -c "check" cmd/ot/main.go` is at least 1, `docs/contracts/ot.md` names `ot check`, and `wc -l cmd/ot/check.go` is under 600.
3. `grep -c "t.Parallel()" cmd/ot/check_test.go` equals the number of `func Test` in it, and it holds tests named `TestCheckHeaderNoSecret`, `TestCheckExitOnFail`, `TestCheckExplainedNamed` and `TestCheckPagesDSNOnly`.
4. `grep -c "EXPLAINED" cmd/ot/check.go` is at least 1 and the three reasons `window_open`, `closed_day`, `no_view` each occur at least once in `cmd/ot/check.go`.

### 2 local-check-scheduler

Module: ot. Size: M.
Files: `cmd/ot/check_scheduler.go`, `cmd/ot/check_scheduler_test.go`
Design: docs/design/local-e2e.md#Decisions
Order: after 1.
Goal: `ot check scheduler`: one line per dataset window of the last session days from `GapReader.Expectations` and one per open gap from `OpenGaps`, with the rules of Decision 4a to 4c, and the `no_view` lines of the Views list for the cursor, the key list and the run reasons.
Acceptance:
1. `grep -c "Expectations(" cmd/ot/check_scheduler.go` is at least 1 and `grep -c "OpenGaps(" cmd/ot/check_scheduler.go` is at least 1.
2. `grep -ci "select \|insert \|database/sql\|pgx" cmd/ot/check_scheduler.go` is 0 (no SQL of its own), and `grep -c "\"fred_series\"\|\"option_eod\"\|\"account_capture\"" cmd/ot/check_scheduler.go` is 0 (no dataset or job list in the check).
3. `cmd/ot/check_scheduler_test.go` holds tests named `TestCheckSchedNothingPlanned`, `TestCheckSchedNoRows`, `TestCheckSchedUnexplainedGap`, `TestCheckSchedWindowOpen`, `TestCheckSchedAsOfDay` and `TestCheckSchedNoView`; `wc -l` of both files is under 600.

### 3 local-check-net

Module: ot. Size: S.
Files: `cmd/ot/check_net.go`, `cmd/ot/check_net_test.go`
Design: docs/design/local-e2e.md#Decisions
Order: after 2 (it adds lines to `ot check scheduler`).
Goal: the `--net` lines: FRED latest stored point against the FRED public latest observation through `internal/sources`, `window_open` while the window is open, FAIL after it; without `--net` one `EXPLAINED fred.public` offline line and no network call.
Acceptance:
1. `grep -c "net/http" cmd/ot/check_net.go` is 0 (the call goes through `internal/sources`), and `grep -c "\-\-net\|\"net\"" cmd/ot/check_net.go` is at least 1.
2. The test file holds tests named `TestCheckNetOffline`, `TestCheckNetStale` and `TestCheckNetWindowOpen`, and the offline test passes a source that fails the test when called (`grep -c "t.Fatal" cmd/ot/check_net_test.go` is at least 1).

### 4 local-check-serve

Module: ot. Size: M.
Files: `cmd/ot/check_serve.go`, `cmd/ot/check_blocks.go`, `cmd/ot/check_blocks.json`, `cmd/ot/check_serve_test.go`
Design: docs/design/local-e2e.md#Decisions
Order: after 1.
Goal: `ot check serve`: every route of `siteHandler` on the in-process handler, status 200, schema valid, block count equal to the schema's required blocks, one freshness line per page, and the block comparison of Decision 4d against `check_blocks.json`.
Acceptance:
1. `cmd/ot/check_blocks.json` names every page and every block of the table "Blocks that must be built" of this design, and each permanent block carries its reason (`grep -c "jp_slice_missing\|no_producer\|not_built_by_design\|permanent_gap" cmd/ot/check_blocks.json` is at least 4).
2. `grep -c "httptest\|siteHandler" cmd/ot/check_serve.go` is at least 1 and `grep -c "ListenAndServe\|net.Listen" cmd/ot/check_serve.go` is 0 (no port).
3. `git diff --name-only BASE -- cmd/ot/site.go cmd/ot/site_test.go cmd/ot/wire.go` is empty (`BASE` is `git merge-base HEAD origin/main`).
4. The test file holds tests named `TestCheckServeRoutes`, `TestCheckBlockNotBuilt`, `TestCheckBlockUnknown` and `TestCheckServeStaleDay`; every file is under 600 lines.

### 5 local-check-pyside

Module: ot. Size: S.
Files: `cmd/ot/check_pyside.go`, `cmd/ot/check_pyside_test.go`
Design: docs/design/local-e2e.md#Decisions
Order: after 1.
Goal: `ot check pyside`: runs `uv run pytest tests/live` with `OT_LIVE_PYSIDE=1` in the pyside directory (flag `--pyside-dir`, default `pyside`), reads the junit file, prints one line per test; skipped is FAIL.
Acceptance:
1. `grep -c "OT_LIVE_PYSIDE" cmd/ot/check_pyside.go` is at least 1 and `grep -c "junitxml" cmd/ot/check_pyside.go` is at least 1.
2. The command is started through a function field on the app (the test replaces it), so the test file runs no `uv`: `grep -c "exec.Command" cmd/ot/check_pyside_test.go` is 0.
3. The test file holds tests named `TestCheckPysideSkipFails`, `TestCheckPysideJunit` and `TestCheckPysideNoSwitch`.

### 6 live-freshness-asserts

Module: pyside. Size: S.
Files: `pyside/tests/live/test_live_fresh.py`, `pyside/tests/live/test_live_pyside.py`, `pyside/tests/live/client.py`
Design: docs/design/local-e2e.md#Decisions
Order: independent of 1 to 5.
Goal: live tests that assert fresh data, read-only: the quote timestamp of SPY is inside the latest session (or the after-hours window of it), the SPY and N225 bars end on the latest trading day, the N225 index quote has no `-1` bid or ask, the option_eod open interest of a SPY put is present; `test_live_health_gateways` fails with a message when `OT_IBKR_HEALTH_PORT` is unset, instead of `pytest.skip`; each test records its as-of day as a junit property (`record_property("as_of", ...)`).
Acceptance:
1. `pyside/tests/live/test_live_fresh.py` holds tests named `test_live_quote_in_session`, `test_live_bars_latest_day`, `test_live_n225_no_sentinel` and `test_live_oi_present`, and `grep -c "record_property" pyside/tests/live/test_live_fresh.py` is at least 4.
2. `grep -c "OT_LIVE_PYSIDE" pyside/tests/live/test_live_fresh.py` is 0 (the switch stays in `conftest.py`), and `grep -ci "place_order\|PlaceOrder\|cancel" pyside/tests/live/test_live_fresh.py` is 0.
3. `git diff --name-only BASE -- pyside/tests/live/allowed.py` is empty, and `grep -c "pytest.skip" pyside/tests/live/test_live_pyside.py` is 0.
4. The file contains no `sleep` (`grep -c "sleep" pyside/tests/live/test_live_fresh.py` is 0) and no absolute path.

## Checked by CI

`go test ./cmd/ot/...` (issues 1 to 5), `go vet`, `golangci-lint`, `go-arch-lint` (the check files import only the roots in the ot row's depends cell), `ruff check`, `mypy --strict` and `pytest -n auto` in `pyside/` (issue 6: `tests/live` stays skipped without `OT_LIVE_PYSIDE=1`, only `test_guard.py` runs). The owner's run of `ot check scheduler|serve|pyside` against the deployment is not CI and is not an acceptance item.

## Left out

- B: the throwaway-database chain test under `internal/e2e` with `//go:build e2e` (fake clock, recorded broker and FRED fixtures): its own later design.
- Browser checks (rendered pages, screenshots): not designed.
- Any new view, role or grant for `ot_ro_pages` (cursor, expected keys, run reasons, session day): the "Views" list above; the owner decides in a separate design.
- A check that writes (run-once, backfill, setup, migrate): `ot check` is read-only by construction.
- Checks of `option_ledger`, notify and alerts delivery: no reader in pages' reach; add when the views exist.

## Adversarial

Read by a fresh general-purpose sub-agent given only this file and the repository, on origin/main ca8ce50; its answers to the six questions are in the dispatcher scratchpad (local-e2e.adversarial.md, not in the repository). Adopted: a dead scheduler writes no window rows, so "nothing planned" is PASS only for a row with `Closed` true, and a missing row or an empty last week is FAIL (Decision 4a); `fred_window` dropped, because the FRED window already includes its 96 hours (Decision 4b, 4g); an older `PresentDay` is the usual stale case (4b); `rd.account_position_mark_r1` is granted to pages (0026), so the false "no view" line is removed; the pages role is in `internal/store/setup/setup.go`; the schema is loaded from the checkout, the block count comes from the table, not from the schema, and the body is captured by the check's own handler (4e); the live tests must not skip, and the health test is changed (4f, issue 6); acceptance 1 of issue 1 also bans the helpers that read another role. Not adopted: letting `ot check scheduler` call the runner-role status for the cursor (it breaks the advisor's one-role rule; the cost is the listed `no view` lines); dropping `check_blocks.json` for an allowlist in code (the table is the design's list and the advisor asked for it); the risk that the table rots is kept and named: it is edited in the same pull request as any block change.
