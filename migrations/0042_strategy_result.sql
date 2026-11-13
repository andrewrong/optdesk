-- +goose Up
-- strategy.result r1: one evaluated result of one strategy in one run, insert-only (docs/design/strategy-framework.md
-- Decisions 4, 5 and 6). A rerun is a new run_id and adds rows. The rows carry account-derived facts (fits by alias),
-- so the grants follow 0018_candidates.sql: ot_w_candidate (made by internal/store/setup) has INSERT and SELECT on
-- the table and SELECT on the view, ot_ro_pages reads the view only, ot_runner gets nothing. No role but the owner
-- may UPDATE, DELETE or TRUNCATE.
CREATE SCHEMA IF NOT EXISTS strategy;
CREATE SCHEMA IF NOT EXISTS rd;

CREATE TABLE strategy.result (
    session_day        date        NOT NULL,
    run_id             text        NOT NULL,
    strategy           text        NOT NULL,
    rule_version       text        NOT NULL,
    param_hash         text        NOT NULL,
    key                text        NOT NULL,
    underlying_id      bigint      NOT NULL,
    status             text        NOT NULL,
    rank               integer,
    rank_in_underlying integer,
    legs               text        NOT NULL,
    checks             text        NOT NULL,
    facts              text        NOT NULL,
    computed_at        timestamptz NOT NULL,
    settings_version   bigint,
    PRIMARY KEY (session_day, run_id, strategy, key),
    CONSTRAINT result_status_ck CHECK (status IN ('ok', 'blocked', 'unknown'))
);

CREATE INDEX result_day_strategy ON strategy.result (session_day, strategy, computed_at DESC);

ALTER SCHEMA strategy OWNER TO ot_owner;
ALTER TABLE strategy.result OWNER TO ot_owner;

-- Every row passes through, every column named.
CREATE VIEW rd.strategy_result_r1 AS
SELECT session_day, run_id, strategy, rule_version, param_hash, key, underlying_id, status, rank, rank_in_underlying,
       legs, checks, facts, computed_at, settings_version
FROM strategy.result;

ALTER VIEW rd.strategy_result_r1 OWNER TO ot_owner;

GRANT USAGE ON SCHEMA strategy TO ot_w_candidate;
GRANT INSERT, SELECT ON strategy.result TO ot_w_candidate;

-- The reader and the shadow job read the view only.
GRANT USAGE ON SCHEMA rd TO ot_w_candidate;
GRANT SELECT ON rd.strategy_result_r1 TO ot_w_candidate;

-- Read-only: the pages read the view and nothing else.
GRANT USAGE ON SCHEMA rd TO ot_ro_pages;
GRANT SELECT ON rd.strategy_result_r1 TO ot_ro_pages;

-- +goose Down
-- USAGE on schema rd is shared with the other views and stays granted; schema rd stays.
REVOKE SELECT ON rd.strategy_result_r1 FROM ot_ro_pages;
REVOKE SELECT ON rd.strategy_result_r1 FROM ot_w_candidate;
REVOKE INSERT, SELECT ON strategy.result FROM ot_w_candidate;
REVOKE USAGE ON SCHEMA strategy FROM ot_w_candidate;
DROP VIEW rd.strategy_result_r1;
DROP TABLE strategy.result;
DROP SCHEMA strategy;
