package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/compute/valuationapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages/companies_page"
	"github.com/andrewrong/optdesk/internal/store"
)

// the fixture is the input of the page builder: the stores below hold its rows, the fake compute functions return
// its figures, and the page built through newCompaniesPage must equal the ok fixture in the blocks this builder owns.

type fixtureInput struct {
	Today    string `json:"today"`
	Subjects []struct {
		Issuer    string `json:"issuer"`
		Market    string `json:"market"`
		Symbol    string `json:"symbol"`
		HandPrice string `json:"hand_price"`
	} `json:"subjects"`
	Readings []struct {
		Name, Subject, Day, State string
		Value                     *string
	} `json:"readings"`
	Facts       []contracts.FundamentalFact `json:"facts"`
	Series      []contracts.SeriesPoint     `json:"series"`
	Instruments []struct {
		ID     int64  `json:"id"`
		Symbol string `json:"symbol"`
	} `json:"instruments"`
	OptionRows []struct {
		SessionDay   string `json:"session_day"`
		InstrumentID int    `json:"instrument_id"`
		UnderlyingID int64  `json:"underlying_id"`
		Right        string `json:"right"`
		Expiry       string `json:"expiry"`
		Strike       string `json:"strike"`
	} `json:"option_rows"`
	Quotes []struct {
		SessionDay   string  `json:"session_day"`
		InstrumentID int     `json:"instrument_id"`
		Last         float64 `json:"last"`
	} `json:"quotes"`
	OwnerText map[string]map[string]string `json:"owner_text"`
}

func loadFixtureInput(t *testing.T) fixtureInput {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "contracts", "fixtures", "page.companies_page.v2.input.json"))
	if err != nil {
		t.Fatal(err)
	}
	var in fixtureInput
	if err := json.Unmarshal(b, &in); err != nil {
		t.Fatal(err)
	}
	return in
}

func fixtureDay(t *testing.T, s string) types.SerializableDate {
	t.Helper()
	d, err := time.Parse(dayLayout, s)
	if err != nil {
		t.Fatal(err)
	}
	return types.SerializableDate{Time: d}
}

type fxFakeView struct{ rows []contracts.ReadingRecord }

func (f fxFakeView) WinnerReadings(_ context.Context, name, _, subject string, _, _ contracts.Day) ([]contracts.ReadingRecord, error) {
	var out []contracts.ReadingRecord
	for _, r := range f.rows {
		if r.Name == name && r.Subject == subject {
			out = append(out, r)
		}
	}
	return out, nil
}

func (fxFakeView) Close() error { return nil }

type fxFakeFacts []contracts.FundamentalFact

func (f fxFakeFacts) FactsFiledBy(_ context.Context, source, subject string, _ contracts.Day) ([]contracts.FundamentalFact, error) {
	var out []contracts.FundamentalFact
	for _, x := range f {
		if string(x.Source) == source && x.Subject == subject {
			out = append(out, x)
		}
	}
	return out, nil
}

type fxFakePoints []contracts.SeriesPoint

func (f fxFakePoints) SeriesPoints(context.Context, string, contracts.Day, contracts.Day) ([]contracts.SeriesPoint, error) {
	return f, nil
}

type fxFakeCatalog []int64

func (f fxFakeCatalog) OptionEodLatest(context.Context, contracts.Day) ([]store.OptionEodLatest, error) {
	out := make([]store.OptionEodLatest, 0, len(f))
	for _, id := range f {
		out = append(out, store.OptionEodLatest{UnderlyingID: id})
	}
	return out, nil
}

type fxFakeNames []store.InstrumentName

func (f fxFakeNames) InstrumentNames(_ context.Context, ids []int64) ([]store.InstrumentName, error) {
	var out []store.InstrumentName
	for _, n := range f {
		for _, id := range ids {
			if n.InstrumentID == id {
				out = append(out, n)
			}
		}
	}
	return out, nil
}

type fxFakeEod struct{ rows []contracts.OptionEod }

func (f fxFakeEod) OptionEod(_ context.Context, id int64, _, _ contracts.Day) ([]contracts.OptionEod, error) {
	var out []contracts.OptionEod
	for _, r := range f.rows {
		if r.UnderlyingID == int(id) {
			out = append(out, r)
		}
	}
	return out, nil
}

type fxFakeTerms []store.OptionTerms

func (f fxFakeTerms) OptionTerms(_ context.Context, ids []int64) ([]store.OptionTerms, error) {
	var out []store.OptionTerms
	for _, t := range f {
		for _, id := range ids {
			if t.InstrumentID == id {
				out = append(out, t)
			}
		}
	}
	return out, nil
}

// fxFakeBars returns one trades bar for every instrument id.
type fxFakeBars struct{}

func (fxFakeBars) PriceBars(_ context.Context, _ int64, basis string, from, to contracts.Day) ([]contracts.PriceBar, error) {
	if basis != "trades" || from > to {
		return nil, nil
	}
	return []contracts.PriceBar{{Close: "10"}}, nil
}

type fxFakeQuotes []contracts.QuoteSnapshot

func (f fxFakeQuotes) QuoteSnapshots(_ context.Context, id int64, _, _ contracts.Day) ([]contracts.QuoteSnapshot, error) {
	var out []contracts.QuoteSnapshot
	for _, q := range f {
		if q.InstrumentID == int(id) {
			out = append(out, q)
		}
	}
	return out, nil
}

type fxFakeOwnerText map[string]map[string]string

func (f fxFakeOwnerText) Moat(_ context.Context, s string) (string, bool, error) {
	t, ok := f["moat"][s]
	return t, ok, nil
}

func (f fxFakeOwnerText) FifthQuestion(_ context.Context, s string) (string, bool, error) {
	t, ok := f["fifth_question"][s]
	return t, ok, nil
}

// fixtureStores builds the fake stores of the fixture.
func fixtureStores(t *testing.T, in fixtureInput) (view fxFakeView, inputs companiesInputs) {
	t.Helper()
	for _, r := range in.Readings {
		rec := contracts.ReadingRecord{Name: r.Name, Subject: r.Subject, Day: fixtureDay(t, r.Day), Value: r.Value}
		if r.State == "ok" {
			rec.State = "ok"
		}
		view.rows = append(view.rows, rec)
	}
	var catalog fxFakeCatalog
	var names fxFakeNames
	for _, i := range in.Instruments {
		catalog = append(catalog, i.ID)
		names = append(names, store.InstrumentName{InstrumentID: i.ID, Type: "stock", Name: i.Symbol})
	}
	var eod fxFakeEod
	var terms fxFakeTerms
	for _, o := range in.OptionRows {
		eod.rows = append(eod.rows, contracts.OptionEod{SessionDay: fixtureDay(t, o.SessionDay), InstrumentID: o.InstrumentID, UnderlyingID: int(o.UnderlyingID)})
		terms = append(terms, store.OptionTerms{InstrumentID: int64(o.InstrumentID), Right: o.Right, Strike: o.Strike, ExpirationDate: contracts.Day(o.Expiry)})
	}
	var quotes fxFakeQuotes
	for _, q := range in.Quotes {
		last := q.Last
		quotes = append(quotes, contracts.QuoteSnapshot{SessionDay: fixtureDay(t, q.SessionDay), InstrumentID: q.InstrumentID, Last: &last})
	}
	return view, companiesInputs{
		Facts: fxFakeFacts(in.Facts), Series: fxFakePoints(in.Series), Catalog: catalog, Names: names, Eod: eod, Terms: terms, Quotes: quotes,
		Bars: fxFakeBars{},
	}
}

func fxFourLines(costs []string, stays, earns string) []valuationapi.ReturnLine {
	var lines []valuationapi.ReturnLine
	for _, c := range costs {
		lines = append(lines,
			valuationapi.ReturnLine{Cost: c, Key: valuationapi.LineSpendingStays, State: valuationapi.StateOK, Value: stays},
			valuationapi.ReturnLine{Cost: c, Key: valuationapi.LineSpendingEarns, State: valuationapi.StateOK, Value: earns},
			valuationapi.ReturnLine{Cost: c, Key: valuationapi.LineAnalystPath, State: "left_out", Reason: "no analyst input"},
			valuationapi.ReturnLine{Cost: c, Key: valuationapi.LineSpendingEarnsAnalyst, State: "left_out", Reason: "no analyst input"},
		)
	}
	return lines
}

// fixtureFuncs return the figures of the fixture for SYN1 (US, told by its SEC facts) and SYN2 (JP, EDINET).
func fixtureFuncs() valuationFuncs {
	jp := func(f valuationapi.Filings) bool { return f.Currency == "JPY" }
	miss := func(k, r string) valuationapi.Question {
		return valuationapi.Question{Key: k, State: valuationapi.StateMissingInput, Reason: r}
	}
	ok := func(k, v string) valuationapi.Question {
		return valuationapi.Question{Key: k, State: valuationapi.StateOK, Value: v}
	}
	dok := func(k, v string) valuationapi.DiagnosisFigure {
		return valuationapi.DiagnosisFigure{Key: k, State: valuationapi.StateOK, Value: v}
	}
	return valuationFuncs{
		StandardOf: func(f []contracts.FundamentalFact) valuationapi.Standard {
			if len(f) > 0 && string(f[0].Source) == "edinet" {
				return valuationapi.JGAAP
			}
			return valuationapi.USGAAP
		},
		BuildFilings: func(_ []contracts.FundamentalFact, _ contracts.Day, s valuationapi.Standard) valuationapi.FilingsResult {
			cur := "USD"
			if s == valuationapi.JGAAP {
				cur = "JPY"
			}
			return valuationapi.FilingsResult{State: valuationapi.StateOK, Filings: valuationapi.Filings{
				Standard: s, Currency: cur, Years: []valuationapi.FilingYear{{FYEnd: "2025-09-30"}},
			}}
		},
		Band: func(id string, f valuationapi.Filings, _ contracts.Day) valuationapi.BandFigure {
			if jp(f) {
				return valuationapi.BandFigure{ID: id, State: valuationapi.StateMissingInput, Reasons: []string{"cyclical"}}
			}
			return valuationapi.BandFigure{ID: id, State: valuationapi.StateOK, Value: "142.50", Low: "114.00", RefHigh: "190.00"}
		},
		ImpliedGrowth: func(f valuationapi.Filings, _ string) valuationapi.ImpliedResult {
			if jp(f) {
				return valuationapi.ImpliedResult{State: valuationapi.StateMissingInput}
			}
			return valuationapi.ImpliedResult{State: valuationapi.StateOK, Primary: "0.041", Rates: []valuationapi.ImpliedRate{
				{Rate: "0.08", Growth: "0.041"}, {Rate: "0.10", Growth: "0.027"}, {Rate: "0.12", Growth: "0.014"},
			}}
		},
		ReturnAtPrice: func(f valuationapi.Filings, costs []string, _ *valuationapi.Analyst, _ contracts.Day) valuationapi.ReturnResult {
			if jp(f) {
				return valuationapi.ReturnResult{State: valuationapi.StateMissingInput, Reasons: []string{"no diluted share count"}}
			}
			return valuationapi.ReturnResult{State: valuationapi.StateOK, Lines: fxFourLines(costs, "0.062", "0.084")}
		},
		Questions: func(f valuationapi.Filings, _ contracts.Day) valuationapi.QuestionsResult {
			if jp(f) {
				return valuationapi.QuestionsResult{Questions: []valuationapi.Question{
					miss("Q1", "no tangible capital"), miss("Q2", "no tangible capital"), miss("Q3", "no gross margin"), miss("Q4", "no operating cash flow"),
				}}
			}
			return valuationapi.QuestionsResult{Questions: []valuationapi.Question{ok("Q1", "0.182"), ok("Q2", "0.41"), ok("Q3", "0.065"), ok("Q4", "1.12")}}
		},
		Diagnosis: func(f valuationapi.Filings, _ contracts.Day) valuationapi.DiagnosisResult {
			if !jp(f) {
				return valuationapi.DiagnosisResult{}
			}
			return valuationapi.DiagnosisResult{Group: "cyclical", Figures: []valuationapi.DiagnosisFigure{
				dok("operating_margin@2025-03-31", "0.072"), dok("inventory_to_revenue@2025-03-31", "0.18"), dok("revenue@2025-03-31", "1200000"),
				dok("operating_margin@2024-03-31", "0.061"), dok("inventory_to_revenue@2024-03-31", "0.16"), dok("revenue@2024-03-31", "1100000"),
				dok("ranked", "10"), dok("latest_rank", "3"),
				dok("operating_margin_usual_low", "0.06"), dok("operating_margin_usual_high", "0.09"),
				dok("inventory_to_revenue_latest", "0.18"),
			}}
		},
	}
}

// fixtureBase is the part of the Config that entry 24 sets: Read, Subjects, Today and Zone.
func fixtureBase(in fixtureInput, view store.ReadingViewReader) companies_page.Config {
	cfg := companies_page.Config{Today: func() contracts.Day { return contracts.Day(in.Today) }, Read: view, Text: fxFakeOwnerText(in.OwnerText)}
	for _, s := range in.Subjects {
		cfg.Subjects = append(cfg.Subjects, companies_page.Subject{Issuer: s.Issuer, Market: s.Market, Symbol: s.Symbol, HandPrice: s.HandPrice})
	}
	return cfg
}

func TestPageOverFixtureInputs(t *testing.T) {
	t.Parallel()
	in := loadFixtureInput(t)
	view, inputs := fixtureStores(t, in)
	cfg := newCompaniesPage(fixtureBase(in, view), inputs, fixtureFuncs())
	got, err := companies_page.Build(t.Context(), cfg, time.Date(2026, 11, 10, 14, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	var page map[string]json.RawMessage
	if err := json.Unmarshal(got, &page); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join("..", "..", "contracts", "fixtures", "page.companies_page.v2.ok.json"))
	if err != nil {
		t.Fatal(err)
	}
	var want map[string]json.RawMessage
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"core", "diagnosis", "questions"} {
		var w bytes.Buffer
		if err := json.Compact(&w, want[key]); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(page[key], w.Bytes()) {
			t.Errorf("%s differs\n got: %s\nwant: %s", key, page[key], w.Bytes())
		}
	}
}

func TestSymbolResolvedViaCatalog(t *testing.T) {
	t.Parallel()
	in := loadFixtureInput(t)
	_, inputs := fixtureStores(t, in)
	// a non-stock instrument of the same name and an id the catalog lists but that has no name do not match first.
	inputs.Catalog = append(inputs.Catalog.(fxFakeCatalog), 900001, 900002)
	inputs.Names = append(inputs.Names.(fxFakeNames), store.InstrumentName{InstrumentID: 900001, Type: "index", Name: "SYN1"})
	cfg := newCompaniesPage(companies_page.Config{}, inputs, valuationFuncs{})
	day := contracts.Day(in.Today)
	spot, ok, err := cfg.Spot(t.Context(), "SYN1", day)
	if err != nil || !ok || spot != "128.4" {
		t.Errorf("spot %q %v %v", spot, ok, err)
	}
	offers, err := cfg.Offers(t.Context(), "SYN1", day)
	if err != nil || len(offers) != 1 || offers[0] != "120" {
		t.Errorf("offers %v %v", offers, err)
	}
	offers, err = cfg.Offers(t.Context(), "SYN2", day)
	if err != nil || len(offers) != 1 || offers[0] != "2500" {
		t.Errorf("SYN2 offers %v %v", offers, err)
	}
}

func TestUnknownSymbolGivesGap(t *testing.T) {
	t.Parallel()
	in := loadFixtureInput(t)
	_, inputs := fixtureStores(t, in)
	cfg := newCompaniesPage(companies_page.Config{}, inputs, valuationFuncs{})
	spot, ok, err := cfg.Spot(t.Context(), "NOPE", contracts.Day(in.Today))
	if err != nil || ok || spot != "" {
		t.Errorf("spot %q %v %v", spot, ok, err)
	}
	if offers, err := cfg.Offers(t.Context(), "NOPE", contracts.Day(in.Today)); err != nil || offers != nil {
		t.Errorf("offers %v %v", offers, err)
	}
	// without the catalog readers the page has no Spot and no Offers at all: a gap on every row.
	bare := newCompaniesPage(companies_page.Config{}, companiesInputs{}, valuationFuncs{})
	if bare.Spot != nil || bare.Offers != nil || bare.Facts != nil || bare.Series != nil {
		t.Error("a nil reader set a field")
	}
}
