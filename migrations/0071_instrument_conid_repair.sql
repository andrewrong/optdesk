-- +goose Up
-- Two option instruments hold an IB contract id that is another contract: instrument 20719 (COST 261211 C720) holds
-- ibkr_conid 940345466, the 690 strike call, and its own id is 940345846; instrument 4038 (WMT 261211 C101) holds
-- 935946396 and its own id is 940466383. ot_w_master has only SELECT and INSERT on master.instrument_identifier (0008),
-- so the owner's migrator closes each wrong row (valid_to) and inserts the right one with valid_from at the migration
-- time. The osi rows and every other identifier row stay. No obs table is touched: the three obs.option_eod rows of
-- instrument 20719 (sessions 2026-11-19, 2026-11-20, 2026-11-23) are quotes of the 690 call and stay as stored.
-- A second run changes nothing: each update needs valid_to IS NULL, each insert is skipped while an open row exists.
UPDATE master.instrument_identifier
SET valid_to = now()
WHERE instrument_id = 20719 AND scheme = 'ibkr_conid' AND value = '940345466' AND valid_to IS NULL;

INSERT INTO master.instrument_identifier (instrument_id, scheme, value, valid_from)
SELECT 20719, 'ibkr_conid', '940345846', now()
WHERE EXISTS (SELECT 1 FROM master.instrument WHERE instrument_id = 20719)
  AND NOT EXISTS (SELECT 1 FROM master.instrument_identifier
                  WHERE instrument_id = 20719 AND scheme = 'ibkr_conid' AND value = '940345846' AND valid_to IS NULL);

UPDATE master.instrument_identifier
SET valid_to = now()
WHERE instrument_id = 4038 AND scheme = 'ibkr_conid' AND value = '935946396' AND valid_to IS NULL;

INSERT INTO master.instrument_identifier (instrument_id, scheme, value, valid_from)
SELECT 4038, 'ibkr_conid', '940466383', now()
WHERE EXISTS (SELECT 1 FROM master.instrument WHERE instrument_id = 4038)
  AND NOT EXISTS (SELECT 1 FROM master.instrument_identifier
                  WHERE instrument_id = 4038 AND scheme = 'ibkr_conid' AND value = '940466383' AND valid_to IS NULL);

-- +goose Down
-- A data repair is not undone.
