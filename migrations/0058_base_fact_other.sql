-- +goose Up
-- docs/design/base-data-page.md entry 5 (Checks table): the view of the six rows contracts, rates, fx, events,
-- calendar and facts of the base data page, one row per (dataset, rule, subject). It adds a view and its grant and
-- touches no table and no row. It reads the rd views and obs.series_definition and obs.macro_event, which the owner
-- role that runs the view may read; the page role gets nothing on them.
--
-- Day is the New York date. The rules of the whole row have the subject ''; fx has the subject USDJPY. The windows the
-- design leaves open are: fx_band and fx_jump look at the last 45 calendar days as the bars rules do, so an old
-- value that cannot be repaired does not keep the row red; series_age counts the active series that have a
-- frequency (a series with the empty frequency has no limit to judge by). The newest day of calendar is the newest
-- stored session day not after Day; its horizon is judged by calendar_horizon.
CREATE VIEW rd.base_fact_other_r1 AS
WITH today AS (
    SELECT (now() AT TIME ZONE 'America/New_York')::date AS day
),
terms AS (
    SELECT (max(valid_from) AT TIME ZONE 'America/New_York')::date AS newest, count(*) AS n
    FROM rd.option_terms_r1
),
series AS (
    SELECT d.series_id, d.frequency,
           (SELECT max(p.observation_date) FROM rd.series_point_r1 p WHERE p.series_id = d.series_id) AS newest
    FROM obs.series_definition d
    WHERE d.active
),
series_newest AS (
    SELECT p.series_id, p.observation_date, p.value
    FROM rd.series_point_r1 p
    JOIN obs.series_definition d ON d.series_id = p.series_id AND d.active
    ORDER BY p.observation_date DESC, p.series_id
    LIMIT 1
),
fx AS (
    SELECT f.pair, f.rate_day, f.rate::numeric AS rate, f.rate AS rate_text,
           lag(f.rate::numeric) OVER (PARTITION BY f.pair ORDER BY f.rate_day) AS prev_rate
    FROM rd.fx_rate_r1 f
    WHERE f.pair = 'USDJPY'
),
fx_window AS (
    SELECT x.* FROM fx x CROSS JOIN today t WHERE x.rate_day >= t.day - 45
),
fx_newest AS (
    SELECT DISTINCT ON (pair) pair, rate_day, rate_text
    FROM fx
    ORDER BY pair, rate_day DESC
),
earn AS (
    SELECT DISTINCT ON (issuer) issuer, status, report_date, observed_on
    FROM rd.earnings_date_r1
    ORDER BY issuer, observed_on DESC, attempt DESC
),
divs AS (
    SELECT DISTINCT ON (instrument_id) instrument_id, status, next_ex_date
    FROM rd.dividend_expectation_r1
    ORDER BY instrument_id, session_day DESC, attempt DESC
),
event_rows AS (
    SELECT (SELECT count(*) FROM rd.earnings_date_r1) + (SELECT count(*) FROM rd.dividend_expectation_r1)
           + (SELECT count(*) FROM obs.macro_event) AS n,
           (SELECT max(observed_on) FROM rd.earnings_date_r1) AS newest
),
macro AS (
    SELECT kind, max(event_date) AS last_date
    FROM obs.macro_event
    GROUP BY kind
),
cal AS (
    SELECT c.calendar_id, max(s.session_day) AS last_day
    FROM (VALUES ('XNYS'), ('XTKS')) AS c (calendar_id)
    LEFT JOIN rd.session_day_r1 s ON s.calendar_id = c.calendar_id
    GROUP BY c.calendar_id
),
cal_rows AS (
    SELECT count(*) AS n,
           max(session_day) FILTER (WHERE is_session AND session_day <= (SELECT day FROM today)) AS newest
    FROM rd.session_day_r1
),
subj AS (
    SELECT subject, max(filed) AS newest
    FROM rd.fundamental_fact_r2
    GROUP BY subject
),
fact_rows AS (
    SELECT count(*) AS n,
           (count(*) FILTER (WHERE period_start IS NOT NULL))::bigint AS dated,
           (count(*) FILTER (WHERE period_start IS NOT NULL AND period_end < period_start))::bigint AS bad_order,
           max(filed) AS newest,
           (array_agg(subject ORDER BY filed DESC NULLS LAST, subject))[1] AS subject
    FROM rd.fundamental_fact_r2
)
-- rule contracts rows
SELECT 'contracts'::text AS dataset, 'rows'::text AS rule, ''::text AS subject, n::bigint AS n, 0::bigint AS bad,
       NULL::date AS day, NULL::text AS value
FROM terms
UNION ALL
-- rule contracts newest
SELECT 'contracts', 'newest', '', 0::bigint, 0::bigint, newest, 'terms ' || n::text
FROM terms
WHERE newest IS NOT NULL
UNION ALL
-- rule rates rows
SELECT 'rates', 'rows', '', count(*)::bigint, 0::bigint, NULL::date, NULL::text
FROM rd.series_point_r1 p
JOIN obs.series_definition d ON d.series_id = p.series_id AND d.active
UNION ALL
-- rule rates newest
SELECT 'rates', 'newest', '', 0::bigint, 0::bigint, observation_date, series_id || ' ' || value
FROM series_newest
UNION ALL
-- rule rates series_age
SELECT 'rates', 'series_age', '', count(*)::bigint,
       (count(*) FILTER (WHERE s.newest IS NULL OR s.newest < t.day - CASE s.frequency
            WHEN 'daily' THEN 4 WHEN 'weekly' THEN 14 WHEN 'monthly' THEN 45 ELSE 120 END))::bigint,
       NULL::date, NULL::text
FROM series s
CROSS JOIN today t
WHERE s.frequency IN ('daily', 'weekly', 'monthly', 'quarterly')
UNION ALL
-- rule fx rows
SELECT 'fx', 'rows', 'USDJPY', (SELECT count(*) FROM fx)::bigint, 0::bigint, NULL::date, NULL::text
UNION ALL
-- rule fx newest
SELECT 'fx', 'newest', pair, 0::bigint, 0::bigint, rate_day, pair || ' ' || rate_text
FROM fx_newest
UNION ALL
-- rule fx fx_band
SELECT 'fx', 'fx_band', '', count(*)::bigint, (count(*) FILTER (WHERE rate < 80 OR rate > 250))::bigint,
       NULL::date, NULL::text
FROM fx_window
UNION ALL
-- rule fx fx_jump
SELECT 'fx', 'fx_jump', '', (count(*) FILTER (WHERE prev_rate IS NOT NULL))::bigint,
       (count(*) FILTER (WHERE prev_rate IS NOT NULL AND abs(rate / prev_rate - 1) > 0.05))::bigint,
       NULL::date, NULL::text
FROM fx_window
UNION ALL
-- rule events rows
SELECT 'events', 'rows', '', n::bigint, 0::bigint, NULL::date, NULL::text
FROM event_rows
UNION ALL
-- rule events newest
SELECT 'events', 'newest', '', 0::bigint, 0::bigint, newest, 'rows ' || n::text
FROM event_rows
WHERE newest IS NOT NULL
UNION ALL
-- rule events earnings_past
SELECT 'events', 'earnings_past', '', (count(*) FILTER (WHERE e.status = 'listed'))::bigint,
       (count(*) FILTER (WHERE e.status = 'listed' AND e.report_date < t.day))::bigint, NULL::date, NULL::text
FROM earn e
CROSS JOIN today t
UNION ALL
-- rule events dividend_past
SELECT 'events', 'dividend_past', '', (count(*) FILTER (WHERE v.status = 'present'))::bigint,
       (count(*) FILTER (WHERE v.status = 'present' AND v.next_ex_date < t.day))::bigint, NULL::date, NULL::text
FROM divs v
CROSS JOIN today t
UNION ALL
-- rule events macro_exhausted
SELECT 'events', 'macro_exhausted', '', count(*)::bigint, (count(*) FILTER (WHERE m.last_date < t.day))::bigint,
       NULL::date, NULL::text
FROM macro m
CROSS JOIN today t
UNION ALL
-- rule calendar rows
SELECT 'calendar', 'rows', '', n::bigint, 0::bigint, NULL::date, NULL::text
FROM cal_rows
UNION ALL
-- rule calendar newest
SELECT 'calendar', 'newest', '', 0::bigint, 0::bigint, newest, 'rows ' || n::text
FROM cal_rows
WHERE newest IS NOT NULL
UNION ALL
-- rule calendar calendar_horizon
SELECT 'calendar', 'calendar_horizon', '', count(*)::bigint,
       (count(*) FILTER (WHERE c.last_day IS NULL OR c.last_day < t.day + 30))::bigint, NULL::date, NULL::text
FROM cal c
CROSS JOIN today t
UNION ALL
-- rule facts rows
SELECT 'facts', 'rows', '', n::bigint, 0::bigint, NULL::date, NULL::text
FROM fact_rows
UNION ALL
-- rule facts newest
SELECT 'facts', 'newest', '', 0::bigint, 0::bigint, newest, subject
FROM fact_rows
WHERE newest IS NOT NULL
UNION ALL
-- rule facts period_order
SELECT 'facts', 'period_order', '', dated, bad_order, NULL::date, NULL::text
FROM fact_rows
UNION ALL
-- rule facts subject_stale
SELECT 'facts', 'subject_stale', '', count(*)::bigint, (count(*) FILTER (WHERE s.newest < t.day - 135))::bigint,
       NULL::date, NULL::text
FROM subj s
CROSS JOIN today t;

ALTER VIEW rd.base_fact_other_r1 OWNER TO ot_owner;

GRANT SELECT ON rd.base_fact_other_r1 TO ot_ro_pages;

-- +goose Down
REVOKE SELECT ON rd.base_fact_other_r1 FROM ot_ro_pages;
DROP VIEW rd.base_fact_other_r1;
