// Package events holds the earnings store: OpenEarningsDate (store.OpenEarningsDateStoreFunc) over the table
// obs.earnings_date (insert) and the view rd.earnings_date_r1 (every read).
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

var _ store.OpenEarningsDateStoreFunc = OpenEarningsDate

// No conflict target: naming one needs SELECT on the table, which no role holds; the primary key is the only
// unique index, so DO NOTHING acts on (observed_on, issuer, attempt).
// Dates travel as text and are cast in SQL, so no driver date range matters.
const (
	insertEarningsSQL = `INSERT INTO obs.earnings_date
 (observed_on, issuer, attempt, symbol, market, status, report_date, fiscal_period_end, time_of_day,
  estimate, error_code, source, received_at)
VALUES ($1::text::date, $2, $3, $4, $5, $6, $7::text::date, $8::text::date, $9, $10, $11, $12, $13)
ON CONFLICT DO NOTHING`

	selectEarningsSQL = `SELECT observed_on, issuer, attempt, symbol, market, status, report_date, fiscal_period_end,
 time_of_day, estimate, error_code, source, received_at
FROM rd.earnings_date_r1 WHERE issuer = $1 AND observed_on >= $2::text::date AND observed_on <= $3::text::date
ORDER BY observed_on, attempt`
)

// scanner reads the columns of the current row.
type scanner interface {
	Scan(dest ...any) error
}

// database is the seam under the store: the one place pgx is touched, replaced by a fake in unit tests.
type database interface {
	// Exec runs one statement.
	Exec(ctx context.Context, sql string, args ...any) error
	// Select runs a query and calls scan once per row, in the order the database returns them.
	Select(ctx context.Context, sql string, args []any, scan func(scanner) error) error
	Close() error
}

type earningsStore struct{ db database }

// OpenEarningsDate connects to the database named by dsn and returns the earnings store.
// It applies no migration: the schema is made by goose with the role ot_owner. The DSN is that of
// ot_w_events (insert and read) or ot_ro_pages (read only).
func OpenEarningsDate(ctx context.Context, dsn string) (store.EarningsDateStore, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, permanent(fmt.Errorf("events: open: %w", err))
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, retryable(fmt.Errorf("events: ping: %w", err))
	}
	return &earningsStore{db: pgDB{pool: pool}}, nil
}

// Close releases the connections.
func (s *earningsStore) Close() error { return s.db.Close() }

// InsertEarningsDate inserts the row; a row whose key exists stays as it is and is no error.
func (s *earningsStore) InsertEarningsDate(ctx context.Context, r contracts.EarningsDate) error {
	if r.Issuer == "" || r.Attempt < 1 {
		return permanent(fmt.Errorf("events: a row needs an issuer and an attempt from 1"))
	}
	err := s.db.Exec(ctx, insertEarningsSQL,
		r.ObservedOn.Format(time.DateOnly), r.Issuer, r.Attempt, r.Symbol, string(r.Market), string(r.Status),
		dayText(r.ReportDate), dayText(r.FiscalPeriodEnd), r.TimeOfDay, r.Estimate, r.ErrorCode, string(r.Source),
		r.ReceivedAt.UTC())
	if err != nil {
		return retryable(fmt.Errorf("events: insert: %w", err))
	}
	return nil
}

func dayText(d *types.SerializableDate) *string {
	if d == nil {
		return nil
	}
	s := d.Format(time.DateOnly)
	return &s
}

// EarningsDates returns the rows of the issuer from from to to, inclusive, ascending by day, the highest attempt
// of each day only, read from the view.
func (s *earningsStore) EarningsDates(ctx context.Context, issuer string, from, to contracts.Day) ([]contracts.EarningsDate, error) {
	f, err := time.Parse(time.DateOnly, string(from))
	if err != nil {
		return nil, permanent(fmt.Errorf("events: bad from %q: %w", from, err))
	}
	t, err := time.Parse(time.DateOnly, string(to))
	if err != nil {
		return nil, permanent(fmt.Errorf("events: bad to %q: %w", to, err))
	}
	if t.Before(f) {
		return nil, permanent(fmt.Errorf("events: range %s to %s is reversed", from, to))
	}
	out := []contracts.EarningsDate{}
	err = s.db.Select(ctx, selectEarningsSQL, []any{issuer, string(from), string(to)}, func(sc scanner) error {
		r, err := scanRow(sc)
		if err != nil {
			return err
		}
		// Rows come ordered by day then attempt, so a later row of the same day has the higher attempt.
		if n := len(out); n > 0 && out[n-1].ObservedOn.Equal(r.ObservedOn.Time) {
			out[n-1] = r
		} else {
			out = append(out, r)
		}
		return nil
	})
	if err != nil {
		return nil, retryable(fmt.Errorf("events: select: %w", err))
	}
	return out, nil
}

func scanRow(sc scanner) (contracts.EarningsDate, error) {
	var r contracts.EarningsDate
	var observed, received time.Time
	var market, status, source string
	var report, fiscal *time.Time
	if err := sc.Scan(&observed, &r.Issuer, &r.Attempt, &r.Symbol, &market, &status, &report, &fiscal,
		&r.TimeOfDay, &r.Estimate, &r.ErrorCode, &source, &received); err != nil {
		return r, err
	}
	// The enum types live in the generated package, which this one does not import: their UnmarshalJSON
	// sets the value and refuses a word the dataset does not list.
	for _, e := range []struct {
		word string
		to   any
	}{{market, &r.Market}, {status, &r.Status}, {source, &r.Source}} {
		if err := json.Unmarshal([]byte(strconv.Quote(e.word)), e.to); err != nil {
			return r, err
		}
	}
	r.ObservedOn = types.SerializableDate{Time: observed}
	if report != nil {
		r.ReportDate = &types.SerializableDate{Time: *report}
	}
	if fiscal != nil {
		r.FiscalPeriodEnd = &types.SerializableDate{Time: *fiscal}
	}
	r.ReceivedAt = received.UTC()
	return r, nil
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

func (d pgDB) Exec(ctx context.Context, sql string, args ...any) error {
	_, err := d.pool.Exec(ctx, sql, args...)
	return err
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
