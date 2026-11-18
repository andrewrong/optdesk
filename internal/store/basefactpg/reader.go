// Package basefactpg is the Postgres implementation of basefact.Reader over pgx, for the DSN line of the page role
// ot_ro_pages (docs/design/base-data-page.md, Interface 1 and issue 4). It reads the four views
// rd.base_fact_*_r1, one query each, and writes nothing.
package basefactpg

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/basefact"
)

// viewTimeout bounds each view query on its own.
const viewTimeout = 10 * time.Second

// columns are the columns of every view, in the order of the scan.
const columns = `dataset, rule, subject, n, bad, day, value`

// view is one of the four views and the group name that Result.Failed gives it.
type view struct{ group, name string }

// views is the fixed list of the four views, in the order they are read.
var views = []view{
	{basefact.GroupBars, "rd.base_fact_bars_r1"},
	{basefact.GroupOption, "rd.base_fact_option_r1"},
	{basefact.GroupOther, "rd.base_fact_other_r1"},
	{basefact.GroupAccount, "rd.base_fact_account_r1"},
}

// record is one row of a view as the database gives it: a null day is nil.
type record struct {
	Dataset, Rule, Subject string
	N, Bad                 int64
	Day                    *time.Time
	Value                  string
}

// source is the seam under the reader: the one place pgx is touched, replaced by a fake in unit tests.
type source interface {
	// Rows runs the query and returns every row, honouring ctx.
	Rows(ctx context.Context, sql string) ([]record, error)
	Close() error
}

type reader struct {
	src     source
	timeout time.Duration
}

var _ basefact.Reader = (*reader)(nil)

// Open connects to the database named by dsn (the line of the role ot_ro_pages, passed in by the caller). It
// runs no statement besides a ping.
func Open(ctx context.Context, dsn string) (basefact.Reader, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("basefactpg: open: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("basefactpg: ping: %w", err)
	}
	return &reader{src: pgSource{pool: pool}, timeout: viewTimeout}, nil
}

var _ basefact.OpenFunc = Open

// Close releases the connections.
func (r *reader) Close() error { return r.src.Close() }

// Facts reads the four views, each under its own timeout. A view that fails or times out is named in Failed and
// its facts are left out; the error is returned only when all four failed. Facts are sorted by dataset, rule and
// subject. Both slices are non-nil.
func (r *reader) Facts(ctx context.Context) (basefact.Result, error) {
	res := basefact.Result{Facts: []basefact.Fact{}, Failed: []string{}}
	var errs []error
	for _, v := range views {
		recs, err := r.read(ctx, v)
		if err != nil {
			res.Failed = append(res.Failed, v.group)
			errs = append(errs, fmt.Errorf("%s: %w", v.name, err))
			continue
		}
		for _, rc := range recs {
			f := basefact.Fact{Dataset: rc.Dataset, Rule: rc.Rule, Subject: rc.Subject, N: rc.N, Bad: rc.Bad, Value: rc.Value}
			if rc.Day != nil {
				f.Day = contracts.Day(rc.Day.Format("2006-01-02"))
			}
			res.Facts = append(res.Facts, f)
		}
	}
	sort.SliceStable(res.Facts, func(i, j int) bool {
		a, b := res.Facts[i], res.Facts[j]
		if a.Dataset != b.Dataset {
			return a.Dataset < b.Dataset
		}
		if a.Rule != b.Rule {
			return a.Rule < b.Rule
		}
		return a.Subject < b.Subject
	})
	if len(res.Failed) == len(views) {
		return res, fmt.Errorf("basefactpg: all four views failed: %w", errors.Join(errs...))
	}
	return res, nil
}

func (r *reader) read(ctx context.Context, v view) ([]record, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	return r.src.Rows(ctx, `SELECT `+columns+` FROM `+v.name)
}

type pgSource struct{ pool *pgxpool.Pool }

func (s pgSource) Close() error { s.pool.Close(); return nil }

func (s pgSource) Rows(ctx context.Context, sql string) ([]record, error) {
	rows, err := s.pool.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []record
	for rows.Next() {
		var rc record
		if err := rows.Scan(&rc.Dataset, &rc.Rule, &rc.Subject, &rc.N, &rc.Bad, &rc.Day, &rc.Value); err != nil {
			return nil, err
		}
		out = append(out, rc)
	}
	return out, rows.Err()
}
