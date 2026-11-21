-- +goose Up
-- docs/design/macro-event-values.md Decision 2 and 11: obs.series_definition rows for the nine FRED series the /macro event
-- table reads its previous and actual values from. WALCL is already registered (0043) and is not inserted here.
-- A row that exists is not touched. Units are in the enum already (percent, index_points, count, ccy_amount); categories are
-- among those series_definition_category accepts (the release figures are 'activity', the fed funds target limits 'rates').
INSERT INTO obs.series_definition (series_id, unit, currency, title, category, frequency)
VALUES
    ('CPIAUCNS',        'index_points', '',    'Consumer Price Index for All Urban Consumers: All Items (NSA)',            'activity', 'monthly'),
    ('PAYEMS',          'count',        '',    'All employees, total nonfarm payrolls',                                    'activity', 'monthly'),
    ('PCEPI',           'index_points', '',    'Personal Consumption Expenditures: Chain-type Price Index',                'activity', 'monthly'),
    ('A191RL1Q225SBEA', 'percent',      '',    'Real gross domestic product, percent change from preceding period',        'activity', 'quarterly'),
    ('PPIFID',          'index_points', '',    'Producer Price Index by Commodity: Final Demand (NSA)',                    'activity', 'monthly'),
    ('RSAFS',           'ccy_amount',   'USD', 'Advance retail sales: retail trade and food services',                     'activity', 'monthly'),
    ('JTSJOL',          'count',        '',    'Job openings: total nonfarm',                                              'activity', 'monthly'),
    ('DFEDTARU',        'percent',      '',    'Federal funds target range, upper limit',                                  'rates',    'daily'),
    ('DFEDTARL',        'percent',      '',    'Federal funds target range, lower limit',                                  'rates',    'daily')
ON CONFLICT (series_id) DO NOTHING;

-- +goose Down
-- Only the nine rows of this file go. A row that already has stored points is kept, as the Down of 0066 keeps it:
-- series_point references the definition.
DELETE FROM obs.series_definition d
WHERE d.series_id IN ('CPIAUCNS', 'PAYEMS', 'PCEPI', 'A191RL1Q225SBEA', 'PPIFID', 'RSAFS', 'JTSJOL', 'DFEDTARU', 'DFEDTARL')
  AND NOT EXISTS (SELECT 1 FROM obs.series_point p WHERE p.series_id = d.series_id);
