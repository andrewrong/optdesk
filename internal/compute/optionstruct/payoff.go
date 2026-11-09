package optionstruct

import (
	"math/big"
	"slices"

	"github.com/andrewrong/optdesk/internal/compute"
)

// payoff is the piecewise linear payoff at expiry (Decision 8): the corners are the strikes of
// the legs plus a margin of 20 percent of the top strike on both sides (the low side stops
// at zero), so at most 2 plus legs points. A covered call includes its shares at the basis.
// An `other` unit has none: its legs need not share an expiry.
func payoff(u unit) []compute.PayoffPoint {
	if u.kind == kindOther || (u.kind == kindCovered && u.basis == nil) {
		return nil
	}
	top := u.legs[len(u.legs)-1].strike
	margin := mul(top, big.NewRat(1, 5))
	prices := []*big.Rat{new(big.Rat), add(top, margin)}
	if low := sub(u.legs[0].strike, margin); low.Sign() > 0 {
		prices[0] = low
	}
	for _, l := range u.legs {
		prices = append(prices, l.strike)
	}
	slices.SortFunc(prices, func(a, b *big.Rat) int { return a.Cmp(b) })
	prices = slices.CompactFunc(prices, func(a, b *big.Rat) bool { return a.Cmp(b) == 0 })
	out := make([]compute.PayoffPoint, 0, len(prices))
	for _, px := range prices {
		out = append(out, compute.PayoffPoint{Price: fixed4(px), PnL: fixed4(pnlAt(u, px))})
	}
	return out
}

// pnlAt is the profit of the unit at expiry if the underlying closes at px.
func pnlAt(u unit, px *big.Rat) *big.Rat {
	total := new(big.Rat)
	for _, l := range u.legs {
		intrinsic := sub(px, l.strike)
		if l.p.Right == "P" {
			intrinsic = sub(l.strike, px)
		}
		if intrinsic.Sign() < 0 {
			intrinsic = new(big.Rat)
		}
		total = add(total, mul(mul(l.qty, l.mult), sub(intrinsic, l.price)))
	}
	if u.kind == kindCovered {
		l := u.legs[0]
		total = add(total, mul(scale(l), sub(px, u.basis)))
	}
	return total
}
