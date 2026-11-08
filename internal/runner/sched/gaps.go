package sched

import (
	"context"
	"errors"
	"slices"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/jonboulle/clockwork"
)

// The gap kinds and the run reasons the syncer reads (Decisions 7, 8 and 9 of docs/design/scheduler.md). The
// reasons are the fixed strings the reconciler writes.
const (
	gapExpired          = "expired"
	gapPartial          = "partial"
	gapCrashedRepeated  = "crashed_repeatedly"
	gapPreflightFailed  = "preflight_failed"
	reasonCrashed       = "crashed"
	reasonAbandonedStop = "abandoned_stop"
	reasonEmptyBefore   = "empty_before_close"
	crashBrake          = 3
)

// gapRank orders the kinds of key gaps of one dataset and day, which share one record: the worst wins.
var gapRank = map[string]int{gapExpired: 1, gapPartial: 2, gapCrashedRepeated: 3}

type gapSyncer struct {
	ops   store.OpsLedger
	sched store.SchedulerLedger
	clk   clockwork.Clock
}

// NewGapSyncer returns the writing side of Decision 9. Sync reads the runs and present marks of a day
// through ops and writes and clears gaps through sched; clk is compared with Window.CloseAt for a key
// still missing at its close. The type is runner.GapSyncer.
func NewGapSyncer(ops store.OpsLedger, sched store.SchedulerLedger, clk clockwork.Clock) runner.GapSyncer {
	return &gapSyncer{ops: ops, sched: sched, clk: clk}
}

// Sync makes the open gaps of day match the ledger: the key gaps of the Close table (one record per dataset
// and day, the worst kind of its keys), and one preflight_failed gap per job of disabled. A gap that no
// longer holds is cleared; one already open with the same kind is not written again.
func (g *gapSyncer) Sync(ctx context.Context, day contracts.Day, disabled map[string]string) error {
	runs, err := g.ops.Runs(ctx, day)
	if err != nil {
		return err
	}
	present, err := g.ops.Present(ctx, day)
	if err != nil {
		return err
	}
	status, err := g.sched.Status(ctx)
	if err != nil {
		return err
	}
	want, err := g.keyGaps(ctx, day, runs, present)
	if err != nil {
		return err
	}
	return g.apply(ctx, day, want, disabled, status.Gaps)
}

// keyGaps returns the kind of the gap of each dataset that needs one.
func (g *gapSyncer) keyGaps(
	ctx context.Context, day contracts.Day, runs []store.Run, present map[contracts.ExpectedKey]store.PresentStatus,
) (map[string]string, error) {
	byKey := map[contracts.ExpectedKey][]store.Run{}
	var keys []contracts.ExpectedKey
	for _, r := range runs {
		if _, ok := byKey[r.Key]; !ok {
			keys = append(keys, r.Key)
		}
		byKey[r.Key] = append(byKey[r.Key], r)
	}
	closeAt := map[string]closeInfo{}
	want := map[string]string{}
	for _, k := range keys {
		ci, ok := closeAt[k.DatasetID]
		if !ok {
			var err error
			if ci, err = g.window(ctx, k.DatasetID, day); err != nil {
				return nil, err
			}
			closeAt[k.DatasetID] = ci
		}
		kind := g.keyKind(byKey[k], present[k], ci.closed)
		if gapRank[kind] > gapRank[want[k.DatasetID]] {
			want[k.DatasetID] = kind
		}
	}
	return want, nil
}

type closeInfo struct{ closed bool }

// window reports whether the window of datasetID has closed at the clock; a dataset with no window never closes.
func (g *gapSyncer) window(ctx context.Context, datasetID string, day contracts.Day) (closeInfo, error) {
	w, err := g.ops.Window(ctx, datasetID, day)
	if err != nil {
		var te contracts.TypedError
		if errors.As(err, &te) && te.Kind() == contracts.NoValue {
			return closeInfo{}, nil
		}
		return closeInfo{}, err
	}
	return closeInfo{closed: !w.CloseAt.IsZero() && !g.clk.Now().Before(w.CloseAt)}, nil
}

// keyKind is the gap kind of one key from its runs and mark, or "" for none. The runs are in order. A key
// that is present has none. A missing key gets the kind of the first row of the Close table that matches,
// the brake (row 1) before the close as well; a key already no_value keeps the kind its runner-written
// reason names, and a no_value with any other reason has none.
func (g *gapSyncer) keyKind(runs []store.Run, mark store.PresentStatus, closed bool) string {
	if mark == store.StatusPresent {
		return ""
	}
	now := g.clk.Now()
	crashes, latest, running := 0, -1, false
	proof, expired, retryableLast := false, false, false
	for i, r := range runs {
		retryableLast = false
		switch {
		case r.Status == store.RunRunning:
			if r.LeaseUntil.Before(now) { // a lapsed lease is a crash, as the reconciler counts it
				crashes++
			} else {
				running = true
			}
		case r.ReasonCode == gapCrashedRepeated:
			proof = true
		case r.ReasonCode == reasonCrashed || r.ReasonCode == reasonAbandonedStop:
			crashes++
		case r.ReasonCode == reasonEmptyBefore:
		case r.Status == store.RunDone && r.ErrorCode == string(contracts.Retryable):
			retryableLast = true
			expired = expired || r.ReasonCode == gapExpired
		default:
			latest = i
			expired = expired || r.ReasonCode == gapExpired
		}
	}
	if proof || crashes >= crashBrake {
		return gapCrashedRepeated
	}
	if mark == store.StatusNoValue {
		return runnerKind(expired, latest, retryableLast, runs)
	}
	if !closed || running {
		return ""
	}
	return runnerKind(true, latest, retryableLast, runs)
}

// runnerKind picks row 2, 3 or 4 of the Close table; it is "" unless ok, which for a no_value key means
// the runner wrote the reason expired.
func runnerKind(ok bool, latest int, retryableLast bool, runs []store.Run) string {
	switch {
	case !ok:
		return ""
	case latest >= 0 && runs[latest].Status == store.RunPartial:
		return gapPartial
	case retryableLast, latest >= 0:
		return gapExpired
	}
	return ""
}

// apply writes the wanted gaps that are not open with the same kind and clears the open gaps of day that
// no longer hold.
func (g *gapSyncer) apply(
	ctx context.Context, day contracts.Day, want, disabled map[string]string, open []store.Gap,
) error {
	have := map[[2]string]string{} // job, dataset -> reason of the open gap of day
	var stale []store.Gap
	for _, o := range open {
		if o.Day != day {
			continue
		}
		have[[2]string{o.JobID, o.DatasetID}] = o.Reason
		if o.JobID != "" && o.DatasetID == "" {
			if _, ok := disabled[o.JobID]; !ok {
				stale = append(stale, o)
			}
		} else if o.JobID == "" {
			if _, ok := want[o.DatasetID]; !ok {
				stale = append(stale, o)
			}
		}
	}
	for _, id := range sortedKeys(want) {
		if have[[2]string{"", id}] != want[id] {
			if err := g.sched.PutGap(ctx, store.Gap{DatasetID: id, Day: day, Reason: want[id]}); err != nil {
				return err
			}
		}
	}
	for _, id := range sortedKeys(disabled) {
		if _, ok := have[[2]string{id, ""}]; !ok {
			if err := g.sched.PutGap(ctx, store.Gap{JobID: id, Day: day, Reason: gapPreflightFailed}); err != nil {
				return err
			}
		}
	}
	for _, s := range stale {
		if err := g.sched.ClearGap(ctx, store.Gap{JobID: s.JobID, DatasetID: s.DatasetID, Day: day}); err != nil {
			return err
		}
	}
	return nil
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}
