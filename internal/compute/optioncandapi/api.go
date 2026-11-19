// Package optioncandapi declares the put rule evaluated in a currency. Declarations only.
// Design: docs/design/candidates-jp.md#3.
package optioncandapi

import "github.com/andrewrong/optdesk/internal/compute"

// CurrencyInput is the candidate input of one run and the currency of its chain.
// An empty Currency is USD.
type CurrencyInput struct {
	compute.CandidateInput
	Currency string
}

// CurrencyRule evaluates the candidate rows in the currency of the input. With an empty
// or USD currency it returns the rows of compute.OptionCandidates.Evaluate.
type CurrencyRule interface {
	EvaluateIn(in CurrencyInput, p compute.CandidateParams) []compute.Candidate
}

// NewCurrencyRuleFunc builds the rule.
type NewCurrencyRuleFunc func() CurrencyRule
