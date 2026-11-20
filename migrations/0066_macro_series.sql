-- +goose Up
-- +goose NO TRANSACTION
-- docs/design/macro-page-v2.md Decision 4: obs.series_definition rows for the seven series the macro page needs.
-- A row that exists is not touched. DEXJPUS is quoted in jpy_per_usd, which the enum obs.series_unit lacks, so the value
-- is added first (ALTER TYPE ... ADD VALUE needs NO TRANSACTION, as in 0014); without it the insert below would fail.
ALTER TYPE obs.series_unit ADD VALUE IF NOT EXISTS 'jpy_per_usd';

INSERT INTO obs.series_definition (series_id, unit, currency, title, category, frequency)
VALUES
    ('DGS2',            'percent',      '', '2-Year Treasury constant maturity rate',               'rates', 'daily'),
    ('T10Y2Y',          'percent',      '', '10-Year minus 2-Year Treasury constant maturity',      'rates', 'daily'),
    ('T10Y3M',          'percent',      '', '10-Year minus 3-Month Treasury constant maturity',     'rates', 'daily'),
    ('T10YIE',          'percent',      '', '10-Year breakeven inflation rate',                     'rates', 'daily'),
    ('DTWEXBGS',        'index_points', '', 'Nominal broad US dollar index',                        'fx',    'daily'),
    ('DEXJPUS',         'jpy_per_usd',  '', 'Japanese yen to one US dollar exchange rate',          'fx',    'daily'),
    ('IRLTLT01JPM156N', 'percent',      '', 'Japan 10-year government bond yield (OECD)',           'rates', 'monthly')
ON CONFLICT (series_id) DO NOTHING;

-- +goose Down
-- Only the seven rows of this file go; the enum value jpy_per_usd cannot be dropped in PostgreSQL and stays.
-- A row that already has stored points is kept, as the Down of 0043 keeps it: series_point references the definition.
DELETE FROM obs.series_definition d
WHERE d.series_id IN ('DGS2', 'T10Y2Y', 'T10Y3M', 'T10YIE', 'DTWEXBGS', 'DEXJPUS', 'IRLTLT01JPM156N')
  AND NOT EXISTS (SELECT 1 FROM obs.series_point p WHERE p.series_id = d.series_id);
