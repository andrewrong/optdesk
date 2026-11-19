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
