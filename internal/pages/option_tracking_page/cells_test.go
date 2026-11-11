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
	if g := asGap(t, numCell("abc", codeNoValue)); g.Reason != "no_data" || !strings.HasPrefix(g.Text, "unreadable") {
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
	if g.Kind != "gap" || g.Reason != "missing_quote" || !strings.HasPrefix(g.Text, "no_mark") {
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
	if g := asGap(t, guardCell(o)); g.Reason != "missing_input" || !strings.HasPrefix(g.Text, "no_guard_row") {
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
		if g := asGap(t, c); g.Reason != "missing_input" || !strings.HasPrefix(g.Text, "no_event_row") {
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
	if g := asGap(t, numCell("", codeNoValue)); g.Reason != "no_data" || !strings.HasPrefix(g.Text, "no_value") {
		t.Fatalf("gap = %+v", g)
	}
}
