package datacheck

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/store/finding"
)

func TestDedupeKeepsOneProblemOnce(t *testing.T) {
	t.Parallel()
	mk := func(rule, ds, day string) finding.Finding {
		return finding.Finding{Rule: rule, Dataset: ds, Day: dayOf(day), Severity: finding.SeverityWarn}
	}
	in := []finding.Finding{
		mk(finding.RuleCaptureGap, "a", "2026-11-24"),
		mk(finding.RuleGapUnclosed, "a", "2026-11-24"), // same (dataset, day): dropped
		mk(finding.RuleGapUnclosed, "a", "2026-11-25"), // other day: kept
		mk(finding.RuleGapUnclosed, "b", "2026-11-24"), // other dataset: kept
		mk(finding.RulePlanAbsent, "a", "2026-11-24"),  // not gap_unclosed: kept
	}
	out := dedupe(in)
	if len(out) != 4 {
		t.Fatalf("out %+v", out)
	}
	for _, f := range out {
		if f.Rule == finding.RuleGapUnclosed && f.Dataset == "a" && f.Day == "2026-11-24" {
			t.Fatalf("not dropped: %+v", f)
		}
	}
	if len(in) != 5 {
		t.Fatal("input changed")
	}
}
