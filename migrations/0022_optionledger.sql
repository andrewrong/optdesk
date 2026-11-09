-- +goose Up
-- optionledger.flex_fetch and optionledger.flex_trade_row: the Flex trade rows of the owner's IBKR accounts,
-- insert-only (docs/design/option-ledger.md Decisions 5 and 6). Alias is the only identity: no broker number
-- is a column and none is inside the attributes text (the job and the store both remove it).
-- The role ot_w_optionledger (made by internal/store/setup) is the only role with a grant on the base tables, with
-- INSERT and SELECT and nothing else; it also reads the views. ot_ro_pages reads the two views and nothing else.
-- No other role, and no notify or sender role, has a grant on the schema.
CREATE SCHEMA IF NOT EXISTS optionledger;
CREATE SCHEMA IF NOT EXISTS rd;

CREATE TABLE optionledger.flex_fetch (
    login              text        NOT NULL,
    query_id           text        NOT NULL,
    first_day          date,
    last_day           date,
    started_at         timestamptz NOT NULL,
    finished_at        timestamptz NOT NULL,
    status             text        NOT NULL,
    error_kind         text        NOT NULL DEFAULT '',
    error_code         text        NOT NULL DEFAULT '',
    raw_sha256         text        NOT NULL DEFAULT '',
    parser_version     text        NOT NULL DEFAULT '',
    statements         integer     NOT NULL DEFAULT 0,
    unmapped_statements integer NOT NULL DEFAULT 0,
    rows_parsed        integer     NOT NULL DEFAULT 0,
    rows_new           integer     NOT NULL DEFAULT 0,
    rows_revised       integer     NOT NULL DEFAULT 0,
    rows_unchanged     integer     NOT NULL DEFAULT 0,
    PRIMARY KEY (login, query_id, started_at)
);

CREATE TABLE optionledger.flex_trade_row (
    alias             text        NOT NULL,
    level_of_detail   text        NOT NULL,
    transaction_id    text        NOT NULL,
    revision          integer     NOT NULL,
    attributes        text        NOT NULL,
    attributes_sha256 text        NOT NULL,
    source            text        NOT NULL,
    recorded_at       timestamptz NOT NULL,
    PRIMARY KEY (alias, level_of_detail, transaction_id, revision),
    CONSTRAINT flex_trade_row_revision_ck CHECK (revision >= 1),
    CONSTRAINT flex_trade_row_source_ck CHECK (source IN ('flex', 'ot_import'))
);

ALTER SCHEMA optionledger OWNER TO ot_owner;
ALTER TABLE optionledger.flex_fetch OWNER TO ot_owner;
ALTER TABLE optionledger.flex_trade_row OWNER TO ot_owner;

CREATE VIEW rd.flex_fetch_r1 AS
SELECT login, query_id, first_day, last_day, started_at, finished_at, status, error_kind, error_code, raw_sha256,
       parser_version, statements, unmapped_statements, rows_parsed, rows_new, rows_revised, rows_unchanged
FROM optionledger.flex_fetch;

CREATE VIEW rd.flex_trade_row_r1 AS
SELECT alias, level_of_detail, transaction_id, revision, attributes, attributes_sha256, source, recorded_at
FROM optionledger.flex_trade_row;

ALTER VIEW rd.flex_fetch_r1 OWNER TO ot_owner;
ALTER VIEW rd.flex_trade_row_r1 OWNER TO ot_owner;

GRANT USAGE ON SCHEMA optionledger TO ot_w_optionledger;
GRANT INSERT, SELECT ON optionledger.flex_fetch TO ot_w_optionledger;
GRANT INSERT, SELECT ON optionledger.flex_trade_row TO ot_w_optionledger;

-- The reader of the store reads the views.
GRANT USAGE ON SCHEMA rd TO ot_w_optionledger;
GRANT SELECT ON rd.flex_fetch_r1 TO ot_w_optionledger;
GRANT SELECT ON rd.flex_trade_row_r1 TO ot_w_optionledger;

-- Read-only: the pages read the views and nothing else.
GRANT USAGE ON SCHEMA rd TO ot_ro_pages;
GRANT SELECT ON rd.flex_fetch_r1 TO ot_ro_pages;
GRANT SELECT ON rd.flex_trade_row_r1 TO ot_ro_pages;

-- +goose Down
REVOKE SELECT ON rd.flex_trade_row_r1 FROM ot_ro_pages;
REVOKE SELECT ON rd.flex_fetch_r1 FROM ot_ro_pages;
REVOKE SELECT ON rd.flex_trade_row_r1 FROM ot_w_optionledger;
REVOKE SELECT ON rd.flex_fetch_r1 FROM ot_w_optionledger;
REVOKE USAGE ON SCHEMA rd FROM ot_w_optionledger;
REVOKE INSERT, SELECT ON optionledger.flex_trade_row FROM ot_w_optionledger;
REVOKE INSERT, SELECT ON optionledger.flex_fetch FROM ot_w_optionledger;
REVOKE USAGE ON SCHEMA optionledger FROM ot_w_optionledger;
DROP VIEW rd.flex_trade_row_r1;
DROP VIEW rd.flex_fetch_r1;
DROP TABLE optionledger.flex_trade_row;
DROP TABLE optionledger.flex_fetch;
DROP SCHEMA optionledger;
