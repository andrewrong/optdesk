package optionstruct

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
)

func TestGreeksSignedSum(t *testing.T) {
	t.Parallel()
	q := quote("P", "90", "2026-12-18", "1.00", "1.20")
	q.Delta, q.Gamma, q.Theta, q.Vega = "0.25", "0.01", "-0.05", "0.10"
	s := firstOf(t, evalMarked([]compute.LedgerPosition{shortPut()}, []compute.StructureQuote{q}, baseParams()))
	g := s.Greeks
	if g.Delta.Value != "-25.0000" || g.Gamma.Value != "-1.0000" || g.Theta.Value != "5.0000" ||
		g.Vega.Value != "-10.0000" || g.Day != "2026-11-06" {
		t.Errorf("greeks = %+v", g)
	}
	// a spread adds the long leg with its own sign: 1 x 100 x 0.10
	b := quote("P", "85", "2026-12-18", "0.30", "0.50")
	b.Delta = "0.10"
	sp := firstOf(t, evalMarked([]compute.LedgerPosition{shortPut(), pos("b1", "P", "85", "1", "0.50")},
		[]compute.StructureQuote{q, b}, baseParams()))
	if sp.Greeks.Delta.Value != "-15.0000" {
		t.Errorf("spread delta = %+v, want -15.0000", sp.Greeks.Delta)
	}
}

func TestGreekMissingOneLegIsGap(t *testing.T) {
	t.Parallel()
	q := quote("P", "90", "2026-12-18", "1.00", "1.20")
	q.Delta, q.Gamma, q.Theta, q.Vega = "0.25", "0.01", "-0.05", "0.10"
	s := firstOf(t, evalMarked([]compute.LedgerPosition{shortPut(), pos("b1", "P", "85", "1", "0.50")},
		[]compute.StructureQuote{q}, baseParams()))
	want := compute.FigureValue{Gap: "missing_quote"}
	g := s.Greeks
	if g.Delta != want || g.Gamma != want || g.Theta != want || g.Vega != want {
		t.Errorf("greeks = %+v, want every one a gap, never a partial sum", g)
	}
	q.Vega = ""
	one := firstOf(t, evalMarked([]compute.LedgerPosition{shortPut()}, []compute.StructureQuote{q}, baseParams()))
	if one.Greeks.Vega != want || one.Greeks.Delta.Value != "-25.0000" {
		t.Errorf("a missing vega: %+v", one.Greeks)
	}
}
