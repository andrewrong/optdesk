package main

import (
	"bytes"
	"context"
	"errors"
	"github.com/jonboulle/clockwork"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/jobs/calendarrefresh"
	"github.com/andrewrong/optdesk/internal/jobs/computefigures"
	"github.com/andrewrong/optdesk/internal/jobs/earningsrefresh"
	"github.com/andrewrong/optdesk/internal/jobs/optioncandidates"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/sources/fred"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/dividend"
	"github.com/andrewrong/optdesk/internal/store/underlying"
	"github.com/andrewrong/optdesk/internal/symbol"
	"github.com/andrewrong/optdesk/internal/symbol/table"
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
func (f fakeLedger) Present(context.Context, contracts.Day) (map[contracts.ExpectedKey]store.PresentStatus, error) {
	return nil, nil
}
func (f fakeLedger) Acquire(context.Context, string, time.Duration) (store.Lease, error) {
	return store.Lease{}, nil
}
func (f fakeLedger) Release(context.Context, store.Lease) error               { return nil }
func (f fakeLedger) Runs(context.Context, contracts.Day) ([]store.Run, error) { return nil, nil }
func (f fakeSeries) Close() error                                             { f.n.Add(1); return nil }

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

func (fakeJob) ID() string { return "fake" }
func (fakeJob) Plan(context.Context, contracts.Day) ([]contracts.ExpectedKey, string, error) {
	return nil, "", nil
}

type recRunner struct{ day *contracts.Day }

func (r recRunner) Reconcile(_ context.Context, d contracts.Day) error { *r.day = d; return nil }

// runOnceFakes fills the deps of a with fakes; opened counts open calls, closed counts Close calls.
func runOnceFakes(a *app, opened, closed *atomic.Int32, day *contracts.Day) {
	clk := a.deps.newClock
	a.deps = deps{
		openFence:           fenceOpener(&fakeInstFence{}),
		newClock:            clk,
		newFwClock:          func() clockwork.Clock { return clockwork.NewFakeClock() },
		openSchedulerLedger: func(context.Context, string) (store.SchedulerLedger, error) { return &fakeSchedLedger{}, nil },
		newKeyRunner:        func(store.OpsLedger, store.SchedulerLedger, clockwork.Clock) runner.KeyRunner { return nil },
		openUnderlying: func(context.Context, string) (underlying.Store, error) {
			return &fakeUnderlyingStore{rows: []underlying.Row{oneUSRow()}}, nil
		},
		newCalendar:             func(clock.SessionSource) clock.Calendar { return nil },
		newCommandSessionSource: func([]string, string, clock.CommandFunc) clock.SessionSource { return nil },
		newDiffer:               func() clock.SessionDayDiffer { return nil },
		newCalendarRefresh: func(
			clock.SessionSource,
			store.SessionDayStore,
			clock.SessionDayDiffer,
			clock.Clock,
			calendarrefresh.CalendarObserver,
		) (contracts.Job, error) {
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
		newFRED: func(sources.FREDConfig) (sources.Source, error) { return emptySource{}, nil },
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
			computefigures.ComputeStores, compute.Figures, compute.Rules, compute.Units, compute.SeriesOps, compute.Ease, clock.Clock,
		) (contracts.Job, error) {
			return fakeJob{}, nil
		},
		newOptionEod: func(broker.Broker, store.InstrumentResolver, store.OptionEodWriter, clock.Clock, symbol.Translator) (contracts.Job, error) {
			return fakeJob{}, nil
		},
		newOptionEodJP: func(broker.Broker, store.InstrumentResolver, store.OptionEodWriter, clock.Clock) (contracts.Job, error) {
			return fakeJob{}, nil
		},
		newQuoteSnapshot: func(broker.Broker, store.InstrumentResolver, store.QuoteSnapshotWriter, clock.Clock) (contracts.Job, error) {
			return fakeJob{}, nil
		},
		newReconciler: func(
			store.OpsLedger, store.SchedulerLedger, clock.Calendar, clockwork.Clock, []contracts.Job, runner.WindowRule, runner.KeyRunner,
		) (runner.Reconciler, runner.Backfiller) {
			return recRunner{day}, nil
		},
		openEarningsDates: func(context.Context, string) (store.EarningsDateStore, error) {
			opened.Add(1)
			return fakeEvents{n: closed}, nil
		},
		newAlphaVantage: func(sources.EarningsConfig) (sources.EarningsSource, error) { return nil, nil },
		newEarningsRefresh: func(
			map[string]sources.EarningsSource, store.EarningsDateWriter, store.EarningsDateReader, []jobs.WatchEntry, clock.Clock, symbol.Translator,
		) (contracts.Job, error) {
			return fakeJob{}, nil
		},
	}
}

func TestRunOnceDayReachesReconcile(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, fullEnv)
	var out bytes.Buffer
	a.stdout = &out
	var opened, closed atomic.Int32
	var got contracts.Day
	runOnceFakes(a, &opened, &closed, &got)
	if err := runOnce(a, []string{"--day", "2026-03-04"}); err != nil {
		t.Fatal(err)
	}
	if got != "2026-03-04" || !strings.Contains(out.String(), "run-once 2026-03-04 status=") {
		t.Fatalf("day %q, out %q", got, out.String())
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
	a.deps.newFRED = func(c sources.FREDConfig) (sources.Source, error) { cfg = c; return emptySource{}, nil }
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
	a, _ := testApp(t, fullEnv)
	var out bytes.Buffer
	a.stdout = &out
	var opened, closed atomic.Int32
	var got contracts.Day
	runOnceFakes(a, &opened, &closed, &got)
	withCursor(a, &opened, &closed, "2026-03-04", nil)
	if err := runOnce(a, []string{"--day", "2026-03-05"}); err != nil {
		t.Fatal(err)
	}
	if got != "2026-03-05" || !strings.Contains(out.String(), "run-once 2026-03-05 status=") {
		t.Fatalf("reconciled %q, out %q", got, out.String())
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

// listingSource is an earnings source that returns one fixed listing.
type listingSource struct{ l sources.EarningsListing }

func (s listingSource) Fetch(context.Context) ([]sources.EarningsListing, error) {
	return []sources.EarningsListing{s.l}, nil
}

func TestRunOnceEarningsShareClass(t *testing.T) {
	t.Parallel()
	r := newRig(t, fullEnv)
	body := "[[underlying]]\nissuer = \"cik:0001067983\"\nmarket = \"US\"\nsymbol = \"BRK B\"\n"
	r.list.rows = listRowsFromWatch(t, body)
	mem := &memEvents{}
	d := &r.a.deps
	d.symbols = table.New()
	d.newEarningsRefresh = func(
		srcs map[string]sources.EarningsSource, w store.EarningsDateWriter, rd store.EarningsDateReader, es []jobs.WatchEntry, _ clock.Clock, tr symbol.Translator,
	) (contracts.Job, error) {
		return earningsrefresh.NewWithSymbols(srcs, w, rd, es, earningsTestClock{}, tr)
	}
	d.openEarningsDates = func(context.Context, string) (store.EarningsDateStore, error) { return mem, nil }
	d.newAlphaVantage = func(sources.EarningsConfig) (sources.EarningsSource, error) {
		return listingSource{sources.EarningsListing{Symbol: "BRK.B", ReportDate: "2026-11-20"}}, nil
	}
	if err := runOnce(r.a, []string{"--day", "2026-11-04"}); err != nil {
		t.Fatal(err)
	}
	planDay(t, earningsJob(t, r), "2026-11-09")
	earningsJob(t, r).Run(t.Context(), "2026-11-09", nil)
	us := rowsOf(mem, "US")
	if len(us) != 1 || us[0].Status != "listed" || us[0].Symbol != "BRK.B" {
		t.Fatalf("US rows %+v, want one listed row for BRK.B", us)
	}
}

func TestRunOnceOptionEodWatchTranslator(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, fullEnv)
	var opened, closed atomic.Int32
	var day contracts.Day
	runOnceFakes(a, &opened, &closed, &day)
	want := table.New()
	a.deps.symbols = want
	var got symbol.Translator
	a.deps.newOptionEodWatch = func(_ broker.Broker, _ store.InstrumentResolver, _ store.OptionEodWriter,
		_ []jobs.WatchEntry, _ clock.Clock, tr symbol.Translator) (contracts.Job, error) {
		got = tr
		return fakeJob{}, nil
	}
	if err := runOnce(a, []string{"--day", "2026-03-04"}); err != nil {
		t.Fatal(err)
	}
	if got == nil || got != want {
		t.Fatalf("translator %v, want the symbols of the deps", got)
	}
}

// callsAcct is an account store that records its close; only the reads are used.
type callsAcct struct {
	store.AccountStore
	closed *atomic.Int32
}

func (callsAcct) Positions(context.Context, string, string) ([]store.Position, error) {
	return nil, nil
}
func (c callsAcct) Close() error { c.closed.Add(1); return nil }

// callsDiv is a dividend store that records its close; only the read is used.
type callsDiv struct {
	dividend.Store
	closed *atomic.Int32
}

func (callsDiv) Dividends(context.Context, int, contracts.Day, contracts.Day) ([]contracts.DividendExpectation, error) {
	return nil, nil
}
func (c callsDiv) Close() error { c.closed.Add(1); return nil }

// callsRig is a run-once rig with the account, candidate and events lines whose option_eod and option_candidates
// constructors record what they were given.
type callsRig struct {
	*rig
	heldAcct   store.AccountReader
	heldTr     symbol.Translator
	callsDiv   dividend.Reader
	calls      atomic.Int32
	puts       atomic.Int32
	acctDSN    atomic.Value
	divDSN     atomic.Value
	closed     atomic.Int32
	acctOpened atomic.Int32
}

func newCallsRig(t *testing.T, env string) *callsRig {
	t.Helper()
	c := &callsRig{rig: newRig(t, env+candidateEnv+"OT_PG_DSN_ACCOUNT=acct-dsn\n")}
	d := &c.a.deps
	d.openAccount = func(_ context.Context, dsn string) (store.AccountStore, error) {
		c.acctOpened.Add(1)
		c.acctDSN.Store(dsn)
		return callsAcct{closed: &c.closed}, nil
	}
	d.openCandidateRW = func(context.Context, string) (candidateRW, error) { return &fakeCandidateRW{closed: &c.closed}, nil }
	d.newCandidateRule = func() compute.OptionCandidates { return fakeRule{} }
	d.newOptionEodHeld = func(
		_ broker.Broker, _ store.InstrumentResolver, _ store.OptionEodWriter, _ []jobs.WatchEntry, acct store.AccountReader, _ clock.Clock, tr symbol.Translator,
	) (contracts.Job, error) {
		c.heldAcct, c.heldTr = acct, tr
		return namedJob{id: jobOptionEod}, nil
	}
	d.newOptionCandidates = func(optioncandidates.CandidateStores, compute.OptionCandidates, []jobs.WatchEntry, clock.Clock) (contracts.Job, error) {
		c.puts.Add(1)
		return candJobStub{id: jobOptionCandidates}, nil
	}
	d.newOptionCandidatesCalls = func(
		_ optioncandidates.CandidateStores, div dividend.Reader, _ compute.OptionCandidates, _ []jobs.WatchEntry, _ clock.Clock,
	) (contracts.Job, error) {
		c.calls.Add(1)
		c.callsDiv = div
		return candJobStub{id: jobOptionCandidates}, nil
	}
	d.newReference = func(context.Context, string, broker.Options) (broker.ReferenceClient, error) { return &divRef{}, nil }
	d.openDividend = func(_ context.Context, dsn string) (dividend.Store, error) {
		c.divDSN.Store(dsn)
		return callsDiv{closed: &c.closed}, nil
	}
	d.newDividendCapture = func(
		broker.ReferenceClient, store.InstrumentResolver, dividend.Writer, dividend.Reader, []jobs.WatchEntry, clock.Clock,
	) (contracts.Job, error) {
		return namedJob{id: "dividend_capture"}, nil
	}
	return c
}

func TestOptionEodUsesHeldReader(t *testing.T) {
	t.Parallel()
	c := newCallsRig(t, fullEnv)
	if err := runOnce(c.a, []string{"--day", "2026-11-04"}); err != nil {
		t.Fatalf("%v\n%s", err, c.log)
	}
	if c.heldAcct == nil {
		t.Fatal("option_eod was not built with the account reader")
	}
	// The store opens on the first read, with the account line, and closes with the run.
	before := c.acctOpened.Load()
	if _, err := c.heldAcct.Positions(context.Background(), "main", "2026-11-04"); err != nil {
		t.Fatal(err)
	}
	if dsn, _ := c.acctDSN.Load().(string); c.acctOpened.Load() != before+1 || dsn != "acct-dsn" {
		t.Fatalf("opened %d->%d dsn %q", before, c.acctOpened.Load(), dsn)
	}
	// Without the account line option_eod is the job of before.
	n := newCallsRig(t, fullEnv)
	n.a.deps.openAccount = nil
	if err := runOnce(n.a, []string{"--day", "2026-11-04"}); err != nil || n.heldAcct != nil {
		t.Fatalf("err %v held %v", err, n.heldAcct)
	}
}

func TestCallsJobUsesDividendReader(t *testing.T) {
	t.Parallel()
	c := newCallsRig(t, fullEnv)
	if err := runOnce(c.a, []string{"--day", "2026-11-04"}); err != nil {
		t.Fatalf("%v\n%s", err, c.log)
	}
	if c.calls.Load() == 0 || c.puts.Load() != 0 || c.callsDiv == nil {
		t.Fatalf("calls %d puts %d reader %v", c.calls.Load(), c.puts.Load(), c.callsDiv)
	}
	if _, err := c.callsDiv.Dividends(context.Background(), 1, "2026-11-04", "2026-12-04"); err != nil {
		t.Fatal(err)
	}
	if dsn, _ := c.divDSN.Load().(string); dsn != "events-dsn" {
		t.Fatalf("dividend store opened with %q", dsn)
	}
	if !slices.Contains(jobIDs(c.jobs), jobOptionCandidates) {
		t.Fatalf("jobs %v", jobIDs(c.jobs))
	}
}

func TestMissingEventsDSNKeepsPutsJob(t *testing.T) {
	t.Parallel()
	c := newCallsRig(t, strings.Replace(fullEnv, "OT_PG_DSN_EVENTS=events-dsn\n", "", 1))
	if err := runOnce(c.a, []string{"--day", "2026-11-04"}); err != nil {
		t.Fatalf("%v\n%s", err, c.log)
	}
	if c.puts.Load() == 0 || c.calls.Load() != 0 {
		t.Fatalf("puts %d calls %d", c.puts.Load(), c.calls.Load())
	}
	if !slices.Contains(jobIDs(c.jobs), jobOptionCandidates) {
		t.Fatalf("jobs %v", jobIDs(c.jobs))
	}
	if !strings.Contains(c.log.String(), reasonNoEventsDSN) || !strings.Contains(c.log.String(), jobOptionCandidates) {
		t.Fatalf("no named line: %s", c.log)
	}
}

func TestRunOnceOptionEodTranslatorWithAccountLine(t *testing.T) {
	t.Parallel()
	c := newCallsRig(t, fullEnv)
	want := table.New()
	c.a.deps.symbols = want
	if err := runOnce(c.a, []string{"--day", "2026-03-04"}); err != nil {
		t.Fatal(err)
	}
	if c.heldAcct == nil || c.heldTr == nil || c.heldTr != want {
		t.Fatalf("held reader %v translator %v, want the symbols of the deps", c.heldAcct, c.heldTr)
	}
}

func TestRunOnceTakesNoFence(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, fullEnv)
	var opened, closed atomic.Int32
	var got contracts.Day
	runOnceFakes(a, &opened, &closed, &got)
	a.deps.openFence = func(context.Context, string) (store.InstanceFence, error) {
		t.Error("openFence called")
		return nil, errors.New("no fence")
	}
	if err := runOnce(a, []string{"--day", "2026-03-04"}); err != nil {
		t.Fatal(err)
	}
	if got != "2026-03-04" {
		t.Fatalf("reconciled %q", got)
	}
}

func TestRunOnceNamesRunningJob(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)
	a, _ := leaseRig(t, []store.Run{
		{ID: "old", Status: store.RunRunning, LeaseUntil: now.Add(-time.Minute), Key: contracts.ExpectedKey{DatasetID: "other.v1"}},
		{ID: "live", Status: store.RunRunning, LeaseUntil: now.Add(time.Minute), Key: contracts.ExpectedKey{DatasetID: "figures.v1"}},
	})
	err := runOnce(a, []string{"--day", "2026-03-04"})
	want := "run-once refused: job compute_figures is running (dataset figures.v1); retry when it has finished"
	if err == nil || err.Error() != want {
		t.Fatalf("err %v, want %q", err, want)
	}
	if strings.Contains(err.Error(), "secret-ledger-text") {
		t.Fatalf("err holds the ledger text: %v", err)
	}
}

func TestRunOnceLeaseHeldNoRun(t *testing.T) {
	t.Parallel()
	a, _ := leaseRig(t, nil)
	err := runOnce(a, []string{"--day", "2026-03-04"})
	if err == nil || !strings.HasPrefix(err.Error(), "run-once refused:") || !strings.Contains(err.Error(), "runner lease is held by another run") ||
		strings.Contains(err.Error(), "secret-ledger-text") {
		t.Fatalf("err %v", err)
	}
}

func TestRunOnceClosesLedger(t *testing.T) {
	t.Parallel()
	a, slClosed := leaseRig(t, nil)
	if err := runOnce(a, []string{"--day", "2026-03-04"}); err == nil {
		t.Fatal("want a refusal")
	}
	if slClosed.Load() != 1 {
		t.Fatalf("refusal path: scheduler ledger closed %d times", slClosed.Load())
	}
	b, _ := testApp(t, fullEnv)
	var opened, closed atomic.Int32
	var got contracts.Day
	runOnceFakes(b, &opened, &closed, &got)
	var sc atomic.Int32
	b.deps.openSchedulerLedger = func(context.Context, string) (store.SchedulerLedger, error) {
		return &countSchedLedger{closed: &sc}, nil
	}
	if err := runOnce(b, []string{"--day", "2026-03-04"}); err != nil {
		t.Fatal(err)
	}
	if sc.Load() != 1 {
		t.Fatalf("success path: scheduler ledger closed %d times", sc.Load())
	}
}

type dbDownRunner struct{}

// Reconcile fails Retryable as ops/ledger.go wraps a database error, e.g. "ops: set cursor: conn reset".
func (dbDownRunner) Reconcile(context.Context, contracts.Day) error { return secretRetryable{} }

func TestRunOnceDBErrorIsNotALeaseRefusal(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, fullEnv)
	var opened, closed atomic.Int32
	var got contracts.Day
	runOnceFakes(a, &opened, &closed, &got)
	a.deps.newReconciler = func(
		store.OpsLedger, store.SchedulerLedger, clock.Calendar, clockwork.Clock, []contracts.Job, runner.WindowRule, runner.KeyRunner,
	) (runner.Reconciler, runner.Backfiller) {
		return dbDownRunner{}, nil
	}
	err := runOnce(a, []string{"--day", "2026-03-04"})
	if err == nil || strings.Contains(err.Error(), "runner lease is held by another run") {
		t.Fatalf("a ledger failure with the lease free was reported as a held lease: %v", err)
	}
}
