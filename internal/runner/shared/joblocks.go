package shared

import (
	"sync"

	"github.com/andrewrong/optdesk/internal/runner"
)

var _ runner.JobLocks = (*jobLocks)(nil)

type jobLocks struct {
	mu     sync.Mutex
	locked map[string]bool
}

// NewJobLocks builds an empty set of per-job locks. TryLock never blocks; the unlock
// func releases the job and may be called more than once.
func NewJobLocks() runner.JobLocks {
	return &jobLocks{locked: map[string]bool{}}
}

func (l *jobLocks) TryLock(jobID string) (func(), bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.locked[jobID] {
		return nil, false
	}
	l.locked[jobID] = true
	var once sync.Once
	return func() {
		once.Do(func() {
			l.mu.Lock()
			delete(l.locked, jobID)
			l.mu.Unlock()
		})
	}, true
}
