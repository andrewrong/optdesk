-- +goose Up
-- docs/design/data-collection.md entry 4 (Decisions 1 and 15): the fiscal year, fiscal period and calendar frame
-- SEC gives a fact. fy and fp are those of the filing that reported the fact. The columns are added with a default
-- and touch no row; the next daily run of `fundamentals` fills the rows already stored (the upsert compares them).
-- The view r1 is not touched: the new view r2 is the old columns and the three new ones.
ALTER TABLE obs.fundamental_fact
    ADD COLUMN fy    integer,
    ADD COLUMN fp    text NOT NULL DEFAULT '',
    ADD COLUMN frame text NOT NULL DEFAULT '';

-- The upsert WHERE reads the compared columns, so SELECT is needed beside UPDATE (INSERT is table-level).
GRANT SELECT (fy, fp, frame) ON obs.fundamental_fact TO ot_w_fundamental;
GRANT UPDATE (fy, fp, frame) ON obs.fundamental_fact TO ot_w_fundamental;

CREATE VIEW rd.fundamental_fact_r2 AS
SELECT source, subject, doc_id, taxonomy, concept, unit,
       CASE WHEN period_start_key = DATE '0001-01-01' THEN NULL ELSE period_start_key END AS period_start,
       period_end, currency, value, form, filed, retrieved_at, raw_sha256,
       fy, fp, frame
FROM obs.fundamental_fact;

ALTER VIEW rd.fundamental_fact_r2 OWNER TO ot_owner;

GRANT SELECT ON rd.fundamental_fact_r2 TO ot_ro_pages;
GRANT SELECT ON rd.fundamental_fact_r2 TO ot_w_fundamental;

-- +goose Down
DROP VIEW rd.fundamental_fact_r2;
REVOKE UPDATE (fy, fp, frame) ON obs.fundamental_fact FROM ot_w_fundamental;
REVOKE SELECT (fy, fp, frame) ON obs.fundamental_fact FROM ot_w_fundamental;
ALTER TABLE obs.fundamental_fact
    DROP COLUMN frame,
    DROP COLUMN fp,
    DROP COLUMN fy;
