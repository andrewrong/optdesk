package reconcile

import (
	"context"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner/shared"
	"github.com/andrewrong/optdesk/internal/store"
)

func TestOrderedStopPartialIsNoAttemptAndNoStrike(t *testing.T) {
	t.Parallel()
	l := bfSetup(bfDay, keyA)
	rec := forceRec(l, afterClose(), &fakeJob{keys: []contracts.ExpectedKey{keyA}})
	mk := func(reason string) []store.Run {
		var runs []store.Run
		for _, id := range []string{"1", "2", "3"} {
			runs = append(runs, store.Run{ID: "2026-11-04|ds|a|" + id, Day: bfDay, Key: keyA, Status: store.RunPartial, ReasonCode: reason})
		}
		return runs
	}
	ordered := mk("ordered_stop")
	if _, start, _ := rec.nextAttempt(ordered, time.Time{}); !start {
		t.Fatal("three ordered_stop runs stop the key or use up its attempts")
	}
	if _, _, ok := closeRow(ordered, time.Time{}); ok {
		t.Fatal("closeRow finds an ordered_stop run")
	}
	if _, start, _ := rec.nextAttempt(mk("abandoned_stop"), time.Time{}); start {
		t.Fatal("three abandoned_stop runs do not stop the key")
	}
}

func TestRunKeyCutWithOrderedStopIsOrderedStop(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		returned int
		cause    error
		status   store.RunStatus
		reason   string
	}{
		{"partial", 2, shared.ErrOrderedStop, store.RunPartial, "ordered_stop"},
		{"no rows", 0, shared.ErrOrderedStop, store.RunAbandoned, "ordered_stop"},
		{"plain cut", 0, nil, store.RunAbandoned, "abandoned_stop"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			clk := clockwork.NewFakeClockAt(utc("2026-11-04T22:00:00Z"))
			job := newBlockJob(contracts.JobResult{Returned: tc.returned})
			ctx, cancel := context.WithCancelCause(context.Background())
			out := startRunKey(t, ctx, newRkLedger(), clk, job, job.started, clk.Now().Add(time.Hour))
			cancel(tc.cause)
			r := <-out
			if r.err != nil {
				t.Fatal(r.err)
			}
			rec := r.run
			if rec.Status != tc.status || rec.ReasonCode != tc.reason {
				t.Fatalf("run = %s %q, want %s %q", rec.Status, rec.ReasonCode, tc.status, tc.reason)
			}
		})
	}
}
