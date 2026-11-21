package macro_page

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"

	"github.com/andrewrong/optdesk/internal/contracts"
)

func groupCfg() Config {
	c := cfg()
	c.Groups = []Group{
		{Key: "rates", Indicators: []Indicator{{SeriesID: "DGS10", Name: "10y", SourceText: "FRED"}, {SeriesID: "DFF", Name: "ff", SourceText: "FRED"}}},
		{Key: "credit", Indicators: []Indicator{{SeriesID: "NFCI", Name: "nfci", SourceText: "FRED"}}},
		{Key: "dollar_japan", Indicators: []Indicator{
			{SeriesID: "DTWEXBGS", Name: "usd", SourceText: "FRED"}, {SeriesID: "DEXJPUS", Name: "yen", SourceText: "FRED"},
			{SeriesID: "IRLTLT01JPM156N", Name: "jgb", SourceText: "FRED"}}},
	}
	return c
}

func monthly(t *testing.T, days ...string) []contracts.SeriesPoint {
	var out []contracts.SeriesPoint
	for i, d := range days {
		out = append(out, pt(t, d, d, "percent", f64(1+0.25*float64(i))))
	}
	return out
}

func groupSeries(t *testing.T) fakeReader {
	return fakeReader{points: map[string][]contracts.SeriesPoint{
		"DGS10":           daily(t, "2026-11-04", 10, 4.0, 0.1),
		"DFF":             daily(t, "2026-11-04", 10, 4.0, 0),
		"NFCI":            daily(t, "2026-11-04", 3, -0.5, 0.01),
		"DTWEXBGS":        daily(t, "2026-11-04", 10, 120, 0.5),
		"DEXJPUS":         daily(t, "2026-11-04", 10, 150, -1),
		"IRLTLT01JPM156N": monthly(t, "2026-09-01", "2026-10-01"),
	}}
}

func TestGroupsKeepOrder(t *testing.T) {
	t.Parallel()
	m := buildWith(t, groupSeries(t), groupCfg())
	var names, groups []string
	for _, r := range m["indicators"].([]any) {
		row := r.(map[string]any)
		names = append(names, row["name"].(string))
		groups = append(groups, row["group"].(string))
	}
	if got := strings.Join(names, ","); got != "10y,ff,nfci,usd,yen,jgb" {
		t.Fatalf("names %s", got)
	}
	if got := strings.Join(groups, ","); got != "rates,rates,credit,dollar_japan,dollar_japan,dollar_japan" {
		t.Fatalf("groups %s", got)
	}
	row := indicator(m, 3)
	latest := row["latest"].(map[string]any)
	if latest["unit"] != "index_points" || latest["text"] != "124.50" {
		t.Fatalf("level figure %v", latest)
	}
	if latest = indicator(m, 4)["latest"].(map[string]any); latest["unit"] != "jpy_per_usd" {
		t.Fatalf("yen %v", latest)
	}
	if latest = indicator(m, 0)["latest"].(map[string]any); latest["unit"] != "percent" || latest["text"] != "4.9%" {
		t.Fatalf("percent %v", latest)
	}
	if indicator(m, 0)["data_day"] != "2026-11-04" || indicator(m, 0)["hint"] == "" {
		t.Fatalf("row %v", indicator(m, 0))
	}
	if indicator(m, 0)["status"] != "good" || indicator(m, 0)["source_text"] != "FRED" || indicator(m, 0)["detail_href"] != nil {
		t.Fatalf("old fields %v", indicator(m, 0))
	}
}

func TestChangeFiveSessions(t *testing.T) {
	t.Parallel()
	m := buildWith(t, groupSeries(t), groupCfg())
	if got := indicator(m, 0)["change_text"]; got != "+0.50 pp vs five sessions" {
		t.Fatalf("rate change %v", got)
	}
	if got := indicator(m, 1)["change_text"]; got != "0.00 pp vs five sessions" {
		t.Fatalf("flat change %v", got)
	}
	if got := indicator(m, 3)["change_text"]; got != "+2.50 index points vs five sessions" {
		t.Fatalf("level change %v", got)
	}
	if got := indicator(m, 4)["change_text"]; got != "-5.00 JPY per USD vs five sessions" {
		t.Fatalf("yen change %v", got)
	}
	if got := indicator(m, 5)["change_text"]; got != "+0.25 pp vs previous month" {
		t.Fatalf("monthly change %v", got)
	}
}

func TestMonthlyLateAfter45Days(t *testing.T) {
	t.Parallel()
	cases := []struct{ day, status string }{{"2026-09-21", "good"}, {"2026-09-20", "late"}}
	for _, tc := range cases {
		t.Run(tc.day, func(t *testing.T) {
			t.Parallel()
			r := groupSeries(t)
			r.points["IRLTLT01JPM156N"] = monthly(t, "2026-08-01", tc.day)
			// 2026-11-05 minus 2026-09-21 is 45 days, minus 2026-09-20 is 46.
			m := buildWith(t, r, groupCfg())
			if got := indicator(m, 5)["status"]; got != tc.status {
				t.Fatalf("status %v", got)
			}
			if indicator(m, 5)["latest"].(map[string]any)["value"] == nil {
				t.Fatal("a late row keeps its figure")
			}
		})
	}
	r := groupSeries(t)
	r.points["DGS10"] = daily(t, "2026-11-01", 10, 4, 0.1)
	if got := indicator(buildWith(t, r, groupCfg()), 0)["status"]; got != "good" {
		t.Fatalf("daily at 4 days %v", got)
	}
	r.points["DGS10"] = daily(t, "2026-10-31", 10, 4, 0.1)
	if got := indicator(buildWith(t, r, groupCfg()), 0)["status"]; got != "late" {
		t.Fatalf("daily at 5 days %v", got)
	}
}

func TestMissingEarlierPointNoChange(t *testing.T) {
	t.Parallel()
	r := groupSeries(t)
	r.points["DGS10"] = daily(t, "2026-11-04", 5, 4, 0.1)
	r.points["IRLTLT01JPM156N"] = monthly(t, "2026-10-01")
	m := buildWith(t, r, groupCfg())
	for _, i := range []int{0, 5} {
		row := indicator(m, i)
		if _, ok := row["change_text"]; ok {
			t.Errorf("row %d has a change: %v", i, row["change_text"])
		}
		if row["status"] != "good" || row["data_day"] == nil {
			t.Errorf("row %d: %v", i, row)
		}
	}
	delete(r.points, "NFCI")
	row := indicator(buildWith(t, r, groupCfg()), 2)
	if row["status"] != "missing" || row["latest"].(map[string]any)["reason"] != "no_data" {
		t.Fatalf("missing row %v", row)
	}
}

func TestEmptyConfigKeepsPayload(t *testing.T) {
	t.Parallel()
	r := fakeReader{points: map[string][]contracts.SeriesPoint{
		"DGS10": {pt(t, "2026-11-03", "2026-11-04", "percent", f64(4.25)), pt(t, "2026-11-04", "2026-11-05", "percent", f64(4.3))},
		"DGS2":  {pt(t, "2026-11-04", "2026-11-05", "percent", nil)},
	}}
	c := cfg("DGS10", "DGS2")
	c.Indicators[0].DetailHref = "/indicator/DGS10"
	c.Groups = []Group{}
	b, err := Build(t.Context(), r, c, "2026-11-05", fixedAt)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != golden {
		t.Fatalf("got\n%s\nwant\n%s", b, golden)
	}
}

func TestHintsOnlyInZhTexts(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("files %v %v", files, err)
	}
	cjk := func(r rune) bool {
		return unicode.Is(unicode.Han, r) || (r >= 0x3000 && r <= 0x303F) || (r >= 0xFF00 && r <= 0xFFEF)
	}
	for _, f := range files {
		if f == "zh_texts.go" {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if i := strings.IndexFunc(string(b), cjk); i >= 0 {
			t.Errorf("%s has CJK text at byte %d", f, i)
		}
	}
}

func TestGroupsOnlyPageKeepsDataDay(t *testing.T) {
	t.Parallel()
	r := fakeReader{points: map[string][]contracts.SeriesPoint{
		"DGS10": {pt(t, "2026-11-04", "2026-11-05", "percent", f64(4.2))},
	}}
	c := cfg("DGS10")
	c.Groups = []Group{{Key: "rates", Indicators: c.Indicators}}
	c.Indicators = nil
	b, err := Build(t.Context(), r, c, "2026-11-05", fixedAt)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), `"No data day"`) || strings.Contains(string(b), `"data_times":[]`) {
		t.Fatalf("page with a 2026-11-04 row says no data day: %s", b)
	}
}

func TestWeeklyLateAfter10Days(t *testing.T) {
	t.Parallel()
	// The page day is 2026-11-05.
	cases := []struct{ day, status, label string }{
		{"2026-10-31", "good", "On time"}, // 5 days
		{"2026-10-26", "good", "On time"}, // 10 days
		{"2026-10-25", "late", "Late"},    // 11 days
	}
	for _, tc := range cases {
		t.Run(tc.day, func(t *testing.T) {
			t.Parallel()
			r := groupSeries(t)
			r.points["NFCI"] = monthly(t, tc.day)
			row := indicator(buildWith(t, r, groupCfg()), 2)
			if row["status"] != tc.status || row["label"] != tc.label {
				t.Fatalf("row %v", row)
			}
			if row["data_day"] != tc.day {
				t.Fatalf("data_day %v", row["data_day"])
			}
		})
	}
}
