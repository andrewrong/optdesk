package reconcile

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
)

const (
	// reasonUpstreamMissing marks a run of a dependent whose upstream was not ok for the day when it started
	// (Decision 6 of docs/design/console-run-lock.md). A run that ended ok may carry it.
	reasonUpstreamMissing = "upstream_missing"
	// reasonJobRunning is the ReasonCode of the lease_held line of a job that another run holds.
	reasonJobRunning = "job_running"
	// maxSessionLookback is how many days back the session of an upstream in another calendar is looked for.
	maxSessionLookback = 10
)

// errJobBusy is returned by runMissingKey when the job's try-lock is held.
var errJobBusy = errors.New("job running")

// dayCtx is the state of one day of one call that starts dependents: the plan and sessions of the day,
// the jobs due in the call that have not ended (Reconcile only), and the dependents started by a trigger.
type dayCtx struct {
	day  contracts.Day
	p    plan
	sess daySessions
	// console is set for a call of BackfillWith: a triggered run is bounded by the job's own run bound.
	console bool

	wg sync.WaitGroup // the goroutines of the dependents started by a trigger

	mu      sync.Mutex
	pending map[string]bool // jobs due in this call whose turn has not ended
	busy    map[string]bool // jobs whose turn found them locked by another run
	err     error
}

func newDayCtx(day contracts.Day, p plan, sess daySessions, console bool) *dayCtx {
	return &dayCtx{day: day, p: p, sess: sess, console: console, pending: map[string]bool{}, busy: map[string]bool{}}
}

func (dc *dayCtx) setPending(id string) {
	dc.mu.Lock()
	dc.pending[id] = true
	dc.mu.Unlock()
}

// finish records that the turn of job id ended; busy tells that it did not run because the job was locked.
func (dc *dayCtx) finish(id string, busy bool) {
	dc.mu.Lock()
	delete(dc.pending, id)
	if busy {
		dc.busy[id] = true
	}
	dc.mu.Unlock()
}

func (dc *dayCtx) running(id string) bool {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	return dc.pending[id] || dc.busy[id]
}

func (dc *dayCtx) fail(err error) {
	dc.mu.Lock()
	if dc.err == nil {
		dc.err = err
	}
	dc.mu.Unlock()
}

func (dc *dayCtx) firstErr() error {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	return dc.err
}

// keysOf are the keys of the plan of the day that job id plans.
func (dc *dayCtx) keysOf(id string) []contracts.ExpectedKey {
	var out []contracts.ExpectedKey
	for _, k := range dc.p.keys {
		if j := dc.p.job[k]; j != nil && j.ID() == id {
			out = append(out, k)
		}
	}
	return out
}

// jobEnd is how the runs of one job in one call ended.
type jobEnd struct{ ran, cut, failed bool }

func (e *jobEnd) add(rec store.Run) {
	e.ran = true
	switch {
	case rec.Status != store.RunDone:
		e.cut = true
	case rec.ErrorCode != "":
		e.failed = true
	}
}

// ended reports whether the job's runs all came to an end, ok or failed (a cut run is no end).
func (e jobEnd) ended() bool { return e.ran && !e.cut }

type upState int

const (
	upOK      upState = iota // every key of the upstream is marked for the day
	upMissing                // failed, ended without ok, or not run
	upRunning                // a run of the upstream is going on
)

func (r *reconciler) jobByID(id string) contracts.Job {
	for _, j := range r.jobs {
		if j.ID() == id {
			return j
		}
	}
	return nil
}

// upstreamDay is the day of job in its own calendar that a dependent of day reads: day itself when the
// calendar has a session in it, else the latest session day before it. "" when there is none.
func (r *reconciler) upstreamDay(ctx context.Context, dc *dayCtx, job contracts.Job) contracts.Day {
	cal := r.jobCalendar(job)
	if _, ok := dc.sess[cal]; ok {
		return dc.day
	}
	t, err := time.Parse("2006-01-02", string(dc.day))
	if err != nil || r.cal == nil {
		return ""
	}
	for i := 1; i <= maxSessionLookback; i++ {
		d := contracts.Day(t.AddDate(0, 0, -i).Format("2006-01-02"))
		if s, err := r.cal.Session(ctx, cal, d); err == nil && s.IsSession {
			return d
		}
	}
	return ""
}

// upstreamState reads the state of upstream id for the day of dc. ends holds the upstreams whose run just
// ended in the triggering call, with their outcome, which wins over the ledger.
func (r *reconciler) upstreamState(ctx context.Context, dc *dayCtx, id string, ends map[string]bool) (upState, error) {
	if ok, has := ends[id]; has {
		if ok {
			return upOK, nil
		}
		return upMissing, nil
	}
	if dc.running(id) {
		return upRunning, nil
	}
	job := r.jobByID(id)
	if job == nil {
		return upOK, nil // an id no job answers to is not an order
	}
	d := r.upstreamDay(ctx, dc, job)
	if d == "" {
		return upMissing, nil
	}
	var keys []contracts.ExpectedKey
	if d == dc.day {
		keys = dc.keysOf(id)
	} else if ks, _, err := job.Plan(ctx, d); err == nil {
		keys = ks
	}
	if len(keys) == 0 {
		return upMissing, nil
	}
	present, byKey, err := r.dayState(ctx, d)
	if err != nil {
		return upMissing, err
	}
	now := r.clk.Now()
	all, live := true, false
	for _, k := range keys {
		if _, ok := present[k]; !ok {
			all = false
		}
		for _, run := range byKey[k] {
			if run.Status == store.RunRunning && !run.LeaseUntil.Before(now) {
				live = true
			}
		}
	}
	switch {
	case all:
		return upOK, nil
	case live:
		return upRunning, nil
	}
	return upMissing, nil
}

// gate decides how job starts on the day of dc: leave is true while an upstream is running or due in the
// call and not ended (no wait: the end of that upstream starts the job); mark is reasonUpstreamMissing when
// an upstream is not ok. A job with no upstream has neither.
func (r *reconciler) gate(ctx context.Context, dc *dayCtx, job contracts.Job, ends map[string]bool) (mark string, leave bool, err error) {
	dep, ok := job.(runner.Dependent)
	if !ok {
		return "", false, nil
	}
	for _, id := range dep.After() {
		st, err := r.upstreamState(ctx, dc, id, ends)
		if err != nil {
			return "", false, err
		}
		switch st {
		case upOK:
		case upRunning:
			leave = true
		case upMissing:
			mark = reasonUpstreamMissing
		}
	}
	return mark, leave, nil
}

// consoleMark is the mark of a console run of job: it is never held by the order and never starts an
// upstream, so an upstream that is not ok (running included) only marks the run.
func (r *reconciler) consoleMark(ctx context.Context, dc *dayCtx, job contracts.Job) (string, error) {
	mark, leave, err := r.gate(ctx, dc, job, nil)
	if leave {
		mark = reasonUpstreamMissing
	}
	return mark, err
}

// trigger starts, in goroutines of dc.wg under ctx, each dependent of the job up that has a session on
// the day, whose slot has passed and that has not run for the day. The caller waits for dc.wg.
func (r *reconciler) trigger(ctx context.Context, dc *dayCtx, up string, upOK bool) {
	for _, d := range r.jobs {
		dep, ok := d.(runner.Dependent)
		if !ok || ctx.Err() != nil || !slices.Contains(dep.After(), up) {
			continue
		}
		if _, has := dc.sess[r.jobCalendar(d)]; !has || len(dc.keysOf(d.ID())) == 0 {
			continue
		}
		if at, sched, err := slotAt(d, dc.sess, r.rule); err != nil || (sched && r.clk.Now().Before(at)) {
			continue // not yet its slot: left to its slot turn
		}
		dc.wg.Add(1)
		go func() {
			defer dc.wg.Done()
			r.runDependent(ctx, dc, d, up, upOK)
		}()
	}
}

// runDependent runs the keys of d after the end of its upstream up, under d's try-lock, unless d is
// running, already ran for the day, or another upstream is still running. Its own dependents follow.
func (r *reconciler) runDependent(ctx context.Context, dc *dayCtx, d contracts.Job, up string, upOK bool) {
	unlock, ok := r.locks.TryLock(d.ID())
	if !ok {
		return
	}
	defer unlock()
	present, byKey, err := r.dayState(ctx, dc.day)
	if err != nil {
		dc.fail(err)
		return
	}
	keys := dc.keysOf(d.ID())
	for _, k := range keys {
		if _, ok := present[k]; ok || len(byKey[k]) > 0 {
			return // it ran for the day
		}
	}
	mark, leave, err := r.gate(ctx, dc, d, map[string]bool{up: upOK})
	if err != nil {
		dc.fail(err)
		return
	}
	if leave {
		return
	}
	var end jobEnd
	for _, k := range keys {
		closeAt, err := r.closeOf(ctx, k.DatasetID, dc.day)
		if err != nil {
			dc.fail(err)
			return
		}
		now := r.clk.Now()
		reason, deadline := mark, deadlineFor(d, closeAt, now)
		if dc.console {
			deadline = now.Add(runBound(d))
		}
		if reason == "" {
			reason = r.plainReason(dc, closeAt, now)
		}
		rec, err := r.kr.RunKey(ctx, d, dc.day, k, len(byKey[k])+1, closeAt, deadline, reason)
		if err != nil {
			dc.fail(err)
			return
		}
		end.add(rec)
	}
	unlock()
	if end.ended() && ctx.Err() == nil {
		r.trigger(ctx, dc, d.ID(), !end.failed)
	}
}

// plainReason is the reason of a run with no mark: backfill for a console call, late after the close.
func (r *reconciler) plainReason(dc *dayCtx, closeAt, now time.Time) string {
	switch {
	case dc.console:
		return reasonBackfill
	case !closeAt.IsZero() && now.After(closeAt):
		return reasonLate
	}
	return ""
}

// jobTurn is the result of the keys of one job in one day. skipped: the job was locked or left to the
// end of its upstream, so a key of it may still have been run by a trigger.
type jobTurn struct {
	complete, skipped bool
	err               error
}

// reconcileJob runs the keys of job under its try-lock: a busy job is skipped and its keys stay missing
// for the next tick. Before the first key that starts it asks the order of its upstreams. When the turn
// ends the job's dependents are triggered.
func (r *reconciler) reconcileJob(ctx context.Context, dc *dayCtx, job contracts.Job, keys []contracts.ExpectedKey, closes map[string]time.Time) jobTurn {
	id := job.ID()
	unlock, ok := r.locks.TryLock(id)
	if !ok {
		dc.finish(id, true)
		return jobTurn{skipped: true}
	}
	defer unlock()
	t, end := r.reconcileKeys(ctx, dc, job, keys, closes)
	unlock()
	dc.finish(id, false)
	// A turn that ended with no run (a key the brake stops, a key already counted) is an end without ok too:
	// the dependents left to it start marked. A turn left to its own upstream, or cut, is no end.
	if t.err == nil && !t.skipped && !end.cut && ctx.Err() == nil {
		r.trigger(ctx, dc, id, !end.failed && t.complete)
	}
	return t
}

func (r *reconciler) reconcileKeys(
	ctx context.Context, dc *dayCtx, job contracts.Job, keys []contracts.ExpectedKey, closes map[string]time.Time,
) (jobTurn, jobEnd) {
	t := jobTurn{complete: true}
	var end jobEnd
	fail := func(err error) (jobTurn, jobEnd) { return jobTurn{err: err}, end }
	present, byKey, err := r.dayState(ctx, dc.day)
	if err != nil {
		return fail(err)
	}
	var (
		gated, leave bool
		mark         string
	)
	for _, k := range keys {
		if _, ok := present[k]; ok {
			continue
		}
		closeAt := closes[k.DatasetID]
		if expired, err := r.expireIfClosed(ctx, byKey[k], closeAt, dc.day, k); err != nil {
			return fail(err)
		} else if expired {
			continue
		}
		crashed, start, reason := r.nextAttempt(byKey[k], closeAt)
		// A lapsed run is closed as crashed even when the brake stops the key.
		if crashed != nil {
			if err := r.markCrashed(ctx, *crashed); err != nil {
				return fail(err)
			}
		}
		if !start {
			t.complete = false
			continue
		}
		if !gated {
			gated = true
			if mark, leave, err = r.gate(ctx, dc, job, nil); err != nil {
				return fail(err)
			}
		}
		if leave {
			t.complete, t.skipped = false, true
			continue
		}
		if mark != "" {
			reason = mark
		}
		rec, ok, err := r.run(ctx, job, dc.day, k, len(byKey[k])+1, closeAt, reason)
		if err != nil {
			return fail(err)
		}
		end.add(rec)
		t.complete = t.complete && ok
	}
	return t, end
}
