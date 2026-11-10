package valuation

import (
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/valuationapi"
)

func figuresByKey(t *testing.T, r valuationapi.DiagnosisResult) map[string]valuationapi.DiagnosisFigure {
	t.Helper()
	m := map[string]valuationapi.DiagnosisFigure{}
	for _, f := range r.Figures {
		if _, dup := m[f.Key]; dup {
			t.Fatalf("key %s twice", f.Key)
		}
		m[f.Key] = f
	}
	return m
}

func TestDiagnosisGolden(t *testing.T) {
	t.Parallel()
	for _, c := range loadQuestionGolden(t, "diagnosis") {
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			res := Diagnosis(filingsOfFigures(c.Input.Figures, c.Input.SIC), "2025-03-01")
			want := c.Expected
			if res.Group != strings.ToLower(want.Kind) {
				t.Fatalf("group %q, want %q", res.Group, strings.ToLower(want.Kind))
			}
			primary, secondary, scale := "operating_margin", "inventory_to_revenue", "revenue"
			if res.Group == "financial" {
				primary, secondary, scale = "roe", "roa", "equity"
			}
			got := figuresByKey(t, res)
			for _, r := range want.Rows {
				sameNumber(t, primary+"@"+r.PeriodEnd, got[primary+"@"+r.PeriodEnd].Value, r.Primary)
				sameNumber(t, secondary+"@"+r.PeriodEnd, got[secondary+"@"+r.PeriodEnd].Value, r.Secondary)
				sameNumber(t, scale+"@"+r.PeriodEnd, got[scale+"@"+r.PeriodEnd].Value, r.Scale)
			}
			sameNumber(t, "ranked", got["ranked"].Value, jsonInt(want.Summary.Ranked))
			sameNumber(t, "loss_years", got["loss_years"].Value, jsonInt(want.Summary.LossYears))
			sameNumber(t, "record_years", got["record_years"].Value, jsonInt(want.Summary.RecordYears))
			stats := []string{"latest_rank", primary + "_median", secondary + "_median", secondary + "_latest"}
			if res.Group == "cyclical" {
				stats = append(stats, "revenue_vs_peak")
			}
			if want.Status == "TOO_SHORT" {
				// Too few years to read: the statistics are missing_input, the rows stay.
				for _, k := range stats {
					if f := got[k]; f.State != valuationapi.StateMissingInput || f.Reason != "too_short" || f.Value != "" {
						t.Errorf("%s: %+v, want missing_input too_short", k, f)
					}
				}
				return
			}
			sameNumber(t, "latest_rank", got["latest_rank"].Value, jsonInt(want.Summary.LatestRank))
			sameNumber(t, primary+"_median", got[primary+"_median"].Value, want.Summary.PrimaryMedian)
			sameNumber(t, secondary+"_median", got[secondary+"_median"].Value, want.Summary.SecondaryMedian)
			sameNumber(t, secondary+"_latest", got[secondary+"_latest"].Value, want.Summary.SecondaryLatest)
			if res.Group == "cyclical" {
				sameNumber(t, "revenue_vs_peak", got["revenue_vs_peak"].Value, want.Summary.RevenueVsPeak)
			}
		})
	}
}

// A company in neither list has no diagnosis group, and a year whose figure cannot be
// formed is missing_input, never zero.
func TestDiagnosisGroupAndGaps(t *testing.T) {
	t.Parallel()
	var cyclical, financial goldenQuestionCase
	for _, c := range loadQuestionGolden(t, "diagnosis") {
		switch c.Expected.Kind {
		case "CYCLICAL":
			cyclical = c
		case "FINANCIAL":
			financial = c
		}
	}
	if r := Diagnosis(filingsOfFigures(cyclical.Input.Figures, "7372"), "2025-03-01"); r.Group != "" || len(r.Figures) != 0 {
		t.Errorf("software SIC: %+v", r)
	}
	if r := Diagnosis(filingsOfFigures(cyclical.Input.Figures, "6798"), "2025-03-01"); r.Group != "" {
		t.Errorf("REIT SIC 6798 is in no list: %q", r.Group)
	}
	// Without the opening balance of the first fiscal year its ROE and ROA cannot be formed.
	figs := map[string]map[string]string{"net_income": financial.Input.Figures["net_income"]}
	for _, k := range []string{"equity", "assets"} {
		figs[k] = map[string]string{}
		for d, v := range financial.Input.Figures[k] {
			if d != "2014-12-31" {
				figs[k][d] = v
			}
		}
	}
	got := figuresByKey(t, Diagnosis(filingsOfFigures(figs, "6022"), "2025-03-01"))
	for _, k := range []string{"roe@2015-12-31", "roa@2015-12-31"} {
		if f := got[k]; f.State != valuationapi.StateMissingInput || f.Value != "" || f.Reason == "" {
			t.Errorf("%s: %+v", k, f)
		}
	}
	if f := got["roe@2016-12-31"]; f.State != valuationapi.StateOK {
		t.Errorf("roe@2016-12-31: %+v", f)
	}
	if f := got["ranked"]; f.Value != "9" {
		t.Errorf("ranked %q, want 9", f.Value)
	}
}

var _ valuationapi.DiagnosisFunc = Diagnosis
