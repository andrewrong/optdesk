package market_page

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

	"github.com/atombender/go-jsonschema/pkg/types"
	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/andrewrong/optdesk/internal/contracts"
)

const (
	schemaDir  = "../../../contracts/schemas/"
	schemaHost = "https://optdesk.invalid/schemas/"
)

var testZone = time.FixedZone("test", 8*3600)

func date(s string) time.Time {
	t, err := time.Parse(dayLayout, s)
	if err != nil {
		panic(err)
	}
	return t
}

func sdate(s string) types.SerializableDate { return types.SerializableDate{Time: date(s)} }

// rec is a stored reading row; code and dataDay are optional.
func rec(day, state, value string, code *int, dataDay string) contracts.ReadingRecord {
	r := contracts.ReadingRecord{Day: sdate(day), ReasonCode: code}
	if state == "ok" {
		r.State = "ok"
	} else {
		r.State = "missing_input"
	}
	if value != "" {
		v := value
		r.Value = &v
	}
	if dataDay != "" {
		raw := `[{"param":"data_day","value_num":null,"value_tag":"` + dataDay + `"}]`
		if err := json.Unmarshal([]byte(raw), &r.Parameters); err != nil {
			panic(err)
		}
	}
	return r
}

func intp(i int) *int { return &i }

func pt(day string, v float64) contracts.SeriesPoint {
	return contracts.SeriesPoint{ObservationDate: sdate(day), RealtimeStart: sdate(day), RealtimeEnd: sdate("9999-12-31"), Value: &v}
}

type fakeSeries struct {
	points map[string][]contracts.SeriesPoint
	err    error
}

func (f fakeSeries) SeriesPoints(_ context.Context, id string, _, _ contracts.Day) ([]contracts.SeriesPoint, error) {
	return f.points[id], f.err
}

// env is the stored data of one build.
type env struct {
	day      string
	readings map[string][]contracts.ReadingRecord
	iv       map[string][]contracts.ReadingRecord // market.iv_ibkr rows by subject
	rv       map[string][]contracts.ReadingRecord // market.rv20 rows by subject
	ivrv     map[string][]contracts.ReadingRecord // market.iv_rv20 rows by subject
	series   map[string][]contracts.SeriesPoint
	rerr     error
	serr     error
	zone     *time.Location
}

func (e env) config(t *testing.T) Config {
	t.Helper()
	zone := e.zone
	if zone == nil {
		zone = testZone
	}
	return Config{RunID: "run1", BuildID: "b1", Scope: "s1", Zone: zone,
		Readings: func(_ context.Context, name, kind, subject string, from, to contracts.Day) ([]contracts.ReadingRecord, error) {
			wantKind := "market"
			if name == figIVIbkr || name == figRv20 || name == figIvRv20 {
				wantKind = "underlying"
			}
			if kind != wantKind {
				t.Errorf("%s: subject kind %q, want %q", name, kind, wantKind)
			}
			day := e.day
			if day == "" {
				day = "2026-11-06"
			}
			if string(to) != day || string(from) != date(day).AddDate(0, 0, -5).Format(dayLayout) {
				t.Errorf("window %s..%s for day %s", from, to, day)
			}
			if name == figIVIbkr {
				return e.iv[subject], e.rerr
			}
			if name == figRv20 {
				return e.rv[subject], e.rerr
			}
			if name == figIvRv20 {
				return e.ivrv[subject], e.rerr
			}
			return e.readings[name], e.rerr
		}}
}

func (e env) build(t *testing.T) ([]byte, error) {
	t.Helper()
	day := e.day
	if day == "" {
		day = "2026-11-06"
	}
	at := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	return Build(context.Background(), e.config(t), fakeSeries{points: e.series, err: e.serr}, contracts.Day(day), at)
}

func (e env) page(t *testing.T) map[string]any {
	t.Helper()
	b, err := e.build(t)
	if err != nil {
		t.Fatal(err)
	}
	return decode(t, b)
}

func decode(t *testing.T, b []byte) map[string]any {
	t.Helper()
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var m map[string]any
	if err := d.Decode(&m); err != nil {
		t.Fatal(err)
	}
	return m
}

// at walks a decoded page by string keys and int indexes.
func at(t *testing.T, v any, path ...any) any {
	t.Helper()
	for _, p := range path {
		switch k := p.(type) {
		case string:
			m, ok := v.(map[string]any)
			if !ok {
				t.Fatalf("%v: not an object at %q", path, k)
			}
			v = m[k]
		case int:
			a, ok := v.([]any)
			if !ok || k >= len(a) {
				t.Fatalf("%v: no index %d", path, k)
			}
			v = a[k]
		}
	}
	return v
}

func rank(t *testing.T, m map[string]any, row int) map[string]any {
	t.Helper()
	return at(t, m, "indices", "rows", row, "rank_3y").(map[string]any)
}

func allOK() env {
	ok := func(v string) []contracts.ReadingRecord {
		return []contracts.ReadingRecord{rec("2026-11-06", "ok", v, nil, "2026-11-06")}
	}
	return env{
		readings: map[string][]contracts.ReadingRecord{
			figVixPctile: ok("0.685000"), figVxnPctile: ok("0.125000"), figRvxPctile: ok("0.135000"),
			figSp500Vs200d: ok("0.031250"), figVixTerm: ok("0.905000"),
		},
		iv: map[string][]contracts.ReadingRecord{
			"SPX": ok("0.215000"), "NDX": ok("0.2349"), "RUT": ok("0.1999"),
		},
		rv: map[string][]contracts.ReadingRecord{
			"SPX": ok("0.162060"), "NDX": ok("0.162500"), "RUT": ok("0.163500"),
		},
		ivrv: map[string][]contracts.ReadingRecord{
			"SPX": ok("0.032000"), "NDX": ok("0.072000"), "RUT": ok("-0.014000"),
		},
		series: map[string][]contracts.SeriesPoint{
			seriesVIX: {pt("2026-11-06", 21.5), pt("2026-10-30", 19.25)},
			seriesVXV: {pt("2026-11-06", 23), pt("2026-10-30", 22)},
		},
	}
}

func withVix(r contracts.ReadingRecord) env {
	e := allOK()
	e.readings[figVixPctile] = []contracts.ReadingRecord{r}
	return e
}

func TestPercentileThreeOk(t *testing.T) {
	t.Parallel()
	m := allOK().page(t)
	for i, want := range []string{"68", "12", "14"} {
		r := rank(t, m, i)
		if r["kind"] != "value" || r["unit"] != "share_percent" || r["text"] != want+"%" || fmt.Sprint(r["value"]) != want {
			t.Errorf("row %d rank_3y = %v, want %s", i, r, want)
		}
	}
	if _, isNum := rank(t, m, 0)["value"].(json.Number); !isNum {
		t.Error("value is not a JSON number")
	}
}

func TestPercentileShortHistoryIsNoData(t *testing.T) {
	t.Parallel()
	r := rank(t, withVix(rec("2026-11-06", "missing_input", "", intp(2), "")).page(t), 0)
	if r["kind"] != "gap" || r["reason"] != "no_data" || r["text"] != "History under 3 years, no percentile" {
		t.Errorf("rank_3y = %v", r)
	}
}

func TestPercentileStaleInputUsesDataDay(t *testing.T) {
	t.Parallel()
	r := rank(t, withVix(rec("2026-11-06", "missing_input", "", intp(3), "2026-11-03")).page(t), 0)
	if r["reason"] != "stale_input" || r["text"] != "Percentile data is stale, data day 11-03" {
		t.Errorf("rank_3y = %v", r)
	}
	r = rank(t, withVix(rec("2026-11-06", "missing_input", "", intp(3), "")).page(t), 0)
	if r["reason"] != "stale_input" || r["text"] != "Percentile data is stale" {
		t.Errorf("no data day: rank_3y = %v", r)
	}
}

func TestPercentileOtherReasonIsMissingInput(t *testing.T) {
	t.Parallel()
	for name, r := range map[string]contracts.ReadingRecord{
		"code 1":         rec("2026-11-06", "missing_input", "", intp(1), ""),
		"ok without val": rec("2026-11-06", "ok", "", nil, ""),
		"bad text":       rec("2026-11-06", "ok", "abc", nil, ""),
		"above one":      rec("2026-11-06", "ok", "1.000001", nil, ""),
		"negative":       rec("2026-11-06", "ok", "-0.1", nil, ""),
	} {
		g := rank(t, withVix(r).page(t), 0)
		if g["reason"] != "missing_input" || g["text"] != "Percentile: n/a, input incomplete" {
			t.Errorf("%s: rank_3y = %v", name, g)
		}
	}
	edge := rank(t, withVix(rec("2026-11-06", "ok", "1.000000", nil, "")).page(t), 0)
	if fmt.Sprint(edge["value"]) != "100" {
		t.Errorf("1.000000: %v", edge)
	}
}

func TestPercentileNoRowIsNoData(t *testing.T) {
	t.Parallel()
	e := allOK()
	delete(e.readings, figVixPctile)
	r := rank(t, e.page(t), 0)
	if r["reason"] != "no_data" || r["text"] != "Percentile: no data yet, nothing stored" {
		t.Errorf("rank_3y = %v", r)
	}
}

func TestPercentileNumericReasonCodes(t *testing.T) {
	t.Parallel()
	codes := []*int{intp(1), intp(2), intp(3), intp(4), intp(5), nil}
	want := []string{"missing_input", "no_data", "stale_input", "missing_input", "missing_input", "missing_input"}
	for i, c := range codes {
		got := rank(t, withVix(rec("2026-11-06", "missing_input", "", c, "")).page(t), 0)["reason"]
		if got != want[i] {
			t.Errorf("case %d: reason = %v, want %s", i, got, want[i])
		}
	}
}

func TestPercentileDataDayNotAsOfDay(t *testing.T) {
	t.Parallel()
	times := func(m map[string]any) map[string]string {
		out := map[string]string{}
		for _, x := range at(t, m, "data_times").([]any) {
			o := x.(map[string]any)
			out[o["source"].(string)] = o["data_time"].(string)
		}
		return out
	}
	e := withVix(rec("2026-11-06", "missing_input", "", intp(3), "2026-11-03"))
	e.series = nil
	e.readings[figVixTerm] = nil
	m := e.page(t)
	if txt := rank(t, m, 0)["text"]; !strings.HasSuffix(txt.(string), "11-03") {
		t.Errorf("text = %v", txt)
	}
	if got := times(m)[seriesVIX]; got != "2026-11-03T00:00:00+08:00" {
		t.Errorf("VIXCLS data time = %q", got)
	}
	// no data_day and no VIXCLS point: no VIXCLS entry.
	e = withVix(rec("2026-11-06", "ok", "0.5", nil, ""))
	e.series = nil
	e.readings[figVixTerm] = nil
	if got, has := times(e.page(t))[seriesVIX]; has {
		t.Errorf("VIXCLS entry %q without a data day", got)
	}
	// with a VIXCLS point the entry is the date of the point.
	e.series = map[string][]contracts.SeriesPoint{seriesVIX: {pt("2026-11-05", 20)}}
	if got := times(e.page(t))[seriesVIX]; got != "2026-11-05T00:00:00+08:00" {
		t.Errorf("VIXCLS data time = %q", got)
	}
}

func TestStaleLabelOldestDataDay(t *testing.T) {
	t.Parallel()
	e := allOK()
	e.readings[figVxnPctile] = []contracts.ReadingRecord{rec("2026-11-06", "ok", "0.125", nil, "2026-11-02")}
	e.day = "2026-11-09"
	e.series = map[string][]contracts.SeriesPoint{seriesVIX: {pt("2026-11-06", 21)}, seriesVXV: {pt("2026-11-06", 22)}}
	m := e.page(t)
	s := at(t, m, "stale").(map[string]any)
	if s["text"] != "Data day 11-02" || s["is_stale"] != true || s["data_time"] != "2026-11-02T00:00:00+08:00" {
		t.Errorf("stale = %v", s)
	}
	if m["stale_after"] != "2026-11-06T09:30:00+08:00" {
		t.Errorf("stale_after = %v", m["stale_after"])
	}
	e.readings[figVxnPctile] = []contracts.ReadingRecord{rec("2026-11-06", "ok", "0.125", nil, "2026-11-05")}
	e.day = "2026-11-09"
	if s := at(t, e.page(t), "stale").(map[string]any); s["is_stale"] != false || s["text"] != "Data day 11-05" {
		t.Errorf("fresh stale = %v", s)
	}
	none := env{}.page(t)
	if none["stale"] != nil {
		t.Errorf("no data day: stale = %v", none["stale"])
	}
	if _, has := none["stale_after"]; has {
		t.Error("stale_after without a data day")
	}
}

func TestGeneratedAtInZone(t *testing.T) {
	t.Parallel()
	m := allOK().page(t)
	if m["generated_at"] != "2026-11-06T20:00:00+08:00" {
		t.Errorf("generated_at = %v", m["generated_at"])
	}
	if m["schema"] != "market_page" || m["schema_version"].(json.Number) != "1" || m["run_id"] != "run1" {
		t.Errorf("envelope = %v", m)
	}
}

func compileSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	c.UseLoader(fileLoader{})
	s, err := c.Compile(schemaHost + "page.market_page.v1.json")
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

func TestBuildMatchesSchema(t *testing.T) {
	t.Parallel()
	schema := compileSchema(t)
	gapVix := func(code *int, state string) env { return withVix(rec("2026-11-06", state, "", code, "2026-11-03")) }
	noSeries := allOK()
	noSeries.series = nil
	oneSeriesPoint := allOK()
	oneSeriesPoint.series = map[string][]contracts.SeriesPoint{seriesVIX: {pt("2026-11-06", 21)}}
	staleSeries := allOK()
	staleSeries.series = map[string][]contracts.SeriesPoint{seriesVIX: {pt("2026-10-30", 21)}, seriesVXV: {pt("2026-11-06", 21)}}
	everyKind := allOK()
	everyKind.readings[figVixPctile] = []contracts.ReadingRecord{rec("2026-11-06", "missing_input", "", intp(2), "")}
	everyKind.readings[figVxnPctile] = []contracts.ReadingRecord{rec("2026-11-06", "missing_input", "", intp(3), "2026-11-03")}
	everyKind.readings[figRvxPctile] = []contracts.ReadingRecord{rec("2026-11-06", "ok", "abc", nil, "")}
	everyKind.readings[figSp500Vs200d] = nil
	everyKind.readings[figVixTerm] = nil
	ivrvGaps := allOK()
	ivrvGaps.ivrv = map[string][]contracts.ReadingRecord{
		"SPX": {rec("2026-11-06", "missing_input", "", intp(3), "2026-10-30")},
		"NDX": {rec("2026-11-06", "ok", "0.01", nil, "")},
	}
	cases := map[string]env{
		"negative iv minus rv": allOK(), "iv minus rv gaps": ivrvGaps,
		"all ok": allOK(), "short history": gapVix(intp(2), "missing_input"), "stale input": gapVix(intp(3), "missing_input"),
		"missing input": gapVix(intp(1), "missing_input"), "no code": gapVix(nil, "missing_input"),
		"no readings": {}, "no term series": noSeries, "one point": oneSeriesPoint, "stale series": staleSeries,
		"every kind": everyKind,
	}
	for name, e := range cases {
		b, err := e.build(t)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var v any
		if v, err = jsonschema.UnmarshalJSON(bytes.NewReader(b)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if err := schema.Validate(v); err != nil {
			t.Errorf("%s: %v\n%s", name, err, b)
		}
	}
}

func TestReaderErrorIsError(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	e := allOK()
	e.rerr = boom
	if _, err := e.build(t); !errors.Is(err, boom) {
		t.Errorf("readings error = %v", err)
	}
	e = allOK()
	e.serr = boom
	if _, err := e.build(t); !errors.Is(err, boom) {
		t.Errorf("series error = %v", err)
	}
}

func TestNilZoneIsError(t *testing.T) {
	t.Parallel()
	e := allOK()
	c := e.config(t)
	c.Zone = nil
	if _, err := Build(context.Background(), c, fakeSeries{}, "2026-11-06", time.Time{}); err == nil {
		t.Error("nil zone: no error")
	}
	c = e.config(t)
	c.Readings = nil
	if _, err := Build(context.Background(), c, fakeSeries{}, "2026-11-06", time.Time{}); err == nil {
		t.Error("nil readings: no error")
	}
	if _, err := Build(context.Background(), e.config(t), fakeSeries{}, "bad", time.Time{}); err == nil {
		t.Error("bad day: no error")
	}
}

func TestRunStatusIgnoresPermanentGaps(t *testing.T) {
	t.Parallel()
	m := allOK().page(t)
	if m["run_status"] != "ok" {
		t.Errorf("all ok: run_status = %v", m["run_status"])
	}
	for _, path := range [][]any{{"breadth"}, {"weather", "chart"}, {"indices", "rows", 3, "iv"}, {"indices", "rows", 3, "rank_3y"}} {
		if at(t, m, append(path, "kind")...) != "gap" {
			t.Errorf("%v is not a gap", path)
		}
	}
	if got := withVix(rec("2026-11-06", "missing_input", "", intp(2), "")).page(t)["run_status"]; got != "partial" {
		t.Errorf("short history: run_status = %v", got)
	}
	e := allOK()
	delete(e.readings, figVixTerm)
	m = e.page(t)
	if m["run_status"] != "ok" || at(t, m, "term", "sentence") != "Term structure ratio: nothing stored." {
		t.Errorf("no term ratio: %v %v", m["run_status"], at(t, m, "term", "sentence"))
	}
	e = allOK()
	delete(e.readings, figSp500Vs200d)
	if e.page(t)["run_status"] != "partial" {
		t.Error("no weather reading: not partial")
	}
	if got := (env{}).page(t)["run_status"]; got != "partial" {
		t.Errorf("empty store: run_status = %v", got)
	}
}

// pageGaps lists the Gap objects of the blocks in page order, found by path.
func pageGaps(t *testing.T, m map[string]any) []any {
	t.Helper()
	var out []any
	take := func(path ...any) {
		if g, ok := at(t, m, path...).(map[string]any); ok && g["kind"] == "gap" {
			out = append(out, g)
		}
	}
	take("weather", "chart")
	for i := range 4 {
		for _, k := range []string{"iv", "rv20", "iv_minus_rv20", "rank_3y"} {
			take("indices", "rows", i, k)
		}
	}
	if at(t, m, "term", "kind") == "gap" {
		take("term")
	} else {
		for i := range 2 {
			take("term", "points", i, "today")
			take("term", "points", i, "five_days_ago")
		}
	}
	take("breadth")
	return out
}

func TestEnvelopeGapsEqualBlockGaps(t *testing.T) {
	t.Parallel()
	mixed := allOK()
	mixed.readings[figVixPctile] = []contracts.ReadingRecord{rec("2026-11-06", "missing_input", "", intp(2), "")}
	mixed.readings[figVxnPctile] = []contracts.ReadingRecord{rec("2026-11-06", "missing_input", "", intp(3), "")}
	mixed.readings[figRvxPctile] = []contracts.ReadingRecord{rec("2026-11-06", "ok", "x", nil, "")}
	mixed.series = map[string][]contracts.SeriesPoint{seriesVIX: {pt("2026-11-01", 20)}, seriesVXV: {pt("2026-11-06", 22)}}
	for name, e := range map[string]env{"all ok": allOK(), "mixed": mixed, "empty": {}} {
		m := e.page(t)
		want := pageGaps(t, m)
		got := at(t, m, "gaps").([]any)
		if len(got) != len(want) || len(got) < 6 {
			t.Fatalf("%s: %d gaps, want %d (at least 6)", name, len(got), len(want))
		}
		for i := range want {
			if fmt.Sprint(got[i]) != fmt.Sprint(want[i]) {
				t.Errorf("%s: gaps[%d] = %v, want %v", name, i, got[i], want[i])
			}
		}
	}
	kinds := map[string]bool{}
	for _, g := range at(t, mixed.page(t), "gaps").([]any) {
		kinds[g.(map[string]any)["reason"].(string)] = true
	}
	for _, k := range []string{"not_built", "no_data", "stale", "missing_input", "stale_input"} {
		if !kinds[k] {
			t.Errorf("mixed page has no gap of reason %s", k)
		}
	}
}

func ivCellOf(t *testing.T, m map[string]any, row int) map[string]any {
	t.Helper()
	return at(t, m, "indices", "rows", row, "iv").(map[string]any)
}

func TestIvShowsStoredIbkrValue(t *testing.T) {
	t.Parallel()
	m := allOK().page(t)
	for i, w := range []struct{ text, value string }{{"21.5", "21.5"}, {"23.5", "23.5"}, {"20.0", "20.0"}} {
		c := ivCellOf(t, m, i)
		if c["kind"] != "value" || c["text"] != w.text || c["unit"] != "vol_points" || fmt.Sprint(c["value"]) != w.value {
			t.Errorf("row %d iv = %v", i, c)
		}
	}
	if m["run_status"] != "ok" {
		t.Errorf("run_status = %v", m["run_status"])
	}
}

func TestIvRvGapsLeaveRunStatusOk(t *testing.T) {
	t.Parallel()
	e := allOK()
	e.ivrv = nil
	m := e.page(t)
	if m["run_status"] != "ok" {
		t.Errorf("run_status = %v", m["run_status"])
	}
	if g := at(t, m, "indices", "rows", 0, "iv_minus_rv20").(map[string]any); g["reason"] != "no_data" {
		t.Errorf("SPX iv_minus_rv20 = %v", g)
	}
	c := at(t, allOK().page(t), "indices", "rows", 2, "iv_minus_rv20").(map[string]any)
	if c["kind"] != "value" || c["text"] != "-1.4" {
		t.Errorf("RUT iv_minus_rv20 = %v", c)
	}
}

func TestIvGapsNameTheIndex(t *testing.T) {
	t.Parallel()
	e := allOK()
	delete(e.iv, "NDX")
	e.iv["RUT"] = []contracts.ReadingRecord{rec("2026-11-06", "missing_input", "", intp(1), "")}
	m := e.page(t)
	if c := ivCellOf(t, m, 1); c["kind"] != "gap" || c["text"] != "NDX implied volatility: no data, not stored yet" {
		t.Errorf("NDX iv = %v", c)
	}
	if c := ivCellOf(t, m, 2); c["kind"] != "gap" || c["text"] != "RUT implied volatility: no data, inputs incomplete" {
		t.Errorf("RUT iv = %v", c)
	}
	for _, v := range []string{"0", "-0.1"} {
		e := allOK()
		e.iv["SPX"] = []contracts.ReadingRecord{rec("2026-11-06", "ok", v, nil, "")}
		if c := ivCellOf(t, e.page(t), 0); c["kind"] != "gap" || c["text"] != "SPX implied volatility: no data, inputs incomplete" {
			t.Errorf("value %s: iv = %v", v, c)
		}
	}
}

func TestN225IvAndUniqueGapTexts(t *testing.T) {
	t.Parallel()
	m := allOK().page(t)
	if c := ivCellOf(t, m, 3); c["text"] != "N225 implied volatility: not built yet" {
		t.Errorf("N225 iv = %v", c)
	}
	seen := map[string]bool{}
	for _, g := range at(t, m, "gaps").([]any) {
		txt := g.(map[string]any)["text"].(string)
		if seen[txt] {
			t.Errorf("gap text twice: %q", txt)
		}
		seen[txt] = true
	}
}
