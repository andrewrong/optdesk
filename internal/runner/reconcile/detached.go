package reconcile

import (
	"context"
	"log/slog"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
)

var _ runner.DetachedReconciler = (*reconciler)(nil)

// ReconcileDetached walks the days after the cursor as Reconcile does, plans each day, and hands every
// due job whose lock it won to spawn, then returns without waiting for any run. ops.run_lease is held
// only while it plans and launches; a run holds none. A job whose lock is held is not planned: its keys
// are in a run's hand, so no key of it is due, and the day it belongs to is not done. A day is done, and
// the cursor set, only when every expected key is marked present and no job was launched or held, so a
// pass that launched runs sets no cursor; a later pass that finds the marks does.
func (r *reconciler) ReconcileDetached(ctx context.Context, day contracts.Day, spawn runner.Spawn) (err error) {
	ctx, release, err := r.holdLease(ctx, owner)
	if err != nil {
		return err
	}
	defer release(&err)

	cur, err := r.ledger.Cursor(ctx)
	if err != nil {
		return err
	}
	days, sessionsByDay, err := r.walk(ctx, cur.Day, day)
	if err != nil {
		return err
	}
	prefix := true
	for _, d := range days {
		done, err := r.launchDay(ctx, d, sessionsByDay[d], spawn)
		if err != nil {
			return err
		}
		prefix = prefix && done
		if !prefix {
			continue
		}
		if err := r.ledger.SetCursor(ctx, store.Cursor{Day: d}); err != nil {
			return err
		}
	}
	return nil
}

// launchDay plans day without the jobs whose lock is held, starts the due ones and reports whether
// the day is already done: nothing held, nothing due, no held-back key missing.
func (r *reconciler) launchDay(ctx context.Context, day contracts.Day, sess daySessions, spawn runner.Spawn) (bool, error) {
	free, held := r.freeJobs()
	pr := *r
	pr.jobs = free
	p, err := pr.plan(ctx, day, sess)
	if err != nil {
		return false, err
	}
	// The expected keys of the day are written only by a pass that saw every job: a plan without the
	// held jobs would drop their keys.
	if held == 0 {
		if _, old, err := r.ledger.Expected(ctx, day); err != nil {
			return false, err
		} else if old != p.hash {
			if err := r.ledger.PutExpected(ctx, day, p.hash, p.keys); err != nil {
				return false, err
			}
		}
	}
	closes, err := r.putWindows(ctx, p, sess)
	if err != nil {
		return false, err
	}
	present, err := r.ledger.Present(ctx, day)
	if err != nil {
		return false, err
	}
	due, complete := dueJobsOf(p, present)
	dc := newDayCtx(day, p, sess, false)
	for _, j := range due.order {
		dc.setPending(j.ID())
	}
	for _, j := range due.order {
		unlock, ok := r.locks.TryLock(j.ID())
		if !ok {
			dc.finish(j.ID(), true)
			continue
		}
		keys := due.keys[j.ID()]
		spawn(j.ID(), func(runCtx context.Context) { r.detachedRun(runCtx, dc, j, keys, closes, unlock) })
	}
	return complete && held == 0 && len(due.order) == 0, nil
}

// freeJobs are the jobs whose lock is not held, and how many are. A lock set that cannot tell
// (no runner.JobLockHolder) holds back nothing.
func (r *reconciler) freeJobs() (free []contracts.Job, held int) {
	h, ok := r.locks.(runner.JobLockHolder)
	if !ok {
		return r.jobs, 0
	}
	for _, j := range r.jobs {
		if _, locked := h.Holder(j.ID()); locked {
			held++
			continue
		}
		free = append(free, j)
	}
	return free, held
}

// detachedRun is one spawned run: the keys of job in order under the lock the pass won, then its
// dependents as reconcileJob triggers them. Every key is marked present or closed before the lock is
// released: unlock is the last deferred call, run by lockedTurn's own defer after the last mark.
func (r *reconciler) detachedRun(
	ctx context.Context, dc *dayCtx, job contracts.Job, keys []contracts.ExpectedKey, closes map[string]time.Time, unlock func(),
) {
	id := job.ID()
	t, end := r.lockedTurn(ctx, dc, job, keys, closes, unlock)
	dc.finish(id, false)
	if t.err != nil {
		r.log.Warn("detached run failed", slog.String("job", id), slog.String("day", string(dc.day)))
		return
	}
	if !t.skipped && !end.cut && ctx.Err() == nil {
		r.trigger(ctx, dc, id, !end.failed && t.complete)
	}
	if t.skipped && ctx.Err() == nil {
		if c, ok := dc.take(id); ok {
			r.runDependent(ctx, dc, job, c.up, c.ok)
		}
	}
	// Dependents it started run on in their own goroutines under their own locks; nothing waits for them,
	// and the next pass finds their marks in the ledger.
}
