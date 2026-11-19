-- +goose Up
-- docs/design/scheduler-per-job.md entry 5 (Decisions 1, 6, 8): one row per run of a whole job. The row records; it is
-- not a guard, so there is no unique index on job_id: a crash leaves a row open and an index would refuse that job for
-- ever. serve closes every open row 'cut' at start-up. outcome is '' while the run is open, then ok, failed or cut.
-- Timestamps other than started_at are null until they happen.
CREATE SCHEMA IF NOT EXISTS ops;
CREATE SCHEMA IF NOT EXISTS rd;

CREATE TABLE ops.job_run (
    id               text        NOT NULL,
    job_id           text        NOT NULL,
    source           text        NOT NULL DEFAULT '',
    started_at       timestamptz NOT NULL,
    heartbeat_at     timestamptz,
    finished_at      timestamptz,
    slow_notified_at timestamptz,
    keys_done        integer     NOT NULL DEFAULT 0,
    keys_total       integer     NOT NULL DEFAULT 0,
    step             text        NOT NULL DEFAULT '',
    outcome          text        NOT NULL DEFAULT '',
    PRIMARY KEY (id),
    CONSTRAINT job_run_outcome_ck CHECK (outcome IN ('', 'ok', 'failed', 'cut')),
    CONSTRAINT job_run_keys_ck CHECK (keys_done >= 0 AND keys_total >= 0)
);

-- The newest ok runs of a job id are read by the usual-duration view; open runs by the running view.
CREATE INDEX job_run_job_finished ON ops.job_run (job_id, finished_at DESC) WHERE outcome = 'ok';
CREATE INDEX job_run_open ON ops.job_run (started_at) WHERE outcome = '';

ALTER TABLE ops.job_run OWNER TO ot_owner;

-- The open runs.
CREATE VIEW rd.job_run_r1 AS
SELECT id, job_id, source, started_at, heartbeat_at, slow_notified_at, keys_done, keys_total, step
FROM ops.job_run
WHERE outcome = '';

-- The usual duration of a job id: the median, in seconds, of the last 20 ok runs and how many runs it has. cut and
-- failed runs are not samples.
CREATE VIEW rd.job_usual_r1 AS
WITH ranked AS (
    SELECT job_id,
           EXTRACT(EPOCH FROM (finished_at - started_at))::double precision AS secs,
           row_number() OVER (PARTITION BY job_id ORDER BY finished_at DESC, id DESC) AS rn
    FROM ops.job_run
    WHERE outcome = 'ok' AND finished_at IS NOT NULL
)
SELECT job_id,
       percentile_cont(0.5) WITHIN GROUP (ORDER BY secs) AS median_seconds,
       count(*)::integer AS samples
FROM ranked
WHERE rn <= 20
GROUP BY job_id;

ALTER VIEW rd.job_run_r1 OWNER TO ot_owner;
ALTER VIEW rd.job_usual_r1 OWNER TO ot_owner;

GRANT USAGE ON SCHEMA rd TO ot_ro_pages, ot_runner;
GRANT SELECT ON rd.job_run_r1, rd.job_usual_r1 TO ot_ro_pages, ot_runner;
-- ot_runner is the only writer. SELECT is for the WHERE of its UPDATEs (0003 granted SELECT on the tables that existed).
GRANT USAGE ON SCHEMA ops TO ot_runner;
GRANT SELECT, INSERT, UPDATE ON ops.job_run TO ot_runner;

-- +goose Down
-- Schemas rd and ops and the USAGE grants on them are shared and stay.
REVOKE SELECT, INSERT, UPDATE ON ops.job_run FROM ot_runner;
REVOKE SELECT ON rd.job_run_r1, rd.job_usual_r1 FROM ot_ro_pages, ot_runner;
DROP VIEW rd.job_usual_r1;
DROP VIEW rd.job_run_r1;
DROP TABLE ops.job_run;
