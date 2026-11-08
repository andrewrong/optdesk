package figures_test

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
)

func TestUnfinishedAreMissing(t *testing.T) {
	t.Parallel()
	f := build(compute.Rule{ID: "iv", Version: 3})
	got := []compute.Figure{
		f.Iv30("iv", nil, compute.Point{}, "2024-01-02"),
		f.IvPercentile("iv", nil, "2024-01-02"),
		f.IvRank("iv", nil, "2024-01-02"),
	}
	for i, g := range got {
		if g.Status != compute.StatusMissing || g.Reason != compute.BadInput || g.ID != "iv" || g.Version != 3 || g.Day != "2024-01-02" {
			t.Errorf("method %d: %+v", i, g)
		}
	}
}
