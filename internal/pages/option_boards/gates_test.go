package option_boards

import (
	"reflect"
	"testing"
)

func TestParseGates(t *testing.T) {
	t.Parallel()
	text := `[{"gate":"quote","status":"pass","reason":""},{"gate":"events","status":"block","reason":"earnings_before_expiry"}]`
	gates, err := parseGates(text)
	if err != nil || len(gates) != 2 {
		t.Fatalf("parseGates = %v, %v", gates, err)
	}
	if s, ok := gateStatus(gates, gateEvents); !ok || s != gateBlock {
		t.Errorf("events = %q, %v", s, ok)
	}
	if s, ok := gateStatus(gates, gateQuote); !ok || s != gatePass {
		t.Errorf("quote = %q, %v", s, ok)
	}
	if _, ok := gateStatus(gates, "cash"); ok {
		t.Error("cash gate should be absent")
	}
	for _, bad := range []string{``, `null`, `{}`, `[{"gate":"Q","status":"pass"}]`,
		`[{"gate":"quote","status":"maybe"}]`, `[{"gate":"quote","status":"block","reason":"Bad Code"}]`} {
		if _, err := parseGates(bad); err == nil {
			t.Errorf("parseGates(%q) want an error", bad)
		}
	}
}

func TestParseMarks(t *testing.T) {
	t.Parallel()
	marks, err := parseMarks(`["earnings_unconfirmed","earnings_beyond_horizon","itm"]`)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Earnings date not confirmed", "Earnings passed, none within 60 days", "itm"}
	if got := markBadges(marks); !reflect.DeepEqual(got, want) {
		t.Errorf("badges = %v, want %v", got, want)
	}
	if _, err := parseMarks(`null`); err == nil {
		t.Error("null marks want an error")
	}
}

func TestMarkBadgesHideBuySource(t *testing.T) {
	t.Parallel()
	got := markBadges([]string{"buy_from_valuation", "itm"})
	if want := []string{"itm"}; !reflect.DeepEqual(got, want) {
		t.Errorf("badges = %v, want %v", got, want)
	}
	if got := markBadges([]string{"buy_from_hand", "buy_from_valuation"}); len(got) != 0 {
		t.Errorf("only buy source marks: badges = %v, want none", got)
	}
}

func TestMarkBadgesUnknownStillRaw(t *testing.T) {
	t.Parallel()
	got := markBadges([]string{"some_future_mark"})
	if want := []string{"some_future_mark"}; !reflect.DeepEqual(got, want) {
		t.Errorf("badges = %v, want %v", got, want)
	}
}
