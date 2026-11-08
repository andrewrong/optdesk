package reconcile

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
)

var _ runner.NewKeyRunnerFunc = NewKeyRunner

const (
	// beat is the period of the heartbeat of a run and of the renewal of the run lease.
	beat = 30 * time.Second
	// reasonAbandonedStop is the reason of a run cut with no rows returned (Decision 10).
	reasonAbandonedStop = "abandoned_stop"
	// defaultRunBound is the run bound of a job that has no Bounded method.
	defaultRunBound = 30 * time.Minute
)

type keyRunner struct {
	ops store.OpsLedger
	clk clockwork.Clock
}

// NewKeyRunner builds the one path that runs a job for one key. It takes no lease of its
// own in stage 1 (the caller holds ops.run_lease), so the SchedulerLedger of the
// constructor type is not used.
func NewKeyRunner(ops store.OpsLedger, _ store.SchedulerLedger, clk clockwork.Clock) runner.KeyRunner {
	return &keyRunner{ops: ops, clk: clk}
}

// RunKey runs key through job under a context that ends at deadline, and records the run
// row with a heartbeat every 30 s of the clock. A run cut with rows is partial, a run cut
// without rows is abandoned (abandoned_stop); neither marks the key present. A job with a
// Postcondition marks the key present only when it answers true with a nil error.
func (k *keyRunner) RunKey(
	ctx context.Context, job contracts.Job, day contracts.Day, key contracts.ExpectedKey, attempt int, closeAt, deadline time.Time, reason string,
) (store.Run, error) {
	// The row is written after a cut (a drain cancels ctx), so the writes outlive ctx.
	wctx := context.WithoutCancel(ctx)
	now := k.clk.Now()
	rec := store.Run{
		ID:          fmt.Sprintf("%s|%s|%s|%d", day, key.DatasetID, key.SubjectKey, attempt),
		Day:         day,
		Key:         key,
		Status:      store.RunRunning,
		LeaseUntil:  now.Add(leaseTTL),
		HeartbeatAt: now,
		ReasonCode:  reason,
	}
	if err := k.ops.RecordRun(wctx, rec); err != nil {
		return rec, err
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	cut := k.clk.AfterFunc(deadline.Sub(now), cancel)
	defer cut.Stop()
	stop := k.heartbeat(wctx, runCtx, rec)

	res := job.Run(runCtx, day, []contracts.ExpectedKey{key})
	cutNow := runCtx.Err() != nil
	stop()

	rec.FinishedAt = k.clk.Now()
	rec.ReturnedN, rec.UsableN = res.Returned, res.Usable
	var status store.PresentStatus
	switch {
	case cutNow && res.Returned > 0:
		rec.Status = store.RunPartial
	case cutNow:
		rec.Status = store.RunAbandoned
		rec.ReasonCode = reasonAbandonedStop
	default:
		rec.Status = store.RunDone
		var err error
		if status, err = k.verdict(ctx, job, day, key, closeAt, res, &rec); err != nil {
			return rec, err
		}
	}
	if err := k.ops.RecordRun(wctx, rec); err != nil {
		return rec, err
	}
	if status == "" {
		return rec, nil
	}
	return rec, k.ops.MarkPresent(wctx, day, key, status)
}

// verdict decides the mark of a run that was not cut and sets ErrorCode or ReasonCode on rec.
func (k *keyRunner) verdict(
	ctx context.Context, job contracts.Job, day contracts.Day, key contracts.ExpectedKey, closeAt time.Time, res contracts.JobResult, rec *store.Run,
) (store.PresentStatus, error) {
	switch {
	case res.Err != nil && res.Err.Kind() == contracts.NoValue:
		return store.StatusNoValue, nil
	case res.Err != nil:
		rec.ErrorCode = string(res.Err.Kind())
		return "", nil
	}
	if pc, ok := job.(runner.Postcondition); ok {
		// A false answer or an error is a done run without a mark: the key stays missing.
		if exists, err := pc.Postcondition(ctx, string(day), key.DatasetID, key.SubjectKey); err == nil && exists {
			return store.StatusPresent, nil
		}
		return "", nil
	}
	switch {
	case res.Usable > 0:
		return store.StatusPresent, nil
	case !closeAt.IsZero() && k.clk.Now().Before(closeAt):
		// An empty answer is no explanation: retry until the window closes.
		rec.ReasonCode = reasonEmptyBeforeClose
		return "", nil
	}
	return store.StatusNoValue, nil
}

// heartbeat renews the run row every 30 s until the returned stop is called, which waits
// for the last write, so the final row is never overwritten by a beat.
func (k *keyRunner) heartbeat(wctx, runCtx context.Context, rec store.Run) (stop func()) {
	return every(runCtx, k.clk, func() bool {
		now := k.clk.Now()
		rec.HeartbeatAt, rec.LeaseUntil = now, now.Add(leaseTTL)
		// A failed beat is not fatal: the next one, or the final write, follows.
		_ = k.ops.RecordRun(wctx, rec)
		return true
	})
}

// every calls fn on each 30 s tick of clk until ctx ends, fn returns false, or stop is
// called; stop waits for a running fn to return.
func every(ctx context.Context, clk clockwork.Clock, fn func() bool) (stop func()) {
	t := clk.NewTicker(beat)
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-t.Chan():
				if !fn() {
					return
				}
			}
		}
	}()
	return func() { close(done); wg.Wait() }
}

// runBound is the longest time one run of job may take.
func runBound(job contracts.Job) time.Duration {
	if b, ok := job.(runner.Bounded); ok {
		if d := b.RunBound(); d > 0 {
			return d
		}
	}
	return defaultRunBound
}

// deadlineFor is the end of a run started at now: the later of the window close and now,
// plus the run bound of the job. Decision 8 of docs/design/scheduler.md says the window's
// CloseAt plus the bound; that cuts a run started more than one bound after the close
// before it starts, and a dataset with no window has no CloseAt. The existing late-run
// behaviour (TestFullCycleLateRun) needs the later of the two, so this is a stated departure
// that the design file is to be amended for.
func deadlineFor(job contracts.Job, closeAt, now time.Time) time.Time {
	base := now
	if closeAt.After(base) {
		base = closeAt
	}
	return base.Add(runBound(job))
}

// withClockwork returns clk as a clockwork.Clock: itself when it is one, else a clock whose
// Now is clk and whose timers are the real ones.
func withClockwork(clk clock.Clock) clockwork.Clock {
	if c, ok := clk.(clockwork.Clock); ok {
		return c
	}
	return nowClock{Clock: clockwork.NewRealClock(), now: clk}
}

type nowClock struct {
	clockwork.Clock
	now clock.Clock
}

func (c nowClock) Now() time.Time { return c.now.Now() }

// holdLease takes ops.run_lease and, with a SchedulerLedger, renews it every 30 s of the
// clock until release is called. A failed renewal cancels the returned context and its
// error becomes the result of the call. release stops the renewal and frees the lease even
// when ctx was cancelled (a drain); it sets *errp when the call had no error of its own.
func (r *reconciler) holdLease(ctx context.Context) (context.Context, func(errp *error), error) {
	lease, err := r.ledger.Acquire(ctx, owner, leaseTTL)
	if err != nil {
		return ctx, nil, err
	}
	wctx := context.WithoutCancel(ctx)
	runCtx, cancel := context.WithCancel(ctx)
	var (
		mu       sync.Mutex
		renewErr error
		stop     = func() {}
	)
	if r.sched != nil {
		stop = every(runCtx, r.fclk, func() bool {
			mu.Lock()
			defer mu.Unlock()
			l, err := r.sched.RenewLease(wctx, lease, leaseTTL)
			if err != nil {
				renewErr = err
				cancel()
				return false
			}
			lease = l
			return true
		})
	}
	return runCtx, func(errp *error) {
		stop()
		cancel()
		mu.Lock()
		defer mu.Unlock()
		_ = r.ledger.Release(wctx, lease)
		if renewErr != nil && *errp == nil {
			*errp = renewErr
		}
	}, nil
}
