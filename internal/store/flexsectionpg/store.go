// Package flexsectionpg is the PostgreSQL implementation of flexsection.Store: Open connects with the role
// ot_w_optionledger (INSERT and SELECT only) to the database of store.DSNEnvOptionLedger, appends the rows and
// statuses of the Flex sections to schema optionledger and reads them through the views
// rd.flex_section_row_r1 and rd.flex_section_status_r1. Nothing is changed or removed. It is the only code with
// SQL for these tables. Design: docs/design/account-flex-fx.md Decisions 6, 8 and 9.
package flexsectionpg

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/andrewrong/optdesk/internal/store/flexsection"
)

var _ flexsection.Store = (*Store)(nil)
var _ flexsection.OpenFunc = func(ctx context.Context, dsn string) (flexsection.Store, error) { return Open(ctx, dsn) }

// scanner reads the columns of the current row.
type scanner interface {
	Scan(dest ...any) error
}

// tx is one open transaction.
type tx interface {
	// Exec runs a statement and returns the number of rows it wrote.
	Exec(ctx context.Context, sql string, args ...any) (int64, error)
	// Select runs a query and calls scan once per row.
	Select(ctx context.Context, sql string, args []any, scan func(scanner) error) error
}

// database is the seam under the store: the one place pgx is touched, replaced by a fake in unit tests.
type database interface {
	// InTx runs fn in one transaction: committed when fn returns nil, rolled back otherwise.
	InTx(ctx context.Context, fn func(tx) error) error
	// Select runs a query and calls scan once per row, in the order the database returns them.
	Select(ctx context.Context, sql string, args []any, scan func(scanner) error) error
	Close() error
}

// Store is the Flex section store.
type Store struct{ db database }

// Open connects to the database named by dsn (the line store.DSNEnvOptionLedger, passed in by the caller). It
// applies no migration: the schema is made by goose with the role ot_owner.
func Open(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("flexsectionpg: open: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("flexsectionpg: ping: %w", err)
	}
	return &Store{db: pgDB{pool: pool}}, nil
}

// Close releases the connections.
func (s *Store) Close() error { return s.db.Close() }

// RecordSections writes the rows and statuses of the fetch in one transaction (see flexsection.Writer). The
// counts of Insert say what was written: a row a concurrent writer wrote first is not counted.
func (s *Store) RecordSections(ctx context.Context, fetchID string, rows []flexsection.Row, statuses []flexsection.Status) (flexsection.Insert, error) {
	if len(rows) == 0 && len(statuses) == 0 {
		return flexsection.Insert{}, nil
	}
	for i, r := range rows {
		if err := checkRow(r); err != nil {
			return flexsection.Insert{}, fmt.Errorf("flexsectionpg: row %d: %w", i, err)
		}
	}
	var out flexsection.Insert
	err := s.db.InTx(ctx, func(t tx) error {
		out = flexsection.Insert{}
		for i, r := range rows {
			var have []revision
			if err := t.Select(ctx, latestRevisionSQL, []any{r.Alias, r.Section, r.RowKey}, scanRevision(&have)); err != nil {
				return err
			}
			next := 1
			if len(have) > 0 {
				if have[0].hash == r.AttributesSHA256 {
					out.Unchanged++
					continue
				}
				next = have[0].n + 1
			}
			n, err := t.Exec(ctx, insertRowSQL, r.Alias, r.Section, r.RowKey, next, fetchID, i, r.Attributes,
				r.AttributesSHA256, optTime(r.RecordedAt))
			if err != nil {
				return err
			}
			switch {
			case n == 0: // a racer wrote this revision first
			case next == 1:
				out.New++
			default:
				out.Revised++
			}
		}
		for _, st := range statuses {
			if _, err := t.Exec(ctx, insertStatusSQL, fetchID, st.Alias, st.Section, st.State, st.RowCount,
				st.RowsRefused, optTime(st.RecordedAt)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return flexsection.Insert{}, fmt.Errorf("flexsectionpg: record sections: %w", err)
	}
	return out, nil
}

// Rows returns the highest revision of every key of the section of the alias, sorted by row key.
func (s *Store) Rows(ctx context.Context, alias, section string) ([]flexsection.Row, error) {
	out := []flexsection.Row{}
	err := s.db.Select(ctx, selectRowsSQL, []any{alias, section}, func(sc scanner) error {
		var r flexsection.Row
		if err := sc.Scan(&r.Alias, &r.Section, &r.RowKey, &r.Revision, &r.FetchID, &r.RowIndex, &r.Attributes,
			&r.AttributesSHA256, &r.RecordedAt); err != nil {
			return err
		}
		out = append(out, r)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("flexsectionpg: read rows: %w", err)
	}
	return out, nil
}

// Statuses returns the newest status per section of the alias, sorted by section.
func (s *Store) Statuses(ctx context.Context, alias string) ([]flexsection.Status, error) {
	out := []flexsection.Status{}
	err := s.db.Select(ctx, selectStatusesSQL, []any{alias}, func(sc scanner) error {
		var st flexsection.Status
		if err := sc.Scan(&st.FetchID, &st.Alias, &st.Section, &st.State, &st.RowCount, &st.RowsRefused,
			&st.RecordedAt); err != nil {
			return err
		}
		out = append(out, st)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("flexsectionpg: read statuses: %w", err)
	}
	return out, nil
}

type revision struct {
	n    int
	hash string
}

func scanRevision(into *[]revision) func(scanner) error {
	return func(sc scanner) error {
		var r revision
		if err := sc.Scan(&r.n, &r.hash); err != nil {
			return err
		}
		*into = append(*into, r)
		return nil
	}
}

// optTime is NULL for the zero time, so the database stamps the row.
func optTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

// checkRow refuses a row with an empty key part or hash, attributes that are not a JSON object, or attributes
// that hold the key accountId or acctAlias: the alias is the only identity and no account number is stored.
func checkRow(r flexsection.Row) error {
	if r.Alias == "" || r.Section == "" || r.AttributesSHA256 == "" {
		return fmt.Errorf("empty alias, section or hash")
	}
	var attrs map[string]json.RawMessage
	if err := json.Unmarshal([]byte(r.Attributes), &attrs); err != nil || attrs == nil {
		return fmt.Errorf("attributes are not a JSON object")
	}
	for _, k := range []string{
		"accountId",
		"acctAlias",
	} {
		if _, ok := attrs[k]; ok {
			return fmt.Errorf("attributes hold the key %s", k)
		}
	}
	return nil
}

// pgDB is the pgx adapter of database.
type pgDB struct{ pool *pgxpool.Pool }

type pgTx struct{ tx pgx.Tx }

func (p pgTx) Exec(ctx context.Context, sql string, args ...any) (int64, error) {
	tag, err := p.tx.Exec(ctx, sql, args...)
	return tag.RowsAffected(), err
}

func (p pgTx) Select(ctx context.Context, sql string, args []any, scan func(scanner) error) error {
	return selectRows(ctx, p.tx, sql, args, scan)
}

type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func selectRows(ctx context.Context, q querier, sql string, args []any, scan func(scanner) error) error {
	rows, err := q.Query(ctx, sql, args...)
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
	defer func() { _ = t.Rollback(ctx) }()
	if err := fn(pgTx{tx: t}); err != nil {
		return err
	}
	return t.Commit(ctx)
}

func (d pgDB) Select(ctx context.Context, sql string, args []any, scan func(scanner) error) error {
	return selectRows(ctx, d.pool, sql, args, scan)
}

func (d pgDB) Close() error {
	d.pool.Close()
	return nil
}
