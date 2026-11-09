-- +goose Up
-- Schema account: the owner's captures, positions, values and executions, insert-only (docs/design/account-data.md
-- Decisions 5 and 6). The role ot_w_account (made by internal/store/setup) gets INSERT and SELECT on the tables and
-- SELECT on the four views, and nothing else. The pages read the rd views only: ot_ro_pages has no grant on a table.
-- Money and quantity are numeric, and every row carries the alias of the gateway entry, never an account id.
CREATE SCHEMA IF NOT EXISTS account;

CREATE TABLE account.capture (
    capture_id  text        PRIMARY KEY,
    alias       text        NOT NULL CHECK (alias <> ''),
    captured_at timestamptz NOT NULL,
    session_day date        NOT NULL,
    status      text        NOT NULL
);

CREATE TABLE account.position (
    capture_id  text        NOT NULL REFERENCES account.capture (capture_id),
    alias       text        NOT NULL CHECK (alias <> ''),
    captured_at timestamptz NOT NULL,
    session_day date        NOT NULL,
    symbol      text        NOT NULL CHECK (symbol <> ''),
    sec_type    text        NOT NULL CHECK (sec_type IN ('stock', 'index', 'option')),
    exchange    text        NOT NULL,
    currency    text        NOT NULL,
    expiry      text        NOT NULL,
    strike      text        NOT NULL,
    multiplier  text        NOT NULL,
    option_right text       NOT NULL CHECK (option_right IN ('', 'put', 'call')),
    con_id      bigint      NOT NULL,
    quantity    numeric     NOT NULL,
    avg_cost    numeric     NOT NULL,
    PRIMARY KEY (capture_id, con_id)
);

CREATE TABLE account.value (
    capture_id  text        NOT NULL REFERENCES account.capture (capture_id),
    alias       text        NOT NULL CHECK (alias <> ''),
    captured_at timestamptz NOT NULL,
    session_day date        NOT NULL,
    tag         text        NOT NULL CHECK (tag <> ''),
    value       numeric     NOT NULL,
    currency    text        NOT NULL,
    PRIMARY KEY (capture_id, tag, currency)
);

CREATE TABLE account.execution (
    alias               text        NOT NULL CHECK (alias <> ''),
    exec_id             text        NOT NULL CHECK (exec_id <> ''),
    perm_id             bigint      NOT NULL,
    executed_at         timestamptz NOT NULL,
    session_day         date        NOT NULL,
    symbol              text        NOT NULL CHECK (symbol <> ''),
    sec_type            text        NOT NULL CHECK (sec_type IN ('stock', 'index', 'option')),
    expiry              text        NOT NULL,
    strike              text        NOT NULL,
    option_right        text        NOT NULL CHECK (option_right IN ('', 'put', 'call')),
    multiplier          text        NOT NULL,
    currency            text        NOT NULL,
    side                text        NOT NULL CHECK (side IN ('BOT', 'SLD')),
    quantity            numeric     NOT NULL,
    price               numeric     NOT NULL,
    commission          numeric     NOT NULL,
    commission_currency text        NOT NULL,
    PRIMARY KEY (alias, exec_id)
);

CREATE VIEW rd.account_capture_r1 AS
SELECT capture_id, alias, captured_at, session_day, status
FROM account.capture;

CREATE VIEW rd.account_position_r1 AS
SELECT capture_id, alias, captured_at, session_day, symbol, sec_type, exchange, currency, expiry, strike,
       multiplier, option_right AS "right", con_id, quantity, avg_cost
FROM account.position;

CREATE VIEW rd.account_value_r1 AS
SELECT capture_id, alias, captured_at, session_day, tag, value, currency
FROM account.value;

CREATE VIEW rd.account_execution_r1 AS
SELECT alias, exec_id, perm_id, executed_at, session_day, symbol, sec_type, expiry, strike,
       option_right AS "right", multiplier, currency, side, quantity, price, commission, commission_currency
FROM account.execution;

ALTER SCHEMA account OWNER TO ot_owner;
ALTER TABLE account.capture OWNER TO ot_owner;
ALTER TABLE account.position OWNER TO ot_owner;
ALTER TABLE account.value OWNER TO ot_owner;
ALTER TABLE account.execution OWNER TO ot_owner;
ALTER VIEW rd.account_capture_r1 OWNER TO ot_owner;
ALTER VIEW rd.account_position_r1 OWNER TO ot_owner;
ALTER VIEW rd.account_value_r1 OWNER TO ot_owner;
ALTER VIEW rd.account_execution_r1 OWNER TO ot_owner;

GRANT USAGE ON SCHEMA account TO ot_w_account;
GRANT INSERT, SELECT ON account.capture, account.position, account.value, account.execution TO ot_w_account;

-- The Reader of internal/store/account reads the views, so ot_w_account may read them.
GRANT USAGE ON SCHEMA rd TO ot_w_account;
GRANT SELECT ON rd.account_capture_r1 TO ot_w_account;
GRANT SELECT ON rd.account_position_r1 TO ot_w_account;
GRANT SELECT ON rd.account_value_r1 TO ot_w_account;
GRANT SELECT ON rd.account_execution_r1 TO ot_w_account;

-- The pages read the views and no table.
GRANT SELECT ON rd.account_capture_r1 TO ot_ro_pages;
GRANT SELECT ON rd.account_position_r1 TO ot_ro_pages;
GRANT SELECT ON rd.account_value_r1 TO ot_ro_pages;
GRANT SELECT ON rd.account_execution_r1 TO ot_ro_pages;

-- +goose Down
REVOKE SELECT ON rd.account_capture_r1, rd.account_position_r1, rd.account_value_r1, rd.account_execution_r1 FROM ot_ro_pages;
REVOKE SELECT ON rd.account_capture_r1, rd.account_position_r1, rd.account_value_r1, rd.account_execution_r1 FROM ot_w_account;
REVOKE USAGE ON SCHEMA rd FROM ot_w_account;
DROP VIEW rd.account_execution_r1;
DROP VIEW rd.account_value_r1;
DROP VIEW rd.account_position_r1;
DROP VIEW rd.account_capture_r1;
REVOKE INSERT, SELECT ON account.capture, account.position, account.value, account.execution FROM ot_w_account;
REVOKE USAGE ON SCHEMA account FROM ot_w_account;
DROP TABLE account.execution;
DROP TABLE account.value;
DROP TABLE account.position;
DROP TABLE account.capture;
DROP SCHEMA account;
