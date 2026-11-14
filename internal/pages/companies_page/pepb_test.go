package companies_page

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/andrewrong/optdesk/internal/compute/pepbapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

var pepbGeneratedAt = time.Date(2026, 11, 10, 14, 0, 0, 0, time.UTC)

func fullReading(kind string) pepbapi.Reading {
	return pepbapi.Reading{
		Kind: kind, State: pepbapi.StateOK, Day: "2026-11-09", Ratio: "18.2",
		BasisEnd: "2026-09-30", BasisFiled: "2026-10-28", Shares: "1520000000", SharesFiled: "2026-10-28",
		Percentile: "34", WindowDays: 1210, WindowFrom: "2021-11-15",
		At: []pepbapi.AtPercentile{
			{Percentile: 20, Ratio: "15.1", Price: "41.20", Text: "PE at the 20th percentile of 1210 days since 2021-11-15: price 41.20 (history, not a buy price)"},
			{Percentile: 50, Ratio: "19.9", Price: "54.00", Text: "PE at the 50th percentile of 1210 days since 2021-11-15: price 54.00 (history, not a buy price)"},
		},
		History: []pepbapi.Point{{Day: "2026-11-06", Text: "18.0", Value: 18}, {Day: "2026-11-09", Text: "18.2", Value: 18.2}},
		Text:    "PE 18.2 (TTM to 2026-09-30, filed 2026-10-28), 34th percentile of 1210 stored days since 2021-11-15",
	}
}

// countingFacts gives as many facts as the last digit of the issuer and counts the calls.
type countingFacts struct {
	mu    sync.Mutex
	calls map[string]int
}

func (f *countingFacts) FactsFiledBy(_ context.Context, source, issuer string, _ contracts.Day) ([]contracts.FundamentalFact, error) {
	f.mu.Lock()
	f.calls[source+"|"+issuer]++
	f.mu.Unlock()
	return make([]contracts.FundamentalFact, int(issuer[len(issuer)-1]-'0')), nil
}

// pepbConfig has a full company (1 fact), a loss company (2 facts) and a Japanese one.
func pepbConfig(facts *countingFacts) Config {
	cfg := testConfig(okBand)
	cfg.Subjects = []Subject{
		{Issuer: "cik:0000000001", Market: "US", Symbol: "FULL"},
		{Issuer: "cik:0000000002", Market: "US", Symbol: "LOSS"},
		{Issuer: "edinet:E00003", Market: "JP", Symbol: "JPN"},
	}
	cfg.Facts = facts
	cfg.Pepb = func(in pepbapi.Input) pepbapi.Result {
		if len(in.Facts) == 2 {
			return pepbapi.Result{
				PE: pepbapi.Reading{Kind: "pe", State: pepbapi.StateLoss, Reason: "loss", Text: "loss, no PE"},
				PB: fullReading("pb"),
			}
		}
		return pepbapi.Result{PE: fullReading("pe"), PB: fullReading("pb")}
	}
	cfg.Bars = func(context.Context, string, contracts.Day, contracts.Day) ([]contracts.PriceBar, error) {
		return []contracts.PriceBar{{Close: "10"}}, nil
	}
	return cfg
}

func buildPepb(t *testing.T, cfg Config) []byte {
	t.Helper()
	b, err := Build(context.Background(), cfg, pepbGeneratedAt)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestPepbNilGivesTheOldPage(t *testing.T) {
	t.Parallel()
	base := buildPepb(t, testConfig(okBand))
	var m map[string]any
	if err := json.Unmarshal(base, &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["pepb"]; ok {
		t.Fatal("pepb key present with both fields nil")
	}
	// Either field alone leaves the page as it was.
	onlyFn := testConfig(okBand)
	onlyFn.Pepb = func(pepbapi.Input) pepbapi.Result { return pepbapi.Result{} }
	onlyBars := testConfig(okBand)
	onlyBars.Bars = func(context.Context, string, contracts.Day, contracts.Day) ([]contracts.PriceBar, error) {
		return nil, nil
	}
	for name, cfg := range map[string]Config{"pepb only": onlyFn, "bars only": onlyBars} {
		if got := buildPepb(t, cfg); !bytes.Equal(got, base) {
			t.Errorf("%s: page differs\n got %s\nwant %s", name, got, base)
		}
	}
}

type pepbPage struct {
	Pepb struct {
		BasisNote string `json:"basis_note"`
		Rows      []struct {
			Symbol  string         `json:"symbol"`
			PE      map[string]any `json:"pe"`
			PB      map[string]any `json:"pb"`
			Forward map[string]any `json:"forward"`
		} `json:"rows"`
	} `json:"pepb"`
}

func TestPepbRowsAndGaps(t *testing.T) {
	t.Parallel()
	facts := &countingFacts{calls: map[string]int{}}
	b := buildPepb(t, pepbConfig(facts))
	// The other blocks read facts too; the pepb block adds exactly one read per cik company.
	before := &countingFacts{calls: map[string]int{}}
	off := pepbConfig(before)
	off.Pepb = nil
	buildPepb(t, off)
	extra := map[string]int{}
	for k, n := range facts.calls {
		extra[k] = n - before.calls[k]
	}
	var page pepbPage
	if err := json.Unmarshal(b, &page); err != nil {
		t.Fatal(err)
	}
	rows := page.Pepb.Rows
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3: %s", len(rows), b)
	}
	if extra["sec|cik:0000000001"] != 1 || extra["sec|cik:0000000002"] != 1 || extra["edinet|edinet:E00003"] != 0 {
		t.Errorf("extra fact reads = %v, want one per cik company and none for edinet", extra)
	}
	full := rows[0].PE
	if full["text"] != fullReading("pe").Text {
		t.Errorf("text = %v", full["text"])
	}
	if r := full["ratio"].(map[string]any); r["text"] != "18.2" || r["value"] != 18.2 || r["unit"] != "ratio" {
		t.Errorf("ratio = %v", r)
	}
	if p := full["percentile"].(map[string]any); p["text"] != "34" || p["value"] != 34.0 {
		t.Errorf("percentile = %v", p)
	}
	at := full["at_percentile"].([]any)
	if len(at) != 2 {
		t.Fatalf("at_percentile = %v", at)
	}
	a0 := at[0].(map[string]any)
	if a0["percentile"] != 20.0 || a0["text"] != fullReading("pe").At[0].Text ||
		a0["ratio"].(map[string]any)["text"] != "15.1" || a0["price"].(map[string]any)["text"] != "41.20" {
		t.Errorf("at_percentile[0] = %v", a0)
	}
	hist := full["history"].([]any)
	if len(hist) != 2 || hist[1].(map[string]any)["day"] != "2026-11-09" || hist[1].(map[string]any)["value"] != 18.2 {
		t.Errorf("history = %v", hist)
	}
	if full["shares"] != "1520000000" || full["shares_filed"] != "2026-10-28" || full["window_days"] != 1210.0 || full["window_from"] != "2021-11-15" || full["basis_end"] != "2026-09-30" {
		t.Errorf("line = %v", full)
	}
	if g := rows[1].PE; g["reason"] != "missing_input" || !bytes.Contains([]byte(g["text"].(string)), []byte("loss, no PE")) {
		t.Errorf("loss pe = %v", g)
	}
	if rows[1].PB["ratio"] == nil {
		t.Errorf("loss company pb = %v, want a line", rows[1].PB)
	}
	for _, g := range []map[string]any{rows[2].PE, rows[2].PB} {
		if g["reason"] != "jp_slice_missing" {
			t.Errorf("edinet gap = %v", g)
		}
	}
	for _, r := range rows {
		if r.Forward["reason"] == nil || r.Forward["text"] == nil || len(r.Forward) != 2 {
			t.Errorf("%s forward = %v, want a Gap", r.Symbol, r.Forward)
		}
	}
}

func TestPepbShortWindowGivesPercentileGap(t *testing.T) {
	t.Parallel()
	cfg := pepbConfig(&countingFacts{calls: map[string]int{}})
	cfg.Pepb = func(pepbapi.Input) pepbapi.Result {
		r := fullReading("pe")
		r.Percentile, r.PercentileReason, r.At, r.History, r.WindowDays = "", "too_few_days", nil, nil, 40
		return pepbapi.Result{PE: r, PB: r}
	}
	var page pepbPage
	if err := json.Unmarshal(buildPepb(t, cfg), &page); err != nil {
		t.Fatal(err)
	}
	g := page.Pepb.Rows[0].PE["percentile"].(map[string]any)
	if g["reason"] != "missing_input" || !bytes.Contains([]byte(g["text"].(string)), []byte("40 days since 2021-11-15")) {
		t.Fatalf("percentile = %v", g)
	}
}

func TestPepbNoBarsGivesNoDataGaps(t *testing.T) {
	t.Parallel()
	cfg := pepbConfig(&countingFacts{calls: map[string]int{}})
	cfg.Bars = func(context.Context, string, contracts.Day, contracts.Day) ([]contracts.PriceBar, error) {
		return nil, nil
	}
	var page pepbPage
	if err := json.Unmarshal(buildPepb(t, cfg), &page); err != nil {
		t.Fatal(err)
	}
	if g := page.Pepb.Rows[0].PE; g["reason"] != "no_data" {
		t.Fatalf("pe = %v", g)
	}
}

func TestPepbNilFactsGivesReaderGap(t *testing.T) {
	t.Parallel()
	cfg := pepbConfig(&countingFacts{calls: map[string]int{}})
	cfg.Facts = nil
	var page struct {
		Pepb map[string]any `json:"pepb"`
	}
	if err := json.Unmarshal(buildPepb(t, cfg), &page); err != nil {
		t.Fatal(err)
	}
	if page.Pepb["reason"] != "no_reader" {
		t.Fatalf("pepb = %v", page.Pepb)
	}
}

func compilePageSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	dir := filepath.Join("..", "..", "..", "contracts", "schemas")
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	for _, f := range []string{"common.v1.json", "page.companies_page.v2.json"} {
		file, err := os.Open(filepath.Join(dir, f))
		if err != nil {
			t.Fatal(err)
		}
		doc, err := jsonschema.UnmarshalJSON(file)
		file.Close()
		if err != nil {
			t.Fatal(err)
		}
		if err := c.AddResource("https://optdesk.invalid/schemas/"+f, doc); err != nil {
			t.Fatal(err)
		}
	}
	s, err := c.Compile("https://optdesk.invalid/schemas/page.companies_page.v2.json")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestPepbValidatesAgainstSchema(t *testing.T) {
	t.Parallel()
	s := compilePageSchema(t)
	cfgs := map[string]Config{"rows": pepbConfig(&countingFacts{calls: map[string]int{}})}
	noFacts := pepbConfig(&countingFacts{calls: map[string]int{}})
	noFacts.Facts = nil
	cfgs["gap"] = noFacts
	for name, cfg := range cfgs {
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(buildPepb(t, cfg)))
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Validate(doc); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}
