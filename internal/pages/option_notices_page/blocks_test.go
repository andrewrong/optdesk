package option_notices_page

import (
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

func oneCapture() fakeAccount {
	return fakeAccount{positions: map[string][]store.Position{"main/2026-11-09": {stock("ACME")}}}
}

func TestRuleOrderKept(t *testing.T) {
	t.Parallel()
	rule := &fakeRule{result: compute.NoticeResult{Notices: []compute.Notice{
		{Code: "cushion_low", Severity: "warn"},
		{Code: "expiry_near", Severity: "warn", Contract: "ACME 2026-11-13 95 P", Expiry: "2026-11-13"},
		{Code: "expiry_today", Severity: "act", Contract: "ZED 2026-11-09 5 P", Expiry: "2026-11-09"},
		{Code: "capture_stale", Severity: "info"},
	}}}
	p := buildPage(t, testConfig(oneCapture(), rule))
	var codes []string
	for _, n := range p.Accounts[0].Notices {
		codes = append(codes, n.Code)
	}
	want := []string{"cushion_low", "expiry_near", "expiry_today", "capture_stale"}
	if len(codes) != 4 || codes[0] != want[0] || codes[1] != want[1] || codes[2] != want[2] || codes[3] != want[3] {
		t.Errorf("codes = %v", codes)
	}
	first := p.Accounts[0].Notices[0]
	if first.Expiry != nil || first.Contract != noContract || first.ID != "notice-main-0-cushion_low" {
		t.Errorf("first = %+v", first)
	}
}

func TestCashByDateText(t *testing.T) {
	t.Parallel()
	rule := &fakeRule{result: compute.NoticeResult{CashByDate: []compute.CashDay{
		{Day: "2026-11-13", Currency: "USD", Amount: "9500"},
		{Day: "2026-11-13", Currency: "JPY", Amount: "1200000.50"},
	}}}
	p := buildPage(t, testConfig(oneCapture(), rule))
	got := p.Accounts[0].CashByDate
	if len(got) != 2 || got[0].Day != "2026-11-13" || got[0].Currency != "USD" || got[0].AmountText != "$9,500.00" ||
		got[1].AmountText != "¥1,200,000.50" {
		t.Errorf("cash = %+v", got)
	}
}

func TestCushionText(t *testing.T) {
	t.Parallel()
	cases := map[string]string{"0.1250": "Cushion 12.5%", "0.634749": "Cushion 63.5%", "n/a": "Cushion n/a"}
	for in, want := range cases {
		p := buildPage(t, testConfig(oneCapture(), &fakeRule{result: compute.NoticeResult{Cushion: in}}))
		if c := p.Accounts[0].CushionText; c == nil || *c != want {
			t.Errorf("cushion %q = %v, want %q", in, c, want)
		}
	}
	p := buildPage(t, testConfig(oneCapture(), &fakeRule{}))
	if p.Accounts[0].CushionText != nil {
		t.Errorf("cushion = %v", *p.Accounts[0].CushionText)
	}
}

func TestCushionFactIsPercent(t *testing.T) {
	t.Parallel()
	rule := &fakeRule{result: compute.NoticeResult{Notices: []compute.Notice{{
		Code: "cushion_low", Severity: "act", Facts: []compute.NoticeFact{{Label: "cushion", Value: "0.04"}},
	}}}}
	n := buildPage(t, testConfig(oneCapture(), rule)).Accounts[0].Notices[0]
	if len(n.Facts) != 1 || n.Facts[0].Text != "4.0%" {
		t.Errorf("facts = %+v", n.Facts)
	}
}

func TestGuardReasonsAndFacts(t *testing.T) {
	t.Parallel()
	rule := &fakeRule{result: compute.NoticeResult{Notices: []compute.Notice{{
		Code: "call_guard", Severity: "warn", Contract: "ACME 2026-12-18 100 C", Expiry: "2026-12-18",
		Facts: []compute.NoticeFact{{Label: "reason", Value: "strike_floor"}, {Label: "reason", Value: "odd_code"}, {Label: "earnings", Value: "2026-12-01"}},
	}}}}
	n := buildPage(t, testConfig(oneCapture(), rule)).Accounts[0].Notices[0]
	if len(n.Reasons) != 2 || n.Reasons[0] != reasonTexts["strike_floor"] || n.Reasons[1] != "odd_code" {
		t.Errorf("reasons = %v", n.Reasons)
	}
	if len(n.Facts) != 1 || n.Facts[0] != (fact{Label: "Earnings date", Text: "2026-12-01"}) {
		t.Errorf("facts = %v", n.Facts)
	}
}

func putPos(symbol, expiry, strike, qty, cur string) store.Position {
	return store.Position{Symbol: symbol, SecType: "option", Expiry: expiry, Strike: strike, Right: "P",
		Multiplier: "100", Quantity: qty, Currency: cur}
}

func TestCashPutsListed(t *testing.T) {
	t.Parallel()
	call := putPos("CALLCO", "2026-11-13", "50", "-1", "USD")
	call.Right = "C"
	pos := []store.Position{
		putPos("AAA", "2026-11-13", "95", "-2", "USD"),
		call,
		putPos("LONGCO", "2026-11-13", "40", "3", "USD"),
		putPos("YENCO", "2026-11-13", "900", "-1", "JPY"),
		putPos("BBB", "2026-11-20", "10.5", "-1", "USD"),
		putPos("CCC", "2026-11-13", "96", "-1", "USD"),
	}
	rule := &fakeRule{result: compute.NoticeResult{CashByDate: []compute.CashDay{
		{Day: "2026-11-13", Currency: "USD", Amount: "28700"},
		{Day: "2026-11-20", Currency: "USD", Amount: "1050"},
		{Day: "2026-11-27", Currency: "USD", Amount: "5"},
	}}}
	a := fakeAccount{positions: map[string][]store.Position{"main/2026-11-09": pos}}
	got := buildPage(t, testConfig(a, rule)).Accounts[0].CashByDate
	want := [][]putRow{
		{{Symbol: "AAA", Expiry: "2026-11-13", StrikeText: "95", Right: "P", QuantityText: "2"},
			{Symbol: "CCC", Expiry: "2026-11-13", StrikeText: "96", Right: "P", QuantityText: "1"}},
		{{Symbol: "BBB", Expiry: "2026-11-20", StrikeText: "10.5", Right: "P", QuantityText: "1"}},
		{},
	}
	for i, w := range want {
		if got[i].Puts == nil || len(got[i].Puts) != len(w) {
			t.Fatalf("row %d puts = %#v", i, got[i].Puts)
		}
		for j := range w {
			if got[i].Puts[j] != w[j] {
				t.Errorf("row %d put %d = %+v, want %+v", i, j, got[i].Puts[j], w[j])
			}
		}
	}
	if got[0].AmountText != "$28,700.00" {
		t.Errorf("amount = %q", got[0].AmountText)
	}
}

func TestHintsSetOnlyWhenShown(t *testing.T) {
	t.Parallel()
	// cushion present, guard rows stored for the day: no note.
	cand := &fakeCandidates{rows: map[string][]store.Candidate{"2026-11-09/held_call_guard": {{Alias: "main"}}}}
	cfg := testConfig(oneCapture(), &fakeRule{result: compute.NoticeResult{Cushion: "0.1"}})
	cfg.Candidates = cand
	b := buildPage(t, cfg).Accounts[0]
	if b.CushionHint == nil || *b.CushionHint != cushionHintText(cfg.Params) || b.NoteHint != nil || b.Note != nil {
		t.Errorf("cushion present: %+v", b)
	}
	// cushion absent, no guard record.
	b = buildPage(t, testConfig(oneCapture(), &fakeRule{})).Accounts[0]
	if b.CushionHint != nil || b.NoteHint == nil || *b.NoteHint != noteHint || b.Note == nil || *b.Note != noteNoGuard {
		t.Errorf("no guard: %+v", b)
	}
	// no capture: note is not the no-guard note, so no hint.
	b = buildPage(t, testConfig(fakeAccount{positions: map[string][]store.Position{"main/2026-11-09": {stock("A")}}},
		&fakeRule{})).Accounts[1]
	if b.Status != "no_capture" || b.NoteHint != nil || b.CushionHint != nil {
		t.Errorf("no capture: %+v", b)
	}
}

func TestCushionHintFollowsParams(t *testing.T) {
	t.Parallel()
	cfg := testConfig(oneCapture(), &fakeRule{result: compute.NoticeResult{Cushion: "0.1"}})
	cfg.Params.CushionWarn, cfg.Params.CushionAct = "0.20", "0.08"
	b := buildPage(t, cfg).Accounts[0]
	if b.CushionHint == nil || strings.Contains(*b.CushionHint, "15%") || strings.Contains(*b.CushionHint, " 5%") {
		t.Errorf("hint does not match params 0.20/0.08: %v", b.CushionHint)
	}
	if !strings.Contains(*b.CushionHint, "20%") || !strings.Contains(*b.CushionHint, "8%") {
		t.Errorf("hint = %q", *b.CushionHint)
	}
}

func TestBlockCarriesCashGlossAndPerDateCash(t *testing.T) {
	t.Parallel()
	rule := &fakeRule{result: compute.NoticeResult{CashByDate: []compute.CashDay{
		{Day: "2026-11-13", Currency: "USD", Amount: "9500", Available: "25000", Left: "15500"},
		{Day: "2026-11-20", Currency: "USD", Amount: "100"},
	}}}
	p := buildPage(t, testConfig(oneCapture(), rule))
	b := p.Accounts[0]
	if b.CashGloss == nil || *b.CashGloss != cashGlossZh {
		t.Errorf("gloss = %v", b.CashGloss)
	}
	got := b.CashByDate
	if len(got) != 2 || got[0].AvailableText == nil || *got[0].AvailableText != "$25,000.00" ||
		got[0].LeftText == nil || *got[0].LeftText != "$15,500.00" {
		t.Fatalf("row 0 = %+v", got)
	}
	if got[1].AvailableText != nil || got[1].LeftText != nil {
		t.Errorf("row 1 = %+v", got[1])
	}
	if nc := p.Accounts[1]; nc.Status != "no_capture" || nc.CashGloss != nil {
		t.Errorf("no capture = %+v", nc)
	}
}

func TestStaleByCases(t *testing.T) {
	t.Parallel()
	s := []contracts.Day{"2026-11-13", "2026-11-16", "2026-11-17", "2026-11-18"}
	for _, c := range []struct {
		name        string
		data, build contracts.Day
		sessions    []contracts.Day
		stale       bool
	}{
		{"build day session", "2026-11-18", "2026-11-18", s, false},
		{"session before", "2026-11-17", "2026-11-18", s, false},
		{"two sessions behind", "2026-11-16", "2026-11-18", s, true},
		{"after build day", "2026-11-19", "2026-11-18", s, true},
		{"off the list", "2026-11-15", "2026-11-18", s, true},
		{"saturday build accepts friday", "2026-11-13", "2026-11-14", s, false},
		{"empty list keeps old rule, older", "2026-11-17", "2026-11-18", nil, true},
		{"empty list keeps old rule, same", "2026-11-18", "2026-11-18", nil, false},
	} {
		if got := staleBy(c.data, c.build, c.sessions); got != c.stale {
			t.Errorf("%s: staleBy = %v, want %v", c.name, got, c.stale)
		}
	}
}
