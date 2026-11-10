-- +goose Up
-- master.option_contract: the one lookup from an option contract to its stored instrument id
-- (docs/design/option-open-pull.md Interface). master.instrument and master.option_terms hold no underlying id and no
-- trading class, so the resolve jobs put each contract they resolve here and the plan job and the gap pass list them.
-- Unique on the contract identity; the row of a contract that is resolved again takes the new instrument id and
-- resolved_at, so the role ot_w_option_open holds INSERT, UPDATE and SELECT. No page role reads it.
CREATE TABLE master.option_contract (
    instrument_id bigint      NOT NULL REFERENCES master.instrument (instrument_id),
    underlying_id bigint      NOT NULL REFERENCES master.instrument (instrument_id),
    currency      text        NOT NULL,
    exchange      text        NOT NULL,
    trading_class text        NOT NULL,
    multiplier    numeric     NOT NULL,
    expiry        date        NOT NULL,
    strike        text        NOT NULL,
    "right"       char(1)     NOT NULL,
    resolved_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (underlying_id, currency, exchange, trading_class, multiplier, expiry, strike, "right"),
    CONSTRAINT option_contract_right_ck CHECK ("right" IN ('P', 'C')),
    CONSTRAINT option_contract_strike_ck CHECK (strike ~ '^[0-9]+(\.[0-9]+)?$'),
    CONSTRAINT option_contract_currency_ck CHECK (currency ~ '^[A-Z]{3}$'),
    CONSTRAINT option_contract_multiplier_ck CHECK (multiplier > 0 AND multiplier < 'Infinity'::numeric),
    CONSTRAINT option_contract_trading_class_ck CHECK (trading_class <> '' AND exchange <> '')
);

CREATE INDEX option_contract_instrument ON master.option_contract (instrument_id);

ALTER TABLE master.option_contract OWNER TO ot_owner;

GRANT USAGE ON SCHEMA master TO ot_w_option_open;
GRANT INSERT, UPDATE, SELECT ON master.option_contract TO ot_w_option_open;

-- +goose Down
REVOKE INSERT, UPDATE, SELECT ON master.option_contract FROM ot_w_option_open;
DROP TABLE master.option_contract;
