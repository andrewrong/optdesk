package shared

import (
	"sync"
	"time"

	"github.com/andrewrong/optdesk/internal/clock/realclock"
	"github.com/andrewrong/optdesk/internal/runner"
)

var (
	_ runner.JobLocks      = (*jobLocks)(nil)
	_ runner.JobLockHolder = (*jobLocks)(nil)
)

type jobLocks struct {
	now    func() time.Time
	mu     sync.Mutex
	locked map[string]time.Time // job id to the instant it was locked
}

// NewJobLocks builds an empty set of per-job locks. TryLock never blocks; the unlock
// func releases the job and may be called more than once.
func NewJobLocks() runner.JobLocks { return NewJobLocksAt(realclock.NewReal().Now) }

// NewJobLocksAt is NewJobLocks with the time source that Holder reports (a fake clock in tests).
func NewJobLocksAt(now func() time.Time) runner.JobLocks {
	return &jobLocks{now: now, locked: map[string]time.Time{}}
}

// Holder gives the instant jobID was locked; ok is false while it is free.
func (l *jobLocks) Holder(jobID string) (since time.Time, ok bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	since, ok = l.locked[jobID]
	return since, ok
}

func (l *jobLocks) TryLock(jobID string) (func(), bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, held := l.locked[jobID]; held {
		return nil, false
	}
	l.locked[jobID] = l.now()
	var once sync.Once
	return func() {
		once.Do(func() {
			l.mu.Lock()
			delete(l.locked, jobID)
			l.mu.Unlock()
		})
	}, true
}
