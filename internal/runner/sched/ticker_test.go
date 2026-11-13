package sched

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"maps"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/jonboulle/clockwork"
)

// tickerJob is a job with a plain schedule; the optional methods are set by fields.
type tickerJob struct {
	id        string
	offset    time.Duration
	sensitive bool
	closeOff  time.Duration
}

func (j tickerJob) ID() string { return j.id }
func (j tickerJob) Plan(context.Context, contracts.Day) ([]contracts.ExpectedKey, string, error) {
	return nil, "", nil
}
func (j tickerJob) Run(context.Context, contracts.Day, []contracts.ExpectedKey) contracts.JobResult {
	return contracts.JobResult{}
}
func (j tickerJob) Schedule() (string, string, string, time.Duration) {
	return "XNYS", "time", "UTC", j.offset
}
func (j tickerJob) RestartSensitive() bool { return j.sensitive }
func (j tickerJob) Window() (string, time.Duration, time.Duration) {
	return "XNYS", 0, j.closeOff
}

// fakeRec is a runner.Reconciler; block, when set, makes Reconcile wait for its context.
type fakeRec struct {
	ev       events
	block    bool
	err      error // when set, Reconcile returns it
	days     []contracts.Day
	mu       sync.Mutex
	inFlight atomic.Int32
	overlap  atomic.Bool
}

func (r *fakeRec) Reconcile(ctx context.Context, day contracts.Day) error {
	if r.err != nil {
		r.ev.emit("reconcile:" + string(day))
		return r.err
	}
	r.inFlight.Add(1)
	defer r.inFlight.Add(-1)
	r.mu.Lock()
	r.days = append(r.days, day)
	r.mu.Unlock()
	r.ev.emit("reconcile:" + string(day))
	if r.block {
		<-ctx.Done()
		r.ev.emit("reconcile-cancelled")
	}
	return nil
}

func (r *fakeRec) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.days)
}

// probeRec adds Probe: reasons[id] holds the answers of its successive calls, then it passes.
type probeRec struct {
	*fakeRec
	mu      sync.Mutex
	reasons map[string][]string
}

func (r *probeRec) Probe(_ context.Context, id string) string {
	if r.inFlight.Load() != 0 {
		r.overlap.Store(true)
	}
	r.ev.emit("probe:" + id)
	r.mu.Lock()
	defer r.mu.Unlock()
	q := r.reasons[id]
	if len(q) == 0 {
		return ""
	}
	r.reasons[id] = q[1:]
	return q[0]
}

// fakeGaps records each Sync with the map it was given.
type fakeGaps struct {
	ev   events
	mu   sync.Mutex
	seen []syncCall
	hook func(day contracts.Day, disabled map[string]string)
}

type syncCall struct {
	day      contracts.Day
	disabled map[string]string
}

func (g *fakeGaps) Sync(_ context.Context, day contracts.Day, disabled map[string]string) error {
	g.mu.Lock()
	g.seen = append(g.seen, syncCall{day, disabled})
	g.mu.Unlock()
	if g.hook != nil {
		g.hook(day, disabled)
	}
	g.ev.emit("sync:" + string(day))
	return nil
}

func (g *fakeGaps) calls() []syncCall {
	g.mu.Lock()
	defer g.mu.Unlock()
	return slices.Clone(g.seen)
}

// harness wires one ticker over the fakes. The clock starts at 14:00 UTC on Wednesday 2025-11-26.
type harness struct {
	t        *testing.T
	ev       events
	clk      *clockwork.FakeClock
	fence    *fakeFence
	led      *fakeLedger
	ops      *fakeOps
	rec      runner.Reconciler
	gaps     runner.GapSyncer
	jobs     []contracts.Job
	disabled map[string]string
	exe      runner.ExecIdentityFunc
	s        runner.Scheduler
	log      *slog.Logger
	stuck    bool // the worker is stuck on purpose: step does not wait for it
}

var tickerStart = utc("2025-11-26T14:00:00Z")

func newHarness(t *testing.T) *harness {
	t.Helper()
	ev := make(events, 4096)
	return &harness{
		t: t, ev: ev, clk: clockwork.NewFakeClockAt(tickerStart), fence: newFakeFence(),
		led: &fakeLedger{ev: ev}, ops: &fakeOps{}, rec: &fakeRec{ev: ev},
		exe: func() (string, error) { return "id", nil },
	}
}

func tickerCal() fakeCal {
	var rows []contracts.SessionDay
	for _, d := range []string{"2025-11-24", "2025-11-25", "2025-11-26", "2025-11-28"} {
		rows = append(rows, contracts.SessionDay{CalendarID: "XNYS", SessionDay: sd(d), IsSession: true,
			OpenAt: utc(d + "T14:30:00Z"), CloseAt: utc(d + "T21:00:00Z")})
	}
	rows = append(rows, contracts.SessionDay{CalendarID: "XNYS", SessionDay: sd("2025-11-27")})
	return fakeCal{rows: rows}
}

// run builds the ticker, starts Run and waits for the first refresh. done receives the result of Run.
func (h *harness) run() (cancel context.CancelFunc, done <-chan error) {
	h.t.Helper()
	h.s = New(h.fence, h.led, h.ops, tickerCal(), h.clk, h.rec, h.gaps, h.jobs, h.disabled, h.exe)
	if h.log != nil {
		h.s.(*ticker).log = h.log
	}
	ctx, cancel := context.WithCancel(context.Background())
	h.t.Cleanup(cancel)
	ch := make(chan error, 1)
	go func() { ch <- h.s.Run(ctx) }()
	h.until("put")
	return cancel, ch
}

// until reads events until want, and returns those before it.
func (h *harness) until(want string) []string {
	h.t.Helper()
	var before []string
	for e := range h.ev {
		if e == want {
			return before
		}
		before = append(before, e)
	}
	panic("unreachable")
}

// pending returns the events already recorded, without waiting.
func (h *harness) pending() []string {
	var out []string
	for {
		select {
		case e := <-h.ev:
			out = append(out, e)
		default:
			return out
		}
	}
}

// step waits for the loop to be asleep on the clock, then advances it by d.
func (h *harness) step(d time.Duration) {
	h.t.Helper()
	if !h.stuck {
		// A turn that finds the worker busy is skipped, so let the last task end first.
		for tk := h.s.(*ticker); tk.busy.Load(); {
			runtime.Gosched()
		}
	}
	if err := h.clk.BlockUntilContext(h.t.Context(), 1); err != nil {
		h.t.Fatal(err)
	}
	h.clk.Advance(d)
}

// waitTick spins (no sleep) until LastTick reaches want.
func waitTick(s runner.Scheduler, want time.Time) {
	for s.LastTick().Before(want) {
		runtime.Gosched()
	}
}

func TestTickerCallsNoLedgerOnLoopGoroutine(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.led.block = make(chan struct{})
	h.stuck = true
	h.s = New(h.fence, h.led, h.ops, tickerCal(), h.clk, h.rec, nil, nil, nil, h.exe)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = h.s.Run(ctx) }()
	h.until("put-blocked") // the worker is stuck in the ledger
	h.step(30 * time.Second)
	waitTick(h.s, tickerStart.Add(30*time.Second)) // the loop woke at the ceiling although the worker is stuck
	close(h.led.block)
}

func TestTickerReconcileAtStartPlus10ThenEvery15(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.run()
	h.until("reconcile:2025-11-26") // the start turn reconciles the start day
	if n := h.rec.(*fakeRec).count(); n != 1 {
		t.Fatalf("Reconcile calls after start = %d, want 1", n)
	}
	h.step(5 * time.Minute)
	if before := h.until("put"); len(before) != 0 {
		t.Fatalf("events before the first tick: %v", before)
	}
	h.step(5 * time.Minute) // 14:10
	if before := h.until("reconcile:2025-11-26"); len(before) != 1 || before[0] != "put" {
		t.Fatalf("before = %v", before)
	}
	if n := h.rec.(*fakeRec).count(); n != 2 {
		t.Fatalf("Reconcile calls after start + 10 minutes = %d, want 2", n)
	}
	h.step(14 * time.Minute) // 14:24
	if before := h.until("put"); len(before) != 0 {
		t.Fatalf("events before the second tick: %v", before)
	}
	h.step(time.Minute) // 14:25
	h.until("reconcile:2025-11-26")
	if n := h.rec.(*fakeRec).count(); n != 3 {
		t.Fatalf("Reconcile calls = %d, want 3", n)
	}
}

func TestTickerSleepsToNextDue(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.jobs = []contracts.Job{tickerJob{id: "a", offset: 14*time.Hour + 20*time.Second}}
	h.run()
	h.until("reconcile:2025-11-26") // the start turn
	h.step(19 * time.Second)
	if err := h.clk.BlockUntilContext(t.Context(), 1); err != nil {
		t.Fatal(err)
	}
	if got := h.pending(); len(got) != 0 {
		t.Fatalf("events before the slot start: %v", got)
	}
	h.clk.Advance(time.Second) // 14:00:20
	h.until("reconcile:2025-11-26")
}

func TestTickerSlotStartCallsReconcileOnce(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.jobs = []contracts.Job{tickerJob{id: "a", offset: 14*time.Hour + 20*time.Second}}
	h.run()
	h.until("reconcile:2025-11-26") // the start turn
	h.step(20 * time.Second)
	h.until("reconcile:2025-11-26")
	h.step(30 * time.Second)
	h.until("put")
	h.step(30 * time.Second)
	h.until("put")
	if got := h.pending(); len(got) != 0 {
		t.Fatalf("further events: %v", got)
	}
	if n := h.rec.(*fakeRec).count(); n != 2 { // the start turn and the slot start
		t.Fatalf("Reconcile calls = %d, want 2", n)
	}
}

func TestTickerCallsGapSyncerAfterReconcile(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	g := &fakeGaps{ev: h.ev}
	h.gaps, h.ops.cursor = g, "2025-11-24"
	h.disabled = map[string]string{"x": "no key"}
	h.run()
	h.until("sync:2025-11-26") // the start turn
	h.step(10 * time.Minute)
	before := h.until("sync:2025-11-26")
	want := []string{"put", "reconcile:2025-11-26", "sync:2025-11-25"}
	if !slices.Equal(before, want) {
		t.Fatalf("events = %v, want %v", before, want)
	}
	calls := g.calls()
	if len(calls) != 4 || calls[2].day != "2025-11-25" || calls[3].day != "2025-11-26" {
		t.Fatalf("Sync calls = %+v", calls)
	}
	if !maps.Equal(calls[0].disabled, h.disabled) {
		t.Fatalf("disabled = %v", calls[0].disabled)
	}
}

func TestTickerNilGapSyncerIsNoop(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.run()
	h.step(10 * time.Minute)
	h.until("reconcile:2025-11-26")
	if n := h.ops.cursors.Load(); n != 0 {
		t.Fatalf("cursor reads = %d, want 0", n)
	}
}

func TestTickerHealthAccessors(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	s := New(h.fence, h.led, h.ops, tickerCal(), h.clk, h.rec, nil, nil, nil, h.exe)
	h.s = s
	if got := s.LastTick(); got.IsZero() || !got.Equal(tickerStart) {
		t.Fatalf("LastTick before the first turn = %v, want %v", got, tickerStart)
	}
	if !s.FenceHeld() {
		t.Fatal("FenceHeld = false while held")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = s.Run(ctx) }()
	h.until("put")
	for i := 1; i <= 2; i++ {
		h.step(30 * time.Second)
		waitTick(s, tickerStart.Add(time.Duration(i)*30*time.Second))
	}
}

func TestFenceLostCancelsContext(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	_, done := h.run()
	close(h.fence.lost)
	if err := <-done; !errors.Is(err, ErrFenceLost) {
		t.Fatalf("Run = %v, want ErrFenceLost", err)
	}
	if h.s.FenceHeld() {
		t.Fatal("FenceHeld = true after the fence was lost")
	}
	if !h.fence.closed.Load() {
		t.Fatal("fence not closed")
	}
}

func TestTickerNeverCallsTryLock(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.jobs = []contracts.Job{tickerJob{id: "a"}}
	cancel, done := h.run()
	h.step(10 * time.Minute)
	h.until("reconcile:2025-11-26")
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if n := h.fence.tryLock.Load(); n != 0 {
		t.Fatalf("TryLock calls = %d, want 0", n)
	}
}

func TestWatchdogExitsOnFrozenLoop(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	var calls atomic.Int32
	release := make(chan struct{})
	defer close(release)
	h.exe = func() (string, error) {
		if calls.Add(1) > 1 { // the tick: the loop blocks here
			h.ev.emit("exe-blocked")
			<-release
		}
		return "id", nil
	}
	_, done := h.run()
	h.step(10 * time.Minute)
	h.until("exe-blocked")
	h.clk.Advance(61 * time.Second)
	if err := <-done; !errors.Is(err, ErrLoopFrozen) {
		t.Fatalf("Run = %v, want ErrLoopFrozen", err)
	}
}

func TestDrainCancelsWorkerAndClosesFence(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.rec = &fakeRec{ev: h.ev, block: true}
	cancel, done := h.run()
	h.until("reconcile:2025-11-26") // the start turn is blocked in Reconcile
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
			if advanced < drainGrace {
				if got := h.pending(); slices.Contains(got, "reconcile-cancelled") {
					t.Fatalf("worker cancelled after %v, before the grace", advanced)
				}
			}
			h.clk.Advance(time.Second)
			advanced += time.Second
			runtime.Gosched()
		}
	}
	if advanced < drainGrace {
		t.Fatalf("drained after %v, want at least %v", advanced, drainGrace)
	}
	if !h.fence.closed.Load() {
		t.Fatal("fence not closed")
	}
	h.until("reconcile-cancelled")
}

func TestJobStatesWrittenWithRestartSensitive(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.jobs = []contracts.Job{
		tickerJob{id: "eod", offset: 20*time.Hour + 30*time.Minute, sensitive: true, closeOff: 2 * time.Hour},
		tickerJob{id: "fred", offset: time.Hour},
	}
	h.disabled = map[string]string{"fred": "no key"}
	h.run()
	got := h.led.last()
	want := []struct {
		id        string
		sensitive bool
		reason    string
		next, end time.Time
	}{
		{"eod", true, "", utc("2025-11-26T20:30:00Z"), utc("2025-11-26T23:00:00Z")},
		{"fred", false, "no key", time.Time{}, time.Time{}},
	}
	if len(got) != len(want) {
		t.Fatalf("states = %+v", got)
	}
	for i, w := range want {
		g := got[i]
		if g.JobID != w.id || g.RestartSensitive != w.sensitive || g.DisabledReason != w.reason ||
			!g.NextStart.Equal(w.next) || !g.WindowEnd.Equal(w.end) {
			t.Fatalf("state %d = %+v, want %+v", i, g, w)
		}
	}
}

func TestExecutableChangeDrainsAndReturnsReplacement(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	var calls atomic.Int32
	h.exe = func() (string, error) {
		if calls.Add(1) > 1 {
			return "new", nil
		}
		return "old", nil
	}
	_, done := h.run()
	h.step(10 * time.Minute)
	if err := <-done; !errors.Is(err, ErrExecutableChanged) {
		t.Fatalf("Run = %v, want ErrExecutableChanged", err)
	}
	if !h.fence.closed.Load() {
		t.Fatal("fence not closed")
	}
	if n := h.rec.(*fakeRec).count(); n != 1 { // the start turn only, none for the tick that saw it
		t.Fatalf("Reconcile calls = %d after the replacement was seen, want 1", n)
	}
}

func TestTickerReprobesDisabledJobsEachTick(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	rec := &probeRec{fakeRec: &fakeRec{ev: h.ev}, reasons: map[string][]string{"a": {"still down", "still down"}}}
	g := &fakeGaps{ev: h.ev}
	h.rec, h.gaps, h.ops.cursor = rec, g, "2025-11-25"
	h.jobs = []contracts.Job{tickerJob{id: "a", offset: 14*time.Hour + 20*time.Second}}
	h.disabled = map[string]string{"a": "no key"}
	h.run()
	if before := h.until("sync:2025-11-26"); !slices.Equal(before, []string{"reconcile:2025-11-26"}) {
		t.Fatalf("start turn events = %v", before) // the start turn probes (the first answer), refreshes, reconciles
	}
	h.step(5 * time.Minute) // between ticks: no probe
	h.until("put")
	h.step(5 * time.Minute) // first tick: the probe fails
	if before := h.until("sync:2025-11-26"); !slices.Equal(before, []string{"probe:a", "put", "reconcile:2025-11-26"}) {
		t.Fatalf("first tick events = %v", before)
	}
	if st := h.led.last()[0]; st.DisabledReason != "still down" || !st.NextStart.IsZero() {
		t.Fatalf("state after a failing probe = %+v", st)
	}
	h.step(15 * time.Minute) // second tick: the probe passes
	if before := h.until("sync:2025-11-26"); !slices.Equal(before, []string{"probe:a", "put", "reconcile:2025-11-26"}) {
		t.Fatalf("second tick events = %v", before)
	}
	if st := h.led.last()[0]; st.DisabledReason != "" || !st.NextStart.Equal(utc("2025-11-28T14:00:20Z")) {
		t.Fatalf("state after a passing probe = %+v", st)
	}
	calls := g.calls()
	if len(calls) != 3 || len(calls[1].disabled) != 1 || calls[1].disabled["a"] != "still down" || len(calls[2].disabled) != 0 {
		t.Fatalf("Sync maps = %+v", calls)
	}
	if rec.overlap.Load() {
		t.Fatal("a probe overlapped a Reconcile")
	}
}

func TestTickerSyncGetsCopyOfDisabled(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	var kept []map[string]string
	g := &fakeGaps{ev: h.ev}
	g.hook = func(_ contracts.Day, m map[string]string) {
		m["written-by-sync"] = "x"
		kept = append(kept, m)
	}
	h.gaps, h.ops.cursor = g, "2025-11-25"
	caller := map[string]string{"a": "no key"}
	h.disabled = caller
	h.run()
	h.until("sync:2025-11-26") // the start turn
	caller["written-by-caller"] = "y"
	h.step(10 * time.Minute)
	h.until("sync:2025-11-26")
	h.step(15 * time.Minute)
	h.until("sync:2025-11-26")
	// Writes after the call race with the ticker if the map is shared.
	for _, m := range kept {
		m["after-call"] = "z"
	}
	calls := g.calls()
	if len(calls) != 3 {
		t.Fatalf("Sync calls = %d", len(calls))
	}
	for i, c := range calls {
		if c.disabled["a"] != "no key" || c.disabled["written-by-caller"] != "" {
			t.Fatalf("call %d saw %v", i, c.disabled)
		}
	}
	if calls[1].disabled["written-by-sync"] != "x" || len(kept) != 3 {
		t.Fatalf("unexpected second map %v", calls[1].disabled)
	}
	if _, ok := caller["written-by-sync"]; ok {
		t.Fatal("a write by Sync reached the caller's map")
	}
}

// lockedBuf is a log buffer safe for the worker goroutine and the test.
type lockedBuf struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuf) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuf) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

type markedTypedErr struct{}

func (markedTypedErr) Error() string             { return "secret-token-hunter2" }
func (markedTypedErr) Kind() contracts.ErrorKind { return contracts.Retryable }

func TestTickerLogsReconcileFailure(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		err  error
		kind string
	}{
		{"typed", markedTypedErr{}, "kind=retryable"},
		{"plain", errors.New("secret-token-hunter2"), "kind=error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			h.rec = &fakeRec{ev: h.ev, err: tc.err}
			buf := &lockedBuf{}
			h.log = slog.New(slog.NewTextHandler(buf, nil))
			h.run()
			h.until("reconcile:2025-11-26")
			for tk := h.s.(*ticker); tk.busy.Load(); {
				runtime.Gosched()
			}
			out := buf.String()
			if n := bytes.Count([]byte(out), []byte("reconcile failed")); n != 1 || bytes.Count([]byte(out), []byte("\n")) != 1 {
				t.Fatalf("log = %q, want one line \"reconcile failed\"", out)
			}
			if !bytes.Contains([]byte(out), []byte("day=2025-11-26")) || !bytes.Contains([]byte(out), []byte(tc.kind)) {
				t.Fatalf("log = %q, want the day and %s", out, tc.kind)
			}
			if bytes.Contains([]byte(out), []byte("secret-token-hunter2")) {
				t.Fatalf("log holds the error text: %q", out)
			}
		})
	}
}
