-- +goose Up
-- master.option_not_listed r1: a contract IBKR answered none for, remembered until retry_at
-- (docs/design/option-open-pull.md Decisions 5 and 6). The resolve job, through ot_w_option_open, is its only writer;
-- a mark is replaced when the contract is found not listed again and removed by key, by underlying or past its expiry,
-- so the role holds INSERT, UPDATE, DELETE and SELECT on this table. No page role reads it.
CREATE TABLE master.option_not_listed (
    underlying_id bigint      NOT NULL REFERENCES master.instrument (instrument_id),
    expiry        date        NOT NULL,
    strike        text        NOT NULL,
    "right"       char(1)     NOT NULL,
    trading_class text        NOT NULL,
    checked_at    timestamptz NOT NULL,
    retry_at      timestamptz NOT NULL,
    error_code    text,
    error_text    text,
    PRIMARY KEY (underlying_id, expiry, strike, "right", trading_class),
    CONSTRAINT option_not_listed_right_ck CHECK ("right" IN ('P', 'C')),
    CONSTRAINT option_not_listed_strike_ck CHECK (strike ~ '^[0-9]+(\.[0-9]+)?$'),
    CONSTRAINT option_not_listed_trading_class_ck CHECK (trading_class <> ''),
    CONSTRAINT option_not_listed_retry_ck CHECK (retry_at > checked_at)
);

CREATE INDEX option_not_listed_retry ON master.option_not_listed (retry_at);

ALTER TABLE master.option_not_listed OWNER TO ot_owner;

GRANT USAGE ON SCHEMA master TO ot_w_option_open;
GRANT INSERT, UPDATE, DELETE, SELECT ON master.option_not_listed TO ot_w_option_open;

-- +goose Down
REVOKE INSERT, UPDATE, DELETE, SELECT ON master.option_not_listed FROM ot_w_option_open;
DROP TABLE master.option_not_listed;
