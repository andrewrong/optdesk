-- +goose Up
-- readings.reading is keyed by the data's own day, without rev (docs/design/readings.md Decisions 1 to 5): a rerun that
-- differs replaces the stored reading (UpsertReadings), and rd.reading_r1 becomes a plain select. rev stays a plain
-- column on the parent and the three children, so rd.reading_parameter_r1 (0016) is not touched. Old revisions are
-- not kept: per key only the winner of the old view rule stays (the highest rev of state ok, else the highest rev), so the
-- pages show the same numbers before and after. The deleted rows do not come back in Down.
-- ot_w_readings gets UPDATE on eleven non-key columns of readings.reading and DELETE on the three children; no
-- DELETE on readings.reading, no TRUNCATE.

-- Children of the losing revisions first, then the losing readings.
CREATE TEMP TABLE reading_loser ON COMMIT DROP AS
SELECT name, version, subject_kind, subject, day, rev
FROM (
    SELECT r.name, r.version, r.subject_kind, r.subject, r.day, r.rev, row_number() OVER (
        PARTITION BY r.name, r.version, r.subject_kind, r.subject, r.day
        ORDER BY (r.state = 'ok') DESC, r.rev DESC) AS winner
    FROM readings.reading r
) ranked
WHERE winner > 1;

DELETE FROM readings.reading_input_key c USING reading_loser l
WHERE (c.name, c.version, c.subject_kind, c.subject, c.day, c.rev) = (l.name, l.version, l.subject_kind, l.subject, l.day, l.rev);
DELETE FROM readings.reading_input c USING reading_loser l
WHERE (c.name, c.version, c.subject_kind, c.subject, c.day, c.rev) = (l.name, l.version, l.subject_kind, l.subject, l.day, l.rev);
DELETE FROM readings.reading_parameter c USING reading_loser l
WHERE (c.name, c.version, c.subject_kind, c.subject, c.day, c.rev) = (l.name, l.version, l.subject_kind, l.subject, l.day, l.rev);
DELETE FROM readings.reading r USING reading_loser l
WHERE (r.name, r.version, r.subject_kind, r.subject, r.day, r.rev) = (l.name, l.version, l.subject_kind, l.subject, l.day, l.rev);

-- The generated names of the three foreign keys and the four primary keys are found in pg_constraint.
-- +goose StatementBegin
DO $$
DECLARE c record;
BEGIN
    FOR c IN SELECT conrelid::regclass AS tbl, conname FROM pg_constraint
             WHERE contype = 'f' AND conrelid IN ('readings.reading_input'::regclass,
                 'readings.reading_input_key'::regclass, 'readings.reading_parameter'::regclass) LOOP
        EXECUTE format('ALTER TABLE %s DROP CONSTRAINT %I', c.tbl, c.conname);
    END LOOP;
    FOR c IN SELECT conrelid::regclass AS tbl, conname FROM pg_constraint
             WHERE contype = 'p' AND conrelid IN ('readings.reading'::regclass, 'readings.reading_input'::regclass,
                 'readings.reading_input_key'::regclass, 'readings.reading_parameter'::regclass) LOOP
        EXECUTE format('ALTER TABLE %s DROP CONSTRAINT %I', c.tbl, c.conname);
    END LOOP;
END
$$;
-- +goose StatementEnd

ALTER TABLE readings.reading ADD PRIMARY KEY (name, version, subject_kind, subject, day);
ALTER TABLE readings.reading_input ADD PRIMARY KEY (name, version, subject_kind, subject, day, ord);
ALTER TABLE readings.reading_input_key ADD PRIMARY KEY (name, version, subject_kind, subject, day, ord, key_ord);
ALTER TABLE readings.reading_parameter ADD PRIMARY KEY (name, version, subject_kind, subject, day, param);

ALTER TABLE readings.reading_input ADD FOREIGN KEY (name, version, subject_kind, subject, day)
    REFERENCES readings.reading (name, version, subject_kind, subject, day);
ALTER TABLE readings.reading_input_key ADD FOREIGN KEY (name, version, subject_kind, subject, day, ord)
    REFERENCES readings.reading_input (name, version, subject_kind, subject, day, ord);
ALTER TABLE readings.reading_parameter ADD FOREIGN KEY (name, version, subject_kind, subject, day)
    REFERENCES readings.reading (name, version, subject_kind, subject, day);

-- Same sixteen columns, no window function; the owner and the grants of the view stay.
CREATE OR REPLACE VIEW rd.reading_r1 AS
SELECT name, version, subject_kind, subject, day, rev, value, unit, currency, state, reason_code, origin, cutoff,
       job_run_id, recorded_at, rows_sha256
FROM readings.reading;

GRANT UPDATE (value, unit, currency, state, reason_code, origin, cutoff, rows_sha256, rev, recorded_at, job_run_id)
    ON readings.reading TO ot_w_readings;
GRANT DELETE ON readings.reading_input, readings.reading_input_key, readings.reading_parameter TO ot_w_readings;

-- +goose Down
-- Restores the old keys and the old view; the rows deleted by Up do not come back.
REVOKE DELETE ON readings.reading_input, readings.reading_input_key, readings.reading_parameter FROM ot_w_readings;
REVOKE UPDATE (value, unit, currency, state, reason_code, origin, cutoff, rows_sha256, rev, recorded_at, job_run_id)
    ON readings.reading FROM ot_w_readings;

CREATE OR REPLACE VIEW rd.reading_r1 AS
SELECT name, version, subject_kind, subject, day, rev, value, unit, currency, state, reason_code, origin, cutoff,
       job_run_id, recorded_at, rows_sha256
FROM (
    SELECT r.*, row_number() OVER (
        PARTITION BY name, version, subject_kind, subject, day
        ORDER BY (state = 'ok') DESC, rev DESC) AS winner
    FROM readings.reading r
) ranked
WHERE winner = 1;

-- +goose StatementBegin
DO $$
DECLARE c record;
BEGIN
    FOR c IN SELECT conrelid::regclass AS tbl, conname FROM pg_constraint
             WHERE contype = 'f' AND conrelid IN ('readings.reading_input'::regclass,
                 'readings.reading_input_key'::regclass, 'readings.reading_parameter'::regclass) LOOP
        EXECUTE format('ALTER TABLE %s DROP CONSTRAINT %I', c.tbl, c.conname);
    END LOOP;
    FOR c IN SELECT conrelid::regclass AS tbl, conname FROM pg_constraint
             WHERE contype = 'p' AND conrelid IN ('readings.reading'::regclass, 'readings.reading_input'::regclass,
                 'readings.reading_input_key'::regclass, 'readings.reading_parameter'::regclass) LOOP
        EXECUTE format('ALTER TABLE %s DROP CONSTRAINT %I', c.tbl, c.conname);
    END LOOP;
END
$$;
-- +goose StatementEnd

ALTER TABLE readings.reading ADD PRIMARY KEY (name, version, subject_kind, subject, day, rev);
ALTER TABLE readings.reading_input ADD PRIMARY KEY (name, version, subject_kind, subject, day, rev, ord);
ALTER TABLE readings.reading_input_key ADD PRIMARY KEY (name, version, subject_kind, subject, day, rev, ord, key_ord);
ALTER TABLE readings.reading_parameter ADD PRIMARY KEY (name, version, subject_kind, subject, day, rev, param);

ALTER TABLE readings.reading_input ADD FOREIGN KEY (name, version, subject_kind, subject, day, rev)
    REFERENCES readings.reading (name, version, subject_kind, subject, day, rev);
ALTER TABLE readings.reading_input_key ADD FOREIGN KEY (name, version, subject_kind, subject, day, rev, ord)
    REFERENCES readings.reading_input (name, version, subject_kind, subject, day, rev, ord);
ALTER TABLE readings.reading_parameter ADD FOREIGN KEY (name, version, subject_kind, subject, day, rev)
    REFERENCES readings.reading (name, version, subject_kind, subject, day, rev);
