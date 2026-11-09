package account

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

var _ store.OpenAccountFunc = Open

// Every statement below only reads or appends: a retry of a run meets the primary key and adds nothing.
const (
	// insertCaptureSQL arguments: capture_id, alias, captured_at, session_day, status.
	insertCaptureSQL = `INSERT INTO account.capture (capture_id, alias, captured_at, session_day, status) ` +
		`VALUES ($1, $2, $3, $4::date, $5) ON CONFLICT DO NOTHING`
	// insertPositionSQL arguments: capture_id, alias, captured_at, session_day, symbol, sec_type, exchange, currency,
	// expiry, strike, multiplier, option_right, con_id, quantity, avg_cost.
	insertPositionSQL = `INSERT INTO account.position (capture_id, alias, captured_at, session_day, symbol, sec_type, ` +
		`exchange, currency, expiry, strike, multiplier, option_right, con_id, quantity, avg_cost) ` +
		`VALUES ($1, $2, $3, $4::date, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14::numeric, $15::numeric) ` +
		`ON CONFLICT DO NOTHING`
	// insertValueSQL arguments: capture_id, alias, captured_at, session_day, tag, value, currency.
	insertValueSQL = `INSERT INTO account.value (capture_id, alias, captured_at, session_day, tag, value, currency) ` +
		`VALUES ($1, $2, $3, $4::date, $5, $6::numeric, $7) ON CONFLICT DO NOTHING`
	// insertExecutionSQL arguments: alias, exec_id, perm_id, executed_at, session_day, symbol, sec_type, expiry, strike,
	// option_right, multiplier, currency, side, quantity, price, commission, commission_currency.
	insertExecutionSQL = `INSERT INTO account.execution (alias, exec_id, perm_id, executed_at, session_day, symbol, ` +
		`sec_type, expiry, strike, option_right, multiplier, currency, side, quantity, price, commission, ` +
		`commission_currency) VALUES ($1, $2, $3, $4, $5::date, $6, $7, $8, $9, $10, $11, $12, $13, ` +
		`$14::numeric, $15::numeric, $16::numeric, $17) ON CONFLICT DO NOTHING`
)

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

type accountStore struct{ db database }

// Open connects to the database named by dsn and returns the account store. It applies no migration: the schema is
// made by goose with the role ot_owner.
func Open(ctx context.Context, dsn string) (store.AccountStore, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, permanent(fmt.Errorf("account: open: %w", err))
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, retryable(fmt.Errorf("account: ping: %w", err))
	}
	return &accountStore{db: pgDB{pool: pool}}, nil
}

// Close releases the connections.
func (s *accountStore) Close() error { return s.db.Close() }

// WriteCapture writes the capture row and, when that row is new, its values and positions, in one transaction. A
// capture whose ID exists writes nothing and is no error. The first four fields of every child come from c.
func (s *accountStore) WriteCapture(ctx context.Context, c store.Capture) error {
	if c.ID == "" || c.Alias == "" {
		return permanent(errors.New("account: capture needs an id and an alias"))
	}
	if err := checkDay(c.SessionDay); err != nil {
		return err
	}
	err := s.db.InTx(ctx, func(tx execer) error {
		n, err := tx.Exec(ctx, insertCaptureSQL, c.ID, c.Alias, c.At, c.SessionDay, c.Status)
		if err != nil || n == 0 {
			return err
		}
		for _, v := range c.Values {
			if _, err := tx.Exec(ctx, insertValueSQL, c.ID, c.Alias, c.At, c.SessionDay, v.Tag, v.Value, v.Currency); err != nil {
				return err
			}
		}
		for _, p := range c.Positions {
			if _, err := tx.Exec(ctx, insertPositionSQL, c.ID, c.Alias, c.At, c.SessionDay, p.Symbol, p.SecType,
				p.Exchange, p.Currency, p.Expiry, p.Strike, p.Multiplier, p.Right, p.ConID, p.Quantity, p.AvgCost); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return wrap(err, "write capture")
	}
	return nil
}

// WriteExecutions inserts the rows under alias in one transaction; a row whose (alias, exec_id) exists stays as it is.
// An empty input makes no database call.
func (s *accountStore) WriteExecutions(ctx context.Context, alias string, rows []store.Execution) error {
	if alias == "" {
		return permanent(errors.New("account: executions need an alias"))
	}
	if len(rows) == 0 {
		return nil
	}
	for _, r := range rows {
		if err := checkDay(r.SessionDay); err != nil {
			return err
		}
	}
	err := s.db.InTx(ctx, func(tx execer) error {
		for _, r := range rows {
			if _, err := tx.Exec(ctx, insertExecutionSQL, alias, r.ExecID, r.PermID, r.ExecutedAt, r.SessionDay,
				r.Symbol, r.SecType, r.Expiry, r.Strike, r.Right, r.Multiplier, r.Currency, r.Side, r.Quantity,
				r.Price, r.Commission, r.CommissionCurrency); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return wrap(err, "write executions")
	}
	return nil
}

// checkDay accepts a YYYY-MM-DD day.
func checkDay(day string) error {
	if _, err := time.Parse(time.DateOnly, day); err != nil {
		return permanent(fmt.Errorf("account: bad day %q: %w", day, err))
	}
	return nil
}

// wrap makes a database error Retryable and keeps a typed error as it is.
func wrap(err error, what string) error {
	var te contracts.TypedError
	if errors.As(err, &te) {
		return err
	}
	return retryable(fmt.Errorf("account: %s: %w", what, err))
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
