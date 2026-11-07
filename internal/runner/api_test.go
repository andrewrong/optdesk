package runner_test

import (
	"context"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
)

type fakeReconciler struct{ days []contracts.Day }

func (f *fakeReconciler) Reconcile(_ context.Context, day contracts.Day) error {
	f.days = append(f.days, day)
	return nil
}

func TestReconcilerContract(t *testing.T) {
	t.Parallel()
	f := &fakeReconciler{}
	var newFn runner.NewFunc = func(store.OpsLedger, clock.Calendar, clock.Clock, []contracts.Job) runner.Reconciler { return f }
	r := newFn(nil, nil, nil, nil)
	if err := r.Reconcile(context.Background(), "2026-11-04"); err != nil {
		t.Fatal(err)
	}
	if len(f.days) != 1 || f.days[0] != "2026-11-04" {
		t.Fatalf("days = %v", f.days)
	}
}

func TestWindowRuleContract(t *testing.T) {
	t.Parallel()
	var got runner.WindowRule
	f := &fakeReconciler{}
	var newFn runner.NewWithRuleFunc = func(_ store.OpsLedger, _ clock.Calendar, _ clock.Clock, _ []contracts.Job, rule runner.WindowRule) runner.Reconciler {
		got = rule
		return f
	}
	want := runner.WindowRule{CalendarID: "X", OpenOffset: time.Minute, CloseOffset: time.Hour}
	r := newFn(nil, nil, nil, nil, want)
	if got != want {
		t.Fatalf("rule = %+v, want %+v", got, want)
	}
	if err := r.Reconcile(context.Background(), "2026-11-04"); err != nil {
		t.Fatal(err)
	}
	if len(f.days) != 1 {
		t.Fatalf("days = %v", f.days)
	}
}

type fakeBackfiller struct {
	result runner.BackfillResult
	gotDay contracts.Day
	gotIDs []string
}

func (f *fakeBackfiller) Backfill(_ context.Context, day contracts.Day, datasets []string) (runner.BackfillResult, error) {
	f.gotDay, f.gotIDs = day, datasets
	return f.result, nil
}

// describe is a caller: it branches on Refused, never on an error.
func describe(res runner.BackfillResult) string {
	switch res.Refused {
	case runner.RefusedAheadOfCursor:
		return "run-once first"
	case runner.RefusedNoPlan:
		return "no plan"
	default:
		return "done"
	}
}

func TestBackfillerRefusalIsDataWithNilError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		refused runner.BackfillRefusal
		wantStr string
		wantTxt string
	}{
		{"ahead of cursor", runner.RefusedAheadOfCursor, "ahead_of_cursor", "run-once first"},
		{"no plan", runner.RefusedNoPlan, "no_plan", "no plan"},
		{"not refused", "", "", "done"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := &fakeBackfiller{result: runner.BackfillResult{Refused: tc.refused}}
			var newFn runner.NewBackfillerWithRuleFunc = func(store.OpsLedger, clock.Calendar, clock.Clock, []contracts.Job, runner.WindowRule) runner.Backfiller {
				return f
			}
			b := newFn(nil, nil, nil, nil, runner.WindowRule{})
			res, err := b.Backfill(context.Background(), "2026-11-06", []string{"DGS10"})
			if err != nil {
				t.Fatalf("a refusal carries a nil error, got %v", err)
			}
			if string(res.Refused) != tc.wantStr {
				t.Fatalf("Refused = %q, want %q", res.Refused, tc.wantStr)
			}
			if got := describe(res); got != tc.wantTxt {
				t.Fatalf("describe = %q, want %q", got, tc.wantTxt)
			}
			if f.gotDay != "2026-11-06" || len(f.gotIDs) != 1 || f.gotIDs[0] != "DGS10" {
				t.Fatalf("arguments = %v %v", f.gotDay, f.gotIDs)
			}
		})
	}
}
