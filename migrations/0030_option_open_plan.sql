-- +goose Up
-- plan.option_open_item r1: one planned contract of the option open pull, written by the plan job before the pull
-- (docs/design/option-open-pull.md Decisions 4 and 5), and its view rd.option_open_item_r1. Append-only. The pages read
-- the view and no table (ot serve reads the plan, Decision 17); the role ot_w_option_open (made by
-- internal/store/setup) inserts into the table and reads the view.
CREATE SCHEMA IF NOT EXISTS plan;
CREATE SCHEMA IF NOT EXISTS rd;

CREATE TABLE plan.option_open_item (
    session_day   date             NOT NULL,
    slot          text             NOT NULL,
    plan_id       uuid             NOT NULL,
    instrument_id bigint           NOT NULL REFERENCES master.instrument (instrument_id),
    underlying_id bigint           NOT NULL REFERENCES master.instrument (instrument_id),
    planned_at    timestamptz      NOT NULL,
    class         text             NOT NULL,
    deferred      boolean          NOT NULL,
    rank          integer          NOT NULL,
    planned_spot  numeric          NOT NULL,
    sigma         numeric          NOT NULL,
    sigma_source  text             NOT NULL,
    PRIMARY KEY (session_day, slot, plan_id, instrument_id),
    CONSTRAINT option_open_item_slot_ck CHECK (slot ~ '^[0-9]{4}$'),
    CONSTRAINT option_open_item_class_ck CHECK (class IN ('around_spot', 'puts', 'calls', 'held')),
    CONSTRAINT option_open_item_rank_ck CHECK (rank >= 0),
    CONSTRAINT option_open_item_spot_ck CHECK (planned_spot >= 0 AND planned_spot < 'Infinity'::numeric),
    CONSTRAINT option_open_item_sigma_ck CHECK (sigma >= 0 AND sigma < 'Infinity'::numeric),
    CONSTRAINT option_open_item_sigma_source_ck CHECK (sigma_source IN ('prior_close', 'last_known', 'default'))
);

CREATE INDEX option_open_item_underlying ON plan.option_open_item (session_day, slot, underlying_id, planned_at DESC);

-- Every row passes through, every column named.
CREATE VIEW rd.option_open_item_r1 AS
SELECT session_day, slot, plan_id, instrument_id, underlying_id, planned_at, class, deferred, rank, planned_spot,
       sigma, sigma_source
FROM plan.option_open_item;

ALTER TABLE plan.option_open_item OWNER TO ot_owner;
ALTER VIEW rd.option_open_item_r1 OWNER TO ot_owner;

GRANT USAGE ON SCHEMA plan TO ot_w_option_open;
GRANT INSERT ON plan.option_open_item TO ot_w_option_open;
GRANT USAGE ON SCHEMA rd TO ot_w_option_open;
GRANT SELECT ON rd.option_open_item_r1 TO ot_w_option_open;

-- Read-only: the pages read the view and nothing else.
GRANT USAGE ON SCHEMA rd TO ot_ro_pages;
GRANT SELECT ON rd.option_open_item_r1 TO ot_ro_pages;

-- +goose Down
REVOKE SELECT ON rd.option_open_item_r1 FROM ot_ro_pages;
REVOKE SELECT ON rd.option_open_item_r1 FROM ot_w_option_open;
DROP VIEW rd.option_open_item_r1;
DROP TABLE plan.option_open_item;
DROP SCHEMA plan;
