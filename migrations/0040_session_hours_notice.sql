-- +goose Up
-- master.session_hours_notice: one row per day on which our exchange calendar and IBKR's hours differ, and its read
-- view rd.session_hours_notice_r1 (docs/design/calendar-ibkr-hours.md Decisions 6, 7 and 9). No row is kept for a day that no longer differs:
-- a replace deletes the market's rows and inserts the differing days in one transaction. The role ot_w_master and
-- ot_ro_pages are made by internal/store/setup.
CREATE TABLE master.session_hours_notice (
    market  text        NOT NULL,
    day     date        NOT NULL,
    ours    text        NOT NULL,
    ibkr    text        NOT NULL,
    seen_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (market, day)
);

CREATE VIEW rd.session_hours_notice_r1 AS
SELECT market, day, ours, ibkr, seen_at
FROM master.session_hours_notice;

ALTER TABLE master.session_hours_notice OWNER TO ot_owner;
ALTER VIEW rd.session_hours_notice_r1 OWNER TO ot_owner;

GRANT USAGE ON SCHEMA master TO ot_w_master;
GRANT INSERT, DELETE, SELECT ON master.session_hours_notice TO ot_w_master;

-- Read-only: the pages read the view and nothing else.
GRANT USAGE ON SCHEMA rd TO ot_ro_pages, ot_w_master;
GRANT SELECT ON rd.session_hours_notice_r1 TO ot_ro_pages, ot_w_master;

-- +goose Down
DROP VIEW rd.session_hours_notice_r1;
DROP TABLE master.session_hours_notice;
