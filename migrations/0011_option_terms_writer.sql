-- +goose Up
-- The writer of the terms of an option: ResolveInstrument inserts the current row when it first creates the option, in
-- the transaction that inserts the instrument. INSERT, and SELECT on the two columns of the conflict target of that
-- insert (instrument_id, valid_to): PostgreSQL reads them for ON CONFLICT (instrument_id) WHERE valid_to IS NULL. The
-- role already has USAGE on schema master (0008).
GRANT INSERT, SELECT (instrument_id, valid_to) ON master.option_terms TO ot_w_master;

-- +goose Down
REVOKE INSERT, SELECT (instrument_id, valid_to) ON master.option_terms FROM ot_w_master;
