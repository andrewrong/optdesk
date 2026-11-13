package reconcile

import (
	"context"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
)

func TestPlanDayStoresPlanAndWindowsOnly(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	l := bfSetup("2026-11-03")
	a := &fakeJob{id: "a", keys: []contracts.ExpectedKey{keyA}}
	b := &fakeJob{id: "b", keys: []contracts.ExpectedKey{{DatasetID: "ds2", SubjectKey: "b"}}}
	rec := forceRec(l, afterClose(), a, b)
	p, ok := any(rec).(runner.DayPlanner)
	if !ok {
		t.Fatal("reconciler is not a DayPlanner")
	}
	if err := p.PlanDay(ctx, bfDay); err != nil {
		t.Fatal(err)
	}
	if got := l.expected[bfDay]; len(got) != 2 {
		t.Fatalf("expected keys %v", got)
	}
	if len(l.windows) != 2 {
		t.Fatalf("windows %v", l.windows)
	}
	if len(a.calls) != 0 || len(b.calls) != 0 {
		t.Fatalf("Run called: %d %d", len(a.calls), len(b.calls))
	}
	if l.cursors.Load() != 0 || l.cursor.Day != "2026-11-03" {
		t.Fatalf("cursor moved: sets %d now %v", l.cursors.Load(), l.cursor)
	}
	if l.acquired.Load() != 0 || l.released.Load() != 0 {
		t.Fatalf("lease taken: %d", l.acquired.Load())
	}
	if l.records.Load() != 0 || l.marks.Load() != 0 {
		t.Fatal("run recorded or key marked")
	}

	// An unchanged plan writes no PutExpected.
	if err := p.PlanDay(ctx, bfDay); err != nil {
		t.Fatal(err)
	}
	if l.expecteds.Load() != 1 {
		t.Fatalf("PutExpected called %d times", l.expecteds.Load())
	}
}
