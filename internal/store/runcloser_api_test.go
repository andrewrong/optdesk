package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/store"
)

// fakeRunCloser implements RunCloser over a set of run statuses.
type fakeRunCloser struct {
	runs map[string]store.Run
}

func (f *fakeRunCloser) CloseRunning(_ context.Context, now time.Time, reason string) (int, error) {
	n := 0
	for id, r := range f.runs {
		if r.Status != store.RunRunning {
			continue
		}
		r.Status, r.ReasonCode, r.FinishedAt = store.RunDone, reason, now
		f.runs[id] = r
		n++
	}
	return n, nil
}

// closeAll is the caller side: it holds only the RunCloser interface.
func closeAll(ctx context.Context, c store.RunCloser, now time.Time) (int, error) {
	return c.CloseRunning(ctx, now, "crashed")
}

func TestRunCloserClosesEveryRunningRun(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 4, 12, 0, 0, 0, time.UTC)
	f := &fakeRunCloser{runs: map[string]store.Run{
		"a": {ID: "a", Status: store.RunRunning},
		"b": {ID: "b", Status: store.RunRunning},
		"c": {ID: "c", Status: store.RunDone},
	}}
	n, err := closeAll(context.Background(), f, now)
	if err != nil || n != 2 {
		t.Fatalf("closed %d, err %v; want 2, nil", n, err)
	}
	if got := f.runs["a"]; got.Status != store.RunDone || got.ReasonCode != "crashed" || !got.FinishedAt.Equal(now) {
		t.Fatalf("run a = %+v", got)
	}
	if f.runs["c"].ReasonCode != "" {
		t.Fatalf("done run changed: %+v", f.runs["c"])
	}
}
