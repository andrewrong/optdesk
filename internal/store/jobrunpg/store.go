// Package jobrunpg implements JobRunLedger and JobRunReader of internal/store/jobrun on PostgreSQL: the table
// ops.job_run written by the runner role and read through the views rd.job_run_r1 and rd.job_usual_r1
// (docs/design/scheduler-per-job.md, entry 5). The role of the DSN decides what the store may do: ot_runner writes
// and reads, ot_ro_pages reads the views only.
package jobrunpg

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/andrewrong/optdesk/internal/store/jobrun"
)

var (
	_ jobrun.JobRunLedger = (*Store)(nil)
	_ jobrun.JobRunReader = (*Store)(nil)
)

// Store is the ledger and the reader over one pool.
type Store struct {
	pool *pgxpool.Pool
}

// Open connects with dsn and pings; it applies no migration, the table is made by goose as ot_owner.
func Open(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("jobrunpg: open: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("jobrunpg: ping: %w", err)
	}
	return &Store{pool: pool}, nil
}

// Release closes the pool. It is not Close: that is the ledger's end of a run.
func (s *Store) Release() error {
	s.pool.Close()
	return nil
}

// nullTime stores the zero time as NULL.
func nullTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

func orZero(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

// Start opens the row r. A second row for the same job id is accepted: the row is no guard.
func (s *Store) Start(ctx context.Context, r jobrun.JobRun) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO ops.job_run
		(id, job_id, source, started_at, heartbeat_at, keys_done, keys_total, step, outcome)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		r.ID, r.JobID, r.Source, r.StartedAt, nullTime(r.HeartbeatAt), r.KeysDone, r.KeysTotal, r.Step, r.Outcome)
	if err != nil {
		return fmt.Errorf("jobrunpg: start %s: %w", r.ID, err)
	}
	return nil
}

func (s *Store) updateOne(ctx context.Context, what, id, sql string, args ...any) error {
	tag, err := s.pool.Exec(ctx, sql, append([]any{id}, args...)...)
	if err != nil {
		return fmt.Errorf("jobrunpg: %s %s: %w", what, id, err)
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("jobrunpg: %s %s: no such run", what, id)
	}
	return nil
}

// Progress records the keys done of total, the step in hand and the heartbeat.
func (s *Store) Progress(ctx context.Context, id string, done, total int, step string, at time.Time) error {
	return s.updateOne(ctx, "progress", id,
		`UPDATE ops.job_run SET keys_done = $2, keys_total = $3, step = $4, heartbeat_at = $5 WHERE id = $1`,
		done, total, step, at)
}

// Close ends the run with outcome "ok", "failed" or "cut" at at.
func (s *Store) Close(ctx context.Context, id, outcome string, at time.Time) error {
	return s.updateOne(ctx, "close", id, `UPDATE ops.job_run SET outcome = $2, finished_at = $3 WHERE id = $1`, outcome, at)
}

// MarkSlowNotified records that the slow notice of the run was sent at at.
func (s *Store) MarkSlowNotified(ctx context.Context, id string, at time.Time) error {
	return s.updateOne(ctx, "mark slow", id, `UPDATE ops.job_run SET slow_notified_at = $2 WHERE id = $1`, at)
}

// CloseOpen closes every open row "cut" at the instant at and returns how many it closed.
func (s *Store) CloseOpen(ctx context.Context, at time.Time) (int, error) {
	tag, err := s.pool.Exec(ctx,
		`UPDATE ops.job_run SET outcome = 'cut', finished_at = $1 WHERE outcome = ''`, at)
	if err != nil {
		return 0, fmt.Errorf("jobrunpg: close open: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// Running returns the open runs, oldest first.
func (s *Store) Running(ctx context.Context) ([]jobrun.JobRun, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, job_id, source, started_at, heartbeat_at, slow_notified_at,
		keys_done, keys_total, step FROM rd.job_run_r1 ORDER BY started_at, id`)
	if err != nil {
		return nil, fmt.Errorf("jobrunpg: running: %w", err)
	}
	defer rows.Close()
	out := []jobrun.JobRun{}
	for rows.Next() {
		var (
			r        jobrun.JobRun
			hb, slow *time.Time
		)
		if err := rows.Scan(&r.ID, &r.JobID, &r.Source, &r.StartedAt, &hb, &slow, &r.KeysDone, &r.KeysTotal, &r.Step); err != nil {
			return nil, fmt.Errorf("jobrunpg: running: %w", err)
		}
		r.HeartbeatAt, r.SlowNotifiedAt = orZero(hb), orZero(slow)
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("jobrunpg: running: %w", err)
	}
	return out, nil
}

// Usuals returns one UsualStat per job id that has an ok run, ordered by job id.
func (s *Store) Usuals(ctx context.Context) ([]jobrun.UsualStat, error) {
	rows, err := s.pool.Query(ctx, `SELECT job_id, median_seconds, samples FROM rd.job_usual_r1 ORDER BY job_id`)
	if err != nil {
		return nil, fmt.Errorf("jobrunpg: usuals: %w", err)
	}
	defer rows.Close()
	out := []jobrun.UsualStat{}
	for rows.Next() {
		var (
			u    jobrun.UsualStat
			secs float64
		)
		if err := rows.Scan(&u.JobID, &secs, &u.Samples); err != nil {
			return nil, fmt.Errorf("jobrunpg: usuals: %w", err)
		}
		u.Median = time.Duration(secs * float64(time.Second))
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("jobrunpg: usuals: %w", err)
	}
	return out, nil
}
