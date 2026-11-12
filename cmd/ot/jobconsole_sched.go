package main

import (
	"context"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/jonboulle/clockwork"
)

// Design: docs/design/job-console.md#Decisions 9 and entry 9.

// consoleReconciler is the Reconciler the ticker holds: the swapReconciler with the console turn added. It
// forwards Reconcile and Probe to the swapReconciler, so a probe that brings a job back still rebuilds the
// reconciler (and, through build, hands the host the new job set). A nil host is a console that is off: no
// turn is taken.
type consoleReconciler struct {
	sw   *swapReconciler
	host *consoleHost
}

var _ runner.Reconciler = (*consoleReconciler)(nil)
var _ runner.Prober = (*consoleReconciler)(nil)
var _ runner.ConsoleTurner = (*consoleReconciler)(nil) //nolint:staticcheck // SA1019: ConsoleTurner stays until a later design deletes it

func (c *consoleReconciler) Reconcile(ctx context.Context, day contracts.Day) error {
	return c.sw.Reconcile(ctx, day)
}

func (c *consoleReconciler) Probe(ctx context.Context, id string) string { return c.sw.Probe(ctx, id) }

// ConsoleTurn takes one request of the queue when the console is on.
func (c *consoleReconciler) ConsoleTurn(ctx context.Context, hold runner.SlotHold) (bool, error) {
	if c.host == nil {
		return false, nil
	}
	if _, on := c.sw.current(); !on {
		return false, nil
	}
	return c.host.ConsoleTurn(ctx, hold)
}

// openConsoleHost opens the consumer with the runner DSN and builds the host over it and the key gap syncer.
// Anything missing (no openers in deps, no syncer constructor, a failed open) leaves the console off: the host
// is nil, one warn line is written and the scheduler goes on. The func closes the consumer.
func openConsoleHost(
	ctx context.Context, a *app, runnerDSN string, newSyncer runner.NewKeyGapSyncerFunc,
	ops store.OpsLedger, sl store.SchedulerLedger, clk clockwork.Clock,
) (*consoleHost, func()) {
	noop := func() {}
	o, ok := jobRequestOpenersOf(a.deps)
	if !ok || o.Consumer == nil || newSyncer == nil {
		return nil, noop
	}
	q, err := o.Consumer(ctx, runnerDSN)
	if err != nil {
		consoleOff(a, "open failed")
		return nil, noop
	}
	return newConsoleHost(q, newSyncer(ops, sl, clk), clk, a.log), func() { _ = q.Close() }
}
