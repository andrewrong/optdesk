-- +goose Up
-- The session days for the pages, runner and loader roles. The view runs with the rights of its owner, so the page
-- role gets nothing in schema master. ot_ro_pages and ot_runner hold USAGE on rd from 0002; ot_loader does not.
CREATE VIEW rd.session_day_r1 AS
SELECT calendar_id, session_day, is_session, open_at, close_at, early_close
FROM master.session_day;

ALTER VIEW rd.session_day_r1 OWNER TO ot_owner;

GRANT SELECT ON rd.session_day_r1 TO ot_ro_pages, ot_runner, ot_loader;
GRANT USAGE ON SCHEMA rd TO ot_loader;

-- +goose Down
DROP VIEW rd.session_day_r1;
REVOKE USAGE ON SCHEMA rd FROM ot_loader;
