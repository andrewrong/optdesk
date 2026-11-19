package main

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/jonboulle/clockwork"
)

// prevBuildLedger is a scheduler ledger whose Status answers a given build or error.
type prevBuildLedger struct {
	*fakeSchedLedger
	build string
	err   error
}

func (l prevBuildLedger) Status(context.Context) (store.SchedulerStatus, error) {
	return store.SchedulerStatus{Build: l.build}, l.err
}

// runCloseAtStart starts the scheduler with a previous-build answer and returns what CloseRunning saw and how many
// times it ran before the ticker was built.
func runCloseAtStart(t *testing.T, prev prevBuildLedger) (reason any, calls, beforeBuild int32) {
	t.Helper()
	r := newSchedRig(t, fullEnv)
	prev.fakeSchedLedger = r.sl
	r.a.deps.openSchedulerLedger = func(context.Context, string) (store.SchedulerLedger, error) { return prev, nil }
	l := &closeRecLedger{fakeLedger: fakeLedger{n: new(atomic.Int32)}}
	useLedger(r, l)
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
	return l.reason.Load(), l.calls.Load(), beforeBuild
}

func TestCloseRunningAtStartOfAnotherBuildIsOrderedStop(t *testing.T) {
	t.Parallel()
	reason, calls, before := runCloseAtStart(t, prevBuildLedger{build: "another-build"})
	if reason != "ordered_stop" || calls != 1 || before != 1 {
		t.Fatalf("reason %v, calls %d, before the ticker %d", reason, calls, before)
	}
}

func TestCloseRunningAtStartOfTheSameBuildIsCrashed(t *testing.T) {
	t.Parallel()
	cases := map[string]prevBuildLedger{
		"same build":   {build: buildIDOf(version)},
		"empty build":  {},
		"status error": {build: "another-build", err: errors.New("dial postgres://u:secret@h/db")},
	}
	for name, prev := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			reason, calls, before := runCloseAtStart(t, prev)
			if reason != "crashed" || calls != 1 || before != 1 {
				t.Fatalf("reason %v, calls %d, before the ticker %d", reason, calls, before)
			}
		})
	}
}
