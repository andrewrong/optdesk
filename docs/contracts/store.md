# store contract

## Responsibility

The only module that opens the database. `store.SessionDayWriter` upserts and `store.SessionDayReader` reads `master.session_day r1` rows; `sessionday.Open` returns both with `Close` as a `store.SessionDayStore`. `internal/store/migrate` applies goose SQL migrations from an `fs.FS` the caller supplies; it embeds none.

## Inputs

- `master.session_day r1` rows (`contracts.SessionDay`: `calendar_id`, `session_day`, `is_session`, `open_at`, `close_at`, `early_close`) to `UpsertSessionDays`.
- A calendar id and two `contracts.Day` values (`YYYY-MM-DD`, market zone) to `SessionDays`.
- A connection string to `sessionday.Open`; a `*sql.DB` and an `fs.FS` of SQL files to `migrate`.
- The administrative DSN and an `fs.FS` of SQL files to `setup.MigrateUp`, which applies them as `ot_owner`; the administrative DSN to `setup.OwnerDSN`, which returns the DSN of the database `optdesk` whose login sets the role `ot_owner`, for `sessionday.Open`.

## Outputs

- `SessionDays` returns the rows of the calendar with `session_day` between the two days, both ends inclusive, ordered by `session_day`; instants are in UTC; no rows gives an empty slice.
- Errors are `contracts.TypedError`: `Retryable` when the database fails, `Permanent` for a malformed day.

## Invariants

- `UpsertSessionDays` is `ON CONFLICT (calendar_id, session_day) DO UPDATE`, runs in one transaction (all rows or none), and does nothing for an empty input.
- The caller's context is passed to every database call.
- The SQL names the six columns of the table and never uses `SELECT *`.
- `session_day` is the market-zone date, not a UTC date.
- Only files under `internal/store/` import pgx.
- `setup.MigrateUp` runs `SET ROLE ot_owner` as the first statement of its one connection, then `SET search_path TO meta` and `CREATE SCHEMA IF NOT EXISTS meta`; the goose version table is `meta.goose_db_version`, owned by `ot_owner`. Its errors for a parse, connect, role or schema step are of fixed text and hold no DSN.
- `ot_runner` has SELECT on `master.session_day` and nothing else there (`migrations/0004_session_day_grant.sql`).

## Version history

- v1: `master.session_day r1` Writer and Reader, migration runner.
- v2: `ot_runner` may read `master.session_day` (migration 0004).
- v3: `setup.MigrateUp` and `setup.OwnerDSN`: migrations and a session-day store as `ot_owner` from the administrative DSN; no new role, grant or credential.

## Contract tests

- `internal/store/api_test.go`: the interface from both sides.
- `internal/store/sessionday/sessionday_test.go`: SQL strings, transaction, empty input, context, range and order against an in-test fake.
- `internal/store/sessionday/sessionday_pg_test.go` (build tag `pg`): the round trip through the real Writer and Reader on a database of its own.
- `internal/store/migrate/migrate_test.go`: the runner's calls against a fake driver.

## Jobs and slots

None. Store runs nothing and owns no slot; jobs and runner call it.

## Forbidden

- Opening the database outside `internal/store/`.
- `SELECT *` or a column outside the table.
- Embedding `migrations/` in the migrate package, or registering a global SQL driver in a test.
- A connection default (host, user, password, database) in code.
