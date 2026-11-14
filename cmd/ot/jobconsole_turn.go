package main

import (
	"context"
	"errors"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
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
	reasonJobRunning  = "job_running"
)

// outcome is how a claimed request ended.
type outcome struct {
	state  jobrequest.State
	reason string
	lines  []jobrequest.KeyLine
}

// turn is ConsoleTurn: claim the first request of the queue and run it here, to its end. The hold of the
// slot is no longer used (Decision 3 of docs/design/console-run-lock.md); the per-job lock replaces it.
func (h *consoleHost) turn(ctx context.Context) (bool, error) {
	took, run := h.claimNext(ctx)
	if run != nil {
		run(ctx)
	}
	return took, nil
}

// claimNext starts the host, reads the queue, finishes the withdrawn and claims the first request left. It
// returns whether it did any work and, for a claimed request, the ConsoleRun that runs it and writes its end.
// It starts no goroutine.
func (h *consoleHost) claimNext(ctx context.Context) (took bool, run runner.ConsoleRun) {
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
		claimed, err := h.claim(ctx, r)
		if err != nil {
			h.warn("console claim failed")
			return took, nil
		}
		if !claimed {
			continue
		}
		return true, func(c context.Context) { h.runClaimed(c, r, jobs, bf) }
	}
	return took, nil
}

// phase runs one store call under the start-phase limit of the injected clock.
func (h *consoleHost) phase(ctx context.Context, f func(context.Context) error) error {
	c, cancel := clockTimeout(ctx, h.clk, startPhase)
	defer cancel()
	return f(c)
}

// claim claims a request.
func (h *consoleHost) claim(ctx context.Context, r jobrequest.Row) (claimed bool, err error) {
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
func (h *consoleHost) runClaimed(ctx context.Context, r jobrequest.Row, jobs []contracts.Job, bf runner.ForceBackfiller) {
	day := contracts.Day(r.Request.Day)
	out := h.execute(ctx, r.Request, jobs, bf)
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

// execute runs the request for any day, with or without force; it refuses only a job id the table lacks.
func (h *consoleHost) execute(ctx context.Context, req jobrequest.Request, jobs []contracts.Job, bf runner.ForceBackfiller) outcome {
	_, ok := jobRowOf(req.JobID)
	if !ok {
		return refused(reasonNotRunnable)
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
		h.logFailed(req, err)
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
	if !h.planned(ctx, req, day, bf) {
		if interrupted(ctx) {
			return outcome{state: jobrequest.StateInterrupted, reason: reasonStopped}
		}
		return outcome{state: jobrequest.StateFailed, reason: reasonPlanFailed}
	}
	res, err := bf.BackfillWith(ctx, day, datasets, runner.BackfillOptions{Force: req.Force, Job: req.JobID})
	out := classify(ctx, res, err)
	if out.reason == reasonError {
		h.logFailed(req, err)
	}
	return out
}

// planned makes sure day has a plan before the one job runs (Decision 7 of docs/design/single-runner.md): a day with
// no expected key and after the cursor (an empty cursor is before every day) is planned by the backfiller's
// PlanDay, which runs no job and moves no cursor. A day at or before the cursor, or a backfiller without PlanDay,
// is left as it is, so BackfillWith refuses it no_plan. It returns false when the ledger or PlanDay failed.
func (h *consoleHost) planned(ctx context.Context, req jobrequest.Request, day contracts.Day, bf runner.ForceBackfiller) bool {
	keys, _, err := h.ledger.Expected(ctx, day)
	if err == nil && len(keys) == 0 {
		var cur store.Cursor
		if cur, err = h.ledger.Cursor(ctx); err == nil && (cur.Day == "" || day > cur.Day) {
			if p, ok := bf.(runner.DayPlanner); ok {
				err = p.PlanDay(ctx, day)
			}
		}
	}
	if err != nil {
		h.logFailed(req, err)
		return false
	}
	return true
}

// logFailed writes the one ERROR line of a request that ends failed with the text of its error.
func (h *consoleHost) logFailed(req jobrequest.Request, err error) {
	if h.log == nil {
		return
	}
	h.log.Error("console request failed", "component", "ot", "status", "failed", "job", req.JobID, "day", req.Day, "error", err.Error())
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
	case !hasGroup(lines, runner.GroupRan) && !hasGroup(lines, runner.GroupRerun) && hasJobRunning(lines):
		return outcome{jobrequest.StateRefused, reasonJobRunning, lines}
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

// hasJobRunning tells whether a lease_held line carries the reason job_running.
func hasJobRunning(lines []jobrequest.KeyLine) bool {
	for _, l := range lines {
		if l.Group == runner.GroupLeaseHeld && l.ReasonCode == reasonJobRunning {
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
