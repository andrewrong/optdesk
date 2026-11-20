-- +goose Up
-- docs/design/bars-fallback.md Decision 6: the store holds yfinance bars next to ibkr bars. The upsert never lets a
-- yfinance row replace an ibkr row. No grant changes.
ALTER TABLE obs.price_bar DROP CONSTRAINT price_bar_source_ck;
ALTER TABLE obs.price_bar ADD CONSTRAINT price_bar_source_ck CHECK (source IN ('ibkr', 'yfinance'));

-- +goose Down
-- Restoring the old check fails while a yfinance row exists: that row would break it, and none is deleted here.
ALTER TABLE obs.price_bar DROP CONSTRAINT price_bar_source_ck;
ALTER TABLE obs.price_bar ADD CONSTRAINT price_bar_source_ck CHECK (source = 'ibkr');
