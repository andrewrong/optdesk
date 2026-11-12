package jobrequestpg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/andrewrong/optdesk/internal/store/jobrequest"
)

// Store is the queue over one pool. It holds all three roles' methods; the grants of the role decide which of
// them the database lets through.
type Store struct {
	pool *pgxpool.Pool

	// Test hooks of the race in Enqueue; nil in production.
	afterMiss   func(ctx context.Context) // after an insert returned no row, before the read of the open row
	beforeRetry func(ctx context.Context) // after the read found no open row, before the second insert
}

// Close releases the connections.
func (s *Store) Close() error {
	s.pool.Close()
	return nil
}

const viewColumns = `id, job_id, to_char(session_day, 'YYYY-MM-DD'), force, requested_at, state, started_at,
	finished_at, reason, hold_job, hold_at, withdrawn_at, keys`

func scanRow(r pgx.Row) (jobrequest.Row, error) {
	var (
		row                          jobrequest.Row
		state                        string
		started, finished, hold, wdr *time.Time
		keys                         []byte
	)
	err := r.Scan(&row.ID, &row.Request.JobID, &row.Request.Day, &row.Request.Force, &row.RequestedAt, &state,
		&started, &finished, &row.Reason, &row.HoldJob, &hold, &wdr, &keys)
	if err != nil {
		return jobrequest.Row{}, err
	}
	row.State = jobrequest.State(state)
	row.StartedAt, row.FinishedAt, row.HoldAt, row.WithdrawnAt = zero(started), zero(finished), zero(hold), zero(wdr)
	if row.Lines, err = decodeKeys(keys); err != nil {
		return jobrequest.Row{}, err
	}
	return row, nil
}

func zero(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

func (s *Store) selectRows(ctx context.Context, where string, args ...any) ([]jobrequest.Row, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+viewColumns+` FROM rd.job_request_r1 `+where, args...)
	if err != nil {
		return nil, fmt.Errorf("jobrequestpg: read: %w", err)
	}
	defer rows.Close()
	out := []jobrequest.Row{}
	for rows.Next() {
		r, err := scanRow(rows)
		if err != nil {
			return nil, fmt.Errorf("jobrequestpg: read: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("jobrequestpg: read: %w", err)
	}
	return out, nil
}

// Recent returns the newest limit requests, newest first.
func (s *Store) Recent(ctx context.Context, limit int) ([]jobrequest.Row, error) {
	return s.selectRows(ctx, `ORDER BY id DESC LIMIT $1`, limit)
}

// Get returns one request.
func (s *Store) Get(ctx context.Context, id int64) (jobrequest.Row, bool, error) {
	rows, err := s.selectRows(ctx, `WHERE id = $1`, id)
	if err != nil || len(rows) == 0 {
		return jobrequest.Row{}, false, err
	}
	return rows[0], true, nil
}

// Queued returns up to limit queued requests, oldest first; a withdrawn one shows its WithdrawnAt.
func (s *Store) Queued(ctx context.Context, limit int) ([]jobrequest.Row, error) {
	return s.selectRows(ctx, `WHERE state = 'queued' ORDER BY id LIMIT $1`, limit)
}

const insertRequest = `INSERT INTO settings.job_request (job_id, session_day, force) VALUES ($1, $2::date, $3)
	ON CONFLICT DO NOTHING RETURNING id`

// Enqueue stores the request unless an open one of the same job, day and force exists.
func (s *Store) Enqueue(ctx context.Context, r jobrequest.Request) (jobrequest.Enqueued, error) {
	for attempt := 0; attempt < 2; attempt++ {
		var id int64
		err := s.pool.QueryRow(ctx, insertRequest, r.JobID, r.Day, r.Force).Scan(&id)
		if err == nil {
			row, _, err := s.Get(ctx, id)
			if err != nil {
				return jobrequest.Enqueued{}, err
			}
			return jobrequest.Enqueued{Row: row, Outcome: jobrequest.EnqueueCreated}, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return jobrequest.Enqueued{}, fmt.Errorf("jobrequestpg: enqueue: %w", err)
		}
		if attempt == 1 {
			break // the insert after an empty read failed too
		}
		if s.afterMiss != nil {
			s.afterMiss(ctx)
		}
		open, found, err := s.readOpen(ctx, r)
		if err != nil {
			return jobrequest.Enqueued{}, err
		}
		if found {
			out := jobrequest.EnqueueExisting
			if !open.WithdrawnAt.IsZero() {
				out = jobrequest.EnqueueWithdrawing
			}
			return jobrequest.Enqueued{Row: open, Outcome: out}, nil
		}
		if s.beforeRetry != nil {
			s.beforeRetry(ctx)
		}
	}
	return jobrequest.Enqueued{Outcome: jobrequest.EnqueueBusy}, nil
}

// readOpen reads the open row of the request's job, day and force with its withdrawal in one transaction.
func (s *Store) readOpen(ctx context.Context, r jobrequest.Request) (jobrequest.Row, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return jobrequest.Row{}, false, fmt.Errorf("jobrequestpg: enqueue: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	row, err := scanRow(tx.QueryRow(ctx, `SELECT `+viewColumns+` FROM rd.job_request_r1
		WHERE job_id = $1 AND session_day = $2::date AND force = $3 AND state IN ('queued', 'running')`,
		r.JobID, r.Day, r.Force))
	if errors.Is(err, pgx.ErrNoRows) {
		return jobrequest.Row{}, false, nil
	}
	if err != nil {
		return jobrequest.Row{}, false, fmt.Errorf("jobrequestpg: enqueue: %w", err)
	}
	return row, true, nil
}

// Withdraw records the withdrawal; a second one changes nothing and an unknown id is an error.
func (s *Store) Withdraw(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO settings.job_request_withdraw (request_id) VALUES ($1)
		ON CONFLICT DO NOTHING`, id)
	if err != nil {
		return fmt.Errorf("jobrequestpg: withdraw %d: %w", id, err)
	}
	return nil
}

// Claim moves a queued, not withdrawn request to running.
func (s *Store) Claim(ctx context.Context, id int64, now time.Time) (bool, error) {
	tag, err := s.pool.Exec(ctx, `UPDATE settings.job_request SET state = 'running', started_at = $2
		WHERE id = $1 AND state = 'queued'
		AND NOT EXISTS (SELECT 1 FROM settings.job_request_withdraw w WHERE w.request_id = $1)`, id, now)
	if err != nil {
		return false, fmt.Errorf("jobrequestpg: claim %d: %w", id, err)
	}
	return tag.RowsAffected() == 1, nil
}

// Hold records why a queued request waits.
func (s *Store) Hold(ctx context.Context, id int64, job string, at time.Time) error {
	tag, err := s.pool.Exec(ctx, `UPDATE settings.job_request SET hold_job = $2, hold_at = $3
		WHERE id = $1 AND state = 'queued'`, id, job, at)
	if err != nil {
		return fmt.Errorf("jobrequestpg: hold %d: %w", id, err)
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("jobrequestpg: hold %d: not a queued request", id)
	}
	return nil
}

// Finish ends an open request in state s.
func (s *Store) Finish(ctx context.Context, id int64, st jobrequest.State, reason string, lines []jobrequest.KeyLine, now time.Time) error {
	keys, err := encodeKeys(lines)
	if err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx, `UPDATE settings.job_request SET state = $2, finished_at = $3, reason = $4, keys = $5
		WHERE id = $1 AND state IN ('queued', 'running')`, id, string(st), now, reason, keys)
	if err != nil {
		return fmt.Errorf("jobrequestpg: finish %d: %w", id, err)
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("jobrequestpg: finish %d: not an open request", id)
	}
	return nil
}

// FailRunning ends every running request as failed with reason.
func (s *Store) FailRunning(ctx context.Context, reason string, now time.Time) error {
	if _, err := s.pool.Exec(ctx, `UPDATE settings.job_request SET state = 'failed', finished_at = $2, reason = $1
		WHERE state = 'running'`, reason, now); err != nil {
		return fmt.Errorf("jobrequestpg: fail running: %w", err)
	}
	return nil
}

// keyDoc is the stored object {"v":1,"lines":[...]}.
type keyDoc struct {
	V     int       `json:"v"`
	Lines []keyJSON `json:"lines"`
}

type keyJSON struct {
	Dataset    string     `json:"dataset"`
	Subject    string     `json:"subject"`
	Group      string     `json:"group"`
	Status     string     `json:"status"`
	ErrorCode  string     `json:"error_code"`
	ReasonCode string     `json:"reason_code"`
	Returned   int        `json:"returned"`
	Usable     int        `json:"usable"`
	Attempt    int        `json:"attempt"`
	Mark       string     `json:"mark"`
	DueAt      *time.Time `json:"due_at,omitempty"`
}

func encodeKeys(lines []jobrequest.KeyLine) ([]byte, error) {
	doc := keyDoc{V: 1, Lines: make([]keyJSON, 0, len(lines))}
	for _, l := range lines {
		k := keyJSON{l.Dataset, l.Subject, l.Group, l.Status, l.ErrorCode, l.ReasonCode, l.Returned, l.Usable, l.Attempt, l.Mark, nil}
		if !l.DueAt.IsZero() {
			due := l.DueAt
			k.DueAt = &due
		}
		doc.Lines = append(doc.Lines, k)
	}
	return json.Marshal(doc)
}

func decodeKeys(raw []byte) ([]jobrequest.KeyLine, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var doc keyDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("jobrequestpg: keys: %w", err)
	}
	if doc.V != 1 {
		return nil, fmt.Errorf("jobrequestpg: keys: version %d", doc.V)
	}
	out := make([]jobrequest.KeyLine, 0, len(doc.Lines))
	for _, k := range doc.Lines {
		l := jobrequest.KeyLine{Dataset: k.Dataset, Subject: k.Subject, Group: k.Group, Status: k.Status,
			ErrorCode: k.ErrorCode, ReasonCode: k.ReasonCode, Mark: k.Mark, Returned: k.Returned, Usable: k.Usable,
			Attempt: k.Attempt}
		if k.DueAt != nil {
			l.DueAt = *k.DueAt
		}
		out = append(out, l)
	}
	return out, nil
}
