package companydata_test

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/companydata"
	"github.com/andrewrong/optdesk/internal/compute/companydataapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

func summarize(t *testing.T, in companydataapi.SummaryInput) companydataapi.Summary {
	t.Helper()
	s, err := companydata.Calc{}.Summarize(in)
	if err != nil {
		t.Fatal(err)
	}
	if s.Missing == nil {
		t.Fatal("Missing is nil")
	}
	return s
}

func hasReason(s companydataapi.Summary, r companydataapi.Reason) bool {
	for _, m := range s.Missing {
		if m.Reason == r {
			return true
		}
	}
	return false
}

func usIn(facts ...contracts.FundamentalFact) companydataapi.SummaryInput {
	return companydataapi.SummaryInput{Market: "US", Issuer: "cik:0000000001", Fundamentals: true,
		HasInstrument: true, Facts: facts}
}

func TestSummaryCountsBars(t *testing.T) {
	t.Parallel()
	in := usIn()
	in.BarDays = []contracts.Day{"2025-03-03", "2025-03-05", "2025-03-04"}
	s := summarize(t, in)
	if s.BarCount != 3 || s.LastBarDay == nil || *s.LastBarDay != "2025-03-05" {
		t.Fatalf("summary = %+v", s)
	}
	if s := summarize(t, companydataapi.SummaryInput{}); s.LastBarDay != nil || s.BarCount != 0 {
		t.Fatalf("empty summary = %+v", s)
	}
}

func TestSummaryNoBars(t *testing.T) {
	t.Parallel()
	in := usIn()
	s := summarize(t, in)
	if !hasReason(s, companydataapi.ReasonNoBars) || hasReason(s, companydataapi.ReasonNoInstrument) {
		t.Fatalf("resolved: %+v", s.Missing)
	}
	for _, m := range s.Missing {
		if m.Reason == companydataapi.ReasonNoBars && m.Text == "" {
			t.Fatal("no text")
		}
	}
	in.HasInstrument = false
	s = summarize(t, in)
	if !hasReason(s, companydataapi.ReasonNoInstrument) || hasReason(s, companydataapi.ReasonNoBars) {
		t.Fatalf("unresolved: %+v", s.Missing)
	}
}

func TestSummaryLatestQuarterIsNewestWithAValue(t *testing.T) {
	t.Parallel()
	// Q2 has only operating income (no revenue, net income or EPS value), so Q1 is the latest.
	s := summarize(t, usIn(
		ni("q1", f1, s25, e1, "100", 2025, "Q1"),
		usd("OperatingIncomeLoss", "q2", f2, s25, e2, "5", 2025, "Q2"),
	))
	if !s.Statements || s.LatestLabel != "FY2025 Q1" || s.LatestEnd == nil || *s.LatestEnd != e1 {
		t.Fatalf("summary = %+v", s)
	}
	s = summarize(t, usIn(
		ni("q1", f1, s25, e1, "100", 2025, "Q1"),
		ni("q2", f2, s25, e2, "250", 2025, "Q2"),
	))
	if s.LatestLabel != "FY2025 Q2" || *s.LatestEnd != e2 {
		t.Fatalf("summary = %+v", s)
	}
	if s := summarize(t, usIn()); s.Statements || s.LatestLabel != "" || s.LatestEnd != nil {
		t.Fatalf("no facts: %+v", s)
	}
}

func TestSummaryEPSQuarterCount(t *testing.T) {
	t.Parallel()
	s := summarize(t, usIn(
		eps("e1", f1, s25, e1, "1.00", "Q1"),
		ni("n2", f2, s25, e2, "250", 2025, "Q2"), // a quarter with no EPS
		eps("e3", f3, q3s, e3, "1.20", "Q3"),
	))
	if s.EPSQuarters != 2 {
		t.Fatalf("EPSQuarters = %d", s.EPSQuarters)
	}
}

func TestSummaryFiscalYearEndMonth(t *testing.T) {
	t.Parallel()
	st := "2024-09-29"
	s := summarize(t, usIn(
		ni("q1", "2025-02-01", st, "2024-12-28", "10", 2025, "Q1"),
		ni("q2", "2025-05-01", st, "2025-03-29", "30", 2025, "Q2"),
		ni("q3", "2025-08-01", st, "2025-06-28", "60", 2025, "Q3"),
		ni("k", "2025-11-01", st, "2025-09-27", "100", 2025, "FY"),
	))
	if s.FiscalYearEndMonth != 9 {
		t.Fatalf("month = %d", s.FiscalYearEndMonth)
	}
}
