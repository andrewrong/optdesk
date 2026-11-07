-- +goose Up
-- The writer of the terms of an option: ResolveInstrument inserts the current row when it first creates the option, in
-- the transaction that inserts the instrument. INSERT only; the role already has USAGE on schema master (0008).
GRANT INSERT ON master.option_terms TO ot_w_master;

-- +goose Down
REVOKE INSERT ON master.option_terms FROM ot_w_master;
