package datacheck

import (
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/finding"
)

// dedupe drops a gap_unclosed finding whose (dataset, day) is also a capture_gap finding of the same run, so one
// problem is one finding. Every other finding is kept, in order.
func dedupe(in []finding.Finding) []finding.Finding {
	type key struct {
		dataset string
		day     contracts.Day
	}
	captured := map[key]bool{}
	for _, f := range in {
		if f.Rule == finding.RuleCaptureGap {
			captured[key{f.Dataset, f.Day}] = true
		}
	}
	out := make([]finding.Finding, 0, len(in))
	for _, f := range in {
		if f.Rule == finding.RuleGapUnclosed && captured[key{f.Dataset, f.Day}] {
			continue
		}
		out = append(out, f)
	}
	return out
}
