package reconcile

import (
	"context"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
)

// rerunMarked reports whether the dependent d, which ran for the day, runs once more as a plain run: the
// newest run of every one of its keys is a done run marked upstream_missing, and every upstream in After()
// is ok for the day. ends holds the upstream whose run just ended, as for upstreamState. The answer is read
// from the ledger, so it holds after a restart; the plain run that follows carries no mark, so it is once.
func (r *reconciler) rerunMarked(
	ctx context.Context, dc *dayCtx, d contracts.Job, keys []contracts.ExpectedKey,
	byKey map[contracts.ExpectedKey][]store.Run, ends map[string]bool,
) (bool, error) {
	dep, ok := d.(runner.Dependent)
	if !ok || len(keys) == 0 {
		return false, nil
	}
	for _, k := range keys {
		runs := byKey[k]
		if len(runs) == 0 {
			return false, nil
		}
		if newest := runs[len(runs)-1]; newest.Status != store.RunDone || newest.ReasonCode != reasonUpstreamMissing {
			return false, nil
		}
	}
	for _, id := range dep.After() {
		st, err := r.upstreamState(ctx, dc, id, ends)
		if err != nil || st != upOK {
			return false, err
		}
	}
	return true, nil
}
