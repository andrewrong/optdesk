package valuation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/compute/valuationapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// goldenQuestionCase is one case of testdata/golden_questions.json. Input.Figures maps the
// concept names of the old parameter files to a figure per date.
type goldenQuestionCase struct {
	Name  string `json:"name"`
	Input struct {
		Kind    string                       `json:"kind"`
		SIC     string                       `json:"sic"`
		Figures map[string]map[string]string `json:"figures"`
	} `json:"input"`
	Expected struct {
		Status string `json:"status"`
		Years  int    `json:"years"`
		Q1     *struct {
			Mean, Worst string
			CappedYears int `json:"capped_years"`
		} `json:"q1"`
		Q2 *struct {
			Value      *string `json:"value"`
			NewCapital string  `json:"new_capital"`
			NewNopat   string  `json:"new_nopat"`
			NoCapital  bool    `json:"no_capital"`
		} `json:"q2"`
		Q3 *struct{ Basis, First, Last string } `json:"q3"`
		Q4 *struct{ Value *string }             `json:"q4"`
		// Diagnosis cases.
		Kind string `json:"kind"`
		Rows []struct {
			PeriodEnd string `json:"period_end"`
			Primary   string
			Secondary string
			Scale     string
		} `json:"rows"`
		Summary struct {
			Ranked          int
			LatestRank      int    `json:"latest_rank"`
			PrimaryMedian   string `json:"primary_median"`
			SecondaryMedian string `json:"secondary_median"`
			SecondaryLatest string `json:"secondary_latest"`
			RevenueVsPeak   string `json:"revenue_vs_peak"`
			LossYears       int    `json:"loss_years"`
			RecordYears     int    `json:"record_years"`
		} `json:"summary"`
	} `json:"expected"`
}

func loadQuestionGolden(t *testing.T, kind string) []goldenQuestionCase {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "golden_questions.json"))
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Cases []goldenQuestionCase `json:"cases"`
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	var out []goldenQuestionCase
	for _, c := range f.Cases {
		if c.Input.Kind == kind {
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		t.Fatalf("no %s golden cases", kind)
	}
	return out
}

// goldenRoles maps the concept names of the old parameter files to the role names a
// FilingYear carries; the others are the same.
var goldenRoles = map[string]string{
	"ocf": roleOperatingCashFlow, "capex": roleCapitalSpending, "sbc": roleStockCompensation,
}

// filingsOfFigures builds Filings from figures by date: one year per date, newest first.
func filingsOfFigures(figs map[string]map[string]string, sic string) valuationapi.Filings {
	byDate := map[string]map[string]string{}
	for name, s := range figs {
		role := name
		if r, ok := goldenRoles[name]; ok {
			role = r
		}
		for d, v := range s {
			if byDate[d] == nil {
				byDate[d] = map[string]string{}
			}
			byDate[d][role] = v
		}
	}
	var days []string
	for d := range byDate {
		days = append(days, d)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(days)))
	f := valuationapi.Filings{Standard: valuationapi.USGAAP, Currency: "USD", SIC: sic}
	for _, d := range days {
		f.Years = append(f.Years, valuationapi.FilingYear{FYEnd: contracts.Day(d), Values: byDate[d]})
	}
	return f
}

// sameNumber compares decimal texts by value: the old system kept trailing zeros that a
// canonical text drops.
func sameNumber(t *testing.T, what, got, want string) {
	t.Helper()
	g, err := decimal.NewFromString(got)
	if err != nil {
		t.Errorf("%s: %q is not a decimal (want %s)", what, got, want)
		return
	}
	if !g.Equal(decimal.RequireFromString(want)) {
		t.Errorf("%s: %s, want %s", what, got, want)
	}
}

func byKey(t *testing.T, r valuationapi.QuestionsResult) map[string]valuationapi.Question {
	t.Helper()
	m := map[string]valuationapi.Question{}
	for _, q := range r.Questions {
		if _, dup := m[q.Key]; dup {
			t.Fatalf("key %s twice", q.Key)
		}
		m[q.Key] = q
	}
	return m
}

func TestQuestionsGolden(t *testing.T) {
	t.Parallel()
	for _, c := range loadQuestionGolden(t, "five_questions") {
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			got := byKey(t, Questions(filingsOfFigures(c.Input.Figures, "7372"), "2025-03-01"))
			want := c.Expected
			if want.Status != "OK" {
				for _, k := range []string{"Q1", "Q2", "Q3", "Q4"} {
					q := got[k]
					if q.State != valuationapi.StateMissingInput || q.Reason != "not_enough_years" || q.Value != "" {
						t.Errorf("%s: %+v, want missing_input not_enough_years", k, q)
					}
				}
				return
			}
			sameNumber(t, "Q1", got["Q1"].Value, want.Q1.Mean)
			sameNumber(t, "Q1.worst", got["Q1.worst"].Value, want.Q1.Worst)
			sameNumber(t, "Q1.capped_years", got["Q1.capped_years"].Value, jsonInt(want.Q1.CappedYears))
			sameNumber(t, "Q2.new_capital", got["Q2.new_capital"].Value, want.Q2.NewCapital)
			sameNumber(t, "Q2.new_nopat", got["Q2.new_nopat"].Value, want.Q2.NewNopat)
			if want.Q2.Value == nil {
				if q := got["Q2"]; q.State != valuationapi.StateMissingInput || q.Value != "" || q.Reason == "" {
					t.Errorf("Q2: %+v, want missing_input with a reason", q)
				}
			} else {
				sameNumber(t, "Q2", got["Q2"].Value, *want.Q2.Value)
			}
			if _, ok := got["Q3.basis."+want.Q3.Basis]; !ok {
				t.Errorf("Q3 basis %s not named", want.Q3.Basis)
			}
			sameNumber(t, "Q3", got["Q3"].Value, want.Q3.Last)
			sameNumber(t, "Q3.first", got["Q3.first"].Value, want.Q3.First)
			sameNumber(t, "Q4", got["Q4"].Value, *want.Q4.Value)
			for k, q := range got {
				if q.State != valuationapi.StateOK && q.State != valuationapi.StateMissingInput {
					t.Errorf("%s: state %q", k, q.State)
				}
			}
		})
	}
}

func jsonInt(n int) string { return decimal.NewFromInt(int64(n)).String() }

func without(figs map[string]map[string]string, role string) map[string]map[string]string {
	out := map[string]map[string]string{}
	for k, v := range figs {
		if k != role {
			out[k] = v
		}
	}
	return out
}

func wantMissing(t *testing.T, got map[string]valuationapi.Question, keys ...string) {
	t.Helper()
	for _, k := range keys {
		if q := got[k]; q.State != valuationapi.StateMissingInput || q.Value != "" || q.Reason == "" {
			t.Errorf("%s: %+v, want missing_input with a reason and no value", k, q)
		}
	}
}

// A figure that cannot be formed is missing_input with a reason and no value, never zero.
func TestMissingInputNeverZero(t *testing.T) {
	t.Parallel()
	strong := loadQuestionGolden(t, "five_questions")[0].Input.Figures
	run := func(figs map[string]map[string]string) map[string]valuationapi.Question {
		return byKey(t, Questions(filingsOfFigures(figs, "7372"), "2025-03-01"))
	}

	got := run(without(strong, "ppe"))
	wantMissing(t, got, "Q1", "Q2")
	if got["Q4"].State != valuationapi.StateOK {
		t.Errorf("Q4 does not need capital: %+v", got["Q4"])
	}
	wantMissing(t, run(without(strong, "ocf")), "Q4")
	wantMissing(t, run(without(strong, "revenue")), "Q3", "Q3.first")

	neg := map[string]string{}
	for d, v := range strong["net_income"] {
		neg[d] = "-" + v
	}
	loss := without(strong, "net_income")
	loss["net_income"] = neg
	wantMissing(t, run(loss), "Q4")

	wantMissing(t, byKey(t, Questions(valuationapi.Filings{}, "2025-03-01")), "Q1", "Q2", "Q3", "Q4")
}

var _ valuationapi.QuestionsFunc = Questions
