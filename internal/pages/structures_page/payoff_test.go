package structures_page_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/atombender/go-jsonschema/pkg/types"
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
	if b := obj(t, p["band"]); b["low"] != 238.0 || b["high"] != 372.0 || !strings.Contains(b["text"].(string), "not a one standard deviation band") {
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
		"short_put": "到期价低于此价时亏损", "put_spread": "到期价低于此价时亏损", "covered_call": "到期价低于此价时亏损",
		"naked_call": "到期价高于此价时亏损", "call_spread": "到期价高于此价时亏损",
		"long_put": "到期价低于此价时盈利", "long_call": "到期价高于此价时盈利",
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
	if c := obj(t, tl["break_even"])["caption"]; c != "没有单一盈亏平衡价" {
		t.Errorf("gap caption = %v", c)
	}
}

func TestGreeksModelText(t *testing.T) {
	t.Parallel()
	m := readFixture(t, "detail_spread").build(t, spreadID, true)
	g := obj(t, m["greeks"])
	if mt := g["model_text"].(string); !strings.Contains(mt, "IBKR model value") || !strings.Contains(mt, "2026-11-06") {
		t.Fatalf("model_text = %q", mt)
	}
	rows := list(t, g["rows"])
	if len(rows) != 3 { // one row per leg and the total
		t.Fatalf("rows = %v", rows)
	}
	total := obj(t, rows[2])
	if total["label"] != "Total, with contracts and multiplier" {
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
	if !strings.Contains(g["model_text"].(string), "IBKR model value") {
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

// twoDayEod holds one contract on 2026-11-05 under a stale price and one on 2026-11-06 under the newer one.
type twoDayEod struct{ stubEod }

func (twoDayEod) OptionEod(context.Context, int64, contracts.Day, contracts.Day) ([]contracts.OptionEod, error) {
	at := func(d int) types.SerializableDate {
		return types.SerializableDate{Time: time.Date(2026, 11, d, 0, 0, 0, 0, time.UTC)}
	}
	old, newer := 322.5, 318.25
	return []contracts.OptionEod{
		{InstrumentID: 1, SessionDay: at(5), UnderlyingPrice: &old},
		{InstrumentID: 2, SessionDay: at(6), UnderlyingPrice: &newer},
	}, nil
}

func TestPayoffSpotIsPickedClose(t *testing.T) {
	t.Parallel()
	fx := readFixture(t, "detail_spread")
	c := fx.config(true)
	c.Eod = twoDayEod{}
	p := obj(t, buildWith(t, c, spreadID)["payoff"])
	if spot := obj(t, p["spot"]); spot["text"] != "318.25" {
		t.Errorf("spot = %v, want the picked print 318.25", spot)
	}
}
