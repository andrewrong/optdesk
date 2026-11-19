package reconcile

import (
	"context"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/jonboulle/clockwork"
)

const lateDay = contracts.Day("2026-11-04")

var lateCloseAt = utc("2026-11-04T21:00:00Z")

func retryableRun(seq string, finished time.Time) store.Run {
	return store.Run{
		ID: "2026-11-04|ds|a|" + seq, Day: lateDay, Key: keyA, Status: store.RunDone,
		ErrorCode: string(contracts.Retryable), FinishedAt: finished,
	}
}

func TestRetryableBeforeCloseGetsLateRun(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	clk := clockwork.NewFakeClockAt(utc("2026-11-04T21:05:00Z"))
	led := newLedger()
	led.cursor = store.Cursor{Day: "2026-11-03"}
	first := retryableRun("1", utc("2026-11-04T15:00:00Z"))
	led.runs[lateDay] = []store.Run{first}
	job := &scriptJob{keys: []contracts.ExpectedKey{keyA}, fn: func(contracts.Day, int) contracts.JobResult { return retryable }}
	rec := newScriptRec(led, oneDay(), clk, job)

	if err := rec.Reconcile(ctx, lateDay); err != nil {
		t.Fatal(err)
	}
	runs := led.runs[lateDay]
	if led.marks != 0 {
		t.Fatalf("MarkPresent called %d times, want none", led.marks)
	}
	if len(runs) != 2 || runs[0] != first {
		t.Fatalf("runs = %+v, want the first run untouched and a second", runs)
	}
	if runs[1].ReasonCode != reasonLate || job.calls() != 1 {
		t.Fatalf("second run reason %q calls %d, want late and 1", runs[1].ReasonCode, job.calls())
	}
}

func TestLateRetryableRunIsExpired(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	clk := clockwork.NewFakeClockAt(utc("2026-11-04T21:05:00Z"))
	led := newLedger()
	led.cursor = store.Cursor{Day: "2026-11-03"}
	led.runs[lateDay] = []store.Run{retryableRun("1", utc("2026-11-04T15:00:00Z"))}
	job := &scriptJob{keys: []contracts.ExpectedKey{keyA}, fn: func(contracts.Day, int) contracts.JobResult { return retryable }}
	rec := newScriptRec(led, oneDay(), clk, job)

	if err := rec.Reconcile(ctx, lateDay); err != nil { // the late run, retryable
		t.Fatal(err)
	}
	clk.Advance(time.Minute)
	if err := rec.Reconcile(ctx, lateDay); err != nil {
		t.Fatal(err)
	}
	runs := led.runs[lateDay]
	if led.present[lateDay][keyA] != store.StatusNoValue {
		t.Fatalf("key = %q, want no_value", led.present[lateDay][keyA])
	}
	if len(runs) != 2 || runs[1].ReasonCode != reasonExpired || job.calls() != 1 {
		t.Fatalf("runs = %+v calls %d, want the late run expired and no third run", runs, job.calls())
	}
}

func TestRetryableAfterCloseExpires(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		finished time.Time
	}{
		{"at close", lateCloseAt},
		{"after close", lateCloseAt.Add(time.Minute)},
		{"zero finish", time.Time{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			clk := clockwork.NewFakeClockAt(utc("2026-11-04T21:05:00Z"))
			led := newLedger()
			runs := []store.Run{retryableRun("1", tc.finished)}
			led.runs[lateDay] = runs
			rec := forceRec(led, clk, &fakeJob{keys: []contracts.ExpectedKey{keyA}})
			closed, err := rec.expireIfClosed(ctx, runs, lateCloseAt, lateDay, keyA)
			if err != nil || !closed {
				t.Fatalf("closed %v err %v, want closed", closed, err)
			}
			if led.runs[lateDay][0].ReasonCode != reasonExpired || led.present[lateDay][keyA] != store.StatusNoValue {
				t.Fatalf("run %+v key %q", led.runs[lateDay][0], led.present[lateDay][keyA])
			}
		})
	}
}
