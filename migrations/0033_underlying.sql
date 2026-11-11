-- +goose Up
-- settings.underlying: the names the owner follows (docs/design/underlyings.md Decision 2 and 3). Insert-only and
-- versioned like settings.rule_param: a row is never updated or deleted, a change is one more row, and the newest row
-- per name_key (ties by id) wins. set_at comes from the database's now(), so no writer holds a clock.
-- The role ot_w_settings (page route and import) and the role ot_w_underlying_resolve (the resolve job, made by
-- internal/store/setup) hold INSERT and SELECT on the table and read the view; ot_ro_pages and ot_runner read the
-- view and no table. ot_w_underlying_resolve holds nothing on any other settings table.
CREATE SCHEMA IF NOT EXISTS settings;
CREATE SCHEMA IF NOT EXISTS rd;

CREATE TABLE settings.underlying (
    id                   bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name_key             text        NOT NULL,
    symbol               text        NOT NULL,
    market               text        NOT NULL CHECK (market IN ('US', 'JP')),
    kind                 text        NOT NULL CHECK (kind IN ('stock', 'etf')),
    exchange             text        NOT NULL,
    currency             text        NOT NULL,
    name                 text        NOT NULL DEFAULT '',
    issuer               text        NOT NULL DEFAULT '',
    con_id               bigint      NULL CHECK (con_id IS NULL OR con_id > 0),
    status               text        NOT NULL CHECK (status IN ('pending', 'active', 'paused', 'retired', 'unresolved')),
    reason               text        NOT NULL DEFAULT '',
    options              boolean     NOT NULL DEFAULT true,
    fundamentals         boolean     NOT NULL DEFAULT false,
    buy_price            text        NOT NULL DEFAULT '',
    sale_price           text        NOT NULL DEFAULT '',
    report_date_override text        NOT NULL DEFAULT '',
    note                 text        NOT NULL DEFAULT '',
    set_at               timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX underlying_key_set_at ON settings.underlying (name_key, set_at DESC, id DESC);

ALTER SCHEMA settings OWNER TO ot_owner;
ALTER TABLE settings.underlying OWNER TO ot_owner;

-- Every row passes through, every column named.
CREATE VIEW rd.underlying_r1 AS
SELECT id, name_key, symbol, market, kind, exchange, currency, name, issuer, con_id, status, reason,
       options, fundamentals, buy_price, sale_price, report_date_override, note, set_at
FROM settings.underlying;

ALTER VIEW rd.underlying_r1 OWNER TO ot_owner;

GRANT USAGE ON SCHEMA settings TO ot_w_settings;
GRANT INSERT, SELECT ON settings.underlying TO ot_w_settings;
GRANT USAGE ON SCHEMA rd TO ot_w_settings;
GRANT SELECT ON rd.underlying_r1 TO ot_w_settings;

-- The resolve job: its own role, one table, one view.
GRANT USAGE ON SCHEMA settings TO ot_w_underlying_resolve;
GRANT INSERT, SELECT ON settings.underlying TO ot_w_underlying_resolve;
GRANT USAGE ON SCHEMA rd TO ot_w_underlying_resolve;
GRANT SELECT ON rd.underlying_r1 TO ot_w_underlying_resolve;

-- The pages read the view and nothing else.
GRANT USAGE ON SCHEMA rd TO ot_ro_pages;
GRANT SELECT ON rd.underlying_r1 TO ot_ro_pages;

-- A job opened with the runner DSN reads the list through the view.
GRANT USAGE ON SCHEMA rd TO ot_runner;
GRANT SELECT ON rd.underlying_r1 TO ot_runner;

-- +goose Down
-- The schemas settings and rd belong to earlier migrations and stay, as do the USAGE grants on rd.
REVOKE SELECT ON rd.underlying_r1 FROM ot_runner;
REVOKE SELECT ON rd.underlying_r1 FROM ot_ro_pages;
REVOKE SELECT ON rd.underlying_r1 FROM ot_w_underlying_resolve;
REVOKE INSERT, SELECT ON settings.underlying FROM ot_w_underlying_resolve;
REVOKE SELECT ON rd.underlying_r1 FROM ot_w_settings;
REVOKE INSERT, SELECT ON settings.underlying FROM ot_w_settings;
DROP VIEW rd.underlying_r1;
DROP TABLE settings.underlying;
