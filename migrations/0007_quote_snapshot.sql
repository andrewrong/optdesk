-- +goose Up
-- obs.quote_snapshot r1 (a planned time slot of quotes) and its view rd.quote_snapshot_r1. No IV column.
-- Append-only: no role but the owner may UPDATE, DELETE or TRUNCATE. The roles ot_owner, ot_runner, ot_w_series_fred,
-- ot_ro_pages and ot_w_quote_snapshot are made by internal/store/setup. Needs the timescaledb extension, whose
-- functions are schema-qualified because the migrator runs with search_path set to meta.
CREATE SCHEMA IF NOT EXISTS obs;
CREATE SCHEMA IF NOT EXISTS rd;

CREATE TABLE obs.quote_snapshot (
    session_day      date        NOT NULL,
    capture_id       uuid        NOT NULL,
    slot             text        NOT NULL,
    instrument_id    bigint      NOT NULL,
    received_at      timestamptz NOT NULL,
    generation       integer     NOT NULL,
    market_data_type smallint    NOT NULL,
    event_ts         timestamptz,
    status           smallint    NOT NULL,
    error_code       text,
    currency         text        NOT NULL,
    bid              numeric,
    ask              numeric,
    bid_size         numeric,
    ask_size         numeric,
    last             numeric,
    last_size        numeric,
    volume           numeric,
    PRIMARY KEY (session_day, capture_id, slot, instrument_id),
    CONSTRAINT quote_snapshot_status_ck CHECK (status BETWEEN 1 AND 5),
    CONSTRAINT quote_snapshot_currency_ck CHECK (currency ~ '^[A-Z]{3}$'),
    -- Prices, sizes and volume: NULL, or at least 0, not NaN, not an infinity.
    CONSTRAINT quote_snapshot_bid_ck CHECK (bid IS NULL OR (bid >= 0 AND bid <> 'NaN'::numeric AND bid <> 'Infinity'::numeric AND bid <> '-Infinity'::numeric)),
    CONSTRAINT quote_snapshot_ask_ck CHECK (ask IS NULL OR (ask >= 0 AND ask <> 'NaN'::numeric AND ask <> 'Infinity'::numeric AND ask <> '-Infinity'::numeric)),
    CONSTRAINT quote_snapshot_bid_size_ck CHECK (bid_size IS NULL OR (bid_size >= 0 AND bid_size <> 'NaN'::numeric AND bid_size <> 'Infinity'::numeric AND bid_size <> '-Infinity'::numeric)),
    CONSTRAINT quote_snapshot_ask_size_ck CHECK (ask_size IS NULL OR (ask_size >= 0 AND ask_size <> 'NaN'::numeric AND ask_size <> 'Infinity'::numeric AND ask_size <> '-Infinity'::numeric)),
    CONSTRAINT quote_snapshot_last_ck CHECK (last IS NULL OR (last >= 0 AND last <> 'NaN'::numeric AND last <> 'Infinity'::numeric AND last <> '-Infinity'::numeric)),
    CONSTRAINT quote_snapshot_last_size_ck CHECK (last_size IS NULL OR (last_size >= 0 AND last_size <> 'NaN'::numeric AND last_size <> 'Infinity'::numeric AND last_size <> '-Infinity'::numeric)),
    CONSTRAINT quote_snapshot_volume_ck CHECK (volume IS NULL OR (volume >= 0 AND volume <> 'NaN'::numeric AND volume <> 'Infinity'::numeric AND volume <> '-Infinity'::numeric))
);

SELECT public.create_hypertable('obs.quote_snapshot', 'session_day', chunk_time_interval => INTERVAL '7 days');

ALTER TABLE obs.quote_snapshot SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'instrument_id',
    timescaledb.compress_orderby = 'session_day DESC, slot, capture_id'
);
SELECT public.add_compression_policy('obs.quote_snapshot', INTERVAL '7 days');

-- Every row passes through, every column named.
CREATE VIEW rd.quote_snapshot_r1 AS
SELECT session_day, capture_id, slot, instrument_id, received_at, generation, market_data_type, event_ts, status,
       error_code, currency, bid, ask, bid_size, ask_size, last, last_size, volume
FROM obs.quote_snapshot;

ALTER TABLE obs.quote_snapshot OWNER TO ot_owner;
ALTER VIEW rd.quote_snapshot_r1 OWNER TO ot_owner;

-- The writer appends rows and may only refer to a run and read it; it has no SELECT on the table.
GRANT USAGE ON SCHEMA obs, ops TO ot_w_quote_snapshot;
GRANT INSERT ON obs.quote_snapshot TO ot_w_quote_snapshot;
GRANT SELECT, REFERENCES ON ops.capture_run TO ot_w_quote_snapshot;

GRANT USAGE ON SCHEMA obs, rd TO ot_runner;
GRANT SELECT ON obs.quote_snapshot TO ot_runner;
GRANT SELECT ON rd.quote_snapshot_r1 TO ot_runner;

-- Read-only: the pages read the view and nothing else.
GRANT USAGE ON SCHEMA rd TO ot_ro_pages;
GRANT SELECT ON rd.quote_snapshot_r1 TO ot_ro_pages;

-- +goose Down
DROP VIEW rd.quote_snapshot_r1;
DROP TABLE obs.quote_snapshot;
