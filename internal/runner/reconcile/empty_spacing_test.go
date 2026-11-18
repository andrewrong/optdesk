package reconcile

import (
	"context"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
)

// newLongRec builds a reconciler whose job closes 96 hours after the session close, as fred_series does.
func newLongRec(led *fakeLedger, clk clockwork.Clock, job *fakeJob) runner.Reconciler {
	wj := windowedJob{fakeJob: job, calendarID: "XNYS", close: 96 * time.Hour}
	return NewWithRule(led, oneDay(), clk, []contracts.Job{wj}, runner.WindowRule{CalendarID: "XNYS"})
}

func TestEmptyBeforeCloseSpacing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	clk := clockwork.NewFakeClockAt(utc("2026-11-04T12:00:00Z"))
	led := newLedger()
	led.cursor = store.Cursor{Day: "2026-11-03"}
	job := &fakeJob{keys: []contracts.ExpectedKey{keyA}, results: empty()}
	rec := newLongRec(led, clk, job)
	step := func(advance time.Duration, wantCalls int) {
		t.Helper()
		clk.Advance(advance)
		if err := rec.Reconcile(ctx, "2026-11-04"); err != nil {
			t.Fatal(err)
		}
		if len(job.calls) != wantCalls {
			t.Fatalf("calls = %d, want %d", len(job.calls), wantCalls)
		}
	}
	step(0, 1)
	step(0, 1)
	step(5*time.Hour+59*time.Minute, 1)
	step(time.Minute, 2)
	present, _ := led.Present(ctx, "2026-11-04")
	if _, ok := present[keyA]; ok || led.cursor.Day != "2026-11-03" {
		t.Fatalf("present=%v cursor=%s", present, led.cursor.Day)
	}
}

func TestEmptyBeforeCloseTwentyFourTicks(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	clk := clockwork.NewFakeClockAt(utc("2026-11-04T12:00:00Z"))
	led := newLedger()
	led.cursor = store.Cursor{Day: "2026-11-03"}
	job := &fakeJob{keys: []contracts.ExpectedKey{keyA}, results: empty()}
	rec := newLongRec(led, clk, job)
	for i := 0; i < 96; i++ {
		if err := rec.Reconcile(ctx, "2026-11-04"); err != nil {
			t.Fatal(err)
		}
		clk.Advance(15 * time.Minute)
	}
	if n := len(job.calls); n > 4 || n == 0 {
		t.Fatalf("runs in one day = %d, want 1..4", n)
	}
}
