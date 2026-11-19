-- +goose Up
-- docs/design/repeated-failures.md Decision 5: the newest run of a key is chosen by finished_at, then run_id, so the
-- view rd.capture_run_r1 passes run_id too. The column is added last, so CREATE OR REPLACE keeps the owner and the
-- SELECT grants of 0036 and every reader that names its columns is unchanged.
CREATE OR REPLACE VIEW rd.capture_run_r1 AS
SELECT session_day, dataset_id, subject_key, status, finished_at, returned_n, usable_n, error_code, reason_code, run_id
FROM ops.capture_run;

-- +goose Down
DROP VIEW rd.capture_run_r1;
CREATE VIEW rd.capture_run_r1 AS
SELECT session_day, dataset_id, subject_key, status, finished_at, returned_n, usable_n, error_code, reason_code
FROM ops.capture_run;
ALTER VIEW rd.capture_run_r1 OWNER TO ot_owner;
GRANT SELECT ON rd.capture_run_r1 TO ot_ro_pages, ot_runner;
