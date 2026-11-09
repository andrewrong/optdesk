package structures_page_test

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
)

func TestLegsTotalEqualsTile(t *testing.T) {
	t.Parallel()
	m := readFixture(t, "detail_spread").build(t, spreadID, true)
	legs := list(t, m["legs"])
	if len(legs) != 2 {
		t.Fatalf("legs = %v", legs)
	}
	// the leg P&L of the fixture are 10 and 130; the tile and the total carry the one compute value 140
	var texts []string
	for _, l := range legs {
		texts = append(texts, obj(t, obj(t, l)["pnl"])["text"].(string))
	}
	if texts[0] != "10.0000" || texts[1] != "130.0000" {
		t.Fatalf("leg pnl = %v", texts)
	}
	total := obj(t, obj(t, m["legs_total"])["pnl"])["text"]
	tile := obj(t, obj(t, obj(t, m["tiles"])["pnl"])["figure"])["text"]
	if total != tile || total != "140.0000" || obj(t, m["legs_total"])["label"] != "合计" {
		t.Fatalf("total = %v, tile = %v", total, tile)
	}
}

func TestLegMidAndPnL(t *testing.T) {
	t.Parallel()
	m := readFixture(t, "detail_spread").build(t, spreadID, true)
	short := obj(t, list(t, m["legs"])[1])
	if short["side_text"] != "卖出" || short["right_text"] != "看跌" || short["quantity"] != -1.0 || short["expiry_text"] != "2026-12-18" ||
		short["label"] != "MSFT 2026-12-18 310 看跌" {
		t.Fatalf("leg = %v", short)
	}
	if p := obj(t, short["price"]); p["text"] != "2.5" || p["value"] != 2.5 {
		t.Errorf("price = %v", p)
	}
	if mk := obj(t, short["mark"]); mk["text"] != "1.2" || mk["value"] != 1.2 || mk["unit"] != "usd" {
		t.Errorf("mark = %v", mk)
	}
	if long := obj(t, list(t, m["legs"])[0]); long["side_text"] != "买入" || long["quantity"] != 1.0 {
		t.Errorf("long leg = %v", long)
	}
	// a leg with no mark has gaps, named by the reason of the missing quote or the missing reader
	fx := readFixture(t, "detail_gaps")
	for quotes, want := range map[bool]string{true: "missing_quote", false: "no_reader"} {
		leg := obj(t, list(t, fx.build(t, gapsID, quotes)["legs"])[0])
		if obj(t, leg["mark"])["reason"] != want || obj(t, leg["pnl"])["reason"] != want {
			t.Errorf("quotes=%v: leg = %v", quotes, leg)
		}
	}
	// a JPY structure is a gap row
	jpy := readFixture(t, "detail_spread")
	jpy.Result.Structures[0].Currency = "JPY"
	if g := obj(t, list(t, jpy.build(t, spreadID, true)["legs"])[0]); g["reason"] != "unsupported_currency" {
		t.Errorf("jpy leg = %v", g)
	}
}

func TestFillsFromRows(t *testing.T) {
	t.Parallel()
	fx := readFixture(t, "detail_spread")
	fx.Positions[1].Fills = append(fx.Positions[1].Fills,
		compute.LedgerFill{Day: "2026-11-02", Action: "close", Quantity: "1", Price: "1.9", Commission: ""},
		compute.LedgerFill{Day: "2026-11-03", Action: "close", Quantity: "x", Price: "1.9", Commission: "0.1"})
	m := fx.build(t, spreadID, true)
	fills := list(t, m["fills"])
	if len(fills) != 3 { // two opens and one close; the row with bad quantity is not shown
		t.Fatalf("fills = %v", fills)
	}
	first := obj(t, fills[0])
	if first["time_text"] != "2026-10-19" || first["action_text"] != "开仓" || first["leg_text"] != "MSFT 2026-12-18 300 看跌" ||
		first["quantity"] != 1.0 || obj(t, first["price"])["text"] != "1" || obj(t, first["fee"])["text"] != "0.6500" {
		t.Fatalf("first fill = %v", first)
	}
	closeRow := obj(t, fills[2])
	if closeRow["action_text"] != "平仓" || obj(t, closeRow["fee"])["reason"] != "no_data" {
		t.Fatalf("close fill = %v", closeRow)
	}
	// the dropped row is named in gaps, so the page is partial
	found := false
	for _, g := range list(t, m["gaps"]) {
		found = found || obj(t, g)["text"] == "部分成交记录无法显示"
	}
	if !found || m["run_status"] != "partial" {
		t.Fatalf("gaps = %v, status = %v", m["gaps"], m["run_status"])
	}
}
