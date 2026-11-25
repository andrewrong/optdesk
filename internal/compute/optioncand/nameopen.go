package optioncand

import "github.com/andrewrong/optdesk/internal/compute/optioncandapi"

const (
	codeNoQuote = "NO_QUOTE"
	codeBudget  = "BUDGET"
)

type nameOpenRule struct{}

// NewNameOpen returns the rule that tells why a name has no open quote, from the status
// definitions of the option open pull. Design: docs/design/sellput-jp-outcomes.md#2-name-open-rule.
func NewNameOpen() optioncandapi.NameOpenRule { return nameOpenRule{} }

var _ optioncandapi.NewNameOpenRuleFunc = NewNameOpen

// Classify counts each row by kind and names the worst kind present.
func (nameOpenRule) Classify(f optioncandapi.NameOpenFacts) optioncandapi.NameOpen {
	if len(f.Facts) == 0 {
		return optioncandapi.NameOpen{Outcome: optioncandapi.OutcomeNoRows}
	}
	var n optioncandapi.NameOpen
	for _, r := range f.Facts {
		switch {
		case r.Status == 1 || r.Status == 2:
			n.Ok++
		case r.Status == 3 && r.ErrorCode == codeNoQuote:
			n.NoQuote++
		case r.Status == 6 || r.Status == 8 || r.Status == 9:
			n.Failed++
		case r.Status == 7 && (r.OutsideBand || r.ErrorCode == "" || r.ErrorCode == codeBudget):
			n.Deferred++
		case r.Status == 7:
			n.NoSpot++
		default:
			n.Other++
		}
	}
	switch {
	case n.Failed > 0:
		n.Outcome = optioncandapi.OutcomeFailed
	case n.NoSpot > 0:
		n.Outcome = optioncandapi.OutcomeNoSpot
	case n.Deferred > 0:
		n.Outcome = optioncandapi.OutcomeDeferred
	case n.NoQuote > 0:
		n.Outcome = optioncandapi.OutcomeNoQuote
	case n.Other > 0:
		n.Outcome = optioncandapi.OutcomeOther
	default:
		n.Outcome = optioncandapi.OutcomeNoCandidate
	}
	return n
}
