package companies_page

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/andrewrong/optdesk/internal/compute/valuationapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

func TestNilReadersGiveGaps(t *testing.T) {
	t.Parallel()
	cfg := testConfig(okBand)
	cfg.Read, cfg.Facts, cfg.Band = nil, nil, nil
	b, err := Build(context.Background(), cfg, time.Date(2026, 11, 10, 14, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	var page struct {
		WatchList struct {
			Rows []struct {
				Cells []testCell `json:"cells"`
			} `json:"rows"`
		} `json:"watch_list"`
		Core      map[string]any `json:"core"`
		Diagnosis map[string]any `json:"diagnosis"`
		Questions map[string]any `json:"questions"`
		Gaps      []struct {
			Reason string `json:"reason"`
		} `json:"gaps"`
		RunStatus string `json:"run_status"`
		DataTimes []struct {
			Source   string `json:"source"`
			DataTime string `json:"data_time"`
		} `json:"data_times"`
	}
	if err := json.Unmarshal(b, &page); err != nil {
		t.Fatal(err)
	}
	if page.RunStatus != "partial" {
		t.Fatalf("run_status = %q, want partial", page.RunStatus)
	}
	if len(page.DataTimes) != 1 || page.DataTimes[0].Source != "companies" || page.DataTimes[0].DataTime != "2026-11-10T00:00:00+08:00" {
		t.Fatalf("data_times = %+v", page.DataTimes)
	}
	if c := page.WatchList.Rows[0].Cells[1]; c.PriceState != "" || c.Text == "" {
		t.Fatalf("cell = %+v", c)
	}
	if len(page.Gaps) != 1 || page.Gaps[0].Reason != "no_reader" {
		t.Fatalf("gaps = %+v", page.Gaps)
	}
	for name, blk := range map[string]map[string]any{"core": page.Core, "diagnosis": page.Diagnosis, "questions": page.Questions} {
		if blk["reason"] != "not_built" {
			t.Fatalf("%s = %v, want a not_built gap", name, blk)
		}
	}
}

func TestHeldRowsCarryTheMoatText(t *testing.T) {
	t.Parallel()
	cfg := testConfig(okBand)
	cfg.Held = func(context.Context) (map[string]bool, error) { return map[string]bool{"cik:0000000001": true}, nil }
	cfg.Text = fakeText{"SYN1": "brand and scale"}
	b, err := Build(context.Background(), cfg, time.Date(2026, 11, 10, 14, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	var page struct {
		Holdings struct {
			Rows []struct {
				Moat []string `json:"moat"`
			} `json:"rows"`
		} `json:"holdings"`
		WatchList struct {
			Rows []any `json:"rows"`
		} `json:"watch_list"`
	}
	if err := json.Unmarshal(b, &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Holdings.Rows) != 1 || page.Holdings.Rows[0].Moat[0] != "brand and scale" || len(page.WatchList.Rows) != 0 {
		t.Fatalf("page = %s", b)
	}
}

func TestUnvaluedLineStatesEachReasonOnce(t *testing.T) {
	t.Parallel()
	cfg := testConfig(okBand)
	cfg.Subjects = []Subject{
		{Issuer: "cik:0000000001", Market: "US", Symbol: "S1"},
		{Issuer: "cik:0000000002", Market: "US", Symbol: "S2"},
		{Issuer: "cik:0000000003", Market: "US", Symbol: "S3"},
	}
	cfg.Facts = fakeFactsByIssuer{}
	cfg.BuildFilings = func(f []contracts.FundamentalFact, _ contracts.Day, _ valuationapi.Standard) valuationapi.FilingsResult {
		reason := "fewer_than_5_years"
		if len(f) == 3 {
			reason = "no_standard"
		}
		return valuationapi.FilingsResult{State: valuationapi.StateUnvalued, Reasons: []string{reason}}
	}
	b, err := Build(context.Background(), cfg, time.Date(2026, 11, 10, 14, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	var page struct {
		Unvalued  *string `json:"unvalued"`
		WatchList struct {
			Rows []struct {
				Cells []testCell `json:"cells"`
			} `json:"rows"`
		} `json:"watch_list"`
	}
	if err := json.Unmarshal(b, &page); err != nil {
		t.Fatal(err)
	}
	if page.Unvalued == nil || *page.Unvalued != "fewer than 5 years: S1, S2; no standard: S3" {
		t.Fatalf("unvalued = %v", page.Unvalued)
	}
	for _, r := range page.WatchList.Rows {
		if r.Cells[1].Text != "unvalued" {
			t.Errorf("price cell = %q", r.Cells[1].Text)
		}
	}
}

// fakeFactsByIssuer gives as many facts as the issuer's last digit, so a test can tell issuers apart.
type fakeFactsByIssuer struct{}

func (fakeFactsByIssuer) FactsFiledBy(_ context.Context, _, issuer string, _ contracts.Day) ([]contracts.FundamentalFact, error) {
	return make([]contracts.FundamentalFact, int(issuer[len(issuer)-1]-'0')), nil
}

func TestFullyBuiltPageIsOk(t *testing.T) {
	t.Parallel()
	cfg := fixtureConfig(loadInput(t))
	var subjects []Subject
	for _, s := range cfg.Subjects {
		if s.Symbol != "SYN2" { // the fixture's cyclical company is listed as unvalued
			subjects = append(subjects, s)
		}
	}
	cfg.Subjects = subjects
	b, err := Build(context.Background(), cfg, time.Date(2026, 11, 10, 14, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	var page struct {
		RunStatus string  `json:"run_status"`
		Unvalued  *string `json:"unvalued"`
		Gaps      []any   `json:"gaps"`
		DataTimes []struct {
			Source string `json:"source"`
		} `json:"data_times"`
	}
	if err := json.Unmarshal(b, &page); err != nil {
		t.Fatal(err)
	}
	if page.Unvalued != nil || len(page.Gaps) != 0 {
		t.Fatalf("fixture is not fully built: unvalued=%q gaps=%v", *page.Unvalued, page.Gaps)
	}
	if page.RunStatus != "ok" || len(page.DataTimes) != 1 || page.DataTimes[0].Source != "companies" {
		t.Fatalf("run_status = %q, data_times = %+v", page.RunStatus, page.DataTimes)
	}
}

// spotPage builds the page of cfg and returns the Last text of each watch and holdings row, the
// header line, the stale text and the notes.
type spotPage struct {
	WatchList struct {
		Rows []struct {
			Cells []testCell `json:"cells"`
		} `json:"rows"`
	} `json:"watch_list"`
	Holdings struct {
		Rows []struct {
			Cells []testCell `json:"cells"`
		} `json:"rows"`
	} `json:"holdings"`
	Header struct {
		Line string `json:"line"`
	} `json:"header"`
	Stale struct {
		Text string `json:"text"`
	} `json:"stale"`
	Notes []string `json:"notes"`
}

func buildSpotPage(t *testing.T, cfg Config) spotPage {
	t.Helper()
	b, err := Build(context.Background(), cfg, time.Date(2026, 11, 10, 14, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	var p spotPage
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func twoSubjectConfig(price string, day contracts.Day) Config {
	cfg := testConfig(okBand)
	cfg.Subjects = []Subject{
		{Issuer: "cik:0000000001", Market: "US", Symbol: "SYN1"},
		{Issuer: "cik:0000000002", Market: "US", Symbol: "SYN2"},
	}
	cfg.Facts = fakeFactsByIssuer{}
	cfg.Held = func(context.Context) (map[string]bool, error) { return map[string]bool{"cik:0000000002": true}, nil }
	cfg.Spot = func(context.Context, string, contracts.Day) (Spot, bool, error) {
		return Spot{Price: price, Day: day}, true, nil
	}
	return cfg
}

func TestBuildLastCellCarriesDay(t *testing.T) {
	t.Parallel()
	p := buildSpotPage(t, twoSubjectConfig("128.40", "2026-11-04"))
	want := "$128.40 · close 2026-11-04"
	if got := p.WatchList.Rows[0].Cells[2].Text; got != want {
		t.Errorf("watch row Last = %q, want %q", got, want)
	}
	if got := p.Holdings.Rows[0].Cells[2].Text; got != want {
		t.Errorf("holdings row Last = %q, want %q", got, want)
	}
}

func TestBuildLastCellJapaneseYen(t *testing.T) {
	t.Parallel()
	cfg := twoSubjectConfig("5574", "2026-11-17")
	cfg.Subjects[0].Market = "JP"
	p := buildSpotPage(t, cfg)
	if got, want := p.WatchList.Rows[0].Cells[2].Text, "¥5,574.00 · close 2026-11-17"; got != want {
		t.Errorf("Last = %q, want %q", got, want)
	}
}

func TestBuildLastCellPriceNotDecimal(t *testing.T) {
	t.Parallel()
	p := buildSpotPage(t, twoSubjectConfig("n/a", "2026-11-04"))
	if got, want := p.WatchList.Rows[0].Cells[2].Text, "n/a · close 2026-11-04"; got != want {
		t.Errorf("Last = %q, want %q", got, want)
	}
}

func TestBuildNoQuoteWithoutDay(t *testing.T) {
	t.Parallel()
	p := buildSpotPage(t, twoSubjectConfig("128.40", ""))
	if got := p.WatchList.Rows[0].Cells[2].Text; got != "no quote" {
		t.Errorf("Last = %q, want no quote", got)
	}
}

func TestBuildDataOfFormat(t *testing.T) {
	t.Parallel()
	p := buildSpotPage(t, twoSubjectConfig("128.40", "2026-11-04"))
	if p.Header.Line != "Watching 2 companies" {
		t.Errorf("header.line = %q", p.Header.Line)
	}
	if p.Stale.Text != "Data of 2026-11-10" {
		t.Errorf("stale.text = %q", p.Stale.Text)
	}
}

func TestBuildNotesHasLastGloss(t *testing.T) {
	t.Parallel()
	p := buildSpotPage(t, twoSubjectConfig("128.40", "2026-11-04"))
	want := "Last = daily close of the day shown; /positions shows the broker mark"
	if len(p.Notes) != 1 || p.Notes[0] != want {
		t.Errorf("notes = %q, want [%q]", p.Notes, want)
	}
}

// farConfig is a page of subjects whose band top is 100 and whose last prices are given by
// symbol; the first subject is held.
func farConfig(prices map[string]string, symbols ...string) Config {
	cfg := testConfig(valuationapi.BandFigure{ID: "valuation.buy_band", State: valuationapi.StateOK, Value: "100", Low: "80", Currency: "USD"})
	cfg.Subjects = nil
	for i, s := range symbols {
		cfg.Subjects = append(cfg.Subjects, Subject{Issuer: "cik:000000000" + string(rune('1'+i%9)), Market: "US", Symbol: s})
	}
	cfg.Facts = fakeFactsByIssuer{}
	cfg.Held = func(context.Context) (map[string]bool, error) {
		return map[string]bool{cfg.Subjects[0].Issuer: true}, nil
	}
	cfg.Spot = func(_ context.Context, symbol string, _ contracts.Day) (Spot, bool, error) {
		p, ok := prices[symbol]
		return Spot{Price: p, Day: "2026-11-09"}, ok, nil
	}
	return cfg
}

func TestFarAboveBandNoteNamesThem(t *testing.T) {
	t.Parallel()
	p := buildSpotPage(t, farConfig(map[string]string{"AAA": "500", "BBB": "200", "CCC": "400"}, "AAA", "BBB", "CCC"))
	want := "Last is over 3 times the top of the price line for: AAA, CCC. " +
		"The line is the price at an 8% free-cash-flow yield on the 5-year median (after stock pay), a no-growth value."
	if len(p.Notes) != 2 || p.Notes[0] != lastGloss || p.Notes[1] != want {
		t.Errorf("notes = %q", p.Notes)
	}
	if got := p.Holdings.Rows[0].Cells[1]; got.Text != "$80.00 to $100.00" || got.PriceState != "awaiting approval" {
		t.Errorf("price cell changed: %+v", got)
	}
}

func TestNoFarAboveBandNoNote(t *testing.T) {
	t.Parallel()
	// exactly 3 times is not over 3 times; an override, no band and no quote are not named.
	cfg := farConfig(map[string]string{"AAA": "300", "BBB": "900", "CCC": "900"}, "AAA", "BBB", "CCC", "DDD")
	cfg.Override = fakeOverride{"BBB": "120"}
	cfg.BuildFilings = func(f []contracts.FundamentalFact, _ contracts.Day, _ valuationapi.Standard) valuationapi.FilingsResult {
		if len(f) == 3 { // CCC
			return valuationapi.FilingsResult{State: valuationapi.StateUnvalued, Reasons: []string{"financial_sic"}}
		}
		return valuationapi.FilingsResult{State: valuationapi.StateOK}
	}
	p := buildSpotPage(t, cfg)
	if len(p.Notes) != 1 || p.Notes[0] != lastGloss {
		t.Errorf("notes = %q", p.Notes)
	}
}

func TestFarAboveBandNoteFitsTheSchema(t *testing.T) {
	t.Parallel()
	prices := map[string]string{}
	var symbols []string
	for _, s := range []string{"AA", "BB", "CC", "DD", "EE", "FF", "GG", "HH", "II", "JJ", "KK", "LL"} {
		symbols = append(symbols, s)
		prices[s] = "1000"
	}
	cfg := farConfig(prices, symbols...)
	b, err := Build(context.Background(), cfg, time.Date(2026, 11, 10, 14, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	var p spotPage
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	if len(p.Notes) != 2 {
		t.Fatalf("notes = %q", p.Notes)
	}
	note := p.Notes[1]
	if n := utf8.RuneCountInString(note); n > 200 {
		t.Errorf("note has %d characters", n)
	}
	if !strings.Contains(note, "AA, BB, CC, DD, EE, FF and 6 more.") || strings.Contains(note, "GG") {
		t.Errorf("note = %q", note)
	}
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	c.UseLoader(schemaLoader{})
	s, err := c.Compile(schemaHost + "page.companies_page.v2.json")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Validate(doc); err != nil {
		t.Errorf("page does not validate: %v", err)
	}
}

const schemaHost = "https://optdesk.invalid/schemas/"

// schemaLoader serves https://optdesk.invalid/schemas/<file> from contracts/schemas.
type schemaLoader struct{}

func (schemaLoader) Load(url string) (any, error) {
	file, ok := strings.CutPrefix(url, schemaHost)
	if !ok || strings.ContainsAny(file, "/\\") {
		return nil, fmt.Errorf("unexpected schema url %q", url)
	}
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "schemas", file))
	if err != nil {
		return nil, err
	}
	return jsonschema.UnmarshalJSON(bytes.NewReader(b))
}
