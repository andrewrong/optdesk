package reconcile

import (
	"context"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
)

// slotUp is the flaky upstream with a slot (session close + offset) and a run bound.
type slotUp struct {
	*flakyUp
	offset, bound time.Duration
}

func (j slotUp) Schedule() (string, string, string, time.Duration) {
	return "XNYS", "session_close", "", j.offset
}
func (j slotUp) RunBound() time.Duration { return j.bound }

func depRuns(led *fakeLedger) []store.Run { return runsOfKey(led, keyC) }

func TestDependentRunsWhenUpstreamEndedPartial(t *testing.T) {
	t.Parallel()
	led := dueLedger()
	up := upJob(keyA, keyB)
	up.results = map[contracts.ExpectedKey]contracts.JobResult{keyB: {Err: kindErr{kind: contracts.Permanent}}}
	d := dep(-10*time.Minute, keyC)
	rec := dueRec(led, "2026-11-05T20:55:00Z", up, d)
	reconcileN(t, rec, 1)
	runs := depRuns(led)
	if len(d.calls) != 1 || len(runs) != 1 || runs[0].ReasonCode != "upstream_partial" {
		t.Fatalf("dep calls %d runs %+v, want one run marked upstream_partial", len(d.calls), runs)
	}
	reconcileN(t, rec, 3)
	if len(d.calls) != 1 {
		t.Fatalf("dep calls %d after more ticks, want 1", len(d.calls))
	}
}

func TestPreOpenRunIsReplaced(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	l := bfSetup(bfDay, keyA, keyB, keyC)
	up, d := upJob(keyA, keyB), dep(0, keyC)
	up.results = map[contracts.ExpectedKey]contracts.JobResult{keyB: {Err: kindErr{kind: contracts.Permanent}}}
	rec := forceRec(l, afterClose(), up, d)
	if _, err := rec.BackfillWith(ctx, bfDay, nil, runner.BackfillOptions{Job: "dep"}); err != nil {
		t.Fatal(err)
	}
	if r := rowOf(t, l.fakeLedger, bfDay, keyC); r.ReasonCode != "upstream_missing" || len(d.calls) != 1 {
		t.Fatalf("row %+v dep %d, want one upstream_missing run", r, len(d.calls))
	}
	if _, err := rec.BackfillWith(ctx, bfDay, nil, runner.BackfillOptions{Job: "up"}); err != nil {
		t.Fatal(err)
	}
	var runs []store.Run
	for _, r := range l.runs[bfDay] {
		if r.Key == keyC {
			runs = append(runs, r)
		}
	}
	if len(d.calls) != 2 || len(runs) != 2 || runs[1].ReasonCode != "upstream_partial" {
		t.Fatalf("dep calls %d runs %+v, want a second run marked upstream_partial", len(d.calls), runs)
	}
}

// boundSetup is a ledger and a flaky upstream whose key B always answers retryable, with its slot at
// 20:30Z (close 21:00Z minus 30 min) and run bound 10 min, and a dependent due since 20:20Z.
func boundSetup() (*fakeLedger, slotUp, depJob) {
	up := slotUp{
		flakyUp: &flakyUp{keys: []contracts.ExpectedKey{keyA, keyB}, fails: map[contracts.ExpectedKey]int{keyB: 100}},
		offset:  -30 * time.Minute, bound: 10 * time.Minute,
	}
	return dueLedger(), up, dep(-40*time.Minute, keyC)
}

func TestRetryableUpstreamIsNotEnded(t *testing.T) {
	t.Parallel()
	led, up, d := boundSetup()
	rec := dueRec(led, "2026-11-05T20:35:00Z", up, d)
	reconcileN(t, rec, 1)
	if got := len(runsOfKey(led, keyB)); got == 0 {
		t.Fatal("the upstream did not run key B")
	}
	reconcileN(t, rec, 2)
	if len(d.calls) != 0 {
		t.Fatalf("dep calls %d, want 0 before slot + run bound", len(d.calls))
	}
}

func TestRetryableUpstreamEndsAtRunBound(t *testing.T) {
	t.Parallel()
	led, up, d := boundSetup()
	reconcileN(t, dueRec(led, "2026-11-05T20:35:00Z", up, d), 1)
	if len(d.calls) != 0 {
		t.Fatalf("dep calls %d before the bound, want 0", len(d.calls))
	}
	rec := dueRec(led, "2026-11-05T20:41:00Z", up, d) // after slot 20:30 + bound 10 min
	reconcileN(t, rec, 1)
	runs := depRuns(led)
	if len(d.calls) != 1 || len(runs) != 1 || runs[0].ReasonCode != "upstream_partial" {
		t.Fatalf("dep calls %d runs %+v, want one upstream_partial run after the bound", len(d.calls), runs)
	}
}

func TestPartialRunIsReplacedWhenUpstreamTurnsOK(t *testing.T) {
	t.Parallel()
	led, up, d := rerunSetup(map[contracts.ExpectedKey]int{keyB: 2})
	rec := dueRec(led, "2026-11-05T20:55:00Z", up, d)
	reconcileN(t, rec, 2) // key B fails twice: the upstream is ended, not ok
	runs := depRuns(led)
	if len(runs) != 1 || runs[0].ReasonCode != "upstream_partial" {
		t.Fatalf("runs %+v, want one upstream_partial run while B fails", runs)
	}
	reconcileN(t, rec, 1) // key B stores
	runs = depRuns(led)
	if len(runs) != 2 || runs[1].ReasonCode != "" {
		t.Fatalf("runs %+v, want an unmarked second run once the upstream is ok", runs)
	}
	reconcileN(t, rec, 4)
	if len(d.calls) != 2 {
		t.Fatalf("dep calls %d, want 2: the unmarked run is the last", len(d.calls))
	}
}
