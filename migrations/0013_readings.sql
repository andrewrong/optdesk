-- +goose Up
-- readings.reading r1 (one stored figure value of one subject on one day, insert-only) with its three child tables, the
-- enum reading_unit, the winner view rd.reading_r1 and the grants of the role ot_w_readings (made by internal/store/setup).
-- Departures from schema v3 (docs/design/compute-figures.md Decisions 1): no alias foreign key, no reading_row table.
-- ot_w_readings gets no SELECT on master or ops.dataset_catalog: the compute job reads its inputs through the other
-- stores' roles. The runner role gets nothing. No role has UPDATE, DELETE or TRUNCATE. Schema rd is made by 0006.
CREATE SCHEMA IF NOT EXISTS readings;
CREATE SCHEMA IF NOT EXISTS rd;

CREATE TYPE readings.reading_unit AS ENUM
    ('decimal', 'percent', 'vol_point', 'per_calendar_day', 'count', 'days', 'money', 'ratio', 'text_tag');

CREATE TABLE readings.reading (
    name         text                  NOT NULL,
    version      integer               NOT NULL CHECK (version >= 1),
    subject_kind text                  NOT NULL,
    subject      text                  NOT NULL,
    day          date                  NOT NULL,
    rev          integer               NOT NULL CHECK (rev >= 1),
    value        numeric(38,12),
    unit         readings.reading_unit NOT NULL,
    currency     text                  NOT NULL,
    state        text                  NOT NULL CHECK (state IN ('ok', 'missing_input')),
    reason_code  smallint              CHECK (reason_code BETWEEN 1 AND 5),
    origin       text                  NOT NULL,
    cutoff       text                  NOT NULL,
    job_run_id   uuid                  NOT NULL,
    recorded_at  timestamptz           NOT NULL,
    rows_sha256  char(64)              NOT NULL,
    PRIMARY KEY (name, version, subject_kind, subject, day, rev)
);

CREATE TABLE readings.reading_input (
    name               text     NOT NULL,
    version            integer  NOT NULL,
    subject_kind       text     NOT NULL,
    subject            text     NOT NULL,
    day                date     NOT NULL,
    rev                integer  NOT NULL,
    ord                smallint NOT NULL,
    input_table        text     NOT NULL,
    code_version       text     NOT NULL,
    convention_version text     NOT NULL,
    PRIMARY KEY (name, version, subject_kind, subject, day, rev, ord),
    FOREIGN KEY (name, version, subject_kind, subject, day, rev)
        REFERENCES readings.reading (name, version, subject_kind, subject, day, rev)
);

CREATE TABLE readings.reading_input_key (
    name         text     NOT NULL,
    version      integer  NOT NULL,
    subject_kind text     NOT NULL,
    subject      text     NOT NULL,
    day          date     NOT NULL,
    rev          integer  NOT NULL,
    ord          smallint NOT NULL,
    key_ord      integer  NOT NULL,
    input_key    text     NOT NULL,
    PRIMARY KEY (name, version, subject_kind, subject, day, rev, ord, key_ord),
    FOREIGN KEY (name, version, subject_kind, subject, day, rev, ord)
        REFERENCES readings.reading_input (name, version, subject_kind, subject, day, rev, ord)
);

CREATE TABLE readings.reading_parameter (
    name         text             NOT NULL,
    version      integer          NOT NULL,
    subject_kind text             NOT NULL,
    subject      text             NOT NULL,
    day          date             NOT NULL,
    rev          integer          NOT NULL,
    param        text             NOT NULL,
    value_num    float8           ,
    value_tag    text,
    PRIMARY KEY (name, version, subject_kind, subject, day, rev, param),
    FOREIGN KEY (name, version, subject_kind, subject, day, rev)
        REFERENCES readings.reading (name, version, subject_kind, subject, day, rev)
);

-- The winner per (name, version, subject_kind, subject, day): the highest rev of state ok; a missing_input row shows
-- only when the day has no ok row (then its highest rev).
CREATE VIEW rd.reading_r1 AS
SELECT name, version, subject_kind, subject, day, rev, value, unit, currency, state, reason_code, origin, cutoff,
       job_run_id, recorded_at, rows_sha256
FROM (
    SELECT r.*, row_number() OVER (
        PARTITION BY name, version, subject_kind, subject, day
        ORDER BY (state = 'ok') DESC, rev DESC) AS winner
    FROM readings.reading r
) ranked
WHERE winner = 1;

ALTER SCHEMA readings OWNER TO ot_owner;
ALTER TYPE readings.reading_unit OWNER TO ot_owner;
ALTER TABLE readings.reading OWNER TO ot_owner;
ALTER TABLE readings.reading_input OWNER TO ot_owner;
ALTER TABLE readings.reading_input_key OWNER TO ot_owner;
ALTER TABLE readings.reading_parameter OWNER TO ot_owner;
ALTER VIEW rd.reading_r1 OWNER TO ot_owner;

GRANT USAGE ON SCHEMA readings TO ot_w_readings;
GRANT USAGE ON SCHEMA rd TO ot_w_readings;
GRANT INSERT, SELECT ON readings.reading, readings.reading_input, readings.reading_input_key,
    readings.reading_parameter TO ot_w_readings;
GRANT SELECT ON rd.reading_r1 TO ot_w_readings, ot_ro_pages;

-- +goose Down
REVOKE SELECT ON rd.reading_r1 FROM ot_w_readings, ot_ro_pages;
REVOKE INSERT, SELECT ON readings.reading, readings.reading_input, readings.reading_input_key,
    readings.reading_parameter FROM ot_w_readings;
REVOKE USAGE ON SCHEMA rd FROM ot_w_readings;
REVOKE USAGE ON SCHEMA readings FROM ot_w_readings;
DROP VIEW rd.reading_r1;
DROP TABLE readings.reading_parameter;
DROP TABLE readings.reading_input_key;
DROP TABLE readings.reading_input;
DROP TABLE readings.reading;
DROP TYPE readings.reading_unit;
DROP SCHEMA readings;
