package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/clock/calendarcli"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/symbol"
)

func init() { register("run-once", runOnce) }

// maxSocketPath is the length from which a Unix socket path is refused: sun_path holds 104 bytes on macOS.
const maxSocketPath = 104

// runOnceConfig is what run-once needs from $OT_HOME/.env and the home; its values are never printed.
type runOnceConfig struct {
	runnerDSN, seriesDSN, optionEodDSN, quoteSnapshotDSN, masterDSN, fredKey string
	pagesDSN, readingsDSN, loaderDSN, calendarUV                             string
	// eventsDSN and alphaKey belong to earnings_refresh alone: an empty one leaves that job out, never the run.
	eventsDSN, alphaKey string
	// candidateDSN belongs to option_candidates alone: an empty one leaves that job out, never the run.
	candidateDSN, accountDSN string
	// jquantsKey is the J-Quants key; an empty one leaves only the JP source out (JP entries get unsupported rows).
	jquantsKey string
	// watch is $OT_HOME/watch.toml, loaded with the lines so that a refusal comes before any lock or store.
	watch      []jobs.WatchEntry
	socketPath string
}

func loadRunOnceConfig(a *app) (runOnceConfig, error) { return loadConfig(a, false) }

// loadConfig reads the lines of run-once from $OT_HOME/.env. With allowNoFredKey an empty FRED_API_KEY is not
// an error: ot scheduler then leaves fred_series out (Decision 8). A missing DSN is always an error.
func loadConfig(a *app, allowNoFredKey bool) (runOnceConfig, error) {
	env, err := a.envFile()
	if err != nil {
		return runOnceConfig{}, err
	}
	c := runOnceConfig{
		runnerDSN:        env[store.DSNEnvRunner],
		seriesDSN:        env[store.DSNEnvSeriesFred],
		optionEodDSN:     env[store.DSNEnvOptionEod],
		quoteSnapshotDSN: env[store.DSNEnvQuoteSnapshot],
		masterDSN:        env[store.DSNEnvMaster],
		fredKey:          env["FRED_API_KEY"],
		pagesDSN:         env[store.DSNEnvPages],
		readingsDSN:      env[store.DSNEnvReadings],
		loaderDSN:        env[store.DSNEnvLoader],
		calendarUV:       env[envCalendarUV],
		eventsDSN:        env[store.DSNEnvEvents],
		alphaKey:         env[envAlphaKey],
		jquantsKey:       env[envJQuantsKey],
		candidateDSN:     env[store.DSNEnvCandidate],
		accountDSN:       env[store.DSNEnvAccount],
	}
	for _, l := range []struct{ name, v string }{
		{store.DSNEnvRunner, c.runnerDSN}, {store.DSNEnvSeriesFred, c.seriesDSN},
		{store.DSNEnvOptionEod, c.optionEodDSN}, {store.DSNEnvQuoteSnapshot, c.quoteSnapshotDSN},
		{store.DSNEnvMaster, c.masterDSN}, {store.DSNEnvPages, c.pagesDSN},
		{store.DSNEnvReadings, c.readingsDSN}, {store.DSNEnvLoader, c.loaderDSN}, {"FRED_API_KEY", c.fredKey},
	} {
		if l.v == "" && (!allowNoFredKey || l.name != "FRED_API_KEY") {
			return runOnceConfig{}, fmt.Errorf("%s is missing in .env", l.name)
		}
	}
	if c.watch, err = loadWatch(a); err != nil {
		return runOnceConfig{}, err
	}
	c.socketPath = filepath.Join(a.getenv("OT_HOME"), "run", "ibkr.sock")
	if len(c.socketPath) >= maxSocketPath {
		return runOnceConfig{}, fmt.Errorf("socket path too long")
	}
	return c, nil
}

// earningsOff is the reason earnings_refresh is left out, or "" when it runs. It names a line, never a value.
func (c runOnceConfig) earningsOff() string {
	switch {
	case c.eventsDSN == "":
		return reasonNoEventsDSN
	case c.alphaKey == "":
		return reasonNoAlphaKey
	}
	return ""
}

func runOnce(a *app, args []string) error {
	fs := flag.NewFlagSet("run-once", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dayFlag := fs.String("day", "", "day YYYY-MM-DD (default: today in "+marketZone+")")
	dryRun := fs.Bool("dry-run", false, "capture through the live broker into memory; no store, no guard, no write")
	var jobNames []string
	fs.Func("job", "with --dry-run: option_eod, option_eod_jp or quote_snapshot (repeatable)", func(v string) error {
		if !slices.Contains(dryRunJobs, v) {
			return fmt.Errorf("--job %q is not one of %s", v, strings.Join(dryRunJobs, ", "))
		}
		jobNames = append(jobNames, v)
		return nil
	})
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(jobNames) > 0 && !*dryRun {
		return fmt.Errorf("--job needs --dry-run")
	}
	if *dryRun {
		return runDryRun(context.Background(), a, jobNames, *dayFlag)
	}
	if a.profile.Name == "demo" {
		return fmt.Errorf("run-once needs a real profile; the demo profile opens no store")
	}
	cfg, err := loadRunOnceConfig(a)
	if err != nil {
		return err
	}
	ctx := context.Background()
	clk := a.deps.newClock()
	day, err := dayOrDefault(clk, *dayFlag)
	if err != nil {
		return err
	}
	fence, err := holdOffScheduler(ctx, a, cfg, "run-once", "run-once refused: ot scheduler holds the lock or the lock cannot be taken; check ot scheduler status")
	if err != nil {
		return err
	}
	defer func() { _ = fence.Close() }()
	if err := reconcileDay(ctx, a, clk, cfg, day); err != nil {
		return fmt.Errorf("run-once %s: %w", day, err)
	}
	return nil
}

// reconcileDay opens the stores, wires the jobs and reconciles day; it closes the stores on every path.
func reconcileDay(ctx context.Context, a *app, clk clock.Clock, cfg runOnceConfig, day contracts.Day) error {
	check := func(ledger store.OpsLedger) error {
		cur, err := ledger.Cursor(ctx)
		if err != nil {
			return fmt.Errorf("read reconcile cursor failed")
		}
		if cur.Day != "" && day <= cur.Day {
			return fmt.Errorf("day %s is at or before the cursor %s: nothing to reconcile; use ot backfill --day %s", day, cur.Day, day)
		}
		return nil
	}
	return withWiring(ctx, a, clk, cfg, check, func(w wiring) error {
		js := w.runnable(a.log)
		rec := a.deps.newRunner(w.ledger, w.cal, clk, js, runner.WindowRule{})
		if err := rec.Reconcile(ctx, day); err != nil {
			return err
		}
		return printOutcomes(ctx, a.stdout, w.ledger, js, day, w.earningsOff)
	})
}

// wiring is what run-once, backfill and ot scheduler build from the open stores.
type wiring struct {
	ledger store.OpsLedger
	cal    clock.Calendar
	jobs   []contracts.Job
	// brokerUp is false when the dial at start failed and the option jobs hold the stand-in broker.
	brokerUp bool
	// factory builds a job anew for the probe of ot scheduler; its stores stay open until use returns.
	factory jobFactory
	// earn is the earnings_refresh job held in jobs.
	earn contracts.Job
	// earningsOff is the reason earn is left out of run-once and backfill; the
	// scheduler keeps the job in jobs so that its ticker knows the schedule (preflight.go).
	earningsOff string
	// cand is the option_candidates job held in jobs (nil for a deps without it); candidatesOff is the reason it is left out.
	cand          contracts.Job
	candidatesOff string
	// dial dials the broker anew; the broker it returns is closed when withWiring returns.
	dial func(ctx context.Context) (broker.Broker, error)
}

// runnable is jobs without earnings_refresh when its line is missing; the left-out job is logged by id and
// reason.
func (w wiring) runnable(log *slog.Logger) []contracts.Job {
	drop := map[contracts.Job]bool{}
	if w.earningsOff != "" {
		log.Warn("job left out", "component", "ot", "job", jobEarningsRefresh, "status", w.earningsOff)
		drop[w.earn] = true
	}
	if w.cand != nil && w.candidatesOff != "" {
		log.Warn("job left out", "component", "ot", "job", jobOptionCandidates, "status", w.candidatesOff)
		drop[w.cand] = true
	}
	if len(drop) == 0 {
		return w.jobs
	}
	out := make([]contracts.Job, 0, len(w.jobs))
	for _, j := range w.jobs {
		if !drop[j] {
			out = append(out, j)
		}
	}
	return out
}

// jobFactory builds the jobs over the stores withWiring opened.
type jobFactory struct {
	d    deps
	clk  clock.Clock
	src  sources.Source
	ser  store.SeriesStore
	eod  store.OptionEodStore
	snap store.QuoteSnapshotStore
	inst store.InstrumentStore
	// compute holds the stores of compute_figures.
	compute jobs.ComputeStores
	uv      string
	log     *slog.Logger
	// loader is the session-day store opened with the loader line; only calendar_refresh gets it.
	loader store.SessionDayStore
	// events, earn and watch are what earnings_refresh gets: its store, one source per market and watch.toml.
	events store.EarningsDateStore
	earn   map[string]sources.EarningsSource
	watch  []jobs.WatchEntry
	// candidates, accounts and earnRd are what option_candidates (optioncandidates.New) gets besides the compute stores;
	// the last two may be nil.
	candidates store.CandidateWriter
	accounts   store.AccountReader
	earnRd     store.EarningsDateReader
}

// earningsRefresh builds earnings_refresh over the events store, the Alpha Vantage source and the watch list.
func (f jobFactory) earningsRefresh() (contracts.Job, error) {
	j, err := f.d.newEarningsRefresh(f.earn, f.events, f.events, f.watch, f.clk, f.d.symbols)
	if err != nil {
		return nil, fmt.Errorf("build earnings_refresh job failed: %w", err)
	}
	return j, nil
}

// noEventsStore stands in for the events store when OT_PG_DSN_EVENTS is empty: the job is left out, so it is
// never called.
type noEventsStore struct{}

func (noEventsStore) InsertEarningsDate(context.Context, contracts.EarningsDate) error {
	return errors.New(reasonNoEventsDSN)
}

func (noEventsStore) EarningsDates(context.Context, string, contracts.Day, contracts.Day) ([]contracts.EarningsDate, error) {
	return nil, errors.New(reasonNoEventsDSN)
}

func (noEventsStore) Close() error { return nil }

// calendarRefresh builds calendar_refresh over the loader store; the command is the uv program of the .env
// line (default uv) and calendarArgs, run by calendarcli.Exec in the default temporary directory.
func (f jobFactory) calendarRefresh() (contracts.Job, error) {
	uv := f.uv
	if uv == "" {
		uv = "uv"
	}
	argv := append([]string{uv}, calendarArgs...)
	src := f.d.newCommandSessionSource(argv, "", calendarcli.Exec)
	// The log handler passes only the keys of profile.logFields, so the change is one value of "status".
	obs := func(c clock.SessionDayChange) {
		f.log.Info("calendar change", "component", "ot", "job", "calendar_refresh",
			"status", fmt.Sprintf("day=%s before=[%s] after=[%s]", c.Day, describeDay(c.Before), describeDay(c.After)))
	}
	j, err := f.d.newCalendarRefresh(src, f.loader, f.d.newDiffer(), f.clk, obs)
	if err != nil {
		return nil, fmt.Errorf("build calendar_refresh job failed: %w", err)
	}
	return j, nil
}

// describeDay renders one session-day row for the log; nil is "none" (the day is absent on that side).
func describeDay(d *contracts.SessionDay) string {
	if d == nil {
		return "none"
	}
	return fmt.Sprintf("session=%t early_close=%t open=%s close=%s", d.IsSession, d.EarlyClose,
		d.OpenAt.UTC().Format(time.RFC3339), d.CloseAt.UTC().Format(time.RFC3339))
}

func (f jobFactory) computeFigures() (contracts.Job, error) {
	j, err := f.d.newComputeFigures(f.compute, f.d.computeFigures, f.d.computeRules, f.d.computeUnits, f.d.computeSeriesOps, f.d.computeEase, f.clk)
	if err != nil {
		return nil, fmt.Errorf("build compute_figures job failed: %w", err)
	}
	return j, nil
}

func (f jobFactory) fred() (contracts.Job, error) {
	j, err := f.d.newFredSeries(f.src, f.ser, f.clk)
	if err != nil {
		return nil, fmt.Errorf("build fred_series job failed: %w", err)
	}
	return j, nil
}

func (f jobFactory) optionEod(b broker.Broker) (contracts.Job, error) {
	return buildOptionEod(f.d, b, f.inst, f.eod, f.watch, f.clk)
}

// buildOptionEod builds option_eod over the watch list: the one constructor path of run-once, the scheduler and
// the dry run.
func buildOptionEod(d deps, b broker.Broker, r store.InstrumentResolver, w store.OptionEodWriter,
	watch []jobs.WatchEntry, clk clock.Clock) (contracts.Job, error) {
	build := d.newOptionEodWatch
	if build == nil {
		// A deps literal of a test that fakes only newOptionEod; newDeps always sets newOptionEodWatch.
		build = func(b broker.Broker, r store.InstrumentResolver, w store.OptionEodWriter, _ []jobs.WatchEntry, c clock.Clock, t symbol.Translator) (contracts.Job, error) {
			return d.newOptionEod(b, r, w, c, t)
		}
	}
	j, err := build(b, r, w, watch, clk, d.symbols)
	if err != nil {
		return nil, fmt.Errorf("build option_eod job failed: %w", err)
	}
	return j, nil
}

// optionEodJP builds option_eod_jp over the same stores as option_eod.
func (f jobFactory) optionEodJP(b broker.Broker) (contracts.Job, error) {
	j, err := f.d.newOptionEodJP(b, f.inst, f.eod, f.clk)
	if err != nil {
		return nil, fmt.Errorf("build option_eod_jp job failed: %w", err)
	}
	return j, nil
}

func (f jobFactory) quoteSnapshot(b broker.Broker) (contracts.Job, error) {
	j, err := f.d.newQuoteSnapshot(b, f.inst, f.snap, f.clk)
	if err != nil {
		return nil, fmt.Errorf("build quote_snapshot job failed: %w", err)
	}
	return j, nil
}

// withWiring opens the stores and the broker, wires fred_series, option_eod, quote_snapshot, compute_figures and calendar_refresh, then
// calls use; it closes what it opened on every path, last opened first. check runs after the ledger is
// open and before the other stores are opened; it may be nil. A failed dial does not stop the call: the
// option jobs get a broker that answers retryable. An empty FRED key (only ot scheduler lets it through)
// gives fred_series a source that answers permanent. No error text holds a DSN, a path or a key.
func withWiring(
	ctx context.Context, a *app, clk clock.Clock, cfg runOnceConfig, check func(store.OpsLedger) error, use func(wiring) error,
) error {
	d := a.deps
	sd, loader, closeDays, err := openSessionDays(ctx, d, cfg)
	if err != nil {
		return err
	}
	defer closeDays()
	ledger, err := d.openLedger(ctx, cfg.runnerDSN)
	if err != nil {
		return fmt.Errorf("open ledger with %s failed", store.DSNEnvRunner)
	}
	defer func() { _ = ledger.Close() }()
	if check != nil {
		if err := check(ledger); err != nil {
			return err
		}
	}
	ser, err := d.openSeries(ctx, cfg.seriesDSN)
	if err != nil {
		return fmt.Errorf("open series store with %s failed", store.DSNEnvSeriesFred)
	}
	defer func() { _ = ser.Close() }()
	eod, err := d.openOptionEod(ctx, cfg.optionEodDSN)
	if err != nil {
		return fmt.Errorf("open option_eod store with %s failed", store.DSNEnvOptionEod)
	}
	defer func() { _ = eod.Close() }()
	snap, err := d.openQuoteSnapshot(ctx, cfg.quoteSnapshotDSN)
	if err != nil {
		return fmt.Errorf("open quote_snapshot store with %s failed", store.DSNEnvQuoteSnapshot)
	}
	defer func() { _ = snap.Close() }()
	instr, err := d.openInstrument(ctx, cfg.masterDSN)
	if err != nil {
		return fmt.Errorf("open instrument store with %s failed", store.DSNEnvMaster)
	}
	defer func() { _ = instr.Close() }()
	cs, closeCompute, err := openComputeStores(ctx, d, cfg, sd)
	if err != nil {
		return err
	}
	defer closeCompute()
	src, err := fredSource(d, cfg, clk)
	if err != nil {
		return err
	}
	events, earn, closeEvents, err := openEarnings(ctx, a, d, cfg)
	if err != nil {
		return err
	}
	defer closeEvents()
	brk, brokerUp := dialOrStandIn(ctx, a, cfg.socketPath)
	defer func() { _ = brk.Close() }()
	var extra closerSet
	defer extra.closeAll()
	return wireJobs(ctx, a, cfg, use, brk, brokerUp, &extra, d.newCalendar(sd),
		jobFactory{d: d, clk: clk, src: src, ser: ser, eod: eod, snap: snap, inst: instr, compute: cs, loader: loader, uv: cfg.calendarUV, log: a.log,
			events: events, earn: earn, watch: cfg.watch}, ledger)
}

// wireJobs builds the jobs over the open stores and calls use. option_eod_jp comes after option_eod and
// is left out, with one named line, when the XTKS calendar answers an error for today (Decision 13).
func wireJobs(
	ctx context.Context, a *app, cfg runOnceConfig, use func(wiring) error, brk broker.Broker, brokerUp bool,
	extra *closerSet, cal clock.Calendar, f jobFactory, ledger store.OpsLedger,
) error {
	fredJob, err := f.fred()
	if err != nil {
		return err
	}
	eodJob, err := f.optionEod(brk)
	if err != nil {
		return err
	}
	jobList := []contracts.Job{fredJob, eodJob}
	if reason := xtksReason(ctx, cal, f.clk); reason != "" {
		a.log.Warn(reason, "component", "ot", "job", jobOptionEodJP)
	} else {
		jpJob, err := f.optionEodJP(brk)
		if err != nil {
			return err
		}
		jobList = append(jobList, jpJob)
	}
	snapJob, err := f.quoteSnapshot(brk)
	if err != nil {
		return err
	}
	figJob, err := f.computeFigures()
	if err != nil {
		return err
	}
	calJob, err := f.calendarRefresh()
	if err != nil {
		return err
	}
	earnJob, err := f.earningsRefresh()
	if err != nil {
		return err
	}
	jobList = append(jobList, snapJob, figJob, calJob, earnJob)
	acctJob, err := a.accountCapture(ctx, f, extra)
	if err != nil {
		return err
	}
	if acctJob != nil {
		jobList = append(jobList, acctJob)
	}
	candJob, err := a.candidateJob(ctx, cfg, &f, extra)
	if err != nil {
		return err
	}
	if candJob != nil {
		jobList = append(jobList, candJob)
	}
	return use(wiring{
		ledger: ledger, cal: cal, jobs: jobList, brokerUp: brokerUp, factory: f,
		earn: earnJob, earningsOff: cfg.earningsOff(), cand: candJob, candidatesOff: cfg.candidatesOff(),
		dial: func(ctx context.Context) (broker.Broker, error) {
			b, err := a.deps.dialBroker(ctx, cfg.socketPath, broker.Options{})
			if err != nil {
				return nil, err
			}
			extra.add(b)
			return b, nil
		},
	})
}

// closerSet holds the brokers, readers and stores opened after start, so that withWiring closes them.
type closerSet struct {
	mu sync.Mutex
	cs []io.Closer
}

func (s *closerSet) add(b broker.Broker) { s.addCloser(b) }

// addCloser holds anything else withWiring opens after start (the account readers and store).
func (s *closerSet) addCloser(c io.Closer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cs = append(s.cs, c)
}

func (s *closerSet) closeAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, b := range s.cs {
		_ = b.Close()
	}
}

// dialOrStandIn dials the broker once; a failure is logged by step name only and gives the stand-in
// broker, whose Close does nothing. The bool is false for the stand-in.
func dialOrStandIn(ctx context.Context, a *app, socketPath string) (broker.Broker, bool) {
	b, err := a.deps.dialBroker(ctx, socketPath, broker.Options{})
	if err != nil {
		a.log.Error("dial broker failed: the option jobs answer retryable", "component", "ot", "step", "dial broker")
		return unreachableBroker{}, false
	}
	return b, true
}

// holdOffScheduler takes the instance fence and keeps it until the caller closes it: a hand command refuses
// while ot scheduler holds it (Decision 6). The error texts hold no DSN.
func holdOffScheduler(ctx context.Context, a *app, cfg runOnceConfig, who, refusal string) (store.Fence, error) {
	inst, err := a.deps.openFence(ctx, cfg.runnerDSN)
	if err != nil {
		return nil, fmt.Errorf("open instance fence with %s failed", store.DSNEnvRunner)
	}
	fence, err := inst.TryLock(ctx, who)
	if err != nil {
		return nil, errors.New(refusal)
	}
	return fence, nil
}

// openSessionDays opens the window store with the runner line and the store of calendar_refresh with the loader
// line; the returned func closes both, the loader store first.
func openSessionDays(ctx context.Context, d deps, cfg runOnceConfig) (sd, loader store.SessionDayStore, closeAll func(), err error) {
	sd, err = d.openSessionDay(ctx, cfg.runnerDSN)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("open session-day store with %s failed", store.DSNEnvRunner)
	}
	loader, err = d.openSessionDay(ctx, cfg.loaderDSN)
	if err != nil {
		_ = sd.Close()
		return nil, nil, nil, fmt.Errorf("open session-day store with %s failed", store.DSNEnvLoader)
	}
	return sd, loader, func() { _ = loader.Close(); _ = sd.Close() }, nil
}

// openComputeStores opens the stores compute_figures reads and writes: catalog, eod, terms, names and series
// with the pages line, the readings store with the readings line; the session days are the runner store
// already open. The returned func closes what was opened, last opened first.
func openComputeStores(ctx context.Context, d deps, cfg runOnceConfig, sd store.SessionDayStore) (jobs.ComputeStores, func(), error) {
	var closers []func() error
	closeAll := func() {
		for i := len(closers) - 1; i >= 0; i-- {
			_ = closers[i]()
		}
	}
	pagesFail := fmt.Errorf("open compute stores with %s failed", store.DSNEnvPages)
	cs := jobs.ComputeStores{SessionDays: sd}
	cat, err := d.openOptionCatalog(ctx, cfg.pagesDSN)
	if err != nil {
		return jobs.ComputeStores{}, nil, pagesFail
	}
	closers = append(closers, cat.Close)
	cs.Catalog = cat
	eod, err := d.openOptionEod(ctx, cfg.pagesDSN)
	if err != nil {
		closeAll()
		return jobs.ComputeStores{}, nil, pagesFail
	}
	closers = append(closers, eod.Close)
	cs.OptionEod = eod
	terms, err := d.openOptionTerms(ctx, cfg.pagesDSN)
	if err != nil {
		closeAll()
		return jobs.ComputeStores{}, nil, pagesFail
	}
	closers = append(closers, terms.Close)
	cs.Terms = terms
	names, err := d.openInstrumentName(ctx, cfg.pagesDSN)
	if err != nil {
		closeAll()
		return jobs.ComputeStores{}, nil, pagesFail
	}
	closers = append(closers, names.Close)
	cs.Names = names
	ser, err := d.openSeries(ctx, cfg.pagesDSN)
	if err != nil {
		closeAll()
		return jobs.ComputeStores{}, nil, pagesFail
	}
	closers = append(closers, ser.Close)
	cs.Series = ser
	rd, err := d.openReading(ctx, cfg.readingsDSN)
	if err != nil {
		closeAll()
		return jobs.ComputeStores{}, nil, fmt.Errorf("open readings store with %s failed", store.DSNEnvReadings)
	}
	closers = append(closers, rd.Close)
	cs.Readings = rd
	return cs, closeAll, nil
}

// openEarnings opens the earnings store with the events line (the stand-in when the line is empty) and builds the
// Alpha Vantage source (none when the key is empty) and the J-Quants source over the JP symbols of the watch list
// (none, with one logged line naming the key, when its key is empty). The returned func closes the store. No error holds a DSN or a key.
func openEarnings(ctx context.Context, a *app, d deps, cfg runOnceConfig) (store.EarningsDateStore, map[string]sources.EarningsSource, func(), error) {
	var events store.EarningsDateStore = noEventsStore{}
	if cfg.eventsDSN != "" {
		var err error
		events, err = d.openEarningsDates(ctx, cfg.eventsDSN)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("open earnings store with %s failed", store.DSNEnvEvents)
		}
	}
	earn := map[string]sources.EarningsSource{}
	if cfg.alphaKey != "" {
		av, err := d.newAlphaVantage(sources.EarningsConfig{BaseURL: alphaVantageBaseURL, APIKey: cfg.alphaKey})
		if err != nil {
			_ = events.Close()
			return nil, nil, nil, fmt.Errorf("build Alpha Vantage source failed: %w", err)
		}
		earn["US"] = av
	}
	if cfg.jquantsKey == "" {
		a.log.Warn("JP earnings source left out", "component", "ot", "status", reasonNoJQuantsKey)
	} else {
		jq, err := d.newJQuants(sources.EarningsConfig{BaseURL: jquantsBaseURL, APIKey: cfg.jquantsKey, Symbols: jpSymbols(cfg.watch)})
		if err != nil {
			_ = events.Close()
			return nil, nil, nil, fmt.Errorf("build J-Quants source failed: %w", err)
		}
		earn["JP"] = jq
	}
	return events, earn, func() { _ = events.Close() }, nil
}

// fredSource is the FRED source, or the stand-in that answers permanent when the key is empty.
func fredSource(d deps, cfg runOnceConfig, clk clock.Clock) (sources.Source, error) {
	if cfg.fredKey == "" {
		return noKeySource{}, nil
	}
	src, err := d.newFRED(sources.FREDConfig{BaseURL: fredBaseURL, APIKey: cfg.fredKey, Clock: clk})
	if err != nil {
		return nil, fmt.Errorf("build FRED source failed: %w", err)
	}
	return src, nil
}

// jpSymbols is the symbol of each JP entry of the watch list, in file order: the codes one J-Quants call each.
func jpSymbols(watch []jobs.WatchEntry) []string {
	var out []string
	for _, e := range watch {
		if e.Market == "JP" {
			out = append(out, e.Contract.Symbol)
		}
	}
	return out
}
