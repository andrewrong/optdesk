package optionstruct

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
)

func TestCloseCostIsMidOfShortLeg(t *testing.T) {
	t.Parallel()
	s := firstOf(t, evalMarked([]compute.LedgerPosition{shortPut()},
		[]compute.StructureQuote{quote("P", "90", "2026-12-18", "0.55", "0.65")}, baseParams()))
	if s.Figures.CloseCost.Value != "60.0000" { // mid 0.60 x 100 x 1
		t.Errorf("close cost = %+v, want 60.0000", s.Figures.CloseCost)
	}
	two := firstOf(t, evalMarked([]compute.LedgerPosition{pos("s1", "P", "90", "-2", "1.50")},
		[]compute.StructureQuote{quote("P", "90", "2026-12-18", "0.55", "0.65")}, baseParams()))
	if two.Figures.CloseCost.Value != "120.0000" {
		t.Errorf("two contracts = %+v, want 120.0000", two.Figures.CloseCost)
	}
}

func TestRollCreditNextThirdFriday(t *testing.T) {
	t.Parallel()
	for expiry, want := range map[string]string{
		"2026-12-18": "2027-01-15", // the third Friday itself: the next month's
		"2026-12-11": "2026-12-18", // a weekly before the third Friday
		"2026-12-31": "2027-01-15",
		"2027-12-17": "2028-01-21", // across a year
	} {
		if got := nextMonthly(expiry); got != want {
			t.Errorf("nextMonthly(%s) = %s, want %s", expiry, got, want)
		}
	}
	qs := quotesOf(t, "roll.json", "roll") // the decoy bid 9.00 is at 2027-02-19
	s := firstOf(t, evalMarked([]compute.LedgerPosition{shortPut()}, qs, baseParams()))
	if s.Figures.RollCredit.Value != "110.0000" {
		t.Errorf("roll credit = %+v, want 110.0000 (the bid at 2027-01-15)", s.Figures.RollCredit)
	}
}

func TestRollNoQuoteIsGap(t *testing.T) {
	t.Parallel()
	s := firstOf(t, evalMarked([]compute.LedgerPosition{shortPut()}, quotesOf(t, "roll.json", "no_roll"), baseParams()))
	if s.Figures.RollCredit != noData || s.Figures.RollNet != noData {
		t.Errorf("credit %+v net %+v, want gap no_data", s.Figures.RollCredit, s.Figures.RollNet)
	}
	if s.Figures.CloseCost.Value != "60.0000" {
		t.Errorf("close cost = %+v, want it kept", s.Figures.CloseCost)
	}
	zero := quotesOf(t, "roll.json", "roll")
	zero[1].Bid = "0"
	z := firstOf(t, evalMarked([]compute.LedgerPosition{shortPut()}, zero, baseParams()))
	if z.Figures.RollCredit != noData {
		t.Errorf("zero bid credit = %+v, want gap no_data", z.Figures.RollCredit)
	}
}

func TestRollNetIsCreditMinusClose(t *testing.T) {
	t.Parallel()
	s := firstOf(t, evalMarked([]compute.LedgerPosition{shortPut()}, quotesOf(t, "roll.json", "roll"), baseParams()))
	f := s.Figures
	if f.CloseCost.Value != "60.0000" || f.RollCredit.Value != "110.0000" || f.RollNet.Value != "50.0000" {
		t.Errorf("close %+v credit %+v net %+v", f.CloseCost, f.RollCredit, f.RollNet)
	}
}

func TestCashAfterIsPremiumMinusCommissionMinusClose(t *testing.T) {
	t.Parallel()
	p := shortPut()
	p.PremiumIncome, p.Commissions = "150.0000", "1.3000"
	qs := []compute.StructureQuote{quote("P", "90", "2026-12-18", "0.55", "0.65")}
	s := firstOf(t, evalMarked([]compute.LedgerPosition{p}, qs, baseParams()))
	if s.Figures.CashAfter.Value != "88.7000" { // 150 - 1.3 - 60
		t.Errorf("cash after = %+v, want 88.7000", s.Figures.CashAfter)
	}
	q := qs[0]
	q.Bid = "0"
	g := firstOf(t, evalMarked([]compute.LedgerPosition{p}, []compute.StructureQuote{q}, baseParams()))
	if g.Figures.CashAfter != (compute.FigureValue{Gap: "missing_quote"}) {
		t.Errorf("no close cost: cash after = %+v, want its gap", g.Figures.CashAfter)
	}
}
