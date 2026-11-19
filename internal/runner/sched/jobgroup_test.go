package sched

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
)

// TestRunTimeoutEndsCutAndLeavesKeys (new): a run that reaches reconcileTimeout on the fake clock
// sees DeadlineExceeded, ends cut at its next key and marks nothing more; the keys stay for the next pass.
func TestRunTimeoutEndsCutAndLeavesKeys(t *testing.T) {
	t.Parallel()
	clk := clockwork.NewFakeClockAt(tickerStart)
	g := newJobGroup(clk)
	t.Cleanup(func() { g.cancel(nil) })

	var mu sync.Mutex
	marked := map[string]bool{}
	outcome := ""
	var cause, errAt error
	atKey := make(chan struct{})
	g.spawn("J", func(ctx context.Context) {
		for i, key := range []string{"k1", "k2", "k3"} {
			if ctx.Err() != nil {
				mu.Lock()
				outcome, cause, errAt = "cut", context.Cause(ctx), ctx.Err()
				mu.Unlock()
				return
			}
			mu.Lock()
			marked[key] = true
			mu.Unlock()
			if i == 0 {
				close(atKey)
				<-ctx.Done() // the second key is slow: only the timeout of the run ends it
			}
		}
		mu.Lock()
		outcome = "ok"
		mu.Unlock()
	})
	<-atKey
	clk.Advance(reconcileTimeout)
	<-g.done()

	mu.Lock()
	defer mu.Unlock()
	if !errors.Is(errAt, context.DeadlineExceeded) {
		t.Fatalf("ctx.Err() after the timeout = %v, want DeadlineExceeded", errAt)
	}
	if outcome != "cut" || !errors.Is(cause, context.DeadlineExceeded) {
		t.Fatalf("outcome=%q cause=%v, want cut with DeadlineExceeded", outcome, cause)
	}
	if !marked["k1"] || marked["k2"] || marked["k3"] {
		t.Fatalf("marked = %v, want only k1; the others stay for the next pass", marked)
	}
}

// TestSpawnedRunHasDeadline (from the review): a spawned run carries the deadline of reconcileTimeout.
func TestSpawnedRunHasDeadline(t *testing.T) {
	t.Parallel()
	clk := clockwork.NewFakeClockAt(tickerStart)
	g := newJobGroup(clk)
	t.Cleanup(func() { g.cancel(nil) })
	got := make(chan time.Time, 1)
	g.spawn("J", func(ctx context.Context) {
		d, ok := ctx.Deadline()
		if !ok {
			d = time.Time{}
		}
		got <- d
	})
	if d := <-got; !d.Equal(tickerStart.Add(reconcileTimeout)) {
		t.Fatalf("deadline = %v, want %v", d, tickerStart.Add(reconcileTimeout))
	}
	<-g.done()
}
