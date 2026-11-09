package optionstruct

import (
	"math/big"

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
	case kindPutSpread, kindCallSpread:
		return spreadRisk(u)
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

// spreadRisk, figure: structure.max_loss of a vertical spread: max gain is the net credit, max loss is the net credit minus the width.
func spreadRisk(u unit) figures {
	s, b := u.legs[0], u.legs[1] // ordered by strike
	if !s.short() {
		s, b = b, s
	}
	perShare := sub(s.price, b.price)
	credit := mul(perShare, scale(s))
	width := mul(new(big.Rat).Abs(sub(s.strike, b.strike)), scale(s))
	be := sub(s.strike, perShare)
	if s.p.Right == "C" {
		be = add(s.strike, perShare)
	}
	return figures{maxLoss: val(sub(credit, width)), maxGain: val(credit), breakEven: val(be)}
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
