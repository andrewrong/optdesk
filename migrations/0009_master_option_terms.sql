-- +goose Up
-- The terms of one option, one row per version (the owner's schema v3 names), its current-version view in schema rd,
-- and read access to the view for the page role. The page role gets nothing in the schema of the table: the view runs
-- with the rights of its owner. No writer role has a right here yet; that comes with the writer. Schema rd is made by 0006.
CREATE TABLE master.option_terms (
    terms_version_id bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    instrument_id    bigint      NOT NULL REFERENCES master.instrument (instrument_id),
    "right"          char(1)     NOT NULL CHECK ("right" IN ('C', 'P')),
    strike           numeric     NOT NULL,
    strike_currency  char(3)     NOT NULL,
    multiplier       numeric     NOT NULL,
    expiration_date  date        NOT NULL,
    exercise_style   text        NOT NULL,
    settlement_type  text        NOT NULL,
    valid_from       timestamptz NOT NULL,
    valid_to         timestamptz
);

CREATE UNIQUE INDEX option_terms_current ON master.option_terms (instrument_id) WHERE valid_to IS NULL;

CREATE VIEW rd.option_terms_r1 AS
SELECT terms_version_id, instrument_id, "right", strike, strike_currency, multiplier, expiration_date, exercise_style,
       settlement_type, valid_from, valid_to
FROM master.option_terms
WHERE valid_to IS NULL;

ALTER TABLE master.option_terms OWNER TO ot_owner;
ALTER VIEW rd.option_terms_r1 OWNER TO ot_owner;

GRANT USAGE ON SCHEMA rd TO ot_ro_pages;
GRANT SELECT ON rd.option_terms_r1 TO ot_ro_pages;

-- +goose Down
DROP VIEW rd.option_terms_r1;
DROP INDEX master.option_terms_current;
DROP TABLE master.option_terms;
