-- +goose Up
-- obs.option_open r1 (one planned option contract pulled at the open of a session day and slot) and its view
-- rd.option_open_r1 (docs/design/option-open-pull.md Decision 7). Append-only: no role but the owner may UPDATE, DELETE
-- or TRUNCATE. The role ot_w_option_open is made by internal/store/setup. The pages read the view and no table. A row of
-- the refresh button is never stored, so slot is four digits here (the value refresh lives in the overlay of ot serve).
-- Needs the timescaledb extension, whose functions are schema-qualified because the migrator runs with search_path
-- set to meta. Open interest is not a column (stale intraday; the page joins the last obs.option_eod value).
CREATE SCHEMA IF NOT EXISTS obs;
CREATE SCHEMA IF NOT EXISTS rd;

CREATE TABLE obs.option_open (
    session_day            date        NOT NULL,
    capture_id             uuid        NOT NULL,
    slot                   text        NOT NULL,
    instrument_id          bigint      NOT NULL REFERENCES master.instrument (instrument_id),
    underlying_id          bigint      NOT NULL REFERENCES master.instrument (instrument_id),
    plan_id                uuid        NOT NULL,
    generation             integer     NOT NULL,
    market_data_type       smallint    NOT NULL,
    requested_at           timestamptz NOT NULL,
    sent_at                timestamptz,
    received_at            timestamptz NOT NULL,
    event_ts               timestamptz,
    connection_generation  integer     NOT NULL,
    status                 smallint    NOT NULL,
    error_code             text,
    skew                   boolean     NOT NULL,
    outside_band           boolean     NOT NULL,
    currency               text        NOT NULL,
    bid                    numeric,
    ask                    numeric,
    bid_size               numeric,
    ask_size               numeric,
    last                   numeric,
    volume                 numeric,
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
    PRIMARY KEY (session_day, capture_id, slot, instrument_id),
    CONSTRAINT option_open_slot_ck CHECK (slot ~ '^[0-9]{4}$'),
    CONSTRAINT option_open_status_ck CHECK (status BETWEEN 1 AND 9),
    CONSTRAINT option_open_currency_ck CHECK (currency ~ '^[A-Z]{3}$'),
    CONSTRAINT option_open_underlying_ccy_ck CHECK (underlying_ccy ~ '^[A-Z]{3}$'),
    -- Prices, sizes, volume and underlying price: NULL, or at least 0, not NaN, not an infinity.
    -- IV, gamma and vega: NULL, or finite and not -1. delta and theta: NULL, or finite (the writer turns -1 into NULL).
    CONSTRAINT option_open_bid_ck CHECK (bid IS NULL OR (bid >= 0 AND bid < 'Infinity'::numeric)),
    CONSTRAINT option_open_ask_ck CHECK (ask IS NULL OR (ask >= 0 AND ask < 'Infinity'::numeric)),
    CONSTRAINT option_open_bid_size_ck CHECK (bid_size IS NULL OR (bid_size >= 0 AND bid_size < 'Infinity'::numeric)),
    CONSTRAINT option_open_ask_size_ck CHECK (ask_size IS NULL OR (ask_size >= 0 AND ask_size < 'Infinity'::numeric)),
    CONSTRAINT option_open_last_ck CHECK (last IS NULL OR (last >= 0 AND last < 'Infinity'::numeric)),
    CONSTRAINT option_open_volume_ck CHECK (volume IS NULL OR (volume >= 0 AND volume < 'Infinity'::numeric)),
    CONSTRAINT option_open_underlying_price_ck CHECK (underlying_price IS NULL OR (underlying_price >= 0 AND underlying_price < 'Infinity'::numeric)),
    CONSTRAINT option_open_iv_bid_decimal_ck CHECK (iv_bid_decimal IS NULL OR (iv_bid_decimal > '-Infinity'::numeric AND iv_bid_decimal < 'Infinity'::numeric AND iv_bid_decimal <> -1)),
    CONSTRAINT option_open_iv_mid_decimal_ck CHECK (iv_mid_decimal IS NULL OR (iv_mid_decimal > '-Infinity'::numeric AND iv_mid_decimal < 'Infinity'::numeric AND iv_mid_decimal <> -1)),
    CONSTRAINT option_open_iv_ask_decimal_ck CHECK (iv_ask_decimal IS NULL OR (iv_ask_decimal > '-Infinity'::numeric AND iv_ask_decimal < 'Infinity'::numeric AND iv_ask_decimal <> -1)),
    CONSTRAINT option_open_iv_decimal_ck CHECK (iv_decimal IS NULL OR (iv_decimal > '-Infinity'::numeric AND iv_decimal < 'Infinity'::numeric AND iv_decimal <> -1)),
    CONSTRAINT option_open_gamma_ck CHECK (gamma IS NULL OR (gamma > '-Infinity'::numeric AND gamma < 'Infinity'::numeric AND gamma <> -1)),
    CONSTRAINT option_open_vega_per_vol_point_ck CHECK (vega_per_vol_point IS NULL OR (vega_per_vol_point > '-Infinity'::numeric AND vega_per_vol_point < 'Infinity'::numeric AND vega_per_vol_point <> -1)),
    CONSTRAINT option_open_delta_ck CHECK (delta IS NULL OR (delta > '-Infinity'::numeric AND delta < 'Infinity'::numeric)),
    CONSTRAINT option_open_theta_per_calendar_day_ck CHECK (theta_per_calendar_day IS NULL OR (theta_per_calendar_day > '-Infinity'::numeric AND theta_per_calendar_day < 'Infinity'::numeric))
);

SELECT public.create_hypertable('obs.option_open', 'session_day', chunk_time_interval => INTERVAL '1 month');

CREATE INDEX option_open_instrument_day ON obs.option_open (instrument_id, session_day DESC);

-- Every row passes through, every column named.
CREATE VIEW rd.option_open_r1 AS
SELECT session_day, capture_id, slot, instrument_id, underlying_id, plan_id, generation, market_data_type, requested_at,
       sent_at, received_at, event_ts, connection_generation, status, error_code, skew, outside_band, currency, bid, ask,
       bid_size, ask_size, last, volume, underlying_price, underlying_ccy, underlying_event_ts, iv_bid_decimal,
       iv_mid_decimal, iv_ask_decimal, iv_decimal, delta, gamma, theta_per_calendar_day, vega_per_vol_point
FROM obs.option_open;

ALTER TABLE obs.option_open OWNER TO ot_owner;
ALTER VIEW rd.option_open_r1 OWNER TO ot_owner;

-- The writer appends rows and reads the view; it has no SELECT on the table.
GRANT USAGE ON SCHEMA obs, rd TO ot_w_option_open;
GRANT INSERT ON obs.option_open TO ot_w_option_open;
GRANT SELECT ON rd.option_open_r1 TO ot_w_option_open;

-- Read-only: the pages read the view and nothing else (no grant on the base table).
GRANT USAGE ON SCHEMA rd TO ot_ro_pages;
GRANT SELECT ON rd.option_open_r1 TO ot_ro_pages;

-- +goose Down
REVOKE SELECT ON rd.option_open_r1 FROM ot_ro_pages;
REVOKE SELECT ON rd.option_open_r1 FROM ot_w_option_open;
DROP VIEW rd.option_open_r1;
DROP TABLE obs.option_open;
