// Package barpg is the bar store over pgx for the DSN line OT_PG_DSN_BAR (role ot_w_bar): the writer of obs.price_bar
// and obs.fx_rate, and the reader of the views rd.price_bar_r1 and rd.fx_rate_r1
// (docs/design/market-bars.md Decisions 4 and 8). It imports contracts and the interface package internal/store/bar (for bar.FxWriteCount); its test asserts
// that *Store satisfies bar.BarStore. obs.price_bar is insert-only; obs.fx_rate is upserted by UpsertFxRates
// (docs/design/data-repair-fx.md Decisions 3 and 4). No statement of this package deletes a row.
package barpg

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
	"github.com/andrewrong/optdesk/internal/store/bar"
)

// The price bar and InsertFxRates statements append: a key that exists is left as it is and adds nothing.
const (
	// insertPriceBarSQL arguments: instrument_id, session_day, basis, adjust_epoch, open, high, low, close, volume,
	// currency, source, received_at.
	insertPriceBarSQL = `INSERT INTO obs.price_bar (instrument_id, session_day, basis, adjust_epoch, open, high, low, ` +
		`close, volume, currency, source, received_at) ` +
		`VALUES ($1, $2::date, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12) ON CONFLICT DO NOTHING`
	// insertFxRateSQL arguments: pair, rate_day, rate, source, received_at.
	insertFxRateSQL = `INSERT INTO obs.fx_rate (pair, rate_day, rate, source, received_at) ` +
		`VALUES ($1, $2::date, $3, $4, $5) ON CONFLICT DO NOTHING`
)

// upsertFxRateSQL arguments: pair, rate_day, rate, source, received_at, written_by. A stored row is replaced only when
// its rate (numeric equality), source or status differs; received_at and the key are never set.
const upsertFxRateSQL = `INSERT INTO obs.fx_rate (pair, rate_day, rate, source, received_at, status, updated_at, written_by) ` +
	`VALUES ($1, $2::date, $3::numeric, $4, $5, 'ok', now(), $6) ON CONFLICT (pair, rate_day) DO UPDATE SET ` +
	`rate = EXCLUDED.rate, source = EXCLUDED.source, status = 'ok', updated_at = EXCLUDED.updated_at, ` +
	`written_by = EXCLUDED.written_by WHERE (obs.fx_rate.rate, obs.fx_rate.source, obs.fx_rate.status) ` +
	`IS DISTINCT FROM (EXCLUDED.rate, EXCLUDED.source, EXCLUDED.status) RETURNING pair`

// execer runs one statement inside a transaction and reports the rows it added.
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

// Store is the bar store: the writers and readers of obs.price_bar and obs.fx_rate.
type Store struct{ db database }

// Open connects to the database named by dsn. It applies no migration: the schema is made by goose with the role
// ot_owner.
func Open(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, permanent(fmt.Errorf("barpg: open: %w", err))
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, retryable(fmt.Errorf("barpg: ping: %w", err))
	}
	return &Store{db: pgDB{pool: pool}}, nil
}

// Close releases the connections.
func (s *Store) Close() error { return s.db.Close() }

// insertAll runs one statement per row in one transaction (all or none) and returns the rows added. An empty input
// makes no database call.
func insertAll[R any](ctx context.Context, db database, sql string, rows []R, argsOf func(R) []any, what string) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	inserted := 0
	err := db.InTx(ctx, func(tx execer) error {
		inserted = 0
		for _, r := range rows {
			n, err := tx.Exec(ctx, sql, argsOf(r)...)
			if err != nil {
				return err
			}
			inserted += int(n)
		}
		return nil
	})
	if err != nil {
		return 0, wrap(err, what)
	}
	return inserted, nil
}

// InsertPriceBars writes the rows in one transaction (all or none) and returns the number inserted; a key that
// exists adds none. An empty input makes no database call.
func (s *Store) InsertPriceBars(ctx context.Context, rows []contracts.PriceBar) (int, error) {
	return insertAll(ctx, s.db, insertPriceBarSQL, rows, func(r contracts.PriceBar) []any {
		return []any{int64(r.InstrumentID), r.SessionDay.Format(time.DateOnly), string(r.Basis), int32(r.AdjustEpoch),
			r.Open, r.High, r.Low, r.Close, r.Volume, r.Currency, string(r.Source), r.ReceivedAt}
	}, "insert price bars")
}

// InsertFxRates writes the rows in one transaction and returns the number inserted; a key that exists adds none.
// An empty input makes no database call.
func (s *Store) InsertFxRates(ctx context.Context, rows []contracts.FxRate) (int, error) {
	return insertAll(ctx, s.db, insertFxRateSQL, rows, func(r contracts.FxRate) []any {
		return []any{r.Pair, r.RateDay.Format(time.DateOnly), r.Rate, string(r.Source), r.ReceivedAt}
	}, "insert fx rates")
}

var _ bar.FxRateUpserter = (*Store)(nil)

// UpsertFxRates writes the rows in one transaction, one statement per row: a differing value replaces the stored one,
// an equal one touches nothing, and no old value is kept. Written is the rows inserted or replaced, Unchanged the rows
// given minus those. An empty input makes no database call.
func (s *Store) UpsertFxRates(ctx context.Context, rows []contracts.FxRate, writtenBy string) (bar.FxWriteCount, error) {
	n, err := insertAll(ctx, s.db, upsertFxRateSQL, rows, func(r contracts.FxRate) []any {
		return []any{r.Pair, r.RateDay.Format(time.DateOnly), r.Rate, string(r.Source), r.ReceivedAt, writtenBy}
	}, "upsert fx rates")
	if err != nil {
		return bar.FxWriteCount{}, err
	}
	return bar.FxWriteCount{Written: n, Unchanged: len(rows) - n}, nil
}

// wrap keeps a typed error as it is, makes a constraint violation (SQLSTATE class 23) Permanent, because the
// database repeats it on every attempt, and makes every other database error Retryable.
func wrap(err error, what string) error {
	var te contracts.TypedError
	if errors.As(err, &te) {
		return err
	}
	if isConstraintViolation(err) {
		return permanent(fmt.Errorf("barpg: %s: %w", what, err))
	}
	return retryable(fmt.Errorf("barpg: %s: %w", what, err))
}

// isConstraintViolation reports whether err is a PostgreSQL integrity constraint violation (SQLSTATE class 23).
func isConstraintViolation(err error) bool {
	var pe *pgconn.PgError
	return errors.As(err, &pe) && strings.HasPrefix(pe.Code, "23")
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
