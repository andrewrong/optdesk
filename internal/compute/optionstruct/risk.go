package optionstruct

import (
	"math/big"
	"slices"

	"github.com/andrewrong/optdesk/internal/compute"
)

// figures are the three figures that need no mark.
type figures struct{ maxLoss, maxGain, breakEven compute.FigureValue }

func val(r *big.Rat) compute.FigureValue { return compute.FigureValue{Value: fixed4(r)} }

var (
	noRule  = compute.FigureValue{Gap: gapNoRule}
	missing = compute.FigureValue{Gap: gapMissing}
)

// scale is quantity times multiplier of a leg, unsigned.
func scale(l leg) *big.Rat { return mul(new(big.Rat).Abs(l.qty), l.mult) }

// premium is the opening price times multiplier times contracts of a leg, unsigned.
func premium(l leg) *big.Rat { return mul(l.price, scale(l)) }

// risk gives the corners of the expiry payoff and the break-even of a unit (Decision 6).
// figure: structure.max_loss and figure: structure.max_gain (the break-even has no register id yet).
// An "other" unit and an unbounded side have no rule.
func risk(u unit) figures {
	l := u.legs[0]
	switch u.kind {
	case kindShortPut:
		return figures{maxLoss: val(new(big.Rat).Neg(sub(mul(l.strike, scale(l)), premium(l)))),
			maxGain: val(premium(l)), breakEven: val(sub(l.strike, l.price))}
	case kindLongPut:
		return figures{maxLoss: val(new(big.Rat).Neg(premium(l))),
			maxGain: val(sub(mul(l.strike, scale(l)), premium(l))), breakEven: val(sub(l.strike, l.price))}
	case kindLongCall:
		return figures{maxLoss: val(new(big.Rat).Neg(premium(l))), maxGain: noRule, breakEven: val(add(l.strike, l.price))}
	case kindNakedCall:
		return figures{maxLoss: noRule, maxGain: val(premium(l)), breakEven: val(add(l.strike, l.price))}
	case kindCovered:
		return coveredRisk(u)
	}
	if len(u.legs) >= 2 && u.kind != kindOther && sameExpiry(u) {
		return payoffFigures(u)
	}
	return figures{noRule, noRule, noRule}
}

// coveredRisk, figure: structure.max_gain of a covered call: no max loss number; max gain is the strike minus the basis of the cycle, times
// shares, plus the premium, which is never deducted from the basis; no break-even rule.
func coveredRisk(u unit) figures {
	l := u.legs[0]
	f := figures{maxLoss: noRule, maxGain: missing, breakEven: noRule}
	if u.basis != nil {
		f.maxGain = val(add(mul(sub(l.strike, u.basis), scale(l)), premium(l)))
	}
	return f
}

// payoffFigures, figure: structure.max_loss of a unit of two or more legs of one expiry, by data and not by kind
// (Decision 9). The payoff at expiry is piecewise linear, so max loss and max gain are the lowest and
// highest value over the corners (each strike and price 0). A side that grows without end is no_rule: the slope
// above the top strike is the signed call quantity, negative for loss and positive for gain. The break-even is
// the one price where the payoff crosses zero, else no_rule.
func payoffFigures(u unit) figures {
	prices := []*big.Rat{new(big.Rat)}
	slope := new(big.Rat)
	top := new(big.Rat)
	for _, l := range u.legs {
		prices = append(prices, l.strike)
		if l.strike.Cmp(top) > 0 {
			top = l.strike
		}
		if l.p.Right == "C" {
			slope = add(slope, mul(l.qty, l.mult))
		}
	}
	slices.SortFunc(prices, func(a, b *big.Rat) int { return a.Cmp(b) })
	prices = slices.CompactFunc(prices, func(a, b *big.Rat) bool { return a.Cmp(b) == 0 })
	vals := make([]*big.Rat, len(prices))
	for i, px := range prices {
		vals[i] = pnlAt(u, px)
	}
	lo, hi := vals[0], vals[0]
	for _, v := range vals {
		if v.Cmp(lo) < 0 {
			lo = v
		}
		if v.Cmp(hi) > 0 {
			hi = v
		}
	}
	f := figures{maxLoss: val(lo), maxGain: val(hi), breakEven: breakEven(prices, vals, slope)}
	if slope.Sign() < 0 {
		f.maxLoss = noRule
	}
	if slope.Sign() > 0 {
		f.maxGain = noRule
	}
	return f
}

// breakEven finds the zero crossings of the payoff given at the sorted prices, with the slope
// beyond the last one. Exactly one crossing gives its price; none, several or a payoff that is
// zero over an interval give no_rule.
func breakEven(prices, vals []*big.Rat, slope *big.Rat) compute.FigureValue {
	last := len(prices) - 1
	if slope.Sign() != 0 {
		// a far point on the tail, past any crossing of it
		reach := new(big.Rat).Quo(new(big.Rat).Abs(vals[last]), new(big.Rat).Abs(slope))
		prices = append(slices.Clone(prices), add(prices[last], add(reach, big.NewRat(1, 1))))
		vals = append(slices.Clone(vals), add(vals[last], mul(slope, sub(prices[last+1], prices[last]))))
	} else {
		prices = append(slices.Clone(prices), add(prices[last], big.NewRat(1, 1)))
		vals = append(slices.Clone(vals), vals[last])
	}
	var found []*big.Rat
	prev := -1 // index of the last point with a non-zero value
	for i, v := range vals {
		if v.Sign() == 0 {
			continue
		}
		if prev >= 0 && vals[prev].Sign() != v.Sign() {
			zeros := i - prev - 1
			switch {
			case zeros > 1:
				return noRule
			case zeros == 1:
				found = append(found, prices[prev+1])
			default:
				// linear between the two points
				d := sub(vals[prev], v)
				t := new(big.Rat).Quo(vals[prev], d)
				found = append(found, add(prices[prev], mul(t, sub(prices[i], prices[prev]))))
			}
		}
		prev = i
	}
	if prev < 0 || len(found) != 1 {
		return noRule
	}
	return val(found[0])
}

// sameExpiry reports whether all legs of the unit expire on one day.
func sameExpiry(u unit) bool {
	return !slices.ContainsFunc(u.legs, func(l leg) bool { return l.p.Expiry != u.legs[0].p.Expiry })
}

// levels are the strikes and the break-even of a unit, without distance. A break-even that
// is a gap gives no level.
func levels(u unit, f figures) []compute.StructureLevel {
	var out []compute.StructureLevel
	for _, l := range u.legs {
		name := "strike"
		if len(u.legs) > 1 {
			name = "long_strike"
			if l.short() {
				name = "short_strike"
			}
		}
		out = append(out, compute.StructureLevel{Name: name, Price: l.p.Strike})
	}
	if f.breakEven.Gap == "" {
		out = append(out, compute.StructureLevel{Name: "break_even", Price: f.breakEven.Value})
	}
	return out
}
