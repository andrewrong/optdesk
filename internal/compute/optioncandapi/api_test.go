package optioncandapi_test

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/optioncandapi"
)

// fakeRule answers one row that names the currency it was asked for.
type fakeRule struct{}

func (fakeRule) EvaluateIn(in optioncandapi.CurrencyInput, _ compute.CandidateParams) []compute.Candidate {
	return []compute.Candidate{{Underlying: in.Underlying, Status: in.Currency}}
}

func TestCurrencyRuleContractFake(t *testing.T) {
	t.Parallel()
	var newRule optioncandapi.NewCurrencyRuleFunc = func() optioncandapi.CurrencyRule { return fakeRule{} }
	in := optioncandapi.CurrencyInput{CandidateInput: compute.CandidateInput{Underlying: "7203"}, Currency: "JPY"}
	rows := newRule().EvaluateIn(in, compute.CandidateParams{})
	if len(rows) != 1 || rows[0].Underlying != "7203" || rows[0].Status != "JPY" {
		t.Fatalf("rows = %+v", rows)
	}
}

// fakeNameRule answers the number of facts as the Failed count.
type fakeNameRule struct{}

func (fakeNameRule) Classify(f optioncandapi.NameOpenFacts) optioncandapi.NameOpen {
	if len(f.Facts) == 0 {
		return optioncandapi.NameOpen{Outcome: optioncandapi.OutcomeNoRows}
	}
	return optioncandapi.NameOpen{Outcome: optioncandapi.OutcomeFailed, Failed: len(f.Facts)}
}

func TestNameOpenRuleContractFake(t *testing.T) {
	t.Parallel()
	var newRule optioncandapi.NewNameOpenRuleFunc = func() optioncandapi.NameOpenRule { return fakeNameRule{} }
	got := newRule().Classify(optioncandapi.NameOpenFacts{Name: "2408", Facts: []optioncandapi.OpenFact{{Status: 6}, {Status: 8}}})
	if got.Outcome != optioncandapi.OutcomeFailed || got.Failed != 2 {
		t.Fatalf("got %+v", got)
	}
	if got := newRule().Classify(optioncandapi.NameOpenFacts{Name: "x"}); got.Outcome != optioncandapi.OutcomeNoRows {
		t.Fatalf("empty facts: %+v", got)
	}
}
