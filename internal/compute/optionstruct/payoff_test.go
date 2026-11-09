package optionstruct

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
)

func TestPayoffCorners(t *testing.T) {
	t.Parallel()
	// Put spread 50/45, credit 1.00: top strike 50, margin 10, so the corners are 35, 45, 50, 60.
	res := evaluate([]compute.LedgerPosition{pos("s1", "P", "50", "-1", "1.50"), pos("b1", "P", "45", "1", "0.50")}, nil)
	want := []compute.PayoffPoint{
		{Price: "35.0000", PnL: "-400.0000"},
		{Price: "45.0000", PnL: "-400.0000"},
		{Price: "50.0000", PnL: "100.0000"},
		{Price: "60.0000", PnL: "100.0000"},
	}
	got := res.Structures[0].Payoff
	if len(got) != len(want) {
		t.Fatalf("payoff = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("point %d = %v, want %v", i, got[i], want[i])
		}
	}
	// Short put 90 at 3.00: the low corner is 72, the high 108.
	sp := evaluate([]compute.LedgerPosition{pos("s", "P", "90", "-1", "3.00")}, nil).Structures[0].Payoff
	if len(sp) != 3 || sp[0] != (compute.PayoffPoint{Price: "72.0000", PnL: "-1500.0000"}) ||
		sp[1] != (compute.PayoffPoint{Price: "90.0000", PnL: "300.0000"}) {
		t.Errorf("short put payoff = %v", sp)
	}
}

func TestPayoffCoveredCallAndLevels(t *testing.T) {
	t.Parallel()
	cycles := []compute.LedgerCycle{{Underlying: "AAPL", Currency: "USD", Shares: "100", CostBasisPerShare: "80"}}
	s := single(t, pos("c1", "C", "90", "-1", "2.00"), cycles)
	last := s.Payoff[len(s.Payoff)-1]
	if last.Price != "108.0000" || last.PnL != "1200.0000" {
		t.Errorf("last point = %v, want the max gain at 108", last)
	}
	if len(s.Levels) != 1 || s.Levels[0].Name != "strike" || s.Levels[0].Price != "90" {
		t.Errorf("levels = %v", s.Levels)
	}
	sp := evaluate([]compute.LedgerPosition{pos("s1", "P", "50", "-1", "1.50"), pos("b1", "P", "45", "1", "0.50")}, nil)
	names := []string{}
	for _, l := range sp.Structures[0].Levels {
		names = append(names, l.Name)
	}
	if len(names) != 3 || names[0] != "long_strike" || names[1] != "short_strike" || names[2] != "break_even" {
		t.Errorf("level names = %v", names)
	}
}
