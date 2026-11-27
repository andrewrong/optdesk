-- +goose Up
-- docs/design/gateway-down-since.md entry 3 (Decisions 1, 10): one row per gateway alias, written by the gateway health
-- pass (ot_runner) and read by the data health page through a view (ot_ro_pages). A row holds an alias, a flag and two
-- times, never free text, an id or a secret. No role but the owner may DELETE or TRUNCATE.
CREATE SCHEMA IF NOT EXISTS ops;
CREATE SCHEMA IF NOT EXISTS rd;

CREATE TABLE ops.gateway_state (
    alias       text        PRIMARY KEY,
    down        boolean     NOT NULL,
    since       timestamptz NOT NULL,
    observed_at timestamptz NOT NULL,
    CONSTRAINT gateway_state_alias_ck CHECK (alias ~ '^[a-z0-9]+$')
);

ALTER TABLE ops.gateway_state OWNER TO ot_owner;

-- The view runs with the rights of its owner, so the page role needs nothing on the table.
CREATE VIEW rd.gateway_state_r1 AS
SELECT g.alias, g.down, g.since, g.observed_at
FROM ops.gateway_state g;

ALTER VIEW rd.gateway_state_r1 OWNER TO ot_owner;

-- SELECT is for the ON CONFLICT DO UPDATE of the writer.
GRANT USAGE ON SCHEMA ops TO ot_runner;
GRANT SELECT, INSERT, UPDATE ON ops.gateway_state TO ot_runner;
GRANT USAGE ON SCHEMA rd TO ot_ro_pages;
GRANT SELECT ON rd.gateway_state_r1 TO ot_ro_pages;

-- +goose Down
-- USAGE on schemas ops and rd stays granted (shared with other objects).
REVOKE SELECT ON rd.gateway_state_r1 FROM ot_ro_pages;
REVOKE SELECT, INSERT, UPDATE ON ops.gateway_state FROM ot_runner;
DROP VIEW rd.gateway_state_r1;
DROP TABLE ops.gateway_state;
