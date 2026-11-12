package main

import (
	"context"
	"errors"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store/jobrequest"
)

// Design: docs/design/job-console.md#Decisions 8 and 9; the order is written in
// docs/contracts/ot/job_console_consumer.md.

// The reason codes the host stores (every ended state has one).
const (
	reasonCompleted   = "completed"
	reasonWithdrawn   = "withdrawn"
	reasonStopped     = "stopped"
	reasonNoPlan      = "no_plan"
	reasonNoKey       = "no_key"
	reasonNotDue      = "not_due"
	reasonNotToday    = "not_today"
	reasonNotRunnable = "not_runnable"
	reasonLeftOut     = "left_out"
	reasonLeaseHeld   = "lease_held"
	reasonPlanFailed  = "plan_failed"
	reasonError       = "error"
	reasonSlotNear    = "slot_near"
)

// outcome is how a claimed request ended.
type outcome struct {
	state  jobrequest.State
	reason string
	lines  []jobrequest.KeyLine
}

// turn is ConsoleTurn: start, read the queue, finish the withdrawn, take the first request no slot holds,
// run it and write its end.
func (h *consoleHost) turn(ctx context.Context, hold runner.SlotHold) (bool, error) {
	if _, _, started := h.current(); !started {
		sctx, cancel := clockTimeout(ctx, h.clk, startPhase)
		err := h.Start(sctx)
		cancel()
		if err != nil {
			h.warn("console start failed")
			return false, nil
		}
	}
	jobs, bf, _ := h.current()
	if bf == nil {
		return false, nil
	}
	var rows []jobrequest.Row
	if err := h.phase(ctx, func(c context.Context) (err error) { rows, err = h.q.Queued(c, queueLimit); return }); err != nil {
		h.warn("console queue read failed")
		return false, nil
	}
	took := false
	var waiting []jobrequest.Row
	for _, r := range rows {
		if r.WithdrawnAt.IsZero() {
			waiting = append(waiting, r)
			continue
		}
		if !h.finish(ctx, r.ID, jobrequest.StateWithdrawn, reasonWithdrawn, nil) {
			return took, nil
		}
		took = true
	}
	for _, r := range waiting {
		if hold != nil {
			job, at, held := hold(turnMax(jobs, r.Request.JobID))
			if held {
				if r.HoldJob != job || !r.HoldAt.Equal(at) {
					if err := h.phase(ctx, func(c context.Context) error { return h.q.Hold(c, r.ID, job, at) }); err != nil {
						h.warn("console hold write failed")
						return took, nil
					}
				}
				continue
			}
		}
		claimed, err := h.claim(ctx, r)
		if err != nil {
			h.warn("console claim failed")
			return took, nil
		}
		if !claimed {
			continue
		}
		h.runClaimed(ctx, hold, r, jobs, bf)
		return true, nil
	}
	return took, nil
}

// phase runs one store call under the start-phase limit of the injected clock.
func (h *consoleHost) phase(ctx context.Context, f func(context.Context) error) error {
	c, cancel := clockTimeout(ctx, h.clk, startPhase)
	defer cancel()
	return f(c)
}

// claim clears the hold of a request that is taken, then claims it.
func (h *consoleHost) claim(ctx context.Context, r jobrequest.Row) (claimed bool, err error) {
	if r.HoldJob != "" || !r.HoldAt.IsZero() {
		if err := h.phase(ctx, func(c context.Context) error { return h.q.Hold(c, r.ID, "", time.Time{}) }); err != nil {
			return false, err
		}
	}
	err = h.phase(ctx, func(c context.Context) (err error) { claimed, err = h.q.Claim(c, r.ID, h.clk.Now()); return })
	return claimed, err
}

// finish ends a request under a context that the stop of the turn does not cancel, with the 30 s limit.
func (h *consoleHost) finish(ctx context.Context, id int64, s jobrequest.State, reason string, lines []jobrequest.KeyLine) bool {
	c, cancel := clockTimeout(context.WithoutCancel(ctx), h.clk, finishTimeout)
	defer cancel()
	if err := h.q.Finish(c, id, s, reason, lines, h.clk.Now()); err != nil {
		h.warn("console finish failed")
		return false
	}
	return true
}

// runClaimed runs a claimed request, writes its end and, after a run of a missing key, syncs the key gaps.
func (h *consoleHost) runClaimed(ctx context.Context, hold runner.SlotHold, r jobrequest.Row, jobs []contracts.Job, bf runner.ForceBackfiller) {
	day := contracts.Day(r.Request.Day)
	out := h.execute(ctx, hold, r.Request, jobs, bf)
	if !h.finish(ctx, r.ID, out.state, out.reason, out.lines) {
		return
	}
	if (out.state != jobrequest.StateDone && out.state != jobrequest.StateFailed) || !hasGroup(out.lines, runner.GroupRan) {
		return
	}
	c, cancel := clockTimeout(ctx, h.clk, syncTimeout)
	defer cancel()
	if err := h.syncer.SyncKeys(c, day); err != nil {
		h.warn("console key gap sync failed")
	}
}

// execute checks the request against the table and the job set, and runs it.
func (h *consoleHost) execute(ctx context.Context, hold runner.SlotHold, req jobrequest.Request, jobs []contracts.Job, bf runner.ForceBackfiller) outcome {
	row, ok := jobRowOf(req.JobID)
	if !ok || !row.runnable() || (req.Force && !row.Force) {
		return refused(reasonNotRunnable)
	}
	allowed, err := dayAllowed(row, h.clk.Now(), req.Day)
	if err != nil {
		return refused(reasonNotRunnable)
	}
	if !allowed {
		return refused(reasonNotToday)
	}
	var job contracts.Job
	for _, j := range jobs {
		if j.ID() == req.JobID {
			job = j
			break
		}
	}
	if job == nil {
		return refused(reasonLeftOut)
	}
	day := contracts.Day(req.Day)
	keys, _, err := job.Plan(ctx, day)
	if interrupted(ctx) {
		return outcome{state: jobrequest.StateInterrupted, reason: reasonStopped}
	}
	if err != nil {
		return outcome{state: jobrequest.StateFailed, reason: reasonPlanFailed}
	}
	var datasets []string
	seen := map[string]bool{}
	for _, k := range keys {
		if !seen[k.DatasetID] {
			seen[k.DatasetID] = true
			datasets = append(datasets, k.DatasetID)
		}
	}
	if len(datasets) == 0 {
		return refused(reasonNoKey)
	}
	res, err := bf.BackfillWith(ctx, day, datasets, runner.BackfillOptions{Force: req.Force, Job: req.JobID, Hold: hold})
	return classify(ctx, res, err)
}

// classify gives the end state of a call in the precedence of Decision 9.
func classify(ctx context.Context, res runner.BackfillResult, err error) outcome {
	lines := keyLines(res.Keys)
	switch {
	case interrupted(ctx):
		return outcome{jobrequest.StateInterrupted, reasonStopped, lines}
	case res.Refused == runner.RefusedNoPlan:
		return outcome{jobrequest.StateRefused, reasonNoPlan, lines}
	case res.Refused == runner.RefusedNoKey:
		return outcome{jobrequest.StateRefused, reasonNoKey, lines}
	case err != nil:
		var te contracts.TypedError
		if errors.As(err, &te) && te.Kind() == contracts.Retryable {
			return outcome{jobrequest.StateFailed, reasonLeaseHeld, lines}
		}
		return outcome{jobrequest.StateFailed, reasonError, lines}
	case !hasGroup(lines, runner.GroupRan) && !hasGroup(lines, runner.GroupRerun) && hasGroup(lines, runner.GroupNotDue):
		return outcome{jobrequest.StateRefused, reasonNotDue, lines}
	}
	for _, l := range lines {
		if l.Group == runner.GroupNotReached && l.ReasonCode == reasonSlotNear {
			return outcome{jobrequest.StateFailed, reasonSlotNear, lines}
		}
	}
	return outcome{jobrequest.StateDone, reasonCompleted, lines}
}

// refused is an outcome that ran nothing.
func refused(reason string) outcome {
	return outcome{state: jobrequest.StateRefused, reason: reason}
}

// interrupted tells whether the turn's context was cancelled by a stop.
func interrupted(ctx context.Context) bool {
	return ctx.Err() != nil && errors.Is(context.Cause(ctx), context.Canceled)
}

// hasGroup tells whether a line of the group exists.
func hasGroup(lines []jobrequest.KeyLine, group string) bool {
	for _, l := range lines {
		if l.Group == group {
			return true
		}
	}
	return false
}

// keyLines converts the lines of the runner to the lines the store keeps.
func keyLines(keys []runner.KeyRun) []jobrequest.KeyLine {
	out := make([]jobrequest.KeyLine, 0, len(keys))
	for _, k := range keys {
		out = append(out, jobrequest.KeyLine{
			Dataset: k.Key.DatasetID, Subject: k.Key.SubjectKey, Group: k.Group, Status: k.Status,
			ErrorCode: k.ErrorCode, ReasonCode: k.ReasonCode, Mark: k.Mark,
			Returned: k.Returned, Usable: k.Usable, Attempt: k.Attempt, DueAt: k.DueAt,
		})
	}
	return out
}

// dayAllowed applies the day rule of the row: an any-day job takes a day not after today of the market zone,
// a today-only job takes today in its own zone.
func dayAllowed(row jobRow, now time.Time, day string) (bool, error) {
	if _, err := time.Parse(dayLayout, day); err != nil {
		return false, err
	}
	zone := row.Zone
	if zone == "" {
		zone = marketZone
	}
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return false, err
	}
	today := now.In(loc).Format(dayLayout)
	if row.Kind == kindTodayOnly {
		return day == today, nil
	}
	return day <= today, nil
}

// turnMax is how long a request of the job can keep the worker: the start phase, the run bound of the first
// key, the Finish and the SyncKeys. A job that is not in the set is refused at the claim and runs no key.
func turnMax(jobs []contracts.Job, id string) time.Duration {
	for _, j := range jobs {
		if j.ID() != id {
			continue
		}
		bound := defaultRunBound
		if b, ok := j.(runner.Bounded); ok {
			bound = b.RunBound()
		}
		return startPhase + bound + finishTimeout + syncTimeout
	}
	return startPhase + finishTimeout
}
