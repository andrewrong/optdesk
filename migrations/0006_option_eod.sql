-- +goose Up
-- obs.option_eod r1 (one option contract on one session day, frozen after the close) and its view rd.option_eod_r1.
-- Append-only: no role but the owner may UPDATE, DELETE or TRUNCATE. The roles ot_owner, ot_runner, ot_w_series_fred,
-- ot_ro_pages and ot_w_option_eod are made by internal/store/setup. Needs the timescaledb extension.
CREATE SCHEMA IF NOT EXISTS obs;
CREATE SCHEMA IF NOT EXISTS rd;

CREATE TABLE obs.option_eod (
    session_day            date        NOT NULL,
    capture_id             uuid        NOT NULL,
    instrument_id          bigint      NOT NULL,
    underlying_id          bigint      NOT NULL,
    generation             integer     NOT NULL,
    market_data_type       smallint    NOT NULL,
    received_at            timestamptz NOT NULL,
    event_ts               timestamptz,
    status                 smallint    NOT NULL,
    error_code             text,
    currency               text        NOT NULL,
    snapshot_kind          text        NOT NULL,
    terms_version_id       bigint,
    bid                    numeric,
    ask                    numeric,
    bid_size               numeric,
    ask_size               numeric,
    last                   numeric,
    volume                 numeric,
    open_interest          numeric,
    oi_asof_date           date,
    underlying_price       numeric,
    underlying_ccy         text        NOT NULL,
    underlying_event_ts    timestamptz,
    iv_bid_decimal         numeric,
    iv_mid_decimal         numeric,
    iv_ask_decimal         numeric,
    iv_decimal             numeric,
    delta                  numeric,
    gamma                  numeric,
    theta_per_calendar_day numeric,
    vega_per_vol_point     numeric,
    PRIMARY KEY (session_day, capture_id, instrument_id),
    CONSTRAINT option_eod_status_ck CHECK (status BETWEEN 1 AND 5),
    CONSTRAINT option_eod_currency_ck CHECK (currency ~ '^[A-Z]{3}$'),
    CONSTRAINT option_eod_underlying_ccy_ck CHECK (underlying_ccy ~ '^[A-Z]{3}$'),
    -- Prices, sizes, volume, open interest and underlying price: NULL, or at least 0, not NaN, not an infinity.
    -- IV, gamma and vega: NULL, or not NaN, not an infinity and not -1.
    -- delta and theta: NULL, or not NaN and not an infinity (-1 is a legal value; the writer turns it into NULL).
    CONSTRAINT option_eod_bid_ck CHECK (bid IS NULL OR (bid >= 0 AND bid <> 'NaN'::numeric AND bid <> 'Infinity'::numeric AND bid <> '-Infinity'::numeric)),
    CONSTRAINT option_eod_ask_ck CHECK (ask IS NULL OR (ask >= 0 AND ask <> 'NaN'::numeric AND ask <> 'Infinity'::numeric AND ask <> '-Infinity'::numeric)),
    CONSTRAINT option_eod_bid_size_ck CHECK (bid_size IS NULL OR (bid_size >= 0 AND bid_size <> 'NaN'::numeric AND bid_size <> 'Infinity'::numeric AND bid_size <> '-Infinity'::numeric)),
    CONSTRAINT option_eod_ask_size_ck CHECK (ask_size IS NULL OR (ask_size >= 0 AND ask_size <> 'NaN'::numeric AND ask_size <> 'Infinity'::numeric AND ask_size <> '-Infinity'::numeric)),
    CONSTRAINT option_eod_last_ck CHECK (last IS NULL OR (last >= 0 AND last <> 'NaN'::numeric AND last <> 'Infinity'::numeric AND last <> '-Infinity'::numeric)),
    CONSTRAINT option_eod_volume_ck CHECK (volume IS NULL OR (volume >= 0 AND volume <> 'NaN'::numeric AND volume <> 'Infinity'::numeric AND volume <> '-Infinity'::numeric)),
    CONSTRAINT option_eod_open_interest_ck CHECK (open_interest IS NULL OR (open_interest >= 0 AND open_interest <> 'NaN'::numeric AND open_interest <> 'Infinity'::numeric AND open_interest <> '-Infinity'::numeric)),
    CONSTRAINT option_eod_underlying_price_ck CHECK (underlying_price IS NULL OR (underlying_price >= 0 AND underlying_price <> 'NaN'::numeric AND underlying_price <> 'Infinity'::numeric AND underlying_price <> '-Infinity'::numeric)),
    CONSTRAINT option_eod_iv_bid_decimal_ck CHECK (iv_bid_decimal IS NULL OR (iv_bid_decimal <> 'NaN'::numeric AND iv_bid_decimal <> 'Infinity'::numeric AND iv_bid_decimal <> '-Infinity'::numeric AND iv_bid_decimal <> -1)),
    CONSTRAINT option_eod_iv_mid_decimal_ck CHECK (iv_mid_decimal IS NULL OR (iv_mid_decimal <> 'NaN'::numeric AND iv_mid_decimal <> 'Infinity'::numeric AND iv_mid_decimal <> '-Infinity'::numeric AND iv_mid_decimal <> -1)),
    CONSTRAINT option_eod_iv_ask_decimal_ck CHECK (iv_ask_decimal IS NULL OR (iv_ask_decimal <> 'NaN'::numeric AND iv_ask_decimal <> 'Infinity'::numeric AND iv_ask_decimal <> '-Infinity'::numeric AND iv_ask_decimal <> -1)),
    CONSTRAINT option_eod_iv_decimal_ck CHECK (iv_decimal IS NULL OR (iv_decimal <> 'NaN'::numeric AND iv_decimal <> 'Infinity'::numeric AND iv_decimal <> '-Infinity'::numeric AND iv_decimal <> -1)),
    CONSTRAINT option_eod_gamma_ck CHECK (gamma IS NULL OR (gamma <> 'NaN'::numeric AND gamma <> 'Infinity'::numeric AND gamma <> '-Infinity'::numeric AND gamma <> -1)),
    CONSTRAINT option_eod_vega_per_vol_point_ck CHECK (vega_per_vol_point IS NULL OR (vega_per_vol_point <> 'NaN'::numeric AND vega_per_vol_point <> 'Infinity'::numeric AND vega_per_vol_point <> '-Infinity'::numeric AND vega_per_vol_point <> -1)),
    CONSTRAINT option_eod_delta_ck CHECK (delta IS NULL OR (delta <> 'NaN'::numeric AND delta <> 'Infinity'::numeric AND delta <> '-Infinity'::numeric)),
    CONSTRAINT option_eod_theta_per_calendar_day_ck CHECK (theta_per_calendar_day IS NULL OR (theta_per_calendar_day <> 'NaN'::numeric AND theta_per_calendar_day <> 'Infinity'::numeric AND theta_per_calendar_day <> '-Infinity'::numeric))
);

SELECT create_hypertable('obs.option_eod', 'session_day', chunk_time_interval => INTERVAL '1 month');

ALTER TABLE obs.option_eod SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'underlying_id',
    timescaledb.compress_orderby = 'session_day DESC, capture_id, instrument_id'
);
SELECT add_compression_policy('obs.option_eod', INTERVAL '7 days');

CREATE INDEX option_eod_instrument_day ON obs.option_eod (instrument_id, session_day DESC);

-- Every row passes through, every column named.
CREATE VIEW rd.option_eod_r1 AS
SELECT session_day, capture_id, instrument_id, underlying_id, generation, market_data_type, received_at, event_ts,
       status, error_code, currency, snapshot_kind, terms_version_id, bid, ask, bid_size, ask_size, last, volume,
       open_interest, oi_asof_date, underlying_price, underlying_ccy, underlying_event_ts, iv_bid_decimal,
       iv_mid_decimal, iv_ask_decimal, iv_decimal, delta, gamma, theta_per_calendar_day, vega_per_vol_point
FROM obs.option_eod;

ALTER TABLE obs.option_eod OWNER TO ot_owner;
ALTER VIEW rd.option_eod_r1 OWNER TO ot_owner;

-- The writer appends rows and may only refer to a run and read it; it has no SELECT on the table.
GRANT USAGE ON SCHEMA obs, ops TO ot_w_option_eod;
GRANT INSERT ON obs.option_eod TO ot_w_option_eod;
GRANT SELECT, REFERENCES ON ops.capture_run TO ot_w_option_eod;

GRANT USAGE ON SCHEMA obs, rd TO ot_runner;
GRANT SELECT ON obs.option_eod TO ot_runner;
GRANT SELECT ON rd.option_eod_r1 TO ot_runner;

-- Read-only: the pages read the view and nothing else.
GRANT USAGE ON SCHEMA rd TO ot_ro_pages;
GRANT SELECT ON rd.option_eod_r1 TO ot_ro_pages;

-- +goose Down
DROP VIEW rd.option_eod_r1;
DROP TABLE obs.option_eod;
