-- +goose Up
-- obs.dividend_expectation r1: one answer about the expected dividend of one instrument on one session day, append
-- only (docs/design/event-sources.md Decision 7). An error is a row and a later success is a new attempt. The rows hold
-- dates, decimal text and status words; no account data. The role ot_w_events (made by internal/store/setup) gets
-- INSERT on the table and SELECT on the view; ot_ro_pages gets SELECT on the view. Neither reads the table; no role
-- but the owner may UPDATE, DELETE or TRUNCATE.
CREATE SCHEMA IF NOT EXISTS obs;
CREATE SCHEMA IF NOT EXISTS rd;

CREATE TABLE obs.dividend_expectation (
    session_day   date        NOT NULL,
    instrument_id integer     NOT NULL,
    attempt       integer     NOT NULL,
    issuer        text        NOT NULL,
    status        text        NOT NULL,
    past_12m      text,
    next_12m      text,
    next_amount   text,
    next_ex_date  date,
    currency      text        NOT NULL,
    error_code    text        NOT NULL,
    source        text        NOT NULL,
    received_at   timestamptz NOT NULL,
    PRIMARY KEY (session_day, instrument_id, attempt),
    CONSTRAINT dividend_expectation_attempt_ck CHECK (attempt >= 1),
    CONSTRAINT dividend_expectation_status_ck CHECK (status IN ('present', 'no_value', 'unsupported', 'error')),
    CONSTRAINT dividend_expectation_source_ck CHECK (source IN ('ibkr_tick_456', 'none')),
    CONSTRAINT dividend_expectation_present_ck CHECK (
        (status = 'present') = (next_ex_date IS NOT NULL AND next_amount IS NOT NULL)
        AND (status = 'present' OR (next_ex_date IS NULL AND next_amount IS NULL))),
    CONSTRAINT dividend_expectation_unsupported_ck CHECK (status <> 'unsupported' OR source = 'none')
);

ALTER TABLE obs.dividend_expectation OWNER TO ot_owner;

CREATE VIEW rd.dividend_expectation_r1 AS
SELECT session_day, instrument_id, attempt, issuer, status, past_12m, next_12m, next_amount, next_ex_date,
       currency, error_code, source, received_at
FROM obs.dividend_expectation;

ALTER VIEW rd.dividend_expectation_r1 OWNER TO ot_owner;

GRANT USAGE ON SCHEMA obs, rd TO ot_w_events;
GRANT USAGE ON SCHEMA rd TO ot_ro_pages;
GRANT INSERT ON obs.dividend_expectation TO ot_w_events;
GRANT SELECT ON rd.dividend_expectation_r1 TO ot_w_events;
GRANT SELECT ON rd.dividend_expectation_r1 TO ot_ro_pages;

-- +goose Down
REVOKE SELECT ON rd.dividend_expectation_r1 FROM ot_ro_pages;
REVOKE SELECT ON rd.dividend_expectation_r1 FROM ot_w_events;
REVOKE INSERT ON obs.dividend_expectation FROM ot_w_events;
DROP VIEW rd.dividend_expectation_r1;
DROP TABLE obs.dividend_expectation;
