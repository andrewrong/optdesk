package reconcile

import (
	"context"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
)

var _ runner.DayPlanner = (*reconciler)(nil)

// PlanDay stores the plan and the windows of one day and nothing else (Decision 7 of
// docs/design/single-runner.md): it plans like reconcileOn, puts the expected keys when the stored hash
// differs and puts the windows, then returns. It runs no job, sets no cursor and takes no lease; an overlap
// with Reconcile of the same day is safe because both write the same plan and windows. A day that is no
// session of any calendar of the jobs has no plan and is left alone.
func (r *reconciler) PlanDay(ctx context.Context, day contracts.Day) error {
	_, sessionsByDay, err := r.walk(ctx, "", day)
	if err != nil {
		return err
	}
	sess, ok := sessionsByDay[day]
	if !ok {
		return nil
	}
	p, err := r.plan(ctx, day, sess)
	if err != nil {
		return err
	}
	if _, old, err := r.ledger.Expected(ctx, day); err != nil {
		return err
	} else if old != p.hash {
		if err := r.ledger.PutExpected(ctx, day, p.hash, p.keys); err != nil {
			return err
		}
	}
	_, err = r.putWindows(ctx, p, sess)
	return err
}
