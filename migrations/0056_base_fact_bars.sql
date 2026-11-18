-- +goose Up
-- docs/design/base-data-page.md entry 5 (Decision 3, Checks table): the view of the bars row of the base data page,
-- one row per (dataset, rule, subject). It adds a view and its grant and touches no table and no row; the page role
-- gets nothing on master.instrument because the view runs with the rights of its owner.
--
-- Window: the rows of rd.price_bar_r1 of basis trades of the last 45 calendar days of the New York date. The keys of
-- obs.price_bar are not led by the day, so this scans the window once; the bound is instruments x sessions x bases.
-- Subjects are the currencies USD and JPY (calendars XNYS and XTKS); the rules of the whole row have the subject ''.
-- No rule bar_gap: the ledger rules already say which keys are missing (Checks, generic rules).
CREATE VIEW rd.base_fact_bars_r1 AS
WITH today AS (
    SELECT (now() AT TIME ZONE 'America/New_York')::date AS day
),
ccy (currency) AS (
    VALUES ('USD'), ('JPY')
),
bars AS (
    SELECT b.instrument_id, b.session_day, i.symbol, i.currency::text AS currency,
           b.open::numeric AS open, b.high::numeric AS high, b.low::numeric AS low, b.close::numeric AS close,
           lag(b.close::numeric) OVER (PARTITION BY b.instrument_id ORDER BY b.session_day) AS prev_close
    FROM rd.price_bar_r1 b
    JOIN master.instrument i ON i.instrument_id = b.instrument_id
    CROSS JOIN today t
    WHERE b.basis = 'trades'
      AND b.session_day >= t.day - 45
      AND i.currency::text IN ('USD', 'JPY')
),
newest AS (
    SELECT DISTINCT ON (currency) currency, session_day, symbol, close
    FROM bars
    ORDER BY currency, session_day DESC, symbol, instrument_id
)
-- rule bars rows
SELECT 'bars'::text AS dataset, 'rows'::text AS rule, c.currency AS subject,
       count(b.instrument_id)::bigint AS n, 0::bigint AS bad, NULL::date AS day, NULL::text AS value
FROM ccy c
LEFT JOIN bars b ON b.currency = c.currency
GROUP BY c.currency
UNION ALL
-- rule bars newest
SELECT 'bars', 'newest', n.currency, 0::bigint, 0::bigint, n.session_day, n.symbol || ' close ' || n.close::text
FROM newest n
UNION ALL
-- rule bars ohlc_order
SELECT 'bars', 'ohlc_order', '', count(*)::bigint,
       (count(*) FILTER (WHERE high < low OR high < greatest(open, close) OR low > least(open, close)))::bigint,
       NULL::date, NULL::text
FROM bars
UNION ALL
-- rule bars bar_jump
SELECT 'bars', 'bar_jump', '', (count(*) FILTER (WHERE prev_close IS NOT NULL))::bigint,
       (count(*) FILTER (WHERE prev_close IS NOT NULL AND abs(close / prev_close - 1) > 0.4))::bigint,
       NULL::date, NULL::text
FROM bars;

ALTER VIEW rd.base_fact_bars_r1 OWNER TO ot_owner;

GRANT SELECT ON rd.base_fact_bars_r1 TO ot_ro_pages;

-- +goose Down
REVOKE SELECT ON rd.base_fact_bars_r1 FROM ot_ro_pages;
DROP VIEW rd.base_fact_bars_r1;
