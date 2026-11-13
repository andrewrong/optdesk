-- +goose Up
-- Run by ot_owner, which owns every object it changes.
-- Instrument and company (docs/design/instrument-company.md, entry 2): master.instrument gains a symbol, a stock side
-- table master.stock and a company table master.issuer exist, the identifier scheme list opens, and rd.stock and
-- rd.option read the two sides. The migration fails loudly rather than guess: a duplicate current identifier of one
-- scheme on one instrument or a JPY symbol with a dot stops it (one transaction) and changes nothing. Nothing here
-- deletes a row.

-- 1. The company (Decision 6).
CREATE TABLE master.issuer (
    issuer_id   bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name        text   NOT NULL,
    country     text   NOT NULL,
    cik         text   NULL,
    edinet_code text   NULL,
    CONSTRAINT issuer_code_ck CHECK (cik IS NOT NULL OR edinet_code IS NOT NULL)
);
CREATE UNIQUE INDEX issuer_cik_uq ON master.issuer (cik) WHERE cik IS NOT NULL;
CREATE UNIQUE INDEX issuer_edinet_code_uq ON master.issuer (edinet_code) WHERE edinet_code IS NOT NULL;
ALTER TABLE master.issuer OWNER TO ot_owner;

-- 2. The symbol (Decision 1); the default keeps the old insert of ResolveInstrument working.
ALTER TABLE master.instrument ADD COLUMN symbol text NOT NULL DEFAULT '';

-- 3. The stock side (Decision 2).
CREATE TABLE master.stock (
    instrument_id    bigint PRIMARY KEY REFERENCES master.instrument (instrument_id),
    name             text   NOT NULL,
    market           text   NULL CHECK (market IN ('US', 'JP', 'EU')),
    primary_exchange text   NULL,
    issuer_id        bigint NULL REFERENCES master.issuer (issuer_id),
    sector_fund      text   NULL
);
ALTER TABLE master.stock OWNER TO ot_owner;

-- 4. Backfill of the symbol (Decision 1): the current name of a stock, etf or index; for an option the current osi
-- identifier, else the name.
UPDATE master.instrument i
SET symbol = CASE
    WHEN i.instrument_type = 'option' THEN COALESCE(
        (SELECT d.value FROM master.instrument_identifier d
         WHERE d.instrument_id = i.instrument_id AND d.scheme = 'osi' AND d.valid_to IS NULL
         ORDER BY d.valid_from DESC LIMIT 1), i.name)
    ELSE i.name
END;

-- 5. Backfill of the stock rows and the issuers (Decision 7); two scratch tables in master, dropped below (the database revokes TEMP from PUBLIC). Of settings.underlying (read through rd.underlying_r1,
-- which passes every version) the newest row of each name_key by set_at, then id, is taken first; only then the
-- rows whose con_id is the instrument's ibkr_conid and whose name is not empty are matched, so an older version of a
-- name never wins over the newest. When two name_keys match one instrument the smaller name_key (byte order) is used.
CREATE TABLE master.underlying_newest_0044 AS
SELECT DISTINCT ON (name_key) name_key, market, exchange, name, issuer, con_id
FROM rd.underlying_r1
ORDER BY name_key, set_at DESC, id DESC;

CREATE TABLE master.stock_seed_0044 AS
SELECT i.instrument_id,
       COALESCE(s.name, i.name) AS name,
       CASE i.currency WHEN 'USD' THEN 'US' WHEN 'JPY' THEN 'JP' WHEN 'EUR' THEN 'EU' END AS market,
       CASE WHEN s.exchange IN ('', 'SMART') THEN NULL ELSE s.exchange END AS primary_exchange,
       CASE WHEN s.issuer ~ '^cik:[0-9]{10}$' THEN substr(s.issuer, 5) END AS cik,
       CASE WHEN s.issuer ~ '^edinet:[A-Za-z0-9]+$' THEN substr(s.issuer, 8) END AS edinet_code,
       s.name AS issuer_name,
       s.market AS issuer_country,
       s.name_key
FROM master.instrument i
LEFT JOIN LATERAL (
    SELECT n.*
    FROM master.underlying_newest_0044 n
    JOIN master.instrument_identifier d
      ON d.instrument_id = i.instrument_id AND d.scheme = 'ibkr_conid' AND d.valid_to IS NULL
    WHERE n.con_id::text = d.value AND n.name <> ''
    ORDER BY n.name_key COLLATE "C"
    LIMIT 1
) s ON true
WHERE i.instrument_type IN ('stock', 'etf', 'index');

-- One issuer row per distinct code, named after the settings row (the smaller name_key, then instrument id). The
-- issuer text carries one code, so a company with a US and a JP line stays two issuers.
INSERT INTO master.issuer (name, country, cik)
SELECT DISTINCT ON (cik) issuer_name, issuer_country, cik
FROM master.stock_seed_0044 WHERE cik IS NOT NULL
ORDER BY cik, name_key COLLATE "C", instrument_id;

INSERT INTO master.issuer (name, country, edinet_code)
SELECT DISTINCT ON (edinet_code) issuer_name, issuer_country, edinet_code
FROM master.stock_seed_0044 WHERE edinet_code IS NOT NULL
ORDER BY edinet_code, name_key COLLATE "C", instrument_id;

INSERT INTO master.stock (instrument_id, name, market, primary_exchange, issuer_id)
SELECT s.instrument_id, s.name, s.market, s.primary_exchange, c.issuer_id
FROM master.stock_seed_0044 s
LEFT JOIN master.issuer c ON (s.cik IS NOT NULL AND c.cik = s.cik) OR (s.edinet_code IS NOT NULL AND c.edinet_code = s.edinet_code);

DROP TABLE master.stock_seed_0044;
DROP TABLE master.underlying_newest_0044;

-- 6. The foreign key and the check of Decision 1; the issuer_id of the instrument and of the stock are one value.
UPDATE master.instrument i SET issuer_id = s.issuer_id
FROM master.stock s WHERE s.instrument_id = i.instrument_id AND s.issuer_id IS NOT NULL;

ALTER TABLE master.instrument
    ADD CONSTRAINT instrument_issuer_fk FOREIGN KEY (issuer_id) REFERENCES master.issuer (issuer_id),
    ADD CONSTRAINT instrument_symbol_jpy_ck CHECK (currency <> 'JPY' OR symbol !~ '\.');

-- 7. The scheme list opens (Decision 5): the two old schemes stay for the rows that exist. One current row per
-- instrument and scheme.
ALTER TABLE master.instrument_identifier DROP CONSTRAINT instrument_identifier_scheme_ck;
ALTER TABLE master.instrument_identifier ADD CONSTRAINT instrument_identifier_scheme_ck CHECK (scheme IN (
    'ibkr_conid', 'osi', 'ticker_exchange', 'jpx_option_code', 'isin', 'cusip', 'figi',
    'sym_fred', 'sym_alphavantage', 'sym_jquants', 'sym_sec', 'sym_eodhd'));
CREATE UNIQUE INDEX instrument_identifier_current_uq ON master.instrument_identifier (instrument_id, scheme) WHERE valid_to IS NULL;

-- 8. The two views for the pages (Decision 4); they run with the rights of their owner.
CREATE VIEW rd.stock AS
SELECT s.instrument_id, i.symbol, s.name, s.market, s.primary_exchange, i.currency, i.mic, s.sector_fund,
       s.issuer_id, c.name AS issuer_name
FROM master.stock s
JOIN master.instrument i ON i.instrument_id = s.instrument_id
LEFT JOIN master.issuer c ON c.issuer_id = s.issuer_id;

CREATE VIEW rd.option AS
SELECT i.instrument_id, i.symbol, i.currency, oc.underlying_id, u.symbol AS underlying_symbol, oc.expiry, oc.strike,
       oc."right", oc.multiplier, oc.trading_class, oc.exchange
FROM master.instrument i
LEFT JOIN master.option_contract oc ON oc.instrument_id = i.instrument_id
LEFT JOIN master.instrument u ON u.instrument_id = oc.underlying_id
WHERE i.instrument_type = 'option';

ALTER VIEW rd.stock OWNER TO ot_owner;
ALTER VIEW rd.option OWNER TO ot_owner;

-- 9. Grants (Decision 11): no delete, no truncate, no close of an identifier row.
GRANT SELECT, INSERT, UPDATE ON master.stock TO ot_w_master;
GRANT SELECT, INSERT, UPDATE ON master.issuer TO ot_w_master;
GRANT UPDATE (symbol, issuer_id) ON master.instrument TO ot_w_master;

GRANT USAGE ON SCHEMA rd TO ot_ro_pages;
GRANT USAGE ON SCHEMA rd TO ot_runner;
GRANT SELECT ON rd.stock TO ot_ro_pages;
GRANT SELECT ON rd.option TO ot_ro_pages;
GRANT SELECT ON rd.stock TO ot_runner;
GRANT SELECT ON rd.option TO ot_runner;

-- +goose Down
-- Refuses when a row carries a scheme the old check does not know.
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM master.instrument_identifier
               WHERE scheme NOT IN ('ibkr_conid', 'osi', 'ticker_exchange', 'jpx_option_code', 'isin')) THEN
        RAISE EXCEPTION '0044 down: master.instrument_identifier holds a row of a scheme the old check does not allow';
    END IF;
END
$$;
-- +goose StatementEnd
REVOKE SELECT ON rd.option FROM ot_runner;
REVOKE SELECT ON rd.stock FROM ot_runner;
REVOKE SELECT ON rd.option FROM ot_ro_pages;
REVOKE SELECT ON rd.stock FROM ot_ro_pages;
DROP VIEW rd.option;
DROP VIEW rd.stock;
DROP INDEX master.instrument_identifier_current_uq;
ALTER TABLE master.instrument_identifier DROP CONSTRAINT instrument_identifier_scheme_ck;
ALTER TABLE master.instrument_identifier ADD CONSTRAINT instrument_identifier_scheme_ck
    CHECK (scheme IN ('ibkr_conid', 'osi', 'ticker_exchange', 'jpx_option_code', 'isin'));
REVOKE UPDATE (issuer_id) ON master.instrument FROM ot_w_master;
ALTER TABLE master.instrument DROP CONSTRAINT instrument_symbol_jpy_ck;
ALTER TABLE master.instrument DROP CONSTRAINT instrument_issuer_fk;
UPDATE master.instrument SET issuer_id = NULL;
DROP TABLE master.stock;
DROP TABLE master.issuer;
ALTER TABLE master.instrument DROP COLUMN symbol;
