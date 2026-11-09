-- +goose Up
-- The other sections of the Flex statement (cash events, open positions, net asset value,
-- conversion rates), as sent, under the account alias; insert only, a changed row is a new
-- revision. Schema optionledger, the role ot_w_optionledger and the USAGE grants on the schemas
-- optionledger and rd are held by 0022_optionledger.sql.
CREATE TABLE optionledger.flex_section_row (
    alias             text        NOT NULL,
    section           text        NOT NULL,
    row_key           text        NOT NULL,
    revision          integer     NOT NULL,
    fetch_id          text        NOT NULL,
    row_index         integer     NOT NULL,
    attributes        text        NOT NULL,
    attributes_sha256 text        NOT NULL,
    recorded_at       timestamptz NOT NULL,
    PRIMARY KEY (alias, section, row_key, revision),
    CONSTRAINT flex_section_row_revision_ck CHECK (revision >= 1),
    CONSTRAINT flex_section_row_index_ck CHECK (row_index >= 0)
);

CREATE TABLE optionledger.flex_section_status (
    fetch_id     text        NOT NULL,
    alias        text        NOT NULL,
    section      text        NOT NULL,
    state        text        NOT NULL,
    row_count    integer     NOT NULL,
    rows_refused integer     NOT NULL,
    recorded_at  timestamptz NOT NULL,
    PRIMARY KEY (fetch_id, alias, section),
    CONSTRAINT flex_section_status_state_ck CHECK (state IN ('present', 'absent')),
    CONSTRAINT flex_section_status_count_ck CHECK (row_count >= 0 AND rows_refused >= 0)
);

CREATE VIEW rd.flex_section_row_r1 AS
SELECT alias, section, row_key, revision, fetch_id, row_index, attributes, attributes_sha256, recorded_at
FROM optionledger.flex_section_row;

CREATE VIEW rd.flex_section_status_r1 AS
SELECT fetch_id, alias, section, state, row_count, rows_refused, recorded_at
FROM optionledger.flex_section_status;

ALTER TABLE optionledger.flex_section_row OWNER TO ot_owner;
ALTER TABLE optionledger.flex_section_status OWNER TO ot_owner;
ALTER VIEW rd.flex_section_row_r1 OWNER TO ot_owner;
ALTER VIEW rd.flex_section_status_r1 OWNER TO ot_owner;

GRANT INSERT, SELECT ON optionledger.flex_section_row TO ot_w_optionledger;
GRANT INSERT, SELECT ON optionledger.flex_section_status TO ot_w_optionledger;

-- The reader of the store reads the views.
GRANT SELECT ON rd.flex_section_row_r1 TO ot_w_optionledger;
GRANT SELECT ON rd.flex_section_status_r1 TO ot_w_optionledger;

-- The pages read the views and no table.
GRANT SELECT ON rd.flex_section_row_r1 TO ot_ro_pages;
GRANT SELECT ON rd.flex_section_status_r1 TO ot_ro_pages;

-- +goose Down
REVOKE SELECT ON rd.flex_section_row_r1 FROM ot_ro_pages;
REVOKE SELECT ON rd.flex_section_status_r1 FROM ot_ro_pages;
REVOKE SELECT ON rd.flex_section_row_r1 FROM ot_w_optionledger;
REVOKE SELECT ON rd.flex_section_status_r1 FROM ot_w_optionledger;
DROP VIEW rd.flex_section_status_r1;
DROP VIEW rd.flex_section_row_r1;
REVOKE INSERT, SELECT ON optionledger.flex_section_status FROM ot_w_optionledger;
REVOKE INSERT, SELECT ON optionledger.flex_section_row FROM ot_w_optionledger;
DROP TABLE optionledger.flex_section_status;
DROP TABLE optionledger.flex_section_row;
