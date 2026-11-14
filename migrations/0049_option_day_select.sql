-- +goose Up
-- Run by ot_owner. The upsert of obs.option_day reads written_by through EXCLUDED, a column the column-level SELECT of 0047 omits.
GRANT SELECT ON obs.option_day TO ot_w_option_eod, ot_w_option_open;

-- +goose Down
REVOKE SELECT ON obs.option_day FROM ot_w_option_eod, ot_w_option_open;
