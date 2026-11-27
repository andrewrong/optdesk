package datacheck

import "github.com/andrewrong/optdesk/internal/store/finding"

// defaultRules returns the rules in the order they run: the rules of wrong values (entry 7 of
// docs/design/data-check.md), then the rules of gaps (entry 8), then the three of
// docs/design/data-check-2.md (entry 4).
func defaultRules() []Rule {
	return []Rule{
		{ID: finding.RuleReadingRepeat, Run: ruleReadingRepeat},
		{ID: finding.RuleOpenLateQuote, Run: ruleOpenLateQuote},
		{ID: finding.RuleCandidatesStale, Run: ruleCandidatesStale},
		{ID: finding.RuleCaptureGap, Run: ruleCaptureGap},
		{ID: finding.RulePlanAbsent, Run: rulePlanAbsent},
		{ID: finding.RuleGapUnclosed, Run: ruleGapUnclosed},
		{ID: finding.RuleCandidateRuleVersion, Run: ruleCandidateVersion},
		{ID: finding.RuleEarningsCoverage, Run: ruleEarningsCoverage},
		{ID: finding.RuleBarOHLCShape, Run: ruleBarShape},
	}
}
