// Package gatewaystatepg is the Postgres implementation of gatewaystate.Reader and gatewaystate.Writer over pgx
// (docs/design/gateway-down-since.md, entry 4, Decision 2). It writes ops.gateway_state (role ot_runner) and reads
// the view rd.gateway_state_r1 only (role ot_ro_pages). It reads no wall clock: every stored time is the at of the
// caller.
package gatewaystatepg

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/andrewrong/optdesk/internal/store/gatewaystate"
)

const (
	// observeSQL arguments: alias, down, at. One statement, so two concurrent passes cannot split a row. A new row
	// has since and observed_at equal to at. On conflict, since moves to at only when down changes; observed_at is
	// set to at; the WHERE leaves a row alone when at is earlier than its observed_at (a late tick never moves
	// since or observed_at back). The right sides of SET read the old row.
	observeSQL = `INSERT INTO ops.gateway_state AS g (alias, down, since, observed_at)
VALUES ($1, $2, $3::timestamptz, $3::timestamptz)
ON CONFLICT (alias) DO UPDATE SET
  since = CASE WHEN g.down <> EXCLUDED.down THEN EXCLUDED.since ELSE g.since END,
  down = EXCLUDED.down,
  observed_at = EXCLUDED.observed_at
WHERE g.observed_at <= EXCLUDED.observed_at`
	// selectSQL reads the view, ordered by alias.
	selectSQL = `SELECT alias, down, since, observed_at FROM rd.gateway_state_r1 ORDER BY alias`
)

// database is the seam under the store: the one place pgx is touched, replaced by a fake in unit tests.
type database interface {
	Exec(ctx context.Context, sql string, args ...any) error
	// Select runs a query and calls scan once per row, in the order the database returns them.
	Select(ctx context.Context, sql string, row func(scan func(dest ...any) error) error) error
	Close() error
}

// Store is the gateway state store; it is both a Reader and a Writer.
type Store struct{ db database }

var (
	_ gatewaystate.Reader         = (*Store)(nil)
	_ gatewaystate.Writer         = (*Store)(nil)
	_ gatewaystate.OpenReaderFunc = OpenReader
	_ gatewaystate.OpenWriterFunc = OpenWriter
)

// Open connects to the database named by dsn. An empty dsn is an error and nothing is dialed. It applies no
// migration.
func Open(ctx context.Context, dsn string) (*Store, error) {
	if dsn == "" {
		return nil, errors.New("gatewaystatepg: open: empty dsn")
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("gatewaystatepg: open: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("gatewaystatepg: ping: %w", err)
	}
	return &Store{db: pgDB{pool: pool}}, nil
}

// OpenReader is a gatewaystate.OpenReaderFunc.
func OpenReader(ctx context.Context, dsn string) (gatewaystate.Reader, error) {
	s, err := Open(ctx, dsn)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// OpenWriter is a gatewaystate.OpenWriterFunc.
func OpenWriter(ctx context.Context, dsn string) (gatewaystate.Writer, error) {
	s, err := Open(ctx, dsn)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// Close releases the connections.
func (s *Store) Close() error { return s.db.Close() }

// States returns every row, ordered by alias; no rows gives an empty non-nil slice.
func (s *Store) States(ctx context.Context) ([]gatewaystate.State, error) {
	out := []gatewaystate.State{}
	err := s.db.Select(ctx, selectSQL, func(scan func(dest ...any) error) error {
		var st gatewaystate.State
		if err := scan(&st.Alias, &st.Down, &st.Since, &st.ObservedAt); err != nil {
			return err
		}
		out = append(out, st)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("gatewaystatepg: read states: %w", err)
	}
	return out, nil
}

// Observe records one observation of alias at the time at, by the rule of Decision 2.
func (s *Store) Observe(ctx context.Context, alias string, down bool, at time.Time) error {
	if err := s.db.Exec(ctx, observeSQL, alias, down, at); err != nil {
		return fmt.Errorf("gatewaystatepg: observe %s: %w", alias, err)
	}
	return nil
}

// pgDB is the pgx adapter of database.
type pgDB struct{ pool *pgxpool.Pool }

func (d pgDB) Exec(ctx context.Context, sql string, args ...any) error {
	_, err := d.pool.Exec(ctx, sql, args...)
	return err
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
