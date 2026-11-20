package market_state_page

import (
	"math"
	"testing"
)

// ramp is n points 1, 2, ... n.
func ramp(n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = float64(i + 1)
	}
	return out
}

func asRange(t *testing.T, v any) rangeState {
	t.Helper()
	r, ok := v.(rangeState)
	if !ok {
		t.Fatalf("range = %#v, want a range", v)
	}
	return r
}

func asGap(t *testing.T, v any) gap {
	t.Helper()
	g, ok := v.(gap)
	if !ok {
		t.Fatalf("range = %#v, want a Gap", v)
	}
	return g
}

func TestRangeVixPublishedCuts(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		fig   float64
		state string
	}{{15, "normal"}, {20, "normal"}, {25, "caution"}, {30, "caution"}, {35, "alarm"}} {
		r := asRange(t, rangeOf("vix", c.fig, nil))
		if r.State != c.state {
			t.Errorf("vix %v: state %q, want %q", c.fig, r.State, c.state)
		}
	}
	r := asRange(t, rangeOf("vix", 25, nil))
	want := []string{"Caution when above 20.0 index points; published trial reading",
		"Alarm when above 30.0 index points; published trial reading"}
	if len(r.Cuts) != 2 || r.Cuts[0].Text != want[0] || r.Cuts[1].Text != want[1] {
		t.Errorf("cuts = %+v", r.Cuts)
	}
	if r.Cuts[0].Basis != "published" || r.Cuts[0].Source != "published; source A Cboe, B S&P Dow Jones Indices" {
		t.Errorf("cut 0 = %+v", r.Cuts[0])
	}
	if r.StateText != "Caution, trial" || r.Notice != "A notice only; nothing is blocked." {
		t.Errorf("texts = %q, %q", r.StateText, r.Notice)
	}
}

func TestRangeSp500Below200d(t *testing.T) {
	t.Parallel()
	pts := ramp(300) // the 5th percentile is 15
	for i := range pts {
		pts[i] -= 100 // -99 ... 200; the 5th percentile is -85
	}
	if r := asRange(t, rangeOf("sp500_vs_200d", 3.12, pts)); r.State != "normal" {
		t.Errorf("above the line: %q", r.State)
	}
	r := asRange(t, rangeOf("sp500_vs_200d", -90, pts))
	if r.State != "alarm" {
		t.Errorf("below the line and the percentile: %q", r.State)
	}
	// caution only: under 0, above the percentile cut
	r = asRange(t, rangeOf("sp500_vs_200d", -0.2, pts))
	if r.State != "caution" {
		t.Errorf("under the line only: %q", r.State)
	}
	if r.Cuts[0].Text != "Caution when below 0.00%; published trial reading" || r.Cuts[0].Source != "published; source C Faber 2007" {
		t.Errorf("cut 0 = %+v", r.Cuts[0])
	}
}

func TestRangePercentileNearestRank(t *testing.T) {
	t.Parallel()
	// 300 points, unsorted input: rank ceil(0.8*300)=240 -> 240, rank ceil(0.95*300)=285 -> 285
	pts := ramp(300)
	pts[0], pts[299] = pts[299], pts[0]
	r := asRange(t, rangeOf("vix_term", 250, pts))
	if r.State != "caution" {
		t.Errorf("state %q", r.State)
	}
	if r.Cuts[0].Text != "Caution when above 240.00, the 80th percentile of 300 days" ||
		r.Cuts[1].Text != "Alarm when above 285.00, the 95th percentile of 300 days" {
		t.Errorf("cuts = %+v", r.Cuts)
	}
	if r.Cuts[0].Basis != "percentile" || r.Cuts[0].Source != "own stored history" {
		t.Errorf("cut 0 = %+v", r.Cuts[0])
	}
	// 250 points: rank ceil(0.8*250)=200 exactly
	r = asRange(t, rangeOf("vix_term", 200, ramp(250)))
	if r.State != "normal" || r.Cuts[0].Text != "Caution when above 200.00, the 80th percentile of 250 days" {
		t.Errorf("250 points: %+v", r)
	}
}

func TestRangeShortHistoryIsGap(t *testing.T) {
	t.Parallel()
	for _, id := range []string{"sp500_vs_200d", "sp500_drawdown", "vix_term", "hy_spread"} {
		g := asGap(t, rangeOf(id, 1, ramp(249)))
		if g.Kind != "gap" || g.Reason != "no_data" || g.Text == "" {
			t.Errorf("%s: %+v", id, g)
		}
	}
	// vix has published cuts only: a short history does not matter
	if r := asRange(t, rangeOf("vix", 12, nil)); r.State != "normal" {
		t.Errorf("vix: %q", r.State)
	}
}

func TestRangeGapFigureIsGap(t *testing.T) {
	t.Parallel()
	for _, id := range []string{"sp500_vs_200d", "sp500_drawdown", "vix", "vix_term", "hy_spread"} {
		g := asGap(t, rangeOf(id, math.NaN(), ramp(300)))
		if g.Reason != "no_data" {
			t.Errorf("%s: %+v", id, g)
		}
	}
}

func TestRangeUnknownRowHasNoRange(t *testing.T) {
	t.Parallel()
	if v := rangeOf("nope", 1, ramp(300)); v != nil {
		t.Errorf("range = %#v", v)
	}
	if hintOf("nope") != "" {
		t.Error("hint of an unknown row")
	}
}

func TestRangeDirectionBothSides(t *testing.T) {
	t.Parallel()
	pts := ramp(1000) // p5=50 p20=200 p80=800 p95=950
	cases := []struct {
		id    string
		fig   float64
		state string
	}{
		{"sp500_drawdown", 500, "normal"}, {"sp500_drawdown", 150, "caution"}, {"sp500_drawdown", 20, "alarm"},
		{"sp500_drawdown", 2000, "normal"},
		{"vix_term", 500, "normal"}, {"vix_term", 900, "caution"}, {"vix_term", 990, "alarm"},
		{"vix_term", -5, "normal"},
		{"hy_spread", 500, "normal"}, {"hy_spread", 900, "caution"}, {"hy_spread", 990, "alarm"},
		{"hy_spread", -5, "normal"},
		{"sp500_vs_200d", 500, "normal"}, {"sp500_vs_200d", -5, "alarm"}, {"sp500_vs_200d", 60, "normal"}, {"sp500_vs_200d", 20, "alarm"},
		{"vix", 10, "normal"}, {"vix", 25, "caution"}, {"vix", 31, "alarm"},
	}
	for _, c := range cases {
		if r := asRange(t, rangeOf(c.id, c.fig, pts)); r.State != c.state {
			t.Errorf("%s %v: %q, want %q", c.id, c.fig, r.State, c.state)
		}
	}
}

func TestRangeDoesNotChangeRunStatus(t *testing.T) {
	t.Parallel()
	// the sample histories hold two points, so every percentile row has a Gap range
	m := build(t, fullConfig(t, sampleResult()))
	rows := marketRows(m)
	gaps := 0
	for _, r := range rows {
		if isGap(obj(r)["range"]) {
			gaps++
		}
	}
	if gaps != 4 {
		t.Fatalf("%d Gap ranges, want 4", gaps)
	}
	if m["run_status"] != "ok" {
		t.Errorf("run_status = %v", m["run_status"])
	}
	if g := list(m["gaps"]); len(g) != 0 {
		t.Errorf("page gaps = %v", g)
	}
}
