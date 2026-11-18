package option_notices_page

import (
	"github.com/andrewrong/optdesk/internal/compute"
	"strings"
	"testing"
)

func TestNoticeIDWithinPattern(t *testing.T) {
	t.Parallel()
	id := noticeID(strings.Repeat("é a/b", 10), 12, "expiry_near")
	if !strings.HasPrefix(id, "notice-") || len(id) > 64 || strings.ContainsAny(id, " /é") {
		t.Errorf("id = %q", id)
	}
}

func TestAmountTextAndClip(t *testing.T) {
	t.Parallel()
	if got := amountText("95000", "USD"); got != "$95,000.00" {
		t.Errorf("amount = %q", got)
	}
	if got := clip(""); got != noContract {
		t.Errorf("clip empty = %q", got)
	}
	if got := clip(strings.Repeat("é", 300)); len([]rune(got)) != 200 {
		t.Errorf("clip long = %d runes", len([]rune(got)))
	}
}

func TestPercentText(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"0.1250": "12.5%", "0.634749": "63.5%", "0.04": "4.0%", "0.0625": "6.2%", "0.0675": "6.8%",
		"1": "100.0%", "n/a": "n/a", "": "",
	}
	for in, want := range cases {
		if got := percentText(in); got != want {
			t.Errorf("percentText(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAmountTextMoney(t *testing.T) {
	t.Parallel()
	cases := []struct{ amount, currency, want string }{
		{"9500", "USD", "$9,500.00"},
		{"1000000", "USD", "$1,000,000.00"},
		{"1200000.50", "JPY", "¥1,200,000.50"},
		{"1200", "EUR", "1,200.00 EUR"},
		{"5", "", "5.00"},
		{"0.125", "USD", "$0.12"},
		{"0.135", "USD", "$0.14"},
		{"-1234.5", "USD", "-$1,234.50"},
	}
	for _, c := range cases {
		if got := amountText(c.amount, c.currency); got != c.want {
			t.Errorf("amountText(%q, %q) = %q, want %q", c.amount, c.currency, got, c.want)
		}
	}
}

func TestAmountTextUnreadable(t *testing.T) {
	t.Parallel()
	for _, in := range []string{"abc", "", "1/2", "1e3", "NaN"} {
		if got := amountText(in, "EUR"); got != in+" EUR" {
			t.Errorf("amountText(%q, EUR) = %q", in, got)
		}
	}
	if got := amountText("abc", ""); got != "abc" {
		t.Errorf("amountText(abc, empty) = %q", got)
	}
}

func TestHintsAtMost200Runes(t *testing.T) {
	t.Parallel()
	for _, h := range []string{cushionHintText(compute.NoticeParams{CushionWarn: "0.15", CushionAct: "0.05"}), noteHint} {
		if n := len([]rune(h)); n == 0 || n > 200 {
			t.Errorf("hint %q has %d runes", h, n)
		}
	}
}

func TestExplanationsAreChinese(t *testing.T) {
	t.Parallel()
	hint := cushionHintText(compute.NoticeParams{CushionWarn: "0.15", CushionAct: "0.05"})
	for name, s := range map[string]string{"noteHint": noteHint, "cushionHint": hint, "cashGloss": cashGlossZh} {
		found := false
		for _, r := range s {
			if r >= 0x4e00 && r <= 0x9fff {
				found = true
			}
		}
		if !found {
			t.Errorf("%s has no CJK rune: %q", name, s)
		}
	}
	if !strings.Contains(hint, "15%") || !strings.Contains(hint, "5%") {
		t.Errorf("hint lost thresholds: %q", hint)
	}
}
