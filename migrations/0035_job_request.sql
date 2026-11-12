-- +goose Up
-- The job console's request queue (docs/design/job-console.md Decisions 2, 3 and 9). The owner's page inserts one
-- request row; the scheduler's worker updates seven columns of it and stores the answer per key in keys. A
-- withdrawal is one more row in its own table, so the web role never updates a request. No role may delete a row.
-- ot_w_settings (the web role) inserts only the columns the post sets, so it cannot insert a running row or a forged
-- keys. ot_runner is the first role other than the owner to hold USAGE on schema settings and an update in it.
CREATE SCHEMA IF NOT EXISTS settings;
CREATE SCHEMA IF NOT EXISTS rd;

CREATE TABLE settings.job_request (
    id           bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    job_id       text        NOT NULL,
    session_day  date        NOT NULL,
    force        boolean     NOT NULL DEFAULT false,
    requested_at timestamptz NOT NULL DEFAULT now(),
    state        text        NOT NULL DEFAULT 'queued'
        CHECK (state IN ('queued', 'running', 'done', 'refused', 'failed', 'interrupted', 'withdrawn')),
    started_at   timestamptz NULL,
    finished_at  timestamptz NULL,
    reason       text        NOT NULL DEFAULT '',
    hold_job     text        NOT NULL DEFAULT '',
    hold_at      timestamptz NULL,
    keys         jsonb       NULL
);

-- A double press of the same kind is one open request; the other force value makes its own.
CREATE UNIQUE INDEX job_request_open ON settings.job_request (job_id, session_day, force)
    WHERE state IN ('queued', 'running');
CREATE INDEX job_request_recent ON settings.job_request (id DESC);

CREATE TABLE settings.job_request_withdraw (
    request_id   bigint      PRIMARY KEY REFERENCES settings.job_request (id),
    withdrawn_at timestamptz NOT NULL DEFAULT now()
);

ALTER SCHEMA settings OWNER TO ot_owner;
ALTER TABLE settings.job_request OWNER TO ot_owner;
ALTER TABLE settings.job_request_withdraw OWNER TO ot_owner;

-- Every row passes through, every column named, with the withdrawal joined in.
CREATE VIEW rd.job_request_r1 AS
SELECT r.id, r.job_id, r.session_day, r.force, r.requested_at, r.state, r.started_at, r.finished_at, r.reason,
       r.hold_job, r.hold_at, r.keys, w.withdrawn_at
FROM settings.job_request r
LEFT JOIN settings.job_request_withdraw w ON w.request_id = r.id;

ALTER VIEW rd.job_request_r1 OWNER TO ot_owner;

-- The web role: read both tables, insert the columns the post sets, nothing else.
GRANT USAGE ON SCHEMA settings TO ot_w_settings;
GRANT SELECT ON settings.job_request TO ot_w_settings;
GRANT SELECT ON settings.job_request_withdraw TO ot_w_settings;
GRANT INSERT (job_id, session_day, force) ON settings.job_request TO ot_w_settings;
GRANT INSERT (request_id) ON settings.job_request_withdraw TO ot_w_settings;
GRANT USAGE ON SCHEMA rd TO ot_w_settings;
GRANT SELECT ON rd.job_request_r1 TO ot_w_settings;

-- The pages read the view and nothing else.
GRANT USAGE ON SCHEMA rd TO ot_ro_pages;
GRANT SELECT ON rd.job_request_r1 TO ot_ro_pages;

-- The runner: read, and update seven columns of the request.
GRANT USAGE ON SCHEMA settings TO ot_runner;
GRANT USAGE ON SCHEMA rd TO ot_runner;
GRANT SELECT ON settings.job_request TO ot_runner;
GRANT SELECT ON settings.job_request_withdraw TO ot_runner;
GRANT UPDATE (state, started_at, finished_at, reason, keys, hold_job, hold_at) ON settings.job_request TO ot_runner;
GRANT SELECT ON rd.job_request_r1 TO ot_runner;

-- +goose Down
-- Schemas settings and rd belong to earlier migrations and stay, as do the USAGE grants of the other roles.
DROP VIEW rd.job_request_r1;
DROP TABLE settings.job_request_withdraw;
DROP TABLE settings.job_request;
REVOKE USAGE ON SCHEMA settings FROM ot_runner;
