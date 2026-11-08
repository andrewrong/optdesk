package ops

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

var (
	_ store.OpenFenceFunc           = OpenFence
	_ store.OpenSchedulerLedgerFunc = OpenSchedulerLedger
)

// fenceKey is the PostgreSQL advisory lock key of the instance fence: one scheduler per database.
const fenceKey int64 = 0x6f7074736368 // "optsch"

const (
	tryFenceSQL = `SELECT pg_try_advisory_lock($1)`
	// renewLeaseSQL extends the lease of the owner from the database's now(); no row comes back when another owner
	// holds it or none exists. The caller passes no instant.
	renewLeaseSQL = `UPDATE ops.run_lease SET expires_at = now() + make_interval(secs => $2) ` +
		`WHERE id AND owner = $1 RETURNING owner, expires_at`
	// putGapSQL inserts a gap or refreshes the reason of the open gap of the same key; first_seen_at is the column default now().
	putGapSQL = `INSERT INTO ops.job_gap (job_id, dataset_id, day, reason) VALUES ($1, $2, $3, $4) ` +
		`ON CONFLICT (job_id, dataset_id, day) WHERE cleared_at IS NULL DO UPDATE SET reason = EXCLUDED.reason`
	clearGapSQL = `UPDATE ops.job_gap SET cleared_at = now() ` +
		`WHERE job_id = $1 AND dataset_id = $2 AND day = $3 AND cleared_at IS NULL`
	markGapReportedSQL = `UPDATE ops.job_gap SET reported_at = now() ` +
		`WHERE job_id = $1 AND dataset_id = $2 AND day = $3 AND cleared_at IS NULL AND reported_at IS NULL`
	setStateSQL = `INSERT INTO ops.scheduler_state (id, build, updated_at) VALUES (true, $1, now()) ` +
		`ON CONFLICT (id) DO UPDATE SET build = EXCLUDED.build, updated_at = now()`
	deleteJobStatesSQL = `DELETE FROM ops.scheduler_job`
	insertJobStateSQL  = `INSERT INTO ops.scheduler_job (job_id, next_start, window_end, restart_sensitive, disabled_reason) ` +
		`VALUES ($1, $2, $3, $4, $5)`
	selectStateSQL     = `SELECT build FROM ops.scheduler_state WHERE id`
	selectJobStatesSQL = `SELECT job_id, next_start, window_end, restart_sensitive, disabled_reason ` +
		`FROM ops.scheduler_job ORDER BY job_id`
	selectLiveRunsSQL = `SELECT run_id, session_day, dataset_id, subject_key, status, lease_until, heartbeat_at, ` +
		`returned_n, usable_n, error_code, reason_code FROM ops.capture_run WHERE status = 'running' ORDER BY run_id`
	selectOpenGapsOpsSQL = `SELECT job_id, dataset_id, day, reason, first_seen_at, reported_at, cleared_at ` +
		`FROM ops.job_gap WHERE cleared_at IS NULL ORDER BY first_seen_at, day, job_id, dataset_id`
)

// OpenFence returns the instance fence over the database named by dsn. It checks the DSN and connects nothing:
// TryLock opens the dedicated connection.
func OpenFence(_ context.Context, dsn string) (store.InstanceFence, error) {
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, permanent(fmt.Errorf("ops: open fence: %w", err))
	}
	return &instanceFence{cfg: cfg}, nil
}

type instanceFence struct{ cfg *pgx.ConnConfig }

// TryLock opens one connection, takes the advisory lock on it without waiting and keeps the connection until Close.
// build names the holder in the application_name of the session. Another holder gives a Retryable error.
func (f *instanceFence) TryLock(ctx context.Context, build string) (store.Fence, error) {
	cfg := f.cfg.Copy()
	if cfg.RuntimeParams == nil {
		cfg.RuntimeParams = map[string]string{}
	}
	cfg.RuntimeParams["application_name"] = "ot-scheduler " + build
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return nil, retryable(fmt.Errorf("ops: fence connect: %w", err))
	}
	var got bool
	if err := conn.QueryRow(ctx, tryFenceSQL, fenceKey).Scan(&got); err != nil {
		_ = conn.Close(context.WithoutCancel(ctx))
		return nil, retryable(fmt.Errorf("ops: fence lock: %w", err))
	}
	if !got {
		_ = conn.Close(context.WithoutCancel(ctx))
		return nil, retryable(errors.New("ops: the fence is held by another scheduler"))
	}
	return newFence(ctx, conn), nil
}

// fence is a held lock with its connection. A goroutine waits on the connection, which is otherwise idle, and
// closes lost when the connection fails; the lock lives and dies with that session.
type fence struct {
	conn   *pgx.Conn
	lost   chan struct{}
	cancel context.CancelFunc
	done   chan struct{}
	once   sync.Once
	err    error
}

func newFence(parent context.Context, conn *pgx.Conn) *fence {
	ctx, cancel := context.WithCancel(context.WithoutCancel(parent))
	f := &fence{conn: conn, lost: make(chan struct{}), cancel: cancel, done: make(chan struct{})}
	go func() {
		defer close(f.done)
		// No channel is listened on, so this returns only when the connection fails or ctx is cancelled.
		if _, err := conn.WaitForNotification(ctx); err != nil && ctx.Err() == nil {
			close(f.lost)
		}
	}()
	return f
}

func (f *fence) Lost() <-chan struct{} { return f.lost }

// Close ends the session, which frees the lock. It is safe to call more than once.
func (f *fence) Close() error {
	f.once.Do(func() {
		f.cancel()
		<-f.done
		f.err = f.conn.Close(context.Background())
		select {
		case <-f.lost:
		default:
			close(f.lost)
		}
	})
	return f.err
}

type schedLedger struct{ db database }

// OpenSchedulerLedger connects to the database named by dsn as the writer of the scheduler tables.
func OpenSchedulerLedger(ctx context.Context, dsn string) (store.SchedulerLedger, error) {
	db, err := openPool(ctx, dsn)
	if err != nil {
		return nil, err
	}
	return &schedLedger{db: db}, nil
}

// openPool connects a pool and pings it; the error is Permanent for a bad DSN and Retryable for a database that is down.
func openPool(ctx context.Context, dsn string) (database, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, permanent(fmt.Errorf("ops: open: %w", err))
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, retryable(fmt.Errorf("ops: ping: %w", err))
	}
	return pgDB{pool: pool}, nil
}

func gapArgs(g store.Gap) (time.Time, error) { return parseDay(g.Day) }

// RenewLease extends l to the database's now() plus ttl. Another owner, or no lease, gives a Retryable error.
func (s *schedLedger) RenewLease(ctx context.Context, l store.Lease, ttl time.Duration) (store.Lease, error) {
	var out store.Lease
	got := false
	err := s.db.Select(ctx, renewLeaseSQL, []any{l.Owner, ttl.Seconds()}, func(sc scanner) error {
		var exp time.Time
		if err := sc.Scan(&out.Owner, &exp); err != nil {
			return err
		}
		out.ExpiresAt = exp.UTC()
		got = true
		return nil
	})
	if err != nil {
		return store.Lease{}, wrap("renew lease", err)
	}
	if !got {
		return store.Lease{}, retryable(fmt.Errorf("ops: lease of %q is held by another owner", l.Owner))
	}
	return out, nil
}

func (s *schedLedger) PutGap(ctx context.Context, g store.Gap) error {
	d, err := gapArgs(g)
	if err != nil {
		return err
	}
	return wrap("put gap", s.db.Exec(ctx, putGapSQL, g.JobID, g.DatasetID, d, g.Reason))
}

func (s *schedLedger) ClearGap(ctx context.Context, g store.Gap) error {
	d, err := gapArgs(g)
	if err != nil {
		return err
	}
	return wrap("clear gap", s.db.Exec(ctx, clearGapSQL, g.JobID, g.DatasetID, d))
}

func (s *schedLedger) MarkGapReported(ctx context.Context, g store.Gap) error {
	d, err := gapArgs(g)
	if err != nil {
		return err
	}
	return wrap("mark gap reported", s.db.Exec(ctx, markGapReportedSQL, g.JobID, g.DatasetID, d))
}

func (s *schedLedger) SetState(ctx context.Context, build string) error {
	return wrap("set state", s.db.Exec(ctx, setStateSQL, build))
}

// PutJobStates deletes every row and inserts states in one transaction.
func (s *schedLedger) PutJobStates(ctx context.Context, states []store.JobState) error {
	err := s.db.InTx(ctx, func(q querier) error {
		if err := q.Exec(ctx, deleteJobStatesSQL); err != nil {
			return err
		}
		for _, st := range states {
			if err := q.Exec(ctx, insertJobStateSQL, st.JobID, nullable(st.NextStart), nullable(st.WindowEnd),
				st.RestartSensitive, st.DisabledReason); err != nil {
				return err
			}
		}
		return nil
	})
	return wrap("put job states", err)
}

// Status reads the build, the running runs, the job states and the open gaps from ops.* in one pass of four queries.
func (s *schedLedger) Status(ctx context.Context) (store.SchedulerStatus, error) {
	st := store.SchedulerStatus{Runs: []store.Run{}, Jobs: []store.JobState{}}
	err := s.db.Select(ctx, selectStateSQL, nil, func(sc scanner) error { return sc.Scan(&st.Build) })
	if err != nil {
		return store.SchedulerStatus{}, wrap("status", err)
	}
	err = s.db.Select(ctx, selectLiveRunsSQL, nil, func(sc scanner) error {
		r := store.Run{}
		var day time.Time
		var status string
		var lease, hb *time.Time
		if err := sc.Scan(&r.ID, &day, &r.Key.DatasetID, &r.Key.SubjectKey, &status, &lease, &hb,
			&r.ReturnedN, &r.UsableN, &r.ErrorCode, &r.ReasonCode); err != nil {
			return err
		}
		r.Day = contracts.Day(day.Format(time.DateOnly))
		r.Status = store.RunStatus(status)
		r.LeaseUntil, r.HeartbeatAt = fromNullable(lease), fromNullable(hb)
		st.Runs = append(st.Runs, r)
		return nil
	})
	if err != nil {
		return store.SchedulerStatus{}, wrap("status runs", err)
	}
	err = s.db.Select(ctx, selectJobStatesSQL, nil, func(sc scanner) error {
		var j store.JobState
		var next, end *time.Time
		if err := sc.Scan(&j.JobID, &next, &end, &j.RestartSensitive, &j.DisabledReason); err != nil {
			return err
		}
		j.NextStart, j.WindowEnd = fromNullable(next), fromNullable(end)
		st.Jobs = append(st.Jobs, j)
		return nil
	})
	if err != nil {
		return store.SchedulerStatus{}, wrap("status jobs", err)
	}
	gaps, err := readGaps(ctx, s.db, selectOpenGapsOpsSQL)
	if err != nil {
		return store.SchedulerStatus{}, wrap("status gaps", err)
	}
	st.Gaps = gaps
	return st, nil
}

func (s *schedLedger) Close() error { return s.db.Close() }
