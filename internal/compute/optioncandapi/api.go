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

// Outcomes of a name's open rows. The set is open: a caller must accept an unknown string.
// Design: docs/design/sellput-jp-outcomes.md#2-name-open-rule.
const (
	OutcomeNoQuote     = "no_quote"     // the exchange has no quote for the contracts
	OutcomeFailed      = "failed"       // a fetch that did not finish
	OutcomeNoSpot      = "no_spot"      // the stock's price was not read, so nothing was asked
	OutcomeDeferred    = "deferred"     // not asked: over a ceiling, a budget or the time bound
	OutcomeNoCandidate = "no_candidate" // only ok rows: quotes exist but no candidate passed
	OutcomeNoRows      = "no_rows"      // the name has no open row at all
	OutcomeOther       = "other"        // any other status
)

// OpenFact is one stored open row of a name, as plain facts.
type OpenFact struct {
	Status      int
	ErrorCode   string
	OutsideBand bool
}

// NameOpenFacts are the open rows of one name.
type NameOpenFacts struct {
	Name  string
	Facts []OpenFact
}

// NameOpen is the outcome of a name and the count of its rows of each kind.
type NameOpen struct {
	Outcome                                      string
	NoQuote, Failed, NoSpot, Deferred, Ok, Other int
}

// NameOpenRule tells why a name has no open quote. It is pure: no store, no clock.
// Empty Facts give OutcomeNoRows; only ok rows give OutcomeNoCandidate.
type NameOpenRule interface {
	Classify(NameOpenFacts) NameOpen
}

// NewNameOpenRuleFunc builds the rule.
type NewNameOpenRuleFunc func() NameOpenRule
