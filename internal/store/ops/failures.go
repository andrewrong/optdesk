package ops

import (
	"context"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/coverage"
)

var _ coverage.FailureReader = (*coverageReader)(nil)

// selectRepeatedFailuresSQL reads rd.capture_run_r1 only (docs/design/repeated-failures.md, Decisions 1, 2, 5).
// runs holds the counted done runs: failed (error code), empty (no error, returned_n > 0, usable_n = 0) or ok.
// A done run with no error and nothing returned or usable, and every run not done, is left out of both the
// counts and the choice of the newest run. $1 and $2 are the first and last session day, $3 the minimum days.
// The view has no run_id, so equal finished_at is settled by session_day, then error_code.
const selectRepeatedFailuresSQL = `WITH runs AS (
  SELECT dataset_id, subject_key, session_day, finished_at, error_code,
    CASE WHEN error_code <> '' THEN 'failed'
         WHEN returned_n > 0 AND usable_n = 0 THEN 'empty'
         ELSE 'ok' END AS kind
  FROM rd.capture_run_r1
  WHERE status = 'done' AND session_day BETWEEN $1 AND $2
    AND (error_code <> '' OR returned_n > 0 OR usable_n > 0)
), newest AS (
  SELECT DISTINCT ON (dataset_id, subject_key) dataset_id, subject_key, kind, error_code
  FROM runs
  ORDER BY dataset_id, subject_key, finished_at DESC NULLS LAST, session_day DESC, error_code DESC
)
SELECT r.dataset_id, r.subject_key, r.kind, count(DISTINCT r.session_day)::int, count(*)::int,
  min(r.session_day), max(r.session_day),
  CASE WHEN n.kind = 'empty' THEN 'empty' ELSE n.error_code END
FROM runs r JOIN newest n USING (dataset_id, subject_key)
WHERE r.kind <> 'ok' AND n.kind <> 'ok'
GROUP BY r.dataset_id, r.subject_key, r.kind, n.kind, n.error_code
HAVING count(DISTINCT r.session_day) >= $3
ORDER BY r.dataset_id, r.subject_key, r.kind`

// RepeatedFailures returns the keys with failed or empty runs on coverage.FailureMinDays or more distinct session
// days from from to to whose newest counted done run is failed or empty. It only reads.
func (r *coverageReader) RepeatedFailures(ctx context.Context, from, to contracts.Day) ([]coverage.Failure, error) {
	f, err := parseDay(from)
	if err != nil {
		return nil, err
	}
	t, err := parseDay(to)
	if err != nil {
		return nil, err
	}
	out := []coverage.Failure{}
	err = r.db.Select(ctx, selectRepeatedFailuresSQL, []any{f, t, coverage.FailureMinDays}, func(s scanner) error {
		var x coverage.Failure
		var kind string
		var first, last time.Time
		if err := s.Scan(&x.Dataset, &x.Key, &kind, &x.Days, &x.Runs, &first, &last, &x.LastClass); err != nil {
			return err
		}
		x.Kind = coverage.FailureKind(kind)
		x.FirstDay, x.LastDay = dayOf(first), dayOf(last)
		out = append(out, x)
		return nil
	})
	if err != nil {
		return nil, wrap("repeated failures", err)
	}
	return out, nil
}
