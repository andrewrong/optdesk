// Package series is the Reader and the Writer of obs.series_point r1 over PostgreSQL.
package series

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

var _ store.OpenSeriesFunc = Open

// StatusSentinelRejected is the status stored with a NULL value when the source sent a sentinel
// (-1, NaN or an infinity); the CHECK series_point_no_sentinel of migrations/0002_series.sql mirrors it.
const StatusSentinelRejected = "sentinel_rejected"

// insertPointSQL inserts one point; the replay of a capture, or a second vintage of a date, is a conflict on the
// primary key (series_id, observation_date) and adds no row.
// Arguments: series_id, observation_date, realtime_start, realtime_end, value, status, unit, currency,
// published_at, source, source_revision.
const insertPointSQL = `INSERT INTO obs.series_point (series_id, observation_date, realtime_start, realtime_end, ` +
	`value, status, unit, currency, published_at, source, source_revision) ` +
	`VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) ON CONFLICT DO NOTHING`

// insertDefinitionSQL makes sure the series of a point exists. Arguments: series_id, unit, currency.
const insertDefinitionSQL = `INSERT INTO obs.series_definition (series_id, unit, currency) ` +
	`VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`

// execer runs one statement inside a transaction.
type execer interface {
	Exec(ctx context.Context, sql string, args ...any) error
	// ExecReturning runs a statement with RETURNING and reports how many rows it returned.
	ExecReturning(ctx context.Context, sql string, args ...any) (int, error)
}

// scanner reads the columns of the current row.
type scanner interface {
	Scan(dest ...any) error
}

// database is the seam under the store: the one place pgx is touched, replaced by a fake in unit tests.
type database interface {
	// InTx runs fn in one transaction: committed when fn returns nil, rolled back otherwise.
	InTx(ctx context.Context, fn func(execer) error) error
	// Select runs a query and calls scan once per row, in the order the database returns them.
	Select(ctx context.Context, sql string, args []any, scan func(scanner) error) error
	Close() error
}

type seriesStore struct{ db database }

// Open connects to the database named by dsn and returns the series store.
// It applies no migration: the schema is made by goose with the role ot_owner.
func Open(ctx context.Context, dsn string) (store.SeriesStore, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, permanent(fmt.Errorf("series: open: %w", err))
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, retryable(fmt.Errorf("series: ping: %w", err))
	}
	return &seriesStore{db: pgDB{pool: pool}}, nil
}

// InsertSeriesPoints inserts the points in one transaction; a point whose key
// (series_id, observation_date, realtime_start) exists is left as it is.
// A value that is -1, NaN or an infinity is stored as NULL with the status StatusSentinelRejected.
// An empty input makes no database call.
func (s *seriesStore) InsertSeriesPoints(ctx context.Context, rows []contracts.SeriesPoint) error {
	if len(rows) == 0 {
		return nil
	}
	err := s.db.InTx(ctx, func(tx execer) error {
		for _, r := range rows {
			value, status := screen(r.Value, r.Status)
			if err := tx.Exec(ctx, insertDefinitionSQL, r.SeriesID, string(r.Unit), r.Currency); err != nil {
				return err
			}
			if err := tx.Exec(ctx, insertPointSQL, r.SeriesID, r.ObservationDate.Time, r.RealtimeStart.Time, r.RealtimeEnd.Time,
				value, status, string(r.Unit), r.Currency, r.PublishedAt, r.Source, r.SourceRevision); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return retryable(fmt.Errorf("series: insert: %w", err))
	}
	return nil
}

// screen returns the value and status to store: NULL and StatusSentinelRejected for a sentinel.
func screen(v *float64, status string) (any, string) {
	if v == nil {
		return nil, status
	}
	if *v == -1 || math.IsNaN(*v) || math.IsInf(*v, 0) {
		return nil, StatusSentinelRejected
	}
	return *v, status
}

// Close releases the connections.
func (s *seriesStore) Close() error { return s.db.Close() }

// typedErr is a contracts.TypedError wrapping a cause.
type typedErr struct {
	kind  contracts.ErrorKind
	cause error
}

func (e *typedErr) Error() string             { return e.cause.Error() }
func (e *typedErr) Unwrap() error             { return e.cause }
func (e *typedErr) Kind() contracts.ErrorKind { return e.kind }

func retryable(err error) error { return &typedErr{kind: contracts.Retryable, cause: err} }
func permanent(err error) error { return &typedErr{kind: contracts.Permanent, cause: err} }

// pgDB is the pgx adapter of database.
type pgDB struct{ pool *pgxpool.Pool }

type pgTx struct{ tx pgx.Tx }

func (t pgTx) Exec(ctx context.Context, sql string, args ...any) error {
	_, err := t.tx.Exec(ctx, sql, args...)
	return err
}

func (t pgTx) ExecReturning(ctx context.Context, sql string, args ...any) (int, error) {
	rows, err := t.tx.Query(ctx, sql, args...)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		n++
	}
	return n, rows.Err()
}

func (d pgDB) InTx(ctx context.Context, fn func(execer) error) error {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return err
	}
	if err := fn(pgTx{tx: tx}); err != nil {
		if rerr := tx.Rollback(context.WithoutCancel(ctx)); rerr != nil && !errors.Is(rerr, pgx.ErrTxClosed) {
			err = errors.Join(err, rerr)
		}
		return err
	}
	return tx.Commit(ctx)
}

func (d pgDB) Select(ctx context.Context, sql string, args []any, scan func(scanner) error) error {
	rows, err := d.pool.Query(ctx, sql, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := scan(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (d pgDB) Close() error {
	d.pool.Close()
	return nil
}

// parseDay turns a contracts.Day into the date it names.
func parseDay(d contracts.Day) (time.Time, error) {
	t, err := time.Parse(time.DateOnly, string(d))
	if err != nil {
		return time.Time{}, permanent(fmt.Errorf("series: bad day %q: %w", d, err))
	}
	return t, nil
}
