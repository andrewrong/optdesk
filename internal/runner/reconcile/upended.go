package reconcile

import (
	"context"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// reasonUpstreamPartial marks a run of a dependent that started when its upstream had ended for the day with a
// key not stored (docs/design/candidates-after-partial-open.md, Decision 2). It is replaced once, by an
// unmarked run, when the upstream turns ok.
const reasonUpstreamPartial = "upstream_partial"

// upstreamEnded classifies an upstream whose keys are not all stored and none is running with a live lease:
// upMissing when no key has a run (it did not run yet), upEnded when every key not stored is ended, else
// upRunning (the dependent is left, no mark). A key is ended when it has at least one run and nextAttempt
// would not start another, or when its newest run is retryable and the run bound of the upstream has passed
// since its slot (an upstream with no slot of the day has no wait).
func (r *reconciler) upstreamEnded(
	ctx context.Context, dc *dayCtx, job contracts.Job, d contracts.Day, keys []contracts.ExpectedKey,
	present map[contracts.ExpectedKey]store.PresentStatus, byKey map[contracts.ExpectedKey][]store.Run,
) (upState, error) {
	now := r.clk.Now()
	var boundAt time.Time
	if d == dc.day {
		if at, sched, err := slotAt(job, dc.sess, r.rule); err == nil && sched {
			boundAt = at.Add(runBound(job))
		}
	}
	ran, ended := false, true
	for _, k := range keys {
		runs := byKey[k]
		if _, ok := present[k]; ok {
			ran = true
			continue
		}
		if len(runs) == 0 {
			ended = false
			continue
		}
		ran = true
		closeAt, err := r.closeOf(ctx, k.DatasetID, d)
		if err != nil {
			return upMissing, err
		}
		_, start, _ := r.nextAttempt(runs, closeAt)
		held := !closeAt.IsZero() && now.Before(closeAt) && now.Sub(lastEmptyFinish(runs)) < emptyRetryGap
		switch {
		case !start && !held:
		case start && isRetryable(runs[len(runs)-1]) && !now.Before(boundAt):
		default:
			ended = false
		}
	}
	switch {
	case !ran:
		return upMissing, nil
	case ended:
		return upEnded, nil
	}
	return upRunning, nil
}
