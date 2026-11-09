package optioncandidates

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/store"
)

func TestFloatToDecimal(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		in   float64
		want string
	}{{1.99, "1.99"}, {0.1, "0.1"}, {100.00049, "100.0005"}, {100.00004, "100"}, {1e-7, "0"},
		{0.00005, "0"}, {0.00015, "0.0002"}, {-0.25, "-0.25"}, {-1e-7, "0"}, {12, "12"}} {
		if got := FloatToDecimal(c.in); got != c.want {
			t.Errorf("FloatToDecimal(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestRateRatio(t *testing.T) {
	t.Parallel()
	if got, ok := RateRatio("4.20"); !ok || got != "0.042" {
		t.Fatalf("RateRatio(4.20) = %q, %v", got, ok)
	}
	for _, bad := range []string{"", "x", "-1", "101"} {
		if _, ok := RateRatio(bad); ok {
			t.Errorf("RateRatio(%q) accepted", bad)
		}
	}
}

func TestRankRunOrdersAcrossUnderlyings(t *testing.T) {
	t.Parallel()
	y := func(s string) *string { return &s }
	rows := []store.Candidate{
		{Status: "ok", AnnualizedYield: y("0.10"), DTE: 30, Strike: "50", InstrumentID: 1},
		{Status: "blocked", AnnualizedYield: y("0.90"), DTE: 30, Strike: "50", InstrumentID: 2},
		{Status: "ok", AnnualizedYield: y("0.20"), DTE: 30, Strike: "50", InstrumentID: 3},
	}
	rankRun(rows)
	if *rows[2].Rank != 1 || *rows[0].Rank != 2 || rows[1].Rank != nil {
		t.Fatalf("ranks %v %v %v", rows[0].Rank, rows[1].Rank, rows[2].Rank)
	}
}
