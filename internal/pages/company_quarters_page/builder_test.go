package company_quarters_page

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

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/andrewrong/optdesk/internal/compute/companydataapi"
	"github.com/andrewrong/optdesk/internal/contracts"
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
	s, err := c.Compile(schemaHost + "page.company_quarters_page.v1.json")
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

type fakeFacts struct {
	called  bool
	source  string
	subject string
	cutoff  contracts.Day
}

func (f *fakeFacts) FactsWithPeriodFiledBy(_ context.Context, source, subject string, cutoff contracts.Day) ([]contracts.FundamentalFact, error) {
	f.called, f.source, f.subject, f.cutoff = true, source, subject, cutoff
	return nil, nil
}

type fakeCalc struct{ quarters []companydataapi.Quarter }

func (f fakeCalc) Quarters([]contracts.FundamentalFact) ([]companydataapi.Quarter, error) {
	return f.quarters, nil
}
func (f fakeCalc) Summarize(companydataapi.SummaryInput) (companydataapi.Summary, error) {
	return companydataapi.Summary{}, errors.New("not used")
}

func val(v string) companydataapi.Cell {
	return companydataapi.Cell{Value: v, Concept: "C"}
}

func quarters() []companydataapi.Quarter {
	q4 := companydataapi.Quarter{Label: "FY2025 Q4", FiscalYear: 2025, FiscalYearEndMonth: 9, Number: 4,
		PeriodEnd: "2025-09-27", FilingDate: "2025-10-31",
		Revenue: val("94930000000"), NetIncome: val("-20.20"), EPSDiluted: val("1.46"),
		FreeCash: companydataapi.Cell{Value: "1234567.80", Derived: true, Concept: "A - B"}}
	q4.EPSDiluted.Derived = true
	q4.Capex = companydataapi.Cell{Missing: true, Why: companydataapi.WhyNoFact}
	q3 := companydataapi.Quarter{Label: "FY2025 Q3", FiscalYear: 2025, FiscalYearEndMonth: 9, Number: 3,
		PeriodEnd: "2025-06-28", FilingDate: "2025-08-01", Revenue: val("85777000000")}
	return []companydataapi.Quarter{q4, q3}
}

func in(key string, rows []underlying.Row, qs []companydataapi.Quarter) (Input, *fakeFacts) {
	f := &fakeFacts{}
	return Input{Now: time.Date(2026, 11, 17, 9, 0, 0, 0, time.UTC), Today: "2026-11-17", Key: key,
		Underlyings: fakeUnderlyings{rows: rows}, Facts: f, Calc: fakeCalc{quarters: qs}}, f
}

var aapl = underlying.Row{Key: "US:AAPL", Symbol: "AAPL", Market: "US", Kind: "stock", Name: "Apple", Issuer: "cik:0000320193", Status: "active"}

func build(t *testing.T, i Input) map[string]any {
	t.Helper()
	b, err := Build(context.Background(), i)
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

func TestQuartersBuildValidatesAgainstSchema(t *testing.T) {
	t.Parallel()
	i, f := in("US:AAPL", []underlying.Row{aapl}, quarters())
	m := build(t, i)
	if !f.called || f.source != "sec" || f.subject != "cik:0000320193" || f.cutoff != "2026-11-17" {
		t.Fatalf("facts read as %+v", f)
	}
	if n := len(m["quarters"].([]any)); n != 2 {
		t.Fatalf("quarters = %d", n)
	}
	q := m["quarters"].([]any)[0].(map[string]any)
	cells := q["cells"].(map[string]any)
	if got := cells["free_cash"].(map[string]any)["text"]; got != "1,234,567.80" {
		t.Fatalf("free cash = %v", got)
	}
	if got := cells["net_income"].(map[string]any)["text"]; got != "−20.20" {
		t.Fatalf("net income = %v", got)
	}
}

func TestUnknownKeyGivesChoicesAndGap(t *testing.T) {
	t.Parallel()
	etf := underlying.Row{Key: "US:SPY", Kind: "etf", Status: "active"}
	gone := underlying.Row{Key: "US:OLD", Kind: "stock", Status: "retired"}
	for _, key := range []string{"", "US:NOPE"} {
		i, f := in(key, []underlying.Row{aapl, etf, gone}, quarters())
		m := build(t, i)
		if m["company"] != nil || f.called {
			t.Fatalf("key %q: company %v, facts read %v", key, m["company"], f.called)
		}
		ch := m["choices"].([]any)
		if len(ch) != 1 || ch[0].(map[string]any)["path"] != "/company-quarters?c=US%3AAAPL" {
			t.Fatalf("choices = %v", ch)
		}
		if len(m["gaps"].([]any)) != 1 || len(m["quarters"].([]any)) != 0 {
			t.Fatalf("gaps %v quarters %v", m["gaps"], m["quarters"])
		}
	}
}

func TestNoIssuerGivesGapLine(t *testing.T) {
	t.Parallel()
	row := aapl
	row.Issuer = ""
	i, f := in("US:AAPL", []underlying.Row{row}, quarters())
	m := build(t, i)
	gaps := m["gaps"].([]any)
	if len(gaps) != 1 || gaps[0].(map[string]any)["reason"] != "no_cik" {
		t.Fatalf("gaps = %v", gaps)
	}
	if f.called || len(m["quarters"].([]any)) != 0 || m["company"] == nil {
		t.Fatal("no facts may be read and the company stays")
	}
}

func TestMissingCellKeepsWhyText(t *testing.T) {
	t.Parallel()
	i, _ := in("US:AAPL", []underlying.Row{aapl}, quarters())
	m := build(t, i)
	c := m["quarters"].([]any)[0].(map[string]any)["cells"].(map[string]any)["capex"].(map[string]any)
	if c["missing"] != true || c["text"] != "" || c["why_text"] == "" {
		t.Fatalf("capex cell = %v", c)
	}
}

func TestDerivedEPSCellCarriesGloss(t *testing.T) {
	t.Parallel()
	i, _ := in("US:AAPL", []underlying.Row{aapl}, quarters())
	m := build(t, i)
	c := m["quarters"].([]any)[0].(map[string]any)["cells"].(map[string]any)["eps_diluted"].(map[string]any)
	want := "annual minus the first three quarters; may differ from the reported Q4 EPS by rounding or a change in the share count"
	if c["derived"] != true || c["why_text"] != want || c["text"] != "1.46" {
		t.Fatalf("eps cell = %v", c)
	}
}

func TestFiscalYearEndTextInHeader(t *testing.T) {
	t.Parallel()
	i, _ := in("US:AAPL", []underlying.Row{aapl}, quarters())
	m := build(t, i)
	if got := m["company"].(map[string]any)["fiscal_year_end"]; got != "September" {
		t.Fatalf("fiscal_year_end = %v", got)
	}
}

func TestEmptyLabelNoteBeforeRefetch(t *testing.T) {
	t.Parallel()
	qs := quarters()
	qs[0].Label, qs[1].Label = "", ""
	i, _ := in("US:AAPL", []underlying.Row{aapl}, qs)
	m := build(t, i)
	rows := m["quarters"].([]any)
	if rows[0].(map[string]any)["label"] != "Quarter ending 2025-09-27" || rows[1].(map[string]any)["label"] != "Quarter ending 2025-06-28" {
		t.Fatalf("labels = %v", rows)
	}
	notes := m["notes"].([]any)
	if len(notes) != 1 || notes[0] != noteNoLabel {
		t.Fatalf("notes = %v", notes)
	}
	qs = quarters()
	qs[1].Label = ""
	i, _ = in("US:AAPL", []underlying.Row{aapl}, qs)
	m = build(t, i)
	if m["quarters"].([]any)[0].(map[string]any)["label"] != "FY2025 Q4" {
		t.Fatal("a quarter with a label keeps it")
	}
	qs = quarters()
	i, _ = in("US:AAPL", []underlying.Row{aapl}, qs)
	if n := len(build(t, i)["notes"].([]any)); n != 0 {
		t.Fatalf("notes = %d with every label set", n)
	}
}

func TestEveryColumnHasGloss(t *testing.T) {
	t.Parallel()
	i, _ := in("US:AAPL", []underlying.Row{aapl}, quarters())
	m := build(t, i)
	glosses := map[string]string{}
	for _, c := range m["columns"].([]any) {
		cm := c.(map[string]any)
		g := cm["gloss"].(string)
		if g == "" || strings.Count(g, ". ") > 0 {
			t.Errorf("column %v gloss %q is not one sentence", cm["key"], g)
		}
		glosses[cm["label"].(string)] = g
	}
	for _, l := range []string{labelFilingDate, labelDilutedEPS, labelDilutedShares, labelFreeCashFlow, labelCapex, labelBuybacks} {
		if glosses[l] == "" {
			t.Errorf("no gloss for %s", l)
		}
	}
	if glosses[labelFilingDate] != "the day the report was filed with the SEC; usually a few days after the earnings release; "+
		"your own report-day record is not changed" {
		t.Errorf("filing date gloss = %q", glosses[labelFilingDate])
	}
}

func TestReadErrorIsGap(t *testing.T) {
	t.Parallel()
	i, _ := in("US:AAPL", nil, nil)
	i.Underlyings = fakeUnderlyings{err: errors.New("down")}
	m := build(t, i)
	if len(m["gaps"].([]any)) != 1 {
		t.Fatalf("gaps = %v", m["gaps"])
	}
}

func hasHan(s string) bool {
	for _, r := range s {
		if r >= 0x4e00 && r <= 0x9fff {
			return true
		}
	}
	return false
}

func TestGapAndReasonTextsAreChinese(t *testing.T) {
	t.Parallel()
	jp := aapl
	jp.Market = "JP"
	noCIK := aapl
	noCIK.Issuer = ""
	rows := []underlying.Row{aapl}
	gapCases := map[string]func() Input{
		"unknown": func() Input { i, _ := in("US:NOPE", rows, quarters()); return i },
		"no_cik":  func() Input { i, _ := in("US:AAPL", []underlying.Row{noCIK}, quarters()); return i },
		"jp":      func() Input { i, _ := in("US:AAPL", []underlying.Row{jp}, quarters()); return i },
		"empty":   func() Input { i, _ := in("US:AAPL", rows, nil); return i },
		"failed": func() Input {
			i, _ := in("US:AAPL", nil, nil)
			i.Underlyings = fakeUnderlyings{err: errors.New("down")}
			return i
		},
	}
	for name, mk := range gapCases {
		gaps := build(t, mk())["gaps"].([]any)
		if len(gaps) != 1 {
			t.Fatalf("%s: gaps = %v", name, gaps)
		}
		if text := gaps[0].(map[string]any)["text"].(string); !hasHan(text) {
			t.Errorf("%s: gap text %q has no Chinese character", name, text)
		}
	}
	for _, why := range []companydataapi.Why{companydataapi.WhyNoFact, companydataapi.WhyCannotDerive,
		companydataapi.WhyPeriodMismatch, companydataapi.WhyShareBasisChanged} {
		qs := quarters()
		qs[0].Capex = companydataapi.Cell{Missing: true, Why: why}
		i, _ := in("US:AAPL", rows, qs)
		c := build(t, i)["quarters"].([]any)[0].(map[string]any)["cells"].(map[string]any)["capex"].(map[string]any)
		if text, _ := c["why_text"].(string); !hasHan(text) {
			t.Errorf("reason %v: text %q has no Chinese character", why, text)
		}
	}
	qs := quarters()
	qs[0].Capex = companydataapi.Cell{Missing: true}
	i, _ := in("US:AAPL", rows, qs)
	c := build(t, i)["quarters"].([]any)[0].(map[string]any)["cells"].(map[string]any)["capex"].(map[string]any)
	if text, _ := c["why_text"].(string); !hasHan(text) {
		t.Errorf("generic reason %q has no Chinese character", text)
	}
}
