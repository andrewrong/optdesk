-- +goose Up
-- obs.earnings_date r1: one answer of one earnings-date source for one issuer on one session day, append only
-- (docs/design/event-sources.md Decision 7). An error is a row and a later success is a new attempt. The rows hold
-- dates, status words and vendor text; no account data. The role ot_w_events (made by internal/store/setup) gets
-- INSERT on the table and SELECT on the view; ot_ro_pages gets SELECT on the view. Neither reads the table; no role
-- but the owner may UPDATE, DELETE or TRUNCATE.
CREATE SCHEMA IF NOT EXISTS obs;
CREATE SCHEMA IF NOT EXISTS rd;

CREATE TABLE obs.earnings_date (
    observed_on       date        NOT NULL,
    issuer            text        NOT NULL,
    attempt           integer     NOT NULL,
    symbol            text        NOT NULL,
    market            text        NOT NULL,
    status            text        NOT NULL,
    report_date       date,
    fiscal_period_end date,
    time_of_day       text        NOT NULL,
    estimate          text,
    error_code        text        NOT NULL,
    source            text        NOT NULL,
    received_at       timestamptz NOT NULL,
    PRIMARY KEY (observed_on, issuer, attempt),
    CONSTRAINT earnings_date_attempt_ck CHECK (attempt >= 1),
    CONSTRAINT earnings_date_market_ck CHECK (market IN ('US', 'JP')),
    CONSTRAINT earnings_date_status_ck CHECK (status IN ('listed', 'absent', 'unsupported', 'error')),
    CONSTRAINT earnings_date_source_ck CHECK (source IN ('alphavantage', 'jquants', 'owner', 'none')),
    CONSTRAINT earnings_date_report_date_ck CHECK ((status = 'listed') = (report_date IS NOT NULL)),
    CONSTRAINT earnings_date_unsupported_ck CHECK (status <> 'unsupported' OR source = 'none')
);

ALTER TABLE obs.earnings_date OWNER TO ot_owner;

CREATE VIEW rd.earnings_date_r1 AS
SELECT observed_on, issuer, attempt, symbol, market, status, report_date, fiscal_period_end, time_of_day,
       estimate, error_code, source, received_at
FROM obs.earnings_date;

ALTER VIEW rd.earnings_date_r1 OWNER TO ot_owner;

GRANT USAGE ON SCHEMA obs, rd TO ot_w_events;
GRANT USAGE ON SCHEMA rd TO ot_ro_pages;
GRANT INSERT ON obs.earnings_date TO ot_w_events;
GRANT SELECT ON rd.earnings_date_r1 TO ot_w_events;
GRANT SELECT ON rd.earnings_date_r1 TO ot_ro_pages;

-- +goose Down
REVOKE SELECT ON rd.earnings_date_r1 FROM ot_ro_pages;
REVOKE SELECT ON rd.earnings_date_r1 FROM ot_w_events;
REVOKE INSERT ON obs.earnings_date FROM ot_w_events;
REVOKE USAGE ON SCHEMA obs, rd FROM ot_w_events;
DROP VIEW rd.earnings_date_r1;
DROP TABLE obs.earnings_date;
