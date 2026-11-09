package market_page

import (
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

func TestIvRvCellsAreNotBuilt(t *testing.T) {
	t.Parallel()
	m := allOK().page(t)
	for i := range 4 {
		for _, k := range []string{"iv", "rv20", "iv_minus_rv20"} {
			g := at(t, m, "indices", "rows", i, k).(map[string]any)
			if g["kind"] != "gap" || g["reason"] != "not_built" {
				t.Errorf("row %d %s = %v", i, k, g)
			}
		}
	}
}

func TestWeatherSignedPercent(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"0.031250": "与 200 日线相差 +3.12%", "0.031350": "与 200 日线相差 +3.14%",
		"-0.031250": "与 200 日线相差 -3.12%", "-0.000040": "与 200 日线相差 0.00%",
	} {
		e := allOK()
		e.readings[figSp500Vs200d] = []contracts.ReadingRecord{rec("2026-11-06", "ok", in, nil, "2026-11-06")}
		m := e.page(t)
		if got := at(t, m, "weather", "sentence"); got != want {
			t.Errorf("%s: sentence = %v, want %s", in, got, want)
		}
		if at(t, m, "weather", "headline") != "标普 500 相对 200 日线的位置" {
			t.Errorf("headline = %v", at(t, m, "weather", "headline"))
		}
	}
	e := allOK()
	e.readings[figSp500Vs200d] = nil
	if got := at(t, e.page(t), "weather", "sentence"); got != "天气暂无数据：还没有存储" {
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

func TestBreadthIsGap(t *testing.T) {
	t.Parallel()
	g := at(t, allOK().page(t), "breadth").(map[string]any)
	if g["kind"] != "gap" || g["reason"] != "not_built" {
		t.Errorf("breadth = %v", g)
	}
}
