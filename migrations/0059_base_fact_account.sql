-- +goose Up
-- docs/design/base-data-page.md entry 5 (Checks table): the view of the rows account and flex of the base data page,
-- one row per (dataset, rule, subject). It adds a view and its grant and touches no table and no row; the page role
-- gets nothing on the account or optionledger schemas because the view reads the rd views and runs with the rights
-- of its owner.
--
-- Day is the New York date. account has the subject USD (calendar XNYS); the rules of the whole row have the
-- subject ''. The newest capture of an alias is the one with the newest captured_at. position_expired reads the
-- option positions of those captures; expiry is text, so when any of them is not an ISO date the fact is left out
-- (the check is unknown) rather than guessed: the gate is a HAVING on the aggregate, so no row comes out at all; an ISO date compares as text.
-- flex_section_absent is a setup note, never a rule: it counts the sections that version 2 wants and that the newest
-- fetch of each alias marks absent.
CREATE VIEW rd.base_fact_account_r1 AS
WITH today AS (
    SELECT (now() AT TIME ZONE 'America/New_York')::date AS day
),
newest_capture AS (
    SELECT DISTINCT ON (alias) alias, capture_id, session_day
    FROM rd.account_capture_r1
    ORDER BY alias, captured_at DESC, capture_id DESC
),
capture_rows AS (
    SELECT count(*) AS n, max(session_day) AS newest FROM rd.account_capture_r1
),
opt AS (
    SELECT p.expiry
    FROM rd.account_position_r1 p
    JOIN newest_capture c ON c.capture_id = p.capture_id
    WHERE p.sec_type = 'option'
),
flex_newest AS (
    SELECT DISTINCT ON (alias) alias, fetch_id
    FROM rd.flex_section_status_r1
    ORDER BY alias, recorded_at DESC, fetch_id DESC
),
flex_sections AS (
    SELECT s.section, n.alias,
           COALESCE(bool_or(st.state = 'absent'), false) AS absent
    FROM (VALUES ('OpenPosition'), ('EquitySummaryByReportDateInBase'), ('ConversionRate')) AS s (section)
    CROSS JOIN flex_newest n
    LEFT JOIN rd.flex_section_status_r1 st
      ON st.alias = n.alias AND st.fetch_id = n.fetch_id AND st.section = s.section
    GROUP BY s.section, n.alias
),
flex_rows AS (
    SELECT count(*) AS n, (max(recorded_at) AT TIME ZONE 'America/New_York')::date AS newest
    FROM rd.flex_trade_row_r1
)
-- rule account rows
SELECT 'account'::text AS dataset, 'rows'::text AS rule, 'USD'::text AS subject, n::bigint AS n, 0::bigint AS bad,
       NULL::date AS day, NULL::text AS value
FROM capture_rows
UNION ALL
-- rule account newest
SELECT 'account', 'newest', 'USD', 0::bigint, 0::bigint, r.newest, 'captures ' || r.n::text
FROM capture_rows r
WHERE r.newest IS NOT NULL
UNION ALL
-- rule account no_netliq
SELECT 'account', 'no_netliq', '', count(*)::bigint,
       (count(*) FILTER (WHERE NOT EXISTS (
           SELECT 1 FROM rd.account_value_r1 v
           WHERE v.capture_id = c.capture_id AND v.tag = 'NetLiquidation')))::bigint,
       NULL::date, NULL::text
FROM newest_capture c
UNION ALL
-- rule account position_expired
SELECT 'account', 'position_expired', '', count(*)::bigint,
       (count(*) FILTER (WHERE o.expiry < to_char(t.day, 'YYYY-MM-DD')))::bigint, NULL::date, NULL::text
FROM opt o
CROSS JOIN today t
HAVING NOT EXISTS (
    SELECT 1 FROM opt x
    WHERE x.expiry !~ '^[0-9]{4}-(0[1-9]|1[0-2])-(0[1-9]|[12][0-9]|3[01])$')
UNION ALL
-- rule flex rows
SELECT 'flex', 'rows', '', n::bigint, 0::bigint, NULL::date, NULL::text
FROM flex_rows
UNION ALL
-- rule flex newest
SELECT 'flex', 'newest', '', 0::bigint, 0::bigint, newest, 'rows ' || n::text
FROM flex_rows
WHERE newest IS NOT NULL
UNION ALL
-- note flex flex_section_absent
SELECT 'flex', 'flex_section_absent', '', count(*)::bigint, (count(*) FILTER (WHERE absent))::bigint,
       NULL::date, NULL::text
FROM flex_sections;

ALTER VIEW rd.base_fact_account_r1 OWNER TO ot_owner;

GRANT SELECT ON rd.base_fact_account_r1 TO ot_ro_pages;

-- +goose Down
REVOKE SELECT ON rd.base_fact_account_r1 FROM ot_ro_pages;
DROP VIEW rd.base_fact_account_r1;
