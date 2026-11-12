package reconcile

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
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

// Backfill first puts the windows the day lacks and adds the keys of the plan of the day that the
// stored plan lacks (Decision 8), then runs the keys of a day that are missing or no_value and whose own window
// has closed at the clock; a key still inside its window is not run and is listed in InWindow.
// A key whose dataset has no window on the day counts as closed. The cursor is not read and not
// moved. The run lease is taken under an owner of its own, so Reconcile and another backfill
// are excluded while it runs.
func (r *reconciler) Backfill(ctx context.Context, day contracts.Day, datasets []string) (res runner.BackfillResult, err error) {
	res = runner.BackfillResult{Day: day}
	ctx, release, err := r.holdLease(ctx, backfillOwner())
	if err != nil {
		return res, err
	}
	defer release(&err)

	stored, _, err := r.ledger.Expected(ctx, day)
	if err != nil {
		return res, err
	}
	if len(stored) == 0 {
		res.Refused = runner.RefusedNoPlan
		return res, nil
	}
	keys, p, _, err := r.backfillPlan(ctx, day, stored, datasets)
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
		closeAt, err := r.closeOf(ctx, k.DatasetID, day)
		if err != nil {
			return res, err
		}
		live, err := r.closeExpired(ctx, byKey[k], now)
		if err != nil {
			return res, err
		}
		if live {
			res.Missing = append(res.Missing, k)
			continue
		}
		_, out, err := r.backfillKey(ctx, p.job[k], day, k, len(byKey[k])+1, closeAt, deadlineFor(p.job[k], closeAt, r.clk.Now()), reasonBackfill)
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

// backfillOwner is the lease owner of one Backfill call: "backfill:" and eight random hex
// digits, never equal to the owner of Reconcile nor to that of another backfill.
func backfillOwner() string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err) // crypto/rand does not fail on supported platforms
	}
	return backfillOwnerPrefix + hex.EncodeToString(b[:])
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

// backfillPlan validates the day, then writes what it lacks: the windows of the datasets that
// have none and the keys of the plan of the day that the stored plan lacks (Decision 8). It
// returns the selected keys, the plan (the key-to-job map and, in held, the keys whose job is not yet due
// on the day) and the sessions of the day. Every error is returned before any write.
func (r *reconciler) backfillPlan(
	ctx context.Context, day contracts.Day, stored []contracts.ExpectedKey, datasets []string,
) ([]contracts.ExpectedKey, plan, daySessions, error) {
	sess, err := r.sessionsOf(ctx, day)
	if err != nil {
		return nil, plan{}, nil, err
	}
	pAll, err := r.plan(ctx, day, nil)
	if err != nil {
		return nil, plan{}, nil, err
	}
	// pAll has planned every job and logged the failing ones: pDay does not log them again.
	pDay, err := r.planLogged(ctx, day, sess, slog.New(slog.DiscardHandler))
	if err != nil {
		return nil, plan{}, nil, err
	}
	added := r.addedKeys(stored, pDay, sess)
	all := append(append([]contracts.ExpectedKey(nil), stored...), added...)
	keys, err := selectKeys(day, all, datasets)
	if err != nil {
		return nil, plan{}, nil, err
	}
	jobs := map[contracts.ExpectedKey]contracts.Job{}
	for k, j := range pAll.job {
		jobs[k] = j
	}
	for _, k := range added {
		jobs[k] = pDay.job[k]
	}
	for _, k := range keys {
		if _, ok := jobs[k]; !ok {
			return nil, plan{}, nil, fmt.Errorf("backfill %s: no job returns a key of dataset %s", day, k.DatasetID)
		}
	}
	p := plan{keys: all, job: jobs, hash: pDay.hash, held: pDay.held}
	if err := r.putMissingWindows(ctx, day, p, sess); err != nil {
		return nil, plan{}, nil, err
	}
	if len(added) > 0 {
		if err := r.ledger.PutExpected(ctx, day, pDay.hash, all); err != nil {
			return nil, plan{}, nil, err
		}
	}
	return keys, p, sess, nil
}

// sessionsOf is the sessions of day: an empty map when it is a session of no calendar.
func (r *reconciler) sessionsOf(ctx context.Context, day contracts.Day) (daySessions, error) {
	t, err := time.Parse("2006-01-02", string(day))
	if err != nil {
		return nil, fmt.Errorf("backfill %s: %w", day, err)
	}
	from := contracts.Day(t.AddDate(0, 0, -1).Format("2006-01-02"))
	_, byDay, err := r.walk(ctx, from, day)
	if err != nil {
		return nil, err
	}
	if s := byDay[day]; s != nil {
		return s, nil
	}
	return daySessions{}, nil
}

// addedKeys are the keys of the plan of the day that the stored plan lacks and whose dataset has a
// window on the day.
func (r *reconciler) addedKeys(stored []contracts.ExpectedKey, pDay plan, sess daySessions) []contracts.ExpectedKey {
	have := map[contracts.ExpectedKey]bool{}
	for _, k := range stored {
		have[k] = true
	}
	var added []contracts.ExpectedKey
	for _, k := range pDay.keys {
		if have[k] {
			continue
		}
		j := pDay.job[k]
		if _, ok := windowFor(ruleOf(j, r.rule), k.DatasetID, sess[r.jobCalendar(j)]); ok {
			added = append(added, k)
		}
	}
	return added
}

// putMissingWindows puts the window of every dataset of p that has none stored and has a window
// rule on the day.
func (r *reconciler) putMissingWindows(ctx context.Context, day contracts.Day, p plan, sess daySessions) error {
	seen := map[string]bool{}
	var need []contracts.ExpectedKey
	for _, k := range p.keys {
		if seen[k.DatasetID] {
			continue
		}
		seen[k.DatasetID] = true
		closeAt, err := r.closeOf(ctx, k.DatasetID, day)
		if err != nil {
			return err
		}
		if closeAt.IsZero() {
			need = append(need, k)
		}
	}
	_, err := r.putWindows(ctx, plan{keys: need, job: p.job}, sess)
	return err
}

// errNoKey is the error of selectKeys for a dataset with no key on the day; BackfillWith answers it as
// runner.RefusedNoKey.
var errNoKey = errors.New("no key on the day")

// selectKeys filters the stored keys by dataset ids; an id with no stored key is an error matching errNoKey.
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
			return nil, fmt.Errorf("backfill %s: dataset %s has %w", day, id, errNoKey)
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

// backfillKey runs k once through RunKey, which must end by deadline, and reports the new run row and the
// mark afterwards: StatusPresent (filled), StatusNoValue (absent) or "" (missing: no usable answer, a job
// error leaves the key unmarked, or no_value as it was, told apart by the ErrorCode of the new run row).
func (r *reconciler) backfillKey(
	ctx context.Context, job contracts.Job, day contracts.Day, k contracts.ExpectedKey, seq int, closeAt, deadline time.Time, reason string,
) (store.Run, store.PresentStatus, error) {
	rec, err := r.kr.RunKey(ctx, job, day, k, seq, closeAt, deadline, reason)
	if err != nil {
		return rec, "", err
	}
	present, err := r.ledger.Present(ctx, day)
	if err != nil {
		return rec, "", err
	}
	status := present[k]
	if status == store.StatusNoValue && rec.ErrorCode != "" {
		return rec, "", nil
	}
	return rec, status, nil
}
