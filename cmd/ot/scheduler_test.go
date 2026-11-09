package main

import (
	"context"
	"errors"
	"maps"
	"net/http"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/alertrules"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/notify"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/runner/sched"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/jonboulle/clockwork"
)

// idJob is a job with an id; it is never run.
type idJob struct {
	contracts.Job
	id string
}

func (j idJob) ID() string { return j.id }

type fakeSchedLedger struct {
	store.SchedulerLedger
	build atomic.Value
	err   error
}

func (f *fakeSchedLedger) Close() error { return nil }

func (f *fakeSchedLedger) SetState(_ context.Context, b string) error { f.build.Store(b); return f.err }

// fakeRec is one reconciler built by the constructor; it records the days it reconciled.
type fakeRec struct {
	ids  []string
	mu   *sync.Mutex
	last *[]string
}

func (r fakeRec) Reconcile(context.Context, contracts.Day) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	*r.last = r.ids
	return nil
}

type fakeGaps struct {
	mu   sync.Mutex
	seen []map[string]string
}

func (g *fakeGaps) Sync(_ context.Context, _ contracts.Day, d map[string]string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.seen = append(g.seen, d)
	return nil
}

// fakeSched stands in for the ticker: it owns a copy of disabled and, on a tick, probes it and syncs the gaps
// as the ticker does. Run ends with ret when ctx ends or runErr is set.
type fakeSched struct {
	fence    store.Fence
	rec      runner.Reconciler
	gaps     runner.GapSyncer
	jobs     []contracts.Job
	disabled map[string]string
	runErr   error
	started  chan struct{}
	ticks    chan chan struct{}
}

func (s *fakeSched) LastTick() time.Time { return time.Time{} }
func (s *fakeSched) FenceHeld() bool     { return true }

func (s *fakeSched) Run(ctx context.Context) error {
	close(s.started)
	if s.runErr != nil {
		return s.runErr
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case ack := <-s.ticks:
			p := s.rec.(runner.Prober)
			for _, id := range slices.Sorted(maps.Keys(s.disabled)) {
				if reason := p.Probe(ctx, id); reason == "" {
					delete(s.disabled, id)
				} else {
					s.disabled[id] = reason
				}
			}
			_ = s.rec.Reconcile(ctx, "2026-11-06")
			_ = s.gaps.Sync(ctx, "2026-11-06", maps.Clone(s.disabled))
			close(ack)
		}
	}
}

// schedRig is an app with every constructor faked.
type schedRig struct {
	a        *app
	errb     interface{ String() string }
	inst     *fakeInstFence
	sl       *fakeSchedLedger
	fw       *clockwork.FakeClock
	gaps     *fakeGaps
	notifier *fakeNotifier
	sched    *fakeSched
	cancel   context.CancelFunc
	dials    atomic.Int32
	dialErrs func(n int32) error // the error of the nth dial, nil for success
	mu       sync.Mutex
	builds   [][]string // the job ids of each reconciler constructor call
	reconned []string   // the job ids of the reconciler that reconciled last
	opened   atomic.Int32
	runErr   error
	built    chan struct{} // closed when the scheduler constructor was called

	alertsMu  sync.Mutex
	gapCfg    notify.Config
	alertCfg  notify.Config
	alertDeps notify.AlertDeps
	viewDSN   string
	sentDSN   string
	viewReads atomic.Int32
	sentOpen  atomic.Int32
	sentShut  atomic.Int32
}

func (r *schedRig) lastBuild() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.builds[len(r.builds)-1])
}

func ids(jobs []contracts.Job) []string {
	out := make([]string, 0, len(jobs))
	for _, j := range jobs {
		out = append(out, j.ID())
	}
	slices.Sort(out)
	return out
}

func newSchedRig(t *testing.T, env string) *schedRig {
	t.Helper()
	a, errb := testApp(t, env+telegramEnv+alertsEnv)
	r := &schedRig{
		a: a, errb: errb, inst: &fakeInstFence{}, sl: &fakeSchedLedger{}, fw: clockwork.NewFakeClock(),
		gaps: &fakeGaps{}, notifier: &fakeNotifier{}, built: make(chan struct{}),
	}
	var closed atomic.Int32
	var day contracts.Day
	runOnceFakes(a, &r.opened, &closed, &day)
	home := a.getenv("OT_HOME")
	a.getenv = func(k string) string {
		switch k {
		case "OT_HOME":
			return home
		case healthPortEnv:
			return "0"
		}
		return ""
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	t.Cleanup(cancel)
	d := &a.deps
	d.openFence = fenceOpener(r.inst)
	d.newFwClock = func() clockwork.Clock { return r.fw }
	d.notifyContext = func(context.Context) (context.Context, context.CancelFunc) { return ctx, func() {} }
	d.openSchedulerLedger = func(context.Context, string) (store.SchedulerLedger, error) { r.opened.Add(1); return r.sl, nil }
	d.newFredSeries = func(sources.Source, store.SeriesWriter, clock.Clock) (contracts.Job, error) {
		return idJob{id: jobFredSeries}, nil
	}
	d.newOptionEod = func(broker.Broker, store.InstrumentResolver, store.OptionEodWriter, clock.Clock) (contracts.Job, error) {
		return idJob{id: jobOptionEod}, nil
	}
	d.newQuoteSnapshot = func(broker.Broker, store.InstrumentResolver, store.QuoteSnapshotWriter, clock.Clock) (contracts.Job, error) {
		return idJob{id: jobQuoteSnapshot}, nil
	}
	d.newComputeFigures = func(
		jobs.ComputeStores, compute.Figures, compute.Rules, compute.Units, compute.SeriesOps, compute.Ease, clock.Clock,
	) (contracts.Job, error) {
		return idJob{id: jobComputeFigures}, nil
	}
	d.newCalendarRefresh = func(clock.SessionSource, store.SessionDayStore, clock.SessionDayDiffer, clock.Clock, jobs.CalendarObserver) (contracts.Job, error) {
		return idJob{id: jobCalendarRefresh}, nil
	}
	d.dialBroker = func(context.Context, string, broker.Options) (broker.Broker, error) {
		n := r.dials.Add(1)
		if r.dialErrs != nil {
			if err := r.dialErrs(n); err != nil {
				return nil, err
			}
		}
		return fakeBrk{n: &closed}, nil
	}
	d.newKeyRunner = func(store.OpsLedger, store.SchedulerLedger, clockwork.Clock) runner.KeyRunner { return nil }
	d.newReconciler = func(
		_ store.OpsLedger, _ store.SchedulerLedger, _ clock.Calendar, _ clockwork.Clock, jobs []contracts.Job,
		_ runner.WindowRule, _ runner.KeyRunner,
	) (runner.Reconciler, runner.Backfiller) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.builds = append(r.builds, ids(jobs))
		return fakeRec{ids: ids(jobs), mu: &r.mu, last: &r.reconned}, nil
	}
	d.newNotifier = func(_ notify.GapSource, _ notify.Sender, _ clockwork.Clock, c notify.Config) (notify.Notifier, error) {
		r.alertsMu.Lock()
		r.gapCfg = c
		r.alertsMu.Unlock()
		return r.notifier, nil
	}
	d.alertRules = alertrules.DefaultRules
	d.loadAlertRules = alertrules.Load
	d.evaluateAlerts = alertrules.Evaluate
	d.openSent = func(_ context.Context, dsn string) (store.SentStore, error) {
		r.alertsMu.Lock()
		r.sentDSN = dsn
		r.alertsMu.Unlock()
		r.sentOpen.Add(1)
		return fakeSent{r: r}, nil
	}
	d.openReadingView = func(_ context.Context, dsn string) (store.ReadingViewReader, error) {
		r.alertsMu.Lock()
		r.viewDSN = dsn
		r.alertsMu.Unlock()
		return schedView{r: r}, nil
	}
	d.newAlertNotifier = func(ad notify.AlertDeps, _ notify.Sender, _ clockwork.Clock, c notify.Config) (notify.AlertNotifier, error) {
		r.alertsMu.Lock()
		r.alertCfg, r.alertDeps = c, ad
		r.alertsMu.Unlock()
		return r.notifier, nil
	}
	d.newTelegramSender = func(string, string, notify.TelegramOptions) (notify.Sender, error) { return nil, nil }
	d.newGapSyncer = func(store.OpsLedger, store.SchedulerLedger, clockwork.Clock) runner.GapSyncer { return r.gaps }
	d.newScheduler = func(
		fence store.Fence, _ store.SchedulerLedger, _ store.OpsLedger, _ clock.Calendar, _ clockwork.Clock,
		rec runner.Reconciler, gaps runner.GapSyncer, jobs []contracts.Job, disabled map[string]string, _ runner.ExecIdentityFunc,
	) runner.Scheduler {
		r.sched = &fakeSched{
			fence: fence, rec: rec, gaps: gaps, jobs: jobs, disabled: maps.Clone(disabled), runErr: r.runErr,
			started: make(chan struct{}), ticks: make(chan chan struct{}),
		}
		close(r.built)
		return r.sched
	}
	return r
}

// start runs ot scheduler in a goroutine and returns the channel of its error.
func (r *schedRig) start() <-chan error {
	done := make(chan error, 1)
	go func() { done <- schedulerCmd(r.a, nil) }()
	return done
}

// waitStarted waits for the fake scheduler to run.
func (r *schedRig) waitStarted(t *testing.T, done <-chan error) {
	t.Helper()
	select {
	case err := <-done:
		t.Fatalf("scheduler ended early: %v", err)
	case <-r.built:
	}
	<-r.sched.started
}

// tick asks the fake scheduler for one tick and waits for it.
func (r *schedRig) tick() {
	ack := make(chan struct{})
	r.sched.ticks <- ack
	<-ack
}

func (r *schedRig) stop(t *testing.T, done <-chan error) error {
	t.Helper()
	r.cancel()
	return <-done
}

// jobComputeFigures and jobCalendarRefresh are the ids of the fourth and fifth job; scheduler.go and preflight.go
// never name them.
const (
	jobComputeFigures  = "compute_figures"
	jobCalendarRefresh = "calendar_refresh"
)

const noKeyEnv = "OT_PG_DSN_RUNNER=run-dsn\nOT_PG_DSN_SERIES_FRED=ser-dsn\n" +
	"OT_PG_DSN_OPTION_EOD=eod-dsn\nOT_PG_DSN_QUOTE_SNAPSHOT=snap-dsn\nOT_PG_DSN_MASTER=master-dsn\n" +
	"OT_PG_DSN_PAGES=pages-dsn\nOT_PG_DSN_READINGS=readings-dsn\nOT_PG_DSN_LOADER=loader-dsn\n"

func TestSchedulerLocksOnce(t *testing.T) {
	t.Parallel()
	r := newSchedRig(t, fullEnv)
	done := r.start()
	r.waitStarted(t, done)
	if err := r.stop(t, done); err != nil {
		t.Fatal(err)
	}
	if r.inst.count() != 1 {
		t.Fatalf("TryLock called %d times", r.inst.count())
	}
	if r.sched.fence != store.Fence(r.inst.fence) {
		t.Fatal("the scheduler did not get the held fence")
	}
	if b, _ := r.sl.build.Load().(string); b != "dev" || r.inst.build != "dev" {
		t.Fatalf("build %q, lock build %q", b, r.inst.build)
	}
	if got := ids(r.sched.jobs); !slices.Equal(got, []string{jobCalendarRefresh, jobComputeFigures, jobFredSeries, jobOptionEod, jobQuoteSnapshot}) {
		t.Fatalf("jobs %v", got)
	}
}

func TestSchedulerExitsNonZeroWhenFenceHeld(t *testing.T) {
	t.Parallel()
	r := newSchedRig(t, fullEnv)
	r.inst.held = true
	done := r.start()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go func() {
		for r.fw.BlockUntilContext(ctx, 1) == nil {
			r.fw.Advance(lockEvery)
		}
	}()
	err := <-done
	var ee *exitError
	if err == nil || errors.As(err, &ee) {
		t.Fatalf("err %v, want a plain error (exit 1)", err)
	}
	if r.inst.count() != 13 || r.sched != nil || r.opened.Load() != 0 {
		t.Fatalf("tries %d, scheduler built %v, opened %d", r.inst.count(), r.sched != nil, r.opened.Load())
	}
}

func TestSchedulerDrainsOnTerm(t *testing.T) {
	t.Parallel()
	r := newSchedRig(t, fullEnv)
	done := r.start()
	r.waitStarted(t, done)
	if err := r.stop(t, done); err != nil {
		t.Fatalf("drain returned %v", err)
	}
	if r.inst.fence.closed.Load() < 1 {
		t.Fatal("the fence was not closed")
	}
}

func TestSchedulerExitsZeroAfterDrain(t *testing.T) {
	t.Parallel()
	r := newSchedRig(t, fullEnv)
	done := r.start()
	r.waitStarted(t, done)
	err := r.stop(t, done)
	var ee *exitError
	if err != nil || errors.As(err, &ee) {
		t.Fatalf("err %v", err)
	}
	if code := exitCodeOf(err); code != 0 {
		t.Fatalf("code %d", code)
	}
}

// exitCodeOf is what run does with the error of a command.
func exitCodeOf(err error) int {
	var ee *exitError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &ee):
		return ee.code
	}
	return 1
}

func TestSchedulerExitsSeventyFiveOnReplacement(t *testing.T) {
	t.Parallel()
	r := newSchedRig(t, fullEnv)
	r.runErr = sched.ErrExecutableChanged
	err := <-r.start()
	if exitCodeOf(err) != 75 {
		t.Fatalf("err %v, code %d", err, exitCodeOf(err))
	}
}

func TestSchedulerExitsNonZeroOnWatchdog(t *testing.T) {
	t.Parallel()
	for _, e := range []error{sched.ErrLoopFrozen, sched.ErrFenceLost} {
		r := newSchedRig(t, fullEnv)
		r.runErr = e
		err := <-r.start()
		if err == nil || exitCodeOf(err) != 1 {
			t.Fatalf("%v: err %v, code %d", e, err, exitCodeOf(err))
		}
	}
}

func TestSchedulerRefusesDemoProfile(t *testing.T) {
	t.Parallel()
	r := newSchedRig(t, fullEnv)
	r.a.profile.Name = "demo"
	err := schedulerCmd(r.a, nil)
	if err == nil || !strings.Contains(err.Error(), "demo profile") || r.inst.count() != 0 || r.opened.Load() != 0 {
		t.Fatalf("err %v tries %d opened %d", err, r.inst.count(), r.opened.Load())
	}
}

func TestSchedulerErrorsHoldNoSecret(t *testing.T) {
	t.Parallel()
	const secret = "postgres://sentinel-user:sentinel-pw@h/db"
	env := "OT_PG_DSN_RUNNER=" + secret + "\nOT_PG_DSN_SERIES_FRED=s\nFRED_API_KEY=sentinel-key\n" +
		"OT_PG_DSN_OPTION_EOD=e\nOT_PG_DSN_QUOTE_SNAPSHOT=q\nOT_PG_DSN_MASTER=m\nOT_PG_DSN_PAGES=p\nOT_PG_DSN_READINGS=rd\nOT_PG_DSN_LOADER=ld\n"
	steps := map[string]func(r *schedRig){
		"open fence": func(r *schedRig) {
			r.a.deps.openFence = func(_ context.Context, dsn string) (store.InstanceFence, error) {
				return nil, errors.New("dial " + dsn)
			}
		},
		"open ledger": func(r *schedRig) {
			r.a.deps.openLedger = func(_ context.Context, dsn string) (store.OpsLedger, error) { return nil, errors.New("dial " + dsn) }
		},
		"scheduler ledger": func(r *schedRig) {
			r.a.deps.openSchedulerLedger = func(_ context.Context, dsn string) (store.SchedulerLedger, error) {
				return nil, errors.New("dial " + dsn)
			}
		},
		"set state": func(r *schedRig) { r.sl.err = errors.New("write " + secret) },
		"run":       func(r *schedRig) { r.runErr = errors.New("run " + secret + " sentinel-key") },
	}
	for name, setup := range steps {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := newSchedRig(t, env)
			setup(r)
			err := <-r.start()
			if err == nil {
				t.Fatal("no error")
			}
			if strings.Contains(err.Error(), "sentinel") || strings.Contains(r.errb.String(), "sentinel") {
				t.Fatalf("leak: %v / %q", err, r.errb.String())
			}
		})
	}
}

func TestSchedulerPassesGapSyncer(t *testing.T) {
	t.Parallel()
	r := newSchedRig(t, fullEnv)
	done := r.start()
	r.waitStarted(t, done)
	r.tick()
	// The scheduler gets the gap syncer wrapped by the notify pass (#550); the wrapper holds it as it is.
	if w, ok := r.sched.gaps.(notifyingGaps); !ok || w.inner != runner.GapSyncer(r.gaps) {
		t.Fatal("the scheduler did not get the gap syncer")
	}
	if len(r.gaps.seen) != 1 || len(r.gaps.seen[0]) != 0 {
		t.Fatalf("sync calls %v", r.gaps.seen)
	}
	_ = r.stop(t, done)
}

func TestSchedulerDisablesFredWithoutKey(t *testing.T) {
	t.Parallel()
	r := newSchedRig(t, noKeyEnv)
	done := r.start()
	r.waitStarted(t, done)
	if got := r.lastBuild(); !slices.Equal(got, []string{jobCalendarRefresh, jobComputeFigures, jobOptionEod, jobQuoteSnapshot}) {
		t.Fatalf("reconciler jobs %v", got)
	}
	if got := ids(r.sched.jobs); !slices.Equal(got, []string{jobCalendarRefresh, jobComputeFigures, jobFredSeries, jobOptionEod, jobQuoteSnapshot}) {
		t.Fatalf("scheduler jobs %v", got)
	}
	if len(r.sched.disabled) != 1 || r.sched.disabled[jobFredSeries] == "" {
		t.Fatalf("disabled %v", r.sched.disabled)
	}
	_ = r.stop(t, done)
	if n := strings.Count(r.errb.String(), "job left out"); n != 1 || !strings.Contains(r.errb.String(), jobFredSeries) {
		t.Fatalf("log %q", r.errb.String())
	}
}

func TestSchedulerDisablesBrokerJobsOnDialFailure(t *testing.T) {
	t.Parallel()
	r := newSchedRig(t, fullEnv)
	r.dialErrs = func(int32) error { return errors.New("no socket") }
	done := r.start()
	r.waitStarted(t, done)
	if got := r.lastBuild(); !slices.Equal(got, []string{jobCalendarRefresh, jobComputeFigures, jobFredSeries}) {
		t.Fatalf("reconciler jobs %v", got)
	}
	if got := ids(r.sched.jobs); len(got) != 5 {
		t.Fatalf("scheduler jobs %v", got)
	}
	d := r.sched.disabled
	if len(d) != 2 || d[jobOptionEod] == "" || d[jobQuoteSnapshot] == "" || d[jobFredSeries] != "" {
		t.Fatalf("disabled %v", d)
	}
	if err := r.stop(t, done); err != nil {
		t.Fatalf("the scheduler did not keep running: %v", err)
	}
}

func TestSchedulerReprobesBrokerJobsEachTick(t *testing.T) {
	t.Parallel()
	r := newSchedRig(t, fullEnv)
	r.dialErrs = func(n int32) error {
		if n == 1 {
			return errors.New("no socket")
		}
		return nil
	}
	done := r.start()
	r.waitStarted(t, done)
	if d := r.sched.disabled; len(d) != 2 {
		t.Fatalf("disabled after start %v", d)
	}
	r.tick()
	if r.dials.Load() != 2 {
		t.Fatalf("dials %d, want 2 (start and one probe)", r.dials.Load())
	}
	if len(r.sched.disabled) != 0 {
		t.Fatalf("disabled after the tick %v", r.sched.disabled)
	}
	if got := r.lastBuild(); !slices.Equal(got, []string{jobCalendarRefresh, jobComputeFigures, jobFredSeries, jobOptionEod, jobQuoteSnapshot}) {
		t.Fatalf("reconciler jobs %v", got)
	}
	r.mu.Lock()
	ran := slices.Clone(r.reconned)
	r.mu.Unlock()
	if !slices.Equal(ran, []string{jobCalendarRefresh, jobComputeFigures, jobFredSeries, jobOptionEod, jobQuoteSnapshot}) {
		t.Fatalf("the swapped reconciler did not run the keys: %v", ran)
	}
	if last := r.gaps.seen[len(r.gaps.seen)-1]; len(last) != 0 {
		t.Fatalf("gap syncer got %v", last)
	}
	r.tick()
	if r.dials.Load() != 2 {
		t.Fatalf("a second tick dialed: %d", r.dials.Load())
	}
	_ = r.stop(t, done)
	if n := strings.Count(r.errb.String(), "job back in the schedule"); n != 2 {
		t.Fatalf("re-enabled lines %d: %q", n, r.errb.String())
	}
}

func TestSchedulerFredStaysDisabledUntilRestart(t *testing.T) {
	t.Parallel()
	r := newSchedRig(t, noKeyEnv)
	done := r.start()
	r.waitStarted(t, done)
	first := r.sched.disabled[jobFredSeries]
	for range 3 {
		r.tick()
		if got := r.sched.disabled[jobFredSeries]; got != first || got == "" {
			t.Fatalf("reason %q, first %q", got, first)
		}
		if got := r.lastBuild(); !slices.Equal(got, []string{jobCalendarRefresh, jobComputeFigures, jobOptionEod, jobQuoteSnapshot}) {
			t.Fatalf("reconciler jobs %v", got)
		}
		if len(r.sched.disabled) != 1 {
			t.Fatalf("disabled %v", r.sched.disabled)
		}
	}
	_ = r.stop(t, done)
}

func TestSchedulerDoesNotStartTheSite(t *testing.T) {
	t.Parallel()
	r := newSchedRig(t, fullEnv)
	var called atomic.Int32
	d := &r.a.deps
	d.pageList = nil
	d.newHandler = func([]byte) http.Handler { called.Add(1); return nil }
	d.macroPage = func(*time.Location, string, compute.ReadingFunc) pages.Page { called.Add(1); return pages.Page{} }
	d.optionDataPage = func(*time.Location, string, sitePageInput) pages.Page { called.Add(1); return pages.Page{} }
	done := r.start()
	r.waitStarted(t, done)
	_ = r.stop(t, done)
	if called.Load() != 0 {
		t.Fatalf("a site constructor was called %d times", called.Load())
	}
}

func TestSchedulerPortInvalid(t *testing.T) {
	t.Parallel()
	for _, v := range []string{"abc", "70000", "-1"} {
		if _, err := healthAddr(func(string) string { return v }); err == nil {
			t.Errorf("%q accepted", v)
		}
	}
}

func TestSchedulerDispatchesStatusAndCanRestart(t *testing.T) {
	t.Parallel()
	st := store.SchedulerStatus{Build: "dispatch-build", Gaps: []store.Gap{{DatasetID: "d", Reason: "expired", Day: "2026-11-06"}}}
	r := newStatusRig(t, st)
	if code := exitCodeOf(schedulerCmd(r.a, []string{"status"})); code != 1 || !strings.Contains(r.out.String(), "dispatch-build") {
		t.Fatalf("status: exit %d, output %q", code, r.out.String())
	}
	r = newStatusRig(t, store.SchedulerStatus{Runs: []store.Run{{ID: "busy", Status: store.RunRunning}}})
	if code := exitCodeOf(schedulerCmd(r.a, []string{"can-restart"})); code != 1 {
		t.Fatalf("can-restart: exit %d, want 1", code)
	}
	// Any other argument keeps the usage error and opens nothing.
	if err := schedulerCmd(r.a, []string{"bogus"}); err == nil || !strings.Contains(err.Error(), "usage: ot scheduler") {
		t.Fatalf("bogus argument: %v", err)
	}
}

// telegramEnv holds the two synthetic Telegram lines the scheduler needs; they are not real.
const telegramEnv = "OT_TELEGRAM_TOKEN=test-token\nOT_TELEGRAM_CHAT_ID=1000001\n"

// fakeNotifier records its passes in order with the gap passes; it can fail or panic.
type fakeNotifier struct {
	mu     sync.Mutex
	passes int
	err    error
	panics bool
	gaps   *fakeGaps
	synced []int // the number of gap passes seen at each notify pass

	order       []string // "gaps" and "alerts" in the order of the calls
	alertPasses int
	alertErr    error
	alertPanics bool
}

func (f *fakeNotifier) NotifyAlerts(context.Context) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.alertPasses++
	f.order = append(f.order, "alerts")
	if f.alertPanics {
		panic("boom")
	}
	return 0, f.alertErr
}

func (f *fakeNotifier) NotifyGaps(context.Context) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.passes++
	f.order = append(f.order, "gaps")
	if f.gaps != nil {
		f.gaps.mu.Lock()
		f.synced = append(f.synced, len(f.gaps.seen))
		f.gaps.mu.Unlock()
	}
	if f.panics {
		panic("boom")
	}
	return 0, f.err
}

func TestSchedulerCallsNotifyAfterGapPass(t *testing.T) {
	t.Parallel()
	r := newSchedRig(t, fullEnv)
	r.notifier.gaps = r.gaps
	done := r.start()
	r.waitStarted(t, done)
	r.tick()
	r.tick()
	_ = r.stop(t, done)
	if got := r.notifier.synced; !slices.Equal(got, []int{1, 2}) {
		t.Fatalf("gap passes seen at each notify pass %v, want [1 2]", got)
	}
}

func TestSchedulerSurvivesNotifyError(t *testing.T) {
	t.Parallel()
	for name, set := range map[string]func(*fakeNotifier){
		"error": func(n *fakeNotifier) { n.err = errors.New("send to https://api.example/bot-secret-token failed") },
		"panic": func(n *fakeNotifier) { n.panics = true },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := newSchedRig(t, fullEnv)
			set(r.notifier)
			done := r.start()
			r.waitStarted(t, done)
			r.tick()
			r.tick()
			if err := r.stop(t, done); err != nil {
				t.Fatalf("the scheduler stopped: %v", err)
			}
			if r.notifier.passes != 2 || len(r.gaps.seen) != 2 {
				t.Fatalf("notify passes %d, gap passes %d, want 2 and 2", r.notifier.passes, len(r.gaps.seen))
			}
			if strings.Contains(r.errb.String(), "secret") || !strings.Contains(r.errb.String(), "notify") {
				t.Fatalf("log %q", r.errb.String())
			}
		})
	}
}

func TestSchedulerNeedsTelegramLines(t *testing.T) {
	t.Parallel()
	r := newSchedRig(t, fullEnv)
	if err := writeFile(r.a.getenv("OT_HOME")+"/.env", fullEnv+"OT_TELEGRAM_TOKEN=test-token\n"); err != nil {
		t.Fatal(err)
	}
	err := <-r.start()
	if err == nil || !strings.Contains(err.Error(), "OT_TELEGRAM_CHAT_ID") || strings.Contains(err.Error(), "test-token") {
		t.Fatalf("err %v", err)
	}
	if r.inst.count() != 0 {
		t.Fatal("the lock was tried before the missing line was reported")
	}
}

func TestSchedulerWiresFiveJobs(t *testing.T) {
	t.Parallel()
	r := newSchedRig(t, fullEnv)
	done := r.start()
	r.waitStarted(t, done)
	want := []string{jobCalendarRefresh, jobComputeFigures, jobFredSeries, jobOptionEod, jobQuoteSnapshot}
	if got := ids(r.sched.jobs); !slices.Equal(got, want) {
		t.Fatalf("scheduler jobs %v, want %v", got, want)
	}
	if got := r.lastBuild(); !slices.Equal(got, want) {
		t.Fatalf("reconciler jobs %v, want %v", got, want)
	}
	if len(r.sched.disabled) != 0 {
		t.Fatalf("disabled %v", r.sched.disabled)
	}
	_ = r.stop(t, done)
}

func TestSchedulerNeedsReadingsDSN(t *testing.T) {
	t.Parallel()
	env := strings.Replace(fullEnv, "OT_PG_DSN_READINGS=readings-dsn\n", "", 1)
	r := newSchedRig(t, env)
	done := r.start()
	err := <-done
	if err == nil || !strings.Contains(err.Error(), store.DSNEnvReadings) {
		t.Fatalf("err %v", err)
	}
}

func TestSchedulerNeedsLoaderDSN(t *testing.T) {
	t.Parallel()
	env := strings.Replace(fullEnv, "OT_PG_DSN_LOADER=loader-dsn\n", "", 1)
	r := newSchedRig(t, env)
	err := <-r.start()
	if err == nil || !strings.Contains(err.Error(), store.DSNEnvLoader) || strings.Contains(err.Error(), "loader-dsn") {
		t.Fatalf("err %v", err)
	}
}
