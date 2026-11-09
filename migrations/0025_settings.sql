-- +goose Up
-- settings.rule_param and settings.option_exclusion: the owner's own changes, insert-only and versioned
-- (docs/design/settings-form.md Decision 7). A row is never updated or deleted; a reset is one more row with an
-- empty value. set_at comes from the database's now(), so the writer holds no clock. The settings of a day are
-- the newest row per key (or instrument) with set_at not after the instant asked, ties by id.
-- The role ot_w_settings (made by internal/store/setup) holds INSERT and SELECT on the two tables and reads the
-- two views; ot_ro_pages and ot_runner read the views and no table. No role but the owner may UPDATE, DELETE or
-- TRUNCATE.
CREATE SCHEMA IF NOT EXISTS settings;
CREATE SCHEMA IF NOT EXISTS rd;

CREATE TABLE settings.rule_param (
    id        bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    param_key text        NOT NULL,
    value     text        NOT NULL,
    set_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE settings.option_exclusion (
    id            bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    instrument_id bigint      NOT NULL,
    excluded      boolean     NOT NULL,
    set_at        timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX rule_param_key_set_at ON settings.rule_param (param_key, set_at DESC, id DESC);
CREATE INDEX option_exclusion_instrument_set_at ON settings.option_exclusion (instrument_id, set_at DESC, id DESC);

ALTER SCHEMA settings OWNER TO ot_owner;
ALTER TABLE settings.rule_param OWNER TO ot_owner;
ALTER TABLE settings.option_exclusion OWNER TO ot_owner;

-- Every row passes through, every column named.
CREATE VIEW rd.rule_param_r1 AS
SELECT id, param_key, value, set_at FROM settings.rule_param;

CREATE VIEW rd.option_exclusion_r1 AS
SELECT id, instrument_id, excluded, set_at FROM settings.option_exclusion;

ALTER VIEW rd.rule_param_r1 OWNER TO ot_owner;
ALTER VIEW rd.option_exclusion_r1 OWNER TO ot_owner;

GRANT USAGE ON SCHEMA settings TO ot_w_settings;
GRANT INSERT, SELECT ON settings.rule_param TO ot_w_settings;
GRANT INSERT, SELECT ON settings.option_exclusion TO ot_w_settings;

-- The writer reads the current values through the views, as every reader does.
GRANT USAGE ON SCHEMA rd TO ot_w_settings;
GRANT SELECT ON rd.rule_param_r1 TO ot_w_settings;
GRANT SELECT ON rd.option_exclusion_r1 TO ot_w_settings;

-- The pages read the views and nothing else.
GRANT USAGE ON SCHEMA rd TO ot_ro_pages;
GRANT SELECT ON rd.rule_param_r1 TO ot_ro_pages;
GRANT SELECT ON rd.option_exclusion_r1 TO ot_ro_pages;

-- A job opened with the runner DSN reads the settings through the views.
GRANT USAGE ON SCHEMA rd TO ot_runner;
GRANT SELECT ON rd.rule_param_r1 TO ot_runner;
GRANT SELECT ON rd.option_exclusion_r1 TO ot_runner;

-- +goose Down
REVOKE SELECT ON rd.option_exclusion_r1 FROM ot_runner;
REVOKE SELECT ON rd.rule_param_r1 FROM ot_runner;
REVOKE USAGE ON SCHEMA rd FROM ot_runner;
REVOKE SELECT ON rd.option_exclusion_r1 FROM ot_ro_pages;
REVOKE SELECT ON rd.rule_param_r1 FROM ot_ro_pages;
REVOKE USAGE ON SCHEMA rd FROM ot_ro_pages;
REVOKE SELECT ON rd.option_exclusion_r1 FROM ot_w_settings;
REVOKE SELECT ON rd.rule_param_r1 FROM ot_w_settings;
REVOKE USAGE ON SCHEMA rd FROM ot_w_settings;
REVOKE INSERT, SELECT ON settings.option_exclusion FROM ot_w_settings;
REVOKE INSERT, SELECT ON settings.rule_param FROM ot_w_settings;
REVOKE USAGE ON SCHEMA settings FROM ot_w_settings;
DROP VIEW rd.option_exclusion_r1;
DROP VIEW rd.rule_param_r1;
DROP TABLE settings.option_exclusion;
DROP TABLE settings.rule_param;
DROP SCHEMA settings;
