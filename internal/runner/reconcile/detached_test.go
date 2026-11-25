package reconcile

import (
	"context"
	"sync"
	"testing"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/runner/shared"
	"github.com/andrewrong/optdesk/internal/store"
)

const detDay contracts.Day = "2026-11-04"

// spawnLog is a Spawn that starts each run in a goroutine, or inline, and records the job ids.
type spawnLog struct {
	mu     sync.Mutex
	ids    []string
	inline bool
	wg     sync.WaitGroup
	done   map[string]chan struct{}
}

func newSpawnLog(inline bool) *spawnLog {
	return &spawnLog{inline: inline, done: map[string]chan struct{}{}}
}

func (s *spawnLog) spawn(id string, run func(ctx context.Context)) {
	s.mu.Lock()
	s.ids = append(s.ids, id)
	ch := make(chan struct{})
	s.done[id] = ch
	s.mu.Unlock()
	if s.inline {
		run(context.Background())
		close(ch)
		return
	}
	s.wg.Go(func() {
		defer close(ch)
		run(context.Background())
	})
}

func (s *spawnLog) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.ids)
}

// gateJob is a fakeJob whose Run waits for release; started is closed when the run begins.
type gateJob struct {
	*fakeJob
	started chan struct{}
	release chan struct{}
}

func (j *gateJob) Run(ctx context.Context, d contracts.Day, k []contracts.ExpectedKey) contracts.JobResult {
	close(j.started)
	<-j.release
	return j.fakeJob.Run(ctx, d, k)
}

// planCount is a fakeJob that counts its Plan calls.
type planCount struct {
	*fakeJob
	mu sync.Mutex
	n  int
}

func (j *planCount) Plan(ctx context.Context, d contracts.Day) ([]contracts.ExpectedKey, string, error) {
	j.mu.Lock()
	j.n++
	j.mu.Unlock()
	return j.fakeJob.Plan(ctx, d)
}

func detRec(led *fakeLedger, clk clockwork.Clock, jobs ...contracts.Job) *reconciler {
	led.cursor = store.Cursor{Day: "2026-11-03"}
	return forceRec(led, clk, jobs...)
}

func detClock() clockwork.Clock {
	return clockwork.NewFakeClockAt(utc("2026-11-04T22:00:00Z"))
}

func presentOf(t *testing.T, l *fakeLedger, k contracts.ExpectedKey) bool {
	t.Helper()
	p, err := l.Present(context.Background(), detDay)
	if err != nil {
		t.Fatal(err)
	}
	_, ok := p[k]
	return ok
}

func TestDetachedReturnsBeforeJobsEnd(t *testing.T) {
	t.Parallel()
	led := newLedger()
	slow := &gateJob{fakeJob: &fakeJob{id: "slow", keys: []contracts.ExpectedKey{keyA}}, started: make(chan struct{}), release: make(chan struct{})}
	fast := &fakeJob{id: "fast", keys: []contracts.ExpectedKey{keyB}}
	rec := detRec(led, detClock(), slow, fast)
	sp := newSpawnLog(false)

	if err := rec.ReconcileDetached(context.Background(), detDay, sp.spawn); err != nil {
		t.Fatal(err)
	}
	<-slow.started
	<-sp.done["fast"]
	if !presentOf(t, led, keyB) {
		t.Fatal("the second job's key is not marked")
	}
	if presentOf(t, led, keyA) {
		t.Fatal("the blocked job's key is marked before it ended")
	}
	if led.cursor.Day != "2026-11-03" {
		t.Fatalf("cursor = %q, a pass that launched runs sets none", led.cursor.Day)
	}
	close(slow.release)
	sp.wg.Wait()
	if !presentOf(t, led, keyA) {
		t.Fatal("the blocked job's key is not marked after it was released")
	}
}

func TestDetachedSkipsKeysOfHeldJob(t *testing.T) {
	t.Parallel()
	led := newLedger()
	held := &planCount{fakeJob: &fakeJob{id: "held", keys: []contracts.ExpectedKey{keyA}}}
	free := &planCount{fakeJob: &fakeJob{id: "free", keys: []contracts.ExpectedKey{keyB}}}
	rec := detRec(led, detClock(), held, free)
	unlock, ok := rec.locks.TryLock("held")
	if !ok {
		t.Fatal("lock refused")
	}
	defer unlock()
	sp := newSpawnLog(true)

	if err := rec.ReconcileDetached(context.Background(), detDay, sp.spawn); err != nil {
		t.Fatal(err)
	}
	if held.n != 0 {
		t.Fatalf("the held job was planned %d times", held.n)
	}
	if sp.count() != 1 || sp.ids[0] != "free" {
		t.Fatalf("spawned %v, want only the free job", sp.ids)
	}
	if len(held.calls) != 0 || !presentOf(t, led, keyB) {
		t.Fatal("the held job ran, or the free job did not")
	}
	if len(led.expected[detDay]) != 0 {
		t.Fatalf("expected keys written from a plan that left a job out: %v", led.expected[detDay])
	}
	if led.cursor.Day != "2026-11-03" {
		t.Fatalf("cursor = %q, a day with a held job is not done", led.cursor.Day)
	}
}

// unlockProbe is a lock set whose unlock reads the ledger first.
type unlockProbe struct {
	runner.JobLocks
	led *fakeLedger
	key contracts.ExpectedKey
	saw chan bool
}

func (u *unlockProbe) TryLock(id string) (func(), bool) {
	unlock, ok := u.JobLocks.TryLock(id)
	if !ok {
		return unlock, ok
	}
	return func() {
		p, _ := u.led.Present(context.Background(), detDay)
		_, marked := p[u.key]
		u.saw <- marked
		unlock()
	}, ok
}

func TestRunMarksBeforeUnlock(t *testing.T) {
	t.Parallel()
	led := newLedger()
	rec := detRec(led, detClock(), &fakeJob{keys: []contracts.ExpectedKey{keyA}})
	inner := shared.NewJobLocks()
	probe := &unlockProbe{JobLocks: inner, led: led, key: keyA, saw: make(chan bool, 4)}
	rec.locks = probe

	if err := rec.ReconcileDetached(context.Background(), detDay, newSpawnLog(true).spawn); err != nil {
		t.Fatal(err)
	}
	if !<-probe.saw {
		t.Fatal("the lock was released before the key was marked")
	}
	if _, held := inner.(runner.JobLockHolder).Holder("fake"); held {
		t.Fatal("the lock is still held after the run")
	}
}

func TestDetachedAndReconcileSameCursor(t *testing.T) {
	t.Parallel()
	mk := func() (*fakeLedger, *reconciler) {
		led := newLedger()
		return led, detRec(led, detClock(), &fakeJob{keys: []contracts.ExpectedKey{keyA, keyB}})
	}
	oldLed, oldRec := mk()
	if err := oldRec.Reconcile(context.Background(), detDay); err != nil {
		t.Fatal(err)
	}
	newLed, newRec := mk()
	sp := newSpawnLog(true)
	for range 2 { // the first pass launches, the second finds the marks
		if err := newRec.ReconcileDetached(context.Background(), detDay, sp.spawn); err != nil {
			t.Fatal(err)
		}
	}
	if oldLed.cursor != newLed.cursor || newLed.cursor.Day != detDay {
		t.Fatalf("cursors differ: Reconcile %+v, ReconcileDetached %+v", oldLed.cursor, newLed.cursor)
	}
	if sp.count() != 1 {
		t.Fatalf("spawned %d runs, want 1 (the second pass finds nothing due)", sp.count())
	}
}

func TestDetachedCursorWaitsForMarks(t *testing.T) {
	t.Parallel()
	led := newLedger()
	job := &gateJob{fakeJob: &fakeJob{keys: []contracts.ExpectedKey{keyA}}, started: make(chan struct{}), release: make(chan struct{})}
	rec := detRec(led, detClock(), job)
	sp := newSpawnLog(false)
	pass := func() {
		t.Helper()
		if err := rec.ReconcileDetached(context.Background(), detDay, sp.spawn); err != nil {
			t.Fatal(err)
		}
	}
	pass()
	<-job.started
	pass() // the run holds the lock: nothing is planned or launched
	if led.cursor.Day != "2026-11-03" || sp.count() != 1 {
		t.Fatalf("cursor %q, spawned %d: a pass over a held job sets no cursor and starts nothing", led.cursor.Day, sp.count())
	}
	close(job.release)
	sp.wg.Wait()
	if led.cursor.Day != "2026-11-03" {
		t.Fatal("the run set the cursor")
	}
	pass()
	if led.cursor.Day != detDay {
		t.Fatalf("cursor = %q after the marks were found, want %s", led.cursor.Day, detDay)
	}
}
