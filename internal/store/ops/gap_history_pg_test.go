//go:build pg

package ops

import (
	"context"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/store"
)

func TestGapsSincePG(t *testing.T) {
	t.Parallel()
	name := newSchedulerDatabase(t)
	ctx := context.Background()
	runner, err := OpenSchedulerLedger(ctx, connURL(t, name, "ot_runner"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = runner.Close() })
	open := store.Gap{DatasetID: "obs.series_point", Day: "2026-11-05", Reason: "expired"}
	cleared := store.Gap{DatasetID: "obs.option_eod", Day: "2026-11-04", Reason: "partial"}
	for _, g := range []store.Gap{open, cleared} {
		if err := runner.PutGap(ctx, g); err != nil {
			t.Fatal(err)
		}
	}
	if err := runner.ClearGap(ctx, cleared); err != nil {
		t.Fatal(err)
	}
	r, err := OpenGapHistory(ctx, connURL(t, name, "ot_ro_pages"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close() })
	got, err := r.GapsSince(ctx, time.Now().Add(-time.Hour))
	if err != nil || len(got) != 2 {
		t.Fatalf("got %+v, %v", got, err)
	}
	if got[0].DatasetID != "obs.series_point" && got[0].DatasetID != "obs.option_eod" {
		t.Errorf("rows = %+v", got)
	}
	var sawCleared bool
	for _, g := range got {
		if g.DatasetID == "obs.option_eod" {
			sawCleared = !g.ClearedAt.IsZero()
		}
	}
	if !sawCleared {
		t.Errorf("cleared row lacks ClearedAt: %+v", got)
	}
	later, err := r.GapsSince(ctx, time.Now().Add(time.Hour))
	if err != nil || later == nil || len(later) != 0 {
		t.Errorf("later = %#v, %v", later, err)
	}
}
