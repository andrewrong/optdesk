// Package reconcile is the runner step: it materialises dataset windows and
// reconciles expected keys against present keys through store.OpsLedger.
// Keys are opaque; dataset content is never read.
package reconcile

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/runner/shared"
	"github.com/andrewrong/optdesk/internal/store"
)

const (
	owner = "runner"
	// backfillOwnerPrefix starts the lease owner of a Backfill call.
	backfillOwnerPrefix = "backfill:"
	leaseTTL            = 10 * time.Minute

	reasonCrashed = "crashed"
	reasonLate    = "late"
	// reasonEmptyBeforeClose marks a finished run that came back empty before the window
	// close: the key stays missing and the run does not count toward the two-run cap.
	reasonEmptyBeforeClose = "empty_before_close"
	// The reason "expired" explains a key whose last run ended retryable and whose window has closed.
	reasonExpired = "expired"
	// reasonCrashedRepeatedly is written at the close on the latest of three crashed or abandoned_stop runs.
	reasonCrashedRepeatedly = "crashed_repeatedly"
)

type reconciler struct {
	ledger store.OpsLedger
	cal    clock.Calendar
	clk    clock.Clock
	jobs   []contracts.Job
	rule   runner.WindowRule
	kr     runner.KeyRunner
	// sched renews the run lease on fclk while a call runs; nil (the commands) renews nothing.
	sched store.SchedulerLedger
	fclk  clockwork.Clock
	// known is the set of the calendars of the jobs, other than the rule's, that answered Session
	// when the reconciler was built; a calendar of it that later runs out is logged and loses its
	// jobs, any other calendar's error is returned.
	known map[string]bool
	log   *slog.Logger
}

// New builds the reconciler with the default window rule (DefaultCalendarID, no offsets).
func New(l store.OpsLedger, cal clock.Calendar, clk clock.Clock, jobs []contracts.Job) runner.Reconciler {
	return NewWithRule(l, cal, clk, jobs, runner.WindowRule{})
}

// NewWithRule builds the reconciler with an explicit window rule.
func NewWithRule(l store.OpsLedger, cal clock.Calendar, clk clock.Clock, jobs []contracts.Job, rule runner.WindowRule) runner.Reconciler {
	return newReconciler(l, nil, cal, clk, jobs, rule, nil)
}

// NewReconciler builds the reconciler and backfiller pair of the scheduler. Both renew
// ops.run_lease through sched every 30 s while they run and release it by defer when
// cancelled; kr runs every key (nil builds NewKeyRunner).
func NewReconciler(
	l store.OpsLedger, sched store.SchedulerLedger, cal clock.Calendar, clk clockwork.Clock, jobs []contracts.Job, rule runner.WindowRule, kr runner.KeyRunner,
) (runner.Reconciler, runner.Backfiller) {
	r := newReconciler(l, sched, cal, clk, jobs, rule, kr)
	return r, r
}

func newReconciler(
	l store.OpsLedger, sched store.SchedulerLedger, cal clock.Calendar, clk clock.Clock, jobs []contracts.Job, rule runner.WindowRule, kr runner.KeyRunner,
) *reconciler {
	fclk := withClockwork(clk)
	if kr == nil {
		kr = NewKeyRunner(l, sched, fclk)
	}
	r := &reconciler{ledger: l, cal: cal, clk: clk, jobs: jobs, rule: rule, kr: kr, sched: sched, fclk: fclk, log: slog.Default()}
	r.known = r.probeKnown()
	return r
}

// Reconcile handles every session day after the cursor up to day, in order. A day that is
// not fully reconciled does not stop the walk: the cursor holds on it, and the days after
// it are still reconciled. The cursor is set to a day only when that day and every day
// before it are done, so every day at or before the cursor is complete.
func (r *reconciler) Reconcile(ctx context.Context, day contracts.Day) (err error) {
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
	prefix := true // every day walked so far is done
	for _, d := range days {
		done, err := r.reconcileOn(ctx, d, sessionsByDay[d])
		if err != nil {
			return err
		}
		if !done {
			prefix = false
		}
		if !prefix {
			continue
		}
		if err := r.ledger.SetCursor(ctx, store.Cursor{Day: d}); err != nil {
			return err
		}
	}
	return nil
}

// plan is the merged plan of all jobs for a day.
type plan struct {
	keys []contracts.ExpectedKey
	job  map[contracts.ExpectedKey]contracts.Job
	hash string
	// held is the set of expected keys whose job is Scheduled and not yet due: the clock is before
	// the job's slot instant on the day. It is set only when plan has the sessions of the day.
	held map[contracts.ExpectedKey]bool
}

// plan merges the plans of the jobs for day. With sess, a job is planned only when its own calendar
// has a session in it; with nil sess every job is planned. A held job is planned like any other, so
// its keys, job and hash are in the plan; only held records that it is not yet due.
func (r *reconciler) plan(ctx context.Context, day contracts.Day, sess daySessions) (plan, error) {
	return r.planLogged(ctx, day, sess, r.log)
}

// planLogged is plan with the logger for the jobs it skips. A job whose Plan returns an error is
// skipped for this pass: its keys, job and hash entry are left out and the other jobs are planned.
// The log line has the job id, the day and the kind of the error, never its text, which may hold a
// secret.
func (r *reconciler) planLogged(ctx context.Context, day contracts.Day, sess daySessions, lg *slog.Logger) (plan, error) {
	p := plan{job: map[contracts.ExpectedKey]contracts.Job{}, held: map[contracts.ExpectedKey]bool{}}
	h := sha256.New()
	for _, j := range r.jobs {
		if _, ok := sess[r.jobCalendar(j)]; sess != nil && !ok {
			continue
		}
		keys, hash, err := j.Plan(ctx, day)
		if err != nil {
			kind := "error"
			var te contracts.TypedError
			if errors.As(err, &te) {
				kind = string(te.Kind())
			}
			lg.Warn("plan failed, job skipped", slog.String("job", j.ID()), slog.String("day", string(day)), slog.String("kind", kind))
			continue
		}
		fmt.Fprintf(h, "%s=%s;", j.ID(), hash)
		notYet, err := r.notDue(j, sess)
		if err != nil {
			return plan{}, err
		}
		for _, k := range keys {
			if _, dup := p.job[k]; !dup {
				p.job[k] = j
				p.keys = append(p.keys, k)
				if notYet {
					p.held[k] = true
				}
			}
		}
	}
	sort.Slice(p.keys, func(a, b int) bool {
		if p.keys[a].DatasetID != p.keys[b].DatasetID {
			return p.keys[a].DatasetID < p.keys[b].DatasetID
		}
		return p.keys[a].SubjectKey < p.keys[b].SubjectKey
	})
	p.hash = hex.EncodeToString(h.Sum(nil))
	return p, nil
}

// notDue reports whether job is Scheduled and the clock is before its slot instant on the day of
// sess. A job that is not Scheduled, a nil sess, or a job whose calendar has no session is never held.
func (r *reconciler) notDue(job contracts.Job, sess daySessions) (bool, error) {
	sc, ok := job.(runner.Scheduled)
	if !ok || sess == nil {
		return false, nil
	}
	cal, anchor, zone, offset := sc.Schedule()
	if cal == "" {
		cal = r.jobCalendar(job)
	}
	s, ok := sess[cal]
	if !ok {
		return false, nil
	}
	at, err := shared.At(runner.Slot{JobID: job.ID(), CalendarID: cal, Anchor: anchor, Zone: zone, Offset: offset}, s)
	if err != nil {
		return false, err
	}
	return r.clk.Now().Before(at), nil
}

// reconcileDay plans the day, puts its windows, starts the repair and crash re-runs
// and reports whether every expected key is present or no_value.
func (r *reconciler) reconcileDay(ctx context.Context, s clock.Session) (bool, error) {
	return r.reconcileOn(ctx, s.Day, daySessions{calendarIDOf(r.rule): s})
}

// reconcileOn is reconcileDay for a day that may be a session of several calendars: sess holds the
// session of each calendar that has one.
func (r *reconciler) reconcileOn(ctx context.Context, day contracts.Day, sess daySessions) (bool, error) {
	p, err := r.plan(ctx, day, sess)
	if err != nil {
		return false, err
	}
	if _, old, err := r.ledger.Expected(ctx, day); err != nil {
		return false, err
	} else if old != p.hash {
		if err := r.ledger.PutExpected(ctx, day, p.hash, p.keys); err != nil {
			return false, err
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
	runs, err := r.ledger.Runs(ctx, day)
	if err != nil {
		return false, err
	}
	byKey := map[contracts.ExpectedKey][]store.Run{}
	for _, run := range runs {
		byKey[run.Key] = append(byKey[run.Key], run)
	}

	complete := true
	for _, k := range p.keys {
		if _, ok := present[k]; ok {
			continue
		}
		if p.held[k] {
			complete = false
			continue
		}
		closeAt := closes[k.DatasetID]
		if expired, err := r.expireIfClosed(ctx, byKey[k], closeAt, day, k); err != nil {
			return false, err
		} else if expired {
			continue
		}
		crashed, start, reason := r.nextAttempt(byKey[k], closeAt)
		// A lapsed run is closed as crashed even when the brake stops the key.
		if crashed != nil {
			if err := r.markCrashed(ctx, *crashed); err != nil {
				return false, err
			}
		}
		if !start {
			complete = false
			continue
		}
		_, ok, err := r.run(ctx, p.job[k], day, k, len(byKey[k])+1, closeAt, reason)
		if err != nil {
			return false, err
		}
		complete = complete && ok
	}
	return complete, nil
}

// putWindows puts the window of every dataset of the plan, from the session of the calendar of the job
// that planned it, and returns the close of each.
func (r *reconciler) putWindows(ctx context.Context, p plan, sess daySessions) (map[string]time.Time, error) {
	closes := map[string]time.Time{}
	seen := map[string]bool{}
	for _, k := range p.keys {
		if seen[k.DatasetID] {
			continue
		}
		seen[k.DatasetID] = true
		w, ok := windowFor(ruleOf(p.job[k], r.rule), k.DatasetID, sess[r.jobCalendar(p.job[k])])
		if !ok {
			continue
		}
		closes[k.DatasetID] = w.CloseAt
		if err := r.ledger.PutWindow(ctx, w); err != nil {
			return nil, err
		}
	}
	return closes, nil
}

// crashBrake is the number of crashed or abandoned_stop runs of one key that stops it (Decision 7).
const crashBrake = 3

// isCrash reports whether run ended without a verdict: crashed (a lapsed lease) or abandoned_stop.
// Such a run does not count toward the two-run cap; three of them stop the key.
func isCrash(run store.Run) bool {
	return run.ReasonCode == reasonCrashed || run.ReasonCode == reasonAbandonedStop
}

// isRetryable reports whether run ended with a retryable error that is not a crash.
func isRetryable(run store.Run) bool {
	return run.Status == store.RunDone && run.ErrorCode == string(contracts.Retryable) && !isCrash(run)
}

// nextAttempt decides whether a run starts for a missing key (Decisions 7 and 8). A run counts
// when it is a verdict: partial, or done (a Permanent error, a false Postcondition, a retryable
// error with no window). Runs that are empty_before_close, retryable before the close, crashed
// or abandoned_stop do not count. A run starts when no run is running, fewer than two runs count,
// the key is not stopped by the brake (three crashed or abandoned_stop runs: crashed_repeatedly,
// written by expireIfClosed at the close) and either no run counts or the latest counted run is
// RunPartial: the second run is only for a partial. A running run with a live lease is pending:
// nothing starts. A lapsed one is returned as crashed (it does not count) and the key goes on by
// the same rule. closeAt is the close of the key's dataset window, zero when it has none.
func (r *reconciler) nextAttempt(runs []store.Run, closeAt time.Time) (crashed *store.Run, start bool, reason string) {
	now := r.clk.Now()
	if !closeAt.IsZero() && now.After(closeAt) {
		reason = reasonLate
	}
	beforeClose := !closeAt.IsZero() && now.Before(closeAt)
	crashes, counted := 0, 0
	var latest *store.Run
	for i, run := range runs {
		switch {
		case run.Status == store.RunRunning:
			if !run.LeaseUntil.Before(now) {
				return nil, false, ""
			}
			if crashed == nil {
				crashed = &runs[i]
			}
		case isCrash(run):
			crashes++
		case run.ReasonCode == reasonEmptyBeforeClose, beforeClose && isRetryable(run):
		default:
			counted++
			latest = &runs[i]
		}
	}
	if crashed != nil {
		crashes++
	}
	if crashes >= crashBrake || counted >= 2 || (latest != nil && latest.Status != store.RunPartial) {
		return crashed, false, ""
	}
	return crashed, true, reason
}

// closeRow picks the row of the Close table of Decision 8 for the runs of a missing key whose
// window has closed: the index of the run to rewrite, the reason it gets, or ok false when no
// row matches. Retryable runs are not counted by rows 2 and 4.
func closeRow(runs []store.Run) (idx int, reason string, ok bool) {
	crashes, latest := 0, -1
	lastCrash := -1
	for i, run := range runs {
		switch {
		case isCrash(run):
			crashes++
			lastCrash = i
		case run.ReasonCode == reasonEmptyBeforeClose, isRetryable(run):
		default:
			latest = i
		}
	}
	switch {
	case crashes >= crashBrake: // row 1 wins over row 2
		return lastCrash, reasonCrashedRepeatedly, true
	case latest >= 0 && runs[latest].Status == store.RunPartial: // row 2
		return latest, reasonExpired, true
	case isRetryable(runs[len(runs)-1]): // row 3
		return len(runs) - 1, reasonExpired, true
	case latest >= 0: // row 4: done without a mark
		return latest, reasonExpired, true
	}
	return 0, "", false
}

// expireIfClosed closes a missing key at its window close by the Close table of Decision 8:
// the key becomes no_value and the run the row names is explained (crashed_repeatedly or
// expired). A key with a running run, with no window, or that no row matches is left alone.
// It reports whether it closed the key.
func (r *reconciler) expireIfClosed(
	ctx context.Context, runs []store.Run, closeAt time.Time, day contracts.Day, k contracts.ExpectedKey,
) (bool, error) {
	if len(runs) == 0 || closeAt.IsZero() || r.clk.Now().Before(closeAt) {
		return false, nil
	}
	for _, run := range runs {
		if run.Status == store.RunRunning {
			return false, nil
		}
	}
	idx, reason, ok := closeRow(runs)
	if !ok {
		return false, nil
	}
	run := runs[idx]
	run.ReasonCode = reason
	if err := r.ledger.RecordRun(ctx, run); err != nil {
		return false, err
	}
	return true, r.ledger.MarkPresent(ctx, day, k, store.StatusNoValue)
}

func (r *reconciler) markCrashed(ctx context.Context, run store.Run) error {
	run.Status = store.RunDone
	run.FinishedAt = r.clk.Now()
	run.ErrorCode = string(contracts.Retryable)
	run.ReasonCode = reasonCrashed
	return r.ledger.RecordRun(ctx, run)
}

// run starts run number seq of k through the key runner; it reports whether k is now marked.
func (r *reconciler) run(
	ctx context.Context, job contracts.Job, day contracts.Day, k contracts.ExpectedKey, seq int, closeAt time.Time, reason string,
) (store.Run, bool, error) {
	rec, err := r.kr.RunKey(ctx, job, day, k, seq, closeAt, deadlineFor(job, closeAt, r.clk.Now()), reason)
	if err != nil {
		return rec, false, err
	}
	present, err := r.ledger.Present(ctx, day)
	if err != nil {
		return rec, false, err
	}
	_, marked := present[k]
	return rec, marked, nil
}

// holdLease takes ops.run_lease under owner and, with a SchedulerLedger, renews it every 30 s of the
// clock until release is called. A failed renewal cancels the returned context and its
// error becomes the result of the call. release stops the renewal and frees the lease even
// when ctx was cancelled (a drain); it sets *errp when the call had no error of its own.
func (r *reconciler) holdLease(ctx context.Context, owner string) (context.Context, func(errp *error), error) {
	lease, err := r.ledger.Acquire(ctx, owner, leaseTTL)
	if err != nil {
		return ctx, nil, err
	}
	wctx := context.WithoutCancel(ctx)
	runCtx, cancel := context.WithCancel(ctx)
	var (
		mu       sync.Mutex
		renewErr error
		stop     = func() {}
	)
	if r.sched != nil {
		stop = every(runCtx, r.fclk, func() bool {
			mu.Lock()
			defer mu.Unlock()
			l, err := r.sched.RenewLease(wctx, lease, leaseTTL)
			if err != nil {
				renewErr = err
				cancel()
				return false
			}
			lease = l
			return true
		})
	}
	return runCtx, func(errp *error) {
		stop()
		cancel()
		mu.Lock()
		defer mu.Unlock()
		_ = r.ledger.Release(wctx, lease)
		if renewErr != nil && *errp == nil {
			*errp = renewErr
		}
	}, nil
}
