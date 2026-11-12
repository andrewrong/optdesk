package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/jobs/calendarrefresh"
	"github.com/andrewrong/optdesk/internal/jobs/computefigures"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/symbol"
)

// onceBackfiller is a Backfiller that is also a ForceBackfiller; it records every BackfillWith call.
type onceBackfiller struct {
	mu       sync.Mutex
	days     []contracts.Day
	datasets [][]string
	opts     []runner.BackfillOptions
	res      runner.BackfillResult
	err      error
}

func (f *onceBackfiller) Backfill(context.Context, contracts.Day, []string) (runner.BackfillResult, error) {
	return runner.BackfillResult{}, errors.New("Backfill must not be used")
}

func (f *onceBackfiller) BackfillWith(_ context.Context, day contracts.Day, datasets []string, opt runner.BackfillOptions) (runner.BackfillResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.days = append(f.days, day)
	f.datasets = append(f.datasets, datasets)
	f.opts = append(f.opts, opt)
	res := f.res
	res.Day = day
	return res, f.err
}

// countLedger counts Close calls of the scheduler ledger.
type countLedger struct {
	store.SchedulerLedger
	n *atomic.Int32
}

func (l countLedger) Close() error { l.n.Add(1); return nil }

// onceJob is a job with one planned key per dataset.
type onceJob struct {
	contracts.Job
	id       string
	datasets []string
}

func (j *onceJob) ID() string { return j.id }
func (j *onceJob) Plan(context.Context, contracts.Day) ([]contracts.ExpectedKey, string, error) {
	var ks []contracts.ExpectedKey
	for _, d := range j.datasets {
		ks = append(ks, contracts.ExpectedKey{DatasetID: d, SubjectKey: "s"})
	}
	return ks, "h", nil
}

// onceNamed is a job with the id that plans one key in the dataset d.<id>.
func onceNamed(id string) contracts.Job { return &onceJob{id: id, datasets: []string{"d." + id}} }

func onceJobDeps(a *app, f *onceBackfiller, g *fakeKeySyncer, ledgerClosed *atomic.Int32) {
	fakeBackfillDeps(a, nil, g)
	a.deps.openSchedulerLedger = func(context.Context, string) (store.SchedulerLedger, error) {
		return countLedger{n: ledgerClosed}, nil
	}
	a.deps.newReconciler = func(
		store.OpsLedger, store.SchedulerLedger, clock.Calendar, clockwork.Clock, []contracts.Job, runner.WindowRule, runner.KeyRunner,
	) (runner.Reconciler, runner.Backfiller) {
		return nil, f
	}
}

// onceFredApp is a real-profile app whose wiring holds one job, fred_series, planning dataset series.fred.
func onceFredApp(t *testing.T, f *onceBackfiller) (a *app, out *bytes.Buffer, syncer *fakeKeySyncer, opened, closed, ledgerClosed *atomic.Int32) {
	t.Helper()
	return onceAppEnv(t, f, fullEnv)
}

// onceAppEnv is onceFredApp over the env body; every constructor of the deps the real wiring calls returns a job that
// carries the id of its family and plans one key.
func onceAppEnv(t *testing.T, f *onceBackfiller, env string) (a *app, out *bytes.Buffer, syncer *fakeKeySyncer, opened, closed, ledgerClosed *atomic.Int32) {
	t.Helper()
	a, _ = testApp(t, env)
	opened, closed, ledgerClosed = new(atomic.Int32), new(atomic.Int32), new(atomic.Int32)
	var day contracts.Day
	runOnceFakes(a, opened, closed, &day)
	a.deps.newFRED = func(sources.FREDConfig) (sources.Source, error) { return nil, nil }
	a.deps.newFredSeries = func(sources.Source, store.SeriesWriter, clock.Clock) (contracts.Job, error) {
		return &onceJob{id: "fred_series", datasets: []string{"series.fred"}}, nil
	}
	named := onceNamed
	a.deps.newOptionEod = func(broker.Broker, store.InstrumentResolver, store.OptionEodWriter, clock.Clock, symbol.Translator) (contracts.Job, error) {
		return named(jobOptionEod), nil
	}
	a.deps.newOptionEodJP = func(broker.Broker, store.InstrumentResolver, store.OptionEodWriter, clock.Clock) (contracts.Job, error) {
		return named(jobOptionEodJP), nil
	}
	a.deps.newQuoteSnapshot = func(broker.Broker, store.InstrumentResolver, store.QuoteSnapshotWriter, clock.Clock) (contracts.Job, error) {
		return named(jobQuoteSnapshot), nil
	}
	a.deps.newComputeFigures = func(
		computefigures.ComputeStores, compute.Figures, compute.Rules, compute.Units, compute.SeriesOps, compute.Ease, clock.Clock,
	) (contracts.Job, error) {
		return named("compute_figures"), nil
	}
	a.deps.newCalendarRefresh = func(clock.SessionSource, store.SessionDayStore, clock.SessionDayDiffer, clock.Clock,
		calendarrefresh.CalendarObserver) (contracts.Job, error) {
		return named("calendar_refresh"), nil
	}
	a.deps.newEarningsRefresh = func(map[string]sources.EarningsSource, store.EarningsDateWriter, store.EarningsDateReader,
		[]jobs.WatchEntry, clock.Clock, symbol.Translator) (contracts.Job, error) {
		return named(jobEarningsRefresh), nil
	}
	syncer = &fakeKeySyncer{}
	onceJobDeps(a, f, syncer, ledgerClosed)
	out = &bytes.Buffer{}
	a.stdout = out
	return a, out, syncer, opened, closed, ledgerClosed
}

func onceWiredApp(t *testing.T, f *onceBackfiller) *app {
	t.Helper()
	a, _ := testApp(t, fullEnv)
	onceJobDeps(a, f, &fakeKeySyncer{}, new(atomic.Int32))
	a.stdout = &bytes.Buffer{}
	return a
}

// onceFakeWiring replaces the real wiring: use gets a wiring with one job per id of jobTable, each planning one key.
func onceFakeWiring() func(context.Context, *app, clock.Clock, runOnceConfig, func(store.OpsLedger) error, func(wiring) error) error {
	return func(_ context.Context, _ *app, _ clock.Clock, _ runOnceConfig, _ func(store.OpsLedger) error, use func(wiring) error) error {
		w := wiring{held: map[string]contracts.Job{}, off: map[string]string{}}
		for _, row := range jobTable {
			w.jobs = append(w.jobs, onceNamed(row.ID))
		}
		return use(w)
	}
}

func TestRunOnceJobAnyJobName(t *testing.T) {
	t.Parallel()
	if len(jobTable) != 24 {
		t.Fatalf("jobTable holds %d rows, want 24", len(jobTable))
	}
	for _, row := range jobTable {
		t.Run(row.ID, func(t *testing.T) {
			t.Parallel()
			f := &onceBackfiller{}
			a, _, _, _, _, _ := onceFredApp(t, f)
			a.deps.runWiring = onceFakeWiring()
			if err := runOnce(a, []string{"--job", row.ID, "--day", "2026-11-04"}); err != nil {
				t.Fatal(err)
			}
			if len(f.opts) != 1 || f.opts[0].Job != row.ID {
				t.Fatalf("opts %+v", f.opts)
			}
			if len(f.datasets) != 1 || len(f.datasets[0]) != 1 || f.datasets[0][0] != "d."+row.ID {
				t.Fatalf("datasets %v, want only those of %s", f.datasets, row.ID)
			}
		})
	}
}

func TestRunOnceJobRunsOnlyIt(t *testing.T) {
	t.Parallel()
	f := &onceBackfiller{}
	a := onceWiredApp(t, f)
	w := wiring{jobs: []contracts.Job{
		&onceJob{id: "fred_series", datasets: []string{"series.fred"}},
		&onceJob{id: "fundamentals", datasets: []string{"series.fund", "series.fund2"}},
	}, held: map[string]contracts.Job{}, off: map[string]string{}}
	if err := runWiredJob(context.Background(), a, w, &fakeSchedLedger{}, clockwork.NewFakeClock(), "2026-11-04", "fundamentals", false); err != nil {
		t.Fatal(err)
	}
	if len(f.datasets) != 1 || strings.Join(f.datasets[0], ",") != "series.fund,series.fund2" {
		t.Fatalf("datasets %v", f.datasets)
	}
}

func TestRunOnceJobUnknownName(t *testing.T) {
	t.Parallel()
	f := &onceBackfiller{}
	a, _, _, opened, _, ledgerClosed := onceFredApp(t, f)
	err := runOnce(a, []string{"--job", "nope", "--day", "2026-11-04"})
	if err == nil || !strings.Contains(err.Error(), "unknown job nope") || !strings.Contains(err.Error(), "fred_series") {
		t.Fatalf("err %v", err)
	}
	if opened.Load() != 0 || ledgerClosed.Load() != 0 || len(f.days) != 0 {
		t.Fatalf("opened %d, ledger closed %d, calls %d", opened.Load(), ledgerClosed.Load(), len(f.days))
	}
}

func TestRunOnceJobLeftOut(t *testing.T) {
	t.Parallel()
	f := &onceBackfiller{}
	a := onceWiredApp(t, f)
	j := &onceJob{id: "fred_series", datasets: []string{"d"}}
	w := wiring{jobs: []contracts.Job{j}, held: map[string]contracts.Job{"fred_series": j}, off: map[string]string{"fred_series": "no FRED key"}}
	err := runWiredJob(context.Background(), a, w, &fakeSchedLedger{}, clockwork.NewFakeClock(), "2026-11-04", "fred_series", false)
	if err == nil || !strings.Contains(err.Error(), "job fred_series is left out: no FRED key") {
		t.Fatalf("err %v", err)
	}
	err = runWiredJob(context.Background(), a, w, &fakeSchedLedger{}, clockwork.NewFakeClock(), "2026-11-04", "fundamentals", false)
	if err == nil || !strings.Contains(err.Error(), "job fundamentals is not wired") {
		t.Fatalf("err %v", err)
	}
	if len(f.days) != 0 {
		t.Fatal("BackfillWith called")
	}
}

type onceLeaseErr struct{}

func (onceLeaseErr) Error() string             { return "secret-fake-error-text" }
func (onceLeaseErr) Kind() contracts.ErrorKind { return contracts.Retryable }

func TestRunOnceJobLeaseHeld(t *testing.T) {
	t.Parallel()
	f := &onceBackfiller{err: onceLeaseErr{}}
	a, _, _, _, _, _ := onceFredApp(t, f)
	err := runOnce(a, []string{"--job", "fred_series", "--day", "2026-11-04"})
	if err == nil || !strings.Contains(err.Error(), "the runner lease is held by another run") || strings.Contains(err.Error(), "secret-fake") {
		t.Fatalf("err %v", err)
	}
	f = &onceBackfiller{err: errors.New("secret-fake-error-text")}
	a, _, _, _, _, _ = onceFredApp(t, f)
	err = runOnce(a, []string{"--job", "fred_series", "--day", "2026-11-04"})
	if err == nil || strings.Contains(err.Error(), "secret-fake") {
		t.Fatalf("err %v", err)
	}
}

func TestRunOnceJobForce(t *testing.T) {
	t.Parallel()
	for _, force := range []bool{false, true} {
		f := &onceBackfiller{}
		a, _, _, _, _, _ := onceFredApp(t, f)
		args := []string{"--job", "fred_series", "--day", "2026-11-04"}
		if force {
			args = append(args, "--force")
		}
		if err := runOnce(a, args); err != nil {
			t.Fatal(err)
		}
		if len(f.opts) != 1 || f.opts[0].Force != force || f.opts[0].Job != "fred_series" {
			t.Fatalf("force=%v options %+v", force, f.opts)
		}
	}
	a, _, _, _, _, _ := onceFredApp(t, &onceBackfiller{})
	if err := runOnce(a, []string{"--force"}); err == nil {
		t.Fatal("--force without --job accepted")
	}
}

func TestRunOnceJobNoPlan(t *testing.T) {
	t.Parallel()
	f := &onceBackfiller{res: runner.BackfillResult{Refused: runner.RefusedNoPlan}}
	a, out, _, _, _, _ := onceFredApp(t, f)
	err := runOnce(a, []string{"--job", "fred_series", "--day", "2026-11-04"})
	if err == nil || !strings.Contains(err.Error(), "2026-11-04") || !strings.Contains(err.Error(), "no plan") {
		t.Fatalf("err %v", err)
	}
	if !strings.Contains(out.String(), "status=refused keys=0") {
		t.Fatalf("out %q", out.String())
	}
}

func TestRunOnceJobClosesLedger(t *testing.T) {
	t.Parallel()
	f := &onceBackfiller{res: runner.BackfillResult{Keys: []runner.KeyRun{
		{Key: contracts.ExpectedKey{DatasetID: "series.fred", SubjectKey: "s"}, Group: runner.GroupRan, Status: "ok"},
	}}}
	a, out, syncer, _, _, ledgerClosed := onceFredApp(t, f)
	if err := runOnce(a, []string{"--job", "fred_series", "--day", "2026-11-04"}); err != nil {
		t.Fatal(err)
	}
	if ledgerClosed.Load() != 1 {
		t.Fatalf("closed %d on success", ledgerClosed.Load())
	}
	if len(syncer.days) != 1 {
		t.Fatalf("sync calls %v", syncer.days)
	}
	want := "run-once job=fred_series day=2026-11-04 dataset=series.fred subject=s group=ran status=ok reason= error=\n" +
		"run-once 2026-11-04 job=fred_series status=ok keys=1\n"
	if out.String() != want {
		t.Fatalf("out %q", out.String())
	}
	f = &onceBackfiller{res: runner.BackfillResult{Refused: runner.RefusedNoPlan}}
	a, _, _, _, _, ledgerClosed = onceFredApp(t, f)
	if err := runOnce(a, []string{"--job", "fred_series", "--day", "2026-11-04"}); err == nil {
		t.Fatal("want a refusal")
	}
	if ledgerClosed.Load() != 1 {
		t.Fatalf("closed %d on refusal", ledgerClosed.Load())
	}
}

func TestRunOnceJobKeyErrorFails(t *testing.T) {
	t.Parallel()
	f := &onceBackfiller{res: runner.BackfillResult{Keys: []runner.KeyRun{
		{Key: contracts.ExpectedKey{DatasetID: "series.fred", SubjectKey: "s"}, Group: runner.GroupRan, Status: "failed", ErrorCode: "permanent"},
	}}}
	a, out, _, _, _, _ := onceFredApp(t, f)
	if err := runOnce(a, []string{"--job", "fred_series", "--day", "2026-11-04"}); err == nil {
		t.Fatal("want an error")
	}
	if !strings.Contains(out.String(), "status=failed keys=1") || !strings.Contains(out.String(), "error=permanent") {
		t.Fatalf("out %q", out.String())
	}
}
