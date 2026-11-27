package macro_page

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/macroevent"
)

// buildWith builds the page of 2026-11-05 with Liquidity set (so the v2 schema applies) and checks it against it.
func buildWith(t *testing.T, r fakeReader, c Config) map[string]any {
	t.Helper()
	c.Liquidity = fixedFunc(fullResult())
	b, err := Build(t.Context(), r, c, "2026-11-05", fixedAt)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateV2(t, b); err != nil {
		t.Fatalf("schema: %v\n%s", err, b)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func windowOf45(stored bool) WindowFunc {
	return func(context.Context, time.Time) (int, bool) { return 45, stored }
}

func leadRows(t *testing.T, m map[string]any) []map[string]any {
	t.Helper()
	l, ok := m["lead"].(map[string]any)
	if !ok {
		t.Fatalf("no lead: %v", m)
	}
	var out []map[string]any
	for _, r := range l["rows"].([]any) {
		out = append(out, r.(map[string]any))
	}
	return out
}

// daily is n daily points ending at last, rising by step.
func daily(t *testing.T, last string, n int, start, step float64) []contracts.SeriesPoint {
	t.Helper()
	end, err := time.Parse(dayLayout, last)
	if err != nil {
		t.Fatal(err)
	}
	var out []contracts.SeriesPoint
	for i := range n {
		d := end.AddDate(0, 0, i-n+1).Format(dayLayout)
		out = append(out, pt(t, d, d, "percent", new(start+step*float64(i))))
	}
	return out
}

func leadSeries(t *testing.T) fakeReader {
	return fakeReader{points: map[string][]contracts.SeriesPoint{
		"DGS3MO":       daily(t, "2026-11-04", 10, 4.0, 0.1),
		"BAMLH0A0HYM2": daily(t, "2026-11-04", 10, 3.0, -0.02),
		"VIXCLS":       {pt(t, "2026-11-04", "2026-11-04", "index", new(17.2))},
		"VXNCLS":       {pt(t, "2026-11-04", "2026-11-04", "index", f64(21))},
		"RVXCLS":       {pt(t, "2026-11-04", "2026-11-04", "index", new(20.1))},
	}}
}

func leadCfg(ev *fakeEvents) Config {
	c := cfg("DGS10")
	c.Window = windowOf45(true)
	if ev != nil {
		c.Events = ev
	}
	return c
}

func TestLeadRowsKeepOrder(t *testing.T) {
	t.Parallel()
	rows := leadRows(t, buildWith(t, leadSeries(t), leadCfg(healthy())))
	var keys []string
	for _, r := range rows {
		keys = append(keys, r["key"].(string))
	}
	if got := strings.Join(keys, ","); got != "tbill,hy_spread,next_event,index_iv" {
		t.Fatalf("order %s", got)
	}
	tb := rows[0]
	if tb["text"] != "4.9%" || tb["data_day"] != "2026-11-04" || tb["state"] != "ok" {
		t.Fatalf("tbill %v", tb)
	}
	if tb["change_text"] != "+0.50 pp vs five sessions" {
		t.Fatalf("tbill change %v", tb["change_text"])
	}
	if rows[1]["change_text"] != "-0.10 pp vs five sessions" {
		t.Fatalf("hy change %v", rows[1]["change_text"])
	}
	if h, _ := tb["hint"].(string); h == "" {
		t.Fatal("no hint")
	}
}

func eventOn(kind, date string) macroevent.Event {
	return macroevent.Event{Kind: kind, Source: "fred", Date: contracts.Day(date), RunDay: "2026-11-04"}
}

func TestLeadNextEventWindow(t *testing.T) {
	t.Parallel()
	cases := []struct{ date, word string }{{"2026-12-20", "inside"}, {"2026-12-21", "after"}}
	for _, tc := range cases {
		t.Run(tc.word, func(t *testing.T) {
			t.Parallel()
			ev := healthy()
			ev.events = append(ev.events, eventOn("cpi", tc.date))
			row := leadRows(t, buildWith(t, leadSeries(t), leadCfg(ev)))[2]
			text := row["text"].(string)
			want := "CPI, " + tc.date + ", in "
			if !strings.HasPrefix(text, want) || !strings.Contains(text, ", "+tc.word+" the 45-day option window (your max_dte setting)") {
				t.Fatalf("text %q", text)
			}
			if row["state"] != "ok" {
				t.Fatalf("state %v", row["state"])
			}
		})
	}
}

func TestLeadNextEventNearestBigKind(t *testing.T) {
	t.Parallel()
	ev := healthy()
	ev.events = append(ev.events, eventOn("ppi", "2026-11-06"), eventOn("gdp", "2026-11-30"), eventOn("cpi", "2026-12-01"))
	text := leadRows(t, buildWith(t, leadSeries(t), leadCfg(ev)))[2]["text"].(string)
	if !strings.HasPrefix(text, "GDP, 2026-11-30, in 25 days") {
		t.Fatalf("text %q", text)
	}
}

func TestLeadGapIsNotZero(t *testing.T) {
	t.Parallel()
	rows := leadRows(t, buildWith(t, fakeReader{}, leadCfg(healthy())))
	for _, r := range rows {
		if r["state"] != "gap" {
			t.Errorf("%v state %v", r["key"], r["state"])
		}
		text := r["text"].(string)
		if r["key"] == "next_event" {
			if text != "no big event stored in the next 60 days" {
				t.Errorf("next_event %q", text)
			}
			continue
		}
		if !strings.Contains(text, "no_data") || strings.Contains(text, "0%") || strings.Contains(text, "0.0") {
			t.Errorf("%v text %q", r["key"], text)
		}
		if r["change_text"] != nil || r["data_day"] != nil {
			t.Errorf("%v carries a change or day: %v", r["key"], r)
		}
	}
}

func TestLeadIndexIvThreeSeries(t *testing.T) {
	t.Parallel()
	r := leadSeries(t)
	row := leadRows(t, buildWith(t, r, leadCfg(nil)))[3]
	if row["text"] != "VIX 17.20 · VXN 21.00 · RVX 20.10" || row["data_day"] != "2026-11-04" || row["state"] != "ok" {
		t.Fatalf("row %v", row)
	}
	r.points["VXNCLS"] = []contracts.SeriesPoint{pt(t, "2026-11-03", "2026-11-03", "index", f64(21))}
	row = leadRows(t, buildWith(t, r, leadCfg(nil)))[3]
	if row["text"] != "VIX 17.20 (2026-11-04) · VXN 21.00 (2026-11-03) · RVX 20.10 (2026-11-04)" {
		t.Fatalf("days differ: %v", row["text"])
	}
	delete(r.points, "RVXCLS")
	row = leadRows(t, buildWith(t, r, leadCfg(nil)))[3]
	if row["state"] != "gap" || !strings.Contains(row["text"].(string), "RVX no_data") {
		t.Fatalf("one missing: %v", row)
	}
}

type failingReader struct{ fakeReader }

func (f failingReader) SeriesPoints(ctx context.Context, id string, a, b contracts.Day) ([]contracts.SeriesPoint, error) {
	if id == "VXNCLS" {
		return nil, errors.New("boom")
	}
	return f.fakeReader.SeriesPoints(ctx, id, a, b)
}

func TestLeadSeriesReadErrorFailsBuild(t *testing.T) {
	t.Parallel()
	_, err := Build(t.Context(), failingReader{leadSeries(t)}, leadCfg(nil), "2026-11-05", fixedAt)
	if err == nil || !strings.Contains(err.Error(), "VXNCLS") {
		t.Fatalf("err %v", err)
	}
}
