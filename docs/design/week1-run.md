# Week 1 run: the slice against a real database

Author: advisor of optdesk (first draft)
Source: `## Left out` of `docs/design/week1-ot.md` (applying the migrations and loading `master.session_day`), the clock
row of `docs/design/modules.md` ("a yearly generated file, cross-checked against IBKR trading hours") and ADR 0004
(open and close times come from a maintained calendar, never from hand-written times). Identifiers were read on main at
3cdfdd1: `cmd/ot` has the subcommands `run-once`, `page`, `serve` and `setup pg`; `migrations/` holds 0001 to 0004
(0004 is the grant of PR 61); `internal/store/setup` is on the import list of the row `ot` and already uses pgx.

Goal of the design: after `ot setup pg`, the owner can run `ot migrate` and have `ot calendar load` ready for the
calendar files that a later generator produces, against a real PostgreSQL, without applying a file by hand. `ot` never
connects to a live broker: nothing here touches `internal/broker`, and every test uses fakes; a test that needs
PostgreSQL carries the build tag `pg` and runs in GitHub CI only.

What was found on main, and what is still missing:

- No yearly calendar file is committed and no generator exists: the only `master.session_day` rows in the repository are
  the six typed rows of `contracts/fixtures/master.session_day.r1.json`, which are test expectations. The generator and
  its yearly files follow `docs/design/pyside-layout.md` (`pyside/src/optdesk/calendar`), a later design. Until it lands
  there is no real calendar file, so a real `ot run-once` still ends in `UnknownCalendar`; this design makes the loader
  ready and tests it with a small synthetic file.
- `ot` cannot apply a migration alone: `migrate.Up` wants a `*sql.DB`, and only `internal/store` may import pgx. The
  row `ot` may import `internal/store/setup`, which has pgx, so the DSN-taking entry point goes there. It does not call
  `internal/store/migrate`: `arch.toml` lets a store sub-package import the root `store` and `contracts` only, so a
  call from `setup` to `migrate` would fail go-arch-lint, and allowing it is a change of `docs/design/modules.md` that
  this design avoids. `setup` uses goose directly, a few lines (see `## Left out`).
- No role of week 1 may write `master.session_day` (`ot_runner` has SELECT only) and `setup.Setup` keeps the password
  of `ot_owner` to itself. No new role, grant or credential is made: migrate and load run under the administrative DSN
  that `setup` already reads (`store.AdminDSNEnv`), and switch to `ot_owner` inside the session, so every object they
  make is owned by `ot_owner`, as `migrations/0002_series.sql` and `migrations/0003_ops_ledger.sql` already assume.
- `migrations/0001_master_session_day.sql` has `NOT NULL` on `open_at` and `close_at`, so a day that is not a session
  cannot carry null hours; the convention is that both equal 00:00 UTC of that day (as the fixture has).

## Modules

No change to `docs/design/modules.md`, `scripts/arch`, `arch.toml` or `.go-arch-lint.yml`: every path below belongs to an
existing row, and this design adds no import edge. `cmd/ot/wire.go` at main already imports `internal/store/setup`,
`internal/store/sessionday`, `internal/store/series` and `internal/store/ops`, the four store sub-packages that the row
`ot` lists, and the other implementation packages of that list; `internal/store/setup` is the only one whose use this
design extends, and `wire.go` stays the one non-test file of `cmd/ot` that imports them.

| issue | module | order |
|---|---|---|
| setup-owner (interface) | store | first |
| ot-migrate-load | ot | after setup-owner |

The cut: the two functions of the store entry are not in an `api.go`, so this is not an interface change in the sense of
`AGENTS.md`; the entry is marked (interface) only because `ot-migrate-load` calls them and cannot compile without them,
and an `ot` issue may not wait for a sibling that is not the marked entry. The `ot` entry holds both subcommands because
both need a new field of `deps` and so both edit `wire.go`; split in two they could not run in parallel. Every entry
lies in one module. No issue edits `go.mod`, `go.sum`, `package.json` or `pnpm-lock.yaml` (goose and pgx are in `go.mod`).

## Interface

No interface file of a module (`internal/*/api.go`, `internal/profile/api.go`) is edited. New names, in
`internal/store/setup`:

| name | file | meaning |
|---|---|---|
| `setup.MigrateUp` | internal/store/setup/migrate.go | `func MigrateUp(ctx context.Context, adminDSN string, fsys fs.FS) error`: connects to the database `optdesk` of the server of `adminDSN`; the first statement of every session is `SET ROLE ot_owner`; then `CREATE SCHEMA IF NOT EXISTS meta` (owned by `ot_owner`); then goose applies the `*.sql` files of `fsys` with the version table `meta.goose_db_version` |
| `setup.OwnerDSN` | internal/store/setup/migrate.go | `func OwnerDSN(adminDSN string) (string, error)`: the DSN of the database `optdesk` with the startup option that sets the role `ot_owner`, for `sessionday.Open`; a server that refuses the role refuses the login, so nothing runs as the admin role |

Where goose's version table lives and how it is configured: in the schema `meta`, not `public`. The session sets
`search_path` to `meta` (after `SET ROLE`), so goose creates and finds its table unqualified there; every statement of
`migrations/` names its schema, so no object lands in `meta` by accident. The schema is made by the same session, under
`ot_owner`, so `ot_owner` owns it and the table.

Errors never carry the DSN. No error returned by pgx, `net/url`, goose or `database/sql` for a connection, parse or
role step is wrapped with `%w` or formatted with `%v`: those texts can hold host, user or DSN pieces. The function returns
a new error of fixed text that names the stage (`parse`, `connect`, `set role`, `schema`); the existing `parseURL` of
`setup.go` already works this way. Only the error of the goose run is wrapped with `%w`, because it comes from the
migration statements and not from the connection.

Names used as they are: `store.AdminDSNEnv`, `store.OpenSessionDayFunc`, `store.SessionDayWriter.UpsertSessionDays`,
`contracts.SessionDay`, `clock.NewCalendarFunc`.

Names fixed in `cmd/ot` (unexported): `deps` gains `migrateUp func(ctx context.Context, adminDSN string, fsys fs.FS) error`
(assigned `setup.MigrateUp`) and `ownerDSN func(adminDSN string) (string, error)` (assigned `setup.OwnerDSN`), 13 fields
in all. The commands: `ot migrate [--dir DIR]` (default `migrations`, a path of the checkout) and
`ot calendar load FILE...`. The function of the second command is `runCalendar`, because `calendar` is an import name
in `wire.go`. The owner's sequence is `ot setup pg`, `ot migrate`, `ot calendar load FILE...` (once a generated file
exists), `ot run-once`.

Owner of the file format: the rows are the `master.session_day r1` dataset, which belongs to contracts, so contracts is
the owner and `cmd/ot` (the loader, row ot) is only a consumer, as the generator of `pyside/src/optdesk/calendar` (row
clock) will only be a producer. The dataset document of contracts is under `docs/contracts/datasets/`, which is generated
from `contracts/datasets/` and is a path of contracts; no entry of this cut is a contracts entry, so the format of the
envelope is defined in this design, and the contracts document for it comes with the generator design (`## Left out`).

File format read by `ot calendar load`: one JSON object `{"provenance": {...}, "rows": [...]}`. `rows` are
`master.session_day r1` objects. `provenance` has five non-empty strings: `generator`, `generator_version`,
`source_library`, `source_library_version`, `generated_at` (RFC 3339). The loader invents none of them.

## Issues

Items under `Checked by CI` are decided by the workflows of the head commit; every other item is a fact read from the
diff, with an offline grep where it is about behaviour.

### setup-owner (interface)

`setup.MigrateUp` and `setup.OwnerDSN`: migrations and a session as `ot_owner` from the admin DSN.
  Goal: the store can apply a directory of migrations under `ot_owner` from the admin DSN, and can give the DSN a session-day store needs to write as `ot_owner`; no new role, grant or credential.
  Files: internal/store/setup/migrate.go, internal/store/setup/migrate_test.go, internal/store/setup/migrate_pg_test.go, docs/contracts/store.md.
  Design: docs/design/week1-run.md#interface; own implementation of the goose call (about 6 lines) instead of `internal/store/migrate`, because go-arch-lint forbids a store sub-package from importing another and the alternative is a change of `docs/design/modules.md`; the pgx standard-library adapter and goose are already in `go.mod`.
  Acceptance:
  - `migrate.go` declares `MigrateUp` and `OwnerDSN` with the signatures of `## Interface` (offline: `grep -c 'func MigrateUp' internal/store/setup/migrate.go` and `grep -c 'func OwnerDSN' internal/store/setup/migrate.go` print 1 each); it does not import `internal/store/migrate` (`grep -c 'store/migrate' internal/store/setup/migrate.go` prints 0) and `setup.go` is unchanged.
  - The session setup is one unexported function `ownerSession(ctx, conn execer, log *slog.Logger) error` whose first statement is `SET ROLE ot_owner`, then `SET search_path TO meta`; `MigrateUp` runs it as the after-connect step of every connection and opens the database with one open connection (offline: `grep -c 'SET ROLE' internal/store/setup/migrate.go` prints 1 and the line is the first `Exec` of `ownerSession`).
  - Condition 1, `TestSetRoleFailureRunsNothing`: a fake `execer` rejects `SET ROLE`; the test asserts that the fake saw exactly that one statement, that the migration function passed in was not called, and that the returned error is of stage `set role` and never continues as the admin role. `TestSetRoleIsFirst`: the first statement a fake sees is `SET ROLE ot_owner` and the second is the `search_path` one. Both drive `migrateWith(ctx, open, up, fsys, log)`, where `open func(ctx, afterConnect) (*sql.DB, error)` and `up func(ctx, *sql.DB, fs.FS) error` are replaced by recording fakes.
  - The schema and the version table are made after the role switch: `TestSchemaAfterRole` asserts with the fake that `CREATE SCHEMA IF NOT EXISTS meta` comes after `SET ROLE ot_owner` and that no statement mentions `public`; offline `grep -c 'goose_db_version' internal/store/setup/migrate.go` prints 0 (the table name is goose's default under the `search_path`).
  - Condition 3, no DSN in an error or in the log: `TestMigrateUpMalformedDSNHoldsNoSecret` (a malformed DSN that contains a sentinel password), `TestMigrateUpUnreachableHoldsNoSecret` (a well-formed DSN with a sentinel password, a wrong user and the loopback interface with port 0, so the connection is refused offline: the stand-in for a wrong password) and `TestOwnerDSNMalformedHoldsNoSecret` each assert that the sentinel password and the DSN are absent from the returned error text and from the output of a `slog` logger writing to a buffer, which the tests pass to the unexported `migrateUp(ctx, adminDSN, fsys, log)` that `MigrateUp` calls with `slog.Default()`. `grep -c '%w' internal/store/setup/migrate.go` prints 1 (the goose error only).
  - `OwnerDSN` returns a `postgres://` URL for the database `optdesk` keeping host, port, user, password and `sslmode` of its argument and adding the option that sets the role `ot_owner` (`TestOwnerDSNSetsRole`, `TestOwnerDSNKeepsSSLMode`); the value is never logged (`grep -n 'log\.' internal/store/setup/migrate.go` lists no line that has a DSN variable as an argument).
  - `migrate_pg_test.go` has the build tag `pg` on its first line; it creates the roles it needs when missing under the advisory lock that `internal/store/series/series_pg_test.go` takes (`grep -c 'pg_advisory_lock' internal/store/setup/migrate_pg_test.go` prints at least 1; it copies the private `ensureRoles` about 25 lines), creates a scratch database owned by `ot_owner`, and calls the unexported `migrateWith(ctx, open, up, fsys, log)` that `MigrateUp` itself calls (offline: `grep -c 'migrateWith(' internal/store/setup/migrate.go` prints at least 2, the definition and the call in `migrateUp`), passing `open` as a function that connects to the scratch database (the `PG*` variables with the scratch database name) as a login role that is a member of `ot_owner` without being a superuser, and not only as the superuser, with the given after-connect step, and `up` as the package function `gooseUp` that `MigrateUp` also passes; so the real `ownerSession`, the real schema step and the real goose run are exercised, and only the database name and the login role differ from `MigrateUp`. Its functions: `TestMigrateUpAsOwnerPG` (applies 0001 to 0004 twice; asserts every table of schemas `master`, `obs` and `ops` has `tableowner` `ot_owner`, `meta.goose_db_version` exists and `public.goose_db_version` does not), `TestMigrateUpRejectedRolePG` (a login role that is not a member of `ot_owner`: error of stage `set role`, and no table exists afterwards), `TestMigrateUpWrongPasswordPG` (a wrong password: the error and the captured log lack the password). Offline: `grep -n 'func Test.*PG' internal/store/setup/migrate_pg_test.go` finds the three, and `grep -c 'postgres://[^"]*:[^"]*@' internal/store/setup/migrate_pg_test.go` prints 0 (DSNs are built from the `PG*` variables).
  - `docs/contracts/store.md` names `MigrateUp` and `OwnerDSN` under `## Inputs`, the version table `meta.goose_db_version` under `## Invariants`, and adds a `- v3:` line to `## Version history`; it keeps the line that says `internal/store/migrate` embeds no files; no heading changes.
  - Every test of `migrate_test.go` calls `t.Parallel()` (offline: `grep -c 't.Parallel()' internal/store/setup/migrate_test.go` is at least `grep -c 'func Test' internal/store/setup/migrate_test.go`); none sleeps or calls `time.Now` (`grep -c 'time\.Now\|time\.Sleep' internal/store/setup/migrate.go` prints 0); each new file stays under 600 lines; no file outside the four listed is in the diff.
  - Checked by CI: ci-go (build, vet, test, golangci-lint with depguard, go-arch-lint component `store/setup`); the `test-pg` job runs the three `PG` tests against a real server (offline substitutes: the greps above and the fake-connection tests).
  Note (not acceptance): importing the pgx standard-library adapter registers the driver name `pgx` globally in the process.
  Depends on: none. Order: first; `ot-migrate-load` waits for it.
  Size: M

### ot-migrate-load

`ot migrate` and `ot calendar load`.
  Goal: the owner applies the migrations and loads calendar files with `ot`, under `ot_owner`, using the administrative DSN of the process environment that `ot setup pg` already reads.
  Files: cmd/ot/migrate.go, cmd/ot/migrate_test.go, cmd/ot/calendar.go, cmd/ot/calendar_test.go, cmd/ot/wire.go, cmd/ot/wire_test.go, cmd/ot/testdata/calendar_synthetic.json.
  Design: docs/design/week1-run.md#interface; one ot issue for both commands because both add a field to `deps` and so edit `wire.go`; the rows are decoded with `encoding/json` into `contracts.SessionDay`, no new dependency; the identifier `calendar` is an import name in `wire.go`, so `calendar.go` declares no package-level `calendar`.
  Acceptance:
  - `wire.go` adds the fields `migrateUp` and `ownerDSN` of `## Interface` and `newDeps()` assigns `setup.MigrateUp` and `setup.OwnerDSN`; `TestNewDepsFilled` of `wire_test.go` expects 13 fields; `wire.go` stays the one non-test file of `cmd/ot` that imports an implementation package (offline: `grep -rlE --include='*.go' 'internal/(clock/(realclock|calendar)|store/(sessionday|series|ops|setup)|sources/fred|jobs/fredseries|runner/reconcile|pages/(macro_rates|embed))' cmd/ot | grep -v _test.go` prints `cmd/ot/wire.go` only).
  - `migrate.go` registers `migrate` in `init()`, takes `--dir` (default `migrations`), refuses the `demo` profile before anything is read, reads `store.AdminDSNEnv` through `a.getenv` (never a flag; an empty value is an error that names the variable and calls nothing), and calls `a.deps.migrateUp` with `os.DirFS(dir)`; an error of `migrateUp` is replaced by a text that names the variable and drops the error text; one slog line on success carries the directory only. Tests: `TestMigrateCallsUp` (the fake receives the DSN and an `fs.FS` that lists the files of a `t.TempDir()`), `TestMigrateDefaultDir`, `TestMigrateDemoRefused`, `TestMigrateUnsetDSN`, `TestMigrateFailureStopsLoad` (a failing fake: the command returns an error and the log holds one error line without the error text); `grep -c 'func TestMigrate' cmd/ot/migrate_test.go` prints 7 once the two DSN tests below are counted.
  - `calendar.go` registers `calendar` in `init()`; the first argument must be `load` and at least one path must follow, else usage and an error (`TestCalendarUsage`); it refuses the `demo` profile before a file is read or a store is opened (`TestCalendarLoadDemoRefused`); it reads `store.AdminDSNEnv` through `a.getenv` and an empty value is an error that names the variable and opens nothing (`TestCalendarLoadUnsetDSN`); it opens the store with `a.deps.openSessionDay` and the value of `a.deps.ownerDSN`, so a refused role means no store and no upsert (`TestCalendarLoadOwnerFailureWritesNothing`: a fake `ownerDSN` error and a fake `openSessionDay` error each leave the fake writer untouched and are returned without their text).
  - Condition 2, provenance: every file is read and decoded, and checked, before the store is opened. `TestCalendarLoadProvenanceRequired` loads `cmd/ot/testdata/calendar_synthetic.json` (three synthetic rows, the made-up calendar id `TEST`, a header with the five fields) and succeeds, then writes the same rows to a `t.TempDir()` without the `provenance` object and asserts that the load is refused with an error naming the path and the missing key, and that the store was not opened; `TestCalendarLoadProvenanceFields` refuses a header with any one of the five fields empty or `generated_at` not RFC 3339.
  - Row checks, all before the store is opened: per calendar id `session_day` strictly ascending and unique; `open_at` not after `close_at`; a day with `is_session` false has `open_at` equal to `close_at` and `early_close` false (0001 makes both hours `NOT NULL`, so a null is not a possibility); a missing file, malformed JSON, an empty `rows` array and an empty calendar id are errors that name the path and no row value. Tests: `TestCalendarLoadRowOrder`, `TestCalendarLoadRowHours` (table-driven; one row where a session day has open strictly before close and loads, one row where a non-session day has open equal to close and `early_close` false and loads, and rows where a session day has open equal to or after close, or a non-session day has open different from close or `early_close` true, are refused), `TestCalendarLoadClosedDayShape`, `TestCalendarLoadMalformed`, `TestCalendarLoadEmptyFile`, `TestCalendarLoadMissingFile`.
  - Idempotence: `TestCalendarLoadIdempotent` loads the synthetic file twice into a fake store that keeps rows in a map keyed by calendar id and session day, as the real upsert does (offline: `grep -c 'ON CONFLICT (calendar_id, session_day)' internal/store/sessionday/writer.go` prints at least 1), and asserts the map after the second load equals the map after the first; the upsert is called once per file with the rows in file order and the store is closed on every path (`TestCalendarLoadClosesStore`).
  - The reader `run-once` uses accepts a loaded file: `TestCalendarLoadFeedsCalendar` loads the synthetic file into the fake store, builds the calendar with `newDeps().newCalendar` over that store (which satisfies `clock.SessionSource` as it is), and asserts `Session` for a session day returns the loaded `OpenAt` and `CloseAt` unchanged, `Session` for a closed day returns `IsSession` false, `NextSession` skips the closed day, and an unknown calendar id gives `UnknownCalendar`. The expectations are typed in the test.
  - No time of day is typed in code: `grep -c -E '[0-9]{1,2}:[0-9]{2}' cmd/ot/migrate.go cmd/ot/calendar.go` prints 0 for each file; hours appear only in the testdata file and in test expectations.
  - Condition 3, the DSN is run-time input only: `ot` never writes it to a file and never logs it (offline: `grep -c 'WriteFile\|os.Create\|os.OpenFile' cmd/ot/migrate.go cmd/ot/calendar.go` prints 0 for each); `TestMigrateMalformedDSN`, `TestMigrateWrongPasswordDSN`, `TestCalendarLoadMalformedDSN` and `TestCalendarLoadWrongPasswordDSN` make the fakes return errors whose text holds a sentinel DSN and a sentinel password (a malformed one, and a well-formed one with a wrong password) and assert that neither appears in the returned error or in the output of the `slog` logger of the `app`, which writes to a buffer. This makes 7 test functions in `migrate_test.go` and 17 in `calendar_test.go`: `grep -c 'func Test' cmd/ot/migrate_test.go` prints 7 and `grep -c 'func Test' cmd/ot/calendar_test.go` prints 17.
  - Every test of both files calls `t.Parallel()` (offline: `grep -c 't.Parallel()'` of each file is at least its `func Test` count); tests use fakes and a temporary directory, open no store, and neither sleep nor call `time.Now` (`grep -c 'time\.Now\|time\.Sleep' cmd/ot/migrate.go cmd/ot/calendar.go` prints 0 for each); each new file stays under 600 lines; no file outside the seven listed is in the diff.
  - Checked by CI: ci-go (build, vet, test, golangci-lint, go-arch-lint component `ot`). Nothing here needs a real server; the pg behaviour is `setup-owner`.
  Note (not acceptance): no real calendar file exists until the generator of `docs/design/pyside-layout.md` lands; this entry reads any path it is given, so it needs no generated file. Needs `setup-owner` merged first, or `wire.go` does not compile.
  Depends on: setup-owner (the one entry marked (interface)). Order: after setup-owner.
  Size: L

## Second opinion

The optdesk advisor answered the brief (Claude Fable 5.1, 2026-11-06); other-vendor models waived by the owner 2026-10-29.

- Condition 1: when `SET ROLE ot_owner` fails, the command fails before any other statement and never continues as the admin role (`TestSetRoleFailureRunsNothing`, `TestSetRoleIsFirst`, `TestSchemaAfterRole` of `setup-owner`; `TestCalendarLoadOwnerFailureWritesNothing` of `ot-migrate-load`); goose's version table `meta.goose_db_version` is created under `ot_owner` in a schema `ot_owner` owns (`TestMigrateUpAsOwnerPG`).
- Condition 2: the calendar file carries a provenance header and the loader refuses a file without it (`TestCalendarLoadProvenanceRequired`, `TestCalendarLoadProvenanceFields`); rows are upserted by calendar id and session day and a re-run changes nothing (`TestCalendarLoadIdempotent`); no open or close time is typed in code (grep on `migrate.go` and `calendar.go`).
- Condition 3: a malformed DSN and a DSN with a wrong password leave neither the DSN nor the password in the returned error or the captured slog output (`TestMigrateUpMalformedDSNHoldsNoSecret`, `TestMigrateUpUnreachableHoldsNoSecret`, `TestMigrateUpWrongPasswordPG` of `setup-owner`; the four DSN tests of `ot-migrate-load`).
- No new role, no new grant and no new credential: both commands run under the admin DSN that `setup` already uses, as `ot_owner`.

Advisor: agrees (optdesk-advisor, Claude Fable 5.1, 2026-11-06) with ot migrate and ot calendar load under the admin DSN with SET ROLE ot_owner, as new writes outside the repository; conditions: fail before any statement if SET ROLE fails and meta.goose_db_version under ot_owner; provenance-gated calendar file with idempotent upsert and strict open < close on session days; DSN and password redaction asserted on error and log.

## Left out

- The calendar generator and its yearly data files follow `docs/design/pyside-layout.md` (pyside/src/optdesk/calendar),
  a later design; so do the Python project, the years after the first, and the choice of calendars (the Tokyo calendar
  and the JPX overlay of ADR 0004 are separate work). The generator must write the format of `## Interface`: an object
  with `provenance` (`generator`, `generator_version`, `source_library`, `source_library_version`, `generated_at` in
  RFC 3339) and `rows`; the loader refuses a file without all five. The dataset document for this envelope belongs to
  contracts (`docs/contracts/datasets/`, generated from `contracts/datasets/`) and comes with the generator design;
  until then the format is defined in this design only.
- The cross-check against IBKR trading hours (clock row of `docs/design/modules.md`, ADR 0004 "at runtime IBKR
  `tradingHours` are the check"): it needs a broker connection, which `ot` never opens. Until then nothing marks the
  rows as cross-checked.
- A dedicated loader role (INSERT on `master.session_day` only) and a stored DSN for it: both commands run as
  `ot_owner` under the admin DSN, and a later store issue may add the role.
- Reading the migrations from the binary instead of the checkout: `docs/contracts/store.md` forbids embedding in
  `internal/store/migrate`, and a Go package at the repository root would be a component the arch generator does not
  know; `ot migrate` and `ot calendar load` read paths of the checkout.
- One goose call in `setup` next to `internal/store/migrate.Up`: the two are unified when `docs/design/modules.md`
  allows a store sub-package to import another (a change not made here).
- A database whose files were applied by hand: it has no version table, so `ot migrate` would try 0001 again and fail;
  the owner starts from a database made by `ot setup pg`. A hint about which stage failed beyond its name is not added,
  to keep error texts free of connection pieces.
- `OT_PG_ADMIN_DSN` (`store.AdminDSNEnv`) is the only variable used: the owner supplies it at run time, `ot` never writes
  it to a file, never logs it, and no error text holds it. `ot setup pg` is not edited.

## Adversarial

Source: the file week1-run.adversarial.md of the dispatcher window (kept there, not published).

- 1 gap against the goal: adopted; the role is now named (`ot_owner` through `SET ROLE` under the admin DSN), the loader checks rows (ascending, unique, open not after close, closed-day shape from the `NOT NULL` of 0001), a test shows a loaded file satisfies the calendar `run-once` uses, and the text says the goal is met for the migrator and the loader, not for `run-once`, until the generator lands.
- 2 biggest risk (role and ownership mismatch): adopted; sessions run as `ot_owner`, so 0001 to 0003 objects share one owner, the version table is in `meta` under `ot_owner`, and the pg test runs its DDL as a non-superuser member of `ot_owner`.
- 3 simpler way: adopted in part; the entry `ot-imports` and every change of `modules.md`, `scripts/arch` and the arch files are gone, the new variable is gone (`store.AdminDSNEnv` is reused), and the functions live in `internal/store/setup`; not adopted as asked: `setup` does not call `internal/store/migrate`, because the generated `arch.toml` forbids a store sub-package from importing another, so `setup` calls goose itself.
- 4 lock-in: accepted and recorded; the file format is fixed here and the generator follows it, the commands read the checkout, and the dedicated loader role and reading migrations from the binary are in `## Left out`.
- 5 false claims: adopted; the ownership sentence follows 0002 and 0003, the `git diff` claim is gone with `ot-imports`, and the error rule is exact: no `%w` of any connection, parse or role error, fixed text per stage, with a grep that counts one `%w` in `migrate.go`.
- 6 slip causes: adopted; the pg tests take the advisory lock and run as an `ot_owner`-shaped role, not only the superuser; stage names in the error text keep a first run debuggable without a connection piece.
- Second-opinion question: no new role, no new grant and no new credential; it runs under the admin DSN that setup already uses, as `ot_owner`; the three conditions of `## Second opinion` bind it.
