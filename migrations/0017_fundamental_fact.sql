-- +goose Up
-- obs.fundamental_fact: one filed value of one concept of one issuer, append only (docs/design/valuation.md Decision 6).
-- period_start_key is NOT NULL and holds 0001-01-01 for an absent period_start, so a repeated instant fact is a
-- duplicate of the key. The role ot_w_fundamental (made by internal/store/setup) gets INSERT on the table and
-- SELECT on the view; ot_ro_pages gets SELECT on the view. Neither reads the table.
CREATE TABLE obs.fundamental_fact (
    source           text        NOT NULL,
    subject          text        NOT NULL,
    doc_id           text        NOT NULL,
    taxonomy         text        NOT NULL,
    concept          text        NOT NULL,
    unit             text        NOT NULL,
    period_start_key date        NOT NULL,
    period_end       date        NOT NULL,
    currency         text        NOT NULL,
    value            text        NOT NULL,
    form             text        NOT NULL,
    filed            date        NOT NULL,
    retrieved_at     timestamptz NOT NULL,
    raw_sha256       text        NOT NULL,
    PRIMARY KEY (source, subject, doc_id, taxonomy, concept, unit, period_start_key, period_end)
);

CREATE INDEX fundamental_fact_subject_filed ON obs.fundamental_fact (subject, filed);

ALTER TABLE obs.fundamental_fact OWNER TO ot_owner;

CREATE VIEW rd.fundamental_fact_r1 AS
SELECT source, subject, doc_id, taxonomy, concept, unit,
       CASE WHEN period_start_key = DATE '0001-01-01' THEN NULL ELSE period_start_key END AS period_start,
       period_end, currency, value, form, filed, retrieved_at, raw_sha256
FROM obs.fundamental_fact;

ALTER VIEW rd.fundamental_fact_r1 OWNER TO ot_owner;

GRANT USAGE ON SCHEMA obs, rd TO ot_w_fundamental;
GRANT INSERT ON obs.fundamental_fact TO ot_w_fundamental;
GRANT SELECT ON rd.fundamental_fact_r1 TO ot_ro_pages;
GRANT SELECT ON rd.fundamental_fact_r1 TO ot_w_fundamental;

-- +goose Down
REVOKE SELECT ON rd.fundamental_fact_r1 FROM ot_w_fundamental;
REVOKE SELECT ON rd.fundamental_fact_r1 FROM ot_ro_pages;
REVOKE INSERT ON obs.fundamental_fact FROM ot_w_fundamental;
REVOKE USAGE ON SCHEMA obs, rd FROM ot_w_fundamental;
DROP VIEW rd.fundamental_fact_r1;
DROP TABLE obs.fundamental_fact;
