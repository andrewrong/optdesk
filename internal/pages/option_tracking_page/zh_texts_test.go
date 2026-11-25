package option_tracking_page

import (
	"strings"
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

	if vm.Title != "我的 short 期权跟踪 (My short option tracking)" {
		t.Errorf("title = %q", vm.Title)
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
	if a.Gap == nil || !hasHan(a.Gap.Text) || a.Gap.Text != "没有未平仓的 short 期权" {
		t.Fatalf("gap = %+v", a.Gap)
	}
}

func TestBadgesAndTitlesBuiltFromOutputAreBilingual(t *testing.T) {
	t.Parallel()
	o := sampleOpen("p")
	o.Guard, o.GuardReason = "block", "early_assignment_risk"
	if c := asValue(t, guardCell(o)); !hasBadge(c, "阻止 (Blocked)") {
		t.Errorf("block = %+v", c)
	}
	o.Guard, o.GuardReason = "unknown", "no_dividend_info"
	if c := asValue(t, guardCell(o)); !hasBadge(c, "未确认 (Unconfirmed)") {
		t.Errorf("unknown = %+v", c)
	}
	o.Guard, o.GuardReason = "ok", ""
	o.GuardBasis = "bid"
	if c := asValue(t, guardCell(o)); !strings.Contains(c.Text, "依据 (Basis) bid") {
		t.Errorf("basis = %+v", c)
	}
	o.Right, o.Guard = "P", "n/a"
	if c := asValue(t, guardCell(o)); !hasBadge(c, "不适用 (Not applicable)") || c.Text != "不适用 (Not applicable)" {
		t.Errorf("put = %+v", c)
	}
	if c := asValue(t, resultsCell(sampleOpen("p"), "")); !hasBadge(c, "期内 (In term)") {
		t.Errorf("results = %+v", c)
	}
	e := sampleClosed("p")
	e.Early = true
	if c := asValue(t, statusCell(e)); !hasBadge(c, "提前 (Early)") {
		t.Errorf("early = %+v", c)
	}
}

func TestEmptyAccountTitleIsBilingual(t *testing.T) {
	t.Parallel()
	a := buildPage(t, cfgFor(map[string]trackapi.TrackResult{"main": {}}, "main")).Accounts[0]
	if want := "账户 (Account: main) · 没有未平仓的 short 期权 (No open short options)"; a.Title != want {
		t.Errorf("title = %q, want %q", a.Title, want)
	}
}
