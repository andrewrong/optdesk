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
	"sort"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
)

const (
	owner    = "runner"
	leaseTTL = 10 * time.Minute

	reasonCrashed = "crashed"
	reasonLate    = "late"
	// reasonEmptyBeforeClose marks a finished run that came back empty before the window
	// close: the key stays missing and the run does not count toward the two-run cap.
	reasonEmptyBeforeClose = "empty_before_close"
	// The reason "expired" explains a key whose last run ended retryable and whose window has closed.
	reasonExpired = "expired"
)

type reconciler struct {
	ledger store.OpsLedger
	cal    clock.Calendar
	clk    clock.Clock
	jobs   []contracts.Job
	rule   runner.WindowRule
}

// New builds the reconciler with the default window rule (DefaultCalendarID, no offsets).
func New(l store.OpsLedger, cal clock.Calendar, clk clock.Clock, jobs []contracts.Job) runner.Reconciler {
	return NewWithRule(l, cal, clk, jobs, runner.WindowRule{})
}

// NewWithRule builds the reconciler with an explicit window rule.
func NewWithRule(l store.OpsLedger, cal clock.Calendar, clk clock.Clock, jobs []contracts.Job, rule runner.WindowRule) runner.Reconciler {
	return &reconciler{ledger: l, cal: cal, clk: clk, jobs: jobs, rule: rule}
}

// Reconcile handles every session day after the cursor up to day, in order. It stops at
// the first day that is not fully reconciled, so the cursor holds on a gap.
func (r *reconciler) Reconcile(ctx context.Context, day contracts.Day) error {
	if err := checkCalendars(r.jobs, r.rule); err != nil {
		return err
	}
	lease, err := r.ledger.Acquire(ctx, owner, leaseTTL)
	if err != nil {
		return err
	}
	defer func() { _ = r.ledger.Release(ctx, lease) }()

	cur, err := r.ledger.Cursor(ctx)
	if err != nil {
		return err
	}
	sessions, err := r.sessions(ctx, cur.Day, day)
	if err != nil {
		return err
	}
	for _, s := range sessions {
		done, err := r.reconcileDay(ctx, s)
		if err != nil {
			return err
		}
		if !done {
			return nil
		}
		if err := r.ledger.SetCursor(ctx, store.Cursor{Day: s.Day}); err != nil {
			return err
		}
	}
	return nil
}

// sessions lists the session days after from up to and including to. With no cursor
// (empty from) only day to itself is considered.
func (r *reconciler) sessions(ctx context.Context, from, to contracts.Day) ([]clock.Session, error) {
	id := calendarIDOf(r.rule)
	if from == "" {
		s, err := r.cal.Session(ctx, id, to)
		if err != nil || !s.IsSession {
			return nil, err
		}
		return []clock.Session{s}, nil
	}
	var out []clock.Session
	for from < to {
		s, err := r.cal.NextSession(ctx, id, from)
		if err != nil {
			var te contracts.TypedError
			if errors.As(err, &te) && te.Kind() == contracts.NoValue {
				break
			}
			return nil, err
		}
		if s.Day > to {
			break
		}
		out = append(out, s)
		from = s.Day
	}
	return out, nil
}

// plan is the merged plan of all jobs for a day.
type plan struct {
	keys []contracts.ExpectedKey
	job  map[contracts.ExpectedKey]contracts.Job
	hash string
}

func (r *reconciler) plan(ctx context.Context, day contracts.Day) (plan, error) {
	p := plan{job: map[contracts.ExpectedKey]contracts.Job{}}
	h := sha256.New()
	for _, j := range r.jobs {
		keys, hash, err := j.Plan(ctx, day)
		if err != nil {
			return plan{}, fmt.Errorf("plan %s %s: %w", j.ID(), day, err)
		}
		fmt.Fprintf(h, "%s=%s;", j.ID(), hash)
		for _, k := range keys {
			if _, dup := p.job[k]; !dup {
				p.job[k] = j
				p.keys = append(p.keys, k)
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

// reconcileDay plans the day, puts its windows, starts the repair and crash re-runs
// and reports whether every expected key is present or no_value.
func (r *reconciler) reconcileDay(ctx context.Context, s clock.Session) (bool, error) {
	day := s.Day
	p, err := r.plan(ctx, day)
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
	closes, err := r.putWindows(ctx, p, s)
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
		closeAt := closes[k.DatasetID]
		if expired, err := r.expireIfClosed(ctx, byKey[k], closeAt, day, k); err != nil {
			return false, err
		} else if expired {
			continue
		}
		crashed, start, reason := r.nextAttempt(byKey[k], closeAt)
		if !start {
			complete = false
			continue
		}
		if crashed != nil {
			if err := r.markCrashed(ctx, *crashed); err != nil {
				return false, err
			}
		}
		ok, err := r.run(ctx, p.job[k], day, k, len(byKey[k])+1, closeAt, reason)
		if err != nil {
			return false, err
		}
		complete = complete && ok
	}
	return complete, nil
}

// putWindows puts the window of every dataset of the plan and returns the close of each.
func (r *reconciler) putWindows(ctx context.Context, p plan, s clock.Session) (map[string]time.Time, error) {
	closes := map[string]time.Time{}
	seen := map[string]bool{}
	for _, k := range p.keys {
		if seen[k.DatasetID] {
			continue
		}
		seen[k.DatasetID] = true
		w, ok := windowFor(ruleOf(p.job[k], r.rule), k.DatasetID, s)
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

// nextAttempt decides whether a run starts for a missing key. Runs that ended empty or
// retryable before the window close do not count toward the cap of two (repair and crash re-run). A running
// run with a live lease is pending: nothing starts. An expired one is returned as crashed
// and re-run once. closeAt is the close of the key's dataset window, zero when it has none.
func (r *reconciler) nextAttempt(runs []store.Run, closeAt time.Time) (crashed *store.Run, start bool, reason string) {
	now := r.clk.Now()
	if !closeAt.IsZero() && now.After(closeAt) {
		reason = reasonLate
	}
	beforeClose := !closeAt.IsZero() && now.Before(closeAt)
	counted := 0
	var running *store.Run
	for i, run := range runs {
		if run.ReasonCode == reasonEmptyBeforeClose {
			continue
		}
		if beforeClose && run.Status == store.RunDone && run.ErrorCode == string(contracts.Retryable) && run.ReasonCode != reasonCrashed {
			continue
		}
		counted++
		if run.Status == store.RunRunning {
			running = &runs[i]
		}
	}
	switch {
	case running != nil:
		if counted == 1 && running.LeaseUntil.Before(now) {
			return running, true, reason
		}
		return nil, false, ""
	case counted == 0:
		return nil, true, reason
	}
	return nil, false, ""
}

// expireIfClosed makes the absence of k final when its last run ended retryable and the window
// has closed: no_value, with the last run explained as expired. It reports whether it did.
func (r *reconciler) expireIfClosed(
	ctx context.Context, runs []store.Run, closeAt time.Time, day contracts.Day, k contracts.ExpectedKey,
) (bool, error) {
	if len(runs) == 0 || closeAt.IsZero() || r.clk.Now().Before(closeAt) {
		return false, nil
	}
	// A crashed run also carries the code, but it is not a vendor answer: it is never expired.
	last := runs[len(runs)-1]
	if last.Status != store.RunDone || last.ErrorCode != string(contracts.Retryable) || last.ReasonCode == reasonCrashed {
		return false, nil
	}
	last.ReasonCode = reasonExpired
	if err := r.ledger.RecordRun(ctx, last); err != nil {
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

// run starts run number seq of k, records it and marks the key; it reports whether k is now present.
func (r *reconciler) run(
	ctx context.Context, job contracts.Job, day contracts.Day, k contracts.ExpectedKey, seq int, closeAt time.Time, reason string,
) (bool, error) {
	now := r.clk.Now()
	rec := store.Run{
		ID:          fmt.Sprintf("%s|%s|%s|%d", day, k.DatasetID, k.SubjectKey, seq),
		Day:         day,
		Key:         k,
		Status:      store.RunRunning,
		LeaseUntil:  now.Add(leaseTTL),
		HeartbeatAt: now,
		ReasonCode:  reason,
	}
	if err := r.ledger.RecordRun(ctx, rec); err != nil {
		return false, err
	}
	res := job.Run(ctx, day, []contracts.ExpectedKey{k})
	rec.Status = store.RunDone
	rec.FinishedAt = r.clk.Now()
	rec.ReturnedN, rec.UsableN = res.Returned, res.Usable
	var status store.PresentStatus
	switch {
	case res.Err != nil && res.Err.Kind() == contracts.NoValue:
		status = store.StatusNoValue
	case res.Err != nil:
		rec.ErrorCode = string(res.Err.Kind())
	case res.Usable > 0:
		status = store.StatusPresent
	case !closeAt.IsZero() && r.clk.Now().Before(closeAt):
		// An empty answer is no explanation: retry until the window closes.
		rec.ReasonCode = reasonEmptyBeforeClose
	default:
		status = store.StatusNoValue
	}
	if err := r.ledger.RecordRun(ctx, rec); err != nil {
		return false, err
	}
	if status == "" {
		return false, nil
	}
	return true, r.ledger.MarkPresent(ctx, day, k, status)
}
