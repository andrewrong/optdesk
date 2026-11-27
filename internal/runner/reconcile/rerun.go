package reconcile

import (
	"context"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
)

// rerunMarked reports whether the dependent d, which ran for the day, runs once more: the newest run of every
// one of its keys is a done run marked upstream_missing or upstream_partial. A missing mark is replaced when
// every upstream in After() is ok or ended; a partial mark (no missing one among the keys) only when every
// upstream is ok. ends holds the upstream whose run just ended ok, as for upstreamState. The answer is read
// from the ledger, so it holds after a restart; the replacing run starts on a better upstream than the one it
// replaces, so no run is replaced twice and the unmarked run is the last. A dependent whose newest runs are
// not marked is run once more when they all finished before an upstream ended ok (issue 4422); a run that
// finished at or after that end is the last one.
func (r *reconciler) rerunMarked(
	ctx context.Context, dc *dayCtx, d contracts.Job, keys []contracts.ExpectedKey,
	byKey map[contracts.ExpectedKey][]store.Run, ends map[string]bool,
) (bool, error) {
	dep, ok := d.(runner.Dependent)
	if !ok || len(keys) == 0 {
		return false, nil
	}
	partial, marked := true, true // no key carries upstream_missing; every newest run is marked
	var newestEnd time.Time       // the latest finish among the newest runs of the keys
	for _, k := range keys {
		runs := byKey[k]
		if len(runs) == 0 {
			return false, nil
		}
		newest := runs[len(runs)-1]
		if newest.Status != store.RunDone {
			return false, nil
		}
		if newest.FinishedAt.After(newestEnd) {
			newestEnd = newest.FinishedAt
		}
		if newest.ReasonCode != reasonUpstreamMissing && newest.ReasonCode != reasonUpstreamPartial {
			marked = false
		}
		if newest.ReasonCode == reasonUpstreamMissing {
			partial = false
		}
	}
	allOK := true
	for _, id := range dep.After() {
		st, err := r.upstreamState(ctx, dc, id, ends)
		if err != nil {
			return false, err
		}
		if st != upOK {
			allOK = false
			if marked && (partial || st != upEnded) {
				return false, nil
			}
		}
	}
	if marked {
		return true, nil
	}
	return allOK && r.finishedBeforeUpstream(dc, dep, byKey, newestEnd), nil
}

// finishedBeforeUpstream reports, for a dependent whose newest runs are unmarked, whether they all finished
// before an upstream ended ok for the day.
func (r *reconciler) finishedBeforeUpstream(
	dc *dayCtx, dep runner.Dependent, byKey map[contracts.ExpectedKey][]store.Run, newestEnd time.Time,
) bool {
	for _, id := range dep.After() {
		if end, has := r.upstreamFinished(dc, id, byKey); has && newestEnd.Before(end) {
			return true
		}
	}
	return false
}

// upstreamFinished is the instant upstream id first ended ok on the day of dc: the latest, over its keys, of the
// first ok run of the key. A later run of a key (a forced re-run, a manual pull) does not move it, so it starts
// no dependent. False when the upstream has no key or a key has no ok run.
func (r *reconciler) upstreamFinished(dc *dayCtx, id string, byKey map[contracts.ExpectedKey][]store.Run) (time.Time, bool) {
	var end time.Time
	for _, k := range dc.keysOf(id) {
		var first time.Time
		for _, run := range byKey[k] {
			if run.Status == store.RunDone && run.ErrorCode == "" && (first.IsZero() || run.FinishedAt.Before(first)) {
				first = run.FinishedAt
			}
		}
		if first.IsZero() {
			return time.Time{}, false
		}
		if first.After(end) {
			end = first
		}
	}
	return end, !end.IsZero()
}
