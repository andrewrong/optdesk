package sched

import (
	"context"
	"sync"

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

// spawn is the runner.Spawn of the group: it starts run in a goroutine of its own under the group
// context, bounded by reconcileTimeout counted on the ticker's clock (a run that hits it sees
// context.DeadlineExceeded as its cause, ends cut and leaves its keys for the next pass). The
// bound is the turn's old one moved to the run; it is the only time limit, and it is not a refusal.
func (g *jobGroup) spawn(_ string, run func(ctx context.Context)) {
	ctx, cancel := context.WithCancelCause(g.ctx)
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
			case <-ctx.Done():
			}
		}()
		run(ctx)
	})
}

var _ runner.Spawn = (*jobGroup)(nil).spawn

// done is closed when every spawned run has returned.
func (g *jobGroup) done() <-chan struct{} {
	c := make(chan struct{})
	go func() { g.wg.Wait(); close(c) }()
	return c
}
