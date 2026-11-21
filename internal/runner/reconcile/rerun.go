package reconcile

import (
	"context"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
)

// rerunMarked reports whether the dependent d, which ran for the day, runs once more: the newest run of every
// one of its keys is a done run marked upstream_missing or upstream_partial. A missing mark is replaced when
// every upstream in After() is ok or ended; a partial mark (no missing one among the keys) only when every
// upstream is ok. ends holds the upstream whose run just ended ok, as for upstreamState. The answer is read
// from the ledger, so it holds after a restart; the replacing run starts on a better upstream than the one it
// replaces, so no run is replaced twice and the unmarked run is the last.
func (r *reconciler) rerunMarked(
	ctx context.Context, dc *dayCtx, d contracts.Job, keys []contracts.ExpectedKey,
	byKey map[contracts.ExpectedKey][]store.Run, ends map[string]bool,
) (bool, error) {
	dep, ok := d.(runner.Dependent)
	if !ok || len(keys) == 0 {
		return false, nil
	}
	partial := true // no key carries upstream_missing
	for _, k := range keys {
		runs := byKey[k]
		if len(runs) == 0 {
			return false, nil
		}
		newest := runs[len(runs)-1]
		if newest.Status != store.RunDone || (newest.ReasonCode != reasonUpstreamMissing && newest.ReasonCode != reasonUpstreamPartial) {
			return false, nil
		}
		if newest.ReasonCode == reasonUpstreamMissing {
			partial = false
		}
	}
	for _, id := range dep.After() {
		st, err := r.upstreamState(ctx, dc, id, ends)
		if err != nil || (st != upOK && (partial || st != upEnded)) {
			return false, err
		}
	}
	return true, nil
}
