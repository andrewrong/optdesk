package option_tracking_page

import (
	"encoding/json"
	"strings"
	"testing"
)

func asGap(t *testing.T, c any) gap {
	t.Helper()
	g, ok := c.(gap)
	if !ok {
		t.Fatalf("cell = %#v, want a gap", c)
	}
	return g
}

func asValue(t *testing.T, c any) valueCell {
	t.Helper()
	v, ok := c.(valueCell)
	if !ok {
		t.Fatalf("cell = %#v, want a value cell", c)
	}
	return v
}

func hasBadge(c valueCell, b string) bool {
	for _, x := range c.Badges {
		if x == b {
			return true
		}
	}
	return false
}

func TestNumericCellsCarryValue(t *testing.T) {
	t.Parallel()
	c := asValue(t, numCell("100.50", codeNoValue))
	if c.Text != "100.50" || c.Value == nil || *c.Value != "100.50" {
		t.Fatalf("cell = %+v", c)
	}
	b, _ := json.Marshal(c)
	if !strings.Contains(string(b), `"value":100.50`) {
		t.Fatalf("json = %s", b)
	}
	if g := asGap(t, numCell("abc", codeNoValue)); g.Reason != "no_data" || g.Text != "Unreadable" {
		t.Fatalf("gap = %+v", g)
	}
	m := asValue(t, moneynessCell(sampleOpen("p")))
	if m.Text != "3.20%" || *m.Value != "3.20" || !hasBadge(m, "out of the money") {
		t.Fatalf("moneyness = %+v", m)
	}
}

func TestMissingMarkIsGapCell(t *testing.T) {
	t.Parallel()
	o := sampleOpen("p")
	o.Mid, o.MarkStatus = "", "no_mark"
	g := asGap(t, numCell(o.Mid, markCode(o)))
	if g.Kind != "gap" || g.Reason != "missing_quote" || g.Text != "No fresh closing price" {
		t.Fatalf("gap = %+v", g)
	}
	for _, d := range openColumns {
		if v, ok := d.cell(o).(valueCell); ok && v.Text == "0" {
			t.Fatalf("column %s has the text 0", d.ID)
		}
	}
}

func TestGuardBadges(t *testing.T) {
	t.Parallel()
	o := sampleOpen("p")
	if c := asValue(t, guardCell(o)); !hasBadge(c, "ok") || c.Tone != "up" || !strings.Contains(c.Text, "Basis bid") {
		t.Fatalf("ok = %+v", c)
	}
	o.Guard, o.GuardReason = "block", "early_assignment_risk"
	if c := asValue(t, guardCell(o)); !hasBadge(c, "Blocked") || c.Tone != "down" || !strings.Contains(c.Text, "Early assignment") {
		t.Fatalf("block = %+v", c)
	}
	o.Guard, o.GuardReason = "unknown", "no_dividend_info"
	if c := asValue(t, guardCell(o)); !hasBadge(c, "Unconfirmed") || !strings.Contains(c.Text, "No dividend information") {
		t.Fatalf("unknown = %+v", c)
	}
	o.GuardReason = "no_guard_row"
	if g := asGap(t, guardCell(o)); g.Reason != "missing_input" || g.Text != codeTexts["no_guard_row"] {
		t.Fatalf("no row = %+v", g)
	}
	o.Right, o.Guard, o.GuardReason = "P", "n/a", ""
	if c := asValue(t, guardCell(o)); !hasBadge(c, "Not applicable") || c.Text != "Not applicable" {
		t.Fatalf("put = %+v", c)
	}
}

func TestEarlyAssignmentBadge(t *testing.T) {
	t.Parallel()
	o := sampleClosed("p")
	if c := asValue(t, statusCell(o)); hasBadge(c, "Early") || !hasBadge(c, "assigned") {
		t.Fatalf("not early = %+v", c)
	}
	o.Early = true
	if c := asValue(t, statusCell(o)); !hasBadge(c, "Early") || !hasBadge(c, "assigned") {
		t.Fatalf("early = %+v", c)
	}
}

func TestEventsInLifeBadge(t *testing.T) {
	t.Parallel()
	o := sampleOpen("p")
	if c := asValue(t, resultsCell(o)); c.Text != "2026-12-01" || !hasBadge(c, "In term") {
		t.Fatalf("results = %+v", c)
	}
	if c := asValue(t, exCell(o)); c.Text != "2026-12-05 0.25" || !hasBadge(c, "In term") {
		t.Fatalf("ex = %+v", c)
	}
	o.EarningsInLife, o.ExInLife = false, false
	if c := asValue(t, resultsCell(o)); hasBadge(c, "In term") {
		t.Fatalf("results = %+v", c)
	}
	o.Right = "P"
	if c := asValue(t, exCell(o)); c.Text != "Not applicable" {
		t.Fatalf("put ex = %+v", c)
	}
	o.Right, o.EventStatus = "C", "no_event_row"
	for _, c := range []any{resultsCell(o), exCell(o)} {
		if g := asGap(t, c); g.Reason != "missing_input" || g.Text != codeTexts["no_event_row"] {
			t.Fatalf("gap = %+v", g)
		}
	}
}

func TestEmptyStringIsNeverZero(t *testing.T) {
	t.Parallel()
	for _, status := range []string{"ok", "no_mark"} {
		o := opt{MarkStatus: status, Right: "C", Guard: "ok", EventStatus: "ok"}
		for _, d := range openColumns {
			switch d.ID {
			case "mid", "spot", "premium", "kept", "kept_percent", "delta", "iv", "moneyness", "qty":
				g := asGap(t, d.cell(o))
				if g.Reason != "missing_quote" && g.Reason != "no_data" {
					t.Fatalf("%s/%s gap = %+v", status, d.ID, g)
				}
			}
		}
	}
	if g := asGap(t, numCell("", codeNoValue)); g.Reason != "no_data" || g.Text != "No such value" {
		t.Fatalf("gap = %+v", g)
	}
}

func TestCodeGapTextHasNoCodeWord(t *testing.T) {
	t.Parallel()
	for _, code := range []string{"no_mark", "no_event_row", "no_guard_row", "no_account"} {
		g := codeGap(code)
		if g.Text != codeTexts[code] || g.Text == "" || strings.Contains(g.Text, "_") {
			t.Errorf("%s: gap = %+v", code, g)
		}
	}
	if g := codeGap("no_mark"); g.Text != "No fresh closing price" {
		t.Fatalf("gap = %+v", g)
	}
}

func TestPremiumCellShowsCurrency(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ in, cur, text string }{
		{"13000.0000", "JPY", "¥13,000.00"},
		{"5550.0000", "JPY", "¥5,550.00"},
		{"250.5", "USD", "$250.50"},
		{"-1234567.891", "USD", "-$1,234,567.89"},
	} {
		v := asValue(t, moneyCell(c.in, c.cur))
		if v.Text != c.text || v.Value == nil {
			t.Errorf("%s %s: cell = %+v", c.in, c.cur, v)
		}
	}
	v := asValue(t, moneyCell("13000.0000", "JPY"))
	if *v.Value != "13000.0000" {
		t.Fatalf("value = %v", *v.Value)
	}
}

func TestPremiumCellUnknownCurrency(t *testing.T) {
	t.Parallel()
	for _, cur := range []string{"", "XYZ"} {
		g := asGap(t, moneyCell("100", cur))
		if g.Reason != "no_data" || g.Text != "Unreadable" {
			t.Errorf("%q: gap = %+v", cur, g)
		}
	}
	if g := asGap(t, moneyCell("", "USD")); g.Text != "No such value" {
		t.Fatalf("gap = %+v", g)
	}
}

func TestDecimalCellRounds(t *testing.T) {
	t.Parallel()
	c := asValue(t, decimalCell("394.8229675292969", codeNoValue, 2))
	if c.Text != "394.82" || c.Value == nil || *c.Value != "394.8229675292969" {
		t.Fatalf("cell = %+v", c)
	}
	if c := asValue(t, decimalCell("3.3250", codeNoValue, 2)); c.Text != "3.33" {
		t.Fatalf("3.3250 = %+v", c)
	}
	if c := asValue(t, decimalCell("0.31254", codeNoValue, 4)); c.Text != "0.3125" {
		t.Fatalf("iv = %+v", c)
	}
	if g := asGap(t, decimalCell("", codeNoMark, 2)); g.Reason != "missing_quote" {
		t.Fatalf("empty = %+v", g)
	}
	if g := asGap(t, decimalCell("abc", codeNoValue, 2)); g.Text != "Unreadable" {
		t.Fatalf("abc = %+v", g)
	}
}
