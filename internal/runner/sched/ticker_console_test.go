package sched

import (
	"context"
	"errors"
	"runtime"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
)

// consoleRec is a Reconciler with the optional ConsoleTurn. block, when set, makes the turn wait for
// release or its context.
type consoleRec struct {
	*fakeRec
	turns     atomic.Int32
	cancelled atomic.Bool
	release   chan struct{}
}

func (r *consoleRec) ConsoleTurn(ctx context.Context, hold runner.SlotHold) (bool, error) {
	r.turns.Add(1)
	r.ev.emit("console")
	if r.release != nil {
		select {
		case <-r.release:
		case <-ctx.Done():
			r.cancelled.Store(true)
			r.ev.emit("console-cancelled")
		}
	}
	return false, nil
}

func holdTicker(t *testing.T) *ticker {
	t.Helper()
	h := newHarness(t)
	return New(h.fence, h.led, h.ops, tickerCal(), h.clk, h.rec, nil, nil, nil, h.exe).(*ticker)
}

// TestConsoleTurnWhenIdle (new): a wake with no due slot or tick gives the request queue a turn.
func TestConsoleTurnWhenIdle(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	rec := &consoleRec{fakeRec: h.rec.(*fakeRec)}
	h.rec = rec
	h.run()
	h.until("reconcile:2025-11-26")
	if rec.turns.Load() != 0 {
		t.Fatal("console turn on the reconcile turn")
	}
	h.step(30 * time.Second)
	h.until("console")
}

// TestSlotDueGoesBeforeRequest (new): with a due slot and a waiting request at one wake the reconcile
// turn is submitted.
func TestSlotDueGoesBeforeRequest(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	rec := &consoleRec{fakeRec: h.rec.(*fakeRec)}
	h.rec = rec
	h.jobs = []contracts.Job{tickerJob{id: "a", offset: 14*time.Hour + 20*time.Second}}
	h.run()
	h.until("reconcile:2025-11-26")
	h.step(20 * time.Second)
	got := h.until("reconcile:2025-11-26")
	if slices.Contains(got, "console") || rec.turns.Load() != 0 {
		t.Fatalf("a request went before the slot: %v", got)
	}
	h.step(30 * time.Second)
	h.until("console")
}

// TestTickDuringTurnNotMarkedCoveredAndRunsAfter (new): a tick that comes due while the turn runs finds
// the worker busy, is not marked covered and runs when the turn ends.
func TestTickDuringTurnNotMarkedCoveredAndRunsAfter(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	rec := &consoleRec{fakeRec: h.rec.(*fakeRec), release: make(chan struct{})}
	h.rec = rec
	h.run()
	h.until("reconcile:2025-11-26")
	h.step(30 * time.Second)
	h.until("console") // 14:00:30, the turn holds the worker
	h.stuck = true
	for range 19 { // to 14:10:00, the first tick
		h.step(30 * time.Second)
	}
	h.step(busyRetry) // the retry finds the worker busy again
	if got := h.pending(); slices.Contains(got, "reconcile:2025-11-26") {
		t.Fatalf("tick ran during the turn: %v", got)
	}
	close(rec.release)
	h.stuck = false
	h.step(busyRetry)
	h.until("reconcile:2025-11-26")
}

// TestHoldReportsStartInsideBound (new): a start in (now, now+bound] is held with its job and instant.
func TestHoldReportsStartInsideBound(t *testing.T) {
	t.Parallel()
	tk := holdTicker(t)
	at := tickerStart.Add(5 * time.Minute)
	tk.covered.Store(tickerStart.UnixNano())
	tk.starts.Store(&[]Start{{JobID: "a", At: at}})
	job, got, held := tk.slotHold(10 * time.Minute)
	if job != "a" || !got.Equal(at) || !held {
		t.Fatalf("hold = %q %v %v", job, got, held)
	}
}

// TestHoldIgnoresStartAfterBound (new): a start after now+bound does not hold.
func TestHoldIgnoresStartAfterBound(t *testing.T) {
	t.Parallel()
	tk := holdTicker(t)
	tk.covered.Store(tickerStart.UnixNano())
	tk.starts.Store(&[]Start{{JobID: "a", At: tickerStart.Add(5 * time.Minute)}})
	if job, _, held := tk.slotHold(4 * time.Minute); held {
		t.Fatalf("held by %q", job)
	}
}

// TestHoldTrueWhenScheduleUnread (new): no published schedule answers held with an empty job.
func TestHoldTrueWhenScheduleUnread(t *testing.T) {
	t.Parallel()
	tk := holdTicker(t)
	job, at, held := tk.slotHold(time.Minute)
	if job != "" || !at.IsZero() || !held {
		t.Fatalf("hold = %q %v %v", job, at, held)
	}
}

// TestHoldTrueForUncoveredStartAtOrBeforeNow (new): a start at or before now that the loop's published
// mark has not covered is held; a covered one is not.
func TestHoldTrueForUncoveredStartAtOrBeforeNow(t *testing.T) {
	t.Parallel()
	tk := holdTicker(t)
	early := tickerStart.Add(-10 * time.Minute)
	tk.starts.Store(&[]Start{{JobID: "a", At: early}, {JobID: "b", At: tickerStart}})
	tk.covered.Store(early.Add(-time.Minute).UnixNano())
	if job, at, held := tk.slotHold(time.Minute); job != "a" || !at.Equal(early) || !held {
		t.Fatalf("uncovered: %q %v %v", job, at, held)
	}
	tk.covered.Store(early.UnixNano())
	if job, _, held := tk.slotHold(time.Minute); job != "b" || !held {
		t.Fatalf("start at now: %q %v", job, held)
	}
	tk.covered.Store(tickerStart.UnixNano())
	if job, _, held := tk.slotHold(time.Minute); held {
		t.Fatalf("covered, held by %q", job)
	}
}

// TestOneRequestPerTurn (new): a wake gives the reconciler one ConsoleTurn call.
func TestOneRequestPerTurn(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	rec := &consoleRec{fakeRec: h.rec.(*fakeRec)}
	h.rec = rec
	h.run()
	h.until("reconcile:2025-11-26")
	h.step(30 * time.Second)
	h.until("console")
	if n := rec.turns.Load(); n != 1 {
		t.Fatalf("turns = %d, want 1", n)
	}
	h.step(30 * time.Second)
	h.until("console")
	if n := rec.turns.Load(); n != 2 {
		t.Fatalf("turns = %d, want 2", n)
	}
}

// TestStopWaitsForConsoleTurn (new): after drainGrace the turn's context is cancelled and Run returns
// after the turn.
func TestStopWaitsForConsoleTurn(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	rec := &consoleRec{fakeRec: h.rec.(*fakeRec), release: make(chan struct{})}
	h.rec = rec
	cancel, done := h.run()
	h.until("reconcile:2025-11-26")
	h.step(30 * time.Second)
	h.until("console")
	cancel()
	advanced := time.Duration(0)
	for finished := false; !finished; {
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
			finished = true
		default:
			if advanced < drainGrace && rec.cancelled.Load() {
				t.Fatalf("turn cancelled after %v, before the grace", advanced)
			}
			h.clk.Advance(time.Second)
			advanced += time.Second
			runtime.Gosched()
		}
	}
	if advanced < drainGrace {
		t.Fatalf("drained after %v, want at least %v", advanced, drainGrace)
	}
	if !rec.cancelled.Load() {
		t.Fatal("Run returned before the turn ended")
	}
}

// TestNoConsoleWithoutInterface (new): a Reconciler without ConsoleTurn gets refresh turns only.
func TestNoConsoleWithoutInterface(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.run()
	h.until("reconcile:2025-11-26")
	h.step(30 * time.Second)
	got := h.until("put")
	if slices.Contains(got, "reconcile:2025-11-26") {
		t.Fatalf("reconcile on a plain wake: %v", got)
	}
	if h.s.(*ticker).turner != nil {
		t.Fatal("console turner found on a plain Reconciler")
	}
}

// claimRec is a Reconciler with ConsoleClaimer only. The first claim returns one ConsoleRun that emits
// "run" and waits for release or its context; later claims return none.
type claimRec struct {
	*fakeRec
	claims    atomic.Int32
	cancelled atomic.Bool
	release   chan struct{}
}

func (r *claimRec) ClaimConsole(context.Context) ([]runner.ConsoleRun, error) {
	if r.claims.Add(1) > 1 {
		return nil, nil
	}
	return []runner.ConsoleRun{func(ctx context.Context) {
		r.ev.emit("run")
		select {
		case <-r.release:
		case <-ctx.Done():
			r.cancelled.Store(true)
			r.ev.emit("run-cancelled")
		}
	}}, nil
}

// TestConsoleRunDoesNotBlockWorker (new): a ConsoleRun that is still running leaves the worker free
// for a tick.
func TestConsoleRunDoesNotBlockWorker(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	rec := &claimRec{fakeRec: h.rec.(*fakeRec), release: make(chan struct{})}
	h.rec = rec
	h.run()
	h.until("reconcile:2025-11-26")
	h.step(30 * time.Second)
	h.until("run")
	// The first tick is 14:10:00; a loop that took a wake late sees it one step later, so step until it ran.
	for ran := false; !ran; {
		h.step(30 * time.Second)
		ran = slices.Contains(h.pending(), "reconcile:2025-11-26")
	}
	close(rec.release)
}

// TestConsoleRunsDrainedAtStop (new): after a stop Run waits drainGrace for a ConsoleRun, cancels its
// context, and returns after it ended.
func TestConsoleRunsDrainedAtStop(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	rec := &claimRec{fakeRec: h.rec.(*fakeRec), release: make(chan struct{})}
	h.rec = rec
	cancel, done := h.run()
	h.until("reconcile:2025-11-26")
	h.step(30 * time.Second)
	h.until("run")
	cancel()
	advanced := time.Duration(0)
	for finished := false; !finished; {
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
			finished = true
		default:
			if advanced < drainGrace && rec.cancelled.Load() {
				t.Fatalf("run cancelled after %v, before the grace", advanced)
			}
			// Past the grace, give the run a chance to see its cancel before the clock moves on.
			for range 10000 {
				if advanced < drainGrace || rec.cancelled.Load() {
					break
				}
				runtime.Gosched()
			}
			h.clk.Advance(time.Second)
			advanced += time.Second
			runtime.Gosched()
		}
	}
	if advanced < drainGrace {
		t.Fatalf("drained after %v, want at least %v", advanced, drainGrace)
	}
	if !rec.cancelled.Load() {
		t.Fatal("Run returned before the ConsoleRun ended")
	}
}

// TestConsoleRunsCancelledOnFenceLoss (new): a lost fence cancels the context of a ConsoleRun at once,
// with no grace of the clock.
func TestConsoleRunsCancelledOnFenceLoss(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	rec := &claimRec{fakeRec: h.rec.(*fakeRec), release: make(chan struct{})}
	h.rec = rec
	_, done := h.run()
	h.until("reconcile:2025-11-26")
	h.step(30 * time.Second)
	h.until("run")
	close(h.fence.lost)
	if err := <-done; !errors.Is(err, ErrFenceLost) {
		t.Fatalf("Run = %v, want ErrFenceLost", err)
	}
	if !rec.cancelled.Load() {
		t.Fatal("the ConsoleRun was not cancelled")
	}
}
