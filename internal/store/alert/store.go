package alert

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

var _ store.SentStore = (*sentStore)(nil)

const (
	selectSentSQL = `SELECT rule_id, day FROM alerts.sent WHERE day >= $1 ORDER BY day, rule_id`
	markSentSQL   = `INSERT INTO alerts.sent (rule_id, day, sent_at) VALUES ($1, $2, now()) ON CONFLICT DO NOTHING`
)

// scanner reads the columns of the current row.
type scanner interface {
	Scan(dest ...any) error
}

// database is the seam under the store: the one place pgx is touched, replaced by a fake in unit tests.
type database interface {
	// Exec runs one statement.
	Exec(ctx context.Context, sql string, args ...any) error
	// Select runs a query and calls scan once per row, in the order the database returns them.
	Select(ctx context.Context, sql string, args []any, scan func(scanner) error) error
	Close() error
}

type sentStore struct{ db database }

// Open connects to the database named by dsn and returns the sent-mark store.
// It applies no migration: the schema is made by goose with the role ot_owner.
func Open(ctx context.Context, dsn string) (store.SentStore, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, permanent(fmt.Errorf("alert: open: %w", err))
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, retryable(fmt.Errorf("alert: ping: %w", err))
	}
	return &sentStore{db: pgDB{pool: pool}}, nil
}

// Close releases the connections.
func (s *sentStore) Close() error { return s.db.Close() }

// SentSince returns the marks with day >= from, ascending by day then rule id; none gives an empty slice.
func (s *sentStore) SentSince(ctx context.Context, from contracts.Day) ([]store.SentMark, error) {
	f, err := parseDay(from)
	if err != nil {
		return nil, err
	}
	out := []store.SentMark{}
	err = s.db.Select(ctx, selectSentSQL, []any{f}, func(sc scanner) error {
		var m store.SentMark
		var day time.Time
		if err := sc.Scan(&m.RuleID, &day); err != nil {
			return err
		}
		m.Day = contracts.Day(day.Format(time.DateOnly))
		out = append(out, m)
		return nil
	})
	if err != nil {
		return nil, retryable(fmt.Errorf("alert: select: %w", err))
	}
	return out, nil
}

// MarkSent records that ruleID was said on day. A mark already there stays as it is and is no error.
func (s *sentStore) MarkSent(ctx context.Context, ruleID string, day contracts.Day) error {
	if ruleID == "" {
		return permanent(fmt.Errorf("alert: empty rule id"))
	}
	d, err := parseDay(day)
	if err != nil {
		return err
	}
	if err := s.db.Exec(ctx, markSentSQL, ruleID, d); err != nil {
		return retryable(fmt.Errorf("alert: mark: %w", err))
	}
	return nil
}

// parseDay turns a contracts.Day into the date it names.
func parseDay(d contracts.Day) (time.Time, error) {
	t, err := time.Parse(time.DateOnly, string(d))
	if err != nil {
		return time.Time{}, permanent(fmt.Errorf("alert: bad day %q: %w", d, err))
	}
	return t, nil
}

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

func (d pgDB) Exec(ctx context.Context, sql string, args ...any) error {
	_, err := d.pool.Exec(ctx, sql, args...)
	return err
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
