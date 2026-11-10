// Package optionopenpg implements openquote.Stores on PostgreSQL: obs.option_open, plan.option_open_item,
// master.option_not_listed and master.option_contract (migrations 0029 to 0032). Only cmd/ot imports it; jobs and
// pages import the declarations in internal/store/openquote. Reads go through the views rd.option_open_r1 and
// rd.option_open_item_r1, so the pages role can read with it.
package optionopenpg

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/openquote"
)

var _ openquote.OpenFunc = Open

// execer runs one statement inside a transaction and reports the rows it affected.
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

// Open connects to the database named by dsn and returns the stores. It applies no migration: the schema is made by
// goose with the role ot_owner. The writes need the role ot_w_option_open (openquote.DSNEnvOptionOpen); with the
// pages line only the readers work.
func Open(ctx context.Context, dsn string) (openquote.Stores, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, permanent(fmt.Errorf("optionopenpg: open: %w", err))
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, retryable(fmt.Errorf("optionopenpg: ping: %w", err))
	}
	return &store{db: pgDB{pool: pool}}, nil
}

// Close releases the connections.
func (s *store) Close() error { return s.db.Close() }

// inTx runs fn in a transaction; a database error is wrapped as Retryable, or Permanent for a constraint violation.
func (s *store) inTx(ctx context.Context, what string, fn func(execer) error) error {
	if err := s.db.InTx(ctx, fn); err != nil {
		return wrap(err, what)
	}
	return nil
}

// selectRows runs a query; a database error is wrapped as in inTx.
func (s *store) selectRows(ctx context.Context, what, sql string, args []any, scan func(scanner) error) error {
	if err := s.db.Select(ctx, sql, args, scan); err != nil {
		return wrap(err, what)
	}
	return nil
}

// parseDay turns a contracts.Day into the date it names; a bad one is Permanent.
func parseDay(d contracts.Day) (time.Time, error) {
	t, err := time.Parse(time.DateOnly, string(d))
	if err != nil {
		return time.Time{}, permanent(fmt.Errorf("optionopenpg: bad day %q: %w", d, err))
	}
	return t, nil
}

func dayOf(t time.Time) contracts.Day { return contracts.Day(t.Format(time.DateOnly)) }

func serial(t time.Time) types.SerializableDate { return types.SerializableDate{Time: t} }

// screen returns nil for NaN, an infinity and -1 (the broker's "no value"), v itself otherwise.
func screen(v *float64) *float64 {
	if v == nil || *v == -1 || math.IsNaN(*v) || math.IsInf(*v, 0) {
		return nil
	}
	return v
}

func utc(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
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
		return permanent(fmt.Errorf("optionopenpg: %s: %w", what, err))
	}
	return retryable(fmt.Errorf("optionopenpg: %s: %w", what, err))
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
