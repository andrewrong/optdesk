-- +goose Up
-- Run by ot_owner. docs/design/source-rank.md entry 4: obs.fx_rate admits the fill source fred_h10 beside ibkr_midpoint.
-- Only fx_rate_source_ck changes; no grant, view or column.
ALTER TABLE obs.fx_rate DROP CONSTRAINT fx_rate_source_ck;
ALTER TABLE obs.fx_rate ADD CONSTRAINT fx_rate_source_ck CHECK (source IN ('ibkr_midpoint', 'fred_h10'));

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM obs.fx_rate WHERE source = 'fred_h10') THEN
        RAISE EXCEPTION 'obs.fx_rate holds fred_h10 rows; the old check would refuse them';
    END IF;
END
$$;
-- +goose StatementEnd
ALTER TABLE obs.fx_rate DROP CONSTRAINT fx_rate_source_ck;
ALTER TABLE obs.fx_rate ADD CONSTRAINT fx_rate_source_ck CHECK (source = 'ibkr_midpoint');
