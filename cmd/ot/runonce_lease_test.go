package main

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs/computefigures"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
)

// leaseLedger is a ledger whose run lease another owner holds: Acquire fails Retryable with secret text, and Runs
// returns runs.
type leaseLedger struct {
	fakeLedger
	runs []store.Run
}

type secretRetryable struct{}

func (secretRetryable) Error() string             { return "secret-ledger-text" }
func (secretRetryable) Kind() contracts.ErrorKind { return contracts.Retryable }

func (l leaseLedger) Acquire(context.Context, string, time.Duration) (store.Lease, error) {
	return store.Lease{}, secretRetryable{}
}
func (l leaseLedger) Runs(context.Context, contracts.Day) ([]store.Run, error) { return l.runs, nil }

// acquireRunner is a reconciler that takes the run lease of the ledger the wiring built, as Reconcile does.
type acquireRunner struct{ led store.OpsLedger }

func (r acquireRunner) Reconcile(ctx context.Context, _ contracts.Day) error {
	_, err := r.led.Acquire(ctx, "owner", time.Minute)
	return err
}

// leasePlanJob is a job that plans one key of dataset.
type leasePlanJob struct {
	fakeJob
	id, dataset string
}

func (j leasePlanJob) ID() string { return j.id }
func (j leasePlanJob) Plan(context.Context, contracts.Day) ([]contracts.ExpectedKey, string, error) {
	return []contracts.ExpectedKey{{DatasetID: j.dataset}}, "h", nil
}

// countSchedLedger counts Close calls.
type countSchedLedger struct {
	fakeSchedLedger
	closed *atomic.Int32
}

func (l *countSchedLedger) Close() error { l.closed.Add(1); return nil }

// leaseRig is run-once deps whose ledger holds the lease; slClosed counts Close of the scheduler ledger.
func leaseRig(t *testing.T, runs []store.Run) (a *app, slClosed *atomic.Int32) {
	t.Helper()
	a, _ = testApp(t, fullEnv)
	var opened, closed atomic.Int32
	var got contracts.Day
	runOnceFakes(a, &opened, &closed, &got)
	now := time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)
	a.deps.newFwClock = func() clockwork.Clock { return clockwork.NewFakeClockAt(now) }
	a.deps.openLedger = func(context.Context, string) (store.OpsLedger, error) {
		return leaseLedger{fakeLedger: fakeLedger{n: &closed}, runs: runs}, nil
	}
	a.deps.newComputeFigures = func(
		computefigures.ComputeStores, compute.Figures, compute.Rules, compute.Units, compute.SeriesOps, compute.Ease, clock.Clock,
	) (contracts.Job, error) {
		return leasePlanJob{id: "compute_figures", dataset: "figures.v1"}, nil
	}
	slClosed = new(atomic.Int32)
	a.deps.openSchedulerLedger = func(context.Context, string) (store.SchedulerLedger, error) {
		return &countSchedLedger{closed: slClosed}, nil
	}
	a.deps.newReconciler = func(
		led store.OpsLedger, _ store.SchedulerLedger, _ clock.Calendar, _ clockwork.Clock, _ []contracts.Job, _ runner.WindowRule, _ runner.KeyRunner,
	) (runner.Reconciler, runner.Backfiller) {
		return acquireRunner{led}, nil
	}
	return a, slClosed
}
