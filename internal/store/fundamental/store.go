// Package fundamental holds the fact store: Open (store.OpenFactStoreFunc) over the table
// obs.fundamental_fact (insert) and the view rd.fundamental_fact_r1 (every read).
package fundamental

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

var _ store.OpenFactStoreFunc = Open

// noStart is the period_start_key of a fact with no period_start (an instant fact).
const noStart = "0001-01-01"

// No conflict target: naming one needs SELECT on the table, which no role holds; the primary key is the only
// unique index, so DO NOTHING acts on the eight-column key.
// Dates travel as text and are cast in SQL, so no driver date range matters.
const (
	insertFactSQL = `INSERT INTO obs.fundamental_fact
 (source, subject, doc_id, taxonomy, concept, unit, period_start_key, period_end,
  currency, value, form, filed, retrieved_at, raw_sha256)
VALUES ($1, $2, $3, $4, $5, $6, $7::text::date, $8::text::date, $9, $10, $11, $12::text::date, $13, $14)
ON CONFLICT DO NOTHING`

	selectFactsSQL = `SELECT source, subject, doc_id, taxonomy, concept, unit, period_start, period_end,
 currency, value, form, filed, retrieved_at, raw_sha256
FROM rd.fundamental_fact_r1 WHERE source = $1 AND subject = $2 AND filed <= $3::text::date
ORDER BY filed, doc_id, taxonomy, concept, unit, period_start NULLS FIRST, period_end`
)

// scanner reads the columns of the current row.
type scanner interface {
	Scan(dest ...any) error
}

// stmt is one statement with its arguments.
type stmt struct {
	sql  string
	args []any
}

// database is the seam under the store: the one place pgx is touched, replaced by a fake in unit tests.
type database interface {
	// ExecAll runs the statements in one transaction.
	ExecAll(ctx context.Context, stmts []stmt) error
	// Select runs a query and calls scan once per row, in the order the database returns them.
	Select(ctx context.Context, sql string, args []any, scan func(scanner) error) error
	Close() error
}

type factStore struct{ db database }

// Open connects to the database named by dsn and returns the fact store.
// It applies no migration: the schema is made by goose with the role ot_owner. The DSN is that of
// ot_w_fundamental (insert and read) or ot_ro_pages (read only).
func Open(ctx context.Context, dsn string) (store.FactStore, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, permanent(fmt.Errorf("fundamental: open: %w", err))
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, retryable(fmt.Errorf("fundamental: ping: %w", err))
	}
	return &factStore{db: pgDB{pool: pool}}, nil
}

// Close releases the connections.
func (s *factStore) Close() error { return s.db.Close() }

// InsertFacts appends the rows in one transaction; a row whose eight-column key exists is left as it is.
// A database error of SQLSTATE class 23 (a constraint violation) is Permanent, since it repeats on every
// attempt; every other error is Retryable.
func (s *factStore) InsertFacts(ctx context.Context, rows []contracts.FundamentalFact) error {
	stmts := make([]stmt, 0, len(rows))
	for _, r := range rows {
		if r.Subject == "" || r.DocID == "" || r.Source == "" {
			return permanent(fmt.Errorf("fundamental: a fact needs source, subject and doc_id"))
		}
		start := noStart
		if r.PeriodStart != nil {
			start = r.PeriodStart.Format(time.DateOnly)
		}
		stmts = append(stmts, stmt{insertFactSQL, []any{
			string(r.Source), r.Subject, r.DocID, r.Taxonomy, r.Concept, r.Unit, start, r.PeriodEnd.Format(time.DateOnly),
			r.Currency, r.Value, r.Form, r.Filed.Format(time.DateOnly), r.RetrievedAt.UTC(), r.RawSha256,
		}})
	}
	if len(stmts) == 0 {
		return nil
	}
	if err := s.db.ExecAll(ctx, stmts); err != nil {
		wrapped := fmt.Errorf("fundamental: insert: %w", err)
		if isConstraintViolation(err) {
			return permanent(wrapped)
		}
		return retryable(wrapped)
	}
	return nil
}

// FactsFiledBy returns every row of the source and subject filed on or before cutoff, read from the view only.
func (s *factStore) FactsFiledBy(ctx context.Context, source, subject string, cutoff contracts.Day) ([]contracts.FundamentalFact, error) {
	if _, err := time.Parse(time.DateOnly, string(cutoff)); err != nil {
		return nil, permanent(fmt.Errorf("fundamental: bad cutoff %q: %w", cutoff, err))
	}
	out := []contracts.FundamentalFact{}
	err := s.db.Select(ctx, selectFactsSQL, []any{source, subject, string(cutoff)}, func(sc scanner) error {
		var f contracts.FundamentalFact
		var source string
		var start *time.Time
		var end, filed time.Time
		if err := sc.Scan(&source, &f.Subject, &f.DocID, &f.Taxonomy, &f.Concept, &f.Unit, &start, &end,
			&f.Currency, &f.Value, &f.Form, &filed, &f.RetrievedAt, &f.RawSha256); err != nil {
			return err
		}
		// The enum type lives in the generated package, which this one does not import: its UnmarshalJSON
		// sets the value and refuses a source the dataset does not list.
		if err := json.Unmarshal([]byte(strconv.Quote(source)), &f.Source); err != nil {
			return err
		}
		if start != nil {
			f.PeriodStart = &types.SerializableDate{Time: *start}
		}
		f.PeriodEnd = types.SerializableDate{Time: end}
		f.Filed = types.SerializableDate{Time: filed}
		f.RetrievedAt = f.RetrievedAt.UTC()
		out = append(out, f)
		return nil
	})
	if err != nil {
		return nil, retryable(fmt.Errorf("fundamental: select: %w", err))
	}
	return out, nil
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

// isConstraintViolation reports whether err holds a PostgreSQL error of SQLSTATE class 23.
func isConstraintViolation(err error) bool {
	var pe *pgconn.PgError
	return errors.As(err, &pe) && strings.HasPrefix(pe.Code, "23")
}

// pgDB is the pgx adapter of database.
type pgDB struct{ pool *pgxpool.Pool }

func (d pgDB) ExecAll(ctx context.Context, stmts []stmt) error {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	for _, s := range stmts {
		if _, err := tx.Exec(ctx, s.sql, s.args...); err != nil {
			return err
		}
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
