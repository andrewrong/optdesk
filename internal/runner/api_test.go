package runner_test

import (
	"context"
	"testing"

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
