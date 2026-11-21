package market_state_page

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/liquidityapi"
	"github.com/andrewrong/optdesk/internal/compute/marketstateapi"
)

func TestSectorRowsAreNotReordered(t *testing.T) {
	t.Parallel()
	m := build(t, fullConfig(t, sampleResult()))
	var got []string
	for _, r := range sectorRows(m) {
		got = append(got, obj(r)["symbol"].(string))
	}
	if strings.Join(got, ",") != "XLK,XLE,XLF,SMH" {
		t.Errorf("order = %v", got)
	}
	rows := sectorRows(m)
	if obj(obj(rows[1])["gap_63"])["text"] != "−3.00 pp" || obj(rows[0])["group"] != "sector" || obj(rows[3])["group"] != "industry" {
		t.Errorf("rows = %v", rows)
	}
	if r := obj(obj(rows[0])["rank_63"]); r["at"].(json.Number) != "1" || r["of"].(json.Number) != "3" {
		t.Errorf("rank = %v", r)
	}
	if g := obj(rows[3])["rank_21"]; !isGap(g) {
		t.Errorf("SMH rank = %v", g)
	}
	if obj(m["sectors"])["end_day"] != "2026-11-13" {
		t.Errorf("end_day = %v", obj(m["sectors"])["end_day"])
	}
}

func TestSMHRowIsAGapWithoutBars(t *testing.T) {
	t.Parallel()
	res := sampleResult()
	res.Industry = nil // the Func result holds no SMH
	m := build(t, fullConfig(t, res))
	rows := sectorRows(m)
	last := obj(rows[len(rows)-1])
	if last["symbol"] != "SMH" || !isGap(last["gap_63"]) || obj(last["gap_63"])["reason"] != "no_data" ||
		obj(last["gap_63"])["text"] == "" {
		t.Errorf("last row = %v", last)
	}
	// even when SMH is not in the listed symbols, and with real compute on bars that hold no SMH
	c := realConfig(t, nil)
	c.Symbols = c.Symbols[:3]
	c.StockIDs = fakeIDs{ids: map[string]int64{"SPY": 1, "XLK": 2, "XLE": 3, "XLF": 4}}
	rows = sectorRows(build(t, c))
	last = obj(rows[len(rows)-1])
	if last["symbol"] != "SMH" || obj(last["gap_21"])["reason"] != "no_data" {
		t.Errorf("last row = %v", last)
	}
}

func TestEmptyEndDayMakesEveryFundAGap(t *testing.T) {
	t.Parallel()
	res := sampleResult()
	res.EndDay = ""
	m := build(t, fullConfig(t, res))
	sec := obj(m["sectors"])
	if g := sec["end_day"]; !isGap(g) || obj(g)["reason"] != "no_data" {
		t.Errorf("end_day = %v", g)
	}
	for _, r := range sectorRows(m) {
		for _, k := range []string{"gap_21", "gap_63", "rank_21", "rank_63", "rotation"} {
			if !isGap(obj(r)[k]) {
				t.Errorf("%v %s = %v", obj(r)["symbol"], k, obj(r)[k])
			}
		}
	}
	if m["day"] != testDay {
		t.Errorf("day = %v", m["day"])
	}
}

func TestRotationNoteSaysClick(t *testing.T) {
	t.Parallel()
	note, _ := obj(obj(build(t, realConfig(t, nil))["sectors"])["rotation_axes"])["note"].(string)
	for _, want := range []string{
		"click a fund's dot to draw its tail",
		"tail points are 5, 10, 15 and 20 sessions before end_day",
	} {
		if !strings.Contains(note, want) {
			t.Errorf("note %q lacks %q", note, want)
		}
	}
}

func TestAxesQuadrantsFixedText(t *testing.T) {
	t.Parallel()
	quadOf := func(m map[string]any) string {
		b, _ := json.Marshal(obj(obj(m["sectors"])["rotation_axes"])["quadrants"])
		return string(b)
	}
	a := quadOf(build(t, fullConfig(t, sampleResult())))
	b := quadOf(build(t, fullConfig(t, marketstateapi.Result{})))
	c := quadOf(build(t, realConfig(t, nil)))
	if a != b || a != c {
		t.Errorf("quadrants differ:\n%s\n%s\n%s", a, b, c)
	}
	for _, q := range quadrants() {
		if !strings.Contains(a, q.Text) {
			t.Errorf("quadrant %s missing", q.ID)
		}
	}
	if len(quadrants()) != 4 {
		t.Errorf("%d quadrants", len(quadrants()))
	}
	// no row text holds a quadrant label
	m := build(t, realConfig(t, nil))
	rb, _ := json.Marshal(sectorRows(m))
	for _, q := range quadrants() {
		label := strings.ToLower(strings.SplitN(q.Text, ":", 2)[0])
		if strings.Contains(strings.ToLower(string(rb)), label) {
			t.Errorf("rows hold %q", label)
		}
	}
}

// numbersIn collects the json.Number values of v by path.
func numbersIn(v any, path string, out map[string]json.Number) {
	switch x := v.(type) {
	case json.Number:
		out[path] = x
	case map[string]any:
		for k, e := range x {
			numbersIn(e, path+"/"+k, out)
		}
	case []any:
		for i, e := range x {
			numbersIn(e, path+"/"+string(rune('a'+i)), out)
		}
	}
}

func TestBarsUseTheRowGaps(t *testing.T) {
	t.Parallel()
	m := build(t, realConfig(t, nil))
	sec := obj(m["sectors"])
	out := map[string]json.Number{}
	for _, k := range []string{"caption", "end_day", "bars_caption", "rotation_axes"} {
		numbersIn(sec[k], k, out)
	}
	if len(out) != 0 {
		t.Errorf("numbers outside the rows: %v", out)
	}
	for _, r := range sectorRows(m) {
		row := obj(r)
		nums := map[string]json.Number{}
		numbersIn(row, "", nums)
		for path := range nums {
			ok := path == "/gap_21/value" || path == "/gap_63/value" ||
				strings.HasPrefix(path, "/rank_") || strings.HasPrefix(path, "/rotation/")
			if !ok {
				t.Errorf("%v: number at %s", row["symbol"], path)
			}
		}
	}
}

func TestSectorsCaptionsWriteGapsInPP(t *testing.T) {
	t.Parallel()
	m := build(t, fullConfig(t, sampleResult()))
	s := obj(m["sectors"])
	axes := obj(s["rotation_axes"])
	if axes["x_caption"] != "63-session gap to SPY, pp" {
		t.Errorf("x_caption = %v", axes["x_caption"])
	}
	if axes["y_caption"] != "21-session gap minus a third of the 63-session gap, pp" {
		t.Errorf("y_caption = %v", axes["y_caption"])
	}
	if c, _ := s["bars_caption"].(string); !strings.Contains(c, "in pp;") || strings.Contains(c, "points") {
		t.Errorf("bars_caption = %v", s["bars_caption"])
	}
}

func TestGapTextUsesTypographicMinus(t *testing.T) {
	t.Parallel()
	cases := []struct{ in, value, text string }{
		{"-3.611", "-3.61", "−3.61 pp"},
		{"2.5", "2.50", "+2.50 pp"},
		{"0", "0.00", "0.00 pp"},
	}
	for _, c := range cases {
		f, ok := gapFigure(c.in, "", "gap").(figure)
		if !ok {
			t.Fatalf("gapFigure(%q) is no figure", c.in)
		}
		if f.Value != json.Number(c.value) || f.Text != c.text {
			t.Errorf("gapFigure(%q) = value %q text %q, want %q %q", c.in, f.Value, f.Text, c.value, c.text)
		}
	}
	for in, want := range map[string]string{"0": "0", "0.0": "0.0", "0.00": "0.00", "-1.5": "−1.5", "1.5": "+1.5"} {
		if got := signed(in); got != want {
			t.Errorf("signed(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSectorGapCellNamesFundAndDay(t *testing.T) {
	t.Parallel()
	res := sampleResult()
	miss := liquidityapi.Num{Day: "2026-11-19", Reason: liquidityapi.MissingInput}
	res.Funds = append(res.Funds, marketstateapi.Fund{Symbol: "XLB", Gap21: miss, Gap63: miss,
		Rank21: marketstateapi.Position{Reason: liquidityapi.MissingInput},
		Rank63: marketstateapi.Position{Reason: liquidityapi.MissingInput}})
	m := build(t, fullConfig(t, res))
	var found bool
	for _, r := range sectorRows(m) {
		row := obj(r)
		switch row["symbol"] {
		case "XLB":
			found = true
			for _, k := range []string{"gap_21", "gap_63", "rank_21", "rank_63"} {
				g := obj(row[k])
				if g["kind"] != "gap" || g["reason"] != "missing_input" || g["text"] != "XLB has no close on 2026-11-19" {
					t.Errorf("XLB %s = %v", k, g)
				}
			}
		case "XLK":
			if isGap(row["gap_21"]) || isGap(row["rank_21"]) {
				t.Errorf("XLK cells are gaps: %v", row)
			}
		}
	}
	if !found {
		t.Fatal("no XLB row")
	}
}
