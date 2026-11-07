package snapshot

import (
	"context"
	"fmt"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

var _ store.OpenOptionEodCatalogFunc = OpenOptionEodCatalog

// latestSQL reads the view of the option end-of-day table, one row per underlying with its greatest session_day.
const latestSQL = `SELECT underlying_id, max(session_day) FROM rd.option_eod_r1 `

// latestSinceSQL bounds the scan below; argument: first day.
const latestSinceSQL = `WHERE session_day >= $1 `

// latestGroupSQL groups and orders the answer.
const latestGroupSQL = `GROUP BY underlying_id ORDER BY underlying_id`

type catalogStore struct{ db database }

// OpenOptionEodCatalog connects to the database named by dsn and returns the option end-of-day catalog.
// It applies no migration: the schema is made by goose with the role ot_owner.
func OpenOptionEodCatalog(ctx context.Context, dsn string) (store.OptionEodCatalogStore, error) {
	db, err := openDB(ctx, "option_eod_catalog", dsn)
	if err != nil {
		return nil, err
	}
	return &catalogStore{db: db}, nil
}

// OptionEodLatest returns one entry per underlying with a row of session_day on or after since, ordered by
// UnderlyingID. An empty since means no lower bound; a since that is not YYYY-MM-DD is a Permanent error and
// the database is not called. No row gives an empty list and no error.
func (s *catalogStore) OptionEodLatest(ctx context.Context, since contracts.Day) ([]store.OptionEodLatest, error) {
	sql := latestSQL + latestGroupSQL
	var args []any
	if since != "" {
		d, err := parseDay(since)
		if err != nil {
			return nil, err
		}
		sql = latestSQL + latestSinceSQL + latestGroupSQL
		args = []any{d}
	}
	out := []store.OptionEodLatest{}
	err := s.db.Select(ctx, sql, args, func(sc scanner) error {
		var id int64
		var day time.Time
		if err := sc.Scan(&id, &day); err != nil {
			return err
		}
		out = append(out, store.OptionEodLatest{UnderlyingID: id, SessionDay: contracts.Day(day.Format(time.DateOnly))})
		return nil
	})
	if err != nil {
		return nil, retryable(fmt.Errorf("option_eod_catalog: select: %w", err))
	}
	return out, nil
}

// Close releases the connections.
func (s *catalogStore) Close() error { return s.db.Close() }
