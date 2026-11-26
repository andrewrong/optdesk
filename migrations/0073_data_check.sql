-- +goose Up
-- docs/design/data-check.md entry 3 (Decisions 7, 10, 14): the runs and findings of the daily data check. All three
-- tables are insert-only: a rerun is a new run_id. ot_w_datacheck (made by internal/store/setup) has INSERT and SELECT on
-- them and no UPDATE, DELETE or TRUNCATE; ot_ro_pages reads the two views only. A finding holds ids and counts, never
-- free text: evidence is a JSON list of {name, n} with name from a fixed vocabulary.
CREATE SCHEMA IF NOT EXISTS datacheck;
CREATE SCHEMA IF NOT EXISTS rd;

CREATE TABLE datacheck.run (
    run_id        bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    check_day     date        NOT NULL,
    started_at    timestamptz NOT NULL,
    finished_at   timestamptz NOT NULL,
    rules_total   integer     NOT NULL,
    rules_unknown integer     NOT NULL,
    CONSTRAINT run_counts_ck CHECK (rules_total >= 0 AND rules_unknown >= 0 AND rules_unknown <= rules_total)
);

CREATE INDEX run_check_day ON datacheck.run (check_day DESC, run_id DESC);

CREATE TABLE datacheck.finding (
    finding_id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    run_id     bigint  NOT NULL REFERENCES datacheck.run (run_id),
    rule       text    NOT NULL,
    dataset    text    NOT NULL,
    day        date    NOT NULL,
    subject    text    NOT NULL DEFAULT '',
    severity   text    NOT NULL,
    evidence   text    NOT NULL,
    is_new     boolean NOT NULL,
    CONSTRAINT finding_severity_ck CHECK (severity IN ('error', 'warn', 'unknown')),
    CONSTRAINT finding_key_uq UNIQUE (run_id, rule, dataset, day, subject)
);

CREATE TABLE datacheck.notice (
    run_id      bigint      PRIMARY KEY REFERENCES datacheck.run (run_id),
    noticed_at  timestamptz NOT NULL
);

ALTER SCHEMA datacheck OWNER TO ot_owner;
ALTER TABLE datacheck.run OWNER TO ot_owner;
ALTER TABLE datacheck.finding OWNER TO ot_owner;
ALTER TABLE datacheck.notice OWNER TO ot_owner;

-- The views run with the rights of their owner, so the page role needs nothing on schema datacheck.
CREATE VIEW rd.data_check_run_r1 AS
SELECT r.run_id, r.check_day, r.started_at, r.finished_at, r.rules_total, r.rules_unknown
FROM datacheck.run r;

CREATE VIEW rd.data_finding_r1 AS
SELECT f.finding_id, f.run_id, f.rule, f.dataset, f.day, f.subject, f.severity, f.evidence, f.is_new
FROM datacheck.finding f;

ALTER VIEW rd.data_check_run_r1 OWNER TO ot_owner;
ALTER VIEW rd.data_finding_r1 OWNER TO ot_owner;

GRANT USAGE ON SCHEMA datacheck TO ot_w_datacheck;
GRANT INSERT, SELECT ON datacheck.run, datacheck.finding, datacheck.notice TO ot_w_datacheck;
GRANT USAGE ON SCHEMA rd TO ot_w_datacheck;
GRANT SELECT ON rd.coverage_day_r1, rd.session_day_r1, rd.reading_r1, rd.option_open_r1, rd.option_candidate_r1,
    rd.rule_param_r1, rd.option_exclusion_r1, rd.job_gap_r1, rd.data_check_run_r1, rd.data_finding_r1
    TO ot_w_datacheck;

GRANT USAGE ON SCHEMA rd TO ot_ro_pages;
GRANT SELECT ON rd.data_check_run_r1, rd.data_finding_r1 TO ot_ro_pages;

-- +goose Down
-- USAGE on schema rd stays granted to ot_ro_pages (shared with the other views); schema rd stays.
REVOKE SELECT ON rd.data_check_run_r1, rd.data_finding_r1 FROM ot_ro_pages;
REVOKE SELECT ON rd.coverage_day_r1, rd.session_day_r1, rd.reading_r1, rd.option_open_r1, rd.option_candidate_r1,
    rd.rule_param_r1, rd.option_exclusion_r1, rd.job_gap_r1, rd.data_check_run_r1, rd.data_finding_r1
    FROM ot_w_datacheck;
REVOKE USAGE ON SCHEMA rd FROM ot_w_datacheck;
REVOKE INSERT, SELECT ON datacheck.run, datacheck.finding, datacheck.notice FROM ot_w_datacheck;
REVOKE USAGE ON SCHEMA datacheck FROM ot_w_datacheck;
DROP VIEW rd.data_finding_r1;
DROP VIEW rd.data_check_run_r1;
DROP TABLE datacheck.notice;
DROP TABLE datacheck.finding;
DROP TABLE datacheck.run;
DROP SCHEMA datacheck;
