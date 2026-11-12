-- +goose Up
-- docs/design/option-open-gap-jp.md entry 2: the confirmed band of an option list group, the reasons of a run and
-- the two rd views the ops page reads. The band columns are null until a listing stores one; no grant changes on
-- master.option_list.
ALTER TABLE master.option_list
    ADD COLUMN band_lo      numeric,
    ADD COLUMN band_hi      numeric,
    ADD COLUMN sigma        numeric,
    ADD COLUMN in_band      integer,
    ADD COLUMN sigma_source text,
    ADD CONSTRAINT option_list_band_ck CHECK (band_lo IS NULL OR band_hi IS NULL OR band_lo <= band_hi),
    ADD CONSTRAINT option_list_sigma_source_ck CHECK (sigma_source IS NULL OR sigma_source IN ('iv', 'rv20', 'default'));

-- ops.option_run_reason: why a run of a (dataset, subject) was short, per scope. A replace deletes the set of the
-- (session_day, dataset_id, subject_key) and inserts the new one, so a clean run clears an earlier reason.
CREATE TABLE ops.option_run_reason (
    session_day date        NOT NULL,
    dataset_id  text        NOT NULL,
    subject_key text        NOT NULL,
    scope       text        NOT NULL,
    code        text        NOT NULL,
    detail      text        NOT NULL DEFAULT '',
    at          timestamptz NOT NULL,
    PRIMARY KEY (session_day, dataset_id, subject_key, scope),
    CONSTRAINT option_run_reason_code_ck CHECK (code IN ('no_con_id', 'no_close', 'no_chain', 'no_list',
        'no_stock_list', 'fetch_failed', 'no_spot', 'late_start', 'held_stale', 'outside_range', 'not_wired'))
);

-- Every row passes through, every column named.
CREATE VIEW rd.capture_run_r1 AS
SELECT session_day, dataset_id, subject_key, status, finished_at, returned_n, usable_n, error_code, reason_code
FROM ops.capture_run;

CREATE VIEW rd.option_run_reason_r1 AS
SELECT session_day, dataset_id, subject_key, scope, code, detail, at
FROM ops.option_run_reason;

ALTER TABLE ops.option_run_reason OWNER TO ot_owner;
ALTER VIEW rd.capture_run_r1 OWNER TO ot_owner;
ALTER VIEW rd.option_run_reason_r1 OWNER TO ot_owner;

-- The jobs write through the runner's ledger store (role ot_runner). SELECT is what ON CONFLICT DO UPDATE needs,
-- DELETE is the replace; the blanket grant of 0003 does not cover later tables.
GRANT SELECT, INSERT, UPDATE, DELETE ON ops.option_run_reason TO ot_runner;

GRANT USAGE ON SCHEMA rd TO ot_ro_pages, ot_runner;
GRANT SELECT ON rd.capture_run_r1, rd.option_run_reason_r1 TO ot_ro_pages;
GRANT SELECT ON rd.capture_run_r1, rd.option_run_reason_r1 TO ot_runner;

-- +goose Down
REVOKE SELECT ON rd.capture_run_r1, rd.option_run_reason_r1 FROM ot_runner;
REVOKE SELECT ON rd.capture_run_r1, rd.option_run_reason_r1 FROM ot_ro_pages;
REVOKE SELECT, INSERT, UPDATE, DELETE ON ops.option_run_reason FROM ot_runner;
DROP VIEW rd.option_run_reason_r1;
DROP VIEW rd.capture_run_r1;
DROP TABLE ops.option_run_reason;
ALTER TABLE master.option_list
    DROP COLUMN sigma_source,
    DROP COLUMN in_band,
    DROP COLUMN sigma,
    DROP COLUMN band_hi,
    DROP COLUMN band_lo;
