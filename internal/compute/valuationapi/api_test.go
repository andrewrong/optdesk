package valuationapi_test

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/valuationapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// The fakes keep the promises of the function types: each remembers its inputs and
// returns one fixed result.
type fakes struct {
	facts   []contracts.FundamentalFact
	std     valuationapi.Standard
	id      string
	spot    string
	costs   []string
	analyst *valuationapi.Analyst
	asOf    contracts.Day
	filings valuationapi.Filings
}

func (f *fakes) standardOf() valuationapi.StandardOfFunc {
	return func(facts []contracts.FundamentalFact) valuationapi.Standard {
		f.facts = facts
		return valuationapi.JGAAP
	}
}

func (f *fakes) buildFilings() valuationapi.BuildFilingsFunc {
	return func(facts []contracts.FundamentalFact, asOf contracts.Day, std valuationapi.Standard) valuationapi.FilingsResult {
		f.facts, f.asOf, f.std = facts, asOf, std
		return valuationapi.FilingsResult{
			Filings: valuationapi.Filings{
				Standard: std, Currency: "USD", SIC: "3571", SharesOutstanding: "100", CoverShares: "101",
				Years: []valuationapi.FilingYear{{FYEnd: "2025-12-31", Filed: "2026-02-01", Values: map[string]string{"ocf": "12.5"}}},
			},
			SIC: "3571", SICState: "ok", State: valuationapi.StateOK, Reasons: []string{"none"},
		}
	}
}

func (f *fakes) band() valuationapi.BandFunc {
	return func(id string, fl valuationapi.Filings, asOf contracts.Day) valuationapi.BandFigure {
		f.id, f.filings, f.asOf = id, fl, asOf
		return valuationapi.BandFigure{
			ID: id, State: valuationapi.StateOK, Value: "80", Low: "64", RefHigh: "107", Currency: "USD",
			BaseFYEnd: "2025-12-31", Flags: []string{"leverage"}, Reasons: []string{"r"},
		}
	}
}

func (f *fakes) implied() valuationapi.ImpliedGrowthFunc {
	return func(fl valuationapi.Filings, spot string) valuationapi.ImpliedResult {
		f.filings, f.spot = fl, spot
		return valuationapi.ImpliedResult{
			State: valuationapi.StateOK, Primary: "0.05",
			Rates: []valuationapi.ImpliedRate{{Rate: "0.10", Growth: "0.05"}}, Reasons: []string{"r"},
		}
	}
}

func (f *fakes) returnAt() valuationapi.ReturnAtPriceFunc {
	return func(fl valuationapi.Filings, costs []string, a *valuationapi.Analyst, asOf contracts.Day) valuationapi.ReturnResult {
		f.filings, f.costs, f.analyst, f.asOf = fl, costs, a, asOf
		keys := []string{
			valuationapi.LineSpendingStays, valuationapi.LineSpendingEarns,
			valuationapi.LineAnalystPath, valuationapi.LineSpendingEarnsAnalyst,
		}
		var lines []valuationapi.ReturnLine
		for _, c := range costs {
			for _, k := range keys {
				lines = append(lines, valuationapi.ReturnLine{Cost: c, Key: k, State: "ok", Value: "0.07", Reason: "r"})
			}
		}
		return valuationapi.ReturnResult{State: valuationapi.StateOK, Lines: lines, Flags: []string{"GROWTH_LIMITED_BY_R"}, Reasons: []string{"r"}}
	}
}

func (f *fakes) questions() valuationapi.QuestionsFunc {
	return func(fl valuationapi.Filings, asOf contracts.Day) valuationapi.QuestionsResult {
		f.filings, f.asOf = fl, asOf
		return valuationapi.QuestionsResult{Questions: []valuationapi.Question{
			{Key: "Q1", State: valuationapi.StateOK, Value: "0.2", Reason: "r"},
			{Key: "Q2", State: valuationapi.StateMissingInput, Reason: "no_input"},
		}}
	}
}

func (f *fakes) diagnosis() valuationapi.DiagnosisFunc {
	return func(fl valuationapi.Filings, asOf contracts.Day) valuationapi.DiagnosisResult {
		f.filings, f.asOf = fl, asOf
		return valuationapi.DiagnosisResult{Group: "financial", Figures: []valuationapi.DiagnosisFigure{
			{Key: "roe", State: valuationapi.StateOK, Value: "0.11", Reason: "r"},
		}}
	}
}

func TestStandardValues(t *testing.T) {
	t.Parallel()
	for std, text := range map[valuationapi.Standard]string{
		valuationapi.USGAAP: "US_GAAP", valuationapi.JGAAP: "J_GAAP", valuationapi.IFRS: "IFRS",
	} {
		if string(std) != text {
			t.Errorf("standard %q, want %q", std, text)
		}
	}
}

var testFacts = []contracts.FundamentalFact{{Source: "sec", Subject: "cik:0000320193"}}

const testDay = contracts.Day("2026-11-09")

func builtFilings(f *fakes) valuationapi.FilingsResult {
	return f.buildFilings()(testFacts, testDay, valuationapi.JGAAP)
}

func TestStandardOfAndBuildFilings(t *testing.T) {
	t.Parallel()
	f := &fakes{}
	std := f.standardOf()(testFacts)
	if std != valuationapi.JGAAP || len(f.facts) != 1 {
		t.Fatalf("standard %q, facts %d", std, len(f.facts))
	}
	fr := f.buildFilings()(testFacts, testDay, std)
	if f.std != std || f.asOf != testDay {
		t.Fatalf("filings saw std %q asOf %q", f.std, f.asOf)
	}
	y := fr.Filings.Years[0]
	if fr.SIC != "3571" || fr.SICState != "ok" || fr.State != valuationapi.StateOK || len(fr.Reasons) != 1 ||
		fr.Filings.Standard != std || fr.Filings.Currency != "USD" || fr.Filings.SIC != "3571" ||
		fr.Filings.SharesOutstanding != "100" || fr.Filings.CoverShares != "101" ||
		y.FYEnd != "2025-12-31" || y.Filed != "2026-02-01" || y.Values["ocf"] != "12.5" {
		t.Fatalf("filings result %+v", fr)
	}
}

func TestBandAndImplied(t *testing.T) {
	t.Parallel()
	f := &fakes{}
	fr := builtFilings(f)
	b := f.band()("AAPL", fr.Filings, testDay)
	if f.id != "AAPL" || b.ID != "AAPL" || b.State != "ok" || b.Value != "80" || b.Low != "64" ||
		b.RefHigh != "107" || b.Currency != "USD" || b.BaseFYEnd != "2025-12-31" ||
		len(b.Flags) != 1 || len(b.Reasons) != 1 {
		t.Fatalf("band %+v", b)
	}
	im := f.implied()(fr.Filings, "150.25")
	if f.spot != "150.25" || im.State != "ok" || im.Primary != "0.05" || len(im.Reasons) != 1 ||
		im.Rates[0].Rate != "0.10" || im.Rates[0].Growth != "0.05" {
		t.Fatalf("implied %+v", im)
	}
}

func TestReturnAtPriceLines(t *testing.T) {
	t.Parallel()
	f := &fakes{}
	fr := builtFilings(f)
	an := &valuationapi.Analyst{Growth: "0.08", AsOf: testDay}
	rr := f.returnAt()(fr.Filings, []string{"100", "90"}, an, testDay)
	if f.analyst != an || len(f.costs) != 2 || len(rr.Lines) != 8 || rr.State != "ok" ||
		len(rr.Flags) != 1 || len(rr.Reasons) != 1 {
		t.Fatalf("return %+v", rr)
	}
	l := rr.Lines[3]
	if l.Cost != "100" || l.Key != valuationapi.LineSpendingEarnsAnalyst || l.State != "ok" || l.Value != "0.07" || l.Reason != "r" {
		t.Fatalf("line %+v", l)
	}
	for i, k := range []string{"SPENDING_STAYS", "SPENDING_EARNS", "ANALYST_PATH", "SPENDING_EARNS_ANALYST"} {
		if rr.Lines[i].Key != k {
			t.Errorf("line %d key %q, want %q", i, rr.Lines[i].Key, k)
		}
	}
}

func TestReturnAtPriceNilAnalyst(t *testing.T) {
	t.Parallel()
	f := &fakes{}
	f.analyst = &valuationapi.Analyst{}
	f.returnAt()(valuationapi.Filings{}, nil, nil, testDay)
	if f.analyst != nil {
		t.Errorf("a nil analyst must reach the function as nil")
	}
}

func TestQuestionsAndDiagnosis(t *testing.T) {
	t.Parallel()
	f := &fakes{}
	fr := builtFilings(f)
	q := f.questions()(fr.Filings, testDay)
	if len(q.Questions) != 2 || q.Questions[0].Key != "Q1" || q.Questions[0].Value != "0.2" ||
		q.Questions[0].State != "ok" || q.Questions[0].Reason != "r" || q.Questions[1].State != "missing_input" {
		t.Fatalf("questions %+v", q)
	}
	d := f.diagnosis()(fr.Filings, testDay)
	if d.Group != "financial" || d.Figures[0].Key != "roe" || d.Figures[0].State != "ok" ||
		d.Figures[0].Value != "0.11" || d.Figures[0].Reason != "r" {
		t.Fatalf("diagnosis %+v", d)
	}
	if valuationapi.StateUnvalued != "unvalued" {
		t.Errorf("unvalued state %q", valuationapi.StateUnvalued)
	}
}
