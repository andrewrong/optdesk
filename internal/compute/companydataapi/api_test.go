package companydataapi_test

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/companydataapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// fakeCalc answers fixed made-up quarters and a summary; an empty input gives empty slices.
type fakeCalc struct{}

func (fakeCalc) Quarters(facts []contracts.FundamentalFact) ([]companydataapi.Quarter, error) {
	if len(facts) == 0 {
		return []companydataapi.Quarter{}, nil
	}
	return []companydataapi.Quarter{
		{
			Label: "FY2025 Q4", FiscalYear: 2025, Number: 4, PeriodEnd: "2025-09-27",
			FreeCash:   companydataapi.Cell{Value: "70", Derived: true, Concept: "A - B"},
			NetIncome:  companydataapi.Cell{Value: "5", Restated: true, Concept: "NetIncomeLoss"},
			EPSDiluted: companydataapi.Cell{Missing: true, Why: companydataapi.WhyShareBasisChanged},
		},
		{Label: "FY2025 Q3", FiscalYear: 2025, Number: 3, PeriodEnd: "2025-06-28"},
	}, nil
}

func (fakeCalc) Summarize(in companydataapi.SummaryInput) (companydataapi.Summary, error) {
	if len(in.Facts) == 0 {
		return companydataapi.Summary{Missing: []companydataapi.Missing{}}, nil
	}
	return companydataapi.Summary{
		Statements: true, EPSQuarters: 1,
		Missing: []companydataapi.Missing{{Reason: companydataapi.ReasonNoNextReport, Text: "no report date"}},
	}, nil
}

// TestCalculatorContractFake: a caller reads FreeCash, Restated and Missing from a fake Calculator.
func TestCalculatorContractFake(t *testing.T) {
	t.Parallel()
	var c companydataapi.Calculator = fakeCalc{}
	facts := []contracts.FundamentalFact{{}}
	qs, err := c.Quarters(facts)
	if err != nil || len(qs) != 2 {
		t.Fatalf("Quarters = %+v, %v", qs, err)
	}
	if qs[0].FreeCash.Value != "70" || !qs[0].FreeCash.Derived || !qs[0].NetIncome.Restated {
		t.Fatalf("quarter = %+v", qs[0])
	}
	if e := qs[0].EPSDiluted; !e.Missing || e.Value != "" || e.Why != companydataapi.WhyShareBasisChanged {
		t.Fatalf("eps = %+v", e)
	}
	s, err := c.Summarize(companydataapi.SummaryInput{Market: "US", Facts: facts})
	if err != nil || !s.Statements || len(s.Missing) != 1 || s.Missing[0].Reason != companydataapi.ReasonNoNextReport {
		t.Fatalf("Summary = %+v, %v", s, err)
	}
}

// TestEveryMethodReturnsNonNil: an empty input gives non-nil slices.
func TestEveryMethodReturnsNonNil(t *testing.T) {
	t.Parallel()
	var c companydataapi.Calculator = fakeCalc{}
	qs, err := c.Quarters(nil)
	if err != nil || qs == nil {
		t.Fatalf("Quarters = %#v, %v", qs, err)
	}
	s, err := c.Summarize(companydataapi.SummaryInput{})
	if err != nil || s.Missing == nil {
		t.Fatalf("Summary = %#v, %v", s, err)
	}
}

// TestWhyCodesAreFive: the five codes and their texts.
func TestWhyCodesAreFive(t *testing.T) {
	t.Parallel()
	got := []companydataapi.Why{
		companydataapi.WhyNone, companydataapi.WhyNoFact, companydataapi.WhyCannotDerive,
		companydataapi.WhyPeriodMismatch, companydataapi.WhyShareBasisChanged,
	}
	want := []string{"", "no_fact", "cannot_derive", "period_mismatch", "share_basis_changed"}
	seen := map[companydataapi.Why]bool{}
	for i, w := range got {
		if string(w) != want[i] {
			t.Fatalf("code %d = %q, want %q", i, w, want[i])
		}
		seen[w] = true
	}
	if len(seen) != 5 {
		t.Fatalf("distinct codes = %d", len(seen))
	}
}
