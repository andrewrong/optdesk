package option_notices_page

import (
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
	if got := amountText("95000", "USD"); got != "95000 USD" {
		t.Errorf("amount = %q", got)
	}
	if got := amountText("5", ""); got != "5" {
		t.Errorf("amount = %q", got)
	}
	if got := clip(""); got != noContract {
		t.Errorf("clip empty = %q", got)
	}
	if got := clip(strings.Repeat("é", 300)); len([]rune(got)) != 200 {
		t.Errorf("clip long = %d runes", len([]rune(got)))
	}
}
