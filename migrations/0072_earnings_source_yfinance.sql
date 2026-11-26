-- +goose Up
-- docs/design/earnings-fallback.md entry 2: the store holds yfinance earnings dates next to the others, and a check row
-- names yfinance when it was the source asked. Only the two source checks change; the unsupported checks stay.
ALTER TABLE obs.earnings_date DROP CONSTRAINT earnings_date_source_ck;
ALTER TABLE obs.earnings_date ADD CONSTRAINT earnings_date_source_ck CHECK (source IN ('alphavantage', 'jquants', 'yfinance', 'owner', 'none'));
ALTER TABLE obs.earnings_check DROP CONSTRAINT earnings_check_source_ck;
ALTER TABLE obs.earnings_check ADD CONSTRAINT earnings_check_source_ck CHECK (source IN ('alphavantage', 'jquants', 'yfinance', 'owner', 'none'));

-- +goose Down
-- Restoring the old checks fails while a yfinance row exists: that row would break it, and none is deleted here.
ALTER TABLE obs.earnings_date DROP CONSTRAINT earnings_date_source_ck;
ALTER TABLE obs.earnings_date ADD CONSTRAINT earnings_date_source_ck CHECK (source IN ('alphavantage', 'jquants', 'owner', 'none'));
ALTER TABLE obs.earnings_check DROP CONSTRAINT earnings_check_source_ck;
ALTER TABLE obs.earnings_check ADD CONSTRAINT earnings_check_source_ck CHECK (source IN ('alphavantage', 'jquants', 'owner', 'none'));
