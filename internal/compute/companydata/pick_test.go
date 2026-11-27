package companydata_test

import (
	"slices"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/compute/companydata"
	"github.com/andrewrong/optdesk/internal/compute/companydataapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

func dayOf(s string) types.SerializableDate {
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return types.SerializableDate{Time: d}
}

// fact builds a made-up us-gaap fact; start "" is an instant; fy 0 is nil.
func fact(concept, unit, doc, filed, start, end, val string, fy int, fp string) contracts.FundamentalFact {
	f := contracts.FundamentalFact{
		Taxonomy: "us-gaap", Concept: concept, Unit: unit, DocID: doc, Filed: dayOf(filed),
		PeriodEnd: dayOf(end), Value: val, Fp: fp,
	}
	if start != "" {
		s := dayOf(start)
		f.PeriodStart = &s
	}
	if fy != 0 {
		f.Fy = &fy
	}
	return f
}

func usd(concept, doc, filed, start, end, val string, fy int, fp string) contracts.FundamentalFact {
	return fact(concept, "USD", doc, filed, start, end, val, fy, fp)
}

func ni(doc, filed, start, end, val string, fy int, fp string) contracts.FundamentalFact {
	return usd("NetIncomeLoss", doc, filed, start, end, val, fy, fp)
}

// The calendar year 2025 as filed: periods and filings.
const (
	s25  = "2025-01-01"
	e1   = "2025-03-31"
	e2   = "2025-06-30"
	e3   = "2025-09-30"
	e4   = "2025-12-31"
	f1   = "2025-05-01"
	f2   = "2025-08-01"
	f3   = "2025-11-01"
	fK   = "2026-02-15"
	netC = "NetIncomeLoss"
)

func must(t *testing.T, facts []contracts.FundamentalFact) []companydataapi.Quarter {
	t.Helper()
	qs, err := companydata.Quarters(facts)
	if err != nil {
		t.Fatal(err)
	}
	return qs
}

// byNumber finds the quarter number n.
func byNumber(t *testing.T, qs []companydataapi.Quarter, n int) companydataapi.Quarter {
	t.Helper()
	for _, q := range qs {
		if q.Number == n {
			return q
		}
	}
	t.Fatalf("no quarter %d in %d quarters", n, len(qs))
	return companydataapi.Quarter{}
}

func wantCell(t *testing.T, name string, c companydataapi.Cell, value string, derived bool) {
	t.Helper()
	if c.Missing || c.Value != value || c.Derived != derived || c.Why != companydataapi.WhyNone {
		t.Fatalf("%s = %+v, want value %q derived %v", name, c, value, derived)
	}
}

func wantMissing(t *testing.T, name string, c companydataapi.Cell, why companydataapi.Why) {
	t.Helper()
	if !c.Missing || c.Value != "" || c.Why != why || c.Concept != "" {
		t.Fatalf("%s = %+v, want missing %q", name, c, why)
	}
}

func TestRestatementLatestWinsOldStays(t *testing.T) {
	t.Parallel()
	facts := []contracts.FundamentalFact{
		ni("a", "2025-02-01", s25, e1, "100", 2025, "Q1"),
		ni("b", "2025-08-01", s25, e1, "90", 2025, "Q1"),
	}
	qs := must(t, facts)
	c := byNumber(t, qs, 1).NetIncome
	wantCell(t, "net income", c, "90", false)
	if !c.Restated || c.Concept != netC {
		t.Fatalf("cell = %+v", c)
	}
	if len(facts) != 2 || facts[0].Value != "100" || facts[1].Value != "90" {
		t.Fatalf("input changed: %+v", facts)
	}
}

func TestRestatementTieBreaksByDocID(t *testing.T) {
	t.Parallel()
	for _, order := range [][2]string{{"a", "b"}, {"b", "a"}} {
		vals := map[string]string{"a": "100", "b": "90"}
		facts := []contracts.FundamentalFact{
			ni(order[0], f1, s25, e1, vals[order[0]], 2025, "Q1"),
			ni(order[1], f1, s25, e1, vals[order[1]], 2025, "Q1"),
		}
		c := byNumber(t, must(t, facts), 1).NetIncome
		wantCell(t, "net income", c, "90", false) // doc b is larger, whatever the input order
		if !c.Restated {
			t.Fatalf("not restated: %+v", c)
		}
	}
}

func TestFilingDateIsEarliestFiled(t *testing.T) {
	t.Parallel()
	facts := []contracts.FundamentalFact{
		ni("q1", f1, s25, e1, "100", 2025, "Q1"),
		ni("later", "2026-05-01", s25, e1, "100", 2026, "Q1"), // comparative of the next year's 10-Q
	}
	q := byNumber(t, must(t, facts), 1)
	if q.FilingDate != f1 || q.FiscalYear != 2025 || q.Label != "FY2025 Q1" {
		t.Fatalf("quarter = %+v", q)
	}
}

func TestRestated12MOrig9MNoDerive(t *testing.T) {
	t.Parallel()
	base := []contracts.FundamentalFact{
		ni("q3", f3, s25, e3, "700", 2025, "Q3"),
		ni("k", fK, s25, e4, "1000", 2025, "FY"),
		ni("k2", "2026-08-01", s25, e4, "1100", 2026, "FY"), // 12M restated, 9M only in the earlier filing
	}
	wantMissing(t, "Q4", byNumber(t, must(t, base), 4).NetIncome, companydataapi.WhyCannotDerive)
	both := append(slices.Clone(base), ni("k2", "2026-08-01", s25, e3, "750", 2026, "Q3"))
	wantCell(t, "Q4", byNumber(t, must(t, both), 4).NetIncome, "350", true)
}

func TestRestated9MOrig12MNoDerive(t *testing.T) {
	t.Parallel()
	facts := []contracts.FundamentalFact{
		ni("q3", f3, s25, e3, "700", 2025, "Q3"),
		ni("k", fK, s25, e4, "1000", 2025, "FY"),
		ni("q3b", "2026-05-01", s25, e3, "720", 2026, "Q3"), // filed after the 12M row
	}
	wantMissing(t, "Q4", byNumber(t, must(t, facts), 4).NetIncome, companydataapi.WhyCannotDerive)
	six := []contracts.FundamentalFact{
		ni("q1", f1, s25, e1, "100", 2025, "Q1"),
		ni("q2", f2, s25, e2, "250", 2025, "Q2"),
		ni("q1b", "2026-05-01", s25, e1, "110", 2026, "Q1"),
	}
	wantMissing(t, "Q2", byNumber(t, must(t, six), 2).NetIncome, companydataapi.WhyCannotDerive)
}

// facts3m is Q1 of 2025 with operating cash flow and capex 3M facts.
func facts3m(ocf, capex string) []contracts.FundamentalFact {
	return []contracts.FundamentalFact{
		usd(ocfC, "q1", f1, s25, e1, ocf, 2025, "Q1"),
		usd(capexC, "q1", f1, s25, e1, capex, 2025, "Q1"),
	}
}
