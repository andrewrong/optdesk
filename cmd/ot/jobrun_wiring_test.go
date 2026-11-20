package main

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/notify"
	"github.com/andrewrong/optdesk/internal/store/jobrun"
	"github.com/jonboulle/clockwork"
)

// fakeJobRunStore records the calls the scheduler makes at start.
type fakeJobRunStore struct {
	jobrun.JobRunLedger
	jobrun.JobRunReader
	mu       sync.Mutex
	dsn      string
	cutAt    []time.Time
	released bool
}

func (f *fakeJobRunStore) CloseOpen(_ context.Context, at time.Time) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cutAt = append(f.cutAt, at)
	return 2, nil
}

func (f *fakeJobRunStore) Release() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.released = true
	return nil
}

// The scheduler opens the job-run store with the runner line and closes the rows the previous process left
// open as cut, once, at the start and at the clock of the scheduler.
func TestStartClosesOpenRunsCut(t *testing.T) {
	t.Parallel()
	r := newSchedRig(t, fullEnv)
	st := &fakeJobRunStore{}
	r.a.deps.openJobRun = func(_ context.Context, dsn string) (jobRunStore, error) {
		st.mu.Lock()
		st.dsn = dsn
		st.mu.Unlock()
		return st, nil
	}
	want := r.fw.Now()
	done := r.start()
	r.waitStarted(t, done)
	r.tick()
	_ = r.stop(t, done)
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.dsn != "run-dsn" {
		t.Fatalf("store opened with %q, want the runner line", st.dsn)
	}
	if len(st.cutAt) != 1 || !st.cutAt[0].Equal(want) {
		t.Fatalf("CloseOpen calls %v, want one at %v", st.cutAt, want)
	}
	if !st.released {
		t.Fatal("the store was not released when the scheduler stopped")
	}
}

type countSlow struct{ n *int }

func (c countSlow) NotifySlowRuns(context.Context) (int, error) { *c.n++; return 0, nil }

// The slow-run pass runs in the notify turn of each tick, over the job-run store.
func TestSchedulerRunsSlowPassEachTick(t *testing.T) {
	t.Parallel()
	r := newSchedRig(t, fullEnv)
	r.a.deps.openJobRun = func(context.Context, string) (jobRunStore, error) { return &fakeJobRunStore{}, nil }
	var passes int
	r.a.deps.newSlowNotifier = func(d notify.SlowDeps, _ notify.Sender, _ clockwork.Clock, _ notify.Config) (notify.SlowNotifier, error) {
		if d.Runs == nil || d.Mark == nil || d.Figures == nil {
			t.Error("slow deps incomplete")
		}
		return countSlow{&passes}, nil
	}
	done := r.start()
	r.waitStarted(t, done)
	r.tick()
	r.tick()
	_ = r.stop(t, done)
	if passes != 2 {
		t.Fatalf("slow passes %d, want 2", passes)
	}
}
