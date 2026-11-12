package figures_test

import (
	"fmt"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

func combiner() compute.Figures {
	return build(
		compute.Rule{ID: "blend", Version: 1, Convention: "vol_blend_v1", Unit: "decimal", Scale: 6, MaxAgeDays: 4},
		compute.Rule{ID: "diff", Version: 1, Convention: "vol_diff_v1", Unit: "decimal", Scale: 6, MaxAgeDays: 4},
	)
}

func part(v float64, dataDay string) compute.Figure {
	return compute.Figure{ID: "p", Version: 1, Day: contracts.Day(dataDay), DataDay: contracts.Day(dataDay),
		Status: compute.StatusOK, Value: v, Observations: 1}
}

func wantValue(t *testing.T, got compute.Figure, want string) {
	t.Helper()
	if got.Status != compute.StatusOK {
		t.Fatalf("status %v reason %v, want ok", got.Status, got.Reason)
	}
	if s := fmt.Sprintf("%.6f", got.Value); s != want {
		t.Errorf("value %s, want %s", s, want)
	}
}

func TestCombineBlendMean(t *testing.T) {
	t.Parallel()
	got := combiner().Combine("blend", []compute.Figure{part(0.20, "2026-11-06"), part(0.30, "2026-11-06"), part(0.40, "2026-11-06")}, "2026-11-06")
	wantValue(t, got, "0.300000")
	if got.ID != "blend" || got.Version != 1 {
		t.Errorf("id/version %s/%d", got.ID, got.Version)
	}
}

func TestCombineDiffSign(t *testing.T) {
	t.Parallel()
	c := combiner()
	d := "2026-11-06"
	wantValue(t, c.Combine("diff", []compute.Figure{part(0.25, d), part(0.20, d)}, contracts.Day(d)), "0.050000")
	wantValue(t, c.Combine("diff", []compute.Figure{part(0.20, d), part(0.25, d)}, contracts.Day(d)), "-0.050000")
}

func TestCombineMissingPart(t *testing.T) {
	t.Parallel()
	d := "2026-11-06"
	gone := part(0, d)
	gone.Status, gone.Reason = compute.StatusMissing, compute.ShortHistory
	got := combiner().Combine("blend", []compute.Figure{part(0.2, d), gone, part(0.4, d)}, contracts.Day(d))
	if got.Status != compute.StatusMissing || got.Reason != compute.NoInput {
		t.Errorf("got %v %v, want missing no_input", got.Status, got.Reason)
	}
}

func TestCombineBadCount(t *testing.T) {
	t.Parallel()
	c := combiner()
	d := "2026-11-06"
	cases := map[string][]compute.Figure{
		"blend": {part(0.2, d)},
		"diff":  {part(0.2, d), part(0.3, d), part(0.4, d)},
	}
	for id, parts := range cases {
		got := c.Combine(id, parts, contracts.Day(d))
		if got.Status != compute.StatusMissing || got.Reason != compute.BadInput {
			t.Errorf("%s: got %v %v, want missing bad_input", id, got.Status, got.Reason)
		}
	}
	if got := c.Combine("nope", []compute.Figure{part(0.2, d), part(0.3, d)}, contracts.Day(d)); got.Reason != compute.BadInput {
		t.Errorf("unknown id: %v", got.Reason)
	}
}

func TestCombineDays(t *testing.T) {
	t.Parallel()
	c := combiner()
	got := c.Combine("diff", []compute.Figure{part(0.3, "2026-10-16"), part(0.2, "2026-11-04")}, "2026-11-06")
	wantValue(t, got, "0.100000")
	if got.Day != "2026-11-06" || got.DataDay != "2026-11-04" || got.Observations != 2 {
		t.Errorf("day %s data day %s observations %d", got.Day, got.DataDay, got.Observations)
	}
	// the newest part is five days old: more than max_age_days (4).
	got = c.Combine("diff", []compute.Figure{part(0.3, "2026-10-16"), part(0.2, "2026-11-01")}, "2026-11-06")
	if got.Status != compute.StatusMissing || got.Reason != compute.StaleInput {
		t.Errorf("got %v %v, want missing stale_input", got.Status, got.Reason)
	}
}
