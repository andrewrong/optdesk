package main

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/store"
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
