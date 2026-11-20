package contracts_test

import (
	"testing"
)

func macroLeadRow(key, state string) map[string]any {
	return map[string]any{
		"key": key, "label": "Label", "text": "3.9%", "data_day": "2026-10-23",
		"change_text": nil, "hint": "hint", "state": state,
	}
}

func macroLead(rows ...map[string]any) map[string]any {
	items := make([]any, 0, len(rows))
	for _, r := range rows {
		items = append(items, r)
	}
	return map[string]any{"title": "Lead", "rows": items, "note": "display only"}
}

func fourLeadRows(state string) []map[string]any {
	return []map[string]any{
		macroLeadRow("tbill", "ok"), macroLeadRow("hy_spread", "ok"),
		macroLeadRow("next_event", "ok"), macroLeadRow("index_iv", state),
	}
}

// TestMacroPageLeadOptional: the golden instance validates unchanged; a lead of four rows, event
// day counts and an indicator with a LevelFigure and the grouped fields validate; a lead of three
// rows or a state of red does not.
func TestMacroPageLeadOptional(t *testing.T) {
	t.Parallel()
	s := compilePageV2(t, "page.macro_page.v2.json")

	t.Run("golden unchanged", func(t *testing.T) {
		t.Parallel()
		if err := s.Validate(loadMacroPageV2(t, "golden")); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("lead and level figure", func(t *testing.T) {
		t.Parallel()
		m := loadMacroPageV2(t, "golden")
		m["lead"] = macroLead(fourLeadRows("gap")...)
		events := m["events"].([]any)
		ev := events[0].(map[string]any)
		ev["days_to"] = 2
		ev["in_window"] = true
		inds := m["indicators"].([]any)
		ind := inds[0].(map[string]any)
		ind["latest"] = map[string]any{"value": 7.52, "text": "7.52", "unit": "jpy_per_usd"}
		ind["group"] = "dollar_japan"
		ind["change_text"] = "+0.4 in five sessions"
		ind["hint"] = "hint"
		ind["data_day"] = "2026-10-23"
		if err := s.Validate(m); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("three rows", func(t *testing.T) {
		t.Parallel()
		m := loadMacroPageV2(t, "golden")
		m["lead"] = macroLead(fourLeadRows("ok")[:3]...)
		if err := s.Validate(m); err == nil {
			t.Fatal("a lead of three rows validated")
		}
	})
	t.Run("state red", func(t *testing.T) {
		t.Parallel()
		m := loadMacroPageV2(t, "golden")
		m["lead"] = macroLead(fourLeadRows("red")...)
		if err := s.Validate(m); err == nil {
			t.Fatal("a state of red validated")
		}
	})
	t.Run("unknown level unit", func(t *testing.T) {
		t.Parallel()
		m := loadMacroPageV2(t, "golden")
		ind := m["indicators"].([]any)[0].(map[string]any)
		ind["latest"] = map[string]any{"value": 1.0, "text": "1", "unit": "bps"}
		if err := s.Validate(m); err == nil {
			t.Fatal("an unknown unit validated")
		}
	})
}
