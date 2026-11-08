-- +goose Up
-- +goose NO TRANSACTION
-- obs.series_point r1 gains the unit index_points (VIX, VXN, RVX and SP500 levels are index points, not a fraction or
-- a percent). Additive; ALTER TYPE ... ADD VALUE cannot run inside a transaction block in older PostgreSQL, hence NO TRANSACTION.
ALTER TYPE obs.series_unit ADD VALUE IF NOT EXISTS 'index_points';

-- +goose Down
-- A value of an enum type cannot be dropped in PostgreSQL: index_points cannot be dropped, so this Down changes nothing.
