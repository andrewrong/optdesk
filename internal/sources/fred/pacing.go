package fred

import (
	"sync"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
)

// requestInterval is the pacing rule: at most one request per second.
const requestInterval = time.Second

// pacer hands out request slots at least requestInterval apart, on the injected clock.
type pacer struct {
	mu   sync.Mutex
	clk  clock.Clock
	next time.Time
}

func newPacer(clk clock.Clock) *pacer { return &pacer{clk: clk} }

// reserve takes the next slot and returns how long the caller must wait before using it.
// It never sleeps; the caller waits (and may be cancelled).
func (p *pacer) reserve() time.Duration {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := p.clk.Now()
	slot := now
	if p.next.After(now) {
		slot = p.next
	}
	p.next = slot.Add(requestInterval)
	return slot.Sub(now)
}
