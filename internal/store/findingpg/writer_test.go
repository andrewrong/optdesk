package findingpg

import (
	"context"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/store/finding"
)

// TestEvidenceIsFixedVocabulary: InsertRun refuses what could carry free text or an id. The writer has no
// connection, so an error that is not a connection error shows the refusal came before any statement.
func TestEvidenceIsFixedVocabulary(t *testing.T) {
	t.Parallel()
	good := finding.Finding{Rule: finding.RuleCaptureGap, Dataset: "bars.eod", Day: "2026-11-24",
		Severity: finding.SeverityWarn, Evidence: []finding.Count{{Name: "missing", N: 1}}}
	long := strings.Repeat("x", 121)
	cases := map[string]func(f *finding.Finding){
		"evidence name": func(f *finding.Finding) { f.Evidence = []finding.Count{{Name: "account DU123", N: 1}} },
		"subject":       func(f *finding.Finding) { f.Subject = long },
		"dataset":       func(f *finding.Finding) { f.Dataset = long },
		"rule":          func(f *finding.Finding) { f.Rule = long },
		"severity":      func(f *finding.Finding) { f.Severity = "fatal" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := good
			mutate(&f)
			_, err := (&pgWriter{}).InsertRun(context.Background(), finding.Run{CheckDay: "2026-11-25", Findings: []finding.Finding{f}})
			if err == nil || !strings.Contains(err.Error(), "findingpg: finding 0") {
				t.Fatalf("err = %v", err)
			}
		})
	}
	if err := validate(finding.Run{Findings: []finding.Finding{good}}); err != nil {
		t.Fatalf("good finding refused: %v", err)
	}
	f := good
	f.Subject = strings.Repeat("x", 120)
	if err := validate(finding.Run{Findings: []finding.Finding{f}}); err != nil {
		t.Fatalf("120 bytes refused: %v", err)
	}
}
