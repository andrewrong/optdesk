# Tracking page session days: the pages role reads the view rd.session_day_r1 and ot serve opens the reader

Author: the optdesk designer
Owner approved: (pending)
Date: 2026-11-10
Revision: 2

Source: the option tracking page served by `ot serve` and `ot page` (#1138) shows the gap `no_sessions`.

Read on origin/main at a31ebb7 (after `git fetch`; revision 1 was read at 9e0a202; origin/main moved to 8f2376d, which changes `cmd/ot/dryrun.go` and `runonce.go` only; every line below was read again): `docs/design/modules.md`, `cmd/ot/site.go` (`siteReaders.sessions`, `openSiteReaders`, `openOptionalStores`, `openOwnerReader`, `withPositionsRules`), `cmd/ot/site_positions.go` lines 93 and 102, `cmd/ot/tracking_site.go` (`trackingReadersOf`, `buildTracking`, line 77 of `ot page`), `cmd/ot/site_test.go` (796 lines, `TestSiteStoresClosed` with `range 8`), `cmd/ot/main_test.go` (`testApp` sets no `openSessionDay`), `cmd/ot/runonce.go` (`openSessionDays`), `internal/store/sessionday/reader.go`, `writer.go`, `sessionday_test.go` and `sessionday_pg_test.go`, `internal/pages/option_tracking_page/context.go` (`gather`, `sessionDays`), `migrations/0001`, `0002`, `0003` (line 87), `0004`, `0005`, `0008`, `0010`, `0019` (line 4), `0025`, `docs/contracts/ot.md` lines 21 and 154, `docs/contracts/store.md` lines 5, 37, 63 and 64, `docs/design/ratchet.json`. Not re-read in full: `docs/design/option-tracking.md` and `docs/contracts/pages.md`; the page behaviour used below is read from the page code.

## Problem

Two things are missing, not one. (a) The role `ot_ro_pages` has no `USAGE` on schema `master` and no `SELECT` on `master.session_day` (0004 grants `ot_runner`, 0005 `ot_loader`, nothing else). (b) `ot serve` never opens a session-day reader with the pages line: `siteReaders.sessions` is never set (`withPositionsRules` says "ot has no store for them yet"), so `trackingReadersOf` hands `nil` to the page, which names the gap `no_sessions`. A grant alone changes nothing; wiring alone fails with a permission error.

## Decision

Give the pages role a view, not the table, and wire the existing reader. Decided by the advisor (adopted by the dispatcher, 2026-11-10).

- Migration `0028_session_day_view.sql` creates the view `rd.session_day_r1` with the six columns of the table (`calendar_id, session_day, is_session, open_at, close_at, early_close`, the names of `migrations/0001`), owned by `ot_owner` like every other `rd` view, and grants `SELECT` on it to `ot_ro_pages`, `ot_runner` and `ot_loader`. The rule is `migrations/0003` line 87 ("ot_ro_pages reads views of rd only") and `0019` line 4: `ot_ro_pages` gets nothing in schema `master`.
- `ot_ro_pages` and `ot_runner` already hold `USAGE` on `rd` (`0002` line 57). `ot_loader` does not (`git grep "ON SCHEMA rd"` on migrations has no row for it), so `0028` also grants `USAGE ON SCHEMA rd TO ot_loader`. `ot_loader` needs the view because the job `calendar_refresh` reads the stored days through the loader store (`cmd/ot/runonce.go` `openSessionDays`, `internal/jobs/calendarrefresh/job.go` line 108).
- `sessionday.Reader` reads the view for every role; the writer keeps the table. One `FROM` of `reader.go` changes. `store.SessionDayReader`, `store.OpenSessionDayFunc` and `option_tracking_page.Config.Sessions` are unchanged, so no interface changes and no page code changes.
- The earlier claim that `0008` is a precedent for a direct master grant is dropped: `0008` gives `ot_ro_pages` `SELECT` on `master.instrument` tables but no `USAGE` on `master`, so that grant is dormant; the pages read names through `rd.instrument_r1` (`0010`).
- The view holds the calendar only, no secret and no personal data.

Deploy order: migrate first, then the binary. If the new `ot serve` runs before `0028` is applied, `sessionday.Open` still succeeds (it only does `pgxpool.New` and `Ping`), and `SessionDays` fails on each request with "relation does not exist"; `gather` returns that error, so `/options/tracking` answers 500 `internal error` until the migration is applied. That is the page contract (`docs/contracts/ot.md` line 154: "A missing reader is the page's own gap, never a failure"; a reader that errors is a failure) and it is the same as for every other `rd` view. Issue 2 therefore does not turn a reader error into the gap `no_sessions`: that would show a missing migration as an empty calendar and would need a change in `internal/pages` (another module). The window runs `ot setup pg` and migrate before it replaces the binary.

Effect on other pages: `rd.sessions` also goes to the structures page (list, detail) and `option_result_page` (`cmd/ot/site_positions.go` lines 93 and 102). Their builders gate on `Ledger == nil || Sessions == nil`, and `rd.ledger` and `rd.account` are never set by `openSiteReaders` (the ledger goes to `optionLedger` only), so those pages stay `no_reader` and read no session day today. The effect is latent: whoever sets `rd.ledger` and `rd.account` later makes them read `rd.session_day_r1`, which the pages role is granted by this design.

The fix is two issues, in two modules (store, ot), no interface change, no contract change, no new dependency.

## Modules

No row of `docs/design/modules.md` changes. Row `store` holds `internal/store/**`, `migrations/**` and `docs/contracts/store.md`, so the migration, the reader and the store contract are one module and one issue; row `ot` holds `cmd/**` and `docs/contracts/ot.md`.

## Interface

None. `store.SessionDayReader`, `store.OpenSessionDayFunc` and `option_tracking_page.Config.Sessions` are unchanged.

## Issues

Every test calls `t.Parallel()`, writes files only under a temporary directory, uses a fake clock, never sleeps, opens no port, and touches no live broker or database (a test that needs PostgreSQL carries the tag `pg`). Acceptance is checked offline by reading, grep and git (`BASE` is `git merge-base HEAD origin/main`); test runs are under "Checked by CI". Test names are under 32 characters.

### 1 session-day-view

Module: store. Size: S.
Files: `migrations/0028_session_day_view.sql`, `internal/store/sessionday/reader.go`, `internal/store/sessionday/sessionday_test.go`, `internal/store/sessionday/sessionday_pg_test.go`, `internal/store/setup/session_day_view_pg_test.go`, `docs/contracts/store.md`
Design: docs/design/tracking-session-days.md#Decision
Order: first. The number `0028` is the next after `0027` on origin/main at a31ebb7 and no open pull request touches `migrations/` (`gh pr list --json number,files` at cut time); the worker takes the next free number if that changed and says so in the pull request.
Goal: the pages, runner and loader roles read the session days through `rd.session_day_r1`, and `ot_ro_pages` holds nothing in schema `master`.
Acceptance:
1. `migrations/0028_session_day_view.sql` has `-- +goose Up` and `-- +goose Down`, and its Up part holds the line `CREATE VIEW rd.session_day_r1 AS`, the line `SELECT calendar_id, session_day, is_session, open_at, close_at, early_close`, the line `FROM master.session_day;` and the line `ALTER VIEW rd.session_day_r1 OWNER TO ot_owner;` (`grep -c` of each is 1).
2. `grep -c "^GRANT" migrations/0028_session_day_view.sql` is 2: `GRANT SELECT ON rd.session_day_r1 TO ot_ro_pages, ot_runner, ot_loader;` and `GRANT USAGE ON SCHEMA rd TO ot_loader;`. `grep -c "^GRANT.*master" migrations/0028_session_day_view.sql` is 0, and `grep -c "ot_w_" migrations/0028_session_day_view.sql` is 0.
3. Its Down part takes back what Up made and no more: `grep -c "^DROP VIEW rd.session_day_r1;" migrations/0028_session_day_view.sql` is 1 and `grep -c "^REVOKE USAGE ON SCHEMA rd FROM ot_loader;" migrations/0028_session_day_view.sql` is 1; `grep -c "^REVOKE" migrations/0028_session_day_view.sql` is 1 (the `USAGE` of `ot_ro_pages` and `ot_runner` on `rd` came from earlier migrations and stays).
4. `git diff --name-only BASE -- migrations` lists only the new file, and the number of that file is greater than that of every other file of `git ls-tree --name-only BASE migrations/`.
5. `grep -c "FROM rd.session_day_r1" internal/store/sessionday/reader.go` is 1 and `grep -c "master.session_day" internal/store/sessionday/reader.go` is 0; `git diff --name-only BASE -- internal/store/api.go internal/store/sessionday/writer.go` is empty.
6. `internal/store/sessionday/sessionday_test.go` expects the view in its select text: `grep -c "FROM rd.session_day_r1" internal/store/sessionday/sessionday_test.go` is 1 and `grep -c "FROM master.session_day" internal/store/sessionday/sessionday_test.go` is 0.
7. `internal/store/sessionday/sessionday_pg_test.go` applies the new migration: `grep -c "0028_session_day_view.sql" internal/store/sessionday/sessionday_pg_test.go` is at least 1, and it makes `ot_owner` the owner of `master.session_day` before that (`grep -c "OWNER TO ot_owner" internal/store/sessionday/sessionday_pg_test.go` is at least 1), because the view runs with the rights of its owner.
8. `internal/store/setup/session_day_view_pg_test.go` starts with the line `//go:build pg`, has `func TestPagesReadSessionDayView`, `func TestPagesNoMasterUsage` and `func TestRunnerLoaderReadView`, and its text holds `has_schema_privilege`, `rd.session_day_r1` and `ot_ro_pages` (`grep -c` of each is at least 1). `TestPagesNoMasterUsage` asserts that `has_schema_privilege('ot_ro_pages', 'master', 'USAGE')` is false and that a `SELECT` of `master.session_day` as `ot_ro_pages` fails; `TestPagesReadSessionDayView` asserts that `ot_ro_pages` can `SELECT` the view and cannot `INSERT`, `UPDATE` or `DELETE` it; `TestRunnerLoaderReadView` asserts the `SELECT` for `ot_runner` and `ot_loader`.
9. `docs/contracts/store.md` names the view and its three readers (`grep -c "rd.session_day_r1" docs/contracts/store.md` is at least 1), says that `ot_ro_pages` has no `USAGE` on `master`, and the line of `ot_loader` grants lists `GRANT USAGE ON SCHEMA rd TO ot_loader;` as given by `0028`.

### 2 tracking-session-days-wire

Module: ot. Size: S.
Files: `cmd/ot/site.go`, `cmd/ot/tracking_site.go`, `cmd/ot/tracking_site_test.go`, `cmd/ot/site_sessions_test.go`, `docs/contracts/ot.md`
Design: docs/design/tracking-session-days.md#Decision
Order: after issue 1 (merge 1, then 2; at deploy the window migrates before it replaces the binary, as the Decision says).
Goal: `openSiteReaders` opens `deps.openSessionDay` once with the pages line (`OT_PG_DSN_PAGES`), tracked and closed with the other stores, and sets `siteReaders.sessions` in the real profile; `ot serve` and `ot page` then give the tracking page its session days.
Acceptance:
1. `cmd/ot/site.go` has `func openSessionReader` (one function, like `openOwnerReader`) whose body holds the line `a.deps.openSessionDay != nil` and the call `a.deps.openSessionDay(ctx, dsn)` (`grep -c "a.deps.openSessionDay" cmd/ot/site.go` is 2), with the same `dsn` variable as `openOptionCatalog`; `grep -c "runnerDSN\|loaderDSN" cmd/ot/site.go` is 0. A nil `deps.openSessionDay` gives a nil reader and no error (`testApp` leaves it nil, and `TestSiteStoresClosed` counts 8 opens for that reason).
2. The opened store goes through `track(` on the line after the call, so `closeAll` closes it with the others; `openSiteReaders` calls `openSessionReader` and its failure goes to `fail()`.
3. The wiring is on the real path only: `git diff BASE -- cmd/ot/site.go | grep -c "^+.*sessions: sessions"` is 1, and `git diff BASE -- cmd/ot/site.go | grep -c "^[+-].*demoTrackingLedger"` is 0 (the demo literal is untouched, and the demo profile keeps the gap `no_sessions`).
4. The comments say what is true: `grep -c "ot has no store for them yet\|session readers stay nil" cmd/ot/site.go` is 0, `grep -c "no role of the pages line reads master.session_day\|nil today" cmd/ot/tracking_site.go` is 0, and `grep -c "Sessions is rd.sessions, opened" cmd/ot/tracking_site.go` is 1.
5. The path from the store to the page is the existing one: `grep -c "Sessions: rd.sessions" cmd/ot/tracking_site.go` is 1 and `grep -c "trackingReadersOf(rd)" cmd/ot/tracking_site.go` is 2 (`ot serve` and `ot page`); `git diff --name-only BASE -- internal/pages internal/store/api.go` is empty.
6. The new tests are in the new file `cmd/ot/site_sessions_test.go` (`cmd/ot/site_test.go` is 796 lines and is not in `docs/design/ratchet.json`, so `git diff --name-only BASE -- cmd/ot/site_test.go` is empty). It has `func TestSiteOpensSessionDay` (a fake `openSessionDay` sees the pages DSN, and `openSiteReaders` returns it in `rd.sessions`), `func TestSiteSessionOpenFails` (a failing open closes the stores opened before it and returns the fixed error text) and `func TestSiteSessionNilOpener` (a nil opener leaves `rd.sessions` nil); each calls `openSiteReaders(` (`grep -c "openSiteReaders(" cmd/ot/site_sessions_test.go` is at least 3), and the file has no `time.Sleep`. The file stays under 600 lines.
7. `cmd/ot/tracking_site_test.go` has `func TestTrackingReadersSessions`, which builds a `siteReaders` with a fake `sessions` and checks `trackingReadersOf` hands that same reader on (`grep -c "trackingReadersOf(" cmd/ot/tracking_site_test.go` is at least 1). The page is faked there, so this test proves the hand-over only; the wiring of acceptance 5 is what shows the reader reaches the page.
8. `docs/contracts/ot.md` no longer says the reader is nil or counts eight stores: `grep -c "session-day reader is nil" docs/contracts/ot.md` is 0 and `grep -c "eight stores" docs/contracts/ot.md` is 0; line 21 lists the session-day store among the stores opened with `OT_PG_DSN_PAGES` and gives no count; line 154 says the page reads the session days through `rd.session_day_r1` (`grep -c "rd.session_day_r1" docs/contracts/ot.md` is at least 1), that the demo profile keeps the gap `no_sessions`, and that before migration `0028` is applied the page answers 500 `internal error` (`grep -c "until migration 0028 is applied" docs/contracts/ot.md` is 1).

## Checked by CI

- `go test` of `internal/store/sessionday` and `cmd/ot`, `golangci-lint`, `go-arch-lint` (no change to `.go-arch-lint.yml` or `arch.toml`), and the `pg` tests of `internal/store/sessionday` and `internal/store/setup`.
- After the merge of both issues, the window runs `ot setup pg`, migrate, and `ot setup pg` again, then replaces the binary (this order is the deploy rule of the Decision).

## Left out

- The demo profile keeps the gap `no_sessions`: it opens no store and its literal is not changed (issue 2, acceptance 3). A demo session-day reader is a separate decision.
- A real page shows days only when `master.session_day` has rows: they come from `ot calendar load` (the loader role) and the calendar id the page asks for (`Config.CalendarID`) must be the one loaded. Loading `XNYS` is the window's step after the deploy; this design does not check the id (not verified, see the answer in `## Adversarial`).
- Turning a session-day reader error into the gap `no_sessions`: not done (Decision, deploy order); it needs a change in `internal/pages`.
- The structures page and `option_result_page` read `rd.sessions` too and stay `no_reader` until `rd.ledger` and `rd.account` are set (Decision, effect on other pages). Their tests and gaps are not changed here.
- Reads of other calendars than the page asks for: unchanged.
- Updating the deployed database: `ot setup pg` and migrate are the window's, after the merge of issue 1.

## Adversarial

The full answer is in the dispatcher scratchpad (tracking-session-days.adversarial.md, not in the repository). Six points, each answered here.

1. Gap against the goal (demo keeps `no_sessions`; no rows without `ot calendar load`; vacuous test of acceptance 6): adopted. Both facts are in Left out; the vacuous test is replaced by a check on the wiring (issue 2, acceptances 3, 5 and 7) and by tests that call the real `openSiteReaders` in a new file.
2. Deploy order (the open does not fail; the page 500s per request) and the other pages: adopted for the facts, not adopted for the proposed degrade. The Decision states the 500 and the order (migrate first). Turning the reader error into `no_sessions` would hide a missing migration as an empty calendar and needs a change in `internal/pages`, which is outside issue 2's module. The effect on the structures and option result pages is stated as latent. The nil guard on `deps.openSessionDay` is issue 2, acceptance 1; the stale assertion `f.got.Sessions != nil` of `TestTrackingRouteServesPage` stays true (its `rd` has no sessions) and is not touched.
3. Simpler way (a view matches every other pages grant): adopted, and it is now the Decision (advisor, 2026-11-10). The cost is one migration and one `FROM`, not a second reader.
4. Lock-in (a direct table grant contradicts `0019` line 4 and `0003` line 87): adopted by removing the direct grant; the pg test asserts `ot_ro_pages` has no `USAGE` on `master`.
5. False claims (the `0008` precedent; "the open fails"; "eight stores" to "nine"): adopted. The precedent is dropped, the open is described as it is, and the count is removed from `docs/contracts/ot.md` (`openSiteReaders` opens seven required stores and up to five optional ones, so any number goes stale; acceptance 8).
6. Likeliest slip (size of `site_test.go`; unreliable greps; renumbering; contradictory Order): adopted. New tests are in the new file `cmd/ot/site_sessions_test.go`; the greps of acceptances 3 and 4 match added or removed diff lines and comment texts only; the number `0028` is fixed at cut time and the worker reports a change instead of renumbering the design; the Order line is one rule.

## Consultation

Second opinion: the owner waived it; the advisor answered it (2026-11-10). Each issue above carries two lines when it is filed: `Second opinion: waived by the owner — the advisor answered it (2026-11-10)` and `Advisor:`, which the advisor gives on the merged head of this design, not before.
