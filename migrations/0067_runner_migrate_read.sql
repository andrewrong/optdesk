-- +goose Up
-- Run by ot_owner, as in 0065_pages_obs_usage.sql. The migrator runs with search_path = meta
-- (internal/store/setup/migrate.go, ownerSession), so goose keeps its version table in meta.goose_db_version, and
-- ot_runner holds no USAGE on schema meta. `ot migrate check` runs with the runner's secrets and must read which
-- versions the database has (#3477). USAGE alone reads nothing else in meta: the role gets SELECT on that one
-- table and no other grant. It only adds grants; the auto-deploy waits until the window runs `setup pg` then
-- `migrate`.
GRANT USAGE ON SCHEMA meta TO ot_runner;
GRANT SELECT ON meta.goose_db_version TO ot_runner;

-- +goose Down
REVOKE SELECT ON meta.goose_db_version FROM ot_runner;
REVOKE USAGE ON SCHEMA meta FROM ot_runner;
