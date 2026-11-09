package main

import (
	"context"
	"fmt"
	"io"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// reasonBeforeClose is how the runner's empty_before_close reason code is printed.
const reasonBeforeClose = "before_close"

// printOutcomes reads day back from ledger and prints one outcome line per key of each job, one left_out line
// when leftOut is not empty, and the closing summary. The status is nothing_captured when no key is captured,
// else ok; the caller's exit code does not depend on it.
func printOutcomes(ctx context.Context, w io.Writer, ledger store.OpsLedger, js []contracts.Job, day contracts.Day, leftOut string) error {
	present, err := ledger.Present(ctx, day)
	if err != nil {
		return fmt.Errorf("read present keys failed")
	}
	runs, err := ledger.Runs(ctx, day)
	if err != nil {
		return fmt.Errorf("read runs failed")
	}
	last := lastRuns(runs)
	captured, keys := 0, 0
	for _, j := range js {
		ks, _, err := j.Plan(ctx, day)
		if err != nil {
			_, _ = fmt.Fprintf(w, "job=%s day=%s outcome=failed reason=plan_failed\n", j.ID(), day)
			continue
		}
		for _, k := range ks {
			keys++
			line, ok := outcomeText(present[k], last, k)
			if ok {
				captured++
			}
			_, _ = fmt.Fprintf(w, "job=%s day=%s dataset=%s subject=%s %s\n", j.ID(), day, k.DatasetID, k.SubjectKey, line)
		}
	}
	if leftOut != "" {
		_, _ = fmt.Fprintf(w, "job=%s day=%s outcome=left_out reason=%s\n", jobEarningsRefresh, day, leftOut)
	}
	status := "status=nothing_captured"
	if captured > 0 {
		status = "status=ok"
	}
	_, _ = fmt.Fprintf(w, "run-once %s %s captured=%d keys=%d\n", day, status, captured, keys)
	return nil
}

// lastRuns keeps, per key, the run that finished last (the later one in the list on a tie).
func lastRuns(runs []store.Run) map[contracts.ExpectedKey]store.Run {
	out := make(map[contracts.ExpectedKey]store.Run, len(runs))
	for _, r := range runs {
		if old, ok := out[r.Key]; !ok || !r.FinishedAt.Before(old.FinishedAt) {
			out[r.Key] = r
		}
	}
	return out
}

// outcomeText is the outcome fields of one key and whether the key is captured.
func outcomeText(st store.PresentStatus, last map[contracts.ExpectedKey]store.Run, k contracts.ExpectedKey) (string, bool) {
	r, hasRun := last[k]
	switch {
	case st == store.StatusPresent:
		return fmt.Sprintf("outcome=captured rows=%d", r.UsableN), true
	case st == store.StatusNoValue:
		return "outcome=no_value reason=no_value", false
	case hasRun && r.ErrorCode != "":
		return "outcome=failed reason=" + r.ErrorCode, false
	case hasRun && r.ReasonCode != "":
		reason := r.ReasonCode
		if reason == "empty_before_close" {
			reason = reasonBeforeClose
		}
		return "outcome=empty reason=" + reason, false
	}
	return "outcome=missing", false
}
