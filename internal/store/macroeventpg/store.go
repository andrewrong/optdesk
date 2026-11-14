// Package macroeventpg implements macroevent.Writer and macroevent.Reader on PostgreSQL: the table obs.macro_event
// (docs/design/macro-events.md Decisions 4 and 5). Only cmd/ot imports it; jobs and pages import the declarations in
// internal/store/macroevent.
package macroeventpg

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/macroevent"
)

var (
	_ macroevent.OpenWriterFunc = OpenWriter
	_ macroevent.OpenReaderFunc = OpenReader
)

const (
	// latestSQL arguments: kind, source. The latest stored date of the kind and source, empty when none.
	latestSQL = `SELECT coalesce(max(event_date)::text, '') FROM obs.macro_event WHERE kind = $1 AND source = $2`

	// upsertSQL arguments: kind, event_date, source, source_ref, run_day. A row that already holds the same source,
	// source_ref and run_day is not touched, so a repeated call changes no row and counts 0.
	upsertSQL = `INSERT INTO obs.macro_event (kind, event_date, source, source_ref, run_day) ` +
		`VALUES ($1, $2::date, $3, $4, $5::date) ON CONFLICT (kind, event_date) DO UPDATE ` +
		`SET source = EXCLUDED.source, source_ref = EXCLUDED.source_ref, run_day = EXCLUDED.run_day ` +
		`WHERE (obs.macro_event.source, obs.macro_event.source_ref, obs.macro_event.run_day) ` +
		`IS DISTINCT FROM (EXCLUDED.source, EXCLUDED.source_ref, EXCLUDED.run_day)`

	// deleteSQL arguments: kind, source, run_day, the dates of the answer. Only rows after the run day and not in the
	// answer.
	deleteSQL = `DELETE FROM obs.macro_event WHERE kind = $1 AND source = $2 AND event_date > $3::date ` +
		`AND event_date <> ALL($4::date[])`

	// holdSQL arguments: kind, run_day. Marks the stored rows of the kind after the run day as kept.
	holdSQL = `UPDATE obs.macro_event SET held_day = $2::date WHERE kind = $1 AND event_date > $2::date ` +
		`AND held_day IS DISTINCT FROM $2::date`

	// clearSQL arguments: kind. A delete that passed clears the hold of the kind.
	clearSQL = `UPDATE obs.macro_event SET held_day = NULL WHERE kind = $1 AND held_day IS NOT NULL`

	// betweenSQL arguments: from, to (inclusive).
	betweenSQL = `SELECT kind, source, source_ref, event_date::text, run_day::text FROM obs.macro_event ` +
		`WHERE event_date BETWEEN $1::date AND $2::date ORDER BY event_date, kind`

	kindsSQL = `SELECT kind, source, max(run_day)::text, max(event_date)::text, coalesce(max(held_day)::text, ''), ` +
		`count(*) FROM obs.macro_event GROUP BY kind, source ORDER BY kind, source`
)

// tx runs statements inside one transaction.
type tx interface {
	Exec(ctx context.Context, sql string, args ...any) (int64, error)
	Select(ctx context.Context, sql string, args []any, scan func(scanner) error) error
}

// scanner reads the columns of the current row.
type scanner interface {
	Scan(dest ...any) error
}

// database is the seam under the store: the one place pgx is touched, replaced by a fake in unit tests.
type database interface {
	// InTx runs fn in one transaction: committed when fn returns nil, rolled back otherwise.
	InTx(ctx context.Context, fn func(tx) error) error
	// Select runs a query and calls scan once per row, in the order the database returns them.
	Select(ctx context.Context, sql string, args []any, scan func(scanner) error) error
	Close() error
}

type store struct{ db database }

// OpenWriter connects with dsn and returns the Writer and its Closer. It applies no migration.
func OpenWriter(ctx context.Context, dsn string) (macroevent.Writer, io.Closer, error) {
	s, err := open(ctx, dsn)
	if err != nil {
		return nil, nil, err
	}
	return s, s, nil
}

// OpenReader connects with dsn and returns the Reader and its Closer. It applies no migration.
func OpenReader(ctx context.Context, dsn string) (macroevent.Reader, io.Closer, error) {
	s, err := open(ctx, dsn)
	if err != nil {
		return nil, nil, err
	}
	return s, s, nil
}

func open(ctx context.Context, dsn string) (*store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, permanent(fmt.Errorf("macroeventpg: open: %w", err))
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, retryable(fmt.Errorf("macroeventpg: ping: %w", err))
	}
	return &store{db: pgDB{pool: pool}}, nil
}

// Close releases the connections.
func (s *store) Close() error { return s.db.Close() }

// ReplaceFuture upserts the events and applies the delete rules of Decision 5 in one transaction. A malformed day, or
// an event of another kind or source, is a Permanent error and nothing is written.
func (s *store) ReplaceFuture(ctx context.Context, kind, source string, runDay contracts.Day,
	events []macroevent.Event) (macroevent.Outcome, error) {
	var out macroevent.Outcome
	if kind == "" || (source != "fred" && source != "file") {
		return out, permanent(errors.New("macroeventpg: a kind and a source of fred or file are needed"))
	}
	if err := checkDay(string(runDay)); err != nil {
		return out, err
	}
	dates := make([]string, 0, len(events))
	latest := ""
	for _, e := range events {
		if e.Kind != kind || e.Source != source {
			return out, permanent(fmt.Errorf("macroeventpg: event %s/%s is not of %s/%s", e.Kind, e.Source, kind, source))
		}
		if err := checkDay(string(e.Date)); err != nil {
			return out, err
		}
		if e.RunDay != "" {
			if err := checkDay(string(e.RunDay)); err != nil {
				return out, err
			}
		}
		dates = append(dates, string(e.Date))
		if string(e.Date) > latest {
			latest = string(e.Date)
		}
	}
	err := s.db.InTx(ctx, func(t tx) error {
		out = macroevent.Outcome{}
		stored := ""
		if err := t.Select(ctx, latestSQL, []any{kind, source}, func(sc scanner) error { return sc.Scan(&stored) }); err != nil {
			return err
		}
		for _, e := range events {
			rd := string(e.RunDay)
			if rd == "" {
				rd = string(runDay)
			}
			n, err := t.Exec(ctx, upsertSQL, kind, string(e.Date), source, e.SourceRef, rd)
			if err != nil {
				return err
			}
			out.Written += int(n)
		}
		// Rule (a): an empty answer deletes nothing. Rule (b): an answer that ends before the stored latest date
		// deletes nothing. Both are refusals and set the hold; rules (c) and (d) narrow the delete in deleteSQL.
		if len(events) == 0 || latest < stored {
			out.Held = true
			_, err := t.Exec(ctx, holdSQL, kind, string(runDay))
			return err
		}
		n, err := t.Exec(ctx, deleteSQL, kind, source, string(runDay), dates)
		if err != nil {
			return err
		}
		out.Deleted = int(n)
		_, err = t.Exec(ctx, clearSQL, kind)
		return err
	})
	if err != nil {
		return macroevent.Outcome{}, wrap(err, "replace future")
	}
	return out, nil
}

// Between returns the events dated from..to inclusive, in date then kind order; an empty slice when none.
func (s *store) Between(ctx context.Context, from, to contracts.Day) ([]macroevent.Event, error) {
	if err := checkDay(string(from)); err != nil {
		return nil, err
	}
	if err := checkDay(string(to)); err != nil {
		return nil, err
	}
	out := []macroevent.Event{}
	err := s.db.Select(ctx, betweenSQL, []any{string(from), string(to)}, func(sc scanner) error {
		var e macroevent.Event
		var d, r string
		if err := sc.Scan(&e.Kind, &e.Source, &e.SourceRef, &d, &r); err != nil {
			return err
		}
		e.Date, e.RunDay = contracts.Day(d), contracts.Day(r)
		out = append(out, e)
		return nil
	})
	if err != nil {
		return nil, wrap(err, "between")
	}
	return out, nil
}

// Kinds returns one row per kind and source, in kind then source order; an empty slice when none.
func (s *store) Kinds(ctx context.Context) ([]macroevent.Kind, error) {
	out := []macroevent.Kind{}
	err := s.db.Select(ctx, kindsSQL, nil, func(sc scanner) error {
		var k macroevent.Kind
		var run, last, held string
		var rows int64
		if err := sc.Scan(&k.Kind, &k.Source, &run, &last, &held, &rows); err != nil {
			return err
		}
		k.LastRunDay, k.LastDate, k.HeldDay, k.Rows = contracts.Day(run), contracts.Day(last), contracts.Day(held), int(rows)
		out = append(out, k)
		return nil
	})
	if err != nil {
		return nil, wrap(err, "kinds")
	}
	return out, nil
}

func checkDay(d string) error {
	if _, err := time.Parse(time.DateOnly, d); err != nil {
		return permanent(fmt.Errorf("macroeventpg: bad day %q: %w", d, err))
	}
	return nil
}

// wrap makes a database error Retryable, except an integrity constraint violation (SQLSTATE class 23), which is
// Permanent; it keeps a typed error as it is.
func wrap(err error, what string) error {
	var te contracts.TypedError
	if errors.As(err, &te) {
		return err
	}
	var pe *pgconn.PgError
	if errors.As(err, &pe) && strings.HasPrefix(pe.Code, "23") {
		return permanent(fmt.Errorf("macroeventpg: %s: %w", what, err))
	}
	return retryable(fmt.Errorf("macroeventpg: %s: %w", what, err))
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

func (t pgTx) Select(ctx context.Context, sql string, args []any, scan func(scanner) error) error {
	rows, err := t.tx.Query(ctx, sql, args...)
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

func (d pgDB) InTx(ctx context.Context, fn func(tx) error) error {
	t, err := d.pool.Begin(ctx)
	if err != nil {
		return err
	}
	if err := fn(pgTx{tx: t}); err != nil {
		if rerr := t.Rollback(context.WithoutCancel(ctx)); rerr != nil && !errors.Is(rerr, pgx.ErrTxClosed) {
			err = errors.Join(err, rerr)
		}
		return err
	}
	return t.Commit(ctx)
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
