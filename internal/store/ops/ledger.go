// Package ops is the OpsLedger over the ops tables of PostgreSQL; its role is ot_runner.
package ops

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

var _ store.OpenLedgerFunc = Open

const (
	deleteExpectedSQL = `DELETE FROM ops.capture_expected_key WHERE session_day = $1`
	insertExpectedSQL = `INSERT INTO ops.capture_expected_key (session_day, plan_hash, dataset_id, subject_key) ` +
		`VALUES ($1, $2, $3, $4)`
	selectExpectedSQL = `SELECT plan_hash, dataset_id, subject_key FROM ops.capture_expected_key ` +
		`WHERE session_day = $1 ORDER BY dataset_id, subject_key`
	markPresentSQL = `INSERT INTO ops.capture_present (session_day, dataset_id, subject_key, status) ` +
		`VALUES ($1, $2, $3, $4) ON CONFLICT (session_day, dataset_id, subject_key) DO UPDATE SET status = EXCLUDED.status`
	selectPresentSQL = `SELECT dataset_id, subject_key, status FROM ops.capture_present WHERE session_day = $1`
	putWindowSQL     = `INSERT INTO ops.dataset_window (dataset_id, session_day, open_at, close_at) ` +
		`VALUES ($1, $2, $3, $4) ON CONFLICT (dataset_id, session_day) DO UPDATE SET ` +
		`open_at = EXCLUDED.open_at, close_at = EXCLUDED.close_at`
	selectWindowSQL = `SELECT open_at, close_at FROM ops.dataset_window WHERE dataset_id = $1 AND session_day = $2`
	recordRunSQL    = `INSERT INTO ops.capture_run (run_id, session_day, dataset_id, subject_key, status, lease_until, ` +
		`heartbeat_at, finished_at, returned_n, usable_n, error_code, reason_code) ` +
		`VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12) ` +
		`ON CONFLICT (run_id) DO UPDATE SET status = EXCLUDED.status, lease_until = EXCLUDED.lease_until, ` +
		`heartbeat_at = EXCLUDED.heartbeat_at, finished_at = EXCLUDED.finished_at, returned_n = EXCLUDED.returned_n, ` +
		`usable_n = EXCLUDED.usable_n, error_code = EXCLUDED.error_code, reason_code = EXCLUDED.reason_code`
	selectRunsSQL = `SELECT run_id, dataset_id, subject_key, status, lease_until, heartbeat_at, finished_at, ` +
		`returned_n, usable_n, error_code, reason_code FROM ops.capture_run WHERE session_day = $1 ORDER BY run_id`
	setCursorSQL    = `INSERT INTO ops.reconcile_cursor (id, session_day) VALUES (true, $1) ON CONFLICT (id) DO UPDATE SET session_day = EXCLUDED.session_day`
	selectCursorSQL = `SELECT session_day FROM ops.reconcile_cursor WHERE id`
	// acquireSQL takes the lease when it is free, expired or already ours; it returns no row while another owner holds it.
	// The instants are the database's: now() is the clock of the lease.
	acquireSQL = `INSERT INTO ops.run_lease (id, owner, expires_at) VALUES (true, $1, now() + make_interval(secs => $2)) ` +
		`ON CONFLICT (id) DO UPDATE SET owner = EXCLUDED.owner, expires_at = EXCLUDED.expires_at ` +
		`WHERE ops.run_lease.expires_at <= now() OR ops.run_lease.owner = EXCLUDED.owner ` +
		`RETURNING owner, expires_at`
	releaseSQL = `DELETE FROM ops.run_lease WHERE owner = $1`
)

// querier runs statements on the database or inside a transaction.
type querier interface {
	Exec(ctx context.Context, sql string, args ...any) error
	// Select runs a query and calls scan once per row, in the order the database returns them.
	Select(ctx context.Context, sql string, args []any, scan func(scanner) error) error
}

// scanner reads the columns of the current row.
type scanner interface {
	Scan(dest ...any) error
}

// database is the seam under the ledger: the one place pgx is touched, replaced by a fake in unit tests.
type database interface {
	querier
	// InTx runs fn in one transaction: committed when fn returns nil, rolled back otherwise.
	InTx(ctx context.Context, fn func(querier) error) error
	Close() error
}

type ledger struct{ db database }

// Open connects to the database named by dsn and returns the ledger. It applies no migration.
func Open(ctx context.Context, dsn string) (store.OpsLedger, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, permanent(fmt.Errorf("ops: open: %w", err))
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, retryable(fmt.Errorf("ops: ping: %w", err))
	}
	return &ledger{db: pgDB{pool: pool}}, nil
}

func (l *ledger) PutExpected(ctx context.Context, day contracts.Day, planHash string, keys []contracts.ExpectedKey) error {
	d, err := parseDay(day)
	if err != nil {
		return err
	}
	err = l.db.InTx(ctx, func(q querier) error {
		if err := q.Exec(ctx, deleteExpectedSQL, d); err != nil {
			return err
		}
		for _, k := range keys {
			if err := q.Exec(ctx, insertExpectedSQL, d, planHash, k.DatasetID, k.SubjectKey); err != nil {
				return err
			}
		}
		return nil
	})
	return wrap("put expected", err)
}

func (l *ledger) Expected(ctx context.Context, day contracts.Day) ([]contracts.ExpectedKey, string, error) {
	d, err := parseDay(day)
	if err != nil {
		return nil, "", err
	}
	keys := []contracts.ExpectedKey{}
	var hash string
	err = l.db.Select(ctx, selectExpectedSQL, []any{d}, func(s scanner) error {
		var k contracts.ExpectedKey
		if err := s.Scan(&hash, &k.DatasetID, &k.SubjectKey); err != nil {
			return err
		}
		keys = append(keys, k)
		return nil
	})
	if err != nil {
		return nil, "", wrap("expected", err)
	}
	return keys, hash, nil
}

func (l *ledger) MarkPresent(ctx context.Context, day contracts.Day, key contracts.ExpectedKey, status store.PresentStatus) error {
	d, err := parseDay(day)
	if err != nil {
		return err
	}
	return wrap("mark present", l.db.Exec(ctx, markPresentSQL, d, key.DatasetID, key.SubjectKey, string(status)))
}

func (l *ledger) Present(ctx context.Context, day contracts.Day) (map[contracts.ExpectedKey]store.PresentStatus, error) {
	d, err := parseDay(day)
	if err != nil {
		return nil, err
	}
	out := map[contracts.ExpectedKey]store.PresentStatus{}
	err = l.db.Select(ctx, selectPresentSQL, []any{d}, func(s scanner) error {
		var k contracts.ExpectedKey
		var st string
		if err := s.Scan(&k.DatasetID, &k.SubjectKey, &st); err != nil {
			return err
		}
		out[k] = store.PresentStatus(st)
		return nil
	})
	if err != nil {
		return nil, wrap("present", err)
	}
	return out, nil
}

func (l *ledger) PutWindow(ctx context.Context, w store.Window) error {
	d, err := parseDay(w.Day)
	if err != nil {
		return err
	}
	return wrap("put window", l.db.Exec(ctx, putWindowSQL, w.DatasetID, d, nullable(w.OpenAt), nullable(w.CloseAt)))
}

func (l *ledger) Window(ctx context.Context, datasetID string, day contracts.Day) (store.Window, error) {
	d, err := parseDay(day)
	if err != nil {
		return store.Window{}, err
	}
	w := store.Window{DatasetID: datasetID, Day: day}
	found := false
	err = l.db.Select(ctx, selectWindowSQL, []any{datasetID, d}, func(s scanner) error {
		var open, closeAt *time.Time
		if err := s.Scan(&open, &closeAt); err != nil {
			return err
		}
		w.OpenAt, w.CloseAt = fromNullable(open), fromNullable(closeAt)
		found = true
		return nil
	})
	if err != nil {
		return store.Window{}, wrap("window", err)
	}
	if !found {
		return store.Window{}, &typedErr{kind: contracts.NoValue,
			cause: fmt.Errorf("ops: no window for dataset %q on %s", datasetID, day)}
	}
	return w, nil
}

// RecordRun inserts r, or updates the row of r.ID: status, lease_until, heartbeat_at, finished_at,
// returned_n, usable_n, error_code and reason_code. A zero time is stored as NULL.
func (l *ledger) RecordRun(ctx context.Context, r store.Run) error {
	d, err := parseDay(r.Day)
	if err != nil {
		return err
	}
	err = l.db.Exec(ctx, recordRunSQL, r.ID, d, r.Key.DatasetID, r.Key.SubjectKey, string(r.Status),
		nullable(r.LeaseUntil), nullable(r.HeartbeatAt), nullable(r.FinishedAt),
		r.ReturnedN, r.UsableN, r.ErrorCode, r.ReasonCode)
	return wrap("record run", err)
}

func (l *ledger) Runs(ctx context.Context, day contracts.Day) ([]store.Run, error) {
	d, err := parseDay(day)
	if err != nil {
		return nil, err
	}
	out := []store.Run{}
	err = l.db.Select(ctx, selectRunsSQL, []any{d}, func(s scanner) error {
		r := store.Run{Day: day}
		var status string
		var lease, hb, fin *time.Time
		if err := s.Scan(&r.ID, &r.Key.DatasetID, &r.Key.SubjectKey, &status, &lease, &hb, &fin,
			&r.ReturnedN, &r.UsableN, &r.ErrorCode, &r.ReasonCode); err != nil {
			return err
		}
		r.Status = store.RunStatus(status)
		r.LeaseUntil, r.HeartbeatAt, r.FinishedAt = fromNullable(lease), fromNullable(hb), fromNullable(fin)
		out = append(out, r)
		return nil
	})
	if err != nil {
		return nil, wrap("runs", err)
	}
	return out, nil
}

func (l *ledger) SetCursor(ctx context.Context, c store.Cursor) error {
	d, err := parseDay(c.Day)
	if err != nil {
		return err
	}
	return wrap("set cursor", l.db.Exec(ctx, setCursorSQL, d))
}

// Cursor returns the stored cursor; with none set it returns the zero Cursor and no error.
func (l *ledger) Cursor(ctx context.Context) (store.Cursor, error) {
	var c store.Cursor
	err := l.db.Select(ctx, selectCursorSQL, nil, func(s scanner) error {
		var d time.Time
		if err := s.Scan(&d); err != nil {
			return err
		}
		c.Day = contracts.Day(d.Format(time.DateOnly))
		return nil
	})
	if err != nil {
		return store.Cursor{}, wrap("cursor", err)
	}
	return c, nil
}

// Acquire takes the lease for owner until the database's now plus ttl. The same owner renews it.
// While another owner holds an unexpired lease it returns a Retryable error.
func (l *ledger) Acquire(ctx context.Context, owner string, ttl time.Duration) (store.Lease, error) {
	var lease store.Lease
	got := false
	err := l.db.Select(ctx, acquireSQL, []any{owner, ttl.Seconds()}, func(s scanner) error {
		var exp time.Time
		if err := s.Scan(&lease.Owner, &exp); err != nil {
			return err
		}
		lease.ExpiresAt = exp.UTC()
		got = true
		return nil
	})
	if err != nil {
		return store.Lease{}, wrap("acquire", err)
	}
	if !got {
		return store.Lease{}, retryable(fmt.Errorf("ops: lease is held by another owner"))
	}
	return lease, nil
}

// Release gives the lease of l.Owner back; releasing a lease that is not held is not an error.
func (l *ledger) Release(ctx context.Context, lease store.Lease) error {
	return wrap("release", l.db.Exec(ctx, releaseSQL, lease.Owner))
}

// Close releases the connections.
func (l *ledger) Close() error { return l.db.Close() }

// nullable stores a zero time as NULL.
func nullable(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}

// fromNullable reads NULL back as the zero time and an instant in UTC.
func fromNullable(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return t.UTC()
}

func wrap(what string, err error) error {
	if err == nil {
		return nil
	}
	return retryable(fmt.Errorf("ops: %s: %w", what, err))
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

type pgQ struct {
	exec  func(ctx context.Context, sql string, args ...any) error
	query func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func (q pgQ) Exec(ctx context.Context, sql string, args ...any) error {
	return q.exec(ctx, sql, args...)
}

func (q pgQ) Select(ctx context.Context, sql string, args []any, scan func(scanner) error) error {
	rows, err := q.query(ctx, sql, args...)
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

func (d pgDB) Exec(ctx context.Context, sql string, args ...any) error {
	_, err := d.pool.Exec(ctx, sql, args...)
	return err
}

func (d pgDB) Select(ctx context.Context, sql string, args []any, scan func(scanner) error) error {
	return pgQ{query: d.pool.Query}.Select(ctx, sql, args, scan)
}

func (d pgDB) InTx(ctx context.Context, fn func(querier) error) error {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return err
	}
	q := pgQ{
		exec: func(ctx context.Context, sql string, args ...any) error {
			_, err := tx.Exec(ctx, sql, args...)
			return err
		},
		query: tx.Query,
	}
	if err := fn(q); err != nil {
		if rerr := tx.Rollback(context.WithoutCancel(ctx)); rerr != nil && !errors.Is(rerr, pgx.ErrTxClosed) {
			err = errors.Join(err, rerr)
		}
		return err
	}
	return tx.Commit(ctx)
}

func (d pgDB) Close() error {
	d.pool.Close()
	return nil
}

// parseDay turns a contracts.Day into the date it names.
func parseDay(d contracts.Day) (time.Time, error) {
	t, err := time.Parse(time.DateOnly, string(d))
	if err != nil {
		return time.Time{}, permanent(fmt.Errorf("ops: bad day %q: %w", d, err))
	}
	return t, nil
}
