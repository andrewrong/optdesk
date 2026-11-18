-- +goose Up
-- The pages hold the ibkr_conid of a contract, obs.option_day is keyed by master.instrument.instrument_id. This view is
-- the one place the two meet for the page role: ot_ro_pages has the table grant on master.instrument_identifier but no
-- USAGE on schema master, so it reads the view, which runs with the rights of its owner. It adds a view and its grant
-- and touches no table and no row.
--
-- Only option instruments and their current ibkr_conid (valid_to IS NULL) are listed. The unique index of 0044 is on
-- (instrument_id, scheme), not on the value, so two instruments may share a conid; the reader checks that.
CREATE VIEW rd.option_conid_r1 AS
SELECT i.instrument_id,
       d.value::bigint AS conid
FROM master.instrument i
JOIN master.instrument_identifier d ON d.instrument_id = i.instrument_id
WHERE i.instrument_type = 'option'
  AND d.scheme = 'ibkr_conid'
  AND d.valid_to IS NULL
  AND d.value ~ '^[0-9]{1,18}$';

ALTER VIEW rd.option_conid_r1 OWNER TO ot_owner;

GRANT SELECT ON rd.option_conid_r1 TO ot_ro_pages;

-- +goose Down
REVOKE SELECT ON rd.option_conid_r1 FROM ot_ro_pages;
DROP VIEW rd.option_conid_r1;
