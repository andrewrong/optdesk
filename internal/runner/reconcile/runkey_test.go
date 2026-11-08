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

var (
	_ runner.NewKeyRunnerFunc  = NewKeyRunner
	_ runner.NewReconcilerFunc = NewReconciler
)

const rkDay contracts.Day = "2026-11-04"

// rkLedger is the in-memory ledger that also reports every run row written and every Release.
type rkLedger struct {
	*fakeLedger
	rows       chan store.Run
	mu         sync.Mutex
	releases   int
	releaseErr error // the Err of the context given to the last Release
}

func newRkLedger() *rkLedger {
	return &rkLedger{fakeLedger: newLedger(), rows: make(chan store.Run, 64)}
}

func (l *rkLedger) RecordRun(ctx context.Context, r store.Run) error {
	l.rows <- r
	return l.fakeLedger.RecordRun(ctx, r)
}

func (l *rkLedger) Release(ctx context.Context, lease store.Lease) error {
	l.mu.Lock()
	l.releases++
	l.releaseErr = ctx.Err()
	l.mu.Unlock()
	return l.fakeLedger.Release(ctx, lease)
}

// rkSched is a SchedulerLedger of which only RenewLease is used; each renewal is reported.
type rkSched struct {
	store.SchedulerLedger
	renewed chan store.Lease
}

func (s *rkSched) RenewLease(_ context.Context, l store.Lease, ttl time.Duration) (store.Lease, error) {
	s.renewed <- l
	return l, nil
}

// blockJob runs until ctx ends or release is closed, then answers res. started gets a
// value when Run begins.
type blockJob struct {
	*fakeJob
	started chan struct{}
	release chan struct{}
	res     contracts.JobResult
}

func newBlockJob(res contracts.JobResult) *blockJob {
	return &blockJob{
		fakeJob: &fakeJob{keys: []contracts.ExpectedKey{keyA}},
		started: make(chan struct{}, 4), release: make(chan struct{}), res: res,
	}
}

func (j *blockJob) Run(ctx context.Context, _ contracts.Day, _ []contracts.ExpectedKey) contracts.JobResult {
	j.started <- struct{}{}
	select {
	case <-ctx.Done():
	case <-j.release:
	}
	return j.res
}

// pcJob is a fakeJob with the optional Postcondition.
type pcJob struct {
	*fakeJob
	exists bool
	err    error
	asked  []string
}

func (j *pcJob) Postcondition(_ context.Context, day, datasetID, subjectKey string) (bool, error) {
	j.asked = append(j.asked, day+"|"+datasetID+"|"+subjectKey)
	return j.exists, j.err
}

type rkResult struct {
	run store.Run
	err error
}

// startRunKey runs keyA in a goroutine and waits until the job runs; it returns the result channel.
func startRunKey(
	t *testing.T, ctx context.Context, led *rkLedger, clk clockwork.Clock, job contracts.Job, started <-chan struct{}, deadline time.Time,
) <-chan rkResult {
	t.Helper()
	out := make(chan rkResult, 1)
	go func() {
		r, err := NewKeyRunner(led, nil, clk).RunKey(ctx, job, rkDay, keyA, 1, time.Time{}, deadline, "")
		out <- rkResult{r, err}
	}()
	<-started
	return out
}

func TestRunKeyDeadlineCutsRun(t *testing.T) {
	t.Parallel()
	clk := clockwork.NewFakeClockAt(utc("2026-11-04T22:00:00Z"))
	led := newRkLedger()
	job := newBlockJob(contracts.JobResult{})
	out := startRunKey(t, context.Background(), led, clk, job, job.started, clk.Now().Add(time.Minute))
	select {
	case <-out:
		t.Fatal("the run ended before the deadline")
	default:
	}
	// One ticker of the heartbeat and the deadline timer wait on the clock.
	if err := clk.BlockUntilContext(context.Background(), 2); err != nil {
		t.Fatal(err)
	}
	clk.Advance(time.Minute)
	r := <-out
	if r.err != nil {
		t.Fatal(r.err)
	}
	if r.run.Status == store.RunDone {
		t.Fatalf("a run cut at the deadline is %s", r.run.Status)
	}
	if !r.run.FinishedAt.Equal(clk.Now()) {
		t.Fatalf("FinishedAt = %v, want %v", r.run.FinishedAt, clk.Now())
	}
}

func TestRunKeyCutWithRowsIsPartial(t *testing.T) {
	t.Parallel()
	clk := clockwork.NewFakeClockAt(utc("2026-11-04T22:00:00Z"))
	led := newRkLedger()
	job := newBlockJob(contracts.JobResult{Returned: 3, Usable: 3})
	out := startRunKey(t, context.Background(), led, clk, job, job.started, clk.Now().Add(time.Minute))
	if err := clk.BlockUntilContext(context.Background(), 2); err != nil {
		t.Fatal(err)
	}
	clk.Advance(time.Minute)
	r := <-out
	if r.err != nil {
		t.Fatal(r.err)
	}
	if r.run.Status != store.RunPartial || r.run.ReturnedN != 3 {
		t.Fatalf("run = %+v, want partial with 3 rows", r.run)
	}
	present, _ := led.Present(context.Background(), rkDay)
	if len(present) != 0 {
		t.Fatalf("a partial run marked %v, even with Usable > 0", present)
	}
	runs, _ := led.Runs(context.Background(), rkDay)
	if len(runs) != 1 || runs[0].Status != store.RunPartial {
		t.Fatalf("stored runs = %+v", runs)
	}
}

func TestRunKeyCutWithoutRowsIsAbandonedStop(t *testing.T) {
	t.Parallel()
	clk := clockwork.NewFakeClockAt(utc("2026-11-04T22:00:00Z"))
	led := newRkLedger()
	job := newBlockJob(contracts.JobResult{})
	ctx, cancel := context.WithCancel(context.Background())
	out := startRunKey(t, ctx, led, clk, job, job.started, clk.Now().Add(time.Hour))
	cancel() // a graceful stop, long before the deadline
	r := <-out
	if r.err != nil {
		t.Fatal(r.err)
	}
	if r.run.Status != store.RunAbandoned || r.run.ReasonCode != "abandoned_stop" {
		t.Fatalf("run = %+v, want abandoned with reason abandoned_stop", r.run)
	}
	runs, _ := led.Runs(context.Background(), rkDay)
	if len(runs) != 1 || runs[0].Status != store.RunAbandoned {
		t.Fatalf("the row was not written after the cancel: %+v", runs)
	}
	present, _ := led.Present(context.Background(), rkDay)
	if len(present) != 0 {
		t.Fatalf("an abandoned run marked %v", present)
	}
}

func TestRunKeyHeartbeatEvery30s(t *testing.T) {
	t.Parallel()
	start := utc("2026-11-04T22:00:00Z")
	clk := clockwork.NewFakeClockAt(start)
	led := newRkLedger()
	job := newBlockJob(contracts.JobResult{Returned: 1, Usable: 1})
	out := startRunKey(t, context.Background(), led, clk, job, job.started, start.Add(time.Hour))
	first := <-led.rows
	if first.Status != store.RunRunning || !first.HeartbeatAt.Equal(start) {
		t.Fatalf("first row = %+v", first)
	}
	if err := clk.BlockUntilContext(context.Background(), 2); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 2; i++ {
		clk.Advance(30 * time.Second)
		beat := <-led.rows
		want := start.Add(time.Duration(i) * 30 * time.Second)
		if beat.Status != store.RunRunning || !beat.HeartbeatAt.Equal(want) || !beat.LeaseUntil.Equal(want.Add(leaseTTL)) {
			t.Fatalf("beat %d = %+v, want heartbeat %v and lease %v", i, beat, want, want.Add(leaseTTL))
		}
	}
	close(job.release)
	r := <-out
	if r.err != nil || r.run.Status != store.RunDone {
		t.Fatalf("run = %+v, err %v", r.run, r.err)
	}
	// The last row written is the verdict, not a beat.
	runs, _ := led.Runs(context.Background(), rkDay)
	if len(runs) != 1 || runs[0].Status != store.RunDone {
		t.Fatalf("stored runs = %+v", runs)
	}
}

func TestRunKeyPostcondition(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		exists  bool
		err     error
		usable  int
		present bool
	}{
		{"true marks present even with no usable rows", true, nil, 0, true},
		{"false leaves the key missing even with usable rows", false, nil, 2, false},
		{"an error leaves the key missing", true, errors.New("boom"), 2, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			clk := clockwork.NewFakeClockAt(utc("2026-11-04T22:00:00Z"))
			led := newRkLedger()
			job := &pcJob{
				fakeJob: &fakeJob{results: map[contracts.ExpectedKey]contracts.JobResult{keyA: {Returned: tc.usable, Usable: tc.usable}}},
				exists:  tc.exists, err: tc.err,
			}
			run, err := NewKeyRunner(led, nil, clk).RunKey(context.Background(), job, rkDay, keyA, 1, time.Time{}, clk.Now().Add(time.Hour), "")
			if err != nil {
				t.Fatal(err)
			}
			if run.Status != store.RunDone || run.ErrorCode != "" {
				t.Fatalf("run = %+v, want a plain done run", run)
			}
			if len(job.asked) != 1 || job.asked[0] != "2026-11-04|ds|a" {
				t.Fatalf("Postcondition asked %v", job.asked)
			}
			present, _ := led.Present(context.Background(), rkDay)
			if got := present[keyA] == store.StatusPresent; got != tc.present {
				t.Fatalf("present = %v, want %v", present, tc.present)
			}
		})
	}
}

// spyRunner is a KeyRunner that records its calls and marks each key present.
type spyRunner struct {
	led   *fakeLedger
	mu    sync.Mutex
	calls []string
}

func (s *spyRunner) RunKey(
	ctx context.Context, _ contracts.Job, day contracts.Day, k contracts.ExpectedKey, _ int, _, deadline time.Time, reason string,
) (store.Run, error) {
	s.mu.Lock()
	s.calls = append(s.calls, string(day)+"|"+k.SubjectKey+"|"+reason)
	s.mu.Unlock()
	if deadline.IsZero() {
		return store.Run{}, errors.New("no deadline")
	}
	return store.Run{}, s.led.MarkPresent(ctx, day, k, store.StatusPresent)
}

func TestReconcileAndBackfillUseRunKey(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	clk := clockwork.NewFakeClockAt(utc("2026-11-04T22:00:00Z"))
	led := newLedger()
	led.cursor = store.Cursor{Day: "2026-11-03"}
	job := &fakeJob{keys: []contracts.ExpectedKey{keyA}}
	spy := &spyRunner{led: led}
	rec, bf := NewReconciler(led, &rkSched{renewed: make(chan store.Lease, 1)}, oneDay(), clk, []contracts.Job{job}, runner.WindowRule{CalendarID: "XNYS"}, spy)
	if err := rec.Reconcile(ctx, "2026-11-04"); err != nil {
		t.Fatal(err)
	}
	// A no_value key is run again by Backfill, through the same runner.
	led.present[rkDay] = map[contracts.ExpectedKey]store.PresentStatus{keyA: store.StatusNoValue}
	led.cursor = store.Cursor{Day: rkDay}
	if _, err := bf.Backfill(ctx, rkDay, nil); err != nil {
		t.Fatal(err)
	}
	want := []string{"2026-11-04|a|late", "2026-11-04|a|backfill"}
	if len(spy.calls) != 2 || spy.calls[0] != want[0] || spy.calls[1] != want[1] {
		t.Fatalf("RunKey calls = %v, want %v", spy.calls, want)
	}
	if len(job.calls) != 0 {
		t.Fatalf("the job ran %d times outside the key runner", len(job.calls))
	}
}

func TestRunLeaseRenewedWhileRunning(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	clk := clockwork.NewFakeClockAt(utc("2026-11-04T22:00:00Z"))
	led := newRkLedger()
	led.cursor = store.Cursor{Day: "2026-11-03"}
	sched := &rkSched{renewed: make(chan store.Lease, 8)}
	job := newBlockJob(contracts.JobResult{Returned: 1, Usable: 1})
	rec, _ := NewReconciler(led, sched, oneDay(), clk, []contracts.Job{job}, runner.WindowRule{CalendarID: "XNYS"}, nil)
	done := make(chan error, 1)
	go func() { done <- rec.Reconcile(ctx, "2026-11-04") }()
	<-job.started
	// Lease ticker, run heartbeat and run deadline.
	if err := clk.BlockUntilContext(ctx, 3); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		clk.Advance(30 * time.Second)
		l := <-sched.renewed
		if l.Owner != "runner" {
			t.Fatalf("renewed lease of %q", l.Owner)
		}
	}
	close(job.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	led.mu.Lock()
	defer led.mu.Unlock()
	if led.releases != 1 {
		t.Fatalf("Release called %d times, want 1", led.releases)
	}
}

func TestReconcileReleasesLeaseWhenCancelled(t *testing.T) {
	t.Parallel()
	clk := clockwork.NewFakeClockAt(utc("2026-11-04T22:00:00Z"))
	led := newRkLedger()
	led.cursor = store.Cursor{Day: "2026-11-03"}
	sched := &rkSched{renewed: make(chan store.Lease, 8)}
	job := newBlockJob(contracts.JobResult{})
	rec, _ := NewReconciler(led, sched, oneDay(), clk, []contracts.Job{job}, runner.WindowRule{CalendarID: "XNYS"}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- rec.Reconcile(ctx, "2026-11-04") }()
	<-job.started
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	led.mu.Lock()
	defer led.mu.Unlock()
	if led.releases != 1 {
		t.Fatalf("Release called %d times, want 1", led.releases)
	}
	if led.releaseErr != nil {
		t.Fatalf("Release got a cancelled context: %v", led.releaseErr)
	}
	runs, _ := led.Runs(context.Background(), rkDay)
	if len(runs) != 1 || runs[0].Status != store.RunAbandoned {
		t.Fatalf("runs = %+v, want one abandoned", runs)
	}
}
