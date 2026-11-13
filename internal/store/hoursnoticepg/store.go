// Package hoursnoticepg is the Postgres implementation of hoursnotice.Store over pgx, for the DSN line of the role
// ot_w_master (docs/design/calendar-ibkr-hours.md Decisions 6, 7 and 9). It writes master.session_hours_notice and
// reads the view rd.session_hours_notice_r1 only. Replace deletes a market's rows and inserts the given ones in one
// transaction.
package hoursnoticepg

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/andrewrong/optdesk/internal/store/hoursnotice"
)

const (
	// deleteSQL argument: market.
	deleteSQL = `DELETE FROM master.session_hours_notice WHERE market = $1`
	// insertSQL arguments: market, day, ours, ibkr.
	insertSQL = `INSERT INTO master.session_hours_notice (market, day, ours, ibkr) VALUES ($1, $2::date, $3, $4)`
	// selectSQL reads the view, ordered by market and day.
	selectSQL = `SELECT market, to_char(day, 'YYYY-MM-DD'), ours, ibkr FROM rd.session_hours_notice_r1 ORDER BY market, day`
)

// execer runs one statement inside a transaction.
type execer interface {
	Exec(ctx context.Context, sql string, args ...any) error
}

// database is the seam under the store: the one place pgx is touched, replaced by a fake in unit tests.
type database interface {
	// InTx runs fn in one transaction: committed when fn returns nil, rolled back otherwise.
	InTx(ctx context.Context, fn func(execer) error) error
	// Select runs a query and calls scan once per row, in the order the database returns them.
	Select(ctx context.Context, sql string, row func(scan func(dest ...any) error) error) error
	Close() error
}

// Store is the hours-notice store.
type Store struct{ db database }

var _ hoursnotice.Store = (*Store)(nil)

// Open connects to the database named by dsn. It applies no migration.
func Open(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("hoursnoticepg: open: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("hoursnoticepg: ping: %w", err)
	}
	return &Store{db: pgDB{pool: pool}}, nil
}

// Close releases the connections.
func (s *Store) Close() error { return s.db.Close() }

// Notices returns every row, ordered by market and day.
func (s *Store) Notices(ctx context.Context) ([]hoursnotice.Notice, error) {
	out := []hoursnotice.Notice{}
	err := s.db.Select(ctx, selectSQL, func(scan func(dest ...any) error) error {
		var n hoursnotice.Notice
		if err := scan(&n.Market, &n.Day, &n.Ours, &n.IBKR); err != nil {
			return err
		}
		out = append(out, n)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("hoursnoticepg: read notices: %w", err)
	}
	return out, nil
}

// Replace deletes the rows of market and inserts rows in one transaction. A row whose Market is not market is
// refused before any statement runs.
func (s *Store) Replace(ctx context.Context, market string, rows []hoursnotice.Notice) error {
	for _, r := range rows {
		if r.Market != market {
			return fmt.Errorf("hoursnoticepg: replace %s: row of market %q", market, r.Market)
		}
	}
	err := s.db.InTx(ctx, func(tx execer) error {
		if err := tx.Exec(ctx, deleteSQL, market); err != nil {
			return err
		}
		for _, r := range rows {
			if err := tx.Exec(ctx, insertSQL, r.Market, r.Day, r.Ours, r.IBKR); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("hoursnoticepg: replace %s: %w", market, err)
	}
	return nil
}

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

func (d pgDB) Select(ctx context.Context, sql string, row func(scan func(dest ...any) error) error) error {
	rows, err := d.pool.Query(ctx, sql)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := row(rows.Scan); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (d pgDB) Close() error {
	d.pool.Close()
	return nil
}
