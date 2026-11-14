package market_state_page

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"
	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/liquidityapi"
	"github.com/andrewrong/optdesk/internal/compute/marketstateapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

const (
	schemaDir  = "../../../contracts/schemas/"
	schemaHost = "https://optdesk.invalid/schemas/"
	testDay    = "2026-11-13"
)

var testZone = time.FixedZone("test", 8*3600)

func sdate(s string) types.SerializableDate {
	t, err := time.Parse(dayLayout, s)
	if err != nil {
		panic(err)
	}
	return types.SerializableDate{Time: t}
}

func rec(day, value, dataDay string) contracts.ReadingRecord {
	r := contracts.ReadingRecord{Day: sdate(day), State: "ok", Value: &value}
	raw := `[{"param":"data_day","value_num":null,"value_tag":"` + dataDay + `"}]`
	if err := json.Unmarshal([]byte(raw), &r.Parameters); err != nil {
		panic(err)
	}
	return r
}

// fakeReadings answers the three readings of the market rows.
func fakeReadings(t *testing.T) compute.ReadingFunc {
	return func(_ context.Context, name, kind, subject string, from, to contracts.Day) ([]contracts.ReadingRecord, error) {
		if kind != "market" {
			t.Errorf("%s: kind %q", name, kind)
		}
		if string(to) != testDay || string(from) != "2026-11-08" {
			t.Errorf("window %s..%s", from, to)
		}
		switch name + "/" + subject {
		case "market.sp500_vs_200d/SP500":
			return []contracts.ReadingRecord{rec(testDay, "0.031200", "2026-11-12")}, nil
		case "market.vix_pctile_3y/VIXCLS":
			return []contracts.ReadingRecord{rec(testDay, "0.383", "2026-11-13")}, nil
		case "market.vix_term_ratio/VIXCLS":
			return []contracts.ReadingRecord{rec(testDay, "0.9215", "2026-11-13")}, nil
		}
		t.Errorf("unexpected reading %s %s", name, subject)
		return nil, nil
	}
}

type fakeSeries struct{ err error }

func (f fakeSeries) SeriesPoints(context.Context, string, contracts.Day, contracts.Day) ([]contracts.SeriesPoint, error) {
	return nil, f.err
}

func line(days ...string) marketstateapi.Line {
	l := marketstateapi.Line{}
	for i, d := range days {
		l.Points = append(l.Points, marketstateapi.Point{Day: contracts.Day(d), Value: fmt.Sprintf("%d.5", 17+i)})
	}
	l.Total = len(days)
	return l
}

func spot(day, x, y string) marketstateapi.Spot {
	return marketstateapi.Spot{Day: contracts.Day(day), X: x, Y: y}
}

// sampleResult is a fake Func result whose funds are not in the order of their gaps.
func sampleResult() marketstateapi.Result {
	pos := func(at int) marketstateapi.Position { return marketstateapi.Position{At: at, Of: 3} }
	num := func(v string) liquidityapi.Num { return liquidityapi.Num{Value: v, Day: "2026-11-13"} }
	return marketstateapi.Result{
		EndDay:   "2026-11-13",
		Drawdown: liquidityapi.Num{Value: "-2.4", Day: "2026-11-13", Aux: "2026-10-29"},
		Funds: []marketstateapi.Fund{
			{Symbol: "XLK", Name: "Technology", Gap21: num("1.5"), Gap63: num("5"), Rank21: pos(2), Rank63: pos(1),
				Rotation: []marketstateapi.Spot{spot("2026-11-06", "4", "1.3"), spot("2026-11-13", "5", "-0.17")}},
			{Symbol: "XLE", Name: "Energy", Gap21: num("-1"), Gap63: num("-3"), Rank21: pos(3), Rank63: pos(3),
				Rotation: []marketstateapi.Spot{spot("2026-11-13", "-3", "0")}},
			{Symbol: "XLF", Name: "Financials", Gap21: num("2"), Gap63: num("1"), Rank21: pos(1), Rank63: pos(2)},
		},
		Industry: []marketstateapi.Fund{
			{Symbol: "SMH", Name: "Semiconductors", Gap21: num("3"), Gap63: num("9"),
				Rotation: []marketstateapi.Spot{spot("2026-11-13", "9", "0")}},
		},
		History: marketstateapi.History{
			Sp500Vs200d: line("2026-11-11", "2026-11-12"), Drawdown: line("2026-11-12", "2026-11-13"),
			VIX: line("2026-11-12", "2026-11-13"), VIXTerm: line("2026-11-12", "2026-11-13"),
			HighYield: line("2026-11-11", "2026-11-12"),
		},
	}
}

func fakeMarket(res marketstateapi.Result) marketstateapi.Func {
	return func(marketstateapi.Input) marketstateapi.Result { return res }
}

func fakeLiquidity(in liquidityapi.Input) liquidityapi.Result {
	return liquidityapi.Result{
		HighYield:     liquidityapi.Num{Value: "3.12", Day: "2026-11-12"},
		HighYieldRank: liquidityapi.Rank{Num: liquidityapi.Num{Value: "41.2", Day: "2026-11-12"}, First: "2023-11-14", Days: 780},
	}
}

var testSymbols = []Symbol{
	{"XLK", "Technology", GroupSector}, {"XLE", "Energy", GroupSector}, {"XLF", "Financials", GroupSector},
	{"SMH", "Semiconductors", GroupIndustry},
}

// fullConfig has every reader; Bars and StockIDs are the fakes of rotation_test.go.
func fullConfig(t *testing.T, res marketstateapi.Result) Config {
	return Config{RunID: "run1", BuildID: "b1", Scope: "s1", Zone: testZone,
		Readings: fakeReadings(t), Bars: &fakeBars{}, StockIDs: fakeIDs{},
		Market: fakeMarket(res), Liquidity: fakeLiquidity, Symbols: testSymbols}
}

func build(t *testing.T, c Config) map[string]any {
	t.Helper()
	b, err := Build(context.Background(), c, fakeSeries{}, testDay, time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return decode(t, b)
}

func decode(t *testing.T, b []byte) map[string]any {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		t.Fatal(err)
	}
	return m
}

func obj(v any) map[string]any          { return v.(map[string]any) }
func list(v any) []any                  { return v.([]any) }
func marketRows(m map[string]any) []any { return list(obj(m["market"])["rows"]) }
func sectorRows(m map[string]any) []any { return list(obj(m["sectors"])["rows"]) }

func isGap(v any) bool {
	m, ok := v.(map[string]any)
	return ok && m["kind"] == "gap"
}

func compileSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	c.UseLoader(fileLoader{})
	s, err := c.Compile(schemaHost + "page.market_state_page.v1.json")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

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

func TestBuildValidatesAgainstSchema(t *testing.T) {
	t.Parallel()
	schema := compileSchema(t)
	noEnd := sampleResult()
	noEnd.EndDay = ""
	cases := map[string]Config{
		"full":      fullConfig(t, sampleResult()),
		"no end":    fullConfig(t, noEnd),
		"empty":     fullConfig(t, marketstateapi.Result{}),
		"no funcs":  {RunID: "run1", BuildID: "b1", Scope: "s1", Zone: testZone, Symbols: testSymbols},
		"no listed": {RunID: "run1", BuildID: "b1", Scope: "s1", Zone: testZone},
	}
	for name, c := range cases {
		b, err := Build(context.Background(), c, fakeSeries{}, testDay, time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if err := schema.Validate(inst); err != nil {
			t.Errorf("%s: %v\n%s", name, err, b)
		}
	}
}

var stateWords = regexp.MustCompile(`(?i)leading|lagging|risk.off|bullish|bearish|contango|backwardation|calm|stress`)

func TestPageHasNoStateWord(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]Config{
		"full":     fullConfig(t, sampleResult()),
		"no funcs": {RunID: "run1", BuildID: "b1", Scope: "s1", Zone: testZone, Symbols: testSymbols},
	} {
		m := build(t, c)
		delete(obj(obj(m["sectors"])["rotation_axes"]), "quadrants")
		b, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		if w := stateWords.Find(b); w != nil {
			t.Errorf("%s: page holds %q", name, w)
		}
	}
}

func TestFreshnessTextNamesEndDay(t *testing.T) {
	t.Parallel()
	text := build(t, fullConfig(t, sampleResult()))["freshness_text"].(string)
	for _, want := range []string{"2026-11-13", "daily closes", "not intraday", "daily FRED series", "its own day"} {
		if !strings.Contains(text, want) {
			t.Errorf("freshness_text lacks %q: %s", want, text)
		}
	}
	noEnd := sampleResult()
	noEnd.EndDay = ""
	if text := build(t, fullConfig(t, noEnd))["freshness_text"].(string); strings.Contains(text, "2026-") {
		t.Errorf("no end day, yet freshness_text names one: %s", text)
	}
}

func TestStaleLabelIsOldestDayAgainstLimitFour(t *testing.T) {
	t.Parallel()
	res := sampleResult()
	m := build(t, fullConfig(t, res))
	st := obj(m["stale"])
	if st["text"] != "Data day 11-12" || st["is_stale"] != false {
		t.Errorf("stale = %v", st)
	}
	res.EndDay = "2026-11-06" // seven days before the page day
	st = obj(build(t, fullConfig(t, res))["stale"])
	if st["text"] != "Data day 11-06" || st["is_stale"] != true {
		t.Errorf("stale = %v", st)
	}
}

func TestNilFuncsGiveNoReader(t *testing.T) {
	t.Parallel()
	base := fullConfig(t, sampleResult())
	for name, edit := range map[string]func(*Config){
		"market":    func(c *Config) { c.Market = nil },
		"liquidity": func(c *Config) { c.Liquidity = nil },
		"readings":  func(c *Config) { c.Readings = nil },
		"bars":      func(c *Config) { c.Bars = nil },
		"stockids":  func(c *Config) { c.StockIDs = nil },
	} {
		c := base
		edit(&c)
		m := build(t, c) // never an error
		rows := marketRows(m)
		if len(rows) != 5 {
			t.Fatalf("%s: %d rows", name, len(rows))
		}
		gapIDs := map[string]bool{}
		for _, r := range rows {
			if f := obj(r)["figure"]; isGap(f) {
				if obj(f)["reason"] != "no_reader" {
					t.Errorf("%s: %v reason %v", name, obj(r)["id"], obj(f)["reason"])
				}
				gapIDs[obj(r)["id"].(string)] = true
			}
		}
		want := map[string][]string{
			"market":    {"sp500_drawdown", "vix"},
			"liquidity": {"hy_spread"},
			"readings":  {"sp500_vs_200d", "vix_term"},
			"bars":      nil, "stockids": nil,
		}[name]
		for _, id := range want {
			if !gapIDs[id] {
				t.Errorf("%s: row %s is not a no_reader gap", name, id)
			}
		}
		if name == "bars" || name == "stockids" || name == "market" {
			for _, r := range sectorRows(m) {
				for _, k := range []string{"gap_21", "gap_63", "rank_21", "rank_63", "rotation"} {
					if g := obj(r)[k]; !isGap(g) || obj(g)["reason"] != "no_reader" {
						t.Errorf("%s: %v %s = %v", name, obj(r)["symbol"], k, g)
					}
				}
			}
			if g := obj(m["sectors"])["end_day"]; !isGap(g) || obj(g)["reason"] != "no_reader" {
				t.Errorf("%s: end_day = %v", name, g)
			}
		}
	}
}

func TestBuildErrors(t *testing.T) {
	t.Parallel()
	c := fullConfig(t, sampleResult())
	at := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	if _, err := Build(context.Background(), c, fakeSeries{err: fmt.Errorf("down")}, testDay, at); err == nil {
		t.Error("series error is swallowed")
	}
	c.Zone = nil
	if _, err := Build(context.Background(), c, fakeSeries{}, testDay, at); err == nil {
		t.Error("nil zone is no error")
	}
	var _ store.SeriesReader = fakeSeries{}
}

// dayAfter is the i-th day after 2024-01-01 (days that are not sessions are fine here).
func dayAfter(i int) contracts.Day {
	return contracts.Day(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, i).Format(dayLayout))
}

func TestNewPageMeta(t *testing.T) {
	t.Parallel()
	p := NewPage(fullConfig(t, sampleResult()))
	if p.Meta.Name != "market_state_page" || p.Meta.Schema != "page.market_state_page.v1" {
		t.Errorf("meta = %v", p.Meta)
	}
	b, err := p.Build(context.Background(), fakeSeries{}, testDay, time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC))
	if err != nil || decode(t, b)["schema"] != "market_state_page" {
		t.Errorf("build: %v", err)
	}
}
