package main

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/jonboulle/clockwork"
)

// The scheduler asserts its ops ledger to store.RunCloser (docs/design/single-runner.md Decision 5), so the
// ops-ledger fakes of the tests that run the scheduler through openLedger close nothing and say so here.
func (fakeLedger) CloseRunning(context.Context, time.Time, string) (int, error) { return 0, nil }
func (evLedger) CloseRunning(context.Context, time.Time, string) (int, error)   { return 0, nil }
func (cursorLedger) CloseRunning(context.Context, time.Time, string) (int, error) {
	return 0, nil
}

// closeRecLedger records the CloseRunning calls of the scheduler.
type closeRecLedger struct {
	fakeLedger
	calls  atomic.Int32
	reason atomic.Value
	now    atomic.Value
	err    error
}

func (l *closeRecLedger) CloseRunning(_ context.Context, now time.Time, reason string) (int, error) {
	l.calls.Add(1)
	l.reason.Store(reason)
	l.now.Store(now)
	return 3, l.err
}

// noCloseLedger is an ops ledger that is no store.RunCloser.
type noCloseLedger struct{ store.OpsLedger }

func (noCloseLedger) Close() error { return nil }

func useLedger(r *schedRig, l store.OpsLedger) {
	r.a.deps.openLedger = func(context.Context, string) (store.OpsLedger, error) { return l, nil }
}

func TestSchedulerClosesRunningRowsAtStartBeforeTheTicker(t *testing.T) {
	t.Parallel()
	r := newSchedRig(t, fullEnv)
	l := &closeRecLedger{fakeLedger: fakeLedger{n: new(atomic.Int32)}}
	useLedger(r, l)
	var beforeBuild int32
	build := r.a.deps.newScheduler
	r.a.deps.newScheduler = func(
		f store.Fence, sl store.SchedulerLedger, o store.OpsLedger, c clock.Calendar, k clockwork.Clock,
		rec runner.Reconciler, g runner.GapSyncer, j []contracts.Job, d map[string]string, e runner.ExecIdentityFunc,
	) runner.Scheduler {
		beforeBuild = l.calls.Load()
		return build(f, sl, o, c, k, rec, g, j, d, e)
	}
	done := r.start()
	r.waitStarted(t, done)
	if err := r.stop(t, done); err != nil {
		t.Fatal(err)
	}
	if beforeBuild != 1 || l.calls.Load() != 1 {
		t.Fatalf("CloseRunning calls: %d before the ticker was built, %d in all", beforeBuild, l.calls.Load())
	}
	if got := l.reason.Load(); got != "crashed" {
		t.Fatalf("reason %v", got)
	}
	if got, _ := l.now.Load().(time.Time); !got.Equal(r.fw.Now()) {
		t.Fatalf("now %v, clock %v", got, r.fw.Now())
	}
}

func TestSchedulerExitsWhenCloseRunningFails(t *testing.T) {
	t.Parallel()
	r := newSchedRig(t, fullEnv)
	l := &closeRecLedger{fakeLedger: fakeLedger{n: new(atomic.Int32)}, err: errors.New("dial postgres://u:secret@h/db")}
	useLedger(r, l)
	err := <-r.start()
	if err == nil || r.sched != nil {
		t.Fatalf("err %v, ticker built %v", err, r.sched != nil)
	}
	if got := err.Error(); strings.Contains(got, "secret") || strings.Contains(got, "postgres://") {
		t.Fatalf("the error leaks: %q", got)
	}
}

func TestSchedulerExitsWhenTheLedgerIsNoRunCloser(t *testing.T) {
	t.Parallel()
	r := newSchedRig(t, fullEnv)
	useLedger(r, noCloseLedger{})
	err := <-r.start()
	if err == nil || r.sched != nil {
		t.Fatalf("err %v, ticker built %v", err, r.sched != nil)
	}
}

func TestSchedulerDoesNotCloseRunningRowsWithoutTheFence(t *testing.T) {
	t.Parallel()
	r := newSchedRig(t, fullEnv)
	l := &closeRecLedger{fakeLedger: fakeLedger{n: new(atomic.Int32)}}
	useLedger(r, l)
	r.inst.held = true
	r.cancel() // lockFence ends at its first wait
	if err := <-r.start(); err == nil {
		t.Fatal("the scheduler started without the fence")
	}
	if l.calls.Load() != 0 {
		t.Fatalf("CloseRunning called %d times without the fence", l.calls.Load())
	}
}
