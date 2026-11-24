-- +goose Up
-- The pages look up the instrument id of a stock by name. rd.instrument_r1 has neither the conid nor the status, so a
-- leftover active stock row with ibkr_conid '0' (JP:4063, id 2406) was returned beside the real one. ot_ro_pages has no
-- USAGE on schema master, so the filter lives in a view, which runs with the rights of its owner. It adds a view and its
-- grant and touches no table and no row.
--
-- Listed: stock instruments with status 'active' that have no current ibkr_conid row of value '0'. A stock with no
-- conid row at all stays listed.
CREATE VIEW rd.stock_id_r1 AS
SELECT i.instrument_id,
       i.instrument_type,
       i.name
FROM master.instrument i
WHERE i.instrument_type = 'stock'
  AND i.status = 'active'
  AND NOT EXISTS (
      SELECT 1 FROM master.instrument_identifier d
      WHERE d.instrument_id = i.instrument_id
        AND d.scheme = 'ibkr_conid'
        AND d.value = '0'
        AND d.valid_to IS NULL);

ALTER VIEW rd.stock_id_r1 OWNER TO ot_owner;

GRANT SELECT ON rd.stock_id_r1 TO ot_ro_pages;

-- +goose Down
REVOKE SELECT ON rd.stock_id_r1 FROM ot_ro_pages;
DROP VIEW rd.stock_id_r1;
