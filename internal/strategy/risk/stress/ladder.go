package stress

import (
	"math/big"

	"github.com/andrewrong/optdesk/internal/strategy"
)

// The ranges of the portfolio-margin valuation, as fractions of the spot (low end, high end).
const (
	// Rule 4210(g)(2)(F) [V]: high-capitalization broad-based index, +6 percent / -8 percent.
	broadHighLow, broadHighHigh = "-0.08", "0.06"
	// Rule 4210(g)(2)(F) [V]: non-high-capitalization broad-based index, +-10 percent.
	broadOtherLow, broadOtherHigh = "-0.10", "0.10"
	// Rule 4210(g)(2)(F) [V]: equity, narrow-based index and other equity-based products, +-15 percent.
	equityLow, equityHigh = "-0.15", "0.15"
)

// intervals is the number of equal intervals of a range (Rule 4210(g)(2)(F) [V]); a ladder has intervals+1 points.
const intervals = 10

// bounds is the range of a class; ok is false for a class that is none of the three.
func bounds(c strategy.RiskClass) (low, high *big.Rat, ok bool) {
	var l, h string
	switch c {
	case "broad_high":
		l, h = broadHighLow, broadHighHigh
	case "broad_other":
		l, h = broadOtherLow, broadOtherHigh
	case "equity":
		l, h = equityLow, equityHigh
	default:
		return nil, nil, false
	}
	low, _ = new(big.Rat).SetString(l)
	high, _ = new(big.Rat).SetString(h)
	return low, high, true
}

// moves are the eleven points low + k * (high - low) / 10, k = 0..10, ends included.
func moves(low, high *big.Rat) []*big.Rat {
	step := new(big.Rat).Sub(high, low)
	step.Quo(step, big.NewRat(intervals, 1))
	out := make([]*big.Rat, intervals+1)
	for k := range out {
		m := new(big.Rat).Mul(big.NewRat(int64(k), 1), step)
		out[k] = m.Add(m, low)
	}
	return out
}
