package home_market

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

	"github.com/atombender/go-jsonschema/pkg/types"
	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

var testZone = time.FixedZone("test", 8*3600)

func sdate(s string) types.SerializableDate {
	t, err := time.Parse(dayLayout, s)
	if err != nil {
		panic(err)
	}
	return types.SerializableDate{Time: t}
}

type fakeSeries map[string][]contracts.SeriesPoint

func (f fakeSeries) SeriesPoints(_ context.Context, id string, from, to contracts.Day) ([]contracts.SeriesPoint, error) {
	var out []contracts.SeriesPoint
	for _, p := range f[id] {
		d := p.ObservationDate.Format(dayLayout)
		if d >= string(from) && d <= string(to) {
			out = append(out, p)
		}
	}
	return out, nil
}

func pt(day string, v float64) contracts.SeriesPoint {
	return contracts.SeriesPoint{ObservationDate: sdate(day), RealtimeStart: sdate(day), Value: &v}
}

func reading(day, state, value string) contracts.ReadingRecord {
	r := contracts.ReadingRecord{Day: sdate(day)}
	if state == "ok" {
		r.State = "ok"
	} else {
		r.State = "missing_input"
	}
	if value != "" {
		r.Value = &value
	}
	return r
}

// readings answers market.iv_ibkr from iv and market.rv20 from rv, both keyed by subject.
func readings(iv, rv map[string]contracts.ReadingRecord) compute.ReadingFunc {
	return func(_ context.Context, name, kind, subject string, _, _ contracts.Day) ([]contracts.ReadingRecord, error) {
		m := iv
		switch {
		case kind != "underlying":
			return nil, errors.New("unexpected reading " + name + " " + kind)
		case name == figRv20:
			m = rv
		case name != figIV:
			return nil, errors.New("unexpected reading " + name + " " + kind)
		}
		if r, ok := m[subject]; ok {
			return []contracts.ReadingRecord{r}, nil
		}
		return nil, nil
	}
}

func config(m map[string]contracts.ReadingRecord) Config { return configRv(m, nil) }

func configRv(iv, rv map[string]contracts.ReadingRecord) Config {
	return Config{RunID: "run1", BuildID: "b1", Scope: "market", Zone: testZone, Readings: readings(iv, rv)}
}

var at = time.Date(2026, 11, 6, 21, 30, 0, 0, time.UTC)

func fullSeries() fakeSeries {
	s := fakeSeries{
		"VIXCLS": {pt("2026-11-05", 17.9), pt("2026-11-06", 18.2)},
		"VXNCLS": {pt("2026-11-06", 21)},
		"RVXCLS": {pt("2026-11-06", 23.456)},
	}
	for i, d := range []string{"2026-11-02", "2026-11-03", "2026-11-04", "2026-11-05", "2026-11-06"} {
		s["SP500"] = append(s["SP500"], pt(d, 5000+float64(i)*10.5))
	}
	return s
}

type fileLoader struct{}

func (fileLoader) Load(url string) (any, error) {
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "schemas", filepath.Base(url)))
	if err != nil {
		return nil, err
	}
	return jsonschema.UnmarshalJSON(bytes.NewReader(b))
}

func validateDoc(t *testing.T, b []byte) {
	t.Helper()
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	c.UseLoader(fileLoader{})
	s, err := c.Compile("https://optdesk.invalid/schemas/page.market_summary.v3.json")
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
}

func build(t *testing.T, c Config, r fakeSeries) []byte {
	t.Helper()
	b, err := Build(context.Background(), c, r, "2026-11-06", at)
	if err != nil {
		t.Fatal(err)
	}
	validateDoc(t, b)
	return b
}

func TestGolden(t *testing.T) {
	t.Parallel()
	c := configRv(map[string]contracts.ReadingRecord{
		"SPX": reading("2026-11-06", "ok", "0.201234"),
		"NDX": reading("2026-11-06", "ok", "0.245"),
		"RUT": reading("2026-11-06", "ok", "0.2175"),
	}, map[string]contracts.ReadingRecord{
		"SPX": reading("2026-11-06", "ok", "0.162060"),
		"NDX": reading("2026-11-06", "ok", "0.162500"),
		"RUT": reading("2026-11-06", "ok", "0.163500"),
	})
	got := build(t, c, fullSeries())
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
		t.Fatalf("golden differs:\n%s", pretty.String())
	}
}

type doc struct {
	RunStatus string `json:"run_status"`
	Card      struct {
		Say   string `json:"say"`
		Chart struct {
			Points []map[string]any `json:"points"`
		} `json:"chart"`
		IndexTable struct {
			Rows []map[string]any `json:"rows"`
		} `json:"index_table"`
		VolIndices []struct {
			Name  string         `json:"name"`
			Level map[string]any `json:"level"`
		} `json:"vol_indices"`
		Gaps []map[string]any `json:"gaps"`
	} `json:"card"`
}

func parse(t *testing.T, b []byte) doc {
	t.Helper()
	var d doc
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestNoSeriesIsGap(t *testing.T) {
	t.Parallel()
	d := parse(t, build(t, config(nil), fakeSeries{}))
	if d.RunStatus != "partial" {
		t.Fatalf("run_status = %s", d.RunStatus)
	}
	if len(d.Card.Chart.Points) != 0 {
		t.Fatalf("points = %v", d.Card.Chart.Points)
	}
	for _, v := range d.Card.VolIndices {
		if v.Level["kind"] != "gap" {
			t.Errorf("%s level = %v", v.Name, v.Level)
		}
	}
	for _, row := range d.Card.IndexTable.Rows {
		if cell(row, "iv")["kind"] != "gap" {
			t.Errorf("iv = %v", row["iv"])
		}
	}
	if len(d.Card.Gaps) != 3+3+1+3+1 {
		t.Errorf("gaps = %d", len(d.Card.Gaps))
	}
}

func TestGapTextsAreDistinctAndNameTheirIndex(t *testing.T) {
	t.Parallel()
	d := parse(t, build(t, config(nil), fakeSeries{}))
	seen := map[string]bool{}
	for _, g := range d.Card.Gaps {
		text, _ := g["text"].(string)
		if seen[text] {
			t.Errorf("gap text %q appears twice", text)
		}
		seen[text] = true
	}
	for _, name := range []string{"SPX", "NDX", "RUT"} {
		n := 0
		for text := range seen {
			if strings.HasPrefix(text, name+" ") {
				n++
			}
		}
		if n != 2 {
			t.Errorf("%s: %d gap texts start with its name, want 2 (iv, rv20)", name, n)
		}
	}
}

func TestOneIndexWithoutSeriesIsGap(t *testing.T) {
	t.Parallel()
	s := fullSeries()
	delete(s, "VXNCLS")
	c := config(map[string]contracts.ReadingRecord{
		"SPX": reading("2026-11-06", "ok", "0.2"),
		"NDX": reading("2026-11-06", "missing_input", ""),
		"RUT": reading("2026-11-06", "ok", "0.0"),
	})
	d := parse(t, build(t, c, s))
	rows := d.Card.IndexTable.Rows
	if cell(rows[0], "iv")["kind"] != "value" || cell(rows[0], "iv")["text"] != "20.0" {
		t.Errorf("SPX iv = %v", rows[0]["iv"])
	}
	for _, i := range []int{1, 2} {
		if cell(rows[i], "iv")["kind"] != "gap" {
			t.Errorf("row %d iv = %v", i, rows[i]["iv"])
		}
	}
	if d.Card.VolIndices[0].Level["kind"] != "value" || d.Card.VolIndices[1].Level["kind"] != "gap" || d.Card.VolIndices[2].Level["kind"] != "value" {
		t.Errorf("vol levels = %+v", d.Card.VolIndices)
	}
	if len(d.Card.Chart.Points) != 5 {
		t.Errorf("points = %d", len(d.Card.Chart.Points))
	}
}

func TestStaleLevelIsGap(t *testing.T) {
	t.Parallel()
	s := fullSeries()
	s["VIXCLS"] = []contracts.SeriesPoint{pt("2026-10-30", 19)}
	d := parse(t, build(t, config(nil), s))
	if d.Card.VolIndices[0].Level["kind"] != "gap" || d.Card.VolIndices[0].Level["reason"] != "stale" {
		t.Errorf("VIX level = %v", d.Card.VolIndices[0].Level)
	}
}

func TestNegativeLevelIsGap(t *testing.T) {
	t.Parallel()
	s := fullSeries()
	s["VXNCLS"] = []contracts.SeriesPoint{pt("2026-11-06", -1)}
	d := parse(t, build(t, config(nil), s))
	if d.Card.VolIndices[1].Level["kind"] != "gap" {
		t.Errorf("VXN level = %v", d.Card.VolIndices[1].Level)
	}
}

func TestReaderErrorIsError(t *testing.T) {
	t.Parallel()
	c := config(nil)
	c.Readings = func(context.Context, string, string, string, contracts.Day, contracts.Day) ([]contracts.ReadingRecord, error) {
		return nil, errors.New("boom")
	}
	if _, err := Build(context.Background(), c, fullSeries(), "2026-11-06", at); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err = %v", err)
	}
}

func TestBadConfig(t *testing.T) {
	t.Parallel()
	bad := map[string]Config{}
	c := config(nil)
	c.Zone = nil
	bad["zone"] = c
	c = config(nil)
	c.Readings = nil
	bad["readings"] = c
	c = config(nil)
	c.RunID = "bad id!"
	bad["run id"] = c
	for name, c := range bad {
		if _, err := Build(context.Background(), c, fullSeries(), "2026-11-06", at); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	if _, err := Build(context.Background(), config(nil), nil, "2026-11-06", at); err == nil {
		t.Error("nil series reader: no error")
	}
	if _, err := Build(context.Background(), config(nil), fullSeries(), "nope", at); err == nil {
		t.Error("bad day: no error")
	}
}

func cell(row map[string]any, key string) map[string]any {
	m, _ := row[key].(map[string]any)
	return m
}

func rowsFor(t *testing.T, names ...string) []indexRow {
	t.Helper()
	var rows []indexRow
	for _, n := range names {
		row, _, err := buildRow(context.Background(), config(nil), n, time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC))
		if err != nil {
			t.Fatal(err)
		}
		rows = append(rows, row)
	}
	return rows
}

func countGaps(gs []gap, text string) int {
	n := 0
	for _, g := range gs {
		if g.Text == text && g.Reason == "not_built" {
			n++
		}
	}
	return n
}

func TestIndexGapsOnePerFeature(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		names []string
		list  string
	}{
		{[]string{"SPX", "NDX", "RUT"}, "SPX, NDX, RUT"},
		{[]string{"SPX"}, "SPX"},
	} {
		rows := rowsFor(t, tc.names...)
		gs := collectGaps(card{IndexTable: indexTable{Rows: rows}})
		if n := countGaps(gs, "Implied minus realised not built yet for "+tc.list); n != 1 {
			t.Errorf("%s: diff gaps = %d", tc.list, n)
		}
		for _, r := range rows {
			if g, ok := r.IVMinusRV20.(gap); !ok || g.Reason != "not_built" || g.Text != "not built yet" {
				t.Errorf("%s cell = %v", r.Name, r.IVMinusRV20)
			}
		}
	}
}

func rowWith(t *testing.T, rv map[string]contracts.ReadingRecord) indexRow {
	t.Helper()
	row, _, err := buildRow(context.Background(), configRv(nil, rv), "SPX", time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return row
}

func TestRowRv20Values(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ stored, want string }{
		{"0.162060", "16.2"},
		{"0.162500", "16.2"},
		{"0.163500", "16.4"},
		{"0.000500", "0.0"},
		{"0.000000", "0.0"},
	} {
		row := rowWith(t, map[string]contracts.ReadingRecord{"SPX": reading("2026-11-06", "ok", tc.stored)})
		v, ok := row.RV20.(volPoints)
		if !ok || v.Kind != "value" || v.Text != tc.want || string(v.Value) != tc.want || v.Unit != "vol_points" {
			t.Errorf("%s: cell = %+v, want value %s", tc.stored, row.RV20, tc.want)
		}
	}
}

func TestRowRv20NoRowIsNoData(t *testing.T) {
	t.Parallel()
	g, ok := rowWith(t, nil).RV20.(gap)
	if !ok || g.Reason != "no_data" {
		t.Fatalf("RV20 = %+v", g)
	}
}

func TestRowRv20NotOkIsMissingInput(t *testing.T) {
	t.Parallel()
	for _, r := range []contracts.ReadingRecord{
		reading("2026-11-06", "missing_input", ""),
		reading("2026-11-06", "ok", ""),
	} {
		g, ok := rowWith(t, map[string]contracts.ReadingRecord{"SPX": r}).RV20.(gap)
		if !ok || g.Reason != "missing_input" {
			t.Fatalf("RV20 = %+v", g)
		}
	}
}

func TestRowRv20GapLeavesIVOk(t *testing.T) {
	t.Parallel()
	iv := map[string]contracts.ReadingRecord{"SPX": reading("2026-11-06", "ok", "0.2")}
	row, ok, err := buildRow(context.Background(), configRv(iv, nil), "SPX", time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC))
	if err != nil || !ok {
		t.Fatalf("ok = %v err = %v", ok, err)
	}
	if g, isGap := row.RV20.(gap); !isGap || g.Reason != "no_data" {
		t.Fatalf("RV20 = %+v", row.RV20)
	}
}
