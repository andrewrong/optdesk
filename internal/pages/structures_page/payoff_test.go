package structures_page_test

import (
	"strings"
	"testing"
)

func TestPayoffPointsFromCompute(t *testing.T) {
	t.Parallel()
	m := readFixture(t, "detail_spread").build(t, spreadID, true)
	p := obj(t, m["payoff"])
	pts := list(t, p["points"])
	wantP := [][2]float64{{238, -850}, {300, -850}, {310, 150}, {372, 150}}
	if len(pts) != len(wantP) {
		t.Fatalf("points = %v", pts)
	}
	for i, w := range wantP {
		pt := obj(t, pts[i])
		if pt["price"] != w[0] || pt["pnl"] != w[1] {
			t.Errorf("point %d = %v, want %v", i, pt, w)
		}
	}
	if spot := obj(t, p["spot"]); spot["text"] != "320" || spot["unit"] != "usd" {
		t.Errorf("spot = %v", spot)
	}
	if be := obj(t, p["break_even"]); be["text"] != "308.5000" {
		t.Errorf("break_even = %v", be)
	}
	if b := obj(t, p["band"]); b["low"] != 238.0 || b["high"] != 372.0 || !strings.Contains(b["text"].(string), "不是一个标准差区间") {
		t.Errorf("band = %v", b)
	}
	// a block with a gap number is that gap: the naked call has no max loss
	g := readFixture(t, "detail_gaps").build(t, gapsID, true)
	if r := obj(t, g["payoff"])["reason"]; r != "no_rule" {
		t.Errorf("gap payoff = %v", g["payoff"])
	}
	// without a quote of the underlying there is no spot
	fx := readFixture(t, "detail_spread")
	if r := obj(t, fx.build(t, spreadID, false)["payoff"])["reason"]; r != "no_reader" {
		t.Errorf("no reader payoff = %v", r)
	}
}

func TestBreakEvenCaption(t *testing.T) {
	t.Parallel()
	for kind, want := range map[string]string{
		"short_put": "到期价低于此价开始亏损", "put_spread": "到期价低于此价开始亏损", "covered_call": "到期价低于此价开始亏损",
		"naked_call": "到期价高于此价开始亏损", "call_spread": "到期价高于此价开始亏损",
		"long_put": "到期价低于此价开始盈利", "long_call": "到期价高于此价开始盈利",
	} {
		fx := readFixture(t, "detail_spread")
		fx.Result.Structures[0].Kind = kind
		tl := obj(t, fx.build(t, spreadID, true)["tiles"])
		if c := obj(t, tl["break_even"])["caption"]; c != want {
			t.Errorf("%s: caption = %v, want %s", kind, c, want)
		}
	}
	fx := readFixture(t, "detail_spread")
	fx.Result.Structures[0].Figures.BreakEven.Value, fx.Result.Structures[0].Figures.BreakEven.Gap = "", "no_rule"
	tl := obj(t, fx.build(t, spreadID, true)["tiles"])
	if c := obj(t, tl["break_even"])["caption"]; c != "无统一的盈亏平衡价" {
		t.Errorf("gap caption = %v", c)
	}
}

func TestGreeksModelText(t *testing.T) {
	t.Parallel()
	m := readFixture(t, "detail_spread").build(t, spreadID, true)
	g := obj(t, m["greeks"])
	if mt := g["model_text"].(string); !strings.Contains(mt, "IBKR 模型值") || !strings.Contains(mt, "2026-11-06") {
		t.Fatalf("model_text = %q", mt)
	}
	rows := list(t, g["rows"])
	if len(rows) != 3 { // one row per leg and the total
		t.Fatalf("rows = %v", rows)
	}
	total := obj(t, rows[2])
	if total["label"] != "合计，含张数与乘数" {
		t.Errorf("total label = %v", total["label"])
	}
	want := []string{"12.5", "-0.4", "3.1", "-8.2"}
	for i, c := range list(t, total["cells"]) {
		if obj(t, c)["text"] != want[i] || obj(t, c)["unit"] != "greek" {
			t.Errorf("total cell %d = %v", i, c)
		}
	}
	// the leg rows copy the stored per-share values of the contract matched by its terms
	leg := obj(t, rows[1]) // the short 310 put: instrument 2
	if cells := list(t, leg["cells"]); obj(t, cells[0])["text"] != "-0.25" || obj(t, cells[3])["text"] != "0.06" {
		t.Errorf("leg row = %v", leg)
	}
}

func TestGreekGapNamed(t *testing.T) {
	t.Parallel()
	fx := readFixture(t, "detail_gaps")
	g := obj(t, fx.build(t, gapsID, true)["greeks"])
	if !strings.Contains(g["model_text"].(string), "IBKR 模型值") {
		t.Errorf("model_text = %v", g["model_text"])
	}
	for _, r := range list(t, g["rows"]) {
		for _, c := range list(t, obj(t, r)["cells"]) {
			if obj(t, c)["reason"] != "missing_quote" {
				t.Errorf("cell = %v", c)
			}
		}
	}
	// with no quote reader the whole block is the gap no_reader
	if r := obj(t, fx.build(t, gapsID, false)["greeks"])["reason"]; r != "no_reader" {
		t.Errorf("no reader greeks = %v", r)
	}
}
