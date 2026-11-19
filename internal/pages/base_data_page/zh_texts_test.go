package base_data_page

import (
	"strings"
	"testing"
	"unicode"

	"github.com/andrewrong/optdesk/internal/compute/basecheckapi"
)

func hasCJK(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

// TestEveryRuleAndRowHasChineseHint: each rule id and row id of the table has a text with a CJK rune.
func TestEveryRuleAndRowHasChineseHint(t *testing.T) {
	t.Parallel()
	for _, r := range basecheckapi.Rules {
		id := r.Row + "." + r.Rule
		if h, ok := Hint(id); !ok || !hasCJK(h) {
			t.Errorf("rule %s has no Chinese hint", id)
		}
	}
	for _, r := range basecheckapi.Rows {
		if h, ok := Hint(r.ID); !ok || !hasCJK(h) {
			t.Errorf("row %s has no Chinese hint", r.ID)
		}
	}
	if _, ok := Hint("nosuchrow.rows"); ok {
		t.Error("a rule of an unknown row has a hint")
	}
	if _, ok := Hint("fx.ohlc_order"); ok {
		t.Error("a rule the row does not have has a hint")
	}
	if _, ok := Hint("nosuchrow"); ok {
		t.Error("an unknown row has a hint")
	}
}

// TestBarJumpHintSaysSplit: the hint of bar_jump says that a split looks like this.
func TestBarJumpHintSaysSplit(t *testing.T) {
	t.Parallel()
	h, _ := Hint("bars.bar_jump")
	if !strings.Contains(h, "拆股") {
		t.Fatalf("hint = %s", h)
	}
}

// TestGlossaryExplainsEveryTerm: every term has a Chinese text, terms are unique, and the four levels and the
// words the page uses (session, ledger, trial, share) are there.
func TestGlossaryExplainsEveryTerm(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for _, g := range glossary() {
		if g.Term == "" || !hasCJK(g.Text) {
			t.Errorf("term %q has no Chinese text", g.Term)
		}
		if seen[g.Term] {
			t.Errorf("term %q twice", g.Term)
		}
		seen[g.Term] = true
	}
	for _, want := range []string{"pass", "warn", "fail", "unknown", "session", "ledger", "trial", "share"} {
		if !seen[want] {
			t.Errorf("term %q missing", want)
		}
	}
}

// TestHistoryLostLabelAndHint: history_lost has the label "History lost" and the hint that says no repair is needed, on each of its five rows.
func TestHistoryLostLabelAndHint(t *testing.T) {
	t.Parallel()
	for _, row := range []string{"chains", "contracts", "calendar", "flex", "open"} {
		h, ok := Hint(row + ".history_lost")
		if !ok || h != "历史缺口，无法补采，无需处理" {
			t.Errorf("Hint(%s.history_lost) = %q, %v", row, h, ok)
		}
	}
	if l := ruleLabel("history_lost"); l != "History lost" {
		t.Errorf("label = %q", l)
	}
}
