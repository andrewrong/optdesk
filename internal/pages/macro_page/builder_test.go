package macro_page

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"
	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/andrewrong/optdesk/internal/contracts"
)

var (
	zone    = time.FixedZone("test", 8*3600)
	fixedAt = time.Date(2026, 11, 5, 16, 0, 0, 0, time.UTC)
)

type fakeReader struct {
	points map[string][]contracts.SeriesPoint
}

func (f fakeReader) SeriesPoints(_ context.Context, id string, from, to contracts.Day) ([]contracts.SeriesPoint, error) {
	var out []contracts.SeriesPoint
	for _, p := range f.points[id] {
		d := contracts.Day(p.ObservationDate.Format(dayLayout))
		if d >= from && d <= to {
			out = append(out, p)
		}
	}
	return out, nil
}

func date(t *testing.T, s string) types.SerializableDate {
	t.Helper()
	d, err := time.Parse(dayLayout, s)
	if err != nil {
		t.Fatal(err)
	}
	return types.SerializableDate{Time: d}
}

func pt(t *testing.T, obs, rt, unit string, v *float64) contracts.SeriesPoint {
	t.Helper()
	return contracts.SeriesPoint{
		ObservationDate: date(t, obs),
		RealtimeStart:   date(t, rt),
		Unit:            contracts.SeriesUnit(unit),
		Value:           v,
	}
}

func f64(v float64) *float64 { return &v }

func cfg(ids ...string) Config {
	c := Config{RunID: "run-1", BuildID: "build-1", Scope: "macro", Zone: zone}
	for _, id := range ids {
		c.Indicators = append(c.Indicators, Indicator{SeriesID: id, Name: "Name " + id, SourceText: "FRED"})
	}
	return c
}

func compileSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	f, err := os.Open(filepath.Join("..", "..", "..", "contracts", "schemas", "page.macro_page.v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	doc, err := jsonschema.UnmarshalJSON(f)
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	if err := c.AddResource("page.macro_page.v1.json", doc); err != nil {
		t.Fatal(err)
	}
	s, err := c.Compile("page.macro_page.v1.json")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func validateBytes(t *testing.T, b []byte) error {
	t.Helper()
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	return compileSchema(t).Validate(v)
}

func build(t *testing.T, r fakeReader, c Config, day string) map[string]any {
	t.Helper()
	b, err := Build(t.Context(), r, c, contracts.Day(day), fixedAt)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateBytes(t, b); err != nil {
		t.Fatalf("schema: %v\n%s", err, b)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func indicator(m map[string]any, i int) map[string]any {
	return m["indicators"].([]any)[i].(map[string]any)
}

const golden = `{"schema":"macro_page","schema_version":1,"run_id":"run-1","build_id":"build-1",` +
	`"scope":"macro","generated_at":"2026-11-06T00:00:00+08:00",` +
	`"data_times":[{"source":"DGS10","data_time":"2026-11-04T00:00:00+08:00"}],` +
	`"run_status":"partial","title":"Macro","sentence":"Macro indicators come from stored series; ease and events: no data yet.",` +
	`"ease_title":"Ease","events_title":"This week and next","indicators_title":"Indicators","notes_title":"Notes",` +
	`"ease":{"reason":"no_data","text":"Ease: no data yet"},"events":[],"indicators":[{"name":"Name DGS10",` +
	`"latest":{"value":4.3,"text":"4.3%","unit":"percent"},"status":"good","label":"On time",` +
	`"source_text":"FRED","detail_href":"/indicator/DGS10"},{"name":"Name DGS2",` +
	`"latest":{"reason":"no_data","text":"no data yet"},"status":"missing","label":"n/a",` +
	`"source_text":"FRED","detail_href":null}],"notes":["Dates come from the FRED release calendar and central bank schedules; dates only, no release times.",` +
	`"Each indicator is the latest stored period of its series; a series with none stored shows n/a."],"gaps":[{"reason":"no_data",` +
	`"text":"Ease: no data yet"},{"reason":"no_data","text":"Event calendar: no data yet"}],` +
	`"stale":{"data_time":"2026-11-04T00:00:00+08:00",` +
	`"stale_after":"2026-11-08T09:30:00+08:00","text":"Data day 11-04"}}`

func TestBuildGolden(t *testing.T) {
	t.Parallel()
	r := fakeReader{points: map[string][]contracts.SeriesPoint{
		"DGS10": {pt(t, "2026-11-03", "2026-11-04", "percent", f64(4.25)), pt(t, "2026-11-04", "2026-11-05", "percent", f64(4.3))},
		"DGS2":  {pt(t, "2026-11-04", "2026-11-05", "percent", nil)},
	}}
	c := cfg("DGS10", "DGS2")
	c.Indicators[0].DetailHref = "/indicator/DGS10"
	b, err := Build(t.Context(), r, c, "2026-11-05", fixedAt)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateBytes(t, b); err != nil {
		t.Fatal(err)
	}
	if string(b) != golden {
		t.Fatalf("got\n%s\nwant\n%s", b, golden)
	}
}

func TestIndicatorStatus(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		pts          []contracts.SeriesPoint
		status, text string
		latestKey    string
	}{
		{"good", []contracts.SeriesPoint{pt(t, "2026-11-04", "2026-11-04", "percent", f64(4.5))}, "good", "On time", "unit"},
		{"good at limit", []contracts.SeriesPoint{pt(t, "2026-11-01", "2026-11-01", "percent", f64(4))}, "good", "On time", "unit"},
		{"late", []contracts.SeriesPoint{pt(t, "2026-10-31", "2026-10-31", "percent", f64(4))}, "late", "Late", "unit"},
		{"missing", nil, "missing", "n/a", "reason"},
		{"nil value only", []contracts.SeriesPoint{pt(t, "2026-11-04", "2026-11-04", "percent", nil)}, "missing", "n/a", "reason"},
		{"other unit", []contracts.SeriesPoint{pt(t, "2026-11-04", "2026-11-04", "index", f64(1))}, "good", "On time", "reason"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := build(t, fakeReader{points: map[string][]contracts.SeriesPoint{"S": tc.pts}}, cfg("S"), "2026-11-05")
			ind := indicator(m, 0)
			if ind["status"] != tc.status || ind["label"] != tc.text {
				t.Fatalf("status/label = %v/%v", ind["status"], ind["label"])
			}
			if _, ok := ind["latest"].(map[string]any)[tc.latestKey]; !ok {
				t.Fatalf("latest = %v, want key %s", ind["latest"], tc.latestKey)
			}
		})
	}
	t.Run("text is shortest decimal", func(t *testing.T) {
		t.Parallel()
		m := build(t, fakeReader{points: map[string][]contracts.SeriesPoint{"S": {pt(t, "2026-11-04", "2026-11-04", "percent", f64(4.30))}}}, cfg("S"), "2026-11-05")
		if got := indicator(m, 0)["latest"].(map[string]any)["text"]; got != "4.3%" {
			t.Fatalf("text = %v", got)
		}
	})
}

func TestVintageBound(t *testing.T) {
	t.Parallel()
	r := fakeReader{points: map[string][]contracts.SeriesPoint{"S": {
		pt(t, "2026-11-04", "2026-11-04", "percent", f64(4.1)),
		pt(t, "2026-11-04", "2026-11-06", "percent", f64(9.9)),
	}}}
	m := build(t, r, cfg("S"), "2026-11-05")
	if got := indicator(m, 0)["latest"].(map[string]any)["value"]; got != 4.1 {
		t.Fatalf("value = %v, want the vintage not after day", got)
	}
	// among vintages on or before day the greatest RealtimeStart wins
	r.points["S"] = append(r.points["S"], pt(t, "2026-11-04", "2026-11-05", "percent", f64(4.2)))
	m = build(t, r, cfg("S"), "2026-11-05")
	if got := indicator(m, 0)["latest"].(map[string]any)["value"]; got != 4.2 {
		t.Fatalf("value = %v, want 4.2", got)
	}
}

func TestStaleLabel(t *testing.T) {
	t.Parallel()
	one := fakeReader{points: map[string][]contracts.SeriesPoint{"A": {pt(t, "2026-11-04", "2026-11-04", "percent", f64(1))}}}
	two := fakeReader{points: map[string][]contracts.SeriesPoint{
		"A": {pt(t, "2026-11-04", "2026-11-04", "percent", f64(1))},
		"B": {pt(t, "2026-11-02", "2026-11-02", "percent", f64(2))},
	}}
	cases := []struct {
		name       string
		r          fakeReader
		ids        []string
		dt, sa, tx string
		nTimes     int
	}{
		{"one", one, []string{"A"}, "2026-11-04T00:00:00+08:00", "2026-11-08T09:30:00+08:00", "Data day 11-04", 1},
		{"two", two, []string{"A", "B"}, "2026-11-02T00:00:00+08:00", "2026-11-06T09:30:00+08:00", "Data day 11-02", 2},
		{"none", fakeReader{}, []string{"A"}, "2026-11-05T00:00:00+08:00", "2026-11-09T09:30:00+08:00", "No data day", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := build(t, tc.r, cfg(tc.ids...), "2026-11-05")
			s := m["stale"].(map[string]any)
			if s["data_time"] != tc.dt || s["stale_after"] != tc.sa || s["text"] != tc.tx {
				t.Fatalf("stale = %v", s)
			}
			if n := len(m["data_times"].([]any)); n != tc.nTimes {
				t.Fatalf("data_times = %d", n)
			}
		})
	}
}

func TestFixturesValidate(t *testing.T) {
	t.Parallel()
	read := func(name string) []byte {
		b, err := os.ReadFile(filepath.Join("..", "..", "..", "web", "fixtures", "macro-page", name))
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	for _, n := range []string{"ok.json", "gap.json", "stale.json"} {
		if err := validateBytes(t, read(n)); err != nil {
			t.Errorf("%s: %v", n, err)
		}
	}
	if err := validateBytes(t, read("bad-version.json")); err == nil {
		t.Error("bad-version.json validates, want a failure")
	}
}

func TestBuildErrors(t *testing.T) {
	t.Parallel()
	mod := func(f func(*Config)) Config { c := cfg("S"); f(&c); return c }
	many := cfg()
	for i := 0; i < 21; i++ {
		many.Indicators = append(many.Indicators, Indicator{SeriesID: "S", Name: "n"})
	}
	cases := []struct {
		name string
		c    Config
		day  string
	}{
		{"bad day", cfg("S"), "2026/11/05"},
		{"nil zone", mod(func(c *Config) { c.Zone = nil }), "2026-11-05"},
		{"bad run id", mod(func(c *Config) { c.RunID = "a b" }), "2026-11-05"},
		{"bad build id", mod(func(c *Config) { c.BuildID = "" }), "2026-11-05"},
		{"bad scope", mod(func(c *Config) { c.Scope = strings.Repeat("a", 65) }), "2026-11-05"},
		{"no indicators", cfg(), "2026-11-05"},
		{"too many indicators", many, "2026-11-05"},
		{"empty series id", mod(func(c *Config) { c.Indicators[0].SeriesID = "" }), "2026-11-05"},
		{"empty name", mod(func(c *Config) { c.Indicators[0].Name = "" }), "2026-11-05"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := Build(t.Context(), fakeReader{}, tc.c, contracts.Day(tc.day), fixedAt); err == nil {
				t.Fatal("want an error")
			}
		})
	}
}

func TestBuildSentenceDropsEaseWhenFilled(t *testing.T) {
	t.Parallel()
	m := easeBuild(t, easeRow(t, easeDay, "56.42", "steady"))
	if m["sentence"] != "Macro indicators come from stored series; events: no data yet." {
		t.Errorf("sentence %v", m["sentence"])
	}
	m = easeBuild(t)
	if m["sentence"] != "Macro indicators come from stored series; ease and events: no data yet." {
		t.Errorf("sentence without a reading %v", m["sentence"])
	}
}
