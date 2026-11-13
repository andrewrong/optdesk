-- +goose Up
-- docs/design/option-field-reasons.md entry 4 (Decision 2): ops.option_run_reason also holds the codes field_missing
-- and fields_complete. Only the check widens; the view and the grants of 0036 are unchanged.
ALTER TABLE ops.option_run_reason DROP CONSTRAINT option_run_reason_code_ck;
ALTER TABLE ops.option_run_reason ADD CONSTRAINT option_run_reason_code_ck CHECK (code IN ('no_con_id', 'no_close',
    'no_chain', 'no_list', 'no_stock_list', 'fetch_failed', 'no_spot', 'late_start', 'held_stale', 'outside_range',
    'not_wired', 'field_missing', 'fields_complete'));

-- +goose Down
-- Rows with the two new codes must be gone before this applies.
ALTER TABLE ops.option_run_reason DROP CONSTRAINT option_run_reason_code_ck;
ALTER TABLE ops.option_run_reason ADD CONSTRAINT option_run_reason_code_ck CHECK (code IN ('no_con_id', 'no_close',
    'no_chain', 'no_list', 'no_stock_list', 'fetch_failed', 'no_spot', 'late_start', 'held_stale', 'outside_range',
    'not_wired'));
