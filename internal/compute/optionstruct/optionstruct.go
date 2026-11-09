package optionstruct

import (
	"cmp"
	"slices"

	"github.com/andrewrong/optdesk/internal/compute"
)

type structures struct{}

// New returns the rule: exact (math/big.Rat), pure, no clock and no I/O.
func New() compute.OptionStructures { return structures{} }

// The gaps this package sets.
const (
	gapNoRule      = "no_rule"
	gapNoData      = "no_data"
	gapMissing     = "missing_input"
	gapUnsupported = "unsupported_currency"
)

var noData = compute.FigureValue{Gap: gapNoData}

// Evaluate recognises the open positions of the input (Decision 4), sets the figures that
// need no mark (Decision 6), the payoff (Decision 8) and the levels, fills the marked figures,
// the Greeks, the compare row and Decide (Decisions 6, 7, 13) from the quotes of the input,
// and orders the structures by Decision 12: the most negative max loss first, gaps last, then
// ID. The capture diff is Decision 2. A quote set that is empty gives FigureValue{Gap: "no_data"}.
func (structures) Evaluate(in compute.StructureInput, params compute.StructureParams) compute.StructureResult {
	var res compute.StructureResult
	m := marker{quotes: in.Quotes, params: params, asOf: in.AsOf, days: in.SessionDays}
	units := recognise(in)
	for _, u := range units {
		s := build(in.Alias, u)
		m.fill(&s, u)
		if u.kind == kindOther {
			res.Other = append(res.Other, s)
		} else {
			res.Structures = append(res.Structures, s)
		}
	}
	slices.SortStableFunc(res.Structures, compareStructures)
	slices.SortStableFunc(res.Other, func(a, b compute.Structure) int { return cmp.Compare(a.ID, b.ID) })
	res.Diff = captureDiff(in, units)
	return res
}

func build(alias string, u unit) compute.Structure {
	s := compute.Structure{
		ID:         alias + ":" + u.legs[0].p.PositionID,
		Alias:      alias,
		Underlying: u.legs[0].p.Underlying,
		Currency:   u.legs[0].p.Currency,
		Kind:       u.kind,
	}
	for _, l := range u.legs {
		s.Legs = append(s.Legs, compute.StructureLeg{
			PositionID: l.p.PositionID, Right: l.p.Right, Strike: l.p.Strike, Expiry: l.p.Expiry,
			Quantity: l.p.Quantity, Multiplier: l.p.Multiplier, OpenPrice: l.p.OpenPrice,
			MarkStatus: l.p.MarkStatus,
		})
	}
	f := risk(u)
	if s.Currency != "USD" {
		unsupported := compute.FigureValue{Gap: gapUnsupported}
		f = figures{unsupported, unsupported, unsupported}
	} else {
		s.Payoff = payoff(u)
	}
	s.Figures = compute.StructureFigures{
		MaxLoss: f.maxLoss, MaxGain: f.maxGain, BreakEven: f.breakEven,
		UnrealizedPnL: noData, EarnedShare: noData, ExpiryDistance: noData,
		CloseCost: noData, RollCredit: noData, RollNet: noData,
		RecentCommission: noData, CashAfter: noData,
	}
	s.Greeks = compute.GreekSet{Delta: noData, Gamma: noData, Theta: noData, Vega: noData}
	s.Levels = levels(u, f)
	return s
}

// compareStructures orders by max loss ascending (the most negative first), gaps last, then ID.
func compareStructures(a, b compute.Structure) int {
	av, aok := parseDec(a.Figures.MaxLoss.Value)
	bv, bok := parseDec(b.Figures.MaxLoss.Value)
	switch {
	case aok && bok:
		if c := av.Cmp(bv); c != 0 {
			return c
		}
	case aok:
		return -1
	case bok:
		return 1
	}
	return cmp.Compare(a.ID, b.ID)
}
