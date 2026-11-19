package main

import (
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner/shared"
	"github.com/andrewrong/optdesk/internal/store/jobrequest"
)

// jcLocks is a fake lock set that reports the jobs in held as locked since the given instant.
type jcLocks struct{ held map[string]time.Time }

func (l *jcLocks) TryLock(job string) (func(), bool) {
	if _, busy := l.held[job]; busy {
		return nil, false
	}
	l.held[job] = time.Time{}
	return func() { delete(l.held, job) }, true
}

func (l *jcLocks) Holder(job string) (time.Time, bool) { at, ok := l.held[job]; return at, ok }

// jcBusy gives the rig a lock set in which the jobs are held since 14:30 UTC.
func jcBusy(r *jcRig, jobs ...string) *jcLocks {
	l := &jcLocks{held: map[string]time.Time{}}
	for _, j := range jobs {
		l.held[j] = time.Date(2026, 11, 12, 14, 30, 0, 0, time.UTC)
	}
	var js []contracts.Job
	for _, id := range []string{"fred_series", "option_ledger"} {
		js = append(js, jcJob{id: id, keys: jcKey})
	}
	r.host.set(js, r.bf)
	giveHostLocks(r.host, l)
	return l
}

func TestClaimSkipsBusyJobTakesNext(t *testing.T) {
	t.Parallel()
	r := newJCRig(t)
	r.add(1, "fred_series", jcDay, false)
	r.add(2, "option_ledger", jcDay, false)
	jcBusy(r, "fred_series")
	if !r.turn(jcNever) {
		t.Fatalf("nothing taken; calls %v", r.log.items)
	}
	if r.log.count("hold:1:fred_series:2026-11-12T14:30:00Z") != 1 || r.log.count("claim:1") != 0 || r.log.count("claim:2") != 1 {
		t.Fatalf("calls %v", r.log.items)
	}
	if f := r.wantEnd(jobrequest.StateDone, "completed"); f.id != 2 || len(r.q.finishes) != 1 {
		t.Fatalf("finished %+v", r.q.finishes)
	}
}

func TestWaitingRequestStartsWhenFree(t *testing.T) {
	t.Parallel()
	r := newJCRig(t)
	r.add(1, "fred_series", jcDay, false)
	locks := jcBusy(r, "fred_series")
	if r.turn(jcNever) || len(r.q.finishes) != 0 || r.log.count("claim:") != 0 || r.log.count("hold:1:fred_series:") != 1 {
		t.Fatalf("a waiting request was claimed or ended; calls %v", r.log.items)
	}
	r.fc.Advance(3 * time.Hour) // no expiry
	if r.turn(jcNever) || len(r.q.finishes) != 0 || r.q.rows[0].State != jobrequest.StateQueued {
		t.Fatalf("waiting request ended; calls %v", r.log.items)
	}
	delete(locks.held, "fred_series")
	if !r.turn(jcNever) || r.log.count("claim:1") != 1 {
		t.Fatalf("not claimed when free; calls %v", r.log.items)
	}
	r.wantEnd(jobrequest.StateDone, "completed")
}

func TestWithdrawWhileWaiting(t *testing.T) {
	t.Parallel()
	r := newJCRig(t)
	r.add(1, "fred_series", jcDay, false)
	jcBusy(r, "fred_series")
	r.turn(jcNever)
	r.q.rows[0].WithdrawnAt = r.fc.Now()
	if !r.turn(jcNever) {
		t.Fatalf("withdrawal not taken; calls %v", r.log.items)
	}
	if f := r.wantEnd(jobrequest.StateWithdrawn, "withdrawn"); f.id != 1 || r.log.count("hold:") != 1 || r.log.count("claim:") != 0 {
		t.Fatalf("finished %+v; calls %v", f, r.log.items)
	}
}

func TestSchedulerWiresLockSetIntoHost(t *testing.T) {
	t.Parallel()
	giveHostLocks(nil, shared.NewJobLocks()) // console off: no panic
	r := newJCRig(t, "fred_series", "option_ledger")
	r.add(1, "fred_series", jcDay, false)
	r.add(2, "option_ledger", jcDay, false)
	locks := shared.NewJobLocks()
	giveHostLocks(r.host, nil) // a nil set is tolerated
	if _, ok := locks.TryLock("fred_series"); !ok {
		t.Fatal("lock not taken")
	}
	giveHostLocks(r.host, locks)
	took, run := r.host.claimQueued(t.Context())
	if !took || run == nil {
		t.Fatalf("B not claimed; calls %v", r.log.items)
	}
	if r.log.count("hold:1:fred_series:") != 1 || r.log.count("claim:1") != 0 || r.log.count("claim:2") != 1 {
		t.Fatalf("calls %v", r.log.items)
	}
	if r.q.rows[0].State != jobrequest.StateQueued {
		t.Fatalf("A is %s, want queued", r.q.rows[0].State)
	}
}
