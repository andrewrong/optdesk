package reconcile

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/jonboulle/clockwork"
)

// scriptJob answers each run through fn, which gets the day and the call number (from 1).
type scriptJob struct {
	keys []contracts.ExpectedKey
	fn   func(day contracts.Day, call int) contracts.JobResult
	mu   sync.Mutex
	n    int
}

func (j *scriptJob) ID() string { return "script" }
func (j *scriptJob) Plan(context.Context, contracts.Day) ([]contracts.ExpectedKey, string, error) {
	return j.keys, "plan-1", nil
}

func (j *scriptJob) Run(_ context.Context, day contracts.Day, _ []contracts.ExpectedKey) contracts.JobResult {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.n++
	return j.fn(day, j.n)
}

func (j *scriptJob) calls() int {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.n
}

var retryable = contracts.JobResult{Err: kindErr{kind: contracts.Retryable}}

func newScriptRec(led *fakeLedger, cal clock.Calendar, clk clockwork.Clock, job *scriptJob) runner.Reconciler {
	return NewWithRule(led, cal, clk, []contracts.Job{job}, runner.WindowRule{CalendarID: "XNYS"})
}

func TestRetryableBeforeCloseRetries(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	const day = contracts.Day("2026-11-04")

	t.Run("second run succeeds", func(t *testing.T) {
		t.Parallel()
		clk := clockwork.NewFakeClockAt(utc("2026-11-04T15:00:00Z"))
		led := newLedger()
		led.cursor = store.Cursor{Day: "2026-11-03"}
		job := &scriptJob{keys: []contracts.ExpectedKey{keyA}, fn: func(_ contracts.Day, n int) contracts.JobResult {
			if n == 1 {
				return retryable
			}
			return contracts.JobResult{Returned: 1, Usable: 1}
		}}
		rec := newScriptRec(led, oneDay(), clk, job)
		if err := rec.Reconcile(ctx, day); err != nil {
			t.Fatal(err)
		}
		present, _ := led.Present(ctx, day)
		if _, ok := present[keyA]; ok || led.cursor.Day != "2026-11-03" {
			t.Fatalf("present=%v cursor=%s, want key missing and cursor held", present, led.cursor.Day)
		}
		if err := rec.Reconcile(ctx, day); err != nil {
			t.Fatal(err)
		}
		if job.calls() != 2 {
			t.Fatalf("calls = %d, want 2", job.calls())
		}
		runs, _ := led.Runs(ctx, day)
		if len(runs) != 2 || runs[1].ID != "2026-11-04|ds|a|2" {
			t.Fatalf("runs = %+v, want attempt 2", runs)
		}
		present, _ = led.Present(ctx, day)
		if present[keyA] != store.StatusPresent || led.cursor.Day != day {
			t.Fatalf("present=%v cursor=%s", present, led.cursor.Day)
		}
	})

	t.Run("three retryable errors are not capped", func(t *testing.T) {
		t.Parallel()
		clk := clockwork.NewFakeClockAt(utc("2026-11-04T15:00:00Z"))
		led := newLedger()
		led.cursor = store.Cursor{Day: "2026-11-03"}
		job := &scriptJob{keys: []contracts.ExpectedKey{keyA}, fn: func(contracts.Day, int) contracts.JobResult { return retryable }}
		rec := newScriptRec(led, oneDay(), clk, job)
		for range 3 {
			if err := rec.Reconcile(ctx, day); err != nil {
				t.Fatal(err)
			}
		}
		if job.calls() != 3 {
			t.Fatalf("calls = %d, want 3", job.calls())
		}
		if led.cursor.Day != "2026-11-03" {
			t.Fatalf("cursor = %s, want held", led.cursor.Day)
		}
	})
}

func TestRetryableAtOrAfterCloseExpired(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	const day = contracts.Day("2026-11-04")
	closeAt := utc("2026-11-04T21:00:00Z")
	for _, tc := range []struct {
		name  string
		after time.Duration
	}{
		{"at close", 0},
		{"after close", time.Hour},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			clk := clockwork.NewFakeClockAt(utc("2026-11-04T15:00:00Z"))
			led := newLedger()
			led.cursor = store.Cursor{Day: "2026-11-03"}
			job := &scriptJob{keys: []contracts.ExpectedKey{keyA}, fn: func(contracts.Day, int) contracts.JobResult { return retryable }}
			rec := newScriptRec(led, oneDay(), clk, job)
			if err := rec.Reconcile(ctx, day); err != nil {
				t.Fatal(err)
			}
			clk.Advance(closeAt.Add(tc.after).Sub(clk.Now()))
			if err := rec.Reconcile(ctx, day); err != nil {
				t.Fatal(err)
			}
			if job.calls() != 1 {
				t.Fatalf("calls = %d, want 1: no new run", job.calls())
			}
			present, _ := led.Present(ctx, day)
			if present[keyA] != store.StatusNoValue {
				t.Fatalf("keyA = %q, want no_value", present[keyA])
			}
			runs, _ := led.Runs(ctx, day)
			if len(runs) != 1 || runs[0].ReasonCode != "expired" {
				t.Fatalf("runs = %+v, want one run with reason expired", runs)
			}
		})
	}
}

func TestExpiredDayCompletesCursorAdvances(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	first := func(day contracts.Day, _ int) contracts.JobResult {
		if day == "2026-11-04" {
			return retryable
		}
		return contracts.JobResult{Returned: 1, Usable: 1}
	}

	t.Run("windowed", func(t *testing.T) {
		t.Parallel()
		clk := clockwork.NewFakeClockAt(utc("2026-11-04T15:00:00Z"))
		led := newLedger()
		led.cursor = store.Cursor{Day: "2026-11-03"}
		job := &scriptJob{keys: []contracts.ExpectedKey{keyA}, fn: first}
		rec := newScriptRec(led, oneDay(), clk, job)
		if err := rec.Reconcile(ctx, "2026-11-05"); err != nil {
			t.Fatal(err)
		}
		if led.cursor.Day != "2026-11-03" {
			t.Fatalf("cursor = %s, want held at 2026-11-03", led.cursor.Day)
		}
		clk.Advance(utc("2026-11-05T22:00:00Z").Sub(clk.Now()))
		if err := rec.Reconcile(ctx, "2026-11-05"); err != nil {
			t.Fatal(err)
		}
		if led.cursor.Day != "2026-11-05" {
			t.Fatalf("cursor = %s, want 2026-11-05", led.cursor.Day)
		}
		present, _ := led.Present(ctx, "2026-11-04")
		if present[keyA] != store.StatusNoValue {
			t.Fatalf("keyA on 11-04 = %q, want no_value", present[keyA])
		}
	})

	t.Run("no window stays missing", func(t *testing.T) {
		t.Parallel()
		// Sessions without a CloseAt give the dataset no close: closeAt is zero.
		cal := fakeCalendar{sessions: []clock.Session{
			{CalendarID: "XNYS", Day: "2026-11-04", IsSession: true},
		}}
		clk := clockwork.NewFakeClockAt(utc("2026-11-06T00:00:00Z"))
		led := newLedger()
		led.cursor = store.Cursor{Day: "2026-11-03"}
		job := &scriptJob{keys: []contracts.ExpectedKey{keyA}, fn: first}
		rec := newScriptRec(led, cal, clk, job)
		for range 2 {
			if err := rec.Reconcile(ctx, "2026-11-04"); err != nil {
				t.Fatal(err)
			}
		}
		present, _ := led.Present(ctx, "2026-11-04")
		if _, ok := present[keyA]; ok {
			t.Fatalf("keyA = %q, want missing", present[keyA])
		}
		runs, _ := led.Runs(ctx, "2026-11-04")
		for _, r := range runs {
			if r.ReasonCode == "expired" {
				t.Fatalf("run %s expired without a window", r.ID)
			}
		}
		if led.cursor.Day != "2026-11-03" {
			t.Fatalf("cursor = %s, want held", led.cursor.Day)
		}
	})
}

func TestCrashedRunIsNotExpired(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	const day = contracts.Day("2026-11-04")
	clk := clockwork.NewFakeClockAt(utc("2026-11-04T22:00:00Z"))
	led := newLedger()
	led.cursor = store.Cursor{Day: "2026-11-03"}
	for _, id := range []string{"2026-11-04|ds|a|1", "2026-11-04|ds|a|2"} {
		led.runs[day] = append(led.runs[day], store.Run{
			ID: id, Day: day, Key: keyA, Status: store.RunDone,
			ErrorCode: string(contracts.Retryable), ReasonCode: reasonCrashed,
		})
	}
	job := &scriptJob{keys: []contracts.ExpectedKey{keyA}, fn: func(contracts.Day, int) contracts.JobResult { return retryable }}
	if err := newScriptRec(led, oneDay(), clk, job).Reconcile(ctx, day); err != nil {
		t.Fatal(err)
	}
	present, _ := led.Present(ctx, day)
	if _, ok := present[keyA]; ok {
		t.Fatalf("keyA = %q, want missing", present[keyA])
	}
	for _, r := range led.runs[day][:2] {
		if r.ReasonCode == "expired" {
			t.Fatalf("crashed run %s was expired", r.ID)
		}
	}
	// A crashed run does not count (runner v4): below the brake of three a late run starts.
	if runs := led.runs[day]; job.calls() != 1 || len(runs) != 3 || runs[2].ReasonCode != "late" || led.cursor.Day != "2026-11-03" {
		t.Fatalf("calls=%d runs=%+v cursor=%s, want a third late run and cursor held", job.calls(), led.runs[day], led.cursor.Day)
	}
}
