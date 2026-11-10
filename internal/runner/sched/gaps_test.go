package sched

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/jonboulle/clockwork"
)

var (
	_ runner.GapSyncer    = NewGapSyncer(nil, nil, nil)
	_ runner.KeyGapSyncer = NewKeyGapSyncer(nil, nil, nil)
)

// gapOps is a store.OpsLedger that holds the runs, marks and windows of one day, and answers Cursor.
type gapOps struct {
	fakeOps
	mu      sync.Mutex
	runs    map[contracts.Day][]store.Run
	present map[contracts.ExpectedKey]store.PresentStatus
	windows map[string]time.Time
}

func newGapOps() *gapOps {
	return &gapOps{runs: map[contracts.Day][]store.Run{}, present: map[contracts.ExpectedKey]store.PresentStatus{},
		windows: map[string]time.Time{}}
}

func (o *gapOps) Runs(_ context.Context, day contracts.Day) ([]store.Run, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return slices.Clone(o.runs[day]), nil
}

func (o *gapOps) Present(context.Context, contracts.Day) (map[contracts.ExpectedKey]store.PresentStatus, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := map[contracts.ExpectedKey]store.PresentStatus{}
	for k, v := range o.present {
		out[k] = v
	}
	return out, nil
}

func (o *gapOps) Window(_ context.Context, id string, _ contracts.Day) (store.Window, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if c, ok := o.windows[id]; ok {
		return store.Window{DatasetID: id, CloseAt: c}, nil
	}
	return store.Window{}, noValueErr{}
}

func (o *gapOps) mark(k contracts.ExpectedKey, s store.PresentStatus) {
	o.mu.Lock()
	o.present[k] = s
	o.mu.Unlock()
}

// gapLedger is a store.SchedulerLedger that keeps the open gaps and counts its writes.
type gapLedger struct {
	fakeLedger
	mu     sync.Mutex
	open   []store.Gap
	puts   int
	clears int
}

func (l *gapLedger) Status(context.Context) (store.SchedulerStatus, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return store.SchedulerStatus{Gaps: slices.Clone(l.open)}, nil
}

func (l *gapLedger) PutGap(_ context.Context, g store.Gap) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.puts++
	for i, o := range l.open {
		if o.JobID == g.JobID && o.DatasetID == g.DatasetID && o.Day == g.Day {
			l.open[i].Reason = g.Reason
			return nil
		}
	}
	l.open = append(l.open, g)
	return nil
}

func (l *gapLedger) ClearGap(_ context.Context, g store.Gap) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.clears++
	l.open = slices.DeleteFunc(l.open, func(o store.Gap) bool {
		return o.JobID == g.JobID && o.DatasetID == g.DatasetID && o.Day == g.Day
	})
	return nil
}

func (l *gapLedger) gaps() []store.Gap {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.open)
}

const gapDay = contracts.Day("2025-11-26")

var (
	gapKey   = contracts.ExpectedKey{DatasetID: "ds", SubjectKey: "a"}
	gapClose = utc("2025-11-26T13:00:00Z") // before tickerStart, so a window past its close
)

type gapRig struct {
	ops *gapOps
	led *gapLedger
	clk *clockwork.FakeClock
	syn runner.GapSyncer
}

func newGapRig(now time.Time) *gapRig {
	r := &gapRig{ops: newGapOps(), led: &gapLedger{}, clk: clockwork.NewFakeClockAt(now)}
	r.ops.windows["ds"] = gapClose
	r.syn = NewGapSyncer(r.ops, r.led, r.clk)
	return r
}

func (r *gapRig) addRun(run store.Run) {
	run.Day, run.Key = gapDay, gapKey
	if run.ID == "" {
		run.ID = string(rune('a' + len(r.ops.runs[gapDay])))
	}
	r.ops.runs[gapDay] = append(r.ops.runs[gapDay], run)
}

func (r *gapRig) sync(t *testing.T, disabled map[string]string) {
	t.Helper()
	if err := r.syn.Sync(t.Context(), gapDay, disabled); err != nil {
		t.Fatal(err)
	}
}

func (r *gapRig) wantGap(t *testing.T, reason string) {
	t.Helper()
	got := r.led.gaps()
	want := []store.Gap{{DatasetID: "ds", Day: gapDay, Reason: reason}}
	if reason == "" {
		want = nil
	}
	if !slices.Equal(got, want) {
		t.Fatalf("gaps = %+v, want %+v", got, want)
	}
}

var (
	retryRun   = store.Run{Status: store.RunDone, ErrorCode: string(contracts.Retryable)}
	permRun    = store.Run{Status: store.RunDone, ErrorCode: string(contracts.Permanent)}
	partialRun = store.Run{Status: store.RunPartial, ReturnedN: 3}
	crashRun   = store.Run{Status: store.RunDone, ErrorCode: string(contracts.Retryable), ReasonCode: "crashed"}
)

func withReason(r store.Run, reason string) store.Run { r.ReasonCode = reason; return r }

func TestGapsExpiredKey(t *testing.T) {
	t.Parallel()
	t.Run("missing past close", func(t *testing.T) {
		t.Parallel()
		r := newGapRig(tickerStart)
		r.addRun(retryRun)
		r.sync(t, nil)
		r.wantGap(t, "expired")
		r.sync(t, nil)
		r.wantGap(t, "expired")
	})
	t.Run("expired by the runner then synced", func(t *testing.T) {
		t.Parallel()
		r := newGapRig(tickerStart)
		r.addRun(withReason(retryRun, "expired"))
		r.ops.mark(gapKey, store.StatusNoValue)
		r.sync(t, nil)
		r.wantGap(t, "expired")
		r.sync(t, nil)
		r.wantGap(t, "expired")
	})
	t.Run("row 4 done without a mark", func(t *testing.T) {
		t.Parallel()
		r := newGapRig(tickerStart)
		r.addRun(permRun)
		r.sync(t, nil)
		r.wantGap(t, "expired")
	})
	t.Run("before close nothing", func(t *testing.T) {
		t.Parallel()
		r := newGapRig(gapClose.Add(-time.Minute))
		r.addRun(retryRun)
		r.sync(t, nil)
		r.wantGap(t, "")
	})
	t.Run("no_value for another reason has no gap", func(t *testing.T) {
		t.Parallel()
		r := newGapRig(tickerStart)
		r.addRun(permRun)
		r.ops.mark(gapKey, store.StatusNoValue)
		r.sync(t, nil)
		r.wantGap(t, "")
	})
}

func TestGapsCrashedRepeatedly(t *testing.T) {
	t.Parallel()
	t.Run("before the close from three runs", func(t *testing.T) {
		t.Parallel()
		r := newGapRig(gapClose.Add(-time.Hour))
		r.addRun(crashRun)
		r.addRun(withReason(retryRun, "abandoned_stop"))
		r.sync(t, nil)
		r.wantGap(t, "")
		r.addRun(crashRun)
		r.sync(t, nil)
		r.wantGap(t, "crashed_repeatedly")
	})
	t.Run("a lapsed running run counts", func(t *testing.T) {
		t.Parallel()
		r := newGapRig(gapClose.Add(-time.Hour))
		r.addRun(crashRun)
		r.addRun(crashRun)
		r.addRun(store.Run{Status: store.RunRunning, LeaseUntil: r.clk.Now().Add(-time.Minute)})
		r.sync(t, nil)
		r.wantGap(t, "crashed_repeatedly")
	})
	t.Run("run rewritten by the runner is proof", func(t *testing.T) {
		t.Parallel()
		r := newGapRig(tickerStart)
		r.addRun(crashRun)
		r.addRun(crashRun)
		r.addRun(withReason(crashRun, "crashed_repeatedly"))
		r.ops.mark(gapKey, store.StatusNoValue)
		r.sync(t, nil)
		r.wantGap(t, "crashed_repeatedly")
	})
}

func TestGapsPreflightFailedOnce(t *testing.T) {
	t.Parallel()
	r := newGapRig(tickerStart)
	dis := map[string]string{"opt": "no key"}
	r.sync(t, dis)
	want := []store.Gap{{JobID: "opt", Day: gapDay, Reason: "preflight_failed"}}
	if got := r.led.gaps(); !slices.Equal(got, want) {
		t.Fatalf("gaps = %+v", got)
	}
	r.sync(t, dis)
	if r.led.puts != 1 {
		t.Fatalf("PutGap calls = %d, want 1", r.led.puts)
	}
}

func TestGapsPreflightClearedOnFirstPass(t *testing.T) {
	t.Parallel()
	r := newGapRig(tickerStart)
	r.sync(t, map[string]string{"opt": "no key", "snap": "no socket"})
	r.sync(t, map[string]string{"snap": "no socket"})
	want := []store.Gap{{JobID: "snap", Day: gapDay, Reason: "preflight_failed"}}
	if got := r.led.gaps(); !slices.Equal(got, want) {
		t.Fatalf("gaps = %+v", got)
	}
	r.sync(t, nil)
	if got := r.led.gaps(); len(got) != 0 {
		t.Fatalf("gaps = %+v", got)
	}
}

func TestGapsClearedWhenPresent(t *testing.T) {
	t.Parallel()
	r := newGapRig(tickerStart)
	r.addRun(retryRun)
	r.sync(t, nil)
	r.wantGap(t, "expired")
	r.ops.mark(gapKey, store.StatusPresent)
	r.sync(t, nil)
	r.wantGap(t, "")
}

func TestGapsIdempotentAcrossCalls(t *testing.T) {
	t.Parallel()
	r := newGapRig(tickerStart)
	r.addRun(partialRun)
	for range 3 {
		r.sync(t, map[string]string{"opt": "x"})
	}
	if r.led.puts != 2 || r.led.clears != 0 || len(r.led.gaps()) != 2 {
		t.Fatalf("puts=%d clears=%d gaps=%+v", r.led.puts, r.led.clears, r.led.gaps())
	}
}

func TestPartialGapAtClose(t *testing.T) {
	t.Parallel()
	t.Run("latest counted run partial", func(t *testing.T) {
		t.Parallel()
		r := newGapRig(tickerStart)
		r.addRun(partialRun)
		r.sync(t, nil)
		r.wantGap(t, "partial")
	})
	t.Run("uncounted retryable after the partial", func(t *testing.T) {
		t.Parallel()
		r := newGapRig(tickerStart)
		r.addRun(partialRun)
		r.addRun(retryRun)
		r.sync(t, nil)
		r.wantGap(t, "partial")
	})
	t.Run("partial rewritten by the runner", func(t *testing.T) {
		t.Parallel()
		r := newGapRig(tickerStart)
		r.addRun(withReason(partialRun, "expired"))
		r.ops.mark(gapKey, store.StatusNoValue)
		r.sync(t, nil)
		r.wantGap(t, "partial")
	})
	t.Run("three crashed runs win over partial", func(t *testing.T) {
		t.Parallel()
		r := newGapRig(tickerStart)
		r.addRun(partialRun)
		r.addRun(crashRun)
		r.addRun(crashRun)
		r.addRun(crashRun)
		r.sync(t, nil)
		r.wantGap(t, "crashed_repeatedly")
	})
	t.Run("backfill makes the key present", func(t *testing.T) {
		t.Parallel()
		r := newGapRig(tickerStart)
		r.addRun(partialRun)
		r.sync(t, nil)
		r.ops.mark(gapKey, store.StatusPresent)
		r.sync(t, nil)
		r.wantGap(t, "")
	})
}

// tickerRig runs sched.New over the gap fakes; the fake reconciler is the one of ticker_test.go.
func tickerRig(t *testing.T, gaps func(h *harness, ops *gapOps, led *gapLedger) runner.GapSyncer, cursor contracts.Day) (*harness, *gapOps, *gapLedger) {
	t.Helper()
	h := newHarness(t)
	ops, led := newGapOps(), &gapLedger{fakeLedger: fakeLedger{ev: h.ev}}
	ops.cursor = cursor
	ops.windows["ds"] = gapClose
	g := gaps(h, ops, led)
	h.s = New(h.fence, led, ops, tickerCal(), h.clk, h.rec, g, nil, nil, h.exe)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = h.s.Run(ctx) }()
	h.until("put")
	return h, ops, led
}

func TestGapsViaTickerHook(t *testing.T) {
	t.Parallel()
	t.Run("fake sees the call and the real syncer writes the gap", func(t *testing.T) {
		t.Parallel()
		fake := &fakeGaps{}
		h, ops, led := tickerRig(t, func(h *harness, ops *gapOps, led *gapLedger) runner.GapSyncer {
			fake.ev = h.ev
			return tee{fake, NewGapSyncer(ops, led, h.clk)}
		}, "2025-11-25")
		ops.runs[gapDay] = []store.Run{{ID: "r", Day: gapDay, Key: gapKey, Status: store.RunDone,
			ErrorCode: string(contracts.Retryable)}}
		h.step(10 * time.Minute)
		h.until("sync:2025-11-26")
		if c := fake.calls(); len(c) != 1 || c[0].day != gapDay {
			t.Fatalf("calls = %+v", c)
		}
		want := []store.Gap{{DatasetID: "ds", Day: gapDay, Reason: "expired"}}
		if got := led.gaps(); !slices.Equal(got, want) {
			t.Fatalf("gaps = %+v", got)
		}
	})
	t.Run("an earlier day gets its gap", func(t *testing.T) {
		t.Parallel()
		fake := &fakeGaps{}
		h, ops, led := tickerRig(t, func(h *harness, ops *gapOps, led *gapLedger) runner.GapSyncer {
			fake.ev = h.ev
			return tee{fake, NewGapSyncer(ops, led, h.clk)}
		}, "2025-11-24")
		earlier := contracts.Day("2025-11-25")
		ops.runs[earlier] = []store.Run{{ID: "r", Day: earlier, Key: gapKey, Status: store.RunPartial, ReturnedN: 2}}
		h.step(10 * time.Minute)
		h.until("sync:2025-11-26")
		c := fake.calls()
		if len(c) != 2 || c[0].day != earlier || c[1].day != gapDay {
			t.Fatalf("calls = %+v", c)
		}
		want := []store.Gap{{DatasetID: "ds", Day: earlier, Reason: "partial"}}
		if got := led.gaps(); !slices.Equal(got, want) {
			t.Fatalf("gaps = %+v", got)
		}
	})
}

// tee calls the real syncer b, then the fake a, so the fake's event follows the real write.
type tee struct{ a, b runner.GapSyncer }

func (t tee) Sync(ctx context.Context, day contracts.Day, d map[string]string) error {
	err := t.b.Sync(ctx, day, d)
	_ = t.a.Sync(ctx, day, d)
	return err
}

func (r *gapRig) syncKeys(t *testing.T) {
	t.Helper()
	if err := NewKeyGapSyncer(r.ops, r.led, r.clk).SyncKeys(t.Context(), gapDay); err != nil {
		t.Fatal(err)
	}
}

func TestSyncKeysClears(t *testing.T) {
	t.Parallel()
	r := newGapRig(tickerStart)
	r.addRun(withReason(retryRun, "expired"))
	r.ops.mark(gapKey, store.StatusNoValue)
	r.sync(t, nil)
	r.wantGap(t, "expired")
	r.addRun(withReason(store.Run{Status: store.RunDone}, "backfill"))
	r.ops.mark(gapKey, store.StatusPresent)
	r.syncKeys(t)
	r.wantGap(t, "")
}

func TestSyncKeysKeepsJobGap(t *testing.T) {
	t.Parallel()
	r := newGapRig(tickerStart)
	r.addRun(withReason(retryRun, "expired"))
	r.ops.mark(gapKey, store.StatusNoValue)
	r.sync(t, map[string]string{"opt": "x"})
	r.ops.mark(gapKey, store.StatusPresent)
	r.syncKeys(t)
	want := []store.Gap{{JobID: "opt", Day: gapDay, Reason: "preflight_failed"}}
	if got := r.led.gaps(); !slices.Equal(got, want) {
		t.Fatalf("gaps = %+v, want %+v", got, want)
	}
}

func TestGenuineNoValueClears(t *testing.T) {
	t.Parallel()
	r := newGapRig(tickerStart)
	r.addRun(withReason(retryRun, "expired"))
	r.ops.mark(gapKey, store.StatusNoValue)
	r.sync(t, nil)
	r.wantGap(t, "expired")
	r.addRun(withReason(store.Run{Status: store.RunDone}, "backfill"))
	r.syncKeys(t)
	r.wantGap(t, "")
}

func TestFailedBackfillKeepsGap(t *testing.T) {
	t.Parallel()
	r := newGapRig(tickerStart)
	r.addRun(withReason(retryRun, "expired"))
	r.ops.mark(gapKey, store.StatusNoValue)
	r.addRun(withReason(retryRun, "backfill"))
	r.syncKeys(t)
	r.wantGap(t, "expired")
}
