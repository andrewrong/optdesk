-- +goose Up
-- The name of each instrument for the page role: four columns, no identifier, issuer or personal data. The view runs
-- with the rights of its owner, so the page role gets nothing in schema master. Schema rd is made by 0006.
CREATE VIEW rd.instrument_r1 AS
SELECT instrument_id, instrument_type, name, currency
FROM master.instrument;

ALTER VIEW rd.instrument_r1 OWNER TO ot_owner;

GRANT SELECT ON rd.instrument_r1 TO ot_ro_pages;

-- +goose Down
DROP VIEW rd.instrument_r1;
