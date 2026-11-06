package fred

import (
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
)

func TestPacing(t *testing.T) {
	t.Parallel()
	clk := clockwork.NewFakeClock()
	p := newPacer(clk)
	if d := p.reserve(); d != 0 {
		t.Fatalf("first request waits %v", d)
	}
	if d := p.reserve(); d != time.Second {
		t.Fatalf("second request waits %v, want 1s", d)
	}
	if d := p.reserve(); d != 2*time.Second {
		t.Fatalf("third request waits %v, want 2s", d)
	}
	clk.Advance(3 * time.Second)
	if d := p.reserve(); d != 0 {
		t.Fatalf("after idle waits %v", d)
	}
	clk.Advance(400 * time.Millisecond)
	if d := p.reserve(); d != 600*time.Millisecond {
		t.Fatalf("waits %v, want 600ms", d)
	}
}
