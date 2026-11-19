package sched

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/runner/shared"
)

// endState is what a blocked fake run saw when it ended.
type endState struct {
	ended atomic.Bool
	cause atomic.Value // error
}

func (e *endState) cancelCause() error {
	v, _ := e.cause.Load().(error)
	return v
}

// detachedRec is a Reconciler with ReconcileDetached only. Pass n starts the jobs of passes[n]; each
// run emits "start:<job>" and blocks until release or its context, which it records in ends[job].
type detachedRec struct {
	*fakeRec
	passes  [][]string
	release chan struct{}
	mu      sync.Mutex
	pass    int
	ends    map[string]*endState
}

func newDetachedRec(base *fakeRec, passes ...[]string) *detachedRec {
	r := &detachedRec{fakeRec: base, passes: passes, release: make(chan struct{}), ends: map[string]*endState{}}
	for _, p := range passes {
		for _, id := range p {
			r.ends[id] = &endState{}
		}
	}
	return r
}

func (r *detachedRec) ReconcileDetached(_ context.Context, _ contracts.Day, spawn runner.Spawn) error {
	r.mu.Lock()
	var ids []string
	if r.pass < len(r.passes) {
		ids = r.passes[r.pass]
	}
	r.pass++
	r.mu.Unlock()
	for _, id := range ids {
		end := r.ends[id]
		spawn(id, func(ctx context.Context) {
			r.ev.emit("start:" + id)
			select {
			case <-r.release:
			case <-ctx.Done():
				end.cause.Store(context.Cause(ctx))
			}
			end.ended.Store(true)
		})
	}
	return nil
}

func (r *detachedRec) endedCount() int {
	n := 0
	for _, e := range r.ends {
		if e.ended.Load() {
			n++
		}
	}
	return n
}

// TestSlotOfJobBStartsWhileJobAIsRunning (new): a due slot of B starts although A's run is blocked.
func TestSlotOfJobBStartsWhileJobAIsRunning(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	rec := newDetachedRec(h.rec.(*fakeRec), []string{"A"}, []string{"B"})
	h.rec = rec
	h.run()
	h.until("start:A")
	h.step(firstTickDelay) // the first tick: the second pass
	h.until("start:B")
	if rec.ends["A"].ended.Load() {
		t.Fatal("A ended before B started")
	}
	close(rec.release)
}

// TestDrainGivesOneGraceToAllRuns (new): after a stop two blocked runs get drainGrace together, then
// both are cancelled with ErrOrderedStop and Run returns after they ended.
func TestDrainGivesOneGraceToAllRuns(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	rec := newDetachedRec(h.rec.(*fakeRec), []string{"A", "B"})
	h.rec = rec
	cancel, done := h.run()
	for started := map[string]bool{}; len(started) < 2; { // the two runs start in either order
		if e := <-h.ev; strings.HasPrefix(e, "start:") {
			started[e] = true
		}
	}
	cancel()
	var advanced time.Duration
	for len(done) == 0 {
		if n := rec.endedCount(); n > 0 {
			if advanced < drainGrace {
				t.Fatalf("a run was cancelled after %v, before the grace", advanced)
			}
			// One cancel for all: the other run ends with it, not a grace later.
			for range 100000 {
				if rec.endedCount() == 2 || len(done) > 0 {
					break
				}
				runtime.Gosched()
			}
			if rec.endedCount() != 2 {
				t.Fatalf("runs ended apart at %v", advanced)
			}
			break
		}
		h.clk.Advance(time.Second)
		advanced += time.Second
		// Past the grace, let the runs see their cancel before the clock moves on to the kill grace.
		for range 100000 {
			if advanced < drainGrace || len(done) > 0 || rec.endedCount() > 0 {
				break
			}
			runtime.Gosched()
		}
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if advanced < drainGrace {
		t.Fatalf("drained after %v, want at least %v", advanced, drainGrace)
	}
	for id, e := range rec.ends {
		if !e.ended.Load() || !errors.Is(e.cancelCause(), shared.ErrOrderedStop) {
			t.Fatalf("run %s: ended=%v cause=%v, want ErrOrderedStop (after %v)", id, e.ended.Load(), e.cancelCause(), advanced)
		}
	}
}

// TestLostFenceCancelsGroupAtOnce (new): a lost fence cancels a spawned run with no grace of the
// clock, and not with the ordered-stop cause.
func TestLostFenceCancelsGroupAtOnce(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	rec := newDetachedRec(h.rec.(*fakeRec), []string{"A"})
	h.rec = rec
	_, done := h.run()
	h.until("start:A")
	close(h.fence.lost)
	if err := <-done; !errors.Is(err, ErrFenceLost) {
		t.Fatalf("Run = %v, want ErrFenceLost", err)
	}
	e := rec.ends["A"]
	if !e.ended.Load() || !errors.Is(e.cancelCause(), context.Canceled) || errors.Is(e.cancelCause(), shared.ErrOrderedStop) {
		t.Fatalf("run A: ended=%v cause=%v, want a plain cancel", e.ended.Load(), e.cancelCause())
	}
}
