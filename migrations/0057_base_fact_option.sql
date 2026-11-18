-- +goose Up
-- docs/design/base-data-page.md entry 5 (Decision 7, Checks table): the view of the option rows of the base data page
-- (quotes, chains, open), one row per (dataset, rule, subject). It adds a view and its grant and touches no table and
-- no row.
--
-- One row per contract (Decision 7): of the newest session_day of each currency (the subject, USD or JPY) the picked
-- row of an instrument_id is the newest received_at among its rows of status 1 (DISTINCT ON). A contract with no
-- status-1 row that day is classified by its newest row: no quote is `empty`, an error is `errors` (both with all
-- contracts of the day as N); not listed, stale and partial are the fact `not_judged`, a number on the row that is in
-- no verdict. The status lists: chains and quotes 1 ok, 2 no quote, 3 not listed, 4 stale, 5 error; open 1 ok,
-- 2 partial, 3 no_data, 4 not_listed, 5 no_subscription, 6 deadline, 7 deferred, 8 error, 9 disconnected.
-- non_live is the share of picked rows whose market_data_type is not 1 (live). Rules of the whole row have the
-- subject ''.
CREATE VIEW rd.base_fact_option_r1 AS
WITH ccy (currency) AS (
    VALUES ('USD'), ('JPY')
),
q_day AS (
    SELECT currency::text AS currency, max(session_day) AS d
    FROM rd.quote_snapshot_r1
    WHERE currency::text IN ('USD', 'JPY')
    GROUP BY currency::text
),
q_rows AS (
    SELECT t.instrument_id, t.currency::text AS currency, t.status, t.received_at, t.market_data_type, t.bid, t.ask
    FROM rd.quote_snapshot_r1 t
    JOIN q_day d ON d.currency = t.currency::text AND d.d = t.session_day
),
q_pick AS (
    SELECT DISTINCT ON (instrument_id) *
    FROM q_rows
    WHERE status = 1
    ORDER BY instrument_id, received_at DESC
),
q_rest AS (
    SELECT DISTINCT ON (r.instrument_id) r.instrument_id, r.status
    FROM q_rows r
    WHERE NOT EXISTS (SELECT 1 FROM q_pick k WHERE k.instrument_id = r.instrument_id)
    ORDER BY r.instrument_id, r.received_at DESC
),
q_class AS (
    SELECT (SELECT count(*) FROM q_pick) + count(*) AS total,
           (count(*) FILTER (WHERE status IN (2)))::bigint AS empty,
           (count(*) FILTER (WHERE status IN (5)))::bigint AS errors,
           (count(*) FILTER (WHERE status IN (3, 4)))::bigint AS not_judged
    FROM q_rest
),
c_day AS (
    SELECT currency::text AS currency, max(session_day) AS d
    FROM rd.option_eod_r1
    WHERE currency::text IN ('USD', 'JPY')
    GROUP BY currency::text
),
c_rows AS (
    SELECT t.instrument_id, t.currency::text AS currency, t.status, t.received_at, t.market_data_type, t.bid, t.ask,
           t.iv_decimal
    FROM rd.option_eod_r1 t
    JOIN c_day d ON d.currency = t.currency::text AND d.d = t.session_day
),
c_pick AS (
    SELECT DISTINCT ON (instrument_id) *
    FROM c_rows
    WHERE status = 1
    ORDER BY instrument_id, received_at DESC
),
c_rest AS (
    SELECT DISTINCT ON (r.instrument_id) r.instrument_id, r.status
    FROM c_rows r
    WHERE NOT EXISTS (SELECT 1 FROM c_pick k WHERE k.instrument_id = r.instrument_id)
    ORDER BY r.instrument_id, r.received_at DESC
),
c_class AS (
    SELECT (SELECT count(*) FROM c_pick) + count(*) AS total,
           (count(*) FILTER (WHERE status IN (2)))::bigint AS empty,
           (count(*) FILTER (WHERE status IN (5)))::bigint AS errors,
           (count(*) FILTER (WHERE status IN (3, 4)))::bigint AS not_judged
    FROM c_rest
),
o_day AS (
    SELECT currency::text AS currency, max(session_day) AS d
    FROM rd.option_open_r1
    WHERE currency::text IN ('USD', 'JPY')
    GROUP BY currency::text
),
o_rows AS (
    SELECT t.instrument_id, t.currency::text AS currency, t.status, t.received_at, t.market_data_type, t.bid, t.ask,
           t.iv_decimal
    FROM rd.option_open_r1 t
    JOIN o_day d ON d.currency = t.currency::text AND d.d = t.session_day
),
o_pick AS (
    SELECT DISTINCT ON (instrument_id) *
    FROM o_rows
    WHERE status = 1
    ORDER BY instrument_id, received_at DESC
),
o_rest AS (
    SELECT DISTINCT ON (r.instrument_id) r.instrument_id, r.status
    FROM o_rows r
    WHERE NOT EXISTS (SELECT 1 FROM o_pick k WHERE k.instrument_id = r.instrument_id)
    ORDER BY r.instrument_id, r.received_at DESC
),
o_class AS (
    SELECT (SELECT count(*) FROM o_pick) + count(*) AS total,
           (count(*) FILTER (WHERE status IN (3)))::bigint AS empty,
           (count(*) FILTER (WHERE status IN (5, 6, 7, 8, 9)))::bigint AS errors,
           (count(*) FILTER (WHERE status IN (2, 4)))::bigint AS not_judged
    FROM o_rest
)
-- rule quotes rows
SELECT 'quotes'::text AS dataset, 'rows'::text AS rule, c.currency AS subject,
       count(k.instrument_id)::bigint AS n, 0::bigint AS bad, NULL::date AS day, NULL::text AS value
FROM ccy c
LEFT JOIN q_pick k ON k.currency = c.currency
GROUP BY c.currency
UNION ALL
-- rule quotes newest
SELECT 'quotes', 'newest', d.currency, 0::bigint, 0::bigint, d.d,
       'rows ' || to_char((SELECT count(*) FROM q_pick k WHERE k.currency = d.currency), 'FM999999999990')
FROM q_day d
UNION ALL
-- rule quotes crossed
SELECT 'quotes', 'crossed', '', count(*)::bigint,
       (count(*) FILTER (WHERE bid > ask AND bid > 0 AND ask > 0))::bigint, NULL::date, NULL::text
FROM q_pick
UNION ALL
-- rule quotes empty
SELECT 'quotes', 'empty', '', total::bigint, empty, NULL::date, NULL::text
FROM q_class
UNION ALL
-- rule quotes errors
SELECT 'quotes', 'errors', '', total::bigint, errors, NULL::date, NULL::text
FROM q_class
UNION ALL
-- rule quotes non_live
SELECT 'quotes', 'non_live', '', count(*)::bigint, (count(*) FILTER (WHERE market_data_type <> 1))::bigint,
       NULL::date, NULL::text
FROM q_pick
UNION ALL
-- fact quotes not_judged (a number shown on the row, in no verdict)
SELECT 'quotes', 'not_judged', '', total::bigint, not_judged, NULL::date, NULL::text
FROM q_class
UNION ALL
-- rule chains rows
SELECT 'chains'::text AS dataset, 'rows'::text AS rule, c.currency AS subject,
       count(k.instrument_id)::bigint AS n, 0::bigint AS bad, NULL::date AS day, NULL::text AS value
FROM ccy c
LEFT JOIN c_pick k ON k.currency = c.currency
GROUP BY c.currency
UNION ALL
-- rule chains newest
SELECT 'chains', 'newest', d.currency, 0::bigint, 0::bigint, d.d,
       'rows ' || to_char((SELECT count(*) FROM c_pick k WHERE k.currency = d.currency), 'FM999999999990')
FROM c_day d
UNION ALL
-- rule chains crossed
SELECT 'chains', 'crossed', '', count(*)::bigint,
       (count(*) FILTER (WHERE bid > ask AND bid > 0 AND ask > 0))::bigint, NULL::date, NULL::text
FROM c_pick
UNION ALL
-- rule chains bad_iv
SELECT 'chains', 'bad_iv', '', count(*)::bigint,
       (count(*) FILTER (WHERE iv_decimal IS NOT NULL AND (iv_decimal <= 0 OR iv_decimal > 5)))::bigint,
       NULL::date, NULL::text
FROM c_pick
UNION ALL
-- rule chains empty
SELECT 'chains', 'empty', '', total::bigint, empty, NULL::date, NULL::text
FROM c_class
UNION ALL
-- rule chains errors
SELECT 'chains', 'errors', '', total::bigint, errors, NULL::date, NULL::text
FROM c_class
UNION ALL
-- rule chains non_live
SELECT 'chains', 'non_live', '', count(*)::bigint, (count(*) FILTER (WHERE market_data_type <> 1))::bigint,
       NULL::date, NULL::text
FROM c_pick
UNION ALL
-- fact chains not_judged (a number shown on the row, in no verdict)
SELECT 'chains', 'not_judged', '', total::bigint, not_judged, NULL::date, NULL::text
FROM c_class
UNION ALL
-- rule open rows
SELECT 'open'::text AS dataset, 'rows'::text AS rule, c.currency AS subject,
       count(k.instrument_id)::bigint AS n, 0::bigint AS bad, NULL::date AS day, NULL::text AS value
FROM ccy c
LEFT JOIN o_pick k ON k.currency = c.currency
GROUP BY c.currency
UNION ALL
-- rule open newest
SELECT 'open', 'newest', d.currency, 0::bigint, 0::bigint, d.d,
       'rows ' || to_char((SELECT count(*) FROM o_pick k WHERE k.currency = d.currency), 'FM999999999990')
FROM o_day d
UNION ALL
-- rule open crossed
SELECT 'open', 'crossed', '', count(*)::bigint,
       (count(*) FILTER (WHERE bid > ask AND bid > 0 AND ask > 0))::bigint, NULL::date, NULL::text
FROM o_pick
UNION ALL
-- rule open bad_iv
SELECT 'open', 'bad_iv', '', count(*)::bigint,
       (count(*) FILTER (WHERE iv_decimal IS NOT NULL AND (iv_decimal <= 0 OR iv_decimal > 5)))::bigint,
       NULL::date, NULL::text
FROM o_pick
UNION ALL
-- rule open empty
SELECT 'open', 'empty', '', total::bigint, empty, NULL::date, NULL::text
FROM o_class
UNION ALL
-- rule open errors
SELECT 'open', 'errors', '', total::bigint, errors, NULL::date, NULL::text
FROM o_class
UNION ALL
-- rule open non_live
SELECT 'open', 'non_live', '', count(*)::bigint, (count(*) FILTER (WHERE market_data_type <> 1))::bigint,
       NULL::date, NULL::text
FROM o_pick
UNION ALL
-- fact open not_judged (a number shown on the row, in no verdict)
SELECT 'open', 'not_judged', '', total::bigint, not_judged, NULL::date, NULL::text
FROM o_class;

ALTER VIEW rd.base_fact_option_r1 OWNER TO ot_owner;

GRANT SELECT ON rd.base_fact_option_r1 TO ot_ro_pages;

-- +goose Down
REVOKE SELECT ON rd.base_fact_option_r1 FROM ot_ro_pages;
DROP VIEW rd.base_fact_option_r1;
