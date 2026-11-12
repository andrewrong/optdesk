package main

import (
	"cmp"
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
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/jobs/computefigures"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/bar"
	"github.com/andrewrong/optdesk/internal/store/dividend"
	"github.com/andrewrong/optdesk/internal/store/openquote"
	"github.com/andrewrong/optdesk/internal/store/underlying"
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
	// ledgerDSN belongs to option_ledger alone: an empty one leaves that job out, never the run.
	ledgerDSN string
	// barDSN belongs to bars_refresh, fx_refresh and the HV step: an empty one leaves only them out, never the run.
	barDSN string
	// openDSN belongs to the four option open jobs: an empty one leaves only them out, never the run.
	openDSN string
	// resolveDSN belongs to underlying_resolve alone: an empty one leaves only that job out, never the run.
	resolveDSN string
	// barsWired is true when the deps hold the constructors of bars_refresh and fx_refresh (newDeps always does).
	barsWired bool
	// jquantsKey is the J-Quants key; an empty one leaves only the JP source out (JP entries get unsupported rows).
	jquantsKey string
	// factDSN and secAgent belong to the fundamentals job and the valuation step of compute_figures: an empty one
	// leaves only them out, never the run.
	factDSN, secAgent string
	socketPath        string
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
		factDSN:          env[store.DSNEnvFundamental],
		secAgent:         env[envSECUserAgent],
		candidateDSN:     env[store.DSNEnvCandidate],
		accountDSN:       env[store.DSNEnvAccount],
		ledgerDSN:        env[store.DSNEnvOptionLedger],
		barDSN:           env[bar.DSNEnvBar],
		openDSN:          env[openquote.DSNEnvOptionOpen],
		resolveDSN:       env[underlying.DSNEnvResolve],
		barsWired:        a.deps.openBar != nil && a.deps.newBarsRefresh != nil && a.deps.newFxRefresh != nil && a.deps.newHistory != nil,
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
	// watch.toml is still checked here until issue 10 of docs/design/underlyings.md; no job follows it.
	if _, err = loadWatch(a); err != nil {
		return runOnceConfig{}, err
	}
	c.socketPath = filepath.Join(a.getenv("OT_HOME"), "run", "ibkr.sock")
	if len(c.socketPath) >= maxSocketPath {
		return runOnceConfig{}, fmt.Errorf("socket path too long")
	}
	return c, nil
}

func runOnce(a *app, args []string) error {
	fs := flag.NewFlagSet("run-once", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dayFlag := fs.String("day", "", "day YYYY-MM-DD (default: today in "+marketZone+")")
	dryRun := fs.Bool("dry-run", false, "capture through the live broker into memory; no store, no guard, no write")
	force := fs.Bool("force", false, "with --job: run a key that is already present again")
	var jobNames []string
	fs.Func("job", "job id to run (one for a real run; with --dry-run repeatable: option_eod, option_eod_jp, quote_snapshot)", func(v string) error {
		jobNames = append(jobNames, v)
		return nil
	})
	if err := fs.Parse(args); err != nil {
		return err
	}
	for _, n := range jobNames {
		if *dryRun && !slices.Contains(dryRunJobs, n) {
			return fmt.Errorf("--job %q is not one of %s", n, strings.Join(dryRunJobs, ", "))
		}
		if _, ok := jobRowOf(n); !*dryRun && !ok {
			return unknownJobError(n)
		}
	}
	if err := checkJobCount(len(jobNames), *dryRun, *force); err != nil {
		return err
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
	ctx, clk := context.Background(), a.deps.newClock()
	day, err := dayOrDefault(clk, *dayFlag)
	if err != nil {
		return err
	}
	if len(jobNames) == 1 && !*dryRun {
		if err := runOnceJob(ctx, a, clk, cfg, day, jobNames[0], *force); err != nil {
			return runOnceErr(err, day)
		}
		return nil
	}
	if err := reconcileDay(ctx, a, clk, cfg, day); err != nil {
		if ref := refusal(""); errors.As(err, &ref) {
			return err
		}
		return fmt.Errorf("run-once %s: %w", day, err)
	}
	return nil
}

// reconcileDay opens the stores, wires the jobs and reconciles day, closing the stores on every path. No fence: Reconcile holds the run lease.
func reconcileDay(ctx context.Context, a *app, clk clock.Clock, cfg runOnceConfig, day contracts.Day) error {
	check := func(ledger store.OpsLedger) error {
		cur, err := ledger.Cursor(ctx)
		if err != nil {
			return errors.New("read reconcile cursor failed")
		}
		if cur.Day != "" && day <= cur.Day {
			return fmt.Errorf("day %s is at or before the cursor %s: nothing to reconcile; use ot backfill --day %s", day, cur.Day, day)
		}
		return nil
	}
	fw := a.deps.newFwClock()
	sl, err := a.deps.openSchedulerLedger(ctx, cfg.runnerDSN)
	if err != nil {
		return fmt.Errorf("open scheduler ledger with %s failed", store.DSNEnvRunner)
	}
	defer func() { _ = sl.Close() }()
	return withWiring(ctx, a, clk, cfg, check, func(w wiring) error {
		js := w.runnable(a.log)
		rec, _ := a.deps.newReconciler(w.ledger, sl, w.cal, fw, js, runner.WindowRule{}, a.deps.newKeyRunner(w.ledger, sl, fw))
		if err := rec.Reconcile(ctx, day); err != nil {
			return cmp.Or(leaseRefusal(ctx, w.ledger, js, day, fw.Now()), err)
		}
		return printOutcomes(ctx, a.stdout, w.ledger, js, day, w.off[jobEarningsRefresh])
	})
}

type refusal string // an answer run-once gives as it stands, without the "run-once <day>:" prefix

func (r refusal) Error() string { return string(r) }

// leaseRefusal answers a Reconcile error with the refusal when another run holds the run lease (probed by taking it
// under another owner), else nil. It names the job whose plan holds the dataset of a live run; no DSN, no ledger error.
func leaseRefusal(ctx context.Context, ledger store.OpsLedger, js []contracts.Job, day contracts.Day, now time.Time) error {
	runs, runsErr := ledger.Runs(ctx, day)
	probe, err := ledger.Acquire(ctx, "run-once-probe", time.Minute)
	if err == nil {
		_ = ledger.Release(ctx, probe)
	}
	if err == nil || runsErr != nil { // the lease is free, or the ledger cannot say (a database failure)
		return nil
	}
	live := func(ds string) bool {
		return slices.ContainsFunc(runs, func(r store.Run) bool {
			return r.Key.DatasetID == ds && r.Status == store.RunRunning && r.LeaseUntil.After(now)
		})
	}
	for _, j := range js {
		keys, _, perr := j.Plan(ctx, day)
		if i := slices.IndexFunc(keys, func(k contracts.ExpectedKey) bool { return live(k.DatasetID) }); perr == nil && i >= 0 {
			return refusal("run-once refused: job " + j.ID() + " is running (dataset " + keys[i].DatasetID + "); retry when it has finished")
		}
	}
	return refusal("run-once refused: the runner lease is held by another run; retry in a minute")
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
	// off is the reason a job is left out, by id: the startOff of every family and the reason a build returned.
	off map[string]string
	// held is the jobs of jobs whose build returned a reason: they are left out of run-once and backfill; the
	// scheduler keeps them in jobs so that its ticker knows the schedule (preflight.go).
	held map[string]contracts.Job
	// dial dials the broker anew; the broker it returns is closed when withWiring returns.
	dial func(ctx context.Context) (broker.Broker, error)
	// bars holds bars_refresh and fx_refresh (empty for a deps without them); barsOff is the reason they are left out
	// of run-once and backfill, barsDSNOff the part of it that only a restart cures; dialHistory dials the history client anew.
	bars        []contracts.Job
	barsOff     string
	barsDSNOff  string
	dialHistory func(ctx context.Context) (broker.HistoryClient, error)
}

// runnable is jobs without the held jobs and the bars jobs that have a reason; each left-out job is logged by id
// and reason.
func (w wiring) runnable(log *slog.Logger) []contracts.Job {
	drop := map[contracts.Job]bool{}
	for _, j := range w.jobs {
		if _, ok := w.held[j.ID()]; ok && w.off[j.ID()] != "" {
			log.Warn("job left out", "component", "ot", "job", j.ID(), "status", w.off[j.ID()])
			drop[j] = true
		}
	}
	if w.barsOff != "" {
		for _, j := range w.bars {
			log.Warn("job left out", "component", "ot", "job", j.ID(), "status", w.barsOff)
			drop[j] = true
		}
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
	compute computefigures.ComputeStores
	uv      string
	log     *slog.Logger
	// loader is the session-day store opened with the loader line; only calendar_refresh gets it.
	loader store.SessionDayStore
	// events and earn are what earnings_refresh gets: its store and the sources of the markets other than JP;
	// jp builds the J-Quants source over the symbols of one Plan (nil without a key).
	events store.EarningsDateStore
	earn   map[string]sources.EarningsSource
	jp     func(symbols []string) (sources.EarningsSource, error)
	// list and held are what every family that follows names gets (wire_underlying.go): the stored underlying list
	// and the held names by row key.
	list underlying.Reader
	// entryFix, when set, rewrites the entries of each Plan of option_eod_jp (it resolves con_ids).
	entryFix func([]jobs.WatchEntry) []jobs.WatchEntry
	held     func(context.Context) (map[string]bool, error)
	// candidates, accounts and earnRd are what option_candidates (optioncandidates.New) gets besides the compute stores;
	// the last two may be nil.
	candidates store.CandidateWriter
	accounts   store.AccountReader
	earnRd     store.EarningsDateReader
	// div is the dividend store of dividend_capture, opened with the events line.
	div dividend.Store
	// bars and hist are what bars_refresh and fx_refresh get: the bar store opened with the bar line and the history client.
	bars bar.BarStore
	hist broker.HistoryClient
	// facts is the fact store opened with the fundamental line (nil when the family left it out): the fundamentals
	// job writes it and the valuation step of compute_figures reads it.
	facts store.FactStore
	// fd holds the constructors of the valuation step (zero when deps hold none).
	fd fundamentalsDeps
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
	cs, closeCompute, err := openComputeStores(ctx, a.log, d, cfg, sd)
	if err != nil {
		return err
	}
	defer closeCompute()
	src, err := fredSource(d, cfg, clk)
	if err != nil {
		return err
	}
	events, earn, jp, closeEvents, err := openEarnings(ctx, a, d, cfg)
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
			events: events, earn: earn, jp: jp}, ledger)
}

// wireJobs builds the jobs over the open stores, one family after the other by order (jobreg.go), and calls use. A
// family left out answers nil; a held one answers its reason, kept in wiring.held and wiring.off.
func wireJobs(
	ctx context.Context, a *app, cfg runOnceConfig, use func(wiring) error, brk broker.Broker, brokerUp bool,
	extra *closerSet, cal clock.Calendar, f jobFactory, ledger store.OpsLedger,
) error {
	wireHeldCalls(cfg, &f, extra, a.log)
	if err := openJobList(ctx, a, cfg, &f, extra); err != nil {
		return err
	}
	jc := &jobCtx{cfg: cfg, f: &f, brk: brk, brokerUp: brokerUp, cal: cal, extra: extra}
	w := wiring{ledger: ledger, cal: cal, brokerUp: brokerUp, off: startDisabled(cfg, brokerUp), held: map[string]contracts.Job{}}
	steps := slices.Clone(jobSteps)
	runSteps := func(below int) error {
		for len(steps) > 0 && (below < 0 || steps[0].after < below) {
			js, err := steps[0].run(ctx, a, jc, &w)
			if err != nil {
				return err
			}
			w.jobs = append(w.jobs, js...)
			steps = steps[1:]
		}
		return nil
	}
	for _, fam := range sortedJobFamilies() {
		if err := runSteps(fam.order); err != nil {
			return err
		}
		j, reason, err := fam.build(ctx, a, jc)
		if err != nil {
			return err
		}
		if j == nil {
			continue
		}
		w.jobs = append(w.jobs, j)
		if reason != "" {
			w.held[fam.id], w.off[fam.id] = j, reason
		}
	}
	if err := runSteps(-1); err != nil {
		return err
	}
	w.factory = f
	w.barsDSNOff = cfg.barsOff()
	w.dial, w.dialHistory = redialers(a, cfg, extra)
	return use(w)
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
// already open. The index-id reader is opened with the pages line when d.openIndexIDs is set; with none, or when
// it fails, IndexIDs stays nil with one warning and the rv20 keys fail. The returned func closes what was opened, last opened first.
func openComputeStores(
	ctx context.Context, log *slog.Logger, d deps, cfg runOnceConfig, sd store.SessionDayStore,
) (computefigures.ComputeStores, func(), error) {
	var closers []func() error
	closeAll := func() {
		for i := len(closers) - 1; i >= 0; i-- {
			_ = closers[i]()
		}
	}
	pagesFail := fmt.Errorf("open compute stores with %s failed", store.DSNEnvPages)
	cs := computefigures.ComputeStores{SessionDays: sd}
	cat, err := d.openOptionCatalog(ctx, cfg.pagesDSN)
	if err != nil {
		return computefigures.ComputeStores{}, nil, pagesFail
	}
	closers = append(closers, cat.Close)
	cs.Catalog = cat
	eod, err := d.openOptionEod(ctx, cfg.pagesDSN)
	if err != nil {
		closeAll()
		return computefigures.ComputeStores{}, nil, pagesFail
	}
	closers = append(closers, eod.Close)
	cs.OptionEod = eod
	terms, err := d.openOptionTerms(ctx, cfg.pagesDSN)
	if err != nil {
		closeAll()
		return computefigures.ComputeStores{}, nil, pagesFail
	}
	closers = append(closers, terms.Close)
	cs.Terms = terms
	names, err := d.openInstrumentName(ctx, cfg.pagesDSN)
	if err != nil {
		closeAll()
		return computefigures.ComputeStores{}, nil, pagesFail
	}
	closers = append(closers, names.Close)
	cs.Names = names
	ser, err := d.openSeries(ctx, cfg.pagesDSN)
	if err != nil {
		closeAll()
		return computefigures.ComputeStores{}, nil, pagesFail
	}
	closers = append(closers, ser.Close)
	cs.Series = ser
	if d.openIndexIDs == nil {
		log.Warn("index id reader left out", "component", "ot", "step", "market.rv20", "status", "no index id opener")
	} else if ids, err := d.openIndexIDs(ctx, cfg.pagesDSN); err != nil {
		log.Warn("index id reader left out", "component", "ot", "step", "market.rv20", "status", "open failed")
	} else {
		closers = append(closers, ids.Close)
		cs.IndexIDs = ids
	}
	rd, err := d.openReading(ctx, cfg.readingsDSN)
	if err != nil {
		closeAll()
		return computefigures.ComputeStores{}, nil, fmt.Errorf("open readings store with %s failed", store.DSNEnvReadings)
	}
	closers = append(closers, rd.Close)
	cs.Readings = rd
	return cs, closeAll, nil
}

// openEarnings opens the earnings store with the events line (the stand-in when the line is empty) and builds the
// Alpha Vantage source (none when the key is empty) and the maker of the J-Quants source, which the job calls at each
// Plan over the JP names of that Plan (nil, with one logged line naming the key, when its key is empty). The returned
// func closes the store. No error holds a DSN or a key.
func openEarnings(ctx context.Context, a *app, d deps, cfg runOnceConfig) (
	store.EarningsDateStore, map[string]sources.EarningsSource, func([]string) (sources.EarningsSource, error), func(), error,
) {
	var events store.EarningsDateStore = noEventsStore{}
	if cfg.eventsDSN != "" {
		var err error
		events, err = d.openEarningsDates(ctx, cfg.eventsDSN)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("open earnings store with %s failed", store.DSNEnvEvents)
		}
	}
	earn := map[string]sources.EarningsSource{}
	if cfg.alphaKey != "" {
		av, err := d.newAlphaVantage(sources.EarningsConfig{BaseURL: alphaVantageBaseURL, APIKey: cfg.alphaKey})
		if err != nil {
			_ = events.Close()
			return nil, nil, nil, nil, fmt.Errorf("build Alpha Vantage source failed: %w", err)
		}
		earn["US"] = av
	}
	var jp func([]string) (sources.EarningsSource, error)
	if cfg.jquantsKey == "" {
		a.log.Warn("JP earnings source left out", "component", "ot", "status", reasonNoJQuantsKey)
	} else {
		jp = func(symbols []string) (sources.EarningsSource, error) {
			jq, err := d.newJQuants(sources.EarningsConfig{BaseURL: jquantsBaseURL, APIKey: cfg.jquantsKey, Symbols: symbols})
			if err != nil {
				return nil, fmt.Errorf("build J-Quants source failed: %w", err)
			}
			return jq, nil
		}
	}
	return events, earn, jp, func() { _ = events.Close() }, nil
}
