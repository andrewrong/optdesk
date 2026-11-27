package datacheck

import "github.com/andrewrong/optdesk/internal/store/finding"

// defaultRules returns the rules in the order they run: the rules of wrong values (entry 7 of
// docs/design/data-check.md). The rules of gaps are added by entry 8.
func defaultRules() []Rule {
	return []Rule{
		{ID: finding.RuleReadingRepeat, Run: ruleReadingRepeat},
		{ID: finding.RuleOpenLateQuote, Run: ruleOpenLateQuote},
		{ID: finding.RuleCandidatesStale, Run: ruleCandidatesStale},
	}
}
