-- +goose Up
-- docs/design/base-data-page.md entry 20: the runs of the reconciliation of the base data page. recon.run is one
-- comparison of one pair for one session day, recon.item holds its findings (a match is never stored). Both tables are
-- insert-only: a rerun is a new run_id and the view rd.recon_latest_r1 takes the newest finished run per
-- (pair_id, session_day), as rd.strategy_result_r1 does (0042_strategy_result.sql). ot_w_recon (made by
-- internal/store/setup) has INSERT and SELECT on both tables and no UPDATE, DELETE or TRUNCATE; ot_ro_pages reads the
-- view only.
CREATE SCHEMA IF NOT EXISTS recon;
CREATE SCHEMA IF NOT EXISTS rd;

CREATE TABLE recon.run (
    run_id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    pair_id        text        NOT NULL,
    session_day    date        NOT NULL,
    started_at     timestamptz NOT NULL,
    finished_at    timestamptz NOT NULL,
    status         text        NOT NULL,
    reason         text        NOT NULL DEFAULT '',
    n              integer     NOT NULL,
    matched        integer     NOT NULL,
    diff           integer     NOT NULL,
    missing_ours   integer     NOT NULL,
    missing_other  integer     NOT NULL,
    skipped        integer     NOT NULL,
    pairs_asked    integer     NOT NULL,
    pairs_compared integer     NOT NULL,
    tolerance_text text        NOT NULL,
    source_id      text        NOT NULL,
    CONSTRAINT run_status_ck CHECK (status IN ('ok', 'partial', 'failed')),
    CONSTRAINT run_counts_ck CHECK (n >= 0 AND matched >= 0 AND diff >= 0 AND missing_ours >= 0 AND missing_other >= 0
        AND skipped >= 0 AND pairs_asked >= 0 AND pairs_compared >= 0)
);

CREATE INDEX run_pair_day ON recon.run (pair_id, session_day, finished_at DESC, run_id DESC);

CREATE TABLE recon.item (
    item_id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    run_id  bigint NOT NULL REFERENCES recon.run (run_id),
    key     text   NOT NULL,
    company text   NOT NULL DEFAULT '',
    ours    text   NOT NULL DEFAULT '',
    other   text   NOT NULL DEFAULT '',
    diff    text   NOT NULL DEFAULT '',
    kind    text   NOT NULL,
    reason  text   NOT NULL DEFAULT '',
    CONSTRAINT item_kind_ck CHECK (kind IN ('diff', 'missing_ours', 'missing_other', 'skipped'))
);

CREATE INDEX item_run ON recon.item (run_id);

ALTER SCHEMA recon OWNER TO ot_owner;
ALTER TABLE recon.run OWNER TO ot_owner;
ALTER TABLE recon.item OWNER TO ot_owner;

-- One row per newest run and item, the run columns repeated; a run with no item has one row with null item columns.
-- item_rank counts the items of a run in the severity order diff, missing_ours, missing_other, skipped, then company
-- and key, so a reader keeps the first LatestItemsMax of them. The view runs with the rights of its owner, so the page
-- role needs nothing on the recon schema.
CREATE VIEW rd.recon_latest_r1 AS
WITH newest AS (
    SELECT DISTINCT ON (pair_id, session_day) *
    FROM recon.run
    ORDER BY pair_id, session_day, finished_at DESC, run_id DESC
),
ranked AS (
    SELECT i.run_id, i.key, i.company, i.ours, i.other, i.diff, i.kind, i.reason,
           row_number() OVER (
               PARTITION BY i.run_id
               ORDER BY CASE i.kind WHEN 'diff' THEN 1 WHEN 'missing_ours' THEN 2 WHEN 'missing_other' THEN 3 ELSE 4 END,
                        i.company, i.key, i.item_id) AS item_rank
    FROM recon.item i
    WHERE i.run_id IN (SELECT run_id FROM newest)
)
SELECT r.run_id, r.pair_id, r.session_day, r.started_at, r.finished_at, r.status, r.reason, r.n, r.matched, r.diff,
       r.missing_ours, r.missing_other, r.skipped, r.pairs_asked, r.pairs_compared, r.tolerance_text, r.source_id,
       k.key AS item_key, k.company AS item_company, k.ours AS item_ours, k.other AS item_other,
       k.diff AS item_diff, k.kind AS item_kind, k.reason AS item_reason, k.item_rank::integer AS item_rank
FROM newest r
LEFT JOIN ranked k ON k.run_id = r.run_id;

ALTER VIEW rd.recon_latest_r1 OWNER TO ot_owner;

GRANT USAGE ON SCHEMA recon TO ot_w_recon;
GRANT INSERT, SELECT ON recon.run, recon.item TO ot_w_recon;

GRANT USAGE ON SCHEMA rd TO ot_ro_pages;
GRANT SELECT ON rd.recon_latest_r1 TO ot_ro_pages;

-- +goose Down
-- USAGE on schema rd is shared with the other views and stays granted; schema rd stays.
REVOKE SELECT ON rd.recon_latest_r1 FROM ot_ro_pages;
REVOKE INSERT, SELECT ON recon.run, recon.item FROM ot_w_recon;
REVOKE USAGE ON SCHEMA recon FROM ot_w_recon;
DROP VIEW rd.recon_latest_r1;
DROP TABLE recon.item;
DROP TABLE recon.run;
DROP SCHEMA recon;
