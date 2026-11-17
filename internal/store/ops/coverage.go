package ops

import (
	"context"
	"sort"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/coverage"
)

var _ coverage.Reader = (*coverageReader)(nil)
var _ coverage.OpenFunc = OpenCoverage

const (
	selectCoverageCellsSQL = `SELECT session_day, dataset_id, keys, ok, no_value, expired, missing, missing_keys ` +
		`FROM rd.coverage_day_r1 WHERE session_day BETWEEN $1 AND $2 ORDER BY session_day, dataset_id`
	selectCoverageSessionsSQL = `SELECT calendar_id, session_day, is_session ` +
		`FROM rd.session_day_r1 WHERE calendar_id = $1 AND session_day BETWEEN $2 AND $3 ORDER BY session_day`
	selectNewestObservationsSQL = `SELECT series_id, max(observation_date) ` +
		`FROM rd.series_point_r1 GROUP BY series_id ORDER BY series_id`
)

type coverageReader struct{ db database }

// OpenCoverage connects to the database named by dsn (the line DSNEnvPages) and reads the views
// rd.coverage_day_r1, rd.session_day_r1 and rd.series_point_r1 only.
func OpenCoverage(ctx context.Context, dsn string) (coverage.Reader, error) {
	db, err := openPool(ctx, dsn)
	if err != nil {
		return nil, err
	}
	return &coverageReader{db: db}, nil
}

func dayOf(t time.Time) contracts.Day { return contracts.Day(t.Format(time.DateOnly)) }

// Cells returns the rows of rd.coverage_day_r1 from from to to, inclusive, ascending by day then dataset.
func (r *coverageReader) Cells(ctx context.Context, from, to contracts.Day) ([]coverage.Cell, error) {
	f, err := parseDay(from)
	if err != nil {
		return nil, err
	}
	t, err := parseDay(to)
	if err != nil {
		return nil, err
	}
	out := []coverage.Cell{}
	err = r.db.Select(ctx, selectCoverageCellsSQL, []any{f, t}, func(s scanner) error {
		var c coverage.Cell
		var day time.Time
		if err := s.Scan(&day, &c.Dataset, &c.Keys, &c.OK, &c.NoValue, &c.Expired, &c.Missing, &c.MissingKeys); err != nil {
			return err
		}
		c.Day = dayOf(day)
		if c.MissingKeys == nil {
			c.MissingKeys = []string{}
		}
		out = append(out, c)
		return nil
	})
	if err != nil {
		return nil, wrap("coverage cells", err)
	}
	return out, nil
}

// Sessions returns the days of one calendar from from to to, inclusive, ascending.
func (r *coverageReader) Sessions(ctx context.Context, calendar string, from, to contracts.Day) ([]coverage.Session, error) {
	f, err := parseDay(from)
	if err != nil {
		return nil, err
	}
	t, err := parseDay(to)
	if err != nil {
		return nil, err
	}
	out := []coverage.Session{}
	err = r.db.Select(ctx, selectCoverageSessionsSQL, []any{calendar, f, t}, func(s scanner) error {
		var x coverage.Session
		var day time.Time
		if err := s.Scan(&x.Calendar, &day, &x.IsSession); err != nil {
			return err
		}
		x.Day = dayOf(day)
		out = append(out, x)
		return nil
	})
	if err != nil {
		return nil, wrap("coverage sessions", err)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Day < out[j].Day })
	return out, nil
}

// NewestObservations returns the newest observation day of every series, ascending by series id.
func (r *coverageReader) NewestObservations(ctx context.Context) ([]coverage.Newest, error) {
	out := []coverage.Newest{}
	err := r.db.Select(ctx, selectNewestObservationsSQL, nil, func(s scanner) error {
		var n coverage.Newest
		var day time.Time
		if err := s.Scan(&n.SeriesID, &day); err != nil {
			return err
		}
		n.Day = dayOf(day)
		out = append(out, n)
		return nil
	})
	if err != nil {
		return nil, wrap("newest observations", err)
	}
	return out, nil
}

func (r *coverageReader) Close() error { return r.db.Close() }
