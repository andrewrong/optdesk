package reconcile

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
)

// flakyUp is the job "up": a key answers a retryable error for the first fails[key] calls, then one usable row.
type flakyUp struct {
	keys  []contracts.ExpectedKey
	fails map[contracts.ExpectedKey]int
	mu    sync.Mutex
	n     map[contracts.ExpectedKey]int
}

func (j *flakyUp) ID() string { return "up" }
func (j *flakyUp) Plan(context.Context, contracts.Day) ([]contracts.ExpectedKey, string, error) {
	return j.keys, "plan-1", nil
}

func (j *flakyUp) Run(_ context.Context, _ contracts.Day, keys []contracts.ExpectedKey) contracts.JobResult {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.n == nil {
		j.n = map[contracts.ExpectedKey]int{}
	}
	j.n[keys[0]]++
	if j.n[keys[0]] <= j.fails[keys[0]] {
		return contracts.JobResult{Err: kindErr{kind: contracts.Retryable}}
	}
	return contracts.JobResult{Returned: 1, Usable: 1}
}

// rerunSetup builds a ledger of dueDay, a clock between the slot of dep and the close, the flaky upstream
// of keyA and keyB and the dependent of keyC.
func rerunSetup(fails map[contracts.ExpectedKey]int) (*fakeLedger, *flakyUp, depJob) {
	up := &flakyUp{keys: []contracts.ExpectedKey{keyA, keyB}, fails: fails}
	return dueLedger(), up, dep(-10*time.Minute, keyC)
}

func reconcileN(t *testing.T, rec runner.Reconciler, n int) {
	t.Helper()
	for range n {
		if err := rec.Reconcile(context.Background(), dueDay); err != nil {
			t.Fatal(err)
		}
	}
}

func TestMarkedDependentRerunsWhenUpstreamOk(t *testing.T) {
	t.Parallel()
	led, up, d := rerunSetup(map[contracts.ExpectedKey]int{keyB: 1})
	rec := dueRec(led, "2026-11-05T20:55:00Z", up, d)
	reconcileN(t, rec, 1)
	if len(d.calls) != 1 {
		t.Fatalf("first turn: dep calls %d, want 1", len(d.calls))
	}
	if r := rowOf(t, led, dueDay, keyC); r.ReasonCode != "upstream_missing" {
		t.Fatalf("row %+v, want upstream_missing", r)
	}
	reconcileN(t, rec, 1)
	if len(d.calls) != 2 {
		t.Fatalf("dep calls %d, want 2", len(d.calls))
	}
	var runs []store.Run
	for _, r := range led.runs[dueDay] {
		if r.Key == keyC {
			runs = append(runs, r)
		}
	}
	if len(runs) != 2 || !strings.HasSuffix(runs[1].ID, "|2") || runs[1].ReasonCode != "" || runs[0].ReasonCode != "upstream_missing" {
		t.Fatalf("runs of dep %+v, want attempt 2 with no reason after the marked one", runs)
	}
}

func TestRerunIsOnce(t *testing.T) {
	t.Parallel()
	led, up, d := rerunSetup(map[contracts.ExpectedKey]int{keyB: 1})
	rec := dueRec(led, "2026-11-05T20:55:00Z", up, d)
	reconcileN(t, rec, 2)
	reconcileN(t, rec, 5)
	if len(d.calls) != 2 {
		t.Fatalf("dep calls %d, want 2", len(d.calls))
	}
}

func TestRerunWaitsWhileUpstreamStillMissing(t *testing.T) {
	t.Parallel()
	led, up, d := rerunSetup(map[contracts.ExpectedKey]int{keyA: 1, keyB: 2})
	rec := dueRec(led, "2026-11-05T20:55:00Z", up, d)
	reconcileN(t, rec, 2) // keyA retried ok, keyB fails again
	if len(d.calls) != 1 {
		t.Fatalf("dep calls %d, want 1 while keyB is missing", len(d.calls))
	}
	reconcileN(t, rec, 1)
	if len(d.calls) != 2 {
		t.Fatalf("dep calls %d, want 2 when keyB is ok", len(d.calls))
	}
}

func TestRerunAfterRestart(t *testing.T) {
	t.Parallel()
	led, up, d := rerunSetup(map[contracts.ExpectedKey]int{keyB: 1})
	reconcileN(t, dueRec(led, "2026-11-05T20:55:00Z", up, d), 1)
	if len(d.calls) != 1 {
		t.Fatalf("dep calls %d, want 1", len(d.calls))
	}
	reconcileN(t, dueRec(led, "2026-11-05T20:55:00Z", up, d), 1) // a new reconciler over the same ledger
	if len(d.calls) != 2 {
		t.Fatalf("dep calls %d after restart, want 2", len(d.calls))
	}
}

func TestUnmarkedDependentNotRerun(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	l := bfSetup(bfDay, keyA, keyC)
	up, d := upJob(keyA), dep(0, keyC)
	rec := forceRec(l, afterClose(), up, d)
	if _, err := rec.BackfillWith(ctx, bfDay, nil, runner.BackfillOptions{Job: "up"}); err != nil {
		t.Fatal(err)
	}
	if r := rowOf(t, l.fakeLedger, bfDay, keyC); len(d.calls) != 1 || r.ReasonCode == "upstream_missing" {
		t.Fatalf("dep calls %d row %+v, want one unmarked run", len(d.calls), r)
	}
	if _, err := rec.BackfillWith(ctx, bfDay, nil, runner.BackfillOptions{Job: "up", Force: true}); err != nil {
		t.Fatal(err)
	}
	if len(up.calls) != 2 || len(d.calls) != 1 {
		t.Fatalf("up %d dep %d, want 2 and 1", len(up.calls), len(d.calls))
	}
}
