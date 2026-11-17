-- +goose Up
-- docs/design/data-health.md entry 3 (Decisions 1 and 15): the one view the data health page reads for the ledger,
-- one row per (session day, dataset). It adds a view and its grant and touches no table and no row; the page role
-- gets nothing on an ops table because the view runs with the rights of its owner.
--
-- A key is expected by ops.capture_expected_key. It is `ok` when ops.capture_present holds it as 'present' and
-- `missing` when that table has no row for it. A key marked 'no_value' is `expired` when the newest
-- ops.capture_run row of the key is not a clean finish (status not 'done', an error code, or the reason 'expired'
-- or 'crashed_repeatedly'), else `no_value`. The newest run is the one finished last (a row still running first).
CREATE VIEW rd.coverage_day_r1 AS
WITH expected AS (
    SELECT DISTINCT session_day, dataset_id, subject_key
    FROM ops.capture_expected_key
),
newest_run AS (
    SELECT DISTINCT ON (r.session_day, r.dataset_id, r.subject_key)
           r.session_day, r.dataset_id, r.subject_key, r.status, r.error_code, r.reason_code
    FROM ops.capture_run r
    JOIN ops.capture_present p
      ON p.session_day = r.session_day AND p.dataset_id = r.dataset_id AND p.subject_key = r.subject_key
    WHERE p.status = 'no_value'
    ORDER BY r.session_day, r.dataset_id, r.subject_key, r.finished_at DESC NULLS FIRST, r.run_id DESC
),
keyed AS (
    SELECT e.session_day, e.dataset_id, e.subject_key,
           CASE
               WHEN p.status IS NULL THEN 'missing'
               WHEN p.status = 'present' THEN 'ok'
               WHEN n.session_day IS NOT NULL
                    AND (n.status <> 'done' OR n.error_code <> '' OR n.reason_code IN ('expired', 'crashed_repeatedly'))
                    THEN 'expired'
               ELSE 'no_value'
           END AS state
    FROM expected e
    LEFT JOIN ops.capture_present p
      ON p.session_day = e.session_day AND p.dataset_id = e.dataset_id AND p.subject_key = e.subject_key
    LEFT JOIN newest_run n
      ON n.session_day = e.session_day AND n.dataset_id = e.dataset_id AND n.subject_key = e.subject_key
)
SELECT session_day,
       dataset_id,
       count(*)::int                                          AS keys,
       (count(*) FILTER (WHERE state = 'ok'))::int            AS ok,
       (count(*) FILTER (WHERE state = 'no_value'))::int      AS no_value,
       (count(*) FILTER (WHERE state = 'expired'))::int       AS expired,
       (count(*) FILTER (WHERE state = 'missing'))::int       AS missing,
       COALESCE((array_agg(subject_key ORDER BY subject_key) FILTER (WHERE state = 'missing'))[1:20],
                ARRAY[]::text[])                              AS missing_keys
FROM keyed
GROUP BY session_day, dataset_id;

ALTER VIEW rd.coverage_day_r1 OWNER TO ot_owner;

GRANT SELECT ON rd.coverage_day_r1 TO ot_ro_pages;

-- +goose Down
REVOKE SELECT ON rd.coverage_day_r1 FROM ot_ro_pages;
DROP VIEW rd.coverage_day_r1;
