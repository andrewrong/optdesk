package option_pnl_page

import (
	"regexp"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
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
		{"1234.5", "JPY", "¥1,234.50"},
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
	if l.Text != "total $78.20 + assignment intrinsic $190.00 = $268.20" || l.Hint == nil || *l.Hint != "合计加行权内在价值等于账本" {
		t.Fatalf("line = %+v", l)
	}
}

func TestLateEntryHintOnLotAndBook(t *testing.T) {
	t.Parallel()
	mk := func(late bool) outcome {
		return outcome{lot: pnlLot{pos: compute.LedgerPosition{Currency: "USD", Status: "closed"}},
			res: optionattrapi.Result{Currency: "USD", Entry: "1", LateEntry: late, LedgerPnL: "1"}}
	}
	outs := []outcome{mk(true), mk(true), mk(false), mk(false), mk(false)}
	books := bookCards("a", outs, []optionattrapi.Book{{Currency: "USD", Lots: 5, Entry: "1", Total: "1"}})
	var entry *line
	for i, l := range books[0].Lines {
		if l.Label == "Entry vs mid" {
			entry = &books[0].Lines[i]
		}
	}
	if entry == nil || entry.Hint == nil || !strings.Contains(*entry.Hint, "5 个仓位中有 2 个在第一条存档报价之前成交（报价从 2026-11-07 起存档）") {
		t.Fatalf("book entry line = %+v", entry)
	}
	entryOf := func(o outcome) line {
		card, err := lotCard(Config{}, "p", "a", o, false)
		if err != nil {
			t.Fatal(err)
		}
		for _, l := range card.Lines {
			if l.Label == "Entry vs mid" {
				return l
			}
		}
		t.Fatal("no entry line")
		return line{}
	}
	if l := entryOf(mk(true)); l.Hint == nil || !strings.Contains(*l.Hint, lateEntryLotZh) {
		t.Errorf("late lot hint = %v", l.Hint)
	}
	if l := entryOf(mk(false)); l.Hint == nil || strings.Contains(*l.Hint, lateEntryLotZh) {
		t.Errorf("plain lot hint = %v", l.Hint)
	}
}

func TestBookLateEntryHintCountsBookLots(t *testing.T) {
	t.Parallel()
	mk := func(late, gap bool) outcome {
		o := outcome{lot: pnlLot{pos: compute.LedgerPosition{Currency: "USD", Status: "closed"}},
			res: optionattrapi.Result{Currency: "USD", Entry: "1", LateEntry: late, LedgerPnL: "1"}}
		if gap {
			o.res.Entry = ""
			o.reason = "no quotes" // a gap lot, which the Booker does not count
		}
		return o
	}
	outs := []outcome{mk(true, false), mk(false, false), mk(false, true)}
	books := bookCards("a", outs, []optionattrapi.Book{{Currency: "USD", Lots: 2, Entry: "1", Total: "1"}})
	for _, l := range books[0].Lines {
		if l.Label == "Entry vs mid" && (l.Hint == nil || !strings.Contains(*l.Hint, "2 个仓位中有 1 个")) {
			t.Fatalf("hint = %v, want M = book lots 2", l.Hint)
		}
	}
}

func TestCardHintsAreChinese(t *testing.T) {
	t.Parallel()
	old := []string{
		"what you earned for selling insurance", "the option loses value as days pass", "convexity:",
		"value moved to the shares", "ledger minus total minus assignment intrinsic", "total + assignment intrinsic = ledger",
		"the bet that the stock does not fall", "implied volatility,", "steps whose IV", "fill price", "the option side", "the ledger's figure",
	}
	o := outcome{lot: pnlLot{pos: compute.LedgerPosition{Currency: "USD", Status: "closed"}},
		res: optionattrapi.Result{Currency: "USD", Entry: "1", Exit: "1", AssignedIntrinsic: "2", LedgerPnL: "3", Unsplit: "1", UnsplitSteps: 1, Time: "1", MoveCost: "1", Direction: "1", IVRemark: "1"}}
	card, err := lotCard(Config{}, "p", "a", o, true)
	if err != nil {
		t.Fatal(err)
	}
	lines := append(append(card.Lines, card.Recon...), ledgerLine("p", figures{Total: "1", AssignedIntrinsic: "2", LedgerPnL: "3"}, "USD", true, false))
	hinted := 0
	for _, l := range lines {
		if l.Hint == nil {
			continue
		}
		hinted++
		if !regexp.MustCompile(`\p{Han}`).MatchString(*l.Hint) {
			t.Errorf("line %s hint %q has no Chinese character", l.ID, *l.Hint)
		}
		for _, e := range old {
			if strings.Contains(*l.Hint, e) {
				t.Errorf("line %s hint %q holds old English %q", l.ID, *l.Hint, e)
			}
		}
	}
	if hinted == 0 {
		t.Fatal("no hinted line")
	}
}
