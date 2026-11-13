package optionfig

import (
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/optionfigapi"
)

func base() optionfigapi.PutFigureInput {
	return optionfigapi.PutFigureInput{
		ChainDay: "2026-11-13", DTE: 30, Spot: "100", Strike: "95", IV: "0.30",
		AnnualYield: "0.09", Rate: "4.5", RateDay: "2026-11-11", MaxRateAgeDays: 7,
	}
}

func TestPutValues(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		mod    func(*optionfigapi.PutFigureInput)
		itm    string
		itmKey string
		ex     string
		above  bool
		exKey  string
	}{
		{"otm put", func(*optionfigapi.PutFigureInput) {}, "27.5%", "2755", "4.5%", true, "450"},
		{"atm zero rate", func(in *optionfigapi.PutFigureInput) {
			in.Strike, in.Rate = "100", "0"
		}, "51.7%", "5172", "9.0%", true, "900"},
		{"below cash", func(in *optionfigapi.PutFigureInput) {
			in.AnnualYield, in.Rate = "0.03", "4"
		}, "", "", "1.0%", false, "-100"},
		{"equal to cash", func(in *optionfigapi.PutFigureInput) {
			in.AnnualYield, in.Rate = "0.04", "4"
		}, "", "", "0.0%", true, "0"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			in := base()
			c.mod(&in)
			got := New().Put(in)
			if got.ItmGap != "" || got.ExcessGap != "" {
				t.Fatalf("unexpected gaps: %+v", got)
			}
			if c.itm != "" && got.ItmPercent != c.itm {
				t.Errorf("ItmPercent = %q, want %q", got.ItmPercent, c.itm)
			}
			if c.itmKey != "" && got.ItmKey != c.itmKey {
				t.Errorf("ItmKey = %q, want %q", got.ItmKey, c.itmKey)
			}
			if got.ExcessPercent != c.ex || got.ExcessAbove != c.above || got.ExcessKey != c.exKey {
				t.Errorf("excess = %q %v %q, want %q %v %q", got.ExcessPercent, got.ExcessAbove, got.ExcessKey, c.ex, c.above, c.exKey)
			}
		})
	}
}

func TestPutGaps(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		mod      func(*optionfigapi.PutFigureInput)
		itmGap   string
		exGap    string
		mention  string
		itmValue bool // ITM is still computed
	}{
		{"empty iv", func(in *optionfigapi.PutFigureInput) { in.IV = "" }, gapMissing, "", "IV", false},
		{"zero iv", func(in *optionfigapi.PutFigureInput) { in.IV = "0" }, gapMissing, "", "IV", false},
		{"bad strike", func(in *optionfigapi.PutFigureInput) { in.Strike = "abc" }, gapMissing, "", "strike", false},
		{"zero spot", func(in *optionfigapi.PutFigureInput) { in.Spot = "0" }, gapMissing, "", "spot", false},
		{"zero dte", func(in *optionfigapi.PutFigureInput) { in.DTE = 0 }, gapMissing, "", "DTE", false},
		{"no rate", func(in *optionfigapi.PutFigureInput) { in.Rate, in.RateDay = "", "" }, gapMissing, gapMissing, "rate", false},
		{"stale rate", func(in *optionfigapi.PutFigureInput) { in.RateDay = "2026-11-04" }, gapStale, gapStale, "9 days", false},
		{"no yield", func(in *optionfigapi.PutFigureInput) { in.AnnualYield = "" }, "", gapMissing, "yield", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			in := base()
			c.mod(&in)
			got := New().Put(in)
			if got.ItmGap != c.itmGap || got.ExcessGap != c.exGap {
				t.Fatalf("gaps = %q/%q, want %q/%q", got.ItmGap, got.ExcessGap, c.itmGap, c.exGap)
			}
			if c.itmGap != "" {
				if got.ItmPercent != "" || got.ItmKey != "" || !strings.Contains(got.ItmWhy, c.mention) {
					t.Errorf("itm gap fields = %+v", got)
				}
			} else if got.ItmPercent != "27.5%" || got.ItmWhy != "" {
				t.Errorf("itm = %+v", got)
			}
			if c.exGap != "" {
				if got.ExcessPercent != "" || got.ExcessKey != "" || got.ExcessAbove || !strings.Contains(got.ExcessWhy, c.mention) {
					t.Errorf("excess gap fields = %+v", got)
				}
			} else if got.ExcessPercent == "" || got.ExcessWhy != "" {
				t.Errorf("excess = %+v", got)
			}
		})
	}
}
