-- +goose Up
-- docs/design/fundamentals-keys.md entry 2 (Decisions 1 to 3, 5 to 8, 13, 14), owner-only DDL, one transaction.
-- obs.earnings_date becomes one row per (issuer, period_key), obs.dividend_expectation one row per (instrument_id,
-- ex_date), obs.fundamental_fact keeps its key and gains updated_at and written_by. The answer of a day moves to the new
-- tables obs.earnings_check (issuer, observed_on) and obs.dividend_check (instrument_id, session_day); the two rd views
-- keep their names and thirteen columns (attempt is the constant 1). Rows that are not listed/present, rows that are
-- not the newest of their new key, and dividend rows whose instrument is not in master.instrument are deleted and do not
-- come back in Down (no dump, owner 2026-11-13). ot_w_events and ot_w_fundamental get UPDATE and SELECT on named
-- columns; no DELETE, no TRUNCATE, no UPDATE of a key.

-- 1. New columns, before anything is dropped.
ALTER TABLE obs.earnings_date
    ADD COLUMN period_key date,
    ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now(),
    ADD COLUMN written_by text NOT NULL DEFAULT '';
UPDATE obs.earnings_date SET updated_at = received_at;
-- Decision 2: the quarter end when the source is alphavantage (it names the quarter), the report date for the others
-- (jquants gives the fiscal-year end, owner has none). Only a listed row has a report date.
UPDATE obs.earnings_date
SET period_key = CASE WHEN source = 'alphavantage' AND fiscal_period_end IS NOT NULL THEN fiscal_period_end
                      ELSE report_date END
WHERE status = 'listed';

ALTER TABLE obs.dividend_expectation
    ADD COLUMN ex_date date,
    ADD COLUMN amount text,
    ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now(),
    ADD COLUMN written_by text NOT NULL DEFAULT '';
UPDATE obs.dividend_expectation SET updated_at = received_at, ex_date = next_ex_date, amount = next_amount;

ALTER TABLE obs.fundamental_fact
    ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now(),
    ADD COLUMN written_by text NOT NULL DEFAULT '';
UPDATE obs.fundamental_fact SET updated_at = retrieved_at;

-- 2. The two answer tables. The composite foreign keys are added once the data tables have their new keys.
CREATE TABLE obs.earnings_check (
    issuer      text        NOT NULL,
    observed_on date        NOT NULL,
    market      text        NOT NULL,
    symbol      text        NOT NULL,
    status      text        NOT NULL,
    error_code  text        NOT NULL,
    source      text        NOT NULL,
    period_key  date,
    received_at timestamptz NOT NULL,
    updated_at  timestamptz NOT NULL DEFAULT now(),
    written_by  text        NOT NULL DEFAULT '',
    CONSTRAINT earnings_check_pk PRIMARY KEY (issuer, observed_on),
    CONSTRAINT earnings_check_market_ck CHECK (market IN ('US', 'JP')),
    CONSTRAINT earnings_check_status_ck CHECK (status IN ('listed', 'absent', 'unsupported', 'error')),
    CONSTRAINT earnings_check_source_ck CHECK (source IN ('alphavantage', 'jquants', 'owner', 'none')),
    CONSTRAINT earnings_check_period_ck CHECK ((status = 'listed') = (period_key IS NOT NULL)),
    CONSTRAINT earnings_check_unsupported_ck CHECK (status <> 'unsupported' OR source = 'none')
);
ALTER TABLE obs.earnings_check OWNER TO ot_owner;

-- No foreign key from instrument_id to master.instrument: a resolve_failed answer is keyed by the con id.
CREATE TABLE obs.dividend_check (
    instrument_id bigint      NOT NULL,
    session_day   date        NOT NULL,
    issuer        text        NOT NULL,
    status        text        NOT NULL,
    error_code    text        NOT NULL,
    source        text        NOT NULL,
    currency      text        NOT NULL,
    ex_date       date,
    past_12m      text,
    next_12m      text,
    received_at   timestamptz NOT NULL,
    updated_at    timestamptz NOT NULL DEFAULT now(),
    written_by    text        NOT NULL DEFAULT '',
    CONSTRAINT dividend_check_pk PRIMARY KEY (instrument_id, session_day),
    CONSTRAINT dividend_check_status_ck CHECK (status IN ('present', 'no_value', 'unsupported', 'error')),
    CONSTRAINT dividend_check_source_ck CHECK (source IN ('ibkr_tick_456', 'none')),
    CONSTRAINT dividend_check_present_ck CHECK ((status = 'present') = (ex_date IS NOT NULL)),
    CONSTRAINT dividend_check_unsupported_ck CHECK (status <> 'unsupported' OR source = 'none')
);
ALTER TABLE obs.dividend_check OWNER TO ot_owner;

-- Fill from the stored answers: the highest attempt of each (issuer, observed_on) and (instrument_id, session_day).
INSERT INTO obs.earnings_check (issuer, observed_on, market, symbol, status, error_code, source, period_key, received_at,
                                updated_at)
SELECT DISTINCT ON (issuer, observed_on)
       issuer, observed_on, market, symbol, status, error_code, source, period_key, received_at, received_at
FROM obs.earnings_date
ORDER BY issuer, observed_on, attempt DESC;

-- An answer whose instrument is not in master.instrument cannot have a data row: it is no_value with no ex_date.
INSERT INTO obs.dividend_check (instrument_id, session_day, issuer, status, error_code, source, currency, ex_date,
                                past_12m, next_12m, received_at, updated_at)
SELECT DISTINCT ON (d.instrument_id, d.session_day)
       d.instrument_id, d.session_day, d.issuer,
       CASE WHEN m.instrument_id IS NULL THEN 'no_value' ELSE d.status END,
       d.error_code, d.source, d.currency,
       CASE WHEN m.instrument_id IS NULL OR d.status <> 'present' THEN NULL ELSE d.next_ex_date END,
       d.past_12m, d.next_12m, d.received_at, d.received_at
FROM obs.dividend_expectation d
LEFT JOIN master.instrument m ON m.instrument_id = d.instrument_id
ORDER BY d.instrument_id, d.session_day, d.attempt DESC;

-- 3. Delete what the new keys do not hold: every row that is not listed/present, then every row that is not the newest
-- (observed_on, attempt) of its new key.
DELETE FROM obs.earnings_date WHERE status <> 'listed';
DELETE FROM obs.earnings_date e
USING (SELECT observed_on, issuer, attempt,
              row_number() OVER (PARTITION BY issuer, period_key ORDER BY observed_on DESC, attempt DESC) AS rn
       FROM obs.earnings_date) r
WHERE (e.observed_on, e.issuer, e.attempt) = (r.observed_on, r.issuer, r.attempt) AND r.rn > 1;

DELETE FROM obs.dividend_expectation d
WHERE d.status <> 'present' OR NOT EXISTS (SELECT 1 FROM master.instrument m WHERE m.instrument_id = d.instrument_id);
DELETE FROM obs.dividend_expectation d
USING (SELECT session_day, instrument_id, attempt,
              row_number() OVER (PARTITION BY instrument_id, ex_date ORDER BY session_day DESC, attempt DESC) AS rn
       FROM obs.dividend_expectation) r
WHERE (d.session_day, d.instrument_id, d.attempt) = (r.session_day, r.instrument_id, r.attempt) AND r.rn > 1;

-- 4. The two views, replaced in place so owner and grants stay: the check table left joined to the data row.
CREATE OR REPLACE VIEW rd.earnings_date_r1 AS
SELECT c.observed_on, c.issuer, 1 AS attempt, c.symbol, c.market, c.status, e.report_date, e.fiscal_period_end,
       COALESCE(e.time_of_day, '') AS time_of_day, e.estimate, c.error_code, c.source, c.updated_at AS received_at
FROM obs.earnings_check c
LEFT JOIN obs.earnings_date e ON e.issuer = c.issuer AND e.period_key = c.period_key;

CREATE OR REPLACE VIEW rd.dividend_expectation_r1 AS
SELECT c.session_day, c.instrument_id::integer AS instrument_id, 1 AS attempt, c.issuer, c.status, c.past_12m,
       c.next_12m, d.amount AS next_amount, d.ex_date AS next_ex_date, c.currency, c.error_code, c.source,
       c.updated_at AS received_at
FROM obs.dividend_check c
LEFT JOIN obs.dividend_expectation d ON d.instrument_id = c.instrument_id AND d.ex_date = c.ex_date;

-- 5. Only now, with the views off them, the old keys, checks and columns go.
ALTER TABLE obs.earnings_date DROP CONSTRAINT earnings_date_pkey;
ALTER TABLE obs.earnings_date DROP CONSTRAINT earnings_date_attempt_ck;
ALTER TABLE obs.earnings_date DROP CONSTRAINT earnings_date_status_ck;
ALTER TABLE obs.earnings_date DROP CONSTRAINT earnings_date_report_date_ck;
ALTER TABLE obs.earnings_date DROP CONSTRAINT earnings_date_unsupported_ck;
ALTER TABLE obs.earnings_date DROP COLUMN observed_on, DROP COLUMN attempt, DROP COLUMN status,
    DROP COLUMN error_code;

ALTER TABLE obs.dividend_expectation DROP CONSTRAINT dividend_expectation_pkey;
ALTER TABLE obs.dividend_expectation DROP CONSTRAINT dividend_expectation_attempt_ck;
ALTER TABLE obs.dividend_expectation DROP CONSTRAINT dividend_expectation_status_ck;
ALTER TABLE obs.dividend_expectation DROP CONSTRAINT dividend_expectation_present_ck;
ALTER TABLE obs.dividend_expectation DROP CONSTRAINT dividend_expectation_unsupported_ck;
ALTER TABLE obs.dividend_expectation DROP COLUMN session_day, DROP COLUMN attempt, DROP COLUMN status,
    DROP COLUMN error_code, DROP COLUMN past_12m, DROP COLUMN next_12m, DROP COLUMN next_amount,
    DROP COLUMN next_ex_date;

-- 6. The new keys, the bigint instrument_id with its foreign key, and the answers' foreign keys.
ALTER TABLE obs.earnings_date ALTER COLUMN period_key SET NOT NULL;
ALTER TABLE obs.earnings_date ALTER COLUMN report_date SET NOT NULL;
ALTER TABLE obs.earnings_date ADD CONSTRAINT earnings_date_pk PRIMARY KEY (issuer, period_key);

ALTER TABLE obs.dividend_expectation ALTER COLUMN instrument_id TYPE bigint;
ALTER TABLE obs.dividend_expectation ALTER COLUMN ex_date SET NOT NULL;
ALTER TABLE obs.dividend_expectation ALTER COLUMN amount SET NOT NULL;
ALTER TABLE obs.dividend_expectation
    ADD CONSTRAINT dividend_expectation_instrument_fk FOREIGN KEY (instrument_id) REFERENCES master.instrument (instrument_id);
ALTER TABLE obs.dividend_expectation ADD CONSTRAINT dividend_expectation_pk PRIMARY KEY (instrument_id, ex_date);

ALTER TABLE obs.earnings_check
    ADD CONSTRAINT earnings_check_earnings_fk FOREIGN KEY (issuer, period_key) REFERENCES obs.earnings_date (issuer, period_key);
ALTER TABLE obs.dividend_check
    ADD CONSTRAINT dividend_check_dividend_fk FOREIGN KEY (instrument_id, ex_date) REFERENCES obs.dividend_expectation (instrument_id, ex_date);

-- 7. Decimal pattern checks (NULL passes). The one on the fact value allows the exponent form of a JSON number and is
-- NOT VALID, so no stored row blocks the migration.
ALTER TABLE obs.earnings_date
    ADD CONSTRAINT earnings_date_estimate_ck CHECK (estimate ~ '^-?[0-9]+(\.[0-9]+)?$');
ALTER TABLE obs.dividend_expectation
    ADD CONSTRAINT dividend_expectation_amount_ck CHECK (amount ~ '^-?[0-9]+(\.[0-9]+)?$');
ALTER TABLE obs.dividend_check
    ADD CONSTRAINT dividend_check_past_12m_ck CHECK (past_12m ~ '^-?[0-9]+(\.[0-9]+)?$'),
    ADD CONSTRAINT dividend_check_next_12m_ck CHECK (next_12m ~ '^-?[0-9]+(\.[0-9]+)?$');
ALTER TABLE obs.fundamental_fact
    ADD CONSTRAINT fundamental_fact_value_ck CHECK (value ~ '^-?[0-9]+(\.[0-9]+)?([eE][+-]?[0-9]+)?$') NOT VALID;

-- 8. Grants (Decision 8): INSERT stays table-wide, UPDATE only on non-key columns (never received_at), SELECT on the key
-- and the columns the upsert statements compare or read through EXCLUDED.
GRANT INSERT ON obs.earnings_check, obs.dividend_check TO ot_w_events;
GRANT UPDATE (symbol, market, report_date, fiscal_period_end, time_of_day, estimate, source, updated_at, written_by)
    ON obs.earnings_date TO ot_w_events;
GRANT SELECT (issuer, period_key, symbol, market, report_date, fiscal_period_end, time_of_day, estimate, source)
    ON obs.earnings_date TO ot_w_events;
GRANT UPDATE (market, symbol, status, error_code, source, period_key, updated_at, written_by)
    ON obs.earnings_check TO ot_w_events;
GRANT SELECT (issuer, observed_on, market, symbol, status, error_code, source, period_key)
    ON obs.earnings_check TO ot_w_events;
GRANT UPDATE (issuer, amount, currency, source, updated_at, written_by) ON obs.dividend_expectation TO ot_w_events;
GRANT SELECT (instrument_id, ex_date, issuer, amount, currency, source) ON obs.dividend_expectation TO ot_w_events;
GRANT UPDATE (issuer, status, error_code, source, currency, ex_date, past_12m, next_12m, updated_at, written_by)
    ON obs.dividend_check TO ot_w_events;
GRANT SELECT (instrument_id, session_day, issuer, status, error_code, source, currency, ex_date, past_12m, next_12m)
    ON obs.dividend_check TO ot_w_events;
GRANT UPDATE (currency, value, form, filed, retrieved_at, raw_sha256, updated_at, written_by)
    ON obs.fundamental_fact TO ot_w_fundamental;
GRANT SELECT (source, subject, doc_id, taxonomy, concept, unit, period_start_key, period_end, currency, value, form,
              filed, raw_sha256, retrieved_at) ON obs.fundamental_fact TO ot_w_fundamental;

-- +goose Down
-- Restores the old columns and keys from the check tables (attempt 1), drops the checks, and does not restore the rows
-- Up deleted. A data row no answer names comes back as a listed/present answer of its received_at day.
CREATE TEMP TABLE earnings_old ON COMMIT DROP AS
SELECT c.observed_on, c.issuer, 1 AS attempt, c.symbol, c.market, c.status, e.report_date, e.fiscal_period_end,
       COALESCE(e.time_of_day, '') AS time_of_day, e.estimate, c.error_code, c.source, c.received_at
FROM obs.earnings_check c
LEFT JOIN obs.earnings_date e ON e.issuer = c.issuer AND e.period_key = c.period_key
UNION ALL
SELECT e.received_at::date, e.issuer, 1, e.symbol, e.market, 'listed', e.report_date, e.fiscal_period_end,
       e.time_of_day, e.estimate, '', e.source, e.received_at
FROM obs.earnings_date e
WHERE NOT EXISTS (SELECT 1 FROM obs.earnings_check c
                  WHERE c.issuer = e.issuer AND (c.period_key = e.period_key OR c.observed_on = e.received_at::date));

CREATE TEMP TABLE dividend_old ON COMMIT DROP AS
SELECT c.session_day, c.instrument_id::integer AS instrument_id, 1 AS attempt, c.issuer, c.status, c.past_12m,
       c.next_12m, d.amount AS next_amount, d.ex_date AS next_ex_date, c.currency, c.error_code, c.source,
       c.received_at
FROM obs.dividend_check c
LEFT JOIN obs.dividend_expectation d ON d.instrument_id = c.instrument_id AND d.ex_date = c.ex_date
UNION ALL
SELECT d.received_at::date, d.instrument_id::integer, 1, d.issuer, 'present', NULL, NULL, d.amount, d.ex_date,
       d.currency, '', d.source, d.received_at
FROM obs.dividend_expectation d
WHERE NOT EXISTS (SELECT 1 FROM obs.dividend_check c
                  WHERE c.instrument_id = d.instrument_id AND (c.ex_date = d.ex_date OR c.session_day = d.received_at::date));

REVOKE SELECT (source, subject, doc_id, taxonomy, concept, unit, period_start_key, period_end, currency, value, form,
               filed, raw_sha256, retrieved_at) ON obs.fundamental_fact FROM ot_w_fundamental;
REVOKE UPDATE (currency, value, form, filed, retrieved_at, raw_sha256, updated_at, written_by)
    ON obs.fundamental_fact FROM ot_w_fundamental;
REVOKE SELECT (instrument_id, ex_date, issuer, amount, currency, source) ON obs.dividend_expectation FROM ot_w_events;
REVOKE UPDATE (issuer, amount, currency, source, updated_at, written_by) ON obs.dividend_expectation FROM ot_w_events;
REVOKE SELECT (issuer, period_key, symbol, market, report_date, fiscal_period_end, time_of_day, estimate, source)
    ON obs.earnings_date FROM ot_w_events;
REVOKE UPDATE (symbol, market, report_date, fiscal_period_end, time_of_day, estimate, source, updated_at, written_by)
    ON obs.earnings_date FROM ot_w_events;

DROP VIEW rd.dividend_expectation_r1;
DROP VIEW rd.earnings_date_r1;
DROP TABLE obs.dividend_check;
DROP TABLE obs.earnings_check;

ALTER TABLE obs.fundamental_fact DROP CONSTRAINT fundamental_fact_value_ck;
ALTER TABLE obs.fundamental_fact DROP COLUMN updated_at, DROP COLUMN written_by;

DELETE FROM obs.earnings_date;
ALTER TABLE obs.earnings_date DROP CONSTRAINT earnings_date_estimate_ck;
ALTER TABLE obs.earnings_date DROP CONSTRAINT earnings_date_pk;
ALTER TABLE obs.earnings_date DROP COLUMN period_key, DROP COLUMN updated_at, DROP COLUMN written_by;
ALTER TABLE obs.earnings_date ALTER COLUMN report_date DROP NOT NULL;
ALTER TABLE obs.earnings_date
    ADD COLUMN observed_on date NOT NULL,
    ADD COLUMN attempt integer NOT NULL,
    ADD COLUMN status text NOT NULL,
    ADD COLUMN error_code text NOT NULL;
ALTER TABLE obs.earnings_date ADD PRIMARY KEY (observed_on, issuer, attempt);
ALTER TABLE obs.earnings_date
    ADD CONSTRAINT earnings_date_attempt_ck CHECK (attempt >= 1),
    ADD CONSTRAINT earnings_date_status_ck CHECK (status IN ('listed', 'absent', 'unsupported', 'error')),
    ADD CONSTRAINT earnings_date_report_date_ck CHECK ((status = 'listed') = (report_date IS NOT NULL)),
    ADD CONSTRAINT earnings_date_unsupported_ck CHECK (status <> 'unsupported' OR source = 'none');
INSERT INTO obs.earnings_date (observed_on, issuer, attempt, symbol, market, status, report_date, fiscal_period_end,
                               time_of_day, estimate, error_code, source, received_at)
SELECT observed_on, issuer, attempt, symbol, market, status, report_date, fiscal_period_end, time_of_day, estimate,
       error_code, source, received_at
FROM earnings_old;

DELETE FROM obs.dividend_expectation;
ALTER TABLE obs.dividend_expectation DROP CONSTRAINT dividend_expectation_amount_ck;
ALTER TABLE obs.dividend_expectation DROP CONSTRAINT dividend_expectation_pk;
ALTER TABLE obs.dividend_expectation DROP CONSTRAINT dividend_expectation_instrument_fk;
ALTER TABLE obs.dividend_expectation DROP COLUMN ex_date, DROP COLUMN amount, DROP COLUMN updated_at,
    DROP COLUMN written_by;
ALTER TABLE obs.dividend_expectation ALTER COLUMN instrument_id TYPE integer;
ALTER TABLE obs.dividend_expectation
    ADD COLUMN session_day date NOT NULL,
    ADD COLUMN attempt integer NOT NULL,
    ADD COLUMN status text NOT NULL,
    ADD COLUMN past_12m text,
    ADD COLUMN next_12m text,
    ADD COLUMN next_amount text,
    ADD COLUMN next_ex_date date,
    ADD COLUMN error_code text NOT NULL;
ALTER TABLE obs.dividend_expectation ADD PRIMARY KEY (session_day, instrument_id, attempt);
ALTER TABLE obs.dividend_expectation
    ADD CONSTRAINT dividend_expectation_attempt_ck CHECK (attempt >= 1),
    ADD CONSTRAINT dividend_expectation_status_ck CHECK (status IN ('present', 'no_value', 'unsupported', 'error')),
    ADD CONSTRAINT dividend_expectation_present_ck CHECK (
        (status = 'present') = (next_ex_date IS NOT NULL AND next_amount IS NOT NULL)
        AND (status = 'present' OR (next_ex_date IS NULL AND next_amount IS NULL))),
    ADD CONSTRAINT dividend_expectation_unsupported_ck CHECK (status <> 'unsupported' OR source = 'none');
INSERT INTO obs.dividend_expectation (session_day, instrument_id, attempt, issuer, status, past_12m, next_12m,
                                      next_amount, next_ex_date, currency, error_code, source, received_at)
SELECT session_day, instrument_id, attempt, issuer, status, past_12m, next_12m, next_amount, next_ex_date, currency,
       error_code, source, received_at
FROM dividend_old;

CREATE VIEW rd.earnings_date_r1 AS
SELECT observed_on, issuer, attempt, symbol, market, status, report_date, fiscal_period_end, time_of_day,
       estimate, error_code, source, received_at
FROM obs.earnings_date;
ALTER VIEW rd.earnings_date_r1 OWNER TO ot_owner;
GRANT SELECT ON rd.earnings_date_r1 TO ot_w_events;
GRANT SELECT ON rd.earnings_date_r1 TO ot_ro_pages;

CREATE VIEW rd.dividend_expectation_r1 AS
SELECT session_day, instrument_id, attempt, issuer, status, past_12m, next_12m, next_amount, next_ex_date,
       currency, error_code, source, received_at
FROM obs.dividend_expectation;
ALTER VIEW rd.dividend_expectation_r1 OWNER TO ot_owner;
GRANT SELECT ON rd.dividend_expectation_r1 TO ot_w_events;
GRANT SELECT ON rd.dividend_expectation_r1 TO ot_ro_pages;
