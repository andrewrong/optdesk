package optionstruct

import (
	"math/big"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// fillGreeks sets the Greek of the structure as the signed sum of quantity times multiplier
// times the IBKR model value of each leg (Decision 7). A leg with no quote, an old quote or
// no value makes that Greek a gap; nothing is priced or solved here. Day is the oldest quote
// day used.
func (m marker) fillGreeks(s *compute.Structure, u unit) {
	if u.kind == kindOther {
		s.Greeks = compute.GreekSet{Delta: noRule, Gamma: noRule, Theta: noRule, Vega: noRule}
		return
	}
	quotes := make([]*compute.StructureQuote, len(u.legs))
	gap := ""
	var day contracts.Day
	for i, l := range u.legs {
		q, g := m.quoteOf(l)
		if g == "" && m.stale(q.Day) {
			g = gapStale
		}
		if g != "" {
			gap = g
			continue
		}
		quotes[i] = q
		if day == "" || q.Day < day {
			day = q.Day
		}
	}
	pick := func(name func(*compute.StructureQuote) string) compute.FigureValue {
		if gap != "" {
			return compute.FigureValue{Gap: gap}
		}
		sum := new(big.Rat)
		for i, l := range u.legs {
			v, ok := parseDec(name(quotes[i]))
			if !ok {
				return compute.FigureValue{Gap: gapMissingQuote}
			}
			sum = add(sum, mul(mul(l.qty, l.mult), v))
		}
		return val(sum)
	}
	s.Greeks = compute.GreekSet{
		Delta: pick(func(q *compute.StructureQuote) string { return q.Delta }),
		Gamma: pick(func(q *compute.StructureQuote) string { return q.Gamma }),
		Theta: pick(func(q *compute.StructureQuote) string { return q.Theta }),
		Vega:  pick(func(q *compute.StructureQuote) string { return q.Vega }),
		Day:   day,
	}
}
