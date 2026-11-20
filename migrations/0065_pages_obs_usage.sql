-- +goose Up
-- Run by ot_owner, as in 0048_macro_event.sql. Since #3435 the home page reads obs.macro_event as ot_ro_pages;
-- 0048 granted SELECT on the table but no USAGE on schema obs, so the read failed with "permission denied for
-- schema obs" and /home answered 500 (#3464). The role still reads only the tables it holds SELECT on. It only adds
-- a grant; the auto-deploy waits until the window runs `setup pg` then `migrate`.
GRANT USAGE ON SCHEMA obs TO ot_ro_pages;

-- +goose Down
REVOKE USAGE ON SCHEMA obs FROM ot_ro_pages;
