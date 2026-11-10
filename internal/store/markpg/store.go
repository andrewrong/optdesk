// Package markpg implements mark.Store on PostgreSQL: the position marks of account.position_mark, insert-only
// (docs/design/position-marks.md Decisions 3 and 4). Only cmd/ot imports it; jobs and pages import the declarations
// in internal/store/mark.
package markpg

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/mark"
)

var _ mark.OpenFunc = Open

const (
	// insertSQL arguments: capture_id, alias, marked_at, session_day, con_id, attempt, currency, price, basis, bid,
	// ask, last, close, market_data_type, status. An empty decimal is passed as null. A key that exists is left as
	// it is: a correction is a row with a higher attempt.
	insertSQL = `INSERT INTO account.position_mark (capture_id, alias, marked_at, session_day, con_id, attempt, ` +
		`currency, price, basis, bid, ask, last, close, market_data_type, status) ` +
		`VALUES ($1, $2, $3, $4::date, $5, $6, $7, $8::numeric, $9, $10::numeric, $11::numeric, $12::numeric, ` +
		`$13::numeric, $14, $15) ON CONFLICT DO NOTHING`

	// marksSQL arguments: alias, day. The newest capture of the day with marks (by marked_at, ties by capture_id),
	// then the highest attempt of each con_id. It reads the view and no table.
	marksSQL = `SELECT DISTINCT ON (con_id) capture_id, alias, marked_at, session_day::text, con_id, attempt, currency, ` +
		`coalesce(price::text, ''), basis, coalesce(bid::text, ''), coalesce(ask::text, ''), coalesce(last::text, ''), ` +
		`coalesce(close::text, ''), market_data_type, status ` +
		`FROM rd.account_position_mark_r1 WHERE alias = $1 AND capture_id = (` +
		`SELECT capture_id FROM rd.account_position_mark_r1 WHERE alias = $1 AND session_day = $2::date ` +
		`ORDER BY marked_at DESC, capture_id DESC LIMIT 1) ORDER BY con_id, attempt DESC`
)

// execer runs one statement inside a transaction.
type execer interface {
	Exec(ctx context.Context, sql string, args ...any) (int64, error)
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

type store struct{ db database }

// Open connects to the database named by dsn (the line mark.DSNEnvPositionMark, passed in by the caller) and returns
// the store. It applies no migration: the schema is made by goose with the role ot_owner.
func Open(ctx context.Context, dsn string) (mark.Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, permanent(fmt.Errorf("markpg: open: %w", err))
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, retryable(fmt.Errorf("markpg: ping: %w", err))
	}
	return &store{db: pgDB{pool: pool}}, nil
}

// Close releases the connections.
func (s *store) Close() error { return s.db.Close() }

// WriteMarks inserts the rows in one transaction; a row whose (CaptureID, ConID, Attempt) exists stays as it is.
// A malformed row is a Permanent error and nothing is written; no rows make no database call.
func (s *store) WriteMarks(ctx context.Context, rows []mark.PositionMark) error {
	if len(rows) == 0 {
		return nil
	}
	for _, r := range rows {
		if r.CaptureID == "" || r.Alias == "" || r.Attempt < 1 {
			return permanent(errors.New("markpg: a mark needs a capture id, an alias and an attempt from 1"))
		}
		if _, err := time.Parse(time.DateOnly, r.SessionDay); err != nil {
			return permanent(fmt.Errorf("markpg: bad day %q: %w", r.SessionDay, err))
		}
	}
	err := s.db.InTx(ctx, func(tx execer) error {
		for _, r := range rows {
			if _, err := tx.Exec(ctx, insertSQL, r.CaptureID, r.Alias, r.MarkedAt, r.SessionDay, r.ConID, r.Attempt,
				r.Currency, nullIfEmpty(r.Price), r.Basis, nullIfEmpty(r.Bid), nullIfEmpty(r.Ask), nullIfEmpty(r.Last),
				nullIfEmpty(r.Close), r.MarketDataType, r.Status); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return wrap(err, "write marks")
	}
	return nil
}

// Marks returns the rows of the newest capture of day with marks for alias, the highest attempt of each ConID,
// sorted by ConID; an empty slice when none.
func (s *store) Marks(ctx context.Context, alias, day string) ([]mark.PositionMark, error) {
	if _, err := time.Parse(time.DateOnly, day); err != nil {
		return nil, permanent(fmt.Errorf("markpg: bad day %q: %w", day, err))
	}
	out := []mark.PositionMark{}
	err := s.db.Select(ctx, marksSQL, []any{alias, day}, func(sc scanner) error {
		var m mark.PositionMark
		if err := sc.Scan(&m.CaptureID, &m.Alias, &m.MarkedAt, &m.SessionDay, &m.ConID, &m.Attempt, &m.Currency,
			&m.Price, &m.Basis, &m.Bid, &m.Ask, &m.Last, &m.Close, &m.MarketDataType, &m.Status); err != nil {
			return err
		}
		m.MarkedAt = m.MarkedAt.UTC()
		out = append(out, m)
		return nil
	})
	if err != nil {
		return nil, wrap(err, "marks")
	}
	return out, nil
}

// nullIfEmpty passes an empty decimal as null.
func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// wrap makes a database error Retryable, except an integrity constraint violation (SQLSTATE class 23), which the
// database repeats on every attempt and is Permanent; it keeps a typed error as it is.
func wrap(err error, what string) error {
	var te contracts.TypedError
	if errors.As(err, &te) {
		return err
	}
	var pe *pgconn.PgError
	if errors.As(err, &pe) && strings.HasPrefix(pe.Code, "23") {
		return permanent(fmt.Errorf("markpg: %s: %w", what, err))
	}
	return retryable(fmt.Errorf("markpg: %s: %w", what, err))
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

type pgTx struct{ tx pgx.Tx }

func (t pgTx) Exec(ctx context.Context, sql string, args ...any) (int64, error) {
	tag, err := t.tx.Exec(ctx, sql, args...)
	return tag.RowsAffected(), err
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
