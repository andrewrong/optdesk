-- +goose Up
-- master.instrument and master.instrument_identifier (the owner's schema v3, section 3.1), and the foreign keys of the
-- observation tables to master.instrument. The role ot_w_master is made by internal/store/setup. Both observation
-- tables are hypertables with compression enabled and hold no live row: compression is switched off while the keys are
-- added and on again with the settings of 0006 and 0007. Timescale functions are schema-qualified because the migrator runs with search_path set to meta.
CREATE TABLE master.instrument (
    instrument_id   bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    instrument_type text        NOT NULL,
    issuer_id       bigint,
    name            text        NOT NULL,
    currency        char(3)     NOT NULL,
    mic             text        NOT NULL,
    calendar_id     text        NOT NULL,
    status          text        NOT NULL,
    price_increment numeric     NOT NULL,
    lot_size        numeric     NOT NULL,
    recorded_at     timestamptz NOT NULL
);

CREATE TABLE master.instrument_identifier (
    instrument_id bigint      NOT NULL REFERENCES master.instrument (instrument_id),
    scheme        text        NOT NULL,
    value         text        NOT NULL,
    valid_from    timestamptz NOT NULL,
    valid_to      timestamptz,
    PRIMARY KEY (scheme, value, valid_from),
    CONSTRAINT instrument_identifier_scheme_ck CHECK (scheme IN ('ibkr_conid', 'osi', 'ticker_exchange', 'jpx_option_code', 'isin'))
);

CREATE INDEX instrument_identifier_instrument_idx ON master.instrument_identifier (instrument_id);

ALTER TABLE master.instrument OWNER TO ot_owner;
ALTER TABLE master.instrument_identifier OWNER TO ot_owner;

SELECT public.remove_compression_policy('obs.option_eod');
SELECT public.remove_compression_policy('obs.quote_snapshot');
ALTER TABLE obs.option_eod SET (timescaledb.compress = false);
ALTER TABLE obs.quote_snapshot SET (timescaledb.compress = false);

ALTER TABLE obs.option_eod
    ADD CONSTRAINT option_eod_instrument_fk FOREIGN KEY (instrument_id) REFERENCES master.instrument (instrument_id),
    ADD CONSTRAINT option_eod_underlying_fk FOREIGN KEY (underlying_id) REFERENCES master.instrument (instrument_id);
ALTER TABLE obs.quote_snapshot
    ADD CONSTRAINT quote_snapshot_instrument_fk FOREIGN KEY (instrument_id) REFERENCES master.instrument (instrument_id);

ALTER TABLE obs.option_eod SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'underlying_id',
    timescaledb.compress_orderby = 'session_day DESC, capture_id, instrument_id'
);
SELECT public.add_compression_policy('obs.option_eod', INTERVAL '7 days', initial_start => now() + INTERVAL '1 day');
ALTER TABLE obs.quote_snapshot SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'instrument_id',
    timescaledb.compress_orderby = 'session_day DESC, slot, capture_id'
);
SELECT public.add_compression_policy('obs.quote_snapshot', INTERVAL '7 days', initial_start => now() + INTERVAL '1 day');

GRANT USAGE ON SCHEMA master TO ot_w_master;
GRANT SELECT, INSERT ON master.instrument TO ot_w_master;
GRANT SELECT, INSERT ON master.instrument_identifier TO ot_w_master;

GRANT SELECT ON master.instrument TO ot_ro_pages;
GRANT SELECT ON master.instrument_identifier TO ot_ro_pages;
GRANT SELECT ON master.instrument TO ot_runner;
GRANT SELECT ON master.instrument_identifier TO ot_runner;

-- +goose Down
ALTER TABLE obs.quote_snapshot DROP CONSTRAINT quote_snapshot_instrument_fk;
ALTER TABLE obs.option_eod DROP CONSTRAINT option_eod_underlying_fk;
ALTER TABLE obs.option_eod DROP CONSTRAINT option_eod_instrument_fk;
DROP TABLE master.instrument_identifier;
DROP TABLE master.instrument;
