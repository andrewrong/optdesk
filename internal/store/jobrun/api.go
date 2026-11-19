// Package jobrun declares the ledger of job runs that the runner writes and the
// reader that the pages and notify side use (docs/design/scheduler-per-job.md#Interface).
// It holds declarations only; the table, its views and the implementation are later issues.
package jobrun

import (
	"context"
	"time"
)

// JobRun is one run of a whole job (all keys it takes in one go). It has no day:
// a run may walk old days, then today.
type JobRun struct {
	ID    string
	JobID string
	// Source is "scheduler" or "console".
	Source         string
	StartedAt      time.Time
	HeartbeatAt    time.Time
	FinishedAt     time.Time
	SlowNotifiedAt time.Time
	KeysDone       int
	KeysTotal      int
	// Step is the key in hand, opaque.
	Step string
	// Outcome is empty while the run is open, then "ok", "failed" or "cut".
	Outcome string
}

// UsualStat is the median duration of the last 20 closed "ok" runs of one job id;
// "cut" and "failed" runs are not samples. Samples is how many runs the median has.
type UsualStat struct {
	JobID   string
	Median  time.Duration
	Samples int
}

// JobRunLedger is the runner's side. The row records; it is not a guard: the
// in-memory lock per job id is. Starting a second run of one job id succeeds.
type JobRunLedger interface {
	// Start opens the row r.
	Start(ctx context.Context, r JobRun) error
	// Progress records the keys done of total, the step in hand and the heartbeat at.
	Progress(ctx context.Context, id string, done, total int, step string, at time.Time) error
	// Close ends the run with outcome "ok", "failed" or "cut" at at.
	Close(ctx context.Context, id, outcome string, at time.Time) error
	// CloseOpen is for start-up: every open row is closed "cut"; it returns how many.
	CloseOpen(ctx context.Context, at time.Time) (int, error)
	// MarkSlowNotified records that the slow notice of the run was sent at at.
	MarkSlowNotified(ctx context.Context, id string, at time.Time) error
}

// JobRunReader is the pages and notify side, through the views rd.job_run_r1
// (open runs) and rd.job_usual_r1 (UsualStat).
type JobRunReader interface {
	// Running returns the open runs.
	Running(ctx context.Context) ([]JobRun, error)
	// Usuals returns one UsualStat per job id that has an "ok" run.
	Usuals(ctx context.Context) ([]UsualStat, error)
}
