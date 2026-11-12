package optionopenpg

import (
	"context"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/openquote"
)

var _ openquote.OptionListStore = (*store)(nil)

// A listed outcome writes every column, the band included (null when the record holds none). A short or error outcome
// writes tried_at and outcome and keeps listed_at, listed_session and the band of the row; a row with none yet holds
// null in each. The band is decimal text: it goes through text so the numeric keeps its digits.
const (
	putListListedSQL = `INSERT INTO master.option_list
  (underlying_id, expiry, class, listed_at, listed_session, count, tried_at, outcome,
   band_lo,
   band_hi,
   sigma,
   in_band,
   sigma_source)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::text::numeric, $10::text::numeric, $11::float8::numeric, $12, $13)
ON CONFLICT (underlying_id, expiry, class) DO UPDATE SET
  listed_at = EXCLUDED.listed_at, listed_session = EXCLUDED.listed_session, count = EXCLUDED.count,
  tried_at = EXCLUDED.tried_at, outcome = EXCLUDED.outcome,
  band_lo = EXCLUDED.band_lo,
  band_hi = EXCLUDED.band_hi,
  sigma = EXCLUDED.sigma,
  in_band = EXCLUDED.in_band,
  sigma_source = EXCLUDED.sigma_source`

	putListTriedSQL = `INSERT INTO master.option_list
  (underlying_id, expiry, class, listed_at, listed_session, count, tried_at, outcome)
VALUES ($1, $2, $3, NULL, NULL, 0, $4, $5)
ON CONFLICT (underlying_id, expiry, class) DO UPDATE SET
  tried_at = EXCLUDED.tried_at, outcome = EXCLUDED.outcome`

	listListsSQL = `SELECT expiry, class, listed_at, listed_session, count, tried_at, outcome,
  band_lo::text,
  band_hi::text,
  sigma::float8,
  in_band,
  sigma_source
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

// nullIf is the text for a nullable column: nil for the empty string.
func nullIf(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

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
			// Sigma and InBand are stored when their companions are: a sigma with no source, or a count with no
			// band, is none.
			var sigma *float64
			if r.SigmaSource != "" {
				sigma = &r.Sigma
			}
			var inBand *int
			if r.BandLo != "" || r.BandHi != "" {
				inBand = &r.InBand
			}
			if _, err := tx.Exec(ctx, putListListedSQL, r.UnderlyingID, exp, r.Class, r.ListedAt.UTC(), sess, r.Count,
				r.TriedAt.UTC(), r.Outcome, nullIf(r.BandLo), nullIf(r.BandHi), sigma, inBand, nullIf(r.SigmaSource)); err != nil {
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
		var lo, hi, src *string
		var sigma *float64
		var inBand *int
		if err := sc.Scan(&exp, &r.Class, &at, &sess, &r.Count, &r.TriedAt, &r.Outcome, &lo, &hi, &sigma, &inBand, &src); err != nil {
			return err
		}
		r.BandLo, r.BandHi, r.SigmaSource = deref(lo), deref(hi), deref(src)
		if sigma != nil {
			r.Sigma = *sigma
		}
		if inBand != nil {
			r.InBand = *inBand
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
