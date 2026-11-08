-- +goose Up
-- alerts.sent: one insert-only record of each alert the notify pass has said, once per (rule_id, day). The role
-- ot_w_alerts (made by internal/store/setup) gets INSERT and SELECT on it and nothing else: no UPDATE, no DELETE,
-- no TRUNCATE, no grant on any page view (docs/design/risk-alerts.md Decisions 4).
CREATE SCHEMA IF NOT EXISTS alerts;

CREATE TABLE alerts.sent (
    rule_id text        NOT NULL,
    day     date        NOT NULL,
    sent_at timestamptz NOT NULL,
    PRIMARY KEY (rule_id, day)
);

ALTER SCHEMA alerts OWNER TO ot_owner;
ALTER TABLE alerts.sent OWNER TO ot_owner;

GRANT USAGE ON SCHEMA alerts TO ot_w_alerts;
GRANT INSERT, SELECT ON alerts.sent TO ot_w_alerts;

-- +goose Down
REVOKE INSERT, SELECT ON alerts.sent FROM ot_w_alerts;
REVOKE USAGE ON SCHEMA alerts FROM ot_w_alerts;
DROP TABLE alerts.sent;
DROP SCHEMA alerts;
