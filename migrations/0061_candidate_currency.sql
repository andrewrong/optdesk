-- +goose Up
-- docs/design/candidates-jp.md Decision 19: candidates.option_candidate gains a nullable currency, the currency the
-- amounts of the row are in. Null reads as USD: no existing row changes. The table stays insert-only.
-- CREATE OR REPLACE VIEW keeps the owner and the privileges of the view, and the privileges of the table are per
-- table, so no role or privilege is added here. currency is appended last, as 0027 appended settings_version.
ALTER TABLE candidates.option_candidate ADD COLUMN currency text;

CREATE OR REPLACE VIEW rd.option_candidate_r1 AS
SELECT session_day, run_id, kind, instrument_id, alias, rule_version, issuer, underlying_id, expiry, dte,
       option_right, strike, multiplier, bid_eod, ask, spot, buy_price, sale_price, break_even, annualized_yield,
       delta, iv, status, gates, marks, fits, next_earnings, earnings_state, next_ex_date, ex_amount, extrinsic,
       rank, rank_in_underlying, computed_at, settings_version, currency
FROM candidates.option_candidate;

-- +goose Down
-- A view cannot lose a column through CREATE OR REPLACE, so the view is dropped and made again as 0027 made it.
-- The new view is owned by the role that runs this and holds none of the privileges the old one had: the owner
-- sets the owner and the privileges of 0018 again after a Down.
DROP VIEW rd.option_candidate_r1;

CREATE VIEW rd.option_candidate_r1 AS
SELECT session_day, run_id, kind, instrument_id, alias, rule_version, issuer, underlying_id, expiry, dte,
       option_right, strike, multiplier, bid_eod, ask, spot, buy_price, sale_price, break_even, annualized_yield,
       delta, iv, status, gates, marks, fits, next_earnings, earnings_state, next_ex_date, ex_amount, extrinsic,
       rank, rank_in_underlying, computed_at, settings_version
FROM candidates.option_candidate;

ALTER TABLE candidates.option_candidate DROP COLUMN currency;
