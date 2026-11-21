package reconcile

import (
	"context"
	"sync"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// emptyThenUsable is a dependent that answers no rows on its first usableAt-1 calls and one usable row after.
type emptyThenUsable struct {
	depJob
	usableAt int
	mu       sync.Mutex
	n        int
}

func (j *emptyThenUsable) Run(_ context.Context, _ contracts.Day, keys []contracts.ExpectedKey) contracts.JobResult {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.n++
	j.calls = append(j.calls, keys...)
	if j.n >= j.usableAt {
		return contracts.JobResult{Returned: 1, Usable: 1}
	}
	return contracts.JobResult{}
}

func runsOfKey(led *fakeLedger, k contracts.ExpectedKey) []store.Run {
	led.mu.Lock()
	defer led.mu.Unlock()
	var out []store.Run
	for _, r := range led.runs[dueDay] {
		if r.Key == k {
			out = append(out, r)
		}
	}
	return out
}

func TestEmptyMarkedDependentRerunsWhenUpstreamOk(t *testing.T) {
	t.Parallel()
	led, up, d := rerunSetup(map[contracts.ExpectedKey]int{keyB: 1})
	e := &emptyThenUsable{depJob: d, usableAt: 2}
	rec := dueRec(led, "2026-11-05T20:55:00Z", up, e)
	reconcileN(t, rec, 1)
	if r := rowOf(t, led, dueDay, keyC); r.ReasonCode != reasonUpstreamPartial {
		t.Fatalf("row %+v, want upstream_missing kept on the empty run", r)
	}
	reconcileN(t, rec, 1)
	runs := runsOfKey(led, keyC)
	if len(e.calls) != 2 || len(runs) != 2 || runs[0].ReasonCode != reasonUpstreamPartial || runs[1].ReasonCode != "" {
		t.Fatalf("calls %d runs %+v, want a marked run then an unmarked one", len(e.calls), runs)
	}
	reconcileN(t, rec, 3)
	if len(e.calls) != 2 {
		t.Fatalf("dep calls %d, want 2", len(e.calls))
	}
}

func TestEmptyMarkedDependentSecondEmptyRunIsNotRepeated(t *testing.T) {
	t.Parallel()
	led, up, d := rerunSetup(map[contracts.ExpectedKey]int{keyB: 1})
	e := &emptyThenUsable{depJob: d, usableAt: 100}
	rec := dueRec(led, "2026-11-05T20:55:00Z", up, e)
	reconcileN(t, rec, 2)
	runs := runsOfKey(led, keyC)
	if len(runs) != 2 || runs[1].ReasonCode != reasonEmptyBeforeClose {
		t.Fatalf("runs %+v, want the second run empty_before_close", runs)
	}
	reconcileN(t, rec, 1) // upstream end is past; only the six hour gap may run it again
	if len(e.calls) != 2 {
		t.Fatalf("dep calls %d, want 2", len(e.calls))
	}
}

func TestEmptyMarkedDependentNotRerunByFailedUpstream(t *testing.T) {
	t.Parallel()
	led, up, d := rerunSetup(map[contracts.ExpectedKey]int{keyB: 100})
	e := &emptyThenUsable{depJob: d, usableAt: 2}
	rec := dueRec(led, "2026-11-05T20:55:00Z", up, e)
	reconcileN(t, rec, 3)
	if len(e.calls) != 1 {
		t.Fatalf("dep calls %d, want 1 while the upstream is not ok", len(e.calls))
	}
	if r := rowOf(t, led, dueDay, keyC); r.ReasonCode != reasonUpstreamPartial {
		t.Fatalf("row %+v, want upstream_missing", r)
	}
}
