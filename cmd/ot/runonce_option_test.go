package main

import (
	"bytes"
	"context"
	"errors"
	"github.com/jonboulle/clockwork"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/symbol"
)

// events is a goroutine-safe list of the steps a rig saw.
type events struct {
	mu sync.Mutex
	l  []string
}

func (e *events) add(s string) { e.mu.Lock(); e.l = append(e.l, s); e.mu.Unlock() }
func (e *events) list() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return slices.Clone(e.l)
}

// evc closes by recording "close <name>".
type evc struct {
	name string
	ev   *events
}

func (c evc) close() error { c.ev.add("close " + c.name); return nil }

type evSD struct {
	store.SessionDayStore
	c evc
}
type evLedger struct {
	store.OpsLedger
	c evc
	d contracts.Day
}
type evSeries struct {
	store.SeriesStore
	c evc
}
type evEod struct {
	store.OptionEodStore
	c evc
}
type evSnap struct {
	store.QuoteSnapshotStore
	c evc
}
type evInstr struct {
	store.InstrumentStore
	c evc
}
type evCat struct {
	store.OptionEodCatalogStore
	c evc
}
type evTerms struct {
	store.OptionTermsStore
	c evc
}
type evNames struct {
	store.InstrumentNameStore
	c evc
}
type evEvents struct {
	store.EarningsDateStore
	c evc
}
type evReading struct {
	store.ReadingStore
	c evc
}
type evBroker struct {
	broker.Broker
	c evc
}

func (f evSD) Close() error     { return f.c.close() }
func (f evLedger) Close() error { return f.c.close() }
func (f evLedger) Cursor(context.Context) (store.Cursor, error) {
	return store.Cursor{Day: f.d}, nil
}
func (f evSeries) Close() error  { return f.c.close() }
func (f evEod) Close() error     { return f.c.close() }
func (f evSnap) Close() error    { return f.c.close() }
func (f evInstr) Close() error   { return f.c.close() }
func (f evBroker) Close() error  { return f.c.close() }
func (f evCat) Close() error     { return f.c.close() }
func (f evTerms) Close() error   { return f.c.close() }
func (f evNames) Close() error   { return f.c.close() }
func (f evReading) Close() error { return f.c.close() }

func (f evEvents) Close() error { return f.c.close() }

// brokerJob is a job whose Run asks the broker it was built with, as the option jobs do.
type brokerJob struct {
	contracts.Job
	b broker.Broker
}

func (j brokerJob) Run(ctx context.Context, _ contracts.Day, _ []contracts.ExpectedKey) contracts.JobResult {
	_, err := j.b.Snapshot(ctx, broker.KindSnapshot, nil)
	var te contracts.TypedError
	if err != nil {
		te, _ = err.(contracts.TypedError)
	}
	return contracts.JobResult{Err: te}
}

type namedJob struct {
	contracts.Job
	id string
}

func (j namedJob) ID() string { return j.id }
func (namedJob) Run(context.Context, contracts.Day, []contracts.ExpectedKey) contracts.JobResult {
	return contracts.JobResult{}
}

// rig fills every constructor of an app with a fake that records into ev.
type rig struct {
	a          *app
	ev         *events
	log        *bytes.Buffer
	fail       map[string]error // step name -> error; "reconcile" fails Reconcile
	dsn        map[string]string
	socket     string
	jobs       []contracts.Job
	rule       runner.WindowRule
	backRule   runner.WindowRule
	instr      store.InstrumentStore
	eodRes     store.InstrumentResolver
	snapRes    store.InstrumentResolver
	eod        store.OptionEodStore
	snap       store.QuoteSnapshotStore
	eodW       store.OptionEodWriter
	snapW      store.QuoteSnapshotWriter
	cursor     contracts.Day
	reconciled int
	compute    jobs.ComputeStores
	loader     store.SessionDayStore // the store opened with the loader line
	calStore   store.SessionDayStore // the store handed to calendar_refresh
	argv       []string
	observer   jobs.CalendarObserver
	// what earnings_refresh was built with
	earnSrcs  map[string]sources.EarningsSource
	earnStore store.EarningsDateWriter
	earnWatch []jobs.WatchEntry
	avCfg     sources.EarningsConfig
}

// loaderDSN is the value of the loader line of the rig's .env.
func (r *rig) loaderDSN() string {
	env, _ := r.a.envFile()
	return env[store.DSNEnvLoader]
}

// pagesDSN is the value of the pages line of the rig's .env; the compute stores open with it.
func (r *rig) pagesDSN() string {
	env, _ := r.a.envFile()
	return env[store.DSNEnvPages]
}

type recReconciler struct{ r *rig }

func (c recReconciler) Reconcile(context.Context, contracts.Day) error {
	c.r.reconciled++
	c.r.ev.add("reconcile")
	return c.r.fail["reconcile"]
}

func newRig(t *testing.T, env string) *rig {
	t.Helper()
	a, log := testApp(t, env)
	r := &rig{a: a, ev: &events{}, log: log, fail: map[string]error{}, dsn: map[string]string{}}
	step := func(name, dsn string) error {
		r.ev.add("open " + name)
		r.dsn[name] = dsn
		return r.fail[name]
	}
	d := &r.a.deps
	d.openFence = fenceOpener(&fakeInstFence{})
	d.newClock = func() clock.Clock { return fakeClock{time.Date(2026, 11, 6, 2, 0, 0, 0, time.UTC)} }
	d.newCalendar = func(clock.SessionSource) clock.Calendar { return nil }
	d.openSessionDay = func(_ context.Context, dsn string) (store.SessionDayStore, error) {
		name := "sd"
		if dsn == r.loaderDSN() {
			name = "sd-loader"
		}
		if err := step(name, dsn); err != nil {
			return nil, err
		}
		e := evSD{c: evc{name, r.ev}}
		if name == "sd-loader" {
			r.loader = e
		}
		return e, nil
	}
	d.newCommandSessionSource = func(argv []string, _ string, _ clock.CommandFunc) clock.SessionSource {
		r.argv = argv
		return nil
	}
	d.newDiffer = func() clock.SessionDayDiffer { return nil }
	d.newCalendarRefresh = func(
		_ clock.SessionSource, sd store.SessionDayStore, _ clock.SessionDayDiffer, _ clock.Clock, obs jobs.CalendarObserver,
	) (contracts.Job, error) {
		r.calStore, r.observer = sd, obs
		return namedJob{id: "calendar_refresh"}, r.fail["caljob"]
	}
	d.openLedger = func(_ context.Context, dsn string) (store.OpsLedger, error) {
		if err := step("ledger", dsn); err != nil {
			return nil, err
		}
		return evLedger{c: evc{"ledger", r.ev}, d: r.cursor}, nil
	}
	d.openSeries = func(_ context.Context, dsn string) (store.SeriesStore, error) {
		name := "series"
		if dsn == r.pagesDSN() {
			name = "c-series"
		}
		if err := step(name, dsn); err != nil {
			return nil, err
		}
		return evSeries{c: evc{name, r.ev}}, nil
	}
	d.openOptionEod = func(_ context.Context, dsn string) (store.OptionEodStore, error) {
		name := "eod"
		if dsn == r.pagesDSN() {
			name = "c-eod"
		}
		if err := step(name, dsn); err != nil {
			return nil, err
		}
		e := evEod{c: evc{name, r.ev}}
		if name == "eod" {
			r.eod = e
		}
		return e, nil
	}
	d.openQuoteSnapshot = func(_ context.Context, dsn string) (store.QuoteSnapshotStore, error) {
		if err := step("snap", dsn); err != nil {
			return nil, err
		}
		r.snap = evSnap{c: evc{"snap", r.ev}}
		return r.snap, nil
	}
	d.openInstrument = func(_ context.Context, dsn string) (store.InstrumentStore, error) {
		if err := step("instr", dsn); err != nil {
			return nil, err
		}
		r.instr = evInstr{c: evc{"instr", r.ev}}
		return r.instr, nil
	}
	d.openOptionCatalog = func(_ context.Context, dsn string) (store.OptionEodCatalogStore, error) {
		if err := step("c-catalog", dsn); err != nil {
			return nil, err
		}
		return evCat{c: evc{"c-catalog", r.ev}}, nil
	}
	d.openOptionTerms = func(_ context.Context, dsn string) (store.OptionTermsStore, error) {
		if err := step("c-terms", dsn); err != nil {
			return nil, err
		}
		return evTerms{c: evc{"c-terms", r.ev}}, nil
	}
	d.openInstrumentName = func(_ context.Context, dsn string) (store.InstrumentNameStore, error) {
		if err := step("c-names", dsn); err != nil {
			return nil, err
		}
		return evNames{c: evc{"c-names", r.ev}}, nil
	}
	d.openReading = func(_ context.Context, dsn string) (store.ReadingStore, error) {
		if err := step("readings", dsn); err != nil {
			return nil, err
		}
		return evReading{c: evc{"readings", r.ev}}, nil
	}
	d.newComputeFigures = func(
		cs jobs.ComputeStores, _ compute.Figures, _ compute.Rules, _ compute.Units, _ compute.SeriesOps, _ compute.Ease, _ clock.Clock,
	) (contracts.Job, error) {
		r.compute = cs
		return namedJob{id: "compute_figures"}, r.fail["figjob"]
	}
	d.openEarningsDates = func(_ context.Context, dsn string) (store.EarningsDateStore, error) {
		if err := step("events", dsn); err != nil {
			return nil, err
		}
		return evEvents{c: evc{"events", r.ev}}, nil
	}
	d.newAlphaVantage = func(cfg sources.EarningsConfig) (sources.EarningsSource, error) {
		r.avCfg = cfg
		return nil, r.fail["avsrc"]
	}
	d.newEarningsRefresh = func(
		srcs map[string]sources.EarningsSource, w store.EarningsDateWriter, _ store.EarningsDateReader, entries []jobs.WatchEntry, _ clock.Clock, _ symbol.Translator,
	) (contracts.Job, error) {
		r.earnSrcs, r.earnStore, r.earnWatch = srcs, w, entries
		return namedJob{id: "earnings_refresh"}, r.fail["earnjob"]
	}
	d.dialBroker = func(_ context.Context, path string, _ broker.Options) (broker.Broker, error) {
		r.ev.add("open broker")
		r.socket = path
		if err := r.fail["broker"]; err != nil {
			return nil, err
		}
		return evBroker{c: evc{"broker", r.ev}}, nil
	}
	d.newFRED = func(sources.FREDConfig) (sources.Source, error) { return nil, nil }
	d.newFredSeries = func(sources.Source, store.SeriesWriter, clock.Clock) (contracts.Job, error) {
		return namedJob{id: "fred_series"}, r.fail["fredjob"]
	}
	d.newOptionEod = func(b broker.Broker, res store.InstrumentResolver, w store.OptionEodWriter, _ clock.Clock, _ symbol.Translator) (contracts.Job, error) {
		r.eodRes, r.eodW = res, w
		return brokerJob{Job: namedJob{id: "option_eod"}, b: b}, r.fail["eodjob"]
	}
	d.newOptionEodJP = func(b broker.Broker, _ store.InstrumentResolver, _ store.OptionEodWriter, _ clock.Clock) (contracts.Job, error) {
		return brokerJob{Job: namedJob{id: "option_eod_jp"}, b: b}, r.fail["eodjpjob"]
	}
	d.newOptionEodJPWatch = nil
	d.newQuoteSnapshot = func(b broker.Broker, res store.InstrumentResolver, w store.QuoteSnapshotWriter, _ clock.Clock) (contracts.Job, error) {
		r.snapRes, r.snapW = res, w
		return brokerJob{Job: namedJob{id: "quote_snapshot"}, b: b}, r.fail["snapjob"]
	}
	d.newRunner = func(_ store.OpsLedger, _ clock.Calendar, _ clock.Clock, js []contracts.Job, rule runner.WindowRule) runner.Reconciler {
		r.jobs, r.rule = js, rule
		return recReconciler{r}
	}
	d.newFwClock = func() clockwork.Clock { return clockwork.NewFakeClock() }
	d.openSchedulerLedger = func(context.Context, string) (store.SchedulerLedger, error) { return &fakeSchedLedger{}, nil }
	d.newKeyRunner = func(store.OpsLedger, store.SchedulerLedger, clockwork.Clock) runner.KeyRunner { return nil }
	d.newKeyGapSyncer = func(store.OpsLedger, store.SchedulerLedger, clockwork.Clock) runner.KeyGapSyncer {
		return &fakeKeySyncer{}
	}
	d.newReconciler = func(
		_ store.OpsLedger, _ store.SchedulerLedger, _ clock.Calendar, _ clockwork.Clock, js []contracts.Job, rule runner.WindowRule, _ runner.KeyRunner,
	) (runner.Reconciler, runner.Backfiller) {
		r.jobs, r.backRule = js, rule
		return nil, &fakeBackfiller{}
	}
	return r
}

func TestRunOnceWiresOptionJobs(t *testing.T) {
	t.Parallel()
	env := "OT_PG_DSN_RUNNER=run-dsn\nOT_PG_DSN_SERIES_FRED=ser-dsn\nFRED_API_KEY=k\n" +
		"OT_PG_DSN_OPTION_EOD=eod-dsn\nOT_PG_DSN_QUOTE_SNAPSHOT=snap-dsn\nOT_PG_DSN_MASTER=master-dsn\n" +
		"OT_PG_DSN_PAGES=pages-dsn\nOT_PG_DSN_READINGS=readings-dsn\nOT_PG_DSN_LOADER=loader-dsn\n" + eventsEnv
	r := newRig(t, env)
	if err := runOnce(r.a, []string{"--day", "2026-11-04"}); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"sd": "run-dsn", "ledger": "run-dsn", "series": "ser-dsn", "eod": "eod-dsn", "snap": "snap-dsn", "instr": "master-dsn",
		"c-catalog": "pages-dsn", "c-eod": "pages-dsn", "c-terms": "pages-dsn", "c-names": "pages-dsn", "c-series": "pages-dsn", "readings": "readings-dsn"}
	for k, v := range want {
		if r.dsn[k] != v {
			t.Errorf("store %s opened with %q, want %q", k, r.dsn[k], v)
		}
	}
	if r.eodRes != store.InstrumentResolver(r.instr) || r.snapRes != store.InstrumentResolver(r.instr) {
		t.Error("the instrument store is not the resolver of both jobs")
	}
	if r.eodW != store.OptionEodWriter(r.eod) || r.snapW != store.QuoteSnapshotWriter(r.snap) {
		t.Error("a writer is not the store of its own job")
	}
	if want := filepath.Join(r.a.getenv("OT_HOME"), "run", "ibkr.sock"); r.socket != want {
		t.Errorf("dialled %q, want %q", r.socket, want)
	}
	var ids []string
	for _, j := range r.jobs {
		ids = append(ids, j.ID())
	}
	if !slices.Equal(ids, []string{"fred_series", "option_eod", "quote_snapshot", "compute_figures", "calendar_refresh", "earnings_refresh"}) {
		t.Errorf("jobs %v", ids)
	}
}

func TestRunOnceRule(t *testing.T) {
	t.Parallel()
	r := newRig(t, fullEnv)
	r.rule = runner.WindowRule{CalendarID: "unset"}
	r.backRule = runner.WindowRule{CalendarID: "unset"}
	if err := runOnce(r.a, []string{"--day", "2026-11-04"}); err != nil {
		t.Fatal(err)
	}
	if err := backfill(r.a, []string{"--day", "2026-11-04"}); err != nil {
		t.Fatal(err)
	}
	if r.rule != (runner.WindowRule{}) || r.backRule != (runner.WindowRule{}) || len(r.jobs) != 6 {
		t.Fatalf("rules %+v %+v, jobs %d", r.rule, r.backRule, len(r.jobs))
	}
}

func TestRunOnceCloses(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	for _, tc := range []struct {
		name string
		fail string
		env  string
	}{
		{"success", "", fullEnv},
		{"missing line", "", "OT_PG_DSN_RUNNER=r\n"},
		{"sd", "sd", fullEnv}, {"ledger", "ledger", fullEnv}, {"series", "series", fullEnv},
		{"eod", "eod", fullEnv}, {"snap", "snap", fullEnv}, {"instr", "instr", fullEnv},
		{"dial", "broker", fullEnv}, {"fred job", "fredjob", fullEnv}, {"eod job", "eodjob", fullEnv},
		{"snap job", "snapjob", fullEnv}, {"compute job", "figjob", fullEnv}, {"calendar job", "caljob", fullEnv},
		{"c-catalog", "c-catalog", fullEnv}, {"c-eod", "c-eod", fullEnv}, {"c-terms", "c-terms", fullEnv},
		{"c-names", "c-names", fullEnv}, {"c-series", "c-series", fullEnv}, {"readings", "readings", fullEnv},
		{"sd-loader", "sd-loader", fullEnv}, {"reconcile", "reconcile", fullEnv},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newRig(t, tc.env)
			if tc.fail != "" {
				r.fail[tc.fail] = boom
			}
			_ = runOnce(r.a, nil)
			var opened, closed []string
			for _, e := range r.ev.list() {
				switch {
				case strings.HasPrefix(e, "open "):
					n := strings.TrimPrefix(e, "open ")
					if r.fail[n] == nil { // an open that failed holds nothing to close
						opened = append(opened, n)
					}
				case strings.HasPrefix(e, "close "):
					closed = append(closed, strings.TrimPrefix(e, "close "))
				}
			}
			slices.Reverse(opened)
			if !slices.Equal(opened, closed) {
				t.Fatalf("opened (reversed) %v, closed %v", opened, closed)
			}
		})
	}
}

func TestRunOnceErrors(t *testing.T) {
	t.Parallel()
	const sentinel = "sentinel-secret"
	env := "OT_PG_DSN_RUNNER=" + sentinel + "-r\nOT_PG_DSN_SERIES_FRED=" + sentinel + "-s\nFRED_API_KEY=" + sentinel + "-k\n" +
		"OT_PG_DSN_OPTION_EOD=" + sentinel + "-e\nOT_PG_DSN_QUOTE_SNAPSHOT=" + sentinel + "-q\nOT_PG_DSN_MASTER=" + sentinel + "-m\n" +
		"OT_PG_DSN_PAGES=" + sentinel + "-p\nOT_PG_DSN_READINGS=" + sentinel + "-rd\nOT_PG_DSN_LOADER=" + sentinel + "-ld\n"
	for _, line := range []string{
		"OT_PG_DSN_OPTION_EOD", "OT_PG_DSN_QUOTE_SNAPSHOT", "OT_PG_DSN_MASTER", "OT_PG_DSN_PAGES", "OT_PG_DSN_READINGS", "OT_PG_DSN_LOADER",
	} {
		t.Run("missing "+line, func(t *testing.T) {
			var kept []string
			for _, l := range strings.Split(env, "\n") {
				if !strings.HasPrefix(l, line+"=") {
					kept = append(kept, l)
				}
			}
			r := newRig(t, strings.Join(kept, "\n"))
			err := runOnce(r.a, nil)
			if err == nil || !strings.Contains(err.Error(), line) || strings.Contains(err.Error(), sentinel) || len(r.ev.list()) != 0 {
				t.Fatalf("err %v events %v", err, r.ev.list())
			}
		})
	}
	for _, tc := range []struct{ step, line string }{
		{"eod", "OT_PG_DSN_OPTION_EOD"}, {"snap", "OT_PG_DSN_QUOTE_SNAPSHOT"}, {"instr", "OT_PG_DSN_MASTER"},
		{"c-catalog", "OT_PG_DSN_PAGES"}, {"c-series", "OT_PG_DSN_PAGES"}, {"readings", "OT_PG_DSN_READINGS"}, {"sd-loader", "OT_PG_DSN_LOADER"},
	} {
		t.Run("open "+tc.step, func(t *testing.T) {
			r := newRig(t, env)
			r.fail[tc.step] = errors.New("pgx dial " + sentinel + " " + r.a.getenv("OT_HOME"))
			err := runOnce(r.a, nil)
			if err == nil || !strings.Contains(err.Error(), tc.line) || strings.Contains(err.Error(), sentinel) ||
				strings.Contains(err.Error(), r.a.getenv("OT_HOME")) || r.reconciled != 0 {
				t.Fatalf("err %v", err)
			}
		})
	}
}

func TestRunOnceErrorsRefused(t *testing.T) {
	t.Parallel()
	const sentinel = "sentinel-secret"
	env := fullEnv
	t.Run("bad day", func(t *testing.T) {
		r := newRig(t, env)
		if err := runOnce(r.a, []string{"--day", "04/11/2026"}); err == nil || len(r.ev.list()) != 0 {
			t.Fatalf("err %v events %v", err, r.ev.list())
		}
	})
	t.Run("demo", func(t *testing.T) {
		r := newRig(t, env)
		r.a.profile.Name = "demo"
		if err := runOnce(r.a, nil); err == nil || len(r.ev.list()) != 0 {
			t.Fatalf("err %v events %v", err, r.ev.list())
		}
	})
	for _, day := range []string{"2026-11-03", "2026-11-04"} {
		t.Run("cursor "+day, func(t *testing.T) {
			r := newRig(t, env)
			r.cursor = "2026-11-04"
			err := runOnce(r.a, []string{"--day", day})
			if err == nil || !strings.Contains(err.Error(), "2026-11-04") || r.reconciled != 0 || strings.Contains(err.Error(), sentinel) {
				t.Fatalf("err %v", err)
			}
		})
	}
}

func TestRunOnceDialFails(t *testing.T) {
	t.Parallel()
	r := newRig(t, fullEnv)
	r.fail["broker"] = errors.New("dial unix secret-dial-text")
	r.fail["reconcile"] = errors.New("from reconcile")
	err := runOnce(r.a, []string{"--day", "2026-11-04"})
	if err == nil || !strings.Contains(err.Error(), "from reconcile") {
		t.Fatalf("err %v, want what Reconcile returned", err)
	}
	if len(r.jobs) != 6 {
		t.Fatalf("Reconcile got %d jobs", len(r.jobs))
	}
	for _, j := range r.jobs[1:3] {
		res := j.Run(t.Context(), "2026-11-04", nil)
		if res.Err == nil || res.Err.Kind() != contracts.Retryable {
			t.Fatalf("%s: err %v", j.ID(), res.Err)
		}
		if strings.Contains(res.Err.Error(), r.socket) || strings.Contains(res.Err.Error(), "secret-dial-text") {
			t.Fatalf("%s: text %q", j.ID(), res.Err.Error())
		}
	}
	if res := r.jobs[0].Run(t.Context(), "2026-11-04", nil); res.Err != nil {
		t.Fatalf("fred job: %v", res.Err)
	}
	if n := strings.Count(r.log.String(), "dial broker"); n != 1 || strings.Contains(r.log.String(), "secret-dial-text") {
		t.Fatalf("log %q", r.log.String())
	}
}

func TestRunOnceSocketPathLength(t *testing.T) {
	t.Parallel()
	const tail = "/run/ibkr.sock"
	for _, tc := range []struct {
		n  int
		ok bool
	}{{103, true}, {104, false}, {150, false}} {
		r := newRig(t, fullEnv)
		home := "/" + strings.Repeat("h", tc.n-len(tail)-1)
		r.a.getenv = func(k string) string {
			if k == "OT_HOME" {
				return home
			}
			return ""
		}
		r.a.deps.readEnv = func(string) (map[string]string, error) {
			return map[string]string{
				"OT_PG_DSN_RUNNER": "a", "OT_PG_DSN_SERIES_FRED": "a", "FRED_API_KEY": "a",
				"OT_PG_DSN_OPTION_EOD": "a", "OT_PG_DSN_QUOTE_SNAPSHOT": "a", "OT_PG_DSN_MASTER": "a",
				"OT_PG_DSN_PAGES": "a", "OT_PG_DSN_READINGS": "a", "OT_PG_DSN_LOADER": "a",
			}, nil
		}
		err := runOnce(r.a, nil)
		if tc.ok {
			if err != nil || len(r.socket) != tc.n {
				t.Fatalf("%d bytes: err %v, dialled %d", tc.n, err, len(r.socket))
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), "socket path too long") || strings.Contains(err.Error(), home) || len(r.ev.list()) != 0 {
			t.Fatalf("%d bytes: err %v events %v", tc.n, err, r.ev.list())
		}
	}
}

func TestRunOnceWiresFourJobs(t *testing.T) {
	t.Parallel()
	r := newRig(t, fullEnv)
	if err := runOnce(r.a, []string{"--day", "2026-11-04"}); err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, j := range r.jobs {
		ids = append(ids, j.ID())
	}
	if !slices.Equal(ids, []string{"fred_series", "option_eod", "quote_snapshot", "compute_figures", "calendar_refresh", "earnings_refresh"}) {
		t.Fatalf("jobs %v", ids)
	}
	c := r.compute
	if c.Catalog == nil || c.OptionEod == nil || c.Terms == nil || c.Names == nil || c.Series == nil || c.Readings == nil || c.SessionDays == nil {
		t.Fatalf("a store of compute_figures is nil: %+v", c)
	}
	if _, ok := c.SessionDays.(evSD); !ok {
		t.Errorf("session days are %T, want the runner store already open", c.SessionDays)
	}
	// backfill hands the runner the same four jobs.
	r.jobs = nil
	if err := backfill(r.a, []string{"--day", "2026-11-04"}); err != nil || len(r.jobs) != 6 {
		t.Fatalf("backfill err %v jobs %d", err, len(r.jobs))
	}
}
