package main

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/jobrun"
)

func busyRuns(hb ...time.Time) store.SchedulerStatus {
	var st store.SchedulerStatus
	for i, h := range hb {
		st.Runs = append(st.Runs, store.Run{ID: "r" + string(rune('1'+i)), Status: store.RunRunning, HeartbeatAt: h})
	}
	return st
}

func busyExitCode(t *testing.T, err error) int {
	t.Helper()
	var ee *exitError
	if !errors.As(err, &ee) {
		t.Fatalf("want an exitError, got %v", err)
	}
	return ee.code
}

func TestBusyNewIdle(t *testing.T) {
	t.Parallel()
	r := newStatusRig(t, store.SchedulerStatus{})
	if err := schedBusyCmd(r.a, nil); err != nil {
		t.Fatal(err)
	}
	if r.out.String() != "idle\n" {
		t.Fatalf("output %q", r.out.String())
	}
	if err := schedBusyCmd(r.a, []string{"x"}); err == nil {
		t.Fatal("extra argument must be a usage error")
	}
}

func TestBusyNewFreshExits75(t *testing.T) {
	t.Parallel()
	r := newStatusRig(t, busyRuns(statusNow.Add(-89*time.Second)))
	err := schedBusyCmd(r.a, nil)
	if c := busyExitCode(t, err); c != 75 {
		t.Fatalf("code %d", c)
	}
	if !strings.Contains(err.Error(), "r1") {
		t.Fatalf("text lacks the run id: %v", err)
	}
}

func TestBusyNewStaleIsIdle(t *testing.T) {
	t.Parallel()
	// killed mid-way: the row is still running, the heartbeat stopped.
	r := newStatusRig(t, busyRuns(statusNow.Add(-90*time.Second), statusNow.Add(-time.Hour), time.Time{}))
	if err := schedBusyCmd(r.a, nil); err != nil {
		t.Fatal(err)
	}
	if r.out.String() != "idle\n" {
		t.Fatalf("output %q", r.out.String())
	}
}

type failLedger struct {
	statusLedger
}

func (l *failLedger) Status(context.Context) (store.SchedulerStatus, error) {
	return store.SchedulerStatus{}, errors.New("down")
}

func TestBusyNewErrorExits1(t *testing.T) {
	t.Parallel()
	r := newStatusRig(t, store.SchedulerStatus{})
	if err := schedBusyCmd(r.a, nil); err != nil {
		t.Fatal(err)
	}
	r.a.deps.openSchedulerLedger = func(context.Context, string) (store.SchedulerLedger, error) {
		return &failLedger{}, nil
	}
	if c := busyExitCode(t, schedBusyCmd(r.a, nil)); c != 1 {
		t.Fatalf("read failure code %d", c)
	}
	r.a.deps.openSchedulerLedger = func(context.Context, string) (store.SchedulerLedger, error) {
		return nil, errors.New("down")
	}
	if c := busyExitCode(t, schedBusyCmd(r.a, nil)); c != 1 {
		t.Fatalf("open failure code %d", c)
	}
}

// writeCountLedger embeds the interface: any method but Status and Close panics, and Status/Close count.
type writeCountLedger struct {
	store.SchedulerLedger
	calls atomic.Int32
}

func (l *writeCountLedger) Status(context.Context) (store.SchedulerStatus, error) {
	l.calls.Add(1)
	return busyRuns(statusNow), nil
}
func (l *writeCountLedger) Close() error { return nil }

func TestBusyNewWritesNothing(t *testing.T) {
	t.Parallel()
	r := newStatusRig(t, store.SchedulerStatus{})
	l := &writeCountLedger{}
	r.a.deps.openSchedulerLedger = func(context.Context, string) (store.SchedulerLedger, error) { return l, nil }
	_ = schedBusyCmd(r.a, nil) // a call of any other method would panic on the nil embedded interface
	if l.calls.Load() != 1 {
		t.Fatalf("Status called %d times", l.calls.Load())
	}
	if n := r.forbiddenOpens.Load(); n != 0 {
		t.Fatalf("a dataset store or the broker was opened %d times", n)
	}
}

// fakeJobRuns embeds the interfaces: any method but Running and Release panics on the nil embedded interface.
type fakeJobRuns struct {
	jobRunStore
	rows     []jobrun.JobRun
	err      error
	released atomic.Int32
}

func (f *fakeJobRuns) Running(context.Context) ([]jobrun.JobRun, error) { return f.rows, f.err }
func (f *fakeJobRuns) Release() error                                   { f.released.Add(1); return nil }

func withJobRuns(r *statusRig, f *fakeJobRuns) {
	r.a.deps.openJobRun = func(context.Context, string) (jobRunStore, error) { return f, nil }
}

func TestBusyJobRunOpenExits75(t *testing.T) {
	t.Parallel()
	r := newStatusRig(t, store.SchedulerStatus{})
	withJobRuns(r, &fakeJobRuns{rows: []jobrun.JobRun{{ID: "jr7", HeartbeatAt: statusNow.Add(-time.Minute)}}})
	err := schedBusyCmd(r.a, nil)
	if c := busyExitCode(t, err); c != 75 {
		t.Fatalf("code %d", c)
	}
	if !strings.HasPrefix(err.Error(), "busy: ") || !strings.Contains(err.Error(), "jr7") {
		t.Fatalf("text %v", err)
	}
}

func TestBusyJobRunNoneIsIdle(t *testing.T) {
	t.Parallel()
	r := newStatusRig(t, store.SchedulerStatus{})
	withJobRuns(r, &fakeJobRuns{})
	if err := schedBusyCmd(r.a, nil); err != nil {
		t.Fatal(err)
	}
	if r.out.String() != "idle\n" {
		t.Fatalf("output %q", r.out.String())
	}
}

func TestBusyJobRunStaleIsIdle(t *testing.T) {
	t.Parallel()
	r := newStatusRig(t, store.SchedulerStatus{})
	withJobRuns(r, &fakeJobRuns{rows: []jobrun.JobRun{{ID: "jr1", HeartbeatAt: statusNow.Add(-11 * time.Minute)}}})
	if err := schedBusyCmd(r.a, nil); err != nil {
		t.Fatal(err)
	}
	if r.out.String() != "idle\n" {
		t.Fatalf("output %q", r.out.String())
	}
}

func TestBusyJobRunReadErrorExits1(t *testing.T) {
	t.Parallel()
	r := newStatusRig(t, store.SchedulerStatus{})
	withJobRuns(r, &fakeJobRuns{err: errors.New("dsn=secret")})
	err := schedBusyCmd(r.a, nil)
	if c := busyExitCode(t, err); c != 1 {
		t.Fatalf("read failure code %d", c)
	}
	if strings.Contains(err.Error(), "secret") {
		t.Fatalf("text leaks: %v", err)
	}
	r.a.deps.openJobRun = func(context.Context, string) (jobRunStore, error) { return nil, errors.New("dsn=secret") }
	err = schedBusyCmd(r.a, nil)
	if c := busyExitCode(t, err); c != 1 {
		t.Fatalf("open failure code %d", c)
	}
	if strings.Contains(err.Error(), "secret") {
		t.Fatalf("text leaks: %v", err)
	}
}

func TestBusyJobRunReleased(t *testing.T) {
	t.Parallel()
	for name, f := range map[string]*fakeJobRuns{
		"busy":    {rows: []jobrun.JobRun{{ID: "jr1", HeartbeatAt: statusNow}}},
		"idle":    {},
		"failing": {err: errors.New("down")},
	} {
		r := newStatusRig(t, store.SchedulerStatus{})
		withJobRuns(r, f)
		_ = schedBusyCmd(r.a, nil)
		if n := f.released.Load(); n != 1 {
			t.Fatalf("%s: Release called %d times", name, n)
		}
	}
}
