-- +goose Up
-- Run by ot_owner, which owns every object it creates, as in 0043_series_registry.sql.
-- docs/design/macro-events.md Decision 4: the stored macro events, one row per kind and date. It only adds a table
-- and its grants; the auto-deploy waits until the window runs `setup pg` then `migrate`.
CREATE TABLE obs.macro_event (
    kind       text NOT NULL,
    event_date date NOT NULL,
    source     text NOT NULL,
    source_ref text NOT NULL,
    run_day    date NOT NULL,
    held_day   date,
    CONSTRAINT macro_event_source CHECK (source IN ('fred', 'file')),
    PRIMARY KEY (kind, event_date)
);

GRANT SELECT, INSERT, UPDATE, DELETE ON obs.macro_event TO ot_w_series_fred;
GRANT SELECT ON obs.macro_event TO ot_ro_pages;

-- +goose Down
REVOKE SELECT ON obs.macro_event FROM ot_ro_pages;
REVOKE SELECT, INSERT, UPDATE, DELETE ON obs.macro_event FROM ot_w_series_fred;
DROP TABLE obs.macro_event;
