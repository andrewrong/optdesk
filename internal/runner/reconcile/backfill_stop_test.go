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
)

// boundBlock is a blockJob with a RunBound that records the cause of the context it returns under.
type boundBlock struct {
	*blockJob
	bound time.Duration
	mu    sync.Mutex
	cause error
}

func (j *boundBlock) RunBound() time.Duration { return j.bound }

func (j *boundBlock) Run(ctx context.Context, d contracts.Day, k []contracts.ExpectedKey) contracts.JobResult {
	res := j.blockJob.Run(ctx, d, k)
	j.mu.Lock()
	j.cause = context.Cause(ctx)
	j.mu.Unlock()
	return res
}

func newBoundBlock(bound time.Duration, res contracts.JobResult) *boundBlock {
	return &boundBlock{blockJob: newBlockJob(res), bound: bound}
}

// boundedFake is a fakeJob with a RunBound.
type boundedFake struct {
	*fakeJob
	bound time.Duration
}

func (j boundedFake) RunBound() time.Duration { return j.bound }

// spyKR records the deadline and the console mark of each RunKey call, then runs it.
type spyKR struct {
	runner.KeyRunner
	mu        sync.Mutex
	deadlines []time.Time
	marked    []bool
}

func (s *spyKR) RunKey(
	ctx context.Context, job contracts.Job, day contracts.Day, k contracts.ExpectedKey, n int, closeAt, deadline time.Time, reason string,
) (store.Run, error) {
	s.mu.Lock()
	s.deadlines = append(s.deadlines, deadline)
	s.marked = append(s.marked, ctx.Value(consoleKey{}) != nil)
	s.mu.Unlock()
	return s.KeyRunner.RunKey(ctx, job, day, k, n, closeAt, deadline, reason)
}

type bfOut struct {
	res runner.BackfillResult
	err error
}

// goBackfill runs BackfillWith in a goroutine and waits until the job runs.
func goBackfill(ctx context.Context, rec *reconciler, opt runner.BackfillOptions, started <-chan struct{}) <-chan bfOut {
	out := make(chan bfOut, 1)
	go func() {
		res, err := rec.BackfillWith(ctx, bfDay, nil, opt)
		out <- bfOut{res, err}
	}()
	<-started
	return out
}

func waitTimers(t *testing.T, clk *clockwork.FakeClock, n int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := clk.BlockUntilContext(ctx, n); err != nil {
		t.Fatal(err)
	}
}

func TestBareRunIsCutAtTheJobsRunBound(t *testing.T) {
	t.Parallel()
	l := bfSetup(bfDay, keyA)
	l.present[bfDay] = map[contracts.ExpectedKey]store.PresentStatus{keyA: store.StatusPresent}
	clk := clockwork.NewFakeClockAt(utc("2026-11-06T00:00:00Z"))
	job := newBoundBlock(5*time.Minute, contracts.JobResult{})
	out := goBackfill(context.Background(), forceRec(l, clk, job), runner.BackfillOptions{Force: true}, job.started)
	waitTimers(t, clk, 1) // the bare run has the bound's timer and no heartbeat
	clk.Advance(5 * time.Minute)
	got := <-out
	if got.err != nil {
		t.Fatal(got.err)
	}
	if line := lineOf(t, got.res, keyA); line.Group != runner.GroupRerun || line.ReasonCode != "cut_at_bound" {
		t.Fatalf("line %+v", line)
	}
	if l.records.Load() != 0 || l.marks.Load() != 0 {
		t.Fatalf("writes: records %d marks %d", l.records.Load(), l.marks.Load())
	}
}

func TestEachKeyRunsUnderItsOwnRunBound(t *testing.T) {
	t.Parallel()
	l := bfSetup(bfDay, keyA, keyB)
	clk := clockwork.NewFakeClockAt(utc("2026-11-04T15:00:00Z")) // inside the window, which closes at 21:00
	job := boundedFake{fakeJob: &fakeJob{keys: []contracts.ExpectedKey{keyA, keyB}}, bound: 10 * time.Minute}
	spy := &spyKR{KeyRunner: NewKeyRunner(l, nil, clk)}
	rec := newReconciler(l, nil, oneDay(), clk, []contracts.Job{job}, runner.WindowRule{CalendarID: "XNYS"}, spy)
	if _, err := rec.BackfillWith(context.Background(), bfDay, nil, runner.BackfillOptions{}); err != nil {
		t.Fatal(err)
	}
	want := clk.Now().Add(10 * time.Minute)
	if len(spy.deadlines) != 2 || !spy.deadlines[0].Equal(want) || !spy.deadlines[1].Equal(want) {
		t.Fatalf("deadlines %v, want start+bound %v (not close+bound)", spy.deadlines, want)
	}
	if !spy.marked[0] || !spy.marked[1] {
		t.Fatal("a key context lacks the console mark")
	}
}

func TestHoldBeforeNextKeyStopsRequest(t *testing.T) {
	t.Parallel()
	l := bfSetup(bfDay, keyA, keyB, keyC)
	job := boundedFake{fakeJob: &fakeJob{keys: []contracts.ExpectedKey{keyA, keyB, keyC}}, bound: 10 * time.Minute}
	var asked []time.Duration
	opt := runner.BackfillOptions{Hold: func(b time.Duration) (string, time.Time, bool) {
		asked = append(asked, b)
		return "slot", time.Time{}, true
	}}
	res, err := forceRec(l, afterClose(), job).BackfillWith(context.Background(), bfDay, nil, opt)
	if err != nil {
		t.Fatal(err)
	}
	if a := lineOf(t, res, keyA); a.Group != runner.GroupRan || a.Mark != "present" {
		t.Fatalf("key before %+v", a)
	}
	for _, k := range []contracts.ExpectedKey{keyB, keyC} {
		if line := lineOf(t, res, k); line.Group != runner.GroupNotReached || line.ReasonCode != "slot_near" {
			t.Fatalf("line %+v", line)
		}
	}
	if len(l.runs[bfDay]) != 1 || len(asked) != 1 || asked[0] != 10*time.Minute+holdMargin {
		t.Fatalf("rows %+v asked %v", l.runs[bfDay], asked)
	}
}

func TestStopAtCancelNotReached(t *testing.T) {
	t.Parallel()
	l := bfSetup(bfDay, keyA, keyB)
	job := &fakeJob{keys: []contracts.ExpectedKey{keyA, keyB}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	res, err := forceRec(l, afterClose(), job).BackfillWith(ctx, bfDay, nil, runner.BackfillOptions{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err %v", err)
	}
	if len(res.Keys) != 2 || len(job.calls) != 0 || len(l.runs[bfDay]) != 0 {
		t.Fatalf("lines %+v calls %v", res.Keys, job.calls)
	}
	for _, line := range res.Keys {
		if line.Group != runner.GroupNotReached || line.ReasonCode != "stopped" {
			t.Fatalf("line %+v", line)
		}
	}
}

func TestDeployCancelWritesConsoleStopRow(t *testing.T) {
	t.Parallel()
	l := bfSetup(bfDay, keyA, keyB)
	clk := afterClose()
	job := newBoundBlock(time.Hour, contracts.JobResult{})
	job.keys = []contracts.ExpectedKey{keyA, keyB}
	ctx, cancel := context.WithCancel(context.Background())
	out := goBackfill(ctx, forceRec(l, clk, job), runner.BackfillOptions{}, job.started)
	cancel()
	got := <-out
	if !errors.Is(got.err, context.Canceled) {
		t.Fatalf("err %v", got.err)
	}
	runs := l.runs[bfDay]
	if len(runs) != 1 || runs[0].Key != keyA || runs[0].ReasonCode != "console_stop" || runs[0].Status != store.RunAbandoned {
		t.Fatalf("runs %+v", runs)
	}
	if b := lineOf(t, got.res, keyB); b.Group != runner.GroupNotReached || b.ReasonCode != "stopped" {
		t.Fatalf("line %+v", b)
	}
	if !errors.Is(job.cause, errConsoleStop) {
		t.Fatalf("cause %v", job.cause)
	}
}

func TestConsoleStopIsNotACrashStrike(t *testing.T) {
	t.Parallel()
	l := bfSetup(bfDay, keyA)
	rec := forceRec(l, afterClose(), &fakeJob{keys: []contracts.ExpectedKey{keyA}})
	var runs []store.Run
	for _, id := range []string{"1", "2", "3"} {
		runs = append(runs, store.Run{ID: "2026-11-04|ds|a|" + id, Day: bfDay, Key: keyA, Status: store.RunAbandoned, ReasonCode: "console_stop"})
	}
	if _, start, _ := rec.nextAttempt(runs, time.Time{}); !start {
		t.Fatal("three console_stop rows stop the key")
	}
	l.runs[bfDay] = runs
	closed, err := rec.expireIfClosed(context.Background(), runs, utc("2026-11-04T21:00:00Z"), bfDay, keyA)
	if err != nil || closed {
		t.Fatalf("closed %v err %v", closed, err)
	}
	for _, r := range l.runs[bfDay] {
		if r.ReasonCode == reasonCrashedRepeatedly {
			t.Fatalf("run %+v", r)
		}
	}
}

func TestConsoleDeadlineCutIsConsoleStop(t *testing.T) {
	t.Parallel()
	l := bfSetup(bfDay, keyA)
	clk := clockwork.NewFakeClockAt(utc("2026-11-04T15:00:00Z"))
	job := newBoundBlock(5*time.Minute, contracts.JobResult{})
	out := goBackfill(context.Background(), forceRec(l, clk, job), runner.BackfillOptions{}, job.started)
	waitTimers(t, clk, 2) // the heartbeat ticker and the deadline timer
	clk.Advance(5 * time.Minute)
	got := <-out
	if got.err != nil {
		t.Fatal(got.err)
	}
	runs := l.runs[bfDay]
	if len(runs) != 1 || runs[0].ReasonCode != "console_stop" || runs[0].Status != store.RunAbandoned {
		t.Fatalf("runs %+v", runs)
	}
	if line := lineOf(t, got.res, keyA); line.ReasonCode != "console_stop" {
		t.Fatalf("line %+v", line)
	}
}

func TestEarlierOfDeployCancelAndKeyDeadlineDecidesCause(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		first string
		want  error
	}{{"cancel first", "cancel", errConsoleStop}, {"deadline first", "deadline", errConsoleBound}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			clk := clockwork.NewFakeClockAt(utc("2026-11-04T22:00:00Z"))
			led := newRkLedger()
			job := newBoundBlock(time.Hour, contracts.JobResult{})
			base, cancel := context.WithCancelCause(context.Background())
			ctx := context.WithValue(base, consoleKey{}, true)
			out := startRunKey(t, ctx, led, clk, job, job.started, clk.Now().Add(time.Minute))
			waitTimers(t, clk, 2)
			if tc.first == "cancel" {
				cancel(errConsoleStop)
			} else {
				clk.Advance(time.Minute)
			}
			r := <-out
			cancel(nil)
			if r.err != nil || r.run.ReasonCode != "console_stop" {
				t.Fatalf("run %+v err %v", r.run, r.err)
			}
			job.mu.Lock()
			defer job.mu.Unlock()
			if !errors.Is(job.cause, tc.want) {
				t.Fatalf("cause %v, want %v", job.cause, tc.want)
			}
		})
	}
}

func TestMissingKeyRunsOnceNotTwice(t *testing.T) {
	t.Parallel()
	l := bfSetup(bfDay, keyA)
	job := &fakeJob{keys: []contracts.ExpectedKey{keyA}, results: map[contracts.ExpectedKey]contracts.JobResult{
		keyA: {Err: kindErr{kind: contracts.Retryable}},
	}}
	if _, err := forceRec(l, afterClose(), job).BackfillWith(context.Background(), bfDay, nil, runner.BackfillOptions{}); err != nil {
		t.Fatal(err)
	}
	if len(job.calls) != 1 || len(l.runs[bfDay]) != 1 {
		t.Fatalf("calls %v rows %+v", job.calls, l.runs[bfDay])
	}
}

func TestConsoleStopDoesNotHideEarlierRetryableRun(t *testing.T) {
	t.Parallel()
	runs := []store.Run{
		{ID: "1", Status: store.RunDone, ErrorCode: string(contracts.Retryable)},
		{ID: "2", Status: store.RunAbandoned, ReasonCode: "console_stop"},
	}
	if idx, reason, ok := closeRow(runs); !ok || idx != 0 || reason != reasonExpired {
		t.Fatalf("closeRow = %d %q %v", idx, reason, ok)
	}
}

func TestConsoleStopIgnoredByCloseRow(t *testing.T) {
	t.Parallel()
	stop := store.Run{ID: "s", Status: store.RunPartial, ReasonCode: "console_stop"}
	if _, _, ok := closeRow([]store.Run{stop, stop}); ok {
		t.Fatal("console_stop rows alone match a row of the Close table")
	}
	verdict := store.Run{ID: "v", Status: store.RunDone}
	if idx, _, ok := closeRow([]store.Run{verdict, stop}); !ok || idx != 0 {
		t.Fatalf("closeRow = %d %v, want the verdict run", idx, ok)
	}
}

func TestSchedulerDeadlineCutIsStillACrash(t *testing.T) {
	t.Parallel()
	clk := clockwork.NewFakeClockAt(utc("2026-11-04T22:00:00Z"))
	led := newLedger()
	led.cursor = store.Cursor{Day: "2026-11-03"}
	job := newBoundBlock(5*time.Minute, contracts.JobResult{})
	done := make(chan error, 1)
	go func() {
		done <- NewWithRule(led, oneDay(), clk, []contracts.Job{job}, runner.WindowRule{CalendarID: "XNYS"}).Reconcile(context.Background(), "2026-11-04")
	}()
	<-job.started
	waitTimers(t, clk, 2)
	clk.Advance(5 * time.Minute)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	runs := led.runs[bfDay]
	if len(runs) != 1 || runs[0].ReasonCode != "abandoned_stop" || !isCrash(runs[0]) {
		t.Fatalf("runs %+v", runs)
	}
}

func TestUnmarkedRunKeyCutIsAbandonedStop(t *testing.T) {
	t.Parallel()
	clk := clockwork.NewFakeClockAt(utc("2026-11-04T22:00:00Z"))
	led := newRkLedger()
	job := newBoundBlock(time.Hour, contracts.JobResult{})
	out := startRunKey(t, context.Background(), led, clk, job, job.started, clk.Now().Add(time.Minute))
	waitTimers(t, clk, 2)
	clk.Advance(time.Minute)
	r := <-out
	if r.err != nil || r.run.ReasonCode != "abandoned_stop" || !isCrash(r.run) {
		t.Fatalf("run %+v err %v", r.run, r.err)
	}
}
