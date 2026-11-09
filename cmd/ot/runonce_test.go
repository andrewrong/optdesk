package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/sources/fred"
	"github.com/andrewrong/optdesk/internal/store"
)

const fullEnv = "OT_PG_DSN_RUNNER=run-dsn\nOT_PG_DSN_SERIES_FRED=ser-dsn\nFRED_API_KEY=k\n" +
	"OT_PG_DSN_OPTION_EOD=eod-dsn\nOT_PG_DSN_QUOTE_SNAPSHOT=snap-dsn\nOT_PG_DSN_MASTER=master-dsn\n" +
	"OT_PG_DSN_PAGES=pages-dsn\nOT_PG_DSN_READINGS=readings-dsn\nOT_PG_DSN_LOADER=loader-dsn\n" + eventsEnv

// eventsEnv holds the two lines of earnings_refresh; the key value starts with a character no real key does.
const alphaTestKey = "-test-alpha"

const eventsEnv = "OT_PG_DSN_EVENTS=events-dsn\n" + "ALPHA_VANTAGE_API_KEY=" + alphaTestKey + "\n"

type fakeEvents struct {
	store.EarningsDateStore
	n *atomic.Int32
}

func (f fakeEvents) Close() error { f.n.Add(1); return nil }

type fakeSD struct {
	store.SessionDayStore
	n *atomic.Int32
}
type fakeLedger struct {
	store.OpsLedger
	n         *atomic.Int32
	cursorDay contracts.Day
	cursorErr error
}
type fakeSeries struct {
	store.SeriesStore
	n *atomic.Int32
}

func (f fakeSD) Close() error     { f.n.Add(1); return nil }
func (f fakeLedger) Close() error { f.n.Add(1); return nil }
func (f fakeLedger) Cursor(context.Context) (store.Cursor, error) {
	return store.Cursor{Day: f.cursorDay}, f.cursorErr
}
func (f fakeSeries) Close() error { f.n.Add(1); return nil }

type fakeInstr struct {
	store.InstrumentStore
	n *atomic.Int32
}
type fakeEod struct {
	store.OptionEodStore
	n *atomic.Int32
}
type fakeSnap struct {
	store.QuoteSnapshotStore
	n *atomic.Int32
}
type fakeCat struct {
	store.OptionEodCatalogStore
	n *atomic.Int32
}
type fakeTerms struct {
	store.OptionTermsStore
	n *atomic.Int32
}
type fakeInstName struct {
	store.InstrumentNameStore
	n *atomic.Int32
}
type fakeReading struct {
	store.ReadingStore
	n *atomic.Int32
}
type fakeBrk struct {
	broker.Broker
	n *atomic.Int32
}

func (f fakeInstr) Close() error    { f.n.Add(1); return nil }
func (f fakeEod) Close() error      { f.n.Add(1); return nil }
func (f fakeSnap) Close() error     { f.n.Add(1); return nil }
func (f fakeBrk) Close() error      { f.n.Add(1); return nil }
func (f fakeCat) Close() error      { f.n.Add(1); return nil }
func (f fakeTerms) Close() error    { f.n.Add(1); return nil }
func (f fakeInstName) Close() error { f.n.Add(1); return nil }
func (f fakeReading) Close() error  { f.n.Add(1); return nil }

type fakeJob struct{ contracts.Job }

type recRunner struct{ day *contracts.Day }

func (r recRunner) Reconcile(_ context.Context, d contracts.Day) error { *r.day = d; return nil }

// runOnceFakes fills the deps of a with fakes; opened counts open calls, closed counts Close calls.
func runOnceFakes(a *app, opened, closed *atomic.Int32, day *contracts.Day) {
	clk := a.deps.newClock
	a.deps = deps{
		openFence:               fenceOpener(&fakeInstFence{}),
		newClock:                clk,
		newCalendar:             func(clock.SessionSource) clock.Calendar { return nil },
		newCommandSessionSource: func([]string, string, clock.CommandFunc) clock.SessionSource { return nil },
		newDiffer:               func() clock.SessionDayDiffer { return nil },
		newCalendarRefresh: func(clock.SessionSource, store.SessionDayStore, clock.SessionDayDiffer, clock.Clock, jobs.CalendarObserver) (contracts.Job, error) {
			return fakeJob{}, nil
		},
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
		dialBroker: func(context.Context, string, broker.Options) (broker.Broker, error) {
			opened.Add(1)
			return fakeBrk{n: closed}, nil
		},
		openInstrument: func(context.Context, string) (store.InstrumentStore, error) {
			opened.Add(1)
			return fakeInstr{n: closed}, nil
		},
		openOptionEod: func(context.Context, string) (store.OptionEodStore, error) {
			opened.Add(1)
			return fakeEod{n: closed}, nil
		},
		openQuoteSnapshot: func(context.Context, string) (store.QuoteSnapshotStore, error) {
			opened.Add(1)
			return fakeSnap{n: closed}, nil
		},
		openOptionCatalog: func(context.Context, string) (store.OptionEodCatalogStore, error) {
			opened.Add(1)
			return fakeCat{n: closed}, nil
		},
		openOptionTerms: func(context.Context, string) (store.OptionTermsStore, error) {
			opened.Add(1)
			return fakeTerms{n: closed}, nil
		},
		openInstrumentName: func(context.Context, string) (store.InstrumentNameStore, error) {
			opened.Add(1)
			return fakeInstName{n: closed}, nil
		},
		openReading: func(context.Context, string) (store.ReadingStore, error) {
			opened.Add(1)
			return fakeReading{n: closed}, nil
		},
		newComputeFigures: func(
			jobs.ComputeStores, compute.Figures, compute.Rules, compute.Units, compute.SeriesOps, compute.Ease, clock.Clock,
		) (contracts.Job, error) {
			return fakeJob{}, nil
		},
		newOptionEod: func(broker.Broker, store.InstrumentResolver, store.OptionEodWriter, clock.Clock) (contracts.Job, error) {
			return fakeJob{}, nil
		},
		newQuoteSnapshot: func(broker.Broker, store.InstrumentResolver, store.QuoteSnapshotWriter, clock.Clock) (contracts.Job, error) {
			return fakeJob{}, nil
		},
		newRunner: func(store.OpsLedger, clock.Calendar, clock.Clock, []contracts.Job, runner.WindowRule) runner.Reconciler {
			return recRunner{day}
		},
		openEarningsDates: func(context.Context, string) (store.EarningsDateStore, error) {
			opened.Add(1)
			return fakeEvents{n: closed}, nil
		},
		newAlphaVantage: func(sources.EarningsConfig) (sources.EarningsSource, error) { return nil, nil },
		newEarningsRefresh: func(
			map[string]sources.EarningsSource, store.EarningsDateWriter, store.EarningsDateReader, []jobs.WatchEntry, clock.Clock,
		) (contracts.Job, error) {
			return fakeJob{}, nil
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
	a, _ := testApp(t, "OT_PG_DSN_RUNNER=r\nOT_PG_DSN_SERIES_FRED=s\nOT_PG_DSN_OPTION_EOD=e\nOT_PG_DSN_QUOTE_SNAPSHOT=q\n"+
		"OT_PG_DSN_MASTER=m\nOT_PG_DSN_PAGES=p\nOT_PG_DSN_READINGS=rd\nOT_PG_DSN_LOADER=ld\n")
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
	if err := runOnce(a, nil); err != nil || opened.Load() != 15 || closed.Load() != 15 {
		t.Fatalf("err %v opened %d closed %d", err, opened.Load(), closed.Load())
	}
	// A failure after two stores are open closes them too.
	opened.Store(0)
	closed.Store(0)
	a.deps.openSeries = func(context.Context, string) (store.SeriesStore, error) { return nil, errors.New("down") }
	if err := runOnce(a, nil); err == nil || closed.Load() != 3 {
		t.Fatalf("err %v closed %d", err, closed.Load())
	}
}

func TestRunOnceSentinelDSN(t *testing.T) {
	t.Parallel()
	const sentinel = "postgres://sentinel-user:sentinel-pw@h/db"
	a, errb := testApp(t, "OT_PG_DSN_RUNNER="+sentinel+"\nOT_PG_DSN_SERIES_FRED=s\nFRED_API_KEY=sentinel-key\n"+
		"OT_PG_DSN_OPTION_EOD=e\nOT_PG_DSN_QUOTE_SNAPSHOT=q\nOT_PG_DSN_MASTER=m\nOT_PG_DSN_PAGES=p\nOT_PG_DSN_READINGS=rd\nOT_PG_DSN_LOADER=ld\n")
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

// withCursor makes the ledger the app opens report cursor day and error.
func withCursor(a *app, opened, closed *atomic.Int32, day contracts.Day, err error) {
	a.deps.openLedger = func(context.Context, string) (store.OpsLedger, error) {
		opened.Add(1)
		return fakeLedger{n: closed, cursorDay: day, cursorErr: err}, nil
	}
}

func TestRunOnceDayBehindCursor(t *testing.T) {
	t.Parallel()
	for _, day := range []string{"2026-03-03", "2026-03-04"} {
		a, errb := testApp(t, fullEnv)
		var opened, closed atomic.Int32
		var got contracts.Day
		runOnceFakes(a, &opened, &closed, &got)
		withCursor(a, &opened, &closed, "2026-03-04", nil)
		err := runOnce(a, []string{"--day", day})
		if err == nil || !strings.Contains(err.Error(), day) || !strings.Contains(err.Error(), "2026-03-04") ||
			!strings.Contains(err.Error(), "use ot backfill --day "+day) {
			t.Fatalf("day %s: err %v", day, err)
		}
		if got != "" || strings.Contains(errb.String(), "status=ok") {
			t.Fatalf("day %s: reconciled %q, log %q", day, got, errb.String())
		}
		if opened.Load() != closed.Load() || opened.Load() == 0 {
			t.Fatalf("day %s: opened %d closed %d", day, opened.Load(), closed.Load())
		}
	}
	a, errb := testApp(t, fullEnv)
	var opened, closed atomic.Int32
	var got contracts.Day
	runOnceFakes(a, &opened, &closed, &got)
	withCursor(a, &opened, &closed, "2026-03-04", nil)
	if err := runOnce(a, []string{"--day", "2026-03-05"}); err != nil {
		t.Fatal(err)
	}
	if got != "2026-03-05" || !strings.Contains(errb.String(), "status=ok") {
		t.Fatalf("reconciled %q, log %q", got, errb.String())
	}
}

func TestRunOnceCursorEmptyAndError(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, fullEnv)
	var opened, closed atomic.Int32
	var got contracts.Day
	runOnceFakes(a, &opened, &closed, &got)
	withCursor(a, &opened, &closed, "", nil)
	for _, day := range []string{"2000-01-01", "2026-03-04"} {
		got = ""
		if err := runOnce(a, []string{"--day", day}); err != nil || got != contracts.Day(day) {
			t.Fatalf("day %s: reconciled %q err %v", day, got, err)
		}
	}
	got = ""
	opened.Store(0)
	closed.Store(0)
	withCursor(a, &opened, &closed, "", errors.New("secret-ledger-text"))
	err := runOnce(a, []string{"--day", "2026-03-04"})
	if err == nil || !strings.Contains(err.Error(), "cursor") || strings.Contains(err.Error(), "secret-ledger-text") {
		t.Fatalf("err %v", err)
	}
	if got != "" || opened.Load() != closed.Load() || opened.Load() == 0 {
		t.Fatalf("reconciled %q opened %d closed %d", got, opened.Load(), closed.Load())
	}
}

func TestRunOnceRefusedWhileSchedulerLive(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, fullEnv)
	var opened, closed atomic.Int32
	var got contracts.Day
	runOnceFakes(a, &opened, &closed, &got)
	inst := &fakeInstFence{held: true}
	a.deps.openFence = fenceOpener(inst)
	err := runOnce(a, []string{"--day", "2026-03-04"})
	if err == nil || !strings.Contains(err.Error(), "ot scheduler status") {
		t.Fatalf("err %v", err)
	}
	if got != "" || opened.Load() != 0 || inst.count() != 1 {
		t.Fatalf("reconciled %q, opened %d, tries %d", got, opened.Load(), inst.count())
	}
}

// fakeFence is a held fence; Close counts.
type fakeFence struct {
	lost   chan struct{}
	closed atomic.Int32
}

func (f *fakeFence) Lost() <-chan struct{} { return f.lost }
func (f *fakeFence) Close() error          { f.closed.Add(1); return nil }

// fakeInstFence is an InstanceFence: held makes every TryLock fail (retryable), else it hands out fence.
type fakeInstFence struct {
	mu    sync.Mutex
	tries int
	held  bool
	fence *fakeFence
	build string
}

func (f *fakeInstFence) TryLock(_ context.Context, build string) (store.Fence, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tries++
	f.build = build
	if f.held {
		return nil, errors.New("held")
	}
	if f.fence == nil {
		f.fence = &fakeFence{lost: make(chan struct{})}
	}
	return f.fence, nil
}

func (f *fakeInstFence) count() int { f.mu.Lock(); defer f.mu.Unlock(); return f.tries }

func fenceOpener(f store.InstanceFence) store.OpenFenceFunc {
	return func(context.Context, string) (store.InstanceFence, error) { return f, nil }
}

func TestRunOnceNeedsReadingsDSN(t *testing.T) {
	t.Parallel()
	const sentinel = "sentinel-readings-secret"
	for name, env := range map[string]string{
		"absent": strings.Replace(fullEnv, "OT_PG_DSN_READINGS=readings-dsn\n", "", 1),
		"empty":  strings.Replace(fullEnv, "OT_PG_DSN_READINGS=readings-dsn\n", "OT_PG_DSN_READINGS=\n", 1),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			a, _ := testApp(t, env+"OT_PG_DSN_X="+sentinel+"\n")
			var opened, closed atomic.Int32
			var got contracts.Day
			runOnceFakes(a, &opened, &closed, &got)
			err := runOnce(a, nil)
			if err == nil || !strings.Contains(err.Error(), store.DSNEnvReadings) || strings.Contains(err.Error(), sentinel) || opened.Load() != 0 {
				t.Fatalf("err %v opened %d", err, opened.Load())
			}
		})
	}
	// The value of the line never reaches the error when its open fails.
	a, _ := testApp(t, fullEnv)
	var opened, closed atomic.Int32
	var got contracts.Day
	runOnceFakes(a, &opened, &closed, &got)
	a.deps.openReading = func(_ context.Context, dsn string) (store.ReadingStore, error) { return nil, errors.New("dial " + dsn) }
	err := runOnce(a, nil)
	if err == nil || !strings.Contains(err.Error(), store.DSNEnvReadings) || strings.Contains(err.Error(), "readings-dsn") {
		t.Fatalf("err %v", err)
	}
}
