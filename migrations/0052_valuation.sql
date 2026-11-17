-- +goose Up
-- Run by ot_owner, which owns every object it changes.
-- docs/design/valuation-structure.md, Issues entry 5 (Decisions 1, 8, 9, 12 and 16): the schema valuation and its five
-- insert-only tables, and the five views the pages read. A row is never updated or deleted, a change is another row,
-- and the newest row per key wins. set_at and entered_at come from the database's now(), so no writer holds a clock.
-- ot_w_settings inserts and reads the tables (the writer reads the status of a review_id before it inserts);
-- ot_ro_pages reads the views and no table. The migration inserts no row.
CREATE SCHEMA IF NOT EXISTS rd;
CREATE SCHEMA valuation;
ALTER SCHEMA valuation OWNER TO ot_owner;

-- 1. The company profile: the newest row per company_key wins.
CREATE TABLE valuation.company_profile (
    id          bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    company_key text        NOT NULL,
    name        text        NOT NULL,
    symbol      text        NOT NULL DEFAULT '',
    currency    text        NOT NULL,
    r_market    text        NOT NULL CHECK (r_market IN ('US', 'JP', 'HKCN')),
    pool        text        NOT NULL CHECK (pool IN ('in_pool', 'watch', 'removed')),
    pool_date   date        NULL,
    note        text        NOT NULL DEFAULT '',
    set_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX company_profile_key_idx ON valuation.company_profile (company_key, set_at DESC, id DESC);

-- 2. The review: rows of one review_id are drafts, at most one effective row and at most one void row (Decision 8).
-- Decision 1 and 12: only review_id, company_key, status and source are required for a draft; the four values of a
-- row that is not a draft (type, version_date, v, p) are required by the one CHECK below.
CREATE TABLE valuation.review (
    id                bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    review_id         text        NOT NULL,
    company_key       text        NOT NULL,
    type              text        NULL,
    version_date      date        NULL,
    fiscal_period_end date        NULL,
    report_date       date        NULL,
    status            text        NOT NULL,
    source            text        NOT NULL,
    v                 text        NULL,
    p                 text        NULL,
    trigger_type      text        NULL,
    earnings_base     text        NULL,
    base_basis        text        NULL,
    forward_factor    text        NULL,
    locked_multiple   text        NULL,
    method_results    jsonb       NULL,
    adjustments       jsonb       NULL,
    scenarios         jsonb       NULL,
    change_memo       text        NULL,
    reaction          text        NULL,
    moat              text        NULL,
    understand        text        NULL,
    management        text        NULL,
    financial         text        NULL,
    reasons           jsonb       NULL,
    report_text       text        NULL,
    void_reason       text        NULL,
    entered_at        timestamptz NOT NULL DEFAULT now(),
    effective_at      timestamptz NULL,
    CONSTRAINT review_status_ck CHECK (status IN ('draft', 'effective', 'void')),
    CONSTRAINT review_source_ck CHECK (source IN ('owner', 'program')),
    CONSTRAINT review_required_ck CHECK (
        status = 'draft'
        OR (type IS NOT NULL AND version_date IS NOT NULL AND v IS NOT NULL AND p IS NOT NULL))
);
CREATE INDEX review_company_idx ON valuation.review (company_key, id DESC);
CREATE UNIQUE INDEX review_effective_uq ON valuation.review (review_id) WHERE status = 'effective';
CREATE UNIQUE INDEX review_void_uq ON valuation.review (review_id) WHERE status = 'void';

-- effective_at is the database's now() on an effective row and null on every other row, whatever the insert says.
-- +goose StatementBegin
CREATE FUNCTION valuation.review_effective_at() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.status = 'effective' THEN
        NEW.effective_at := now();
    ELSE
        NEW.effective_at := NULL;
    END IF;
    RETURN NEW;
END
$$;
-- +goose StatementEnd
ALTER FUNCTION valuation.review_effective_at() OWNER TO ot_owner;
CREATE TRIGGER review_effective_at BEFORE INSERT ON valuation.review
    FOR EACH ROW EXECUTE FUNCTION valuation.review_effective_at();

-- 3. The R version: one row per version.
CREATE TABLE valuation.r_version (
    id             bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    effective_date date        NOT NULL,
    yield_used     text        NOT NULL DEFAULT '',
    r_us           text        NOT NULL,
    r_jp           text        NOT NULL,
    r_hkcn         text        NOT NULL,
    ai_switch      boolean     NOT NULL DEFAULT false,
    ai_expiry      text        NOT NULL DEFAULT '',
    note           text        NOT NULL DEFAULT '',
    set_at         timestamptz NOT NULL DEFAULT now()
);

-- 4. The two parameter tables: one row per bucket; the rows of a version share effective_date and set_at because
-- they are inserted in one transaction (Decision 9).
CREATE TABLE valuation.r_table (
    id             bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    effective_date date        NOT NULL,
    set_at         timestamptz NOT NULL DEFAULT now(),
    yield_up_to    text        NOT NULL,
    us             text        NOT NULL,
    jp             text        NOT NULL,
    hkcn           text        NOT NULL,
    CONSTRAINT r_table_bucket_uq UNIQUE (effective_date, set_at, yield_up_to)
);

CREATE TABLE valuation.margin_table (
    id             bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    effective_date date        NOT NULL,
    set_at         timestamptz NOT NULL DEFAULT now(),
    p_from         text        NOT NULL,
    m              text        NOT NULL,
    w              text        NOT NULL,
    h              text        NOT NULL,
    s              text        NOT NULL,
    CONSTRAINT margin_table_bucket_uq UNIQUE (effective_date, set_at, p_from)
);

ALTER TABLE valuation.company_profile OWNER TO ot_owner;
ALTER TABLE valuation.review OWNER TO ot_owner;
ALTER TABLE valuation.r_version OWNER TO ot_owner;
ALTER TABLE valuation.r_table OWNER TO ot_owner;
ALTER TABLE valuation.margin_table OWNER TO ot_owner;

-- Every row passes through, every column named.
CREATE VIEW rd.valuation_company_profile_r1 AS
SELECT id, company_key, name, symbol, currency, r_market, pool, pool_date, note, set_at
FROM valuation.company_profile;

CREATE VIEW rd.valuation_review_r1 AS
SELECT id, review_id, company_key, type, version_date, fiscal_period_end, report_date, status, source, v, p,
       trigger_type, earnings_base, base_basis, forward_factor, locked_multiple, method_results, adjustments,
       scenarios, change_memo, reaction, moat, understand, management, financial, reasons, report_text,
       void_reason, entered_at, effective_at
FROM valuation.review;

CREATE VIEW rd.valuation_r_version_r1 AS
SELECT id, effective_date, yield_used, r_us, r_jp, r_hkcn, ai_switch, ai_expiry, note, set_at
FROM valuation.r_version;

CREATE VIEW rd.valuation_r_table_r1 AS
SELECT id, effective_date, set_at, yield_up_to, us, jp, hkcn
FROM valuation.r_table;

CREATE VIEW rd.valuation_margin_table_r1 AS
SELECT id, effective_date, set_at, p_from, m, w, h, s
FROM valuation.margin_table;

ALTER VIEW rd.valuation_company_profile_r1 OWNER TO ot_owner;
ALTER VIEW rd.valuation_review_r1 OWNER TO ot_owner;
ALTER VIEW rd.valuation_r_version_r1 OWNER TO ot_owner;
ALTER VIEW rd.valuation_r_table_r1 OWNER TO ot_owner;
ALTER VIEW rd.valuation_margin_table_r1 OWNER TO ot_owner;

GRANT USAGE ON SCHEMA valuation TO ot_w_settings;
GRANT INSERT, SELECT ON valuation.company_profile, valuation.review, valuation.r_version, valuation.r_table,
    valuation.margin_table TO ot_w_settings;
GRANT USAGE ON SCHEMA rd TO ot_w_settings, ot_ro_pages;
GRANT SELECT ON rd.valuation_company_profile_r1, rd.valuation_review_r1, rd.valuation_r_version_r1,
    rd.valuation_r_table_r1, rd.valuation_margin_table_r1 TO ot_ro_pages, ot_w_settings;

-- +goose Down
-- Never drops owner data: as 0044 does for a check it cannot undo, it raises when any of the five tables holds a row
-- and drops the views, the tables and the schema only when all five are empty.
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM valuation.company_profile)
       OR EXISTS (SELECT 1 FROM valuation.review)
       OR EXISTS (SELECT 1 FROM valuation.r_version)
       OR EXISTS (SELECT 1 FROM valuation.r_table)
       OR EXISTS (SELECT 1 FROM valuation.margin_table) THEN
        RAISE EXCEPTION '0052 down: a valuation table holds a row; the migration does not drop owner data';
    END IF;
END
$$;
-- +goose StatementEnd
DROP VIEW rd.valuation_margin_table_r1;
DROP VIEW rd.valuation_r_table_r1;
DROP VIEW rd.valuation_r_version_r1;
DROP VIEW rd.valuation_review_r1;
DROP VIEW rd.valuation_company_profile_r1;
DROP TABLE valuation.margin_table;
DROP TABLE valuation.r_table;
DROP TABLE valuation.r_version;
DROP TABLE valuation.review;
DROP FUNCTION valuation.review_effective_at();
DROP TABLE valuation.company_profile;
DROP SCHEMA valuation;
