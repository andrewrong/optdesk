package structures_page_test

import "testing"

func TestTimelineDays(t *testing.T) {
	t.Parallel()
	m := readFixture(t, "detail_spread").build(t, spreadID, true)
	st := list(t, m["timeline"])
	if len(st) != 3 {
		t.Fatalf("timeline = %v", st)
	}
	// opened 2026-10-19, today 2026-11-09 (21 days), expiry 2026-12-18 (60 days in all)
	type want struct{ name, date, text, days string }
	for i, w := range []want{
		{"开仓", "2026-10-19", "0.00%", "第 0 天"},
		{"今天", "2026-11-09", "35.00%", "已持有 21 天，距到期 39 天"},
		{"到期", "2026-12-18", "100.00%", "共 60 天"},
	} {
		s := obj(t, st[i])
		el := obj(t, s["elapsed"])
		if s["name"] != w.name || s["date_text"] != w.date || el["text"] != w.text || s["days_text"] != w.days || el["unit"] != "share_percent" {
			t.Errorf("stage %d = %v, want %+v", i, s, w)
		}
	}
	if v := obj(t, obj(t, st[1])["elapsed"])["value"]; v != 35.0 {
		t.Errorf("today value = %v", v)
	}
	// no ledger piece of the leg: no stages and a named gap
	g := readFixture(t, "detail_gaps").build(t, gapsID, true)
	if len(list(t, g["timeline"])) != 0 {
		t.Errorf("timeline = %v", g["timeline"])
	}
	if gaps := list(t, g["gaps"]); len(gaps) == 0 {
		t.Error("no gap named")
	}
}
