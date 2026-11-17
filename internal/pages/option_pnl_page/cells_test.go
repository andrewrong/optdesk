package option_pnl_page

import (
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/optionattrapi"
)

func TestMoneyText(t *testing.T) {
	t.Parallel()
	cases := []struct{ in, cur, want string }{
		{"-186", "USD", "−$186.00"},
		{"1234.505", "USD", "$1,234.51"},
		{"-1234.505", "USD", "−$1,234.51"},
		{"0.004", "USD", "$0.00"},
		{"-0.004", "USD", "$0.00"},
		{"1234.5", "JPY", "JPY 1,234.50"},
		{"1234567.891", "USD", "$1,234,567.89"},
		{"", "USD", "not stored"},
	}
	for _, c := range cases {
		if got := money(c.in, c.cur); got != c.want {
			t.Errorf("money(%q, %s) = %q, want %q", c.in, c.cur, got, c.want)
		}
	}
}

func TestPercentText(t *testing.T) {
	t.Parallel()
	cases := map[string]string{"0.286": "28.6%", "0.30949": "30.9%", "-0.0005": "−0.1%", "": "not stored"}
	for in, want := range cases {
		if got := percent(in); got != want {
			t.Errorf("percent(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEmptyRefIsNotStored(t *testing.T) {
	t.Parallel()
	l := ivLine("x", optionattrapi.Result{Steps: []optionattrapi.Step{{IVFrom: "0.309", IVTo: "0.286"}}})
	if l == nil || !strings.Contains(l.Text, "stored IBKR IV not stored") || strings.Contains(l.Text, "0.0%") {
		t.Fatalf("iv line = %+v", l)
	}
}

func TestEveryJargonLabelHasHint(t *testing.T) {
	t.Parallel()
	f := figures{Time: "1", MoveCost: "1", Direction: "1", IVRemark: "1", Unsplit: "0", Entry: "1", Exit: "1", Commission: "-1",
		Total: "1", AssignedIntrinsic: "1", LedgerPnL: "1", Difference: "0"}
	for _, l := range splitLines("p", f, "USD", true) {
		if l.Label != "Commissions" && l.Hint == nil {
			t.Errorf("line %s has no hint", l.Label)
		}
	}
}

func TestAssignmentLedgerLine(t *testing.T) {
	t.Parallel()
	f := figures{Total: "78.2", AssignedIntrinsic: "190", LedgerPnL: "268.2"}
	l := ledgerLine("p", f, "USD", true, false)
	if l.Text != "total $78.20 + assignment intrinsic $190.00 = $268.20" || l.Hint == nil || *l.Hint != "total + assignment intrinsic = ledger" {
		t.Fatalf("line = %+v", l)
	}
}
