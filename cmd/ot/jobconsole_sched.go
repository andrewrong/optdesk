package main

import (
	"context"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/coverage"
	"github.com/andrewrong/optdesk/internal/store/jobrequest"
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
var _ runner.ConsoleClaimer = (*consoleReconciler)(nil)
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

// ClaimConsole claims the requests of the queue through the host; it returns none when the console is off.
func (c *consoleReconciler) ClaimConsole(ctx context.Context) ([]runner.ConsoleRun, error) {
	if c.host == nil {
		return nil, nil
	}
	if _, on := c.sw.current(); !on {
		return nil, nil
	}
	return c.host.ClaimConsole(ctx)
}

// openConsoleHost opens the consumer with the runner DSN and builds the host over it and the key gap syncer.
// Anything missing (no openers in deps, no syncer constructor, a failed open) leaves the console off: the host
// is nil, one warn line is written and the scheduler goes on. The catch-up pass needs a poster (runner DSN) and a
// coverage reader (pages DSN); with either missing the pass is off, one warn line says so and the console works.
// The func closes everything opened.
func openConsoleHost(
	ctx context.Context, a *app, runnerDSN, pagesDSN string, newSyncer runner.NewKeyGapSyncerFunc,
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
	post, cov := openCatchUp(ctx, a, o, runnerDSN, pagesDSN)
	closeAll := func() {
		_ = q.Close()
		if post != nil {
			_ = post.Close()
		}
		if cov != nil {
			_ = cov.Close()
		}
	}
	return newConsoleHost(q, newSyncer(ops, sl, clk), ops, clk, a.log, post, cov), closeAll
}

// openCatchUp opens the poster and the coverage reader of the catch-up pass; both are nil (and one warn line is
// written) when either cannot be had.
func openCatchUp(ctx context.Context, a *app, o jobRequestOpeners, runnerDSN, pagesDSN string) (jobrequest.Poster, coverage.Reader) {
	off := func(why string) (jobrequest.Poster, coverage.Reader) {
		a.log.Warn("catch-up off: "+why, "component", "ot", "status", "catch-up off")
		return nil, nil
	}
	d, ok := a.deps.ext[extDataHealth].(dataHealthDeps)
	switch {
	case o.Poster == nil:
		return off("no request poster")
	case !ok || d.openCoverage == nil || pagesDSN == "":
		return off("no coverage reader")
	}
	post, err := o.Poster(ctx, runnerDSN)
	if err != nil {
		return off("poster open failed")
	}
	cov, err := d.openCoverage(ctx, pagesDSN)
	if err != nil {
		_ = post.Close()
		return off("coverage open failed")
	}
	return post, cov
}
