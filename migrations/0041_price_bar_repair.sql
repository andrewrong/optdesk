-- +goose Up
-- docs/design/data-repair-bars.md entry 2 (Decisions 1 to 6): obs.price_bar is keyed (instrument_id, basis, session_day)
-- and upserted. The rows the view hid (an adjust_epoch below the maximum of their instrument and basis) are deleted, and
-- adjust_epoch becomes 1 in the rest (it stays as a column with DEFAULT 1, which no code writes). The five numbers become
-- numeric; open, high, low and close may be NULL (a status other than ok); status, updated_at and written_by follow
-- received_at. ot_w_bar keeps table-level INSERT and gets column rights: UPDATE on open, high, low, close, volume,
-- currency, source, status, updated_at, written_by; SELECT on instrument_id, basis, session_day, open, high, low, close,
-- volume, currency, source, status. No DELETE, no TRUNCATE, no UPDATE of the key, received_at or adjust_epoch.
DROP VIEW rd.price_bar_r1;
DELETE FROM obs.price_bar b
WHERE b.adjust_epoch < (SELECT max(m.adjust_epoch) FROM obs.price_bar m
                        WHERE m.instrument_id = b.instrument_id AND m.basis = b.basis);
UPDATE obs.price_bar SET adjust_epoch = 1;
ALTER TABLE obs.price_bar DROP CONSTRAINT price_bar_pkey;
ALTER TABLE obs.price_bar DROP CONSTRAINT price_bar_open_ck;
ALTER TABLE obs.price_bar DROP CONSTRAINT price_bar_high_ck;
ALTER TABLE obs.price_bar DROP CONSTRAINT price_bar_low_ck;
ALTER TABLE obs.price_bar DROP CONSTRAINT price_bar_close_ck;
ALTER TABLE obs.price_bar ALTER COLUMN open TYPE numeric USING open::numeric;
ALTER TABLE obs.price_bar ALTER COLUMN high TYPE numeric USING high::numeric;
ALTER TABLE obs.price_bar ALTER COLUMN low TYPE numeric USING low::numeric;
ALTER TABLE obs.price_bar ALTER COLUMN close TYPE numeric USING close::numeric;
ALTER TABLE obs.price_bar ALTER COLUMN volume TYPE numeric USING volume::numeric;
ALTER TABLE obs.price_bar ALTER COLUMN open DROP NOT NULL;
ALTER TABLE obs.price_bar ALTER COLUMN high DROP NOT NULL;
ALTER TABLE obs.price_bar ALTER COLUMN low DROP NOT NULL;
ALTER TABLE obs.price_bar ALTER COLUMN close DROP NOT NULL;
ALTER TABLE obs.price_bar ADD PRIMARY KEY (instrument_id, basis, session_day);
ALTER TABLE obs.price_bar ALTER COLUMN adjust_epoch SET DEFAULT 1;
ALTER TABLE obs.price_bar ADD COLUMN status text NOT NULL DEFAULT 'ok';
ALTER TABLE obs.price_bar ADD CONSTRAINT price_bar_status_ck CHECK (status IN ('ok', 'no_value', 'void'));
ALTER TABLE obs.price_bar ADD COLUMN updated_at timestamptz;
UPDATE obs.price_bar SET updated_at = received_at;
ALTER TABLE obs.price_bar ALTER COLUMN updated_at SET NOT NULL;
ALTER TABLE obs.price_bar ALTER COLUMN updated_at SET DEFAULT now();
ALTER TABLE obs.price_bar ADD COLUMN written_by text NOT NULL DEFAULT '';
ALTER TABLE obs.price_bar ADD CONSTRAINT price_bar_positive_ck
    CHECK (status = 'no_value' OR (open > 0 AND high > 0 AND low > 0 AND close > 0));

CREATE VIEW rd.price_bar_r1 AS
SELECT instrument_id, session_day, basis, adjust_epoch, open::text AS open, high::text AS high, low::text AS low,
       close::text AS close, volume::text AS volume, currency, source, received_at
FROM obs.price_bar
WHERE status = 'ok';
ALTER VIEW rd.price_bar_r1 OWNER TO ot_owner;
GRANT SELECT ON rd.price_bar_r1 TO ot_w_bar;
GRANT SELECT ON rd.price_bar_r1 TO ot_ro_pages;

GRANT SELECT (instrument_id, basis, session_day, open, high, low, close, volume, currency, source, status) ON obs.price_bar TO ot_w_bar;
GRANT UPDATE (open, high, low, close, volume, currency, source, status, updated_at, written_by) ON obs.price_bar TO ot_w_bar;

-- +goose Down
-- Down does not bring back the rows Up deleted (those of an epoch below the maximum of their instrument and basis).
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM obs.price_bar
               WHERE status <> 'ok' OR open IS NULL OR high IS NULL OR low IS NULL OR close IS NULL
                  OR NOT (open::text ~ '^[0-9]+(\.[0-9]+)?$') OR NOT (high::text ~ '^[0-9]+(\.[0-9]+)?$')
                  OR NOT (low::text ~ '^[0-9]+(\.[0-9]+)?$') OR NOT (close::text ~ '^[0-9]+(\.[0-9]+)?$')) THEN
        RAISE EXCEPTION 'obs.price_bar holds rows the text columns cannot take (status other than ok, a NULL price, or a price of another form)';
    END IF;
END
$$;
-- +goose StatementEnd
REVOKE UPDATE (open, high, low, close, volume, currency, source, status, updated_at, written_by) ON obs.price_bar FROM ot_w_bar;
REVOKE SELECT (instrument_id, basis, session_day, open, high, low, close, volume, currency, source, status) ON obs.price_bar FROM ot_w_bar;
DROP VIEW rd.price_bar_r1;
ALTER TABLE obs.price_bar DROP CONSTRAINT price_bar_positive_ck;
ALTER TABLE obs.price_bar DROP CONSTRAINT price_bar_status_ck;
ALTER TABLE obs.price_bar DROP CONSTRAINT price_bar_pkey;
ALTER TABLE obs.price_bar DROP COLUMN status;
ALTER TABLE obs.price_bar DROP COLUMN updated_at;
ALTER TABLE obs.price_bar DROP COLUMN written_by;
ALTER TABLE obs.price_bar ALTER COLUMN adjust_epoch DROP DEFAULT;
ALTER TABLE obs.price_bar ALTER COLUMN open TYPE text USING open::text;
ALTER TABLE obs.price_bar ALTER COLUMN high TYPE text USING high::text;
ALTER TABLE obs.price_bar ALTER COLUMN low TYPE text USING low::text;
ALTER TABLE obs.price_bar ALTER COLUMN close TYPE text USING close::text;
ALTER TABLE obs.price_bar ALTER COLUMN volume TYPE text USING volume::text;
ALTER TABLE obs.price_bar ALTER COLUMN open SET NOT NULL;
ALTER TABLE obs.price_bar ALTER COLUMN high SET NOT NULL;
ALTER TABLE obs.price_bar ALTER COLUMN low SET NOT NULL;
ALTER TABLE obs.price_bar ALTER COLUMN close SET NOT NULL;
ALTER TABLE obs.price_bar ADD PRIMARY KEY (instrument_id, basis, adjust_epoch, session_day);
ALTER TABLE obs.price_bar ADD CONSTRAINT price_bar_open_ck CHECK (open ~ '^[0-9]+(\.[0-9]+)?$' AND open::numeric > 0);
ALTER TABLE obs.price_bar ADD CONSTRAINT price_bar_high_ck CHECK (high ~ '^[0-9]+(\.[0-9]+)?$' AND high::numeric > 0);
ALTER TABLE obs.price_bar ADD CONSTRAINT price_bar_low_ck CHECK (low ~ '^[0-9]+(\.[0-9]+)?$' AND low::numeric > 0);
ALTER TABLE obs.price_bar ADD CONSTRAINT price_bar_close_ck CHECK (close ~ '^[0-9]+(\.[0-9]+)?$' AND close::numeric > 0);
CREATE VIEW rd.price_bar_r1 AS
SELECT b.instrument_id, b.session_day, b.basis, b.adjust_epoch, b.open, b.high, b.low, b.close, b.volume,
       b.currency, b.source, b.received_at
FROM obs.price_bar b
WHERE b.adjust_epoch = (SELECT max(m.adjust_epoch) FROM obs.price_bar m
                        WHERE m.instrument_id = b.instrument_id AND m.basis = b.basis);
ALTER VIEW rd.price_bar_r1 OWNER TO ot_owner;
GRANT SELECT ON rd.price_bar_r1 TO ot_w_bar;
GRANT SELECT ON rd.price_bar_r1 TO ot_ro_pages;
