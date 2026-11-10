package candidate

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
	"github.com/andrewrong/optdesk/internal/store"
)

// columns are the columns of the table and of the view r1, in the order of the argument list of a row and of the
// scan of a row.
const columns = `session_day, run_id, kind, instrument_id, alias, rule_version, issuer, underlying_id, expiry, dte, ` +
	`option_right, strike, multiplier, bid_eod, ask, spot, buy_price, sale_price, break_even, annualized_yield, ` +
	`delta, iv, status, gates, marks, fits, next_earnings, earnings_state, next_ex_date, ex_amount, extrinsic, ` +
	`rank, rank_in_underlying, computed_at, settings_version`

const columnCount = 35

// insertSQL is a plain INSERT: the table is insert-only and a duplicate key is an error.
var insertSQL = "INSERT INTO candidates.option_candidate (" + columns + ") VALUES (" + placeholders(columnCount) + ")"

func placeholders(n int) string {
	s := ""
	for i := 1; i <= n; i++ {
		if i > 1 {
			s += ", "
		}
		s += fmt.Sprintf("$%d", i)
	}
	return s
}

// scanner reads the columns of the current row.
type scanner interface {
	Scan(dest ...any) error
}

// database is the seam under the store: the one place pgx is touched, replaced by a fake in unit tests.
type database interface {
	// InsertAll runs one statement once per argument list, all in one transaction.
	InsertAll(ctx context.Context, sql string, rows [][]any) error
	// Select runs a query and calls scan once per row, in the order the database returns them.
	Select(ctx context.Context, sql string, args []any, scan func(scanner) error) error
	Close() error
}

// Store is the candidate store: a store.CandidateWriter and a store.CandidateReader.
type Store struct{ db database }

// Open connects to the database named by dsn (the line store.DSNEnvCandidate, passed in by the caller) and
// returns the store. It applies no migration: the schema is made by goose with the role ot_owner.
func Open(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, permanent(fmt.Errorf("candidate: open: %w", err))
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, retryable(fmt.Errorf("candidate: ping: %w", err))
	}
	return &Store{db: pgDB{pool: pool}}, nil
}

// Close releases the connections.
func (s *Store) Close() error { return s.db.Close() }

// InsertCandidates inserts the rows in one transaction; none of them is replaced. No rows inserts none and makes
// no database call. A row with no session day, run id or kind, or with an unknown kind, fails the call before
// anything is written. A constraint violation of the database (SQLSTATE class 23: duplicate key, not null, foreign
// key, check) is a Permanent error, since the same rows fail again; every other database error is Retryable.
func (s *Store) InsertCandidates(ctx context.Context, rows []store.Candidate) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	args := make([][]any, 0, len(rows))
	for i, c := range rows {
		a, err := rowArgs(c)
		if err != nil {
			return 0, fmt.Errorf("candidate: row %d: %w", i, err)
		}
		args = append(args, a)
	}
	if err := s.db.InsertAll(ctx, insertSQL, args); err != nil {
		err = fmt.Errorf("candidate: insert: %w", err)
		if isConstraintViolation(err) {
			return 0, permanent(err)
		}
		return 0, retryable(err)
	}
	return len(rows), nil
}

func rowArgs(c store.Candidate) ([]any, error) {
	day, err := parseDay(c.SessionDay)
	if err != nil {
		return nil, err
	}
	if c.RunID == "" {
		return nil, permanent(fmt.Errorf("empty run id"))
	}
	if !validKind(c.Kind) {
		return nil, permanent(fmt.Errorf("unknown kind %q", c.Kind))
	}
	expiry, err := parseDay(c.Expiry)
	if err != nil {
		return nil, err
	}
	next, err := parseOptDay(c.NextEarnings)
	if err != nil {
		return nil, err
	}
	ex, err := parseOptDay(c.NextExDate)
	if err != nil {
		return nil, err
	}
	return []any{day, c.RunID, c.Kind, c.InstrumentID, c.Alias, c.RuleVersion, c.Issuer, c.UnderlyingID, expiry, c.DTE,
		c.Right, c.Strike, c.Multiplier, c.BidEod, c.Ask, c.Spot, c.BuyPrice, c.SalePrice, c.BreakEven,
		c.AnnualizedYield, c.Delta, c.Iv, c.Status, c.Gates, c.Marks, c.Fits, next, c.EarningsState, ex,
		c.ExAmount, c.Extrinsic, c.Rank, c.RankInUnderlying, c.ComputedAt, c.SettingsVersion}, nil
}

func validKind(k string) bool {
	return k == "short_put" || k == "covered_call" || k == "held_call_guard"
}

// parseDay turns a contracts.Day into the date it names.
func parseDay(d contracts.Day) (time.Time, error) {
	t, err := time.Parse(time.DateOnly, string(d))
	if err != nil {
		return time.Time{}, permanent(fmt.Errorf("candidate: bad day %q: %w", d, err))
	}
	return t, nil
}

func parseOptDay(d *contracts.Day) (*time.Time, error) {
	if d == nil {
		return nil, nil
	}
	t, err := parseDay(*d)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// typedErr is a contracts.TypedError wrapping a cause.
type typedErr struct {
	kind  contracts.ErrorKind
	cause error
}

func (e *typedErr) Error() string             { return e.cause.Error() }
func (e *typedErr) Unwrap() error             { return e.cause }
func (e *typedErr) Kind() contracts.ErrorKind { return e.kind }

// isConstraintViolation reports whether err holds a PostgreSQL error of SQLSTATE class 23.
func isConstraintViolation(err error) bool {
	var pe *pgconn.PgError
	return errors.As(err, &pe) && strings.HasPrefix(pe.Code, "23")
}

func retryable(err error) error { return &typedErr{kind: contracts.Retryable, cause: err} }
func permanent(err error) error { return &typedErr{kind: contracts.Permanent, cause: err} }

// pgDB is the pgx adapter of database.
type pgDB struct{ pool *pgxpool.Pool }

func (d pgDB) InsertAll(ctx context.Context, sql string, rows [][]any) error {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var b pgx.Batch
	for _, r := range rows {
		b.Queue(sql, r...)
	}
	res := tx.SendBatch(ctx, &b)
	for range rows {
		if _, err := res.Exec(); err != nil {
			_ = res.Close()
			return err
		}
	}
	if err := res.Close(); err != nil {
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
