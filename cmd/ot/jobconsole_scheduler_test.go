package main

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/jobrequest"
	"github.com/jonboulle/clockwork"
)

// orderedSched wraps the scheduler the rig builds and records when Run is entered.
type orderedSched struct {
	runner.Scheduler
	log *jcLog
}

func (s orderedSched) Run(ctx context.Context) error {
	s.log.add("run")
	return s.Scheduler.Run(ctx)
}

// plainBackfill is a Backfiller that is also a ForceBackfiller, as the reconciler is.
type plainBackfill struct{ *jcBackfill }

func (plainBackfill) Backfill(context.Context, contracts.Day, []string) (runner.BackfillResult, error) {
	return runner.BackfillResult{}, nil
}

type noopRec struct{}

func (noopRec) Reconcile(context.Context, contracts.Day) error { return nil }

// consoleSchedRig is a scheduler rig whose deps hold the fake queue and a ForceBackfiller; dsn is the DSN the
// consumer was opened with.
func consoleSchedRig(t *testing.T, q *jcQueue, openErr error) (*schedRig, *string) {
	t.Helper()
	r := newSchedRig(t, fullEnv)
	d := &r.a.deps
	var dsn string
	d.ext = map[string]any{jobConsoleExtKey: jobRequestOpeners{
		Consumer: func(_ context.Context, got string) (jobrequest.Consumer, error) {
			dsn = got
			if openErr != nil {
				return nil, openErr
			}
			return q, nil
		},
	}}
	d.newKeyGapSyncer = func(store.OpsLedger, store.SchedulerLedger, clockwork.Clock) runner.KeyGapSyncer { return q.log }
	bf := &jcBackfill{log: q.log}
	base := d.newReconciler
	d.newReconciler = func(
		o store.OpsLedger, s store.SchedulerLedger, c clock.Calendar, k clockwork.Clock, jobs []contracts.Job, w runner.WindowRule, kr runner.KeyRunner,
	) (runner.Reconciler, runner.Backfiller) {
		rec, _ := base(o, s, c, k, jobs, w, kr)
		return rec, plainBackfill{bf}
	}
	newSched := d.newScheduler
	d.newScheduler = func(
		f store.Fence, sl store.SchedulerLedger, o store.OpsLedger, c clock.Calendar, k clockwork.Clock, rec runner.Reconciler,
		g runner.GapSyncer, jobs []contracts.Job, dis map[string]string, e runner.ExecIdentityFunc,
	) runner.Scheduler {
		return orderedSched{Scheduler: newSched(f, sl, o, c, k, rec, g, jobs, dis, e), log: q.log}
	}
	return r, &dsn
}

func TestBackfillerFromBuildIsForceBackfiller(t *testing.T) {
	t.Parallel()
	_, bf := newDeps().newReconciler(nil, nil, nil, clockwork.NewFakeClock(), nil, runner.WindowRule{}, nil)
	if _, ok := bf.(runner.ForceBackfiller); !ok {
		t.Fatalf("%T is no runner.ForceBackfiller", bf)
	}
}

func TestStartRunsBeforeTicker(t *testing.T) {
	t.Parallel()
	q := &jcQueue{log: &jcLog{}}
	r, dsn := consoleSchedRig(t, q, nil)
	done := r.start()
	r.waitStarted(t, done)
	if err := r.stop(t, done); err != nil {
		t.Fatal(err)
	}
	if *dsn != "run-dsn" {
		t.Fatalf("the consumer opened with %q, not the runner DSN", *dsn)
	}
	fail, run := -1, -1
	for i, s := range q.log.items {
		switch {
		case strings.HasPrefix(s, "fail_running:restart:") && fail < 0:
			fail = i
		case s == "run":
			run = i
		}
	}
	if fail < 0 || run < 0 || fail > run {
		t.Fatalf("calls %v: a running row must be failed with reason restart before the ticker runs", q.log.items)
	}
}

func TestHostSeesJobBroughtBackByProbe(t *testing.T) {
	t.Parallel()
	r := newJCRig(t)
	probes := 0
	build := func(jobs []contracts.Job) runner.Reconciler {
		r.host.set(jobs, r.bf)
		return noopRec{}
	}
	back := jcJob{id: "calendar_refresh", keys: jcKey}
	probe := func(_ context.Context, id string) (contracts.Job, string) {
		probes++
		return back, ""
	}
	swap := newSwapReconciler([]contracts.Job{jcJob{id: "fred_series", keys: jcKey}}, build, probe, slog.New(slog.DiscardHandler))
	rec := &consoleReconciler{sw: swap, host: r.host}
	r.add(1, "calendar_refresh", jcDay, false)
	if !r.turnOf(rec) {
		t.Fatalf("before the probe: %v", r.log.items)
	}
	if r.wantEnd(jobrequest.StateRefused, "left_out"); r.log.count("backfill") != 0 {
		t.Fatalf("before the probe: %v", r.log.items)
	}
	if reason := rec.Probe(t.Context(), "calendar_refresh"); reason != "" || probes != 1 {
		t.Fatalf("probe %q (%d)", reason, probes)
	}
	r.add(2, "calendar_refresh", jcDay, false)
	r.bf.res = jcLines(runner.GroupRan)
	if !r.turnOf(rec) || r.log.count("backfill") != 1 {
		t.Fatalf("after the probe: %v", r.log.items)
	}
	if jobs, on := swap.current(); !on || !slices.ContainsFunc(jobs, func(j contracts.Job) bool { return j.ID() == "calendar_refresh" }) {
		t.Fatalf("current %v %v", jobs, on)
	}
}

func (r *jcRig) turnOf(c runner.ConsoleTurner) bool {
	r.t.Helper()
	took, err := c.ConsoleTurn(r.t.Context(), jcNever)
	if err != nil {
		r.t.Fatal(err)
	}
	return took
}

type countRec struct{ reconciled, probed *int }

func (c countRec) Reconcile(context.Context, contracts.Day) error { *c.reconciled++; return nil }

func TestWrapperForwardsProbeAndConsoleTurn(t *testing.T) {
	t.Parallel()
	r := jcOne(t, "fred_series")
	reconciled, probed := 0, 0
	build := func([]contracts.Job) runner.Reconciler { return countRec{&reconciled, &probed} }
	probe := func(context.Context, string) (contracts.Job, string) { probed++; return nil, "no key" }
	rec := &consoleReconciler{sw: newSwapReconciler(nil, build, probe, slog.New(slog.DiscardHandler)), host: r.host}
	if err := rec.Reconcile(t.Context(), "2026-11-12"); err != nil || reconciled != 1 {
		t.Fatalf("Reconcile: %v, %d", err, reconciled)
	}
	if reason := rec.Probe(t.Context(), "x"); reason != "no key" || probed != 1 {
		t.Fatalf("Probe: %q, %d", reason, probed)
	}
	r.bf.res = jcLines(runner.GroupRan)
	if !r.turnOf(rec) || r.log.count("claim:1") != 1 {
		t.Fatalf("ConsoleTurn did not reach the host: %v", r.log.items)
	}
	off := &consoleReconciler{sw: rec.sw}
	if took, err := off.ConsoleTurn(t.Context(), nil); took || err != nil {
		t.Fatalf("console off: %v %v", took, err)
	}
}

func TestSchedulerRunsWithConsoleOffWhenOpenFails(t *testing.T) {
	t.Parallel()
	q := &jcQueue{log: &jcLog{}}
	r, _ := consoleSchedRig(t, q, errors.New("dial postgres://runner:hunter2@db/x"))
	done := r.start()
	r.waitStarted(t, done)
	r.tick()
	if err := r.stop(t, done); err != nil {
		t.Fatal(err)
	}
	out := r.errb.String()
	if n := strings.Count(out, "console off"); n != 1 || strings.Contains(out, "hunter2") {
		t.Fatalf("%d console-off lines, log %q", n, out)
	}
	if q.log.count("fail_running") != 0 {
		t.Fatalf("the queue was used: %v", q.log.items)
	}
}

func TestSchedulerRunsWithConsoleOffWhenStartFails(t *testing.T) {
	t.Parallel()
	q := &jcQueue{log: &jcLog{}, failErrs: []error{errors.New("relation settings.job_request does not exist")}}
	r, _ := consoleSchedRig(t, q, nil)
	done := r.start()
	r.waitStarted(t, done)
	if err := r.stop(t, done); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(r.errb.String(), "console off"); n != 1 {
		t.Fatalf("%d console-off lines: %q", n, r.errb.String())
	}
	// the host tries Start again at the next turn and so needs no restart once the table exists.
	if took, err := r.sched.rec.(runner.ConsoleTurner).ConsoleTurn(t.Context(), jcNever); err != nil || took {
		t.Fatalf("turn %v %v", took, err)
	}
	if q.log.count("fail_running") != 2 {
		t.Fatalf("calls %v", q.log.items)
	}
}

func TestSchedulerRunsWithZeroOpenerAndNilBackfiller(t *testing.T) {
	t.Parallel()
	r := newSchedRig(t, fullEnv) // no ext key jobconsole; its newReconciler returns a nil Backfiller
	done := r.start()
	r.waitStarted(t, done)
	r.tick()
	turner, ok := r.sched.rec.(runner.ConsoleTurner)
	if !ok {
		t.Fatal("the reconciler cannot turn the console")
	}
	if took, err := turner.ConsoleTurn(t.Context(), nil); took || err != nil {
		t.Fatalf("console off: %v %v", took, err)
	}
	if err := r.stop(t, done); err != nil {
		t.Fatal(err)
	}
}
