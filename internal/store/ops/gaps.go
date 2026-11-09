package ops

import (
	"context"
	"sort"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/gaphistory"
	"github.com/andrewrong/optdesk/internal/store"
)

var _ store.OpenGapReaderFunc = OpenGapReader
var _ gaphistory.OpenFunc = OpenGapHistory
var _ gaphistory.Reader = (*gapReader)(nil)

const (
	selectOpenGapsViewSQL = `SELECT job_id, dataset_id, day, reason, first_seen_at, reported_at, cleared_at ` +
		`FROM rd.job_gap_r1 WHERE cleared_at IS NULL ORDER BY first_seen_at, day, job_id, dataset_id`
	selectGapsSinceViewSQL = `SELECT job_id, dataset_id, day, reason, first_seen_at, reported_at, cleared_at ` +
		`FROM rd.job_gap_r1 WHERE first_seen_at >= $1 OR cleared_at >= $1 ORDER BY first_seen_at, day, job_id, dataset_id`
	selectExpectationsSQL = `SELECT dataset_id, expected_day, present_day, due_by, gap, closed ` +
		`FROM rd.expectation_r1 WHERE expected_day BETWEEN $1 AND $2 ORDER BY expected_day, dataset_id`
)

type gapReader struct{ db database }

// OpenGapReader connects to the database named by dsn (the line DSNEnvPages) and reads the two rd views only.
func OpenGapReader(ctx context.Context, dsn string) (store.GapReader, error) {
	db, err := openPool(ctx, dsn)
	if err != nil {
		return nil, err
	}
	return &gapReader{db: db}, nil
}

// OpenGapHistory connects like OpenGapReader and returns the gap history reader; it reads rd.job_gap_r1 only.
func OpenGapHistory(ctx context.Context, dsn string) (gaphistory.Reader, error) {
	db, err := openPool(ctx, dsn)
	if err != nil {
		return nil, err
	}
	return &gapReader{db: db}, nil
}

// readGaps runs a query of the seven gap columns (job_id, dataset_id, day, reason, first_seen_at, reported_at, cleared_at).
func readGaps(ctx context.Context, q querier, sql string, args ...any) ([]store.Gap, error) {
	out := []store.Gap{}
	err := q.Select(ctx, sql, args, func(s scanner) error {
		var g store.Gap
		var day, first time.Time
		var reported, cleared *time.Time
		if err := s.Scan(&g.JobID, &g.DatasetID, &day, &g.Reason, &first, &reported, &cleared); err != nil {
			return err
		}
		g.Day = contracts.Day(day.Format(time.DateOnly))
		g.FirstSeenAt = first.UTC()
		g.ReportedAt, g.ClearedAt = fromNullable(reported), fromNullable(cleared)
		out = append(out, g)
		return nil
	})
	return out, err
}

func (r *gapReader) OpenGaps(ctx context.Context) ([]store.Gap, error) {
	g, err := readGaps(ctx, r.db, selectOpenGapsViewSQL)
	return g, wrap("open gaps", err)
}

// GapsSince returns the rows of rd.job_gap_r1 first seen or cleared at or after since, ascending by first seen then
// day, job and dataset; an empty slice, not nil, for none.
func (r *gapReader) GapsSince(ctx context.Context, since time.Time) ([]gaphistory.Gap, error) {
	rows, err := readGaps(ctx, r.db, selectGapsSinceViewSQL, since)
	if err != nil {
		return nil, wrap("gaps since", err)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if !a.FirstSeenAt.Equal(b.FirstSeenAt) {
			return a.FirstSeenAt.Before(b.FirstSeenAt)
		}
		if a.Day != b.Day {
			return a.Day < b.Day
		}
		if a.JobID != b.JobID {
			return a.JobID < b.JobID
		}
		return a.DatasetID < b.DatasetID
	})
	out := make([]gaphistory.Gap, len(rows))
	for i, g := range rows {
		out[i] = gaphistory.Gap{JobID: g.JobID, DatasetID: g.DatasetID, Day: g.Day, Reason: g.Reason,
			FirstSeenAt: g.FirstSeenAt, ReportedAt: g.ReportedAt, ClearedAt: g.ClearedAt}
	}
	return out, nil
}

// Expectations returns the rows of rd.expectation_r1 with expected_day from from to to, inclusive.
func (r *gapReader) Expectations(ctx context.Context, from, to contracts.Day) ([]store.Expectation, error) {
	f, err := parseDay(from)
	if err != nil {
		return nil, err
	}
	t, err := parseDay(to)
	if err != nil {
		return nil, err
	}
	out := []store.Expectation{}
	err = r.db.Select(ctx, selectExpectationsSQL, []any{f, t}, func(s scanner) error {
		var e store.Expectation
		var expected time.Time
		var present, due *time.Time
		if err := s.Scan(&e.DatasetID, &expected, &present, &due, &e.Gap, &e.Closed); err != nil {
			return err
		}
		e.ExpectedDay = contracts.Day(expected.Format(time.DateOnly))
		if present != nil {
			e.PresentDay = contracts.Day(present.Format(time.DateOnly))
		}
		e.DueBy = fromNullable(due)
		out = append(out, e)
		return nil
	})
	if err != nil {
		return nil, wrap("expectations", err)
	}
	return out, nil
}

func (r *gapReader) Close() error { return r.db.Close() }
