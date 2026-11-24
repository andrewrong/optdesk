package reconcile

import (
	"context"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/jonboulle/clockwork"
)

func cappedRuns(n int) []store.Run {
	runs := make([]store.Run, n)
	for i := range runs {
		runs[i] = retryableRun(string(rune('1'+i)), utc("2026-11-04T15:00:00Z"))
	}
	return runs
}

func newCapRec(now string) *reconciler {
	job := &scriptJob{keys: []contracts.ExpectedKey{keyA}}
	led := newLedger()
	led.windows[keyA.DatasetID+"|"+string(lateDay)] = store.Window{CloseAt: lateCloseAt}
	return newReconciler(led, nil, oneDay(), clockwork.NewFakeClockAt(utc(now)), []contracts.Job{job}, runner.WindowRule{CalendarID: "XNYS"}, nil)
}

func TestRetryableRunsBeforeCloseAreCapped(t *testing.T) {
	t.Parallel()
	rec := newCapRec("2026-11-04T16:00:00Z")

	if _, start, _ := rec.nextAttempt(cappedRuns(retryBeforeCloseCap-1), lateCloseAt); !start {
		t.Fatalf("%d retryable runs before the close: no run starts, want one", retryBeforeCloseCap-1)
	}
	runs := cappedRuns(retryBeforeCloseCap)
	if _, start, _ := rec.nextAttempt(runs, lateCloseAt); start {
		t.Fatalf("%d retryable runs before the close: a run starts, want none", retryBeforeCloseCap)
	}

	dc := &dayCtx{day: lateDay}
	state, err := rec.upstreamEnded(context.Background(), dc, rec.jobs[0], lateDay, []contracts.ExpectedKey{keyA},
		map[contracts.ExpectedKey]store.PresentStatus{}, map[contracts.ExpectedKey][]store.Run{keyA: runs})
	if err != nil {
		t.Fatal(err)
	}
	if state != upEnded {
		t.Fatalf("upstreamEnded = %v, want upEnded", state)
	}
}

func TestRetryCapIgnoredAfterClose(t *testing.T) {
	t.Parallel()
	rec := newCapRec("2026-11-04T21:05:00Z")
	_, start, reason := rec.nextAttempt(cappedRuns(retryBeforeCloseCap), lateCloseAt)
	if !start || reason != reasonLate {
		t.Fatalf("after the close: start %v reason %q, want the late run", start, reason)
	}
}
