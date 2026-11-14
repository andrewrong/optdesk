package fit

import (
	"math/big"

	"github.com/andrewrong/optdesk/internal/strategy"
)

// reasonNoRule is the reason of a structure no formula of Decision 7 covers, or whose numbers are unusable.
const reasonNoRule = "no_rule"

var (
	pctTwenty  = big.NewRat(20, 100)
	pctFifteen = big.NewRat(15, 100)
	pctTen     = big.NewRat(10, 100)
)

// capital is the capital of one candidate by the formulas of docs/design/strategy-risk.md, Decision 7.
// It returns ok false with the reason no_rule for a structure none of them covers. All arithmetic is Rat.
func capital(c strategy.RiskCandidate, book strategy.RiskBook) (*big.Rat, string, bool) {
	var opts, shares []strategy.RiskHolding
	for _, l := range c.Legs {
		if l.Option == nil {
			shares = append(shares, l)
		} else {
			opts = append(opts, l)
		}
	}
	switch {
	case len(opts) == 1 && opts[0].Option != nil:
		return singleShort(c.Class, opts[0], shares, book)
	case len(opts) == 2 && len(shares) == 0:
		return putCreditSpread(opts[0], opts[1])
	}
	return nil, reasonNoRule, false
}

func singleShort(class strategy.RiskClass, o strategy.RiskHolding, shares []strategy.RiskHolding, book strategy.RiskBook) (*big.Rat, string, bool) {
	qty, mult, strike, spot, ok := terms(o)
	if !ok || qty.Sign() >= 0 {
		return nil, reasonNoRule, false
	}
	qty.Neg(qty)
	units := new(big.Rat).Mul(mult, qty) // underlying units covered by the short
	isPut := o.Option.Right == "P"
	isCall := o.Option.Right == "C"
	if !isPut && !isCall {
		return nil, reasonNoRule, false
	}
	otm := new(big.Rat)
	if isCall && strike.Cmp(spot) > 0 {
		otm.Sub(strike, spot)
	}
	if isPut && spot.Cmp(strike) > 0 {
		otm.Sub(spot, strike)
	}
	underlying := spot // the base of the percentages; a put's minimum uses the strike
	floorBase := spot
	if isPut {
		floorBase = strike
	}
	index := class == "broad_high" || class == "broad_other"
	switch {
	case index:
		// short broad-index option: 15 percent of the index value less the out-of-the-money amount, minimum
		// 10 percent (of the index value, of the strike for a put) [V, Cboe, FINRA 4210], per unit times units.
		per := new(big.Rat).Mul(pctFifteen, underlying)
		per.Sub(per, otm)
		per = maxRat(per, new(big.Rat).Mul(pctTen, floorBase))
		return per.Mul(per, units), "", true
	case isCall && coveredBy(o, shares, book, units):
		// covered call: nil, the shares are the cover [arithmetic].
		return new(big.Rat), "", true
	case isPut:
		// cash-secured short put: strike times multiplier times quantity [V, Cboe strategy-based margin page].
		return new(big.Rat).Mul(strike, units), "", true
	}
	// short equity option, uncovered: option value plus 20 percent of the underlying less the out-of-the-money
	// amount, floor option value plus 10 percent of the underlying [V for 20 and 10 percent; the OTM deduction
	// [S]], per unit times units.
	mark, ok := rat(o.Mark)
	if !ok {
		return nil, reasonNoRule, false
	}
	per := new(big.Rat).Mul(pctTwenty, underlying)
	per.Sub(per, otm)
	per = maxRat(per, new(big.Rat).Mul(pctTen, floorBase))
	per.Add(per, mark)
	return per.Mul(per, units), "", true
}

// coveredBy reports whether shares of the same underlying, in the candidate or the book, cover units.
func coveredBy(o strategy.RiskHolding, cand []strategy.RiskHolding, book strategy.RiskBook, units *big.Rat) bool {
	have := new(big.Rat)
	add := func(hs []strategy.RiskHolding) {
		for _, h := range hs {
			if h.Option != nil || h.Underlying != o.Underlying || h.Currency != o.Currency {
				continue
			}
			if q, ok := rat(h.Quantity); ok && q.Sign() > 0 {
				have.Add(have, q)
			}
		}
	}
	add(cand)
	add(book.Holdings)
	return have.Cmp(units) >= 0
}

// putCreditSpread is a short put and a long put of one underlying, expiry and size: width less credit,
// times multiplier [arithmetic].
func putCreditSpread(a, b strategy.RiskHolding) (*big.Rat, string, bool) {
	qa, ma, ka, _, ok1 := terms(a)
	qb, mb, kb, _, ok2 := terms(b)
	if !ok1 || !ok2 || a.Option.Right != "P" || b.Option.Right != "P" || a.Underlying != b.Underlying ||
		a.Option.Expiry != b.Option.Expiry || ma.Cmp(mb) != 0 {
		return nil, reasonNoRule, false
	}
	if qa.Sign() > 0 {
		a, b, qa, qb, ka, kb = b, a, qb, qa, kb, ka
	}
	if qa.Sign() >= 0 || qb.Sign() <= 0 || new(big.Rat).Add(qa, qb).Sign() != 0 || ka.Cmp(kb) <= 0 {
		return nil, reasonNoRule, false
	}
	ms, ok1 := rat(a.Mark)
	ml, ok2 := rat(b.Mark)
	if !ok1 || !ok2 {
		return nil, reasonNoRule, false
	}
	per := new(big.Rat).Sub(ka, kb)
	per.Sub(per, new(big.Rat).Sub(ms, ml)) // width less credit
	per = maxRat(per, new(big.Rat))
	return per.Mul(per, new(big.Rat).Mul(ma, qb)), "", true
}

func terms(h strategy.RiskHolding) (qty, mult, strike, spot *big.Rat, ok bool) {
	var o1, o2, o3, o4 bool
	qty, o1 = rat(h.Quantity)
	mult, o2 = rat(h.Multiplier)
	strike, o3 = rat(h.Option.Strike)
	spot, o4 = rat(h.Spot)
	return qty, mult, strike, spot, o1 && o2 && o3 && o4
}

func rat(s string) (*big.Rat, bool) {
	r, ok := new(big.Rat).SetString(s)
	return r, ok && s != ""
}

func maxRat(a, b *big.Rat) *big.Rat {
	if a.Cmp(b) >= 0 {
		return a
	}
	return b
}
