package optionstruct

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
)

func single(t *testing.T, p compute.LedgerPosition, cycles []compute.LedgerCycle) compute.Structure {
	t.Helper()
	res := evaluate([]compute.LedgerPosition{p}, cycles)
	if len(res.Structures) != 1 {
		t.Fatalf("structures = %v", kinds(res.Structures))
	}
	return res.Structures[0]
}

func wantFigure(t *testing.T, name string, got compute.FigureValue, want compute.FigureValue) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %+v, want %+v", name, got, want)
	}
}

func TestMaxLossShortPut(t *testing.T) {
	t.Parallel()
	s := single(t, pos("s1", "P", "90", "-1", "3.00"), nil)
	wantFigure(t, "max loss", s.Figures.MaxLoss, compute.FigureValue{Value: "-8700.0000"})
	wantFigure(t, "max gain", s.Figures.MaxGain, compute.FigureValue{Value: "300.0000"})
	two := single(t, pos("s1", "P", "90", "-2", "3.00"), nil)
	wantFigure(t, "two contracts", two.Figures.MaxLoss, compute.FigureValue{Value: "-17400.0000"})
}

func TestMaxLossSpread(t *testing.T) {
	t.Parallel()
	res := evaluate([]compute.LedgerPosition{pos("s1", "P", "50", "-1", "1.50"), pos("b1", "P", "45", "1", "0.50")}, nil)
	f := res.Structures[0].Figures
	wantFigure(t, "max loss", f.MaxLoss, compute.FigureValue{Value: "-400.0000"})
	wantFigure(t, "max gain", f.MaxGain, compute.FigureValue{Value: "100.0000"})
	call := evaluate([]compute.LedgerPosition{pos("s1", "C", "100", "-1", "2.00"), pos("b1", "C", "105", "1", "0.80")}, nil)
	wantFigure(t, "call max loss", call.Structures[0].Figures.MaxLoss, compute.FigureValue{Value: "-380.0000"})
}

func TestMaxGainCoveredCall(t *testing.T) {
	t.Parallel()
	cycles := []compute.LedgerCycle{{Underlying: "AAPL", Currency: "USD", Shares: "100", CostBasisPerShare: "80"}}
	s := single(t, pos("c1", "C", "90", "-1", "2.00"), cycles)
	wantFigure(t, "max gain", s.Figures.MaxGain, compute.FigureValue{Value: "1200.0000"}) // (90-80)*100 + 200
	wantFigure(t, "max loss", s.Figures.MaxLoss, compute.FigureValue{Gap: "no_rule"})
	cycles[0].CostBasisPerShare = ""
	s = single(t, pos("c1", "C", "90", "-1", "2.00"), cycles)
	wantFigure(t, "no basis", s.Figures.MaxGain, compute.FigureValue{Gap: "missing_input"})
}

func TestNakedCallUnbounded(t *testing.T) {
	t.Parallel()
	s := single(t, pos("c1", "C", "90", "-1", "2.00"), nil)
	wantFigure(t, "max loss", s.Figures.MaxLoss, compute.FigureValue{Gap: "no_rule"})
	wantFigure(t, "max gain", s.Figures.MaxGain, compute.FigureValue{Value: "200.0000"})
	lc := single(t, pos("l1", "C", "90", "1", "2.00"), nil)
	wantFigure(t, "long call max gain", lc.Figures.MaxGain, compute.FigureValue{Gap: "no_rule"})
	wantFigure(t, "long call max loss", lc.Figures.MaxLoss, compute.FigureValue{Value: "-200.0000"})
}

func TestBreakEven(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		ps   []compute.LedgerPosition
		want string
	}{
		{"short put", []compute.LedgerPosition{pos("s", "P", "90", "-1", "3.00")}, "87.0000"},
		{"long put", []compute.LedgerPosition{pos("l", "P", "90", "1", "3.25")}, "86.7500"},
		{"naked call", []compute.LedgerPosition{pos("c", "C", "90", "-1", "2.00")}, "92.0000"},
		{"put spread", []compute.LedgerPosition{pos("s", "P", "50", "-1", "1.50"), pos("b", "P", "45", "1", "0.50")}, "49.0000"},
		{"call spread", []compute.LedgerPosition{pos("s", "C", "100", "-1", "2.00"), pos("b", "C", "105", "1", "0.80")}, "101.2000"},
	} {
		res := evaluate(tc.ps, nil)
		if len(res.Structures) != 1 {
			t.Fatalf("%s: %v", tc.name, kinds(res.Structures))
		}
		s := res.Structures[0]
		wantFigure(t, tc.name, s.Figures.BreakEven, compute.FigureValue{Value: tc.want})
		last := s.Levels[len(s.Levels)-1]
		if last.Name != "break_even" || last.Price != tc.want || last.Distance != "" {
			t.Errorf("%s: last level = %+v", tc.name, last)
		}
	}
}

func TestJPYIsUnsupported(t *testing.T) {
	t.Parallel()
	p := pos("s1", "P", "90", "-1", "3.00")
	p.Currency = "JPY"
	s := single(t, p, nil)
	if s.Kind != "short_put" {
		t.Errorf("kind = %s, want short_put", s.Kind)
	}
	want := compute.FigureValue{Gap: "unsupported_currency"}
	wantFigure(t, "max loss", s.Figures.MaxLoss, want)
	wantFigure(t, "max gain", s.Figures.MaxGain, want)
	wantFigure(t, "break even", s.Figures.BreakEven, want)
	if len(s.Payoff) != 0 {
		t.Errorf("payoff = %v, want none", s.Payoff)
	}
}

// unitOf builds a unit of the given kind from positions, legs ordered as recognise does.
func unitOf(t *testing.T, kind string, ps ...compute.LedgerPosition) compute.Structure {
	t.Helper()
	u := unit{kind: kind}
	for _, p := range ps {
		l, ok := parseLeg(p)
		if !ok {
			t.Fatalf("leg %s does not parse", p.PositionID)
		}
		u.legs = append(u.legs, l)
	}
	sortLegs(u.legs)
	return build("acct", u)
}

func wantFigures(t *testing.T, s compute.Structure, loss, gain, be compute.FigureValue) {
	t.Helper()
	wantFigure(t, "max loss", s.Figures.MaxLoss, loss)
	wantFigure(t, "max gain", s.Figures.MaxGain, gain)
	wantFigure(t, "break even", s.Figures.BreakEven, be)
}

func TestPayoffFiguresCreditVertical(t *testing.T) {
	t.Parallel()
	put := unitOf(t, kindPutSpread, pos("s1", "P", "50", "-1", "1.50"), pos("b1", "P", "45", "1", "0.50"))
	wantFigures(t, put, compute.FigureValue{Value: "-400.0000"}, compute.FigureValue{Value: "100.0000"}, compute.FigureValue{Value: "49.0000"})
	call := unitOf(t, kindCallSpread, pos("s1", "C", "100", "-1", "2.00"), pos("b1", "C", "105", "1", "0.80"))
	wantFigures(t, call, compute.FigureValue{Value: "-380.0000"}, compute.FigureValue{Value: "120.0000"}, compute.FigureValue{Value: "101.2000"})
}

func TestPayoffFiguresFourLegs(t *testing.T) {
	t.Parallel()
	s := unitOf(t, "iron_condor",
		pos("a", "P", "90", "1", "0.50"), pos("b", "P", "95", "-1", "1.50"),
		pos("c", "C", "105", "-1", "1.50"), pos("d", "C", "110", "1", "0.50"))
	wantFigures(t, s, compute.FigureValue{Value: "-300.0000"}, compute.FigureValue{Value: "200.0000"}, noRule)
}

func TestPayoffFiguresSlices(t *testing.T) {
	t.Parallel()
	one := unitOf(t, kindPutSpread, pos("s1", "P", "50", "-3", "1.50"), pos("b1", "P", "45", "3", "0.50"))
	three := unitOf(t, kindPutSpread, pos("s1", "P", "50", "-1", "1.50"), pos("s2", "P", "50", "-1", "1.50"),
		pos("s3", "P", "50", "-1", "1.50"), pos("b1", "P", "45", "3", "0.50"))
	wantFigures(t, three, one.Figures.MaxLoss, one.Figures.MaxGain, one.Figures.BreakEven)
	wantFigure(t, "max loss", three.Figures.MaxLoss, compute.FigureValue{Value: "-1200.0000"})
}

func TestPayoffFiguresUnbounded(t *testing.T) {
	t.Parallel()
	long := unitOf(t, "long_straddle", pos("p", "P", "100", "1", "3.00"), pos("c", "C", "100", "1", "2.00"))
	wantFigure(t, "straddle gain", long.Figures.MaxGain, noRule)
	wantFigure(t, "straddle loss", long.Figures.MaxLoss, compute.FigureValue{Value: "-500.0000"})
	short := unitOf(t, "short_strangle", pos("p", "P", "95", "-1", "1.00"), pos("c", "C", "105", "-1", "1.00"))
	wantFigure(t, "strangle loss", short.Figures.MaxLoss, noRule)
	wantFigure(t, "strangle gain", short.Figures.MaxGain, compute.FigureValue{Value: "200.0000"})
}

func TestPayoffFiguresOtherKeepsNoRule(t *testing.T) {
	t.Parallel()
	s := unitOf(t, kindOther, pos("s1", "P", "50", "-1", "1.50"), pos("b1", "P", "45", "1", "0.50"))
	wantFigures(t, s, noRule, noRule, noRule)
	if len(s.Payoff) != 0 {
		t.Errorf("payoff = %v, want none", s.Payoff)
	}
}

func TestPayoffFiguresCalendar(t *testing.T) {
	t.Parallel()
	far := pos("b1", "P", "50", "1", "2.00")
	far.Expiry = "2027-01-15"
	s := unitOf(t, "calendar_spread", pos("s1", "P", "50", "-1", "1.50"), far)
	wantFigures(t, s, noRule, noRule, noRule)
	if len(s.Payoff) != 0 {
		t.Errorf("payoff = %v, want none", s.Payoff)
	}
}
