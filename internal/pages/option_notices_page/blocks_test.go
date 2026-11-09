package option_notices_page

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
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
	if len(got) != 2 || got[0] != (cashRow{Day: "2026-11-13", Currency: "USD", AmountText: "9500 USD"}) ||
		got[1].AmountText != "1200000.50 JPY" {
		t.Errorf("cash = %+v", got)
	}
}

func TestCushionText(t *testing.T) {
	t.Parallel()
	p := buildPage(t, testConfig(oneCapture(), &fakeRule{result: compute.NoticeResult{Cushion: "0.1250"}}))
	if c := p.Accounts[0].CushionText; c == nil || *c != "Cushion 0.1250" {
		t.Errorf("cushion = %v", c)
	}
	p = buildPage(t, testConfig(oneCapture(), &fakeRule{}))
	if p.Accounts[0].CushionText != nil {
		t.Errorf("cushion = %v", *p.Accounts[0].CushionText)
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
	if len(n.Facts) != 1 || n.Facts[0] != (fact{Label: "财报日", Text: "2026-12-01"}) {
		t.Errorf("facts = %v", n.Facts)
	}
}
