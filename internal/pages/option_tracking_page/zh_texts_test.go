package option_tracking_page

import (
	"testing"
	"unicode"

	"github.com/andrewrong/optdesk/internal/compute/trackapi"
)

func hasHan(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func TestTrackingTextsAreChinese(t *testing.T) {
	t.Parallel()
	res := okResult()
	o := sampleOpen("p")
	o.Currency = "USD"
	o.MarkStatus, o.Mid, o.Spot = "no_mark", "", ""
	o.Delta, o.Iv, o.Kept, o.KeptPercent, o.OtmPercent, o.Moneyness = "", "", "", "", "", ""
	o.EventStatus, o.Guard, o.GuardReason = "no_event_row", "unknown", "no_guard_row"
	o.Right = "C"
	res.Open = []opt{o}
	res.LongLegsLeftOut = 2
	vm := buildPage(t, cfgFor(map[string]trackapi.TrackResult{"main": res}, "main"))

	if hasHan(vm.Title) {
		t.Errorf("title = %q has Chinese", vm.Title)
	}
	for _, n := range append([]string{vm.MarkNote}, vm.Notes...) {
		if !hasHan(n) {
			t.Errorf("note %q has no Chinese character", n)
		}
	}
	tbl := vm.Accounts[0].Open
	for _, c := range tbl.Columns {
		if hasHan(c.Label) {
			t.Errorf("label %q has Chinese", c.Label)
		}
		if c.Hint != nil && !hasHan(*c.Hint) {
			t.Errorf("hint of %s = %q has no Chinese character", c.ID, *c.Hint)
		}
	}
	gaps := 0
	for id, cell := range tbl.Rows[0].Cells {
		m, ok := cell.(map[string]any)
		if !ok || m["kind"] != "gap" {
			continue
		}
		gaps++
		if txt, _ := m["text"].(string); !hasHan(txt) {
			t.Errorf("gap %s = %q has no Chinese character", id, txt)
		}
	}
	if gaps == 0 {
		t.Fatal("no gap cell in the page")
	}
}

func TestEveryCodeTextIsChinese(t *testing.T) {
	t.Parallel()
	for code, txt := range codeTexts {
		if !hasHan(txt) {
			t.Errorf("%s = %q has no Chinese character", code, txt)
		}
	}
}

func TestNoOpenGapTextIsChinese(t *testing.T) {
	t.Parallel()
	c := cfgFor(map[string]trackapi.TrackResult{"main": {}}, "main")
	c.Account = nil
	a := buildPage(t, c).Accounts[0]
	if a.Gap == nil || !hasHan(a.Gap.Text) || a.Gap.Text == "No open short options" {
		t.Fatalf("gap = %+v", a.Gap)
	}
}
