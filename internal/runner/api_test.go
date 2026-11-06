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
