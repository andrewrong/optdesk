-- +goose Up
-- obs.series_point r1 and its read view rd.series_point_r1.
-- The roles ot_owner, ot_runner, ot_w_series_fred and ot_ro_pages are made by internal/store/setup.
CREATE SCHEMA IF NOT EXISTS obs;
CREATE SCHEMA IF NOT EXISTS rd;

CREATE TYPE obs.series_unit AS ENUM ('percent', 'ratio', 'ccy_amount', 'count');

CREATE TABLE obs.series_definition (
    series_id text            NOT NULL,
    unit      obs.series_unit NOT NULL,
    currency  text            NOT NULL DEFAULT '',
    PRIMARY KEY (series_id),
    CONSTRAINT series_definition_ccy_currency CHECK (unit <> 'ccy_amount' OR currency <> '')
);

CREATE TABLE obs.series_point (
    series_id        text             NOT NULL REFERENCES obs.series_definition (series_id),
    observation_date date             NOT NULL,
    realtime_start   date             NOT NULL,
    realtime_end     date             NOT NULL,
    value            double precision,
    status           text             NOT NULL,
    unit             obs.series_unit  NOT NULL,
    currency         text             NOT NULL DEFAULT '',
    published_at     timestamptz      NOT NULL,
    source           text             NOT NULL,
    source_revision  text             NOT NULL,
    PRIMARY KEY (series_id, observation_date, realtime_start),
    CONSTRAINT series_point_ccy_currency CHECK (unit <> 'ccy_amount' OR currency <> ''),
    -- Mirrors the Writer: the sentinel -1, NaN and the infinities are never stored as a value.
    CONSTRAINT series_point_no_sentinel CHECK (
        value IS NULL OR (value <> -1 AND value <> 'NaN'::float8 AND value <> 'Infinity'::float8 AND value <> '-Infinity'::float8)
    ),
    CONSTRAINT series_point_present_has_value CHECK (value IS NOT NULL OR status <> 'present')
);

CREATE VIEW rd.series_point_r1 AS
SELECT series_id, observation_date, realtime_start, realtime_end, value, status, unit, currency,
       published_at, source, source_revision
FROM obs.series_point;

ALTER SCHEMA obs OWNER TO ot_owner;
ALTER SCHEMA rd OWNER TO ot_owner;
ALTER TYPE obs.series_unit OWNER TO ot_owner;
ALTER TABLE obs.series_definition OWNER TO ot_owner;
ALTER TABLE obs.series_point OWNER TO ot_owner;
ALTER VIEW rd.series_point_r1 OWNER TO ot_owner;

GRANT USAGE ON SCHEMA obs TO ot_runner, ot_w_series_fred;
GRANT USAGE ON TYPE obs.series_unit TO ot_runner, ot_w_series_fred;
GRANT SELECT ON obs.series_definition, obs.series_point TO ot_runner;
GRANT INSERT ON obs.series_definition, obs.series_point TO ot_w_series_fred;

-- Read-only: the pages read the view and nothing else.
GRANT USAGE ON SCHEMA rd TO ot_ro_pages;
GRANT SELECT ON rd.series_point_r1 TO ot_ro_pages;

-- +goose Down
DROP VIEW rd.series_point_r1;
DROP TABLE obs.series_point;
DROP TABLE obs.series_definition;
DROP TYPE obs.series_unit;
