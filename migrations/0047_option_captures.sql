-- +goose Up
SET LOCAL lock_timeout = '5s';
-- docs/design/option-captures.md entry 3 (Decisions 5, 6, 7, 8, 12, 13). It only adds: the plain table obs.option_day
-- (one last good quote per contract and day, kept for ever) and its view rd.option_day_r1, compression of obs.option_open
-- and the grants the upserts need. It touches no row and no setting of obs.option_eod and changes no key. lock_timeout
-- comes first so a long reader makes the migration fail and roll back instead of queueing every later reader behind it.
-- Needs the timescaledb extension, whose functions are schema-qualified because the migrator runs with search_path set
-- to meta.

-- 1. Compression of obs.option_open (never compressed; the orderby holds the whole key).
ALTER TABLE obs.option_open SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'underlying_id',
    timescaledb.compress_orderby = 'session_day DESC, capture_id, slot, instrument_id'
);
SELECT public.add_compression_policy('obs.option_open', INTERVAL '7 days');

-- 2. obs.option_day: the last good quote of one contract on one session day.
CREATE TABLE obs.option_day (
    instrument_id          bigint      NOT NULL REFERENCES master.instrument (instrument_id),
    session_day            date        NOT NULL,
    underlying_id          bigint      NOT NULL REFERENCES master.instrument (instrument_id),
    source                 text        NOT NULL,
    received_at            timestamptz NOT NULL,
    status                 smallint    NOT NULL,
    currency               text        NOT NULL,
    bid                    numeric,
    ask                    numeric,
    bid_size               numeric,
    ask_size               numeric,
    last                   numeric,
    volume                 numeric,
    underlying_price       numeric,
    underlying_ccy         text        NOT NULL,
    iv_bid_decimal         numeric,
    iv_mid_decimal         numeric,
    iv_ask_decimal         numeric,
    iv_decimal             numeric,
    delta                  numeric,
    gamma                  numeric,
    theta_per_calendar_day numeric,
    vega_per_vol_point     numeric,
    open_interest          numeric,
    oi_asof_date           date,
    updated_at             timestamptz NOT NULL DEFAULT now(),
    written_by             text        NOT NULL DEFAULT '',
    PRIMARY KEY (instrument_id, session_day),
    CONSTRAINT option_day_source_ck CHECK (source IN ('option_eod', 'option_open')),
    CONSTRAINT option_day_status_ck CHECK (status BETWEEN 1 AND 9),
    CONSTRAINT option_day_currency_ck CHECK (currency ~ '^[A-Z]{3}$'),
    CONSTRAINT option_day_underlying_ccy_ck CHECK (underlying_ccy ~ '^[A-Z]{3}$'),
    CONSTRAINT option_day_bid_ck CHECK (bid IS NULL OR (bid >= 0 AND bid < 'Infinity'::numeric)),
    CONSTRAINT option_day_ask_ck CHECK (ask IS NULL OR (ask >= 0 AND ask < 'Infinity'::numeric)),
    CONSTRAINT option_day_bid_size_ck CHECK (bid_size IS NULL OR (bid_size >= 0 AND bid_size < 'Infinity'::numeric)),
    CONSTRAINT option_day_ask_size_ck CHECK (ask_size IS NULL OR (ask_size >= 0 AND ask_size < 'Infinity'::numeric)),
    CONSTRAINT option_day_last_ck CHECK (last IS NULL OR (last >= 0 AND last < 'Infinity'::numeric)),
    CONSTRAINT option_day_volume_ck CHECK (volume IS NULL OR (volume >= 0 AND volume < 'Infinity'::numeric)),
    CONSTRAINT option_day_underlying_price_ck CHECK (underlying_price IS NULL OR (underlying_price >= 0 AND underlying_price < 'Infinity'::numeric)),
    CONSTRAINT option_day_open_interest_ck CHECK (open_interest IS NULL OR (open_interest >= 0 AND open_interest < 'Infinity'::numeric)),
    CONSTRAINT option_day_iv_bid_decimal_ck CHECK (iv_bid_decimal IS NULL OR (iv_bid_decimal > '-Infinity'::numeric AND iv_bid_decimal < 'Infinity'::numeric AND iv_bid_decimal <> -1)),
    CONSTRAINT option_day_iv_mid_decimal_ck CHECK (iv_mid_decimal IS NULL OR (iv_mid_decimal > '-Infinity'::numeric AND iv_mid_decimal < 'Infinity'::numeric AND iv_mid_decimal <> -1)),
    CONSTRAINT option_day_iv_ask_decimal_ck CHECK (iv_ask_decimal IS NULL OR (iv_ask_decimal > '-Infinity'::numeric AND iv_ask_decimal < 'Infinity'::numeric AND iv_ask_decimal <> -1)),
    CONSTRAINT option_day_iv_decimal_ck CHECK (iv_decimal IS NULL OR (iv_decimal > '-Infinity'::numeric AND iv_decimal < 'Infinity'::numeric AND iv_decimal <> -1)),
    CONSTRAINT option_day_gamma_ck CHECK (gamma IS NULL OR (gamma > '-Infinity'::numeric AND gamma < 'Infinity'::numeric AND gamma <> -1)),
    CONSTRAINT option_day_vega_per_vol_point_ck CHECK (vega_per_vol_point IS NULL OR (vega_per_vol_point > '-Infinity'::numeric AND vega_per_vol_point < 'Infinity'::numeric AND vega_per_vol_point <> -1)),
    CONSTRAINT option_day_delta_ck CHECK (delta IS NULL OR (delta > '-Infinity'::numeric AND delta < 'Infinity'::numeric)),
    CONSTRAINT option_day_theta_per_calendar_day_ck CHECK (theta_per_calendar_day IS NULL OR (theta_per_calendar_day > '-Infinity'::numeric AND theta_per_calendar_day < 'Infinity'::numeric))
);

CREATE INDEX option_day_underlying_day ON obs.option_day (underlying_id, session_day DESC);

-- Every row passes through, every column named.
CREATE VIEW rd.option_day_r1 AS
SELECT instrument_id, session_day, underlying_id, source, received_at, status, currency, bid, ask, bid_size, ask_size,
       last, volume, underlying_price, underlying_ccy, iv_bid_decimal, iv_mid_decimal, iv_ask_decimal, iv_decimal,
       delta, gamma, theta_per_calendar_day, vega_per_vol_point, open_interest, oi_asof_date, updated_at, written_by
FROM obs.option_day;

ALTER TABLE obs.option_day OWNER TO ot_owner;
ALTER VIEW rd.option_day_r1 OWNER TO ot_owner;

-- 3. Grants (Decision 8): INSERT stays table-wide. On the two compressed hypertables obs.option_eod and obs.option_open
-- the writers get table-level SELECT and UPDATE, because TimescaleDB 2.19 copies a column-level grant to the compressed
-- twin, which has no user columns there (SQLSTATE 42703); the guarantee that no key column is updated is held by a test
-- of the upsert statement. obs.option_day is a plain table: UPDATE only on non-key columns, SELECT on the key and the
-- columns the upsert compares or reads through EXCLUDED. Nothing removes rows.
GRANT INSERT ON obs.option_day TO ot_w_option_eod, ot_w_option_open;
GRANT SELECT, UPDATE ON obs.option_eod TO ot_w_option_eod;
GRANT SELECT, UPDATE ON obs.option_open TO ot_w_option_open;
GRANT UPDATE (
    underlying_id, source, received_at, status, currency, bid, ask, bid_size, ask_size, last, volume,
      underlying_price, underlying_ccy, iv_bid_decimal, iv_mid_decimal, iv_ask_decimal, iv_decimal, delta, gamma,
      theta_per_calendar_day, vega_per_vol_point, open_interest, oi_asof_date, updated_at, written_by
) ON obs.option_day TO ot_w_option_eod;
GRANT SELECT (
    instrument_id, session_day, received_at, underlying_id, source, status, currency, bid, ask, bid_size, ask_size,
      last, volume, underlying_price, underlying_ccy, iv_bid_decimal, iv_mid_decimal, iv_ask_decimal, iv_decimal,
      delta, gamma, theta_per_calendar_day, vega_per_vol_point, open_interest, oi_asof_date
) ON obs.option_day TO ot_w_option_eod;
GRANT UPDATE (
    underlying_id, source, received_at, status, currency, bid, ask, bid_size, ask_size, last, volume,
      underlying_price, underlying_ccy, iv_bid_decimal, iv_mid_decimal, iv_ask_decimal, iv_decimal, delta, gamma,
      theta_per_calendar_day, vega_per_vol_point, open_interest, oi_asof_date, updated_at, written_by
) ON obs.option_day TO ot_w_option_open;
GRANT SELECT (
    instrument_id, session_day, received_at, underlying_id, source, status, currency, bid, ask, bid_size, ask_size,
      last, volume, underlying_price, underlying_ccy, iv_bid_decimal, iv_mid_decimal, iv_ask_decimal, iv_decimal,
      delta, gamma, theta_per_calendar_day, vega_per_vol_point, open_interest, oi_asof_date
) ON obs.option_day TO ot_w_option_open;

GRANT USAGE ON SCHEMA rd TO ot_runner, ot_ro_pages;
GRANT SELECT ON rd.option_day_r1 TO ot_runner, ot_ro_pages;

-- +goose Down
REVOKE SELECT ON rd.option_day_r1 FROM ot_runner, ot_ro_pages;
REVOKE SELECT, UPDATE ON obs.option_open FROM ot_w_option_open;
REVOKE SELECT, UPDATE ON obs.option_eod FROM ot_w_option_eod;
DROP VIEW rd.option_day_r1;
DROP TABLE obs.option_day;

SELECT public.remove_compression_policy('obs.option_open', if_exists => true);
SELECT public.decompress_chunk(c, if_compressed => true) FROM public.show_chunks('obs.option_open') c;
ALTER TABLE obs.option_open SET (timescaledb.compress = false);
