-- +goose Up
GRANT USAGE ON SCHEMA master TO ot_runner;
GRANT SELECT ON master.session_day TO ot_runner;

-- +goose Down
REVOKE SELECT ON master.session_day FROM ot_runner;
REVOKE USAGE ON SCHEMA master FROM ot_runner;
