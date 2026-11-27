package sched

import (
	"context"
	"errors"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
)

// cancelSignal lets a test follow a turn or run that sees its cancel. sawCancel, when set, is closed once when
// it sees ctx.Done(). exit, when set, then holds it until the test closes it, so Run is seen waiting for it.
// exited is set when it is about to return.
type cancelSignal struct {
	sawCancel chan struct{}
	exit      chan struct{}
	exited    atomic.Bool
	once      sync.Once
}

func newCancelSignal() cancelSignal {
	return cancelSignal{sawCancel: make(chan struct{}), exit: make(chan struct{})}
}

// sawDone is called by the turn or run on ctx.Done().
func (c *cancelSignal) sawDone() {
	c.once.Do(func() {
		if c.sawCancel != nil {
			close(c.sawCancel)
		}
	})
	if c.exit != nil {
		<-c.exit
	}
	c.exited.Store(true)
}

// consoleRec is a Reconciler with the optional ConsoleTurn. block, when set, makes the turn wait for
// release or its context.
type consoleRec struct {
	*fakeRec
	turns     atomic.Int32
	cancelled atomic.Bool
	release   chan struct{}
	cancelSignal
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
			r.sawDone()
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

// drainSpy is the clock of a ticker under test. Once armed, it reports the first timer made by waitDone, the
// wait of Run's drain; the loop makes timers of its own, also after the stop was ordered, and those do not count.
type drainSpy struct {
	clockwork.Clock
	armed atomic.Bool
	waits atomic.Int32
	drain chan struct{} // closed when Run makes the drain timer
	kill  chan struct{} // closed when Run makes the timer for the grace after the cancel
}

func (s *drainSpy) NewTimer(d time.Duration) clockwork.Timer {
	tm := s.Clock.NewTimer(d)
	if s.armed.Load() && calledFrom("(*ticker).waitDone") {
		switch s.waits.Add(1) {
		case 1:
			close(s.drain)
		case 2:
			close(s.kill)
		}
	}
	return tm
}

// calledFrom reports whether a function whose name ends in name is on the stack.
func calledFrom(name string) bool {
	pcs := make([]uintptr, 16)
	frames := runtime.CallersFrames(pcs[:runtime.Callers(2, pcs)])
	for f, more := frames.Next(); ; f, more = frames.Next() {
		if strings.HasSuffix(f.Function, name) {
			return true
		}
		if !more {
			return false
		}
	}
}

// runSpied is harness.run with the clock of the ticker wrapped in a drainSpy.
func runSpied(h *harness) (cancel context.CancelFunc, done <-chan error, spy *drainSpy) {
	h.t.Helper()
	spy = &drainSpy{Clock: h.clk, drain: make(chan struct{}), kill: make(chan struct{})}
	h.s = New(h.fence, h.led, h.ops, tickerCal(), spy, h.rec, h.gaps, h.jobs, h.disabled, h.exe)
	ctx, cancel := context.WithCancel(context.Background())
	h.t.Cleanup(cancel)
	ch := make(chan error, 1)
	go func() { ch <- h.s.Run(ctx) }()
	h.until("put")
	return cancel, ch, spy
}

// stopAndDrain stops Run and moves the fake clock one second at a time once Run waits for its drain. Nothing
// may see its cancel before drainGrace has passed; once it has, the clock stops and the test waits on sig.sawCancel.
// The turn is then held, so Run must still be waiting for it: Run returning now is a failure. The turn is released
// and Run must return after it ended. It returns the time advanced.
func stopAndDrain(t *testing.T, h *harness, cancel context.CancelFunc, done <-chan error, spy *drainSpy,
	sig *cancelSignal, cancelled *atomic.Bool) time.Duration {
	t.Helper()
	// The loop is asleep on its timer; from here on the next timer is Run's drain timer.
	if err := h.clk.BlockUntilContext(t.Context(), 1); err != nil {
		t.Fatal(err)
	}
	spy.armed.Store(true)
	cancel()
	<-spy.drain
	advanced := time.Duration(0)
	for advanced < drainGrace {
		if cancelled.Load() {
			t.Fatalf("cancelled after %v, before the grace", advanced)
		}
		h.clk.Advance(time.Second)
		advanced += time.Second
	}
	<-sig.sawCancel
	<-spy.kill // Run waits for the turn, with the clock stopped
	select {
	case <-done:
		t.Fatal("Run returned before the turn ended")
	default:
	}
	close(sig.exit)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if !sig.exited.Load() {
		t.Fatal("Run returned before the turn ended")
	}
	return advanced
}

// TestStopWaitsForConsoleTurn (new): after drainGrace the turn's context is cancelled and Run returns
// after the turn.
func TestStopWaitsForConsoleTurn(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	rec := &consoleRec{fakeRec: h.rec.(*fakeRec), release: make(chan struct{}), cancelSignal: newCancelSignal()}
	h.rec = rec
	cancel, done, spy := runSpied(h)
	h.until("reconcile:2025-11-26")
	h.step(30 * time.Second)
	h.until("console")
	advanced := stopAndDrain(t, h, cancel, done, spy, &rec.cancelSignal, &rec.cancelled)
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
	cancelSignal
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
			r.sawDone()
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
	rec := &claimRec{fakeRec: h.rec.(*fakeRec), release: make(chan struct{}), cancelSignal: newCancelSignal()}
	h.rec = rec
	cancel, done, spy := runSpied(h)
	h.until("reconcile:2025-11-26")
	h.step(30 * time.Second)
	h.until("run")
	advanced := stopAndDrain(t, h, cancel, done, spy, &rec.cancelSignal, &rec.cancelled)
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
