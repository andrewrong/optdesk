package reconcile

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
)

var _ runner.NewBackfillerWithRuleFunc = NewBackfillerWithRule

// reasonBackfill is the ReasonCode of a run started by Backfill.
const reasonBackfill = "backfill"

// NewBackfillerWithRule builds the backfiller with an explicit window rule.
func NewBackfillerWithRule(l store.OpsLedger, cal clock.Calendar, clk clock.Clock, jobs []contracts.Job, rule runner.WindowRule) runner.Backfiller {
	return newReconciler(l, nil, cal, clk, jobs, rule, nil)
}

// Backfill runs the keys of a day at or behind the cursor that are missing or no_value.
// It never calls SetCursor. A key that today's plan has and the stored plan lacks is an added key:
// it joins the stored plan (PutExpected, and PutWindow for its dataset) before the runs.
func (r *reconciler) Backfill(ctx context.Context, day contracts.Day, datasets []string) (res runner.BackfillResult, err error) {
	res = runner.BackfillResult{Day: day}
	ctx, release, err := r.holdLease(ctx)
	if err != nil {
		return res, err
	}
	defer release(&err)

	cur, err := r.ledger.Cursor(ctx)
	if err != nil {
		return res, err
	}
	if cur.Day == "" || day > cur.Day {
		res.Refused = runner.RefusedAheadOfCursor
		return res, nil
	}
	stored, _, err := r.ledger.Expected(ctx, day)
	if err != nil {
		return res, err
	}
	if len(stored) == 0 {
		res.Refused = runner.RefusedNoPlan
		return res, nil
	}
	keys, p, err := r.backfillPlan(ctx, day, stored, datasets)
	if err != nil {
		return res, err
	}

	present, byKey, err := r.dayState(ctx, day)
	if err != nil {
		return res, err
	}
	now := r.clk.Now()
	for _, k := range keys {
		if present[k] == store.StatusPresent {
			res.AlreadyPresent = append(res.AlreadyPresent, k)
			continue
		}
		live, err := r.closeExpired(ctx, byKey[k], now)
		if err != nil {
			return res, err
		}
		if live {
			res.Missing = append(res.Missing, k)
			continue
		}
		closeAt, err := r.closeOf(ctx, k.DatasetID, day)
		if err != nil {
			return res, err
		}
		out, err := r.backfillKey(ctx, p.job[k], day, k, len(byKey[k])+1, closeAt)
		if err != nil {
			return res, err
		}
		switch out {
		case store.StatusPresent:
			res.Filled = append(res.Filled, k)
		case store.StatusNoValue:
			res.Absent = append(res.Absent, k)
		default:
			res.Missing = append(res.Missing, k)
		}
	}
	return res, nil
}

// dayState reads the present marks and the runs of the day, the runs grouped by key.
func (r *reconciler) dayState(
	ctx context.Context, day contracts.Day,
) (map[contracts.ExpectedKey]store.PresentStatus, map[contracts.ExpectedKey][]store.Run, error) {
	present, err := r.ledger.Present(ctx, day)
	if err != nil {
		return nil, nil, err
	}
	runs, err := r.ledger.Runs(ctx, day)
	if err != nil {
		return nil, nil, err
	}
	byKey := map[contracts.ExpectedKey][]store.Run{}
	for _, run := range runs {
		byKey[run.Key] = append(byKey[run.Key], run)
	}
	return present, byKey, nil
}

// backfillPlan selects the keys to run from the stored keys and the added keys, reads the
// key-to-job map; every key is validated before the added keys are written and before any key runs.
func (r *reconciler) backfillPlan(
	ctx context.Context, day contracts.Day, stored []contracts.ExpectedKey, datasets []string,
) ([]contracts.ExpectedKey, plan, error) {
	p, err := r.plan(ctx, day, nil)
	if err != nil {
		return nil, plan{}, err
	}
	have := map[contracts.ExpectedKey]bool{}
	for _, k := range stored {
		have[k] = true
	}
	want := map[string]bool{}
	for _, id := range datasets {
		want[id] = true
	}
	var added []contracts.ExpectedKey
	for _, k := range p.keys {
		if !have[k] && (len(datasets) == 0 || want[k.DatasetID]) {
			have[k] = true
			added = append(added, k)
		}
	}
	if len(datasets) == 0 {
		datasets = nil
	}
	all := append(append([]contracts.ExpectedKey{}, stored...), added...)
	keys, err := selectKeys(day, all, datasets)
	if err != nil {
		return nil, plan{}, err
	}
	for _, k := range keys {
		if _, ok := p.job[k]; !ok {
			return nil, plan{}, fmt.Errorf("backfill %s: no job returns a key of dataset %s", day, k.DatasetID)
		}
	}
	if len(added) > 0 {
		if err := r.storeAdded(ctx, day, p, all, added); err != nil {
			return nil, plan{}, err
		}
	}
	return keys, p, nil
}

// storeAdded writes all (the stored keys and the added) as the stored plan of day and the window of each added dataset.
func (r *reconciler) storeAdded(
	ctx context.Context, day contracts.Day, p plan, all, added []contracts.ExpectedKey,
) error {
	if err := r.ledger.PutExpected(ctx, day, p.hash, all); err != nil {
		return err
	}
	parsed, err := time.Parse("2006-01-02", string(day))
	if err != nil {
		return fmt.Errorf("backfill %s: %w", day, err)
	}
	_, sessions, err := r.walk(ctx, contracts.Day(parsed.AddDate(0, 0, -1).Format("2006-01-02")), day)
	if err != nil {
		return err
	}
	_, err = r.putWindows(ctx, plan{keys: added, job: p.job}, sessions[day])
	return err
}

// selectKeys filters the stored keys by dataset ids; an id with no stored key is an error.
func selectKeys(day contracts.Day, stored []contracts.ExpectedKey, datasets []string) ([]contracts.ExpectedKey, error) {
	if datasets == nil {
		return stored, nil
	}
	want := map[string]bool{}
	for _, id := range datasets {
		want[id] = false
	}
	var out []contracts.ExpectedKey
	for _, k := range stored {
		if _, ok := want[k.DatasetID]; ok {
			want[k.DatasetID] = true
			out = append(out, k)
		}
	}
	for _, id := range datasets {
		if !want[id] {
			return nil, fmt.Errorf("backfill %s: dataset %s has no key on the day", day, id)
		}
	}
	return out, nil
}

// closeExpired closes every running run whose lease expired as crashed and reports
// whether a running run with a live lease remains.
func (r *reconciler) closeExpired(ctx context.Context, runs []store.Run, now time.Time) (bool, error) {
	live := false
	for _, run := range runs {
		if run.Status != store.RunRunning {
			continue
		}
		if !run.LeaseUntil.Before(now) {
			live = true
			continue
		}
		if err := r.markCrashed(ctx, run); err != nil {
			return false, err
		}
	}
	return live, nil
}

// closeOf is the stored window close of the dataset, zero when it has no window.
func (r *reconciler) closeOf(ctx context.Context, datasetID string, day contracts.Day) (time.Time, error) {
	w, err := r.ledger.Window(ctx, datasetID, day)
	if err != nil {
		var te contracts.TypedError
		if errors.As(err, &te) && te.Kind() == contracts.NoValue {
			return time.Time{}, nil
		}
		return time.Time{}, err
	}
	return w.CloseAt, nil
}

// backfillKey runs k once and reports its mark afterwards: StatusPresent (filled),
// StatusNoValue (absent) or "" (missing: no usable answer, a job error leaves the key
// unmarked, or no_value as it was, told apart by the ErrorCode of the new run row).
func (r *reconciler) backfillKey(
	ctx context.Context, job contracts.Job, day contracts.Day, k contracts.ExpectedKey, seq int, closeAt time.Time,
) (store.PresentStatus, error) {
	rec, _, err := r.run(ctx, job, day, k, seq, closeAt, reasonBackfill)
	if err != nil {
		return "", err
	}
	present, err := r.ledger.Present(ctx, day)
	if err != nil {
		return "", err
	}
	status := present[k]
	if status != store.StatusNoValue {
		return status, nil
	}
	if rec.ErrorCode != "" {
		return "", nil
	}
	return status, nil
}
