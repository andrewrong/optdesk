package structures_page_test

import "testing"

func TestTimelineDays(t *testing.T) {
	t.Parallel()
	m := readFixture(t, "detail_spread").build(t, spreadID, true)
	st := list(t, m["timeline"])
	if len(st) != 2 {
		t.Fatalf("timeline = %v", st)
	}
	// the dates are copied: opened 2026-10-19 (ledger piece), first expiry 2026-12-18 (legs); no day count is written
	type want struct{ name, date, text, days string }
	for i, w := range []want{
		{"开仓", "2026-10-19", "0%", "开仓日"},
		{"到期", "2026-12-18", "100%", "到期日"},
	} {
		s := obj(t, st[i])
		el := obj(t, s["elapsed"])
		if s["name"] != w.name || s["date_text"] != w.date || el["text"] != w.text || s["days_text"] != w.days || el["unit"] != "share_percent" {
			t.Errorf("stage %d = %v, want %+v", i, s, w)
		}
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
