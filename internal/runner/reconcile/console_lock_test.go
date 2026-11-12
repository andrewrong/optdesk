package reconcile

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/runner/shared"
	"github.com/andrewrong/optdesk/internal/store"
)

// depJob is a scheduled fakeJob that declares its upstreams.
type depJob struct {
	scheduledJob
	after []string
}

func (j depJob) After() []string { return j.after }

// evLog records the start and the end of job runs in order.
type evLog struct {
	mu sync.Mutex
	ev []string
}

func (l *evLog) add(s string) {
	l.mu.Lock()
	l.ev = append(l.ev, s)
	l.mu.Unlock()
}

func (l *evLog) all() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.ev)
}

// logJob logs "start:<id>" and "end:<id>" around the run of a fakeJob.
type logJob struct {
	*fakeJob
	log *evLog
}

func (j logJob) Run(ctx context.Context, d contracts.Day, k []contracts.ExpectedKey) contracts.JobResult {
	j.log.add("start:" + j.id)
	defer j.log.add("end:" + j.id)
	return j.fakeJob.Run(ctx, d, k)
}

// logDep is a depJob that logs its runs.
type logDep struct {
	depJob
	log *evLog
}

func (j logDep) Run(ctx context.Context, d contracts.Day, k []contracts.ExpectedKey) contracts.JobResult {
	j.log.add("start:" + j.id)
	defer j.log.add("end:" + j.id)
	return j.fakeJob.Run(ctx, d, k)
}

func upJob(keys ...contracts.ExpectedKey) *fakeJob { return &fakeJob{id: "up", keys: keys} }

// dep is the dependent "dep" of "up": its slot is the session close plus offset.
func dep(offset time.Duration, keys ...contracts.ExpectedKey) depJob {
	return depJob{
		scheduledJob: scheduledJob{fakeJob: &fakeJob{id: "dep", keys: keys}, calendarID: "XNYS", anchor: "session_close", offset: offset},
		after:        []string{"up"},
	}
}

func rowOf(t *testing.T, l *fakeLedger, day contracts.Day, k contracts.ExpectedKey) store.Run {
	t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, r := range l.runs[day] {
		if r.Key == k {
			return r
		}
	}
	t.Fatalf("no run row for %v", k)
	return store.Run{}
}

func TestBackfillWithTakesNoLease(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for _, force := range []bool{false, true} {
		l := bfSetup(bfDay, keyA)
		if force {
			l.present[bfDay] = map[contracts.ExpectedKey]store.PresentStatus{keyA: store.StatusPresent}
		}
		job := &fakeJob{keys: []contracts.ExpectedKey{keyA}}
		if _, err := forceRec(l, afterClose(), job).BackfillWith(ctx, bfDay, nil, runner.BackfillOptions{Force: force}); err != nil {
			t.Fatal(err)
		}
		if l.acquired.Load() != 0 || l.released.Load() != 0 {
			t.Fatalf("force %v: BackfillWith acquired %d released %d", force, l.acquired.Load(), l.released.Load())
		}
	}
	// A lease held by another owner does not refuse it.
	h := &heldLedger{spyLedger: bfSetup(bfDay, keyA), holder: "runner"}
	job := &fakeJob{keys: []contracts.ExpectedKey{keyA}}
	res, err := forceRec(h, afterClose(), job).BackfillWith(ctx, bfDay, nil, runner.BackfillOptions{})
	if err != nil || len(job.calls) != 1 || lineOf(t, res, keyA).Group != runner.GroupRan {
		t.Fatalf("res %+v err %v calls %d", res, err, len(job.calls))
	}
	// Reconcile still takes its own.
	l := bfSetup("2026-11-03")
	if err := NewWithRule(l, oneDay(), afterClose(), []contracts.Job{job}, runner.WindowRule{CalendarID: "XNYS"}).Reconcile(ctx, bfDay); err != nil {
		t.Fatal(err)
	}
	if l.acquired.Load() != 1 {
		t.Fatalf("Reconcile acquired %d", l.acquired.Load())
	}
}

func TestJobLockReleasedOnEveryReturn(t *testing.T) {
	t.Parallel()
	free := func(t *testing.T, locks runner.JobLocks, id string) {
		t.Helper()
		unlock, ok := locks.TryLock(id)
		if !ok {
			t.Fatal("the job's lock was not released")
		}
		unlock()
	}
	results := map[string]contracts.JobResult{
		"ok":    {Returned: 1, Usable: 1},
		"error": {Err: kindErr{kind: contracts.Permanent}},
	}
	for name, res := range results {
		job := &fakeJob{keys: []contracts.ExpectedKey{keyA}, results: map[contracts.ExpectedKey]contracts.JobResult{keyA: res}}
		ctx := context.Background()

		locks := shared.NewJobLocks()
		l := bfSetup(bfDay, keyA)
		l.present[bfDay] = map[contracts.ExpectedKey]store.PresentStatus{keyA: store.StatusPresent}
		rec := forceRec(l, afterClose(), job)
		WithJobLocks(rec, locks)
		if _, err := rec.BackfillWith(ctx, bfDay, nil, runner.BackfillOptions{Force: true}); err != nil {
			t.Fatal(err)
		}
		free(t, locks, "fake") // bareRun

		locks = shared.NewJobLocks()
		rec = forceRec(bfSetup(bfDay, keyA), afterClose(), job)
		WithJobLocks(rec, locks)
		if _, err := rec.BackfillWith(ctx, bfDay, nil, runner.BackfillOptions{}); err != nil {
			t.Fatal(err)
		}
		free(t, locks, "fake") // runMissingKey

		locks = shared.NewJobLocks()
		rl := bfSetup("2026-11-03")
		rc := NewWithRule(rl, oneDay(), afterClose(), []contracts.Job{job}, runner.WindowRule{CalendarID: "XNYS"})
		WithJobLocks(rc, locks)
		if err := rc.Reconcile(ctx, bfDay); err != nil {
			t.Fatal(err)
		}
		free(t, locks, "fake") // the key run of Reconcile
		_ = name
	}

	// A cancelled context returns too.
	locks := shared.NewJobLocks()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rec := forceRec(bfSetup(bfDay, keyA), afterClose(), &fakeJob{keys: []contracts.ExpectedKey{keyA}})
	WithJobLocks(rec, locks)
	_, _ = rec.BackfillWith(ctx, bfDay, nil, runner.BackfillOptions{})
	free(t, locks, "fake")
}

func TestBusyJobIsJobRunning(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	locks := shared.NewJobLocks()
	// The lock is held by a run of another reconciler of the same set (a run begun before a swap).
	other := newReconciler(nil, nil, nil, nil, nil, runner.WindowRule{}, stubKeyRunner{})
	WithJobLocks(other, locks)
	unlock, ok := other.locks.TryLock("fake")
	if !ok {
		t.Fatal("lock refused")
	}

	for _, force := range []bool{false, true} {
		l := bfSetup(bfDay, keyA)
		if force {
			l.present[bfDay] = map[contracts.ExpectedKey]store.PresentStatus{keyA: store.StatusPresent}
		}
		job := &fakeJob{keys: []contracts.ExpectedKey{keyA}}
		rec := forceRec(l, afterClose(), job)
		WithJobLocks(rec, locks)
		res, err := rec.BackfillWith(ctx, bfDay, nil, runner.BackfillOptions{Force: force})
		if err != nil {
			t.Fatal(err)
		}
		line := lineOf(t, res, keyA)
		if line.Group != runner.GroupLeaseHeld || line.ReasonCode != "job_running" || len(job.calls) != 0 || len(l.runs[bfDay]) != 0 {
			t.Fatalf("force %v: line %+v calls %d runs %d", force, line, len(job.calls), len(l.runs[bfDay]))
		}
	}

	// Reconcile leaves the key: no run, no cursor move, and it returns without waiting.
	rl := bfSetup("2026-11-03")
	job := &fakeJob{keys: []contracts.ExpectedKey{keyA}}
	rc := NewWithRule(rl, oneDay(), afterClose(), []contracts.Job{job}, runner.WindowRule{CalendarID: "XNYS"})
	WithJobLocks(rc, locks)
	if err := rc.Reconcile(ctx, bfDay); err != nil {
		t.Fatal(err)
	}
	if len(job.calls) != 0 || rl.cursor.Day != "2026-11-03" {
		t.Fatalf("calls %d cursor %q", len(job.calls), rl.cursor.Day)
	}

	// Once the holder ends, the next tick runs the key.
	unlock()
	if err := rc.Reconcile(ctx, bfDay); err != nil {
		t.Fatal(err)
	}
	if len(job.calls) != 1 || rl.cursor.Day != bfDay {
		t.Fatalf("calls %d cursor %q", len(job.calls), rl.cursor.Day)
	}
}

func TestReconcileJobsParallelPerDay(t *testing.T) {
	t.Parallel()
	l := bfSetup("2026-11-03")
	a, b := newBlockJob(contracts.JobResult{Returned: 1, Usable: 1}), newBlockJob(contracts.JobResult{Returned: 1, Usable: 1})
	a.id, b.id = "a", "b"
	b.keys = []contracts.ExpectedKey{keyC}
	rc := NewWithRule(l, oneDay(), afterClose(), []contracts.Job{a, b}, runner.WindowRule{CalendarID: "XNYS"})
	done := make(chan error, 1)
	go func() { done <- rc.Reconcile(context.Background(), bfDay) }()
	<-a.started // both jobs run at once: neither waits for the other to end
	<-b.started
	if cur, _ := l.Cursor(context.Background()); cur.Day != "2026-11-03" {
		t.Fatalf("cursor %q moved before the jobs ended", cur.Day)
	}
	close(a.release)
	close(b.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if cur, _ := l.Cursor(context.Background()); cur.Day != bfDay {
		t.Fatalf("cursor %q after the day", cur.Day)
	}
	if l.acquired.Load() != 1 || l.released.Load() != 1 {
		t.Fatalf("lease: acquired %d released %d", l.acquired.Load(), l.released.Load())
	}
}

func TestDependentWaitsForItsSlot(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	led := dueLedger()
	up, d := upJob(keyA), dep(0, keyC)
	if err := dueRec(led, "2026-11-05T20:59:00Z", up, d).Reconcile(ctx, dueDay); err != nil {
		t.Fatal(err)
	}
	if len(up.calls) != 1 || len(d.calls) != 0 {
		t.Fatalf("before the slot: up %d dep %d", len(up.calls), len(d.calls))
	}
	if err := dueRec(led, "2026-11-05T21:00:00Z", up, d).Reconcile(ctx, dueDay); err != nil {
		t.Fatal(err)
	}
	if len(up.calls) != 1 || len(d.calls) != 1 {
		t.Fatalf("at the slot: up %d dep %d", len(up.calls), len(d.calls))
	}
	if r := rowOf(t, led, dueDay, keyC); r.ReasonCode == "upstream_missing" {
		t.Fatalf("upstream was ok: row %+v", r)
	}
}

func TestDependentAfterUpstreamInSameTick(t *testing.T) {
	t.Parallel()
	log := &evLog{}
	led := dueLedger()
	up := logJob{fakeJob: upJob(keyA), log: log}
	d := logDep{depJob: dep(0, keyC), log: log}
	if err := dueRec(led, "2026-11-05T21:30:00Z", up, d).Reconcile(context.Background(), dueDay); err != nil {
		t.Fatal(err)
	}
	if got, want := log.all(), []string{"start:up", "end:up", "start:dep", "end:dep"}; !slices.Equal(got, want) {
		t.Fatalf("events %v, want %v", got, want)
	}
	if r := rowOf(t, led, dueDay, keyC); r.ReasonCode == "upstream_missing" {
		t.Fatalf("marked although the upstream ended ok: %+v", r)
	}
	if cur, _ := led.Cursor(context.Background()); cur.Day != dueDay {
		t.Fatalf("cursor %q: the day is complete", cur.Day)
	}
}

func TestDependentRunsWithMarkWhenUpstreamFailed(t *testing.T) {
	t.Parallel()
	led := dueLedger()
	// The upstream's only run ended with a permanent error: the key is missing and nothing retries it.
	led.runs[dueDay] = []store.Run{{ID: "2026-11-05|ds|a|1", Day: dueDay, Key: keyA, Status: store.RunDone, ErrorCode: string(contracts.Permanent)}}
	up, d := upJob(keyA), dep(-10*time.Minute, keyC)
	if err := dueRec(led, "2026-11-05T20:55:00Z", up, d).Reconcile(context.Background(), dueDay); err != nil {
		t.Fatal(err)
	}
	if len(up.calls) != 0 || len(d.calls) != 1 {
		t.Fatalf("up %d dep %d", len(up.calls), len(d.calls))
	}
	if r := rowOf(t, led, dueDay, keyC); r.ReasonCode != "upstream_missing" {
		t.Fatalf("row %+v", r)
	}
}

func TestDependentLeftWhileUpstreamRunsAndRunsWhenItEnds(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	l := bfSetup(bfDay, keyA, keyC)
	up, d := upJob(keyA), dep(0, keyC)
	// A live run of the upstream: at the slot the dependent is left, with no wait.
	l.runs[bfDay] = []store.Run{{ID: "2026-11-04|ds|a|1", Day: bfDay, Key: keyA, Status: store.RunRunning, LeaseUntil: afterClose().Now().Add(time.Hour)}}
	rec := NewWithRule(l, oneDay(), afterClose(), []contracts.Job{up, d}, runner.WindowRule{CalendarID: "XNYS"})
	if err := rec.Reconcile(ctx, bfDay); err != nil {
		t.Fatal(err)
	}
	if len(d.calls) != 0 {
		t.Fatalf("dependent ran beside a running upstream: %d", len(d.calls))
	}
}

func TestDependentRunsWhenLateUpstreamEnds(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	l := bfSetup(bfDay, keyA, keyC)
	up, d := upJob(keyA), dep(0, keyC)
	rec := forceRec(l, afterClose(), up, d)
	// The slot has passed and the upstream ends ok later: the dependent runs then, once.
	if _, err := rec.BackfillWith(ctx, bfDay, nil, runner.BackfillOptions{Job: "up"}); err != nil {
		t.Fatal(err)
	}
	if len(up.calls) != 1 || len(d.calls) != 1 {
		t.Fatalf("up %d dep %d", len(up.calls), len(d.calls))
	}
	if r := rowOf(t, l.fakeLedger, bfDay, keyC); r.ReasonCode == "upstream_missing" {
		t.Fatalf("row %+v", r)
	}
	// A forced re-run of the upstream does not run the dependent again.
	if _, err := rec.BackfillWith(ctx, bfDay, nil, runner.BackfillOptions{Job: "up", Force: true}); err != nil {
		t.Fatal(err)
	}
	if len(up.calls) != 2 || len(d.calls) != 1 {
		t.Fatalf("after a forced re-run: up %d dep %d", len(up.calls), len(d.calls))
	}
}

func TestDependentRunsMarkedAfterFailedUpstreamEnd(t *testing.T) {
	t.Parallel()
	l := bfSetup(bfDay, keyA, keyC)
	up := upJob(keyA)
	up.results = map[contracts.ExpectedKey]contracts.JobResult{keyA: {Err: kindErr{kind: contracts.Permanent}}}
	d := dep(0, keyC)
	if _, err := forceRec(l, afterClose(), up, d).BackfillWith(context.Background(), bfDay, nil, runner.BackfillOptions{Job: "up"}); err != nil {
		t.Fatal(err)
	}
	if len(d.calls) != 1 {
		t.Fatalf("dep %d", len(d.calls))
	}
	if r := rowOf(t, l.fakeLedger, bfDay, keyC); r.ReasonCode != "upstream_missing" {
		t.Fatalf("row %+v", r)
	}
}

func TestUpstreamTriggersDependents(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// A dependent whose slot has not passed is left to its slot turn.
	l := bfSetup(bfDay, keyA, keyC)
	up, d := upJob(keyA), dep(0, keyC)
	early := clockwork.NewFakeClockAt(utc("2026-11-04T20:00:00Z"))
	if _, err := forceRec(l, early, up, d).BackfillWith(ctx, bfDay, nil, runner.BackfillOptions{Job: "up"}); err != nil {
		t.Fatal(err)
	}
	if len(d.calls) != 0 {
		t.Fatalf("dependent started before its slot: %d", len(d.calls))
	}

	// A forced bare re-run of the upstream writes no row, and triggers the dependent when it returns ok.
	l = bfSetup(bfDay, keyA, keyC)
	l.present[bfDay] = map[contracts.ExpectedKey]store.PresentStatus{keyA: store.StatusPresent}
	up, d = upJob(keyA), dep(0, keyC)
	if _, err := forceRec(l, afterClose(), up, d).BackfillWith(ctx, bfDay, nil, runner.BackfillOptions{Job: "up", Force: true}); err != nil {
		t.Fatal(err)
	}
	if len(d.calls) != 1 || len(l.runs[bfDay]) != 1 {
		t.Fatalf("dep %d runs %d", len(d.calls), len(l.runs[bfDay]))
	}

	// A running dependent is skipped, and the call does not wait for it.
	l = bfSetup(bfDay, keyA, keyC)
	up, d = upJob(keyA), dep(0, keyC)
	locks := shared.NewJobLocks()
	rec := forceRec(l, afterClose(), up, d)
	WithJobLocks(rec, locks)
	unlock, _ := locks.TryLock("dep")
	if _, err := rec.BackfillWith(ctx, bfDay, nil, runner.BackfillOptions{Job: "up"}); err != nil {
		t.Fatal(err)
	}
	unlock()
	if len(up.calls) != 1 || len(d.calls) != 0 {
		t.Fatalf("up %d dep %d", len(up.calls), len(d.calls))
	}

	// The chain follows: the dependent's own dependent runs when it ends, before the call returns.
	l = bfSetup(bfDay, keyA, keyC, keyB)
	up, d = upJob(keyA), dep(0, keyC)
	last := depJob{
		scheduledJob: scheduledJob{fakeJob: &fakeJob{id: "last", keys: []contracts.ExpectedKey{keyB}}, calendarID: "XNYS", anchor: "session_close"},
		after:        []string{"dep"},
	}
	if _, err := forceRec(l, afterClose(), up, d, last).BackfillWith(ctx, bfDay, nil, runner.BackfillOptions{Job: "up"}); err != nil {
		t.Fatal(err)
	}
	if len(last.calls) != 1 {
		t.Fatalf("last %d", len(last.calls))
	}
}

func TestUpstreamMissingMark(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for _, force := range []bool{false, true} {
		l := bfSetup(bfDay, keyA, keyC)
		up, d := upJob(keyA), dep(0, keyC)
		if force {
			l.present[bfDay] = map[contracts.ExpectedKey]store.PresentStatus{keyC: store.StatusPresent}
		}
		res, err := forceRec(l, afterClose(), up, d).BackfillWith(ctx, bfDay, nil, runner.BackfillOptions{Job: "dep", Force: force})
		if err != nil {
			t.Fatal(err)
		}
		line := lineOf(t, res, keyC)
		if line.ReasonCode != "upstream_missing" || len(d.calls) != 1 || len(up.calls) != 0 {
			t.Fatalf("force %v: line %+v dep %d up %d", force, line, len(d.calls), len(up.calls))
		}
		if !force {
			if line.Group != runner.GroupRan {
				t.Fatalf("line %+v", line)
			}
			if r := rowOf(t, l.fakeLedger, bfDay, keyC); r.ReasonCode != "upstream_missing" {
				t.Fatalf("row %+v: the mark replaces the backfill reason", r)
			}
		}
	}

	// An upstream with an ok run for the day leaves no mark.
	l := bfSetup(bfDay, keyA, keyC)
	l.present[bfDay] = map[contracts.ExpectedKey]store.PresentStatus{keyA: store.StatusPresent}
	d := dep(0, keyC)
	res, err := forceRec(l, afterClose(), upJob(keyA), d).BackfillWith(ctx, bfDay, nil, runner.BackfillOptions{Job: "dep"})
	if err != nil {
		t.Fatal(err)
	}
	if line := lineOf(t, res, keyC); line.ReasonCode == "upstream_missing" {
		t.Fatalf("line %+v", line)
	}
}
