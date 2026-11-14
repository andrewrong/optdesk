package home_options_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/pages/home_options"
	"github.com/andrewrong/optdesk/internal/store"
)

const day = "2026-11-05"

var zone = time.FixedZone("CST", 8*3600)
var generated = time.Date(2026, 11, 6, 9, 30, 0, 0, zone)

// ledger is a fake reader: one alias per key, a row per transaction (the attributes are not read by the fake replay).
type ledger struct {
	aliases map[string]int
	err     error
}

func (l ledger) FlexTradeRows(_ context.Context, alias string) ([]store.FlexTradeRow, error) {
	if l.err != nil {
		return nil, l.err
	}
	rows := []store.FlexTradeRow{}
	for i := 0; i < l.aliases[alias]; i++ {
		rows = append(rows, store.FlexTradeRow{Alias: alias, TransactionID: string(rune('a' + i)), Revision: 1, Attributes: "{}"})
	}
	return rows, nil
}

func (ledger) LatestOkFetch(context.Context, string, string) (store.FlexFetchRecord, bool, error) {
	return store.FlexFetchRecord{}, false, nil
}

func (l ledger) StoredAliases(context.Context) ([]string, error) {
	if l.err != nil {
		return nil, l.err
	}
	out := []string{}
	for a := range l.aliases {
		out = append(out, a)
	}
	return out, nil
}

// replayer returns fixed positions whatever the trades are.
type replayer struct{ positions []compute.LedgerPosition }

func (r replayer) Replay(compute.LedgerInput, compute.LedgerParams) compute.LedgerResult {
	return compute.LedgerResult{RuleVersion: 1, Positions: r.positions}
}

func pos() compute.LedgerPosition {
	return compute.LedgerPosition{Currency: "USD", Status: "closed", OpenedOn: "2026-10-20", ClosedOn: "2026-11-04"}
}

// results is a fake OptionResults: it returns sections whatever the input is and records the input.
type results struct {
	sections []compute.ResultSection
	seen     *compute.ResultInput
}

func (r results) Summarise(in compute.ResultInput) compute.ResultSummary {
	if r.seen != nil {
		*r.seen = in
	}
	return compute.ResultSummary{Sections: r.sections}
}

func sold(premium, realized, kept string, months ...compute.ResultMonth) compute.ResultSection {
	return compute.ResultSection{Side: "sold", Currency: "USD", Months: months, Premium: premium, Realized: realized, KeptShare: kept}
}

func cfg(l store.OptionLedgerReader, r compute.OptionResults, ps ...compute.LedgerPosition) home_options.Config {
	return home_options.Config{
		RunID: "run-test", BuildID: "build-test", Scope: "scope-test", Zone: zone, Ledger: l,
		Replay:  func() compute.OptionLedger { return replayer{ps} },
		Results: func() compute.OptionResults { return r },
	}
}

func okCfg(secs ...compute.ResultSection) home_options.Config {
	return cfg(ledger{aliases: map[string]int{"main": 2}}, results{sections: secs}, pos())
}

type fileLoader struct{}

func (fileLoader) Load(url string) (any, error) {
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "schemas", filepath.Base(url)))
	if err != nil {
		return nil, err
	}
	return jsonschema.UnmarshalJSON(bytes.NewReader(b))
}

func build(t *testing.T, c home_options.Config) []byte {
	t.Helper()
	b, err := home_options.Build(context.Background(), c, day, generated)
	if err != nil {
		t.Fatal(err)
	}
	sc := jsonschema.NewCompiler()
	sc.AssertFormat()
	sc.UseLoader(fileLoader{})
	s, err := sc.Compile("https://optdesk.invalid/schemas/page.options_summary.v4.json")
	if err != nil {
		t.Fatal(err)
	}
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Validate(v); err != nil {
		t.Fatalf("output is invalid: %v\n%s", err, b)
	}
	return b
}

func card(t *testing.T, c home_options.Config) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(build(t, c), &m); err != nil {
		t.Fatal(err)
	}
	return m["card"].(map[string]any)
}

func tileFigure(c map[string]any, k string) map[string]any {
	return c["tiles"].(map[string]any)[k].(map[string]any)["figure"].(map[string]any)
}

func TestGoldenOK(t *testing.T) {
	t.Parallel()
	got := build(t, okCfg(sold("1234.5000", "-15.2500", "-0.012355",
		compute.ResultMonth{Month: "2026-09", Premium: "200.0000"},
		compute.ResultMonth{Month: "2026-11", Premium: "300.0000"})))
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, got, "", "  "); err != nil {
		t.Fatal(err)
	}
	pretty.WriteByte('\n')
	want, err := os.ReadFile(filepath.Join("testdata", "ok.golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(pretty.Bytes(), want) {
		t.Fatalf("golden differs:\n%s", pretty.Bytes())
	}
}

// The premium tile is the Premium of the sold USD section, copied; the caption carries Realized
// and KeptShare.
func TestTilesEqualTheSectionFields(t *testing.T) {
	t.Parallel()
	c := card(t, okCfg(sold("1234.5000", "-15.2500", "-0.012355")))
	f := tileFigure(c, "premium_month_usd")
	if f["text"] != "+$1,234.50" || f["value"].(float64) != 1234.5 || f["tone"] != "up" || f["unit"] != "usd_signed" {
		t.Fatalf("figure = %v", f)
	}
	cap := c["tiles"].(map[string]any)["premium_month_usd"].(map[string]any)["caption"].(string)
	if !strings.Contains(cap, "−$15.25") || !strings.Contains(cap, "-0.012355") {
		t.Fatalf("caption = %q", cap)
	}
	if !strings.Contains(c["say"].(string), "+$1,234.50") {
		t.Fatalf("say = %q", c["say"])
	}
}

func TestNegativePremiumKeepsItsSign(t *testing.T) {
	t.Parallel()
	f := tileFigure(card(t, okCfg(sold("-80.0000", "", ""))), "premium_month_usd")
	if f["text"] != "−$80.00" || f["tone"] != "down" || f["value"].(float64) != -80 {
		t.Fatalf("figure = %v", f)
	}
}

// The premium of a point is the Premium of the month of the sold section with that Month; a month
// the section does not list is a gap, and the closed series stays a gap.
func TestSeriesPremiumByCloseMonth(t *testing.T) {
	t.Parallel()
	c := card(t, okCfg(sold("500.0000", "-15.2500", "",
		compute.ResultMonth{Month: "2026-09", Premium: "200.0000"},
		compute.ResultMonth{Month: "2026-11", Premium: "300.0000"})))
	pts := c["premium_series_6m"].([]any)
	want := []string{"2026-06", "2026-07", "2026-08", "2026-09", "2026-10", "2026-11"}
	text := map[int]string{3: "+$200.00", 5: "+$300.00"}
	for i, p := range pts {
		m := p.(map[string]any)
		if m["month"] != want[i] {
			t.Fatalf("month %d = %v", i, m["month"])
		}
		pr := m["premium"].(map[string]any)
		if tx, ok := text[i]; ok {
			if pr["kind"] != "value" || pr["text"] != tx {
				t.Fatalf("premium of %v = %v", m["month"], pr)
			}
		} else if pr["kind"] != "gap" || pr["reason"] != "no_data" || pr["text"] != "No sold records closed this month" {
			t.Fatalf("premium of %v = %v", m["month"], pr)
		}
		if cl := m["closed"].(map[string]any); cl["kind"] != "gap" || cl["reason"] != "no_data" {
			t.Fatalf("closed of %v = %v", m["month"], cl)
		}
	}
	if strings.Contains(string(mustJSON(t, c["gaps"])), "Settled premium by closing month: no data yet") {
		t.Fatalf("gaps = %v", c["gaps"])
	}
}

// With no month figure every point is a gap and the series gap stays in the card gaps.
func TestSeriesWithoutFigureKeepsGap(t *testing.T) {
	t.Parallel()
	c := card(t, okCfg(sold("1234.5000", "-15.2500", "")))
	for _, p := range c["premium_series_6m"].([]any) {
		if g := p.(map[string]any)["premium"].(map[string]any); g["kind"] != "gap" {
			t.Fatalf("premium = %v", g)
		}
	}
	if !strings.Contains(string(mustJSON(t, c["gaps"])), "Settled premium by closing month: no data yet") {
		t.Fatalf("gaps = %v", c["gaps"])
	}
}

// The caption states the counting rule, the notes the closed P/L explanation; the series names stay short.
func TestSeriesCardTextStatesRule(t *testing.T) {
	t.Parallel()
	c := card(t, okCfg(sold("1.0000", "", "")))
	if c["premium_series_text"] != "Settled premium" || c["closed_series_text"] != "Closed P/L" {
		t.Fatalf("names = %v, %v", c["premium_series_text"], c["closed_series_text"])
	}
	s := c["premium_series_caption"].(string)
	for _, w := range []string{
		"by closing month", "expiry, exercise, assignment and buy-back", "fees are not deducted",
		"year and month of the closing day", "no time zone conversion", "by account alias, never merged",
	} {
		if !strings.Contains(s, w) {
			t.Fatalf("caption lacks %q: %s", w, s)
		}
	}
	if !strings.Contains(string(mustJSON(t, c["notes"])), "monthly closed P/L is not given on this page") {
		t.Fatalf("notes = %v", c["notes"])
	}
}

func TestResultsGetThePositionsOfEveryAliasAndTheDay(t *testing.T) {
	t.Parallel()
	var in compute.ResultInput
	c := cfg(ledger{aliases: map[string]int{"a": 1, "b": 1}}, results{sections: []compute.ResultSection{sold("1.0000", "", "")}, seen: &in}, pos())
	build(t, c)
	if in.AsOf != day || len(in.Positions) != 2 {
		t.Fatalf("input = %+v", in)
	}
}

func TestOtherCurrencyIsNamed(t *testing.T) {
	t.Parallel()
	hkd := sold("5.0000", "", "")
	hkd.Currency = "HKD"
	c := card(t, okCfg(sold("1.0000", "", ""), hkd))
	if !strings.Contains(string(mustJSON(t, c["gaps"])), "unsupported_currency") {
		t.Fatalf("gaps = %v", c["gaps"])
	}
}

func TestPremiumWithSubCentPlacesIsGapNotRounded(t *testing.T) {
	t.Parallel()
	g := tileFigure(card(t, okCfg(sold("1.2345", "", ""))), "premium_month_usd")
	if g["kind"] != "gap" || g["reason"] != "no_data" {
		t.Fatalf("figure = %v", g)
	}
}

func TestNoSoldSectionIsNoDataGap(t *testing.T) {
	t.Parallel()
	g := tileFigure(card(t, okCfg()), "premium_month_usd")
	if g["kind"] != "gap" || g["reason"] != "no_data" {
		t.Fatalf("figure = %v", g)
	}
}

func TestNotBuiltTilesAreGaps(t *testing.T) {
	t.Parallel()
	c := card(t, okCfg(sold("1.0000", "", "")))
	for _, k := range []string{"sold_count_month", "open_premium_usd", "max_loss_usd"} {
		if g := tileFigure(c, k); g["kind"] != "gap" || g["reason"] != "no_data" {
			t.Fatalf("%s = %v", k, g)
		}
	}
}

func allGaps(t *testing.T, c map[string]any, text string) {
	t.Helper()
	for _, k := range []string{"sold_count_month", "premium_month_usd", "open_premium_usd", "max_loss_usd"} {
		if g := tileFigure(c, k); g["kind"] != "gap" || g["reason"] != "no_data" {
			t.Fatalf("%s = %v", k, g)
		}
	}
	if !strings.Contains(string(mustJSON(t, c["gaps"])), text) {
		t.Fatalf("gaps = %v", c["gaps"])
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestNilReaderIsNoDataEverywhere(t *testing.T) {
	t.Parallel()
	c := cfg(nil, results{})
	c.Ledger = nil
	allGaps(t, card(t, c), "Option ledger: not connected yet")
}

func TestEmptyLedgerIsNoDataEverywhere(t *testing.T) {
	t.Parallel()
	c := card(t, cfg(ledger{aliases: map[string]int{}}, results{}))
	allGaps(t, c, "Ledger: no records yet")
	if c["say"] != "Option ledger: no records yet." {
		t.Fatalf("say = %v", c["say"])
	}
}

func TestLedgerErrorIsError(t *testing.T) {
	t.Parallel()
	c := cfg(ledger{err: errors.New("down")}, results{})
	if _, err := home_options.Build(context.Background(), c, day, generated); err == nil {
		t.Fatal("want an error")
	}
}

func TestBadConfigIsError(t *testing.T) {
	t.Parallel()
	good := okCfg(sold("1.0000", "", ""))
	noZone, noResults, noReplay, badID := good, good, good, good
	noZone.Zone = nil
	noResults.Results = nil
	noReplay.Replay = nil
	badID.RunID = "bad id"
	for name, c := range map[string]home_options.Config{"zone": noZone, "results": noResults, "replay": noReplay, "id": badID} {
		if _, err := home_options.Build(context.Background(), c, day, generated); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
	if _, err := home_options.Build(context.Background(), good, "2026-13-45", generated); err == nil {
		t.Error("bad day: want an error")
	}
}

func TestSeriesNamesAreShortAndCaptionHoldsExplanation(t *testing.T) {
	t.Parallel()
	cards := map[string]map[string]any{
		"ok":      card(t, okCfg(sold("1.0000", "", ""))),
		"no sold": card(t, okCfg()),
	}
	for name, c := range cards {
		if c["premium_series_text"] != "Settled premium" || c["closed_series_text"] != "Closed P/L" {
			t.Errorf("%s: names = %v, %v", name, c["premium_series_text"], c["closed_series_text"])
		}
		for _, k := range []string{"premium_series_text", "closed_series_text"} {
			if strings.ContainsAny(c[k].(string), ";:") {
				t.Errorf("%s: %s holds a sentence: %s", name, k, c[k])
			}
		}
		caption := c["premium_series_caption"].(string)
		for _, w := range []string{
			"by closing month", "expiry, exercise, assignment and buy-back", "fees are not deducted",
			"year and month of the closing day", "no time zone conversion", "by account alias, never merged",
		} {
			if !strings.Contains(caption, w) {
				t.Errorf("%s: caption lacks %q: %s", name, w, caption)
			}
		}
		notes := c["notes"].([]any)
		if len(notes) != 3 || notes[2] != "Closed P/L has no data yet: monthly closed P/L is not given on this page." {
			t.Errorf("%s: notes = %v", name, notes)
		}
	}
}
