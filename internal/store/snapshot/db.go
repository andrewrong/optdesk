// Package snapshot is the Reader and the Writer of obs.option_eod r1 over PostgreSQL.
package snapshot

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// execer runs one statement inside a transaction.
type execer interface {
	Exec(ctx context.Context, sql string, args ...any) error
}

// scanner reads the columns of the current row.
type scanner interface {
	Scan(dest ...any) error
}

// database is the seam under the stores of this package: the one place pgx is touched, replaced by a fake in unit tests.
type database interface {
	// InTx runs fn in one transaction: committed when fn returns nil, rolled back otherwise.
	InTx(ctx context.Context, fn func(execer) error) error
	// Select runs a query and calls scan once per row, in the order the database returns them.
	Select(ctx context.Context, sql string, args []any, scan func(scanner) error) error
	Close() error
}

// openDB connects to dsn and checks the connection; name prefixes the errors.
func openDB(ctx context.Context, name, dsn string) (database, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, permanent(fmt.Errorf("%s: open: %w", name, err))
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, retryable(fmt.Errorf("%s: ping: %w", name, err))
	}
	return pgDB{pool: pool}, nil
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

func (t pgTx) Exec(ctx context.Context, sql string, args ...any) error {
	_, err := t.tx.Exec(ctx, sql, args...)
	return err
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
		return time.Time{}, permanent(fmt.Errorf("snapshot: bad day %q: %w", d, err))
	}
	return t, nil
}
