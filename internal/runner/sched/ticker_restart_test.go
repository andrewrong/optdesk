package sched

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/runner/reconcile"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/jonboulle/clockwork"
)

var eodKey = contracts.ExpectedKey{DatasetID: "option_eod", SubjectKey: "SPY"}

// eodJob is a job of the shape of option_eod: slot at the session close + 15 minutes, window to the
// close + 5 hours (XNYS 16:15 ET, open until 10:00 KL the next morning).
type eodJob struct{}

func (eodJob) ID() string { return "option_eod" }
func (eodJob) Plan(context.Context, contracts.Day) ([]contracts.ExpectedKey, string, error) {
	return []contracts.ExpectedKey{eodKey}, "plan-1", nil
}
func (eodJob) Run(context.Context, contracts.Day, []contracts.ExpectedKey) contracts.JobResult {
	return contracts.JobResult{}
}
func (eodJob) Schedule() (string, string, string, time.Duration) {
	return "XNYS", "session_close", "UTC", 15 * time.Minute
}
func (eodJob) Window() (string, time.Duration, time.Duration) { return "XNYS", 0, 5 * time.Hour }

// memOps is an in-memory store.OpsLedger that the real reconciler runs on.
type memOps struct {
	store.OpsLedger // unimplemented methods panic: the reconciler must not call them
	mu              sync.Mutex
	runs            []store.Run
	present         map[contracts.ExpectedKey]store.PresentStatus
	cursor          store.Cursor
}

func (l *memOps) PutExpected(context.Context, contracts.Day, string, []contracts.ExpectedKey) error {
	return nil
}
func (l *memOps) Expected(context.Context, contracts.Day) ([]contracts.ExpectedKey, string, error) {
	return nil, "", nil
}
func (l *memOps) PutWindow(context.Context, store.Window) error { return nil }
func (l *memOps) Acquire(_ context.Context, owner string, ttl time.Duration) (store.Lease, error) {
	return store.Lease{Owner: owner, ExpiresAt: time.Time{}.Add(ttl)}, nil
}
func (l *memOps) Release(context.Context, store.Lease) error { return nil }
func (l *memOps) Cursor(context.Context) (store.Cursor, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.cursor, nil
}
func (l *memOps) SetCursor(_ context.Context, c store.Cursor) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.cursor = c
	return nil
}
func (l *memOps) MarkPresent(_ context.Context, _ contracts.Day, k contracts.ExpectedKey, s store.PresentStatus) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.present == nil {
		l.present = map[contracts.ExpectedKey]store.PresentStatus{}
	}
	l.present[k] = s
	return nil
}
func (l *memOps) Present(context.Context, contracts.Day) (map[contracts.ExpectedKey]store.PresentStatus, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := map[contracts.ExpectedKey]store.PresentStatus{}
	for k, v := range l.present {
		out[k] = v
	}
	return out, nil
}
func (l *memOps) RecordRun(_ context.Context, r store.Run) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	for i, o := range l.runs {
		if o.ID == r.ID {
			l.runs[i] = r
			return nil
		}
	}
	l.runs = append(l.runs, r)
	return nil
}
func (l *memOps) Runs(context.Context, contracts.Day) ([]store.Run, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]store.Run(nil), l.runs...), nil
}

// startRunner is a runner.KeyRunner that counts the starts and leaves a running run behind, as a
// process killed in the middle of the run does.
type startRunner struct {
	mu     sync.Mutex
	starts int
	ops    *memOps
	clk    clockwork.Clock
}

func (k *startRunner) RunKey(
	ctx context.Context, _ contracts.Job, day contracts.Day, key contracts.ExpectedKey, attempt int, _, _ time.Time, _ string,
) (store.Run, error) {
	k.mu.Lock()
	k.starts++
	k.mu.Unlock()
	r := store.Run{ID: string(day) + "|" + key.DatasetID + "|" + key.SubjectKey + "|" + string(rune('0'+attempt)), Day: day, Key: key,
		Status: store.RunRunning, LeaseUntil: k.clk.Now().Add(10 * time.Minute)}
	return r, k.ops.RecordRun(ctx, r)
}

func (k *startRunner) count() int {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.starts
}

// recSpy reports each finished Reconcile of the real reconciler on the event channel.
type recSpy struct {
	inner runner.Reconciler
	ev    events
}

func (r recSpy) Reconcile(ctx context.Context, day contracts.Day) error {
	err := r.inner.Reconcile(ctx, day)
	r.ev.emit("reconcile:" + string(day))
	return err
}

func restartCal() fakeCal {
	var rows []contracts.SessionDay
	for _, d := range []string{"2026-11-09", "2026-11-10"} {
		rows = append(rows, contracts.SessionDay{CalendarID: "XNYS", SessionDay: sd(d), IsSession: true,
			OpenAt: utc(d + "T14:30:00Z"), CloseAt: utc(d + "T21:00:00Z")})
	}
	return fakeCal{rows: rows}
}

// restartEnv is the real reconciler over memOps and a counting key runner, behind a ticker that
// starts at the given instant.
type restartEnv struct {
	h   *harness
	ops *memOps
	kr  *startRunner
}

func newRestartEnv(t *testing.T, start time.Time, ops *memOps) *restartEnv {
	t.Helper()
	h := newHarness(t)
	h.clk = clockwork.NewFakeClockAt(start)
	kr := &startRunner{ops: ops, clk: h.clk}
	rec, _ := reconcile.NewReconciler(ops, nil, restartCal(), h.clk, []contracts.Job{eodJob{}}, runner.WindowRule{}, kr)
	h.rec = recSpy{inner: rec, ev: h.ev}
	h.jobs = []contracts.Job{eodJob{}}
	h.s = New(h.fence, h.led, h.ops, restartCal(), h.clk, h.rec, nil, h.jobs, nil, h.exe)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = h.s.Run(ctx) }()
	return &restartEnv{h: h, ops: ops, kr: kr}
}

func newMemOps() *memOps { return &memOps{cursor: store.Cursor{Day: "2026-11-09"}} }

func TestTickerRestartInsideWindowRunsKey(t *testing.T) {
	t.Parallel()
	// 05:40 KL on 11-11 = 21:40 UTC on 11-10, inside the window 21:15 UTC - 02:00 UTC.
	env := newRestartEnv(t, utc("2026-11-10T21:40:00Z"), newMemOps())
	before := env.h.until("reconcile:2026-11-10") // before any tick: the clock has not moved
	if len(before) != 1 || before[0] != "put" {
		t.Fatalf("events before the start Reconcile = %v", before)
	}
	if n := env.kr.count(); n != 1 {
		t.Fatalf("key started %d times, want 1", n)
	}
	if got := env.h.pending(); len(got) != 0 {
		t.Fatalf("further events before the first tick: %v", got)
	}
}

func TestTickerRestartAfterWindowCloseRunsNothing(t *testing.T) {
	t.Parallel()
	// 10:30 KL on 11-11 = 02:30 UTC, after the window closed at 02:00 UTC. An earlier run that
	// ended with a Permanent error is on the ledger: the Close table row 4 expires the key.
	ops := newMemOps()
	ops.runs = []store.Run{{ID: "2026-11-10|option_eod|SPY|1", Day: "2026-11-10", Key: eodKey, Status: store.RunDone,
		ErrorCode: string(contracts.Permanent), FinishedAt: utc("2026-11-10T22:00:00Z")}}
	env := newRestartEnv(t, utc("2026-11-11T02:30:00Z"), ops)
	env.h.until("reconcile:2026-11-11")
	if n := env.kr.count(); n != 0 {
		t.Fatalf("key started %d times after the close, want 0", n)
	}
	got, _ := ops.Present(context.Background(), "2026-11-10")
	if got[eodKey] != store.StatusNoValue {
		t.Fatalf("present = %v, want the key closed as no_value", got)
	}
	runs, _ := ops.Runs(context.Background(), "2026-11-10")
	if len(runs) != 1 || runs[0].ReasonCode != "expired" {
		t.Fatalf("runs = %+v, want one run closed as expired", runs)
	}
}

func TestTickerRestartTwiceInsideWindowRunsKeyOnce(t *testing.T) {
	t.Parallel()
	ops := newMemOps()
	first := newRestartEnv(t, utc("2026-11-10T21:40:00Z"), ops)
	first.h.until("reconcile:2026-11-10")
	// Three minutes later a new process starts on the same ledger; the run of the first is still
	// running with a live lease.
	second := newRestartEnv(t, utc("2026-11-10T21:43:00Z"), ops)
	second.h.until("reconcile:2026-11-10")
	if n := first.kr.count() + second.kr.count(); n != 1 {
		t.Fatalf("key started %d times over two starts, want 1", n)
	}
	runs, _ := ops.Runs(context.Background(), "2026-11-10")
	if len(runs) != 1 || runs[0].Status != store.RunRunning {
		t.Fatalf("runs = %+v, want the one running run", runs)
	}
}
