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
)

type reconciler struct {
	ledger store.OpsLedger
	cal    clock.Calendar
	clk    clock.Clock
	jobs   []contracts.Job
	rule   WindowRule
}

// New builds the reconciler with the default window rule (DefaultCalendarID, no offsets).
func New(l store.OpsLedger, cal clock.Calendar, clk clock.Clock, jobs []contracts.Job) runner.Reconciler {
	return NewWithRule(l, cal, clk, jobs, WindowRule{})
}

// NewWithRule builds the reconciler with an explicit window rule.
func NewWithRule(l store.OpsLedger, cal clock.Calendar, clk clock.Clock, jobs []contracts.Job, rule WindowRule) runner.Reconciler {
	return &reconciler{ledger: l, cal: cal, clk: clk, jobs: jobs, rule: rule}
}

// Reconcile handles every session day after the cursor up to day, in order. It stops at
// the first day that is not fully reconciled, so the cursor holds on a gap.
func (r *reconciler) Reconcile(ctx context.Context, day contracts.Day) error {
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
	id := r.rule.calendarID()
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
	var closeAt time.Time
	seen := map[string]bool{}
	for _, k := range p.keys {
		if seen[k.DatasetID] {
			continue
		}
		seen[k.DatasetID] = true
		w, ok := r.rule.Window(k.DatasetID, s)
		if !ok {
			continue
		}
		closeAt = w.CloseAt
		if err := r.ledger.PutWindow(ctx, w); err != nil {
			return false, err
		}
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
		attempt, reason := r.nextAttempt(byKey[k], closeAt)
		if attempt == 0 {
			complete = false
			continue
		}
		if attempt > 1 {
			if err := r.markCrashed(ctx, byKey[k][0]); err != nil {
				return false, err
			}
		}
		ok, err := r.run(ctx, p.job[k], day, k, attempt, reason)
		if err != nil {
			return false, err
		}
		complete = complete && ok
	}
	return complete, nil
}

// nextAttempt returns the attempt number to start for a missing key, or 0 when none
// is allowed: a live run is pending, or the repair run and the crash re-run are spent.
func (r *reconciler) nextAttempt(runs []store.Run, closeAt time.Time) (int, string) {
	reason := ""
	if !closeAt.IsZero() && r.clk.Now().After(closeAt) {
		reason = reasonLate
	}
	switch len(runs) {
	case 0:
		return 1, reason
	case 1:
		if runs[0].Status == store.RunRunning && runs[0].LeaseUntil.Before(r.clk.Now()) {
			return 2, reason
		}
	}
	return 0, ""
}

func (r *reconciler) markCrashed(ctx context.Context, run store.Run) error {
	run.Status = store.RunDone
	run.FinishedAt = r.clk.Now()
	run.ErrorCode = string(contracts.Retryable)
	run.ReasonCode = reasonCrashed
	return r.ledger.RecordRun(ctx, run)
}

// run starts one attempt of k, records it and marks the key; it reports whether k is now present.
func (r *reconciler) run(ctx context.Context, job contracts.Job, day contracts.Day, k contracts.ExpectedKey, attempt int, reason string) (bool, error) {
	now := r.clk.Now()
	rec := store.Run{
		ID:          fmt.Sprintf("%s|%s|%s|%d", day, k.DatasetID, k.SubjectKey, attempt),
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
