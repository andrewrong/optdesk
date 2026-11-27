-- +goose Up
-- docs/design/data-check-2.md entry 2 (Decisions 5, 7): the data check reads the earnings check rows and the price bars through
-- their views to count listed names and malformed bars. Read only; the role gains no table.
GRANT SELECT ON rd.price_bar_r1, rd.earnings_date_r1 TO ot_w_datacheck;

-- +goose Down
REVOKE SELECT ON rd.price_bar_r1, rd.earnings_date_r1 FROM ot_w_datacheck;
