package main

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
)

const fullEnv = "OT_PG_DSN_RUNNER=run-dsn\nOT_PG_DSN_SERIES_FRED=ser-dsn\nFRED_API_KEY=k\n"

type fakeSD struct {
	store.SessionDayStore
	n *atomic.Int32
}
type fakeLedger struct {
	store.OpsLedger
	n *atomic.Int32
}
type fakeSeries struct {
	store.SeriesStore
	n *atomic.Int32
}

func (f fakeSD) Close() error     { f.n.Add(1); return nil }
func (f fakeLedger) Close() error { f.n.Add(1); return nil }
func (f fakeSeries) Close() error { f.n.Add(1); return nil }

type fakeJob struct{ contracts.Job }

type recRunner struct{ day *contracts.Day }

func (r recRunner) Reconcile(_ context.Context, d contracts.Day) error { *r.day = d; return nil }

// runOnceFakes fills the deps of a with fakes; opened counts open calls, closed counts Close calls.
func runOnceFakes(a *app, opened, closed *atomic.Int32, day *contracts.Day) {
	clk := a.deps.newClock
	a.deps = deps{
		newClock:    clk,
		newCalendar: func(clock.SessionSource) clock.Calendar { return nil },
		openSessionDay: func(context.Context, string) (store.SessionDayStore, error) {
			opened.Add(1)
			return fakeSD{n: closed}, nil
		},
		openLedger: func(context.Context, string) (store.OpsLedger, error) {
			opened.Add(1)
			return fakeLedger{n: closed}, nil
		},
		openSeries: func(context.Context, string) (store.SeriesStore, error) {
			opened.Add(1)
			return fakeSeries{n: closed}, nil
		},
		newFRED: func(sources.FREDConfig) (sources.Source, error) { return nil, nil },
		newFredSeries: func(sources.Source, store.SeriesWriter, clock.Clock) (contracts.Job, error) {
			return fakeJob{}, nil
		},
		newRunner: func(store.OpsLedger, clock.Calendar, clock.Clock, []contracts.Job) runner.Reconciler {
			return recRunner{day}
		},
	}
}

func TestRunOnceDayReachesReconcile(t *testing.T) {
	t.Parallel()
	a, errb := testApp(t, fullEnv)
	var opened, closed atomic.Int32
	var got contracts.Day
	runOnceFakes(a, &opened, &closed, &got)
	if err := runOnce(a, []string{"--day", "2026-03-04"}); err != nil {
		t.Fatal(err)
	}
	if got != "2026-03-04" || !strings.Contains(errb.String(), "2026-03-04") {
		t.Fatalf("day %q, log %q", got, errb.String())
	}
	got = ""
	if err := runOnce(a, nil); err != nil || got != "2026-11-05" {
		t.Fatalf("default day %q err %v", got, err)
	}
}

func TestRunOnceDemoRefused(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, fullEnv)
	a.profile.Name = "demo"
	var opened, closed atomic.Int32
	var got contracts.Day
	runOnceFakes(a, &opened, &closed, &got)
	if err := runOnce(a, nil); err == nil || opened.Load() != 0 {
		t.Fatalf("err %v opened %d", err, opened.Load())
	}
}

func TestRunOnceMissingKey(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_PG_DSN_RUNNER=r\nOT_PG_DSN_SERIES_FRED=s\n")
	var opened, closed atomic.Int32
	var got contracts.Day
	runOnceFakes(a, &opened, &closed, &got)
	err := runOnce(a, nil)
	if err == nil || !strings.Contains(err.Error(), "FRED_API_KEY") || opened.Load() != 0 {
		t.Fatalf("err %v opened %d", err, opened.Load())
	}
}

func TestRunOnceClosesStores(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, fullEnv)
	var opened, closed atomic.Int32
	var got contracts.Day
	runOnceFakes(a, &opened, &closed, &got)
	if err := runOnce(a, nil); err != nil || opened.Load() != 3 || closed.Load() != 3 {
		t.Fatalf("err %v opened %d closed %d", err, opened.Load(), closed.Load())
	}
	// A failure after two stores are open closes them too.
	opened.Store(0)
	closed.Store(0)
	a.deps.openSeries = func(context.Context, string) (store.SeriesStore, error) { return nil, errors.New("down") }
	if err := runOnce(a, nil); err == nil || closed.Load() != 2 {
		t.Fatalf("err %v closed %d", err, closed.Load())
	}
}

func TestRunOnceSentinelDSN(t *testing.T) {
	t.Parallel()
	const sentinel = "postgres://sentinel-user:sentinel-pw@h/db"
	a, errb := testApp(t, "OT_PG_DSN_RUNNER="+sentinel+"\nOT_PG_DSN_SERIES_FRED=s\nFRED_API_KEY=sentinel-key\n")
	var opened, closed atomic.Int32
	var got contracts.Day
	runOnceFakes(a, &opened, &closed, &got)
	a.deps.openLedger = func(_ context.Context, dsn string) (store.OpsLedger, error) {
		return nil, errors.New("dial " + dsn)
	}
	err := runOnce(a, nil)
	if err == nil {
		t.Fatal("no error")
	}
	if strings.Contains(err.Error(), "sentinel") || strings.Contains(errb.String(), "sentinel") {
		t.Fatalf("leak: %v / %q", err, errb.String())
	}
	if !strings.Contains(err.Error(), store.DSNEnvRunner) {
		t.Fatalf("error does not name the variable: %v", err)
	}
}
