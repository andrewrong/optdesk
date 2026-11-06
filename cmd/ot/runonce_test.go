package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/sources/fred"
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
		newRunner: func(store.OpsLedger, clock.Calendar, clock.Clock, []contracts.Job, runner.WindowRule) runner.Reconciler {
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

func TestRunOnceFREDPathIsObservations(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, fullEnv)
	var opened, closed atomic.Int32
	var got contracts.Day
	runOnceFakes(a, &opened, &closed, &got)
	var cfg sources.FREDConfig
	a.deps.newFRED = func(c sources.FREDConfig) (sources.Source, error) { cfg = c; return nil, nil }
	if err := runOnce(a, nil); err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(cfg.BaseURL)
	if err != nil {
		t.Fatal(err)
	}
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_, _ = w.Write([]byte(`{"observations":[]}`))
	}))
	defer srv.Close()
	src, err := fred.New(sources.FREDConfig{BaseURL: srv.URL + u.Path, APIKey: "k", Clock: cfg.Clock})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = src.Fetch(t.Context(), sources.FetchPlan{SeriesIDs: []string{"DGS10"}, From: "2026-11-02", To: "2026-11-04"})
	if path != "/fred/series/observations" {
		t.Fatalf("FRED request path %q, want /fred/series/observations", path)
	}
}

type windowJob struct{ contracts.Job }

func (windowJob) Window() (string, time.Duration, time.Duration) { return "XNYS", 0, 24 * time.Hour }

func recordRule(a *app, rule *runner.WindowRule, day *contracts.Day) {
	a.deps.newRunner = func(_ store.OpsLedger, _ clock.Calendar, _ clock.Clock, _ []contracts.Job, r runner.WindowRule) runner.Reconciler {
		*rule = r
		return recRunner{day}
	}
}

func TestRunOnceRuleFromJob(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, fullEnv)
	var opened, closed atomic.Int32
	var got contracts.Day
	runOnceFakes(a, &opened, &closed, &got)
	a.deps.newFredSeries = func(sources.Source, store.SeriesWriter, clock.Clock) (contracts.Job, error) {
		return windowJob{}, nil
	}
	var rule runner.WindowRule
	recordRule(a, &rule, &got)
	if err := runOnce(a, []string{"--day", "2026-03-04"}); err != nil {
		t.Fatal(err)
	}
	if want := (runner.WindowRule{CalendarID: "XNYS", CloseOffset: 24 * time.Hour}); rule != want {
		t.Fatalf("rule %+v, want %+v", rule, want)
	}
}

func TestRunOnceZeroRule(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, fullEnv)
	var opened, closed atomic.Int32
	var got contracts.Day
	runOnceFakes(a, &opened, &closed, &got)
	rule := runner.WindowRule{CalendarID: "unset"}
	recordRule(a, &rule, &got)
	if err := runOnce(a, []string{"--day", "2026-03-04"}); err != nil {
		t.Fatal(err)
	}
	if rule != (runner.WindowRule{}) || got != "2026-03-04" {
		t.Fatalf("rule %+v day %q", rule, got)
	}
}
