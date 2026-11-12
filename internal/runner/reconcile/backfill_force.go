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

// BackfillWith is Backfill for the console (Decisions 4, 5, 7 and 8 of docs/design/job-console.md, Decisions 1, 2
// and 6 of docs/design/console-run-lock.md): it goes over the selected keys, those of opt.Job when set, in
// order. It takes no run lease: a run is refused only by a run of the same job. Per key: a present key without
// Force is already_present; a key held before its slot is not_due (with DueAt); a present key with Force is
// re-run bare (no run row, no mark, under the job's run bound); a key with a live run, or whose job another run
// holds (reason job_running), is lease_held; any other key takes the ordinary run once under its own run
// bound. Before every key after the first the context and opt.Hold are checked; the keys left are
// not_reached. A dataset with no key on the day is RefusedNoKey. When the runs of a job have ended, the
// dependents of that job whose slot has passed and that have not run for the day are run and waited for.
func (r *reconciler) BackfillWith(
	ctx context.Context, day contracts.Day, datasets []string, opt runner.BackfillOptions,
) (res runner.BackfillResult, err error) {
	res = runner.BackfillResult{Day: day}
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

	dc := newDayCtx(day, p, sess, true)
	var order []string
	ends := map[string]*jobEnd{}
	for i, k := range keys {
		if stop, why := r.mustStop(ctx, i, opt, p.job[k]); stop {
			r.notReached(&res, keys[i:], present, why)
			if why == reasonStopped {
				err = ctx.Err()
			}
			return res, err
		}
		id := p.job[k].ID()
		line, end, err := r.backfillOne(ctx, dc, k, byKey[k], present, opt)
		if line.Group != "" {
			res.Keys = append(res.Keys, line)
		}
		if err != nil {
			r.notReached(&res, keys[i+1:], present, reasonStopped)
			return res, err
		}
		fillLists(&res, k, line)
		if end.ran {
			if ends[id] == nil {
				ends[id] = &jobEnd{}
				order = append(order, id)
			}
			ends[id].ran = true
			ends[id].cut = ends[id].cut || end.cut
			ends[id].failed = ends[id].failed || end.failed
		}
	}
	tctx, stop := consoleCtx(ctx)
	defer stop()
	for _, id := range order {
		if e := ends[id]; e.ended() && ctx.Err() == nil {
			r.trigger(tctx, dc, id, !e.failed)
		}
	}
	dc.wg.Wait()
	return res, dc.firstErr()
}

// consoleCtx is the context of a console run: it outlives nothing of ctx but its cancel, which it tags
// errConsoleStop, and it is marked as a console key. stop releases it.
func consoleCtx(ctx context.Context) (context.Context, func()) {
	kctx, cancel := context.WithCancelCause(context.WithoutCancel(ctx))
	stopBridge := context.AfterFunc(ctx, func() { cancel(errConsoleStop) })
	return context.WithValue(kctx, consoleKey{}, true), func() {
		stopBridge()
		cancel(nil)
	}
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

// mustStop reports whether the call ends before key number i: the turn's context is done, or (after the
// first key) opt.Hold says a slot start lies inside the key's bound.
func (r *reconciler) mustStop(ctx context.Context, i int, opt runner.BackfillOptions, job contracts.Job) (bool, string) {
	if ctx.Err() != nil {
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

// backfillOne handles one key in the order of Decision 4 and returns its line and how the run of its job
// ended. A job another run holds gives a lease_held line with the reason job_running and no run; a job with
// an upstream that is not ok runs anyway with the reason upstream_missing on its line and its row.
func (r *reconciler) backfillOne(
	ctx context.Context, dc *dayCtx, k contracts.ExpectedKey,
	runs []store.Run, present map[contracts.ExpectedKey]store.PresentStatus, opt runner.BackfillOptions,
) (runner.KeyRun, jobEnd, error) {
	job, day := dc.p.job[k], dc.day
	mark := string(present[k])
	line := runner.KeyRun{Key: k, Mark: mark}
	var end jobEnd
	isPresent := present[k] == store.StatusPresent
	switch {
	case isPresent && !opt.Force:
		line.Group = runner.GroupAlreadyPresent
		return line, end, nil
	}

	kctx, stop := consoleCtx(ctx)
	defer stop()
	busy := func() (runner.KeyRun, jobEnd, error) {
		return runner.KeyRun{Key: k, Mark: mark, Group: runner.GroupLeaseHeld, ReasonCode: reasonJobRunning}, jobEnd{}, nil
	}

	upMark, err := r.consoleMark(ctx, dc, job)
	if err != nil {
		return line, end, err
	}
	if isPresent {
		line.Group = runner.GroupRerun
		if !r.bareRun(kctx, job, day, k, &line) {
			return busy()
		}
		switch line.ReasonCode {
		case reasonStopped, reasonBareCut:
			end = jobEnd{ran: true, cut: true}
		default:
			end = jobEnd{ran: true, failed: line.ErrorCode != ""}
		}
		if upMark != "" {
			line.ReasonCode = upMark
		}
		return line, end, nil
	}
	closeAt, err := r.closeOf(ctx, k.DatasetID, day)
	if err != nil {
		return line, end, err
	}
	live, err := r.closeExpired(ctx, runs, r.clk.Now())
	if err != nil {
		return line, end, err
	}
	if live {
		line.Group = runner.GroupLeaseHeld
		return line, end, nil
	}
	reason := reasonBackfill
	if upMark != "" {
		reason = upMark
	}
	rec, status, err := r.runMissingKey(kctx, job, day, k, len(runs)+1, closeAt, reason)
	if errors.Is(err, errJobBusy) {
		return busy()
	}
	line.Group = runner.GroupRan
	line.Mark = string(status)
	line.Status, line.ErrorCode, line.ReasonCode = string(rec.Status), rec.ErrorCode, rec.ReasonCode
	line.Returned, line.Usable, line.Attempt = rec.ReturnedN, rec.UsableN, len(runs)+1
	if err == nil {
		end.add(rec)
	}
	return line, end, err
}

// bareRun re-runs a present key through job.Run under the job's try-lock and a timeout of the job's run
// bound and writes nothing: no run row, no mark. The line takes the counts, the kind of the job's error (a
// NoValue error is the reason no_value) and, for a cut at the bound, the reason cut_at_bound. It reports
// false, having run nothing, when the job is locked.
func (r *reconciler) bareRun(ctx context.Context, job contracts.Job, day contracts.Day, k contracts.ExpectedKey, line *runner.KeyRun) bool {
	unlock, ok := r.locks.TryLock(job.ID())
	if !ok {
		return false
	}
	defer unlock()
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
	return true
}

// runMissingKey is the ordinary run of a missing key under the job's try-lock (errJobBusy when it is
// locked): one call of backfillKey with reason (reasonBackfill, or reasonUpstreamMissing) whose
// deadline is now plus the job's own run bound, never the scheduler's deadline.
func (r *reconciler) runMissingKey(
	ctx context.Context, job contracts.Job, day contracts.Day, k contracts.ExpectedKey, seq int, closeAt time.Time, reason string,
) (store.Run, store.PresentStatus, error) {
	unlock, ok := r.locks.TryLock(job.ID())
	if !ok {
		return store.Run{}, "", errJobBusy
	}
	defer unlock()
	return r.backfillKey(ctx, job, day, k, seq, closeAt, r.clk.Now().Add(runBound(job)), reason)
}
