package market_state_page

import (
	"encoding/json"
	"strings"
	"testing"

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
	if obj(obj(rows[1])["gap_63"])["text"] != "-3.00 points" || obj(rows[0])["group"] != "sector" || obj(rows[3])["group"] != "industry" {
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
