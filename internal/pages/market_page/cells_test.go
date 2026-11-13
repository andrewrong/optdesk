package market_page

import (
	"fmt"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
)

func TestN225IsGap(t *testing.T) {
	t.Parallel()
	m := allOK().page(t)
	var names []any
	for i := range 4 {
		names = append(names, at(t, m, "indices", "rows", i, "name"))
	}
	if got := names; got[0] != "SPX" || got[1] != "NDX" || got[2] != "RUT" || got[3] != "N225" || len(got) != 4 {
		t.Errorf("row names = %v", got)
	}
	if g := rank(t, m, 3); g["kind"] != "gap" || g["reason"] != "no_data" {
		t.Errorf("N225 rank_3y = %v", g)
	}
}

func rv20Of(t *testing.T, m map[string]any, row int) map[string]any {
	t.Helper()
	return at(t, m, "indices", "rows", row, "rv20").(map[string]any)
}

func TestRv20CellShowsStoredValue(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"0.162060": "16.2", "0.162500": "16.2", "0.163500": "16.4", "0.000500": "0.0", "0.000000": "0.0",
	} {
		e := allOK()
		e.rv["SPX"] = []contracts.ReadingRecord{rec("2026-11-06", "ok", in, nil, "2026-11-06")}
		c := rv20Of(t, e.page(t), 0)
		if c["kind"] != "value" || c["text"] != want || c["unit"] != "vol_points" || fmt.Sprint(c["value"]) != want {
			t.Errorf("%s: rv20 = %v, want %s", in, c, want)
		}
	}
}

func TestRv20CellNoRowIsNoData(t *testing.T) {
	t.Parallel()
	e := allOK()
	delete(e.rv, "NDX")
	c := rv20Of(t, e.page(t), 1)
	if c["kind"] != "gap" || c["reason"] != "no_data" || c["text"] != "NDX 20-day realised volatility: no data, not stored yet" {
		t.Errorf("NDX rv20 = %v", c)
	}
}

func TestRv20CellNotOkIsMissingInput(t *testing.T) {
	t.Parallel()
	e := allOK()
	e.rv["RUT"] = []contracts.ReadingRecord{rec("2026-11-06", "missing_input", "", intp(2), "")}
	e.rv["SPX"] = []contracts.ReadingRecord{rec("2026-11-06", "ok", "-0.1", nil, "")}
	m := e.page(t)
	if c := rv20Of(t, m, 2); c["kind"] != "gap" || c["reason"] != "missing_input" || c["text"] != "RUT 20-day realised volatility: no data, inputs incomplete" {
		t.Errorf("RUT rv20 = %v", c)
	}
	if c := rv20Of(t, m, 0); c["kind"] != "gap" || c["reason"] != "missing_input" {
		t.Errorf("negative SPX rv20 = %v", c)
	}
}

func TestRv20SmallNegativeIsMissingInput(t *testing.T) {
	t.Parallel()
	e := allOK()
	e.rv["SPX"] = []contracts.ReadingRecord{rec("2026-11-06", "ok", "-0.000400", nil, "2026-11-06")}
	if c := rv20Of(t, e.page(t), 0); c["kind"] != "gap" || c["reason"] != "missing_input" {
		t.Errorf("rv20 = %v, want missing_input gap", c)
	}
}

func TestN225RvStaysNotBuilt(t *testing.T) {
	t.Parallel()
	m := allOK().page(t)
	if g := rv20Of(t, m, 3); g["kind"] != "gap" || g["reason"] != "not_built" || g["text"] != "N225 20-day realised volatility: not built yet" {
		t.Errorf("N225 rv20 = %v", g)
	}
	g := at(t, m, "indices", "rows", 3, "iv_minus_rv20").(map[string]any)
	if g["kind"] != "gap" || g["reason"] != "not_built" || g["text"] != "N225 implied minus realised: not built yet" {
		t.Errorf("N225 iv_minus_rv20 = %v", g)
	}
	for i, want := range []string{"+3.2", "+7.2", "-1.4"} {
		c := at(t, m, "indices", "rows", i, "iv_minus_rv20").(map[string]any)
		if c["kind"] != "value" || c["text"] != want || c["unit"] != "vol_points" {
			t.Errorf("row %d iv_minus_rv20 = %v", i, c)
		}
	}
	if g := at(t, m, "indices", "rows", 3, "iv").(map[string]any); g["reason"] != "not_built" {
		t.Errorf("N225 iv = %v", g)
	}
}

func TestIndicesHeaderNamesTheUnbuiltColumn(t *testing.T) {
	t.Parallel()
	s, _ := at(t, allOK().page(t), "indices", "sentence").(string)
	if !strings.Contains(s, "implied minus realised is built for SPX, NDX and RUT, not the Nikkei") || strings.Contains(s, "realised columns") {
		t.Errorf("indices sentence = %q", s)
	}
	p, _ := allOK().page(t)["sentence"].(string)
	if strings.Contains(p, "realised columns") || strings.Contains(p, "implied minus realised") {
		t.Errorf("page sentence = %q", p)
	}
}

func TestRv20GapsLeaveFailedCountUnchanged(t *testing.T) {
	t.Parallel()
	gapped := allOK()
	gapped.rv = nil
	for name, e := range map[string]env{"filled": allOK(), "gaps": gapped} {
		m := e.page(t)
		if m["run_status"] != "ok" {
			t.Errorf("%s: run_status = %v", name, m["run_status"])
		}
	}
	if c := rv20Of(t, gapped.page(t), 0); c["kind"] != "gap" {
		t.Fatalf("rv20 not a gap: %v", c)
	}
	// A failed percentile still makes the page partial, whatever rv20 shows.
	bad := allOK()
	bad.readings[figVixPctile] = []contracts.ReadingRecord{rec("2026-11-06", "missing_input", "", intp(2), "")}
	if bad.page(t)["run_status"] != "partial" {
		t.Error("failed percentile: not partial")
	}
}

func TestWeatherSignedPercent(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"0.031250": "Gap to the 200-day average: +3.12%", "0.031350": "Gap to the 200-day average: +3.14%",
		"-0.031250": "Gap to the 200-day average: -3.12%", "-0.000040": "Gap to the 200-day average: 0.00%",
	} {
		e := allOK()
		e.readings[figSp500Vs200d] = []contracts.ReadingRecord{rec("2026-11-06", "ok", in, nil, "2026-11-06")}
		m := e.page(t)
		if got := at(t, m, "weather", "sentence"); got != want {
			t.Errorf("%s: sentence = %v, want %s", in, got, want)
		}
		if at(t, m, "weather", "headline") != "S&P 500 against its 200-day average" {
			t.Errorf("headline = %v", at(t, m, "weather", "headline"))
		}
	}
	e := allOK()
	e.readings[figSp500Vs200d] = nil
	if got := at(t, e.page(t), "weather", "sentence"); got != "Weather: no data yet, nothing stored" {
		t.Errorf("no reading: sentence = %v", got)
	}
}

func TestWeatherChartIsGap(t *testing.T) {
	t.Parallel()
	g := at(t, allOK().page(t), "weather", "chart").(map[string]any)
	if g["kind"] != "gap" || g["reason"] != "not_built" {
		t.Errorf("chart = %v", g)
	}
}

func TestChartGapTextAgreesWithWeatherSentence(t *testing.T) {
	t.Parallel()
	m := allOK().page(t)
	chart, _ := at(t, m, "weather", "chart").(map[string]any)["text"].(string)
	sentence, _ := at(t, m, "weather", "sentence").(string)
	if strings.Contains(chart, "no stored 200-day average") {
		t.Errorf("chart text contradicts the weather sentence: %q", chart)
	}
	if !strings.HasPrefix(sentence, "Gap to the 200-day average") {
		t.Errorf("weather sentence = %q", sentence)
	}
}

func TestBreadthIsGap(t *testing.T) {
	t.Parallel()
	g := at(t, allOK().page(t), "breadth").(map[string]any)
	if g["kind"] != "gap" || g["reason"] != "not_built" {
		t.Errorf("breadth = %v", g)
	}
}
