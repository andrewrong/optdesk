package flex

import (
	"sync"
	"time"
)

// requestInterval is the pacing rule: any two requests of the process to the service are at least
// this far apart.
const requestInterval = 7 * time.Second

// pacer hands out request slots at least requestInterval apart.
type pacer struct {
	mu   sync.Mutex
	next time.Time
}

// sharedPacer is the one pacer of every client (every login) of the process.
var sharedPacer = &pacer{}

// reserve takes the next slot, given the current instant of the injected Clock, and returns how
// long the caller must wait before using it. It never sleeps.
func (p *pacer) reserve(now time.Time) time.Duration {
	p.mu.Lock()
	defer p.mu.Unlock()
	slot := now
	if p.next.After(now) {
		slot = p.next
	}
	p.next = slot.Add(requestInterval)
	return slot.Sub(now)
}
