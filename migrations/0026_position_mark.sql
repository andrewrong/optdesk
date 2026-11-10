-- +goose Up
-- account.position_mark: the stored mark of each open position of an account capture, insert-only
-- (docs/design/position-marks.md Decisions 3 and 4). A row is never updated or deleted; a correction is a row with a
-- higher attempt. Decimals are numeric, null when absent (price is null when basis is none).
-- The role ot_w_posmark (made by internal/store/setup) holds INSERT and SELECT on the table and reads the view;
-- ot_ro_pages reads the view and no table. No role but the owner may UPDATE, DELETE or TRUNCATE.
CREATE TABLE account.position_mark (
    capture_id       text        NOT NULL,
    alias            text        NOT NULL,
    marked_at        timestamptz NOT NULL,
    session_day      date        NOT NULL,
    con_id           bigint      NOT NULL,
    attempt          integer     NOT NULL CHECK (attempt >= 1),
    currency         text        NOT NULL,
    price            numeric,
    basis            text        NOT NULL CHECK (basis IN ('mid', 'last', 'close', 'none')),
    bid              numeric,
    ask              numeric,
    last             numeric,
    close            numeric,
    market_data_type integer     NOT NULL DEFAULT 0,
    status           text        NOT NULL CHECK (status IN ('ok', 'partial', 'no_data', 'no_subscription', 'timeout', 'error', 'disconnected')),
    PRIMARY KEY (capture_id, con_id, attempt)
);

CREATE VIEW rd.account_position_mark_r1 AS
SELECT capture_id, alias, marked_at, session_day, con_id, attempt, currency, price, basis, bid, ask, last, close,
       market_data_type, status
FROM account.position_mark;

ALTER TABLE account.position_mark OWNER TO ot_owner;
ALTER VIEW rd.account_position_mark_r1 OWNER TO ot_owner;

GRANT USAGE ON SCHEMA account TO ot_w_posmark;
GRANT INSERT, SELECT ON account.position_mark TO ot_w_posmark;

-- The store reads the view (markpg.Marks), so ot_w_posmark may read it.
GRANT USAGE ON SCHEMA rd TO ot_w_posmark;
GRANT SELECT ON rd.account_position_mark_r1 TO ot_w_posmark;

-- The pages read the view and no table.
GRANT SELECT ON
    rd.account_position_mark_r1
    TO ot_ro_pages;

-- +goose Down
REVOKE SELECT ON rd.account_position_mark_r1 FROM ot_ro_pages;
REVOKE SELECT ON rd.account_position_mark_r1 FROM ot_w_posmark;
REVOKE USAGE ON SCHEMA rd FROM ot_w_posmark;
DROP VIEW rd.account_position_mark_r1;
REVOKE INSERT, SELECT ON account.position_mark FROM ot_w_posmark;
REVOKE USAGE ON SCHEMA account FROM ot_w_posmark;
DROP TABLE account.position_mark;
