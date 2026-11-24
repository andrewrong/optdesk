package macro_page

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/macroevent"
)

// monthlySeries is n monthly points of id from start (a first of the month), the value of point i being val(i).
func monthlySeries(t *testing.T, id, unit, start string, n int, val func(i int) *float64) []contracts.SeriesPoint {
	t.Helper()
	s, err := time.Parse(dayLayout, start)
	if err != nil {
		t.Fatal(err)
	}
	var out []contracts.SeriesPoint
	for i := 0; i < n; i++ {
		d := s.AddDate(0, i, 0).Format(dayLayout)
		out = append(out, pt(t, d, d, unit, val(i)))
	}
	return out
}

// yoyIndex is an index that is 100 in 2025 and 103 from 2026 (start 2025-01-01), so its y/y is 3.0% from 2026-01.
func yoyIndex(i int) *float64 {
	if i < 12 {
		return f64(100)
	}
	return f64(103)
}

func evAt(kind, date string) macroevent.Event {
	src := "fred"
	if kind == "fomc" || kind == "fomc_minutes" || kind == "boj" || kind == "quarter_end" || kind == "us_tax" {
		src = "file"
	}
	return macroevent.Event{Kind: kind, Source: src, Date: contracts.Day(date), RunDay: "2026-11-04"}
}

func valuesBuild(t *testing.T, r fakeReader, events ...macroevent.Event) map[string]map[string]any {
	t.Helper()
	ev := healthy()
	ev.events = append(ev.events, events...)
	c := cfg("A")
	c.Events = ev
	out := map[string]map[string]any{}
	for _, row := range eventRows(buildWith(t, r, c)) {
		out[row["name_text"].(string)+" "+row["date"].(string)] = row
	}
	return out
}

func figText(t *testing.T, v any) string {
	t.Helper()
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("not an object: %v", v)
	}
	if _, isGap := m["reason"]; isGap {
		t.Fatalf("a gap, want a figure: %v", m)
	}
	return m["text"].(string)
}

func cellGap(t *testing.T, v any) (string, string) {
	t.Helper()
	m, ok := v.(map[string]any)
	if !ok || m["reason"] == nil {
		t.Fatalf("want a gap, got %v", v)
	}
	return m["reason"].(string), m["text"].(string)
}

func daily2(t *testing.T, id, from string, n int, val func(i int) float64) []contracts.SeriesPoint {
	t.Helper()
	s, _ := time.Parse(dayLayout, from)
	var out []contracts.SeriesPoint
	for i := 0; i < n; i++ {
		d := s.AddDate(0, 0, i).Format(dayLayout)
		out = append(out, pt(t, d, d, "percent", f64(val(i))))
	}
	return out
}

func TestEventValuesPerKind(t *testing.T) {
	t.Parallel()
	// Future events: stored months run to Sep 2026 (Aug for JOLTS), so each previous is the newest stored figure.
	ppi := monthlySeries(t, "PPIFID", "index_points", "2025-01-01", 21, yoyIndex)
	payems := monthlySeries(t, "PAYEMS", "count", "2025-01-01", 21, func(i int) *float64 { return f64(158_000_000 + 200_000*float64(i)) })
	rsafs := monthlySeries(t, "RSAFS", "ccy_amount", "2025-01-01", 21, func(i int) *float64 { return f64(700e9 * (1 + 0.004*float64(i-20))) })
	jolts := monthlySeries(t, "JTSJOL", "count", "2025-01-01", 20, func(int) *float64 { return f64(7_520_000) })
	gdp := monthlySeries(t, "A191RL1Q225SBEA", "percent", "2025-01-01", 6, func(int) *float64 { return f64(2.84) })[:0]
	r := fakeReader{points: map[string][]contracts.SeriesPoint{
		"CPIAUCNS": monthlySeries(t, "CPIAUCNS", "index_points", "2025-01-01", 21, yoyIndex),
		"PCEPI":    monthlySeries(t, "PCEPI", "index_points", "2025-01-01", 21, yoyIndex),
		"PPIFID":   ppi,
		"PAYEMS":   payems,
		"RSAFS":    rsafs,
		"JTSJOL":   jolts,
		"WALCL":    {pt(t, "2026-10-28", "2026-10-28", "ccy_amount", f64(6.8e12)), pt(t, "2026-11-04", "2026-11-04", "ccy_amount", f64(6.74e12))},
		"DFEDTARU": daily2(t, "DFEDTARU", "2026-10-20", 16, func(int) float64 { return 4.25 }),
		"DFEDTARL": daily2(t, "DFEDTARL", "2026-10-20", 16, func(int) float64 { return 4.0 }),
	}}
	// GDP quarters are dated the first of the quarter month: Q1 2025 to Q2 2026 are stored at 2025-01, 04, 07, 10, 2026-01, 04.
	for _, d := range []string{"2025-01-01", "2025-04-01", "2025-07-01", "2025-10-01", "2026-01-01", "2026-04-01"} {
		gdp = append(gdp, pt(t, d, d, "percent", f64(2.84)))
	}
	r.points["A191RL1Q225SBEA"] = gdp
	rows := valuesBuild(t, r,
		evAt("cpi", "2026-11-12"), evAt("pce", "2026-11-13"), evAt("ppi", "2026-11-14"),
		evAt("employment_situation", "2026-11-06"), evAt("retail_sales", "2026-11-17"),
		evAt("jolts", "2026-11-10"), evAt("h41", "2026-11-12"), evAt("gdp", "2026-11-26"), evAt("fomc", "2026-12-09"),
		evAt("fomc_minutes", "2026-11-20"), evAt("boj", "2026-12-19"), evAt("quarter_end", "2026-12-31"), evAt("us_tax", "2026-12-15"))
	want := map[string]string{
		"CPI 2026-11-12": "3.0%", "PCE 2026-11-13": "3.0%", "PPI 2026-11-14": "3.0%",
		"Employment situation 2026-11-06": "+200K", "Retail sales 2026-11-17": "+0.4%",
		"JOLTS 2026-11-10": "7.52M", "H.4.1 2026-11-12": "$6.74T", "GDP 2026-11-26": "2.8%",
		"FOMC decision 2026-12-09": "4.00-4.25%",
	}
	for k, w := range want {
		row := rows[k]
		if row == nil {
			t.Errorf("%s: no row", k)
			continue
		}
		if row["previous"] != w || figText(t, row["previous_figure"]) != w {
			t.Errorf("%s: previous %v figure %v, want %s", k, row["previous"], row["previous_figure"], w)
		}
		if reason, text := cellGap(t, row["actual"]); reason != "no_data" || text != gapNotYetReleased {
			t.Errorf("%s: future actual %s %s", k, reason, text)
		}
	}
	if m := rows["FOMC decision 2026-12-09"]["previous_figure"].(map[string]any); m["unit"] != "percent_range" || m["value"] != 4.25 {
		t.Errorf("fomc figure %v", m)
	}
	if m := rows["Employment situation 2026-11-06"]["previous_figure"].(map[string]any); m["unit"] != "thousands" || m["value"] != float64(200) {
		t.Errorf("nfp figure %v", m)
	}
	for _, k := range []string{"FOMC minutes 2026-11-20", "BoJ decision 2026-12-19", "Quarter-end 2026-12-31", "US corporate tax date 2026-12-15"} {
		row := rows[k]
		if row["previous"] != "-" || row["actual"] != nil || row["previous_figure"] != nil {
			t.Errorf("%s: %v", k, row)
		}
	}
}

func TestEventValuesActualOfPastEvent(t *testing.T) {
	t.Parallel()
	cpi := monthlySeries(t, "CPIAUCNS", "index_points", "2025-01-01", 22, func(i int) *float64 {
		if i == 21 {
			return f64(104)
		}
		return yoyIndex(i)
	})
	payems := monthlySeries(t, "PAYEMS", "count", "2025-01-01", 22, func(i int) *float64 { return f64(158_000_000 + 200_000*float64(i) + 27_000*float64(i/21)) })
	r := fakeReader{points: map[string][]contracts.SeriesPoint{"CPIAUCNS": cpi, "PAYEMS": payems}}
	r.points["DFEDTARU"] = daily2(t, "DFEDTARU", "2026-10-20", 14, func(i int) float64 {
		if i >= 13 { // from 2026-11-02
			return 4.25
		}
		return 4.5
	})
	r.points["DFEDTARL"] = daily2(t, "DFEDTARL", "2026-10-20", 14, func(i int) float64 {
		if i >= 13 {
			return 4.0
		}
		return 4.25
	})
	rows := valuesBuild(t, r, evAt("cpi", "2026-11-02"), evAt("employment_situation", "2026-11-02"), evAt("fomc", "2026-11-01"))
	for k, w := range map[string][2]string{
		"CPI 2026-11-02":                  {"3.0%", "4.0%"},
		"Employment situation 2026-11-02": {"+200K", "+227K"},
		"FOMC decision 2026-11-01":        {"4.25-4.50%", "4.00-4.25%"},
	} {
		row := rows[k]
		if row["previous"] != w[0] || figText(t, row["actual"]) != w[1] {
			t.Errorf("%s: previous %v actual %v, want %v", k, row["previous"], row["actual"], w)
		}
	}
	if m := rows["CPI 2026-11-02"]["actual"].(map[string]any); m["value"] != 4.0 || m["unit"] != "percent" {
		t.Errorf("actual figure %v", m)
	}
}

func TestEventValuesYoYHole(t *testing.T) {
	t.Parallel()
	cpi := monthlySeries(t, "CPIAUCNS", "index_points", "2025-01-01", 22, func(i int) *float64 {
		if i == 9 { // 2025-10-01, the shutdown month, stored with no value
			return nil
		}
		return yoyIndex(i)
	})
	rows := valuesBuild(t, fakeReader{points: map[string][]contracts.SeriesPoint{"CPIAUCNS": cpi}}, evAt("cpi", "2026-11-02"))
	row := rows["CPI 2026-11-02"]
	reason, text := cellGap(t, row["actual"])
	if reason != "no_data" || !strings.Contains(text, "Oct 2025") || !strings.Contains(text, "CPIAUCNS") {
		t.Errorf("actual gap %s %q", reason, text)
	}
	if row["previous"] != "3.0%" {
		t.Errorf("previous %v: a hole in Oct 2025 must not touch Sep", row["previous"])
	}
	// the point is absent altogether: still a gap that names the month
	rows = valuesBuild(t, fakeReader{points: map[string][]contracts.SeriesPoint{"CPIAUCNS": append(cpi[:9:9], cpi[10:]...)}}, evAt("cpi", "2026-11-02"))
	if reason, text := cellGap(t, rows["CPI 2026-11-02"]["actual"]); reason != "no_data" || !strings.Contains(text, "Oct 2025") {
		t.Errorf("absent point: %s %q", reason, text)
	}
}

func TestEventValuesGdpRevision(t *testing.T) {
	t.Parallel()
	var gdp []contracts.SeriesPoint
	for i, d := range []string{"2025-10-01", "2026-01-01", "2026-04-01", "2026-07-01"} {
		gdp = append(gdp, pt(t, d, d, "percent", f64([]float64{1.1, 1.5, 2.6, 2.9}[i])))
	}
	rows := valuesBuild(t, fakeReader{points: map[string][]contracts.SeriesPoint{"A191RL1Q225SBEA": gdp}},
		evAt("gdp", "2026-10-29"), evAt("gdp", "2026-11-04"))
	first, second := rows["GDP 2026-10-29"], rows["GDP 2026-11-04"]
	if first["previous"] != "2.6%" || figText(t, first["actual"]) != "2.9%" {
		t.Errorf("first estimate: %v %v", first["previous"], first["actual"])
	}
	for _, v := range []any{second["previous"], second["previous_figure"]} {
		if reason, text := cellGap(t, v); reason != "no_data" || text != gapEarlierEstimate {
			t.Errorf("second estimate previous %s %q", reason, text)
		}
	}
	if figText(t, second["actual"]) != "2.9%" {
		t.Errorf("second actual %v", second["actual"])
	}
}

func TestEventValuesFomcLag(t *testing.T) {
	t.Parallel()
	r := fakeReader{points: map[string][]contracts.SeriesPoint{
		"DFEDTARU": daily2(t, "DFEDTARU", "2026-10-20", 15, func(int) float64 { return 4.25 }),
		"DFEDTARL": daily2(t, "DFEDTARL", "2026-10-20", 15, func(int) float64 { return 4.0 }),
	}}
	// points run to 2026-11-03; the meeting is 2026-11-04, so D-1 is stored and D+1 is not
	row := valuesBuild(t, r, evAt("fomc", "2026-11-04"))["FOMC decision 2026-11-04"]
	if row["previous"] != "4.00-4.25%" {
		t.Errorf("previous %v", row["previous"])
	}
	if reason, text := cellGap(t, row["actual"]); reason != "no_data" || text != gapNotYetPublished {
		t.Errorf("actual %s %q", reason, text)
	}
}

type errSeries struct{ fakeReader }

func (e errSeries) SeriesPoints(ctx context.Context, id string, from, to contracts.Day) ([]contracts.SeriesPoint, error) {
	if id == "CPIAUCNS" {
		return nil, errors.New("boom")
	}
	return e.fakeReader.SeriesPoints(ctx, id, from, to)
}

func TestEventValuesGaps(t *testing.T) {
	t.Parallel()
	cpi := monthlySeries(t, "CPIAUCNS", "index_points", "2025-01-01", 21, yoyIndex) // to Sep 2026
	rows := valuesBuild(t, fakeReader{points: map[string][]contracts.SeriesPoint{"CPIAUCNS": cpi}},
		evAt("cpi", "2026-12-10"), evAt("pce", "2026-11-13"))
	// the Dec release expects Nov, so its previous is Oct; the newest stored is Sep
	if reason, text := cellGap(t, rows["CPI 2026-12-10"]["previous"]); reason != "stale" || text != "latest stored: 2026-09-01" {
		t.Errorf("stale previous %s %q", reason, text)
	}
	if reason, text := cellGap(t, rows["PCE 2026-11-13"]["previous_figure"]); reason != "no_data" || !strings.Contains(text, "PCEPI") {
		t.Errorf("missing series %s %q", reason, text)
	}
	if reason, text := cellGap(t, rows["PCE 2026-11-13"]["actual"]); reason != "no_data" || !strings.Contains(text, "PCEPI") {
		t.Errorf("missing series actual %s %q", reason, text)
	}
	// a read error is an error of Build, never a gap
	ev := healthy()
	ev.events = append(ev.events, evAt("cpi", "2026-11-12"))
	c := cfg("A")
	c.Events = ev
	c.Liquidity = fixedFunc(fullResult())
	if _, err := Build(t.Context(), errSeries{}, c, eventsDay, fixedAt); err == nil || !strings.Contains(err.Error(), "CPIAUCNS") {
		t.Fatalf("want a read error naming the series, got %v", err)
	}
}

func TestPastEventsNotLead(t *testing.T) {
	t.Parallel()
	build := func(events ...macroevent.Event) (map[string]any, []map[string]any) {
		ev := healthy()
		ev.events = append(ev.events, events...)
		c := leadCfg(ev)
		m := buildWith(t, leadSeries(t), c)
		return m, eventRows(m)
	}
	m, rows := build(evAt("cpi", "2026-11-02"), evAt("gdp", "2026-12-01"))
	var past map[string]any
	for _, r := range rows {
		if r["date"] == "2026-11-02" {
			past = r
		}
	}
	if past == nil || past["days_to"] != float64(-3) || past["in_window"] != false {
		t.Fatalf("past row %v", past)
	}
	for _, r := range leadRows(t, m) {
		if r["key"] == "next_event" && !strings.Contains(r["text"].(string), fmt.Sprintf("GDP, %s", "2026-12-01")) {
			t.Errorf("next_event %q", r["text"])
		}
	}
	m, _ = build(evAt("cpi", "2026-11-02"))
	for _, r := range leadRows(t, m) {
		if r["key"] == "next_event" && r["text"] != noBigEvent {
			t.Errorf("a past event is the next event: %q", r["text"])
		}
	}
}
