package reconcile

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/jobrun"
)

// ledgerCall is one call the fake JobRunLedger took.
type ledgerCall struct {
	op            string // start, progress or close
	run           jobrun.JobRun
	done, total   int
	step, outcome string
	at            time.Time
	ctxDone       bool // the context was already done when the call came
}

// fakeRunLedger is an in-memory jobrun.JobRunLedger that keeps its calls in order.
type fakeRunLedger struct {
	mu       sync.Mutex
	calls    []ledgerCall
	startErr error
	otherErr error
}

func (f *fakeRunLedger) add(c ledgerCall) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, c)
}

func (f *fakeRunLedger) Start(ctx context.Context, r jobrun.JobRun) error {
	f.add(ledgerCall{op: "start", run: r, ctxDone: ctx.Err() != nil})
	return f.startErr
}

func (f *fakeRunLedger) Progress(ctx context.Context, _ string, done, total int, step string, at time.Time) error {
	f.add(ledgerCall{op: "progress", done: done, total: total, step: step, at: at, ctxDone: ctx.Err() != nil})
	return f.otherErr
}

func (f *fakeRunLedger) Close(ctx context.Context, _, outcome string, at time.Time) error {
	f.add(ledgerCall{op: "close", outcome: outcome, at: at, ctxDone: ctx.Err() != nil})
	return f.otherErr
}

func (f *fakeRunLedger) CloseOpen(context.Context, time.Time) (int, error) { return 0, nil }

func (f *fakeRunLedger) MarkSlowNotified(context.Context, string, time.Time) error { return nil }

func (f *fakeRunLedger) ops() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, c := range f.calls {
		out = append(out, c.op)
	}
	return out
}

func TestRunRowProgressPerKey(t *testing.T) {
	t.Parallel()
	led := newLedger()
	clk := detClock()
	rl := &fakeRunLedger{}
	job := &fakeJob{keys: []contracts.ExpectedKey{keyA, keyB}}
	rec := detRec(led, clk, job)
	WithJobRunLedger(rec, rl)

	if err := rec.ReconcileDetached(context.Background(), detDay, newSpawnLog(true).spawn); err != nil {
		t.Fatal(err)
	}
	want := []ledgerCall{
		{op: "start"},
		{op: "progress", done: 1, total: 2, step: "ds/a"},
		{op: "progress", done: 2, total: 2, step: "ds/b"},
		{op: "close", outcome: "ok"},
	}
	if len(rl.calls) != len(want) {
		t.Fatalf("calls = %+v, want %d", rl.calls, len(want))
	}
	for i, w := range want {
		g := rl.calls[i]
		if g.op != w.op || g.done != w.done || g.total != w.total || g.step != w.step || g.outcome != w.outcome {
			t.Fatalf("call %d = %+v, want %+v", i, g, w)
		}
		if !g.at.Equal(clk.Now()) && g.op != "start" {
			t.Fatalf("call %d at %v, want the clock's %v", i, g.at, clk.Now())
		}
	}
	s := rl.calls[0].run
	if s.JobID != "fake" || s.Source != "scheduler" || !s.StartedAt.Equal(clk.Now()) || s.ID == "" || s.KeysTotal != 2 {
		t.Fatalf("started row = %+v", s)
	}
}

// cancelJob cancels the context of its run while it runs.
type cancelJob struct {
	*fakeJob
	cancel context.CancelFunc
}

func (j *cancelJob) Run(ctx context.Context, d contracts.Day, k []contracts.ExpectedKey) contracts.JobResult {
	j.cancel()
	return j.fakeJob.Run(ctx, d, k)
}

func TestRunRowClosedOnCut(t *testing.T) {
	t.Parallel()
	led := newLedger()
	rl := &fakeRunLedger{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	job := &cancelJob{fakeJob: &fakeJob{keys: []contracts.ExpectedKey{keyA}}, cancel: cancel}
	rec := detRec(led, detClock(), job)
	WithJobRunLedger(rec, rl)
	spawn := runner.Spawn(func(_ string, run func(ctx context.Context)) { run(ctx) })

	_ = rec.ReconcileDetached(context.Background(), detDay, spawn)
	last := rl.calls[len(rl.calls)-1]
	if last.op != "close" || last.outcome != "cut" {
		t.Fatalf("last call = %+v, want close cut; all %v", last, rl.ops())
	}
	if last.ctxDone {
		t.Fatal("close was given a cancelled context")
	}
}

func TestBackfillWritesRow(t *testing.T) {
	t.Parallel()
	l := bfSetup(bfDay, keyA)
	rl := &fakeRunLedger{}
	clk := afterClose()
	job := &fakeJob{keys: []contracts.ExpectedKey{keyA}}
	rec := forceRec(l, clk, job)
	WithJobRunLedger(rec, rl)

	if _, err := rec.BackfillWith(context.Background(), bfDay, nil, runner.BackfillOptions{Job: "fake"}); err != nil {
		t.Fatal(err)
	}
	if got := rl.ops(); len(got) != 3 || got[0] != "start" || got[1] != "progress" || got[2] != "close" {
		t.Fatalf("calls = %v", got)
	}
	if s := rl.calls[0].run; s.Source != "console" || s.JobID != "fake" {
		t.Fatalf("started row = %+v", s)
	}
	if p := rl.calls[1]; p.done != 1 || p.total != 1 || p.step != "ds/a" {
		t.Fatalf("progress = %+v", p)
	}
	if c := rl.calls[2]; c.outcome != "ok" || !c.at.Equal(clk.Now()) {
		t.Fatalf("close = %+v", c)
	}
}

func TestLedgerFailureNeverStopsRun(t *testing.T) {
	t.Parallel()
	led := newLedger()
	rl := &fakeRunLedger{startErr: errors.New("down"), otherErr: errors.New("down")}
	job := &fakeJob{keys: []contracts.ExpectedKey{keyA}}
	rec := detRec(led, detClock(), job)
	WithJobRunLedger(rec, rl)

	if err := rec.ReconcileDetached(context.Background(), detDay, newSpawnLog(true).spawn); err != nil {
		t.Fatal(err)
	}
	if !presentOf(t, led, keyA) {
		t.Fatal("the key was not run after the ledger failed")
	}
}

func TestNoLedgerRecordsNothing(t *testing.T) {
	t.Parallel()
	led := newLedger()
	job := &fakeJob{keys: []contracts.ExpectedKey{keyA}}
	rec := detRec(led, detClock(), job)
	if err := rec.ReconcileDetached(context.Background(), detDay, newSpawnLog(true).spawn); err != nil {
		t.Fatal(err)
	}
	if !presentOf(t, led, keyA) {
		t.Fatal("the key was not run without a ledger")
	}
}

// lateLedger takes the fake clock past the slot of the dependent when the first row (the upstream's)
// closes, so the dependent is not due when the pass starts and runs only when the end of its upstream
// triggers it.
type lateLedger struct {
	*fakeRunLedger
	clk  *clockwork.FakeClock
	once sync.Once
}

func (l *lateLedger) Close(ctx context.Context, id, outcome string, at time.Time) error {
	err := l.fakeRunLedger.Close(ctx, id, outcome, at)
	l.once.Do(func() { l.clk.Advance(6 * time.Hour) })
	return err
}

// unlockSignal closes done when the first lock taken for job id is released: a dependent releases it
// after its row is closed.
type unlockSignal struct {
	runner.JobLocks
	id   string
	done chan struct{}
	once sync.Once
}

func (u *unlockSignal) TryLock(id string) (func(), bool) {
	unlock, ok := u.JobLocks.TryLock(id)
	if !ok || id != u.id {
		return unlock, ok
	}
	return func() {
		unlock()
		u.once.Do(func() { close(u.done) })
	}, ok
}

// triggeredDep builds a reconciler whose dependent "dep" (key keyC) is run only by the end of "up".
func triggeredDep(led *fakeLedger, rl *fakeRunLedger, d depJob) (*reconciler, <-chan struct{}) {
	clk := clockwork.NewFakeClockAt(utc("2026-11-04T22:00:00Z"))
	rec := detRec(led, clk, upJob(keyA), d)
	WithJobRunLedger(rec, &lateLedger{fakeRunLedger: rl, clk: clk})
	sig := &unlockSignal{JobLocks: rec.locks, id: "dep", done: make(chan struct{})}
	rec.locks = sig
	return rec, sig.done
}

func TestDependentRunWritesRow(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		results map[contracts.ExpectedKey]contracts.JobResult
		outcome string
	}{
		{"ok", nil, "ok"},
		{"failed", map[contracts.ExpectedKey]contracts.JobResult{keyC: {Err: kindErr{kind: contracts.Permanent}}}, "failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			led := newLedger()
			rl := &fakeRunLedger{}
			d := dep(5*time.Hour, keyC)
			d.results = tc.results
			rec, done := triggeredDep(led, rl, d)

			if err := rec.ReconcileDetached(context.Background(), detDay, newSpawnLog(true).spawn); err != nil {
				t.Fatal(err)
			}
			<-done
			var depCalls []ledgerCall
			in := false
			for _, c := range rl.calls {
				if c.op == "start" {
					in = c.run.JobID == "dep"
				}
				if in {
					depCalls = append(depCalls, c)
				}
			}
			if len(depCalls) != 3 {
				t.Fatalf("dependent calls = %+v, want start, progress, close", depCalls)
			}
			if s := depCalls[0].run; s.Source != "scheduler" || s.KeysTotal != 1 {
				t.Fatalf("started row = %+v", s)
			}
			if depCalls[1].op != "progress" || depCalls[1].done != 1 || depCalls[1].total != 1 {
				t.Fatalf("progress = %+v", depCalls[1])
			}
			if depCalls[2].op != "close" || depCalls[2].outcome != tc.outcome {
				t.Fatalf("close = %+v, want outcome %s", depCalls[2], tc.outcome)
			}
		})
	}
}

// A dependent that already ran for the day is not run again when its upstream ends, and opens no row.
func TestSkippedDependentWritesNoRow(t *testing.T) {
	t.Parallel()
	led := newLedger()
	rl := &fakeRunLedger{}
	d := dep(5*time.Hour, keyC)
	rec, done := triggeredDep(led, rl, d)
	if err := led.MarkPresent(context.Background(), detDay, keyC, store.StatusPresent); err != nil {
		t.Fatal(err)
	}

	if err := rec.ReconcileDetached(context.Background(), detDay, newSpawnLog(true).spawn); err != nil {
		t.Fatal(err)
	}
	<-done
	var started []string
	for _, c := range rl.calls {
		if c.op == "start" {
			started = append(started, c.run.JobID)
		}
	}
	if len(started) != 1 || started[0] != "up" {
		t.Fatalf("rows started for %v, want only the upstream", started)
	}
	if len(d.calls) != 0 {
		t.Fatalf("dependent ran for %v", d.calls)
	}
}
