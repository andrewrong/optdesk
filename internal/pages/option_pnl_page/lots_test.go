package option_pnl_page

import (
	"errors"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/optionattrapi"
	"github.com/andrewrong/optdesk/internal/store/optionday"
)

func TestMaxRateAgeDaysPassed(t *testing.T) {
	t.Parallel()
	r := newRig()
	r.cfg.MaxRateAgeDays = 9
	r.replay.positions = []compute.LedgerPosition{lotPos("p1", "AAA", "101", "90.00", "open")}
	r.days.rows[101] = []optionday.Day{dayRow(101, "2026-11-17", "1.20", "1.30", "95")}
	r.attr.results["90.00"] = optionattrapi.Result{Currency: "USD", NoQuotes: true, LedgerPnL: "100"}
	r.build(t)
	in := r.attr.ins[0]
	if in.MaxRateAgeDays != 9 || len(in.Rates) != 1 || in.Rates[0].Percent != "5.25" || in.Rates[0].Day != "2026-11-14" {
		t.Fatalf("input = %+v", in)
	}
}

func TestFillsOnlyOpenAndClose(t *testing.T) {
	t.Parallel()
	r := newRig()
	p := lotPos("p1", "BBB", "202", "38.00", "assigned")
	p.Fills = append(p.Fills, compute.LedgerFill{Day: "2026-11-10", Action: "close", Quantity: "1", Price: "0.5", Commission: "0.6"},
		compute.LedgerFill{Day: "2026-11-13", Action: "assign", Quantity: "1"}, compute.LedgerFill{Day: "2026-11-13", Action: "expire", Quantity: "1"})
	r.replay.positions = []compute.LedgerPosition{p}
	r.bars.close[900] = "36.10"
	r.attr.results["38.00"] = optionattrapi.Result{Currency: "USD", NoQuotes: true, LedgerPnL: "50"}
	r.build(t)
	f := r.attr.ins[0].Lot.Fills
	if len(f) != 2 || f[0].Side != "open" || f[1].Side != "close" || f[1].Price != "0.5" || f[1].Commission != "0.6" {
		t.Fatalf("fills = %+v", f)
	}
	if in := r.attr.ins[0]; in.LedgerPnL != "50" || in.Lot.EndDay != "2026-11-13" || in.Lot.End != "assigned" {
		t.Fatalf("input = %+v", in)
	}
}

func TestRowsPassedUnfiltered(t *testing.T) {
	t.Parallel()
	r := newRig()
	r.replay.positions = []compute.LedgerPosition{lotPos("p1", "AAA", "101", "90.00", "open")}
	r.days.rows[101] = []optionday.Day{dayRow(101, "2026-11-16", "1.10", "0", "94"), dayRow(101, "2026-11-17", "1.20", "1.30", "95")}
	r.attr.results["90.00"] = optionattrapi.Result{Currency: "USD", NoQuotes: true, LedgerPnL: "100"}
	r.build(t)
	q := r.attr.ins[0].Quotes
	if len(q) != 2 || q[0].Ask != "0" || q[0].Day != "2026-11-16" || q[1].Ref.IV != "0.289" {
		t.Fatalf("quotes = %+v", q)
	}
}

func TestRuleFailingRowIsLotGap(t *testing.T) {
	t.Parallel()
	r := newRig()
	r.replay.positions = []compute.LedgerPosition{lotPos("p1", "AAA", "101", "90.00", "open")}
	// the only row has ask 0: no mark is made, the replay answers no_mark
	r.days.rows[101] = []optionday.Day{dayRow(101, "2026-11-17", "1.20", "0", "95")}
	vm := r.build(t)
	g := vm.Accounts[0].Lots[0].Gap
	if g == nil || g.Text != "no stored quote of AAA within 3 sessions: no split" || len(vm.Accounts[0].Lots[0].Lines) != 0 {
		t.Fatalf("lot = %+v", vm.Accounts[0].Lots[0])
	}
	if len(r.attr.ins) != 0 {
		t.Fatal("Attribute must not be called for a lot with a reason")
	}
}

func TestStaleValidRowIsLotGap(t *testing.T) {
	t.Parallel()
	r := newRig()
	r.replay.positions = []compute.LedgerPosition{lotPos("p1", "AAA", "101", "90.00", "open")}
	r.days.rows[101] = []optionday.Day{dayRow(101, "2026-11-10", "1.20", "1.30", "95")} // valid, 5 sessions old
	vm := r.build(t)
	if g := vm.Accounts[0].Lots[0].Gap; g == nil || g.Text != "no stored quote of AAA within 3 sessions: no split" {
		t.Fatalf("stale valid row must give the no_mark gap; lot = %+v", vm.Accounts[0].Lots[0])
	}
	if len(r.attr.ins) != 0 {
		t.Fatal("Attribute must not be called for a lot with a reason")
	}
}

func TestFreshValidRowIsNotLotGap(t *testing.T) {
	t.Parallel()
	r := newRig()
	r.replay.positions = []compute.LedgerPosition{lotPos("p1", "AAA", "101", "90.00", "open")}
	r.days.rows[101] = []optionday.Day{dayRow(101, "2026-11-13", "1.20", "1.30", "95")} // 2 sessions old
	r.attr.results["90.00"] = optionattrapi.Result{Currency: "USD", NoQuotes: true, LedgerPnL: "100"}
	vm := r.build(t)
	if g := vm.Accounts[0].Lots[0].Gap; g != nil {
		t.Fatalf("a row within 3 sessions must give a mark; gap = %+v", g)
	}
}

func TestBadInputBecomesLotGap(t *testing.T) {
	t.Parallel()
	r := newRig()
	r.replay.positions = []compute.LedgerPosition{lotPos("p1", "AAA", "101", "90.00", "open")}
	r.days.rows[101] = []optionday.Day{dayRow(101, "2026-11-17", "1.20", "1.30", "95")}
	r.attr.err = optionattrapi.ErrBadInput
	vm := r.build(t)
	g := vm.Accounts[0].Lots[0].Gap
	if g == nil || !strings.HasPrefix(g.Text, "input refused: ") || len(vm.Accounts[0].Lots[0].Lines) != 0 {
		t.Fatalf("lot = %+v", vm.Accounts[0].Lots[0])
	}
	r.attr.err = errors.New("pricer broke")
	if _, err := Build(t.Context(), r.cfg, nil, buildDay, buildAt); err == nil || !strings.Contains(err.Error(), "pricer broke") {
		t.Fatalf("err = %v", err)
	}
}

func TestFillQuantityIsUnsigned(t *testing.T) {
	t.Parallel()
	r := newRig()
	p := lotPos("p1", "BBB", "202", "38.00", "closed")
	p.Fills = []compute.LedgerFill{{Day: "2026-11-03", Action: "open", Quantity: "-2", Price: "1.80", Commission: "1.30"},
		{Day: "2026-11-10", Action: "close", Quantity: "2", Price: "0.5", Commission: "0.6"}}
	r.replay.positions = []compute.LedgerPosition{p}
	r.attr.results["38.00"] = optionattrapi.Result{Currency: "USD", NoQuotes: true, LedgerPnL: "50"}
	r.build(t)
	f := r.attr.ins[0].Lot.Fills
	if len(f) != 2 || f[0].Quantity != "2" || f[1].Quantity != "2" {
		t.Fatalf("fills = %+v", f)
	}
}
