package flex

import (
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
)

func TestPacerSevenSecondsOnFakeClock(t *testing.T) {
	t.Parallel()
	clk := clockwork.NewFakeClock()
	p := &pacer{}
	if d := p.reserve(clk.Now()); d != 0 {
		t.Fatalf("first waits %v", d)
	}
	if d := p.reserve(clk.Now()); d != 7*time.Second {
		t.Fatalf("second waits %v, want 7s", d)
	}
	if d := p.reserve(clk.Now()); d != 14*time.Second {
		t.Fatalf("third waits %v, want 14s", d)
	}
	clk.Advance(30 * time.Second)
	if d := p.reserve(clk.Now()); d != 0 {
		t.Fatalf("after idle waits %v", d)
	}
	clk.Advance(2 * time.Second)
	if d := p.reserve(clk.Now()); d != 5*time.Second {
		t.Fatalf("waits %v, want 5s", d)
	}
}
