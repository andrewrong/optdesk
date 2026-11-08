package main

import (
	"bytes"
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/jonboulle/clockwork"
)

// statusLedger answers Status with a fixed value.
type statusLedger struct {
	store.SchedulerLedger
	st     store.SchedulerStatus
	closed atomic.Int32
}

func (l *statusLedger) Status(context.Context) (store.SchedulerStatus, error) { return l.st, nil }
func (l *statusLedger) Close() error                                          { l.closed.Add(1); return nil }

var statusNow = time.Date(2026, 11, 6, 15, 0, 0, 0, time.UTC)

// statusRig is an app whose only working store is the scheduler ledger; every dataset store and the broker
// dial count their calls.
type statusRig struct {
	a        *app
	out      *bytes.Buffer
	led      *statusLedger
	datasets atomic.Int32
}

func newStatusRig(t *testing.T, st store.SchedulerStatus) *statusRig {
	t.Helper()
	a, _ := testApp(t, fullEnv)
	r := &statusRig{a: a, out: &bytes.Buffer{}, led: &statusLedger{st: st}}
	a.stdout = r.out
	a.deps.newFwClock = func() clockwork.Clock { return clockwork.NewFakeClockAt(statusNow) }
	a.deps.openSchedulerLedger = func(context.Context, string) (store.SchedulerLedger, error) { return r.led, nil }
	a.deps.openSeries = func(context.Context, string) (store.SeriesStore, error) { r.datasets.Add(1); return nil, nil }
	a.deps.openOptionEod = func(context.Context, string) (store.OptionEodStore, error) { r.datasets.Add(1); return nil, nil }
	a.deps.openQuoteSnapshot = func(context.Context, string) (store.QuoteSnapshotStore, error) { r.datasets.Add(1); return nil, nil }
	a.deps.openInstrument = func(context.Context, string) (store.InstrumentStore, error) { r.datasets.Add(1); return nil, nil }
	a.deps.dialBroker = func(context.Context, string, broker.Options) (broker.Broker, error) {
		r.datasets.Add(1)
		return nil, nil
	}
	return r
}

func TestSchedStatusOpensNoDatasetStore(t *testing.T) {
	t.Parallel()
	r := newStatusRig(t, store.SchedulerStatus{Build: "b1"})
	if err := schedStatusCmd(r.a, nil); err != nil {
		t.Fatal(err)
	}
	if err := schedCanRestartCmd(r.a, nil); err != nil {
		t.Fatal(err)
	}
	if n := r.datasets.Load(); n != 0 {
		t.Fatalf("a dataset store or the broker was opened %d times", n)
	}
	if r.led.closed.Load() != 2 {
		t.Fatalf("ledger closed %d times, want 2", r.led.closed.Load())
	}
	if !strings.Contains(r.out.String(), "build: b1") {
		t.Fatalf("output lacks the build: %q", r.out.String())
	}
}

func TestSchedStatusGreen(t *testing.T) {
	t.Parallel()
	r := newStatusRig(t, store.SchedulerStatus{
		Build: "b1",
		Runs:  []store.Run{{ID: "run-1", Status: store.RunRunning, HeartbeatAt: statusNow.Add(-30 * time.Second)}},
		Jobs:  []store.JobState{{JobID: "fred_series", NextStart: statusNow.Add(time.Hour)}},
	})
	if err := schedStatusCmd(r.a, nil); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"run-1", "heartbeat 30s ago", "fred_series next 2026-11-06T16:00:00Z"} {
		if !strings.Contains(r.out.String(), want) {
			t.Errorf("output lacks %q: %q", want, r.out.String())
		}
	}
}

func TestSchedStatusRedOnOpenGapOrDisabledJob(t *testing.T) {
	t.Parallel()
	gap := store.Gap{JobID: "option_eod", Reason: "preflight_failed", Day: "2026-11-06"}
	r := newStatusRig(t, store.SchedulerStatus{
		Build: "b1",
		Jobs:  []store.JobState{{JobID: "option_eod", DisabledReason: "broker down"}},
		Gaps:  []store.Gap{gap},
	})
	err := schedStatusCmd(r.a, nil)
	if exitCodeOf(err) != 1 {
		t.Fatalf("exit %d (%v), want 1", exitCodeOf(err), err)
	}
	if n := strings.Count(r.out.String(), "preflight_failed option_eod 2026-11-06"); n != 1 {
		t.Fatalf("gap listed %d times, want 1: %q", n, r.out.String())
	}
	// A disabled job alone is red too.
	r = newStatusRig(t, store.SchedulerStatus{Jobs: []store.JobState{{JobID: "x", DisabledReason: "no key"}}})
	if exitCodeOf(schedStatusCmd(r.a, nil)) != 1 {
		t.Fatal("a left-out job is not red")
	}
}

func TestCanRestart(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		st   store.SchedulerStatus
		exit int
		text string
	}{
		{"running run", store.SchedulerStatus{Runs: []store.Run{{ID: "r1", Status: store.RunRunning}}}, 1, "r1"},
		{"sensitive job inside its window", store.SchedulerStatus{Jobs: []store.JobState{{
			JobID: "option_eod", RestartSensitive: true,
			NextStart: statusNow.Add(20 * time.Hour), WindowEnd: statusNow.Add(time.Hour),
		}}}, 1, "option_eod"},
		{"sensitive job before its slot", store.SchedulerStatus{Jobs: []store.JobState{{
			JobID: "option_eod", RestartSensitive: true,
			NextStart: statusNow.Add(time.Hour), WindowEnd: statusNow.Add(2 * time.Hour),
		}}}, 0, ""},
		{"idle", store.SchedulerStatus{Jobs: []store.JobState{{
			JobID: "fred_series", NextStart: statusNow.Add(20 * time.Hour), WindowEnd: statusNow.Add(time.Hour),
		}}}, 0, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			r := newStatusRig(t, c.st)
			err := schedCanRestartCmd(r.a, nil)
			if exitCodeOf(err) != c.exit {
				t.Fatalf("exit %d (%v), want %d", exitCodeOf(err), err, c.exit)
			}
			if c.text != "" && !strings.Contains(err.Error(), c.text) {
				t.Fatalf("reason %q lacks %q", err, c.text)
			}
		})
	}
}
