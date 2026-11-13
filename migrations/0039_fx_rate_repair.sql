-- +goose Up
-- docs/design/data-repair-fx.md entry 2 (Decisions 1 to 3): obs.fx_rate is upserted by key. It gains status, updated_at
-- and written_by after received_at (a positional INSERT of five values still works), rate becomes numeric, and ot_w_bar
-- gets column rights: UPDATE on rate, source, status, updated_at, written_by; SELECT on pair, rate_day, rate, source,
-- status (the arbiter and DO UPDATE ... WHERE columns). No DELETE, no TRUNCATE, no UPDATE of the key or received_at.
DROP VIEW rd.fx_rate_r1;
ALTER TABLE obs.fx_rate DROP CONSTRAINT fx_rate_rate_ck;
ALTER TABLE obs.fx_rate ALTER COLUMN rate TYPE numeric USING rate::numeric;
ALTER TABLE obs.fx_rate ALTER COLUMN rate DROP NOT NULL;
ALTER TABLE obs.fx_rate ADD COLUMN status text NOT NULL DEFAULT 'ok';
ALTER TABLE obs.fx_rate ADD CONSTRAINT fx_rate_status_ck CHECK (status IN ('ok', 'no_value', 'void'));
ALTER TABLE obs.fx_rate ADD CONSTRAINT fx_rate_rate_ck CHECK (status = 'no_value' OR rate > 0);
ALTER TABLE obs.fx_rate ADD COLUMN updated_at timestamptz;
UPDATE obs.fx_rate SET updated_at = received_at;
ALTER TABLE obs.fx_rate ALTER COLUMN updated_at SET NOT NULL;
ALTER TABLE obs.fx_rate ALTER COLUMN updated_at SET DEFAULT now();
ALTER TABLE obs.fx_rate ADD COLUMN written_by text NOT NULL DEFAULT '';

CREATE VIEW rd.fx_rate_r1 AS
SELECT pair, rate_day, rate::text AS rate, source, received_at
FROM obs.fx_rate
WHERE status = 'ok';
ALTER VIEW rd.fx_rate_r1 OWNER TO ot_owner;
GRANT SELECT ON rd.fx_rate_r1 TO ot_w_bar;
GRANT SELECT ON rd.fx_rate_r1 TO ot_ro_pages;

GRANT SELECT (pair, rate_day, rate, source, status) ON obs.fx_rate TO ot_w_bar;
GRANT UPDATE (rate, source, status, updated_at, written_by) ON obs.fx_rate TO ot_w_bar;

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM obs.fx_rate
               WHERE status <> 'ok' OR rate IS NULL OR NOT (rate::text ~ '^[0-9]+(\.[0-9]+)?$')) THEN
        RAISE EXCEPTION 'obs.fx_rate holds rows the text column cannot take (status other than ok, or a rate of another form)';
    END IF;
END
$$;
-- +goose StatementEnd
REVOKE UPDATE (rate, source, status, updated_at, written_by) ON obs.fx_rate FROM ot_w_bar;
REVOKE SELECT (pair, rate_day, rate, source, status) ON obs.fx_rate FROM ot_w_bar;
DROP VIEW rd.fx_rate_r1;
ALTER TABLE obs.fx_rate DROP CONSTRAINT fx_rate_rate_ck;
ALTER TABLE obs.fx_rate ALTER COLUMN rate TYPE text USING rate::text;
ALTER TABLE obs.fx_rate ALTER COLUMN rate SET NOT NULL;
ALTER TABLE obs.fx_rate DROP COLUMN status;
ALTER TABLE obs.fx_rate DROP COLUMN updated_at;
ALTER TABLE obs.fx_rate DROP COLUMN written_by;
ALTER TABLE obs.fx_rate ADD CONSTRAINT fx_rate_rate_ck CHECK (rate ~ '^[0-9]+(\.[0-9]+)?$' AND rate::numeric > 0);
CREATE VIEW rd.fx_rate_r1 AS
SELECT pair, rate_day, rate, source, received_at
FROM obs.fx_rate;
ALTER VIEW rd.fx_rate_r1 OWNER TO ot_owner;
GRANT SELECT ON rd.fx_rate_r1 TO ot_w_bar;
GRANT SELECT ON rd.fx_rate_r1 TO ot_ro_pages;
