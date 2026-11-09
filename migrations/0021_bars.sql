-- +goose Up
-- obs.price_bar r1 and obs.fx_rate r1: daily closes and the daily FX rate, insert only
-- (docs/design/market-bars.md Decisions 4 and 8). A restated day is a new row under the next adjust_epoch; the view
-- rd.price_bar_r1 holds the rows of the highest epoch of each instrument and basis. Prices are decimal text.
-- The role ot_w_bar (made by internal/store/setup) gets INSERT on the tables and SELECT on the views; ot_ro_pages
-- gets SELECT on the views. No role but the owner may UPDATE, DELETE or TRUNCATE.
CREATE SCHEMA IF NOT EXISTS obs;
CREATE SCHEMA IF NOT EXISTS rd;

CREATE TABLE obs.price_bar (
    instrument_id bigint      NOT NULL,
    session_day   date        NOT NULL,
    basis         text        NOT NULL,
    adjust_epoch  integer     NOT NULL,
    open          text        NOT NULL,
    high          text        NOT NULL,
    low           text        NOT NULL,
    close         text        NOT NULL,
    volume        text,
    currency      text        NOT NULL,
    source        text        NOT NULL,
    received_at   timestamptz NOT NULL,
    PRIMARY KEY (instrument_id, basis, adjust_epoch, session_day),
    CONSTRAINT price_bar_basis_ck CHECK (basis IN ('trades', 'adjusted_last')),
    CONSTRAINT price_bar_epoch_ck CHECK (adjust_epoch >= 1),
    CONSTRAINT price_bar_source_ck CHECK (source = 'ibkr'),
    CONSTRAINT price_bar_open_ck CHECK (open ~ '^[0-9]+(\.[0-9]+)?$' AND open::numeric > 0),
    CONSTRAINT price_bar_high_ck CHECK (high ~ '^[0-9]+(\.[0-9]+)?$' AND high::numeric > 0),
    CONSTRAINT price_bar_low_ck CHECK (low ~ '^[0-9]+(\.[0-9]+)?$' AND low::numeric > 0),
    CONSTRAINT price_bar_close_ck CHECK (close ~ '^[0-9]+(\.[0-9]+)?$' AND close::numeric > 0)
);

CREATE TABLE obs.fx_rate (
    pair        text        NOT NULL,
    rate_day    date        NOT NULL,
    rate        text        NOT NULL,
    source      text        NOT NULL,
    received_at timestamptz NOT NULL,
    PRIMARY KEY (pair, rate_day),
    CONSTRAINT fx_rate_pair_ck CHECK (pair ~ '^[A-Z]{6}$'),
    CONSTRAINT fx_rate_source_ck CHECK (source = 'ibkr_midpoint'),
    CONSTRAINT fx_rate_rate_ck CHECK (rate ~ '^[0-9]+(\.[0-9]+)?$' AND rate::numeric > 0)
);

ALTER TABLE obs.price_bar OWNER TO ot_owner;
ALTER TABLE obs.fx_rate OWNER TO ot_owner;

CREATE VIEW rd.price_bar_r1 AS
SELECT b.instrument_id, b.session_day, b.basis, b.adjust_epoch, b.open, b.high, b.low, b.close, b.volume,
       b.currency, b.source, b.received_at
FROM obs.price_bar b
WHERE b.adjust_epoch = (SELECT max(m.adjust_epoch) FROM obs.price_bar m
                        WHERE m.instrument_id = b.instrument_id AND m.basis = b.basis);

CREATE VIEW rd.fx_rate_r1 AS
SELECT pair, rate_day, rate, source, received_at
FROM obs.fx_rate;

ALTER VIEW rd.price_bar_r1 OWNER TO ot_owner;
ALTER VIEW rd.fx_rate_r1 OWNER TO ot_owner;

GRANT USAGE ON SCHEMA obs, rd TO ot_w_bar;
GRANT USAGE ON SCHEMA rd TO ot_ro_pages;
GRANT INSERT ON obs.price_bar TO ot_w_bar;
GRANT INSERT ON obs.fx_rate TO ot_w_bar;
GRANT SELECT ON rd.price_bar_r1 TO ot_w_bar;
GRANT SELECT ON rd.fx_rate_r1 TO ot_w_bar;
GRANT SELECT ON rd.price_bar_r1 TO ot_ro_pages;
GRANT SELECT ON rd.fx_rate_r1 TO ot_ro_pages;

-- +goose Down
REVOKE SELECT ON rd.price_bar_r1 FROM ot_ro_pages;
REVOKE SELECT ON rd.fx_rate_r1 FROM ot_ro_pages;
REVOKE SELECT ON rd.price_bar_r1 FROM ot_w_bar;
REVOKE SELECT ON rd.fx_rate_r1 FROM ot_w_bar;
REVOKE INSERT ON obs.price_bar FROM ot_w_bar;
REVOKE INSERT ON obs.fx_rate FROM ot_w_bar;
REVOKE USAGE ON SCHEMA obs, rd FROM ot_w_bar;
DROP VIEW rd.fx_rate_r1;
DROP VIEW rd.price_bar_r1;
DROP TABLE obs.fx_rate;
DROP TABLE obs.price_bar;
