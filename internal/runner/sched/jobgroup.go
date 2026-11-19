package sched

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/jonboulle/clockwork"
)

// jobGroup owns every run that a DetachedReconciler spawns: one context, not cancelled by the
// ticker's runCtx (like the console context), and one wait group. Run drains it with the worker
// and the console, then cancels it with the ordered-stop cause, or at once on a lost fence.
type jobGroup struct {
	clk    clockwork.Clock
	ctx    context.Context
	cancel context.CancelCauseFunc
	wg     sync.WaitGroup
}

func newJobGroup(clk clockwork.Clock) *jobGroup {
	ctx, cancel := context.WithCancelCause(context.Background())
	return &jobGroup{clk: clk, ctx: ctx, cancel: cancel}
}

// runContext is the context of one spawned run: a cancelable child of the group context whose
// deadline is reconcileTimeout after the spawn on the ticker's clock. It reports that deadline through
// Deadline, so callees that pass it on see the bound, and Err is context.DeadlineExceeded (not
// Canceled) once the bound ended the run, as with context.WithTimeout. A real context.WithTimeout
// would count wall time and could not be driven by the fake clock of the tests.
type runContext struct {
	context.Context // the cancelable child
	deadline        time.Time
}

func (c runContext) Deadline() (time.Time, bool) { return c.deadline, true }

func (c runContext) Err() error {
	err := c.Context.Err()
	if err != nil && errors.Is(context.Cause(c.Context), context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	return err
}

// spawn is the runner.Spawn of the group: it starts run in a goroutine of its own under the group
// context, bounded by reconcileTimeout counted on the ticker's clock (a run that hits it sees
// context.DeadlineExceeded as its Err and its cause, ends cut and leaves its keys for the next pass).
// The bound is the turn's old one moved to the run; it is the only time limit, and it is not a refusal.
// spawn is called only from the worker, which Run waits for before done, so no Add races the Wait.
func (g *jobGroup) spawn(_ string, run func(ctx context.Context)) {
	child, cancel := context.WithCancelCause(g.ctx)
	ctx := runContext{Context: child, deadline: g.clk.Now().Add(reconcileTimeout)}
	tm := g.clk.NewTimer(reconcileTimeout)
	g.wg.Go(func() {
		defer tm.Stop()
		stopped := make(chan struct{})
		defer func() { <-stopped }()
		defer cancel(nil)
		go func() {
			defer close(stopped)
			select {
			case <-tm.Chan():
				cancel(context.DeadlineExceeded)
			case <-child.Done():
			}
		}()
		run(ctx) //nolint:contextcheck // runContext wraps a child of the group context
	})
}

var _ runner.Spawn = (*jobGroup)(nil).spawn

// done is closed when every spawned run has returned.
func (g *jobGroup) done() <-chan struct{} {
	c := make(chan struct{})
	go func() { g.wg.Wait(); close(c) }()
	return c
}
