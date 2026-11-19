// Package reconpg implements reconstore.Writer and reconstore.Reader on PostgreSQL over pgx: the tables recon.run and
// recon.item and the view rd.recon_latest_r1 of migrations/0060_recon.sql (docs/design/base-data-page.md, entry 20).
// Only cmd/ot imports it; jobs and pages import the declarations in internal/store/recon.
package reconpg

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	reconstore "github.com/andrewrong/optdesk/internal/store/recon"
)

// tx runs the statements of one transaction.
type tx interface {
	// InsertReturningID runs an INSERT ... RETURNING id and returns the id.
	InsertReturningID(ctx context.Context, sql string, args ...any) (int64, error)
	Exec(ctx context.Context, sql string, args ...any) error
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

var (
	_ reconstore.Writer = (*store)(nil)
	_ reconstore.Reader = (*store)(nil)
)

func open(ctx context.Context, dsn string) (*store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("reconpg: open: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("reconpg: ping: %w", err)
	}
	return &store{db: pgDB{pool: pool}}, nil
}

// OpenWriter connects to the database named by dsn, the line of the role ot_w_recon (reconstore.DSNEnvRecon). It
// applies no migration: the schema is made by goose.
func OpenWriter(ctx context.Context, dsn string) (reconstore.Writer, error) {
	s, err := open(ctx, dsn)
	if err != nil {
		return nil, err
	}
	return s, nil
}

var _ reconstore.OpenWriterFunc = OpenWriter

// Close releases the connections.
func (s *store) Close() error { return s.db.Close() }

const insertRunSQL = `INSERT INTO recon.run (pair_id, session_day, started_at, finished_at, status, reason, n, matched,
 diff, missing_ours, missing_other, skipped, pairs_asked, pairs_compared, tolerance_text, source_id)
 VALUES ($1, $2::date, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16) RETURNING run_id`

const insertItemsHead = `INSERT INTO recon.item (run_id, key, company, ours, other, diff, kind, reason) VALUES `

// itemKinds are the kinds a run stores; a match is never stored.
var itemKinds = map[string]bool{"diff": true, "missing_ours": true, "missing_other": true, "skipped": true}

// InsertRun inserts the run and its items in one transaction and returns the new run id. A second run for the same
// pair and day is a new run. It refuses, before any statement, more than reconstore.RunItemsMax items and any item
// whose kind is not diff, missing_ours, missing_other or skipped (so no match).
func (s *store) InsertRun(ctx context.Context, r reconstore.Run) (int64, error) {
	if len(r.Items) > reconstore.RunItemsMax {
		return 0, fmt.Errorf("reconpg: %d items, at most %d", len(r.Items), reconstore.RunItemsMax)
	}
	for _, it := range r.Items {
		if !itemKinds[it.Kind] {
			return 0, fmt.Errorf("reconpg: item %q has kind %q: a match or an unknown kind is not stored", it.Key, it.Kind)
		}
	}
	var id int64
	err := s.db.InTx(ctx, func(t tx) error {
		var err error
		id, err = t.InsertReturningID(ctx, insertRunSQL, r.PairID, string(r.SessionDay), r.StartedAt.UTC(),
			r.FinishedAt.UTC(), r.Status, r.Reason, r.N, r.Matched, r.Diff, r.MissingOurs, r.MissingOther, r.Skipped,
			r.PairsAsked, r.PairsCompared, r.ToleranceText, r.SourceID)
		if err != nil || len(r.Items) == 0 {
			return err
		}
		var sb strings.Builder
		sb.WriteString(insertItemsHead)
		args := make([]any, 0, 8*len(r.Items))
		for i, it := range r.Items {
			if i > 0 {
				sb.WriteString(", ")
			}
			n := len(args)
			fmt.Fprintf(&sb, "($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d)", n+1, n+2, n+3, n+4, n+5, n+6, n+7, n+8)
			args = append(args, id, it.Key, it.Company, it.Ours, it.Other, it.Diff, it.Kind, it.Reason)
		}
		return t.Exec(ctx, sb.String(), args...)
	})
	if err != nil {
		return 0, fmt.Errorf("reconpg: insert run: %w", err)
	}
	return id, nil
}

// pgDB is the pgx adapter of database.
type pgDB struct{ pool *pgxpool.Pool }

func (d pgDB) Close() error { d.pool.Close(); return nil }

func (d pgDB) InTx(ctx context.Context, fn func(tx) error) error {
	t, err := d.pool.Begin(ctx)
	if err != nil {
		return err
	}
	if err := fn(pgTx{t}); err != nil {
		_ = t.Rollback(ctx)
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

type pgTx struct{ t pgx.Tx }

func (p pgTx) InsertReturningID(ctx context.Context, sql string, args ...any) (int64, error) {
	var id int64
	err := p.t.QueryRow(ctx, sql, args...).Scan(&id)
	return id, err
}

func (p pgTx) Exec(ctx context.Context, sql string, args ...any) error {
	_, err := p.t.Exec(ctx, sql, args...)
	return err
}
