package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/store/runview"
)

// Design: docs/design/option-open-gap-jp.md#Same-morning-visibility (Decision 11, entry 8).

// The fixed key of the not_wired set: the day, dataset and subject of every row; the scope is the job id.
const (
	codeNotWired     = "not_wired"
	subjectNotWired  = "scheduler"
	notWiredLogLabel = "not_wired reasons not recorded"
)

// notWiredRecorder collects the jobs a startup leaves out, each with the NAME of the key it lacks (never a
// value), and writes them in one call at the end of the wiring. A nil recorder records nothing: only the
// scheduler has one.
type notWiredRecorder struct {
	mu   sync.Mutex
	w    runview.ReasonWriter
	clk  clock.Clock
	rows []runview.Reason
}

func newNotWiredRecorder(w runview.ReasonWriter, clk clock.Clock) *notWiredRecorder {
	return &notWiredRecorder{w: w, clk: clk}
}

// record notes that job is left out for want of the key named key. A nil recorder ignores it.
func (n *notWiredRecorder) record(job, key string) {
	if n == nil {
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	var at time.Time
	if n.clk != nil {
		at = n.clk.Now()
	}
	n.rows = append(n.rows, runview.Reason{
		Day: runview.NotWiredDay, Dataset: runview.NotWiredDataset, Subject: subjectNotWired,
		Scope: job, Code: codeNotWired, Detail: key, At: at,
	})
}

// flush replaces the whole set with the rows of this startup: empty when every job is wired, so a job left out
// before and wired now has no row. A nil recorder or one without a writer does nothing.
func (n *notWiredRecorder) flush(ctx context.Context) error {
	if n == nil || n.w == nil {
		return nil
	}
	n.mu.Lock()
	rows := append([]runview.Reason{}, n.rows...)
	n.mu.Unlock()
	if err := n.w.ReplaceReasons(ctx, runview.NotWiredDay, runview.NotWiredDataset, subjectNotWired, rows); err != nil {
		return fmt.Errorf("%s: %w", notWiredLogLabel, err)
	}
	return nil
}
