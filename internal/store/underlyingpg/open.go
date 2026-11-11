// Package underlyingpg implements underlying.Store on PostgreSQL: the names the owner follows, one insert-only
// versioned table settings.underlying read through the view rd.underlying_r1 (docs/design/underlyings.md Decisions 2,
// 3 and 7). Only cmd/ot imports it; pages and jobs import the declarations in internal/store/underlying.
package underlyingpg

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/underlying"
)

var (
	_ underlying.OpenFunc = Open
	_ underlying.Store    = (*Store)(nil)
)

// scanner reads the columns of the current row.
type scanner interface {
	Scan(dest ...any) error
}

// querier runs statements; inside a transaction every call of one InTx shares it.
type querier interface {
	// Select runs a query and calls scan once per row, in the order the database returns them.
	Select(ctx context.Context, sql string, args []any, scan func(scanner) error) error
	// Exec runs a statement that returns no rows.
	Exec(ctx context.Context, sql string, args ...any) error
}

// database is the seam under the store: the one place pgx is touched, replaced by a fake in unit tests.
type database interface {
	querier
	// InTx runs fn in one transaction: committed when fn returns nil, rolled back otherwise.
	InTx(ctx context.Context, fn func(querier) error) error
	Close() error
}

// Store is the underlying store: an underlying.Store.
type Store struct{ db database }

// Open connects to the database named by dsn and returns the store. It applies no migration: the schema is made
// by goose with the role ot_owner. A role without INSERT on the table (a page role) opens and reads; its Apply and
// Settle fail with a Permanent error.
func Open(ctx context.Context, dsn string) (underlying.Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, permanent(fmt.Errorf("underlyingpg: open: %w", err))
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, retryable(fmt.Errorf("underlyingpg: ping: %w", err))
	}
	return &Store{db: pgDB{pool: pool}}, nil
}

// Close releases the connections.
func (s *Store) Close() error { return s.db.Close() }

// classify makes a Permanent error of one the database will repeat (a SQLSTATE of class 42: no grant, no table)
// and a Retryable error of the rest. An error that already has a kind keeps it.
func classify(err error) error {
	var te contracts.TypedError
	if errors.As(err, &te) {
		return err
	}
	var pe *pgconn.PgError
	if errors.As(err, &pe) && strings.HasPrefix(pe.Code, "42") {
		return permanent(err)
	}
	return retryable(err)
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

// pgxRunner is what a pool and a transaction both do.
type pgxRunner interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// pgQuerier is the pgx adapter of querier.
type pgQuerier struct{ r pgxRunner }

func (p pgQuerier) Select(ctx context.Context, sql string, args []any, scan func(scanner) error) error {
	rows, err := p.r.Query(ctx, sql, args...)
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

func (p pgQuerier) Exec(ctx context.Context, sql string, args ...any) error {
	_, err := p.r.Exec(ctx, sql, args...)
	return err
}

func (d pgDB) Select(ctx context.Context, sql string, args []any, scan func(scanner) error) error {
	return pgQuerier{d.pool}.Select(ctx, sql, args, scan)
}

func (d pgDB) Exec(ctx context.Context, sql string, args ...any) error {
	return pgQuerier{d.pool}.Exec(ctx, sql, args...)
}

func (d pgDB) InTx(ctx context.Context, fn func(querier) error) error {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(pgQuerier{tx}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (d pgDB) Close() error {
	d.pool.Close()
	return nil
}
