package optionledger

import (
	"reflect"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

func TestPremiumIncomeReportedApart(t *testing.T) {
	t.Parallel()
	// 2 contracts opened at 1.50 and bought back at 0.40, commissions 2.60.
	r := replay(
		optRow("1", "-2", "300", "-1.30", "O", "20260105", "111", "50", "20260220", "P", ""),
		optRow("2", "2", "-80", "-1.30", "C", "20260110", "111", "50", "20260220", "P", ""),
	)
	p := r.Positions[0]
	if p.PremiumIncome != "220.0000" || p.Commissions != "2.6000" || p.RealizedPnL != "217.4000" || p.Status != "closed" {
		t.Fatalf("%+v", p)
	}
}

func TestExpiredPutRealizedPnl(t *testing.T) {
	t.Parallel()
	checkFixture(t, "expired_put")
}

func markInput(markDay string, sessions []string) compute.LedgerInput {
	in := compute.LedgerInput{
		Alias: "a", AsOf: "2026-01-20",
		Trades: []compute.LedgerTrade{optRow("1", "-1", "100", "-0.65", "O", "20260105", "111", "50", "20260220", "P", "")},
		Marks:  []compute.LedgerMark{{Contract: "111", Day: toDay(markDay), Bid: "0.40", Ask: "0.50"}},
	}
	for _, s := range sessions {
		in.SessionDays = append(in.SessionDays, toDay(s))
	}
	return in
}

func TestMarkMidOfBidAsk(t *testing.T) {
	t.Parallel()
	r := New().Replay(markInput("2026-01-20", []string{"2026-01-20"}), compute.LedgerParams{MarkMaxAgeSessions: 5})
	p := r.Positions[0]
	// 100 - 0.65 - 0.45 x 1 x 100 = 54.35
	if p.MarkStatus != "ok" || p.Mark != "0.4500" || p.MarkDay != "2026-01-20" || p.UnrealizedPnL != "54.3500" || p.RealizedPnL != "" {
		t.Fatalf("%+v", p)
	}
}

func TestMarkOlderThanFiveSessionsIsNoMark(t *testing.T) {
	t.Parallel()
	days := []string{"2026-01-12", "2026-01-13", "2026-01-14", "2026-01-15", "2026-01-16", "2026-01-20"}
	// mark on 01-12: five session days after it (13 14 15 16 20) is still used
	ok := New().Replay(markInput("2026-01-12", days), compute.LedgerParams{MarkMaxAgeSessions: 5}).Positions[0]
	if ok.MarkStatus != "ok" {
		t.Fatalf("age 5: %+v", ok)
	}
	old := New().Replay(markInput("2026-01-12", days), compute.LedgerParams{MarkMaxAgeSessions: 4}).Positions[0] // five sessions after the mark, limit four
	if old.MarkStatus != "no_mark" || old.Mark != "" || old.UnrealizedPnL != "" {
		t.Fatalf("too old: %+v", old)
	}
	none := New().Replay(compute.LedgerInput{Alias: "a", AsOf: "2026-01-20", Trades: markInput("", nil).Trades},
		compute.LedgerParams{MarkMaxAgeSessions: 5}).Positions[0]
	if none.MarkStatus != "no_mark" {
		t.Fatalf("no row: %+v", none)
	}
}

func toDay(s string) contracts.Day { return contracts.Day(s) }

// withFlex adds Flex attributes to a row.
func withFlex(t compute.LedgerTrade, kv ...string) compute.LedgerTrade {
	for i := 0; i+1 < len(kv); i += 2 {
		t.Attributes[kv[i]] = kv[i+1]
	}
	return t
}

// oldFigures blanks the fields that Replay printed before issue 801.
func oldFigures(r compute.LedgerResult) compute.LedgerResult {
	ps := make([]compute.LedgerPosition, len(r.Positions))
	for i, p := range r.Positions {
		p.Right, p.Expiry, p.Strike, p.OpenedOn, p.ClosedOn = "", "", "", "", ""
		p.OpenPrice, p.Fills, p.BrokerRealized = "", nil, ""
		ps[i] = p
	}
	r.Positions = ps
	return r
}

func TestTermsFilled(t *testing.T) {
	t.Parallel()
	r := replay(
		optRow("1", "-1", "100", "-0.65", "O", "20260105", "111", "52.5", "20260220", "P", ""),
		optRow("2", "-1", "90", "-0.65", "O", "20260106", "222", "60", "20260320", "C", ""),
	)
	want := []struct{ right, expiry, strike string }{{"P", "2026-02-20", "52.5"}, {"C", "2026-03-20", "60"}}
	for i, w := range want {
		p := r.Positions[i]
		if p.Right != w.right || p.Expiry != w.expiry || p.Strike != w.strike {
			t.Errorf("position %d: %+v", i, p)
		}
	}
	if r.Positions[0].OpenedOn != "2026-01-05" || r.Positions[0].ClosedOn != "" {
		t.Errorf("days %+v", r.Positions[0])
	}
}

func TestOpenPriceAndFills(t *testing.T) {
	t.Parallel()
	r := replay(
		withFlex(optRow("10", "-2", "300", "-2", "O", "20260105", "111", "50", "20260220", "P", ""), "tradePrice", "1.5"),
		withFlex(optRow("12", "1", "-40", "-0.5", "C", "20260110", "111", "50", "20260220", "P", ""), "tradePrice", "0.4"),
	)
	if len(r.Positions) != 2 {
		t.Fatalf("%+v", r.Positions)
	}
	closed := r.Positions[0]
	wantFills := []compute.LedgerFill{
		{Day: "2026-01-05", Action: "open", Quantity: "-1", Price: "1.5", Commission: "1.0000"},
		{Day: "2026-01-10", Action: "close", Quantity: "1", Price: "0.4", Commission: "0.5000"},
	}
	if closed.OpenPrice != "1.5" || !reflect.DeepEqual(closed.Fills, wantFills) {
		t.Errorf("closed %+v", closed)
	}
	open := r.Positions[1]
	if open.OpenPrice != "1.5" || len(open.Fills) != 1 || open.Fills[0].Quantity != "-1" || open.Fills[0].Commission != "1.0000" {
		t.Errorf("open %+v", open)
	}
}

func TestClosedOnPerStatus(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ name, notes, status string }{
		{"closed", "", "closed"}, {"expired", "Ep", "expired"},
		{"assigned", "A", "assigned"}, {"exercised", "Ex", "exercised"},
	} {
		r := replay(
			optRow("1", "-1", "100", "-0.65", "O", "20260105", "111", "50", "20260220", "P", ""),
			optRow("2", "1", "0", "0", "C", "20260210", "111", "50", "20260220", "P", c.notes),
		)
		p := r.Positions[0]
		if p.Status != c.status || p.ClosedOn != "2026-02-10" {
			t.Errorf("%s: %+v", c.name, p)
		}
	}
	open := replay(optRow("1", "-1", "100", "-0.65", "O", "20260105", "111", "50", "20260220", "P", ""))
	if open.Positions[0].Status != "open" || open.Positions[0].ClosedOn != "" {
		t.Errorf("open %+v", open.Positions[0])
	}
}

func TestBrokerRealizedSummed(t *testing.T) {
	t.Parallel()
	r := replay(
		optRow("1", "-2", "300", "-1", "O", "20260105", "111", "50", "20260220", "P", ""),
		withFlex(optRow("2", "2", "-80", "-1", "C", "20260110", "111", "50", "20260220", "P", ""), "fifoPnlRealized", "219.5"),
	)
	if got := r.Positions[0].BrokerRealized; got != "219.5000" {
		t.Fatalf("broker %q", got)
	}
	// A close split over two lots shares the figure by quantity.
	s := replay(
		optRow("1", "-1", "100", "-1", "O", "20260105", "111", "50", "20260220", "P", ""),
		optRow("2", "-1", "100", "-1", "O", "20260106", "111", "50", "20260220", "P", ""),
		withFlex(optRow("3", "2", "-80", "-1", "C", "20260110", "111", "50", "20260220", "P", ""), "fifoPnlRealized", "-10"),
	)
	if s.Positions[0].BrokerRealized != "-5.0000" || s.Positions[1].BrokerRealized != "-5.0000" {
		t.Fatalf("split %+v", s.Positions)
	}
}

func TestBrokerRealizedEmptyWhenAbsent(t *testing.T) {
	t.Parallel()
	r := replay(
		optRow("1", "-1", "100", "-1", "O", "20260105", "111", "50", "20260220", "P", ""),
		optRow("2", "1", "-80", "-1", "C", "20260110", "111", "50", "20260220", "P", ""),
		withFlex(optRow("3", "-1", "100", "-1", "O", "20260105", "333", "50", "20260220", "P", ""), "fifoPnlRealized", "5"),
	)
	for _, p := range r.Positions {
		if p.BrokerRealized != "" {
			t.Errorf("%s: %q", p.PositionID, p.BrokerRealized)
		}
	}
}

func TestExistingFiguresUnchanged(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"close_without_open", "early_assignment", "expired_put", "two_currencies", "wheel_one_cycle"} {
		f := loadFixture(t, name)
		got := oldFigures(New().Replay(f.Input, f.Params))
		if !reflect.DeepEqual(got, f.Expected) {
			t.Errorf("%s: an existing figure changed", name)
		}
	}
}
