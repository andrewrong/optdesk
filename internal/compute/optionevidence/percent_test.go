package optionevidence

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/optionevidenceapi"
)

type percentFile struct {
	Cases []struct{ In, Out string }
	Row   optionevidenceapi.Row
	Want  optionevidenceapi.Evidence
}

func loadPercent(t *testing.T) percentFile {
	t.Helper()
	raw, err := os.ReadFile("testdata/percent.json")
	if err != nil {
		t.Fatal(err)
	}
	var f percentFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestPctTextFractionTimes100(t *testing.T) {
	t.Parallel()
	for _, c := range loadPercent(t).Cases {
		if c.In == "" || c.In == "abc" {
			continue
		}
		if got := pctText(c.In); got != c.Out {
			t.Errorf("pctText(%q) = %q, want %q", c.In, got, c.Out)
		}
	}
}

func TestPctTextEmptyOrBad(t *testing.T) {
	t.Parallel()
	for _, in := range []string{"", "abc"} {
		if got := pctText(in); got != "" {
			t.Errorf("pctText(%q) = %q", in, got)
		}
	}
}

func TestEvaluateFillsPercents(t *testing.T) {
	t.Parallel()
	f := loadPercent(t)
	e := New(nil, DefaultParams())
	got, err := e.Evaluate(f.Row)
	if err != nil {
		t.Fatal(err)
	}
	w := f.Want
	if got.SkewPct != w.SkewPct || got.ATMIVPct != w.ATMIVPct || got.ForecastPct != w.ForecastPct ||
		got.ForecastErrPct != w.ForecastErrPct || got.IVPPct != w.IVPPct || got.VolEdgePct != w.VolEdgePct {
		t.Errorf("percents: %+v", got)
	}
	noATM := f.Row
	noATM.ATMIV, noATM.ATMStrike = "", ""
	got, err = e.Evaluate(noATM)
	if err != nil {
		t.Fatal(err)
	}
	if got.SkewPct != "" || got.ATMIVPct != "" {
		t.Errorf("no ATM: SkewPct %q ATMIVPct %q", got.SkewPct, got.ATMIVPct)
	}
}
