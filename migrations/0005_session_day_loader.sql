-- +goose Up
GRANT USAGE ON SCHEMA master TO ot_loader;
GRANT SELECT, INSERT ON master.session_day TO ot_loader;
GRANT UPDATE (is_session, open_at, close_at, early_close) ON master.session_day TO ot_loader;

-- +goose Down
REVOKE UPDATE (is_session, open_at, close_at, early_close) ON master.session_day FROM ot_loader;
REVOKE SELECT, INSERT ON master.session_day FROM ot_loader;
REVOKE USAGE ON SCHEMA master FROM ot_loader;
