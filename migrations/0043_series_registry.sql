-- +goose Up
-- Run by ot_owner, which owns every object it changes.
-- Series registry (docs/design/series-registry.md, entry 2): obs.series_definition becomes the registry,
-- obs.series_point is keyed (series_id, observation_date) and a run replaces a differing value.
-- Owners and the grants of 0002 stay: the view is changed with CREATE OR REPLACE, never dropped.

-- 1. Registry columns (Decision 1); the defaults keep the old insert of the Writer working.
ALTER TABLE obs.series_definition
    ADD COLUMN title               text    NOT NULL DEFAULT '',
    ADD COLUMN category            text    NOT NULL DEFAULT '',
    ADD COLUMN frequency           text    NOT NULL DEFAULT '',
    ADD COLUMN source              text    NOT NULL DEFAULT 'fred',
    ADD COLUMN source_series_id    text    NOT NULL DEFAULT '',
    ADD COLUMN secondary_source    text    NOT NULL DEFAULT '',
    ADD COLUMN seasonal_adjustment text    NOT NULL DEFAULT '',
    ADD COLUMN first_observation   date,
    ADD COLUMN active              boolean NOT NULL DEFAULT true,
    ADD CONSTRAINT series_definition_category CHECK (
        category IN ('', 'rates', 'credit', 'liquidity', 'volatility', 'fx', 'activity')),
    ADD CONSTRAINT series_definition_frequency CHECK (frequency IN ('', 'daily', 'weekly', 'monthly', 'quarterly')),
    ADD CONSTRAINT series_definition_secondary CHECK (secondary_source IN ('', 'nyfed'));

-- 2. The 12 series of the job (Decision 1) and the four new ones (Decision 6).
UPDATE obs.series_definition d
SET title = v.title, category = v.category, frequency = v.frequency
FROM (VALUES
    ('DGS10',        '10-Year Treasury constant maturity rate',           'rates',      'daily'),
    ('DGS3MO',       '3-Month Treasury constant maturity rate',           'rates',      'daily'),
    ('SP500',        'S&P 500 index level',                               'activity',   'daily'),
    ('VIXCLS',       'CBOE Volatility Index (VIX)',                       'volatility', 'daily'),
    ('VXNCLS',       'CBOE NASDAQ 100 Volatility Index (VXN)',            'volatility', 'daily'),
    ('RVXCLS',       'CBOE Russell 2000 Volatility Index (RVX)',          'volatility', 'daily'),
    ('VXVCLS',       'CBOE S&P 500 3-Month Volatility Index (VXV)',       'volatility', 'daily'),
    ('WALCL',        'Federal Reserve total assets',                      'liquidity',  'weekly'),
    ('WDTGAL',       'Treasury General Account at the Federal Reserve',   'liquidity',  'weekly'),
    ('RRPONTSYD',    'Overnight reverse repurchase agreements',           'liquidity',  'daily'),
    ('DFF',          'Federal funds effective rate',                      'rates',      'daily'),
    ('BAMLH0A0HYM2', 'ICE BofA US High Yield option-adjusted spread',     'credit',     'daily')
) AS v (series_id, title, category, frequency)
WHERE d.series_id = v.series_id;

INSERT INTO obs.series_definition (series_id, unit, currency, title, category, frequency, secondary_source)
VALUES
    ('BAMLC0A0CM', 'percent',    '',    'ICE BofA US Corporate option-adjusted spread',      'credit',    'daily',  ''),
    ('BAA10Y',     'percent',    '',    'Moody''s Baa corporate yield minus the 10-Year',    'credit',    'daily',  ''),
    ('SOFR',       'percent',    '',    'Secured Overnight Financing Rate',                  'liquidity', 'daily',  'nyfed'),
    ('WRESBAL',    'ccy_amount', 'USD', 'Reserve balances at Federal Reserve Banks',         'liquidity', 'weekly', '')
ON CONFLICT (series_id) DO UPDATE
SET title = EXCLUDED.title, category = EXCLUDED.category, frequency = EXCLUDED.frequency,
    secondary_source = EXCLUDED.secondary_source;

-- 3. Keep only the newest realtime_start of each (series_id, observation_date) (Decision 2).
DELETE FROM obs.series_point p
USING obs.series_point q
WHERE p.series_id = q.series_id
  AND p.observation_date = q.observation_date
  AND p.realtime_start < q.realtime_start;

-- 4. The new key.
ALTER TABLE obs.series_point DROP CONSTRAINT series_point_pkey;
ALTER TABLE obs.series_point ADD PRIMARY KEY (series_id, observation_date);

-- 5. updated_at (set from published_at for the stored rows) and written_by.
ALTER TABLE obs.series_point
    ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now(),
    ADD COLUMN written_by text        NOT NULL DEFAULT '';
UPDATE obs.series_point SET updated_at = published_at;

-- 6. The view hides void rows (Decision 3): same eleven columns, owner and grants stay.
CREATE OR REPLACE VIEW rd.series_point_r1 AS
SELECT series_id, observation_date, realtime_start, realtime_end, value, status, unit, currency,
       published_at, source, source_revision
FROM obs.series_point
WHERE status <> 'void';

-- 7. Grants of the writer role (Decision 4): no DELETE, no TRUNCATE, no UPDATE of the key.
GRANT UPDATE (value, status, unit, currency, realtime_start, realtime_end, published_at, source, source_revision,
              updated_at, written_by) ON obs.series_point TO ot_w_series_fred;
GRANT SELECT (series_id, observation_date, value, status, unit, currency, realtime_start, realtime_end,
              published_at, source, source_revision) ON obs.series_point TO ot_w_series_fred;
GRANT SELECT ON obs.series_definition TO ot_w_series_fred;

-- +goose Down
-- Deleted vintages are not restored.
REVOKE SELECT ON obs.series_definition FROM ot_w_series_fred;
REVOKE SELECT (series_id, observation_date, value, status, unit, currency, realtime_start, realtime_end,
               published_at, source, source_revision) ON obs.series_point FROM ot_w_series_fred;
REVOKE UPDATE (value, status, unit, currency, realtime_start, realtime_end, published_at, source, source_revision,
               updated_at, written_by) ON obs.series_point FROM ot_w_series_fred;

CREATE OR REPLACE VIEW rd.series_point_r1 AS
SELECT series_id, observation_date, realtime_start, realtime_end, value, status, unit, currency,
       published_at, source, source_revision
FROM obs.series_point;

ALTER TABLE obs.series_point DROP COLUMN written_by, DROP COLUMN updated_at;
ALTER TABLE obs.series_point DROP CONSTRAINT series_point_pkey;
ALTER TABLE obs.series_point ADD PRIMARY KEY (series_id, observation_date, realtime_start);

DELETE FROM obs.series_definition d
WHERE d.series_id IN ('BAMLC0A0CM', 'BAA10Y', 'SOFR', 'WRESBAL')
  AND NOT EXISTS (SELECT 1 FROM obs.series_point p WHERE p.series_id = d.series_id);
ALTER TABLE obs.series_definition
    DROP CONSTRAINT series_definition_secondary,
    DROP CONSTRAINT series_definition_frequency,
    DROP CONSTRAINT series_definition_category,
    DROP COLUMN active, DROP COLUMN first_observation, DROP COLUMN seasonal_adjustment,
    DROP COLUMN secondary_source, DROP COLUMN source_series_id, DROP COLUMN source,
    DROP COLUMN frequency, DROP COLUMN category, DROP COLUMN title;
