-- +goose Up
-- master.option_list and master.option_list_run: what the pre-open list job stored and why a run stopped
-- (docs/design/option-contract-lists.md Decision 6).
-- option_list: one row per (underlying, expiry, class) asked of the broker. listed_at and listed_session are null
-- until a listing is stored; a short or error outcome updates tried_at and outcome and keeps them.
CREATE TABLE master.option_list (
    underlying_id  bigint      NOT NULL REFERENCES master.instrument (instrument_id),
    expiry         date        NOT NULL,
    class          text        NOT NULL,
    listed_at      timestamptz,
    listed_session date,
    count          integer     NOT NULL DEFAULT 0,
    tried_at       timestamptz NOT NULL,
    outcome        text        NOT NULL,
    PRIMARY KEY (underlying_id, expiry, class),
    CONSTRAINT option_list_class_ck CHECK (class <> ''),
    CONSTRAINT option_list_count_ck CHECK (count >= 0),
    CONSTRAINT option_list_outcome_ck CHECK (outcome IN ('listed', 'short', 'error'))
);

-- option_list_run: one row per (session, underlying). expected is null until the name's chain answer is known; a
-- non-empty expected is never replaced (the writer's rule), listed is recounted by the job.
CREATE TABLE master.option_list_run (
    session       date        NOT NULL,
    underlying_id bigint      NOT NULL REFERENCES master.instrument (instrument_id),
    expected      text[],
    listed        integer     NOT NULL DEFAULT 0,
    reason        text        NOT NULL,
    updated_at    timestamptz NOT NULL,
    PRIMARY KEY (session, underlying_id),
    CONSTRAINT option_list_run_listed_ck CHECK (listed >= 0),
    CONSTRAINT option_list_run_reason_ck CHECK (reason IN ('not_finished', 'none', 'no_expiry_in_window',
        'listchains_failed', 'budget', 'stop_time', 'timeouts', 'unserved', 'broker_down', 'list_max_new',
        'all_short', 'some_short'))
);

ALTER TABLE master.option_list OWNER TO ot_owner;
ALTER TABLE master.option_list_run OWNER TO ot_owner;

GRANT USAGE ON SCHEMA master TO ot_w_option_open;
GRANT INSERT, UPDATE, SELECT ON master.option_list TO ot_w_option_open;
GRANT INSERT, UPDATE, SELECT ON master.option_list_run TO ot_w_option_open;

-- +goose Down
REVOKE INSERT, UPDATE, SELECT ON master.option_list_run FROM ot_w_option_open;
REVOKE INSERT, UPDATE, SELECT ON master.option_list FROM ot_w_option_open;
DROP TABLE master.option_list_run;
DROP TABLE master.option_list;
