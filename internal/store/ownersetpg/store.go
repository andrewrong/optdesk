// Package ownersetpg implements ownerset.Store on PostgreSQL: the owner's rule parameters and option exclusions,
// insert-only and versioned (docs/design/settings-form.md Decision 7). Only cmd/ot imports it; pages and jobs
// import the declarations in internal/store/ownerset.
package ownersetpg

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/ownerset"
)

var _ ownerset.OpenFunc = Open

// Plain INSERTs: the tables are insert-only and set_at is the database's now().
const (
	insertParamSQL     = `INSERT INTO settings.rule_param (param_key, value) VALUES ($1, $2)`
	insertExclusionSQL = `INSERT INTO settings.option_exclusion (instrument_id, excluded) VALUES ($1, $2)`
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

// Store is the settings store: an ownerset.Store.
type Store struct{ db database }

// Open connects to the database named by dsn (the line ownerset.DSNEnvSettings, passed in by the caller) and
// returns the store. It applies no migration: the schema is made by goose with the role ot_owner.
func Open(ctx context.Context, dsn string) (ownerset.Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, permanent(fmt.Errorf("ownersetpg: open: %w", err))
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, retryable(fmt.Errorf("ownersetpg: ping: %w", err))
	}
	return &Store{db: pgDB{pool: pool}}, nil
}

// Close releases the connections.
func (s *Store) Close() error { return s.db.Close() }

// Apply reads the values in force and inserts one row for each field that differs, all in one transaction, and
// returns the number of rows inserted. A parameter with no row is in force as the empty value (the code default)
// and an instrument with no row as not excluded, so those fields insert nothing when set to that. An error,
// from the database or from no grant (a Permanent error), leaves no row behind.
func (s *Store) Apply(ctx context.Context, c ownerset.Change) (int, error) {
	if len(c.Params) == 0 && len(c.Exclusions) == 0 {
		return 0, nil
	}
	n := 0
	err := s.db.InTx(ctx, func(q querier) error {
		n = 0
		params, err := readCurrentParams(ctx, q)
		if err != nil {
			return err
		}
		excl, err := readCurrentExclusions(ctx, q)
		if err != nil {
			return err
		}
		for _, p := range c.Params {
			if params[p.Key] == p.Value {
				continue
			}
			if err := q.Exec(ctx, insertParamSQL, p.Key, p.Value); err != nil {
				return err
			}
			params[p.Key] = p.Value
			n++
		}
		for _, e := range c.Exclusions {
			if excl[e.InstrumentID] == e.Excluded {
				continue
			}
			if err := q.Exec(ctx, insertExclusionSQL, e.InstrumentID, e.Excluded); err != nil {
				return err
			}
			excl[e.InstrumentID] = e.Excluded
			n++
		}
		return nil
	})
	if err != nil {
		return 0, classify(fmt.Errorf("ownersetpg: apply: %w", err))
	}
	return n, nil
}

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
