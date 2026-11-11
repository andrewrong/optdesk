package optionopenpg

import (
	"context"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/openquote"
)

var _ openquote.OptionListStore = (*store)(nil)

// A listed outcome writes every column. A short or error outcome writes tried_at and outcome and keeps listed_at and
// listed_session of the row; a row with none yet holds null in both.
const (
	putListListedSQL = `INSERT INTO master.option_list
  (underlying_id, expiry, class, listed_at, listed_session, count, tried_at, outcome)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (underlying_id, expiry, class) DO UPDATE SET
  listed_at = EXCLUDED.listed_at, listed_session = EXCLUDED.listed_session, count = EXCLUDED.count,
  tried_at = EXCLUDED.tried_at, outcome = EXCLUDED.outcome`

	putListTriedSQL = `INSERT INTO master.option_list
  (underlying_id, expiry, class, listed_at, listed_session, count, tried_at, outcome)
VALUES ($1, $2, $3, NULL, NULL, 0, $4, $5)
ON CONFLICT (underlying_id, expiry, class) DO UPDATE SET
  tried_at = EXCLUDED.tried_at, outcome = EXCLUDED.outcome`

	listListsSQL = `SELECT expiry, class, listed_at, listed_session, count, tried_at, outcome
FROM master.option_list WHERE underlying_id = $1 ORDER BY expiry, class`

	// expected: null is replaced by any write, an empty one by a non-empty one, a non-empty one never.
	putListRunSQL = `INSERT INTO master.option_list_run (session, underlying_id, expected, listed, reason, updated_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (session, underlying_id) DO UPDATE SET
  expected = CASE
    WHEN master.option_list_run.expected IS NULL
      OR (cardinality(master.option_list_run.expected) = 0 AND cardinality(EXCLUDED.expected) > 0)
    THEN EXCLUDED.expected ELSE master.option_list_run.expected END,
  listed = EXCLUDED.listed, reason = EXCLUDED.reason, updated_at = EXCLUDED.updated_at`

	listRunsSQL = `SELECT underlying_id, expected, listed, reason, updated_at
FROM master.option_list_run WHERE session = $1 ORDER BY underlying_id`
)

// PutLists writes the rows in one transaction; see openquote.OptionListStore.
func (s *store) PutLists(ctx context.Context, rows []openquote.ListRecord) error {
	if len(rows) == 0 {
		return nil
	}
	return s.inTx(ctx, "put lists", func(tx execer) error {
		for _, r := range rows {
			exp, err := parseDay(r.Expiry)
			if err != nil {
				return err
			}
			if r.Outcome != "listed" {
				if _, err := tx.Exec(ctx, putListTriedSQL, r.UnderlyingID, exp, r.Class, r.TriedAt.UTC(), r.Outcome); err != nil {
					return err
				}
				continue
			}
			sess, err := parseDay(r.ListedSession)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, putListListedSQL, r.UnderlyingID, exp, r.Class, r.ListedAt.UTC(), sess, r.Count,
				r.TriedAt.UTC(), r.Outcome); err != nil {
				return err
			}
		}
		return nil
	})
}

// Lists returns the rows of the underlying; see openquote.OptionListStore.
func (s *store) Lists(ctx context.Context, underlyingID int64) ([]openquote.ListRecord, error) {
	out := []openquote.ListRecord{}
	err := s.selectRows(ctx, "lists", listListsSQL, []any{underlyingID}, func(sc scanner) error {
		r := openquote.ListRecord{UnderlyingID: underlyingID}
		var exp time.Time
		var at, sess *time.Time
		if err := sc.Scan(&exp, &r.Class, &at, &sess, &r.Count, &r.TriedAt, &r.Outcome); err != nil {
			return err
		}
		r.Expiry = dayOf(exp)
		if at != nil {
			r.ListedAt = at.UTC()
		}
		if sess != nil {
			r.ListedSession = dayOf(*sess)
		}
		r.TriedAt = r.TriedAt.UTC()
		out = append(out, r)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// PutListRuns writes the rows in one transaction; see openquote.OptionListStore. A non-empty Expected of a stored
// row is never replaced.
func (s *store) PutListRuns(ctx context.Context, rows []openquote.ListRunRecord) error {
	if len(rows) == 0 {
		return nil
	}
	return s.inTx(ctx, "put list runs", func(tx execer) error {
		for _, r := range rows {
			sess, err := parseDay(r.Session)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, putListRunSQL, sess, r.UnderlyingID, r.Expected, r.Listed, r.Reason,
				r.UpdatedAt.UTC()); err != nil {
				return err
			}
		}
		return nil
	})
}

// ListRuns returns the rows of the session; see openquote.OptionListStore.
func (s *store) ListRuns(ctx context.Context, session contracts.Day) ([]openquote.ListRunRecord, error) {
	d, err := parseDay(session)
	if err != nil {
		return nil, err
	}
	out := []openquote.ListRunRecord{}
	err = s.selectRows(ctx, "list runs", listRunsSQL, []any{d}, func(sc scanner) error {
		r := openquote.ListRunRecord{Session: session}
		if err := sc.Scan(&r.UnderlyingID, &r.Expected, &r.Listed, &r.Reason, &r.UpdatedAt); err != nil {
			return err
		}
		r.UpdatedAt = r.UpdatedAt.UTC()
		out = append(out, r)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
