package computefigures

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/compute/own"
	"github.com/andrewrong/optdesk/internal/compute/rules"
	"github.com/andrewrong/optdesk/internal/compute/valuationapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
)

const (
	vIssuer = "cik:0000320193"
	vSymbol = "AAPL"
)

// vFacts is a fact reader that records what it was asked.
type vFacts struct {
	facts           []contracts.FundamentalFact
	source, subject string
	cutoff          contracts.Day
	calls           int
}

func (f *vFacts) FactsFiledBy(_ context.Context, source, subject string, cutoff contracts.Day) ([]contracts.FundamentalFact, error) {
	f.calls++
	f.source, f.subject, f.cutoff = source, subject, cutoff
	return f.facts, nil
}

// vFakes are the five compute functions with the answers a test sets.
type vFakes struct {
	std       valuationapi.Standard
	res       valuationapi.FilingsResult
	gotStd    valuationapi.Standard
	buildCall int
}

func (v *vFakes) deps(facts *vFacts, entries []jobs.WatchEntry) ValuationDeps {
	return ValuationDeps{
		Facts: facts, Entries: entries,
		StandardOf: func([]contracts.FundamentalFact) valuationapi.Standard { return v.std },
		BuildFilings: func(_ []contracts.FundamentalFact, _ contracts.Day, std valuationapi.Standard) valuationapi.FilingsResult {
			v.buildCall++
			v.gotStd = std
			return v.res
		},
		Band: func(id string, _ valuationapi.Filings, _ contracts.Day) valuationapi.BandFigure {
			return valuationapi.BandFigure{ID: id, State: valuationapi.StateOK, Value: "182.5000", Low: "146.0000",
				RefHigh: "243.3333", Currency: "USD", BaseFYEnd: "2025-09-27", Flags: []string{"leverage"}}
		},
		Questions: func(valuationapi.Filings, contracts.Day) valuationapi.QuestionsResult {
			return valuationapi.QuestionsResult{Questions: []valuationapi.Question{
				{Key: "Q1", State: valuationapi.StateOK, Value: "0.31"},
				{Key: "Q2", State: valuationapi.StateMissingInput, Reason: "too_short"}}}
		},
		Diagnosis: func(valuationapi.Filings, contracts.Day) valuationapi.DiagnosisResult {
			return valuationapi.DiagnosisResult{Group: "cyclical", Figures: []valuationapi.DiagnosisFigure{
				{Key: "operating_margin_median", State: valuationapi.StateOK, Value: "0.07"}}}
		},
	}
}

func okResult() valuationapi.FilingsResult {
	return valuationapi.FilingsResult{State: valuationapi.StateOK, SIC: "3571", SICState: "ok",
		Filings: valuationapi.Filings{Standard: valuationapi.USGAAP, Currency: "USD"}}
}

func entry(issuer, symbol string) jobs.WatchEntry {
	return jobs.WatchEntry{Issuer: issuer, Market: "US", Contract: broker.Contract{Symbol: symbol}}
}

func valuationJobOf(t *testing.T, f *fx, d ValuationDeps) contracts.Job {
	t.Helper()
	j, err := NewWithValuation(f.stores(), newFigs(), rules.New(), own.NewUnits(), own.NewSeriesOps(), d,
		clockwork.NewFakeClockAt(closeD.Add(20*time.Minute)))
	if err != nil {
		t.Fatalf("NewWithValuation: %v", err)
	}
	return j
}

func valuationKeys(t *testing.T, j contracts.Job) []string {
	t.Helper()
	var out []string
	for _, k := range plan(t, j) {
		if strings.HasPrefix(k.SubjectKey, "valuation.") {
			out = append(out, k.SubjectKey)
		}
	}
	return out
}

func TestNilFuncsNoValuationKey(t *testing.T) {
	t.Parallel()
	entries := []jobs.WatchEntry{entry(vIssuer, vSymbol)}
	for _, tc := range []struct {
		name string
		cut  func(*ValuationDeps)
	}{
		{"StandardOf", func(d *ValuationDeps) { d.StandardOf = nil }},
		{"BuildFilings", func(d *ValuationDeps) { d.BuildFilings = nil }},
		{"Band", func(d *ValuationDeps) { d.Band = nil }},
		{"Questions", func(d *ValuationDeps) { d.Questions = nil }},
		{"Diagnosis", func(d *ValuationDeps) { d.Diagnosis = nil }},
	} {
		fk := &vFakes{std: valuationapi.USGAAP, res: okResult()}
		d := fk.deps(&vFacts{}, entries)
		tc.cut(&d)
		if got := valuationKeys(t, valuationJobOf(t, world(), d)); len(got) != 0 {
			t.Fatalf("%s nil: keys %v, want none", tc.name, got)
		}
	}
	fk := &vFakes{std: valuationapi.USGAAP, res: okResult()}
	if got := valuationKeys(t, valuationJobOf(t, world(), fk.deps(&vFacts{}, entries))); len(got) != 3 {
		t.Fatalf("all set: keys %v, want 3", got)
	}
}

func TestStandardFromFacts(t *testing.T) {
	t.Parallel()
	facts := &vFacts{facts: []contracts.FundamentalFact{{Concept: "AccountingStandard", Unit: "J_GAAP"}}}
	fk := &vFakes{std: valuationapi.JGAAP, res: okResult()}
	f := world()
	j := valuationJobOf(t, f, fk.deps(facts, []jobs.WatchEntry{entry(vIssuer, vSymbol)}))
	if r := runAll(t, j); r.Err != nil {
		t.Fatalf("run: %v", r.Err)
	}
	if fk.gotStd != valuationapi.JGAAP || fk.buildCall != 1 {
		t.Fatalf("BuildFilings saw std %q in %d calls, want J_GAAP once", fk.gotStd, fk.buildCall)
	}
	if facts.source != "sec" || facts.subject != vIssuer || facts.cutoff != dayD || facts.calls != 1 {
		t.Fatalf("facts asked %+v, want sec, %s, %s, once", facts, vIssuer, dayD)
	}

	none := &vFakes{std: "", res: okResult()}
	f2 := world()
	j2 := valuationJobOf(t, f2, none.deps(&vFacts{}, []jobs.WatchEntry{entry(vIssuer, vSymbol)}))
	runAll(t, j2)
	r := reading(t, f2, valuationBand, "underlying", vSymbol)
	if r.State != "missing_input" || tagOf(r, "reason") != "no_standard" {
		t.Fatalf("band reading %s %q, want missing_input no_standard", r.State, tagOf(r, "reason"))
	}
}

func TestThreeReadingsPerCompany(t *testing.T) {
	t.Parallel()
	fk := &vFakes{std: valuationapi.USGAAP, res: okResult()}
	f := world()
	j := valuationJobOf(t, f, fk.deps(&vFacts{}, []jobs.WatchEntry{entry(vIssuer, vSymbol), entry("", "NOID")}))
	if got := valuationKeys(t, j); len(got) != 3 {
		t.Fatalf("keys %v, want 3 (none for the entry without issuer)", got)
	}
	if r := runAll(t, j); r.Err != nil {
		t.Fatalf("run: %v", r.Err)
	}
	checkThreeOK(t, f)
	for _, n := range valuationNames {
		if rows, _ := f.LatestReadings(context.Background(), n, "underlying", "NOID", dayD, dayD); len(rows) != 0 {
			t.Fatalf("%s: %d readings for the entry without issuer, want 0", n, len(rows))
		}
	}
	// a second run of the day writes no new rev
	runAll(t, j)
	for _, n := range valuationNames {
		if r := reading(t, f, n, "underlying", vSymbol); r.Rev != 1 {
			t.Fatalf("%s rev %d after a second run, want 1", n, r.Rev)
		}
	}
}

func TestBankGivesMissingInputBand(t *testing.T) {
	t.Parallel()
	fk := &vFakes{std: valuationapi.USGAAP, res: valuationapi.FilingsResult{
		State: valuationapi.StateUnvalued, SIC: "6021", SICState: "ok", Reasons: []string{"financial_sic"},
		Filings: valuationapi.Filings{Standard: valuationapi.USGAAP, SIC: "6021"}}}
	facts := &vFacts{facts: []contracts.FundamentalFact{{Taxonomy: "sec", Concept: "SIC", Value: "6021"}}}
	f := world()
	j := valuationJobOf(t, f, fk.deps(facts, []jobs.WatchEntry{entry("cik:0000072971", "WFC")}))
	runAll(t, j)
	band := reading(t, f, valuationBand, "underlying", "WFC")
	if band.State != "missing_input" || band.Value != nil || tagOf(band, "reason") != "financial_sic" {
		t.Fatalf("band reading %+v, want missing_input financial_sic", band)
	}
	q := reading(t, f, valuationQuestion, "underlying", "WFC")
	if q.State != "missing_input" || tagOf(q, "reason") != "financial_sic" {
		t.Fatalf("question reading %+v, want missing_input financial_sic", q)
	}
	d := reading(t, f, valuationDiagnosis, "underlying", "WFC")
	if d.State != "ok" || tagOf(d, "operating_margin_median") != "0.07" {
		t.Fatalf("diagnosis reading %+v, want the diagnosis of the refused company", d)
	}
}

func TestValuationKeepsOtherKeys(t *testing.T) {
	t.Parallel()
	fk := &vFakes{std: valuationapi.USGAAP, res: okResult()}
	f := world()
	f.addPoint("VIXCLS", dayD, 20)
	j := valuationJobOf(t, f, fk.deps(&vFacts{}, []jobs.WatchEntry{entry(vIssuer, vSymbol)}))
	if keys := plan(t, j); len(keys) != 3+len(marketIDs)+3 {
		t.Fatalf("keys %d, want the 9 of the plain job and 3 valuation keys", len(keys))
	}
	runAll(t, j)
	if r := reading(t, f, "market.iv30_own", "underlying", "SPY"); r.State == "" {
		t.Fatalf("iv30 reading missing: %+v", r)
	}
}

// checkThreeOK checks the three readings of a company whose filings are ok.
func checkThreeOK(t *testing.T, f *fx) {
	t.Helper()
	band := reading(t, f, valuationBand, "underlying", vSymbol)
	if band.State != "ok" || band.Value == nil || *band.Value != "182.5000" || band.Currency != "USD" ||
		tagOf(band, "low") != "146.0000" || tagOf(band, "ref_high") != "243.3333" ||
		tagOf(band, "base_fy_end") != "2025-09-27" || tagOf(band, "flag") != "leverage" {
		t.Fatalf("band reading %+v", band)
	}
	q := reading(t, f, valuationQuestion, "underlying", vSymbol)
	if q.State != "ok" || *q.Value != "1" || tagOf(q, "Q1") != "0.31" || tagOf(q, "Q2.reason") != "too_short" {
		t.Fatalf("question reading %+v", q)
	}
	d := reading(t, f, valuationDiagnosis, "underlying", vSymbol)
	if d.State != "ok" || tagOf(d, "group") != "cyclical" || tagOf(d, "operating_margin_median") != "0.07" {
		t.Fatalf("diagnosis reading %+v", d)
	}
}

func TestEdinetIssuerReadsEdinetFacts(t *testing.T) {
	t.Parallel()
	facts := &vFacts{}
	fk := &vFakes{std: valuationapi.JGAAP, res: okResult()}
	f := world()
	j := valuationJobOf(t, f, fk.deps(facts, []jobs.WatchEntry{entry("edinet:E02144", "7203")}))
	runAll(t, j)
	if facts.source != "edinet" || facts.subject != "edinet:E02144" {
		t.Fatalf("facts read with source %q for %q, want edinet", facts.source, facts.subject)
	}
}
