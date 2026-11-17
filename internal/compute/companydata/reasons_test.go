package companydata_test

import (
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/companydataapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

func textOf(s companydataapi.Summary, r companydataapi.Reason) string {
	for _, m := range s.Missing {
		if m.Reason == r {
			return m.Text
		}
	}
	return ""
}

func TestReasonJP(t *testing.T) {
	t.Parallel()
	s := summarize(t, companydataapi.SummaryInput{Market: "JP", Issuer: "edinet:E00001", Fundamentals: true})
	if !hasReason(s, companydataapi.ReasonJP) || hasReason(s, companydataapi.ReasonNoStatements) {
		t.Fatalf("missing = %+v", s.Missing)
	}
}

func TestReasonNoCIK(t *testing.T) {
	t.Parallel()
	for _, issuer := range []string{"", "edinet:E00001"} {
		in := usIn()
		in.Issuer = issuer
		s := summarize(t, in)
		if !hasReason(s, companydataapi.ReasonNoCIK) || hasReason(s, companydataapi.ReasonNoStatements) {
			t.Fatalf("issuer %q: %+v", issuer, s.Missing)
		}
	}
}

func TestReasonFundamentalsOff(t *testing.T) {
	t.Parallel()
	in := usIn()
	in.Fundamentals = false
	s := summarize(t, in)
	if !hasReason(s, companydataapi.ReasonFundamentalsOff) || hasReason(s, companydataapi.ReasonNoStatements) {
		t.Fatalf("missing = %+v", s.Missing)
	}
}

func TestReasonNoStatements(t *testing.T) {
	t.Parallel()
	s := summarize(t, usIn())
	if !hasReason(s, companydataapi.ReasonNoStatements) {
		t.Fatalf("missing = %+v", s.Missing)
	}
	// a us-gaap fact removes it
	s = summarize(t, usIn(ni("q1", f1, s25, e1, "100", 2025, "Q1")))
	if hasReason(s, companydataapi.ReasonNoStatements) {
		t.Fatalf("missing = %+v", s.Missing)
	}
}

func annualFact(form string) contracts.FundamentalFact {
	f := ni("k", fK, s25, e4, "1000", 2025, "FY")
	f.Form = form
	return f
}

func TestReasonAnnualOnly(t *testing.T) {
	t.Parallel()
	s := summarize(t, usIn(annualFact("20-F")))
	if !hasReason(s, companydataapi.ReasonAnnualOnly) {
		t.Fatalf("missing = %+v", s.Missing)
	}
	q := ni("q1", f1, s25, e1, "100", 2025, "Q1")
	q.Form = "10-Q"
	s = summarize(t, usIn(annualFact("20-F"), q))
	if hasReason(s, companydataapi.ReasonAnnualOnly) {
		t.Fatalf("with a 10-Q: %+v", s.Missing)
	}
}

func TestNoIFRSMarkerNeeded(t *testing.T) {
	t.Parallel()
	// An IFRS filer leaves no stored row: the input is the same as a name the run has not reached.
	s := summarize(t, usIn())
	txt := textOf(s, companydataapi.ReasonNoStatements)
	if txt == "" || !strings.Contains(txt, "IFRS filer") || !strings.Contains(txt, "daily run") {
		t.Fatalf("text = %q", txt)
	}
}

func TestReasonNoNextReport(t *testing.T) {
	t.Parallel()
	in := usIn()
	if !hasReason(summarize(t, in), companydataapi.ReasonNoNextReport) {
		t.Fatal("nil NextReport gives no reason")
	}
	d := contracts.Day("2026-01-02")
	in.NextReport = &d
	if hasReason(summarize(t, in), companydataapi.ReasonNoNextReport) {
		t.Fatal("a report date still gives the reason")
	}
}

func TestMissingTextsAreSentences(t *testing.T) {
	t.Parallel()
	inputs := []companydataapi.SummaryInput{
		{Market: "JP"},
		{Market: "US"},
		{Market: "US", Issuer: "cik:0000000001"},
		usIn(),
		usIn(annualFact("40-F")),
	}
	off := usIn()
	off.Fundamentals = false
	inputs = append(inputs, off)
	seen := map[companydataapi.Reason]bool{}
	for _, in := range inputs {
		for _, m := range summarize(t, in).Missing {
			seen[m.Reason] = true
			x := m.Text
			if !strings.HasSuffix(x, ".") || strings.Contains(x[:len(x)-1], ". ") ||
				strings.Contains(strings.ToLower(x), "coverage") || strings.Contains(x, "\n") {
				t.Errorf("%s: %q is not one sentence", m.Reason, x)
			}
			if m.Reason == companydataapi.ReasonJP && !strings.Contains(x, "left out on purpose") {
				t.Errorf("jp text = %q", x)
			}
		}
	}
	if len(seen) != 8 {
		t.Fatalf("reasons reached = %d, want 8", len(seen))
	}
}
