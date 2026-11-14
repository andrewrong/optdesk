package main

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/jobrequest"
	"github.com/jonboulle/clockwork"
)

// Design: docs/design/job-console.md#Decisions 8 and 9. The host knows the interfaces of the queue, the
// backfiller and the gap syncer, and nothing of what implements them.

// The limits of one turn (Decision 8).
const (
	// startPhase bounds Queued, Claim and each Hold of the queue.
	startPhase = time.Minute
	// finishTimeout bounds the Finish that ends a request.
	finishTimeout = 30 * time.Second
	// syncTimeout bounds the SyncKeys after a request.
	syncTimeout = 2 * time.Minute
	// warnEvery is the least gap between two warn lines about the store.
	warnEvery = time.Minute
	// queueLimit is how many waiting requests one turn reads.
	queueLimit = 50
	// reasonRestart is the reason of a request that a restart found running.
	reasonRestart = "restart"
)

// consoleHost takes a queued request as one turn of the scheduler's worker.
type consoleHost struct {
	q      jobrequest.Consumer
	syncer runner.KeyGapSyncer
	ledger store.OpsLedger
	clk    clockwork.Clock
	log    *slog.Logger

	mu       sync.Mutex
	started  bool
	lastWarn time.Time
	jobs     []contracts.Job
	bf       runner.ForceBackfiller
}

// newConsoleHost returns a host over the queue, the gap syncer, the ops ledger and the clock. The ledger tells
// whether a day has a plan (docs/design/single-runner.md Decision 7). Nothing runs until Start has succeeded and
// set has been called.
func newConsoleHost(q jobrequest.Consumer, syncer runner.KeyGapSyncer, ledger store.OpsLedger, clk clockwork.Clock, log *slog.Logger) *consoleHost {
	return &consoleHost{q: q, syncer: syncer, ledger: ledger, clk: clk, log: log}
}

// Start ends every request a dead process left running as failed with the reason restart. The fence makes
// this process the only consumer, so a running row is never live. ConsoleTurn calls it again until it has
// succeeded.
func (h *consoleHost) Start(ctx context.Context) error {
	if err := h.q.FailRunning(ctx, reasonRestart, h.clk.Now()); err != nil {
		return err
	}
	h.mu.Lock()
	h.started = true
	h.mu.Unlock()
	return nil
}

// set stores the job set and the backfiller of the last build.
func (h *consoleHost) set(jobs []contracts.Job, bf runner.ForceBackfiller) {
	h.mu.Lock()
	h.jobs, h.bf = jobs, bf
	h.mu.Unlock()
}

// ConsoleTurn is one turn of the queue (runner.ConsoleTurner); the order of it is in turn.go.
// The slot hold is ignored: the console no longer holds a request for a slot.
func (h *consoleHost) ConsoleTurn(ctx context.Context, _ runner.SlotHold) (bool, error) {
	return h.turn(ctx)
}

// ClaimConsole claims the first request of the queue left after the withdrawn are finished, and returns one
// ConsoleRun for it (none when the queue holds nothing to take). It starts no goroutine; the ticker runs the
// ConsoleRun. A store failure is warned and ends the claim with no run, as in ConsoleTurn.
func (h *consoleHost) ClaimConsole(ctx context.Context) ([]runner.ConsoleRun, error) {
	_, run := h.claimNext(ctx)
	if run == nil {
		return nil, nil
	}
	return []runner.ConsoleRun{run}, nil
}

// current returns the job set and the backfiller, and whether the host has started.
func (h *consoleHost) current() (jobs []contracts.Job, bf runner.ForceBackfiller, started bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.jobs, h.bf, h.started
}

// warn writes a warn line about the store at most once a minute. It never carries the error text, which may
// hold a DSN.
func (h *consoleHost) warn(msg string) {
	now := h.clk.Now()
	h.mu.Lock()
	due := h.lastWarn.IsZero() || now.Sub(h.lastWarn) >= warnEvery
	if due {
		h.lastWarn = now
	}
	h.mu.Unlock()
	if due && h.log != nil {
		h.log.Warn(msg, "component", "ot", "status", "console off")
	}
}

// clockTimeout is a context cancelled by the injected clock after d, so a test needs no sleep.
func clockTimeout(parent context.Context, clk clockwork.Clock, d time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancelCause(parent)
	t := clk.AfterFunc(d, func() { cancel(context.DeadlineExceeded) })
	return ctx, func() {
		t.Stop()
		cancel(context.Canceled)
	}
}
