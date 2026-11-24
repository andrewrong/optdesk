package company_data_page

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/atombender/go-jsonschema/pkg/types"
	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/andrewrong/optdesk/internal/compute/companydataapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/contracts/gen"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/underlying"
)

const (
	schemaDir  = "../../../contracts/schemas/"
	schemaHost = "https://optdesk.invalid/schemas/"
)

type fileLoader struct{}

func (fileLoader) Load(url string) (any, error) {
	file, ok := strings.CutPrefix(url, schemaHost)
	if !ok || strings.ContainsAny(file, "/\\") {
		return nil, fmt.Errorf("unexpected schema url %q", url)
	}
	b, err := os.ReadFile(schemaDir + file)
	if err != nil {
		return nil, err
	}
	return jsonschema.UnmarshalJSON(bytes.NewReader(b))
}

func validate(t *testing.T, b []byte) {
	t.Helper()
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	c.UseLoader(fileLoader{})
	s, err := c.Compile(schemaHost + "page.company_data_page.v2.json")
	if err != nil {
		t.Fatal(err)
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Validate(inst); err != nil {
		t.Fatalf("%v\n%s", err, b)
	}
}

type fakeUnderlyings struct {
	rows []underlying.Row
	err  error
}

func (f fakeUnderlyings) Current(context.Context) ([]underlying.Row, error) { return f.rows, f.err }
func (f fakeUnderlyings) History(context.Context, int) ([]underlying.Row, error) {
	return nil, nil
}

type fakeBars struct {
	bars    map[int64][]contracts.PriceBar
	failFor int64
	basis   string
	from    contracts.Day
}

func (f *fakeBars) PriceBars(_ context.Context, id int64, basis string, from, _ contracts.Day) ([]contracts.PriceBar, error) {
	f.basis, f.from = basis, from
	if id == f.failFor {
		return nil, errors.New("down")
	}
	return f.bars[id], nil
}

// fakeIDs answers StockIDs from ids (symbol to the rows it returns) and counts the calls and the names asked.
type fakeIDs struct {
	rows  []store.InstrumentName
	calls int
	names [][]string
}

func (f *fakeIDs) StockIDs(_ context.Context, names []string) ([]store.InstrumentName, error) {
	f.calls++
	f.names = append(f.names, names)
	return f.rows, nil
}

type fakeFacts struct{}

func (fakeFacts) FactsWithPeriodFiledBy(context.Context, string, string, contracts.Day) ([]contracts.FundamentalFact, error) {
	return nil, nil
}

type fakeEarnings struct{ rows []contracts.EarningsDate }

func (f fakeEarnings) EarningsDates(context.Context, string, contracts.Day, contracts.Day) ([]contracts.EarningsDate, error) {
	return f.rows, nil
}

type fakeCalc struct {
	sum companydataapi.Summary
	got []companydataapi.SummaryInput
}

func (f *fakeCalc) Quarters([]contracts.FundamentalFact) ([]companydataapi.Quarter, error) {
	return nil, nil
}
func (f *fakeCalc) Summarize(in companydataapi.SummaryInput) (companydataapi.Summary, error) {
	f.got = append(f.got, in)
	return f.sum, nil
}

func day(s string) types.SerializableDate {
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return types.SerializableDate{Time: d}
}

func mkBar(s string) contracts.PriceBar { return contracts.PriceBar{SessionDay: day(s)} }

func dp(s string) *contracts.Day { d := contracts.Day(s); return &d }

var (
	aapl = underlying.Row{Key: "US:AAPL", Symbol: "AAPL", Market: "US", Kind: "stock", Name: "Apple",
		Issuer: "cik:0000320193", ConID: 1, Status: "active", Fundamentals: true}
	msft = underlying.Row{Key: "US:MSFT", Symbol: "MSFT", Market: "US", Kind: "stock", Issuer: "cik:0000789019", ConID: 2, Status: "active"}
)

func summary() companydataapi.Summary {
	return companydataapi.Summary{LastBarDay: dp("2026-11-16"), BarCount: 120, FiscalYearEndMonth: 9, Statements: true,
		LatestLabel: "FY2025 Q4", LatestEnd: dp("2025-09-27"), EPSQuarters: 6,
		Missing: []companydataapi.Missing{{Reason: "no_next_report", Text: "No report date is listed."}}}
}

func newInput(rows []underlying.Row, c *fakeCalc) (Input, *fakeBars) {
	in, b, _ := newInputIDs(rows, c)
	return in, b
}

func newInputIDs(rows []underlying.Row, c *fakeCalc) (Input, *fakeBars, *fakeIDs) {
	ids := &fakeIDs{rows: []store.InstrumentName{{InstrumentID: 1, Type: "stock", Name: "AAPL"}, {InstrumentID: 2, Type: "stock", Name: "MSFT"}}}
	b := &fakeBars{bars: map[int64][]contracts.PriceBar{1: {mkBar("2026-11-14"), mkBar("2026-11-16")}, 2: {mkBar("2026-11-13")}}}
	return Input{Now: time.Date(2026, 11, 17, 9, 0, 0, 0, time.UTC), Today: "2026-11-17",
		Underlyings: fakeUnderlyings{rows: rows}, Bars: b, Facts: fakeFacts{}, Earnings: fakeEarnings{}, Calc: c, StockIDs: ids}, b, ids
}

func build(t *testing.T, in Input) map[string]any {
	t.Helper()
	b, err := Build(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	validate(t, b)
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func companies(m map[string]any) []map[string]any {
	var out []map[string]any
	for _, c := range m["companies"].([]any) {
		out = append(out, c.(map[string]any))
	}
	return out
}

func TestDataBuildValidatesAgainstSchema(t *testing.T) {
	t.Parallel()
	in, b := newInput([]underlying.Row{aapl, msft}, &fakeCalc{sum: summary()})
	m := build(t, in)
	if len(companies(m)) != 2 {
		t.Fatalf("companies = %v", m["companies"])
	}
	if b.basis != "trades" || b.from != "2025-10-13" {
		t.Fatalf("bars read as basis %q from %q", b.basis, b.from)
	}
}

func TestOneRowPerCompany(t *testing.T) {
	t.Parallel()
	retired := underlying.Row{Key: "US:OLD", Kind: "stock", Status: "retired"}
	etf := underlying.Row{Key: "US:SPY", Kind: "etf", Status: "active"}
	in, _ := newInput([]underlying.Row{msft, retired, aapl, etf}, &fakeCalc{sum: summary()})
	cs := companies(build(t, in))
	if len(cs) != 2 || cs[0]["key"] != "US:MSFT" || cs[1]["key"] != "US:AAPL" {
		t.Fatalf("rows = %v", cs)
	}
}

func TestRowCopiesSummary(t *testing.T) {
	t.Parallel()
	in, _ := newInput([]underlying.Row{aapl}, &fakeCalc{sum: summary()})
	c := companies(build(t, in))[0]
	want := map[string]any{"bar_count": "120", "price_to": "2026-11-16", "latest_quarter": "FY2025 Q4",
		"fiscal_year_end": "September", "eps_quarters": "6", "statements": true, "quarters_path": "/company-quarters?c=US%3AAAPL"}
	for k, v := range want {
		if c[k] != v {
			t.Errorf("%s = %v, want %v", k, c[k], v)
		}
	}
}

func TestJPRowSaysLeftOutOnPurpose(t *testing.T) {
	t.Parallel()
	jp := underlying.Row{Key: "JP:7203", Symbol: "7203", Market: "JP", Kind: "stock", Issuer: "edinet:E02144", Status: "active"}
	calc := &fakeCalc{sum: companydataapi.Summary{Missing: []companydataapi.Missing{{Reason: "jp", Text: "日本公司的财报有意不做。"}}}}
	in, _ := newInput([]underlying.Row{jp}, calc)
	c := companies(build(t, in))[0]
	miss := c["missing"].([]any)[0].(map[string]any)
	if miss["code"] != "jp" || !strings.Contains(miss["text"].(string), "有意不做") {
		t.Fatalf("missing = %v", miss)
	}
	if c["price_to"] != nil || c["statements"] != false || c["quarters_path"] != "" {
		t.Fatalf("row = %v", c)
	}
	for _, g := range build(t, in)["glossary"].([]any) {
		if g.(map[string]any)["term"] == "jp" && strings.Contains(g.(map[string]any)["text"].(string), "有意不做") {
			return
		}
	}
	t.Fatal("glossary jp entry does not say 有意不做")
}

func TestMissingListKeepsReasonOrder(t *testing.T) {
	t.Parallel()
	s := companydataapi.Summary{Missing: []companydataapi.Missing{
		{Reason: "no_instrument", Text: "a"}, {Reason: "no_cik", Text: "b"}, {Reason: "no_next_report", Text: "c"}}}
	in, _ := newInput([]underlying.Row{aapl}, &fakeCalc{sum: s})
	var got []string
	for _, m := range companies(build(t, in))[0]["missing"].([]any) {
		got = append(got, m.(map[string]any)["code"].(string))
	}
	if strings.Join(got, ",") != "no_instrument,no_cik,no_next_report" {
		t.Fatalf("order = %v", got)
	}
}

func TestReaderErrorIsGapNotEmptyRow(t *testing.T) {
	t.Parallel()
	in, b := newInput([]underlying.Row{aapl, msft}, &fakeCalc{sum: summary()})
	b.failFor = 1
	m := build(t, in)
	cs := companies(m)
	if len(cs) != 1 || cs[0]["key"] != "US:MSFT" {
		t.Fatalf("rows = %v", cs)
	}
	gaps := m["gaps"].([]any)
	if len(gaps) != 1 || !strings.Contains(gaps[0].(map[string]any)["text"].(string), "US:AAPL") {
		t.Fatalf("gaps = %v", gaps)
	}
}

func TestUnderlyingsErrorIsGap(t *testing.T) {
	t.Parallel()
	in, _ := newInput(nil, &fakeCalc{})
	in.Underlyings = fakeUnderlyings{err: errors.New("down")}
	m := build(t, in)
	if len(companies(m)) != 0 || len(m["gaps"].([]any)) != 1 {
		t.Fatalf("doc = %v", m)
	}
}

func listed(report string) contracts.EarningsDate {
	d := day(report)
	return contracts.EarningsDate{Status: gen.EarningsDateStatusListed, ReportDate: &d, ObservedOn: day("2026-11-16")}
}

func TestNextReportEmptyWhenNoneListed(t *testing.T) {
	t.Parallel()
	in, _ := newInput([]underlying.Row{aapl}, &fakeCalc{sum: summary()})
	in.Earnings = fakeEarnings{rows: []contracts.EarningsDate{{Status: gen.EarningsDateStatusAbsent}}}
	if got := companies(build(t, in))[0]["next_report"]; got != nil {
		t.Fatalf("next_report = %v", got)
	}
	in.Earnings = fakeEarnings{rows: []contracts.EarningsDate{listed("2026-11-10")}}
	if got := companies(build(t, in))[0]["next_report"]; got != nil {
		t.Fatalf("a listed date before today gave %v", got)
	}
	in.Earnings = fakeEarnings{rows: []contracts.EarningsDate{listed("2026-12-01"), listed("2026-12-03")}}
	if got := companies(build(t, in))[0]["next_report"]; got != "2026-12-03" {
		t.Fatalf("newest listed row gave %v", got)
	}
}

func TestNoForm(t *testing.T) {
	t.Parallel()
	in, _ := newInput([]underlying.Row{aapl}, &fakeCalc{sum: summary()})
	m := build(t, in)
	if _, ok := m["form"]; ok {
		t.Fatal("document has a form key")
	}
	if m["data_health_path"] != "/data-health" {
		t.Fatalf("data_health_path = %v", m["data_health_path"])
	}
}

func TestGlossaryCoversEveryTerm(t *testing.T) {
	t.Parallel()
	in, _ := newInput(nil, &fakeCalc{})
	terms := map[string]string{}
	for _, g := range build(t, in)["glossary"].([]any) {
		gm := g.(map[string]any)
		terms[gm["term"].(string)] = gm["text"].(string)
	}
	want := []string{"Fiscal year end", "Price up to", "Daily bars", "Statements", "Latest quarter", "EPS quarters", "Next report",
		"jp", "no_cik", "fundamentals_off", "no_statements", "annual_only", "no_instrument", "no_bars", "no_next_report"}
	for _, w := range want {
		txt := terms[w]
		if txt == "" || strings.Count(txt, "。") != 1 || !strings.HasSuffix(txt, "。") {
			t.Errorf("term %q text %q is not one sentence", w, txt)
		}
	}
	if len(terms) != len(want) {
		t.Errorf("glossary has %d terms, want %d", len(terms), len(want))
	}
}

type windowEarnings struct{ rows []contracts.EarningsDate }

func (f windowEarnings) EarningsDates(_ context.Context, _ string, from, to contracts.Day) ([]contracts.EarningsDate, error) {
	out := []contracts.EarningsDate{}
	for _, r := range f.rows {
		d := contracts.Day(r.ObservedOn.Format("2006-01-02"))
		if d >= from && d <= to {
			out = append(out, r)
		}
	}
	return out, nil
}

func TestNextReportFromYesterdaysObservation(t *testing.T) {
	t.Parallel()
	in, _ := newInput([]underlying.Row{aapl}, &fakeCalc{sum: summary()})
	r := listed("2026-12-01")
	r.ObservedOn = day("2026-11-16") // the day before Today (2026-11-17)
	in.Earnings = windowEarnings{rows: []contracts.EarningsDate{r}}
	if got := companies(build(t, in))[0]["next_report"]; got != "2026-12-01" {
		t.Fatalf("next_report = %v, want 2026-12-01", got)
	}
}

func TestBarsSourceMixed(t *testing.T) {
	t.Parallel()
	in, b := newInput([]underlying.Row{aapl}, &fakeCalc{sum: summary()})
	yf := mkBar("2026-11-16")
	yf.Source = "yfinance"
	b.bars[1] = []contracts.PriceBar{mkBar("2026-11-14"), yf}
	m := build(t, in) // build validates against the v2 schema
	if m["schema_version"] != float64(2) || m["bars_source"] != "mixed" || m["bars_source_note"] == "" {
		t.Fatalf("source = %v %v, version %v", m["bars_source"], m["bars_source_note"], m["schema_version"])
	}
	in, _ = newInput([]underlying.Row{aapl}, &fakeCalc{sum: summary()})
	ok := build(t, in)
	if _, has := ok["bars_source_note"]; has || ok["bars_source"] != nil {
		t.Fatalf("all-ibkr page carries a source: %v", ok["bars_source"])
	}
}

func TestEmptyNameShowsSymbol(t *testing.T) {
	t.Parallel()
	jp := underlying.Row{Key: "JP:7203", Symbol: "7203", Market: "JP", Kind: "stock", Issuer: "edinet:E02144", Status: "active"}
	in, _ := newInput([]underlying.Row{msft, jp, aapl}, &fakeCalc{sum: summary()})
	cs := companies(build(t, in))
	for i, want := range []string{"MSFT", "7203", "Apple"} {
		if cs[i]["name"] != want {
			t.Errorf("row %d name = %v, want %q", i, cs[i]["name"], want)
		}
	}
}

func TestBarsReadByResolvedInstrumentID(t *testing.T) {
	t.Parallel()
	in, b, ids := newInputIDs([]underlying.Row{aapl}, &fakeCalc{})
	ids.rows = []store.InstrumentName{{InstrumentID: 77, Type: "stock", Name: "AAPL"}}
	b.bars = map[int64][]contracts.PriceBar{77: {mkBar("2026-11-13"), mkBar("2026-11-16")}}
	in.Calc = &countingCalc{}
	c := companies(build(t, in))[0]
	if c["bar_count"] != "2" {
		t.Fatalf("bar_count = %v", c["bar_count"])
	}
}

// countingCalc reports the number of bar days it was given as BarCount.
type countingCalc struct{ fakeCalc }

func (*countingCalc) Summarize(in companydataapi.SummaryInput) (companydataapi.Summary, error) {
	return companydataapi.Summary{BarCount: len(in.BarDays)}, nil
}

type countBars struct{ calls int }

func (f *countBars) PriceBars(context.Context, int64, string, contracts.Day, contracts.Day) ([]contracts.PriceBar, error) {
	f.calls++
	return []contracts.PriceBar{mkBar("2026-11-16")}, nil
}

func TestSharedSymbolReadsNoBars(t *testing.T) {
	t.Parallel()
	in, _, ids := newInputIDs([]underlying.Row{aapl}, &fakeCalc{})
	ids.rows = []store.InstrumentName{{InstrumentID: 1, Name: "AAPL"}, {InstrumentID: 9, Name: "AAPL"}}
	cb := &countBars{}
	in.Bars, in.Calc = cb, &countingCalc{}
	c := companies(build(t, in))[0]
	if c["bar_count"] != "0" || cb.calls != 0 {
		t.Fatalf("bar_count = %v, PriceBars calls = %d", c["bar_count"], cb.calls)
	}
}

func TestNilStockIDsReadsNoBars(t *testing.T) {
	t.Parallel()
	in, _ := newInput([]underlying.Row{aapl}, &fakeCalc{})
	cb := &countBars{}
	in.Bars, in.StockIDs = cb, nil
	c := companies(build(t, in))[0]
	if c["bar_count"] != "0" || cb.calls != 0 {
		t.Fatalf("bar_count = %v, PriceBars calls = %d", c["bar_count"], cb.calls)
	}
}

func TestStockIDsReadOncePerBatchOf500(t *testing.T) {
	t.Parallel()
	var rows []underlying.Row
	for i := 0; i < 1001; i++ {
		s := fmt.Sprintf("S%d", i)
		rows = append(rows, underlying.Row{Key: "US:" + s, Symbol: s, Market: "US", Kind: "stock", Status: "active"})
	}
	in, _, ids := newInputIDs(rows, &fakeCalc{})
	build(t, in)
	if ids.calls != 3 || len(ids.names[0]) != 500 || len(ids.names[2]) != 1 {
		t.Fatalf("calls = %d, batches %d", ids.calls, len(ids.names))
	}
}

var oldEnglish = []string{"the month in which", "the day of the newest", "how many daily", "whether at least", "the newest fiscal quarter",
	"how many quarters", "the date of the next", "what is not stored", "The month in which", "The day of the newest", "How many",
	"Yes when", "The newest fiscal", "The date of the next", "A Japanese company", "A US company", "The statements flag",
	"No US-GAAP", "The filer reports", "The broker has not", "The company is resolved", "No report date",
	"could not be read"}

func hasHan(s string) bool {
	for _, r := range s {
		if r >= 0x4e00 && r <= 0x9fff {
			return true
		}
	}
	return false
}

func TestCompanyDataTextsAreChinese(t *testing.T) {
	t.Parallel()
	check := func(what, s string) {
		t.Helper()
		if !hasHan(s) {
			t.Errorf("%s %q holds no Chinese character", what, s)
		}
		for _, e := range oldEnglish {
			if strings.Contains(s, e) {
				t.Errorf("%s %q holds the old English %q", what, s, e)
			}
		}
	}
	in, b := newInput([]underlying.Row{aapl, msft}, &fakeCalc{sum: summary()})
	b.failFor = 1
	m := build(t, in)
	for _, c := range m["columns"].([]any) {
		cm := c.(map[string]any)
		check("gloss", cm["gloss"].(string))
		if hasHan(cm["label"].(string)) || hasHan(cm["key"].(string)) {
			t.Errorf("label or key %v holds Chinese", cm)
		}
	}
	if len(m["columns"].([]any)) != 8 {
		t.Errorf("columns = %d, want 8", len(m["columns"].([]any)))
	}
	for _, g := range m["glossary"].([]any) {
		gm := g.(map[string]any)
		check("glossary text", gm["text"].(string))
		if hasHan(gm["term"].(string)) {
			t.Errorf("term %q holds Chinese", gm["term"])
		}
	}
	gaps := m["gaps"].([]any)
	if len(gaps) != 1 {
		t.Fatalf("gaps = %v", gaps)
	}
	check("gap", gaps[0].(map[string]any)["text"].(string))
	in2, _ := newInput(nil, &fakeCalc{})
	in2.Underlyings = fakeUnderlyings{err: errors.New("down")}
	g2 := build(t, in2)["gaps"].([]any)
	if got := g2[0].(map[string]any)["text"]; got != "读不到公司列表，所以不显示任何行。" {
		t.Errorf("empty state = %v", got)
	}
	if got := readFailedText("US:AAPL"); got != "US:AAPL：读不到已存的数值，所以这家公司没有行。" {
		t.Errorf("gap = %q", got)
	}
	long := readFailedText(strings.Repeat("甲", 300))
	if n := len([]rune(long)); n != 200 || !utf8.ValidString(long) {
		t.Errorf("cut gap has %d runes, valid=%v", n, utf8.ValidString(long))
	}
}
