-- +goose Up
-- docs/design/auto-catch-up.md entry 5 (Decision 9): the scheduler's runner role may insert a request, so the catch-up
-- pass posts with the connection the scheduler already holds. Only the three columns the Poster's insert sets are
-- granted; state and keys keep their defaults. The id is an identity column, so no sequence grant is needed.
GRANT INSERT (job_id, session_day, force) ON settings.job_request TO ot_runner;

-- +goose Down
REVOKE INSERT (job_id, session_day, force) ON settings.job_request FROM ot_runner;
