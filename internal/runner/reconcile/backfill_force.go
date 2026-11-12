package reconcile

import (
	"context"
	"errors"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
)

var _ runner.ForceBackfiller = (*reconciler)(nil)

const (
	// reasonSlotNear is the reason of a key not reached because a slot start lies inside the next key's bound.
	reasonSlotNear = "slot_near"
	// reasonStopped is the reason of a key not reached because the call was cancelled or failed.
	reasonStopped = "stopped"
	// reasonBareCut is the reason of a bare run cut at the job's run bound.
	reasonBareCut = "cut_at_bound"
	// reasonNoValue is the reason of a bare run whose job answered a NoValue error.
	reasonNoValue = "no_value"
	// holdMargin is added to a key's run bound when the hold is asked: the Finish timeout (30 s) and the
	// SyncKeys timeout (2 min) that follow the request.
	holdMargin = 30*time.Second + 2*time.Minute
)

// BackfillWith is Backfill for the console (Decisions 4, 5, 7 and 8 of docs/design/job-console.md): it goes over
// the selected keys, those of opt.Job when set, in order. Per key: a present key without Force is
// already_present; a key held before its slot is not_due (with DueAt); a present key with Force is re-run
// bare (no run row, no mark, under the job's run bound); a key with a live run is lease_held; any other key
// takes the ordinary run once under its own run bound. Before every key after the first the context and
// opt.Hold are checked; the keys left are not_reached. A dataset with no key on the day is RefusedNoKey.
func (r *reconciler) BackfillWith(
	ctx context.Context, day contracts.Day, datasets []string, opt runner.BackfillOptions,
) (res runner.BackfillResult, err error) {
	res = runner.BackfillResult{Day: day}
	// The lease context is cancelled only by a failed renewal: the turn's own cancel reaches a key
	// through the bridge below, which tags it errConsoleStop.
	leaseCtx, release, err := r.holdLease(context.WithoutCancel(ctx), backfillOwner())
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
	keys, p, sess, err := r.backfillPlan(ctx, day, stored, datasets)
	if errors.Is(err, errNoKey) {
		res.Refused = runner.RefusedNoKey
		return res, nil
	}
	if err != nil {
		return res, err
	}
	if opt.Job != "" {
		keys = keysOfJob(keys, p, opt.Job)
	}
	present, byKey, err := r.dayState(ctx, day)
	if err != nil {
		return res, err
	}

	for i, k := range keys {
		if stop, why := r.mustStop(ctx, leaseCtx, i, p.job[k], opt); stop {
			r.notReached(&res, keys[i:], present, why)
			if why == reasonStopped {
				err = ctx.Err()
			}
			return res, err
		}
		line, err := r.backfillOne(ctx, leaseCtx, p, sess, day, k, byKey[k], present, opt)
		if line.Group != "" {
			res.Keys = append(res.Keys, line)
		}
		if err != nil {
			r.notReached(&res, keys[i+1:], present, reasonStopped)
			return res, err
		}
		fillLists(&res, k, line)
	}
	return res, nil
}

// keysOfJob keeps the keys whose job has the id.
func keysOfJob(keys []contracts.ExpectedKey, p plan, id string) []contracts.ExpectedKey {
	var out []contracts.ExpectedKey
	for _, k := range keys {
		if p.job[k].ID() == id {
			out = append(out, k)
		}
	}
	return out
}

// mustStop reports whether the call ends before key number i: the turn's context is done, the lease was
// lost, or (after the first key) opt.Hold says a slot start lies inside the key's bound.
func (r *reconciler) mustStop(ctx, leaseCtx context.Context, i int, job contracts.Job, opt runner.BackfillOptions) (bool, string) {
	if ctx.Err() != nil || leaseCtx.Err() != nil {
		return true, reasonStopped
	}
	if i > 0 && opt.Hold != nil {
		if _, _, held := opt.Hold(holdBound(job)); held {
			return true, reasonSlotNear
		}
	}
	return false, ""
}

// notReached adds a line for each key the call did not reach.
func (r *reconciler) notReached(res *runner.BackfillResult, keys []contracts.ExpectedKey, present map[contracts.ExpectedKey]store.PresentStatus, why string) {
	for _, k := range keys {
		res.Keys = append(res.Keys, runner.KeyRun{Key: k, Group: runner.GroupNotReached, ReasonCode: why, Mark: string(present[k])})
	}
}

// fillLists keeps the six lists of Backfill for the line of k.
func fillLists(res *runner.BackfillResult, k contracts.ExpectedKey, line runner.KeyRun) {
	switch line.Group {
	case runner.GroupAlreadyPresent, runner.GroupRerun:
		res.AlreadyPresent = append(res.AlreadyPresent, k)
	case runner.GroupNotDue:
		res.InWindow = append(res.InWindow, k)
	case runner.GroupRan:
		switch store.PresentStatus(line.Mark) {
		case store.StatusPresent:
			res.Filled = append(res.Filled, k)
		case store.StatusNoValue:
			res.Absent = append(res.Absent, k)
		default:
			res.Missing = append(res.Missing, k)
		}
	default:
		res.Missing = append(res.Missing, k)
	}
}

// backfillOne handles one key in the order of Decision 4 and returns its line.
func (r *reconciler) backfillOne(
	ctx, leaseCtx context.Context, p plan, sess daySessions, day contracts.Day, k contracts.ExpectedKey,
	runs []store.Run, present map[contracts.ExpectedKey]store.PresentStatus, opt runner.BackfillOptions,
) (runner.KeyRun, error) {
	job := p.job[k]
	mark := string(present[k])
	line := runner.KeyRun{Key: k, Mark: mark}
	isPresent := present[k] == store.StatusPresent
	switch {
	case isPresent && !opt.Force:
		line.Group = runner.GroupAlreadyPresent
		return line, nil
	case p.held[k]:
		line.Group = runner.GroupNotDue
		if at, ok, err := slotAt(job, sess, r.rule); err != nil {
			return line, err
		} else if ok {
			line.DueAt = at
		}
		return line, nil
	}

	kctx, cancel := context.WithCancelCause(leaseCtx)
	defer cancel(nil)
	stopBridge := context.AfterFunc(ctx, func() { cancel(errConsoleStop) })
	defer stopBridge()
	kctx = context.WithValue(kctx, consoleKey{}, true)

	if isPresent {
		line.Group = runner.GroupRerun
		r.bareRun(kctx, job, day, k, &line)
		return line, nil
	}
	closeAt, err := r.closeOf(ctx, k.DatasetID, day)
	if err != nil {
		return line, err
	}
	live, err := r.closeExpired(ctx, runs, r.clk.Now())
	if err != nil {
		return line, err
	}
	if live {
		line.Group = runner.GroupLeaseHeld
		return line, nil
	}
	rec, status, err := r.runMissingKey(kctx, job, day, k, len(runs)+1, closeAt)
	line.Group = runner.GroupRan
	line.Mark = string(status)
	line.Status, line.ErrorCode, line.ReasonCode = string(rec.Status), rec.ErrorCode, rec.ReasonCode
	line.Returned, line.Usable, line.Attempt = rec.ReturnedN, rec.UsableN, len(runs)+1
	return line, err
}

// bareRun re-runs a present key through job.Run under a timeout of the job's run bound and writes
// nothing: no run row, no mark. The line takes the counts, the kind of the job's error (a NoValue error is
// the reason no_value) and, for a cut at the bound, the reason cut_at_bound.
func (r *reconciler) bareRun(ctx context.Context, job contracts.Job, day contracts.Day, k contracts.ExpectedKey, line *runner.KeyRun) {
	bctx, cancel := context.WithCancel(ctx)
	defer cancel()
	cut := r.fclk.AfterFunc(runBound(job), cancel)
	defer cut.Stop()
	out := job.Run(bctx, day, []contracts.ExpectedKey{k})
	line.Returned, line.Usable = out.Returned, out.Usable
	switch {
	case ctx.Err() != nil:
		line.ReasonCode = reasonStopped
	case bctx.Err() != nil:
		line.ReasonCode = reasonBareCut
	case out.Err != nil && out.Err.Kind() == contracts.NoValue:
		line.ReasonCode = reasonNoValue
	case out.Err != nil:
		line.ErrorCode = string(out.Err.Kind())
	}
}

// runMissingKey is the ordinary run of a missing key: one call of backfillKey (it starts the run with
// reasonBackfill, as Backfill does) whose
// deadline is now plus the job's own run bound, never the scheduler's deadline.
func (r *reconciler) runMissingKey(
	ctx context.Context, job contracts.Job, day contracts.Day, k contracts.ExpectedKey, seq int, closeAt time.Time,
) (store.Run, store.PresentStatus, error) {
	return r.backfillKey(ctx, job, day, k, seq, closeAt, r.clk.Now().Add(runBound(job)))
}
