package macro_rates

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"
	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
)

var _ pages.BuildMacroRatesFunc = Build

var fixedAt = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

// fakeReader is an in-memory SeriesReader.
type fakeReader struct {
	points map[string][]contracts.SeriesPoint
	err    error
}

func (f fakeReader) SeriesPoints(_ context.Context, id string, from, to contracts.Day) ([]contracts.SeriesPoint, error) {
	if f.err != nil {
		return nil, f.err
	}
	var out []contracts.SeriesPoint
	for _, p := range f.points[id] {
		d := contracts.Day(p.ObservationDate.Format("2006-01-02"))
		if d >= from && d <= to {
			out = append(out, p)
		}
	}
	return out, nil
}

func pt(t *testing.T, day string, v *float64) contracts.SeriesPoint {
	t.Helper()
	d, err := time.Parse("2006-01-02", day)
	if err != nil {
		t.Fatal(err)
	}
	return contracts.SeriesPoint{
		SeriesID:        "DGS10",
		ObservationDate: types.SerializableDate{Time: d},
		Unit:            contracts.SeriesUnit("percent"),
		Value:           v,
	}
}

func f64(v float64) *float64 { return &v }

type fixtureInput struct {
	SeriesID     string `json:"series_id"`
	Observations []struct {
		Date  string   `json:"date"`
		Value *float64 `json:"value"`
	} `json:"observations"`
}

func fixturePath(name string) string {
	return filepath.Join("..", "..", "..", "contracts", "fixtures", name)
}

func readerFromFixture(t *testing.T) fakeReader {
	t.Helper()
	b, err := os.ReadFile(fixturePath("page.macro_rates.v1.input.json"))
	if err != nil {
		t.Fatal(err)
	}
	var in fixtureInput
	if err := json.Unmarshal(b, &in); err != nil {
		t.Fatal(err)
	}
	var pts []contracts.SeriesPoint
	for _, o := range in.Observations {
		pts = append(pts, pt(t, o.Date, o.Value))
	}
	return fakeReader{points: map[string][]contracts.SeriesPoint{in.SeriesID: pts}}
}

func decode(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func golden(t *testing.T) map[string]any {
	t.Helper()
	b, err := os.ReadFile(fixturePath("page.macro_rates.v2.golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	return decode(t, b)
}

func compileSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	for _, name := range []string{"common.v2.json", "page.macro_rates.v2.json"} {
		f, err := os.Open(filepath.Join("..", "..", "..", "contracts", "schemas", name))
		if err != nil {
			t.Fatal(err)
		}
		doc, err := jsonschema.UnmarshalJSON(f)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		if err := c.AddResource("https://optdesk.invalid/schemas/"+name, doc); err != nil {
			t.Fatal(err)
		}
	}
	s, err := c.Compile("https://optdesk.invalid/schemas/page.macro_rates.v2.json")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestBuildGolden(t *testing.T) {
	t.Parallel()
	out, err := Build(t.Context(), readerFromFixture(t), "2026-11-04", fixedAt)
	if err != nil {
		t.Fatal(err)
	}
	got := decode(t, out)
	if !reflect.DeepEqual(got, golden(t)) {
		t.Fatalf("json = %s\nwant %v", out, golden(t))
	}
	if err := compileSchema(t).Validate(got); err != nil {
		t.Fatal(err)
	}
}

func TestBuildDemo(t *testing.T) {
	t.Parallel()
	demo := fakeReader{points: map[string][]contracts.SeriesPoint{"DGS10": {
		pt(t, "2026-11-02", f64(4.1)),
		pt(t, "2026-11-03", nil),
		pt(t, "2026-11-04", f64(4.2)),
	}}}
	out, err := Build(t.Context(), demo, "2026-11-04", fixedAt)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decode(t, out), golden(t)) {
		t.Fatalf("json = %s", out)
	}
}

func buildGaps(t *testing.T, day contracts.Day, pts ...contracts.SeriesPoint) []map[string]any {
	t.Helper()
	r := fakeReader{points: map[string][]contracts.SeriesPoint{"DGS10": pts}}
	out, err := Build(t.Context(), r, day, fixedAt)
	if err != nil {
		t.Fatal(err)
	}
	m := decode(t, out)
	if err := compileSchema(t).Validate(m); err != nil {
		t.Fatal(err)
	}
	var res []map[string]any
	raw, _ := m["gaps"].([]any)
	for _, g := range raw {
		res = append(res, g.(map[string]any))
	}
	return res
}

func wantGap(date, reason, text string) map[string]any {
	return map[string]any{"series_id": "DGS10", "date": date, "reason": reason, "text": text}
}

func TestGapsAndStale(t *testing.T) {
	t.Run("latest", testLatest)
	t.Parallel()
	t.Run("nil value is one gap, not zero", func(t *testing.T) {
		t.Parallel()
		got := buildGaps(t, "2026-11-04", pt(t, "2026-11-03", nil), pt(t, "2026-11-04", f64(4.2)))
		if want := []map[string]any{wantGap("2026-11-03", "no_value", "no value on 2026-11-03")}; !reflect.DeepEqual(got, want) {
			t.Fatalf("gaps = %v, want %v", got, want)
		}
	})
	t.Run("nil point is null in points", func(t *testing.T) {
		t.Parallel()
		r := fakeReader{points: map[string][]contracts.SeriesPoint{"DGS10": {pt(t, "2026-11-04", nil)}}}
		out, err := Build(t.Context(), r, "2026-11-04", fixedAt)
		if err != nil {
			t.Fatal(err)
		}
		s := decode(t, out)["series"].([]any)[0].(map[string]any)
		p := s["points"].([]any)[0].(map[string]any)
		if v, ok := p["value"]; !ok || v != nil {
			t.Fatalf("value = %v, want null", v)
		}
	})
	t.Run("exactly the limit is fresh", func(t *testing.T) {
		t.Parallel()
		if got := buildGaps(t, "2026-11-08", pt(t, "2026-11-04", f64(4.2))); len(got) != 0 {
			t.Fatalf("gaps = %v, want none", got)
		}
	})
	t.Run("older than the limit is stale at Day", func(t *testing.T) {
		t.Parallel()
		got := buildGaps(t, "2026-11-09", pt(t, "2026-11-04", f64(4.2)))
		if want := []map[string]any{wantGap("2026-11-09", "stale", "no point within 4 days of 2026-11-09")}; !reflect.DeepEqual(got, want) {
			t.Fatalf("gaps = %v, want %v", got, want)
		}
	})
	t.Run("no point in range is stale at Day", func(t *testing.T) {
		t.Parallel()
		got := buildGaps(t, "2026-11-09")
		if want := []map[string]any{wantGap("2026-11-09", "stale", "no point within 4 days of 2026-11-09")}; !reflect.DeepEqual(got, want) {
			t.Fatalf("gaps = %v, want %v", got, want)
		}
	})
	t.Run("stale last point with nil value is one gap", func(t *testing.T) {
		t.Parallel()
		got := buildGaps(t, "2026-11-20", pt(t, "2026-11-04", nil))
		if want := []map[string]any{wantGap("2026-11-20", "stale", "no point within 4 days of 2026-11-20")}; !reflect.DeepEqual(got, want) {
			t.Fatalf("gaps = %v, want %v", got, want)
		}
	})
	t.Run("points before the lookback are not read", func(t *testing.T) {
		t.Parallel()
		got := buildGaps(t, "2026-12-10", pt(t, "2026-11-09", f64(4.0)))
		if want := []map[string]any{wantGap("2026-12-10", "stale", "no point within 4 days of 2026-12-10")}; !reflect.DeepEqual(got, want) {
			t.Fatalf("gaps = %v, want %v", got, want)
		}
	})
}

// latestOf builds a one-series page and returns the latest of its series.
func latestOf(t *testing.T, day contracts.Day, pts ...contracts.SeriesPoint) map[string]any {
	t.Helper()
	r := fakeReader{points: map[string][]contracts.SeriesPoint{"DGS10": pts}}
	out, err := Build(t.Context(), r, day, fixedAt)
	if err != nil {
		t.Fatal(err)
	}
	m := decode(t, out)
	if err := compileSchema(t).Validate(m); err != nil {
		t.Fatal(err)
	}
	return m["series"].([]any)[0].(map[string]any)["latest"].(map[string]any)
}

func testLatest(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		day  contracts.Day
		pts  []contracts.SeriesPoint
		want map[string]any
	}{
		{"stale", "2026-11-09", []contracts.SeriesPoint{pt(t, "2026-11-04", f64(4.2))},
			map[string]any{"kind": "gap", "reason": "stale", "text": "latest point 2026-11-04 is old"}},
		{"no point", "2026-11-09", nil,
			map[string]any{"kind": "gap", "reason": "no_data", "text": "no points"}},
		{"null last", "2026-11-04", []contracts.SeriesPoint{pt(t, "2026-11-04", nil)},
			map[string]any{"kind": "gap", "reason": "no_data", "text": "no value on 2026-11-04"}},
		{"value", "2026-11-04", []contracts.SeriesPoint{pt(t, "2026-11-04", f64(4.2))},
			map[string]any{"kind": "value", "date": "2026-11-04", "value": 4.2, "text": "4.20%"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := latestOf(t, c.day, c.pts...); !reflect.DeepEqual(got, c.want) {
				t.Fatalf("latest = %v, want %v", got, c.want)
			}
		})
	}
}

func TestText(t *testing.T) {
	t.Parallel()
	cases := []struct{ name, got, want string }{
		{"percent", pointText("percent", f64(4.1)), "4.10%"},
		{"ratio", pointText("ratio", f64(0.5)), "0.5000"},
		{"ccy_amount", pointText("ccy_amount", f64(1234.5)), "1234.50"},
		{"count", pointText("count", f64(42)), "42"},
		{"null", pointText("percent", nil), "no value"},
		{"title", pageTitle, "Macro rates"},
		{"sentence", pageSentence(1, "2026-11-04"), "1 series on 2026-11-04."},
		{"series title", seriesTitle("DGS10"), "10-year Treasury yield"},
		{"series title other", seriesTitle("X"), "X"},
		{"no value gap", noValueText("2026-11-03"), "no value on 2026-11-03"},
		{"stale gap line", staleGapText("2026-11-04"), "no point within 4 days of 2026-11-04"},
		{"stale latest", staleLatestText("2026-10-28"), "latest point 2026-10-28 is old"},
		{"no points", noPointsText, "no points"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, c.got, c.want)
		}
	}
}

func TestBuildFixtures(t *testing.T) {
	t.Parallel()
	cases := map[string]fakeReader{
		"page.macro_rates.v2.gap.json": {points: map[string][]contracts.SeriesPoint{"DGS10": {
			pt(t, "2026-11-02", f64(4.1)), pt(t, "2026-11-03", f64(4.2)), pt(t, "2026-11-04", nil)}}},
		"page.macro_rates.v2.stale.json": {points: map[string][]contracts.SeriesPoint{"DGS10": {
			pt(t, "2026-10-27", f64(4.1)), pt(t, "2026-10-28", f64(4.2))}}},
	}
	for name, r := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			out, err := Build(t.Context(), r, "2026-11-04", fixedAt)
			if err != nil {
				t.Fatal(err)
			}
			b, err := os.ReadFile(fixturePath(name))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(decode(t, out), decode(t, b)) {
				t.Fatalf("json = %s\nwant %s", out, b)
			}
		})
	}
}

func TestBuildErrors(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	if _, err := Build(t.Context(), fakeReader{err: boom}, "2026-11-04", fixedAt); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
	if _, err := Build(t.Context(), fakeReader{}, "not-a-day", fixedAt); err == nil {
		t.Fatal("want error for a bad day")
	}
}
