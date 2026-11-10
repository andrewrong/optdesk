package companies_page

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
)

var builtAt = time.Date(2026, 11, 10, 14, 0, 0, 0, time.UTC)

// buildBlocks builds the page and returns its top-level keys as raw JSON.
func buildBlocks(t *testing.T, cfg Config) map[string]json.RawMessage {
	t.Helper()
	b, err := Build(context.Background(), cfg, builtAt)
	if err != nil {
		t.Fatal(err)
	}
	var page map[string]json.RawMessage
	if err := json.Unmarshal(b, &page); err != nil {
		t.Fatal(err)
	}
	return page
}

type fakeSeries struct {
	points   []contracts.SeriesPoint
	id       string
	from, to contracts.Day
}

func (f *fakeSeries) SeriesPoints(_ context.Context, id string, from, to contracts.Day) ([]contracts.SeriesPoint, error) {
	f.id, f.from, f.to = id, from, to
	return f.points, nil
}

func point(day string, v float64) contracts.SeriesPoint {
	d, err := time.Parse(dayLayout, day)
	if err != nil {
		panic(err)
	}
	return contracts.SeriesPoint{SeriesID: "DGS10", ObservationDate: types.SerializableDate{Time: d}, Status: "present", Value: &v}
}

// coreConfig is a ready config: filings with one year, a quote, the given offers and no
// analyst; the return function gives four lines per cost.
func coreConfig(costs ...string) Config {
	cfg := testConfig(okBand)
	cfg.Spot = func(context.Context, string, contracts.Day) (string, bool, error) { return "128.40", true, nil }
	cfg.Offers = func(context.Context, string, contracts.Day) ([]string, error) { return costs, nil }
	cfg.BuildFilings = func([]contracts.FundamentalFact, contracts.Day, valuationapi.Standard) valuationapi.FilingsResult {
		return valuationapi.FilingsResult{State: valuationapi.StateOK, Filings: valuationapi.Filings{
			Years: []valuationapi.FilingYear{{FYEnd: "2025-12-31"}},
		}}
	}
	cfg.ReturnAtPrice = func(_ valuationapi.Filings, costs []string, _ *valuationapi.Analyst, _ contracts.Day) valuationapi.ReturnResult {
		return valuationapi.ReturnResult{State: valuationapi.StateOK, Lines: fourLines(costs, "0.062", "0.084")}
	}
	cfg.Series = &fakeSeries{points: []contracts.SeriesPoint{point("2026-11-09", 4.15)}}
	return cfg
}

func fourLines(costs []string, stays, earns string) []valuationapi.ReturnLine {
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

type testCore struct {
	Reason   string `json:"reason"`
	Text     string `json:"text"`
	Treasury struct {
		Reason        string  `json:"reason"`
		Text          string  `json:"text"`
		MarkerPercent float64 `json:"marker_percent"`
	} `json:"treasury"`
	JpRate struct {
		Reason string `json:"reason"`
	} `json:"jp_rate"`
	Rows []struct {
		Symbol    string `json:"symbol"`
		PriceText string `json:"price_text"`
		Returns   []struct {
			Assumption    string  `json:"assumption"`
			Text          string  `json:"text"`
			MarkerPercent float64 `json:"marker_percent"`
		} `json:"returns"`
	} `json:"rows"`
}

func coreOf(t *testing.T, cfg Config) testCore {
	t.Helper()
	var c testCore
	if err := json.Unmarshal(buildBlocks(t, cfg)["core"], &c); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestStandardChosenFromFacts(t *testing.T) {
	t.Parallel()
	cfg := coreConfig("120")
	var got valuationapi.Standard
	cfg.StandardOf = func([]contracts.FundamentalFact) valuationapi.Standard { return valuationapi.IFRS }
	build := cfg.BuildFilings
	cfg.BuildFilings = func(f []contracts.FundamentalFact, d contracts.Day, s valuationapi.Standard) valuationapi.FilingsResult {
		got = s
		return build(f, d, s)
	}
	coreOf(t, cfg)
	if got != valuationapi.IFRS {
		t.Fatalf("BuildFilings saw standard %q, want IFRS", got)
	}
}

func TestCostIsTheBareStrike(t *testing.T) {
	t.Parallel()
	cfg := coreConfig("120", "90.5")
	var costs []string
	ret := cfg.ReturnAtPrice
	cfg.ReturnAtPrice = func(f valuationapi.Filings, c []string, a *valuationapi.Analyst, d contracts.Day) valuationapi.ReturnResult {
		costs = c
		return ret(f, c, a, d)
	}
	core := coreOf(t, cfg)
	if len(costs) != 2 || costs[0] != "120" || costs[1] != "90.5" {
		t.Fatalf("costs = %v, want the strikes as given", costs)
	}
	if len(core.Rows) != 2 || core.Rows[0].PriceText != "120" || core.Rows[1].PriceText != "90.5" {
		t.Fatalf("rows = %+v", core.Rows)
	}
	if r := core.Rows[0].Returns; len(r) != 4 || r[0].Text != "6.2% a year" || r[0].MarkerPercent != 31 || r[1].MarkerPercent != 42 {
		t.Fatalf("returns = %+v", r)
	}
}

func TestNoAnalystLeavesTwoOut(t *testing.T) {
	t.Parallel()
	cfg := coreConfig("120")
	analyst := &valuationapi.Analyst{}
	ret := cfg.ReturnAtPrice
	cfg.ReturnAtPrice = func(f valuationapi.Filings, c []string, a *valuationapi.Analyst, d contracts.Day) valuationapi.ReturnResult {
		analyst = a
		return ret(f, c, a, d)
	}
	r := coreOf(t, cfg).Rows[0].Returns
	if analyst != nil {
		t.Fatalf("analyst = %+v, want nil", analyst)
	}
	if r[2].Text != "left out: no analyst input" || r[3].Text != "left out: no analyst input" ||
		r[2].MarkerPercent != 0 || r[3].MarkerPercent != 0 || r[2].Assumption != "Analyst path" ||
		r[3].Assumption != "Spending earns, analyst path" {
		t.Fatalf("returns = %+v", r)
	}
}

func TestTreasuryStaleGivesGap(t *testing.T) {
	t.Parallel()
	cfg := coreConfig("120")
	s := &fakeSeries{points: []contracts.SeriesPoint{point("2026-11-02", 4.0)}}
	cfg.Series = s
	core := coreOf(t, cfg)
	if core.Treasury.Reason != "stale" || core.Treasury.Text == "" {
		t.Fatalf("treasury = %+v, want a stale gap", core.Treasury)
	}
	if s.id != "DGS10" || s.from != "2026-11-03" || s.to != "2026-11-10" {
		t.Fatalf("read %s %s..%s", s.id, s.from, s.to)
	}
	cfg.Series = &fakeSeries{points: []contracts.SeriesPoint{point("2026-11-05", 4.0), point("2026-11-09", 4.15), point("2026-11-08", 3.0)}}
	core = coreOf(t, cfg)
	if core.Treasury.Text != "US 10-year 4.15%" || core.Treasury.MarkerPercent != 41.5 {
		t.Fatalf("treasury = %+v, want the newest point", core.Treasury)
	}
}

func TestJpHasRateGap(t *testing.T) {
	t.Parallel()
	cfg := coreConfig("2500")
	cfg.Subjects = []Subject{{Issuer: "edinet:E00001", Market: "JP", Symbol: "SYN2"}}
	s := &fakeSeries{points: []contracts.SeriesPoint{point("2026-11-09", 4.15)}}
	cfg.Series = s
	core := coreOf(t, cfg)
	if core.JpRate.Reason != "no_data" || core.Treasury.Reason == "" || s.id != "" {
		t.Fatalf("core = %+v, series read %q: a JP-only page has a rate gap and no US yield", core, s.id)
	}
	cfg = coreConfig("120")
	if core := coreOf(t, cfg); core.JpRate.Reason != "no_data" || core.Treasury.Text == "" {
		t.Fatalf("US page core = %+v", core)
	}
}

func TestRowsKeepCallerOrder(t *testing.T) {
	t.Parallel()
	cfg := coreConfig("1")
	cfg.Subjects = []Subject{
		{Issuer: "cik:0000000009", Market: "US", Symbol: "ZZZ"},
		{Issuer: "cik:0000000001", Market: "US", Symbol: "AAA"},
		{Issuer: "cik:0000000005", Market: "US", Symbol: "MMM"},
	}
	core := coreOf(t, cfg)
	var got []string
	for _, r := range core.Rows {
		got = append(got, r.Symbol)
	}
	if len(got) != 3 || got[0] != "ZZZ" || got[1] != "AAA" || got[2] != "MMM" {
		t.Fatalf("rows = %v, want ZZZ AAA MMM", got)
	}
}

func TestNilOffersGivesGap(t *testing.T) {
	t.Parallel()
	cfg := coreConfig("120")
	cfg.Offers = nil
	core := coreOf(t, cfg)
	if core.Reason != "no_reader" || core.Text == "" || len(core.Rows) != 0 {
		t.Fatalf("core = %+v, want a no_reader gap", core)
	}
}

// input is the part of the input fixture the builder reads.
type input struct {
	Today    string `json:"today"`
	Subjects []struct {
		Issuer    string `json:"issuer"`
		Market    string `json:"market"`
		Symbol    string `json:"symbol"`
		HandPrice string `json:"hand_price"`
	} `json:"subjects"`
	Readings    []fixtureReading            `json:"readings"`
	Facts       []contracts.FundamentalFact `json:"facts"`
	Series      []contracts.SeriesPoint     `json:"series"`
	Instruments []struct {
		ID     int    `json:"id"`
		Symbol string `json:"symbol"`
	} `json:"instruments"`
	OptionRows []struct {
		UnderlyingID int    `json:"underlying_id"`
		Strike       string `json:"strike"`
	} `json:"option_rows"`
	OwnerText map[string]map[string]string `json:"owner_text"`
}

// fixtureReading is the part of a reading the page reads; the generated record does not take
// the fixture's reason_code text.
type fixtureReading struct {
	Name, Subject, Day, State string
	Value                     *string
}

func (r fixtureReading) record() contracts.ReadingRecord {
	d, err := time.Parse(dayLayout, r.Day)
	if err != nil {
		panic(err)
	}
	rec := contracts.ReadingRecord{Name: r.Name, Subject: r.Subject, Day: types.SerializableDate{Time: d}, Value: r.Value}
	if r.State == "ok" { // any other state stays the zero value, which the page does not read
		rec.State = "ok"
	}
	return rec
}

func fixturePath(name string) string {
	return filepath.Join("..", "..", "..", "contracts", "fixtures", name)
}

func loadInput(t *testing.T) input {
	t.Helper()
	b, err := os.ReadFile(fixturePath("page.companies_page.v2.input.json"))
	if err != nil {
		t.Fatal(err)
	}
	var in input
	if err := json.Unmarshal(b, &in); err != nil {
		t.Fatal(err)
	}
	return in
}

type fixtureRead struct{ rows []fixtureReading }

func (f fixtureRead) WinnerReadings(_ context.Context, name, _, subject string, _, _ contracts.Day) ([]contracts.ReadingRecord, error) {
	var out []contracts.ReadingRecord
	for _, r := range f.rows {
		if r.Name == name && r.Subject == subject {
			out = append(out, r.record())
		}
	}
	return out, nil
}

func (fixtureRead) Close() error { return nil }

type fixtureFacts []contracts.FundamentalFact

func (f fixtureFacts) FactsFiledBy(_ context.Context, source, subject string, _ contracts.Day) ([]contracts.FundamentalFact, error) {
	var out []contracts.FundamentalFact
	for _, x := range f {
		if string(x.Source) == source && x.Subject == subject {
			out = append(out, x)
		}
	}
	return out, nil
}

type fixtureText map[string]map[string]string

func (f fixtureText) Moat(_ context.Context, s string) (string, bool, error) {
	t, ok := f["moat"][s]
	return t, ok, nil
}

func (f fixtureText) FifthQuestion(_ context.Context, s string) (string, bool, error) {
	t, ok := f["fifth_question"][s]
	return t, ok, nil
}

// fakeCompute holds the valuationapi values the fake functions return for the two companies of
// the fixture: SYN1 (USD) and SYN2 (JPY), told apart by the currency of their filings.
func fakeCompute(cfg *Config) {
	cfg.StandardOf = func(f []contracts.FundamentalFact) valuationapi.Standard {
		if len(f) > 0 && string(f[0].Source) == "edinet" {
			return valuationapi.JGAAP
		}
		return valuationapi.USGAAP
	}
	cfg.BuildFilings = func(f []contracts.FundamentalFact, _ contracts.Day, s valuationapi.Standard) valuationapi.FilingsResult {
		cur := "USD"
		if s == valuationapi.JGAAP {
			cur = "JPY"
		}
		return valuationapi.FilingsResult{State: valuationapi.StateOK, Filings: valuationapi.Filings{
			Standard: s, Currency: cur, Years: []valuationapi.FilingYear{{FYEnd: "2025-09-30"}},
		}}
	}
	cfg.Band = func(id string, f valuationapi.Filings, _ contracts.Day) valuationapi.BandFigure {
		if f.Currency == "JPY" {
			return valuationapi.BandFigure{ID: id, State: valuationapi.StateMissingInput, Reasons: []string{"cyclical"}}
		}
		return valuationapi.BandFigure{ID: id, State: valuationapi.StateOK, Value: "142.50", Low: "114.00", RefHigh: "190.00"}
	}
	cfg.ImpliedGrowth = func(f valuationapi.Filings, _ string) valuationapi.ImpliedResult {
		if f.Currency == "JPY" {
			return valuationapi.ImpliedResult{State: valuationapi.StateMissingInput}
		}
		return valuationapi.ImpliedResult{State: valuationapi.StateOK, Primary: "0.041", Rates: []valuationapi.ImpliedRate{
			{Rate: "0.08", Growth: "0.041"}, {Rate: "0.10", Growth: "0.027"}, {Rate: "0.12", Growth: "0.014"},
		}}
	}
	cfg.ReturnAtPrice = func(f valuationapi.Filings, costs []string, _ *valuationapi.Analyst, _ contracts.Day) valuationapi.ReturnResult {
		if f.Currency == "JPY" {
			return valuationapi.ReturnResult{State: valuationapi.StateMissingInput, Reasons: []string{"no diluted share count"}}
		}
		return valuationapi.ReturnResult{State: valuationapi.StateOK, Lines: fourLines(costs, "0.062", "0.084")}
	}
	cfg.Questions = func(f valuationapi.Filings, _ contracts.Day) valuationapi.QuestionsResult {
		if f.Currency == "JPY" {
			m := func(k, r string) valuationapi.Question {
				return valuationapi.Question{Key: k, State: valuationapi.StateMissingInput, Reason: r}
			}
			return valuationapi.QuestionsResult{Questions: []valuationapi.Question{
				m("Q1", "no tangible capital"), m("Q2", "no tangible capital"), m("Q3", "no gross margin"), m("Q4", "no operating cash flow"),
			}}
		}
		ok := func(k, v string) valuationapi.Question {
			return valuationapi.Question{Key: k, State: valuationapi.StateOK, Value: v}
		}
		return valuationapi.QuestionsResult{Questions: []valuationapi.Question{
			ok("Q1", "0.182"), ok("Q2", "0.41"), ok("Q3", "0.065"), ok("Q4", "1.12"),
		}}
	}
	cfg.Diagnosis = func(f valuationapi.Filings, _ contracts.Day) valuationapi.DiagnosisResult {
		if f.Currency != "JPY" {
			return valuationapi.DiagnosisResult{}
		}
		ok := func(k, v string) valuationapi.DiagnosisFigure {
			return valuationapi.DiagnosisFigure{Key: k, State: valuationapi.StateOK, Value: v}
		}
		return valuationapi.DiagnosisResult{Group: "cyclical", Figures: []valuationapi.DiagnosisFigure{
			ok("operating_margin@2025-03-31", "0.072"), ok("inventory_to_revenue@2025-03-31", "0.18"), ok("revenue@2025-03-31", "1200000"),
			ok("operating_margin@2024-03-31", "0.061"), ok("inventory_to_revenue@2024-03-31", "0.16"), ok("revenue@2024-03-31", "1100000"),
			ok("ranked", "10"), ok("latest_rank", "3"),
			ok("operating_margin_usual_low", "0.06"), ok("operating_margin_usual_high", "0.09"),
			ok("inventory_to_revenue_latest", "0.18"),
		}}
	}
}

// fixtureConfig builds the Config the input fixture describes, with the fake compute functions.
func fixtureConfig(in input) Config {
	cfg := Config{
		Today: func() contracts.Day { return contracts.Day(in.Today) },
		Read:  fixtureRead{rows: in.Readings}, Facts: fixtureFacts(in.Facts), Text: fixtureText(in.OwnerText),
		Series: &fakeSeries{points: in.Series},
	}
	idOf := map[string]int{}
	for _, s := range in.Subjects {
		cfg.Subjects = append(cfg.Subjects, Subject{Issuer: s.Issuer, Market: s.Market, Symbol: s.Symbol, HandPrice: s.HandPrice})
	}
	for _, i := range in.Instruments {
		idOf[i.Symbol] = i.ID
	}
	cfg.Spot = func(context.Context, string, contracts.Day) (string, bool, error) { return "128.4", true, nil }
	cfg.Offers = func(_ context.Context, symbol string, _ contracts.Day) ([]string, error) {
		var out []string
		for _, o := range in.OptionRows {
			if o.UnderlyingID == idOf[symbol] {
				out = append(out, o.Strike)
			}
		}
		return out, nil
	}
	fakeCompute(&cfg)
	return cfg
}

func TestBuildEqualsOkFixture(t *testing.T) {
	t.Parallel()
	in := loadInput(t)
	page := buildBlocks(t, fixtureConfig(in))
	raw, err := os.ReadFile(fixturePath("page.companies_page.v2.ok.json"))
	if err != nil {
		t.Fatal(err)
	}
	var want map[string]json.RawMessage
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	// Only these three blocks belong to this builder; the rest of the document is another
	// entry's.
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
