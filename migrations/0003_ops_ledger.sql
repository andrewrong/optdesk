-- +goose Up
-- The ops ledger of the runner: one writer (ot_runner), no reader of its own.
CREATE SCHEMA IF NOT EXISTS ops;

CREATE TABLE ops.capture_run (
    run_id       text        NOT NULL,
    session_day  date        NOT NULL,
    dataset_id   text        NOT NULL,
    subject_key  text        NOT NULL,
    status       text        NOT NULL,
    lease_until  timestamptz,
    heartbeat_at timestamptz,
    finished_at  timestamptz,
    returned_n   integer     NOT NULL DEFAULT 0,
    usable_n     integer     NOT NULL DEFAULT 0,
    error_code   text        NOT NULL DEFAULT '',
    reason_code  text        NOT NULL DEFAULT '',
    PRIMARY KEY (run_id),
    CHECK (status <> 'running' OR lease_until IS NOT NULL)
);
CREATE INDEX capture_run_session_day ON ops.capture_run (session_day);

-- The expected keys of a day under one plan; PutExpected replaces the plan of the day.
CREATE TABLE ops.capture_expected_key (
    session_day date NOT NULL,
    plan_hash   text NOT NULL,
    dataset_id  text NOT NULL,
    subject_key text NOT NULL,
    PRIMARY KEY (session_day, plan_hash, dataset_id, subject_key)
);

-- The keys known to be present for a day, with their status (MarkPresent).
CREATE TABLE ops.capture_present (
    session_day date NOT NULL,
    dataset_id  text NOT NULL,
    subject_key text NOT NULL,
    status      text NOT NULL,
    PRIMARY KEY (session_day, dataset_id, subject_key)
);

CREATE TABLE ops.dataset_window (
    dataset_id  text        NOT NULL,
    session_day date        NOT NULL,
    open_at     timestamptz,
    close_at    timestamptz,
    PRIMARY KEY (dataset_id, session_day)
);

-- One row: the day up to which the runner has reconciled.
CREATE TABLE ops.reconcile_cursor (
    id          boolean NOT NULL DEFAULT true,
    session_day date    NOT NULL,
    PRIMARY KEY (id),
    CHECK (id)
);

-- One row: the lease of Acquire and Release.
CREATE TABLE ops.run_lease (
    id         boolean     NOT NULL DEFAULT true,
    owner      text        NOT NULL,
    expires_at timestamptz NOT NULL,
    PRIMARY KEY (id),
    CHECK (id)
);

ALTER SCHEMA ops OWNER TO ot_owner;
ALTER TABLE ops.capture_run OWNER TO ot_owner;
ALTER TABLE ops.capture_expected_key OWNER TO ot_owner;
ALTER TABLE ops.capture_present OWNER TO ot_owner;
ALTER TABLE ops.dataset_window OWNER TO ot_owner;
ALTER TABLE ops.reconcile_cursor OWNER TO ot_owner;
ALTER TABLE ops.run_lease OWNER TO ot_owner;

GRANT USAGE ON SCHEMA ops TO ot_runner, ot_w_series_fred;

-- ot_runner is the only writer of ops.*. SELECT is needed by the reads of the ledger and by ON CONFLICT DO UPDATE.
GRANT SELECT ON ALL TABLES IN SCHEMA ops TO ot_runner;
GRANT INSERT ON ops.capture_run TO ot_runner;
GRANT UPDATE (status, finished_at, lease_until, heartbeat_at, returned_n, usable_n, error_code, reason_code)
    ON ops.capture_run TO ot_runner;
GRANT INSERT, UPDATE, DELETE ON ops.capture_expected_key, ops.capture_present, ops.dataset_window,
    ops.reconcile_cursor, ops.run_lease TO ot_runner;

-- The series writer may only refer to a run and read it.
GRANT REFERENCES, SELECT ON ops.capture_run TO ot_w_series_fred;

-- ot_ro_pages reads views of rd only: no grant on ops.

-- +goose Down
DROP TABLE ops.run_lease;
DROP TABLE ops.reconcile_cursor;
DROP TABLE ops.dataset_window;
DROP TABLE ops.capture_present;
DROP TABLE ops.capture_expected_key;
DROP TABLE ops.capture_run;
