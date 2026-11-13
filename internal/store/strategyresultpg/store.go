// Package strategyresultpg is the Postgres implementation of strategyresult.Store over pgx, for the DSN line of the
// role ot_w_candidate (docs/design/strategy-framework.md, 5 result-store, Decisions 4 to 6). It writes
// strategy.result and reads the view rd.strategy_result_r1 only. NewCandidateReader answers store.CandidateReader
// for the short put from these rows, and Compare lists the differences to the old reader.
package strategyresultpg

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/strategyresult"
)

// columns are the columns of the table and of the view r1, in the order of the argument list of a row and of the
// scan of a row.
const columns = `session_day, run_id, strategy, rule_version, param_hash, key, underlying_id, status, rank, ` +
	`rank_in_underlying, legs, checks, facts, computed_at, settings_version`

// insertSQL is insert-only: a row whose primary key exists is skipped, never replaced.
const insertSQL = `INSERT INTO strategy.result (` + columns + `) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, ` +
	`$11, $12, $13, $14, $15) ON CONFLICT (session_day, run_id, strategy, key) DO NOTHING`

// selectSQL reads every run of one day and strategy through the view; the newest run is picked in Go.
const selectSQL = `SELECT ` + columns + ` FROM rd.strategy_result_r1 WHERE session_day = $1 AND strategy = $2 ` +
	`ORDER BY rank ASC NULLS LAST, key, run_id`

// scanner reads the columns of the current row.
type scanner interface {
	Scan(dest ...any) error
}

// execer runs one statement inside a transaction and returns the rows it affected.
type execer interface {
	Exec(ctx context.Context, sql string, args ...any) (int64, error)
}

// database is the seam under the store: the one place pgx is touched, replaced by a fake in unit tests.
type database interface {
	// InTx runs fn in one transaction: committed when fn returns nil, rolled back otherwise.
	InTx(ctx context.Context, fn func(execer) error) error
	// Select runs a query and calls scan once per row, in the order the database returns them.
	Select(ctx context.Context, sql string, args []any, scan func(scanner) error) error
	Close() error
}

type resultStore struct{ db database }

var _ strategyresult.Store = (*resultStore)(nil)

// Open connects to the database named by dsn (the line of the role ot_w_candidate, passed in by the caller). It
// applies no migration.
func Open(ctx context.Context, dsn string) (strategyresult.Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, permanent(fmt.Errorf("strategyresultpg: open: %w", err))
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, retryable(fmt.Errorf("strategyresultpg: ping: %w", err))
	}
	return &resultStore{db: pgDB{pool: pool}}, nil
}

// Close releases the connections.
func (s *resultStore) Close() error { return s.db.Close() }

// InsertResults inserts the rows in one transaction, all or nothing; a row whose key exists is skipped and not
// counted. No rows makes no database call. A row with no day, run id, strategy or key fails the call before any
// statement. A constraint violation (SQLSTATE class 23) is Permanent, any other database error Retryable.
func (s *resultStore) InsertResults(ctx context.Context, rows []strategyresult.Row) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	args := make([][]any, 0, len(rows))
	for i, r := range rows {
		a, err := rowArgs(r)
		if err != nil {
			return 0, fmt.Errorf("strategyresultpg: row %d: %w", i, err)
		}
		args = append(args, a)
	}
	inserted := 0
	err := s.db.InTx(ctx, func(tx execer) error {
		inserted = 0
		for _, a := range args {
			n, err := tx.Exec(ctx, insertSQL, a...)
			if err != nil {
				return err
			}
			inserted += int(n)
		}
		return nil
	})
	if err != nil {
		err = fmt.Errorf("strategyresultpg: insert: %w", err)
		if isConstraintViolation(err) {
			return 0, permanent(err)
		}
		return 0, retryable(err)
	}
	return inserted, nil
}

func rowArgs(r strategyresult.Row) ([]any, error) {
	day, err := parseDay(r.SessionDay)
	if err != nil {
		return nil, err
	}
	if r.RunID == "" || r.Strategy == "" || r.Key == "" {
		return nil, permanent(errors.New("empty run id, strategy or key"))
	}
	return []any{day, r.RunID, r.Strategy, r.RuleVersion, r.ParamHash, r.Key, r.UnderlyingID, r.Status, r.Rank,
		r.RankInUnderlying, r.Legs, r.Checks, r.Facts, r.ComputedAt, r.SettingsVersion}, nil
}

// Results returns the rows of the newest run (the latest computed_at; a tie goes to the larger run id) of one day
// and strategy, ranked rows first, then by the numeric instrument id of the key, its alias and the run id. None
// gives an empty slice. A zero or bad day or an empty strategy is a Permanent error.
func (s *resultStore) Results(ctx context.Context, day contracts.Day, strategy string) ([]strategyresult.Row, error) {
	d, err := parseDay(day)
	if err != nil {
		return nil, err
	}
	if strategy == "" {
		return nil, permanent(errors.New("strategyresultpg: empty strategy"))
	}
	var all []strategyresult.Row
	err = s.db.Select(ctx, selectSQL, []any{d, strategy}, func(sc scanner) error {
		r, err := scanRow(sc)
		if err != nil {
			return err
		}
		all = append(all, r)
		return nil
	})
	if err != nil {
		return nil, retryable(fmt.Errorf("strategyresultpg: select: %w", err))
	}
	out := newestRun(all)
	sortRows(out)
	return out, nil
}

// newestRun keeps the rows of the run with the latest ComputedAt; a tie goes to the larger run id.
func newestRun(all []strategyresult.Row) []strategyresult.Row {
	out := []strategyresult.Row{}
	if len(all) == 0 {
		return out
	}
	best := all[0]
	for _, r := range all[1:] {
		if r.ComputedAt.After(best.ComputedAt) || (r.ComputedAt.Equal(best.ComputedAt) && r.RunID > best.RunID) {
			best = r
		}
	}
	for _, r := range all {
		if r.RunID == best.RunID {
			out = append(out, r)
		}
	}
	return out
}

// sortRows orders rows as internal/store/candidate does: rank ascending with unranked last, then the numeric
// instrument id of the key, then its alias, then the run id.
func sortRows(rows []strategyresult.Row) {
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if (a.Rank == nil) != (b.Rank == nil) {
			return a.Rank != nil
		}
		if a.Rank != nil && *a.Rank != *b.Rank {
			return *a.Rank < *b.Rank
		}
		ai, aa := splitKey(a.Key)
		bi, ba := splitKey(b.Key)
		if ai != bi {
			return ai < bi
		}
		if aa != ba {
			return aa < ba
		}
		return a.RunID < b.RunID
	})
}

// splitKey reads a key as the instrument id of its first leg, then ":" and the rest (the alias of a per-alias row).
// A first part that is no number sorts as 0.
func splitKey(key string) (id int, alias string) {
	first, rest, _ := strings.Cut(key, ":")
	id, _ = strconv.Atoi(first)
	return id, rest
}

func scanRow(sc scanner) (strategyresult.Row, error) {
	var r strategyresult.Row
	var day time.Time
	err := sc.Scan(&day, &r.RunID, &r.Strategy, &r.RuleVersion, &r.ParamHash, &r.Key, &r.UnderlyingID, &r.Status,
		&r.Rank, &r.RankInUnderlying, &r.Legs, &r.Checks, &r.Facts, &r.ComputedAt, &r.SettingsVersion)
	if err != nil {
		return r, err
	}
	r.SessionDay = contracts.Day(day.Format(time.DateOnly))
	return r, nil
}

func parseDay(d contracts.Day) (time.Time, error) {
	t, err := time.Parse(time.DateOnly, string(d))
	if err != nil {
		return time.Time{}, permanent(fmt.Errorf("strategyresultpg: bad day %q: %w", d, err))
	}
	return t, nil
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
