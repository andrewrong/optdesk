-- +goose Up
-- The scheduler's tables (writer ot_runner, owner ot_owner) and the two views pages read. ot_ro_pages gets no grant on
-- an ops table: it reads rd.job_gap_r1 and rd.expectation_r1 only.

-- One record per gap. A key gap (expired, partial, crashed_repeatedly) has dataset_id and day and an empty job_id; a
-- preflight_failed gap has job_id and an empty dataset_id. first_seen_at, reported_at and cleared_at are the database's now().
CREATE TABLE ops.job_gap (
    job_id        text        NOT NULL DEFAULT '',
    dataset_id    text        NOT NULL DEFAULT '',
    day           date        NOT NULL,
    reason        text        NOT NULL,
    first_seen_at timestamptz NOT NULL DEFAULT now(),
    reported_at   timestamptz,
    cleared_at    timestamptz
);
-- At most one open gap per key; a cleared gap stays as history and the key may open again.
CREATE UNIQUE INDEX job_gap_open_key ON ops.job_gap (job_id, dataset_id, day) WHERE cleared_at IS NULL;

-- One row: the build of the running scheduler.
CREATE TABLE ops.scheduler_state (
    id         boolean     NOT NULL DEFAULT true,
    build      text        NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (id),
    CHECK (id)
);

-- The state of each job as the ticker last saw it; PutJobStates replaces all rows.
CREATE TABLE ops.scheduler_job (
    job_id            text    NOT NULL,
    next_start        timestamptz,
    window_end        timestamptz,
    restart_sensitive boolean NOT NULL DEFAULT false,
    disabled_reason   text    NOT NULL DEFAULT '',
    PRIMARY KEY (job_id)
);

ALTER TABLE ops.job_gap OWNER TO ot_owner;
ALTER TABLE ops.scheduler_state OWNER TO ot_owner;
ALTER TABLE ops.scheduler_job OWNER TO ot_owner;

GRANT SELECT, INSERT ON ops.job_gap TO ot_runner;
GRANT UPDATE (reason, reported_at, cleared_at) ON ops.job_gap TO ot_runner;
GRANT SELECT, INSERT, UPDATE ON ops.scheduler_state TO ot_runner;
GRANT SELECT, INSERT, DELETE ON ops.scheduler_job TO ot_runner;

-- The gaps, open and cleared.
CREATE VIEW rd.job_gap_r1 AS
SELECT job_id, dataset_id, day, reason, first_seen_at, reported_at, cleared_at
FROM ops.job_gap;

-- One row per dataset window. due_by is the close of the window; present_day is the latest day on or before the
-- expected day that has a present key and whose expected keys are all present or no_value; gap is the reason of the open gap of the dataset on that day ('' when none). A window with neither
-- open_at nor close_at is a day without a session: closed.
CREATE VIEW rd.expectation_r1 AS
SELECT w.dataset_id,
       w.session_day AS expected_day,
       (SELECT max(p.session_day) FROM ops.capture_present p
         WHERE p.dataset_id = w.dataset_id AND p.status = 'present' AND p.session_day <= w.session_day
           AND NOT EXISTS (SELECT 1 FROM ops.capture_expected_key k
                            WHERE k.dataset_id = p.dataset_id AND k.session_day = p.session_day
                              AND NOT EXISTS (SELECT 1 FROM ops.capture_present q
                                               WHERE q.session_day = k.session_day AND q.dataset_id = k.dataset_id
                                                 AND q.subject_key = k.subject_key
                                                 AND q.status IN ('present', 'no_value')))) AS present_day,
       w.close_at AS due_by,
       COALESCE((SELECT g.reason FROM ops.job_gap g
                  WHERE g.dataset_id = w.dataset_id AND g.day = w.session_day AND g.cleared_at IS NULL
                  ORDER BY g.first_seen_at LIMIT 1), '') AS gap,
       (w.open_at IS NULL AND w.close_at IS NULL) AS closed
FROM ops.dataset_window w;

ALTER VIEW rd.job_gap_r1 OWNER TO ot_owner;
ALTER VIEW rd.expectation_r1 OWNER TO ot_owner;

GRANT USAGE ON SCHEMA rd TO ot_ro_pages, ot_runner;
GRANT SELECT ON rd.job_gap_r1 TO ot_ro_pages, ot_runner;
GRANT SELECT ON rd.expectation_r1 TO ot_ro_pages, ot_runner;

-- +goose Down
DROP VIEW rd.expectation_r1;
DROP VIEW rd.job_gap_r1;
DROP TABLE ops.scheduler_job;
DROP TABLE ops.scheduler_state;
DROP TABLE ops.job_gap;
